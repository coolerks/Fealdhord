package appagent

import (
	"context"
	"crypto/rand"
	"errors"
	"slices"
	"strings"

	"github.com/coolerks/Fealdhord/internal/agentrun"
)

type ConfirmPlanInput struct {
	Title   string   `json:"title" jsonschema:"required,description=计划的简短标题"`
	Summary string   `json:"summary,omitempty" jsonschema:"description=计划目标与范围的简短概述"`
	Steps   []string `json:"steps" jsonschema:"required,description=按顺序排列的清晰执行步骤（1 到 20 步）"`
}

type ConfirmPlanOutput struct {
	Approved bool   `json:"approved"`
	Rejected bool   `json:"rejected,omitempty"`
	Feedback string `json:"feedback,omitempty"`
}

type PendingPlan struct {
	ID             string   `json:"id"`
	ConversationID string   `json:"conversationId"`
	Title          string   `json:"title"`
	Summary        string   `json:"summary,omitempty"`
	Steps          []string `json:"steps"`
}

type planReply struct {
	decision string // "approve" | "revise" | "reject"
	feedback string
}

type planWait struct {
	item  PendingPlan
	reply chan planReply
}

// WithPlanApprovedCallback attaches a callback fired when the user approves a ConfirmPlan call.
func WithPlanApprovedCallback(ctx context.Context, fn func()) context.Context {
	return agentrun.WithPlanApprovedCallback(ctx, fn)
}

func (r *Registry) PendingPlans() []PendingPlan {
	r.mu.Lock()
	defer r.mu.Unlock()
	pending := make([]PendingPlan, 0, len(r.plans))
	for _, wait := range r.plans {
		pending = append(pending, wait.item)
	}
	slices.SortFunc(pending, func(a, b PendingPlan) int { return strings.Compare(a.ID, b.ID) })
	return pending
}

func (r *Registry) ResolvePlan(id, decision, feedback string) bool {
	if decision != "approve" && decision != "revise" && decision != "reject" {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	wait, ok := r.plans[id]
	if ok {
		delete(r.plans, id)
		wait.reply <- planReply{decision: decision, feedback: strings.TrimSpace(feedback)}
	}
	return ok
}

func (r *Registry) confirmPlan(ctx context.Context, conversationID string, input ConfirmPlanInput) (ConfirmPlanOutput, error) {
	if conversationID == "" {
		return ConfirmPlanOutput{}, errors.New("InteractiveSessionRequired")
	}
	title := strings.TrimSpace(input.Title)
	if title == "" || len(input.Steps) == 0 || len(input.Steps) > 20 {
		return ConfirmPlanOutput{}, errors.New("ValidationError")
	}
	steps := make([]string, 0, len(input.Steps))
	for _, step := range input.Steps {
		cleaned := strings.TrimSpace(step)
		if cleaned == "" {
			return ConfirmPlanOutput{}, errors.New("ValidationError")
		}
		steps = append(steps, cleaned)
	}

	id := rand.Text()
	item := PendingPlan{
		ID:             id,
		ConversationID: conversationID,
		Title:          title,
		Summary:        strings.TrimSpace(input.Summary),
		Steps:          steps,
	}
	r.mu.Lock()
	if r.plans == nil {
		r.plans = make(map[string]planWait)
	}
	if len(r.plans) >= 16 {
		r.mu.Unlock()
		return ConfirmPlanOutput{}, errors.New("PlanLimitExceeded")
	}
	reply := make(chan planReply, 1)
	r.plans[id] = planWait{item: item, reply: reply}
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		delete(r.plans, id)
		r.mu.Unlock()
	}()

	select {
	case response := <-reply:
		if err := ctx.Err(); err != nil {
			return ConfirmPlanOutput{}, err
		}
		switch response.decision {
		case "approve":
			agentrun.NotifyPlanApproved(ctx)
			return ConfirmPlanOutput{Approved: true}, nil
		case "revise":
			return ConfirmPlanOutput{Approved: false, Feedback: response.feedback}, nil
		default:
			return ConfirmPlanOutput{Approved: false, Rejected: true, Feedback: response.feedback}, nil
		}
	case <-ctx.Done():
		return ConfirmPlanOutput{}, ctx.Err()
	}
}

func registerConfirmPlanTool(r *Registry) {
	Register(r, "ConfirmPlan", "在计划模式下，向用户提交整理好的执行计划并等待确认。用户确认后才可执行计划，也可以提出修改意见或选择不执行计划。", false, func(ctx context.Context, input ConfirmPlanInput) any {
		result, err := r.confirmPlan(ctx, conversationIDFromContext(ctx), input)
		if err != nil {
			return map[string]string{"error": err.Error()}
		}
		return result
	})
}
