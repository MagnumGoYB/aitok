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

type Claude struct {
	Home string
}

func NewClaude(opts Options) Claude {
	return Claude{Home: cleanHome(opts.Home)}
}

func (c Claude) Name() usage.Tool {
	return usage.ToolClaude
}

func (c Claude) Read(ctx context.Context) ([]usage.UsageEvent, error) {
	root := filepath.Join(c.Home, ".claude", "projects")
	var events []usage.UsageEvent
	seen := map[string]struct{}{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d == nil || d.IsDir() || filepath.Ext(path) != ".jsonl" {
			return nil
		}
		return readJSONLines(ctx, path, func(obj map[string]any) error {
			event, ok := c.parseEvent(path, obj)
			if !ok {
				return nil
			}
			if _, exists := seen[event.ID]; exists {
				return nil
			}
			seen[event.ID] = struct{}{}
			events = append(events, event)
			return nil
		})
	})
	return events, err
}

func (c Claude) parseEvent(path string, obj map[string]any) (usage.UsageEvent, bool) {
	if stringValue(obj["type"]) != "assistant" {
		return usage.UsageEvent{}, false
	}
	msg := objectValue(obj["message"])
	if msg == nil {
		return usage.UsageEvent{}, false
	}
	rawUsage := objectValue(msg["usage"])
	if rawUsage == nil {
		return usage.UsageEvent{}, false
	}
	ts, err := time.Parse(time.RFC3339Nano, stringValue(obj["timestamp"]))
	if err != nil {
		return usage.UsageEvent{}, false
	}
	tokens := usage.TokenUsage{
		Input:         intValue(rawUsage["input_tokens"]),
		Output:        intValue(rawUsage["output_tokens"]),
		CachedInput:   intValue(rawUsage["cache_read_input_tokens"]),
		CacheCreation: intValue(rawUsage["cache_creation_input_tokens"]),
	}
	if tokens.NormalizedTotal() == 0 && tokens.CachedInput == 0 {
		return usage.UsageEvent{}, false
	}
	model := usage.Unknown(stringValue(msg["model"]))
	id := stringValue(obj["uuid"])
	if id == "" {
		id = claudeHash(ts, model, tokens)
	}
	return usage.UsageEvent{
		ID:        id,
		Timestamp: ts,
		Tool:      usage.ToolClaude,
		Model:     model,
		Provider:  "unknown",
		CWD:       stringValue(obj["cwd"]),
		Source:    path,
		Usage:     tokens,
	}, true
}

func claudeHash(ts time.Time, model string, tokens usage.TokenUsage) string {
	h := sha1.Sum([]byte(fmt.Sprintf("%s|%s|%d|%d|%d|%d", ts.Format(time.RFC3339Nano), model, tokens.Input, tokens.Output, tokens.CachedInput, tokens.CacheCreation)))
	return hex.EncodeToString(h[:])
}
