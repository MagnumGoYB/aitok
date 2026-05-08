package sources

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestClaudeReadsJSONLAndDeduplicates(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".claude", "projects", "repo")
	mustMkdir(t, dir)
	line := `{"type":"assistant","uuid":"same","timestamp":"2026-05-08T01:02:03Z","cwd":"/repo","message":{"model":"anthropic/claude-sonnet-4.5","usage":{"input_tokens":10,"output_tokens":2,"cache_read_input_tokens":3,"cache_creation_input_tokens":4}}}`
	mustWrite(t, filepath.Join(dir, "session.jsonl"), line+"\n"+line+"\n"+"{bad json}\n")
	events, err := NewClaude(Options{Home: home}).Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(events))
	}
	if events[0].Model != "anthropic/claude-sonnet-4.5" || events[0].Usage.Input != 10 || events[0].Usage.CachedInput != 3 {
		t.Fatalf("unexpected event: %+v", events[0])
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

func TestCodexKeepsLatestTokenSnapshotPerTurn(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".codex", "sessions", "2026", "05", "08")
	mustMkdir(t, dir)
	body := `{"type":"session_meta","timestamp":"2026-05-08T01:00:00Z","payload":{"model_provider":"openai","cwd":"/repo"}}` + "\n" +
		`{"type":"turn_context","timestamp":"2026-05-08T01:00:01Z","payload":{"model":"gpt-5.4","cwd":"/repo"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-05-08T01:00:02Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":10,"output_tokens":2}}}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-05-08T01:00:03Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":20,"output_tokens":4}}}}` + "\n"
	mustWrite(t, filepath.Join(dir, "rollout.jsonl"), body)
	events, err := NewCodex(Options{Home: home}).Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(events))
	}
	if events[0].Usage.Input != 20 || events[0].Usage.Output != 4 {
		t.Fatalf("unexpected latest snapshot: %+v", events[0])
	}
	if got := events[0].Timestamp.Format("15:04:05"); got != "01:00:03" {
		t.Fatalf("timestamp = %s, want latest token_count timestamp", got)
	}
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
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
