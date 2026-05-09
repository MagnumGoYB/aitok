package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/MagnumGoYB/aitok/internal/buildinfo"
	"github.com/MagnumGoYB/aitok/internal/pricing"
	"github.com/MagnumGoYB/aitok/internal/query"
	"github.com/MagnumGoYB/aitok/internal/report"
	"github.com/MagnumGoYB/aitok/internal/setup"
	"github.com/MagnumGoYB/aitok/internal/sources"
	"github.com/MagnumGoYB/aitok/internal/tui"
	"github.com/MagnumGoYB/aitok/internal/usage"
	"github.com/spf13/cobra"
)

type App struct {
	Out          io.Writer
	Err          io.Writer
	Now          func() time.Time
	VersionCheck func(context.Context, VersionCheckOptions) error
	Update       func(context.Context, UpdateOptions) error
}

type VersionCheckOptions struct {
	Home string
	In   io.Reader
	Err  io.Writer
	Now  time.Time
}

type UpdateOptions struct {
	Home string
	In   io.Reader
	Out  io.Writer
	Err  io.Writer
	Now  time.Time
}

type flags struct {
	period         string
	format         string
	groupBy        string
	tools          []string
	models         []string
	providers      []string
	cwd            string
	home           string
	pricing        string
	lang           string
	renderTUI      bool
	dryRun         bool
	noVersionCheck bool
	version        bool
	limitUSD       float64
}

// New constructs the root Cobra command for the CLI using the provided App
// dependencies and default flag state.
//
// The returned command has sensible defaults for App I/O and time when those
// fields are nil, registers global flags (--home, --pricing, --no-version-check,
// -v/--version), wires the primary subcommands (summary, report, pricing audit,
// budget check, tui, doctor, version, update, setup gemini), and performs a
// low-frequency version check via App.VersionCheck unless explicitly disabled.
func New(app App) *cobra.Command {
	if app.Out == nil {
		app.Out = io.Discard
	}
	if app.Err == nil {
		app.Err = io.Discard
	}
	if app.Now == nil {
		app.Now = time.Now
	}
	f := &flags{format: "table", period: string(query.PeriodToday)}
	root := &cobra.Command{
		Use:           "aitok",
		Short:         "Offline token usage summaries for AI coding tools",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if f.version {
				fmt.Fprintln(app.Out, buildinfo.Version)
				return nil
			}
			return cmd.Help()
		},
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if app.VersionCheck == nil || f.noVersionCheck || f.version || skipsVersionCheck(cmd) {
				return nil
			}
			return app.VersionCheck(cmd.Context(), VersionCheckOptions{Home: resolveHome(f.home), In: os.Stdin, Err: app.Err, Now: app.Now()})
		},
	}
	root.SetOut(app.Out)
	root.SetErr(app.Err)
	root.PersistentFlags().StringVar(&f.home, "home", "", "home directory override")
	root.PersistentFlags().StringVar(&f.pricing, "pricing", "", "pricing JSON override")
	root.PersistentFlags().BoolVar(&f.noVersionCheck, "no-version-check", false, "skip the low-frequency update check")
	root.Flags().BoolVarP(&f.version, "version", "v", false, "print version")

	summary := &cobra.Command{
		Use:   "summary",
		Short: "Print a token usage summary",
		RunE: func(cmd *cobra.Command, args []string) error {
			payload, err := buildPayload(cmd.Context(), f, app.Now())
			if err != nil {
				return err
			}
			return report.Write(app.Out, f.format, payload)
		},
	}
	addQueryFlags(summary, f)

	reportCmd := &cobra.Command{
		Use:   "report",
		Short: "Generate a token usage report",
		RunE: func(cmd *cobra.Command, args []string) error {
			payload, err := buildPayload(cmd.Context(), f, app.Now())
			if err != nil {
				return err
			}
			return report.Write(app.Out, f.format, payload)
		},
	}
	addQueryFlags(reportCmd, f)

	pricingCmd := &cobra.Command{
		Use:   "pricing",
		Short: "Inspect offline pricing coverage",
	}
	pricingAudit := &cobra.Command{
		Use:   "audit",
		Short: "Report local usage events without matching prices",
		RunE: func(cmd *cobra.Command, args []string) error {
			payload, err := buildPricingAudit(cmd.Context(), f, app.Now())
			if err != nil {
				return err
			}
			return report.WritePricingAudit(app.Out, f.format, payload)
		},
	}
	addQueryFlags(pricingAudit, f)
	pricingCmd.AddCommand(pricingAudit)

	budgetCmd := &cobra.Command{
		Use:   "budget",
		Short: "Check local estimated usage cost against a budget",
	}
	budgetCheck := &cobra.Command{
		Use:   "check",
		Short: "Fail when estimated local usage cost exceeds a limit",
		RunE: func(cmd *cobra.Command, args []string) error {
			payload, err := buildBudgetCheck(cmd.Context(), f, app.Now())
			if err != nil {
				return err
			}
			if err := report.WriteBudget(app.Out, f.format, payload); err != nil {
				return err
			}
			if payload.UnpricedEvents > 0 {
				fmt.Fprintf(app.Err, "warning: %d events had no matching price; estimated cost may be low\n", payload.UnpricedEvents)
			}
			if payload.Exceeded {
				return budgetExceededError{Limit: payload.LimitUSD, Total: payload.TotalUSD}
			}
			return nil
		},
	}
	addQueryFlags(budgetCheck, f)
	budgetCheck.Flags().Float64Var(&f.limitUSD, "limit-usd", 0, "budget limit in USD; required and must be greater than 0")
	budgetCmd.AddCommand(budgetCheck)

	tuiCmd := &cobra.Command{
		Use:   "tui",
		Short: "Open the terminal dashboard",
		RunE: func(cmd *cobra.Command, args []string) error {
			payload, err := buildPayload(cmd.Context(), f, app.Now())
			if err != nil {
				return err
			}
			if f.renderTUI {
				fmt.Fprint(app.Out, tui.RenderWidthWithLanguage(payload, 140, tui.Language(f.lang)))
				return nil
			}
			refresh := func() (report.Payload, error) {
				return buildPayload(cmd.Context(), f, app.Now())
			}
			return tui.RunWithRefresh(app.Out, payload, tui.Language(f.lang), refresh)
		},
	}
	addQueryFlags(tuiCmd, f)
	tuiCmd.Flags().BoolVar(&f.renderTUI, "render", false, "render the dashboard once without starting the interactive TUI")
	tuiCmd.Flags().StringVar(&f.lang, "lang", "en", "TUI language: en or zh-CN")

	doctor := &cobra.Command{
		Use:   "doctor",
		Short: "Inspect local data source availability",
		RunE: func(cmd *cobra.Command, args []string) error {
			payload, err := buildDoctor(cmd.Context(), f, app.Now())
			if err != nil {
				return err
			}
			return report.WriteDoctor(app.Out, f.format, payload)
		},
	}
	doctor.Flags().StringVar(&f.format, "format", "table", "format: table, json, markdown")

	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print the aitok version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintln(app.Out, buildinfo.Version)
		},
	}

	updateCmd := &cobra.Command{
		Use:   "update",
		Short: "Check for and install the latest aitok version",
		RunE: func(cmd *cobra.Command, args []string) error {
			if app.Update == nil {
				fmt.Fprintln(app.Out, "No update provider configured.")
				return nil
			}
			return app.Update(cmd.Context(), UpdateOptions{Home: resolveHome(f.home), In: os.Stdin, Out: app.Out, Err: app.Err, Now: app.Now()})
		},
	}

	setupCmd := &cobra.Command{
		Use:   "setup",
		Short: "Configure local tool telemetry",
	}
	gemini := &cobra.Command{
		Use:   "gemini",
		Short: "Configure Gemini CLI local telemetry",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := setup.ConfigureGemini(resolveHome(f.home), f.dryRun)
			if err != nil {
				return err
			}
			if f.format == "json" {
				encoder := json.NewEncoder(app.Out)
				encoder.SetIndent("", "  ")
				return encoder.Encode(result)
			}
			fmt.Fprintf(app.Out, "Gemini settings: %s\nTelemetry outfile: %s\nDry run: %t\nChanged: %t\n", result.Path, result.Outfile, result.DryRun, result.Changed)
			if result.DryRun {
				fmt.Fprintln(app.Out, "\nProposed settings:")
				fmt.Fprint(app.Out, result.Content)
			}
			return nil
		},
	}
	gemini.Flags().BoolVar(&f.dryRun, "dry-run", false, "print settings without writing")
	gemini.Flags().StringVar(&f.format, "format", "table", "output format: table or json")
	setupCmd.AddCommand(gemini)

	root.AddCommand(summary, reportCmd, pricingCmd, budgetCmd, tuiCmd, doctor, versionCmd, updateCmd, setupCmd)
	return root
}

func addQueryFlags(cmd *cobra.Command, f *flags) {
	cmd.Flags().StringVar(&f.period, "period", string(query.PeriodToday), "period: today, yesterday, this-week, last-week, this-month")
	cmd.Flags().StringVar(&f.format, "format", "table", "format: table, json, markdown")
	cmd.Flags().StringVar(&f.groupBy, "group-by", "tool,model,provider", "comma-separated groups: tool,model,provider,day,cwd")
	cmd.Flags().StringArrayVar(&f.tools, "tool", nil, "filter by tool")
	cmd.Flags().StringArrayVar(&f.models, "model", nil, "filter by model")
	cmd.Flags().StringArrayVar(&f.providers, "provider", nil, "filter by provider/auth type")
	cmd.Flags().StringVar(&f.cwd, "cwd", "", "filter by cwd substring")
}

func skipsVersionCheck(cmd *cobra.Command) bool {
	switch cmd.Name() {
	case "version", "update":
		return true
	default:
		return false
	}
}

// buildPayload builds a report.Payload for the selected period and flags by loading pricing and streaming local usage events into an accumulator.
// It parses f.period, computes the time window, loads the pricing catalog, applies filters from f, groups results according to f.groupBy, and aggregates costed usage.
// Returns an error if period parsing, pricing loading, or iteration over local sources fails.
func buildPayload(ctx context.Context, f *flags, now time.Time) (report.Payload, error) {
	period, err := query.ParsePeriod(f.period)
	if err != nil {
		return report.Payload{}, err
	}
	window := query.WindowFor(period, now, time.Local)
	opts := sources.Options{Home: resolveHome(f.home)}
	catalog, err := loadPricing(f, opts.Home)
	if err != nil {
		return report.Payload{}, err
	}
	groupBy := query.ParseGroupBy(f.groupBy)
	acc := query.NewAccumulator(window, query.Filters{
		Tools:     query.SplitCSV(f.tools),
		Models:    query.SplitCSV(f.models),
		Providers: query.SplitCSV(f.providers),
		CWD:       f.cwd,
	}, groupBy, func(event usage.UsageEvent) query.Cost {
		return query.Cost{USD: catalog.CostFor(event).USD}
	})
	err = sources.ForEach(ctx, sources.Defaults(opts), func(event usage.UsageEvent) error {
		acc.Add(event)
		return nil
	})
	if err != nil {
		return report.Payload{}, err
	}
	return report.Payload{GeneratedAt: now, Window: window, GroupBy: groupBy, Results: acc.Results()}, nil
}

// loadPricing loads a pricing catalog for the CLI.
// If f.pricing is empty it delegates to pricing.Load(home).
// Otherwise it reads the file at f.pricing and unmarshals its JSON into a pricing.Catalog.
// Returns the catalog on success or an error from reading, unmarshalling, or pricing.Load.
func loadPricing(f *flags, home string) (pricing.Catalog, error) {
	if f.pricing == "" {
		return pricing.Load(home)
	}
	data, err := os.ReadFile(f.pricing)
	if err != nil {
		return pricing.Catalog{}, err
	}
	var catalog pricing.Catalog
	if err := json.Unmarshal(data, &catalog); err != nil {
		return pricing.Catalog{}, err
	}
	return catalog, nil
}

type budgetExceededError struct {
	Limit float64
	Total float64
}

func (e budgetExceededError) Error() string {
	return fmt.Sprintf("budget exceeded: total %s > limit %s", report.FormatUSD(e.Total), report.FormatUSD(e.Limit))
}

// buildBudgetCheck builds a budget payload that aggregates usage and cost over the configured period and indicates whether the USD limit is exceeded.
// It validates that f.limitUSD is greater than 0 and returns an error otherwise. The function parses the period, loads pricing, applies filters from f, streams matching events from local sources to an accumulator using the pricing catalog to compute USD costs, and counts events/tokens that lack pricing coverage.
// The returned report.BudgetPayload contains GeneratedAt, Window, LimitUSD, TotalUSD, Exceeded (`true` when TotalUSD > LimitUSD), UnpricedEvents, UnpricedTokens, and per-group Results. Errors from period parsing, pricing loading, or source iteration are propagated.
func buildBudgetCheck(ctx context.Context, f *flags, now time.Time) (report.BudgetPayload, error) {
	if f.limitUSD <= 0 {
		return report.BudgetPayload{}, fmt.Errorf("--limit-usd must be greater than 0")
	}
	period, err := query.ParsePeriod(f.period)
	if err != nil {
		return report.BudgetPayload{}, err
	}
	window := query.WindowFor(period, now, time.Local)
	opts := sources.Options{Home: resolveHome(f.home)}
	catalog, err := loadPricing(f, opts.Home)
	if err != nil {
		return report.BudgetPayload{}, err
	}
	groupBy := query.ParseGroupBy(f.groupBy)
	filters := query.Filters{Tools: query.SplitCSV(f.tools), Models: query.SplitCSV(f.models), Providers: query.SplitCSV(f.providers), CWD: f.cwd}
	acc := query.NewAccumulator(window, filters, groupBy, func(event usage.UsageEvent) query.Cost {
		return query.Cost{USD: catalog.CostFor(event).USD}
	})
	var unpriced unpricedPricingCount
	err = sources.ForEach(ctx, sources.Defaults(opts), func(event usage.UsageEvent) error {
		if !window.Contains(event.Timestamp) || !eventMatches(event, filters) {
			return nil
		}
		if !catalog.Covers(event) {
			unpriced.Events++
			unpriced.Tokens += event.Usage.NormalizedTotal()
		}
		acc.Add(event)
		return nil
	})
	if err != nil {
		return report.BudgetPayload{}, err
	}
	results := acc.Results()
	var total float64
	for _, result := range results {
		total += result.CostUSD
	}
	return report.BudgetPayload{
		GeneratedAt:    now,
		Window:         window,
		LimitUSD:       f.limitUSD,
		TotalUSD:       total,
		Exceeded:       total > f.limitUSD,
		UnpricedEvents: unpriced.Events,
		UnpricedTokens: unpriced.Tokens,
		Results:        results,
	}, nil
}

type unpricedPricingCount struct {
	Events int
	Tokens int64
}

// buildPricingAudit builds a pricing audit payload listing usage events in the selected window and filters that do not have a matching pricing entry, grouped by tool, model, and provider.
// The returned payload contains the generation time, the queried time window, a slice of unpriced results sorted by descending normalized token usage (ties ordered lexicographically by tool|model|provider), and a JSON-like pricing skeleton string.
// Errors may be returned from period parsing, pricing loading, or iterating local usage sources.
func buildPricingAudit(ctx context.Context, f *flags, now time.Time) (report.PricingAuditPayload, error) {
	period, err := query.ParsePeriod(f.period)
	if err != nil {
		return report.PricingAuditPayload{}, err
	}
	window := query.WindowFor(period, now, time.Local)
	opts := sources.Options{Home: resolveHome(f.home)}
	catalog, err := loadPricing(f, opts.Home)
	if err != nil {
		return report.PricingAuditPayload{}, err
	}
	filters := query.Filters{Tools: query.SplitCSV(f.tools), Models: query.SplitCSV(f.models), Providers: query.SplitCSV(f.providers), CWD: f.cwd}
	type bucket struct {
		item report.PricingAuditResult
	}
	buckets := map[string]*bucket{}
	err = sources.ForEach(ctx, sources.Defaults(opts), func(event usage.UsageEvent) error {
		if !window.Contains(event.Timestamp) || !eventMatches(event, filters) || catalog.Covers(event) {
			return nil
		}
		model := usage.Unknown(event.Model)
		provider := usage.Unknown(event.Provider)
		key := string(event.Tool) + "|" + model + "|" + provider
		if buckets[key] == nil {
			buckets[key] = &bucket{item: report.PricingAuditResult{Tool: string(event.Tool), Model: model, Provider: provider, Example: event.CWD}}
		}
		buckets[key].item.Events++
		buckets[key].item.Usage = buckets[key].item.Usage.Add(event.Usage)
		if buckets[key].item.Example == "" {
			buckets[key].item.Example = event.CWD
		}
		return nil
	})
	if err != nil {
		return report.PricingAuditPayload{}, err
	}
	unpriced := make([]report.PricingAuditResult, 0, len(buckets))
	for _, bucket := range buckets {
		unpriced = append(unpriced, bucket.item)
	}
	sort.Slice(unpriced, func(i, j int) bool {
		left := unpriced[i].Usage.NormalizedTotal()
		right := unpriced[j].Usage.NormalizedTotal()
		if left == right {
			return unpriced[i].Tool+"|"+unpriced[i].Model+"|"+unpriced[i].Provider < unpriced[j].Tool+"|"+unpriced[j].Model+"|"+unpriced[j].Provider
		}
		return left > right
	})
	return report.PricingAuditPayload{GeneratedAt: now, Window: window, Unpriced: unpriced, Skeleton: pricingSkeleton(unpriced)}, nil
}

// pricingSkeleton generates a JSON-like skeleton string representing model pricing entries
// for the pricing audit results.
//
// If items is empty, it returns an empty string.
func pricingSkeleton(items []report.PricingAuditResult) string {
	if len(items) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("{\n  \"models\": [\n")
	for i, item := range items {
		if i > 0 {
			b.WriteString(",\n")
		}
		fmt.Fprintf(&b, "    {\n      \"match\": %q,\n      \"provider\": %q,\n      \"input_usd_per_mtok\": 0,\n      \"output_usd_per_mtok\": 0,\n      \"cache_hit_usd_per_mtok\": 0,\n      \"cache_make_usd_per_mtok\": 0,\n      \"multiplier\": 1\n    }", item.Model, item.Provider)
	}
	b.WriteString("\n  ]\n}\n")
	return b.String()
}

// buildDoctor builds a diagnostic payload that inspects available local data sources and pricing coverage.
// The returned payload includes Gemini CLI telemetry state, per-source scan results (name, status, event counts, latest event), aggregated pricing statistics (priced and unpriced events/tokens) and the count of distinct unpriced model/tool/provider combinations.
// If pricing cannot be loaded this function returns an error; errors encountered while scanning individual sources are captured in each DoctorSource.Status and are not returned.
func buildDoctor(ctx context.Context, f *flags, now time.Time) (report.DoctorPayload, error) {
	opts := sources.Options{Home: resolveHome(f.home)}
	catalog, err := loadPricing(f, opts.Home)
	if err != nil {
		return report.DoctorPayload{}, err
	}
	var payload report.DoctorPayload
	payload.GeneratedAt = now
	payload.Gemini = inspectGemini(opts.Home)
	unpricedModels := map[string]struct{}{}
	for _, source := range sources.Defaults(opts) {
		result := report.DoctorSource{Name: string(source.Name()), Status: "ok"}
		err := source.Scan(ctx, func(event usage.UsageEvent) error {
			result.Events++
			if result.LatestEvent == nil || event.Timestamp.After(*result.LatestEvent) {
				ts := event.Timestamp
				result.LatestEvent = &ts
			}
			if catalog.Covers(event) {
				payload.Pricing.PricedEvents++
			} else {
				payload.Pricing.UnpricedEvents++
				payload.Pricing.UnpricedTokens += event.Usage.NormalizedTotal()
				unpricedModels[string(event.Tool)+"|"+usage.Unknown(event.Model)+"|"+usage.Unknown(event.Provider)] = struct{}{}
			}
			return nil
		})
		if err != nil {
			result.Status = err.Error()
		}
		payload.Sources = append(payload.Sources, result)
	}
	payload.Pricing.UnpricedModels = len(unpricedModels)
	return payload, nil
}

// "not configured", "settings parse error", "telemetry outfile missing", "logPrompts is not false", or "ok".
func inspectGemini(home string) report.DoctorGeminiState {
	settingsPath := filepath.Join(home, ".gemini", "settings.json")
	state := report.DoctorGeminiState{SettingsPath: settingsPath, Status: "not configured"}
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return state
	}
	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		state.Status = "settings parse error"
		return state
	}
	telemetry, _ := settings["telemetry"].(map[string]any)
	if telemetry == nil {
		return state
	}
	state.Configured = true
	state.Outfile = expandHomeForCLI(home, stringFromAny(telemetry["outfile"]))
	logPrompts, hasLogPrompts := telemetry["logPrompts"].(bool)
	state.LogPromptsSafe = hasLogPrompts && !logPrompts
	if state.Outfile == "" {
		state.Status = "telemetry outfile missing"
		return state
	}
	if !state.LogPromptsSafe {
		state.Status = "logPrompts is not false"
		return state
	}
	state.Status = "ok"
	return state
}

// expandHomeForCLI expands CLI-typed "~" path patterns relative to the provided home.
//
// If path is empty, it returns an empty string. If path is exactly "~", it returns
// home. If path begins with "~" followed by a path separator (e.g., "~/..."), it
// returns the result of joining home with the remainder of path. Otherwise it
// returns path unchanged.
func expandHomeForCLI(home, path string) string {
	if path == "" {
		return ""
	}
	if path == "~" {
		return home
	}
	if len(path) > 2 && path[0] == '~' && os.IsPathSeparator(path[1]) {
		return filepath.Join(home, path[2:])
	}
	return path
}

// stringFromAny returns the string value if the provided any is a string, otherwise the empty string.
func stringFromAny(value any) string {
	if out, ok := value.(string); ok {
		return out
	}
	return ""
}

// eventMatches reports whether the provided UsageEvent satisfies all criteria in filters.
// For each non-empty filter field the corresponding event value must match:
// - Tools, Models, Providers: must equal one of the filter entries (case-insensitive, trimmed comparison).
// - CWD: the event's CWD must contain the filter substring.
// Returns true only if all applicable filters match.
func eventMatches(event usage.UsageEvent, filters query.Filters) bool {
	if len(filters.Tools) > 0 && !containsString(filters.Tools, string(event.Tool)) {
		return false
	}
	if len(filters.Models) > 0 && !containsString(filters.Models, event.Model) {
		return false
	}
	if len(filters.Providers) > 0 && !containsString(filters.Providers, event.Provider) {
		return false
	}
	if filters.CWD != "" && !strings.Contains(event.CWD, filters.CWD) {
		return false
	}
	return true
}

// containsString reports whether target matches any entry in values after
// trimming leading and trailing whitespace and performing a case-insensitive
// comparison. It returns true on the first match, false otherwise.
func containsString(values []string, target string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), target) {
			return true
		}
	}
	return false
}

// IsBudgetExceeded reports whether the given error represents a budget threshold violation.
// It returns true if err is or wraps a budgetExceededError, false otherwise.
func IsBudgetExceeded(err error) bool {
	var budgetErr budgetExceededError
	return errors.As(err, &budgetErr)
}

// resolveHome returns the given home directory when non-empty; otherwise it returns the default home directory from sources.DefaultOptions().Home.
func resolveHome(home string) string {
	if home != "" {
		return home
	}
	return sources.DefaultOptions().Home
}
