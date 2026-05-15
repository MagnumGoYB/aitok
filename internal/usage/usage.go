package usage

import "time"

type Tool string

const (
	ToolClaude Tool = "claude"
	ToolCodex  Tool = "codex"
	ToolGemini Tool = "gemini"
)

type TokenUsage struct {
	Input           int64 `json:"input"`
	Output          int64 `json:"output"`
	CachedInput     int64 `json:"cached_input"`
	CacheCreation   int64 `json:"cache_creation"`
	CacheCreation5m int64 `json:"cache_creation_5m,omitempty"`
	CacheCreation1h int64 `json:"cache_creation_1h,omitempty"`
	Reasoning       int64 `json:"reasoning"`
	Tool            int64 `json:"tool"`
	Total           int64 `json:"total"`
}

func (u TokenUsage) NormalizedTotal() int64 {
	if u.Total > 0 {
		return u.Total
	}
	return u.Input + u.Output + u.CacheCreation + u.Reasoning + u.Tool
}

func (u TokenUsage) Add(v TokenUsage) TokenUsage {
	return TokenUsage{
		Input:           u.Input + v.Input,
		Output:          u.Output + v.Output,
		CachedInput:     u.CachedInput + v.CachedInput,
		CacheCreation:   u.CacheCreation + v.CacheCreation,
		CacheCreation5m: u.CacheCreation5m + v.CacheCreation5m,
		CacheCreation1h: u.CacheCreation1h + v.CacheCreation1h,
		Reasoning:       u.Reasoning + v.Reasoning,
		Tool:            u.Tool + v.Tool,
		Total:           u.NormalizedTotal() + v.NormalizedTotal(),
	}
}

type ProviderAttribution string

const (
	ProviderAttributionModel            ProviderAttribution = "model"
	ProviderAttributionExactRequest     ProviderAttribution = "exact_request"
	ProviderAttributionInferredTimeline ProviderAttribution = "inferred_timeline"
	ProviderAttributionSessionFallback  ProviderAttribution = "session_fallback"
)

type UsageEvent struct {
	ID                  string     `json:"id"`
	TurnID              string     `json:"turn_id,omitempty"`
	Timestamp           time.Time  `json:"timestamp"`
	Tool                Tool       `json:"tool"`
	Model               string     `json:"model"`
	Provider            string     `json:"provider"`
	ProviderAttribution string     `json:"provider_attribution,omitempty"`
	CWD                 string     `json:"cwd,omitempty"`
	Source              string     `json:"source,omitempty"`
	ThreadID            string     `json:"thread_id,omitempty"`
	ThreadName          string     `json:"thread_name,omitempty"`
	ThreadSource        string     `json:"thread_source,omitempty"`
	ThreadCreatedAt     time.Time  `json:"thread_created_at,omitempty"`
	ThreadLastActiveAt  time.Time  `json:"thread_last_active_at,omitempty"`
	Complete            bool       `json:"-"`
	Usage               TokenUsage `json:"usage"`
}

func Unknown(value string) string {
	if value == "" {
		return "unknown"
	}
	return value
}
