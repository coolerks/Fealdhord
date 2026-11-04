package database

import (
	"context"
	"errors"
	"github.com/coolerks/Fealdhord/migrations"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func oldMigrations(t *testing.T) fstest.MapFS {
	t.Helper()
	content, err := fs.ReadFile(migrations.Files, "00001_ui_preferences.sql")
	if err != nil {
		t.Fatal(err)
	}
	return fstest.MapFS{"00001_ui_preferences.sql": &fstest.MapFile{Data: content}}
}
func testStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(context.Background(), filepath.Join(t.TempDir(), "library.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	})
	return s
}
func TestUpgradeWALBackupRestoreAndInstanceLock(t *testing.T) {
	ctx := context.Background()
	file := filepath.Join(t.TempDir(), "数据.db")
	old, err := openWithMigrations(ctx, file, oldMigrations(t))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := old.SavePreferences(ctx, `{"theme":"dark"}`, 0, 100); err != nil {
		t.Fatal(err)
	}
	if err := old.Close(); err != nil {
		t.Fatal(err)
	}
	s, err := Open(ctx, file)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	backups, err := filepath.Glob(file + ".backups/*.db")
	if err != nil || len(backups) != 1 {
		t.Fatalf("升级前快照缺失：%v %v", backups, err)
	}
	snapshotDB, err := openSnapshot(backups[0])
	if err != nil {
		t.Fatal(err)
	}
	defer snapshotDB.Close()
	var raw string
	if err := snapshotDB.QueryRowContext(ctx, "SELECT value_json FROM app_settings").Scan(&raw); err != nil || raw != `{"theme":"dark"}` {
		t.Fatalf("快照丢失旧数据：%s %v", raw, err)
	}
	if _, err := Open(ctx, file); !errors.Is(err, ErrDatabaseInUse) {
		t.Fatalf("未拒绝第二实例：%v", err)
	}
	if _, err := s.db.ExecContext(ctx, "INSERT INTO feeds(id,title,url,normalized_url,created_at,updated_at) VALUES('feed','测试','https://example.test/rss','https://example.test/rss',1,1)"); err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(t.TempDir(), "snapshot.db")
	if err := s.Backup(ctx, backup); err != nil {
		t.Fatal(err)
	}
	if err := s.Backup(ctx, backup); !errors.Is(err, os.ErrExist) {
		t.Fatalf("允许覆盖现有备份：%v", err)
	}
	target := filepath.Join(t.TempDir(), "restored.db")
	if err := RestoreSnapshot(ctx, backup, target); err != nil {
		t.Fatal(err)
	}
	restored, err := Open(ctx, target)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	var count int
	if err := restored.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM feeds WHERE id='feed'").Scan(&count); err != nil || count != 1 {
		t.Fatalf("WAL 内容未恢复：%d %v", count, err)
	}
	if err := RestoreSnapshot(ctx, backup, target); err == nil {
		t.Fatal("恢复覆盖了已有目标")
	}
	if _, err := restored.LoadPreferences(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestFailedMigrationRollsBackAndKeepsBackup(t *testing.T) {
	ctx := context.Background()
	file := filepath.Join(t.TempDir(), "broken-upgrade.db")
	files := oldMigrations(t)
	old, err := openWithMigrations(ctx, file, files)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := old.SavePreferences(ctx, `{"theme":"dark"}`, 0, 10); err != nil {
		t.Fatal(err)
	}
	if err := old.Close(); err != nil {
		t.Fatal(err)
	}
	files["00002_failure.sql"] = &fstest.MapFile{Data: []byte("-- +goose Up\nCREATE TABLE partial_write(id TEXT);\nINSERT INTO missing_table VALUES(1);\n-- +goose Down\nDROP TABLE partial_write;")}
	if store, err := openWithMigrations(ctx, file, files); err == nil || store != nil {
		t.Fatal("错误迁移仍开放仓储")
	}
	db, err := openSnapshot(file)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var tables int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_schema WHERE name='partial_write'").Scan(&tables); err != nil || tables != 0 {
		t.Fatalf("部分 DDL 未回滚：%d %v", tables, err)
	}
	var value string
	if err := db.QueryRowContext(ctx, "SELECT value_json FROM app_settings").Scan(&value); err != nil || value != `{"theme":"dark"}` {
		t.Fatalf("旧数据改变：%s %v", value, err)
	}
	backups, _ := filepath.Glob(file + ".backups/*.db")
	if len(backups) != 1 {
		t.Fatal("失败后备份丢失")
	}
	recovered, err := Open(ctx, file)
	if err != nil {
		t.Fatalf("修复迁移后锁未释放：%v", err)
	}
	defer recovered.Close()
}

func TestFutureDatabaseVersionIsRejected(t *testing.T) {
	ctx := context.Background()
	file := filepath.Join(t.TempDir(), "future.db")
	s, err := Open(ctx, file)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, "INSERT INTO goose_db_version(version_id,is_applied) VALUES(999,1)"); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if store, err := Open(ctx, file); err == nil || store != nil {
		t.Fatal("旧应用打开了未来版本库")
	}
}
