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
		"DELETE FROM goose_db_version WHERE version_id IN (28,31,32)",
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
	if err != nil || settings.SummarizeOnOpen != 1 || settings.SummaryHanLimit != 600 || settings.SummaryEnglishLimit != 350 {
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

func TestArticleAIOpenDefaults(t *testing.T) {
	t.Run("新库", func(t *testing.T) {
		ctx := t.Context()
		store, err := Open(ctx, filepath.Join(t.TempDir(), "library.db"))
		if err != nil {
			t.Fatal(err)
		}
		defer store.Close()
		settings, err := store.ArticleAISettings(ctx)
		if err != nil || settings.TranslateOnOpen != 0 || settings.SummarizeOnOpen != 1 {
			t.Fatalf("新库打开文章默认设置：%+v %v", settings, err)
		}
	})

	t.Run("升级保留已保存设置", func(t *testing.T) {
		ctx := t.Context()
		filename := filepath.Join(t.TempDir(), "library.db")
		old, err := openWithMigrations(ctx, filename, sqlMigrationsBefore(t, "00032"), goose.WithGoMigrations(
			goose.NewGoMigration(21, &goose.GoFunc{RunTx: repairNotificationSchema}, nil),
			goose.NewGoMigration(22, &goose.GoFunc{RunTx: repairAISchema}, nil),
		))
		if err != nil {
			t.Fatal(err)
		}
		_, err = old.db.ExecContext(ctx, `UPDATE article_ai_settings SET revision=2, translate_on_open=1,
 summarize_on_open=0, requests_per_hour=60, tokens_per_hour=10000 WHERE id=1`)
		if closeErr := old.Close(); err != nil || closeErr != nil {
			t.Fatalf("保存旧库设置：%v %v", err, closeErr)
		}
		upgraded, err := Open(ctx, filename)
		if err != nil {
			t.Fatal(err)
		}
		defer upgraded.Close()
		settings, err := upgraded.ArticleAISettings(ctx)
		if err != nil || settings.Revision != 2 || settings.TranslateOnOpen != 1 || settings.SummarizeOnOpen != 0 ||
			settings.RequestsPerHour != 60 || settings.TokensPerHour != 10000 || settings.TranslationBatchKib != 0 {
			t.Fatalf("升级后设置：%+v %v", settings, err)
		}
		backups, err := filepath.Glob(filepath.Join(filename+".backups", "before-v31-*.db"))
		if err != nil || len(backups) != 1 {
			t.Fatalf("升级前备份：%v %v", backups, err)
		}
	})
}

func TestSummaryLanguageMigrationKeepsOldTextAndRejectsStaleLanguage(t *testing.T) {
	ctx := t.Context()
	filename := filepath.Join(t.TempDir(), "library.db")
	old, err := openWithMigrations(ctx, filename, sqlMigrationsBefore(t, "00031"), goose.WithGoMigrations(
		goose.NewGoMigration(21, &goose.GoFunc{RunTx: repairNotificationSchema}, nil),
		goose.NewGoMigration(22, &goose.GoFunc{RunTx: repairAISchema}, nil),
	))
	if err != nil {
		t.Fatal(err)
	}
	_, err = old.db.ExecContext(ctx, `
		INSERT INTO feeds (id,title,url,normalized_url,created_at,updated_at)
		VALUES ('source','订阅','https://example.test/feed','https://example.test/feed',1,1);
		INSERT INTO articles(id,feed_id,title,summary,content_text,fetched_at,sort_at,content_hash,dedup_key,feed_content_explicit)
		VALUES ('old','source','旧文章','短摘录','完整正文',1,1,'old-version','guid:old',1);
		INSERT INTO article_summaries(article_id,source_mode,source_version,status,summary_text,model_id,han_limit,english_limit,updated_at)
		VALUES ('old','feed','old-version','generated','Existing summary','test',0,0,1);
	`)
	if closeErr := old.Close(); err != nil || closeErr != nil {
		t.Fatalf("legacy database setup: %v %v", err, closeErr)
	}

	store, err := Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	key := sqlc.GetArticleSummaryParams{ArticleID: "old", SourceMode: "feed", SourceVersion: "old-version"}
	record, err := store.GetArticleSummary(ctx, key)
	if err != nil || record.SummaryText != "Existing summary" || record.TargetLang != "" {
		t.Fatalf("stored text was lost or given an assumed language: %+v %v", record, err)
	}
	params := sqlc.SaveArticleSummaryParams{ArticleID: "old", SourceMode: "feed", SourceVersion: "old-version", Status: "generated",
		SummaryText: "本次更新包含两项安排。", TargetLang: "zh-CN"}
	if err := store.SaveCurrentArticleSummary(ctx, params); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, `UPDATE article_ai_settings SET translation_target_lang='en'`); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveCurrentArticleSummary(ctx, params); !errors.Is(err, ErrConflict) {
		t.Fatalf("wrong-language result was written after settings changed: %v", err)
	}
}
