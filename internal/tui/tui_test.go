package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/MagnumGoYB/aitok/internal/query"
	"github.com/MagnumGoYB/aitok/internal/report"
	"github.com/MagnumGoYB/aitok/internal/usage"
	tea "github.com/charmbracelet/bubbletea"
)

func TestRenderSmoke(t *testing.T) {
	view := RenderWidth(samplePayload(), 140)
	for _, expected := range []string{
		"Usage Dashboard",
		"Monitor AI model usage and estimated cost",
		"[All]",
		"Claude Code",
		"Codex",
		"Gemini",
		"Requests",
		"Estimated Cost",
		"Total Tokens",
		"Cached Tokens",
		"Model Usage",
		"Search:",
		"/ search",
		"l language",
	} {
		if !strings.Contains(view, expected) {
			t.Fatalf("view missing %q: %s", expected, view)
		}
	}
	for _, unexpected := range []string{"使用统计", "总请求数", "模型用量"} {
		if strings.Contains(view, unexpected) {
			t.Fatalf("default TUI render must prefer English and not include %q: %s", unexpected, view)
		}
	}
	if strings.Contains(view, "↻ 30s") {
		t.Fatalf("TUI toolbar must not render auto-refresh copy: %s", view)
	}
}

func TestRenderChinese(t *testing.T) {
	view := RenderWidthWithLanguage(samplePayload(), 140, LanguageChinese)
	for _, expected := range []string{
		"使用统计",
		"查看 AI 模型的使用情况和成本统计",
		"[全部]",
		"总请求数",
		"总成本",
		"总 Token 数",
		"缓存 Token",
		"模型用量",
		"/ 搜索",
		"l 语言",
	} {
		if !strings.Contains(view, expected) {
			t.Fatalf("Chinese view missing %q: %s", expected, view)
		}
	}
}

func TestModelTogglesLanguage(t *testing.T) {
	m := NewModel(samplePayload())
	if !strings.Contains(m.View(), "Usage Dashboard") {
		t.Fatalf("default model should render English: %s", m.View())
	}
	updated, _ := m.Update(keyMsg("l"))
	m = updated.(model)
	if !strings.Contains(m.View(), "使用统计") {
		t.Fatalf("language toggle should render Chinese: %s", m.View())
	}
	updated, _ = m.Update(keyMsg("l"))
	m = updated.(model)
	if !strings.Contains(m.View(), "Usage Dashboard") {
		t.Fatalf("second language toggle should render English: %s", m.View())
	}
}

func TestModelDoesNotStartBackgroundRefresh(t *testing.T) {
	if cmd := NewModel(samplePayload()).Init(); cmd != nil {
		t.Fatalf("TUI must not start background refresh commands: %#v", cmd)
	}
}

func samplePayload() report.Payload {
	return report.Payload{
		Window: query.Window{Start: time.Date(2026, 5, 8, 0, 0, 0, 0, time.UTC), End: time.Date(2026, 5, 9, 0, 0, 0, 0, time.UTC)},
		Results: []query.Result{{
			Key:      map[string]string{"tool": "codex", "model": "gpt-5.4"},
			Requests: 2,
			Events:   2,
			Usage:    usage.TokenUsage{Input: 1000, Output: 200, CachedInput: 50, CacheCreation: 25},
			CostUSD:  0.1234,
		}},
	}
}

func keyMsg(value string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(value)}
}

func TestModelFiltersBySearchAndTool(t *testing.T) {
	m := NewModel(report.Payload{
		Results: []query.Result{
			{Key: map[string]string{"tool": "codex", "model": "gpt-5.4"}, Requests: 1, Usage: usage.TokenUsage{Input: 100}},
			{Key: map[string]string{"tool": "claude", "model": "claude-sonnet-4"}, Requests: 1, Usage: usage.TokenUsage{Input: 200}},
		},
	})
	m.activeTool = "codex"
	m.search = "gpt"
	view := m.View()
	if !strings.Contains(view, "gpt-5.4") || strings.Contains(view, "claude-sonnet-4") {
		t.Fatalf("unexpected view: %s", view)
	}
}
