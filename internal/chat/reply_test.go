package chat

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/coolerks/Fealdhord/internal/ai"
)

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
