package tui

import (
	"sort"
	"strings"

	"github.com/MagnumGoYB/aitok/internal/query"
)

type totals struct {
	requests int
	cost     float64
	total    int64
	cached   int64
}

type viewContext struct {
	activeTool   string
	sortBy       query.SortMetric
	search       string
	shownResults int
	totalResults int
	shownThreads int
	totalThreads int
}

func (m model) currentViewContext() viewContext {
	return viewContext{
		activeTool:   m.activeTool,
		sortBy:       normalizePayloadSort(m.sortBy),
		search:       strings.TrimSpace(m.search),
		shownResults: len(m.filteredResults()),
		totalResults: len(m.payload.Results),
		shownThreads: len(m.filteredThreads()),
		totalThreads: len(m.payload.Threads),
	}
}

func (m model) filteredResults() []query.Result {
	var out []query.Result
	needle := strings.ToLower(strings.TrimSpace(m.search))
	for _, result := range m.payload.Results {
		tool := strings.ToLower(result.Key["tool"])
		if m.activeTool != allTools && tool != m.activeTool {
			continue
		}
		label := strings.ToLower(resultLabel(result))
		if needle != "" && !strings.Contains(label, needle) {
			continue
		}
		out = append(out, result)
	}
	sort.Slice(out, func(i, j int) bool {
		return compareResults(out[i], out[j], m.sortBy)
	})
	return out
}

func summarize(results []query.Result) totals {
	var out totals
	for _, result := range results {
		out.requests += result.Requests
		out.cost += result.CostUSD
		out.total += result.Usage.NormalizedTotal()
		out.cached += result.Usage.CachedInput + result.Usage.CacheCreation
	}
	return out
}

func (m model) filteredThreads() []query.ThreadResult {
	var out []query.ThreadResult
	needle := strings.ToLower(strings.TrimSpace(m.search))
	for _, thread := range m.payload.Threads {
		tool := strings.ToLower(thread.Tool)
		if m.activeTool != allTools && tool != m.activeTool {
			continue
		}
		if needle != "" && !strings.Contains(threadSearchText(thread), needle) {
			continue
		}
		out = append(out, thread)
	}
	sort.Slice(out, func(i, j int) bool {
		if compareThreads(out[i], out[j], m.sortBy) {
			return true
		}
		if compareThreads(out[j], out[i], m.sortBy) {
			return false
		}
		if !out[i].LastActiveAt.Equal(out[j].LastActiveAt) {
			return out[i].LastActiveAt.After(out[j].LastActiveAt)
		}
		return out[i].Tool+"|"+out[i].ID < out[j].Tool+"|"+out[j].ID
	})
	return out
}

func threadSearchText(thread query.ThreadResult) string {
	return strings.ToLower(strings.Join([]string{
		thread.ID,
		thread.Name,
		thread.Tool,
		thread.Model,
		thread.Provider,
		thread.Source,
	}, " "))
}

func compareResults(left, right query.Result, sortBy query.SortMetric) bool {
	switch normalizePayloadSort(sortBy) {
	case query.SortByCost:
		if left.CostUSD != right.CostUSD {
			return left.CostUSD > right.CostUSD
		}
	default:
		leftTokens := left.Usage.NormalizedTotal()
		rightTokens := right.Usage.NormalizedTotal()
		if leftTokens != rightTokens {
			return leftTokens > rightTokens
		}
	}
	if normalizePayloadSort(sortBy) == query.SortByCost {
		leftTokens := left.Usage.NormalizedTotal()
		rightTokens := right.Usage.NormalizedTotal()
		if leftTokens != rightTokens {
			return leftTokens > rightTokens
		}
	} else if left.CostUSD != right.CostUSD {
		return left.CostUSD > right.CostUSD
	}
	return formatKey(left.Key) < formatKey(right.Key)
}

func compareThreads(left, right query.ThreadResult, sortBy query.SortMetric) bool {
	switch normalizePayloadSort(sortBy) {
	case query.SortByCost:
		if left.CostUSD != right.CostUSD {
			return left.CostUSD > right.CostUSD
		}
	default:
		leftTokens := left.Usage.NormalizedTotal()
		rightTokens := right.Usage.NormalizedTotal()
		if leftTokens != rightTokens {
			return leftTokens > rightTokens
		}
	}
	if normalizePayloadSort(sortBy) == query.SortByCost {
		leftTokens := left.Usage.NormalizedTotal()
		rightTokens := right.Usage.NormalizedTotal()
		if leftTokens != rightTokens {
			return leftTokens > rightTokens
		}
	} else if left.CostUSD != right.CostUSD {
		return left.CostUSD > right.CostUSD
	}
	return false
}
