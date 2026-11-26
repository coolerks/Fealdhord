package database

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/pressly/goose/v3"
)

func TestChatGroupMetadataMigrationKeepsLegacyGroupsAndFolders(t *testing.T) {
	ctx := t.Context()
	filename := filepath.Join(t.TempDir(), "chat.db")
	old, err := openWithMigrations(ctx, filename, sqlMigrationsBefore(t, "00033"), goose.WithGoMigrations(
		goose.NewGoMigration(21, &goose.GoFunc{RunTx: repairNotificationSchema}, nil),
		goose.NewGoMigration(22, &goose.GoFunc{RunTx: repairAISchema}, nil),
	))
	if err != nil {
		t.Fatal(err)
	}
	_, err = old.db.ExecContext(ctx, `
		INSERT INTO chat_groups(id,name,created_at) VALUES ('old','旧分组',1);
		INSERT INTO chat_group_folders(group_id,position,path) VALUES ('old',0,'legacy-path');
		INSERT INTO conversations(id,scope_type,scope_id,title,created_at,updated_at,group_id)
		VALUES ('chat','global','','Existing chat',1,1,'old');
	`)
	if closeErr := old.Close(); err != nil || closeErr != nil {
		t.Fatalf("旧库准备：%v %v", err, closeErr)
	}

	store, err := Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	group, err := store.ChatQueries().GetChatGroup(ctx, "old")
	if err != nil || group.Name != "旧分组" || group.Icon != "" || group.Color != "" || group.ArchivedAt.Valid {
		t.Fatalf("升级后分组：%+v %v", group, err)
	}
	folders, err := store.ChatQueries().ListChatGroupFolders(ctx, "old")
	if err != nil || len(folders) != 1 || folders[0] != "legacy-path" {
		t.Fatalf("升级后目录：%v %v", folders, err)
	}
	chats, err := store.ChatQueries().ListGroupConversations(ctx, sql.NullString{String: "old", Valid: true})
	if err != nil || len(chats) != 1 || chats[0].ID != "chat" {
		t.Fatalf("升级后会话：%v %v", chats, err)
	}
	backups, err := filepath.Glob(filename + ".backups/before-v32-*.db")
	if err != nil || len(backups) != 1 {
		t.Fatalf("升级前备份：%v %v", backups, err)
	}
}
