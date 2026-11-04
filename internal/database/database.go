// Package database 管理数据库连接、版本化迁移和参数化查询。
package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/coolerks/Fealdhord/internal/database/sqlc"
	"github.com/coolerks/Fealdhord/migrations"
	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

type PreferenceRecord struct {
	JSON          string
	SchemaVersion int64
	Revision      int64
}

type Store struct {
	db      *sql.DB
	queries *sqlc.Queries
}

func Open(ctx context.Context, filename string) (_ *Store, err error) {
	if err := os.MkdirAll(filepath.Dir(filename), 0o700); err != nil {
		return nil, fmt.Errorf("创建数据目录: %w", err)
	}
	path, err := filepath.Abs(filename)
	if err != nil {
		return nil, fmt.Errorf("定位数据库: %w", err)
	}
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
	provider, err := goose.NewProvider(goose.DialectSQLite3, db, migrations.Files)
	if err != nil {
		return nil, fmt.Errorf("初始化迁移: %w", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		return nil, fmt.Errorf("迁移数据库: %w", err)
	}
	return &Store{db: db, queries: sqlc.New(db)}, nil
}

func (s *Store) Close() error { return s.db.Close() }

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
