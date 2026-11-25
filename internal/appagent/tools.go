package appagent

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/coolerks/Fealdhord/internal/agentrun"
	"github.com/coolerks/Fealdhord/internal/ai"
	"github.com/coolerks/Fealdhord/internal/database/sqlc"
	"github.com/invopop/jsonschema"
)

type Tool struct {
	Name             string `json:"name"`
	Description      string `json:"description"`
	RequiresApproval bool   `json:"requiresApproval"`
	permissionID     string
	externalName     string
	schema           json.RawMessage
	invoke           func(context.Context, json.RawMessage) (any, error)
	verifyScope      func(context.Context, json.RawMessage) error
}

type PendingAction struct {
	ID             string `json:"id"`
	Principal      string `json:"principal"`
	Tool           string `json:"tool"`
	Arguments      string `json:"arguments"`
	ConversationID string `json:"conversationId,omitempty"`
}

type AuditEntry struct {
	Principal  string    `json:"principal"`
	Tool       string    `json:"tool"`
	Status     string    `json:"status"`
	StartedAt  time.Time `json:"startedAt"`
	FinishedAt time.Time `json:"finishedAt"`
}

type approval struct {
	action   PendingAction
	decision chan bool
}

type Registry struct {
	tools     []Tool
	mu        sync.Mutex
	pending   map[string]approval
	questions map[string]questionWait
	auditDir  string
	auditMu   sync.Mutex
	scopeDB   *sqlc.Queries
}

func NewRegistry() *Registry {
	return &Registry{
		pending:   make(map[string]approval),
		questions: make(map[string]questionWait),
	}
}

func Register[I any, O any](r *Registry, name, description string, guarded bool, f func(context.Context, I) O) {
	reflector := jsonschema.Reflector{DoNotReference: true, ExpandedStruct: true, RequiredFromJSONSchemaTags: true}
	schema, err := json.Marshal(reflector.Reflect(new(I)))
	if err != nil {
		panic(err)
	}
	r.tools = append(r.tools, Tool{Name: name, Description: description, RequiresApproval: guarded, schema: schema, invoke: func(ctx context.Context, raw json.RawMessage) (any, error) {
		var input I
		if len(raw) == 0 || len(raw) > 64<<10 {
			return nil, errors.New("ValidationError")
		}
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&input) != nil || decoder.Decode(new(json.RawMessage)) != io.EOF {
			return nil, errors.New("ValidationError")
		}
		return f(ctx, input), nil
	}})
}

func (r *Registry) Tools() []Tool { return slices.Clone(r.tools) }

func (r *Registry) Pending() []PendingAction {
	r.mu.Lock()
	defer r.mu.Unlock()
	pending := make([]PendingAction, 0, len(r.pending))
	for _, item := range r.pending {
		pending = append(pending, item.action)
	}
	slices.SortFunc(pending, func(a, b PendingAction) int { return strings.Compare(a.ID, b.ID) })
	return pending
}

func (r *Registry) Decide(id string, allow bool) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	item, ok := r.pending[id]
	if ok {
		delete(r.pending, id)
		item.decision <- allow
	}
	return ok
}

func (r *Registry) requestApproval(ctx context.Context, principal, conversationID, name string, args json.RawMessage) error {
	id := rand.Text()
	action := PendingAction{ID: id, Principal: principal, ConversationID: conversationID, Tool: name, Arguments: string(args)}
	r.mu.Lock()
	if len(r.pending) >= 32 {
		r.mu.Unlock()
		return errors.New("ApprovalLimitExceeded")
	}
	decision := make(chan bool, 1)
	r.pending[id] = approval{action: action, decision: decision}
	r.mu.Unlock()
	defer func() { r.mu.Lock(); delete(r.pending, id); r.mu.Unlock() }()
	wait, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	select {
	case allow := <-decision:
		if err := wait.Err(); err != nil {
			return err
		}
		if !allow {
			return errors.New("PermissionDenied")
		}
		return ctx.Err()
	case <-wait.Done():
		return wait.Err()
	}
}

func (r *Registry) execute(ctx context.Context, principal string, tool Tool, args json.RawMessage) (string, error) {
	return r.executeTool(ctx, principal, "", tool, args)
}

func (r *Registry) executeTool(ctx context.Context, principal, conversationID string, tool Tool, args json.RawMessage) (string, error) {
	store := agentrun.ResultStoreFrom(ctx)
	if store == nil {
		store = &agentrun.ResultStore{}
	}
	return r.executePaged(ctx, principal, conversationID, conversationID, store, tool, args)
}

type articleResult struct {
	source agentrun.ArticleSource
	legacy any
}

func (r *Registry) executePaged(ctx context.Context, principal, conversationID, owner string, store *agentrun.ResultStore, tool Tool, args json.RawMessage) (output string, resultError error) {
	started := time.Now()
	defer func() {
		status := toolAuditStatus(output, resultError)
		r.audit(principal, tool.Name, status, started)
	}()
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if !json.Valid(args) || len(args) > 64<<10 || !strings.HasPrefix(strings.TrimSpace(string(args)), "{") {
		return "", errors.New("ValidationError")
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(args, &fields) != nil {
		return "", errors.New("ValidationError")
	}
	if key := resultCursorKey(tool); key != "" && fields[key] != nil {
		raw := fields[key]
		var cursor string
		if len(fields) != 1 || json.Unmarshal(raw, &cursor) != nil || cursor == "" {
			return "", errors.New("ValidationError")
		}
		return store.Continue(owner, tool.Name, cursor, func(original string) error {
			if tool.verifyScope != nil {
				return tool.verifyScope(ctx, json.RawMessage(original))
			}
			return ctx.Err()
		})
	}
	if err := r.authorizeTool(ctx, principal, conversationID, tool, args); err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	result, err := tool.invoke(withConversationID(ctx, conversationID), args)
	if err != nil {
		return "", err
	}
	if article, ok := result.(articleResult); ok {
		data, err := json.Marshal(article.legacy)
		if err != nil {
			return "", err
		}
		if conversationID == "" && len(data) <= 8<<10 && article.source.Start == 0 && !article.source.PageOnly {
			return string(data), nil
		}
		return store.PublishArticle(owner, tool.Name, string(args), article.source)
	}
	data, err := json.Marshal(result)
	if err != nil {
		return "", err
	}
	if resultCursorKey(tool) == "" && len(data) > agentrun.ResultPageBytes {
		return "", agentrun.ErrResultLimit
	}
	page, err := store.PublishJSON(ctx, owner, tool.Name, string(args), data)
	if errors.Is(err, agentrun.ErrResultLimit) {
		return `{"error":"ToolResultLimit","instruction":"结果超过暂存上限。若原操作已提交，请查询状态，避免重复执行。"}`, nil
	}
	return page, err
}

func (r *Registry) authorizeTool(ctx context.Context, principal, conversationID string, tool Tool, args json.RawMessage) error {
	var mode string
	if conversationID != "" {
		mode = agentrun.ApprovalMode(ctx)
	}
	if mode == agentrun.ApprovalCustom {
		mode = agentrun.ToolPermission(ctx, toolPermissionID(tool))
	} else if !tool.RequiresApproval {
		return nil
	}
	switch mode {
	case agentrun.ApprovalDeny:
		return errors.New("PermissionDenied")
	case agentrun.ApprovalAsk:
		return r.requestApproval(ctx, principal, conversationID, tool.Name, args)
	case agentrun.ApprovalBypass:
		return nil
	default:
		if tool.RequiresApproval {
			return r.requestApproval(ctx, principal, conversationID, tool.Name, args)
		}
		return nil
	}
}

func (r *Registry) ChatTools(conversationID string) []agentrun.ReadTool {
	tools := make([]agentrun.ReadTool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, r.chatTool(conversationID, strings.ReplaceAll(tool.Name, ".", "_"), tool))
	}
	return tools
}

func (r *Registry) chatTool(conversationID, wire string, tool Tool) agentrun.ReadTool {
	schema := tool.schema
	description := tool.Description
	if key := resultCursorKey(tool); key != "" {
		schema = resultCursorSchema(schema, key)
		if key != "result_cursor" {
			description += "。使用 " + key + " 续读。"
		}
	}
	read := agentrun.ReadTool{
		Definition: ai.ToolDefinition{Name: wire, Description: description, InputSchema: schema},
		Read: func(ctx context.Context, call ai.ToolCall) (string, error) {
			result, err := r.executeTool(ctx, "会话 "+conversationID, conversationID, tool, json.RawMessage(call.ArgumentsJSON))
			if err != nil {
				return chatToolError(ctx, err)
			}
			return result, nil
		},
	}
	switch tool.Name {
	case "article.get", "article.extract":
		read.ArticleNext = func(ctx context.Context, _ ai.ToolCall, cursor string) (string, error) {
			return agentrun.ResultStoreFrom(ctx).Continue(conversationID, tool.Name, cursor, nil)
		}
	}
	return read
}

func resultCursorKey(tool Tool) string {
	if tool.externalName == "" {
		return "result_cursor"
	}
	const key = "_fealdhord_result_cursor"
	var schema struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if json.Unmarshal(tool.schema, &schema) != nil || schema.Properties[key] != nil {
		return ""
	}
	return key
}

func resultCursorSchema(schema json.RawMessage, key string) json.RawMessage {
	var root map[string]json.RawMessage
	if json.Unmarshal(schema, &root) != nil || root == nil {
		return schema
	}
	var properties map[string]json.RawMessage
	_ = json.Unmarshal(root["properties"], &properties)
	if properties == nil {
		properties = make(map[string]json.RawMessage)
	}
	if _, occupied := properties[key]; occupied {
		return schema
	}
	properties[key] = json.RawMessage(`{"type":"string"}`)
	root["properties"], _ = json.Marshal(properties)
	original := make(map[string]json.RawMessage)
	for _, keyword := range []string{"required", "anyOf", "oneOf", "allOf"} {
		if value := root[keyword]; len(value) > 0 {
			original[keyword] = value
			delete(root, keyword)
		}
	}
	if len(original) > 0 {
		cursorRequired, _ := json.Marshal([]string{key})
		root["anyOf"], _ = json.Marshal([]map[string]json.RawMessage{
			original,
			{"required": cursorRequired},
		})
	}
	updated, err := json.Marshal(root)
	if err != nil {
		return schema
	}
	return updated
}

func chatToolError(ctx context.Context, err error) (string, error) {
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	data, _ := json.Marshal(map[string]string{"error": err.Error()})
	return string(data), nil
}

func toolAuditStatus(output string, err error) string {
	switch {
	case errors.Is(err, context.Canceled):
		return "cancelled"
	case errors.Is(err, context.DeadlineExceeded):
		return "expired"
	case err != nil:
		switch err.Error() {
		case "PermissionDenied":
			return "denied"
		case "MCPResultUnknown":
			return "unknown"
		default:
			return "failed"
		}
	}
	var result struct {
		OK      *bool           `json:"ok"`
		IsError bool            `json:"isError"`
		Error   json.RawMessage `json:"error"`
	}
	if json.Unmarshal([]byte(output), &result) == nil && (result.OK != nil && !*result.OK || result.IsError || len(result.Error) > 0) {
		return "failed"
	}
	return "complete"
}

func (r *Registry) audit(principal, tool, status string, started time.Time) {
	if r.auditDir == "" {
		return
	}
	r.auditMu.Lock()
	defer r.auditMu.Unlock()
	if os.MkdirAll(r.auditDir, 0700) != nil {
		return
	}
	path := filepath.Join(r.auditDir, "tool-audit.jsonl")
	if info, err := os.Stat(path); err == nil && info.Size() > 2<<20 {
		if os.Rename(path, path+".previous") != nil {
			return
		}
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0600)
	if err != nil {
		return
	}
	defer file.Close()
	_ = json.NewEncoder(file).Encode(AuditEntry{Principal: principal, Tool: tool, Status: status, StartedAt: started, FinishedAt: time.Now()})
}

func (r *Registry) RecentAudit() []AuditEntry {
	if r.auditDir == "" {
		return nil
	}
	r.auditMu.Lock()
	defer r.auditMu.Unlock()
	file, err := os.Open(filepath.Join(r.auditDir, "tool-audit.jsonl"))
	if err != nil {
		return nil
	}
	defer file.Close()
	var recent [20]AuditEntry
	var count int
	scanner := bufio.NewScanner(io.LimitReader(file, 3<<20))
	for scanner.Scan() {
		var entry AuditEntry
		if json.Unmarshal(scanner.Bytes(), &entry) == nil {
			recent[count%len(recent)] = entry
			count++
		}
	}
	out := make([]AuditEntry, 0, min(count, len(recent)))
	for i := count - 1; i >= max(0, count-len(recent)); i-- {
		out = append(out, recent[i%len(recent)])
	}
	return out
}
