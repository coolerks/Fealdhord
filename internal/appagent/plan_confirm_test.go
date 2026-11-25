package appagent

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestConfirmPlanApproveReviseReject(t *testing.T) {
	r := NewRegistry()
	registerConfirmPlanTool(r)

	var confirm Tool
	for _, tool := range r.Tools() {
		if tool.Name == "ConfirmPlan" {
			confirm = tool
			break
		}
	}
	if confirm.Name == "" {
		t.Fatal("ConfirmPlan tool not registered")
	}

	args := json.RawMessage(`{"title":"实现方案","summary":"概述","steps":["步骤一","步骤二"]}`)

	t.Run("approve triggers callback", func(t *testing.T) {
		approved := false
		ctx := WithPlanApprovedCallback(withConversationID(context.Background(), "conv-1"), func() {
			approved = true
		})
		done := make(chan any, 1)
		go func() {
			out, err := confirm.invoke(ctx, args)
			if err != nil {
				t.Errorf("invoke err: %v", err)
			}
			done <- out
		}()
		var pending []PendingPlan
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			pending = r.PendingPlans()
			if len(pending) == 1 {
				break
			}
			time.Sleep(5 * time.Millisecond)
		}
		if len(pending) != 1 || pending[0].Title != "实现方案" || len(pending[0].Steps) != 2 {
			t.Fatalf("unexpected pending plans: %+v", pending)
		}
		if !r.ResolvePlan(pending[0].ID, "approve", "") {
			t.Fatal("ResolvePlan returned false")
		}
		got := (<-done).(ConfirmPlanOutput)
		if !got.Approved || got.Rejected || !approved {
			t.Fatalf("unexpected approve result: %+v callback=%v", got, approved)
		}
	})

	t.Run("revise returns feedback", func(t *testing.T) {
		ctx := withConversationID(context.Background(), "conv-2")
		done := make(chan any, 1)
		go func() {
			out, _ := confirm.invoke(ctx, args)
			done <- out
		}()
		var pending []PendingPlan
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			pending = r.PendingPlans()
			if len(pending) == 1 {
				break
			}
			time.Sleep(5 * time.Millisecond)
		}
		if !r.ResolvePlan(pending[0].ID, "revise", "请拆分步骤二") {
			t.Fatal("ResolvePlan revise returned false")
		}
		got := (<-done).(ConfirmPlanOutput)
		if got.Approved || got.Rejected || got.Feedback != "请拆分步骤二" {
			t.Fatalf("unexpected revise result: %+v", got)
		}
	})

	t.Run("reject stops execution", func(t *testing.T) {
		ctx := withConversationID(context.Background(), "conv-3")
		done := make(chan any, 1)
		go func() {
			out, _ := confirm.invoke(ctx, args)
			done <- out
		}()
		var pending []PendingPlan
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			pending = r.PendingPlans()
			if len(pending) == 1 {
				break
			}
			time.Sleep(5 * time.Millisecond)
		}
		if !r.ResolvePlan(pending[0].ID, "reject", "") {
			t.Fatal("ResolvePlan reject returned false")
		}
		got := (<-done).(ConfirmPlanOutput)
		if got.Approved || !got.Rejected {
			t.Fatalf("unexpected reject result: %+v", got)
		}
	})
}
