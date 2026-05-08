package query

import (
	"fmt"
	"time"
)

type Period string

const (
	PeriodToday     Period = "today"
	PeriodYesterday Period = "yesterday"
	PeriodThisWeek  Period = "this-week"
	PeriodLastWeek  Period = "last-week"
	PeriodThisMonth Period = "this-month"
)

type Window struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

func ParsePeriod(raw string) (Period, error) {
	if raw == "" {
		return PeriodToday, nil
	}
	p := Period(raw)
	switch p {
	case PeriodToday, PeriodYesterday, PeriodThisWeek, PeriodLastWeek, PeriodThisMonth:
		return p, nil
	default:
		return "", fmt.Errorf("unknown period %q", raw)
	}
}

func WindowFor(period Period, now time.Time, loc *time.Location) Window {
	if loc == nil {
		loc = time.Local
	}
	n := now.In(loc)
	dayStart := time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, loc)
	switch period {
	case PeriodYesterday:
		start := dayStart.AddDate(0, 0, -1)
		return Window{Start: start, End: dayStart}
	case PeriodThisWeek:
		start := mondayStart(dayStart)
		return Window{Start: start, End: start.AddDate(0, 0, 7)}
	case PeriodLastWeek:
		end := mondayStart(dayStart)
		return Window{Start: end.AddDate(0, 0, -7), End: end}
	case PeriodThisMonth:
		start := time.Date(n.Year(), n.Month(), 1, 0, 0, 0, 0, loc)
		return Window{Start: start, End: start.AddDate(0, 1, 0)}
	case PeriodToday:
		fallthrough
	default:
		return Window{Start: dayStart, End: dayStart.AddDate(0, 0, 1)}
	}
}

func mondayStart(dayStart time.Time) time.Time {
	weekday := int(dayStart.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	return dayStart.AddDate(0, 0, -(weekday - 1))
}

func (w Window) Contains(ts time.Time) bool {
	if ts.IsZero() {
		return false
	}
	t := ts.In(w.Start.Location())
	return !t.Before(w.Start) && t.Before(w.End)
}
