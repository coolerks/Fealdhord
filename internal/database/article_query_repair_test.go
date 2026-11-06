package database

import (
	"context"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/coolerks/Fealdhord/migrations"
)

func TestUpgradeRepairsArticleQueryBirth(t *testing.T) {
	ctx := context.Background()
	version7 := fstest.MapFS{}
	files, err := fs.ReadDir(migrations.Files, ".")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".sql") || file.Name() >= "00008" {
			continue
		}
		body, err := fs.ReadFile(migrations.Files, file.Name())
		if err != nil {
			t.Fatal(err)
		}
		version7[file.Name()] = &fstest.MapFile{Data: body}
	}
	for _, scenario := range []string{"缺少表及写入触发器逻辑", "已有表且缺一条记录"} {
		t.Run(scenario, func(t *testing.T) {
			filename := filepath.Join(t.TempDir(), "library.db")
			old, err := openWithMigrations(ctx, filename, version7)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if old != nil {
					old.Close()
				}
			}()
			source, err := old.EnsureFeed(ctx, "https://example.test/feed", "订阅")
			if err != nil {
				t.Fatal(err)
			}
			first, err := old.UpsertArticle(ctx, ArticleInput{FeedID: source, GUID: "first", Title: "原有文章", FetchedAt: 1})
			if err != nil {
				t.Fatal(err)
			}
			second, err := old.UpsertArticle(ctx, ArticleInput{FeedID: source, GUID: "second", Title: "另一篇", FetchedAt: 2})
			if err != nil {
				t.Fatal(err)
			}
			if err = old.SetRead(ctx, first, true); err != nil {
				t.Fatal(err)
			}
			if err = old.SetFavorite(ctx, first, true); err != nil {
				t.Fatal(err)
			}
			var originalBirth int64
			if err = old.db.QueryRowContext(ctx, "SELECT generation FROM article_query_birth WHERE article_id=?", first).Scan(&originalBirth); err != nil {
				t.Fatal(err)
			}
			if scenario == "缺少表及写入触发器逻辑" {
				_, err = old.db.ExecContext(ctx, `DROP TRIGGER articles_generation_insert;
DROP TABLE article_query_birth;
CREATE TRIGGER articles_generation_insert AFTER INSERT ON articles BEGIN
 UPDATE search_state SET generation=generation+1 WHERE id=1;
END;`)
			} else {
				_, err = old.db.ExecContext(ctx, "DELETE FROM article_query_birth WHERE article_id=?", second)
			}
			if err != nil {
				t.Fatal(err)
			}
			if err = old.Close(); err != nil {
				t.Fatal(err)
			}
			old = nil

			store, err := Open(ctx, filename)
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			scope := ArticleScope{Kind: "feed", ID: source}
			page, err := store.QueryArticles(ctx, ArticleQuery{Scope: scope, Limit: 1})
			if err != nil || len(page.Items) != 1 || page.Items[0].ID != second || page.NextCursor == "" {
				t.Fatalf("历史首页未恢复：%+v %v", page, err)
			}
			next, err := store.QueryArticles(ctx, ArticleQuery{Scope: scope, Limit: 1, Cursor: page.NextCursor})
			if err != nil || len(next.Items) != 1 || next.Items[0].ID != first || next.Items[0].IsRead != 1 || next.Items[0].IsFavorite != 1 {
				t.Fatalf("旧文或阅读状态丢失：%+v %v", next, err)
			}
			if scenario == "已有表且缺一条记录" {
				var birth int64
				err := store.db.QueryRowContext(ctx, "SELECT generation FROM article_query_birth WHERE article_id=?", first).Scan(&birth)
				if err != nil || birth != originalBirth {
					t.Fatalf("现有出生代次变化：%d -> %d (%v)", originalBirth, birth, err)
				}
			}
			added, err := store.UpsertArticle(ctx, ArticleInput{FeedID: source, GUID: "new", Title: "更新文章", FetchedAt: 3})
			if err != nil {
				t.Fatal(err)
			}
			page, err = store.QueryArticles(ctx, ArticleQuery{Scope: scope})
			if err != nil || len(page.Items) != 3 || page.Items[0].ID != added {
				t.Fatalf("新文未进入分页：%+v %v", page, err)
			}
		})
	}
}
