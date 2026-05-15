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
	"github.com/MagnumGoYB/aitok/internal/query"
	"github.com/MagnumGoYB/aitok/internal/report"
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

func TestSummaryJSONOmitsThreadsByDefaultAndIncludesWithFlag(t *testing.T) {
	home := t.TempDir()
	session := filepath.Join(home, ".codex", "sessions", "2026", "05", "08", "rollout-2026-05-08T01-00-00-019e0000-0000-7000-8000-000000000001.jsonl")
	writeFixture(t, session,
		`{"type":"session_meta","timestamp":"2026-05-08T01:00:00Z","payload":{"id":"thread-a","model_provider":"openai","cwd":"/repo"}}`+"\n"+
			`{"type":"response_item","timestamp":"2026-05-08T01:00:01Z","payload":{"type":"message","role":"user","content":"Fix login bug"}}`+"\n"+
			`{"type":"turn_context","timestamp":"2026-05-08T01:00:02Z","payload":{"id":"turn-a","model":"gpt-5.4","cwd":"/repo"}}`+"\n"+
			`{"type":"event_msg","timestamp":"2026-05-08T01:00:03Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":10,"output_tokens":2,"total_tokens":12}}}}`+"\n")
	now := func() time.Time { return time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC) }

	var out bytes.Buffer
	cmd := New(App{Out: &out, Now: now})
	cmd.SetArgs([]string{"--home", home, "--no-version-check", "summary", "--period", "today", "--format", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), `"threads"`) {
		t.Fatalf("summary JSON should omit threads by default: %s", out.String())
	}

	out.Reset()
	cmd = New(App{Out: &out, Now: now})
	cmd.SetArgs([]string{"--home", home, "--no-version-check", "summary", "--period", "today", "--format", "json", "--threads"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"threads"`) || !strings.Contains(out.String(), `"id": "thread-a"`) || !strings.Contains(out.String(), `"name": "Fix login bug"`) {
		t.Fatalf("summary --threads should include thread payload: %s", out.String())
	}
}

func TestSummarySortsModelUsageAndThreadsByCost(t *testing.T) {
	home := t.TempDir()
	pricingPath := filepath.Join(home, "pricing.json")
	writeFixture(t, pricingPath, `{"models":[{"match":"cheap","input_usd_per_mtok":1},{"match":"expensive","input_usd_per_mtok":100}]}`)
	writeFixture(t, filepath.Join(home, ".codex", "sessions", "2026", "05", "08", "cheap.jsonl"),
		`{"type":"session_meta","timestamp":"2026-05-08T01:00:00Z","payload":{"id":"cheap-thread","model_provider":"openai","cwd":"/repo"}}`+"\n"+
			`{"type":"turn_context","timestamp":"2026-05-08T01:00:01Z","payload":{"model":"cheap","cwd":"/repo"}}`+"\n"+
			`{"type":"event_msg","timestamp":"2026-05-08T01:00:02Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":1000000,"total_tokens":1000000}}}}`+"\n")
	writeFixture(t, filepath.Join(home, ".codex", "sessions", "2026", "05", "08", "expensive.jsonl"),
		`{"type":"session_meta","timestamp":"2026-05-08T02:00:00Z","payload":{"id":"expensive-thread","model_provider":"openai","cwd":"/repo"}}`+"\n"+
			`{"type":"turn_context","timestamp":"2026-05-08T02:00:01Z","payload":{"model":"expensive","cwd":"/repo"}}`+"\n"+
			`{"type":"event_msg","timestamp":"2026-05-08T02:00:02Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":100000,"total_tokens":100000}}}}`+"\n")
	var out bytes.Buffer
	cmd := New(App{Out: &out, Now: func() time.Time {
		return time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
	}})
	cmd.SetArgs([]string{"--home", home, "--pricing", pricingPath, "--no-version-check", "summary", "--period", "today", "--format", "json", "--threads", "--sort", "cost"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var payload report.Payload
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("summary --sort cost should emit valid JSON: %v\n%s", err, out.String())
	}
	if payload.SortBy != "cost" || payload.Results[0].Key["model"] != "expensive" || payload.Threads[0].ID != "expensive-thread" {
		t.Fatalf("summary should sort model usage and threads by cost desc: %+v", payload)
	}
}

func TestSummaryRejectsUnknownSortMetric(t *testing.T) {
	home := t.TempDir()
	var out bytes.Buffer
	cmd := New(App{Out: &out, Now: func() time.Time {
		return time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
	}})
	cmd.SetArgs([]string{"--home", home, "--no-version-check", "summary", "--sort", "requests"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "expected tokens or cost") {
		t.Fatalf("summary should reject unsupported sort metric, err=%v", err)
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
	if !strings.Contains(out.String(), `"source": "custom"`) || !strings.Contains(out.String(), `"input_usd_per_mtok": 2`) || !strings.Contains(out.String(), `"output_usd_per_mtok": 20`) {
		t.Fatalf("custom pricing details not included: %s", out.String())
	}
}

func TestSummaryShowsDifferentPricesForModelProviderPairs(t *testing.T) {
	home := t.TempDir()
	writeFixture(t, filepath.Join(home, ".aitok", "pricing.json"), `{"models":[{"match":"gpt-5.4","provider":"team-a","input_usd_per_mtok":2,"output_usd_per_mtok":20,"cache_hit_usd_per_mtok":0.2,"cache_make_usd_per_mtok":2}]}`)
	writeFixture(t, filepath.Join(home, ".codex", "sessions", "2026", "05", "08", "team-a.jsonl"),
		`{"type":"session_meta","timestamp":"2026-05-08T01:00:00Z","payload":{"model_provider":"team-a","cwd":"/repo"}}`+"\n"+
			`{"type":"turn_context","timestamp":"2026-05-08T01:00:01Z","payload":{"model":"gpt-5.4","cwd":"/repo"}}`+"\n"+
			`{"type":"event_msg","timestamp":"2026-05-08T01:00:02Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":1000000,"total_tokens":1000000}}}}`+"\n")
	writeFixture(t, filepath.Join(home, ".codex", "sessions", "2026", "05", "08", "openai.jsonl"),
		`{"type":"session_meta","timestamp":"2026-05-08T02:00:00Z","payload":{"model_provider":"openai","cwd":"/repo"}}`+"\n"+
			`{"type":"turn_context","timestamp":"2026-05-08T02:00:01Z","payload":{"model":"gpt-5.4","cwd":"/repo"}}`+"\n"+
			`{"type":"event_msg","timestamp":"2026-05-08T02:00:02Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":1000000,"total_tokens":1000000}}}}`+"\n")
	var out bytes.Buffer
	cmd := New(App{Out: &out, Now: func() time.Time {
		return time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
	}})
	cmd.SetArgs([]string{"--home", home, "--no-version-check", "summary", "--period", "today", "--format", "json", "--group-by", "model,provider,day"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var payload report.Payload
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("summary json should be valid: %v\n%s", err, out.String())
	}
	if len(payload.Results) != 2 {
		t.Fatalf("len(results) = %d, want 2: %+v", len(payload.Results), payload.Results)
	}
	byProvider := map[string]struct {
		source string
		input  float64
	}{}
	for _, result := range payload.Results {
		if result.Price == nil {
			t.Fatalf("result missing price details: %+v", result)
		}
		byProvider[result.Key["provider"]] = struct {
			source string
			input  float64
		}{source: result.Price.Source, input: result.Price.InputUSDPerMTok}
	}
	if byProvider["team-a"].source != "custom" || byProvider["team-a"].input != 2 {
		t.Fatalf("team-a should use custom pricing: %+v", byProvider)
	}
	if byProvider["openai"].source != "official" || byProvider["openai"].input != 2.5 {
		t.Fatalf("openai should use official pricing: %+v", byProvider)
	}
}

func TestSummaryPricesProviderSwitchesWithinSameCodexSession(t *testing.T) {
	home := t.TempDir()
	writeFixture(t, filepath.Join(home, ".aitok", "pricing.json"), `{"models":[{"match":"gpt-5.4","provider":"team-a","input_usd_per_mtok":2},{"match":"gpt-5.4","provider":"team-b","input_usd_per_mtok":8}]}`)
	writeFixture(t, filepath.Join(home, ".codex", "sessions", "2026", "05", "08", "mixed-provider.jsonl"),
		`{"type":"session_meta","timestamp":"2026-05-08T01:00:00Z","payload":{"model_provider":"team-a","cwd":"/repo"}}`+"\n"+
			`{"type":"turn_context","timestamp":"2026-05-08T01:00:01Z","payload":{"id":"turn-a","model":"team-a/gpt-5.4","cwd":"/repo"}}`+"\n"+
			`{"type":"event_msg","timestamp":"2026-05-08T01:00:02Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":1000000,"total_tokens":1000000}}}}`+"\n"+
			`{"type":"turn_context","timestamp":"2026-05-08T02:00:01Z","payload":{"id":"turn-b","model":"team-b/gpt-5.4","cwd":"/repo"}}`+"\n"+
			`{"type":"event_msg","timestamp":"2026-05-08T02:00:02Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":1000000,"total_tokens":1000000}}}}`+"\n")
	var out bytes.Buffer
	cmd := New(App{Out: &out, Now: func() time.Time {
		return time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
	}})
	cmd.SetArgs([]string{"--home", home, "--no-version-check", "summary", "--period", "today", "--format", "json", "--group-by", "model,provider,day"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var payload report.Payload
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("summary json should be valid: %v\n%s", err, out.String())
	}
	byProvider := map[string]float64{}
	for _, result := range payload.Results {
		byProvider[result.Key["provider"]] = result.CostUSD
	}
	if len(byProvider) != 2 || byProvider["team-a"] != 2 || byProvider["team-b"] != 8 {
		t.Fatalf("provider-switched session should price each event by its active provider: %+v\n%s", byProvider, out.String())
	}
}

func TestSummaryModelUsageUsesThreadBaselineForMixedProviderThreads(t *testing.T) {
	home := t.TempDir()
	writeFixture(t, filepath.Join(home, ".aitok", "pricing.json"), `{"models":[{"match":"gpt-5.4","provider":"team-a","input_usd_per_mtok":10},{"match":"gpt-5.4","provider":"team-b","input_usd_per_mtok":100}]}`)
	writeFixture(t, filepath.Join(home, ".codex", "sessions", "2026", "05", "08", "mixed-provider-thread.jsonl"),
		`{"type":"session_meta","timestamp":"2026-05-08T01:00:00Z","payload":{"id":"thread-a","model_provider":"team-a","cwd":"/repo"}}`+"\n"+
			`{"type":"turn_context","timestamp":"2026-05-08T01:00:01Z","payload":{"id":"turn-a","model":"team-a/gpt-5.4","cwd":"/repo"}}`+"\n"+
			`{"type":"event_msg","timestamp":"2026-05-08T01:00:02Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":1000000,"total_tokens":1000000}}}}`+"\n"+
			`{"type":"turn_context","timestamp":"2026-05-08T02:00:01Z","payload":{"id":"turn-b","model":"team-b/gpt-5.4","cwd":"/repo"}}`+"\n"+
			`{"type":"event_msg","timestamp":"2026-05-08T02:00:02Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":100000,"total_tokens":100000}}}}`+"\n")
	var out bytes.Buffer
	cmd := New(App{Out: &out, Now: func() time.Time {
		return time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
	}})
	cmd.SetArgs([]string{"--home", home, "--no-version-check", "summary", "--period", "today", "--format", "json", "--threads"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var payload report.Payload
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("summary json should be valid: %v\n%s", err, out.String())
	}
	if len(payload.Threads) != 1 {
		t.Fatalf("len(threads) = %d, want 1: %+v", len(payload.Threads), payload.Threads)
	}
	thread := payload.Threads[0]
	if thread.Provider != "team-a,team-b" || thread.Usage.NormalizedTotal() != 1_100_000 || thread.CostUSD != 20 {
		t.Fatalf("thread baseline mismatch: %+v", thread)
	}
	if len(thread.CostBreakdown) != 2 || thread.CostBreakdown[0].Provider != "team-a" || thread.CostBreakdown[0].USD != 10 || thread.CostBreakdown[1].Provider != "team-b" || thread.CostBreakdown[1].USD != 10 {
		t.Fatalf("unexpected thread cost breakdown: %+v", thread.CostBreakdown)
	}
	if len(thread.AttributionBreakdown) != 2 {
		t.Fatalf("unexpected thread attribution breakdown: %+v", thread.AttributionBreakdown)
	}
	if thread.AttributionBreakdown[0].Provider != "team-a" || len(thread.AttributionBreakdown[0].BySource) != 1 || thread.AttributionBreakdown[0].BySource[0].Source != "model" || thread.AttributionBreakdown[0].BySource[0].USD != 10 {
		t.Fatalf("unexpected team-a attribution breakdown: %+v", thread.AttributionBreakdown[0])
	}
	if thread.AttributionBreakdown[1].Provider != "team-b" || len(thread.AttributionBreakdown[1].BySource) != 1 || thread.AttributionBreakdown[1].BySource[0].Source != "model" || thread.AttributionBreakdown[1].BySource[0].USD != 10 {
		t.Fatalf("unexpected team-b attribution breakdown: %+v", thread.AttributionBreakdown[1])
	}
	if len(thread.Turns) != 2 {
		t.Fatalf("len(turns) = %d, want 2: %+v", len(thread.Turns), thread.Turns)
	}
	if thread.Turns[0].ID != "turn-a" || thread.Turns[0].Provider != "team-a" || thread.Turns[0].ProviderAttribution != "model" || thread.Turns[0].CostUSD != 10 {
		t.Fatalf("unexpected first turn: %+v", thread.Turns[0])
	}
	if thread.Turns[1].ID != "turn-b" || thread.Turns[1].Provider != "team-b" || thread.Turns[1].ProviderAttribution != "model" || thread.Turns[1].CostUSD != 10 {
		t.Fatalf("unexpected second turn: %+v", thread.Turns[1])
	}
	if len(payload.Results) != 2 {
		t.Fatalf("len(results) = %d, want 2: %+v", len(payload.Results), payload.Results)
	}
	byProviderResult := map[string]query.Result{}
	for _, result := range payload.Results {
		byProviderResult[result.Key["provider"]] = result
	}
	if byProviderResult["team-a"].Usage.NormalizedTotal() != 1_000_000 || byProviderResult["team-a"].CostUSD != 10 {
		t.Fatalf("team-a model usage mismatch: %+v\n%s", byProviderResult["team-a"], out.String())
	}
	if byProviderResult["team-b"].Usage.NormalizedTotal() != 100_000 || byProviderResult["team-b"].CostUSD != 10 {
		t.Fatalf("team-b model usage mismatch: %+v\n%s", byProviderResult["team-b"], out.String())
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

func TestTUIRenderThreadsRespectPeriodWindow(t *testing.T) {
	home := t.TempDir()
	writeFixture(t, filepath.Join(home, ".codex", "sessions", "2026", "05", "08", "today.jsonl"),
		`{"type":"session_meta","timestamp":"2026-05-08T01:00:00Z","payload":{"id":"today-thread","model_provider":"openai","cwd":"/repo"}}`+"\n"+
			`{"type":"response_item","timestamp":"2026-05-08T01:00:01Z","payload":{"type":"message","role":"user","content":"Today thread"}}`+"\n"+
			`{"type":"turn_context","timestamp":"2026-05-08T01:00:02Z","payload":{"model":"gpt-5.5","cwd":"/repo"}}`+"\n"+
			`{"type":"event_msg","timestamp":"2026-05-08T01:00:03Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":10,"output_tokens":2,"total_tokens":12}}}}`+"\n")
	writeFixture(t, filepath.Join(home, ".codex", "sessions", "2026", "05", "07", "old.jsonl"),
		`{"type":"session_meta","timestamp":"2026-05-07T01:00:00Z","payload":{"id":"old-thread","model_provider":"openai","cwd":"/repo"}}`+"\n"+
			`{"type":"response_item","timestamp":"2026-05-07T01:00:01Z","payload":{"type":"message","role":"user","content":"Old thread"}}`+"\n"+
			`{"type":"turn_context","timestamp":"2026-05-07T01:00:02Z","payload":{"model":"gpt-5.5","cwd":"/repo"}}`+"\n"+
			`{"type":"event_msg","timestamp":"2026-05-07T01:00:03Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":20,"output_tokens":4,"total_tokens":24}}}}`+"\n")

	var out bytes.Buffer
	cmd := New(App{Out: &out, Now: func() time.Time {
		return time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
	}})
	cmd.SetArgs([]string{"--home", home, "--no-version-check", "tui", "--period", "today", "--render"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "Today thread") || strings.Contains(got, "Old thread") || strings.Contains(got, "old-thread") {
		t.Fatalf("tui threads should respect the requested period window: %s", got)
	}
}

func TestSummaryThreadsKeepSingleRowForSameThreadWithModelAndProviderLists(t *testing.T) {
	home := t.TempDir()
	writeFixture(t, filepath.Join(home, ".codex", "sessions", "2026", "05", "08", "mixed.jsonl"),
		`{"type":"session_meta","timestamp":"2026-05-08T01:00:00Z","payload":{"id":"thread-a","model_provider":"bcb","cwd":"/repo"}}`+"\n"+
			`{"type":"custom-title","timestamp":"2026-05-08T01:00:00Z","customTitle":"Mixed provider thread"}`+"\n"+
			`{"type":"turn_context","timestamp":"2026-05-08T01:00:01Z","payload":{"id":"turn-a","model":"gpt-5.5","cwd":"/repo"}}`+"\n"+
			`{"type":"event_msg","timestamp":"2026-05-08T01:00:02Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":100,"total_tokens":100}}}}`+"\n"+
			`{"type":"session_meta","timestamp":"2026-05-08T02:00:00Z","payload":{"id":"thread-a","model_provider":"openai","cwd":"/repo"}}`+"\n"+
			`{"type":"turn_context","timestamp":"2026-05-08T02:00:01Z","payload":{"id":"turn-b","model":"gpt-5.4","cwd":"/repo"}}`+"\n"+
			`{"type":"event_msg","timestamp":"2026-05-08T02:00:02Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":10,"total_tokens":10}}}}`+"\n")
	var out bytes.Buffer
	cmd := New(App{Out: &out, Now: func() time.Time {
		return time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
	}})
	cmd.SetArgs([]string{"--home", home, "--no-version-check", "summary", "--period", "today", "--format", "json", "--threads"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var payload report.Payload
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("summary --threads should emit valid JSON: %v\n%s", err, out.String())
	}
	if len(payload.Threads) != 1 {
		t.Fatalf("summary --threads should keep a single row per thread id: %+v", payload.Threads)
	}
	thread := payload.Threads[0]
	if thread.ID != "thread-a" || thread.Model != "gpt-5.4,gpt-5.5" || thread.Provider != "bcb,openai" || thread.Usage.NormalizedTotal() != 110 {
		t.Fatalf("summary --threads should keep one row and summarize models/providers: %+v", thread)
	}
}

func TestSummaryTableUsesCompactDefaultAndFullExpands(t *testing.T) {
	home := t.TempDir()
	writeFixture(t, filepath.Join(home, ".codex", "sessions", "2026", "05", "08", "rollout.jsonl"),
		`{"type":"session_meta","timestamp":"2026-05-08T01:00:00Z","payload":{"model_provider":"openai","cwd":"/repo"}}`+"\n"+
			`{"type":"turn_context","timestamp":"2026-05-08T01:00:01Z","payload":{"model":"gpt-5.4","cwd":"/repo"}}`+"\n"+
			`{"type":"event_msg","timestamp":"2026-05-08T01:00:02Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":10,"output_tokens":2,"total_tokens":12}}}}`+"\n")
	var out bytes.Buffer
	cmd := New(App{Out: &out, Now: func() time.Time {
		return time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
	}})
	cmd.SetArgs([]string{"--home", home, "--no-version-check", "summary", "--period", "today"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	compact := out.String()
	if !strings.Contains(compact, "REQ") || strings.Contains(compact, "EVENTS") || strings.Contains(compact, "CACHE_CREATE") {
		t.Fatalf("summary default table should stay compact: %s", compact)
	}

	out.Reset()
	cmd = New(App{Out: &out, Now: func() time.Time {
		return time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
	}})
	cmd.SetArgs([]string{"--home", home, "--no-version-check", "summary", "--period", "today", "--full"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	full := out.String()
	if !strings.Contains(full, "EVENTS") || !strings.Contains(full, "CACHE_CREATE") {
		t.Fatalf("summary --full should expand columns: %s", full)
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

func TestPricingConfigureCommandWritesInteractiveOverride(t *testing.T) {
	home := t.TempDir()
	var out bytes.Buffer
	cmd := New(App{
		Out: &out,
		In:  strings.NewReader("gpt-5.4\nteam-a\n2\n20\n0.2\n2.5\n\n1.25\n"),
	})
	cmd.SetArgs([]string{"--home", home, "pricing", "configure"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Q1: Which model should this price match?") || !strings.Contains(out.String(), "A: ") || !strings.Contains(out.String(), "Pricing override saved") || !strings.Contains(out.String(), "Provider/auth label: team-a") {
		t.Fatalf("interactive configure output missing save summary: %s", out.String())
	}
	data, err := os.ReadFile(filepath.Join(home, ".aitok", "pricing.json"))
	if err != nil {
		t.Fatal(err)
	}
	if got := string(data); !strings.Contains(got, `"match": "gpt-5.4"`) || !strings.Contains(got, `"provider": "team-a"`) || !strings.Contains(got, `"multiplier": 1.25`) {
		t.Fatalf("pricing config not written correctly: %s", got)
	}
}

func TestPricingConfigureCommandSupportsFlagOnlyJSON(t *testing.T) {
	home := t.TempDir()
	var out bytes.Buffer
	cmd := New(App{Out: &out, In: strings.NewReader("")})
	cmd.SetArgs([]string{
		"--home", home,
		"pricing", "configure",
		"--model", "gpt-5.4",
		"--provider", "team-b",
		"--input-usd-per-mtok", "7",
		"--output-usd-per-mtok", "70",
		"--cache-hit-usd-per-mtok", "0.7",
		"--cache-make-usd-per-mtok", "7",
		"--cache-make-1h-usd-per-mtok", "8",
		"--multiplier", "1",
		"--format", "json",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "Q1:") {
		t.Fatalf("flag-only json configure should not print prompts: %s", out.String())
	}
	var payload struct {
		Path  string `json:"path"`
		Price struct {
			Match    string `json:"match"`
			Provider string `json:"provider"`
		} `json:"price"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("configure json should be valid: %v\n%s", err, out.String())
	}
	if payload.Path == "" || payload.Price.Match != "gpt-5.4" || payload.Price.Provider != "team-b" {
		t.Fatalf("unexpected configure payload: %+v", payload)
	}
}

func TestPricingConfigureJSONKeepsPromptsOffStdout(t *testing.T) {
	home := t.TempDir()
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd := New(App{
		Out: &out,
		Err: &stderr,
		In:  strings.NewReader("gpt-5.4\nteam-json\n2\n20\n0.2\n2.5\n\n1\n"),
	})
	cmd.SetArgs([]string{"--home", home, "pricing", "configure", "--format", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "Q1:") {
		t.Fatalf("json stdout should not contain prompts: %s", out.String())
	}
	if !strings.Contains(stderr.String(), "Q1: Which model should this price match?") {
		t.Fatalf("interactive prompts should be written to stderr for json mode: %s", stderr.String())
	}
	var payload struct {
		Price struct {
			Provider string `json:"provider"`
		} `json:"price"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("json stdout should remain machine-readable: %v\n%s", err, out.String())
	}
	if payload.Price.Provider != "team-json" {
		t.Fatalf("unexpected json payload: %s", out.String())
	}
}

func TestPricingConfigureRejectsRawAPIKeyProvider(t *testing.T) {
	tests := []string{
		"sk-test-secret",
		"AIzaSyD8Qw9eRtYuIoPaSdFgHjKlZxCvBnMq12",
	}
	for _, provider := range tests {
		t.Run(provider[:12], func(t *testing.T) {
			home := t.TempDir()
			var out bytes.Buffer
			cmd := New(App{Out: &out, In: strings.NewReader("")})
			cmd.SetArgs([]string{
				"--home", home,
				"pricing", "configure",
				"--model", "gpt-5.4",
				"--provider", provider,
				"--input-usd-per-mtok", "1",
				"--output-usd-per-mtok", "1",
				"--cache-hit-usd-per-mtok", "0",
				"--cache-make-usd-per-mtok", "1",
				"--cache-make-1h-usd-per-mtok", "1",
				"--multiplier", "1",
				"--format", "json",
			})
			err := cmd.Execute()
			if err == nil || !strings.Contains(err.Error(), "not a raw API key") {
				t.Fatalf("err = %v, want raw API key rejection", err)
			}
			if out.Len() != 0 || strings.Contains(out.String(), provider) {
				t.Fatalf("raw API key must not be echoed to JSON stdout: %s", out.String())
			}
			if _, statErr := os.Stat(filepath.Join(home, ".aitok", "pricing.json")); !os.IsNotExist(statErr) {
				t.Fatalf("raw API key rejection must not write pricing config, stat err=%v", statErr)
			}
		})
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
