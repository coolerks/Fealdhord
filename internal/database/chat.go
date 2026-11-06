package database

import (
	"context"

	"github.com/coolerks/Fealdhord/internal/database/sqlc"
)

func (s *Store) ChatQueries() *sqlc.Queries { return s.queries }

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
