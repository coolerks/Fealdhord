package chat

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/coolerks/Fealdhord/internal/aiconfig"
	"github.com/coolerks/Fealdhord/internal/database"
	"github.com/coolerks/Fealdhord/internal/database/sqlc"
)

func TestContextCheckpointPreservesVisibleHistoryAcrossReopen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "context.db")
	db, err := database.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	s := &Service{store: db, config: aiconfig.New(db.AIProviderQueries(), nil)}
	conversation, err := s.Create(ctx, Scope{Kind: "global"}, "checkpoint")
	if err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{"user", "assistant"} {
		_, err = db.ChatQueries().AppendChatMessage(ctx, sqlc.AppendChatMessageParams{
			ID: role, ConversationID: conversation.ID, Role: role, Status: "complete", Content: "test",
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := NewAPI(s).SaveContext(ctx, conversation.ID, "checkpoint summary", false); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = database.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s.store = db
	q := db.ChatQueries()
	config, err := q.GetConversation(ctx, conversation.ID)
	if err != nil || config.ContextAfter != 2 || config.ContextSummary != "checkpoint summary" {
		t.Fatalf("checkpoint not persisted: %+v %v", config, err)
	}
	rows, err := q.ListContextMessages(ctx, sqlc.ListContextMessagesParams{ConversationID: conversation.ID, ContextAfter: config.ContextAfter})
	visible, err2 := q.ListConversationMessages(ctx, conversation.ID)
	if err != nil || err2 != nil || len(rows) != 0 || len(visible) != 2 {
		t.Fatalf("checkpoint modified visible history: context=%v visible=%v err=%v/%v", rows, visible, err, err2)
	}
	if err := NewAPI(s).SaveContext(ctx, conversation.ID, "", true); err != nil {
		t.Fatal(err)
	}
	config, err = q.GetConversation(ctx, conversation.ID)
	if err != nil || config.ContextAfter != 0 || config.ContextSummary != "" {
		t.Fatalf("checkpoint not restored: %+v %v", config, err)
	}
}
