package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/MagnumGoYB/aitok/internal/query"
	"github.com/MagnumGoYB/aitok/internal/report"
	"github.com/charmbracelet/lipgloss"
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
	cur := threadCurrency2(thread)
	if len(thread.CostBreakdown) == 0 {
		return tuiFormatCost(thread.CostUSD, cur)
	}
	markers := tuiThreadCostMarkers(thread.CostBreakdown, cur)
	if len(markers) > 0 {
		var usdTotal float64
		for _, item := range thread.CostBreakdown {
			if item.Currency == "" || item.Currency == "USD" {
				usdTotal += item.USD
			}
		}
		return tuiFormatCost(usdTotal, "USD") + "[" + strings.Join(markers, "") + "]"
	}
	return tuiFormatCost(thread.CostUSD, cur)
}

func tuiThreadCostMarkers(breakdown []query.ThreadCost, threadCurrency string) []string {
	seen := map[string]struct{}{}
	for _, item := range breakdown {
		cur := item.Currency
		if cur == "" {
			continue
		}
		if strings.EqualFold(cur, threadCurrency) {
			continue
		}
		if cur == "USD" {
			continue
		}
		seen["+"+report.CurrencySymbol(cur)] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for s := range seen {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

func tuiThreadCostDetail(thread query.ThreadResult) string {
	cur := threadCurrency2(thread)
	totalUSD := thread.CostUSD
	if totalUSD == 0 && len(thread.CostBreakdown) > 0 {
		for _, item := range thread.CostBreakdown {
			totalUSD += item.USD
		}
	}
	total := tuiFormatCost(totalUSD, cur)
	if len(thread.CostBreakdown) == 0 {
		return total
	}
	parts := make([]string, 0, len(thread.CostBreakdown))
	for _, item := range thread.CostBreakdown {
		if item.Provider == "" {
			continue
		}
		itemCurrency := item.Currency
		if itemCurrency == "" {
			itemCurrency = cur
		}
		parts = append(parts, item.Provider+" "+tuiFormatCost(item.USD, itemCurrency))
	}
	if len(parts) == 0 {
		return total
	}
	return total + " (" + strings.Join(parts, " / ") + ")"
}

func threadCurrency2(thread query.ThreadResult) string {
	if thread.Price != nil && thread.Price.Currency != "" {
		return thread.Price.Currency
	}
	return "USD"
}

func resultCurrency2(result query.Result) string {
	if result.Price != nil && result.Price.Currency != "" {
		return result.Price.Currency
	}
	return "USD"
}

func tuiFormatCost(value float64, currency string) string {
	return report.FormatCost(value, currency)
}

func tuiFormatCosts(costs map[string]float64) string {
	if len(costs) == 0 {
		return tuiFormatCost(0, "USD")
	}
	usd := costs["USD"]
	cny := costs["CNY"] + costs["RMB"]
	parts := make([]string, 0, len(costs))
	if usd != 0 || cny == 0 {
		parts = append(parts, tuiFormatCost(usd, "USD"))
	}
	if cny != 0 {
		cnyPart := tuiFormatCost(cny, "CNY")
		if len(parts) == 0 {
			parts = append(parts, cnyPart)
		} else {
			parts = append(parts, "("+cnyPart+")")
		}
	}
	otherCurrencies := make([]string, 0, len(costs))
	for currency := range costs {
		upper := strings.ToUpper(currency)
		if upper == "" || upper == "USD" || upper == "CNY" || upper == "RMB" {
			continue
		}
		otherCurrencies = append(otherCurrencies, currency)
	}
	sort.Strings(otherCurrencies)
	for _, currency := range otherCurrencies {
		parts = append(parts, "("+tuiFormatCost(costs[currency], currency)+")")
	}
	return strings.Join(parts, " ")
}

func tuiPrimaryCurrency(costs map[string]float64) string {
	if len(costs) == 0 {
		return "USD"
	}
	if costs["USD"] != 0 {
		return "USD"
	}
	if costs["CNY"] != 0 || costs["RMB"] != 0 {
		return "CNY"
	}
	currencies := make([]string, 0, len(costs))
	for currency, amount := range costs {
		if amount != 0 {
			currencies = append(currencies, currency)
		}
	}
	if len(currencies) == 0 {
		return "USD"
	}
	sort.Strings(currencies)
	return currencies[0]
}

func tuiCurrencyIcon(currency string) string {
	switch strings.ToUpper(currency) {
	case "CNY", "RMB":
		return "¥"
	default:
		return "$"
	}
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
	if width <= 0 || lipgloss.Width(value) <= width {
		return value
	}
	limit := width - lipgloss.Width(suffix)
	if limit <= 0 {
		return suffix
	}
	var b strings.Builder
	used := 0
	for _, r := range value {
		charWidth := lipgloss.Width(string(r))
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
