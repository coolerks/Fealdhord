package database

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/coolerks/Fealdhord/internal/database/sqlc"
	"github.com/pressly/goose/v3"
)

func dropArticleSummarySchema() []string {
	return []string{
		"DROP TABLE article_summaries",
		"ALTER TABLE articles DROP COLUMN feed_content_explicit",
		"ALTER TABLE article_ai_settings DROP COLUMN summary_english_limit",
		"ALTER TABLE article_ai_settings DROP COLUMN summary_han_limit",
		"ALTER TABLE article_ai_settings DROP COLUMN summarize_on_open",
		"DELETE FROM goose_db_version WHERE version_id=28",
	}
}

func TestArticleSummaryUpgradeAndPersistence(t *testing.T) {
	ctx := t.Context()
	filename := filepath.Join(t.TempDir(), "library.db")
	old, err := openWithMigrations(ctx, filename, sqlMigrationsBefore(t, "00028"), goose.WithGoMigrations(
		goose.NewGoMigration(21, &goose.GoFunc{RunTx: repairNotificationSchema}, nil),
		goose.NewGoMigration(22, &goose.GoFunc{RunTx: repairAISchema}, nil),
	))
	if err != nil {
		t.Fatal(err)
	}
	_, err = old.db.ExecContext(ctx, `
		INSERT INTO feeds (id,title,url,normalized_url,created_at,updated_at)
		VALUES ('source','订阅','https://example.test/feed','https://example.test/feed',1,1);
		INSERT INTO articles(id,feed_id,title,summary,content_text,fetched_at,sort_at,content_hash,dedup_key)
		VALUES ('old','source','旧文章','短摘录','短摘录',1,1,'old-version','guid:old');
		INSERT INTO article_translations(article_id,target_lang,source_mode,content_hash,status,translated_summary,updated_at)
		VALUES ('old','en','summary','old-version','completed','Existing translation',1);
	`)
	closeErr := old.Close()
	if err != nil || closeErr != nil {
		t.Fatalf("旧库准备失败：%v %v", err, closeErr)
	}
	store, err := Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	backup, err := filepath.Glob(filepath.Join(filename+".backups", "before-v27-*.db"))
	if err != nil || len(backup) != 1 {
		t.Fatalf("升级备份：%v %v", backup, err)
	}
	settings, err := store.ArticleAISettings(ctx)
	if err != nil || settings.SummarizeOnOpen != 0 || settings.SummaryHanLimit != 600 || settings.SummaryEnglishLimit != 350 {
		t.Fatalf("摘要默认设置：%+v %v", settings, err)
	}
	translation, err := store.GetArticleTranslation(ctx, "old", "en", "summary")
	if err != nil || translation.TranslatedSummary != "Existing translation" {
		t.Fatalf("旧译文丢失：%+v %v", translation, err)
	}
	if _, err := store.ArticleSummarySource(ctx, "old", "feed", "old-version"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("旧 RSS 摘录误判为全文：%v", err)
	}
	input := ArticleInput{FeedID: "source", GUID: "new", Title: "全文", Text: "文章全文", FeedContentExplicit: true}
	id, err := store.UpsertArticle(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	version := articleHash(input)
	record := sqlc.SaveArticleSummaryParams{ArticleID: id, SourceMode: "feed", SourceVersion: version, Status: "skipped_length", HanLimit: 600, EnglishLimit: 350}
	if err := store.SaveCurrentArticleSummary(ctx, record); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	saved, err := reopened.GetArticleSummary(ctx, sqlc.GetArticleSummaryParams{ArticleID: id, SourceMode: "feed", SourceVersion: version})
	if err != nil || saved.Status != record.Status || saved.HanLimit != record.HanLimit {
		t.Fatalf("重开未保留结果：%+v %v", saved, err)
	}
	input.Text = "更新后的全文"
	if _, err := reopened.UpsertArticle(ctx, input); err != nil {
		t.Fatal(err)
	}
	if err := reopened.SaveCurrentArticleSummary(ctx, record); !errors.Is(err, ErrConflict) {
		t.Fatalf("允许旧结果写入：%v", err)
	}
}
