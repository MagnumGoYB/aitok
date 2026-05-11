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
	"github.com/mattn/go-runewidth"
)

func TestRenderSmoke(t *testing.T) {
	view := RenderWidth(samplePayload(), 140)
	for _, expected := range []string{
		"Usage Dashboard",
		"Monitor AI model usage and estimated cost",
		"All",
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
		"全部",
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
	if !strings.Contains(stripANSI(view), "1,225") || !strings.Contains(stripANSI(view), "\n│  Model") {
		t.Fatalf("model usage chart and table must be separated by a blank line: %s", view)
	}
}

func TestThreadsRenderBeforeBorderedModelUsage(t *testing.T) {
	payload := samplePayload()
	payload.Threads = []query.ThreadResult{
		{ID: "thread-a", Name: "Login bug", Tool: "codex", Model: "gpt-5.4", Provider: "openai", Usage: usage.TokenUsage{Input: 10}},
	}
	view := RenderWidth(payload, 160)
	threadsIndex := strings.Index(view, "Threads")
	modelIndex := strings.Index(view, "Model Usage")
	if threadsIndex < 0 || modelIndex < 0 {
		t.Fatalf("view must render both threads and model usage: %s", view)
	}
	if threadsIndex > modelIndex {
		t.Fatalf("threads must render before model usage: %s", view)
	}
	if !strings.Contains(view, "╭") || !strings.Contains(view, "Model Usage") {
		t.Fatalf("model usage should render inside a bordered section: %s", view)
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

func TestThreadsBoxHasNoTrailingColumnAndUsesEdgeAlignment(t *testing.T) {
	payload := samplePayload()
	payload.Threads = []query.ThreadResult{
		{ID: "019e167b-b7e8-7743-8bb3-fd9951e5ef2f", Name: "修正日期范围与threads列表", Tool: "codex", Model: "gpt-5.5", Provider: "bcb", Requests: 199, Events: 199, Usage: usage.TokenUsage{Input: 28_345_680}, CostUSD: 22.0954},
	}
	m := NewModel(payload)
	m.width = 180
	box := stripANSI(m.threadsBox(copyFor(LanguageEnglish)))
	if strings.Contains(box, "Tokens │") || strings.Contains(box, "28.3m │") {
		t.Fatalf("threads rows should not render a trailing vertical column: %s", box)
	}
	if !strings.Contains(box, "ID             Name") {
		t.Fatalf("ID and Name columns should have a larger left-aligned gap: %s", box)
	}
	if !strings.Contains(box, "Req    Events") {
		t.Fatalf("Req should be left-aligned and Events should remain right-aligned: %s", box)
	}
}

func TestThreadRowAlignmentPolicy(t *testing.T) {
	header := threadRow("ID", "Name", "Tool", "Model", "Provider", "Req", "Events", "Cost", "Tokens")
	row := threadRow("019e", "Fix title", "codex", "gpt-5.5", "bcb", "261", "261", "$31.3324", "41.4m")

	for _, expected := range []string{
		"Name                         Tool",
		"Tool     Model",
		"Model              Provider",
		"Provider   Req",
		"Req    Events",
	} {
		if !strings.Contains(header, expected) {
			t.Fatalf("header should keep left-aligned gap %q:\n%s", expected, header)
		}
	}
	for _, expected := range []string{
		"Fix title                    codex",
		"codex    gpt-5.5",
		"gpt-5.5            bcb",
		"bcb        261",
	} {
		if !strings.Contains(row, expected) {
			t.Fatalf("row should keep left-aligned gap %q:\n%s", expected, row)
		}
	}
	for _, expected := range []string{
		"   261  $31.3324",
		"$31.3324     41.4m",
	} {
		if !strings.Contains(row, expected) {
			t.Fatalf("events/cost/tokens should remain right-aligned %q:\n%s", expected, row)
		}
	}
}

func TestThreadsBoxAlignsWideCharactersAndTruncatesName(t *testing.T) {
	payload := samplePayload()
	payload.Threads = []query.ThreadResult{
		{ID: "019e167b-b7e8-7743-8bb3-fd9951e5ef2f", Name: "修正日期范围与threads列表很长很长", Tool: "codex", Model: "gpt-5.5", Provider: "bcb", Requests: 199, Events: 199, Usage: usage.TokenUsage{Input: 28_345_680}, CostUSD: 22.0954},
		{ID: "019e1522-e729-70c2-b013-bf66207c6b51", Name: "mini_program_wechat", Tool: "codex", Model: "gpt-5.5", Provider: "bcb", Requests: 61, Events: 61, Usage: usage.TokenUsage{Input: 6_125_217}, CostUSD: 6.7136},
	}
	m := NewModel(payload)
	m.width = 180
	box := m.threadsBox(copyFor(LanguageEnglish))
	lines := strings.Split(box, "\n")
	var rowWidths []int
	for _, line := range lines {
		if strings.Contains(line, "019e") {
			rowWidths = append(rowWidths, runewidth.StringWidth(stripANSI(line)))
		}
	}
	if len(rowWidths) != 2 {
		t.Fatalf("expected two thread rows, got %d\n%s", len(rowWidths), box)
	}
	if rowWidths[0] != rowWidths[1] {
		t.Fatalf("thread row widths must align, got %v\n%s", rowWidths, box)
	}
	if strings.Contains(box, "很长很长") || !strings.Contains(box, "…") {
		t.Fatalf("thread name should be truncated in TUI display: %s", box)
	}
}

func TestThreadsKeyboardSelectionAndCopyStatus(t *testing.T) {
	payload := samplePayload()
	payload.Threads = []query.ThreadResult{
		{ID: "thread-a", Name: "Login bug", Tool: "codex", Model: "gpt-5.4", Provider: "openai", Usage: usage.TokenUsage{Input: 10}},
		{ID: "thread-b", Name: "Deploy", Tool: "claude", Model: "claude-sonnet", Provider: "unknown", Usage: usage.TokenUsage{Input: 8}},
	}
	m := NewModel(payload)
	updated, _ := m.Update(keyMsg("j"))
	m = updated.(model)
	if m.threadCursor != 1 {
		t.Fatalf("j should move thread cursor without requiring t focus first, got %d", m.threadCursor)
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

func stripANSI(value string) string {
	var b strings.Builder
	inEscape := false
	for i := 0; i < len(value); i++ {
		ch := value[i]
		if inEscape {
			if ch >= '@' && ch <= '~' {
				inEscape = false
			}
			continue
		}
		if ch == 0x1b {
			inEscape = true
			continue
		}
		b.WriteByte(ch)
	}
	return b.String()
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
