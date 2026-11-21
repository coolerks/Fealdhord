package appagent

import (
	"context"
	"errors"
	"io"
	"os"
)

// API binds desktop controls only. Tool execution is reached through Chat or authenticated MCP.
type API struct {
	platform  *Platform
	pickSkill func(context.Context) (string, error)
}

func NewAPI(platform *Platform, pickSkill func(context.Context) (string, error)) *API {
	return &API{platform: platform, pickSkill: pickSkill}
}
func (a *API) ToolList() []Tool                        { return a.platform.ToolList() }
func (a *API) PendingActions() []PendingAction         { return a.platform.PendingActions() }
func (a *API) DecideAction(id string, allow bool) bool { return a.platform.DecideAction(id, allow) }
func (a *API) MCPServer() ServerStatus                 { return a.platform.MCPServer() }
func (a *API) SaveMCPServer(config ServerConfig) error { return a.platform.SaveMCPServer(config) }
func (a *API) RotateMCPToken() (string, error)         { return a.platform.RotateMCPToken() }
func (a *API) ConnectMCP(ctx context.Context, config ClientConfig) (ClientStatus, error) {
	return a.platform.ConnectMCP(ctx, config)
}
func (a *API) MCPClients() []ClientStatus { return a.platform.MCPClients() }
func (a *API) DisconnectMCP(name string)  { a.platform.DisconnectMCP(name) }
func (a *API) Skills() []Skill            { return a.platform.Skills() }
func (a *API) SaveSkill(markdown string, enabled bool) (Skill, error) {
	return a.platform.saveSkill(markdown, enabled)
}
func (a *API) DeleteSkill(name string) error { return a.platform.removeSkill(name) }

func (a *API) ForgetMCP(name string) error { return a.platform.ForgetMCP(name) }

func (a *API) Grants() []MCPGrant { return a.platform.Grants() }
func (a *API) SaveGrant(grant MCPGrant, revoke bool) (string, error) {
	return a.platform.SaveGrant(grant, revoke)
}

func (a *API) ImportSkill(ctx context.Context) (Skill, error) {
	if a.pickSkill == nil {
		return Skill{}, errors.New("Unavailable")
	}
	path, err := a.pickSkill(ctx)
	if err != nil {
		return Skill{}, err
	}
	if path == "" {
		return Skill{}, errors.New("Cancelled")
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 256<<10 {
		return Skill{}, errors.New("InvalidSkill")
	}
	file, err := os.Open(path)
	if err != nil {
		return Skill{}, errors.New("StorageError")
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, (256<<10)+1))
	if err != nil {
		return Skill{}, errors.New("StorageError")
	}
	return parseSkill(string(data), false)
}

func (a *API) RecentAudit() []AuditEntry { return a.platform.registry.RecentAudit() }
