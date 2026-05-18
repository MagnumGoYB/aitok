package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/MagnumGoYB/aitok/internal/query"
	"github.com/MagnumGoYB/aitok/internal/report"
	"github.com/mattn/go-runewidth"
)

type threadSelectionDetail struct {
	id         string
	name       string
	tool       string
	model      string
	provider   string
	source     string
	lastActive string
	cost       string
	split      string
	tokens     string
}

func resultLabel(result query.Result) string {
	if model := result.Key["model"]; model != "" && model != "unknown" {
		if provider := result.Key["provider"]; provider != "" && provider != "unknown" {
			return fmt.Sprintf("%s (%s)", model, provider)
		}
		return model
	}
	if tool := result.Key["tool"]; tool != "" {
		return tool
	}
	return formatKey(result.Key)
}

func tuiThreadCost(thread query.ThreadResult) string {
	return report.FormatUSD(thread.CostUSD)
}

func tuiThreadCostDetail(thread query.ThreadResult) string {
	totalUSD := thread.CostUSD
	if totalUSD == 0 && len(thread.CostBreakdown) > 0 {
		for _, item := range thread.CostBreakdown {
			totalUSD += item.USD
		}
	}
	total := report.FormatUSD(totalUSD)
	if len(thread.CostBreakdown) == 0 {
		return total
	}
	parts := make([]string, 0, len(thread.CostBreakdown))
	for _, item := range thread.CostBreakdown {
		if item.Provider == "" {
			continue
		}
		parts = append(parts, item.Provider+" "+report.FormatUSD(item.USD))
	}
	if len(parts) == 0 {
		return total
	}
	return total + " (" + strings.Join(parts, " / ") + ")"
}

func primaryThreadValue(value string) string {
	parts := strings.Split(value, ",")
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			return trimmed
		}
	}
	return value
}

func selectedThreadDetail(thread query.ThreadResult) threadSelectionDetail {
	lastActive := "-"
	if !thread.LastActiveAt.IsZero() {
		lastActive = thread.LastActiveAt.In(time.Local).Format("2006-01-02 15:04")
	}
	return threadSelectionDetail{
		id:         thread.ID,
		name:       thread.Name,
		tool:       thread.Tool,
		model:      thread.Model,
		provider:   thread.Provider,
		source:     thread.Source,
		lastActive: lastActive,
		cost:       tuiThreadCost(thread),
		split:      tuiThreadCostDetail(thread),
		tokens:     compact(thread.Usage.NormalizedTotal()),
	}
}

func formatKey(key map[string]string) string {
	parts := make([]string, 0, len(key))
	for k, v := range key {
		parts = append(parts, k+"="+v)
	}
	sort.Strings(parts)
	return strings.Join(parts, ", ")
}

func formatInt(value int64) string {
	if value < 1000 {
		return fmt.Sprintf("%d", value)
	}
	text := fmt.Sprintf("%d", value)
	var parts []string
	for len(text) > 3 {
		parts = append([]string{text[len(text)-3:]}, parts...)
		text = text[:len(text)-3]
	}
	parts = append([]string{text}, parts...)
	return strings.Join(parts, ",")
}

func compact(value int64) string {
	if value >= 1_000_000 {
		return fmt.Sprintf("%.1fm", float64(value)/1_000_000)
	}
	if value >= 1_000 {
		return fmt.Sprintf("%.1fk", float64(value)/1_000)
	}
	return fmt.Sprintf("%d", value)
}

func tableText(value string, width int) string {
	value = strings.Join(strings.Fields(value), " ")
	return displayTextWithSuffix(value, width, "...")
}

func displayTextWithSuffix(value string, width int, suffix string) string {
	if width <= 0 || runewidth.StringWidth(value) <= width {
		return value
	}
	limit := width - runewidth.StringWidth(suffix)
	if limit <= 0 {
		return suffix
	}
	var b strings.Builder
	used := 0
	for _, r := range value {
		charWidth := runewidth.RuneWidth(r)
		if used+charWidth > limit {
			break
		}
		b.WriteRune(r)
		used += charWidth
	}
	return b.String() + suffix
}

func priceLabel(price *query.Price, source string) string {
	return report.FormatPriceCompact(price, source)
}
