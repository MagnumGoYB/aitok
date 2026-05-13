package sources

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
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
	var events []usage.UsageEvent
	err := c.Scan(ctx, func(event usage.UsageEvent) error {
		events = append(events, event)
		return nil
	})
	return events, err
}

func (c Codex) Scan(ctx context.Context, handle func(usage.UsageEvent) error) error {
	roots := []string{
		filepath.Join(c.Home, ".codex", "sessions"),
		filepath.Join(c.Home, ".codex", "archived_sessions"),
	}
	index := readCodexSessionIndex(filepath.Join(c.Home, ".codex", "session_index.jsonl"))
	seen := map[string]struct{}{}
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d == nil || d.IsDir() || filepath.Ext(path) != ".jsonl" {
				return nil
			}
			meta := parseCodexThreadMeta(path)
			if meta.Skip {
				return nil
			}
			if title := index[meta.ID]; title != "" {
				meta.Name = chooseThreadTitle(title, meta.Name)
			}
			state := codexState{provider: "unknown", model: "unknown", thread: meta}
			return readJSONLines(ctx, path, func(obj map[string]any) error {
				state.update(obj)
				event, ok := c.parseEvent(path, obj, &state)
				if ok {
					if _, exists := seen[event.ID]; exists {
						return nil
					}
					seen[event.ID] = struct{}{}
					return handle(event)
				}
				return nil
			})
		})
		if err != nil {
			return err
		}
	}
	return nil
}

type codexCumulativeUsage struct {
	Input       int64
	CachedInput int64
	Output      int64
	Reasoning   int64
	Total       int64
}

func (u codexCumulativeUsage) delta(prev *codexCumulativeUsage) usage.TokenUsage {
	if prev == nil {
		return usage.TokenUsage{
			Input:       u.Input,
			Output:      u.Output,
			CachedInput: u.CachedInput,
			Reasoning:   u.Reasoning,
			Total:       normalizedCodexTotal(u),
		}
	}
	return usage.TokenUsage{
		Input:       saturatingSub(u.Input, prev.Input),
		Output:      saturatingSub(u.Output, prev.Output),
		CachedInput: saturatingSub(u.CachedInput, prev.CachedInput),
		Reasoning:   saturatingSub(u.Reasoning, prev.Reasoning),
		Total:       saturatingSub(normalizedCodexTotal(u), normalizedCodexTotal(*prev)),
	}
}

func normalizedCodexTotal(u codexCumulativeUsage) int64 {
	if u.Total > 0 {
		return u.Total
	}
	return u.Input + u.Output + u.Reasoning
}

func saturatingSub(current, previous int64) int64 {
	if current <= previous {
		return 0
	}
	return current - previous
}

func parseCodexCumulativeUsage(raw map[string]any) (codexCumulativeUsage, bool) {
	if raw == nil {
		return codexCumulativeUsage{}, false
	}
	tokens := codexCumulativeUsage{
		Input:       intValue(raw["input_tokens"]),
		Output:      intValue(raw["output_tokens"]),
		CachedInput: firstIntValue(raw, "cached_input_tokens", "cache_read_input_tokens"),
		Reasoning:   intValue(raw["reasoning_output_tokens"]),
		Total:       intValue(raw["total_tokens"]),
	}
	if normalizedCodexTotal(tokens) == 0 && tokens.CachedInput == 0 {
		return codexCumulativeUsage{}, false
	}
	return tokens, true
}

func firstIntValue(obj map[string]any, keys ...string) int64 {
	for _, key := range keys {
		if value := intValue(obj[key]); value != 0 {
			return value
		}
	}
	return 0
}

func normalizeCodexModel(raw string) string {
	name := strings.ToLower(strings.TrimSpace(raw))
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		name = name[idx+1:]
	}
	if len(name) > 11 {
		suffix := name[len(name)-11:]
		if suffix[0] == '-' &&
			allDigits(suffix[1:5]) &&
			suffix[5] == '-' &&
			allDigits(suffix[6:8]) &&
			suffix[8] == '-' &&
			allDigits(suffix[9:11]) {
			name = name[:len(name)-11]
		}
	}
	if idx := strings.LastIndex(name, "-"); idx >= 0 && len(name)-idx == 9 && allDigits(name[idx+1:]) {
		name = name[:idx]
	}
	if name == "" {
		return "unknown"
	}
	return name
}

func allDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

type codexState struct {
	provider   string
	model      string
	cwd        string
	turnID     string
	eventIndex int
	prevTotal  *codexCumulativeUsage
	thread     threadMeta
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
			s.model = normalizeCodexModel(model)
		}
		if cwd := stringValue(payload["cwd"]); cwd != "" {
			s.cwd = cwd
		}
	}
}

func (c Codex) parseEvent(path string, obj map[string]any, state *codexState) (usage.UsageEvent, bool) {
	payload := objectValue(obj["payload"])
	if payload == nil || stringValue(payload["type"]) != "token_count" {
		return usage.UsageEvent{}, false
	}
	info := objectValue(payload["info"])
	if info == nil {
		return usage.UsageEvent{}, false
	}
	if model := firstNonEmptyString(info, "model", "model_name"); model != "" {
		state.model = normalizeCodexModel(model)
	} else if model := stringValue(payload["model"]); model != "" {
		state.model = normalizeCodexModel(model)
	}
	ts, err := time.Parse(time.RFC3339Nano, stringValue(obj["timestamp"]))
	if err != nil {
		return usage.UsageEvent{}, false
	}
	tokens, ok := state.codexTokenDelta(info)
	if !ok {
		return usage.UsageEvent{}, false
	}
	tokens.CachedInput = minInt64(tokens.CachedInput, tokens.Input)
	if tokens.NormalizedTotal() == 0 && tokens.CachedInput == 0 {
		return usage.UsageEvent{}, false
	}
	state.eventIndex++
	id := codexEventID(state.thread.ID, state.turnID, state.eventIndex, ts, tokens)
	return usage.UsageEvent{
		ID:                 id,
		Timestamp:          ts,
		Tool:               usage.ToolCodex,
		Model:              normalizeCodexModel(state.model),
		Provider:           usage.Unknown(state.provider),
		CWD:                state.cwd,
		Source:             path,
		ThreadID:           state.thread.ID,
		ThreadName:         state.thread.Name,
		ThreadSource:       state.thread.Source,
		ThreadCreatedAt:    state.thread.CreatedAt,
		ThreadLastActiveAt: state.thread.LastActiveAt,
		Usage:              tokens,
	}, true
}

func (s *codexState) codexTokenDelta(info map[string]any) (usage.TokenUsage, bool) {
	if total, ok := parseCodexCumulativeUsage(objectValue(info["total_token_usage"])); ok {
		tokens := total.delta(s.prevTotal)
		current := total
		s.prevTotal = &current
		return tokens, true
	}
	if last, ok := parseCodexCumulativeUsage(objectValue(info["last_token_usage"])); ok {
		return usage.TokenUsage{
			Input:       last.Input,
			Output:      last.Output,
			CachedInput: last.CachedInput,
			Reasoning:   last.Reasoning,
			Total:       normalizedCodexTotal(last),
		}, true
	}
	return usage.TokenUsage{}, false
}

func minInt64(left, right int64) int64 {
	if left < right {
		return left
	}
	return right
}

func codexEventID(threadID, turnID string, eventIndex int, ts time.Time, tokens usage.TokenUsage) string {
	if threadID == "" {
		threadID = "unknown"
	}
	if turnID == "" {
		turnID = ts.Format(time.RFC3339Nano)
	}
	return fmt.Sprintf("codex:%s:%s:%d:%d:%d:%d:%d", threadID, turnID, eventIndex, tokens.Input, tokens.Output, tokens.CachedInput, tokens.NormalizedTotal())
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
