package sources

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/MagnumGoYB/aitok/internal/usage"
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

func TestCodexKeepsDistinctTokenCountsWithinTurn(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".codex", "sessions", "2026", "05", "08")
	mustMkdir(t, dir)
	body := `{"type":"session_meta","timestamp":"2026-05-08T01:00:00Z","payload":{"model_provider":"openai","cwd":"/repo"}}` + "\n" +
		`{"type":"turn_context","timestamp":"2026-05-08T01:00:01Z","payload":{"model":"gpt-5.4","cwd":"/repo"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-05-08T01:00:02Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":10,"output_tokens":2}}}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-05-08T01:00:03Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":20,"output_tokens":4}}}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-05-08T01:00:04Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":20,"output_tokens":4}}}}` + "\n"
	mustWrite(t, filepath.Join(dir, "rollout.jsonl"), body)
	events, err := NewCodex(Options{Home: home}).Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("len(events) = %d, want 2", len(events))
	}
	if events[0].Usage.Input != 10 || events[0].Usage.Output != 2 {
		t.Fatalf("unexpected first token count: %+v", events[0])
	}
	if events[1].Usage.Input != 20 || events[1].Usage.Output != 4 {
		t.Fatalf("unexpected second token count: %+v", events[1])
	}
	if got := events[1].Timestamp.Format("15:04:05"); got != "01:00:03" {
		t.Fatalf("timestamp = %s, want first duplicate token_count timestamp", got)
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
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
