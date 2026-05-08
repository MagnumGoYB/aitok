package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

func writeFixture(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
