package query

import (
	"fmt"
	"testing"
	"time"

	"github.com/MagnumGoYB/aitok/internal/usage"
)

func BenchmarkAccumulatorAdd(b *testing.B) {
	loc := time.UTC
	window := Window{Start: time.Date(2026, 5, 8, 0, 0, 0, 0, loc), End: time.Date(2026, 5, 9, 0, 0, 0, 0, loc)}
	events := make([]usage.UsageEvent, 4096)
	for i := range events {
		events[i] = usage.UsageEvent{
			Timestamp: time.Date(2026, 5, 8, i%24, 0, 0, 0, loc),
			Tool:      usage.ToolCodex,
			Model:     fmt.Sprintf("gpt-5.%d", i%8),
			Provider:  "openai",
			CWD:       fmt.Sprintf("/repo/%d", i%16),
			Usage:     usage.TokenUsage{Input: int64(100 + i%50), Output: int64(10 + i%10)},
		}
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		acc := NewAccumulator(window, Filters{}, GroupBy{"tool", "model", "provider", "cwd"}, nil)
		for _, event := range events {
			acc.Add(event)
		}
		_ = acc.Results()
	}
}

func BenchmarkThreadAccumulatorAddAndGroupResults(b *testing.B) {
	loc := time.UTC
	window := Window{Start: time.Date(2026, 5, 8, 0, 0, 0, 0, loc), End: time.Date(2026, 5, 9, 0, 0, 0, 0, loc)}
	events := make([]usage.UsageEvent, 4096)
	for i := range events {
		threadID := fmt.Sprintf("thread-%d", i%256)
		provider := "openai"
		if i%5 == 0 {
			provider = "bcb"
		}
		events[i] = usage.UsageEvent{
			ID:        fmt.Sprintf("event-%d", i),
			TurnID:    fmt.Sprintf("turn-%d", i),
			Timestamp: time.Date(2026, 5, 8, i%24, 0, 0, 0, loc),
			Tool:      usage.ToolCodex,
			Model:     fmt.Sprintf("gpt-5.%d", i%8),
			Provider:  provider,
			ThreadID:  threadID,
			Usage:     usage.TokenUsage{Input: int64(100 + i%50), Output: int64(10 + i%10), Total: int64(110 + i%60)},
		}
	}
	costFor := func(event usage.UsageEvent) Cost {
		return Cost{USD: float64(event.Usage.Input) / 1_000_000, Source: "default", InputUSDPerMTok: 1}
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		acc := NewThreadAccumulator(window, Filters{}, costFor)
		for _, event := range events {
			acc.Add(event)
		}
		_ = acc.GroupResults(GroupBy{"tool", "model", "provider"})
	}
}

func BenchmarkAccumulatorAddWithFilters(b *testing.B) {
	loc := time.UTC
	window := Window{Start: time.Date(2026, 5, 8, 0, 0, 0, 0, loc), End: time.Date(2026, 5, 9, 0, 0, 0, 0, loc)}
	events := make([]usage.UsageEvent, 4096)
	for i := range events {
		tool := usage.ToolCodex
		if i%3 == 0 {
			tool = usage.ToolClaude
		}
		provider := "openai"
		if i%4 == 0 {
			provider = "team-a"
		}
		events[i] = usage.UsageEvent{
			Timestamp: time.Date(2026, 5, 8, i%24, 0, 0, 0, loc),
			Tool:      tool,
			Model:     fmt.Sprintf("gpt-5.%d", i%8),
			Provider:  provider,
			CWD:       fmt.Sprintf("/repo/%d/service", i%16),
			Usage:     usage.TokenUsage{Input: int64(100 + i%50), Output: int64(10 + i%10)},
		}
	}
	filters := Filters{Tools: []string{"codex"}, Models: []string{"gpt-5.4", "gpt-5.5"}, Providers: []string{"openai", "team-a"}, CWD: "/repo/1"}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		acc := NewAccumulator(window, filters, GroupBy{"tool", "model", "provider", "cwd"}, nil)
		for _, event := range events {
			acc.Add(event)
		}
		_ = acc.Results()
	}
}
