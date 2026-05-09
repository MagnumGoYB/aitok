package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MagnumGoYB/aitok/internal/buildinfo"
)

func TestSummaryIntegrationJSON(t *testing.T) {
	home := t.TempDir()
	writeFixture(t, filepath.Join(home, ".codex", "sessions", "2026", "05", "08", "rollout.jsonl"),
		`{"type":"session_meta","timestamp":"2026-05-08T01:00:00Z","payload":{"model_provider":"openai","cwd":"/repo"}}`+"\n"+
			`{"type":"turn_context","timestamp":"2026-05-08T01:00:01Z","payload":{"model":"gpt-5.4","cwd":"/repo"}}`+"\n"+
			`{"type":"event_msg","timestamp":"2026-05-08T01:00:02Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":10,"output_tokens":2,"total_tokens":12}}}}`+"\n")
	var out bytes.Buffer
	cmd := New(App{Out: &out, Now: func() time.Time {
		return time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
	}})
	cmd.SetArgs([]string{"--home", home, "summary", "--period", "today", "--format", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"tool": "codex"`) || !strings.Contains(out.String(), `"requests": 1`) || !strings.Contains(out.String(), `"cost_usd"`) || !strings.Contains(out.String(), `"total": 12`) {
		t.Fatalf("unexpected output: %s", out.String())
	}
}

func TestAgentJSONSummaryKeepsStdoutMachineReadableAndStderrEmpty(t *testing.T) {
	home := t.TempDir()
	writeFixture(t, filepath.Join(home, ".codex", "sessions", "2026", "05", "08", "rollout.jsonl"),
		`{"type":"session_meta","timestamp":"2026-05-08T01:00:00Z","payload":{"model_provider":"openai","cwd":"/repo"}}`+"\n"+
			`{"type":"turn_context","timestamp":"2026-05-08T01:00:01Z","payload":{"model":"gpt-5.4","cwd":"/repo"}}`+"\n"+
			`{"type":"event_msg","timestamp":"2026-05-08T01:00:02Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":10,"output_tokens":2,"total_tokens":12}}}}`+"\n")
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd := New(App{
		Out: &out,
		Err: &stderr,
		VersionCheck: func(ctx context.Context, opts VersionCheckOptions) error {
			t.Fatal("agent commands should use --no-version-check")
			return nil
		},
		Now: func() time.Time {
			return time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
		},
	})
	cmd.SetArgs([]string{"--home", home, "--no-version-check", "summary", "--period", "today", "--format", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("agent JSON command should keep stderr empty on success: %s", stderr.String())
	}
	var payload struct {
		Results []struct {
			Key map[string]string `json:"key"`
		} `json:"results"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("stdout should be a complete JSON object: %v\n%s", err, out.String())
	}
	if len(payload.Results) != 1 || payload.Results[0].Key["tool"] != "codex" {
		t.Fatalf("stdout should be a JSON object: %s", out.String())
	}
}

func TestSummaryUsesCustomPricingFile(t *testing.T) {
	home := t.TempDir()
	pricingPath := filepath.Join(home, "pricing.json")
	writeFixture(t, pricingPath, `{"models":[{"match":"gpt-5.4","input_usd_per_mtok":2,"output_usd_per_mtok":20,"multiplier":2}]}`)
	writeFixture(t, filepath.Join(home, ".codex", "sessions", "2026", "05", "08", "rollout.jsonl"),
		`{"type":"turn_context","timestamp":"2026-05-08T01:00:01Z","payload":{"model":"gpt-5.4"}}`+"\n"+
			`{"type":"event_msg","timestamp":"2026-05-08T01:00:02Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":1000000,"output_tokens":1000000}}}}`+"\n")
	var out bytes.Buffer
	cmd := New(App{Out: &out, Now: func() time.Time {
		return time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
	}})
	cmd.SetArgs([]string{"--home", home, "--pricing", pricingPath, "summary", "--period", "today", "--format", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"cost_usd": 44`) {
		t.Fatalf("custom pricing not applied: %s", out.String())
	}
}

func TestTUIRenderCommandPrintsDashboard(t *testing.T) {
	home := t.TempDir()
	writeFixture(t, filepath.Join(home, ".codex", "sessions", "2026", "05", "08", "rollout.jsonl"),
		`{"type":"turn_context","timestamp":"2026-05-08T01:00:01Z","payload":{"model":"gpt-5.5"}}`+"\n"+
			`{"type":"event_msg","timestamp":"2026-05-08T01:00:02Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":1000000,"output_tokens":1000}}}}`+"\n")
	var out bytes.Buffer
	cmd := New(App{Out: &out, Now: func() time.Time {
		return time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
	}})
	cmd.SetArgs([]string{"--home", home, "tui", "--period", "today", "--render"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"Usage Dashboard", "Requests", "Estimated Cost", "Total Tokens", "Cached Tokens", "gpt-5.5"} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("render output missing %q: %s", expected, out.String())
		}
	}
	if strings.Contains(out.String(), "使用统计") {
		t.Fatalf("default TUI render should prefer English: %s", out.String())
	}
}

func TestTUIRenderCommandSupportsChinese(t *testing.T) {
	home := t.TempDir()
	writeFixture(t, filepath.Join(home, ".codex", "sessions", "2026", "05", "08", "rollout.jsonl"),
		`{"type":"turn_context","timestamp":"2026-05-08T01:00:01Z","payload":{"model":"gpt-5.5"}}`+"\n"+
			`{"type":"event_msg","timestamp":"2026-05-08T01:00:02Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":1000000,"output_tokens":1000}}}}`+"\n")
	var out bytes.Buffer
	cmd := New(App{Out: &out, Now: func() time.Time {
		return time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
	}})
	cmd.SetArgs([]string{"--home", home, "tui", "--period", "today", "--render", "--lang", "zh-CN"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"使用统计", "总请求数", "总成本", "总 Token 数", "缓存 Token", "gpt-5.5"} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("render output missing %q: %s", expected, out.String())
		}
	}
}

func TestSetupGeminiDryRunCommand(t *testing.T) {
	home := t.TempDir()
	var out bytes.Buffer
	cmd := New(App{Out: &out})
	cmd.SetArgs([]string{"--home", home, "setup", "gemini", "--dry-run"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Dry run: true") || !strings.Contains(out.String(), "logPrompts") {
		t.Fatalf("unexpected output: %s", out.String())
	}
}

func TestPricingAuditReportsUnpricedModels(t *testing.T) {
	home := t.TempDir()
	writeFixture(t, filepath.Join(home, ".codex", "sessions", "2026", "05", "08", "rollout.jsonl"),
		`{"type":"turn_context","timestamp":"2026-05-08T01:00:01Z","payload":{"model":"mystery-model","cwd":"/repo"}}`+"\n"+
			`{"type":"event_msg","timestamp":"2026-05-08T01:00:02Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":10,"output_tokens":2}}}}`+"\n")
	var out bytes.Buffer
	cmd := New(App{Out: &out, Now: func() time.Time {
		return time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
	}})
	cmd.SetArgs([]string{"--home", home, "pricing", "audit", "--period", "today", "--format", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"model": "mystery-model"`) || !strings.Contains(out.String(), `"skeleton"`) {
		t.Fatalf("pricing audit missing unpriced model and skeleton: %s", out.String())
	}
}

func TestPricingAuditOmitsKnownModels(t *testing.T) {
	home := t.TempDir()
	writeFixture(t, filepath.Join(home, ".codex", "sessions", "2026", "05", "08", "rollout.jsonl"),
		`{"type":"turn_context","timestamp":"2026-05-08T01:00:01Z","payload":{"model":"gpt-5.4","cwd":"/repo"}}`+"\n"+
			`{"type":"event_msg","timestamp":"2026-05-08T01:00:02Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":10,"output_tokens":2}}}}`+"\n")
	var out bytes.Buffer
	cmd := New(App{Out: &out, Now: func() time.Time {
		return time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
	}})
	cmd.SetArgs([]string{"--home", home, "pricing", "audit", "--period", "today", "--format", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"unpriced": []`) {
		t.Fatalf("known model should not appear in pricing audit: %s", out.String())
	}
}

func TestBudgetCheckFailsWhenLimitExceeded(t *testing.T) {
	home := t.TempDir()
	writeFixture(t, filepath.Join(home, ".codex", "sessions", "2026", "05", "08", "rollout.jsonl"),
		`{"type":"session_meta","timestamp":"2026-05-08T01:00:00Z","payload":{"model_provider":"openai","cwd":"/repo"}}`+"\n"+
			`{"type":"turn_context","timestamp":"2026-05-08T01:00:01Z","payload":{"model":"gpt-5.4","cwd":"/repo"}}`+"\n"+
			`{"type":"event_msg","timestamp":"2026-05-08T01:00:02Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":1000000,"output_tokens":1000000}}}}`+"\n")
	var out bytes.Buffer
	cmd := New(App{Out: &out, Now: func() time.Time {
		return time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
	}})
	cmd.SetArgs([]string{"--home", home, "budget", "check", "--period", "today", "--limit-usd", "1", "--format", "json"})
	err := cmd.Execute()
	if !IsBudgetExceeded(err) {
		t.Fatalf("err = %v, want budget exceeded", err)
	}
	if !strings.Contains(out.String(), `"exceeded": true`) || !strings.Contains(out.String(), `"limit_usd": 1`) {
		t.Fatalf("budget output missing exceeded payload: %s", out.String())
	}
}

func TestAgentBudgetExceededKeepsPayloadOnStdoutAndSummaryOnStderr(t *testing.T) {
	home := t.TempDir()
	writeFixture(t, filepath.Join(home, ".codex", "sessions", "2026", "05", "08", "rollout.jsonl"),
		`{"type":"session_meta","timestamp":"2026-05-08T01:00:00Z","payload":{"model_provider":"openai","cwd":"/repo"}}`+"\n"+
			`{"type":"turn_context","timestamp":"2026-05-08T01:00:01Z","payload":{"model":"gpt-5.4","cwd":"/repo"}}`+"\n"+
			`{"type":"event_msg","timestamp":"2026-05-08T01:00:02Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":1000000,"output_tokens":1000000}}}}`+"\n")
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd := New(App{Out: &out, Err: &stderr, Now: func() time.Time {
		return time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
	}})
	cmd.SetArgs([]string{"--home", home, "--no-version-check", "budget", "check", "--period", "today", "--limit-usd", "1", "--format", "json"})
	err := cmd.Execute()
	if !IsBudgetExceeded(err) {
		t.Fatalf("err = %v, want budget exceeded", err)
	}
	stdout := out.String()
	if !strings.HasPrefix(strings.TrimSpace(stdout), "{") || !strings.Contains(stdout, `"exceeded": true`) || !strings.Contains(stdout, `"unpriced_events"`) {
		t.Fatalf("stdout should keep complete budget JSON payload: %s", stdout)
	}
	if stderr.Len() != 0 {
		t.Fatalf("library command should not write stderr directly; main handles returned error: %s", stderr.String())
	}
}

func TestBudgetCheckRequiresPositiveLimit(t *testing.T) {
	cmd := New(App{Out: io.Discard})
	cmd.SetArgs([]string{"budget", "check", "--limit-usd", "0"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "--limit-usd must be greater than 0") {
		t.Fatalf("err = %v, want positive limit error", err)
	}
}

func TestDoctorReportsGeminiSafetyAndPricing(t *testing.T) {
	home := t.TempDir()
	writeFixture(t, filepath.Join(home, ".gemini", "settings.json"), `{"telemetry":{"enabled":true,"target":"local","outfile":"~/.gemini/telemetry.log","logPrompts":false}}`)
	writeFixture(t, filepath.Join(home, ".gemini", "telemetry.log"),
		`{"timestamp":"2026-05-08T01:00:00Z","name":"gemini_cli.api_response","attributes":{"model":"unknown-gemini","auth_type":"oauth","input_token_count":11,"output_token_count":5,"prompt_id":"p1"}}`+"\n")
	var out bytes.Buffer
	cmd := New(App{Out: &out, Now: func() time.Time {
		return time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
	}})
	cmd.SetArgs([]string{"--home", home, "doctor", "--format", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"log_prompts_safe": true`) || !strings.Contains(out.String(), `"unpriced_events": 1`) {
		t.Fatalf("doctor output missing gemini safety or pricing diagnostics: %s", out.String())
	}
}

func TestDoctorRequiresGeminiLocalTelemetryEnabled(t *testing.T) {
	tests := []struct {
		name     string
		settings string
		status   string
	}{
		{
			name:     "disabled",
			settings: `{"telemetry":{"enabled":false,"target":"local","outfile":"~/.gemini/telemetry.log","logPrompts":false}}`,
			status:   "telemetry disabled",
		},
		{
			name:     "not local",
			settings: `{"telemetry":{"enabled":true,"target":"gcp","outfile":"~/.gemini/telemetry.log","logPrompts":false}}`,
			status:   "telemetry target not local",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			writeFixture(t, filepath.Join(home, ".gemini", "settings.json"), tt.settings)
			state := inspectGemini(home)
			if state.Configured {
				t.Fatalf("configured = true, want false")
			}
			if state.Status != tt.status {
				t.Fatalf("status = %q, want %q", state.Status, tt.status)
			}
		})
	}
}

func TestVersionCheckRunsBeforeCommand(t *testing.T) {
	home := t.TempDir()
	var called bool
	cmd := New(App{
		Out: io.Discard,
		VersionCheck: func(ctx context.Context, opts VersionCheckOptions) error {
			called = true
			if opts.Home != home {
				t.Fatalf("home = %q, want %q", opts.Home, home)
			}
			return nil
		},
	})
	cmd.SetArgs([]string{"--home", home, "doctor"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("version check did not run")
	}
}

func TestVersionCheckRunsForRootHelp(t *testing.T) {
	home := t.TempDir()
	var called bool
	cmd := New(App{
		Out: io.Discard,
		VersionCheck: func(ctx context.Context, opts VersionCheckOptions) error {
			called = true
			if opts.Home != home {
				t.Fatalf("home = %q, want %q", opts.Home, home)
			}
			return nil
		},
	})
	cmd.SetArgs([]string{"--home", home})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("version check did not run for root command")
	}
}

func TestNoVersionCheckFlagSkipsVersionCheck(t *testing.T) {
	home := t.TempDir()
	cmd := New(App{
		Out: io.Discard,
		VersionCheck: func(ctx context.Context, opts VersionCheckOptions) error {
			t.Fatal("version check should be skipped")
			return nil
		},
	})
	cmd.SetArgs([]string{"--home", home, "--no-version-check", "doctor"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
}

func TestVersionCommandsPrintVersion(t *testing.T) {
	for _, args := range [][]string{{"version"}, {"-v"}, {"--version"}} {
		var out bytes.Buffer
		cmd := New(App{Out: &out})
		cmd.SetArgs(args)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		if got := strings.TrimSpace(out.String()); got != buildinfo.Version {
			t.Fatalf("%v output = %q, want %s", args, got, buildinfo.Version)
		}
	}
}

func TestVersionCommandSkipsVersionCheck(t *testing.T) {
	cmd := New(App{
		Out: io.Discard,
		VersionCheck: func(ctx context.Context, opts VersionCheckOptions) error {
			t.Fatal("version command should not run update check")
			return nil
		},
	})
	cmd.SetArgs([]string{"version"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateCommandRunsUpdater(t *testing.T) {
	home := t.TempDir()
	var called bool
	cmd := New(App{
		Out: io.Discard,
		Update: func(ctx context.Context, opts UpdateOptions) error {
			called = true
			if opts.Home != home {
				t.Fatalf("home = %q, want %q", opts.Home, home)
			}
			return nil
		},
	})
	cmd.SetArgs([]string{"--home", home, "update"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("update command did not run updater")
	}
}

func TestUpdateCommandSkipsVersionCheck(t *testing.T) {
	cmd := New(App{
		Out: io.Discard,
		VersionCheck: func(ctx context.Context, opts VersionCheckOptions) error {
			t.Fatal("update command should not run low-frequency version check")
			return nil
		},
		Update: func(ctx context.Context, opts UpdateOptions) error {
			return nil
		},
	})
	cmd.SetArgs([]string{"update"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
}

func writeFixture(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
