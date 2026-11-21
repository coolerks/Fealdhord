package database

import (
	"io/fs"
	"path/filepath"
	"testing"
	"testing/fstest"

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
