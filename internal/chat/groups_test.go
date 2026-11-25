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
