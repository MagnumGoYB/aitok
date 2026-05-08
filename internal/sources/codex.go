package sources

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io/fs"
	"path/filepath"
	"time"

	"github.com/MagnumGoYB/aitok/internal/usage"
)

type Codex struct {
	Home string
}

func NewCodex(opts Options) Codex {
	return Codex{Home: cleanHome(opts.Home)}
}

func (c Codex) Name() usage.Tool {
	return usage.ToolCodex
}

func (c Codex) Read(ctx context.Context) ([]usage.UsageEvent, error) {
	root := filepath.Join(c.Home, ".codex", "sessions")
	var events []usage.UsageEvent
	seen := map[string]int{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d == nil || d.IsDir() || filepath.Ext(path) != ".jsonl" {
			return nil
		}
		state := codexState{provider: "unknown", model: "unknown"}
		return readJSONLines(ctx, path, func(obj map[string]any) error {
			state.update(obj)
			event, ok := c.parseEvent(path, obj, state)
			if ok {
				if index, exists := seen[event.ID]; exists {
					events[index] = event
					return nil
				}
				seen[event.ID] = len(events)
				events = append(events, event)
			}
			return nil
		})
	})
	return events, err
}

type codexState struct {
	provider string
	model    string
	cwd      string
	turnID   string
}

func (s *codexState) update(obj map[string]any) {
	payload := objectValue(obj["payload"])
	if payload == nil {
		return
	}
	switch stringValue(obj["type"]) {
	case "session_meta":
		if provider := stringValue(payload["model_provider"]); provider != "" {
			s.provider = provider
		}
		if cwd := stringValue(payload["cwd"]); cwd != "" {
			s.cwd = cwd
		}
	case "turn_context":
		s.turnID = codexTurnID(obj, payload)
		if model := stringValue(payload["model"]); model != "" {
			s.model = model
		}
		if cwd := stringValue(payload["cwd"]); cwd != "" {
			s.cwd = cwd
		}
	}
}

func (c Codex) parseEvent(path string, obj map[string]any, state codexState) (usage.UsageEvent, bool) {
	payload := objectValue(obj["payload"])
	if payload == nil || stringValue(payload["type"]) != "token_count" {
		return usage.UsageEvent{}, false
	}
	info := objectValue(payload["info"])
	if info == nil {
		return usage.UsageEvent{}, false
	}
	rawUsage := objectValue(info["last_token_usage"])
	if rawUsage == nil {
		return usage.UsageEvent{}, false
	}
	ts, err := time.Parse(time.RFC3339Nano, stringValue(obj["timestamp"]))
	if err != nil {
		return usage.UsageEvent{}, false
	}
	tokens := usage.TokenUsage{
		Input:       intValue(rawUsage["input_tokens"]),
		Output:      intValue(rawUsage["output_tokens"]),
		CachedInput: intValue(rawUsage["cached_input_tokens"]),
		Reasoning:   intValue(rawUsage["reasoning_output_tokens"]),
		Total:       intValue(rawUsage["total_tokens"]),
	}
	if tokens.NormalizedTotal() == 0 && tokens.CachedInput == 0 && tokens.Reasoning == 0 {
		return usage.UsageEvent{}, false
	}
	id := state.turnID
	if id == "" {
		id = codexHash(path, state, tokens)
	}
	return usage.UsageEvent{
		ID:        id,
		Timestamp: ts,
		Tool:      usage.ToolCodex,
		Model:     usage.Unknown(state.model),
		Provider:  usage.Unknown(state.provider),
		CWD:       state.cwd,
		Source:    path,
		Usage:     tokens,
	}, true
}

func codexTurnID(obj map[string]any, payload map[string]any) string {
	if id := stringValue(payload["id"]); id != "" {
		return id
	}
	if id := stringValue(payload["turn_id"]); id != "" {
		return id
	}
	if id := stringValue(obj["id"]); id != "" {
		return id
	}
	if id := stringValue(obj["uuid"]); id != "" {
		return id
	}
	if ts := stringValue(obj["timestamp"]); ts != "" {
		return "turn:" + ts
	}
	return ""
}

func codexHash(path string, state codexState, tokens usage.TokenUsage) string {
	h := sha1.Sum([]byte(fmt.Sprintf("%s|%s|%s|%s|%d|%d|%d|%d|%d", path, state.turnID, state.provider, state.model, tokens.Input, tokens.Output, tokens.CachedInput, tokens.Reasoning, tokens.Total)))
	return hex.EncodeToString(h[:])
}
