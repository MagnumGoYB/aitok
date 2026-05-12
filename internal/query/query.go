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

type ThreadResult struct {
	ID           string           `json:"id"`
	Name         string           `json:"name"`
	Tool         string           `json:"tool"`
	Model        string           `json:"model"`
	Provider     string           `json:"provider"`
	Source       string           `json:"source,omitempty"`
	CreatedAt    time.Time        `json:"created_at,omitempty"`
	LastActiveAt time.Time        `json:"last_active_at,omitempty"`
	Events       int              `json:"events"`
	Requests     int              `json:"requests"`
	CostUSD      float64          `json:"cost_usd"`
	Usage        usage.TokenUsage `json:"usage"`
}

type Accumulator struct {
	window  Window
	filters Filters
	groupBy GroupBy
	costFor func(usage.UsageEvent) Cost
	buckets map[string]*Result
}

type ThreadAccumulator struct {
	window  Window
	filters Filters
	costFor func(usage.UsageEvent) Cost
	buckets map[string]*threadBucket
}

type threadBucket struct {
	result    ThreadResult
	models    map[string]struct{}
	providers map[string]struct{}
}

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

func Aggregate(events []usage.UsageEvent, window Window, filters Filters, groupBy GroupBy) []Result {
	return AggregateWithCosts(events, window, filters, groupBy, nil)
}

func AggregateWithCosts(events []usage.UsageEvent, window Window, filters Filters, groupBy GroupBy, costFor func(usage.UsageEvent) Cost) []Result {
	acc := NewAccumulator(window, filters, groupBy, costFor)
	for _, event := range events {
		acc.Add(event)
	}
	return acc.Results()
}

func NewAccumulator(window Window, filters Filters, groupBy GroupBy, costFor func(usage.UsageEvent) Cost) *Accumulator {
	return &Accumulator{window: window, filters: filters, groupBy: groupBy, costFor: costFor, buckets: map[string]*Result{}}
}

func NewThreadAccumulator(window Window, filters Filters, costFor func(usage.UsageEvent) Cost) *ThreadAccumulator {
	return &ThreadAccumulator{window: window, filters: filters, costFor: costFor, buckets: map[string]*threadBucket{}}
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

func (a *ThreadAccumulator) Add(event usage.UsageEvent) {
	if !a.window.Contains(event.Timestamp) || !matches(event, a.filters) {
		return
	}
	id := usage.Unknown(event.ThreadID)
	if id == "unknown" {
		id = event.ID
	}
	if id == "" {
		return
	}
	model := usage.Unknown(event.Model)
	provider := usage.Unknown(event.Provider)
	bucket := a.buckets[string(event.Tool)+"|"+id]
	if bucket == nil {
		bucket = &threadBucket{
			result: ThreadResult{
				ID:           id,
				Name:         usage.Unknown(event.ThreadName),
				Tool:         string(event.Tool),
				Model:        model,
				Provider:     provider,
				Source:       event.ThreadSource,
				CreatedAt:    event.ThreadCreatedAt,
				LastActiveAt: event.ThreadLastActiveAt,
			},
			models:    map[string]struct{}{},
			providers: map[string]struct{}{},
		}
		a.buckets[string(event.Tool)+"|"+id] = bucket
	}
	bucket.result.Events++
	bucket.result.Requests++
	bucket.result.Usage = bucket.result.Usage.Add(event.Usage)
	if a.costFor != nil {
		bucket.result.CostUSD += a.costFor(event).USD
	}
	if bucket.result.Name == "unknown" && event.ThreadName != "" {
		bucket.result.Name = event.ThreadName
	}
	if model != "unknown" {
		bucket.models[model] = struct{}{}
	}
	if provider != "unknown" {
		bucket.providers[provider] = struct{}{}
	}
	if bucket.result.Source == "" {
		bucket.result.Source = event.ThreadSource
	}
	if bucket.result.CreatedAt.IsZero() {
		bucket.result.CreatedAt = event.ThreadCreatedAt
	}
	if bucket.result.LastActiveAt.IsZero() || event.Timestamp.After(bucket.result.LastActiveAt) {
		bucket.result.LastActiveAt = event.Timestamp
	}
}

func (a *ThreadAccumulator) Results() []ThreadResult {
	results := make([]ThreadResult, 0, len(a.buckets))
	for _, result := range a.buckets {
		thread := result.result
		thread.Model = summarizeValues(result.models, "unknown")
		thread.Provider = summarizeProvider(result.providers, thread.Provider)
		results = append(results, thread)
	}
	sort.Slice(results, func(i, j int) bool {
		leftTokens := results[i].Usage.NormalizedTotal()
		rightTokens := results[j].Usage.NormalizedTotal()
		if leftTokens != rightTokens {
			return leftTokens > rightTokens
		}
		if results[i].CostUSD != results[j].CostUSD {
			return results[i].CostUSD > results[j].CostUSD
		}
		leftCreated := results[i].CreatedAt
		rightCreated := results[j].CreatedAt
		if leftCreated.IsZero() {
			leftCreated = results[i].LastActiveAt
		}
		if rightCreated.IsZero() {
			rightCreated = results[j].LastActiveAt
		}
		if !leftCreated.Equal(rightCreated) {
			return leftCreated.After(rightCreated)
		}
		return results[i].Tool+"|"+results[i].ID < results[j].Tool+"|"+results[j].ID
	})
	return results
}

func summarizeValues(values map[string]struct{}, fallback string) string {
	if len(values) == 0 {
		return fallback
	}
	items := make([]string, 0, len(values))
	for value := range values {
		items = append(items, value)
	}
	sort.Strings(items)
	if len(items) <= 2 {
		return strings.Join(items, ",")
	}
	return strings.Join(items[:2], ",") + ",..."
}

func summarizeProvider(values map[string]struct{}, fallback string) string {
	if len(values) == 0 {
		return fallback
	}
	if len(values) == 1 {
		for value := range values {
			return value
		}
	}
	return "mixed"
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
