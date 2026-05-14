package sources

import (
	"context"
	"errors"
	"os"
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
	if events[1].Provider != "openai" || events[1].Model != "gpt-5.4" {
		t.Fatalf("second event should use provider/model from switched turn: %+v", events[1])
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
