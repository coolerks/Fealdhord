package chat

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/coolerks/Fealdhord/internal/database"
	"github.com/coolerks/Fealdhord/internal/database/sqlc"
)

func TestConversationGroupsPersistWorkspaceAndKeepDefaultDirectoryLazy(t *testing.T) {
	ctx := t.Context()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	path := filepath.Join(home, "chat.db")
	db, err := database.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	service, err := New(ctx, db, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	folders := []string{t.TempDir(), t.TempDir()}
	for i, folder := range folders {
		folders[i], err = filepath.EvalSymlinks(folder)
		if err != nil {
			t.Fatal(err)
		}
	}
	group, err := service.CreateGroup(ctx, "资料", folders)
	if err != nil {
		t.Fatal(err)
	}
	empty, err := service.CreateGroup(ctx, "空分组", nil)
	if err != nil {
		t.Fatal(err)
	}
	grouped, err := service.CreateInGroup(ctx, group.ID, "组内")
	if err != nil {
		t.Fatal(err)
	}
	ordinary, err := service.Create(ctx, Scope{Kind: "global"}, "普通")
	if err != nil {
		t.Fatal(err)
	}
	emptyChat, err := service.CreateInGroup(ctx, empty.ID, "空目录")
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.AppendChatTurn(ctx, sqlc.AppendChatMessageParams{ID: "question", ConversationID: grouped.ID, Role: "user", Status: "complete"},
		sqlc.AppendChatMessageParams{ID: "answer", ConversationID: grouped.ID, Role: "assistant", Status: "complete"}, time.Now().UnixMilli())
	if err != nil {
		t.Fatal(err)
	}
	branch, err := service.Branch(ctx, grouped.ID, "answer")
	if err != nil || branch.GroupID != group.ID {
		t.Fatalf("branch lost its group: %+v %v", branch, err)
	}
	service.Close()
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	db, err = database.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	service, err = New(ctx, db, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	ordinaryChats, err := service.List(ctx, Scope{Kind: "global"})
	if err != nil || len(ordinaryChats) != 1 || ordinaryChats[0].ID != ordinary.ID {
		t.Fatalf("ordinary listing %+v %v", ordinaryChats, err)
	}
	groupChats, err := service.GroupConversations(ctx, group.ID)
	if err != nil || len(groupChats) != 2 {
		t.Fatalf("group listing %+v %v", groupChats, err)
	}
	row, err := db.ChatQueries().GetConversation(ctx, grouped.ID)
	if err != nil {
		t.Fatal(err)
	}
	scope, prompt, err := service.conversationWorkspace(ctx, row)
	if err != nil || !slices.Equal(scope.Roots, folders) || !strings.Contains(prompt, "当前会话分组") || scope.OutputDir != "" {
		t.Fatalf("group directory %+v %q %v", scope, prompt, err)
	}
	for _, item := range []Conversation{ordinary, emptyChat} {
		row, err := db.ChatQueries().GetConversation(ctx, item.ID)
		if err != nil {
			t.Fatal(err)
		}
		scope, prompt, err := service.conversationWorkspace(ctx, row)
		expected := filepath.Join(home, ".Fealdhord", "会话文件", row.WorkspaceDay, item.ID)
		if err != nil || scope.OutputDir != expected || !strings.Contains(prompt, "只有任务需要写新文件") {
			t.Fatalf("default scope %+v %q %v", scope, prompt, err)
		}
	}
	if _, err := os.Stat(filepath.Join(home, ".Fealdhord")); !os.IsNotExist(err) {
		t.Fatal("workspace directory was created prematurely", err)
	}
}

func TestGroupArchiveRestoreAndRemovalPreserveChatsAndFolders(t *testing.T) {
	ctx := t.Context()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "chat.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	service, err := New(ctx, db, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()

	folder := t.TempDir()
	group, err := service.SaveGroup(ctx, GroupInput{Name: "Research", Icon: "lucide:Library", Color: "#AB1234FF", Folders: []string{folder}})
	if err != nil {
		t.Fatal(err)
	}
	saved, err := service.GroupDetails(ctx, group.ID)
	if err != nil || saved.Icon != group.Icon || saved.Color != group.Color || len(saved.Folders) != 1 {
		t.Fatalf("saved group: %+v %v", saved, err)
	}
	if _, err := service.SaveGroup(ctx, GroupInput{ID: group.ID, Name: "Edited", Icon: "books", Folders: []string{folder}}); err != nil {
		t.Fatal(err)
	}
	open, err := service.CreateInGroup(ctx, group.ID, "Open")
	if err != nil {
		t.Fatal(err)
	}
	private, err := service.CreateInGroup(ctx, group.ID, "Archived separately")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.SetArchived(ctx, private.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := service.SetGroupArchived(ctx, group.ID, true); err != nil {
		t.Fatal(err)
	}
	archived, err := service.ArchivedGroups(ctx)
	if err != nil || len(archived) != 1 || archived[0].Name != "Edited" || archived[0].ArchivedAt == 0 {
		t.Fatalf("archived groups: %+v %v", archived, err)
	}
	if rows, err := service.Groups(ctx); err != nil || len(rows) != 0 {
		t.Fatalf("active groups: %+v %v", rows, err)
	}
	if rows, err := service.GroupConversations(ctx, group.ID); err != nil || len(rows) != 0 {
		t.Fatalf("hidden chats: %+v %v", rows, err)
	}
	if _, err := service.CreateInGroup(ctx, group.ID, "New"); err == nil || err.Error() != "ChatGroupUnavailable" {
		t.Fatalf("created a chat in an archived group: %v", err)
	}
	if err := service.Send(ctx, SendInput{ConversationID: open.ID, Text: "Hi"}); err == nil || err.Error() != "ChatGroupUnavailable" {
		t.Fatalf("sent to an archived group: %v", err)
	}
	if err := service.SetGroupArchived(ctx, group.ID, false); err != nil {
		t.Fatal(err)
	}
	if rows, err := service.GroupConversations(ctx, group.ID); err != nil || len(rows) != 1 || rows[0].ID != open.ID {
		t.Fatalf("restored chats: %+v %v", rows, err)
	}
	if err := service.SetGroupArchived(ctx, group.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := service.DeleteGroup(ctx, group.ID, true); err != nil {
		t.Fatal(err)
	}
	if rows, err := service.List(ctx, Scope{Kind: "global"}); err != nil || len(rows) != 1 || rows[0].ID != open.ID || rows[0].GroupID != "" {
		t.Fatalf("detached chats: %+v %v", rows, err)
	}
	if rows, err := service.Archived(ctx, 0); err != nil || len(rows.Items) != 1 || rows.Items[0].ID != private.ID {
		t.Fatalf("independently archived chats: %+v %v", rows, err)
	}
	if info, err := os.Stat(folder); err != nil || !info.IsDir() {
		t.Fatalf("group removal changed the folder: %v", err)
	}
}
