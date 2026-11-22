package chat

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/coolerks/Fealdhord/internal/ai"
	"github.com/coolerks/Fealdhord/internal/aiconfig"
	"github.com/coolerks/Fealdhord/internal/database/sqlc"
)

type ReplyDetails struct {
	DurationMS        int64  `json:"durationMs"`
	Rounds            int    `json:"rounds"`
	ReasoningEffort   string `json:"reasoningEffort"`
	InputTokens       *int   `json:"inputTokens,omitempty"`
	CachedInputTokens *int   `json:"cachedInputTokens,omitempty"`
	OutputTokens      *int   `json:"outputTokens,omitempty"`
}

func (s *Service) Branch(ctx context.Context, id, messageID string) (Conversation, error) {
	row, err := s.store.BranchChat(ctx, sqlc.CreateChatBranchParams{
		NewID: rand.Text(), Now: time.Now().UnixMilli(), SourceID: id, MessageID: messageID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return Conversation{}, modelError("ChatBranchUnavailable")
	}
	if err != nil {
		return Conversation{}, modelError("StorageError")
	}
	return Conversation{ID: row.ID, Title: row.Title}, nil
}

func effortLabel(model aiconfig.Model, effort string) string {
	for _, level := range model.ReasoningLevels {
		if level.Value == effort {
			return level.Label
		}
	}
	return "服务默认"
}

type replyUsage struct {
	rounds []ai.Usage
}

func (u *replyUsage) startRound() {
	u.rounds = append(u.rounds, ai.Usage{})
}

func (u *replyUsage) observe(value *ai.Usage) {
	if len(u.rounds) == 0 || value == nil {
		return
	}
	current := &u.rounds[len(u.rounds)-1]
	if value.InputTokens != nil {
		current.InputTokens = value.InputTokens
	}
	if value.CachedInputTokens != nil {
		current.CachedInputTokens = value.CachedInputTokens
	}
	if value.OutputTokens != nil {
		current.OutputTokens = value.OutputTokens
	}
}

func (u replyUsage) total(tokens func(ai.Usage) *int) *int {
	if len(u.rounds) == 0 {
		return nil
	}
	sum := 0
	for _, round := range u.rounds {
		n := tokens(round)
		if n == nil || *n < 0 {
			return nil
		}
		sum += *n
	}
	return &sum
}

func (u replyUsage) details(duration time.Duration, effort string) string {
	data, _ := json.Marshal(ReplyDetails{
		DurationMS: duration.Milliseconds(), Rounds: len(u.rounds), ReasoningEffort: effort,
		InputTokens:       u.total(func(round ai.Usage) *int { return round.InputTokens }),
		CachedInputTokens: u.total(func(round ai.Usage) *int { return round.CachedInputTokens }),
		OutputTokens:      u.total(func(round ai.Usage) *int { return round.OutputTokens }),
	})
	return string(data)
}
