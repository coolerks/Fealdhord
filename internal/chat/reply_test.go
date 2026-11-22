package chat

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/coolerks/Fealdhord/internal/ai"
)

func TestToolPreviewKeepsValuesAndMasksCredentials(t *testing.T) {
	input := `{"nested":[{"Authorization":"fake-auth","url":"https://someone:fake-password@example.test/?token=fake-token&q=hello","text":"{\"clientSecret\":\"fake-secret\"}"}],"query":"科技","count":9007199254740993}`
	display, truncated := toolPreview(input)
	if truncated || strings.Contains(*display, "fake-") || !strings.Contains(*display, "科技") ||
		!strings.Contains(*display, "q=hello") || !strings.Contains(*display, "9007199254740993") {
		t.Fatalf("invalid tool preview: %s", *display)
	}
	display, truncated = toolPreview(strings.Repeat("文", maxToolPreviewBytes))
	if !truncated || len(*display) > maxToolPreviewBytes || len(*display)%len("文") != 0 {
		t.Fatalf("tool preview did not truncate at a character boundary: %d", len(*display))
	}
}

func TestReplyDetailsMeasureFirstVisibleTextAndFinalRoundSpeed(t *testing.T) {
	start := time.Now()
	var work replyWork
	var usage replyUsage
	observe := func(kind, text string, elapsed time.Duration) {
		work.observe(ai.ModelEvent{Type: kind, Text: text}, start.Add(elapsed))
	}
	usage.startRound()
	observe("message_start", "", 0)
	observe("reasoning_summary_delta", "查找文章", time.Second)
	observe("text_delta", "  ", 2*time.Second)
	observe("text_delta", "先查找文章。", 3*time.Second)
	firstOutput := 80
	usage.observe(&ai.Usage{OutputTokens: &firstOutput})

	usage.startRound()
	observe("message_start", "", 4*time.Second)
	observe("reasoning_summary_delta", "整理结果", 5*time.Second)
	observe("text_delta", "结果如下。", 6*time.Second)
	finalOutput := 40
	usage.observe(&ai.Usage{OutputTokens: &finalOutput})

	var details ReplyDetails
	if err := json.Unmarshal([]byte(usage.details(start, start.Add(8*time.Second), "低", &work, "complete")), &details); err != nil {
		t.Fatal(err)
	}
	if details.FirstTokenMS == nil || *details.FirstTokenMS != 3000 || details.OutputTokensPerSecond == nil || *details.OutputTokensPerSecond != 20 {
		t.Fatalf("unexpected reply timing: %v", details)
	}
	if details.Work == nil || details.Work.ReasoningSummary != "查找文章\n\n整理结果" || details.Work.DurationMS != 5000 {
		t.Fatalf("unexpected work: %+v", details.Work)
	}
}

func TestReplyWorkKeepsReasoningAroundToolAtSameOffset(t *testing.T) {
	start := time.Now()
	var work replyWork
	work.observe(ai.ModelEvent{Type: "reasoning_text_delta", Text: "先查🙂"}, start.Add(time.Second))
	work.observe(ai.ModelEvent{Type: "tool_work_started", ToolName: "search"}, start.Add(2*time.Second))
	work.observe(ai.ModelEvent{Type: "tool_work_finished", ToolStatus: "completed"}, start.Add(3*time.Second))
	work.observe(ai.ModelEvent{Type: "reasoning_text_delta", Text: "已找到"}, start.Add(4*time.Second))
	work.observe(ai.ModelEvent{Type: "text_delta", Text: "结果"}, start.Add(5*time.Second))
	positions := work.snapshot(start, start.Add(6*time.Second), "complete").Positions
	if len(positions) != 3 || positions[0].ReasoningEnd != 4 || positions[0].ToolEnd != 0 ||
		positions[1].ReasoningEnd != 4 || positions[1].ToolEnd != 1 ||
		positions[2].ReasoningEnd != 7 || positions[2].ToolEnd != 1 ||
		positions[2].Offset != 0 || positions[2].DurationMS != 5000 {
		t.Fatalf("unexpected order and timing: %+v", positions)
	}
}
