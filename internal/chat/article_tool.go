package chat

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"io"
	"strings"
	"time"

	"github.com/coolerks/Fealdhord/internal/agentrun"
	"github.com/coolerks/Fealdhord/internal/ai"
	"github.com/coolerks/Fealdhord/internal/database/sqlc"
)

func (s *Service) articleReadTool(conversationID, messageID string, ids []string) agentrun.ReadTool {
	allowed := make(map[string]bool, len(ids))
	for _, id := range ids {
		allowed[id] = true
	}
	return agentrun.ReadTool{
		Definition: ai.ToolDefinition{
			Name:        agentrun.AuthorizedArticleTool,
			Description: "分批读取本轮用户授权的本地文章正文。只可使用已列出的文章 ID。返回 next_cursor 时可仅传 result_cursor 续读。",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"id":{"type":"string"},"result_cursor":{"type":"string"},"offset":{"type":"integer","minimum":0},"version":{"type":"string"},"page_only":{"type":"boolean","description":"只读一页以核对引文"}},"additionalProperties":false}`),
		},
		ArticleNext: func(ctx context.Context, _ ai.ToolCall, cursor string) (string, error) {
			return agentrun.ResultStoreFrom(ctx).Continue(conversationID, agentrun.AuthorizedArticleTool, cursor, nil)
		},
		Read: func(ctx context.Context, call ai.ToolCall) (result string, runErr error) {
			q := s.store.ChatQueries()
			sum := sha256.Sum256([]byte(call.ID))
			callID := hex.EncodeToString(sum[:])
			if err := q.StartChatToolAudit(ctx, sqlc.StartChatToolAuditParams{
				MessageID: messageID, CallID: callID, ToolName: call.Name, StartedAt: time.Now().UnixMilli(),
			}); err != nil {
				return "", modelError("StorageError")
			}
			status, errorCode := "completed", ""
			defer func() {
				if ctx.Err() != nil {
					status, errorCode = "cancelled", "Cancelled"
				}
				finishCtx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				err := q.FinishChatToolAudit(finishCtx, sqlc.FinishChatToolAuditParams{
					MessageID: messageID, CallID: callID, Status: status, ErrorCode: errorCode,
					FinishedAt: sql.NullInt64{Int64: time.Now().UnixMilli(), Valid: true},
				})
				if err != nil {
					runErr = modelError("StorageError")
				}
			}()
			var input struct {
				ID       string `json:"id"`
				Cursor   string `json:"result_cursor"`
				Offset   int    `json:"offset"`
				Version  string `json:"version"`
				PageOnly bool   `json:"page_only"`
			}
			decoder := json.NewDecoder(bytes.NewBufferString(call.ArgumentsJSON))
			decoder.DisallowUnknownFields()
			if decoder.Decode(&input) != nil || decoder.Decode(new(any)) != io.EOF || (input.ID == "") == (input.Cursor == "") {
				status, errorCode = "failed", "ValidationError"
				return `{"error":"ValidationError","message":"参数必须包含文章 ID。"}`, nil
			}
			store := agentrun.ResultStoreFrom(ctx)
			if store == nil {
				store = &agentrun.ResultStore{}
			}
			if input.Cursor != "" {
				if input.Offset != 0 || input.Version != "" || input.PageOnly {
					status, errorCode = "failed", "ValidationError"
					return `{"error":"ValidationError"}`, nil
				}
				page, err := store.Continue(conversationID, agentrun.AuthorizedArticleTool, input.Cursor, nil)
				if err != nil {
					status, errorCode = "failed", err.Error()
					return `{"error":"ToolResultUnavailable"}`, nil
				}
				return page, nil
			}
			if !allowed[input.ID] {
				status, errorCode = "denied", "PermissionError"
				return `{"error":"PermissionError","message":"用户没有授权读取该文章。"}`, nil
			}
			if agentrun.ApprovalMode(ctx) == agentrun.ApprovalCustom {
				if err := s.toolPermissions.AuthorizeArticleRead(ctx, conversationID, json.RawMessage(call.ArgumentsJSON)); err != nil {
					status, errorCode = "denied", "PermissionDenied"
					return `{"error":"PermissionDenied","message":"用户没有批准读取该文章。"}`, ctx.Err()
				}
			}
			text, err := s.store.ArticleText(ctx, input.ID)
			if err != nil {
				status, errorCode = "failed", "ContextUnavailable"
				return `{"error":"ContextUnavailable","message":"无法读取授权的文章。"}`, nil
			}
			source := "feed"
			if text == "" {
				source = "summary_only"
				if article, err := q.ChatArticleContext(ctx, input.ID); err == nil {
					text = article.Summary
				}
			}
			article, err := agentrun.ResumeArticle(agentrun.NewArticleSource(input.ID, source, text), input.Offset, input.Version)
			if err != nil {
				status, errorCode = "failed", err.Error()
				encoded, _ := json.Marshal(map[string]string{"error": errorCode})
				return string(encoded), nil
			}
			article.PageOnly = input.PageOnly
			result, err = store.PublishArticle(conversationID, agentrun.AuthorizedArticleTool, call.ArgumentsJSON, article)
			if err != nil {
				status, errorCode = "failed", err.Error()
				return `{"error":"ToolResultLimit"}`, nil
			}
			return result, nil
		},
	}
}

// The prior reply supplies a location to resume, never authorization.
func articleResumeContext(rows []sqlc.Message) string {
	if len(rows) == 0 {
		return ""
	}
	last := rows[len(rows)-1]
	if last.Role != "assistant" || last.Status != "failed" {
		return ""
	}
	var previous ReplyDetails
	if json.Unmarshal([]byte(last.ReplyDetails), &previous) != nil || previous.Work == nil {
		return ""
	}
	var result strings.Builder
	count := 0
	for i := len(previous.Work.Tools) - 1; i >= 0 && count < 3; i-- {
		step := previous.Work.Tools[i]
		switch step.Name {
		case "article_get", "article_extract", agentrun.AuthorizedArticleTool:
		default:
			continue
		}
		if step.Result == nil {
			continue
		}
		var data struct {
			Error   string
			Article agentrun.ArticleSource
			End     int
			Total   int `json:"total_bytes"`
		}
		if json.Unmarshal([]byte(*step.Result), &data) != nil || data.Error != "ArticleReadIncomplete" ||
			len(data.Article.ID) == 0 || len(data.Article.ID) > 256 || len(data.Article.Version) != 64 ||
			data.End < 0 || data.End >= data.Total {
			continue
		}
		marker, _ := json.Marshal(map[string]any{"tool": step.Name, "id": data.Article.ID,
			"source": data.Article.Source, "version": data.Article.Version, "offset": data.End})
		result.WriteByte('\n')
		result.Write(marker)
		count++
	}
	if result.Len() == 0 {
		return ""
	}
	return "\n上一轮未读完的正文位置。仅当用户要求续读且本轮工具有权限时使用：" + result.String()
}
