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
