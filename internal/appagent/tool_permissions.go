package appagent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/coolerks/Fealdhord/internal/agentrun"
)

const toolPermissionsFile = "tool-permissions.json"
const maxToolPermissionsBytes = 512 << 10

var errToolPermissionsUnavailable = errors.New("ToolPermissionsUnavailable")

type ToolPermission struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Connection       string `json:"connection,omitempty"`
	Available        bool   `json:"available"`
	RequiresApproval bool   `json:"requiresApproval"`
	Mode             string `json:"mode"`
}

func toolPermissionID(tool Tool) string {
	if tool.permissionID != "" {
		return tool.permissionID
	}
	return "builtin:" + tool.Name
}

func externalToolPermissionID(config ClientConfig, kind, name string) string {
	identity, _ := json.Marshal(struct {
		Connection, Endpoint, Command, Kind, Name string
		Args                                      []string `json:",omitempty"`
	}{config.Name, config.Endpoint, config.Command, kind, name, config.Args})
	sum := sha256.Sum256(identity)
	return "mcp:" + hex.EncodeToString(sum[:])
}

func (a *Platform) savedToolPermissions() (map[string]ToolPermission, error) {
	if a.dir == "" {
		return nil, errToolPermissionsUnavailable
	}
	path := filepath.Join(a.dir, toolPermissionsFile)
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return make(map[string]ToolPermission), nil
	}
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxToolPermissionsBytes {
		return nil, errToolPermissionsUnavailable
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, errToolPermissionsUnavailable
	}
	defer file.Close()
	info, err = file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxToolPermissionsBytes {
		return nil, errToolPermissionsUnavailable
	}
	data, err := io.ReadAll(io.LimitReader(file, maxToolPermissionsBytes+1))
	var rules map[string]ToolPermission
	if err != nil || len(data) > maxToolPermissionsBytes || json.Unmarshal(data, &rules) != nil || rules == nil {
		return nil, errToolPermissionsUnavailable
	}
	for id, rule := range rules {
		if id != rule.ID || rule.Name == "" || !agentrun.ValidToolPermission(rule.Mode) {
			return nil, errToolPermissionsUnavailable
		}
	}
	return rules, nil
}

func (a *Platform) LoadToolPermissions() (map[string]string, error) {
	rules, err := a.savedToolPermissions()
	if err != nil {
		return nil, err
	}
	modes := make(map[string]string, len(rules))
	for id, rule := range rules {
		modes[id] = rule.Mode
	}
	return modes, nil
}

func (a *Platform) availableToolPermissions() map[string]ToolPermission {
	tools := make(map[string]ToolPermission)
	add := func(name, connection string, tool Tool) {
		id := toolPermissionID(tool)
		tools[id] = ToolPermission{ID: id, Name: name, Connection: connection, Available: true, RequiresApproval: tool.RequiresApproval}
	}
	for _, tool := range a.registry.tools {
		add(tool.Name, "", tool)
	}
	add(agentrun.AuthorizedArticleTool, "", Tool{Name: agentrun.AuthorizedArticleTool})
	a.mu.Lock()
	defer a.mu.Unlock()
	for name, conn := range a.clients {
		for _, tool := range conn.chatTools() {
			add(tool.externalName, name, tool)
		}
	}
	return tools
}

func (a *Platform) ToolPermissions() ([]ToolPermission, error) {
	saved, err := a.savedToolPermissions()
	if err != nil {
		return nil, err
	}
	tools := a.availableToolPermissions()
	for id, rule := range saved {
		if tool, ok := tools[id]; ok {
			tool.Mode = rule.Mode
			tools[id] = tool
		} else {
			rule.Available = false
			tools[id] = rule
		}
	}
	result := make([]ToolPermission, 0, len(tools))
	for _, tool := range tools {
		result = append(result, tool)
	}
	slices.SortFunc(result, func(a, b ToolPermission) int {
		if order := strings.Compare(a.Connection, b.Connection); order != 0 {
			return order
		}
		if order := strings.Compare(a.Name, b.Name); order != 0 {
			return order
		}
		if a.Available != b.Available {
			if a.Available {
				return -1
			}
			return 1
		}
		return strings.Compare(a.ID, b.ID)
	})
	return result, nil
}

// An empty mode restores the tool's default even when it is disconnected.
func (a *Platform) SaveToolPermission(id, mode string) error {
	if id == "" || len(id) > 128 || (mode != "" && !agentrun.ValidToolPermission(mode)) {
		return errors.New("ValidationError")
	}
	a.permissionEdit.Lock()
	defer a.permissionEdit.Unlock()
	rules, err := a.savedToolPermissions()
	if err != nil {
		return err
	}
	if mode == "" {
		if _, exists := rules[id]; !exists {
			return nil
		}
		delete(rules, id)
	} else {
		tool, exists := a.availableToolPermissions()[id]
		if !exists {
			return errors.New("UnknownTool")
		}
		tool.Mode = mode
		rules[id] = tool
	}
	data, err := json.Marshal(rules)
	if err != nil || len(data) > maxToolPermissionsBytes || atomicFile(a.dir, toolPermissionsFile, data) != nil {
		return errToolPermissionsUnavailable
	}
	return nil
}

func (a *Platform) AuthorizeArticleRead(ctx context.Context, conversationID string, args json.RawMessage) error {
	return a.registry.authorizeTool(ctx, "会话 "+conversationID, conversationID, Tool{Name: agentrun.AuthorizedArticleTool}, args)
}
