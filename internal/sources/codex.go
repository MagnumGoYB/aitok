package sources

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/MagnumGoYB/aitok/internal/usage"
)

type Codex struct {
	Home             string
	WindowStart      time.Time
	WindowEnd        time.Time
	providerTimeline codexProviderTimeline
}

func NewCodex(opts Options) Codex {
	return Codex{Home: cleanHome(opts.Home), WindowStart: opts.WindowStart, WindowEnd: opts.WindowEnd}
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
	targets := c.collectProviderThreads(ctx, roots)
	c.providerTimeline = readCodexProviderTimeline(ctx, c.Home, targets)
	index := readCodexSessionIndex(filepath.Join(c.Home, ".codex", "session_index.jsonl"))
	seen := map[string]struct{}{}
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d == nil || d.IsDir() || filepath.Ext(path) != ".jsonl" {
				return nil
			}
			if !c.fileMayOverlapWindow(path) {
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
			var pending *codexBufferedTurn
			var turns []*codexBufferedTurn
			readErr := readJSONLines(ctx, path, func(obj map[string]any) error {
				if pending != nil && codexTurnContextBoundaryID(obj) != "" && codexTurnContextBoundaryID(obj) != pending.id {
					turns = append(turns, pending)
					pending = nil
				}
				state.update(obj)
				event, ok := c.parseBufferedEvent(obj, &state)
				if ok {
					if pending != nil && event.turnID != "" && event.turnID != pending.id {
						turns = append(turns, pending)
						pending = nil
					}
					pending = appendCodexBufferedTurn(pending, state, event)
				}
				return nil
			})
			if readErr != nil {
				return readErr
			}
			if pending != nil {
				turns = append(turns, pending)
			}
			resolved := c.resolveBufferedTurnProviders(state.thread.ID, turns)
			for i, turn := range turns {
				if err := c.flushBufferedTurn(path, &state, turn, resolved[i], seen, handle); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func appendCodexBufferedTurn(pending *codexBufferedTurn, state codexState, event codexBufferedEvent) *codexBufferedTurn {
	if pending == nil || pending.id != event.turnID {
		startedAt := state.turnStartedAt
		if startedAt.IsZero() || state.turnID != event.turnID {
			startedAt = event.at
		}
		pending = &codexBufferedTurn{
			id:                  event.turnID,
			startedAt:           startedAt,
			endedAt:             event.at,
			model:               event.model,
			provider:            state.provider,
			providerFromModel:   state.providerFromModel,
			providerAttribution: providerAttributionFromState(state),
			cwd:                 event.cwd,
		}
	}
	pending.endedAt = event.at
	if pending.startedAt.IsZero() {
		pending.startedAt = event.at
	}
	if event.model != "" {
		pending.model = event.model
	}
	if event.cwd != "" {
		pending.cwd = event.cwd
	}
	if state.provider != "" {
		pending.provider = state.provider
	}
	if state.providerFromModel {
		pending.providerFromModel = true
		pending.providerAttribution = string(usage.ProviderAttributionModel)
	} else if pending.providerAttribution == "" {
		pending.providerAttribution = providerAttributionFromState(state)
	}
	pending.events = append(pending.events, event)
	return pending
}

func (c Codex) flushBufferedTurn(path string, state *codexState, pending *codexBufferedTurn, resolved codexResolvedProvider, seen map[string]struct{}, handle func(usage.UsageEvent) error) error {
	for _, event := range pending.events {
		usageEvent := c.codexUsageEvent(path, state, event.turnID, event.at, event.tokens, event.model, resolved.Provider, resolved.Attribution, event.cwd)
		if _, exists := seen[usageEvent.ID]; exists {
			continue
		}
		seen[usageEvent.ID] = struct{}{}
		if err := handle(usageEvent); err != nil {
			return err
		}
	}
	return nil
}

func (c Codex) collectProviderThreads(ctx context.Context, roots []string) codexProviderTargets {
	targets := newCodexProviderTargets()
	for _, root := range roots {
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			if err != nil || d == nil || d.IsDir() || filepath.Ext(path) != ".jsonl" {
				return nil
			}
			if !c.fileMayOverlapWindow(path) {
				return nil
			}
			meta := parseCodexThreadMeta(path)
			targets.addThread(meta.ID)
			return nil
		})
	}
	return targets
}

func (c Codex) fileMayOverlapWindow(path string) bool {
	if c.WindowStart.IsZero() {
		return true
	}
	info, err := os.Stat(path)
	if err != nil {
		return true
	}
	return !info.ModTime().Before(c.WindowStart)
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

func parseCodexModel(raw string) (model string, provider string) {
	name := strings.ToLower(strings.TrimSpace(raw))
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		provider = strings.TrimSpace(name[:idx])
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
		name = "unknown"
	}
	return name, provider
}

func normalizeCodexModel(raw string) string {
	model, _ := parseCodexModel(raw)
	return model
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
	provider          string
	sessionProvider   string
	providerFromModel bool
	model             string
	cwd               string
	turnID            string
	turnStartedAt     time.Time
	eventIndex        int
	prevTotal         *codexCumulativeUsage
	thread            threadMeta
}

type codexBufferedEvent struct {
	turnID string
	at     time.Time
	model  string
	cwd    string
	tokens usage.TokenUsage
}

type codexBufferedTurn struct {
	id                  string
	startedAt           time.Time
	endedAt             time.Time
	model               string
	provider            string
	providerFromModel   bool
	providerAttribution string
	cwd                 string
	events              []codexBufferedEvent
}

type codexResolvedProvider struct {
	Provider    string
	Attribution string
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
			s.sessionProvider = provider
			s.providerFromModel = false
		}
		if cwd := stringValue(payload["cwd"]); cwd != "" {
			s.cwd = cwd
		}
	case "turn_context":
		s.turnID = codexTurnID(obj, payload)
		if ts, ok := codexTimestamp(obj); ok {
			s.turnStartedAt = ts
		}
		if model := stringValue(payload["model"]); model != "" {
			s.updateModel(model, true)
		}
		if cwd := stringValue(payload["cwd"]); cwd != "" {
			s.cwd = cwd
		}
	}
}

func (s *codexState) updateModel(raw string, clearProviderWhenBare bool) {
	model, provider := parseCodexModel(raw)
	s.model = model
	if provider != "" {
		s.provider = provider
		s.providerFromModel = true
	} else if clearProviderWhenBare {
		if s.sessionProvider != "" {
			s.provider = s.sessionProvider
		}
		s.providerFromModel = false
	}
}

func (c Codex) parseBufferedEvent(obj map[string]any, state *codexState) (codexBufferedEvent, bool) {
	payload := objectValue(obj["payload"])
	if payload == nil {
		return codexBufferedEvent{}, false
	}
	ts, ok := codexTimestamp(obj)
	if !ok {
		return codexBufferedEvent{}, false
	}

	payloadType := stringValue(payload["type"])
	turnID := state.turnID
	if turnID == "" {
		turnID = codexTurnID(obj, payload)
	}

	if payloadType != "token_count" {
		return codexBufferedEvent{}, false
	}

	info := objectValue(payload["info"])
	if info == nil {
		return codexBufferedEvent{}, false
	}
	if model := firstNonEmptyString(info, "model", "model_name"); model != "" {
		state.updateModel(model, false)
	} else if model := stringValue(payload["model"]); model != "" {
		state.updateModel(model, false)
	}
	tokens, ok := state.codexTokenDelta(info)
	if !ok {
		return codexBufferedEvent{}, false
	}
	tokens.CachedInput = minInt64(tokens.CachedInput, tokens.Input)
	if tokens.NormalizedTotal() == 0 && tokens.CachedInput == 0 {
		return codexBufferedEvent{}, false
	}
	return codexBufferedEvent{
		turnID: turnID,
		at:     ts,
		model:  normalizeCodexModel(state.model),
		cwd:    state.cwd,
		tokens: tokens,
	}, true
}

func (c Codex) providerForBufferedTurn(threadID string, pending *codexBufferedTurn) (string, string) {
	if pending == nil {
		return "", ""
	}
	if pending.providerFromModel && pending.provider != "" {
		return pending.provider, string(usage.ProviderAttributionModel)
	}
	if c.providerTimeline != nil {
		if provider, found := c.providerTimeline.exactProviderForTurn(threadID, pending.id); found && provider != "" {
			return provider, string(usage.ProviderAttributionExactRequest)
		}
		if provider := c.providerTimeline.inferredProviderForTime(threadID, pending.inferenceAt()).Provider; provider != "" {
			return provider, string(usage.ProviderAttributionInferredTimeline)
		}
	}
	attribution := pending.providerAttribution
	if attribution == "" {
		attribution = string(usage.ProviderAttributionSessionFallback)
	}
	return pending.provider, attribution
}

func (c Codex) resolveBufferedTurnProviders(threadID string, turns []*codexBufferedTurn) []codexResolvedProvider {
	resolved := make([]codexResolvedProvider, len(turns))
	labeled := make([]bool, len(turns))
	for i, turn := range turns {
		if turn == nil {
			continue
		}
		if provider, attribution, ok := c.labeledProviderForBufferedTurn(threadID, turn); ok {
			resolved[i] = codexResolvedProvider{Provider: provider, Attribution: attribution}
			labeled[i] = true
		}
	}
	for i := 0; i < len(turns); {
		if labeled[i] {
			i++
			continue
		}
		j := i
		for j < len(turns) && !labeled[j] {
			j++
		}
		for k := i; k < j; k++ {
			provider, attribution := c.providerForBufferedTurn(threadID, turns[k])
			resolved[k] = codexResolvedProvider{Provider: provider, Attribution: attribution}
		}
		i = j
	}
	return resolved
}

func (c Codex) labeledProviderForBufferedTurn(threadID string, pending *codexBufferedTurn) (string, string, bool) {
	if pending == nil {
		return "", "", false
	}
	if pending.providerFromModel && pending.provider != "" {
		return pending.provider, string(usage.ProviderAttributionModel), true
	}
	if c.providerTimeline != nil {
		if provider, found := c.providerTimeline.exactProviderForTurn(threadID, pending.id); found && provider != "" {
			return provider, string(usage.ProviderAttributionExactRequest), true
		}
	}
	return "", "", false
}

func (c Codex) codexUsageEvent(path string, state *codexState, turnID string, ts time.Time, tokens usage.TokenUsage, model string, provider string, attribution string, cwd string) usage.UsageEvent {
	tokens.CachedInput = minInt64(tokens.CachedInput, tokens.Input)
	state.eventIndex++
	id := codexEventID(state.thread.ID, turnID, state.eventIndex, ts, tokens)
	return usage.UsageEvent{
		ID:                  id,
		TurnID:              turnID,
		Timestamp:           ts,
		Tool:                usage.ToolCodex,
		Model:               normalizeCodexModel(model),
		Provider:            usage.Unknown(provider),
		ProviderAttribution: attribution,
		CWD:                 cwd,
		Source:              path,
		ThreadID:            state.thread.ID,
		ThreadName:          state.thread.Name,
		ThreadSource:        state.thread.Source,
		ThreadCreatedAt:     state.thread.CreatedAt,
		ThreadLastActiveAt:  state.thread.LastActiveAt,
		Usage:               tokens,
	}
}

func providerAttributionFromState(state codexState) string {
	if state.providerFromModel {
		return string(usage.ProviderAttributionModel)
	}
	if state.sessionProvider != "" {
		return string(usage.ProviderAttributionSessionFallback)
	}
	return ""
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

func (p *codexBufferedTurn) inferenceAt() time.Time {
	if p == nil {
		return time.Time{}
	}
	if !p.endedAt.IsZero() {
		return p.endedAt
	}
	return p.startedAt
}

func codexTurnContextBoundaryID(obj map[string]any) string {
	payload := objectValue(obj["payload"])
	if payload == nil {
		return ""
	}
	if stringValue(obj["type"]) == "turn_context" {
		return codexTurnID(obj, payload)
	}
	return ""
}

func codexTimestamp(obj map[string]any) (time.Time, bool) {
	ts, err := time.Parse(time.RFC3339Nano, stringValue(obj["timestamp"]))
	if err != nil {
		return time.Time{}, false
	}
	return ts, true
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
