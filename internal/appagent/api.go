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
func (a *API) ToolList() []Tool                           { return a.platform.ToolList() }
func (a *API) ToolPermissions() ([]ToolPermission, error) { return a.platform.ToolPermissions() }
func (a *API) SaveToolPermission(id, mode string) error {
	return a.platform.SaveToolPermission(id, mode)
}
func (a *API) SaveToolPermissions(changes []ToolPermissionChange) error {
	return a.platform.SaveToolPermissions(changes)
}
func (a *API) ShellSettings() (ShellSettings, error) { return a.platform.ShellSettings() }
func (a *API) SaveDefaultShell(id string) (ShellSettings, error) {
	return a.platform.SaveDefaultShell(id)
}
func (a *API) SaveCustomShell(path string) (ShellSettings, error) {
	return a.platform.SaveCustomShell(path)
}
func (a *API) PendingActions() []PendingAction         { return a.platform.PendingActions() }
func (a *API) DecideAction(id string, allow bool) bool { return a.platform.DecideAction(id, allow) }
func (a *API) PendingQuestions() []PendingQuestion     { return a.platform.PendingQuestions() }
func (a *API) AnswerQuestion(id string, answers []UserQuestionAnswer, cancelled bool) bool {
	return a.platform.AnswerQuestion(id, answers, cancelled)
}
func (a *API) PendingPlans() []PendingPlan { return a.platform.PendingPlans() }
func (a *API) ResolvePlan(id, decision, feedback string) bool {
	return a.platform.ResolvePlan(id, decision, feedback)
}
func (a *API) MCPServer() ServerStatus                 { return a.platform.MCPServer() }
func (a *API) SaveMCPServer(config ServerConfig) error { return a.platform.SaveMCPServer(config) }
func (a *API) RotateMCPToken() (string, error)         { return a.platform.RotateMCPToken() }
func (a *API) ConnectMCP(ctx context.Context, config ClientConfig) (ClientStatus, error) {
	return a.platform.ConnectMCP(ctx, config)
}
func (a *API) MCPClients() ([]ClientStatus, error) { return a.platform.MCPClients() }
func (a *API) DisconnectMCP(id string) error       { return a.platform.DisconnectMCP(id) }
func (a *API) Skills() []Skill {
	skills := a.platform.Skills()
	for i := range skills {
		skills[i].Markdown, skills[i].Resources = "", nil
	}
	return skills
}
func (a *API) SkillDetails(name string) (Skill, error) {
	return a.platform.skillDetails(name)
}
func (a *API) SetSkillEnabled(name string, enabled bool) (Skill, error) {
	return a.platform.setSkillEnabled(name, enabled)
}
func (a *API) CancelSkillImport(id string) { a.platform.cancelSkillImport(id) }
func (a *API) SaveSkill(markdown string, enabled bool) (Skill, error) {
	return a.platform.saveSkill(markdown, enabled)
}
func (a *API) DeleteSkill(name string) error { return a.platform.removeSkill(name) }

func (a *API) ForgetMCP(name string) error { return a.platform.ForgetMCP(name) }

func (a *API) Grants() []MCPGrant { return a.platform.Grants() }
func (a *API) SaveGrant(grant MCPGrant, revoke bool) (string, error) {
	return a.platform.SaveGrant(grant, revoke)
}
func (a *API) UpdateGrant(grant MCPGrant) error { return a.platform.UpdateGrant(grant) }
func (a *API) RotateGrantToken(name string) (string, error) {
	return a.platform.RotateGrantToken(name)
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
	return a.platform.previewSkillImport(ctx, path)
}

func (a *API) RecentAudit() []AuditEntry { return a.platform.registry.RecentAudit() }

func (a *API) SaveSkillPackage(input Skill) (Skill, error) {
	return a.platform.saveSkillInput(input)
}

func (a *API) MCPClientLogs(id string, attempt bool) []MCPClientLog {
	return a.platform.MCPClientLogs(id, attempt)
}
