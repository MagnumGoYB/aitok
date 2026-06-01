package query

import (
	"reflect"
	"testing"
	"time"

	"github.com/MagnumGoYB/aitok/internal/usage"
)

func TestDefaultGroupByReturnsIndependentSlice(t *testing.T) {
	first := DefaultGroupBy()
	first[0] = "cwd"

	got := DefaultGroupBy()
	want := GroupBy{"tool", "model", "provider"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("DefaultGroupBy() = %#v, want %#v", got, want)
	}
}

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
	if got := results[0].Tool; got != "codex" {
		t.Fatalf("tool = %q, want codex", got)
	}
	if got := results[0].CostUSD; got != 2 {
		t.Fatalf("cost = %.4f, want 2", got)
	}
}

func TestThreadAccumulatorGroupResultsPopulatesTool(t *testing.T) {
	loc := time.UTC
	window := Window{Start: time.Date(2026, 5, 8, 0, 0, 0, 0, loc), End: time.Date(2026, 5, 9, 0, 0, 0, 0, loc)}
	acc := NewThreadAccumulator(window, Filters{}, nil)
	acc.Add(usage.UsageEvent{
		ID:        "evt-1",
		ThreadID:  "thread-1",
		Timestamp: time.Date(2026, 5, 8, 1, 0, 0, 0, loc),
		Tool:      usage.ToolCodex,
		Model:     "gpt-5.4",
		Provider:  "openai",
		Usage:     usage.TokenUsage{Input: 1},
	})
	results := acc.GroupResults(DefaultGroupBy())
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	if got := results[0].Tool; got != "codex" {
		t.Fatalf("tool = %q, want codex", got)
	}
}

func TestAggregateMarksMixedToolWhenGroupCombinesTools(t *testing.T) {
	loc := time.UTC
	window := Window{Start: time.Date(2026, 5, 8, 0, 0, 0, 0, loc), End: time.Date(2026, 5, 9, 0, 0, 0, 0, loc)}
	events := []usage.UsageEvent{
		{Timestamp: time.Date(2026, 5, 8, 1, 0, 0, 0, loc), Tool: usage.ToolCodex, Model: "gpt-5.4", Provider: "openai", Usage: usage.TokenUsage{Input: 1}},
		{Timestamp: time.Date(2026, 5, 8, 2, 0, 0, 0, loc), Tool: usage.ToolClaude, Model: "gpt-5.4", Provider: "openai", Usage: usage.TokenUsage{Input: 2}},
	}
	results := Aggregate(events, window, Filters{}, GroupBy{"model"})
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	if got := results[0].Tool; got != "mixed" {
		t.Fatalf("tool = %q, want mixed", got)
	}
}

func TestAggregateSortsCostByRawAmountAcrossCurrencies(t *testing.T) {
	loc := time.UTC
	window := Window{Start: time.Date(2026, 5, 8, 0, 0, 0, 0, loc), End: time.Date(2026, 5, 9, 0, 0, 0, 0, loc)}
	events := []usage.UsageEvent{
		{Timestamp: time.Date(2026, 5, 8, 1, 0, 0, 0, loc), Tool: usage.ToolCodex, Model: "usd-model", Provider: "openai", Usage: usage.TokenUsage{Input: 1}},
		{Timestamp: time.Date(2026, 5, 8, 2, 0, 0, 0, loc), Tool: usage.ToolReasonix, Model: "cny-model", Provider: "deepseek", Usage: usage.TokenUsage{Input: 1}},
	}
	acc := NewAccumulatorWithSort(window, Filters{}, GroupBy{"model"}, SortByCost, func(event usage.UsageEvent) Cost {
		if event.Provider == "deepseek" {
			return Cost{USD: 2, Currency: "CNY", Source: "default"}
		}
		return Cost{USD: 10, Currency: "USD", Source: "default"}
	})
	for _, event := range events {
		acc.Add(event)
	}
	results := acc.Results()
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}
	if results[0].Key["model"] != "usd-model" || results[1].Key["model"] != "cny-model" {
		t.Fatalf("cost sort should use raw amount only, got %+v", results)
	}
}

func TestAggregateCarriesPriceDetails(t *testing.T) {
	loc := time.UTC
	window := Window{Start: time.Date(2026, 5, 8, 0, 0, 0, 0, loc), End: time.Date(2026, 5, 9, 0, 0, 0, 0, loc)}
	events := []usage.UsageEvent{
		{Timestamp: time.Date(2026, 5, 8, 1, 0, 0, 0, loc), Tool: usage.ToolCodex, Model: "gpt-5.4", Provider: "team-a", Usage: usage.TokenUsage{Input: 1_000_000}},
		{Timestamp: time.Date(2026, 5, 8, 2, 0, 0, 0, loc), Tool: usage.ToolCodex, Model: "gpt-5.4", Provider: "openai", Usage: usage.TokenUsage{Input: 1_000_000}},
	}
	results := AggregateWithCosts(events, window, Filters{}, GroupBy{"model", "provider"}, func(event usage.UsageEvent) Cost {
		if event.Provider == "team-a" {
			return Cost{USD: 2, Source: "user", InputUSDPerMTok: 2, OutputUSDPerMTok: 20, CacheHitUSDPerMTok: 0.2, CacheMakeUSDPerMTok: 2}
		}
		return Cost{USD: 1, Source: "default", InputUSDPerMTok: 1, OutputUSDPerMTok: 10, CacheHitUSDPerMTok: 0.1, CacheMakeUSDPerMTok: 1}
	})
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}
	byProvider := map[string]Result{}
	for _, result := range results {
		byProvider[result.Key["provider"]] = result
	}
	if got := byProvider["team-a"].Price; got == nil || got.Source != "custom" || got.InputUSDPerMTok != 2 {
		t.Fatalf("custom price not carried: %+v", byProvider["team-a"])
	}
	if got := byProvider["openai"].Price; got == nil || got.Source != "official" || got.InputUSDPerMTok != 1 {
		t.Fatalf("official price not carried: %+v", byProvider["openai"])
	}
}

func TestAggregateMarksMixedPricesWhenGroupCombinesSources(t *testing.T) {
	loc := time.UTC
	window := Window{Start: time.Date(2026, 5, 8, 0, 0, 0, 0, loc), End: time.Date(2026, 5, 9, 0, 0, 0, 0, loc)}
	events := []usage.UsageEvent{
		{Timestamp: time.Date(2026, 5, 8, 1, 0, 0, 0, loc), Tool: usage.ToolCodex, Model: "gpt-5.4", Provider: "team-a", Usage: usage.TokenUsage{Input: 1}},
		{Timestamp: time.Date(2026, 5, 8, 2, 0, 0, 0, loc), Tool: usage.ToolCodex, Model: "gpt-5.4", Provider: "openai", Usage: usage.TokenUsage{Input: 1}},
	}
	results := AggregateWithCosts(events, window, Filters{}, GroupBy{"model"}, func(event usage.UsageEvent) Cost {
		if event.Provider == "team-a" {
			return Cost{USD: 2, Source: "user", InputUSDPerMTok: 2}
		}
		return Cost{USD: 1, Source: "default", InputUSDPerMTok: 1}
	})
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	if results[0].PriceSource != "mixed" || results[0].Price == nil || results[0].Price.Source != "mixed" {
		t.Fatalf("combined group should mark mixed pricing: %+v", results[0])
	}
	if got := results[0].Price.Components; len(got) != 2 || got[0].Source != "custom" || got[0].InputUSDPerMTok != 2 || got[1].Source != "official" || got[1].InputUSDPerMTok != 1 {
		t.Fatalf("mixed pricing should retain component rates, got %+v", got)
	}
}

func TestMergeAggregatedPricePreservesComponentsFromMultipleMixedPrices(t *testing.T) {
	existing := &Price{Source: "mixed", Components: []Price{
		{Source: "custom", InputUSDPerMTok: 2, OutputUSDPerMTok: 20},
		{Source: "official", InputUSDPerMTok: 5, OutputUSDPerMTok: 30},
	}}
	next := &Price{Source: "mixed", Components: []Price{
		{Source: "custom", InputUSDPerMTok: 2, OutputUSDPerMTok: 20},
		{Source: "official", InputUSDPerMTok: 7, OutputUSDPerMTok: 40},
	}}
	got := mergeAggregatedPrice(existing, next)
	if got == nil || got.Source != "mixed" {
		t.Fatalf("merge should keep mixed price: %+v", got)
	}
	if len(got.Components) != 3 {
		t.Fatalf("merge should preserve unique mixed components, got %+v", got.Components)
	}
	wantInputs := []float64{2, 5, 7}
	for i, want := range wantInputs {
		if got.Components[i].InputUSDPerMTok != want {
			t.Fatalf("component[%d].input = %.4g, want %.4g: %+v", i, got.Components[i].InputUSDPerMTok, want, got.Components)
		}
	}
	if len(existing.Components) != 2 || len(next.Components) != 2 {
		t.Fatalf("merge should not mutate inputs: existing=%+v next=%+v", existing.Components, next.Components)
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

func TestThreadAccumulatorSortsByTokenUsageThenCost(t *testing.T) {
	loc := time.UTC
	window := Window{Start: time.Date(2026, 5, 8, 0, 0, 0, 0, loc), End: time.Date(2026, 5, 9, 0, 0, 0, 0, loc)}
	events := []usage.UsageEvent{
		{ID: "older-expensive", Timestamp: time.Date(2026, 5, 8, 3, 0, 0, 0, loc), Tool: usage.ToolCodex, ThreadID: "older-expensive", ThreadCreatedAt: time.Date(2026, 5, 8, 1, 0, 0, 0, loc), Usage: usage.TokenUsage{Input: 1_000_000}},
		{ID: "newer-cheap", Timestamp: time.Date(2026, 5, 8, 4, 0, 0, 0, loc), Tool: usage.ToolCodex, ThreadID: "newer-cheap", ThreadCreatedAt: time.Date(2026, 5, 8, 2, 0, 0, 0, loc), Usage: usage.TokenUsage{Input: 1}},
		{ID: "newer-expensive", Timestamp: time.Date(2026, 5, 8, 5, 0, 0, 0, loc), Tool: usage.ToolCodex, ThreadID: "newer-expensive", ThreadCreatedAt: time.Date(2026, 5, 8, 2, 0, 0, 0, loc), Usage: usage.TokenUsage{Input: 2}},
	}
	acc := NewThreadAccumulator(window, Filters{}, func(event usage.UsageEvent) Cost {
		return Cost{USD: float64(event.Usage.Input)}
	})
	for _, event := range events {
		acc.Add(event)
	}
	got := acc.Results()
	if len(got) != 3 {
		t.Fatalf("len(threads) = %d, want 3", len(got))
	}
	want := []string{"older-expensive", "newer-expensive", "newer-cheap"}
	for i, id := range want {
		if got[i].ID != id {
			t.Fatalf("thread order[%d] = %s, want %s; all=%+v", i, got[i].ID, id, got)
		}
	}
}

func TestThreadAccumulatorSortsByCostWhenRequested(t *testing.T) {
	loc := time.UTC
	window := Window{Start: time.Date(2026, 5, 8, 0, 0, 0, 0, loc), End: time.Date(2026, 5, 9, 0, 0, 0, 0, loc)}
	events := []usage.UsageEvent{
		{ID: "high-token-low-cost", Timestamp: time.Date(2026, 5, 8, 1, 0, 0, 0, loc), Tool: usage.ToolCodex, ThreadID: "high-token-low-cost", Usage: usage.TokenUsage{Input: 100}},
		{ID: "low-token-high-cost", Timestamp: time.Date(2026, 5, 8, 2, 0, 0, 0, loc), Tool: usage.ToolCodex, ThreadID: "low-token-high-cost", Usage: usage.TokenUsage{Input: 10}},
	}
	acc := NewThreadAccumulatorWithSort(window, Filters{}, SortByCost, func(event usage.UsageEvent) Cost {
		if event.ThreadID == "low-token-high-cost" {
			return Cost{USD: 10}
		}
		return Cost{USD: 1}
	})
	for _, event := range events {
		acc.Add(event)
	}
	got := acc.Results()
	if len(got) != 2 || got[0].ID != "low-token-high-cost" {
		t.Fatalf("cost sort should order threads by estimated cost desc, got %+v", got)
	}
}

func TestAccumulatorSortsByCostWhenRequested(t *testing.T) {
	loc := time.UTC
	window := Window{Start: time.Date(2026, 5, 8, 0, 0, 0, 0, loc), End: time.Date(2026, 5, 9, 0, 0, 0, 0, loc)}
	events := []usage.UsageEvent{
		{Timestamp: time.Date(2026, 5, 8, 1, 0, 0, 0, loc), Tool: usage.ToolCodex, Model: "cheap", Usage: usage.TokenUsage{Input: 100}},
		{Timestamp: time.Date(2026, 5, 8, 2, 0, 0, 0, loc), Tool: usage.ToolCodex, Model: "expensive", Usage: usage.TokenUsage{Input: 10}},
	}
	acc := NewAccumulatorWithSort(window, Filters{}, GroupBy{"model"}, SortByCost, func(event usage.UsageEvent) Cost {
		if event.Model == "expensive" {
			return Cost{USD: 10}
		}
		return Cost{USD: 1}
	})
	for _, event := range events {
		acc.Add(event)
	}
	got := acc.Results()
	if len(got) != 2 || got[0].Key["model"] != "expensive" {
		t.Fatalf("cost sort should order model usage by estimated cost desc, got %+v", got)
	}
}

func TestThreadAccumulatorGroupsUsageAndCostByThread(t *testing.T) {
	loc := time.UTC
	window := Window{Start: time.Date(2026, 5, 8, 0, 0, 0, 0, loc), End: time.Date(2026, 5, 9, 0, 0, 0, 0, loc)}
	events := []usage.UsageEvent{
		{ID: "a", Timestamp: time.Date(2026, 5, 8, 1, 0, 0, 0, loc), Tool: usage.ToolCodex, Model: "gpt-5.4", Provider: "openai", CWD: "/repo", ThreadID: "thread-a", ThreadName: "Custom title", ThreadSource: "/tmp/a.jsonl", Usage: usage.TokenUsage{Input: 1_000_000}},
		{ID: "b", Timestamp: time.Date(2026, 5, 8, 2, 0, 0, 0, loc), Tool: usage.ToolCodex, Model: "gpt-5.4", Provider: "openai", CWD: "/repo", ThreadID: "thread-a", ThreadName: "Custom title", ThreadSource: "/tmp/a.jsonl", Usage: usage.TokenUsage{Output: 100_000}},
		{ID: "c", Timestamp: time.Date(2026, 5, 8, 3, 0, 0, 0, loc), Tool: usage.ToolClaude, Model: "claude", Provider: "unknown", ThreadID: "thread-b", ThreadName: "Other", Usage: usage.TokenUsage{Input: 2}},
		{ID: "old", Timestamp: time.Date(2026, 5, 7, 1, 0, 0, 0, loc), Tool: usage.ToolCodex, Model: "gpt-5.4", Provider: "openai", ThreadID: "old", Usage: usage.TokenUsage{Input: 9}},
	}
	acc := NewThreadAccumulator(window, Filters{Tools: []string{"codex"}}, func(event usage.UsageEvent) Cost {
		return Cost{USD: float64(event.Usage.Input)/1_000_000 + float64(event.Usage.Output)/100_000}
	})
	for _, event := range events {
		acc.Add(event)
	}
	got := acc.Results()
	if len(got) != 1 {
		t.Fatalf("len(threads) = %d, want 1", len(got))
	}
	thread := got[0]
	if thread.ID != "thread-a" || thread.Name != "Custom title" || thread.Tool != "codex" || thread.Source != "/tmp/a.jsonl" {
		t.Fatalf("unexpected thread metadata: %+v", thread)
	}
	if thread.Requests != 2 || thread.Events != 2 || thread.Usage.NormalizedTotal() != 1_100_000 || thread.CostUSD != 2 {
		t.Fatalf("unexpected thread totals: %+v", thread)
	}
}

func TestThreadAccumulatorFormatsModelAndProviderListsWithinSameThread(t *testing.T) {
	loc := time.UTC
	window := Window{Start: time.Date(2026, 5, 8, 0, 0, 0, 0, loc), End: time.Date(2026, 5, 9, 0, 0, 0, 0, loc)}
	events := []usage.UsageEvent{
		{
			ID:                 "a",
			Timestamp:          time.Date(2026, 5, 8, 1, 0, 0, 0, loc),
			Tool:               usage.ToolCodex,
			Model:              "gpt-5.5",
			Provider:           "bcb",
			ThreadID:           "thread-a",
			ThreadName:         "Mixed provider thread",
			ThreadSource:       "/tmp/a.jsonl",
			ThreadCreatedAt:    time.Date(2026, 5, 8, 0, 30, 0, 0, loc),
			ThreadLastActiveAt: time.Date(2026, 5, 8, 1, 0, 0, 0, loc),
			Usage:              usage.TokenUsage{Input: 100},
		},
		{
			ID:                 "b",
			Timestamp:          time.Date(2026, 5, 8, 2, 0, 0, 0, loc),
			Tool:               usage.ToolCodex,
			Model:              "gpt-5.4",
			Provider:           "openai",
			ThreadID:           "thread-a",
			ThreadName:         "Mixed provider thread",
			ThreadSource:       "/tmp/a.jsonl",
			ThreadCreatedAt:    time.Date(2026, 5, 8, 0, 30, 0, 0, loc),
			ThreadLastActiveAt: time.Date(2026, 5, 8, 2, 0, 0, 0, loc),
			Usage:              usage.TokenUsage{Input: 10},
		},
		{
			ID:                 "c",
			Timestamp:          time.Date(2026, 5, 8, 3, 0, 0, 0, loc),
			Tool:               usage.ToolCodex,
			Model:              "gpt-5.3",
			Provider:           "openai",
			ThreadID:           "thread-a",
			ThreadName:         "Mixed provider thread",
			ThreadSource:       "/tmp/a.jsonl",
			ThreadCreatedAt:    time.Date(2026, 5, 8, 0, 30, 0, 0, loc),
			ThreadLastActiveAt: time.Date(2026, 5, 8, 3, 0, 0, 0, loc),
			Usage:              usage.TokenUsage{Input: 1},
		},
	}
	acc := NewThreadAccumulator(window, Filters{}, nil)
	for _, event := range events {
		acc.Add(event)
	}
	got := acc.Results()
	if len(got) != 1 {
		t.Fatalf("len(threads) = %d, want 1", len(got))
	}
	if got[0].ID != "thread-a" || got[0].Model != "gpt-5.3,gpt-5.4,..." || got[0].Provider != "bcb,openai" || got[0].Usage.NormalizedTotal() != 111 {
		t.Fatalf("thread row = %+v, want id=thread-a model=gpt-5.3,gpt-5.4,... provider=bcb,openai total=111", got[0])
	}
}

func TestThreadAccumulatorIncludesProviderCostBreakdown(t *testing.T) {
	loc := time.UTC
	window := Window{Start: time.Date(2026, 5, 8, 0, 0, 0, 0, loc), End: time.Date(2026, 5, 9, 0, 0, 0, 0, loc)}
	events := []usage.UsageEvent{
		{
			ID:        "a",
			Timestamp: time.Date(2026, 5, 8, 1, 0, 0, 0, loc),
			Tool:      usage.ToolCodex,
			Model:     "gpt-5.5",
			Provider:  "bcb",
			ThreadID:  "thread-a",
			Usage:     usage.TokenUsage{Input: 100},
		},
		{
			ID:        "b",
			Timestamp: time.Date(2026, 5, 8, 2, 0, 0, 0, loc),
			Tool:      usage.ToolCodex,
			Model:     "gpt-5.5",
			Provider:  "toska",
			ThreadID:  "thread-a",
			Usage:     usage.TokenUsage{Input: 10},
		},
	}
	acc := NewThreadAccumulator(window, Filters{}, func(event usage.UsageEvent) Cost {
		if event.Provider == "bcb" {
			return Cost{USD: 1}
		}
		return Cost{USD: 10}
	})
	for _, event := range events {
		acc.Add(event)
	}
	got := acc.Results()
	if len(got) != 1 {
		t.Fatalf("len(threads) = %d, want 1", len(got))
	}
	if got[0].Provider != "bcb,toska" || got[0].CostUSD != 11 {
		t.Fatalf("thread should summarize provider list and total cost: %+v", got[0])
	}
	if len(got[0].CostBreakdown) != 2 || got[0].CostBreakdown[0].Provider != "toska" || got[0].CostBreakdown[0].USD != 10 || got[0].CostBreakdown[1].Provider != "bcb" || got[0].CostBreakdown[1].USD != 1 {
		t.Fatalf("unexpected cost breakdown: %+v", got[0].CostBreakdown)
	}
}

func TestThreadAccumulatorIncludesProviderAttributionBreakdown(t *testing.T) {
	loc := time.UTC
	window := Window{Start: time.Date(2026, 5, 8, 0, 0, 0, 0, loc), End: time.Date(2026, 5, 9, 0, 0, 0, 0, loc)}
	events := []usage.UsageEvent{
		{
			ID:                  "a",
			TurnID:              "turn-a",
			Timestamp:           time.Date(2026, 5, 8, 1, 0, 0, 0, loc),
			Tool:                usage.ToolCodex,
			Model:               "gpt-5.5",
			Provider:            "toska",
			ProviderAttribution: string(usage.ProviderAttributionInferredTimeline),
			ThreadID:            "thread-a",
			Usage:               usage.TokenUsage{Input: 100},
		},
		{
			ID:                  "b",
			TurnID:              "turn-b",
			Timestamp:           time.Date(2026, 5, 8, 2, 0, 0, 0, loc),
			Tool:                usage.ToolCodex,
			Model:               "gpt-5.5",
			Provider:            "toska",
			ProviderAttribution: string(usage.ProviderAttributionExactRequest),
			ThreadID:            "thread-a",
			Usage:               usage.TokenUsage{Input: 10},
		},
		{
			ID:                  "c",
			TurnID:              "turn-c",
			Timestamp:           time.Date(2026, 5, 8, 3, 0, 0, 0, loc),
			Tool:                usage.ToolCodex,
			Model:               "gpt-5.5",
			Provider:            "bcb",
			ProviderAttribution: string(usage.ProviderAttributionModel),
			ThreadID:            "thread-a",
			Usage:               usage.TokenUsage{Input: 1},
		},
	}
	acc := NewThreadAccumulator(window, Filters{}, func(event usage.UsageEvent) Cost {
		switch event.ID {
		case "a":
			return Cost{USD: 100}
		case "b":
			return Cost{USD: 10}
		default:
			return Cost{USD: 1}
		}
	})
	for _, event := range events {
		acc.Add(event)
	}
	got := acc.Results()
	if len(got) != 1 {
		t.Fatalf("len(threads) = %d, want 1", len(got))
	}
	thread := got[0]
	if len(thread.AttributionBreakdown) != 2 {
		t.Fatalf("len(attribution_breakdown) = %d, want 2: %+v", len(thread.AttributionBreakdown), thread.AttributionBreakdown)
	}
	if thread.AttributionBreakdown[0].Provider != "toska" || len(thread.AttributionBreakdown[0].BySource) != 2 {
		t.Fatalf("unexpected toska attribution breakdown: %+v", thread.AttributionBreakdown)
	}
	if thread.AttributionBreakdown[0].BySource[0].Source != string(usage.ProviderAttributionInferredTimeline) || thread.AttributionBreakdown[0].BySource[0].USD != 100 || thread.AttributionBreakdown[0].BySource[0].Requests != 1 || thread.AttributionBreakdown[0].BySource[0].Usage.Input != 100 {
		t.Fatalf("unexpected inferred attribution item: %+v", thread.AttributionBreakdown[0].BySource[0])
	}
	if thread.AttributionBreakdown[0].BySource[1].Source != string(usage.ProviderAttributionExactRequest) || thread.AttributionBreakdown[0].BySource[1].USD != 10 {
		t.Fatalf("unexpected exact attribution item: %+v", thread.AttributionBreakdown[0].BySource[1])
	}
	if thread.AttributionBreakdown[1].Provider != "bcb" || len(thread.AttributionBreakdown[1].BySource) != 1 || thread.AttributionBreakdown[1].BySource[0].Source != string(usage.ProviderAttributionModel) || thread.AttributionBreakdown[1].BySource[0].USD != 1 {
		t.Fatalf("unexpected bcb attribution breakdown: %+v", thread.AttributionBreakdown[1])
	}
	if len(thread.Turns) != 3 {
		t.Fatalf("len(turns) = %d, want 3: %+v", len(thread.Turns), thread.Turns)
	}
	if thread.Turns[0].ID != "turn-a" || thread.Turns[0].EventID != "a" || thread.Turns[0].Provider != "toska" || thread.Turns[0].ProviderAttribution != string(usage.ProviderAttributionInferredTimeline) || thread.Turns[0].CostUSD != 100 {
		t.Fatalf("unexpected first turn breakdown: %+v", thread.Turns[0])
	}
	if thread.Turns[2].ID != "turn-c" || thread.Turns[2].Provider != "bcb" || thread.Turns[2].ProviderAttribution != string(usage.ProviderAttributionModel) || thread.Turns[2].CostUSD != 1 {
		t.Fatalf("unexpected last turn breakdown: %+v", thread.Turns[2])
	}
}

func TestThreadAccumulatorGroupResultsUseThreadBaselineForMixedProviderThread(t *testing.T) {
	loc := time.UTC
	window := Window{Start: time.Date(2026, 5, 8, 0, 0, 0, 0, loc), End: time.Date(2026, 5, 9, 0, 0, 0, 0, loc)}
	events := []usage.UsageEvent{
		{
			ID:        "a",
			Timestamp: time.Date(2026, 5, 8, 1, 0, 0, 0, loc),
			Tool:      usage.ToolCodex,
			Model:     "gpt-5.4",
			Provider:  "team-a",
			ThreadID:  "thread-a",
			Usage:     usage.TokenUsage{Input: 100},
		},
		{
			ID:        "b",
			Timestamp: time.Date(2026, 5, 8, 2, 0, 0, 0, loc),
			Tool:      usage.ToolCodex,
			Model:     "gpt-5.4",
			Provider:  "team-b",
			ThreadID:  "thread-a",
			Usage:     usage.TokenUsage{Input: 10},
		},
	}
	acc := NewThreadAccumulatorWithSort(window, Filters{}, SortByTokens, func(event usage.UsageEvent) Cost {
		if event.Provider == "team-a" {
			return Cost{USD: 1, Source: "user", InputUSDPerMTok: 1}
		}
		return Cost{USD: 2, Source: "default", InputUSDPerMTok: 2}
	})
	for _, event := range events {
		acc.Add(event)
	}
	got := acc.GroupResults(GroupBy{"tool", "model", "provider"})
	if len(got) != 2 {
		t.Fatalf("len(results) = %d, want 2: %+v", len(got), got)
	}
	byProvider := map[string]Result{}
	for _, result := range got {
		byProvider[result.Key["provider"]] = result
	}
	if byProvider["team-a"].Key["tool"] != "codex" || byProvider["team-a"].Key["model"] != "gpt-5.4" {
		t.Fatalf("team-a key mismatch: %+v", byProvider["team-a"].Key)
	}
	if byProvider["team-a"].Usage.NormalizedTotal() != 100 || byProvider["team-a"].CostUSD != 1 {
		t.Fatalf("team-a totals mismatch: %+v", byProvider["team-a"])
	}
	if byProvider["team-a"].PriceSource != "custom" || byProvider["team-a"].Price == nil || byProvider["team-a"].Price.Source != "custom" {
		t.Fatalf("team-a price mismatch: %+v", byProvider["team-a"])
	}
	if byProvider["team-b"].Key["tool"] != "codex" || byProvider["team-b"].Key["model"] != "gpt-5.4" {
		t.Fatalf("team-b key mismatch: %+v", byProvider["team-b"].Key)
	}
	if byProvider["team-b"].Usage.NormalizedTotal() != 10 || byProvider["team-b"].CostUSD != 2 {
		t.Fatalf("team-b totals mismatch: %+v", byProvider["team-b"])
	}
	if byProvider["team-b"].PriceSource != "official" || byProvider["team-b"].Price == nil || byProvider["team-b"].Price.Source != "official" {
		t.Fatalf("team-b price mismatch: %+v", byProvider["team-b"])
	}
}

func TestThreadAccumulatorGroupResultsSplitsSessionFallbackAndExactProvidersWithinThread(t *testing.T) {
	loc := time.UTC
	window := Window{Start: time.Date(2026, 5, 8, 0, 0, 0, 0, loc), End: time.Date(2026, 5, 9, 0, 0, 0, 0, loc)}
	events := []usage.UsageEvent{
		{
			ID:                  "a",
			Timestamp:           time.Date(2026, 5, 8, 1, 0, 0, 0, loc),
			Tool:                usage.ToolCodex,
			Model:               "gpt-5.5",
			Provider:            "bcb",
			ProviderAttribution: string(usage.ProviderAttributionSessionFallback),
			ThreadID:            "thread-a",
			Usage:               usage.TokenUsage{Input: 100},
		},
		{
			ID:                  "b",
			Timestamp:           time.Date(2026, 5, 8, 2, 0, 0, 0, loc),
			Tool:                usage.ToolCodex,
			Model:               "gpt-5.5",
			Provider:            "toska",
			ProviderAttribution: string(usage.ProviderAttributionExactRequest),
			ThreadID:            "thread-a",
			Usage:               usage.TokenUsage{Input: 10},
		},
	}
	acc := NewThreadAccumulator(window, Filters{}, func(event usage.UsageEvent) Cost {
		if event.ID == "a" {
			return Cost{USD: 100}
		}
		return Cost{USD: 10}
	})
	for _, event := range events {
		acc.Add(event)
	}
	got := acc.GroupResults(GroupBy{"tool", "model", "provider"})
	if len(got) != 2 {
		t.Fatalf("len(results) = %d, want 2: %+v", len(got), got)
	}
	byProvider := map[string]Result{}
	for _, result := range got {
		byProvider[result.Key["provider"]] = result
	}
	if byProvider["bcb"].CostUSD != 100 || byProvider["bcb"].Usage.NormalizedTotal() != 100 {
		t.Fatalf("bcb totals mismatch: %+v", byProvider["bcb"])
	}
	if byProvider["toska"].CostUSD != 10 || byProvider["toska"].Usage.NormalizedTotal() != 10 {
		t.Fatalf("toska totals mismatch: %+v", byProvider["toska"])
	}
}

func TestThreadAccumulatorGroupResultsRebalancesMixedProviderBridgeByTurnBuckets(t *testing.T) {
	loc := time.UTC
	window := Window{Start: time.Date(2026, 5, 8, 0, 0, 0, 0, loc), End: time.Date(2026, 5, 9, 0, 0, 0, 0, loc)}
	events := []usage.UsageEvent{
		{
			ID:                  "exact-a",
			TurnID:              "turn-a",
			Timestamp:           time.Date(2026, 5, 8, 1, 0, 0, 0, loc),
			Tool:                usage.ToolCodex,
			Model:               "gpt-5.5",
			Provider:            "toska",
			ProviderAttribution: string(usage.ProviderAttributionExactRequest),
			ThreadID:            "thread-a",
			Usage:               usage.TokenUsage{Input: 1, Total: 1},
		},
		{
			ID:                  "bridge-b1",
			TurnID:              "turn-b",
			Timestamp:           time.Date(2026, 5, 8, 1, 1, 0, 0, loc),
			Tool:                usage.ToolCodex,
			Model:               "gpt-5.5",
			Provider:            "toska",
			ProviderAttribution: string(usage.ProviderAttributionInferredTimeline),
			ThreadID:            "thread-a",
			Usage:               usage.TokenUsage{Input: 10, Total: 10},
		},
		{
			ID:                  "bridge-b2",
			TurnID:              "turn-b",
			Timestamp:           time.Date(2026, 5, 8, 1, 1, 5, 0, loc),
			Tool:                usage.ToolCodex,
			Model:               "gpt-5.5",
			Provider:            "toska",
			ProviderAttribution: string(usage.ProviderAttributionInferredTimeline),
			ThreadID:            "thread-a",
			Usage:               usage.TokenUsage{Input: 20, Total: 20},
		},
		{
			ID:                  "bridge-b3",
			TurnID:              "turn-b",
			Timestamp:           time.Date(2026, 5, 8, 1, 1, 10, 0, loc),
			Tool:                usage.ToolCodex,
			Model:               "gpt-5.5",
			Provider:            "toska",
			ProviderAttribution: string(usage.ProviderAttributionInferredTimeline),
			ThreadID:            "thread-a",
			Usage:               usage.TokenUsage{Input: 30, Total: 30},
		},
		{
			ID:                  "bridge-c1",
			TurnID:              "turn-c",
			Timestamp:           time.Date(2026, 5, 8, 1, 2, 0, 0, loc),
			Tool:                usage.ToolCodex,
			Model:               "gpt-5.5",
			Provider:            "toska",
			ProviderAttribution: string(usage.ProviderAttributionInferredTimeline),
			ThreadID:            "thread-a",
			Usage:               usage.TokenUsage{Input: 40, Total: 40},
		},
		{
			ID:                  "bridge-c2",
			TurnID:              "turn-c",
			Timestamp:           time.Date(2026, 5, 8, 1, 2, 5, 0, loc),
			Tool:                usage.ToolCodex,
			Model:               "gpt-5.5",
			Provider:            "toska",
			ProviderAttribution: string(usage.ProviderAttributionInferredTimeline),
			ThreadID:            "thread-a",
			Usage:               usage.TokenUsage{Input: 50, Total: 50},
		},
		{
			ID:                  "exact-d",
			TurnID:              "turn-d",
			Timestamp:           time.Date(2026, 5, 8, 1, 3, 0, 0, loc),
			Tool:                usage.ToolCodex,
			Model:               "gpt-5.5",
			Provider:            "bcb",
			ProviderAttribution: string(usage.ProviderAttributionExactRequest),
			ThreadID:            "thread-a",
			Usage:               usage.TokenUsage{Input: 60, Total: 60},
		},
	}
	costs := map[string]float64{
		"exact-a":   1,
		"bridge-b1": 10,
		"bridge-b2": 20,
		"bridge-b3": 30,
		"bridge-c1": 40,
		"bridge-c2": 50,
		"exact-d":   60,
	}
	acc := NewThreadAccumulator(window, Filters{}, func(event usage.UsageEvent) Cost {
		return Cost{USD: costs[event.ID]}
	})
	for _, event := range events {
		acc.Add(event)
	}
	got := acc.GroupResults(GroupBy{"tool", "model", "provider"})
	if len(got) != 2 {
		t.Fatalf("len(results) = %d, want 2: %+v", len(got), got)
	}
	byProvider := map[string]Result{}
	for _, result := range got {
		byProvider[result.Key["provider"]] = result
	}
	if byProvider["toska"].CostUSD != 19 || byProvider["toska"].Usage.NormalizedTotal() != 19 {
		t.Fatalf("toska bridge carry mismatch: %+v", byProvider["toska"])
	}
	if byProvider["bcb"].CostUSD != 192 || byProvider["bcb"].Usage.NormalizedTotal() != 192 {
		t.Fatalf("bcb bridge rebalance mismatch: %+v", byProvider["bcb"])
	}
	if byProvider["toska"].PriceSource != "" || byProvider["toska"].Price != nil {
		t.Fatalf("toska should preserve missing price details without forcing bare mixed: %+v", byProvider["toska"])
	}
	if byProvider["bcb"].PriceSource != "" || byProvider["bcb"].Price != nil {
		t.Fatalf("bcb should preserve missing price details without forcing bare mixed: %+v", byProvider["bcb"])
	}
}

func TestThreadAccumulatorGroupResultsRebalancedBridgeRepricesByFinalProvider(t *testing.T) {
	loc := time.UTC
	window := Window{Start: time.Date(2026, 5, 8, 0, 0, 0, 0, loc), End: time.Date(2026, 5, 9, 0, 0, 0, 0, loc)}
	events := []usage.UsageEvent{
		{
			ID:                  "exact-a",
			TurnID:              "turn-a",
			Timestamp:           time.Date(2026, 5, 8, 1, 0, 0, 0, loc),
			Tool:                usage.ToolCodex,
			Model:               "gpt-5.5",
			Provider:            "toska",
			ProviderAttribution: string(usage.ProviderAttributionExactRequest),
			ThreadID:            "thread-a",
			Usage:               usage.TokenUsage{Input: 1, Total: 1},
		},
		{
			ID:                  "bridge-b1",
			TurnID:              "turn-b",
			Timestamp:           time.Date(2026, 5, 8, 1, 1, 0, 0, loc),
			Tool:                usage.ToolCodex,
			Model:               "gpt-5.5",
			Provider:            "toska",
			ProviderAttribution: string(usage.ProviderAttributionInferredTimeline),
			ThreadID:            "thread-a",
			Usage:               usage.TokenUsage{Input: 10, Total: 10},
		},
		{
			ID:                  "bridge-b2",
			TurnID:              "turn-b",
			Timestamp:           time.Date(2026, 5, 8, 1, 1, 5, 0, loc),
			Tool:                usage.ToolCodex,
			Model:               "gpt-5.5",
			Provider:            "toska",
			ProviderAttribution: string(usage.ProviderAttributionInferredTimeline),
			ThreadID:            "thread-a",
			Usage:               usage.TokenUsage{Input: 20, Total: 20},
		},
		{
			ID:                  "exact-d",
			TurnID:              "turn-d",
			Timestamp:           time.Date(2026, 5, 8, 1, 2, 0, 0, loc),
			Tool:                usage.ToolCodex,
			Model:               "gpt-5.5",
			Provider:            "bcb",
			ProviderAttribution: string(usage.ProviderAttributionExactRequest),
			ThreadID:            "thread-a",
			Usage:               usage.TokenUsage{Input: 30, Total: 30},
		},
	}
	acc := NewThreadAccumulator(window, Filters{}, func(event usage.UsageEvent) Cost {
		if event.Provider == "bcb" {
			return Cost{USD: float64(event.Usage.Input) * 5, Source: "default", InputUSDPerMTok: 5, OutputUSDPerMTok: 30, CacheHitUSDPerMTok: 0.5, CacheMakeUSDPerMTok: 5}
		}
		return Cost{USD: float64(event.Usage.Input) * 2, Source: "user", InputUSDPerMTok: 2, OutputUSDPerMTok: 20, CacheHitUSDPerMTok: 0.2, CacheMakeUSDPerMTok: 2}
	})
	for _, event := range events {
		acc.Add(event)
	}
	got := acc.GroupResults(GroupBy{"tool", "model", "provider"})
	if len(got) != 2 {
		t.Fatalf("len(results) = %d, want 2: %+v", len(got), got)
	}
	byProvider := map[string]Result{}
	for _, result := range got {
		byProvider[result.Key["provider"]] = result
	}
	bcb := byProvider["bcb"]
	if bcb.PriceSource != "official" || bcb.Price == nil || bcb.Price.Source != "official" {
		t.Fatalf("bcb should be repriced with the final provider price: %+v", bcb)
	}
	if bcb.Price.InputUSDPerMTok != 5 || bcb.Price.OutputUSDPerMTok != 30 {
		t.Fatalf("bcb final provider price mismatch: %+v", bcb.Price)
	}
	if bcb.CostUSD != 210 {
		t.Fatalf("bcb cost should be recomputed after rebalance, got %.4f", bcb.CostUSD)
	}
}

func TestThreadAccumulatorGroupResultsDoNotReassignLongInferredTimelineSegment(t *testing.T) {
	loc := time.UTC
	window := Window{Start: time.Date(2026, 5, 8, 0, 0, 0, 0, loc), End: time.Date(2026, 5, 9, 0, 0, 0, 0, loc)}
	events := []usage.UsageEvent{
		{
			ID:                  "exact-a",
			TurnID:              "turn-a",
			Timestamp:           time.Date(2026, 5, 8, 1, 0, 0, 0, loc),
			Tool:                usage.ToolCodex,
			Model:               "gpt-5.5",
			Provider:            "toska",
			ProviderAttribution: string(usage.ProviderAttributionExactRequest),
			ThreadID:            "thread-a",
			Usage:               usage.TokenUsage{Input: 1, Total: 1},
		},
		{
			ID:                  "bridge-b",
			TurnID:              "turn-b",
			Timestamp:           time.Date(2026, 5, 8, 1, 1, 0, 0, loc),
			Tool:                usage.ToolCodex,
			Model:               "gpt-5.5",
			Provider:            "toska",
			ProviderAttribution: string(usage.ProviderAttributionInferredTimeline),
			ThreadID:            "thread-a",
			Usage:               usage.TokenUsage{Input: 10, Total: 10},
		},
		{
			ID:                  "bridge-c",
			TurnID:              "turn-c",
			Timestamp:           time.Date(2026, 5, 8, 1, 2, 0, 0, loc),
			Tool:                usage.ToolCodex,
			Model:               "gpt-5.5",
			Provider:            "toska",
			ProviderAttribution: string(usage.ProviderAttributionInferredTimeline),
			ThreadID:            "thread-a",
			Usage:               usage.TokenUsage{Input: 20, Total: 20},
		},
		{
			ID:                  "bridge-d",
			TurnID:              "turn-d",
			Timestamp:           time.Date(2026, 5, 8, 1, 3, 0, 0, loc),
			Tool:                usage.ToolCodex,
			Model:               "gpt-5.5",
			Provider:            "toska",
			ProviderAttribution: string(usage.ProviderAttributionInferredTimeline),
			ThreadID:            "thread-a",
			Usage:               usage.TokenUsage{Input: 30, Total: 30},
		},
		{
			ID:                  "bridge-e",
			TurnID:              "turn-e",
			Timestamp:           time.Date(2026, 5, 8, 1, 4, 0, 0, loc),
			Tool:                usage.ToolCodex,
			Model:               "gpt-5.5",
			Provider:            "toska",
			ProviderAttribution: string(usage.ProviderAttributionInferredTimeline),
			ThreadID:            "thread-a",
			Usage:               usage.TokenUsage{Input: 40, Total: 40},
		},
		{
			ID:                  "bridge-f",
			TurnID:              "turn-f",
			Timestamp:           time.Date(2026, 5, 8, 1, 5, 0, 0, loc),
			Tool:                usage.ToolCodex,
			Model:               "gpt-5.5",
			Provider:            "toska",
			ProviderAttribution: string(usage.ProviderAttributionInferredTimeline),
			ThreadID:            "thread-a",
			Usage:               usage.TokenUsage{Input: 50, Total: 50},
		},
		{
			ID:                  "bridge-g",
			TurnID:              "turn-g",
			Timestamp:           time.Date(2026, 5, 8, 1, 6, 0, 0, loc),
			Tool:                usage.ToolCodex,
			Model:               "gpt-5.5",
			Provider:            "toska",
			ProviderAttribution: string(usage.ProviderAttributionInferredTimeline),
			ThreadID:            "thread-a",
			Usage:               usage.TokenUsage{Input: 60, Total: 60},
		},
		{
			ID:                  "exact-h",
			TurnID:              "turn-h",
			Timestamp:           time.Date(2026, 5, 8, 1, 7, 0, 0, loc),
			Tool:                usage.ToolCodex,
			Model:               "gpt-5.5",
			Provider:            "bcb",
			ProviderAttribution: string(usage.ProviderAttributionExactRequest),
			ThreadID:            "thread-a",
			Usage:               usage.TokenUsage{Input: 70, Total: 70},
		},
	}
	costs := map[string]float64{
		"exact-a":  1,
		"bridge-b": 10,
		"bridge-c": 20,
		"bridge-d": 30,
		"bridge-e": 40,
		"bridge-f": 50,
		"bridge-g": 60,
		"exact-h":  70,
	}
	acc := NewThreadAccumulator(window, Filters{}, func(event usage.UsageEvent) Cost {
		return Cost{USD: costs[event.ID]}
	})
	for _, event := range events {
		acc.Add(event)
	}
	got := acc.GroupResults(GroupBy{"tool", "model", "provider"})
	if len(got) != 2 {
		t.Fatalf("len(results) = %d, want 2: %+v", len(got), got)
	}
	byProvider := map[string]Result{}
	for _, result := range got {
		byProvider[result.Key["provider"]] = result
	}
	if byProvider["toska"].CostUSD != 211 || byProvider["toska"].Usage.NormalizedTotal() != 211 {
		t.Fatalf("long inferred timeline segment should remain on original provider: %+v", byProvider["toska"])
	}
	if byProvider["bcb"].CostUSD != 70 || byProvider["bcb"].Usage.NormalizedTotal() != 70 {
		t.Fatalf("bcb exact totals mismatch: %+v", byProvider["bcb"])
	}
}
