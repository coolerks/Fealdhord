package agentrun

import "context"

const (
	ApprovalAsk           = "ask"
	ApprovalBypass        = "bypass"
	ApprovalDeny          = "deny"
	ApprovalCustom        = "custom"
	AuthorizedArticleTool = "read_article"
)

func ValidApprovalMode(mode string) bool {
	return ValidToolPermission(mode) || mode == ApprovalCustom
}

func ValidToolPermission(mode string) bool {
	return mode == ApprovalAsk || mode == ApprovalBypass || mode == ApprovalDeny
}

type approvalKey struct{}
type toolPermissionsKey struct{}

func WithApprovalMode(ctx context.Context, mode string) context.Context {
	return context.WithValue(ctx, approvalKey{}, mode)
}

func ApprovalMode(ctx context.Context) string {
	mode, _ := ctx.Value(approvalKey{}).(string)
	return mode
}

// WithToolPermissions carries an immutable snapshot for one Chat send.
func WithToolPermissions(ctx context.Context, rules map[string]string) context.Context {
	return context.WithValue(ctx, toolPermissionsKey{}, rules)
}

func ToolPermission(ctx context.Context, id string) string {
	rules, _ := ctx.Value(toolPermissionsKey{}).(map[string]string)
	return rules[id]
}
