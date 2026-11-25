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
			const source = "旧订阅"
			// 用旧库的字段造数据，避免当前 sqlc 查询要求新版字段。
			if _, err := old.db.ExecContext(ctx, `INSERT INTO feeds
				(id,title,url,normalized_url,created_at,updated_at) VALUES (?, ?, ?, ?, 1, 1)`,
				source, "订阅", "https://example.test/feed", "https://example.test/feed"); err != nil {
				t.Fatal(err)
			}
			const first, second = "first", "second"
			_, err = old.db.ExecContext(ctx, `INSERT INTO articles
				(id,feed_id,guid,title,fetched_at,sort_at,content_hash,dedup_key)
				VALUES (?,?,'first','原有文章',1,1,'hash-first','guid:first'),
				       (?,?,'second','另一篇',2,2,'hash-second','guid:second')`, first, source, second, source)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = old.db.ExecContext(ctx, `INSERT INTO reading_states (article_id,is_read,updated_at) VALUES (?,1,1)`, first); err != nil {
				t.Fatal(err)
			}
			if _, err = old.db.ExecContext(ctx, `INSERT INTO favorites (article_id,created_at) VALUES (?,1)`, first); err != nil {
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
			feed, err := store.GetSubscription(ctx, source)
			if err != nil || feed.CustomIconURL != "" {
				t.Fatalf("旧订阅的自定义图标默认值错误：%+v %v", feed, err)
			}
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
