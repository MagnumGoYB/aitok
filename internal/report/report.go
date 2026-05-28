package report

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/MagnumGoYB/aitok/internal/query"
	"github.com/mattn/go-runewidth"
)

const threadNameTableWidth = 28

type Payload struct {
	GeneratedAt time.Time            `json:"generated_at"`
	Period      query.Period         `json:"period,omitempty"`
	Window      query.Window         `json:"window"`
	GroupBy     query.GroupBy        `json:"group_by"`
	SortBy      query.SortMetric     `json:"sort_by,omitempty"`
	Results     []query.Result       `json:"results"`
	Threads     []query.ThreadResult `json:"threads,omitempty"`
}

type Options struct {
	Full bool
}

func Write(w io.Writer, format string, payload Payload, opts ...Options) error {
	var option Options
	if len(opts) > 0 {
		option = opts[0]
	}
	switch format {
	case "", "table":
		if err := WriteTable(w, payload.Results, option); err != nil {
			return err
		}
		if len(payload.Threads) > 0 {
			if _, err := fmt.Fprintln(w); err != nil {
				return err
			}
			return WriteThreadsTable(w, payload.Threads, option)
		}
		return nil
	case "json":
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(payload)
	case "markdown":
		if err := WriteMarkdown(w, payload.Results, option); err != nil {
			return err
		}
		if len(payload.Threads) > 0 {
			if _, err := fmt.Fprintln(w, "\n## Threads"); err != nil {
				return err
			}
			return WriteThreadsMarkdown(w, payload.Threads, option)
		}
		return nil
	default:
		return fmt.Errorf("unknown format %q", format)
	}
}

func WriteThreadsTable(w io.Writer, threads []query.ThreadResult, opts ...Options) error {
	var option Options
	if len(opts) > 0 {
		option = opts[0]
	}
	headers := []string{"ID", "NAME", "TOOL", "MODEL", "PROVIDER", "REQ", "COST", "PRICE", "TOTAL"}
	if option.Full {
		headers = []string{"ID", "NAME", "TOOL", "MODEL", "PROVIDER", "REQ", "EVENTS", "COST", "PRICE", "TOTAL"}
	}
	rows := make([][]string, 0, len(threads))
	for _, thread := range threads {
		row := []string{
			thread.ID,
			displayCell(thread.Name, threadNameTableWidth),
			thread.Tool,
			thread.Model,
			thread.Provider,
			fmt.Sprint(thread.Requests),
			FormatThreadCost(thread),
			formatPrice(thread.Price, thread.PriceSource),
			fmt.Sprint(thread.Usage.NormalizedTotal()),
		}
		if option.Full {
			row = []string{
				thread.ID,
				displayCell(thread.Name, threadNameTableWidth),
				thread.Tool,
				thread.Model,
				thread.Provider,
				fmt.Sprint(thread.Requests),
				fmt.Sprint(thread.Events),
				FormatThreadCost(thread),
				formatPrice(thread.Price, thread.PriceSource),
				fmt.Sprint(thread.Usage.NormalizedTotal()),
			}
		}
		rows = append(rows, row)
	}
	return writeBorderedTable(w, headers, rows)
}

func WriteTable(w io.Writer, results []query.Result, opts ...Options) error {
	var option Options
	if len(opts) > 0 {
		option = opts[0]
	}
	headers := []string{"GROUP", "REQ", "COST", "PRICE", "TOTAL"}
	if option.Full {
		headers = []string{"GROUP", "REQ", "EVENTS", "COST", "PRICE", "INPUT", "OUTPUT", "CACHED", "CACHE_CREATE", "REASONING", "TOOL", "TOTAL"}
	}
	rows := make([][]string, 0, len(results))
	for _, result := range results {
		row := []string{
			formatKey(result.Key),
			fmt.Sprint(result.Requests),
			FormatCost(result.CostUSD, resultCurrency(result)),
			formatPrice(result.Price, result.PriceSource),
			fmt.Sprint(result.Usage.NormalizedTotal()),
		}
		if option.Full {
			row = []string{
				formatKey(result.Key),
				fmt.Sprint(result.Requests),
				fmt.Sprint(result.Events),
				FormatCost(result.CostUSD, resultCurrency(result)),
				formatPrice(result.Price, result.PriceSource),
				fmt.Sprint(result.Usage.Input),
				fmt.Sprint(result.Usage.Output),
				fmt.Sprint(result.Usage.CachedInput),
				fmt.Sprint(result.Usage.CacheCreation),
				fmt.Sprint(result.Usage.Reasoning),
				fmt.Sprint(result.Usage.Tool),
				fmt.Sprint(result.Usage.NormalizedTotal()),
			}
		}
		rows = append(rows, row)
	}
	return writeBorderedTable(w, headers, rows)
}

func writeBorderedTable(w io.Writer, headers []string, rows [][]string) error {
	widths := make([]int, len(headers))
	for i, header := range headers {
		widths[i] = runewidth.StringWidth(header)
	}
	for _, row := range rows {
		for i, value := range row {
			if width := runewidth.StringWidth(value); width > widths[i] {
				widths[i] = width
			}
		}
	}
	border := tableBorder(widths)
	if _, err := fmt.Fprintln(w, border); err != nil {
		return err
	}
	if err := writeTableRow(w, headers, widths); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, border); err != nil {
		return err
	}
	for _, row := range rows {
		if err := writeTableRow(w, row, widths); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(w, border)
	return err
}

func tableBorder(widths []int) string {
	var b strings.Builder
	b.WriteString("+")
	for _, width := range widths {
		b.WriteString(strings.Repeat("-", width+2))
		b.WriteString("+")
	}
	return b.String()
}

func writeTableRow(w io.Writer, row []string, widths []int) error {
	var b strings.Builder
	b.WriteString("|")
	for i, value := range row {
		b.WriteString(" ")
		if i == 0 {
			b.WriteString(value)
			b.WriteString(strings.Repeat(" ", widths[i]-runewidth.StringWidth(value)))
		} else {
			b.WriteString(strings.Repeat(" ", widths[i]-runewidth.StringWidth(value)))
			b.WriteString(value)
		}
		b.WriteString(" |")
	}
	_, err := fmt.Fprintln(w, b.String())
	return err
}

func WriteMarkdown(w io.Writer, results []query.Result, opts ...Options) error {
	var option Options
	if len(opts) > 0 {
		option = opts[0]
	}
	if option.Full {
		if _, err := fmt.Fprintln(w, "| Group | Req | Events | Cost | Price | Input | Output | Cached | Cache Create | Reasoning | Tool | Total |"); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w, "| --- | ---: | ---: | ---: | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |"); err != nil {
			return err
		}
		for _, result := range results {
			if _, err := fmt.Fprintf(w, "| %s | %d | %d | %s | %s | %d | %d | %d | %d | %d | %d | %d |\n",
				escapeMarkdown(formatKey(result.Key)),
				result.Requests,
				result.Events,
				FormatCost(result.CostUSD, resultCurrency(result)),
				escapeMarkdown(formatPrice(result.Price, result.PriceSource)),
				result.Usage.Input,
				result.Usage.Output,
				result.Usage.CachedInput,
				result.Usage.CacheCreation,
				result.Usage.Reasoning,
				result.Usage.Tool,
				result.Usage.NormalizedTotal(),
			); err != nil {
				return err
			}
		}
		return nil
	}
	if _, err := fmt.Fprintln(w, "| Group | Req | Cost | Price | Total |"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "| --- | ---: | ---: | --- | ---: |"); err != nil {
		return err
	}
	for _, result := range results {
		if _, err := fmt.Fprintf(w, "| %s | %d | %s | %s | %d |\n",
			escapeMarkdown(formatKey(result.Key)),
			result.Requests,
			FormatCost(result.CostUSD, resultCurrency(result)),
			escapeMarkdown(formatPrice(result.Price, result.PriceSource)),
			result.Usage.NormalizedTotal(),
		); err != nil {
			return err
		}
	}
	return nil
}

func WriteThreadsMarkdown(w io.Writer, threads []query.ThreadResult, opts ...Options) error {
	var option Options
	if len(opts) > 0 {
		option = opts[0]
	}
	if option.Full {
		if _, err := fmt.Fprintln(w, "| ID | Name | Tool | Model | Provider | Req | Events | Cost | Price | Total |"); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w, "| --- | --- | --- | --- | --- | ---: | ---: | ---: | --- | ---: |"); err != nil {
			return err
		}
		for _, thread := range threads {
			if _, err := fmt.Fprintf(w, "| %s | %s | %s | %s | %s | %d | %d | %s | %s | %d |\n",
				escapeMarkdown(thread.ID),
				escapeMarkdown(thread.Name),
				escapeMarkdown(thread.Tool),
				escapeMarkdown(thread.Model),
				escapeMarkdown(thread.Provider),
				thread.Requests,
				thread.Events,
				FormatThreadCost(thread),
				escapeMarkdown(formatPrice(thread.Price, thread.PriceSource)),
				thread.Usage.NormalizedTotal(),
			); err != nil {
				return err
			}
		}
		return nil
	}
	if _, err := fmt.Fprintln(w, "| ID | Name | Tool | Model | Provider | Req | Cost | Price | Total |"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "| --- | --- | --- | --- | --- | ---: | ---: | --- | ---: |"); err != nil {
		return err
	}
	for _, thread := range threads {
		if _, err := fmt.Fprintf(w, "| %s | %s | %s | %s | %s | %d | %s | %s | %d |\n",
			escapeMarkdown(thread.ID),
			escapeMarkdown(thread.Name),
			escapeMarkdown(thread.Tool),
			escapeMarkdown(thread.Model),
			escapeMarkdown(thread.Provider),
			thread.Requests,
			FormatThreadCost(thread),
			escapeMarkdown(formatPrice(thread.Price, thread.PriceSource)),
			thread.Usage.NormalizedTotal(),
		); err != nil {
			return err
		}
	}
	return nil
}

func currencySymbol(currency string) string {
	switch strings.ToUpper(currency) {
	case "CNY", "RMB":
		return "¥"
	default:
		return "$"
	}
}

func FormatUSD(value float64) string {
	return FormatCost(value, "")
}

func FormatCost(value float64, currency string) string {
	return fmt.Sprintf("%s%.4f", currencySymbol(currency), value)
}

func FormatThreadCost(thread query.ThreadResult) string {
	currency := threadCurrency(thread)
	total := FormatCost(thread.CostUSD, currency)
	if len(thread.CostBreakdown) == 0 {
		return total
	}
	parts := make([]string, 0, len(thread.CostBreakdown))
	for _, item := range thread.CostBreakdown {
		parts = append(parts, item.Provider+" "+FormatCost(item.USD, currency))
	}
	return total + " (" + strings.Join(parts, ", ") + ")"
}

func threadCurrency(thread query.ThreadResult) string {
	if thread.Price != nil && thread.Price.Currency != "" {
		return thread.Price.Currency
	}
	return "USD"
}

func resultCurrency(result query.Result) string {
	if result.Price != nil && result.Price.Currency != "" {
		return result.Price.Currency
	}
	return "USD"
}

func formatPrice(price *query.Price, source string) string {
	return FormatPrice(price, source)
}

func priceCurrency(price *query.Price) string {
	if price == nil {
		return "USD"
	}
	if price.Currency != "" {
		return price.Currency
	}
	if price.Source == "mixed" && len(price.Components) > 0 {
		if cur := price.Components[0].Currency; cur != "" {
			return cur
		}
	}
	return "USD"
}

func FormatPrice(price *query.Price, source string) string {
	if price == nil {
		return displayPriceSource(source)
	}
	cur := priceCurrency(price)
	if price.Source == "mixed" {
		return formatMixedPrice(price, source)
	}
	if price.Source == "unpriced" {
		return price.Source
	}
	return fmt.Sprintf("%s in=%s out=%s cache=%s make=%s",
		displayPriceSource(price.Source),
		formatRateWithCurrency(price.InputUSDPerMTok, cur),
		formatRateWithCurrency(price.OutputUSDPerMTok, cur),
		formatRateWithCurrency(price.CacheHitUSDPerMTok, cur),
		formatRateWithCurrency(price.CacheMakeUSDPerMTok, cur),
	)
}

func FormatPriceCompact(price *query.Price, source string) string {
	if price == nil {
		return displayPriceSource(source)
	}
	cur := priceCurrency(price)
	if price.Source == "mixed" {
		return formatMixedPriceCompact(price, source)
	}
	if price.Source == "unpriced" {
		return price.Source
	}
	return fmt.Sprintf("%s in=%s out=%s",
		displayPriceSource(price.Source),
		formatRateWithCurrency(price.InputUSDPerMTok, cur),
		formatRateWithCurrency(price.OutputUSDPerMTok, cur),
	)
}

func formatMixedPrice(price *query.Price, source string) string {
	cur := priceCurrency(price)
	if len(price.Components) == 0 {
		return displayPriceSource(coalescePriceSource(price.Source, source))
	}
	return fmt.Sprintf("%s %s in=%s out=%s cache=%s make=%s",
		displayPriceSource(price.Source),
		mixedPriceSources(price.Components),
		formatRateRange(price.Components, cur, func(component query.Price) float64 { return component.InputUSDPerMTok }),
		formatRateRange(price.Components, cur, func(component query.Price) float64 { return component.OutputUSDPerMTok }),
		formatRateRange(price.Components, cur, func(component query.Price) float64 { return component.CacheHitUSDPerMTok }),
		formatRateRange(price.Components, cur, func(component query.Price) float64 { return component.CacheMakeUSDPerMTok }),
	)
}

func formatMixedPriceCompact(price *query.Price, source string) string {
	cur := priceCurrency(price)
	if len(price.Components) == 0 {
		return displayPriceSource(coalescePriceSource(price.Source, source))
	}
	return fmt.Sprintf("%s %s %s/%s",
		displayPriceSource(price.Source),
		mixedPriceSources(price.Components),
		formatRateValueRange(price.Components, cur, func(component query.Price) float64 { return component.InputUSDPerMTok }),
		strings.TrimPrefix(formatRateRange(price.Components, cur, func(component query.Price) float64 { return component.OutputUSDPerMTok }), currencySymbol(cur)),
	)
}

func mixedPriceSources(components []query.Price) string {
	seen := map[string]struct{}{}
	sources := make([]string, 0, len(components))
	for _, component := range components {
		source := displayPriceSource(component.Source)
		if _, ok := seen[source]; ok {
			continue
		}
		seen[source] = struct{}{}
		sources = append(sources, source)
	}
	sort.Strings(sources)
	return strings.Join(sources, "+")
}

func formatRateRange(components []query.Price, currency string, valueFor func(query.Price) float64) string {
	return formatRateValueRange(components, currency, valueFor) + "/M"
}

func formatRateValueRange(components []query.Price, currency string, valueFor func(query.Price) float64) string {
	if len(components) == 0 {
		return formatRateValueWithCurrency(0, currency)
	}
	var minValue, maxValue float64
	hasValue := false
	for _, component := range components {
		value := valueFor(component)
		if !hasValue || value < minValue {
			minValue = value
		}
		if !hasValue || value > maxValue {
			maxValue = value
		}
		hasValue = true
	}
	if minValue == maxValue {
		return formatRateValueWithCurrency(minValue, currency)
	}
	sym := currencySymbol(currency)
	return fmt.Sprintf("%s..%s", formatRateValueWithCurrency(minValue, currency), strings.TrimPrefix(formatRateValueWithCurrency(maxValue, currency), sym))
}

func coalescePriceSource(primary, fallback string) string {
	if strings.TrimSpace(primary) != "" {
		return primary
	}
	return fallback
}

func displayPriceSource(source string) string {
	if source == "" {
		return "unknown"
	}
	return source
}

func formatRate(value float64) string {
	return formatRateWithCurrency(value, "") + "/M"
}

func formatRateWithCurrency(value float64, currency string) string {
	return formatRateValueWithCurrency(value, currency) + "/M"
}

func formatRateValue(value float64) string {
	return formatRateValueWithCurrency(value, "")
}

func formatRateValueWithCurrency(value float64, currency string) string {
	return fmt.Sprintf("%s%.4g", currencySymbol(currency), value)
}

func formatKey(key map[string]string) string {
	keys := make([]string, 0, len(key))
	for k := range key {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		parts = append(parts, k+"="+key[k])
	}
	return strings.Join(parts, ", ")
}

func displayCell(value string, width int) string {
	value = strings.Join(strings.Fields(value), " ")
	return runewidth.Truncate(value, width, "…")
}

func escapeMarkdown(s string) string {
	return strings.ReplaceAll(s, "|", "\\|")
}
