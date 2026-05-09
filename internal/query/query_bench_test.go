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
