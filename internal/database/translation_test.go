package database

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/coolerks/Fealdhord/internal/database/sqlc"
)

func TestArticleTranslationStoreAndUpgrade(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "fealdhord.db")

	// 先建立到最新库并模拟回退到 00025 之前的结构，再验证存量升级路径。
	store, err := Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("open fresh db: %v", err)
	}
	now := time.Now().UTC().UnixMilli()
	feedID, err := store.AddSubscription(ctx, SubscriptionInput{
		URL:               "https://example.com/feed.xml",
		Title:             "Example Feed",
		AutoTranslateMode: "on",
	}, []ArticleInput{{
		GUID:        "g1",
		Title:       "Hello World",
		URL:         "https://example.com/1",
		Summary:     "A short English summary.",
		HTML:        "<p>First paragraph.</p>",
		Text:        "First paragraph.",
		PublishedAt: &now,
		FetchedAt:   now,
	}}, FeedCheck{At: now, Next: now + 60000})
	if err != nil {
		t.Fatalf("add subscription: %v", err)
	}
	sub, err := store.GetSubscription(ctx, feedID)
	if err != nil || sub.AutoTranslateMode != "on" {
		t.Fatalf("expected auto_translate_mode=on, got %+v err=%v", sub, err)
	}

	articles, err := store.ListArticles(ctx, 10)
	if err != nil || len(articles) != 1 {
		t.Fatalf("list articles: %v len=%d", err, len(articles))
	}
	articleID := articles[0].ID

	// 模拟未开启全局开关但单源为 on 时，NextArticleForTranslation 应命中该文章。
	nextID, scope, targetLang, err := store.NextArticleForTranslation(ctx, time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("next article for translation: %v", err)
	}
	if nextID != articleID || scope != "summary" || targetLang != "zh-CN" {
		t.Fatalf("unexpected next translation: id=%q scope=%q lang=%q", nextID, scope, targetLang)
	}

	articleInput, err := store.GetArticleForTranslation(ctx, articleID)
	if err != nil {
		t.Fatalf("get article for translation: %v", err)
	}
	if err := store.SaveArticleTranslation(ctx, sqlc.SaveArticleTranslationParams{
		ArticleID:         articleID,
		TargetLang:        "zh-CN",
		SourceMode:        "summary",
		ContentHash:       articleInput.ContentHash,
		DetectedLang:      "en",
		Status:            "completed",
		TranslatedTitle:   "你好，世界",
		TranslatedSummary: "一段简短的英文摘要。",
		BlocksJson:        "[]",
		ModelID:           "m1",
		InputTokens:       sql.NullInt64{Int64: 100, Valid: true},
		OutputTokens:      sql.NullInt64{Int64: 50, Valid: true},
		UpdatedAt:         now,
	}); err != nil {
		t.Fatalf("save translation: %v", err)
	}

	// 批量摘要读取应命中。
	summaries, err := store.ArticleTranslationSummaries(ctx, []string{articleID})
	if err != nil {
		t.Fatalf("summaries: %v", err)
	}
	if got := summaries[articleID]; got.Title != "你好，世界" || got.Summary != "一段简短的英文摘要。" {
		t.Fatalf("unexpected summary map: %+v", got)
	}

	// 已完成 summary 且 scope=summary 时，不应再重复调度。
	nextID, _, _, err = store.NextArticleForTranslation(ctx, time.Now().Add(-24*time.Hour))
	if err != nil || nextID != "" {
		t.Fatalf("expected no next translation, got id=%q err=%v", nextID, err)
	}

	if _, err := store.db.ExecContext(ctx, "UPDATE article_ai_settings SET auto_translate_scope='full'"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, "UPDATE article_translations SET status='skipped_same_lang' WHERE article_id=?", articleID); err != nil {
		t.Fatal(err)
	}
	fullCandidate := func(wantID string) {
		t.Helper()
		id, scope, _, err := store.NextArticleForTranslation(ctx, time.Now().Add(-24*time.Hour))
		if err != nil || id != wantID || (wantID != "" && scope != "full") {
			t.Fatalf("full candidate = %q (%q), want %q, err=%v", id, scope, wantID, err)
		}
	}
	body := sqlc.SaveArticleTranslationParams{
		ArticleID: articleID, TargetLang: "zh-CN", SourceMode: "feed", ContentHash: articleInput.ContentHash,
		Status: "completed", TranslatedBlocks: 1, TotalBlocks: 1, BlocksJson: "[]", UpdatedAt: now,
	}
	if err := store.SaveArticleTranslation(ctx, body); err != nil {
		t.Fatal(err)
	}
	fullCandidate(articleID)
	if err := store.DeferArticleAI(ctx, articleID, "translation_full", time.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	fullCandidate("")
	if err := store.DeferArticleAI(ctx, articleID, "translation_full", time.Now().Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	body.SourceMode, body.TotalBlocks = "reader", 2
	if err := store.SaveArticleTranslation(ctx, body); err != nil {
		t.Fatal(err)
	}
	fullCandidate(articleID)
	body.TranslatedBlocks = 2
	if err := store.SaveArticleTranslation(ctx, body); err != nil {
		t.Fatal(err)
	}
	fullCandidate("")

	if err := store.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	// 模拟存量库（撤销 00026 版本记录并移除 00026 新增对象），验证 Open 触发备份 + 升级 + 保留既有数据。
	rawDB, err := sql.Open("sqlite", formatSQLiteFileDSN(dbPath, ""))
	if err != nil {
		t.Fatalf("open raw sqlite: %v", err)
	}
	stmts := []string{
		`DROP TABLE article_translations`,
		`ALTER TABLE feeds DROP COLUMN auto_translate_mode`,
		`ALTER TABLE article_ai_settings DROP COLUMN auto_translate`,
		`ALTER TABLE article_ai_settings DROP COLUMN auto_translate_scope`,
		`ALTER TABLE article_ai_settings DROP COLUMN translate_on_open`,
		`ALTER TABLE article_ai_settings DROP COLUMN translation_target_lang`,
		`ALTER TABLE article_ai_settings DROP COLUMN translation_prompt`,
		`DELETE FROM goose_db_version WHERE version_id=26`,
	}
	for _, stmt := range stmts {
		if _, err := rawDB.ExecContext(ctx, stmt); err != nil {
			rawDB.Close()
			t.Fatalf("downgrade stmt %q: %v", stmt, err)
		}
	}
	rawDB.Close()

	upgraded, err := Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("upgrade from v25 to v26: %v", err)
	}
	defer upgraded.Close()

	settingsRow, err := upgraded.ArticleAISettings(ctx)
	if err != nil {
		t.Fatalf("read upgraded settings: %v", err)
	}
	if settingsRow.AutoTranslate != 0 || settingsRow.AutoTranslateScope != "summary" || settingsRow.TranslateOnOpen != 1 || settingsRow.TranslationTargetLang != "zh-CN" {
		t.Fatalf("unexpected upgraded defaults: %+v", settingsRow)
	}
	upgradedSub, err := upgraded.GetSubscription(ctx, feedID)
	if err != nil || upgradedSub.AutoTranslateMode != "global" {
		t.Fatalf("expected default auto_translate_mode=global after upgrade, got %+v err=%v", upgradedSub, err)
	}
}
