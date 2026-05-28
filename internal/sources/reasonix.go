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

type Reasonix struct {
	Home        string
	WindowStart time.Time
	WindowEnd   time.Time
}

type reasonixUsageRecord struct {
	Ts               int64             `json:"ts"`
	Session          *string           `json:"session"`
	Model            string            `json:"model"`
	PromptTokens     int64             `json:"promptTokens"`
	CompletionTokens int64             `json:"completionTokens"`
	CacheHitTokens   int64             `json:"cacheHitTokens"`
	CacheMissTokens  int64             `json:"cacheMissTokens"`
	CostUsd          float64           `json:"costUsd"`
	ClaudeEquivUsd   float64           `json:"claudeEquivUsd"`
	Kind             string            `json:"kind,omitempty"`
	Subagent         *reasonixSubagent `json:"subagent,omitempty"`
}

type reasonixSubagent struct {
	SkillName   string `json:"skillName,omitempty"`
	TaskPreview string `json:"taskPreview"`
	ToolIters   int64  `json:"toolIters"`
	DurationMs  int64  `json:"durationMs"`
}

func NewReasonix(opts Options) Reasonix {
	return Reasonix{
		Home:        cleanHome(opts.Home),
		WindowStart: opts.WindowStart,
		WindowEnd:   opts.WindowEnd,
	}
}

func (r Reasonix) Name() usage.Tool {
	return usage.ToolReasonix
}

func (r Reasonix) Read(ctx context.Context) ([]usage.UsageEvent, error) {
	var events []usage.UsageEvent
	err := r.Scan(ctx, func(event usage.UsageEvent) error {
		events = append(events, event)
		return nil
	})
	return events, err
}

func (r Reasonix) Scan(ctx context.Context, handle func(usage.UsageEvent) error) error {
	sessions := r.sessionMetaByID()
	usageLog := filepath.Join(r.Home, ".reasonix", "usage.jsonl")
	err := readJSONLines(ctx, usageLog, func(obj map[string]any) error {
		event, ok := r.parseEvent(obj)
		if !ok {
			return nil
		}
		if meta, found := sessions[event.ThreadID]; found {
			event.ThreadName = meta.Name
			event.ThreadSource = meta.Source
			event.ThreadCreatedAt = meta.CreatedAt
			event.ThreadLastActiveAt = meta.LastActiveAt
		}
		if !r.WindowStart.IsZero() && event.Timestamp.Before(r.WindowStart) {
			return nil
		}
		if !r.WindowEnd.IsZero() && !event.Timestamp.Before(r.WindowEnd) {
			return nil
		}
		return handle(event)
	})
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func (r Reasonix) sessionMetaByID() map[string]threadMeta {
	out := map[string]threadMeta{}
	sessionsDir := filepath.Join(r.Home, ".reasonix", "sessions")
	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		return out
	}
	for _, entry := range entries {
		name := entry.Name()
		// Directory format: sessions/<name>/.meta.json
		if entry.IsDir() {
			meta := r.parseSessionMeta(sessionsDir, name)
			out[name] = *meta
			continue
		}
		// Flat format: sessions/<name>.meta.json
		if strings.HasSuffix(name, ".meta.json") {
			sessionName := strings.TrimSuffix(name, ".meta.json")
			meta := r.parseSessionMeta(sessionsDir, sessionName)
			out[sessionName] = *meta
			continue
		}
		// Flat format: sessions/<name>.jsonl — infer from session log
		if strings.HasSuffix(name, ".jsonl") {
			sessionName := strings.TrimSuffix(name, ".jsonl")
			if _, exists := out[sessionName]; !exists {
				meta := r.parseSessionMeta(sessionsDir, sessionName)
				out[sessionName] = *meta
			}
		}
	}
	return out
}

func (r Reasonix) sessionMetaPath(sessionsDir, sessionName string) string {
	return filepath.Join(sessionsDir, sessionName, ".meta.json")
}

func (r Reasonix) sessionMetaPathFlat(sessionsDir, sessionName string) string {
	return filepath.Join(sessionsDir, sessionName+".meta.json")
}

func (r Reasonix) sessionLogPath(sessionsDir, sessionName string) string {
	return filepath.Join(sessionsDir, sessionName, sessionName+".jsonl")
}

func (r Reasonix) sessionLogPathFlat(sessionsDir, sessionName string) string {
	return filepath.Join(sessionsDir, sessionName+".jsonl")
}

type reasonixSessionMeta struct {
	Workspace string `json:"workspace"`
	Summary   string `json:"summary"`
}

func (r Reasonix) parseSessionMeta(sessionsDir, sessionName string) *threadMeta {
	// Try both path formats for .meta.json
	metaPath := r.sessionMetaPath(sessionsDir, sessionName)
	flatMetaPath := r.sessionMetaPathFlat(sessionsDir, sessionName)
	summary := ""
	workspace := ""

	data, err := os.ReadFile(metaPath)
	if err != nil {
		data, err = os.ReadFile(flatMetaPath)
	}
	if err == nil {
		var meta reasonixSessionMeta
		if json.Unmarshal(data, &meta) == nil {
			summary = meta.Summary
			workspace = meta.Workspace
		}
	}

	// Fallback: try first user message from session JSONL
	firstUser := r.readFirstUserMessage(sessionsDir, sessionName)
	name := chooseThreadTitle(summary, firstUser, pathBase(workspace), sessionName)
	return &threadMeta{
		ID:     sessionName,
		Name:   name,
		Source: metaPath,
	}
}

func (r Reasonix) readFirstUserMessage(sessionsDir, sessionName string) string {
	logPath := r.sessionLogPath(sessionsDir, sessionName)
	data, err := os.ReadFile(logPath)
	if err != nil {
		logPath = r.sessionLogPathFlat(sessionsDir, sessionName)
		data, err = os.ReadFile(logPath)
	}
	if err != nil {
		return ""
	}
	// Scan for the first user message
	lines := 0
	start := 0
	for i := 0; i < len(data); i++ {
		if data[i] == '\n' {
			line := data[start:i]
			start = i + 1
			lines++
			if lines > 50 {
				break
			}
			var obj map[string]any
			if json.Unmarshal(line, &obj) != nil {
				continue
			}
			role := stringValue(obj["role"])
			if role != "user" {
				continue
			}
			text := extractText(obj["content"])
			if isRealUserTitle(text) {
				return text
			}
		}
	}
	return ""
}

func (r Reasonix) parseEvent(obj map[string]any) (usage.UsageEvent, bool) {
	ts := intValue(obj["ts"])
	if ts == 0 {
		return usage.UsageEvent{}, false
	}
	timestamp := time.UnixMilli(ts)

	model := usage.Unknown(stringValue(obj["model"]))
	if model == "unknown" {
		return usage.UsageEvent{}, false
	}

	promptTokens := intValue(obj["promptTokens"])
	completionTokens := intValue(obj["completionTokens"])
	cacheHitTokens := intValue(obj["cacheHitTokens"])
	cacheMissTokens := intValue(obj["cacheMissTokens"])

	tokens := usage.TokenUsage{
		Input:         promptTokens,
		Output:        completionTokens,
		CachedInput:   cacheHitTokens,
		CacheCreation: cacheMissTokens,
	}
	if tokens.NormalizedTotal() == 0 && tokens.CachedInput == 0 {
		return usage.UsageEvent{}, false
	}

	session := usage.Unknown(pointersString(obj["session"]))
	threadID := session
	threadName := session
	if session == "unknown" || stringsPointerEmpty(obj["session"]) {
		id := reasonixEventID(timestamp, model, tokens)
		threadID = id
		if stringsPointerEmpty(obj["session"]) {
			threadName = "(ephemeral)"
		} else {
			threadName = id
		}
	}

	tsMicro := timestamp.UnixMicro()
	return usage.UsageEvent{
		ID:           reasonixUUID(tsMicro, model, tokens),
		Timestamp:    timestamp,
		Tool:         usage.ToolReasonix,
		Model:        model,
		Provider:     "deepseek",
		Source:       "reasonix",
		ThreadID:     threadID,
		ThreadName:   threadName,
		ThreadSource: "reasonix",
		Complete:     true,
		Usage:        tokens,
	}, true
}

func reasonixEventID(ts time.Time, model string, tokens usage.TokenUsage) string {
	return reasonixUUID(ts.UnixMicro(), model, tokens)
}

func reasonixUUID(tsMicro int64, model string, tokens usage.TokenUsage) string {
	h := tsMicro ^ int64(tokens.Input) ^ int64(tokens.Output) ^ int64(tokens.CachedInput) ^ int64(tokens.CacheCreation)
	return usage.Unknown(model) + "-" + formatInt64(h)
}

func pointersString(p any) string {
	if p == nil {
		return ""
	}
	if s, ok := p.(string); ok {
		return s
	}
	return ""
}

func stringsPointerEmpty(p any) bool {
	return p == nil || pointersString(p) == ""
}

func formatInt64(n int64) string {
	if n < 0 {
		n = -n
	}
	if n < 0 {
		return "0"
	}
	u := uint64(n)
	alphabet := "0123456789abcdefghijklmnopqrstuv"
	buf := make([]byte, 13)
	for i := 12; i >= 0; i-- {
		buf[i] = alphabet[u%32]
		u /= 32
	}
	i := 0
	for i < 12 && buf[i] == '0' {
		i++
	}
	return string(buf[i:])
}
