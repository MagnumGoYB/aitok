package query

import (
	"fmt"
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

type SortMetric string

const (
	SortByTokens SortMetric = "tokens"
	SortByCost   SortMetric = "cost"
)

type Cost struct {
	USD                   float64 `json:"usd"`
	Source                string  `json:"source,omitempty"`
	InputUSDPerMTok       float64 `json:"input_usd_per_mtok,omitempty"`
	OutputUSDPerMTok      float64 `json:"output_usd_per_mtok,omitempty"`
	CacheHitUSDPerMTok    float64 `json:"cache_hit_usd_per_mtok,omitempty"`
	CacheMakeUSDPerMTok   float64 `json:"cache_make_usd_per_mtok,omitempty"`
	CacheMake1hUSDPerMTok float64 `json:"cache_make_1h_usd_per_mtok,omitempty"`
}

type Price struct {
	Source                string  `json:"source"`
	InputUSDPerMTok       float64 `json:"input_usd_per_mtok,omitempty"`
	OutputUSDPerMTok      float64 `json:"output_usd_per_mtok,omitempty"`
	CacheHitUSDPerMTok    float64 `json:"cache_hit_usd_per_mtok,omitempty"`
	CacheMakeUSDPerMTok   float64 `json:"cache_make_usd_per_mtok,omitempty"`
	CacheMake1hUSDPerMTok float64 `json:"cache_make_1h_usd_per_mtok,omitempty"`
}

type Result struct {
	Key         map[string]string `json:"key"`
	Events      int               `json:"events"`
	Requests    int               `json:"requests"`
	CostUSD     float64           `json:"cost_usd"`
	PriceSource string            `json:"price_source,omitempty"`
	Price       *Price            `json:"price,omitempty"`
	Usage       usage.TokenUsage  `json:"usage"`
	Examples    map[string]string `json:"examples,omitempty"`
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
	PriceSource  string           `json:"price_source,omitempty"`
	Price        *Price           `json:"price,omitempty"`
	Usage        usage.TokenUsage `json:"usage"`
}

type Accumulator struct {
	window  Window
	filters Filters
	groupBy GroupBy
	sortBy  SortMetric
	costFor func(usage.UsageEvent) Cost
	buckets map[string]*Result
}

type ThreadAccumulator struct {
	window  Window
	filters Filters
	sortBy  SortMetric
	costFor func(usage.UsageEvent) Cost
	buckets map[string]*threadBucket
}

type threadBucket struct {
	result       ThreadResult
	models       map[string]struct{}
	providers    map[string]struct{}
	priceSources map[string]struct{}
	price        *Price
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

func ParseSortMetric(raw string) (SortMetric, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", string(SortByTokens), "token":
		return SortByTokens, nil
	case string(SortByCost), "cost_usd":
		return SortByCost, nil
	default:
		return "", fmt.Errorf("unknown sort metric %q; expected tokens or cost", raw)
	}
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
	return NewAccumulatorWithSort(window, filters, groupBy, SortByTokens, costFor)
}

func NewThreadAccumulator(window Window, filters Filters, costFor func(usage.UsageEvent) Cost) *ThreadAccumulator {
	return NewThreadAccumulatorWithSort(window, filters, SortByTokens, costFor)
}

func NewAccumulatorWithSort(window Window, filters Filters, groupBy GroupBy, sortBy SortMetric, costFor func(usage.UsageEvent) Cost) *Accumulator {
	return &Accumulator{window: window, filters: filters, groupBy: groupBy, sortBy: normalizeSortMetric(sortBy), costFor: costFor, buckets: map[string]*Result{}}
}

func NewThreadAccumulatorWithSort(window Window, filters Filters, sortBy SortMetric, costFor func(usage.UsageEvent) Cost) *ThreadAccumulator {
	return &ThreadAccumulator{window: window, filters: filters, sortBy: normalizeSortMetric(sortBy), costFor: costFor, buckets: map[string]*threadBucket{}}
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
		cost := a.costFor(event)
		a.buckets[bucketKey].CostUSD += cost.USD
		a.buckets[bucketKey].PriceSource = mergePriceSource(a.buckets[bucketKey].PriceSource, cost.Source)
		a.buckets[bucketKey].Price = mergePrice(a.buckets[bucketKey].Price, cost)
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
		if compareResults(results[i], results[j], a.sortBy) != 0 {
			return compareResults(results[i], results[j], a.sortBy) < 0
		}
		return serializeKey(a.groupBy, results[i].Key) < serializeKey(a.groupBy, results[j].Key)
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
			models:       map[string]struct{}{},
			providers:    map[string]struct{}{},
			priceSources: map[string]struct{}{},
		}
		a.buckets[string(event.Tool)+"|"+id] = bucket
	}
	bucket.result.Events++
	bucket.result.Requests++
	bucket.result.Usage = bucket.result.Usage.Add(event.Usage)
	if a.costFor != nil {
		cost := a.costFor(event)
		bucket.result.CostUSD += cost.USD
		if source := priceSourceLabel(cost.Source); source != "" {
			bucket.priceSources[source] = struct{}{}
		}
		bucket.price = mergePrice(bucket.price, cost)
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
		thread.PriceSource = summarizeValues(result.priceSources, "")
		thread.Price = result.price
		results = append(results, thread)
	}
	sort.Slice(results, func(i, j int) bool {
		if compareThreads(results[i], results[j], a.sortBy) != 0 {
			return compareThreads(results[i], results[j], a.sortBy) < 0
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

func mergePriceSource(existing, next string) string {
	next = priceSourceLabel(next)
	if next == "" {
		return existing
	}
	if existing == "" || existing == next {
		return next
	}
	return "mixed"
}

func priceSourceLabel(source string) string {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case "user", "configured":
		return "custom"
	case "default":
		return "official"
	case "unknown":
		return "unpriced"
	case "mixed":
		return "mixed"
	default:
		return ""
	}
}

func mergePrice(existing *Price, cost Cost) *Price {
	next := priceFromCost(cost)
	if next == nil {
		return existing
	}
	if existing == nil {
		return next
	}
	if pricesEqual(*existing, *next) {
		return existing
	}
	return &Price{Source: "mixed"}
}

func priceFromCost(cost Cost) *Price {
	source := priceSourceLabel(cost.Source)
	if source == "" {
		return nil
	}
	return &Price{
		Source:                source,
		InputUSDPerMTok:       cost.InputUSDPerMTok,
		OutputUSDPerMTok:      cost.OutputUSDPerMTok,
		CacheHitUSDPerMTok:    cost.CacheHitUSDPerMTok,
		CacheMakeUSDPerMTok:   cost.CacheMakeUSDPerMTok,
		CacheMake1hUSDPerMTok: cost.CacheMake1hUSDPerMTok,
	}
}

func pricesEqual(left, right Price) bool {
	return left.Source == right.Source &&
		left.InputUSDPerMTok == right.InputUSDPerMTok &&
		left.OutputUSDPerMTok == right.OutputUSDPerMTok &&
		left.CacheHitUSDPerMTok == right.CacheHitUSDPerMTok &&
		left.CacheMakeUSDPerMTok == right.CacheMakeUSDPerMTok &&
		left.CacheMake1hUSDPerMTok == right.CacheMake1hUSDPerMTok
}

func normalizeSortMetric(sortBy SortMetric) SortMetric {
	switch sortBy {
	case SortByCost:
		return SortByCost
	default:
		return SortByTokens
	}
}

func compareResults(left, right Result, sortBy SortMetric) int {
	switch normalizeSortMetric(sortBy) {
	case SortByCost:
		if left.CostUSD != right.CostUSD {
			if left.CostUSD > right.CostUSD {
				return -1
			}
			return 1
		}
		return compareInt64Desc(left.Usage.NormalizedTotal(), right.Usage.NormalizedTotal())
	default:
		if cmp := compareInt64Desc(left.Usage.NormalizedTotal(), right.Usage.NormalizedTotal()); cmp != 0 {
			return cmp
		}
		if left.CostUSD != right.CostUSD {
			if left.CostUSD > right.CostUSD {
				return -1
			}
			return 1
		}
		return 0
	}
}

func compareThreads(left, right ThreadResult, sortBy SortMetric) int {
	switch normalizeSortMetric(sortBy) {
	case SortByCost:
		if left.CostUSD != right.CostUSD {
			if left.CostUSD > right.CostUSD {
				return -1
			}
			return 1
		}
		return compareInt64Desc(left.Usage.NormalizedTotal(), right.Usage.NormalizedTotal())
	default:
		if cmp := compareInt64Desc(left.Usage.NormalizedTotal(), right.Usage.NormalizedTotal()); cmp != 0 {
			return cmp
		}
		if left.CostUSD != right.CostUSD {
			if left.CostUSD > right.CostUSD {
				return -1
			}
			return 1
		}
		return 0
	}
}

func compareInt64Desc(left, right int64) int {
	if left > right {
		return -1
	}
	if left < right {
		return 1
	}
	return 0
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
