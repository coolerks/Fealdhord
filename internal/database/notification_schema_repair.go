package database

import (
	"context"
	"database/sql"
)

// Version 19 was used by dev builds before the notification schema was final.
func repairNotificationSchema(ctx context.Context, tx *sql.Tx) error {
	if err := addMissingColumns(ctx, tx,
		migrationColumn{"notification_article_events", "offline_since", "INTEGER NOT NULL DEFAULT 0"},
		migrationColumn{"notification_article_events", "resumed_at", "INTEGER NOT NULL DEFAULT 0"},
		migrationColumn{"scheduled_runs", "task_revision", "INTEGER NOT NULL DEFAULT 0"},
		migrationColumn{"scheduled_runs", "local_pending", "INTEGER NOT NULL DEFAULT 0 CHECK(local_pending IN (0,1))"},
	); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS notification_runtime (
    id INTEGER PRIMARY KEY CHECK(id=1),
    last_seen INTEGER NOT NULL,
    offline_since INTEGER NOT NULL,
    resumed_at INTEGER NOT NULL
);
INSERT INTO notification_runtime(id,last_seen,offline_since,resumed_at) VALUES(1,0,0,0)
ON CONFLICT(id) DO NOTHING;
CREATE TABLE IF NOT EXISTS notification_rule_candidates (
    rule_id TEXT NOT NULL REFERENCES notification_rules(id),
    rule_revision INTEGER NOT NULL,
    event_seq INTEGER NOT NULL REFERENCES notification_article_events(seq) ON DELETE CASCADE,
    expires_at INTEGER NOT NULL,
    PRIMARY KEY(rule_id, event_seq)
);
CREATE INDEX IF NOT EXISTS notification_candidate_expiry ON notification_rule_candidates(expires_at);
CREATE TABLE IF NOT EXISTS notification_digest_items (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    rule_id TEXT NOT NULL REFERENCES notification_rules(id),
    rule_revision INTEGER NOT NULL,
    article_id TEXT NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
    occurrence INTEGER NOT NULL,
    recovery INTEGER NOT NULL,
    payload TEXT NOT NULL CHECK(json_valid(payload)),
    queued INTEGER NOT NULL DEFAULT 0 CHECK(queued IN (0,1)),
    UNIQUE(rule_id, article_id)
);
CREATE INDEX IF NOT EXISTS notification_digest_due ON notification_digest_items(rule_id, rule_revision, queued, occurrence, recovery, id);
`)
	return err
}
