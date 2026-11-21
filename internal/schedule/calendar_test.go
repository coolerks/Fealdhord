package schedule

import (
	"testing"
	"time"
)

func TestCalendarSpringGapAndRepeatedHour(t *testing.T) {
	schedule := Calendar{Kind: "daily", Timezone: "America/New_York", LocalTime: "02:30"}
	start := time.Date(2026, time.March, 8, 6, 0, 0, 0, time.UTC)
	if got := schedule.Next(start).Format(time.RFC3339); got != "2026-03-08T03:00:00-04:00" {
		t.Fatalf("spring gap: %s", got)
	}
	schedule.LocalTime = "01:30"
	start = time.Date(2026, time.November, 1, 4, 0, 0, 0, time.UTC)
	first := schedule.Next(start)
	if got := first.Format(time.RFC3339); got != "2026-11-01T01:30:00-04:00" {
		t.Fatalf("repeated hour first occurrence: %s", got)
	}
	if got := schedule.Next(first).Format(time.RFC3339); got != "2026-11-02T01:30:00-05:00" {
		t.Fatalf("repeated hour must not send twice: %s", got)
	}
}
