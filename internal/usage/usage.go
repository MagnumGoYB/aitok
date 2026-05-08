package usage

import "time"

type Tool string

const (
	ToolClaude Tool = "claude"
	ToolCodex  Tool = "codex"
	ToolGemini Tool = "gemini"
)

type TokenUsage struct {
	Input         int64 `json:"input"`
	Output        int64 `json:"output"`
	CachedInput   int64 `json:"cached_input"`
	CacheCreation int64 `json:"cache_creation"`
	Reasoning     int64 `json:"reasoning"`
	Tool          int64 `json:"tool"`
	Total         int64 `json:"total"`
}

func (u TokenUsage) NormalizedTotal() int64 {
	if u.Total > 0 {
		return u.Total
	}
	return u.Input + u.Output + u.CacheCreation + u.Reasoning + u.Tool
}

func (u TokenUsage) Add(v TokenUsage) TokenUsage {
	return TokenUsage{
		Input:         u.Input + v.Input,
		Output:        u.Output + v.Output,
		CachedInput:   u.CachedInput + v.CachedInput,
		CacheCreation: u.CacheCreation + v.CacheCreation,
		Reasoning:     u.Reasoning + v.Reasoning,
		Tool:          u.Tool + v.Tool,
		Total:         u.NormalizedTotal() + v.NormalizedTotal(),
	}
}

type UsageEvent struct {
	ID        string     `json:"id"`
	Timestamp time.Time  `json:"timestamp"`
	Tool      Tool       `json:"tool"`
	Model     string     `json:"model"`
	Provider  string     `json:"provider"`
	CWD       string     `json:"cwd,omitempty"`
	Source    string     `json:"source,omitempty"`
	Usage     TokenUsage `json:"usage"`
}

func Unknown(value string) string {
	if value == "" {
		return "unknown"
	}
	return value
}
