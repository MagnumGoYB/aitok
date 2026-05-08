package query

import (
	"testing"
	"time"

	"github.com/sosbs/aitok/internal/usage"
)

func TestAggregateFiltersAndGroups(t *testing.T) {
	loc := time.UTC
	window := Window{Start: time.Date(2026, 5, 8, 0, 0, 0, 0, loc), End: time.Date(2026, 5, 9, 0, 0, 0, 0, loc)}
	events := []usage.UsageEvent{
		{Timestamp: time.Date(2026, 5, 8, 1, 0, 0, 0, loc), Tool: usage.ToolCodex, Model: "gpt-5.4", Provider: "openai", CWD: "/repo", Usage: usage.TokenUsage{Input: 10, Output: 2}},
		{Timestamp: time.Date(2026, 5, 8, 2, 0, 0, 0, loc), Tool: usage.ToolCodex, Model: "gpt-5.4", Provider: "openai", CWD: "/repo", Usage: usage.TokenUsage{Input: 3, Output: 1}},
		{Timestamp: time.Date(2026, 5, 7, 2, 0, 0, 0, loc), Tool: usage.ToolClaude, Model: "claude", Provider: "unknown", Usage: usage.TokenUsage{Input: 100}},
	}
	results := Aggregate(events, window, Filters{Tools: []string{"codex"}, Models: []string{"gpt-5.4"}}, GroupBy{"tool", "model", "provider", "day"})
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	if results[0].Events != 2 {
		t.Fatalf("events = %d, want 2", results[0].Events)
	}
	if got := results[0].Usage.NormalizedTotal(); got != 16 {
		t.Fatalf("total = %d, want 16", got)
	}
	if got := results[0].Key["day"]; got != "2026-05-08" {
		t.Fatalf("day = %s, want 2026-05-08", got)
	}
}
