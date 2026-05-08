package query

import (
	"testing"
	"time"
)

func TestWindowForPeriods(t *testing.T) {
	loc := time.FixedZone("test", 8*60*60)
	now := time.Date(2026, 5, 8, 13, 30, 0, 0, loc)
	tests := []struct {
		period Period
		start  string
		end    string
	}{
		{PeriodToday, "2026-05-08T00:00:00+08:00", "2026-05-09T00:00:00+08:00"},
		{PeriodYesterday, "2026-05-07T00:00:00+08:00", "2026-05-08T00:00:00+08:00"},
		{PeriodThisWeek, "2026-05-04T00:00:00+08:00", "2026-05-11T00:00:00+08:00"},
		{PeriodLastWeek, "2026-04-27T00:00:00+08:00", "2026-05-04T00:00:00+08:00"},
		{PeriodThisMonth, "2026-05-01T00:00:00+08:00", "2026-06-01T00:00:00+08:00"},
	}
	for _, tt := range tests {
		window := WindowFor(tt.period, now, loc)
		if got := window.Start.Format(time.RFC3339); got != tt.start {
			t.Fatalf("%s start = %s, want %s", tt.period, got, tt.start)
		}
		if got := window.End.Format(time.RFC3339); got != tt.end {
			t.Fatalf("%s end = %s, want %s", tt.period, got, tt.end)
		}
	}
}
