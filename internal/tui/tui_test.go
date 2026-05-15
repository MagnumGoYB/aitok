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
	"github.com/charmbracelet/lipgloss"
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
		"Model Usage [All Threads Cost]",
		"Search:",
		"[Tokens]",
		"Price",
		"s sort",
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
		"模型用量 [全部会话成本]",
		"搜索:",
		"[按 Tokens]",
		"价格",
		"s 排序",
		"模型",
		"请求",
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
				Price:    &query.Price{Source: "official", InputUSDPerMTok: 5, OutputUSDPerMTok: 30},
			},
			{
				Key:      map[string]string{"tool": "codex", "model": "gpt-5.5", "provider": "openrouter"},
				Requests: 307,
				Usage:    usage.TokenUsage{Input: 35_500_000, Output: 166_100},
				CostUSD:  30.6730,
				Price:    &query.Price{Source: "custom", InputUSDPerMTok: 2, OutputUSDPerMTok: 20},
			},
		},
	}
	view := RenderWidth(payload, 140)
	for _, expected := range []string{"gpt-5.5 (openai)", "gpt-5.5 (openrouter)", "official in=$5/M out=$30/M", "custom in=$2/M out=$20/M"} {
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

func TestModelUsageTableShowsPriceSourceAndRates(t *testing.T) {
	payload := report.Payload{
		Results: []query.Result{
			{
				Key:      map[string]string{"tool": "codex", "model": "gpt-5.4", "provider": "team-a"},
				Requests: 1,
				Usage:    usage.TokenUsage{Input: 1_000_000},
				CostUSD:  2,
				Price:    &query.Price{Source: "custom", InputUSDPerMTok: 2, OutputUSDPerMTok: 20},
			},
			{
				Key:      map[string]string{"tool": "codex", "model": "gpt-5.4", "provider": "openai"},
				Requests: 1,
				Usage:    usage.TokenUsage{Input: 1_000_000},
				CostUSD:  2.5,
				Price:    &query.Price{Source: "official", InputUSDPerMTok: 2.5, OutputUSDPerMTok: 15},
			},
		},
	}
	view := RenderWidth(payload, 180)
	for _, expected := range []string{"Price", "custom in=$2/M out=$20/M", "official in=$2.5/M out=$15/M"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("TUI should show price details %q: %s", expected, view)
		}
	}
	for _, line := range strings.Split(stripANSI(view), "\n") {
		if strings.Contains(line, "gpt-5.4 (") && !strings.Contains(line, "Cached") && !strings.Contains(line, "│") {
			if !strings.Contains(line, "custom") && !strings.Contains(line, "official") {
				t.Fatalf("model usage row should keep price on the same line: %q\n%s", line, view)
			}
		}
	}
}

func TestModelUsageChartAndTableAreSeparated(t *testing.T) {
	view := RenderWidth(samplePayload(), 140)
	lines := strings.Split(stripANSI(view), "\n")
	for i, line := range lines {
		if strings.Contains(line, "1,225") && strings.Contains(line, "█") {
			if i+2 >= len(lines) || strings.Trim(lines[i+1], " │") != "" || !strings.Contains(lines[i+2], "Model") {
				t.Fatalf("model usage chart and table must be separated by a blank line: %s", view)
			}
			return
		}
	}
	t.Fatalf("model usage chart line missing: %s", view)
}

func TestModelUsageChartKeepsSmallTokenRatiosVisible(t *testing.T) {
	payload := report.Payload{
		Results: []query.Result{
			{Key: map[string]string{"tool": "codex", "model": "gpt-5.5", "provider": "bcb"}, Usage: usage.TokenUsage{Input: 107_947_474}},
			{Key: map[string]string{"tool": "claude", "model": "claude-opus-4-7"}, Usage: usage.TokenUsage{Input: 955_754}},
			{Key: map[string]string{"tool": "codex", "model": "gpt-5.5", "provider": "openai"}, Usage: usage.TokenUsage{Input: 585_484}},
		},
	}
	view := stripANSI(RenderWidth(payload, 160))
	wideUnits := modelUsageBarUnits(t, view, "gpt-5.5 (bcb)")
	midUnits := modelUsageBarUnits(t, view, "claude-opus-4-7")
	smallUnits := modelUsageBarUnits(t, view, "gpt-5.5 (openai)")
	if !(wideUnits > midUnits && midUnits > smallUnits) {
		t.Fatalf("model usage chart should keep token ratios distinct, got wide=%d mid=%d small=%d\n%s", wideUnits, midUnits, smallUnits, view)
	}
}

func TestModelUsageBarStyleUsesSameHueWithDepthByRank(t *testing.T) {
	first := modelUsageBarStyle(0, 4).GetForeground()
	second := modelUsageBarStyle(1, 4).GetForeground()
	last := modelUsageBarStyle(3, 4).GetForeground()
	if first == second || second == last || first == last {
		t.Fatalf("bar shades should vary by rank, got first=%v second=%v last=%v", first, second, last)
	}
	if first != lipgloss.Color("#0A84D6") {
		t.Fatalf("highest-usage bar should use the darkest shade, got %v", first)
	}
	if last != lipgloss.Color("#7CCDF5") {
		t.Fatalf("lowest-usage bar should use the lightest shade, got %v", last)
	}
}

func TestModelUsageTableAlignsMixedWidthLabels(t *testing.T) {
	payload := report.Payload{
		Results: []query.Result{
			{
				Key:      map[string]string{"tool": "codex", "model": "gpt-5.5-中文模型", "provider": "bcb"},
				Requests: 1,
				Usage:    usage.TokenUsage{Input: 28_345_680, Output: 123_456},
				CostUSD:  145.4321,
			},
			{
				Key:      map[string]string{"tool": "codex", "model": "gpt-5.5", "provider": "bcb"},
				Requests: 1,
				Usage:    usage.TokenUsage{Input: 6_125_217, Output: 65_432},
				CostUSD:  32.5890,
			},
		},
	}
	view := stripANSI(RenderWidth(payload, 160))
	var costEnds []int
	var inputEnds []int
	for _, line := range strings.Split(view, "\n") {
		if !strings.Contains(line, "gpt-5.5") || !strings.Contains(line, "$") {
			continue
		}
		costValue := "$145.4321"
		costStart := strings.Index(line, costValue)
		if costStart < 0 {
			costValue = "$32.5890"
			costStart = strings.Index(line, costValue)
		}
		inputValue := "28.3m"
		inputStart := strings.Index(line, inputValue)
		if inputStart < 0 {
			inputValue = "6.1m"
			inputStart = strings.Index(line, inputValue)
		}
		costEnds = append(costEnds, runewidth.StringWidth(line[:costStart])+runewidth.StringWidth(costValue))
		inputEnds = append(inputEnds, runewidth.StringWidth(line[:inputStart])+runewidth.StringWidth(inputValue))
	}
	if len(costEnds) != 2 || costEnds[0] != costEnds[1] {
		t.Fatalf("Cost column should right-align for mixed-width model labels, ends=%v\n%s", costEnds, view)
	}
	if len(inputEnds) != 2 || inputEnds[0] != inputEnds[1] {
		t.Fatalf("Input column should right-align for mixed-width model labels, ends=%v\n%s", inputEnds, view)
	}
}

func TestModelUsageTableIncludesTotalTokens(t *testing.T) {
	payload := report.Payload{
		Results: []query.Result{{
			Key:      map[string]string{"tool": "codex", "model": "gpt-5.5", "provider": "bcb"},
			Requests: 1,
			Usage:    usage.TokenUsage{Input: 1000, Output: 200, CachedInput: 50, CacheCreation: 25},
			CostUSD:  0.1234,
		}},
	}
	view := stripANSI(RenderWidth(payload, 160))
	if !strings.Contains(view, "Tokens") {
		t.Fatalf("model usage table must include a total Tokens column:\n%s", view)
	}
	if !strings.Contains(view, "1.2k") {
		t.Fatalf("model usage table Tokens column should include normalized total tokens:\n%s", view)
	}
}

func TestModelUsageCapsRowsWhenProvidersAreMany(t *testing.T) {
	payload := samplePayload()
	payload.Results = nil
	for i := 0; i < 12; i++ {
		payload.Results = append(payload.Results, query.Result{
			Key:      map[string]string{"tool": "codex", "model": "gpt-5.5", "provider": "provider-" + string(rune('a'+i))},
			Requests: 10 - i,
			Usage:    usage.TokenUsage{Input: int64(1_000_000 - i*10_000)},
			CostUSD:  float64(12 - i),
		})
	}
	view := stripANSI(RenderWidth(payload, 160))
	if strings.Contains(view, "provider-h") || strings.Contains(view, "provider-l") {
		t.Fatalf("model usage should cap provider-heavy output to top rows:\n%s", view)
	}
	if !strings.Contains(view, "provider-a") || !strings.Contains(view, "provider-f") {
		t.Fatalf("model usage should keep the most important provider rows:\n%s", view)
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
	if !strings.Contains(view, "gpt-5.5 (bcb)") || !strings.Contains(view, "    200") {
		t.Fatalf("model usage table output must match summary output tokens without reasoning: %s", view)
	}
	if strings.Contains(view, "    250") {
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
	if !strings.Contains(view, "2026-05-04 00:00 ~ 2026-05-11 00:00 CST") || strings.Contains(view, "～") {
		t.Fatalf("non-today toolbar date should show window range and zone: %s", view)
	}
}

func TestViewUsesCompactSectionSpacing(t *testing.T) {
	payload := samplePayload()
	payload.Threads = []query.ThreadResult{
		{ID: "thread-a", Name: "Login bug", Tool: "codex", Model: "gpt-5.4", Provider: "openai", Requests: 1, Events: 1, Usage: usage.TokenUsage{Input: 10}, CostUSD: 0.001},
	}
	view := stripANSI(RenderWidth(payload, 160))
	if strings.Contains(view, "\n\n\n") {
		t.Fatalf("TUI sections should not use excessive vertical gaps:\n%s", view)
	}
}

func TestThreadsBoxRendersSelectionAndScrollBar(t *testing.T) {
	payload := samplePayload()
	for i := 0; i < 12; i++ {
		payload.Threads = append(payload.Threads, query.ThreadResult{
			ID:       "thread-" + string(rune('a'+i)),
			Name:     "Login bug",
			Tool:     "codex",
			Model:    "gpt-5.4",
			Provider: "openai",
			Requests: 1,
			Events:   1,
			Usage:    usage.TokenUsage{Input: int64(10 + i)},
			CostUSD:  0.001,
		})
	}
	view := RenderWidth(payload, 160)
	for _, expected := range []string{"Threads", "ID", "Name", "Provider", "Login bug", "thread-l", "┃"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("threads box missing %q: %s", expected, view)
		}
	}
	if strings.Contains(view, "thread-a") {
		t.Fatalf("threads first page should show the highest-token rows first:\n%s", view)
	}
}

func TestThreadViewportHeightCapsAtSixRows(t *testing.T) {
	m := NewModel(samplePayload())
	m.width = 160
	if got := m.threadViewportHeight(); got != 6 {
		t.Fatalf("large screens should cap threads viewport at 6 rows, got %d", got)
	}
	m.width = 80
	if got := m.threadViewportHeight(); got != 6 {
		t.Fatalf("narrow screens should keep compact threads viewport at 6 rows, got %d", got)
	}
}

func TestTUILayoutUsesCompactToolbarAndCards(t *testing.T) {
	m := NewModel(samplePayload())
	toolbar := stripANSI(m.toolbar(copyFor(LanguageEnglish)))
	if got := len(strings.Split(toolbar, "\n")); got > 3 {
		t.Fatalf("toolbar should stay compact, got %d lines:\n%s", got, toolbar)
	}
	card := stripANSI(cardWithWidth("Requests", "4,906", "↯", blue, 28))
	if got := len(strings.Split(card, "\n")); got > 5 {
		t.Fatalf("summary cards should stay compact, got %d lines:\n%s", got, card)
	}
}

func TestThreadRowColumnsAlignHeaderAndContent(t *testing.T) {
	header := stripANSI(threadRow("ID", "Name", "Tool", "Model", "Provider", "Req", "Cost", "Split", "Tokens"))
	row := stripANSI(threadRow("019e167b-b…", "修正日期范围与threads列表", "codex", "gpt-5.5", "bcb", "297", "$34.9399", "toska/bcb", "45.5m"))

	for _, label := range []string{"Name", "Tool", "Model", "Provider", "Req", "Cost", "Split", "Tokens"} {
		want := runewidth.StringWidth(header[:strings.Index(header, label)])
		got := runewidth.StringWidth(row[:strings.Index(row, strings.TrimSpace(columnValueForLabel(label, row)))])
		if got != want && label != "Cost" && label != "Tokens" {
			t.Fatalf("%s column should start at width %d, got %d\nheader=%q\nrow=%q", label, want, got, header, row)
		}
	}
	if runewidth.StringWidth(header) != runewidth.StringWidth(row) {
		t.Fatalf("header and row should have equal display width, got %d/%d\n%s\n%s", runewidth.StringWidth(header), runewidth.StringWidth(row), header, row)
	}
	assertRightAlignedColumn(t, header, row, "Cost", "$34.9399")
	assertRightAlignedColumn(t, header, row, "Tokens", "45.5m")
}

func columnValueForLabel(label, row string) string {
	switch label {
	case "Name":
		return "修正日期范围与threads列表"
	case "Tool":
		return "codex"
	case "Model":
		return "gpt-5.5"
	case "Provider":
		return "bcb"
	case "Req":
		return "297"
	case "Cost":
		return "$34.9399"
	case "Split":
		return "toska/bcb"
	case "Tokens":
		return "45.5m"
	default:
		return ""
	}
}

func assertRightAlignedColumn(t *testing.T, header, row, headerLabel, rowValue string) {
	t.Helper()
	headerStart := strings.Index(header, headerLabel)
	rowStart := strings.Index(row, rowValue)
	if headerStart < 0 || rowStart < 0 {
		t.Fatalf("missing alignment values %q/%q\nheader=%q\nrow=%q", headerLabel, rowValue, header, row)
	}
	headerEnd := runewidth.StringWidth(header[:headerStart]) + runewidth.StringWidth(headerLabel)
	rowEnd := runewidth.StringWidth(row[:rowStart]) + runewidth.StringWidth(rowValue)
	if headerEnd != rowEnd {
		t.Fatalf("%s column should right-align header and content at width %d, got %d\nheader=%q\nrow=%q", headerLabel, headerEnd, rowEnd, header, row)
	}
}

func TestThreadsBoxHasNoTrailingColumnAndUsesEdgeAlignment(t *testing.T) {
	payload := samplePayload()
	payload.Threads = []query.ThreadResult{
		{ID: "019e167b-b7e8-7743-8bb3-fd9951e5ef2f", Name: "修正日期范围与threads列表", Tool: "codex", Model: "gpt-5.5", Provider: "bcb", Requests: 199, Events: 199, Usage: usage.TokenUsage{Input: 28_345_680}, CostUSD: 22.0954},
	}
	m := NewModel(payload)
	m.width = 180
	box := stripANSI(m.threadsBox(m.filteredThreads(), copyFor(LanguageEnglish)))
	if strings.Contains(box, "Tokens │") || strings.Contains(box, "28.3m │") {
		t.Fatalf("threads rows should not render a trailing vertical column: %s", box)
	}
	if !strings.Contains(box, "ID             Name") {
		t.Fatalf("ID and Name columns should have a larger left-aligned gap: %s", box)
	}
	if strings.Contains(box, "Events") {
		t.Fatalf("threads compact box should not render a separate Events column: %s", box)
	}
}

func TestThreadRowAlignmentPolicy(t *testing.T) {
	header := threadRow("ID", "Name", "Tool", "Model", "Provider", "Req", "Cost", "Split", "Tokens")
	row := threadRow("019e", "Fix title", "codex", "gpt-5.5", "bcb", "261", "$31.3324", "toska/bcb", "41.4m")

	for _, expected := range []string{
		"Name                       Tool",
		"Tool    Model",
		"Model           Provider",
		"Provider   Req",
		"Req             Cost",
		"Cost  Split",
	} {
		if !strings.Contains(header, expected) {
			t.Fatalf("header should keep left-aligned gap %q:\n%s", expected, header)
		}
	}
	for _, expected := range []string{
		"Fix title                  codex",
		"codex   gpt-5.5",
		"gpt-5.5         bcb",
		"bcb        261",
		"$31.3324  toska/bcb",
	} {
		if !strings.Contains(row, expected) {
			t.Fatalf("row should keep left-aligned gap %q:\n%s", expected, row)
		}
	}
	for _, expected := range []string{
		"261         $31.3324",
		"toska/bcb      41.4m",
	} {
		if !strings.Contains(row, expected) {
			t.Fatalf("req/cost/split/tokens should preserve the numeric alignment %q:\n%s", expected, row)
		}
	}
}

func TestThreadsBoxShowsProviderListAndCostBreakdown(t *testing.T) {
	payload := samplePayload()
	payload.Threads = []query.ThreadResult{
		{
			ID:       "019e2491-5335-7420-91bc-d555ae79337e",
			Name:     "Provider switch",
			Tool:     "codex",
			Model:    "gpt-5.5",
			Provider: "bcb,toska",
			Requests: 472,
			Events:   472,
			Usage:    usage.TokenUsage{Input: 66_636_577},
			CostUSD:  301.11,
			CostBreakdown: []query.ThreadCost{
				{Provider: "toska", USD: 299.7687},
				{Provider: "bcb", USD: 1.3412},
			},
		},
	}
	m := NewModel(payload)
	m.width = 180
	box := stripANSI(m.threadsBox(m.filteredThreads(), copyFor(LanguageEnglish)))
	for _, expected := range []string{"bcb,toska", "$301.1100", "toska/bcb", "66.6m"} {
		if !strings.Contains(box, expected) {
			t.Fatalf("threads box should show provider list and cost split %q:\n%s", expected, box)
		}
	}
	if strings.Contains(box, "$301.1100+") {
		t.Fatalf("threads box should keep Cost numeric and use Split instead of plus marker:\n%s", box)
	}
}

func TestThreadsBoxUsesSplitPlaceholderWhenCostIsNotSplit(t *testing.T) {
	payload := samplePayload()
	payload.Threads = []query.ThreadResult{
		{ID: "019e167b-b7e8-7743-8bb3-fd9951e5ef2f", Name: "Single provider", Tool: "codex", Model: "gpt-5.5", Provider: "bcb", Requests: 199, Events: 199, Usage: usage.TokenUsage{Input: 28_345_680}, CostUSD: 22.0954},
	}
	m := NewModel(payload)
	m.width = 180
	box := stripANSI(m.threadsBox(m.filteredThreads(), copyFor(LanguageEnglish)))
	for _, expected := range []string{"Cost", "Split", "$22.0954", "  -"} {
		if !strings.Contains(box, expected) {
			t.Fatalf("threads box should show stable split placeholder %q:\n%s", expected, box)
		}
	}
}

func TestThreadsBoxAlignsCostColumnAcrossRows(t *testing.T) {
	payload := samplePayload()
	payload.Threads = []query.ThreadResult{
		{ID: "019e167b-b7e8-7743-8bb3-fd9951e5ef2f", Name: "修正日期范围与threads列表很长很长", Tool: "codex", Model: "gpt-5.5", Provider: "bcb", Requests: 199, Events: 199, Usage: usage.TokenUsage{Input: 28_345_680}, CostUSD: 22.0954},
		{ID: "019e1522-e729-70c2-b013-bf66207c6b51", Name: "mini_program_wechat", Tool: "codex", Model: "gpt-5.5", Provider: "bcb", Requests: 61, Events: 61, Usage: usage.TokenUsage{Input: 6_125_217}, CostUSD: 6.7136},
	}
	m := NewModel(payload)
	box := stripANSI(m.threadsBox(m.filteredThreads(), copyFor(LanguageEnglish)))
	var ends []int
	for _, line := range strings.Split(box, "\n") {
		if !strings.Contains(line, "019e") {
			continue
		}
		cost := "$22.0954"
		start := strings.Index(line, cost)
		if start < 0 {
			cost = "$6.7136"
			start = strings.Index(line, cost)
		}
		if start < 0 {
			t.Fatalf("cost text not found in threads row:\n%s\nfull box:\n%s", line, box)
		}
		ends = append(ends, runewidth.StringWidth(line[:start])+runewidth.StringWidth(cost))
	}
	if len(ends) != 2 || ends[0] != ends[1] {
		t.Fatalf("threads Cost column should right-align across rows, ends=%v\n%s", ends, box)
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
	box := m.threadsBox(m.filteredThreads(), copyFor(LanguageEnglish))
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
	if strings.Contains(box, "很长很长") || !strings.Contains(box, "...") {
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

func TestThreadsFilterByActiveTool(t *testing.T) {
	payload := samplePayload()
	payload.Threads = []query.ThreadResult{
		{ID: "codex-thread", Name: "Codex task", Tool: "codex", Model: "gpt-5.4", Provider: "openai", Usage: usage.TokenUsage{Input: 10}},
		{ID: "claude-thread", Name: "Claude task", Tool: "claude", Model: "claude-opus", Provider: "unknown", Usage: usage.TokenUsage{Input: 20}},
	}
	m := NewModel(payload)
	updated, _ := m.Update(keyMsg("3"))
	m = updated.(model)
	view := stripANSI(m.View())
	if !strings.Contains(view, "codex-thread") || strings.Contains(view, "claude-thread") {
		t.Fatalf("threads should follow active tool filter:\n%s", view)
	}
}

func TestThreadsDefaultOrderFollowsTokenUsage(t *testing.T) {
	payload := samplePayload()
	payload.Threads = []query.ThreadResult{
		{ID: "low-token", Name: "Low token task", Tool: "codex", Model: "gpt-5.4", Provider: "openai", Usage: usage.TokenUsage{Input: 10}},
		{ID: "high-token", Name: "High token task", Tool: "codex", Model: "gpt-5.4", Provider: "openai", Usage: usage.TokenUsage{Input: 100}},
	}
	m := NewModel(payload)
	threads := m.filteredThreads()
	if len(threads) != 2 {
		t.Fatalf("len(threads) = %d, want 2", len(threads))
	}
	if threads[0].ID != "high-token" {
		t.Fatalf("threads should default to token usage desc, got %+v", threads)
	}
}

func TestTUISortToggleOrdersThreadsAndModelUsageByCost(t *testing.T) {
	payload := samplePayload()
	payload.Results = []query.Result{
		{Key: map[string]string{"tool": "codex", "model": "cheap"}, Usage: usage.TokenUsage{Input: 100}, CostUSD: 1},
		{Key: map[string]string{"tool": "codex", "model": "expensive"}, Usage: usage.TokenUsage{Input: 10}, CostUSD: 10},
	}
	payload.Threads = []query.ThreadResult{
		{ID: "cheap-thread", Name: "Cheap", Tool: "codex", Model: "cheap", Provider: "openai", Usage: usage.TokenUsage{Input: 100}, CostUSD: 1},
		{ID: "expensive-thread", Name: "Expensive", Tool: "codex", Model: "expensive", Provider: "openai", Usage: usage.TokenUsage{Input: 10}, CostUSD: 10},
	}
	m := NewModel(payload)
	if m.filteredThreads()[0].ID != "cheap-thread" || m.filteredResults()[0].Key["model"] != "cheap" {
		t.Fatalf("default sort should use tokens desc")
	}
	updated, _ := m.Update(keyMsg("s"))
	m = updated.(model)
	view := stripANSI(m.View())
	if m.filteredThreads()[0].ID != "expensive-thread" || m.filteredResults()[0].Key["model"] != "expensive" {
		t.Fatalf("s should switch threads and model usage to cost desc")
	}
	if !strings.Contains(view, "[Cost]") {
		t.Fatalf("TUI should show active cost sort badge:\n%s", view)
	}
}

func TestModelUsageChartUsesCostMetricWhenSortingByCost(t *testing.T) {
	payload := samplePayload()
	payload.SortBy = query.SortByCost
	payload.Results = []query.Result{
		{Key: map[string]string{"tool": "codex", "model": "cheap"}, Usage: usage.TokenUsage{Input: 1_000_000}, CostUSD: 1},
		{Key: map[string]string{"tool": "codex", "model": "expensive"}, Usage: usage.TokenUsage{Input: 100_000}, CostUSD: 10},
	}
	view := stripANSI(RenderWidth(payload, 160))
	expensiveUnits := modelUsageBarUnits(t, view, "expensive")
	cheapUnits := modelUsageBarUnits(t, view, "cheap")
	if expensiveUnits <= cheapUnits {
		t.Fatalf("cost sort chart should scale bars by cost, expensive=%d cheap=%d\n%s", expensiveUnits, cheapUnits, view)
	}
	if !strings.Contains(view, "$10.0000") || strings.Contains(view, "1,000,000") {
		t.Fatalf("cost sort chart should label rows with USD amounts, not tokens:\n%s", view)
	}
}

func TestTUIHelpRemovesThreadsFocusShortcut(t *testing.T) {
	view := stripANSI(RenderWidth(samplePayload(), 140))
	if strings.Contains(view, "t threads") || strings.Contains(view, "t 会话") {
		t.Fatalf("TUI help should not expose t threads shortcut:\n%s", view)
	}
}

func TestThreadsScrollBarFollowsCursorOffset(t *testing.T) {
	payload := samplePayload()
	for i := 0; i < 16; i++ {
		payload.Threads = append(payload.Threads, query.ThreadResult{
			ID:       "thread-" + string(rune('a'+i)),
			Name:     "Login bug",
			Tool:     "codex",
			Model:    "gpt-5.4",
			Provider: "openai",
			Usage:    usage.TokenUsage{Input: int64(100 - i)},
		})
	}
	m := NewModel(payload)
	m.width = 160
	updated, _ := m.Update(keyMsg("end"))
	m = updated.(model)
	box := stripANSI(m.threadsBox(m.filteredThreads(), copyFor(LanguageEnglish)))
	if !strings.Contains(box, "thread-p") || strings.Contains(box, "thread-a") {
		t.Fatalf("end should scroll the viewport to the last thread:\n%s", box)
	}
	lines := strings.Split(box, "\n")
	lastThumb := -1
	for i, line := range lines {
		if strings.Contains(line, "┃") {
			lastThumb = i
		}
	}
	if lastThumb < len(lines)-4 {
		t.Fatalf("scroll thumb should move near the bottom after end, line=%d total=%d\n%s", lastThumb, len(lines), box)
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

func modelUsageBarUnits(t *testing.T, view, label string) int {
	t.Helper()
	for _, line := range strings.Split(view, "\n") {
		if !strings.Contains(line, label) {
			continue
		}
		count := 0
		for _, r := range line {
			switch r {
			case '█':
				count += 8
			case '▉':
				count += 7
			case '▊':
				count += 6
			case '▋':
				count += 5
			case '▌':
				count += 4
			case '▍':
				count += 3
			case '▎':
				count += 2
			case '▏':
				count++
			}
		}
		return count
	}
	t.Fatalf("model usage chart line for %q missing:\n%s", label, view)
	return 0
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
