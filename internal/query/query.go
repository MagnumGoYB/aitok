package query

import (
	"sort"
	"strings"
	"time"

	"github.com/sosbs/aitok/internal/usage"
)

type Filters struct {
	Tools     []string
	Models    []string
	Providers []string
	CWD       string
}

type GroupBy []string

type Result struct {
	Key      map[string]string `json:"key"`
	Events   int               `json:"events"`
	Usage    usage.TokenUsage  `json:"usage"`
	Examples map[string]string `json:"examples,omitempty"`
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
	buckets := map[string]*Result{}
	for _, event := range events {
		if !window.Contains(event.Timestamp) || !matches(event, filters) {
			continue
		}
		key := keyFor(event, groupBy, window.Start.Location())
		bucketKey := serializeKey(groupBy, key)
		if buckets[bucketKey] == nil {
			buckets[bucketKey] = &Result{Key: key, Examples: map[string]string{}}
		}
		buckets[bucketKey].Events++
		buckets[bucketKey].Usage = buckets[bucketKey].Usage.Add(event.Usage)
		if event.CWD != "" && buckets[bucketKey].Examples["cwd"] == "" {
			buckets[bucketKey].Examples["cwd"] = event.CWD
		}
	}
	results := make([]Result, 0, len(buckets))
	for _, result := range buckets {
		if len(result.Examples) == 0 {
			result.Examples = nil
		}
		results = append(results, *result)
	}
	sort.Slice(results, func(i, j int) bool {
		left := results[i].Usage.NormalizedTotal()
		right := results[j].Usage.NormalizedTotal()
		if left == right {
			return serializeKey(groupBy, results[i].Key) < serializeKey(groupBy, results[j].Key)
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
