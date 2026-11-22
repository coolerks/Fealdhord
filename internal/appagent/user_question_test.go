package appagent

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/coolerks/Fealdhord/internal/ai"
)

func TestAskUserQuestionRoundTrip(t *testing.T) {
	r := NewRegistry()
	registerAskUserQuestionTool(r)
	tools := r.ChatTools("conv-1")
	if len(tools) != 1 || tools[0].Definition.Name != "AskUserQuestion" {
		t.Fatalf("unexpected tools: %+v", tools)
	}

	args := `{"questions":[{"header":"范围","question":"想看哪类内容？","multi_select":false,"options":[{"label":"仅未读"},{"label":"全部文章"}]},{"header":"格式","question":"希望包含哪些字段？","multi_select":true,"options":[{"label":"标题"},{"label":"摘要"}]}]}`
	done := make(chan string, 1)
	go func() {
		out, err := tools[0].Read(context.Background(), ai.ToolCall{ID: "call-1", Name: "AskUserQuestion", ArgumentsJSON: args})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		done <- out
	}()

	var pending PendingQuestion
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		items := r.PendingQuestions()
		if len(items) == 1 {
			pending = items[0]
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if pending.ID == "" || pending.ConversationID != "conv-1" || len(pending.Questions) != 2 {
		t.Fatalf("unexpected pending question: %+v", pending)
	}

	ok := r.AnswerQuestion(pending.ID, []UserQuestionAnswer{
		{Question: "想看哪类内容？", Selected: []string{"仅未读"}, Note: "最近三天"},
		{Question: "希望包含哪些字段？", Selected: []string{"标题", "摘要"}},
	}, false)
	if !ok {
		t.Fatal("expected AnswerQuestion to return true")
	}

	raw := <-done
	var output AskUserQuestionOutput
	if err := json.Unmarshal([]byte(raw), &output); err != nil {
		t.Fatalf("unmarshal output: %v (%s)", err, raw)
	}
	if output.Skipped || len(output.Answers) != 2 || output.Answers[0].Note != "最近三天" || len(output.Answers[1].Selected) != 2 {
		t.Fatalf("unexpected output: %+v", output)
	}
}

func TestAskUserQuestionSkippedAndNonInteractive(t *testing.T) {
	r := NewRegistry()
	registerAskUserQuestionTool(r)
	tool := r.Tools()[0]

	args := json.RawMessage(`{"questions":[{"question":"是否继续？","options":[{"label":"继续"},{"label":"取消"}]}]}`)
	out, err := r.execute(context.Background(), "mcp-client", tool, args)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if out != `{"error":"InteractiveSessionRequired"}` {
		t.Fatalf("expected InteractiveSessionRequired, got %s", out)
	}

	chatTool := r.ChatTools("conv-2")[0]
	done := make(chan string, 1)
	go func() {
		res, _ := chatTool.Read(context.Background(), ai.ToolCall{ID: "call-2", Name: "AskUserQuestion", ArgumentsJSON: string(args)})
		done <- res
	}()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if items := r.PendingQuestions(); len(items) == 1 {
			r.AnswerQuestion(items[0].ID, nil, true)
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	var skipped AskUserQuestionOutput
	if err := json.Unmarshal([]byte(<-done), &skipped); err != nil || !skipped.Skipped {
		t.Fatalf("expected skipped output, got %+v (err=%v)", skipped, err)
	}
}
