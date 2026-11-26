package database

import (
	"context"

	"github.com/coolerks/Fealdhord/internal/database/sqlc"
)

func (s *Store) ChatQueries() *sqlc.Queries { return s.queries }

func (s *Store) CreateChatGroup(ctx context.Context, group sqlc.InsertChatGroupParams, folders []string) error {
	return s.transaction(ctx, func(q *sqlc.Queries) error {
		if err := q.InsertChatGroup(ctx, group); err != nil {
			return err
		}
		return insertChatGroupFolders(ctx, q, group.ID, folders)
	})
}

func (s *Store) UpdateChatGroup(ctx context.Context, input sqlc.UpdateChatGroupParams, folders []string) (sqlc.ChatGroup, error) {
	var group sqlc.ChatGroup
	err := s.transaction(ctx, func(q *sqlc.Queries) error {
		var err error
		if group, err = q.UpdateChatGroup(ctx, input); err != nil {
			return err
		}
		if err := q.ClearChatGroupFolders(ctx, group.ID); err != nil {
			return err
		}
		return insertChatGroupFolders(ctx, q, group.ID, folders)
	})
	return group, err
}

func insertChatGroupFolders(ctx context.Context, q *sqlc.Queries, id string, folders []string) error {
	for position, folder := range folders {
		if err := q.InsertChatGroupFolder(ctx, sqlc.InsertChatGroupFolderParams{
			GroupID: id, Position: int64(position), Path: folder,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) BranchChat(ctx context.Context, input sqlc.CreateChatBranchParams) (sqlc.Conversation, error) {
	var branch sqlc.Conversation
	err := s.transaction(ctx, func(q *sqlc.Queries) error {
		var err error
		branch, err = q.CreateChatBranch(ctx, input)
		if err != nil {
			return err
		}
		return q.CopyChatBranchMessages(ctx, sqlc.CopyChatBranchMessagesParams{
			NewID: branch.ID, SourceID: input.SourceID, MessageID: input.MessageID,
		})
	})
	return branch, err
}

func (s *Store) AppendChatTurn(ctx context.Context, user, assistant sqlc.AppendChatMessageParams, updatedAt int64) (sqlc.Message, error) {
	s.write.Lock()
	defer s.write.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return sqlc.Message{}, err
	}
	defer tx.Rollback()
	q := s.queries.WithTx(tx)
	if _, err := q.AppendChatMessage(ctx, user); err != nil {
		return sqlc.Message{}, err
	}
	message, err := q.AppendChatMessage(ctx, assistant)
	if err != nil {
		return sqlc.Message{}, err
	}
	if err := q.TouchConversation(ctx, sqlc.TouchConversationParams{ID: user.ConversationID, UpdatedAt: updatedAt}); err != nil {
		return sqlc.Message{}, err
	}
	return message, tx.Commit()
}

// InterruptChat recovers persisted active messages and tool reads together.
func (s *Store) InterruptChat(ctx context.Context, now int64) error {
	return s.transaction(ctx, func(q *sqlc.Queries) error {
		if err := q.InterruptChatMessages(ctx); err != nil {
			return err
		}
		return q.InterruptChatToolAudit(ctx, nullTime(&now))
	})
}
