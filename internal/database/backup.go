package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/gofrs/flock"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
)

func checkDatabase(ctx context.Context, db *sql.DB) error {
	var status string
	if err := db.QueryRowContext(ctx, "PRAGMA quick_check").Scan(&status); err != nil {
		return err
	}
	if status != "ok" {
		return fmt.Errorf("数据库完整性检查失败: %s", status)
	}
	rows, err := db.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		return err
	}
	defer rows.Close()
	if rows.Next() {
		return errors.New("数据库存在失效外键")
	}
	return rows.Err()
}

// snapshot 只创建新目标。VACUUM INTO 包含已提交 WAL，临时文件通过后才发布。
func snapshot(ctx context.Context, db *sql.DB, destination string) (err error) {
	if _, err := os.Stat(destination); !errors.Is(err, os.ErrNotExist) {
		if err == nil {
			return os.ErrExist
		}
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(destination), ".fealdhord-snapshot-*")
	if err != nil {
		return err
	}
	path := temp.Name()
	defer func() { _ = os.Remove(path) }()
	if err := temp.Close(); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, "VACUUM INTO ?", path); err != nil {
		return err
	}
	copyDB, err := openSnapshot(path)
	if err != nil {
		return err
	}
	checkErr := checkDatabase(ctx, copyDB)
	closeErr := copyDB.Close()
	if err := errors.Join(checkErr, closeErr); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	syncErr := file.Sync()
	closeErr = file.Close()
	if err := errors.Join(syncErr, closeErr); err != nil {
		return err
	}
	// 同一目录硬链接采用 no-replace 语义，避免检查后被并发创建的目标遭覆盖。
	if err := os.Link(path, destination); err != nil {
		return err
	}
	if runtime.GOOS != "windows" {
		directory, err := os.Open(filepath.Dir(destination))
		if err != nil {
			return err
		}
		return errors.Join(directory.Sync(), directory.Close())
	}
	return nil
}

func openSnapshot(filename string) (*sql.DB, error) {
	path, err := filepath.Abs(filename)
	if err != nil {
		return nil, err
	}
	dsn := (&url.URL{Scheme: "file", Path: filepath.ToSlash(path), RawQuery: "mode=ro"}).String()
	return sql.Open("sqlite", dsn)
}

func (s *Store) Backup(ctx context.Context, destination string) error {
	s.write.Lock()
	defer s.write.Unlock()
	return snapshot(ctx, s.db, destination)
}

// RestoreSnapshot 恢复到不存在的新路径，绝不替换正在使用的库。
func RestoreSnapshot(ctx context.Context, source, destination string) error {
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return err
	}
	lock := flock.New(destination + ".lock")
	locked, err := lock.TryLock()
	if err != nil || !locked {
		_ = lock.Close()
		return errors.Join(ErrDatabaseInUse, err)
	}
	defer lock.Close()
	db, err := openSnapshot(source)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := checkDatabase(ctx, db); err != nil {
		return err
	}
	return snapshot(ctx, db, destination)
}
