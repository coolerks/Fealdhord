package agentrun

import "context"

const (
	ApprovalAsk    = "ask"
	ApprovalBypass = "bypass"
	ApprovalDeny   = "deny"
)

func ValidApprovalMode(mode string) bool {
	return mode == ApprovalAsk || mode == ApprovalBypass || mode == ApprovalDeny
}

type approvalKey struct{}

func WithApprovalMode(ctx context.Context, mode string) context.Context {
	return context.WithValue(ctx, approvalKey{}, mode)
}

func ApprovalMode(ctx context.Context) string {
	mode, _ := ctx.Value(approvalKey{}).(string)
	return mode
}
