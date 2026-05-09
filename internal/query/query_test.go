package query

import (
	"testing"
	"time"

	"github.com/MagnumGoYB/aitok/internal/usage"
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

func TestAggregateIncludesRequestsAndCost(t *testing.T) {
	loc := time.UTC
	window := Window{Start: time.Date(2026, 5, 8, 0, 0, 0, 0, loc), End: time.Date(2026, 5, 9, 0, 0, 0, 0, loc)}
	events := []usage.UsageEvent{
		{Timestamp: time.Date(2026, 5, 8, 1, 0, 0, 0, loc), Tool: usage.ToolCodex, Model: "gpt-5.4", Provider: "openai", Usage: usage.TokenUsage{Input: 1_000_000}},
		{Timestamp: time.Date(2026, 5, 8, 2, 0, 0, 0, loc), Tool: usage.ToolCodex, Model: "gpt-5.4", Provider: "openai", Usage: usage.TokenUsage{Output: 100_000}},
	}
	results := AggregateWithCosts(events, window, Filters{}, GroupBy{"tool", "model"}, func(event usage.UsageEvent) Cost {
		return Cost{USD: float64(event.Usage.Input)/1_000_000 + float64(event.Usage.Output)/100_000}
	})
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	if got := results[0].Requests; got != 2 {
		t.Fatalf("requests = %d, want 2", got)
	}
	if got := results[0].CostUSD; got != 2 {
		t.Fatalf("cost = %.4f, want 2", got)
	}
}

func TestAccumulatorMatchesAggregateWithCosts(t *testing.T) {
	loc := time.UTC
	window := Window{Start: time.Date(2026, 5, 8, 0, 0, 0, 0, loc), End: time.Date(2026, 5, 9, 0, 0, 0, 0, loc)}
	events := []usage.UsageEvent{
		{Timestamp: time.Date(2026, 5, 8, 1, 0, 0, 0, loc), Tool: usage.ToolCodex, Model: "gpt-5.4", Provider: "openai", CWD: "/repo", Usage: usage.TokenUsage{Input: 1_000_000}},
		{Timestamp: time.Date(2026, 5, 8, 2, 0, 0, 0, loc), Tool: usage.ToolCodex, Model: "gpt-5.4", Provider: "openai", CWD: "/repo", Usage: usage.TokenUsage{Output: 100_000}},
		{Timestamp: time.Date(2026, 5, 7, 2, 0, 0, 0, loc), Tool: usage.ToolClaude, Model: "claude", Provider: "unknown", Usage: usage.TokenUsage{Input: 100}},
	}
	costFor := func(event usage.UsageEvent) Cost {
		return Cost{USD: float64(event.Usage.Input)/1_000_000 + float64(event.Usage.Output)/100_000}
	}
	acc := NewAccumulator(window, Filters{Tools: []string{"codex"}}, GroupBy{"tool", "model"}, costFor)
	for _, event := range events {
		acc.Add(event)
	}
	got := acc.Results()
	if len(got) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(got))
	}
	wantUsage := usage.TokenUsage{Input: 1_000_000, Output: 100_000, Total: 1_100_000}
	if got[0].Key["tool"] != "codex" || got[0].Key["model"] != "gpt-5.4" {
		t.Fatalf("key = %+v, want codex/gpt-5.4", got[0].Key)
	}
	if got[0].Requests != 2 || got[0].Events != 2 {
		t.Fatalf("requests/events = %d/%d, want 2/2", got[0].Requests, got[0].Events)
	}
	if got[0].CostUSD != 2 {
		t.Fatalf("cost = %.4f, want 2", got[0].CostUSD)
	}
	if got[0].Usage != wantUsage {
		t.Fatalf("usage = %+v, want %+v", got[0].Usage, wantUsage)
	}
}
