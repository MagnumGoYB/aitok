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
	if strings.Contains(view, "📅") {
		t.Fatalf("TUI toolbar must not render date emoji: %s", view)
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

func TestToolbarDateFormatsTodayWithoutRangeAndWeekWithRange(t *testing.T) {
	loc := time.FixedZone("CST", 8*60*60)
	payload := samplePayload()
	payload.Period = query.PeriodToday
	payload.Window = query.Window{Start: time.Date(2026, 5, 11, 0, 0, 0, 0, loc), End: time.Date(2026, 5, 12, 0, 0, 0, 0, loc)}
	view := RenderWidth(payload, 160)
	if !strings.Contains(view, "2026-05-11 CST") || strings.Contains(view, "～") || strings.Contains(view, "📅") {
		t.Fatalf("today toolbar date should show date and zone only: %s", view)
	}

	payload.Period = query.PeriodThisWeek
	payload.Window = query.Window{Start: time.Date(2026, 5, 4, 0, 0, 0, 0, loc), End: time.Date(2026, 5, 11, 0, 0, 0, 0, loc)}
	view = RenderWidth(payload, 180)
	if !strings.Contains(view, "2026-05-04 00:00 ～ 2026-05-11 00:00 CST") {
		t.Fatalf("non-today toolbar date should show window range and zone: %s", view)
	}
}

func TestThreadsBoxRendersSelectionAndScrollBar(t *testing.T) {
	payload := samplePayload()
	payload.Threads = []query.ThreadResult{
		{ID: "thread-a", Name: "Login bug", Tool: "codex", Model: "gpt-5.4", Provider: "openai", Requests: 1, Events: 1, Usage: usage.TokenUsage{Input: 10}, CostUSD: 0.001},
		{ID: "thread-b", Name: "Deploy", Tool: "claude", Model: "claude-sonnet", Provider: "unknown", Requests: 1, Events: 1, Usage: usage.TokenUsage{Input: 8}, CostUSD: 0.002},
	}
	view := RenderWidth(payload, 160)
	for _, expected := range []string{"Threads", "ID", "Name", "Provider", "Login bug", "thread-a", "│"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("threads box missing %q: %s", expected, view)
		}
	}
}

func TestThreadsKeyboardSelectionAndCopyStatus(t *testing.T) {
	payload := samplePayload()
	payload.Threads = []query.ThreadResult{
		{ID: "thread-a", Name: "Login bug", Tool: "codex", Model: "gpt-5.4", Provider: "openai", Usage: usage.TokenUsage{Input: 10}},
		{ID: "thread-b", Name: "Deploy", Tool: "claude", Model: "claude-sonnet", Provider: "unknown", Usage: usage.TokenUsage{Input: 8}},
	}
	m := NewModel(payload)
	updated, _ := m.Update(keyMsg("t"))
	m = updated.(model)
	updated, _ = m.Update(keyMsg("j"))
	m = updated.(model)
	if m.threadCursor != 1 {
		t.Fatalf("thread cursor = %d, want 1", m.threadCursor)
	}
	updated, cmd := m.Update(keyMsg("c"))
	m = updated.(model)
	if cmd == nil || !strings.Contains(m.copyStatus, "thread-b") {
		t.Fatalf("copy should set status and emit command, status=%q cmd=%v", m.copyStatus, cmd)
	}
	updated, _ = m.Update(keyMsg("home"))
	m = updated.(model)
	if m.threadCursor != 0 {
		t.Fatalf("home should move to first thread, got %d", m.threadCursor)
	}
	updated, _ = m.Update(keyMsg("end"))
	m = updated.(model)
	if m.threadCursor != 1 {
		t.Fatalf("end should move to last thread, got %d", m.threadCursor)
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
