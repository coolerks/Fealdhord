package chat

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/coolerks/Fealdhord/internal/agentrun"
	"github.com/coolerks/Fealdhord/internal/ai"
	"github.com/coolerks/Fealdhord/internal/aiconfig"
	"github.com/coolerks/Fealdhord/internal/database/sqlc"
)

type ReplyDetails struct {
	Limit                 *agentrun.RunLimit `json:"limit,omitempty"`
	DurationMS            int64              `json:"durationMs"`
	Rounds                int                `json:"rounds"`
	ReasoningEffort       string             `json:"reasoningEffort"`
	InputTokens           *int               `json:"inputTokens,omitempty"`
	CachedInputTokens     *int               `json:"cachedInputTokens,omitempty"`
	OutputTokens          *int               `json:"outputTokens,omitempty"`
	FirstTokenMS          *int64             `json:"firstTokenMs,omitempty"`
	OutputTokensPerSecond *float64           `json:"outputTokensPerSecond,omitempty"`
	Work                  *ReplyWork         `json:"work,omitempty"`
}

type ReplyWork struct {
	DurationMS         int64               `json:"durationMs"`
	ReasoningObserved  bool                `json:"reasoningObserved"`
	ReasoningSummary   string              `json:"reasoningSummary,omitempty"`
	ReasoningText      string              `json:"reasoningText,omitempty"`
	ReasoningTruncated bool                `json:"reasoningTruncated,omitempty"`
	Tools              []ReplyToolStep     `json:"tools,omitempty"`
	Positions          []ReplyWorkPosition `json:"positions,omitempty"`
	Steers             []ReplySteer        `json:"steers,omitempty"`
	RetryAttempt       int                 `json:"retryAttempt,omitempty"`
	RetryMax           int                 `json:"retryMax,omitempty"`
	RetryCount         int                 `json:"retryCount,omitempty"`
}

type ReplySteer struct {
	Text   string                    `json:"text"`
	Offset int                       `json:"offset"`
	At     int64                     `json:"at"`
	Skills []agentrun.SkillReference `json:"skills,omitempty"`
}

// All string offsets use UTF-16 code units to match the frontend's source text.
type ReplyWorkPosition struct {
	Offset       int   `json:"offset"`
	SummaryEnd   int   `json:"summaryEnd"`
	ReasoningEnd int   `json:"reasoningEnd"`
	ToolEnd      int   `json:"toolEnd"`
	DurationMS   int64 `json:"durationMs"`
}

type ReplyToolStep struct {
	Name               string  `json:"name"`
	Status             string  `json:"status"`
	Arguments          *string `json:"arguments,omitempty"`
	Result             *string `json:"result,omitempty"`
	ArgumentsTruncated bool    `json:"argumentsTruncated,omitempty"`
	ResultTruncated    bool    `json:"resultTruncated,omitempty"`
}

type replyWork struct {
	firstText         time.Time
	roundFirstText    time.Time
	lastActivity      time.Time
	reasoningObserved bool
	summary           reasoningRecord
	thinking          reasoningRecord
	tools             []ReplyToolStep
	textOffset        int
	lastText          time.Time
	positions         []ReplyWorkPosition
	steers            []ReplySteer
	activity          string
	retryAttempt      int
	retryMax          int
	retryCount        int
}

type replyWorkCheckpoint struct {
	firstText         time.Time
	roundFirstText    time.Time
	lastActivity      time.Time
	reasoningObserved bool
	summaryText       string
	summaryNewRound   bool
	summaryTruncated  bool
	summaryLength     int
	thinkingText      string
	thinkingNewRound  bool
	thinkingTruncated bool
	thinkingLength    int
	tools             []ReplyToolStep
	textOffset        int
	lastText          time.Time
	positions         []ReplyWorkPosition
	steers            []ReplySteer
	activity          string
}

func (w *replyWork) checkpoint() replyWorkCheckpoint {
	return replyWorkCheckpoint{
		firstText:         w.firstText,
		roundFirstText:    w.roundFirstText,
		lastActivity:      w.lastActivity,
		reasoningObserved: w.reasoningObserved,
		summaryText:       w.summary.text.String(),
		summaryNewRound:   w.summary.newRound,
		summaryTruncated:  w.summary.truncated,
		summaryLength:     w.summary.length,
		thinkingText:      w.thinking.text.String(),
		thinkingNewRound:  w.thinking.newRound,
		thinkingTruncated: w.thinking.truncated,
		thinkingLength:    w.thinking.length,
		tools:             slices.Clone(w.tools),
		textOffset:        w.textOffset,
		lastText:          w.lastText,
		positions:         slices.Clone(w.positions),
		steers:            slices.Clone(w.steers),
		activity:          w.activity,
	}
}

func (w *replyWork) restore(cp replyWorkCheckpoint) {
	w.firstText = cp.firstText
	w.roundFirstText = cp.roundFirstText
	w.lastActivity = cp.lastActivity
	w.reasoningObserved = cp.reasoningObserved
	w.summary.text.Reset()
	w.summary.text.WriteString(cp.summaryText)
	w.summary.newRound = cp.summaryNewRound
	w.summary.truncated = cp.summaryTruncated
	w.summary.length = cp.summaryLength
	w.thinking.text.Reset()
	w.thinking.text.WriteString(cp.thinkingText)
	w.thinking.newRound = cp.thinkingNewRound
	w.thinking.truncated = cp.thinkingTruncated
	w.thinking.length = cp.thinkingLength
	w.tools = slices.Clone(cp.tools)
	w.textOffset = cp.textOffset
	w.lastText = cp.lastText
	w.positions = slices.Clone(cp.positions)
	w.steers = slices.Clone(cp.steers)
	w.activity = cp.activity
}

func (w *replyWork) addSteer(text string, skills []agentrun.SkillReference, now time.Time) {
	w.lastActivity = now
	w.steers = append(w.steers, ReplySteer{
		Text:   text,
		Offset: w.textOffset,
		At:     now.UnixMilli(),
		Skills: skills,
	})
}

func (w *replyWork) setRetry(attempt, maxRetries int) {
	w.retryAttempt = attempt
	w.retryMax = maxRetries
}

func (w *replyWork) clearRetry() bool {
	if w.retryAttempt == 0 && w.retryMax == 0 {
		return false
	}
	w.retryAttempt = 0
	w.retryMax = 0
	return true
}

type reasoningRecord struct {
	text      strings.Builder
	newRound  bool
	truncated bool
	length    int
}

func (r *reasoningRecord) append(delta string, limit int) {
	if r.truncated {
		return
	}
	if r.newRound && r.text.Len() > 0 && r.text.Len() < limit-2 {
		r.text.WriteString("\n\n")
		r.length += 2
	}
	r.newRound = false
	part := boundedText(delta, limit-r.text.Len())
	r.truncated = len(part) < len(delta)
	r.text.WriteString(part)
	r.length += utf16Length(part)
}

func utf16Length(text string) int {
	length := 0
	for _, r := range text {
		length++
		if r > 0xffff {
			length++
		}
	}
	return length
}

func (w *replyWork) observe(event ai.ModelEvent, now time.Time) {
	activity := "tool"
	switch event.Type {
	case "message_start":
		w.roundFirstText = time.Time{}
		w.summary.newRound, w.thinking.newRound = true, true
		return
	case "text_delta":
		if event.Text != "" {
			w.textOffset += utf16Length(event.Text)
			w.lastText = now
		}
		if w.roundFirstText.IsZero() && strings.TrimSpace(event.Text) != "" {
			w.roundFirstText = now
			if w.firstText.IsZero() {
				w.firstText = now
			}
		}
		return
	case "reasoning_text_delta", "reasoning_summary_delta":
		if event.Text == "" {
			return
		}
		w.reasoningObserved = true
		activity = "reasoning"
		if event.Type == "reasoning_text_delta" {
			w.thinking.append(event.Text, 64<<10)
		} else {
			w.summary.append(event.Text, 16<<10)
		}
	case "tool_work_started":
		step := ReplyToolStep{Name: event.ToolName, Status: "running"}
		if event.ToolCall != nil {
			step.Arguments, step.ArgumentsTruncated = toolPreview(event.ToolCall.ArgumentsJSON)
		}
		w.tools = append(w.tools, step)
	case "tool_work_finished":
		// The tool runtime executes one call at a time and emits a start before each finish.
		if len(w.tools) > 0 {
			step := &w.tools[len(w.tools)-1]
			step.Status = event.ToolStatus
			if event.ToolOutput != nil {
				step.Result, step.ResultTruncated = toolPreview(*event.ToolOutput)
			}
		}
	}
	w.lastActivity = now
	n := len(w.positions)
	if n == 0 || w.positions[n-1].Offset != w.textOffset || w.activity != activity {
		w.positions = append(w.positions, ReplyWorkPosition{Offset: w.textOffset})
		n++
	}
	w.activity = activity
	pos := &w.positions[n-1]
	pos.SummaryEnd, pos.ReasoningEnd, pos.ToolEnd = w.summary.length, w.thinking.length, len(w.tools)
	if !w.lastText.IsZero() {
		pos.DurationMS = now.Sub(w.lastText).Milliseconds()
	}
}

func (w *replyWork) snapshot(started, ended time.Time, terminalStatus string) *ReplyWork {
	if w.lastActivity.IsZero() && len(w.steers) == 0 && w.retryCount == 0 && (terminalStatus != "running" || w.retryAttempt == 0) {
		return nil
	}
	until := ended
	if terminalStatus != "running" && !w.firstText.IsZero() {
		until = w.firstText
		if w.lastActivity.After(until) {
			until = w.lastActivity
		}
	}
	tools := slices.Clone(w.tools)
	for i := range tools {
		if tools[i].Status == "running" && terminalStatus != "running" {
			tools[i].Status = "interrupted"
			if terminalStatus == "cancelled" {
				tools[i].Status = "cancelled"
			}
		}
	}
	positions := slices.Clone(w.positions)
	for i := range positions {
		if positions[i].Offset != 0 {
			break
		}
		initialEnd := w.lastActivity
		if positions[len(positions)-1].Offset == 0 {
			initialEnd = until
		}
		// The initial position updates as the run progresses and freezes at the first visible text.
		if !w.firstText.IsZero() && initialEnd.After(w.firstText) {
			initialEnd = w.firstText
		}
		positions[i].DurationMS = initialEnd.Sub(started).Milliseconds()
	}
	retryAttempt, retryMax := 0, 0
	if terminalStatus == "running" {
		retryAttempt, retryMax = w.retryAttempt, w.retryMax
	}
	return &ReplyWork{
		DurationMS: until.Sub(started).Milliseconds(), Tools: tools,
		ReasoningObserved: w.reasoningObserved, ReasoningSummary: w.summary.text.String(), ReasoningText: w.thinking.text.String(),
		ReasoningTruncated: w.summary.truncated || w.thinking.truncated,
		Positions:          positions,
		Steers:             slices.Clone(w.steers),
		RetryAttempt:       retryAttempt,
		RetryMax:           retryMax,
		RetryCount:         w.retryCount,
	}
}

func (s *Service) Branch(ctx context.Context, id, messageID string) (Conversation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if messageID == "" {
		if s.active[id] != nil {
			return Conversation{}, modelError("Conflict")
		}
		var err error
		messageID, err = s.store.ChatQueries().LatestGlobalBranchPoint(ctx, id)
		if errors.Is(err, sql.ErrNoRows) {
			return Conversation{}, modelError("ChatBranchUnavailable")
		}
		if err != nil {
			return Conversation{}, modelError("StorageError")
		}
	}
	row, err := s.store.BranchChat(ctx, sqlc.CreateChatBranchParams{
		NewID: rand.Text(), Now: time.Now().UnixMilli(), SourceID: id, MessageID: messageID,
		WorkspaceDay: time.Now().Format(time.DateOnly),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return Conversation{}, modelError("ChatBranchUnavailable")
	}
	if err != nil {
		return Conversation{}, modelError("StorageError")
	}
	return conversationDTO(row), nil
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
	limit  *agentrun.RunLimit
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

func (u replyUsage) details(started, ended time.Time, effort string, work *replyWork, status string) string {
	details := ReplyDetails{
		DurationMS: ended.Sub(started).Milliseconds(), Rounds: len(u.rounds), ReasoningEffort: effort,
		InputTokens:       u.total(func(round ai.Usage) *int { return round.InputTokens }),
		CachedInputTokens: u.total(func(round ai.Usage) *int { return round.CachedInputTokens }),
		OutputTokens:      u.total(func(round ai.Usage) *int { return round.OutputTokens }),
		Work:              work.snapshot(started, ended, status),
	}
	if status == "failed" {
		details.Limit = u.limit
	}
	if !work.firstText.IsZero() {
		first := work.firstText.Sub(started).Milliseconds()
		details.FirstTokenMS = &first
	}
	if status == "complete" && len(u.rounds) > 0 && !work.roundFirstText.IsZero() {
		output := u.rounds[len(u.rounds)-1].OutputTokens
		if interval := ended.Sub(work.roundFirstText); interval >= time.Millisecond && output != nil && *output >= 0 {
			rate := float64(*output) / interval.Seconds()
			details.OutputTokensPerSecond = &rate
		}
	}
	data, _ := json.Marshal(details)
	return string(data)
}
