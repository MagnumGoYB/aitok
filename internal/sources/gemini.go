package sources

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/MagnumGoYB/aitok/internal/usage"
)

type Gemini struct {
	Home string
}

func NewGemini(opts Options) Gemini {
	return Gemini{Home: cleanHome(opts.Home)}
}

func (g Gemini) Name() usage.Tool {
	return usage.ToolGemini
}

func (g Gemini) Read(ctx context.Context) ([]usage.UsageEvent, error) {
	var events []usage.UsageEvent
	err := g.Scan(ctx, func(event usage.UsageEvent) error {
		events = append(events, event)
		return nil
	})
	return events, err
}

func (g Gemini) Scan(ctx context.Context, handle func(usage.UsageEvent) error) error {
	outfile := g.telemetryOutfile()
	if outfile == "" {
		return nil
	}
	err := readJSONLines(ctx, outfile, func(obj map[string]any) error {
		if event, ok := g.parseEvent(outfile, obj); ok {
			return handle(event)
		}
		return nil
	})
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func (g Gemini) telemetryOutfile() string {
	settingsPath := filepath.Join(g.Home, ".gemini", "settings.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return ""
	}
	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		return ""
	}
	telemetry := objectValue(settings["telemetry"])
	if telemetry == nil {
		return ""
	}
	if enabled, ok := telemetry["enabled"].(bool); ok && !enabled {
		return ""
	}
	outfile := stringValue(telemetry["outfile"])
	return expandHome(g.Home, outfile)
}

func (g Gemini) parseEvent(path string, obj map[string]any) (usage.UsageEvent, bool) {
	flat := map[string]any{}
	flatten("", obj, flat)
	if !isGeminiUsage(flat) {
		return usage.UsageEvent{}, false
	}
	ts := parseAnyTime(firstString(flat, "timestamp", "time", "observedTimestamp", "body.time"))
	if ts.IsZero() {
		return usage.UsageEvent{}, false
	}
	model := firstString(flat, "attributes.model", "model", "attributes.gen_ai.request.model", "gen_ai.request.model")
	provider := firstString(flat, "attributes.auth_type", "auth_type", "attributes.gen_ai.provider.name", "gen_ai.provider.name")
	tokens := usage.TokenUsage{
		Input:       firstInt(flat, "attributes.input_token_count", "input_token_count", "attributes.gen_ai.usage.input_tokens", "gen_ai.usage.input_tokens"),
		Output:      firstInt(flat, "attributes.output_token_count", "output_token_count", "attributes.gen_ai.usage.output_tokens", "gen_ai.usage.output_tokens"),
		CachedInput: firstInt(flat, "attributes.cached_content_token_count", "cached_content_token_count"),
		Reasoning:   firstInt(flat, "attributes.thoughts_token_count", "thoughts_token_count"),
		Tool:        firstInt(flat, "attributes.tool_token_count", "tool_token_count"),
		Total:       firstInt(flat, "attributes.total_token_count", "total_token_count"),
	}
	if tokens.NormalizedTotal() == 0 && tokens.CachedInput == 0 && tokens.Reasoning == 0 {
		return usage.UsageEvent{}, false
	}
	id := firstString(flat, "attributes.prompt_id", "prompt_id", "spanId", "traceId")
	if id == "" {
		id = path + "|" + ts.Format(time.RFC3339Nano) + "|" + model
	}
	return usage.UsageEvent{
		ID:        id,
		Timestamp: ts,
		Tool:      usage.ToolGemini,
		Model:     usage.Unknown(model),
		Provider:  usage.Unknown(provider),
		Source:    path,
		Usage:     tokens,
	}, true
}

func isGeminiUsage(flat map[string]any) bool {
	name := firstString(flat, "name", "event.name", "body.name")
	if strings.Contains(name, "gemini_cli.api_response") || strings.Contains(name, "gen_ai.client.inference.operation.details") {
		return true
	}
	for key := range flat {
		if strings.Contains(key, "gen_ai.usage.input_tokens") || strings.HasSuffix(key, "input_token_count") {
			return true
		}
	}
	return false
}

func flatten(prefix string, value any, out map[string]any) {
	switch v := value.(type) {
	case map[string]any:
		for key, child := range v {
			next := key
			if prefix != "" {
				next = prefix + "." + key
			}
			flatten(next, child, out)
		}
	case []any:
		for _, child := range v {
			flatten(prefix, child, out)
		}
	default:
		out[prefix] = v
	}
}

func firstString(flat map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := stringValue(flat[key]); value != "" {
			return value
		}
	}
	return ""
}

func firstInt(flat map[string]any, keys ...string) int64 {
	for _, key := range keys {
		if value := intValue(flat[key]); value != 0 {
			return value
		}
	}
	return 0
}

func parseAnyTime(raw string) time.Time {
	if raw == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05.000Z"} {
		if ts, err := time.Parse(layout, raw); err == nil {
			return ts
		}
	}
	return time.Time{}
}
