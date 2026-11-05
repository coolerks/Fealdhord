// Package database 管理数据库连接、版本化迁移和参数化查询。
package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/coolerks/Fealdhord/internal/database/sqlc"
	"github.com/coolerks/Fealdhord/migrations"
	"github.com/gofrs/flock"
	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

type PreferenceRecord struct {
	JSON          string
	SchemaVersion int64
	Revision      int64
}

type Store struct {
	cursorOnce   sync.Once
	cursorSecret string
	db           *sql.DB
	queries      *sqlc.Queries
	lock         *flock.Flock
	write        sync.Mutex
}

var ErrDatabaseInUse = errors.New("数据库正在被另一个实例使用")

func Open(ctx context.Context, filename string) (*Store, error) {
	return openWithMigrations(ctx, filename, migrations.Files)
}

func openWithMigrations(ctx context.Context, filename string, files fs.FS) (_ *Store, err error) {
	if err := os.MkdirAll(filepath.Dir(filename), 0o700); err != nil {
		return nil, fmt.Errorf("创建数据目录: %w", err)
	}
	path, err := filepath.Abs(filename)
	if err != nil {
		return nil, fmt.Errorf("定位数据库: %w", err)
	}
	info, statErr := os.Stat(path)
	existed := statErr == nil && info.Size() > 0
	if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		return nil, statErr
	}
	lock := flock.New(path + ".lock")
	locked, lockErr := lock.TryLock()
	if lockErr != nil || !locked {
		_ = lock.Close()
		return nil, errors.Join(ErrDatabaseInUse, lockErr)
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, lock.Close())
		}
	}()
	path = filepath.ToSlash(path)
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	query := url.Values{"_pragma": {"foreign_keys(1)", "busy_timeout(5000)", "journal_mode(WAL)"}}
	dsn := (&url.URL{Scheme: "file", Path: path, RawQuery: query.Encode()}).String()
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("打开数据库: %w", err)
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, db.Close())
		}
	}()
	db.SetMaxOpenConns(1)
	provider, err := goose.NewProvider(goose.DialectSQLite3, db, files)
	if err != nil {
		return nil, fmt.Errorf("初始化迁移: %w", err)
	}
	current, target, err := provider.GetVersions(ctx)
	if err != nil {
		return nil, fmt.Errorf("读取数据库版本: %w", err)
	}
	if current > target {
		return nil, fmt.Errorf("数据库版本 %d 高于应用支持的 %d", current, target)
	}
	pending, err := provider.HasPending(ctx)
	if err != nil {
		return nil, err
	}
	if existed && pending {
		backupDir := filename + ".backups"
		if err := os.MkdirAll(backupDir, 0o700); err != nil {
			return nil, err
		}
		backup := filepath.Join(backupDir, fmt.Sprintf("before-v%d-%s.db", current, time.Now().UTC().Format("20060102T150405.000000000")))
		if err := snapshot(ctx, db, backup); err != nil {
			return nil, fmt.Errorf("升级前备份失败: %w", err)
		}
	}
	if _, err := provider.Up(ctx); err != nil {
		return nil, fmt.Errorf("迁移数据库: %w", err)
	}
	if err := checkDatabase(ctx, db); err != nil {
		return nil, err
	}
	return &Store{db: db, queries: sqlc.New(db), lock: lock}, nil
}

func (s *Store) Close() error { return errors.Join(s.db.Close(), s.lock.Close()) }

func (s *Store) LoadPreferences(ctx context.Context) (PreferenceRecord, error) {
	row, err := s.queries.GetPreferences(ctx)
	return PreferenceRecord{JSON: row.ValueJson, SchemaVersion: row.SchemaVersion, Revision: row.Revision}, err
}

func (s *Store) SavePreferences(ctx context.Context, value string, revision, updatedAt int64) (bool, error) {
	rows, err := s.queries.UpdatePreferences(ctx, sqlc.UpdatePreferencesParams{
		ValueJson: value, UpdatedAt: updatedAt, Revision: revision,
	})
	return rows == 1, err
}
