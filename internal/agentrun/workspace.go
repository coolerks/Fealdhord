package agentrun

import "context"

// Workspace is the directory scope resolved by the chat service for one run.
type Workspace struct {
	Roots         []string
	OutputDir     string
	HomeDir       string
	ReadOnlyPaths []string
}

type workspaceKey struct{}
type localOutputBudgetKey struct{}
type planApprovedCallbackKey struct{}

// WithPlanApprovedCallback attaches a callback fired when a plan is approved.
func WithPlanApprovedCallback(ctx context.Context, fn func()) context.Context {
	if fn == nil {
		return ctx
	}
	return context.WithValue(ctx, planApprovedCallbackKey{}, fn)
}

// NotifyPlanApproved invokes the registered plan-approval callback if present.
func NotifyPlanApproved(ctx context.Context) {
	if fn, ok := ctx.Value(planApprovedCallbackKey{}).(func()); ok && fn != nil {
		fn()
	}
}

func LocalOutputBudget(ctx context.Context, ceiling int) int {
	tokens, _ := ctx.Value(localOutputBudgetKey{}).(int)
	if tokens <= 0 {
		return ceiling
	}
	return min(ceiling, max(512, tokens/2))
}

func WithWorkspace(ctx context.Context, workspace Workspace) context.Context {
	return context.WithValue(ctx, workspaceKey{}, workspace)
}

func WorkspaceFrom(ctx context.Context) Workspace {
	workspace, _ := ctx.Value(workspaceKey{}).(Workspace)
	return workspace
}
