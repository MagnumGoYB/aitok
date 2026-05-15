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
	Components            []Price `json:"components,omitempty"`
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
	ID                   string              `json:"id"`
	Name                 string              `json:"name"`
	Tool                 string              `json:"tool"`
	Model                string              `json:"model"`
	Provider             string              `json:"provider"`
	Source               string              `json:"source,omitempty"`
	CreatedAt            time.Time           `json:"created_at,omitempty"`
	LastActiveAt         time.Time           `json:"last_active_at,omitempty"`
	Events               int                 `json:"events"`
	Requests             int                 `json:"requests"`
	CostUSD              float64             `json:"cost_usd"`
	CostBreakdown        []ThreadCost        `json:"cost_breakdown,omitempty"`
	AttributionBreakdown []ThreadAttribution `json:"attribution_breakdown,omitempty"`
	Turns                []ThreadTurn        `json:"turns,omitempty"`
	PriceSource          string              `json:"price_source,omitempty"`
	Price                *Price              `json:"price,omitempty"`
	Usage                usage.TokenUsage    `json:"usage"`
}

type ThreadCost struct {
	Provider string  `json:"provider"`
	USD      float64 `json:"usd"`
}

type ThreadAttribution struct {
	Provider string            `json:"provider"`
	BySource []AttributionCost `json:"by_source,omitempty"`
}

type AttributionCost struct {
	Source   string           `json:"source"`
	Requests int              `json:"requests"`
	USD      float64          `json:"usd"`
	Usage    usage.TokenUsage `json:"usage"`
}

type ThreadTurn struct {
	ID                  string           `json:"id"`
	EventID             string           `json:"event_id,omitempty"`
	Timestamp           time.Time        `json:"timestamp"`
	Model               string           `json:"model"`
	Provider            string           `json:"provider"`
	ProviderAttribution string           `json:"provider_attribution,omitempty"`
	CostUSD             float64          `json:"cost_usd"`
	Usage               usage.TokenUsage `json:"usage"`
	TurnEventIndex      int              `json:"-"`
	PriceSource         string           `json:"-"`
	Price               *Price           `json:"-"`
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
	result                ThreadResult
	models                map[string]struct{}
	providers             map[string]struct{}
	costByProvider        map[string]float64
	attributionByProvider map[string]map[string]*AttributionCost
	priceSources          map[string]struct{}
	price                 *Price
	groupBuckets          map[string]*Result
	turnEventCounts       map[string]int
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

func SupportsThreadBaseline(groupBy GroupBy) bool {
	for _, group := range groupBy {
		switch group {
		case "tool", "model", "provider":
			continue
		default:
			return false
		}
	}
	return true
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
			models:                map[string]struct{}{},
			providers:             map[string]struct{}{},
			costByProvider:        map[string]float64{},
			attributionByProvider: map[string]map[string]*AttributionCost{},
			priceSources:          map[string]struct{}{},
			groupBuckets:          map[string]*Result{},
			turnEventCounts:       map[string]int{},
		}
		a.buckets[string(event.Tool)+"|"+id] = bucket
	}
	bucket.result.Events++
	bucket.result.Requests++
	bucket.result.Usage = bucket.result.Usage.Add(event.Usage)
	if a.costFor != nil {
		cost := a.costFor(event)
		bucket.result.CostUSD += cost.USD
		bucket.costByProvider[provider] += cost.USD
		recordThreadAttribution(bucket, provider, event.ProviderAttribution, event.Usage, cost.USD)
		recordThreadTurn(bucket, event, cost.USD, mergePriceSource("", cost.Source), priceFromCost(cost))
		if source := priceSourceLabel(cost.Source); source != "" {
			bucket.priceSources[source] = struct{}{}
		}
		bucket.price = mergePrice(bucket.price, cost)
	} else {
		recordThreadAttribution(bucket, provider, event.ProviderAttribution, event.Usage, 0)
		recordThreadTurn(bucket, event, 0, "", nil)
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
	groupKey := keyFor(event, DefaultGroupBy(), time.UTC)
	groupBucketID := serializeKey(DefaultGroupBy(), groupKey)
	if bucket.groupBuckets[groupBucketID] == nil {
		bucket.groupBuckets[groupBucketID] = &Result{Key: groupKey}
	}
	groupBucket := bucket.groupBuckets[groupBucketID]
	groupBucket.Events++
	groupBucket.Requests++
	groupBucket.Usage = groupBucket.Usage.Add(event.Usage)
	if a.costFor != nil {
		cost := a.costFor(event)
		groupBucket.CostUSD += cost.USD
		groupBucket.PriceSource = mergePriceSource(groupBucket.PriceSource, cost.Source)
		groupBucket.Price = mergePrice(groupBucket.Price, cost)
	}
}

func (a *ThreadAccumulator) Results() []ThreadResult {
	results := make([]ThreadResult, 0, len(a.buckets))
	for _, result := range a.buckets {
		thread := result.result
		thread.Model = summarizeValues(result.models, "unknown")
		thread.Provider = summarizeValues(result.providers, thread.Provider)
		thread.CostBreakdown = summarizeThreadCosts(result.costByProvider)
		thread.AttributionBreakdown = summarizeThreadAttributions(result.attributionByProvider)
		thread.PriceSource = summarizePriceSources(result.priceSources)
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

func (a *ThreadAccumulator) GroupResults(groupBy GroupBy) []Result {
	buckets := map[string]*Result{}
	for _, thread := range a.buckets {
		threadBuckets := a.groupBucketsForThread(thread, groupBy)
		for _, groupBucket := range threadBuckets {
			key := regroupThreadBucketKey(groupBucket.Key, groupBy)
			mergeGroupedResult(buckets, groupBy, key, *groupBucket)
		}
	}
	results := make([]Result, 0, len(buckets))
	for _, bucket := range buckets {
		results = append(results, *bucket)
	}
	sort.Slice(results, func(i, j int) bool {
		if compareResults(results[i], results[j], a.sortBy) != 0 {
			return compareResults(results[i], results[j], a.sortBy) < 0
		}
		return serializeKey(groupBy, results[i].Key) < serializeKey(groupBy, results[j].Key)
	})
	return results
}

func (a *ThreadAccumulator) groupBucketsForThread(thread *threadBucket, groupBy GroupBy) map[string]*Result {
	if thread == nil {
		return nil
	}
	if !supportsProviderRebalance(groupBy) {
		return thread.groupBuckets
	}
	rebalanced, changed := rebalanceMixedProviderThread(thread.result.Tool, thread.result.Turns, a.costFor)
	if !changed || len(rebalanced) == 0 {
		return thread.groupBuckets
	}
	grouped := map[string]*Result{}
	for _, item := range rebalanced {
		key := map[string]string{
			"tool":     thread.result.Tool,
			"model":    usage.Unknown(item.Model),
			"provider": usage.Unknown(item.Provider),
		}
		bucketID := serializeKey(DefaultGroupBy(), key)
		if grouped[bucketID] == nil {
			grouped[bucketID] = &Result{
				Key:         key,
				PriceSource: item.PriceSource,
				Price:       clonePrice(item.Price),
			}
		}
		bucket := grouped[bucketID]
		bucket.Events += item.Events
		bucket.Requests += item.Requests
		bucket.CostUSD += item.CostUSD
		bucket.Usage = bucket.Usage.Add(item.Usage)
		bucket.PriceSource = mergePriceSource(bucket.PriceSource, item.PriceSource)
		bucket.Price = mergeAggregatedPrice(bucket.Price, item.Price)
	}
	return grouped
}

func mergeGroupedResult(buckets map[string]*Result, groupBy GroupBy, key map[string]string, next Result) {
	bucketKey := serializeKey(groupBy, key)
	if buckets[bucketKey] == nil {
		buckets[bucketKey] = &Result{Key: key}
	}
	bucket := buckets[bucketKey]
	bucket.Events += next.Events
	bucket.Requests += next.Requests
	bucket.CostUSD += next.CostUSD
	bucket.Usage = bucket.Usage.Add(next.Usage)
	bucket.PriceSource = mergePriceSource(bucket.PriceSource, next.PriceSource)
	bucket.Price = mergeAggregatedPrice(bucket.Price, next.Price)
}

func supportsProviderRebalance(groupBy GroupBy) bool {
	if !SupportsThreadBaseline(groupBy) {
		return false
	}
	for _, group := range groupBy {
		if group == "provider" {
			return true
		}
	}
	return false
}

type rebalancedThreadEvent struct {
	Model       string
	Provider    string
	Events      int
	Requests    int
	CostUSD     float64
	Usage       usage.TokenUsage
	PriceSource string
	Price       *Price
}

const (
	codexMixedProviderBridgeCarryRequests = 1.4
	codexMixedProviderBridgeMaxTurns      = 5
)

func rebalanceMixedProviderThread(tool string, turns []ThreadTurn, costFor func(usage.UsageEvent) Cost) ([]rebalancedThreadEvent, bool) {
	if len(turns) == 0 {
		return nil, false
	}
	items := make([]rebalancedThreadEvent, 0, len(turns))
	changed := false
	for i := 0; i < len(turns); {
		if !isBridgeTurnStart(turns, i) {
			items = append(items, threadTurnToRebalancedEvent(tool, turns[i], usage.Unknown(turns[i].Provider), 1, costFor))
			i++
			continue
		}
		prevProvider := usage.Unknown(turns[i-1].Provider)
		nextProvider, end := bridgeTargetProvider(turns, i)
		if nextProvider == "" || nextProvider == prevProvider || !isRebalanceableBridgeSegment(turns, i, end) {
			items = append(items, threadTurnToRebalancedEvent(tool, turns[i], usage.Unknown(turns[i].Provider), 1, costFor))
			i++
			continue
		}
		changed = true
		splitTurnID := turns[i].ID
		for j := i; j < end; j++ {
			if turns[j].ID == splitTurnID {
				items = append(items, splitBridgeTurn(tool, turns[j], prevProvider, nextProvider, costFor)...)
				continue
			}
			items = append(items, threadTurnToRebalancedEvent(tool, turns[j], nextProvider, 1, costFor))
		}
		i = end
	}
	if !changed {
		return nil, false
	}
	grouped := map[string]*rebalancedThreadEvent{}
	order := []string{}
	for _, item := range items {
		key := usage.Unknown(item.Model) + "|" + usage.Unknown(item.Provider)
		if grouped[key] == nil {
			grouped[key] = &rebalancedThreadEvent{
				Model:    usage.Unknown(item.Model),
				Provider: usage.Unknown(item.Provider),
			}
			order = append(order, key)
		}
		groupedItem := grouped[key]
		groupedItem.Events += item.Events
		groupedItem.Requests += item.Requests
		groupedItem.CostUSD += item.CostUSD
		groupedItem.Usage = groupedItem.Usage.Add(item.Usage)
		groupedItem.PriceSource = mergePriceSource(groupedItem.PriceSource, item.PriceSource)
		groupedItem.Price = mergeAggregatedPrice(groupedItem.Price, item.Price)
	}
	results := make([]rebalancedThreadEvent, 0, len(order))
	for _, key := range order {
		results = append(results, *grouped[key])
	}
	return results, true
}

func isBridgeTurnStart(turns []ThreadTurn, idx int) bool {
	if idx <= 0 || idx >= len(turns)-1 {
		return false
	}
	current := turns[idx]
	if normalizeProviderAttribution(current.ProviderAttribution) != string(usage.ProviderAttributionInferredTimeline) {
		return false
	}
	prev := turns[idx-1]
	if normalizeProviderAttribution(prev.ProviderAttribution) != string(usage.ProviderAttributionExactRequest) {
		return false
	}
	nextProvider, _ := bridgeTargetProvider(turns, idx)
	return nextProvider != "" && nextProvider != usage.Unknown(current.Provider)
}

func bridgeTargetProvider(turns []ThreadTurn, start int) (string, int) {
	currentProvider := usage.Unknown(turns[start].Provider)
	for i := start + 1; i < len(turns); i++ {
		if turns[i].ID == turns[start].ID {
			continue
		}
		provider := usage.Unknown(turns[i].Provider)
		if provider == currentProvider && normalizeProviderAttribution(turns[i].ProviderAttribution) == string(usage.ProviderAttributionInferredTimeline) {
			continue
		}
		return provider, i
	}
	return "", len(turns)
}

func isRebalanceableBridgeSegment(turns []ThreadTurn, start, end int) bool {
	if start <= 0 || end <= start || end >= len(turns) {
		return false
	}
	if normalizeProviderAttribution(turns[end].ProviderAttribution) != string(usage.ProviderAttributionExactRequest) {
		return false
	}
	seenTurns := map[string]struct{}{}
	for i := start; i < end; i++ {
		if normalizeProviderAttribution(turns[i].ProviderAttribution) != string(usage.ProviderAttributionInferredTimeline) {
			return false
		}
		id := strings.TrimSpace(turns[i].ID)
		if id == "" {
			id = strings.TrimSpace(turns[i].EventID)
		}
		seenTurns[id] = struct{}{}
		if len(seenTurns) > codexMixedProviderBridgeMaxTurns {
			return false
		}
	}
	return true
}

func clonePrice(price *Price) *Price {
	if price == nil {
		return nil
	}
	copy := *price
	copy.Components = clonePriceComponents(price.Components)
	return &copy
}

func clonePriceComponents(components []Price) []Price {
	if len(components) == 0 {
		return nil
	}
	out := make([]Price, len(components))
	for i := range components {
		out[i] = *clonePrice(&components[i])
	}
	return out
}

func threadTurnToRebalancedEvent(tool string, turn ThreadTurn, provider string, fraction float64, costFor func(usage.UsageEvent) Cost) rebalancedThreadEvent {
	scaledUsage := scaleTokenUsage(turn.Usage, fraction)
	costUSD := turn.CostUSD * fraction
	priceSource := turn.PriceSource
	price := clonePrice(turn.Price)
	if costFor != nil {
		cost := costFor(usage.UsageEvent{
			ID:                  turn.EventID,
			TurnID:              turn.ID,
			Timestamp:           turn.Timestamp,
			Tool:                usage.Tool(tool),
			Model:               usage.Unknown(turn.Model),
			Provider:            usage.Unknown(provider),
			ProviderAttribution: turn.ProviderAttribution,
			Usage:               turn.Usage,
		})
		costUSD = cost.USD * fraction
		priceSource = mergePriceSource("", cost.Source)
		price = priceFromCost(cost)
	}
	return rebalancedThreadEvent{
		Model:       usage.Unknown(turn.Model),
		Provider:    usage.Unknown(provider),
		Events:      1,
		Requests:    1,
		CostUSD:     costUSD,
		Usage:       scaledUsage,
		PriceSource: priceSource,
		Price:       price,
	}
}

func splitBridgeTurn(tool string, turn ThreadTurn, prevProvider, nextProvider string, costFor func(usage.UsageEvent) Cost) []rebalancedThreadEvent {
	sequence := turn.TurnEventIndex
	preserve := 0.0
	switch {
	case strings.TrimSpace(turn.EventID) == "":
		preserve = 1
	case sequence == 1:
		preserve = minFloat64(1, codexMixedProviderBridgeCarryRequests)
	case sequence == 2:
		preserve = maxFloat64(0, minFloat64(1, codexMixedProviderBridgeCarryRequests-1))
	}
	if preserve <= 0 {
		return []rebalancedThreadEvent{threadTurnToRebalancedEvent(tool, turn, nextProvider, 1, costFor)}
	}
	if preserve >= 1 {
		return []rebalancedThreadEvent{
			threadTurnToRebalancedEvent(tool, turn, prevProvider, 1, costFor),
		}
	}
	return []rebalancedThreadEvent{
		threadTurnToRebalancedEvent(tool, turn, prevProvider, preserve, costFor),
		threadTurnToRebalancedEvent(tool, turn, nextProvider, 1-preserve, costFor),
	}
}

func scaleTokenUsage(tokens usage.TokenUsage, fraction float64) usage.TokenUsage {
	if fraction <= 0 {
		return usage.TokenUsage{}
	}
	if fraction >= 1 {
		return tokens
	}
	return usage.TokenUsage{
		Input:           scaleInt64(tokens.Input, fraction),
		Output:          scaleInt64(tokens.Output, fraction),
		CachedInput:     scaleInt64(tokens.CachedInput, fraction),
		CacheCreation:   scaleInt64(tokens.CacheCreation, fraction),
		CacheCreation5m: scaleInt64(tokens.CacheCreation5m, fraction),
		CacheCreation1h: scaleInt64(tokens.CacheCreation1h, fraction),
		Reasoning:       scaleInt64(tokens.Reasoning, fraction),
		Tool:            scaleInt64(tokens.Tool, fraction),
		Total:           scaleInt64(tokens.Total, fraction),
	}
}

func scaleInt64(value int64, fraction float64) int64 {
	return int64(float64(value) * fraction)
}

func minFloat64(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func maxFloat64(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func regroupThreadBucketKey(source map[string]string, groupBy GroupBy) map[string]string {
	key := map[string]string{}
	for _, group := range groupBy {
		key[group] = source[group]
	}
	return key
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
	case "custom", "official", "unpriced":
		return strings.ToLower(strings.TrimSpace(source))
	case "mixed":
		return "mixed"
	default:
		return ""
	}
}

func mergePrice(existing *Price, cost Cost) *Price {
	next := priceFromCost(cost)
	return mergeAggregatedPrice(existing, next)
}

func mergeAggregatedPrice(existing, next *Price) *Price {
	if next == nil {
		return existing
	}
	if existing == nil {
		return clonePrice(next)
	}
	if pricesEqual(*existing, *next) {
		return existing
	}
	components := mergePriceComponents(existing, next)
	if len(components) == 1 {
		return clonePrice(&components[0])
	}
	return &Price{Source: "mixed", Components: components}
}

func mergePriceComponents(prices ...*Price) []Price {
	componentsByKey := map[string]Price{}
	for _, price := range prices {
		for _, component := range priceComponents(price) {
			component.Components = nil
			componentsByKey[priceComponentKey(component)] = component
		}
	}
	components := make([]Price, 0, len(componentsByKey))
	for _, component := range componentsByKey {
		components = append(components, component)
	}
	sort.Slice(components, func(i, j int) bool {
		return priceComponentKey(components[i]) < priceComponentKey(components[j])
	})
	return components
}

func priceComponents(price *Price) []Price {
	if price == nil {
		return nil
	}
	if price.Source == "mixed" && len(price.Components) > 0 {
		out := make([]Price, 0, len(price.Components))
		for _, component := range price.Components {
			out = append(out, priceComponents(&component)...)
		}
		return out
	}
	component := *price
	component.Components = nil
	return []Price{component}
}

func priceComponentKey(price Price) string {
	return fmt.Sprintf("%s|%.12g|%.12g|%.12g|%.12g|%.12g",
		price.Source,
		price.InputUSDPerMTok,
		price.OutputUSDPerMTok,
		price.CacheHitUSDPerMTok,
		price.CacheMakeUSDPerMTok,
		price.CacheMake1hUSDPerMTok,
	)
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

func summarizeThreadCosts(costs map[string]float64) []ThreadCost {
	if len(costs) <= 1 {
		return nil
	}
	items := make([]ThreadCost, 0, len(costs))
	for provider, usd := range costs {
		if provider == "" || provider == "unknown" {
			continue
		}
		items = append(items, ThreadCost{Provider: provider, USD: usd})
	}
	if len(items) <= 1 {
		return nil
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].USD != items[j].USD {
			return items[i].USD > items[j].USD
		}
		return items[i].Provider < items[j].Provider
	})
	return items
}

func recordThreadAttribution(bucket *threadBucket, provider string, attribution string, tokens usage.TokenUsage, usd float64) {
	if bucket == nil {
		return
	}
	provider = usage.Unknown(provider)
	attribution = normalizeProviderAttribution(attribution)
	if bucket.attributionByProvider == nil {
		bucket.attributionByProvider = map[string]map[string]*AttributionCost{}
	}
	if bucket.attributionByProvider[provider] == nil {
		bucket.attributionByProvider[provider] = map[string]*AttributionCost{}
	}
	item := bucket.attributionByProvider[provider][attribution]
	if item == nil {
		item = &AttributionCost{Source: attribution}
		bucket.attributionByProvider[provider][attribution] = item
	}
	item.Requests++
	item.USD += usd
	item.Usage = item.Usage.Add(tokens)
}

func recordThreadTurn(bucket *threadBucket, event usage.UsageEvent, usd float64, priceSource string, price *Price) {
	if bucket == nil {
		return
	}
	id := strings.TrimSpace(event.TurnID)
	if id == "" {
		id = strings.TrimSpace(event.ID)
	}
	bucket.turnEventCounts[id]++
	bucket.result.Turns = append(bucket.result.Turns, ThreadTurn{
		ID:                  id,
		EventID:             event.ID,
		Timestamp:           event.Timestamp,
		Model:               usage.Unknown(event.Model),
		Provider:            usage.Unknown(event.Provider),
		ProviderAttribution: normalizeProviderAttribution(event.ProviderAttribution),
		CostUSD:             usd,
		Usage:               event.Usage,
		TurnEventIndex:      bucket.turnEventCounts[id],
		PriceSource:         priceSource,
		Price:               clonePrice(price),
	})
}

func normalizeProviderAttribution(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return string(usage.ProviderAttributionSessionFallback)
	}
	return value
}

func summarizeThreadAttributions(items map[string]map[string]*AttributionCost) []ThreadAttribution {
	if len(items) == 0 {
		return nil
	}
	providers := make([]string, 0, len(items))
	for provider := range items {
		providers = append(providers, provider)
	}
	sort.Slice(providers, func(i, j int) bool {
		return providers[i] < providers[j]
	})
	out := make([]ThreadAttribution, 0, len(providers))
	for _, provider := range providers {
		sources := items[provider]
		keys := make([]string, 0, len(sources))
		for source := range sources {
			keys = append(keys, source)
		}
		sort.Slice(keys, func(i, j int) bool {
			left := sources[keys[i]]
			right := sources[keys[j]]
			if left.USD != right.USD {
				return left.USD > right.USD
			}
			if left.Requests != right.Requests {
				return left.Requests > right.Requests
			}
			return keys[i] < keys[j]
		})
		row := ThreadAttribution{Provider: provider, BySource: make([]AttributionCost, 0, len(keys))}
		for _, key := range keys {
			row.BySource = append(row.BySource, *sources[key])
		}
		out = append(out, row)
	}
	sort.Slice(out, func(i, j int) bool {
		left := sumAttributionUSD(out[i].BySource)
		right := sumAttributionUSD(out[j].BySource)
		if left != right {
			return left > right
		}
		return out[i].Provider < out[j].Provider
	})
	return out
}

func sumAttributionUSD(items []AttributionCost) float64 {
	var total float64
	for _, item := range items {
		total += item.USD
	}
	return total
}

func summarizePriceSources(values map[string]struct{}) string {
	switch len(values) {
	case 0:
		return ""
	case 1:
		for value := range values {
			return value
		}
	}
	return "mixed"
}

func projectKey(source map[string]string, groupBy GroupBy) map[string]string {
	key := map[string]string{}
	for _, group := range groupBy {
		key[group] = source[group]
	}
	return key
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
