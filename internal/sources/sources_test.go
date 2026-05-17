package sources

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/MagnumGoYB/aitok/internal/usage"
)

func TestClaudeReadsJSONLAndDeduplicates(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".claude", "projects", "repo")
	mustMkdir(t, dir)
	line := `{"type":"assistant","uuid":"same","timestamp":"2026-05-08T01:02:03Z","cwd":"/repo","message":{"id":"msg_same","model":"anthropic/claude-sonnet-4.5","stop_reason":"end_turn","usage":{"input_tokens":10,"output_tokens":2,"cache_read_input_tokens":3,"cache_creation_input_tokens":4,"cache_creation":{"ephemeral_5m_input_tokens":1,"ephemeral_1h_input_tokens":3}}}}`
	mustWrite(t, filepath.Join(dir, "session.jsonl"), line+"\n"+line+"\n"+"{bad json}\n")
	events, err := NewClaude(Options{Home: home}).Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(events))
	}
	if events[0].Model != "claude-sonnet-4.5" || events[0].Usage.Input != 10 || events[0].Usage.CachedInput != 3 || events[0].Usage.CacheCreation5m != 1 || events[0].Usage.CacheCreation1h != 3 {
		t.Fatalf("unexpected event: %+v", events[0])
	}
}

func TestClaudeKeepsFinalStopReasonEntryByMessageID(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".claude", "projects", "repo")
	mustMkdir(t, dir)
	body := `{"type":"assistant","uuid":"early","timestamp":"2026-05-08T01:02:03Z","cwd":"/repo","message":{"id":"msg_1","model":"anthropic/claude-opus-4-7-20260501","usage":{"input_tokens":10,"output_tokens":2,"cache_read_input_tokens":3}}}` + "\n" +
		`{"type":"assistant","uuid":"final","timestamp":"2026-05-08T01:02:04Z","cwd":"/repo","message":{"id":"msg_1","model":"anthropic/claude-opus-4-7-20260501","stop_reason":"end_turn","usage":{"input_tokens":10,"output_tokens":5,"cache_read_input_tokens":3}}}` + "\n" +
		`{"type":"assistant","uuid":"unfinished","timestamp":"2026-05-08T01:02:05Z","cwd":"/repo","message":{"id":"msg_2","model":"claude-opus-4-7","usage":{"input_tokens":100,"output_tokens":50}}}` + "\n"
	mustWrite(t, filepath.Join(dir, "session.jsonl"), body)
	events, err := NewClaude(Options{Home: home}).Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(events))
	}
	if events[0].ID != "msg_1" || events[0].Model != "claude-opus-4-7" || events[0].Usage.Output != 5 {
		t.Fatalf("unexpected final event: %+v", events[0])
	}
}

func TestForEachStreamsEvents(t *testing.T) {
	sentinel := errors.New("stop after first event")
	source := &streamingSource{}
	var seen int
	err := ForEach(context.Background(), []Source{source}, func(event usage.UsageEvent) error {
		seen++
		if !source.scanStarted {
			t.Fatal("handler ran before Scan started")
		}
		if event.Model != "gpt-5.4" {
			t.Fatalf("unexpected streamed event: %+v", event)
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("err = %v, want sentinel", err)
	}
	if !source.scanStarted {
		t.Fatal("ForEach did not call Scan")
	}
	if seen != 1 || source.afterCallback {
		t.Fatalf("ForEach did not stop at handler error: seen=%d afterCallback=%t", seen, source.afterCallback)
	}
}

func TestCodexAssociatesContextWithTokenCounts(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".codex", "sessions", "2026", "05", "08")
	mustMkdir(t, dir)
	body := `{"type":"session_meta","timestamp":"2026-05-08T01:00:00Z","payload":{"model_provider":"openai","cwd":"/repo"}}` + "\n" +
		`{"type":"turn_context","timestamp":"2026-05-08T01:00:01Z","payload":{"model":"gpt-5.4","cwd":"/repo"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-05-08T01:00:02Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":10,"cached_input_tokens":4,"output_tokens":2,"reasoning_output_tokens":1,"total_tokens":12}}}}` + "\n"
	mustWrite(t, filepath.Join(dir, "rollout.jsonl"), body)
	events, err := NewCodex(Options{Home: home}).Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(events))
	}
	if events[0].Model != "gpt-5.4" || events[0].Provider != "openai" || events[0].Usage.Reasoning != 1 {
		t.Fatalf("unexpected event: %+v", events[0])
	}
}

func TestCodexUsesTotalTokenUsageDeltasWithinTurn(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".codex", "sessions", "2026", "05", "08")
	mustMkdir(t, dir)
	body := `{"type":"session_meta","timestamp":"2026-05-08T01:00:00Z","payload":{"model_provider":"openai","cwd":"/repo"}}` + "\n" +
		`{"type":"turn_context","timestamp":"2026-05-08T01:00:01Z","payload":{"model":"openai/GPT-5.4-2026-03-05","cwd":"/repo"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-05-08T01:00:02Z","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":10,"cached_input_tokens":4,"output_tokens":2,"total_tokens":12},"last_token_usage":{"input_tokens":10,"cached_input_tokens":4,"output_tokens":2,"total_tokens":12}}}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-05-08T01:00:03Z","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":30,"cached_input_tokens":10,"output_tokens":6,"total_tokens":36},"last_token_usage":{"input_tokens":20,"cached_input_tokens":6,"output_tokens":4,"total_tokens":24}}}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-05-08T01:00:04Z","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":30,"cached_input_tokens":10,"output_tokens":6,"total_tokens":36},"last_token_usage":{"input_tokens":20,"cached_input_tokens":6,"output_tokens":4,"total_tokens":24}}}}` + "\n"
	mustWrite(t, filepath.Join(dir, "rollout.jsonl"), body)
	events, err := NewCodex(Options{Home: home}).Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("len(events) = %d, want 2", len(events))
	}
	if events[0].Model != "gpt-5.4" || events[0].Usage.Input != 10 || events[0].Usage.Output != 2 || events[0].Usage.CachedInput != 4 {
		t.Fatalf("unexpected first token count: %+v", events[0])
	}
	if events[1].Usage.Input != 20 || events[1].Usage.Output != 4 || events[1].Usage.CachedInput != 6 {
		t.Fatalf("unexpected second delta token count: %+v", events[1])
	}
	if got := events[1].Timestamp.Format("15:04:05"); got != "01:00:03" {
		t.Fatalf("timestamp = %s, want first duplicate token_count timestamp", got)
	}
}

func TestCodexSkipsSessionFilesBeforeWindow(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".codex", "sessions", "2026", "05", "08")
	mustMkdir(t, dir)
	oldPath := filepath.Join(dir, "old.jsonl")
	currentPath := filepath.Join(dir, "current.jsonl")
	oldBody := `{"type":"session_meta","timestamp":"2026-05-08T01:00:00Z","payload":{"id":"old-thread","model_provider":"openai","cwd":"/repo"}}` + "\n" +
		`{"type":"turn_context","timestamp":"2026-05-08T01:00:01Z","payload":{"id":"turn-old","model":"gpt-5.4","cwd":"/repo"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-05-08T01:00:02Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":10,"total_tokens":10}}}}` + "\n"
	currentBody := `{"type":"session_meta","timestamp":"2026-05-09T01:00:00Z","payload":{"id":"current-thread","model_provider":"openai","cwd":"/repo"}}` + "\n" +
		`{"type":"turn_context","timestamp":"2026-05-09T01:00:01Z","payload":{"id":"turn-current","model":"gpt-5.4","cwd":"/repo"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-05-09T01:00:02Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":20,"total_tokens":20}}}}` + "\n"
	mustWrite(t, oldPath, oldBody)
	mustWrite(t, currentPath, currentBody)
	windowStart := time.Date(2026, 5, 9, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(oldPath, windowStart.Add(-time.Hour), windowStart.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(currentPath, windowStart.Add(time.Hour), windowStart.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}

	events, err := NewCodex(Options{Home: home, WindowStart: windowStart, WindowEnd: windowStart.AddDate(0, 0, 1)}).Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].ThreadID != "current-thread" {
		t.Fatalf("window prefilter should skip stale session files, got %+v", events)
	}
}

func TestCodexUsesProviderFromModelPrefixWithinSession(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".codex", "sessions", "2026", "05", "08")
	mustMkdir(t, dir)
	body := `{"type":"session_meta","timestamp":"2026-05-08T01:00:00Z","payload":{"model_provider":"bcb","cwd":"/repo"}}` + "\n" +
		`{"type":"turn_context","timestamp":"2026-05-08T01:00:01Z","payload":{"id":"turn-a","model":"bcb/gpt-5.5","cwd":"/repo"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-05-08T01:00:02Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":100,"total_tokens":100}}}}` + "\n" +
		`{"type":"turn_context","timestamp":"2026-05-08T01:01:01Z","payload":{"id":"turn-b","model":"openai/gpt-5.4","cwd":"/repo"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-05-08T01:01:02Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":10,"total_tokens":10}}}}` + "\n"
	mustWrite(t, filepath.Join(dir, "rollout.jsonl"), body)
	events, err := NewCodex(Options{Home: home}).Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("len(events) = %d, want 2", len(events))
	}
	if events[0].Provider != "bcb" || events[0].Model != "gpt-5.5" {
		t.Fatalf("first event should keep initial provider/model: %+v", events[0])
	}
	if events[0].ProviderAttribution != string(usage.ProviderAttributionModel) {
		t.Fatalf("first event attribution = %q, want %q", events[0].ProviderAttribution, usage.ProviderAttributionModel)
	}
	if events[1].Provider != "openai" || events[1].Model != "gpt-5.4" {
		t.Fatalf("second event should use provider/model from switched turn: %+v", events[1])
	}
	if events[1].ProviderAttribution != string(usage.ProviderAttributionModel) {
		t.Fatalf("second event attribution = %q, want %q", events[1].ProviderAttribution, usage.ProviderAttributionModel)
	}
}

func TestCodexBareTurnAfterProviderQualifiedModelFallsBackToSessionProvider(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".codex", "sessions", "2026", "05", "08")
	mustMkdir(t, dir)
	body := `{"type":"session_meta","timestamp":"2026-05-08T01:00:00Z","payload":{"model_provider":"team-a","cwd":"/repo"}}` + "\n" +
		`{"type":"turn_context","timestamp":"2026-05-08T01:00:01Z","payload":{"id":"turn-a","model":"team-b/gpt-5.5","cwd":"/repo"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-05-08T01:00:02Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":100,"total_tokens":100}}}}` + "\n" +
		`{"type":"turn_context","timestamp":"2026-05-08T01:01:01Z","payload":{"id":"turn-b","model":"gpt-5.5","cwd":"/repo"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-05-08T01:01:02Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":10,"total_tokens":10}}}}` + "\n"
	mustWrite(t, filepath.Join(dir, "rollout.jsonl"), body)
	events, err := NewCodex(Options{Home: home}).Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("len(events) = %d, want 2", len(events))
	}
	if events[0].Provider != "team-b" || events[1].Provider != "team-a" {
		t.Fatalf("bare turn should not inherit previous provider-qualified model: %+v", events)
	}
}

func TestCodexUsesRequestHostProviderWhenModelIsBare(t *testing.T) {
	home := t.TempDir()
	mustWrite(t, filepath.Join(home, ".codex", "config.toml"), `
[model_providers]
[model_providers.team_a]
name = "team-a"
base_url = "https://team-a.example/v1"

[model_providers.team_b]
name = "team-b"
base_url = "https://team-b.example"
`)
	sessionDir := filepath.Join(home, ".codex", "sessions", "2026", "05", "08")
	mustMkdir(t, sessionDir)
	body := `{"type":"session_meta","timestamp":"2026-05-08T01:00:00Z","payload":{"id":"019e0000-0000-7000-8000-000000000001","model_provider":"team-a","cwd":"/repo"}}` + "\n" +
		`{"type":"turn_context","timestamp":"2026-05-08T01:00:01Z","payload":{"id":"turn-a","model":"gpt-5.5","cwd":"/repo"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-05-08T01:00:02Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":100,"total_tokens":100}}}}` + "\n" +
		`{"type":"turn_context","timestamp":"2026-05-08T01:01:01Z","payload":{"id":"turn-b","model":"gpt-5.5","cwd":"/repo"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-05-08T01:01:02Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":10,"total_tokens":10}}}}` + "\n"
	mustWrite(t, filepath.Join(sessionDir, "rollout-2026-05-08T09-00-00-019e0000-0000-7000-8000-000000000001.jsonl"), body)
	logDir := filepath.Join(home, ".codex", "log")
	mustMkdir(t, logDir)
	mustWrite(t, filepath.Join(logDir, "codex-tui.log"),
		`2026-05-08T01:00:01.500000Z  INFO session_loop{thread_id=019e0000-0000-7000-8000-000000000001}:turn{turn.id=turn-a model=gpt-5.5}:endpoint_session.stream_with{http.method=POST api.path="responses"}: POST to https://team-a.example/v1/responses: {"model":"gpt-5.5"}`+"\n"+
			`2026-05-08T01:01:01.500000Z  INFO session_loop{thread_id=019e0000-0000-7000-8000-000000000001}:turn{turn.id=turn-b model=gpt-5.5}:endpoint_session.stream_with{http.method=POST api.path="responses"}: POST to https://team-b.example/responses: {"model":"gpt-5.5"}`+"\n")

	events, err := NewCodex(Options{Home: home}).Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("len(events) = %d, want 2", len(events))
	}
	if events[0].Provider != "team-a" || events[1].Provider != "team-b" {
		t.Fatalf("request host providers not applied: %+v", events)
	}
	if events[0].ProviderAttribution != string(usage.ProviderAttributionExactRequest) || events[1].ProviderAttribution != string(usage.ProviderAttributionExactRequest) {
		t.Fatalf("request host attribution mismatch: %+v", events)
	}
}

func TestCodexUsesSQLiteBodyThreadIDWhenThreadColumnMissing(t *testing.T) {
	home := t.TempDir()
	mustWrite(t, filepath.Join(home, ".codex", "config.toml"), `
[model_providers]
[model_providers.team_a]
name = "team-a"
base_url = "https://team-a.example/v1"

[model_providers.team_b]
name = "team-b"
base_url = "https://team-b.example"
`)
	sessionDir := filepath.Join(home, ".codex", "sessions", "2026", "05", "08")
	mustMkdir(t, sessionDir)
	body := `{"type":"session_meta","timestamp":"2026-05-08T01:00:00Z","payload":{"id":"019e0000-0000-7000-8000-000000000099","model_provider":"team-a","cwd":"/repo"}}` + "\n" +
		`{"type":"turn_context","timestamp":"2026-05-08T01:00:01Z","payload":{"id":"turn-a","model":"gpt-5.5","cwd":"/repo"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-05-08T01:00:02Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":100,"total_tokens":100}}}}` + "\n"
	mustWrite(t, filepath.Join(sessionDir, "rollout-2026-05-08T09-00-00-019e0000-0000-7000-8000-000000000099.jsonl"), body)
	logDir := filepath.Join(home, ".codex", "log")
	mustMkdir(t, logDir)
	mustWrite(t, filepath.Join(logDir, "codex-tui.log"), "")
	sqlitePath := filepath.Join(home, ".codex", "logs_2.sqlite")
	mustWriteSQLite(t, sqlitePath, []string{
		`create table logs (
			id integer primary key autoincrement,
			ts integer not null,
			ts_nanos integer not null,
			level text not null,
			target text not null,
			feedback_log_body text,
			module_path text,
			file text,
			line integer,
			thread_id text,
			process_uuid text,
			estimated_bytes integer not null default 0
		);`,
		`insert into logs (ts, ts_nanos, level, target, feedback_log_body, thread_id) values (
			1746666002,
			0,
			'INFO',
			'codex_client::default_client',
			'session_loop:turn{otel.name="session_task.turn" thread.id=019e0000-0000-7000-8000-000000000099 turn.id=turn-a model=gpt-5.5}:run_turn:run_sampling_request: Request completed method=POST url=https://team-b.example/responses status=200 OK',
			null
		);`,
	})

	events, err := NewCodex(Options{Home: home}).Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(events))
	}
	if events[0].Provider != "team-b" {
		t.Fatalf("sqlite body thread.id provider not applied: %+v", events[0])
	}
	if events[0].ProviderAttribution != string(usage.ProviderAttributionExactRequest) {
		t.Fatalf("sqlite body thread.id attribution = %q, want exact_request", events[0].ProviderAttribution)
	}
}

func TestCodexPrefersConfiguredProviderHostOverChatgptAuthMode(t *testing.T) {
	home := t.TempDir()
	mustWrite(t, filepath.Join(home, ".codex", "config.toml"), `
[model_providers]
[model_providers.openai]
name = "openai"
base_url = "https://api.openai.com/v1"

[model_providers.toska]
name = "toska"
base_url = "https://api.toskaxy.xyz/v1"
`)
	sessionDir := filepath.Join(home, ".codex", "sessions", "2026", "05", "08")
	mustMkdir(t, sessionDir)
	body := `{"type":"session_meta","timestamp":"2026-05-08T01:00:00Z","payload":{"id":"019e0000-0000-7000-8000-000000000188","model_provider":"toska","cwd":"/repo"}}` + "\n" +
		`{"type":"turn_context","timestamp":"2026-05-08T01:00:01Z","payload":{"id":"turn-a","model":"gpt-5.4","cwd":"/repo"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-05-08T01:00:02Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":100,"total_tokens":100}}}}` + "\n"
	mustWrite(t, filepath.Join(sessionDir, "rollout-2026-05-08T09-00-00-019e0000-0000-7000-8000-000000000188.jsonl"), body)
	logDir := filepath.Join(home, ".codex", "log")
	mustMkdir(t, logDir)
	mustWrite(t, filepath.Join(logDir, "codex-tui.log"), "")
	sqlitePath := filepath.Join(home, ".codex", "logs_2.sqlite")
	mustWriteSQLite(t, sqlitePath, []string{
		`create table logs (
			id integer primary key autoincrement,
			ts integer not null,
			ts_nanos integer not null,
			level text not null,
			target text not null,
			feedback_log_body text,
			module_path text,
			file text,
			line integer,
			thread_id text,
			process_uuid text,
			estimated_bytes integer not null default 0
		);`,
		`insert into logs (ts, ts_nanos, level, target, feedback_log_body, thread_id) values (
			1746666001,
			0,
			'INFO',
			'codex_client::default_client',
			'session_loop:turn{otel.name="session_task.turn" thread.id=019e0000-0000-7000-8000-000000000188 turn.id=turn-a model=gpt-5.4}:run_turn:run_sampling_request: Request completed method=POST url=https://api.toskaxy.xyz/v1/responses status=200 OK',
			null
		);`,
		`insert into logs (ts, ts_nanos, level, target, feedback_log_body, thread_id) values (
			1746666001,
			500000000,
			'INFO',
			'codex_otel.log_only',
			'session_loop:turn{otel.name="session_task.turn" thread.id=019e0000-0000-7000-8000-000000000188 turn.id=turn-a model=gpt-5.4}:model_client.stream_responses_api:endpoint_session.stream_with:event.name="codex.api_request" auth_mode="Chatgpt" model=gpt-5.4 slug=gpt-5.4',
			null
		);`,
	})

	events, err := NewCodex(Options{Home: home}).Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(events))
	}
	if events[0].Provider != "toska" {
		t.Fatalf("configured provider host should override auth_mode=Chatgpt: %+v", events[0])
	}
	if events[0].ProviderAttribution != string(usage.ProviderAttributionExactRequest) {
		t.Fatalf("auth_mode attribution = %q, want exact_request", events[0].ProviderAttribution)
	}
}

func TestCodexUsesConfigBaseURLWhenProvidersShareHost(t *testing.T) {
	home := t.TempDir()
	mustWrite(t, filepath.Join(home, ".codex", "config.toml"), `
[model_providers]
[model_providers.team_a]
name = "team-a"
base_url = "https://shared.example/team-a"

[model_providers.team_b]
name = "team-b"
base_url = "https://shared.example/team-b"
`)
	sessionDir := filepath.Join(home, ".codex", "sessions", "2026", "05", "08")
	mustMkdir(t, sessionDir)
	body := `{"type":"session_meta","timestamp":"2026-05-08T01:00:00Z","payload":{"id":"019e0000-0000-7000-8000-000000000001","model_provider":"team-a","cwd":"/repo"}}` + "\n" +
		`{"type":"turn_context","timestamp":"2026-05-08T01:00:01Z","payload":{"id":"turn-a","model":"gpt-5.5","cwd":"/repo"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-05-08T01:00:02Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":100,"total_tokens":100}}}}` + "\n" +
		`{"type":"turn_context","timestamp":"2026-05-08T01:01:01Z","payload":{"id":"turn-b","model":"gpt-5.5","cwd":"/repo"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-05-08T01:01:02Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":10,"total_tokens":10}}}}` + "\n"
	mustWrite(t, filepath.Join(sessionDir, "rollout-2026-05-08T09-00-00-019e0000-0000-7000-8000-000000000001.jsonl"), body)
	logDir := filepath.Join(home, ".codex", "log")
	mustMkdir(t, logDir)
	mustWrite(t, filepath.Join(logDir, "codex-tui.log"),
		`2026-05-08T01:00:01.500000Z  INFO session_loop{thread_id=019e0000-0000-7000-8000-000000000001}:turn{turn.id=turn-a model=gpt-5.5}:endpoint_session.stream_with{http.method=POST api.path="responses"}: Request completed method=POST url=https://shared.example/team-b/responses status=200 OK`+"\n"+
			`2026-05-08T01:01:01.500000Z  INFO session_loop{thread_id=019e0000-0000-7000-8000-000000000001}:turn{turn.id=turn-b model=gpt-5.5}:endpoint_session.stream_with{http.method=POST api.path="responses"}: Http::connect; scheme=Some("https"), host=Some("shared.example"), port=None`+"\n")

	events, err := NewCodex(Options{Home: home}).Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("len(events) = %d, want 2", len(events))
	}
	if events[0].Provider != "team-b" || events[1].Provider != "team-a" {
		t.Fatalf("base_url path should disambiguate shared hosts and host-only evidence should stay ambiguous: %+v", events)
	}
}

func TestCodexRequestHostDoesNotBleedAcrossTurns(t *testing.T) {
	home := t.TempDir()
	mustWrite(t, filepath.Join(home, ".codex", "config.toml"), `
[model_providers]
[model_providers.team_a]
name = "team-a"
base_url = "https://team-a.example/v1"

[model_providers.team_b]
name = "team-b"
base_url = "https://team-b.example"
`)
	sessionDir := filepath.Join(home, ".codex", "sessions", "2026", "05", "08")
	mustMkdir(t, sessionDir)
	body := `{"type":"session_meta","timestamp":"2026-05-08T01:00:00Z","payload":{"id":"019e0000-0000-7000-8000-000000000001","model_provider":"team-a","cwd":"/repo"}}` + "\n" +
		`{"type":"turn_context","timestamp":"2026-05-08T01:00:01Z","payload":{"id":"turn-a","model":"gpt-5.5","cwd":"/repo"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-05-08T01:00:02Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":100,"total_tokens":100}}}}` + "\n" +
		`{"type":"turn_context","timestamp":"2026-05-08T01:01:01Z","payload":{"id":"turn-b","model":"gpt-5.5","cwd":"/repo"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-05-08T01:01:02Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":10,"total_tokens":10}}}}` + "\n"
	mustWrite(t, filepath.Join(sessionDir, "rollout-2026-05-08T09-00-00-019e0000-0000-7000-8000-000000000001.jsonl"), body)
	logDir := filepath.Join(home, ".codex", "log")
	mustMkdir(t, logDir)
	mustWrite(t, filepath.Join(logDir, "codex-tui.log"),
		`2026-05-08T01:00:01.500000Z  INFO session_loop{thread_id=019e0000-0000-7000-8000-000000000001}:turn{turn.id=turn-a model=gpt-5.5}:endpoint_session.stream_with{http.method=POST api.path="responses"}: POST to https://team-b.example/responses: {"model":"gpt-5.5"}`+"\n"+
			`2026-05-08T01:01:01.500000Z  INFO session_loop{thread_id=019e0000-0000-7000-8000-000000000001}:turn{turn.id=turn-b model=gpt-5.5}:endpoint_session.stream_with{http.method=POST api.path="responses"}: POST to https://rotated-team-b.example/responses: {"model":"gpt-5.5"}`+"\n")

	events, err := NewCodex(Options{Home: home}).Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("len(events) = %d, want 2", len(events))
	}
	if events[0].Provider != "team-b" || events[1].Provider != "team-a" {
		t.Fatalf("known host must apply only to its turn and unknown rotated host must fall back: %+v", events)
	}
}

func TestCodexProviderQualifiedModelOverridesRequestHost(t *testing.T) {
	home := t.TempDir()
	mustWrite(t, filepath.Join(home, ".codex", "config.toml"), `
[model_providers]
[model_providers.team_a]
name = "team-a"
base_url = "https://team-a.example"

[model_providers.team_b]
name = "team-b"
base_url = "https://team-b.example"
`)
	sessionDir := filepath.Join(home, ".codex", "sessions", "2026", "05", "08")
	mustMkdir(t, sessionDir)
	body := `{"type":"session_meta","timestamp":"2026-05-08T01:00:00Z","payload":{"id":"019e0000-0000-7000-8000-000000000001","model_provider":"team-a","cwd":"/repo"}}` + "\n" +
		`{"type":"turn_context","timestamp":"2026-05-08T01:00:01Z","payload":{"id":"turn-a","model":"team-b/gpt-5.5","cwd":"/repo"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-05-08T01:00:02Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":100,"total_tokens":100}}}}` + "\n"
	mustWrite(t, filepath.Join(sessionDir, "rollout-2026-05-08T09-00-00-019e0000-0000-7000-8000-000000000001.jsonl"), body)
	logDir := filepath.Join(home, ".codex", "log")
	mustMkdir(t, logDir)
	mustWrite(t, filepath.Join(logDir, "codex-tui.log"),
		`2026-05-08T01:00:01.500000Z  INFO session_loop{thread_id=019e0000-0000-7000-8000-000000000001}:turn{turn.id=turn-a model=team-b/gpt-5.5}:endpoint_session.stream_with{http.method=POST api.path="responses"}: POST to https://team-a.example/responses: {"model":"gpt-5.5"}`+"\n")

	events, err := NewCodex(Options{Home: home}).Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(events))
	}
	if events[0].Provider != "team-b" || events[0].Model != "gpt-5.5" {
		t.Fatalf("provider-qualified model should override request host: %+v", events)
	}
}

func TestCodexTokenCountEventUsesOwnTurnIDForProviderTimeline(t *testing.T) {
	home := t.TempDir()
	mustWrite(t, filepath.Join(home, ".codex", "config.toml"), `
[model_providers]
[model_providers.team_a]
name = "team-a"
base_url = "https://team-a.example"

[model_providers.team_b]
name = "team-b"
base_url = "https://team-b.example"
`)
	sessionDir := filepath.Join(home, ".codex", "sessions", "2026", "05", "08")
	mustMkdir(t, sessionDir)
	body := `{"type":"session_meta","timestamp":"2026-05-08T01:00:00Z","payload":{"id":"019e0000-0000-7000-8000-000000000001","model_provider":"team-a","cwd":"/repo"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-05-08T01:00:02Z","payload":{"id":"turn-b","type":"token_count","info":{"last_token_usage":{"input_tokens":100,"total_tokens":100}}}}` + "\n"
	mustWrite(t, filepath.Join(sessionDir, "rollout-2026-05-08T09-00-00-019e0000-0000-7000-8000-000000000001.jsonl"), body)
	logDir := filepath.Join(home, ".codex", "log")
	mustMkdir(t, logDir)
	mustWrite(t, filepath.Join(logDir, "codex-tui.log"),
		`2026-05-08T01:00:01.500000Z  INFO session_loop{thread_id=019e0000-0000-7000-8000-000000000001}:turn{turn.id=turn-b model=gpt-5.5}:endpoint_session.stream_with{http.method=POST api.path="responses"}: POST to https://team-b.example/responses: {"model":"gpt-5.5"}`+"\n")

	events, err := NewCodex(Options{Home: home}).Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(events))
	}
	if events[0].Provider != "team-b" {
		t.Fatalf("token_count event should use its own turn id for provider lookup: %+v", events)
	}
}

func TestCodexIgnoresQuotedProviderEvidenceFromToolOutput(t *testing.T) {
	home := t.TempDir()
	mustWrite(t, filepath.Join(home, ".codex", "config.toml"), `
[model_providers]
[model_providers.team_a]
name = "team-a"
base_url = "https://team-a.example"

[model_providers.team_b]
name = "team-b"
base_url = "https://team-b.example"
`)
	sessionDir := filepath.Join(home, ".codex", "sessions", "2026", "05", "08")
	mustMkdir(t, sessionDir)
	body := `{"type":"session_meta","timestamp":"2026-05-08T01:00:00Z","payload":{"id":"019e0000-0000-7000-8000-000000000001","model_provider":"team-a","cwd":"/repo"}}` + "\n" +
		`{"type":"turn_context","timestamp":"2026-05-08T01:00:01Z","payload":{"id":"turn-a","model":"gpt-5.5","cwd":"/repo"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-05-08T01:00:02Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":100,"total_tokens":100}}}}` + "\n"
	mustWrite(t, filepath.Join(sessionDir, "rollout-2026-05-08T09-00-00-019e0000-0000-7000-8000-000000000001.jsonl"), body)
	logDir := filepath.Join(home, ".codex", "log")
	mustMkdir(t, logDir)
	mustWrite(t, filepath.Join(logDir, "codex-tui.log"),
		`2026-05-08T01:00:01.500000Z  INFO session_loop{thread_id=019e0000-0000-7000-8000-000000000001}:turn{turn.id=turn-a model=gpt-5.5}: codex_core::stream_events_utils: observed prior note url=https://team-b.example/responses host=Some("team-b.example")`+"\n")

	events, err := NewCodex(Options{Home: home}).Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(events))
	}
	if events[0].Provider != "team-a" {
		t.Fatalf("quoted provider evidence must not override session provider: %+v", events)
	}
}

func TestCodexKeepsEarlierTimelineProviderUntilSwitchEvidence(t *testing.T) {
	home := t.TempDir()
	mustWrite(t, filepath.Join(home, ".codex", "config.toml"), `
[model_providers]
[model_providers.toska]
name = "toska"
base_url = "https://api.toskaxy.xyz/v1"

[model_providers.bcb]
name = "bcb"
base_url = "https://www.aiixiao.shop"
`)
	sessionDir := filepath.Join(home, ".codex", "sessions", "2026", "05", "08")
	mustMkdir(t, sessionDir)
	body := `{"type":"session_meta","timestamp":"2026-05-08T07:44:00Z","payload":{"id":"019e0000-0000-7000-8000-000000000001","model_provider":"toska","cwd":"/repo"}}` + "\n" +
		`{"type":"turn_context","timestamp":"2026-05-08T07:45:00Z","payload":{"id":"turn-a","model":"gpt-5.5","cwd":"/repo"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-05-08T07:45:10Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":100,"total_tokens":100}}}}` + "\n" +
		`{"type":"turn_context","timestamp":"2026-05-08T07:48:00Z","payload":{"id":"turn-b","model":"gpt-5.5","cwd":"/repo"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-05-08T07:48:10Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":110,"total_tokens":110}}}}` + "\n" +
		`{"type":"turn_context","timestamp":"2026-05-08T07:56:00Z","payload":{"id":"turn-c","model":"gpt-5.5","cwd":"/repo"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-05-08T07:56:10Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":120,"total_tokens":120}}}}` + "\n" +
		`{"type":"turn_context","timestamp":"2026-05-08T08:02:00Z","payload":{"id":"turn-d","model":"gpt-5.5","cwd":"/repo"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-05-08T08:02:10Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":130,"total_tokens":130}}}}` + "\n"
	mustWrite(t, filepath.Join(sessionDir, "rollout-2026-05-08T15-44-00-019e0000-0000-7000-8000-000000000001.jsonl"), body)
	logDir := filepath.Join(home, ".codex", "log")
	mustMkdir(t, logDir)
	mustWrite(t, filepath.Join(logDir, "codex-tui.log"),
		`2026-05-08T07:40:00.000000Z  INFO session_loop{thread_id=019e0000-0000-7000-8000-000000000001}:turn{turn.id=turn-z model=gpt-5.5}: codex_core::session::turn: Turn error: unexpected status 403 Forbidden, url: https://api.toskaxy.xyz/v1/responses`+"\n"+
			`2026-05-08T07:56:05.000000Z  INFO session_loop{thread_id=019e0000-0000-7000-8000-000000000001}:turn{turn.id=turn-c model=gpt-5.5}:endpoint_session.stream_with{http.method=POST api.path="responses"}: POST to https://www.aiixiao.shop/responses: {"model":"gpt-5.5"}`+"\n"+
			`2026-05-08T08:02:05.000000Z  INFO session_loop{thread_id=019e0000-0000-7000-8000-000000000001}:turn{turn.id=turn-d model=gpt-5.5}:endpoint_session.stream_with{http.method=POST api.path="responses"}: POST to https://www.aiixiao.shop/responses: {"model":"gpt-5.5"}`+"\n")

	events, err := NewCodex(Options{Home: home}).Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 4 {
		t.Fatalf("len(events) = %d, want 4", len(events))
	}
	if events[0].Provider != "toska" {
		t.Fatalf("first sparse turn should stay on earlier provider segment: %+v", events[0])
	}
	if events[0].ProviderAttribution != string(usage.ProviderAttributionInferredTimeline) {
		t.Fatalf("first sparse turn attribution = %q, want inferred_timeline", events[0].ProviderAttribution)
	}
	if events[1].Provider != "toska" {
		t.Fatalf("second sparse turn before switch evidence should stay on earlier provider segment: %+v", events[1])
	}
	if events[1].ProviderAttribution != string(usage.ProviderAttributionInferredTimeline) {
		t.Fatalf("second sparse turn attribution = %q, want inferred_timeline", events[1].ProviderAttribution)
	}
	if events[2].Provider != "bcb" || events[3].Provider != "bcb" {
		t.Fatalf("direct evidence turns should remain bcb: %+v", events)
	}
	if events[2].ProviderAttribution != string(usage.ProviderAttributionExactRequest) || events[3].ProviderAttribution != string(usage.ProviderAttributionExactRequest) {
		t.Fatalf("direct evidence attribution mismatch: %+v", events)
	}
}

func TestCodexKeepsSameBareTurnTokenCountsOnEarlierTimelineProvider(t *testing.T) {
	home := t.TempDir()
	mustWrite(t, filepath.Join(home, ".codex", "config.toml"), `
[model_providers]
[model_providers.toska]
name = "toska"
base_url = "https://api.toskaxy.xyz/v1"

[model_providers.bcb]
name = "bcb"
base_url = "https://www.aiixiao.shop"
`)
	sessionDir := filepath.Join(home, ".codex", "sessions", "2026", "05", "08")
	mustMkdir(t, sessionDir)
	body := `{"type":"session_meta","timestamp":"2026-05-08T07:44:00Z","payload":{"id":"019e0000-0000-7000-8000-000000000003","model_provider":"toska","cwd":"/repo"}}` + "\n" +
		`{"type":"turn_context","timestamp":"2026-05-08T07:48:00Z","payload":{"id":"turn-b","model":"gpt-5.5","cwd":"/repo"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-05-08T07:45:10Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":100,"total_tokens":100}}}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-05-08T07:55:50Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":50,"total_tokens":50}}}}` + "\n"
	mustWrite(t, filepath.Join(sessionDir, "rollout-2026-05-08T15-44-00-019e0000-0000-7000-8000-000000000003.jsonl"), body)
	logDir := filepath.Join(home, ".codex", "log")
	mustMkdir(t, logDir)
	mustWrite(t, filepath.Join(logDir, "codex-tui.log"),
		`2026-05-08T07:40:00.000000Z  INFO session_loop{thread_id=019e0000-0000-7000-8000-000000000003}:turn{turn.id=turn-prev model=gpt-5.5}: codex_core::session::turn: Turn error: unexpected status 403 Forbidden, url: https://api.toskaxy.xyz/v1/responses`+"\n"+
			`2026-05-08T07:56:05.000000Z  INFO session_loop{thread_id=019e0000-0000-7000-8000-000000000003}:turn{turn.id=turn-next model=gpt-5.5}:endpoint_session.stream_with{http.method=POST api.path="responses"}: POST to https://www.aiixiao.shop/responses: {"model":"gpt-5.5"}`+"\n")

	events, err := NewCodex(Options{Home: home}).Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("len(events) = %d, want 2", len(events))
	}
	if events[0].Provider != "toska" || events[0].ProviderAttribution != string(usage.ProviderAttributionInferredTimeline) {
		t.Fatalf("token_count before provider switch should stay on earlier provider: %+v", events[0])
	}
	if events[1].Provider != "toska" || events[1].ProviderAttribution != string(usage.ProviderAttributionInferredTimeline) {
		t.Fatalf("token_count before future provider switch should stay on earlier provider: %+v", events[1])
	}
}

func TestCodexAppliesSameTurnCompletedRequestOnlyAfterItArrives(t *testing.T) {
	home := t.TempDir()
	mustWrite(t, filepath.Join(home, ".codex", "config.toml"), `
[model_providers]
[model_providers.toska]
name = "toska"
base_url = "https://api.toskaxy.xyz/v1"

[model_providers.bcb]
name = "bcb"
base_url = "https://www.aiixiao.shop"
`)
	sessionDir := filepath.Join(home, ".codex", "sessions", "2026", "05", "08")
	mustMkdir(t, sessionDir)
	body := `{"type":"session_meta","timestamp":"2026-05-08T07:44:00Z","payload":{"id":"019e0000-0000-7000-8000-000000000007","model_provider":"toska","cwd":"/repo"}}` + "\n" +
		`{"type":"turn_context","timestamp":"2026-05-08T07:48:00Z","payload":{"id":"turn-b","model":"gpt-5.5","cwd":"/repo"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-05-08T07:48:10Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":100,"total_tokens":100}}}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-05-08T07:49:10Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":50,"total_tokens":50}}}}` + "\n"
	mustWrite(t, filepath.Join(sessionDir, "rollout-2026-05-08T15-44-00-019e0000-0000-7000-8000-000000000007.jsonl"), body)
	logDir := filepath.Join(home, ".codex", "log")
	mustMkdir(t, logDir)
	mustWrite(t, filepath.Join(logDir, "codex-tui.log"),
		`2026-05-08T07:47:00.000000Z  INFO session_loop{thread_id=019e0000-0000-7000-8000-000000000007}:turn{turn.id=turn-prev model=gpt-5.5}: codex_core::session::turn: Turn error: unexpected status 403 Forbidden, url: https://api.toskaxy.xyz/v1/responses`+"\n"+
			`2026-05-08T07:48:30.000000Z  INFO session_loop{thread_id=019e0000-0000-7000-8000-000000000007}:turn{turn.id=turn-b model=gpt-5.5}:run_turn:run_sampling_request: Request completed method=POST url=https://www.aiixiao.shop/responses status=200 OK`+"\n")

	events, err := NewCodex(Options{Home: home}).Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("len(events) = %d, want 2", len(events))
	}
	if events[0].Provider != "toska" || events[0].ProviderAttribution != string(usage.ProviderAttributionSessionFallback) {
		t.Fatalf("earlier token_count should not use future same-turn request evidence: %+v", events[0])
	}
	if events[1].Provider != "bcb" || events[1].ProviderAttribution != string(usage.ProviderAttributionExactRequest) {
		t.Fatalf("later token_count should use completed request evidence after it arrives: %+v", events[1])
	}
}

func TestCodexPrefersExplicitProviderHostOverChatGPTAuthMode(t *testing.T) {
	home := t.TempDir()
	mustWrite(t, filepath.Join(home, ".codex", "config.toml"), `
[model_providers]
[model_providers.openai]
name = "openai"
base_url = "https://api.openai.com/v1"

[model_providers.toska]
name = "toska"
base_url = "https://api.toskaxy.xyz/v1"
`)
	sessionDir := filepath.Join(home, ".codex", "sessions", "2026", "05", "08")
	mustMkdir(t, sessionDir)
	body := `{"type":"session_meta","timestamp":"2026-05-08T07:40:00Z","payload":{"id":"019e0000-0000-7000-8000-000000000208","model_provider":"toska","cwd":"/repo"}}` + "\n" +
		`{"type":"turn_context","timestamp":"2026-05-08T07:41:00Z","payload":{"id":"turn-explicit-provider","model":"gpt-5.5","cwd":"/repo"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-05-08T07:41:10Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":100,"total_tokens":100}}}}` + "\n"
	mustWrite(t, filepath.Join(sessionDir, "rollout-2026-05-08T15-40-00-019e0000-0000-7000-8000-000000000208.jsonl"), body)
	logDir := filepath.Join(home, ".codex", "log")
	mustMkdir(t, logDir)
	mustWrite(t, filepath.Join(logDir, "codex-tui.log"),
		`2026-05-08T07:41:05.000000Z  INFO session_loop{thread_id=019e0000-0000-7000-8000-000000000208}:turn{turn.id=turn-explicit-provider model=gpt-5.5}:stream_request:model_client.stream_responses_api{model=gpt-5.5 wire_api=responses transport="responses_http" http.method="POST" api.path="responses" turn.has_metadata_header=true}:responses.stream_request:responses.stream:endpoint_session.stream_with: POST to https://api.toskaxy.xyz/v1/responses: event.name="codex.api_request" auth_mode="Chatgpt"`+"\n")

	events, err := NewCodex(Options{Home: home}).Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(events))
	}
	if events[0].Provider != "toska" || events[0].ProviderAttribution != string(usage.ProviderAttributionExactRequest) {
		t.Fatalf("explicit provider host should outrank auth_mode=Chatgpt: %+v", events[0])
	}
}

func TestCodexSameTurnWebsocketProviderCutoverUsesEventTime(t *testing.T) {
	home := t.TempDir()
	mustWrite(t, filepath.Join(home, ".codex", "config.toml"), `
[model_providers]
[model_providers.openai]
name = "openai"
base_url = "https://api.openai.com/v1"

[model_providers.toska]
name = "toska"
base_url = "https://api.toskaxy.xyz/v1"
`)
	sessionDir := filepath.Join(home, ".codex", "sessions", "2026", "05", "08")
	mustMkdir(t, sessionDir)
	body := `{"type":"session_meta","timestamp":"2026-05-08T07:40:00Z","payload":{"id":"019e0000-0000-7000-8000-000000000108","model_provider":"toska","cwd":"/repo"}}` + "\n" +
		`{"type":"turn_context","timestamp":"2026-05-08T07:41:00Z","payload":{"id":"turn-cutover","model":"gpt-5.4","cwd":"/repo"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-05-08T07:41:10Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":100,"total_tokens":100}}}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-05-08T07:41:40Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":50,"total_tokens":50}}}}` + "\n"
	mustWrite(t, filepath.Join(sessionDir, "rollout-2026-05-08T15-40-00-019e0000-0000-7000-8000-000000000108.jsonl"), body)
	logDir := filepath.Join(home, ".codex", "log")
	mustMkdir(t, logDir)
	mustWrite(t, filepath.Join(logDir, "codex-tui.log"),
		`2026-05-08T07:41:05.000000Z  INFO session_loop{thread_id=019e0000-0000-7000-8000-000000000108}:turn{turn.id=turn-cutover model=gpt-5.4}:model_client.stream_responses_websocket{model=gpt-5.4 wire_api=responses transport="responses_websocket" api.path="responses" turn.has_metadata_header=true websocket.warmup=false}:model_client.websocket_connection{provider=OpenAI wire_api=responses transport="responses_websocket" api.path="responses" turn.has_metadata_header=true}: codex_core::client: new`+"\n"+
			`2026-05-08T07:41:19.000000Z  INFO session_loop{thread_id=019e0000-0000-7000-8000-000000000108}:turn{turn.id=turn-cutover model=gpt-5.4}:run_turn:run_sampling_request: Request completed method=POST url=https://api.toskaxy.xyz/v1/responses status=200 OK`+"\n")

	events, err := NewCodex(Options{Home: home}).Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("len(events) = %d, want 2", len(events))
	}
	if events[0].Provider != "openai" || events[0].ProviderAttribution != string(usage.ProviderAttributionExactRequest) {
		t.Fatalf("earlier token_count should stay on openai before websocket cutover: %+v", events[0])
	}
	if events[1].Provider != "toska" || events[1].ProviderAttribution != string(usage.ProviderAttributionExactRequest) {
		t.Fatalf("later token_count should switch to toska after request completion: %+v", events[1])
	}
}

func TestCodexDoesNotLetIsolatedSQLiteWebsocketProviderOverrideSessionFallback(t *testing.T) {
	home := t.TempDir()
	mustWrite(t, filepath.Join(home, ".codex", "config.toml"), `
[model_providers]
[model_providers.openai]
name = "openai"
base_url = "https://api.openai.com/v1"

[model_providers.toska]
name = "toska"
base_url = "https://api.toskaxy.xyz/v1"
`)
	sessionDir := filepath.Join(home, ".codex", "sessions", "2026", "05", "08")
	mustMkdir(t, sessionDir)
	body := `{"type":"session_meta","timestamp":"2026-05-08T01:00:00Z","payload":{"id":"019e0000-0000-7000-8000-000000000189","model_provider":"toska","cwd":"/repo"}}` + "\n" +
		`{"type":"turn_context","timestamp":"2026-05-08T01:00:01Z","payload":{"id":"turn-a","model":"gpt-5.4","cwd":"/repo"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-05-08T01:00:02Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":100,"total_tokens":100}}}}` + "\n"
	mustWrite(t, filepath.Join(sessionDir, "rollout-2026-05-08T09-00-00-019e0000-0000-7000-8000-000000000189.jsonl"), body)
	logDir := filepath.Join(home, ".codex", "log")
	mustMkdir(t, logDir)
	mustWrite(t, filepath.Join(logDir, "codex-tui.log"), "")
	sqlitePath := filepath.Join(home, ".codex", "logs_2.sqlite")
	mustWriteSQLite(t, sqlitePath, []string{
		`create table logs (
			id integer primary key autoincrement,
			ts integer not null,
			ts_nanos integer not null,
			level text not null,
			target text not null,
			feedback_log_body text,
			module_path text,
			file text,
			line integer,
			thread_id text,
			process_uuid text,
			estimated_bytes integer not null default 0
		);`,
		`insert into logs (ts, ts_nanos, level, target, feedback_log_body, thread_id) values (
			1746666001,
			0,
			'INFO',
			'codex_otel.log_only',
			'session_loop:turn{otel.name="session_task.turn" thread.id=019e0000-0000-7000-8000-000000000189 turn.id=turn-a model=gpt-5.4}:model_client.stream_responses_websocket{model=gpt-5.4 wire_api=responses transport="responses_websocket" api.path="responses" turn.has_metadata_header=true websocket.warmup=false}:model_client.websocket_connection{provider=OpenAI wire_api=responses transport="responses_websocket" api.path="responses" turn.has_metadata_header=true}: codex_core::client: new',
			null
		);`,
	})

	events, err := NewCodex(Options{Home: home}).Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(events))
	}
	if events[0].Provider != "toska" || events[0].ProviderAttribution != string(usage.ProviderAttributionSessionFallback) {
		t.Fatalf("isolated sqlite websocket provider should keep session fallback: %+v", events[0])
	}
}

func TestCodexUsesThreadlessSQLiteConversationProviderEvidence(t *testing.T) {
	home := t.TempDir()
	mustWrite(t, filepath.Join(home, ".codex", "config.toml"), `
[model_providers]
[model_providers.openai]
name = "openai"
base_url = "https://api.openai.com/v1"

[model_providers.toska]
name = "toska"
base_url = "https://api.toskaxy.xyz/v1"
`)
	sessionDir := filepath.Join(home, ".codex", "sessions", "2026", "05", "08")
	mustMkdir(t, sessionDir)
	body := `{"type":"session_meta","timestamp":"2026-05-08T01:00:00Z","payload":{"id":"019e0000-0000-7000-8000-00000000018a","model_provider":"openai","cwd":"/repo"}}` + "\n" +
		`{"type":"turn_context","timestamp":"2026-05-08T01:00:01Z","payload":{"id":"turn-a","model":"gpt-5.4","cwd":"/repo"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-05-08T01:00:03Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":100,"total_tokens":100}}}}` + "\n"
	mustWrite(t, filepath.Join(sessionDir, "rollout-2026-05-08T09-00-00-019e0000-0000-7000-8000-00000000018a.jsonl"), body)
	logDir := filepath.Join(home, ".codex", "log")
	mustMkdir(t, logDir)
	mustWrite(t, filepath.Join(logDir, "codex-tui.log"), "")
	sqlitePath := filepath.Join(home, ".codex", "logs_2.sqlite")
	mustWriteSQLite(t, sqlitePath, []string{
		`create table logs (
			id integer primary key autoincrement,
			ts integer not null,
			ts_nanos integer not null,
			level text not null,
			target text not null,
			feedback_log_body text,
			module_path text,
			file text,
			line integer,
			thread_id text,
			process_uuid text,
			estimated_bytes integer not null default 0
		);`,
		`insert into logs (ts, ts_nanos, level, target, feedback_log_body, thread_id) values (
			1746666002,
			0,
			'INFO',
			'codex_client::transport',
			'model_client.stream_responses_api{model=gpt-5.4}:responses.stream_request: event.name="codex.api_request" auth_mode="Chatgpt" conversation.id=019e0000-0000-7000-8000-00000000018a turn.id=turn-a',
			null
		);`,
		`insert into logs (ts, ts_nanos, level, target, feedback_log_body, thread_id) values (
			1746666002,
			500000000,
			'INFO',
			'codex_client::transport',
			'model_client.stream_responses_api{model=gpt-5.4}:responses.stream_request: POST to https://api.toskaxy.xyz/v1/responses conversation.id=019e0000-0000-7000-8000-00000000018a turn.id=turn-a',
			null
		);`,
	})

	events, err := NewCodex(Options{Home: home}).Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(events))
	}
	if events[0].Provider != "toska" || events[0].ProviderAttribution != string(usage.ProviderAttributionExactRequest) {
		t.Fatalf("threadless conversation-id sqlite evidence should resolve exact provider: %+v", events[0])
	}
}

func TestCodexDoesNotInferFutureProviderBeforeFirstTimelinePoint(t *testing.T) {
	home := t.TempDir()
	mustWrite(t, filepath.Join(home, ".codex", "config.toml"), `
[model_providers]
[model_providers.team_a]
name = "team-a"
base_url = "https://team-a.example"

[model_providers.team_b]
name = "team-b"
base_url = "https://team-b.example"
`)
	sessionDir := filepath.Join(home, ".codex", "sessions", "2026", "05", "08")
	mustMkdir(t, sessionDir)
	body := `{"type":"session_meta","timestamp":"2026-05-08T01:00:00Z","payload":{"id":"019e0000-0000-7000-8000-000000000002","model_provider":"team-a","cwd":"/repo"}}` + "\n" +
		`{"type":"turn_context","timestamp":"2026-05-08T01:01:00Z","payload":{"id":"turn-a","model":"gpt-5.5","cwd":"/repo"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-05-08T01:01:10Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":90,"total_tokens":90}}}}` + "\n" +
		`{"type":"turn_context","timestamp":"2026-05-08T01:05:00Z","payload":{"id":"turn-b","model":"gpt-5.5","cwd":"/repo"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-05-08T01:05:10Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":120,"total_tokens":120}}}}` + "\n"
	mustWrite(t, filepath.Join(sessionDir, "rollout-2026-05-08T09-00-00-019e0000-0000-7000-8000-000000000002.jsonl"), body)
	logDir := filepath.Join(home, ".codex", "log")
	mustMkdir(t, logDir)
	mustWrite(t, filepath.Join(logDir, "codex-tui.log"),
		`2026-05-08T01:05:05.000000Z  INFO session_loop{thread_id=019e0000-0000-7000-8000-000000000002}:turn{turn.id=turn-b model=gpt-5.5}:endpoint_session.stream_with{http.method=POST api.path="responses"}: POST to https://team-b.example/responses: {"model":"gpt-5.5"}`+"\n")

	events, err := NewCodex(Options{Home: home}).Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("len(events) = %d, want 2", len(events))
	}
	if events[0].Provider != "team-a" {
		t.Fatalf("first turn should stay on session provider before any evidence exists: %+v", events[0])
	}
	if events[0].ProviderAttribution != string(usage.ProviderAttributionSessionFallback) {
		t.Fatalf("first turn attribution = %q, want session_fallback", events[0].ProviderAttribution)
	}
	if events[1].Provider != "team-b" {
		t.Fatalf("exact evidence turn should use team-b: %+v", events[1])
	}
	if events[1].ProviderAttribution != string(usage.ProviderAttributionExactRequest) {
		t.Fatalf("second turn attribution = %q, want exact_request", events[1].ProviderAttribution)
	}
}

func TestCodexPrefersSessionProviderOverIsolatedEarlierWebsocketProvider(t *testing.T) {
	home := t.TempDir()
	mustWrite(t, filepath.Join(home, ".codex", "config.toml"), `
[model_providers]
[model_providers.openai]
name = "openai"
base_url = "https://api.openai.com/v1"

[model_providers.toska]
name = "toska"
base_url = "https://api.toskaxy.xyz/v1"
`)
	sessionDir := filepath.Join(home, ".codex", "sessions", "2026", "05", "08")
	mustMkdir(t, sessionDir)
	body := `{"type":"session_meta","timestamp":"2026-05-08T07:40:00Z","payload":{"id":"019e0000-0000-7000-8000-000000000208","model_provider":"toska","cwd":"/repo"}}` + "\n" +
		`{"type":"turn_context","timestamp":"2026-05-08T07:40:10Z","payload":{"id":"turn-openai","model":"gpt-5.4","cwd":"/repo"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-05-08T07:40:20Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":100,"total_tokens":100}}}}` + "\n" +
		`{"type":"turn_context","timestamp":"2026-05-08T07:41:00Z","payload":{"id":"turn-session","model":"gpt-5.4","cwd":"/repo"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-05-08T07:41:10Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":120,"total_tokens":120}}}}` + "\n"
	mustWrite(t, filepath.Join(sessionDir, "rollout-2026-05-08T15-40-00-019e0000-0000-7000-8000-000000000208.jsonl"), body)
	logDir := filepath.Join(home, ".codex", "log")
	mustMkdir(t, logDir)
	mustWrite(t, filepath.Join(logDir, "codex-tui.log"),
		`2026-05-08T07:40:15.000000Z  INFO session_loop{thread_id=019e0000-0000-7000-8000-000000000208}:turn{turn.id=turn-openai model=gpt-5.4}:model_client.stream_responses_websocket{model=gpt-5.4 wire_api=responses transport="responses_websocket" api.path="responses" turn.has_metadata_header=true websocket.warmup=false}:model_client.websocket_connection{provider=OpenAI wire_api=responses transport="responses_websocket" api.path="responses" turn.has_metadata_header=true}: codex_core::client: new`+"\n")

	events, err := NewCodex(Options{Home: home}).Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("len(events) = %d, want 2", len(events))
	}
	if events[0].Provider != "toska" || events[0].ProviderAttribution != string(usage.ProviderAttributionSessionFallback) {
		t.Fatalf("isolated websocket provider should not override session fallback: %+v", events[0])
	}
	if events[1].Provider != "toska" || events[1].ProviderAttribution != string(usage.ProviderAttributionSessionFallback) {
		t.Fatalf("conflicting earlier inference should not override current session provider: %+v", events[1])
	}
}

func TestCodexAllowsBoundedTimelineInferenceToOverrideSessionFallback(t *testing.T) {
	home := t.TempDir()
	mustWrite(t, filepath.Join(home, ".codex", "config.toml"), `
[model_providers]
[model_providers.team_a]
name = "team-a"
base_url = "https://team-a.example"

[model_providers.team_b]
name = "team-b"
base_url = "https://team-b.example"
`)
	sessionDir := filepath.Join(home, ".codex", "sessions", "2026", "05", "08")
	mustMkdir(t, sessionDir)
	body := `{"type":"session_meta","timestamp":"2026-05-08T01:00:00Z","payload":{"id":"019e0000-0000-7000-8000-000000000209","model_provider":"team-b","cwd":"/repo"}}` + "\n" +
		`{"type":"turn_context","timestamp":"2026-05-08T01:00:01Z","payload":{"id":"turn-prev","model":"gpt-5.5","cwd":"/repo"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-05-08T01:00:10Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":100,"total_tokens":100}}}}` + "\n" +
		`{"type":"turn_context","timestamp":"2026-05-08T01:01:00Z","payload":{"id":"turn-mid","model":"gpt-5.5","cwd":"/repo"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-05-08T01:01:10Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":110,"total_tokens":110}}}}` + "\n" +
		`{"type":"turn_context","timestamp":"2026-05-08T01:02:00Z","payload":{"id":"turn-next","model":"gpt-5.5","cwd":"/repo"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-05-08T01:02:10Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":120,"total_tokens":120}}}}` + "\n"
	mustWrite(t, filepath.Join(sessionDir, "rollout-2026-05-08T09-00-00-019e0000-0000-7000-8000-000000000209.jsonl"), body)
	logDir := filepath.Join(home, ".codex", "log")
	mustMkdir(t, logDir)
	mustWrite(t, filepath.Join(logDir, "codex-tui.log"),
		`2026-05-08T01:00:05.000000Z  INFO session_loop{thread_id=019e0000-0000-7000-8000-000000000209}:turn{turn.id=turn-prev model=gpt-5.5}:endpoint_session.stream_with{http.method=POST api.path="responses"}: POST to https://team-a.example/responses: {"model":"gpt-5.5"}`+"\n"+
			`2026-05-08T01:02:05.000000Z  INFO session_loop{thread_id=019e0000-0000-7000-8000-000000000209}:turn{turn.id=turn-next model=gpt-5.5}:endpoint_session.stream_with{http.method=POST api.path="responses"}: POST to https://team-a.example/responses: {"model":"gpt-5.5"}`+"\n")

	events, err := NewCodex(Options{Home: home}).Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 {
		t.Fatalf("len(events) = %d, want 3", len(events))
	}
	if events[0].Provider != "team-a" || events[0].ProviderAttribution != string(usage.ProviderAttributionExactRequest) {
		t.Fatalf("previous anchor turn should remain exact provider: %+v", events[0])
	}
	if events[1].Provider != "team-a" || events[1].ProviderAttribution != string(usage.ProviderAttributionInferredTimeline) {
		t.Fatalf("bounded timeline inference should override conflicting session fallback: %+v", events[1])
	}
	if events[2].Provider != "team-a" || events[2].ProviderAttribution != string(usage.ProviderAttributionExactRequest) {
		t.Fatalf("next anchor turn should remain exact provider: %+v", events[2])
	}
}

func TestCodexUsesTurnlessSQLiteProviderAnchorsForBoundedTimelineInference(t *testing.T) {
	home := t.TempDir()
	mustWrite(t, filepath.Join(home, ".codex", "config.toml"), `
[model_providers]
[model_providers.openai]
name = "openai"
base_url = "https://api.openai.com/v1"

[model_providers.toska]
name = "toska"
base_url = "https://api.toskaxy.xyz/v1"
`)
	sessionDir := filepath.Join(home, ".codex", "sessions", "2026", "05", "08")
	mustMkdir(t, sessionDir)
	body := `{"type":"session_meta","timestamp":"2026-05-08T01:24:00Z","payload":{"id":"019e0000-0000-7000-8000-00000000020a","model_provider":"toska","cwd":"/repo"}}` + "\n" +
		`{"type":"turn_context","timestamp":"2026-05-08T01:24:00Z","payload":{"id":"turn-prev","model":"gpt-5.4","cwd":"/repo"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-05-08T01:24:08Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":100,"total_tokens":100}}}}` + "\n" +
		`{"type":"turn_context","timestamp":"2026-05-08T01:24:40Z","payload":{"id":"turn-mid","model":"gpt-5.4","cwd":"/repo"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-05-08T01:24:52Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":110,"total_tokens":110}}}}` + "\n" +
		`{"type":"turn_context","timestamp":"2026-05-08T01:41:20Z","payload":{"id":"turn-next","model":"gpt-5.4","cwd":"/repo"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-05-08T01:41:30Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":120,"total_tokens":120}}}}` + "\n"
	mustWrite(t, filepath.Join(sessionDir, "rollout-2026-05-08T09-24-00-019e0000-0000-7000-8000-00000000020a.jsonl"), body)
	logDir := filepath.Join(home, ".codex", "log")
	mustMkdir(t, logDir)
	mustWrite(t, filepath.Join(logDir, "codex-tui.log"),
		`2026-05-08T01:24:53.000000Z  INFO session_loop{thread_id=019e0000-0000-7000-8000-00000000020a}:turn{turn.id=turn-mid model=gpt-5.4}:model_client.stream_responses_websocket{model=gpt-5.4 wire_api=responses transport="responses_websocket" api.path="responses" turn.has_metadata_header=true websocket.warmup=false}:model_client.websocket_connection{provider=OpenAI wire_api=responses transport="responses_websocket" api.path="responses" turn.has_metadata_header=true}: codex_core::client: close time.busy=8.42µs time.idle=8.33µs`+"\n"+
			`2026-05-08T01:41:25.000000Z  INFO session_loop{thread_id=019e0000-0000-7000-8000-00000000020a}:turn{turn.id=turn-next model=gpt-5.4}:endpoint_session.stream_with{http.method=POST api.path="responses"}: POST to https://api.toskaxy.xyz/v1/responses: {"model":"gpt-5.4"}`+"\n")
	sqlitePath := filepath.Join(home, ".codex", "logs_2.sqlite")
	mustWriteSQLite(t, sqlitePath, []string{
		`create table logs (
			id integer primary key autoincrement,
			ts integer not null,
			ts_nanos integer not null,
			level text not null,
			target text not null,
			feedback_log_body text,
			module_path text,
			file text,
			line integer,
			thread_id text,
			process_uuid text,
			estimated_bytes integer not null default 0
		);`,
		`insert into logs (ts, ts_nanos, level, target, feedback_log_body, thread_id) values (
			1778203448,
			0,
			'INFO',
			'codex_client::default_client',
			'session_loop:thread{thread.id=019e0000-0000-7000-8000-00000000020a model=gpt-5.4}:run_turn:run_sampling_request: Request completed method=POST url=https://api.openai.com/v1/responses status=200 OK',
			null
		);`,
		`insert into logs (ts, ts_nanos, level, target, feedback_log_body, thread_id) values (
			1778204419,
			0,
			'INFO',
			'codex_client::default_client',
			'session_loop:thread{thread.id=019e0000-0000-7000-8000-00000000020a model=gpt-5.4}:run_turn:run_sampling_request: Request completed method=POST url=https://api.openai.com/v1/responses status=200 OK',
			null
		);`,
	})

	events, err := NewCodex(Options{Home: home}).Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 {
		t.Fatalf("len(events) = %d, want 3", len(events))
	}
	if events[1].Provider != "openai" || events[1].ProviderAttribution != string(usage.ProviderAttributionInferredTimeline) {
		t.Fatalf("turnless sqlite anchor should allow bounded openai inference: %+v", events[1])
	}
	if events[2].Provider != "toska" || events[2].ProviderAttribution != string(usage.ProviderAttributionExactRequest) {
		t.Fatalf("next turn should keep exact toska provider: %+v", events[2])
	}
}

func TestCodexIgnoresTurnlessSQLiteSSEAuthModeAnchorsForTimelineInference(t *testing.T) {
	home := t.TempDir()
	mustWrite(t, filepath.Join(home, ".codex", "config.toml"), `
[model_providers]
[model_providers.toska]
name = "toska"
base_url = "https://api.toskaxy.xyz/v1"
`)
	sessionDir := filepath.Join(home, ".codex", "sessions", "2026", "05", "08")
	mustMkdir(t, sessionDir)
	body := `{"type":"session_meta","timestamp":"2026-05-08T01:24:00Z","payload":{"id":"019e0000-0000-7000-8000-00000000020c","model_provider":"toska","cwd":"/repo"}}` + "\n" +
		`{"type":"turn_context","timestamp":"2026-05-08T01:24:40Z","payload":{"id":"turn-mid","model":"gpt-5.4","cwd":"/repo"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-05-08T01:24:52Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":110,"total_tokens":110}}}}` + "\n"
	mustWrite(t, filepath.Join(sessionDir, "rollout-2026-05-08T09-24-00-019e0000-0000-7000-8000-00000000020c.jsonl"), body)
	logDir := filepath.Join(home, ".codex", "log")
	mustMkdir(t, logDir)
	mustWrite(t, filepath.Join(logDir, "codex-tui.log"), "")
	sqlitePath := filepath.Join(home, ".codex", "logs_2.sqlite")
	mustWriteSQLite(t, sqlitePath, []string{
		`create table logs (
			id integer primary key autoincrement,
			ts integer not null,
			ts_nanos integer not null,
			level text not null,
			target text not null,
			feedback_log_body text,
			module_path text,
			file text,
			line integer,
			thread_id text,
			process_uuid text,
			estimated_bytes integer not null default 0
		);`,
		`insert into logs (ts, ts_nanos, level, target, feedback_log_body, thread_id) values (
			1778203448,
			0,
			'INFO',
			'codex_otel.log_only',
			'event.name="codex.sse_event" event.kind=response.completed event.timestamp=2026-05-08T01:24:08Z conversation.id=019e0000-0000-7000-8000-00000000020c auth_mode="Chatgpt" model=gpt-5.4 slug=gpt-5.4',
			null
		);`,
		`insert into logs (ts, ts_nanos, level, target, feedback_log_body, thread_id) values (
			1778204419,
			0,
			'INFO',
			'codex_otel.log_only',
			'event.name="codex.sse_event" event.kind=response.completed event.timestamp=2026-05-08T01:40:19Z conversation.id=019e0000-0000-7000-8000-00000000020c auth_mode="Chatgpt" model=gpt-5.4 slug=gpt-5.4',
			null
		);`,
	})

	events, err := NewCodex(Options{Home: home}).Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(events))
	}
	if events[0].Provider != "toska" || events[0].ProviderAttribution != string(usage.ProviderAttributionSessionFallback) {
		t.Fatalf("turnless sse auth anchors should not override session fallback: %+v", events[0])
	}
}

func TestCodexIgnoresToolResultLinesAsProviderEvidence(t *testing.T) {
	home := t.TempDir()
	mustWrite(t, filepath.Join(home, ".codex", "config.toml"), `
[model_providers]
[model_providers.openai]
name = "openai"
base_url = "https://api.openai.com/v1"

[model_providers.toska]
name = "toska"
base_url = "https://api.toskaxy.xyz/v1"
`)
	sessionDir := filepath.Join(home, ".codex", "sessions", "2026", "05", "08")
	mustMkdir(t, sessionDir)
	body := `{"type":"session_meta","timestamp":"2026-05-08T03:10:00Z","payload":{"id":"019e0000-0000-7000-8000-00000000020d","model_provider":"toska","cwd":"/repo"}}` + "\n" +
		`{"type":"turn_context","timestamp":"2026-05-08T03:11:00Z","payload":{"id":"turn-main","model":"gpt-5.5","cwd":"/repo"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-05-08T03:11:12Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":120,"total_tokens":120}}}}` + "\n"
	mustWrite(t, filepath.Join(sessionDir, "rollout-2026-05-08T11-10-00-019e0000-0000-7000-8000-00000000020d.jsonl"), body)
	logDir := filepath.Join(home, ".codex", "log")
	mustMkdir(t, logDir)
	mustWrite(t, filepath.Join(logDir, "codex-tui.log"), "")
	sqlitePath := filepath.Join(home, ".codex", "logs_2.sqlite")
	mustWriteSQLite(t, sqlitePath, []string{
		`create table logs (
			id integer primary key autoincrement,
			ts integer not null,
			ts_nanos integer not null,
			level text not null,
			target text not null,
			feedback_log_body text,
			module_path text,
			file text,
			line integer,
			thread_id text,
			process_uuid text,
			estimated_bytes integer not null default 0
		);`,
		`insert into logs (ts, ts_nanos, level, target, feedback_log_body, thread_id) values (
			1778209872,
			159988000,
			'INFO',
			'codex_otel.log_only',
			'session_loop{thread_id=019e0000-0000-7000-8000-00000000020d}:turn{turn.id=turn-main model=gpt-5.5}:dispatch_tool_call_with_code_mode_result: event.name="codex.tool_result" tool_name=exec_command output=Chunk ID: demo Output: Request completed method=POST url=https://api.openai.com/v1/responses status=200 OK conversation.id=019e0000-0000-7000-8000-00000000020d',
			'019e0000-0000-7000-8000-00000000020d'
		);`,
		`insert into logs (ts, ts_nanos, level, target, feedback_log_body, thread_id) values (
			1778209909,
			0,
			'INFO',
			'codex_client::default_client',
			'session_loop{thread_id=019e0000-0000-7000-8000-00000000020d}:turn{turn.id=turn-main model=gpt-5.5}:run_turn:run_sampling_request: Request completed method=POST url=https://api.toskaxy.xyz/v1/responses status=200 OK',
			'019e0000-0000-7000-8000-00000000020d'
		);`,
	})

	events, err := NewCodex(Options{Home: home}).Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(events))
	}
	if events[0].Provider != "toska" || events[0].ProviderAttribution != string(usage.ProviderAttributionSessionFallback) {
		t.Fatalf("tool result text should not affect early event attribution: %+v", events[0])
	}
}

func TestCodexIgnoresTurnlessSQLiteAnchorsWhenModelsDoNotMatch(t *testing.T) {
	home := t.TempDir()
	mustWrite(t, filepath.Join(home, ".codex", "config.toml"), `
[model_providers]
[model_providers.toska]
name = "toska"
base_url = "https://api.toskaxy.xyz/v1"
`)
	sessionDir := filepath.Join(home, ".codex", "sessions", "2026", "05", "08")
	mustMkdir(t, sessionDir)
	body := `{"type":"session_meta","timestamp":"2026-05-08T02:10:00Z","payload":{"id":"019e0000-0000-7000-8000-00000000020b","model_provider":"toska","cwd":"/repo"}}` + "\n" +
		`{"type":"turn_context","timestamp":"2026-05-08T02:10:00Z","payload":{"id":"turn-main","model":"gpt-5.5","cwd":"/repo"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-05-08T02:10:16Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":100,"total_tokens":100}}}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-05-08T02:10:23Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":120,"total_tokens":120}}}}` + "\n"
	mustWrite(t, filepath.Join(sessionDir, "rollout-2026-05-08T10-10-00-019e0000-0000-7000-8000-00000000020b.jsonl"), body)
	logDir := filepath.Join(home, ".codex", "log")
	mustMkdir(t, logDir)
	mustWrite(t, filepath.Join(logDir, "codex-tui.log"),
		`2026-05-08T02:10:17.340000Z  INFO session_loop{thread_id=019e0000-0000-7000-8000-00000000020b}:turn{turn.id=turn-main model=gpt-5.5}:model_client.stream_responses_websocket{model=gpt-5.5 wire_api=responses transport="responses_websocket" api.path="responses" turn.has_metadata_header=true websocket.warmup=false}:model_client.websocket_connection{provider=OpenAI wire_api=responses transport="responses_websocket" api.path="responses" turn.has_metadata_header=true}: codex_core::client: close time.busy=7.46µs time.idle=7.33µs`+"\n")
	sqlitePath := filepath.Join(home, ".codex", "logs_2.sqlite")
	mustWriteSQLite(t, sqlitePath, []string{
		`create table logs (
			id integer primary key autoincrement,
			ts integer not null,
			ts_nanos integer not null,
			level text not null,
			target text not null,
			feedback_log_body text,
			module_path text,
			file text,
			line integer,
			thread_id text,
			process_uuid text,
			estimated_bytes integer not null default 0
		);`,
		`insert into logs (ts, ts_nanos, level, target, feedback_log_body, thread_id) values (
			1778206226,
			896650000,
			'INFO',
			'codex_otel.log_only',
			'event.name="codex.sse_event" event.kind=response.completed input_token_count=235491 output_token_count=3744 event.timestamp=2026-05-08T02:10:26.896Z conversation.id=019e0000-0000-7000-8000-00000000020b auth_mode="Chatgpt" model=gpt-5.4-mini slug=gpt-5.4-mini',
			null
		);`,
		`insert into logs (ts, ts_nanos, level, target, feedback_log_body, thread_id) values (
			1778207126,
			896650000,
			'INFO',
			'codex_otel.log_only',
			'event.name="codex.sse_event" event.kind=response.completed input_token_count=235491 output_token_count=3744 event.timestamp=2026-05-08T02:25:26.896Z conversation.id=019e0000-0000-7000-8000-00000000020b auth_mode="Chatgpt" model=gpt-5.4-mini slug=gpt-5.4-mini',
			null
		);`,
	})

	events, err := NewCodex(Options{Home: home}).Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("len(events) = %d, want 2", len(events))
	}
	if events[0].Provider != "toska" || events[0].ProviderAttribution != string(usage.ProviderAttributionSessionFallback) {
		t.Fatalf("mismatched turnless model should not override first event: %+v", events[0])
	}
	if events[1].Provider != "toska" || events[1].ProviderAttribution != string(usage.ProviderAttributionSessionFallback) {
		t.Fatalf("mismatched turnless model should not override second event: %+v", events[1])
	}
}

func TestCodexProviderTimelineInferenceForMixedThread(t *testing.T) {
	threadID := "thread-1"
	prev := codexProviderPoint{
		At:       time.Date(2026, 5, 8, 7, 40, 0, 0, time.UTC),
		TurnID:   "turn-prev",
		Provider: "toska",
		Strength: codexProviderStrengthWeak,
	}
	next := codexProviderPoint{
		At:       time.Date(2026, 5, 8, 7, 56, 5, 0, time.UTC),
		TurnID:   "turn-next",
		Provider: "bcb",
		Strength: codexProviderStrengthStrong,
	}
	timeline := codexProviderTimeline{
		threadID: {prev, next},
	}

	got := timeline.inferenceForTurn(threadID, "turn-mid", time.Date(2026, 5, 8, 7, 48, 10, 0, time.UTC))
	if got.Provider != "toska" {
		t.Fatalf("provider = %q, want toska", got.Provider)
	}
	if got.Exact {
		t.Fatal("mixed-thread midpoint inference should not be exact")
	}
	if got.Prev == nil || got.Prev.TurnID != prev.TurnID || got.Next == nil || got.Next.TurnID != next.TurnID {
		t.Fatalf("unexpected anchors: %+v", got)
	}
}

func TestCodexDoesNotPreferStrongerFutureProviderEvidenceWithinMixedThread(t *testing.T) {
	threadID := "thread-2"
	prev := codexProviderPoint{
		At:       time.Date(2026, 5, 8, 7, 40, 0, 0, time.UTC),
		TurnID:   "turn-prev",
		Provider: "toska",
		Strength: codexProviderStrengthWeak,
	}
	next := codexProviderPoint{
		At:       time.Date(2026, 5, 8, 7, 56, 0, 0, time.UTC),
		TurnID:   "turn-next",
		Provider: "bcb",
		Strength: codexProviderStrengthStrong,
	}
	timeline := codexProviderTimeline{
		threadID: {prev, next},
	}

	got := timeline.inferenceForTurn(threadID, "turn-mid", time.Date(2026, 5, 8, 7, 47, 0, 0, time.UTC))
	if got.Provider != "toska" {
		t.Fatalf("provider = %q, want toska", got.Provider)
	}
	if got.Prev == nil || got.Prev.Provider != "toska" || got.Next == nil || got.Next.Provider != "bcb" {
		t.Fatalf("unexpected anchors: %+v", got)
	}
}

func TestCodexExactProviderForTurnAtUsesLatestPriorPoint(t *testing.T) {
	threadID := "thread-cutover"
	turnID := "turn-cutover"
	timeline := codexProviderTimeline{
		threadID: {
			{
				At:       time.Date(2026, 5, 8, 7, 41, 5, 0, time.UTC),
				TurnID:   turnID,
				Provider: "openai",
				Strength: codexProviderStrengthWeak,
			},
			{
				At:       time.Date(2026, 5, 8, 7, 41, 19, 0, time.UTC),
				TurnID:   turnID,
				Provider: "toska",
				Strength: codexProviderStrengthStrong,
			},
		},
	}

	if provider, found := timeline.exactProviderForTurnAt(threadID, turnID, time.Date(2026, 5, 8, 7, 41, 10, 0, time.UTC)); !found || provider != "openai" {
		t.Fatalf("early provider = %q, found=%t, want openai/true", provider, found)
	}
	if provider, found := timeline.exactProviderForTurnAt(threadID, turnID, time.Date(2026, 5, 8, 7, 41, 40, 0, time.UTC)); !found || provider != "toska" {
		t.Fatalf("late provider = %q, found=%t, want toska/true", provider, found)
	}
	if provider, found := timeline.exactProviderForTurnAt(threadID, turnID, time.Date(2026, 5, 8, 7, 41, 4, 0, time.UTC)); found || provider != "" {
		t.Fatalf("pre-evidence provider = %q, found=%t, want empty/false", provider, found)
	}
}

func TestCodexPreservesTimelineAcrossMultipleBareTurns(t *testing.T) {
	home := t.TempDir()
	mustWrite(t, filepath.Join(home, ".codex", "config.toml"), `
[model_providers]
[model_providers.toska]
name = "toska"
base_url = "https://api.toskaxy.xyz/v1"

[model_providers.bcb]
name = "bcb"
base_url = "https://www.aiixiao.shop"
`)
	sessionDir := filepath.Join(home, ".codex", "sessions", "2026", "05", "08")
	mustMkdir(t, sessionDir)
	body := `{"type":"session_meta","timestamp":"2026-05-08T07:07:00Z","payload":{"id":"019e0000-0000-7000-8000-000000000004","model_provider":"toska","cwd":"/repo"}}` + "\n" +
		`{"type":"turn_context","timestamp":"2026-05-08T07:08:00Z","payload":{"id":"turn-a","model":"gpt-5.5","cwd":"/repo"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-05-08T07:08:10Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":100,"total_tokens":100}}}}` + "\n" +
		`{"type":"turn_context","timestamp":"2026-05-08T07:10:30Z","payload":{"id":"turn-b","model":"gpt-5.5","cwd":"/repo"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-05-08T07:10:54Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":110,"total_tokens":110}}}}` + "\n" +
		`{"type":"turn_context","timestamp":"2026-05-08T07:14:53Z","payload":{"id":"turn-c","model":"gpt-5.5","cwd":"/repo"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-05-08T07:15:40Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":120,"total_tokens":120}}}}` + "\n" +
		`{"type":"turn_context","timestamp":"2026-05-08T07:37:18Z","payload":{"id":"turn-d","model":"gpt-5.5","cwd":"/repo"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-05-08T07:37:40Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":130,"total_tokens":130}}}}` + "\n"
	mustWrite(t, filepath.Join(sessionDir, "rollout-2026-05-08T15-07-00-019e0000-0000-7000-8000-000000000004.jsonl"), body)
	logDir := filepath.Join(home, ".codex", "log")
	mustMkdir(t, logDir)
	mustWrite(t, filepath.Join(logDir, "codex-tui.log"),
		`2026-05-08T07:08:05.000000Z  INFO session_loop{thread_id=019e0000-0000-7000-8000-000000000004}:turn{turn.id=turn-a model=gpt-5.5}:endpoint_session.stream_with{http.method=POST api.path="responses"}: POST to https://api.toskaxy.xyz/v1/responses: {"model":"gpt-5.5"}`+"\n"+
			`2026-05-08T07:37:20.000000Z  INFO session_loop{thread_id=019e0000-0000-7000-8000-000000000004}:turn{turn.id=turn-d model=gpt-5.5}:endpoint_session.stream_with{http.method=POST api.path="responses"}: POST to https://www.aiixiao.shop/responses: {"model":"gpt-5.5"}`+"\n")

	events, err := NewCodex(Options{Home: home}).Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 4 {
		t.Fatalf("len(events) = %d, want 4", len(events))
	}
	if events[1].Provider != "toska" || events[1].ProviderAttribution != string(usage.ProviderAttributionInferredTimeline) {
		t.Fatalf("first unlabeled turn should stay on earlier inferred provider: %+v", events[1])
	}
	if events[2].Provider != "toska" || events[2].ProviderAttribution != string(usage.ProviderAttributionInferredTimeline) {
		t.Fatalf("second unlabeled turn should also keep timeline provider: %+v", events[2])
	}
	if events[3].Provider != "bcb" || events[3].ProviderAttribution != string(usage.ProviderAttributionExactRequest) {
		t.Fatalf("evidence turn should remain exact bcb: %+v", events[3])
	}
}

func TestCodexKeepsLongBareSegmentsOnEarlierTimelineProvider(t *testing.T) {
	home := t.TempDir()
	mustWrite(t, filepath.Join(home, ".codex", "config.toml"), `
[model_providers]
[model_providers.toska]
name = "toska"
base_url = "https://api.toskaxy.xyz/v1"

[model_providers.bcb]
name = "bcb"
base_url = "https://www.aiixiao.shop"
`)
	sessionDir := filepath.Join(home, ".codex", "sessions", "2026", "05", "08")
	mustMkdir(t, sessionDir)
	body := `{"type":"session_meta","timestamp":"2026-05-08T07:07:00Z","payload":{"id":"019e0000-0000-7000-8000-000000000006","model_provider":"toska","cwd":"/repo"}}` + "\n" +
		`{"type":"turn_context","timestamp":"2026-05-08T07:08:00Z","payload":{"id":"turn-a","model":"gpt-5.5","cwd":"/repo"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-05-08T07:08:10Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":100,"total_tokens":100}}}}` + "\n" +
		`{"type":"turn_context","timestamp":"2026-05-08T07:10:30Z","payload":{"id":"turn-b","model":"gpt-5.5","cwd":"/repo"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-05-08T07:10:54Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":110,"total_tokens":110}}}}` + "\n" +
		`{"type":"turn_context","timestamp":"2026-05-08T07:14:53Z","payload":{"id":"turn-c","model":"gpt-5.5","cwd":"/repo"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-05-08T07:15:40Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":120,"total_tokens":120}}}}` + "\n" +
		`{"type":"turn_context","timestamp":"2026-05-08T07:19:40Z","payload":{"id":"turn-d","model":"gpt-5.5","cwd":"/repo"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-05-08T07:19:46Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":130,"total_tokens":130}}}}` + "\n" +
		`{"type":"turn_context","timestamp":"2026-05-08T07:37:18Z","payload":{"id":"turn-e","model":"gpt-5.5","cwd":"/repo"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-05-08T07:37:40Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":140,"total_tokens":140}}}}` + "\n"
	mustWrite(t, filepath.Join(sessionDir, "rollout-2026-05-08T15-07-00-019e0000-0000-7000-8000-000000000006.jsonl"), body)
	logDir := filepath.Join(home, ".codex", "log")
	mustMkdir(t, logDir)
	mustWrite(t, filepath.Join(logDir, "codex-tui.log"),
		`2026-05-08T07:08:05.000000Z  INFO session_loop{thread_id=019e0000-0000-7000-8000-000000000006}:turn{turn.id=turn-a model=gpt-5.5}:endpoint_session.stream_with{http.method=POST api.path="responses"}: POST to https://api.toskaxy.xyz/v1/responses: {"model":"gpt-5.5"}`+"\n"+
			`2026-05-08T07:37:39.000000Z  INFO session_loop{thread_id=019e0000-0000-7000-8000-000000000006}:turn{turn.id=turn-e model=gpt-5.5}:endpoint_session.stream_with{http.method=POST api.path="responses"}: POST to https://www.aiixiao.shop/responses: {"model":"gpt-5.5"}`+"\n")

	events, err := NewCodex(Options{Home: home}).Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 5 {
		t.Fatalf("len(events) = %d, want 5", len(events))
	}
	if events[1].Provider != "toska" || events[1].ProviderAttribution != string(usage.ProviderAttributionInferredTimeline) {
		t.Fatalf("long bare segment should keep first unlabeled turn on earlier provider: %+v", events[1])
	}
	if events[2].Provider != "toska" || events[2].ProviderAttribution != string(usage.ProviderAttributionInferredTimeline) {
		t.Fatalf("long bare segment should keep second unlabeled turn on earlier provider: %+v", events[2])
	}
	if events[3].Provider != "toska" || events[3].ProviderAttribution != string(usage.ProviderAttributionInferredTimeline) {
		t.Fatalf("third unlabeled turn should also keep earlier provider: %+v", events[3])
	}
	if events[4].Provider != "bcb" || events[4].ProviderAttribution != string(usage.ProviderAttributionExactRequest) {
		t.Fatalf("evidence turn should remain exact bcb: %+v", events[4])
	}
}

func TestCodexIgnoresUnknownOrAmbiguousRequestHosts(t *testing.T) {
	home := t.TempDir()
	mustWrite(t, filepath.Join(home, ".codex", "config.toml"), `
[model_providers]
[model_providers.team_a]
name = "team-a"
base_url = "https://shared.example/v1"

[model_providers.team_b]
name = "team-b"
base_url = "https://shared.example"
`)
	sessionDir := filepath.Join(home, ".codex", "sessions", "2026", "05", "08")
	mustMkdir(t, sessionDir)
	body := `{"type":"session_meta","timestamp":"2026-05-08T01:00:00Z","payload":{"id":"019e0000-0000-7000-8000-000000000001","model_provider":"team-a","cwd":"/repo"}}` + "\n" +
		`{"type":"turn_context","timestamp":"2026-05-08T01:00:01Z","payload":{"id":"turn-a","model":"gpt-5.5","cwd":"/repo"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-05-08T01:00:02Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":100,"total_tokens":100}}}}` + "\n" +
		`{"type":"turn_context","timestamp":"2026-05-08T01:01:01Z","payload":{"id":"turn-b","model":"gpt-5.5","cwd":"/repo"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-05-08T01:01:02Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":10,"total_tokens":10}}}}` + "\n"
	mustWrite(t, filepath.Join(sessionDir, "rollout-2026-05-08T09-00-00-019e0000-0000-7000-8000-000000000001.jsonl"), body)
	logDir := filepath.Join(home, ".codex", "log")
	mustMkdir(t, logDir)
	mustWrite(t, filepath.Join(logDir, "codex-tui.log"),
		`2026-05-08T01:00:01.500000Z  INFO session_loop{thread_id=019e0000-0000-7000-8000-000000000001}:turn{turn.id=turn-a model=gpt-5.5}:endpoint_session.stream_with{http.method=POST api.path="responses"}: POST to https://shared.example/v1/responses: {"model":"gpt-5.5"}`+"\n"+
			`2026-05-08T01:01:01.500000Z  INFO session_loop{thread_id=019e0000-0000-7000-8000-000000000001}:turn{turn.id=turn-b model=gpt-5.5}:endpoint_session.stream_with{http.method=POST api.path="responses"}: POST to https://rotated.example/responses: {"model":"gpt-5.5"}`+"\n")

	events, err := NewCodex(Options{Home: home}).Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("len(events) = %d, want 2", len(events))
	}
	if events[0].Provider != "team-a" || events[1].Provider != "team-a" {
		t.Fatalf("ambiguous or unknown hosts should not override session provider: %+v", events)
	}
}

func TestCodexIgnoresListModelsProviderEvidence(t *testing.T) {
	home := t.TempDir()
	mustWrite(t, filepath.Join(home, ".codex", "config.toml"), `
[model_providers]
[model_providers.toska]
name = "toska"
base_url = "https://api.toskaxy.xyz/v1"

[model_providers.bcb]
name = "bcb"
base_url = "https://www.aiixiao.shop"
`)
	sessionDir := filepath.Join(home, ".codex", "sessions", "2026", "05", "08")
	mustMkdir(t, sessionDir)
	body := `{"type":"session_meta","timestamp":"2026-05-08T10:00:00Z","payload":{"id":"019e0000-0000-7000-8000-000000000005","model_provider":"bcb","cwd":"/repo"}}` + "\n" +
		`{"type":"turn_context","timestamp":"2026-05-08T10:00:01Z","payload":{"id":"turn-a","model":"gpt-5.5","cwd":"/repo"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-05-08T10:00:02Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":100,"total_tokens":100}}}}` + "\n"
	mustWrite(t, filepath.Join(sessionDir, "rollout-2026-05-08T18-00-00-019e0000-0000-7000-8000-000000000005.jsonl"), body)
	logDir := filepath.Join(home, ".codex", "log")
	mustMkdir(t, logDir)
	mustWrite(t, filepath.Join(logDir, "codex-tui.log"),
		`2026-05-08T10:00:01.500000Z  INFO session_loop{thread_id=019e0000-0000-7000-8000-000000000005}:turn{turn.id=turn-a model=gpt-5.5}:run_turn:list_models{refresh_strategy=online_if_uncached}:endpoint_session.execute_with{http.method=GET api.path="models"}: GET to https://api.toskaxy.xyz/v1/models`+"\n")

	events, err := NewCodex(Options{Home: home}).Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(events))
	}
	if events[0].Provider != "bcb" {
		t.Fatalf("list_models evidence should not override session provider: %+v", events[0])
	}
	if events[0].ProviderAttribution != string(usage.ProviderAttributionSessionFallback) {
		t.Fatalf("list_models attribution = %q, want session_fallback", events[0].ProviderAttribution)
	}
}

func TestCodexFallsBackToLastTokenUsageWhenTotalMissing(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".codex", "sessions", "2026", "05", "08")
	mustMkdir(t, dir)
	body := `{"type":"session_meta","timestamp":"2026-05-08T01:00:00Z","payload":{"model_provider":"openai","cwd":"/repo"}}` + "\n" +
		`{"type":"turn_context","timestamp":"2026-05-08T01:00:01Z","payload":{"id":"turn-a","model":"gpt-5.4","cwd":"/repo"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-05-08T01:00:02Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":20,"output_tokens":4}}}}` + "\n"
	mustWrite(t, filepath.Join(dir, "rollout.jsonl"), body)
	events, err := NewCodex(Options{Home: home}).Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Usage.Input != 20 || events[0].Usage.Output != 4 {
		t.Fatalf("unexpected fallback token count: %+v", events)
	}
}

func TestCodexReadsArchivedSessions(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".codex", "archived_sessions")
	mustMkdir(t, dir)
	body := `{"type":"session_meta","timestamp":"2026-05-08T01:00:00Z","payload":{"id":"archive-thread","model_provider":"openai","cwd":"/repo"}}` + "\n" +
		`{"type":"turn_context","timestamp":"2026-05-08T01:00:01Z","payload":{"id":"turn-a","model":"gpt-5.4","cwd":"/repo"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-05-08T01:00:02Z","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":20,"output_tokens":4}}}}` + "\n"
	mustWrite(t, filepath.Join(dir, "rollout.jsonl"), body)
	events, err := NewCodex(Options{Home: home}).Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].ThreadID != "archive-thread" {
		t.Fatalf("unexpected archived events: %+v", events)
	}
}

func TestCodexKeepsMatchingTokenCountsAcrossTurns(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".codex", "sessions", "2026", "05", "08")
	mustMkdir(t, dir)
	body := `{"type":"session_meta","timestamp":"2026-05-08T01:00:00Z","payload":{"model_provider":"openai","cwd":"/repo"}}` + "\n" +
		`{"type":"turn_context","timestamp":"2026-05-08T01:00:01Z","payload":{"id":"turn-a","model":"gpt-5.4","cwd":"/repo"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-05-08T01:00:02Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":20,"output_tokens":4}}}}` + "\n" +
		`{"type":"turn_context","timestamp":"2026-05-08T01:01:01Z","payload":{"id":"turn-b","model":"gpt-5.4","cwd":"/repo"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-05-08T01:01:02Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":20,"output_tokens":4}}}}` + "\n"
	mustWrite(t, filepath.Join(dir, "rollout.jsonl"), body)
	events, err := NewCodex(Options{Home: home}).Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("len(events) = %d, want 2", len(events))
	}
	if events[0].ID == events[1].ID {
		t.Fatalf("matching token counts across turns must keep unique ids: %q", events[0].ID)
	}
}

func TestCodexThreadTitlePriorityAndMetadata(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".codex", "sessions", "2026", "05", "08")
	path := filepath.Join(dir, "rollout-2026-05-08T01-00-00-019e0000-0000-7000-8000-000000000001.jsonl")
	body := `{"type":"session_meta","timestamp":"2026-05-08T01:00:00Z","payload":{"id":"019e0000-0000-7000-8000-000000000001","model_provider":"openai","cwd":"/repo"}}` + "\n" +
		`{"type":"response_item","timestamp":"2026-05-08T01:00:01Z","payload":{"type":"message","role":"user","content":"First real user message"}}` + "\n" +
		`{"type":"response_item","timestamp":"2026-05-08T01:00:02Z","payload":{"type":"message","role":"assistant","content":"AI summary title"}}` + "\n" +
		`{"type":"custom-title","timestamp":"2026-05-08T01:00:03Z","customTitle":"Custom title"}` + "\n" +
		`{"type":"turn_context","timestamp":"2026-05-08T01:00:04Z","payload":{"id":"turn-a","model":"gpt-5.4","cwd":"/repo"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-05-08T01:00:05Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":10,"output_tokens":2}}}}` + "\n"
	mustWrite(t, path, body)
	events, err := NewCodex(Options{Home: home}).Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(events))
	}
	event := events[0]
	if event.ThreadID != "019e0000-0000-7000-8000-000000000001" || event.ThreadName != "Custom title" || event.ThreadSource != path || event.ThreadCreatedAt.IsZero() || event.ThreadLastActiveAt.IsZero() {
		t.Fatalf("unexpected thread metadata: %+v", event)
	}
}

func TestCodexThreadTitleFallsBackToAISummaryThenUserThenCWD(t *testing.T) {
	home := t.TempDir()
	base := filepath.Join(home, ".codex", "sessions", "2026", "05", "08")
	mustWrite(t, filepath.Join(home, ".codex", "session_index.jsonl"),
		`{"id":"ai-thread","thread_name":"Codex UI title"}`+"\n")
	mustWrite(t, filepath.Join(base, "ai.jsonl"),
		`{"type":"session_meta","timestamp":"2026-05-08T01:00:00Z","payload":{"id":"ai-thread","cwd":"/repo-a"}}`+"\n"+
			`{"type":"response_item","timestamp":"2026-05-08T01:00:01Z","payload":{"type":"message","role":"user","content":"First user"}}`+"\n"+
			`{"type":"response_item","timestamp":"2026-05-08T01:00:02Z","payload":{"type":"message","role":"assistant","content":"AI summary"}}`+"\n"+
			`{"type":"turn_context","timestamp":"2026-05-08T01:00:03Z","payload":{"model":"gpt-5.4"}}`+"\n"+
			`{"type":"event_msg","timestamp":"2026-05-08T01:00:04Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":1}}}}`+"\n")
	mustWrite(t, filepath.Join(base, "user.jsonl"),
		`{"type":"session_meta","timestamp":"2026-05-08T02:00:00Z","payload":{"id":"user-thread","cwd":"/repo-b"}}`+"\n"+
			`{"type":"response_item","timestamp":"2026-05-08T02:00:01Z","payload":{"type":"message","role":"user","content":"User title"}}`+"\n"+
			`{"type":"turn_context","timestamp":"2026-05-08T02:00:03Z","payload":{"model":"gpt-5.4"}}`+"\n"+
			`{"type":"event_msg","timestamp":"2026-05-08T02:00:04Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":1}}}}`+"\n")
	mustWrite(t, filepath.Join(base, "summary.jsonl"),
		`{"type":"session_meta","timestamp":"2026-05-08T02:30:00Z","payload":{"id":"summary-thread","cwd":"/repo-summary"}}`+"\n"+
			`{"type":"response_item","timestamp":"2026-05-08T02:30:01Z","payload":{"type":"message","role":"user","content":"First user summary fallback"}}`+"\n"+
			`{"type":"response_item","timestamp":"2026-05-08T02:30:02Z","payload":{"type":"thread-title","title":"Explicit AI title"}}`+"\n"+
			`{"type":"turn_context","timestamp":"2026-05-08T02:30:03Z","payload":{"model":"gpt-5.4"}}`+"\n"+
			`{"type":"event_msg","timestamp":"2026-05-08T02:30:04Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":1}}}}`+"\n")
	mustWrite(t, filepath.Join(base, "assistant.jsonl"),
		`{"type":"session_meta","timestamp":"2026-05-08T02:40:00Z","payload":{"id":"assistant-thread","cwd":"/repo-assistant"}}`+"\n"+
			`{"type":"response_item","timestamp":"2026-05-08T02:40:01Z","payload":{"type":"message","role":"user","content":"First user wins"}}`+"\n"+
			`{"type":"response_item","timestamp":"2026-05-08T02:40:02Z","payload":{"type":"message","role":"assistant","content":"Regular assistant response, not a title"}}`+"\n"+
			`{"type":"turn_context","timestamp":"2026-05-08T02:40:03Z","payload":{"model":"gpt-5.4"}}`+"\n"+
			`{"type":"event_msg","timestamp":"2026-05-08T02:40:04Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":1}}}}`+"\n")
	mustWrite(t, filepath.Join(base, "ide.jsonl"),
		`{"type":"session_meta","timestamp":"2026-05-08T02:50:00Z","payload":{"id":"ide-thread","cwd":"/repo-ide"}}`+"\n"+
			`{"type":"response_item","timestamp":"2026-05-08T02:50:01Z","payload":{"type":"message","role":"user","content":"# Context from my IDE setup:\n\n## Active file: src/main.go"}}`+"\n"+
			`{"type":"response_item","timestamp":"2026-05-08T02:50:02Z","payload":{"type":"message","role":"user","content":"Real user title"}}`+"\n"+
			`{"type":"turn_context","timestamp":"2026-05-08T02:50:03Z","payload":{"model":"gpt-5.4","summary":"none"}}`+"\n"+
			`{"type":"event_msg","timestamp":"2026-05-08T02:50:04Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":1}}}}`+"\n")
	mustWrite(t, filepath.Join(base, "aborted.jsonl"),
		`{"type":"session_meta","timestamp":"2026-05-08T02:55:00Z","payload":{"id":"aborted-thread","cwd":"/repo-aborted"}}`+"\n"+
			`{"type":"response_item","timestamp":"2026-05-08T02:55:01Z","payload":{"type":"message","role":"user","content":"<turn_aborted> The user interrupted the previous turn on purpose."}}`+"\n"+
			`{"type":"response_item","timestamp":"2026-05-08T02:55:02Z","payload":{"type":"message","role":"user","content":"Actual request title"}}`+"\n"+
			`{"type":"turn_context","timestamp":"2026-05-08T02:55:03Z","payload":{"model":"gpt-5.4"}}`+"\n"+
			`{"type":"event_msg","timestamp":"2026-05-08T02:55:04Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":1}}}}`+"\n")
	mustWrite(t, filepath.Join(base, "cwd.jsonl"),
		`{"type":"session_meta","timestamp":"2026-05-08T03:00:00Z","payload":{"id":"cwd-thread","cwd":"/tmp/my-project"}}`+"\n"+
			`{"type":"turn_context","timestamp":"2026-05-08T03:00:03Z","payload":{"model":"gpt-5.4"}}`+"\n"+
			`{"type":"event_msg","timestamp":"2026-05-08T03:00:04Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":1}}}}`+"\n")
	events, err := NewCodex(Options{Home: home}).Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]string{}
	for _, event := range events {
		names[event.ThreadID] = event.ThreadName
	}
	if names["ai-thread"] != "Codex UI title" || names["summary-thread"] != "Explicit AI title" || names["assistant-thread"] != "First user wins" || names["ide-thread"] != "Real user title" || names["aborted-thread"] != "Actual request title" || names["user-thread"] != "User title" || names["cwd-thread"] != "my-project" {
		t.Fatalf("unexpected thread names: %+v", names)
	}
}

type streamingSource struct {
	scanStarted   bool
	afterCallback bool
}

func (s *streamingSource) Name() usage.Tool {
	return usage.ToolCodex
}

func (s *streamingSource) Read(ctx context.Context) ([]usage.UsageEvent, error) {
	return nil, errors.New("Read should not be called")
}

func (s *streamingSource) Scan(ctx context.Context, handle func(usage.UsageEvent) error) error {
	s.scanStarted = true
	if err := handle(usage.UsageEvent{Tool: usage.ToolCodex, Model: "gpt-5.4"}); err != nil {
		return err
	}
	s.afterCallback = true
	return nil
}

func TestGeminiReadsConfiguredTelemetryOutfile(t *testing.T) {
	home := t.TempDir()
	mustMkdir(t, filepath.Join(home, ".gemini"))
	outfile := filepath.Join(home, ".gemini", "telemetry.log")
	settings := `{"telemetry":{"enabled":true,"target":"local","outfile":"` + outfile + `","logPrompts":false}}`
	mustWrite(t, filepath.Join(home, ".gemini", "settings.json"), settings)
	line := `{"timestamp":"2026-05-08T01:00:00Z","name":"gemini_cli.api_response","attributes":{"model":"gemini-2.5-pro","auth_type":"oauth","input_token_count":11,"output_token_count":5,"cached_content_token_count":3,"thoughts_token_count":2,"tool_token_count":1,"total_token_count":20,"prompt_id":"p1"}}`
	mustWrite(t, outfile, line+"\n")
	events, err := NewGemini(Options{Home: home}).Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(events))
	}
	if events[0].Model != "gemini-2.5-pro" || events[0].Provider != "oauth" || events[0].Usage.Tool != 1 {
		t.Fatalf("unexpected event: %+v", events[0])
	}
}

func TestGeminiSkipsUsageWithoutTimestamp(t *testing.T) {
	home := t.TempDir()
	mustMkdir(t, filepath.Join(home, ".gemini"))
	outfile := filepath.Join(home, ".gemini", "telemetry.log")
	settings := `{"telemetry":{"enabled":true,"target":"local","outfile":"` + outfile + `","logPrompts":false}}`
	mustWrite(t, filepath.Join(home, ".gemini", "settings.json"), settings)
	line := `{"name":"gemini_cli.api_response","attributes":{"model":"gemini-2.5-pro","auth_type":"oauth","input_token_count":11,"output_token_count":5,"prompt_id":"p1"}}`
	mustWrite(t, outfile, line+"\n")
	events, err := NewGemini(Options{Home: home}).Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("len(events) = %d, want 0", len(events))
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func mustWriteSQLite(t *testing.T, path string, statements []string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	sqlite, err := exec.LookPath("sqlite3")
	if err != nil {
		t.Fatalf("sqlite3 not found: %v", err)
	}
	for _, stmt := range statements {
		cmd := exec.Command(sqlite, path, stmt)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("sqlite3 statement failed: %v\nstatement: %s\noutput: %s", err, stmt, string(out))
		}
	}
}
