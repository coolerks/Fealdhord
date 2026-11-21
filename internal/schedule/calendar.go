// Package schedule handles persisted local-calendar task timing.
package schedule

import (
	_ "time/tzdata"

	"slices"
	"time"
)

type Calendar struct {
	Kind      string `json:"kind"`
	Timezone  string `json:"timezone"`
	LocalTime string `json:"local_time"`
	Weekdays  []int  `json:"weekdays,omitempty"`
	OnceAt    string `json:"once_at,omitempty"`
}

func (c Calendar) Valid() bool {
	if c.Timezone == "" || c.Timezone == "Local" {
		return false
	}
	if _, err := time.LoadLocation(c.Timezone); err != nil {
		return false
	}
	if c.Kind == "once" {
		return !c.once().IsZero()
	}
	if _, err := time.Parse("15:04", c.LocalTime); err != nil {
		return false
	}
	switch c.Kind {
	case "daily":
		return true
	case "weekly":
		return len(c.Weekdays) > 0 && !slices.ContainsFunc(c.Weekdays, func(day int) bool { return day < 0 || day > 6 })
	default:
		return false
	}
}

func WallTime(day time.Time, clock string) time.Time {
	parsed, err := time.Parse("15:04", clock)
	if err != nil {
		return time.Time{}
	}
	wall := time.Date(day.Year(), day.Month(), day.Day(), parsed.Hour(), parsed.Minute(), 0, 0, time.UTC)
	_, earlier := wall.Add(-36 * time.Hour).In(day.Location()).Zone()
	_, later := wall.Add(36 * time.Hour).In(day.Location()).Zone()
	for advance := 0; advance <= 24*60; advance++ {
		var found time.Time
		for _, offset := range []int{earlier, later} {
			candidate := wall.Add(-time.Duration(offset) * time.Second).In(day.Location())
			if candidate.Format("2006-01-02 15:04") == wall.Format("2006-01-02 15:04") &&
				(found.IsZero() || candidate.Before(found)) {
				found = candidate
			}
		}
		if !found.IsZero() {
			return found
		}
		wall = wall.Add(time.Minute)
	}
	return time.Time{}
}

func (c Calendar) once() time.Time {
	if at, err := time.Parse(time.RFC3339, c.OnceAt); err == nil {
		return at
	}
	loc, err := time.LoadLocation(c.Timezone)
	if err != nil {
		return time.Time{}
	}
	local, err := time.Parse("2006-01-02T15:04", c.OnceAt)
	if err != nil {
		return time.Time{}
	}
	day := time.Date(local.Year(), local.Month(), local.Day(), 12, 0, 0, 0, loc)
	return WallTime(day, local.Format("15:04"))
}

func (c Calendar) Next(after time.Time) time.Time {
	if !c.Valid() {
		return time.Time{}
	}
	if c.Kind == "once" {
		at := c.once()
		if at.After(after) {
			return at
		}
		return time.Time{}
	}
	loc, _ := time.LoadLocation(c.Timezone)
	day := after.In(loc)
	for i := 0; i <= 7; i++ {
		if c.Kind == "daily" || slices.Contains(c.Weekdays, int(day.Weekday())) {
			if candidate := WallTime(day, c.LocalTime); candidate.After(after) {
				return candidate
			}
		}
		day = day.AddDate(0, 0, 1)
	}
	return time.Time{}
}

func Millis(value time.Time) int64 {
	if value.IsZero() {
		return 0
	}
	return value.UnixMilli()
}
