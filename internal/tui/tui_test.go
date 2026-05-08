package tui

import (
	"io"
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

func TestModelStartsBackgroundRefreshWhenLoaderIsConfigured(t *testing.T) {
	m := NewModelWithRefresh(samplePayload(), LanguageEnglish, func() (report.Payload, error) {
		return samplePayload(), nil
	})
	if cmd := m.Init(); cmd == nil {
		t.Fatal("interactive TUI must schedule background refresh")
	}
}

func TestModelRefreshResultUpdatesPayloadAndSchedulesNextRefresh(t *testing.T) {
	m := NewModelWithRefresh(samplePayload(), LanguageEnglish, func() (report.Payload, error) {
		return samplePayload(), nil
	})
	next := samplePayload()
	next.Results[0].Key["model"] = "gpt-5.5"
	next.Results[0].Usage.Input = 2000
	updated, cmd := m.Update(refreshResultMsg{payload: next})
	m = updated.(model)
	if !strings.Contains(m.View(), "gpt-5.5") || !strings.Contains(m.View(), "2,225") {
		t.Fatalf("refresh result did not update TUI payload: %s", m.View())
	}
	if cmd == nil {
		t.Fatal("refresh result must schedule the next background refresh")
	}
}

func TestModelRefreshErrorKeepsCurrentPayloadAndSchedulesRetry(t *testing.T) {
	m := NewModelWithRefresh(samplePayload(), LanguageEnglish, func() (report.Payload, error) {
		return samplePayload(), nil
	})
	updated, cmd := m.Update(refreshResultMsg{err: io.ErrUnexpectedEOF})
	m = updated.(model)
	if !strings.Contains(m.View(), "gpt-5.4") {
		t.Fatalf("refresh error should keep existing payload: %s", m.View())
	}
	if cmd == nil {
		t.Fatal("refresh error must schedule a retry")
	}
}

func TestModelUsageLabelsIncludeProviderAndKeepColumnGap(t *testing.T) {
	payload := report.Payload{
		Results: []query.Result{
			{
				Key:      map[string]string{"tool": "codex", "model": "gpt-5.5", "provider": "openai"},
				Requests: 1909,
				Usage:    usage.TokenUsage{Input: 256_100_000, Output: 652_300},
				CostUSD:  195.7068,
			},
			{
				Key:      map[string]string{"tool": "codex", "model": "gpt-5.5", "provider": "openrouter"},
				Requests: 307,
				Usage:    usage.TokenUsage{Input: 35_500_000, Output: 166_100},
				CostUSD:  30.6730,
			},
		},
	}
	view := RenderWidth(payload, 140)
	for _, expected := range []string{"gpt-5.5 (openai)", "gpt-5.5 (openrouter)"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("view missing provider-qualified model label %q: %s", expected, view)
		}
	}
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "gpt-5.5 (openai)") && strings.Contains(line, "$195.7068") {
			gap := strings.SplitN(line, "gpt-5.5 (openai)", 2)[1]
			if !strings.HasPrefix(gap, "  ") || !strings.Contains(gap, "1909") {
				t.Fatalf("model table row must keep right padding before requests column: %q", line)
			}
		}
	}
}

func TestModelUsageChartAndTableAreSeparated(t *testing.T) {
	view := RenderWidth(samplePayload(), 140)
	if !strings.Contains(view, "1,225\n\nModel") {
		t.Fatalf("model usage chart and table must be separated by a blank line: %s", view)
	}
}

func TestModelUsageTableOutputDoesNotIncludeReasoning(t *testing.T) {
	payload := report.Payload{
		Results: []query.Result{{
			Key:      map[string]string{"tool": "codex", "model": "gpt-5.5", "provider": "bcb"},
			Requests: 1,
			Usage:    usage.TokenUsage{Input: 1000, Output: 200, Reasoning: 50, CachedInput: 25},
			CostUSD:  0.1234,
		}},
	}
	view := RenderWidth(payload, 140)
	if !strings.Contains(view, "gpt-5.5 (bcb)") || !strings.Contains(view, "         200") {
		t.Fatalf("model usage table output must match summary output tokens without reasoning: %s", view)
	}
	if strings.Contains(view, "         250") {
		t.Fatalf("model usage table output must not add reasoning tokens into output: %s", view)
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
