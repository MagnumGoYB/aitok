package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/sosbs/aitok/internal/query"
	"github.com/sosbs/aitok/internal/report"
	"github.com/sosbs/aitok/internal/setup"
	"github.com/sosbs/aitok/internal/sources"
	"github.com/sosbs/aitok/internal/tui"
	"github.com/spf13/cobra"
)

type App struct {
	Out io.Writer
	Err io.Writer
	Now func() time.Time
}

type flags struct {
	period    string
	format    string
	groupBy   string
	tools     []string
	models    []string
	providers []string
	cwd       string
	home      string
	dryRun    bool
}

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
	}
	root.PersistentFlags().StringVar(&f.home, "home", "", "home directory override")

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

	tuiCmd := &cobra.Command{
		Use:   "tui",
		Short: "Open the terminal dashboard",
		RunE: func(cmd *cobra.Command, args []string) error {
			payload, err := buildPayload(cmd.Context(), f, app.Now())
			if err != nil {
				return err
			}
			return tui.Run(app.Out, payload)
		},
	}
	addQueryFlags(tuiCmd, f)

	doctor := &cobra.Command{
		Use:   "doctor",
		Short: "Inspect local data source availability",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDoctor(cmd.Context(), app.Out, f.home)
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

	root.AddCommand(summary, reportCmd, tuiCmd, doctor, setupCmd)
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

func buildPayload(ctx context.Context, f *flags, now time.Time) (report.Payload, error) {
	period, err := query.ParsePeriod(f.period)
	if err != nil {
		return report.Payload{}, err
	}
	window := query.WindowFor(period, now, time.Local)
	opts := sources.Options{Home: resolveHome(f.home)}
	events, err := sources.Collect(ctx, sources.Defaults(opts))
	if err != nil {
		return report.Payload{}, err
	}
	results := query.Aggregate(events, window, query.Filters{
		Tools:     query.SplitCSV(f.tools),
		Models:    query.SplitCSV(f.models),
		Providers: query.SplitCSV(f.providers),
		CWD:       f.cwd,
	}, query.ParseGroupBy(f.groupBy))
	return report.Payload{GeneratedAt: now, Window: window, GroupBy: query.ParseGroupBy(f.groupBy), Results: results}, nil
}

func runDoctor(ctx context.Context, out io.Writer, home string) error {
	opts := sources.Options{Home: resolveHome(home)}
	for _, source := range sources.Defaults(opts) {
		events, err := source.Read(ctx)
		status := "ok"
		if err != nil {
			status = err.Error()
		}
		fmt.Fprintf(out, "%s\t%d events\t%s\n", source.Name(), len(events), status)
	}
	return nil
}

func resolveHome(home string) string {
	if home != "" {
		return home
	}
	return sources.DefaultOptions().Home
}
