package appagent

import (
	"context"
	"errors"
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
func (a *API) MCPClients() ([]ClientStatus, error) { return a.platform.MCPClients() }
func (a *API) DisconnectMCP(name string)           { a.platform.DisconnectMCP(name) }
func (a *API) Skills() []Skill                     { return a.platform.Skills() }
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
	return loadSkillFile(path)
}

func (a *API) RecentAudit() []AuditEntry { return a.platform.registry.RecentAudit() }

func (a *API) SaveSkillPackage(input Skill) (Skill, error) {
	return a.platform.saveSkillResources(input.Markdown, input.Enabled, input.Resources)
}
