package chat

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/coolerks/Fealdhord/internal/database/sqlc"
)

const archivedPageSize = 50

type ArchivedPage struct {
	Items   []Conversation `json:"items"`
	HasMore bool           `json:"hasMore"`
}

func conversationDTO(row sqlc.Conversation) Conversation {
	return Conversation{ID: row.ID, Title: row.Title, Pinned: row.PinnedAt.Valid, ArchivedAt: row.ArchivedAt.Int64}
}

func (s *Service) Archived(ctx context.Context, offset int64) (ArchivedPage, error) {
	if offset < 0 {
		return ArchivedPage{}, modelError("ValidationError")
	}
	rows, err := s.store.ChatQueries().ListArchivedConversations(ctx, offset)
	if err != nil {
		return ArchivedPage{}, modelError("StorageError")
	}
	items := make([]Conversation, 0, min(len(rows), archivedPageSize))
	for _, row := range rows[:min(len(rows), archivedPageSize)] {
		items = append(items, conversationDTO(row))
	}
	return ArchivedPage{Items: items, HasMore: len(rows) > archivedPageSize}, nil
}

func conversationUpdate(row sqlc.Conversation, err error) (Conversation, error) {
	if errors.Is(err, sql.ErrNoRows) {
		return Conversation{}, modelError("ChatConversationUnavailable")
	}
	if err != nil {
		return Conversation{}, modelError("StorageError")
	}
	return conversationDTO(row), nil
}

func (s *Service) Rename(ctx context.Context, id, title string) (Conversation, error) {
	title = strings.TrimSpace(title)
	if title == "" || len(title) > 320 || !utf8.ValidString(title) {
		return Conversation{}, modelError("ValidationError")
	}
	return conversationUpdate(s.store.ChatQueries().RenameGlobalConversation(ctx, sqlc.RenameGlobalConversationParams{ID: id, Title: title}))
}

func (s *Service) SetPinned(ctx context.Context, id string, pinned bool) (Conversation, error) {
	return conversationUpdate(s.store.ChatQueries().SetGlobalConversationPin(ctx, sqlc.SetGlobalConversationPinParams{
		ID: id, PinnedAt: sql.NullInt64{Int64: time.Now().UnixMilli(), Valid: pinned},
	}))
}

func (s *Service) SetArchived(ctx context.Context, id string, archived bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active[id] != nil {
		return modelError("Conflict")
	}
	q := s.store.ChatQueries()
	now := time.Now().UnixMilli()
	if archived {
		return conversationChange(q.ArchiveGlobalConversation(ctx, sqlc.ArchiveGlobalConversationParams{
			ID: id, Now: sql.NullInt64{Int64: now, Valid: true},
		}))
	}
	return conversationChange(q.RestoreGlobalConversation(ctx, sqlc.RestoreGlobalConversationParams{ID: id, Now: now}))
}

func (s *Service) Delete(ctx context.Context, id string, archived bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active[id] != nil {
		return modelError("Conflict")
	}
	q := s.store.ChatQueries()
	if archived {
		return conversationChange(q.DeleteArchivedGlobalConversation(ctx, id))
	}
	return conversationChange(q.DeleteGlobalConversation(ctx, id))
}

func conversationChange(rows int64, err error) error {
	if err != nil {
		return modelError("StorageError")
	}
	if rows == 0 {
		return modelError("ChatConversationUnavailable")
	}
	return nil
}
