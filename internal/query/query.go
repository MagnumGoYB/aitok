package query

import (
	"sort"
	"strings"
	"time"

	"github.com/MagnumGoYB/aitok/internal/usage"
)

type Filters struct {
	Tools     []string
	Models    []string
	Providers []string
	CWD       string
}

type GroupBy []string

type Cost struct {
	USD float64 `json:"usd"`
}

type Result struct {
	Key      map[string]string `json:"key"`
	Events   int               `json:"events"`
	Requests int               `json:"requests"`
	CostUSD  float64           `json:"cost_usd"`
	Usage    usage.TokenUsage  `json:"usage"`
	Examples map[string]string `json:"examples,omitempty"`
}

type Accumulator struct {
	window  Window
	filters Filters
	groupBy GroupBy
	costFor func(usage.UsageEvent) Cost
	buckets map[string]*Result
}

// DefaultGroupBy returns the default grouping dimensions used for aggregation.
// The default order is "tool", "model", then "provider".
func DefaultGroupBy() GroupBy {
	return GroupBy{"tool", "model", "provider"}
}

func ParseGroupBy(raw string) GroupBy {
	if strings.TrimSpace(raw) == "" {
		return DefaultGroupBy()
	}
	var groups []string
	for _, item := range strings.Split(raw, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			groups = append(groups, item)
		}
	}
	if len(groups) == 0 {
		return DefaultGroupBy()
	}
	return groups
}

// Aggregate aggregates usage events into grouped Results according to the provided window, filters, and groupBy.
// Per-event cost computation is not applied (no cost callback).
func Aggregate(events []usage.UsageEvent, window Window, filters Filters, groupBy GroupBy) []Result {
	return AggregateWithCosts(events, window, filters, groupBy, nil)
}

// AggregateWithCosts aggregates usage events into bucketed Results according to the
// provided window, filters, and groupBy, applying the optional costFor callback to
// accumulate per-event cost when non-nil.
// It returns a slice of Result values sorted by descending Usage.NormalizedTotal(),
// with ties broken by the serialized group key.
func AggregateWithCosts(events []usage.UsageEvent, window Window, filters Filters, groupBy GroupBy, costFor func(usage.UsageEvent) Cost) []Result {
	acc := NewAccumulator(window, filters, groupBy, costFor)
	for _, event := range events {
		acc.Add(event)
	}
	return acc.Results()
}

// NewAccumulator constructs an Accumulator configured with the provided window, filters, groupBy, and costFor, and initializes an empty buckets map.
// If costFor is nil, cost accumulation is disabled.
func NewAccumulator(window Window, filters Filters, groupBy GroupBy, costFor func(usage.UsageEvent) Cost) *Accumulator {
	return &Accumulator{window: window, filters: filters, groupBy: groupBy, costFor: costFor, buckets: map[string]*Result{}}
}

func (a *Accumulator) Add(event usage.UsageEvent) {
	if !a.window.Contains(event.Timestamp) || !matches(event, a.filters) {
		return
	}
	key := keyFor(event, a.groupBy, a.window.Start.Location())
	bucketKey := serializeKey(a.groupBy, key)
	if a.buckets[bucketKey] == nil {
		a.buckets[bucketKey] = &Result{Key: key, Examples: map[string]string{}}
	}
	a.buckets[bucketKey].Events++
	a.buckets[bucketKey].Requests++
	a.buckets[bucketKey].Usage = a.buckets[bucketKey].Usage.Add(event.Usage)
	if a.costFor != nil {
		a.buckets[bucketKey].CostUSD += a.costFor(event).USD
	}
	if event.CWD != "" && a.buckets[bucketKey].Examples["cwd"] == "" {
		a.buckets[bucketKey].Examples["cwd"] = event.CWD
	}
}

func (a *Accumulator) Results() []Result {
	results := make([]Result, 0, len(a.buckets))
	for _, result := range a.buckets {
		if len(result.Examples) == 0 {
			result.Examples = nil
		}
		results = append(results, *result)
	}
	sort.Slice(results, func(i, j int) bool {
		left := results[i].Usage.NormalizedTotal()
		right := results[j].Usage.NormalizedTotal()
		if left == right {
			return serializeKey(a.groupBy, results[i].Key) < serializeKey(a.groupBy, results[j].Key)
		}
		return left > right
	})
	return results
}

func matches(event usage.UsageEvent, filters Filters) bool {
	if len(filters.Tools) > 0 && !contains(filters.Tools, string(event.Tool)) {
		return false
	}
	if len(filters.Models) > 0 && !contains(filters.Models, event.Model) {
		return false
	}
	if len(filters.Providers) > 0 && !contains(filters.Providers, event.Provider) {
		return false
	}
	if filters.CWD != "" && !strings.Contains(event.CWD, filters.CWD) {
		return false
	}
	return true
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), target) {
			return true
		}
	}
	return false
}

func keyFor(event usage.UsageEvent, groupBy GroupBy, loc *time.Location) map[string]string {
	key := map[string]string{}
	for _, group := range groupBy {
		switch group {
		case "tool":
			key[group] = string(event.Tool)
		case "model":
			key[group] = usage.Unknown(event.Model)
		case "provider":
			key[group] = usage.Unknown(event.Provider)
		case "day":
			key[group] = event.Timestamp.In(loc).Format("2006-01-02")
		case "cwd":
			key[group] = usage.Unknown(event.CWD)
		default:
			key[group] = ""
		}
	}
	return key
}

func serializeKey(groupBy GroupBy, key map[string]string) string {
	var parts []string
	for _, group := range groupBy {
		parts = append(parts, group+"="+key[group])
	}
	return strings.Join(parts, "|")
}

func SplitCSV(values []string) []string {
	var out []string
	for _, value := range values {
		for _, item := range strings.Split(value, ",") {
			item = strings.TrimSpace(item)
			if item != "" {
				out = append(out, item)
			}
		}
	}
	return out
}
