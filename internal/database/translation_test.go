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
	fullCandidate("")
	if _, err := store.db.ExecContext(ctx, "UPDATE articles SET summary='' WHERE id=?", articleID); err != nil {
		t.Fatal(err)
	}
	fullCandidate(articleID)
	if _, err := store.db.ExecContext(ctx, "UPDATE articles SET summary=? WHERE id=?", articleInput.Summary, articleID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, "UPDATE article_translations SET status='completed' WHERE article_id=?", articleID); err != nil {
		t.Fatal(err)
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
	doc := ReaderDocument{HTML: "<p>Full source.</p>", Hash: articleInput.ContentHash, Version: "test"}
	if err := store.SaveReaderDocument(ctx, articleID, doc); err != nil {
		t.Fatal(err)
	}
	current, err := store.ReaderDocument(ctx, articleID)
	if err != nil || current.SourceVersion == "" {
		t.Fatalf("current reader source: %+v, err=%v", current, err)
	}
	body.SourceVersion = current.SourceVersion
	if err := store.SaveArticleTranslation(ctx, body); err != nil {
		t.Fatal(err)
	}
	fullCandidate(articleID)
	body.TranslatedBlocks = 2
	if err := store.SaveArticleTranslation(ctx, body); err != nil {
		t.Fatal(err)
	}
	fullCandidate("")
	doc.HTML = "<p>Full source has changed.</p>"
	if err := store.SaveReaderDocument(ctx, articleID, doc); err != nil {
		t.Fatal(err)
	}
	fullCandidate(articleID)
	if err := store.SaveCurrentArticleTranslation(ctx, body); err != ErrConflict {
		t.Fatalf("an old source write should conflict, got %v", err)
	}

	if err := store.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	// 模拟存量库（撤销 00026 版本记录并移除 00026 新增对象），验证 Open 触发备份 + 升级 + 保留既有数据。
	rawDB, err := sql.Open("sqlite", formatSQLiteFileDSN(dbPath, ""))
	if err != nil {
		t.Fatalf("open raw sqlite: %v", err)
	}
	stmts := append(dropArticleSummarySchema(), []string{
		`DROP TABLE article_translations`,
		`ALTER TABLE reader_contents DROP COLUMN source_version`,
		`ALTER TABLE feeds DROP COLUMN auto_translate_mode`,
		`ALTER TABLE article_ai_settings DROP COLUMN auto_translate`,
		`ALTER TABLE article_ai_settings DROP COLUMN auto_translate_scope`,
		`ALTER TABLE article_ai_settings DROP COLUMN translate_on_open`,
		`ALTER TABLE article_ai_settings DROP COLUMN translation_target_lang`,
		`ALTER TABLE article_ai_settings DROP COLUMN translation_prompt`,
		`ALTER TABLE article_ai_settings DROP COLUMN translation_batch_kib`,
		`ALTER TABLE article_ai_settings DROP COLUMN translation_engine`,
		`DROP TABLE translation_providers`,
		"DROP INDEX conversations_group",
		"ALTER TABLE conversations DROP COLUMN workspace_day",
		"ALTER TABLE conversations DROP COLUMN group_id",
		"DROP TABLE chat_group_folders",
		"DROP TABLE chat_groups",
		"ALTER TABLE messages DROP COLUMN attachments",
		"ALTER TABLE messages DROP COLUMN created_at",
		"ALTER TABLE messages DROP COLUMN selected_skills",
		"DELETE FROM goose_db_version WHERE version_id IN (26,27,29,30,33,34,35,36)",
	}...)
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
	if settingsRow.AutoTranslate != 0 || settingsRow.AutoTranslateScope != "summary" || settingsRow.TranslateOnOpen != 0 || settingsRow.TranslationTargetLang != "zh-CN" {
		t.Fatalf("unexpected upgraded defaults: %+v", settingsRow)
	}
	upgradedSub, err := upgraded.GetSubscription(ctx, feedID)
	if err != nil || upgradedSub.AutoTranslateMode != "global" {
		t.Fatalf("expected default auto_translate_mode=global after upgrade, got %+v err=%v", upgradedSub, err)
	}
}

func TestTranslationCheckpointMigrationPreservesV26Cache(t *testing.T) {
	ctx := context.Background()
	file := filepath.Join(t.TempDir(), "v26.db")
	store, err := Open(ctx, file)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UnixMilli()
	_, err = store.AddSubscription(ctx, SubscriptionInput{URL: "https://example.test/feed", Title: "旧库"}, []ArticleInput{
		{GUID: "original", Title: "Old title", HTML: "<p>Original text.</p>", FetchedAt: now},
	}, FeedCheck{At: now, Next: now + 60000})
	if err != nil {
		t.Fatal(err)
	}
	articles, err := store.ListArticles(ctx, 1)
	if err != nil || len(articles) != 1 {
		t.Fatalf("load article: %+v %v", articles, err)
	}
	id := articles[0].ID
	article, err := store.GetArticleForTranslation(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveReaderDocument(ctx, id, ReaderDocument{HTML: "<p>Original text.</p>", Hash: article.ContentHash, Version: "test"}); err != nil {
		t.Fatal(err)
	}
	blocks := `[{"id":"b0","sourceHash":"previous","translatedHtml":"已有译文"}]`
	if err := store.SaveArticleTranslation(ctx, sqlc.SaveArticleTranslationParams{
		ArticleID: id, TargetLang: "zh-CN", SourceMode: "reader", ContentHash: article.ContentHash,
		Status: "completed", BlocksJson: blocks, TranslatedBlocks: 1, TotalBlocks: 1, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	for _, statement := range append(dropArticleSummarySchema(), []string{
		"ALTER TABLE article_translations DROP COLUMN source_version",
		"ALTER TABLE article_translations DROP COLUMN checkpoint_json",
		"ALTER TABLE reader_contents DROP COLUMN source_version",
		"ALTER TABLE article_ai_settings DROP COLUMN translation_batch_kib",
		"ALTER TABLE article_ai_settings DROP COLUMN translation_engine",
		"DROP TABLE translation_providers",
		"DROP INDEX conversations_group",
		"ALTER TABLE conversations DROP COLUMN workspace_day",
		"ALTER TABLE conversations DROP COLUMN group_id",
		"DROP TABLE chat_group_folders",
		"DROP TABLE chat_groups",
		"ALTER TABLE messages DROP COLUMN attachments",
		"ALTER TABLE messages DROP COLUMN created_at",
		"ALTER TABLE messages DROP COLUMN selected_skills",
		"DELETE FROM goose_db_version WHERE version_id IN (27,29,30,33,34,35,36)",
	}...) {
		if _, err := store.db.ExecContext(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	upgraded, err := Open(ctx, file)
	if err != nil {
		t.Fatal(err)
	}
	defer upgraded.Close()
	snapshots, err := filepath.Glob(file + ".backups/before-v26-*.db")
	if err != nil || len(snapshots) != 1 {
		t.Fatalf("missing pre-upgrade snapshot: %v %v", snapshots, err)
	}
	row, err := upgraded.GetArticleTranslation(ctx, id, "zh-CN", "reader")
	if err != nil || row.BlocksJson != blocks || row.CheckpointJson != "{}" || row.SourceVersion != "" {
		t.Fatalf("legacy result changed: %+v %v", row, err)
	}
	doc, err := upgraded.ReaderDocument(ctx, id)
	if err != nil || doc.SourceVersion == "" || doc.HTML != "<p>Original text.</p>" {
		t.Fatalf("legacy reader source: %+v %v", doc, err)
	}
	source, err := upgraded.CurrentTranslationSource(ctx, id)
	if err != nil || source.ReaderSourceVersion != doc.SourceVersion {
		t.Fatalf("source version not filled durably: %+v %v", source, err)
	}
}
