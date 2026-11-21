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
	"unicode/utf8"

	"github.com/coolerks/Fealdhord/internal/agentrun"
	"github.com/coolerks/Fealdhord/internal/ai"
	"github.com/coolerks/Fealdhord/internal/database/sqlc"
	"github.com/invopop/jsonschema"
)

type Tool struct {
	Name             string `json:"name"`
	Description      string `json:"description"`
	RequiresApproval bool   `json:"requiresApproval"`
	schema           json.RawMessage
	invoke           func(context.Context, json.RawMessage) (any, error)
}

type PendingAction struct {
	ID        string `json:"id"`
	Principal string `json:"principal"`
	Tool      string `json:"tool"`
	Arguments string `json:"arguments"`
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
	tools    []Tool
	mu       sync.Mutex
	pending  map[string]approval
	auditDir string
	auditMu  sync.Mutex
	scopeDB  *sqlc.Queries
}

func NewRegistry() *Registry { return &Registry{pending: make(map[string]approval)} }

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

func (r *Registry) requestApproval(ctx context.Context, principal, name string, args json.RawMessage) error {
	id := rand.Text()
	action := PendingAction{ID: id, Principal: principal, Tool: name, Arguments: string(args)}
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
		if !allow {
			return errors.New("PermissionDenied")
		}
		return ctx.Err()
	case <-wait.Done():
		return wait.Err()
	}
}

func (r *Registry) execute(ctx context.Context, principal string, tool Tool, args json.RawMessage) (output string, resultError error) {
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
	if tool.RequiresApproval {
		if err := r.requestApproval(ctx, principal, tool.Name, args); err != nil {
			return "", err
		}
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	result, err := tool.invoke(ctx, args)
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(result)
	if err != nil {
		return "", err
	}
	if len(data) > 12<<10 {
		end := 10 << 10
		for end > 0 && !utf8.RuneStart(data[end]) {
			end--
		}
		data, err = json.Marshal(map[string]any{"truncated": true, "preview": string(data[:end])})
		if err != nil || len(data) > 12<<10 {
			return `{"truncated":true,"instruction":"使用更小的分页或缩小范围"}`, nil
		}
	}
	return string(data), nil
}

func (r *Registry) ChatTools(conversationID string) []agentrun.ReadTool {
	tools := make([]agentrun.ReadTool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, r.chatTool(conversationID, strings.ReplaceAll(tool.Name, ".", "_"), tool))
	}
	return tools
}

func (r *Registry) chatTool(conversationID, wire string, tool Tool) agentrun.ReadTool {
	return agentrun.ReadTool{
		Definition: ai.ToolDefinition{Name: wire, Description: tool.Description, InputSchema: tool.schema},
		Read: func(ctx context.Context, call ai.ToolCall) (string, error) {
			result, err := r.execute(ctx, "会话 "+conversationID, tool, json.RawMessage(call.ArgumentsJSON))
			if err != nil {
				if ctx.Err() != nil {
					return "", ctx.Err()
				}
				data, _ := json.Marshal(map[string]string{"error": err.Error()})
				return string(data), nil
			}
			return result, nil
		},
	}
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
