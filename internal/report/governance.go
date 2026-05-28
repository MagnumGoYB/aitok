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
	Currency       string         `json:"currency,omitempty"`
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
		if err := writeBorderedTable(w, headers, rows); err != nil {
			return err
		}
		if payload.Skeleton != "" {
			if _, err := fmt.Fprintln(w, "\nSuggested pricing skeleton:"); err != nil {
				return err
			}
			if _, err := fmt.Fprint(w, payload.Skeleton); err != nil {
				return err
			}
		}
		return nil
	case "json":
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(payload)
	case "markdown":
		if _, err := fmt.Fprintln(w, "| Tool | Model | Provider | Events | Total | Example |"); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w, "| --- | --- | --- | ---: | ---: | --- |"); err != nil {
			return err
		}
		for _, item := range payload.Unpriced {
			if _, err := fmt.Fprintf(w, "| %s | %s | %s | %d | %d | %s |\n",
				escapeMarkdown(item.Tool),
				escapeMarkdown(item.Model),
				escapeMarkdown(item.Provider),
				item.Events,
				item.Usage.NormalizedTotal(),
				escapeMarkdown(item.Example),
			); err != nil {
				return err
			}
		}
		if payload.Skeleton != "" {
			if _, err := fmt.Fprintln(w, "\n```json"); err != nil {
				return err
			}
			if _, err := fmt.Fprint(w, strings.TrimSpace(payload.Skeleton)); err != nil {
				return err
			}
			if _, err := fmt.Fprintln(w, "\n```"); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("unknown format %q", format)
	}
}

func WriteBudget(w io.Writer, format string, payload BudgetPayload) error {
	limitLabel := budgetCurrencyLabel(payload.Currency)
	switch format {
	case "", "table":
		if _, err := fmt.Fprintf(w, "Limit %s: %s\nTotal %s: %s\nExceeded: %t\nUnpriced events: %d\n\n", limitLabel, FormatCost(payload.LimitUSD, payload.Currency), limitLabel, FormatCost(payload.TotalUSD, payload.Currency), payload.Exceeded, payload.UnpricedEvents); err != nil {
			return err
		}
		return WriteTable(w, payload.Results)
	case "json":
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(payload)
	case "markdown":
		if _, err := fmt.Fprintf(w, "| Limit %s | Total %s | Exceeded | Unpriced Events |\n", limitLabel, limitLabel); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "| ---: | ---: | --- | ---: |\n"); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "| %s | %s | %t | %d |\n\n", FormatCost(payload.LimitUSD, payload.Currency), FormatCost(payload.TotalUSD, payload.Currency), payload.Exceeded, payload.UnpricedEvents); err != nil {
			return err
		}
		return WriteMarkdown(w, payload.Results)
	default:
		return fmt.Errorf("unknown format %q", format)
	}
}

func budgetCurrencyLabel(currency string) string {
	switch strings.ToUpper(currency) {
	case "CNY", "RMB":
		return "CNY"
	default:
		return "USD"
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
		if err := writeBorderedTable(w, headers, rows); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "Pricing: priced=%d unpriced=%d unpriced_models=%d unpriced_tokens=%d\n", payload.Pricing.PricedEvents, payload.Pricing.UnpricedEvents, payload.Pricing.UnpricedModels, payload.Pricing.UnpricedTokens); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "Gemini: %s settings=%s outfile=%s log_prompts_safe=%t\n", payload.Gemini.Status, payload.Gemini.SettingsPath, payload.Gemini.Outfile, payload.Gemini.LogPromptsSafe); err != nil {
			return err
		}
		return nil
	case "json":
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(payload)
	case "markdown":
		if _, err := fmt.Fprintln(w, "| Source | Events | Latest Event | Status |"); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w, "| --- | ---: | --- | --- |"); err != nil {
			return err
		}
		for _, source := range payload.Sources {
			latest := ""
			if source.LatestEvent != nil {
				latest = source.LatestEvent.Format(time.RFC3339)
			}
			if _, err := fmt.Fprintf(w, "| %s | %d | %s | %s |\n", escapeMarkdown(source.Name), source.Events, latest, escapeMarkdown(source.Status)); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(w, "\nPricing: priced=%d unpriced=%d unpriced_models=%d unpriced_tokens=%d\n", payload.Pricing.PricedEvents, payload.Pricing.UnpricedEvents, payload.Pricing.UnpricedModels, payload.Pricing.UnpricedTokens); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "\nGemini: %s; settings=%s; outfile=%s; log_prompts_safe=%t\n", payload.Gemini.Status, payload.Gemini.SettingsPath, payload.Gemini.Outfile, payload.Gemini.LogPromptsSafe); err != nil {
			return err
		}
		return nil
	default:
		return fmt.Errorf("unknown format %q", format)
	}
}
