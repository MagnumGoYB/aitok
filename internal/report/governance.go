package report

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/MagnumGoYB/aitok/internal/query"
	"github.com/MagnumGoYB/aitok/internal/usage"
)

type PricingAuditPayload struct {
	GeneratedAt time.Time            `json:"generated_at"`
	Window      query.Window         `json:"window"`
	Unpriced    []PricingAuditResult `json:"unpriced"`
	Skeleton    string               `json:"skeleton,omitempty"`
}

type PricingAuditResult struct {
	Tool     string           `json:"tool"`
	Model    string           `json:"model"`
	Provider string           `json:"provider"`
	Events   int              `json:"events"`
	Usage    usage.TokenUsage `json:"usage"`
	Example  string           `json:"example,omitempty"`
}

type BudgetPayload struct {
	GeneratedAt    time.Time      `json:"generated_at"`
	Window         query.Window   `json:"window"`
	LimitUSD       float64        `json:"limit_usd"`
	TotalUSD       float64        `json:"total_usd"`
	Exceeded       bool           `json:"exceeded"`
	UnpricedEvents int            `json:"unpriced_events"`
	UnpricedTokens int64          `json:"unpriced_tokens"`
	Results        []query.Result `json:"results"`
}

type DoctorPayload struct {
	GeneratedAt time.Time         `json:"generated_at"`
	Sources     []DoctorSource    `json:"sources"`
	Pricing     DoctorPricing     `json:"pricing"`
	Gemini      DoctorGeminiState `json:"gemini"`
}

type DoctorSource struct {
	Name        string     `json:"name"`
	Status      string     `json:"status"`
	Events      int        `json:"events"`
	LatestEvent *time.Time `json:"latest_event,omitempty"`
}

type DoctorPricing struct {
	PricedEvents   int   `json:"priced_events"`
	UnpricedEvents int   `json:"unpriced_events"`
	UnpricedModels int   `json:"unpriced_models"`
	UnpricedTokens int64 `json:"unpriced_tokens"`
}

type DoctorGeminiState struct {
	Configured     bool   `json:"configured"`
	SettingsPath   string `json:"settings_path"`
	Outfile        string `json:"outfile,omitempty"`
	LogPromptsSafe bool   `json:"log_prompts_safe"`
	Status         string `json:"status"`
}

// WritePricingAudit writes a pricing audit payload to w in the specified format.
// 
// WritePricingAudit supports three formats:
// - table (default/empty): renders a bordered table with columns TOOL, MODEL, PROVIDER, EVENTS, TOTAL and EXAMPLE; if Payload.Skeleton is non-empty it is printed after the table prefixed by "Suggested pricing skeleton:".
// - json: writes the payload as indented JSON.
// - markdown: writes a Markdown table with the same columns and, if Payload.Skeleton is non-empty, appends it in a fenced ```json``` code block.
// 
// For any other format the function returns an error indicating the unknown format.
func WritePricingAudit(w io.Writer, format string, payload PricingAuditPayload) error {
	switch format {
	case "", "table":
		headers := []string{"TOOL", "MODEL", "PROVIDER", "EVENTS", "TOTAL", "EXAMPLE"}
		rows := make([][]string, 0, len(payload.Unpriced))
		for _, item := range payload.Unpriced {
			rows = append(rows, []string{item.Tool, item.Model, item.Provider, fmt.Sprint(item.Events), fmt.Sprint(item.Usage.NormalizedTotal()), item.Example})
		}
		writeBorderedTable(w, headers, rows)
		if payload.Skeleton != "" {
			fmt.Fprintln(w, "\nSuggested pricing skeleton:")
			fmt.Fprint(w, payload.Skeleton)
		}
		return nil
	case "json":
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(payload)
	case "markdown":
		fmt.Fprintln(w, "| Tool | Model | Provider | Events | Total | Example |")
		fmt.Fprintln(w, "| --- | --- | --- | ---: | ---: | --- |")
		for _, item := range payload.Unpriced {
			fmt.Fprintf(w, "| %s | %s | %s | %d | %d | %s |\n",
				escapeMarkdown(item.Tool),
				escapeMarkdown(item.Model),
				escapeMarkdown(item.Provider),
				item.Events,
				item.Usage.NormalizedTotal(),
				escapeMarkdown(item.Example),
			)
		}
		if payload.Skeleton != "" {
			fmt.Fprintln(w, "\n```json")
			fmt.Fprint(w, strings.TrimSpace(payload.Skeleton))
			fmt.Fprintln(w, "\n```")
		}
		return nil
	default:
		return fmt.Errorf("unknown format %q", format)
	}
}

// WriteBudget writes a budget summary and detailed results to w in the requested format.
// It supports three formats: table (default/""), "json", and "markdown".
// In table mode it prints formatted USD totals, exceeded/unpriced flags, and a tabular listing of results.
// In JSON mode it encodes the full payload with two-space indentation.
// In markdown mode it emits a markdown table of the summary fields followed by markdown-formatted results.
// It returns any write/encoding error or an error if the format is unrecognized.
func WriteBudget(w io.Writer, format string, payload BudgetPayload) error {
	switch format {
	case "", "table":
		fmt.Fprintf(w, "Limit USD: %s\nTotal USD: %s\nExceeded: %t\nUnpriced events: %d\n\n", FormatUSD(payload.LimitUSD), FormatUSD(payload.TotalUSD), payload.Exceeded, payload.UnpricedEvents)
		return WriteTable(w, payload.Results)
	case "json":
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(payload)
	case "markdown":
		fmt.Fprintf(w, "| Limit USD | Total USD | Exceeded | Unpriced Events |\n")
		fmt.Fprintf(w, "| ---: | ---: | --- | ---: |\n")
		fmt.Fprintf(w, "| %s | %s | %t | %d |\n\n", FormatUSD(payload.LimitUSD), FormatUSD(payload.TotalUSD), payload.Exceeded, payload.UnpricedEvents)
		return WriteMarkdown(w, payload.Results)
	default:
		return fmt.Errorf("unknown format %q", format)
	}
}

// WriteDoctor writes the doctor report payload to w in the requested format.
// Supported formats are "" or "table" (tabular output), "json" (pretty-printed JSON), and "markdown" (markdown table).
// Table and markdown outputs include a sources table followed by pricing and Gemini summaries; each source's LatestEvent is formatted as RFC3339 when present.
// It returns an error for an unrecognized format or if JSON encoding fails.
func WriteDoctor(w io.Writer, format string, payload DoctorPayload) error {
	switch format {
	case "", "table":
		headers := []string{"SOURCE", "EVENTS", "LATEST_EVENT", "STATUS"}
		rows := make([][]string, 0, len(payload.Sources))
		for _, source := range payload.Sources {
			latest := ""
			if source.LatestEvent != nil {
				latest = source.LatestEvent.Format(time.RFC3339)
			}
			rows = append(rows, []string{source.Name, fmt.Sprint(source.Events), latest, source.Status})
		}
		writeBorderedTable(w, headers, rows)
		fmt.Fprintf(w, "Pricing: priced=%d unpriced=%d unpriced_models=%d unpriced_tokens=%d\n", payload.Pricing.PricedEvents, payload.Pricing.UnpricedEvents, payload.Pricing.UnpricedModels, payload.Pricing.UnpricedTokens)
		fmt.Fprintf(w, "Gemini: %s settings=%s outfile=%s log_prompts_safe=%t\n", payload.Gemini.Status, payload.Gemini.SettingsPath, payload.Gemini.Outfile, payload.Gemini.LogPromptsSafe)
		return nil
	case "json":
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(payload)
	case "markdown":
		fmt.Fprintln(w, "| Source | Events | Latest Event | Status |")
		fmt.Fprintln(w, "| --- | ---: | --- | --- |")
		for _, source := range payload.Sources {
			latest := ""
			if source.LatestEvent != nil {
				latest = source.LatestEvent.Format(time.RFC3339)
			}
			fmt.Fprintf(w, "| %s | %d | %s | %s |\n", escapeMarkdown(source.Name), source.Events, latest, escapeMarkdown(source.Status))
		}
		fmt.Fprintf(w, "\nPricing: priced=%d unpriced=%d unpriced_models=%d unpriced_tokens=%d\n", payload.Pricing.PricedEvents, payload.Pricing.UnpricedEvents, payload.Pricing.UnpricedModels, payload.Pricing.UnpricedTokens)
		fmt.Fprintf(w, "\nGemini: %s; settings=%s; outfile=%s; log_prompts_safe=%t\n", payload.Gemini.Status, payload.Gemini.SettingsPath, payload.Gemini.Outfile, payload.Gemini.LogPromptsSafe)
		return nil
	default:
		return fmt.Errorf("unknown format %q", format)
	}
}
