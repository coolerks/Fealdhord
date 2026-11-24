package chat

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"github.com/coolerks/Fealdhord/internal/database"
	"github.com/coolerks/Fealdhord/internal/database/sqlc"
)

func TestManageGlobalConversations(t *testing.T) {
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
	scope := Scope{Kind: "global"}
	source, err := service.Create(ctx, scope, "原会话")
	if err != nil {
		t.Fatal(err)
	}
	q := db.ChatQueries()
	for _, item := range []struct{ user, assistant, answer string }{
		{"user-1", "assistant-1", "首条回复"}, {"user-2", "assistant-2", "最新回复"},
	} {
		_, err := db.AppendChatTurn(ctx,
			sqlc.AppendChatMessageParams{ID: item.user, ConversationID: source.ID, Role: "user", Status: "complete", Content: "问题"},
			sqlc.AppendChatMessageParams{ID: item.assistant, ConversationID: source.ID, Role: "assistant", Status: "running"}, 1)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := q.FinishChatMessage(ctx, sqlc.FinishChatMessageParams{Status: "complete", Content: item.answer, ID: item.assistant}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := service.Create(ctx, scope, "最近会话"); err != nil {
		t.Fatal(err)
	}
	if result, err := service.Rename(ctx, source.ID, "  改名后  "); err != nil || result.Title != "改名后" {
		t.Fatalf("rename result: %+v %v", result, err)
	}
	if _, err := service.SetPinned(ctx, source.ID, true); err != nil {
		t.Fatal(err)
	}
	if items, err := service.List(ctx, scope); err != nil || len(items) != 2 || items[0].ID != source.ID || !items[0].Pinned {
		t.Fatalf("pinned ordering: %+v %v", items, err)
	}

	service.active[source.ID] = &run{}
	if err := service.SetArchived(ctx, source.ID, true); err == nil || err.Error() != "Conflict" {
		t.Fatalf("archived a running conversation: %v", err)
	}
	if err := service.Delete(ctx, source.ID, false); err == nil || err.Error() != "Conflict" {
		t.Fatalf("deleted a running conversation: %v", err)
	}
	delete(service.active, source.ID)
	branch, err := service.Branch(ctx, source.ID, "")
	if err != nil || branch.ID == source.ID || branch.Pinned || branch.Title != "改名后" {
		t.Fatalf("latest branch: %+v %v", branch, err)
	}
	if rows, err := service.Messages(ctx, branch.ID); err != nil || len(rows) != 4 || rows[3].Text != "最新回复" {
		t.Fatalf("branch messages: %+v %v", rows, err)
	}
	if err := service.SetArchived(ctx, source.ID, true); err != nil {
		t.Fatal(err)
	}
	if items, err := service.List(ctx, scope); err != nil || len(items) != 2 {
		t.Fatalf("archived entry still active: %+v %v", items, err)
	}
	if err := service.Send(ctx, SendInput{ConversationID: source.ID, Text: "不得发送"}); err == nil || err.Error() != "ChatConversationUnavailable" {
		t.Fatalf("archived conversation accepted a send: %v", err)
	}
	if _, err := service.Messages(ctx, source.ID); err == nil {
		t.Fatal("archived messages exposed by normal chat")
	}
	if _, err := service.Branch(ctx, source.ID, "assistant-2"); err == nil {
		t.Fatal("archived chat can branch")
	}
	if err := service.Delete(ctx, source.ID, false); err == nil {
		t.Fatal("a stale active-list delete removed an archived item")
	}
	archived, err := service.Archived(ctx, 0)
	if err != nil || len(archived.Items) != 1 || archived.Items[0].ID != source.ID || archived.Items[0].ArchivedAt == 0 {
		t.Fatalf("archive list: %+v %v", archived, err)
	}
	if err := service.SetArchived(ctx, source.ID, false); err != nil {
		t.Fatal(err)
	}
	if err := service.Delete(ctx, source.ID, true); err == nil {
		t.Fatal("a stale archive-list delete removed a restored item")
	}
	if rows, err := service.Messages(ctx, source.ID); err != nil || len(rows) != 4 || rows[3].Text != "最新回复" {
		t.Fatalf("restored messages: %+v %v", rows, err)
	}
	if err := q.StartChatToolAudit(ctx, sqlc.StartChatToolAuditParams{
		MessageID: "assistant-2", CallID: "tool", ToolName: "test", StartedAt: 1,
	}); err != nil {
		t.Fatal(err)
	}
	if err := service.SetArchived(ctx, source.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := service.Delete(ctx, source.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := q.GetConversation(ctx, source.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("deleted chat remains: %v", err)
	}
	if rows, err := q.ListConversationMessages(ctx, source.ID); err != nil || len(rows) != 0 {
		t.Fatalf("deleted chat retained messages: %+v %v", rows, err)
	}
	if err := service.Delete(ctx, "", false); err == nil {
		t.Fatal("missing conversation accepted")
	}
}
