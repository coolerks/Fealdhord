package database

import (
	"context"
	"database/sql"
	"fmt"
)

// Earlier dev databases recorded the AI migrations before these fields were added.
func repairAISchema(ctx context.Context, tx *sql.Tx) error {
	if err := addMissingColumns(ctx, tx,
		migrationColumn{"conversations", "auto_compact", "INTEGER NOT NULL DEFAULT 0 CHECK(auto_compact IN (0,1))"},
		// A legacy result has no known profile. Keep its score and let a future enabled evaluation refresh it.
		migrationColumn{"article_ai_results", "profile_key", "TEXT NOT NULL DEFAULT ''"},
		migrationColumn{"article_ai_results", "reason", "TEXT NOT NULL DEFAULT ''"},
	); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS article_ai_deferrals (
 article_id TEXT NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
 task TEXT NOT NULL,
 until_at INTEGER NOT NULL,
 PRIMARY KEY(article_id,task)
);
`)
	return err
}

type migrationColumn struct {
	table, name, definition string
}

// Only migration-owned constants are interpolated into the DDL.
func addMissingColumns(ctx context.Context, tx *sql.Tx, columns ...migrationColumn) error {
	for _, column := range columns {
		var exists bool
		err := tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM pragma_table_info(?) WHERE name=?)", column.table, column.name).Scan(&exists)
		if err != nil {
			return err
		}
		if exists {
			continue
		}
		if _, err := tx.ExecContext(ctx, fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", column.table, column.name, column.definition)); err != nil {
			return fmt.Errorf("添加字段 %s.%s: %w", column.table, column.name, err)
		}
	}
	return nil
}
