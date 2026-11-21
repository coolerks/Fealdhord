package database

import (
	"io/fs"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/coolerks/Fealdhord/internal/database/sqlc"
	"github.com/coolerks/Fealdhord/migrations"
)

func TestNotificationMigrationPreservesQueuedRecipient(t *testing.T) {
	ctx := t.Context()
	oldMigrations := fstest.MapFS{}
	entries, err := fs.ReadDir(migrations.Files, ".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".sql" || entry.Name() >= "00020" {
			continue
		}
		data, err := fs.ReadFile(migrations.Files, entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		oldMigrations[entry.Name()] = &fstest.MapFile{Data: data}
	}
	filename := filepath.Join(t.TempDir(), "library.db")
	old, err := openWithMigrations(ctx, filename, oldMigrations)
	if err != nil {
		t.Fatal(err)
	}
	_, err = old.db.ExecContext(ctx, `
		INSERT INTO notification_channels(id,name,platform,target,credential_ref,enabled,verified)
		VALUES('existing','Telegram','telegram','1234','ref',1,1);
		INSERT INTO notification_deliveries(id,channel_id,source_id,source_revision,payload,due_at,created_at)
		VALUES('previous','existing','manual',0,'{}',1,1);
	`)
	closeErr := old.Close()
	if err != nil || closeErr != nil {
		t.Fatalf("old database: %v, close: %v", err, closeErr)
	}
	store, err := Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	q := store.NotificationQueries()
	row, err := q.NextNotificationDelivery(ctx, 1)
	if err != nil || row.ID != "previous" || row.RecipientID == "" {
		t.Fatalf("delivery = %+v, %v", row, err)
	}
	recipient, err := q.GetNotificationRecipient(ctx, row.RecipientID)
	if err != nil || recipient.Target != "1234" || recipient.Verified != 1 {
		t.Fatalf("recipient = %+v, %v", recipient, err)
	}
	if count, err := q.BeginNotificationDelivery(ctx, row.ID); count != 1 || err != nil {
		t.Fatalf("claim = %v, %v", count, err)
	}
}

func TestNotificationSchemaRepairUpgradesEarlyDevDatabase(t *testing.T) {
	ctx := t.Context()
	filename := filepath.Join(t.TempDir(), "library.db")
	// Use only the SQL migrations to reproduce a database that recorded v20.
	old, err := openWithMigrations(ctx, filename, migrations.Files)
	if err != nil {
		t.Fatal(err)
	}
	_, err = old.db.ExecContext(ctx, `
		DROP TABLE notification_rule_candidates;
		DROP TABLE notification_digest_items;
		DROP TABLE notification_runtime;
		ALTER TABLE notification_article_events DROP COLUMN offline_since;
		ALTER TABLE notification_article_events DROP COLUMN resumed_at;
		ALTER TABLE scheduled_runs DROP COLUMN task_revision;
		ALTER TABLE scheduled_runs DROP COLUMN local_pending;
		INSERT INTO scheduled_tasks(id,configuration,enabled,next_at) VALUES('old-task','{}',1,10);
		INSERT INTO scheduled_runs(task_id,occurrence,state,result) VALUES('old-task',1,'complete','saved'),('old-task',2,'running','');
	`)
	closeErr := old.Close()
	if err != nil || closeErr != nil {
		t.Fatalf("early database: %v, close: %v", err, closeErr)
	}
	store, err := Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	q := store.NotificationQueries()
	if err := q.ResumeNotificationRuntime(ctx, sqlc.ResumeNotificationRuntimeParams{ResumedAt: 10, LastSeen: 10}); err != nil {
		t.Fatal(err)
	}
	if err := q.PruneNotificationCandidates(ctx, 10); err != nil {
		t.Fatal(err)
	}
	if _, err := q.ListNotificationEvents(ctx, sqlc.ListNotificationEventsParams{PageSize: 100}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, "UPDATE notification_digest_items SET queued=1 WHERE id=0"); err != nil {
		t.Fatal(err)
	}
	if err := q.RecoverScheduledRuns(ctx); err != nil {
		t.Fatal(err)
	}
	runs, err := q.ListUnfinishedScheduledRuns(ctx)
	if err != nil || len(runs) != 1 || runs[0].TaskRevision != 0 || runs[0].State != "interrupted" {
		t.Fatalf("interrupted runs = %+v, %v", runs, err)
	}
	var result string
	err = store.db.QueryRowContext(ctx, "SELECT result FROM scheduled_runs WHERE task_id='old-task' AND occurrence=1").Scan(&result)
	if err != nil || result != "saved" {
		t.Fatalf("completed run = %q, %v", result, err)
	}
	backups, err := filepath.Glob(filepath.Join(filename+".backups", "before-v20-*.db"))
	if err != nil || len(backups) != 1 {
		t.Fatalf("pre-migration backups = %v, %v", backups, err)
	}
}
