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
		"Model Usage",
		"Search:",
		"[Tokens]",
		"Price",
		"? help",
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
		"搜索:",
		"[按 Tokens]",
		"价格",
		"模型",
		"请求",
		"? help",
	} {
		if !strings.Contains(view, expected) {
			t.Fatalf("Chinese view missing %q: %s", expected, view)
		}
	}
}

func TestModelShowsHelpOnlyWhenRequested(t *testing.T) {
	m := NewModel(samplePayload())
	view := stripANSI(m.View())
	if !strings.Contains(view, "? help") || strings.Contains(view, "1 All") {
		t.Fatalf("default footer should stay compact and hide full shortcuts:\n%s", view)
	}
	updated, _ := m.Update(keyMsg("?"))
	m = updated.(model)
	view = stripANSI(m.View())
	if !strings.Contains(view, "1 All") || !strings.Contains(view, "4 Gemini") || strings.Contains(view, "1=All") || !strings.Contains(view, "q quit") {
		t.Fatalf("? should toggle full shortcut help:\n%s", view)
	}
}

func TestHeaderPlacesHelpOnTitleRowAndSearchBelow(t *testing.T) {
	view := stripANSI(RenderWidth(samplePayload(), 140))
	titleIndex := strings.Index(view, "Usage Dashboard")
	subtitleIndex := strings.Index(view, "Monitor AI model usage and estimated cost")
	helpIndex := strings.Index(view, "? help")
	searchIndex := strings.Index(view, "Search:")
	if titleIndex < 0 || subtitleIndex < 0 || helpIndex < 0 || searchIndex < 0 {
		t.Fatalf("view missing expected header parts:\n%s", view)
	}
	if !(subtitleIndex < helpIndex && helpIndex < searchIndex) {
		t.Fatalf("help should stay after subtitle and before toolbar metadata:\n%s", view)
	}
	lines := strings.Split(view, "\n")
	if len(lines) < 4 || strings.TrimSpace(lines[0]) != "" || strings.TrimSpace(lines[1]) != "" {
		t.Fatalf("header should keep top padding:\n%s", view)
	}
	for i, line := range lines {
		if !strings.Contains(line, "Monitor AI model usage and estimated cost") {
			continue
		}
		if !strings.HasPrefix(lines[i-1], "  Usage Dashboard") || !strings.HasPrefix(line, "  Monitor AI model usage and estimated cost") {
			t.Fatalf("header should keep left padding:\n%s", view)
		}
		if !strings.Contains(line, "      ? help") {
			t.Fatalf("compact help should render after subtitle with spacing:\n%s", view)
		}
		if i+1 >= len(lines) || strings.TrimSpace(lines[i+1]) == "" {
			t.Fatalf("header should not add extra vertical padding before toolbar:\n%s", view)
		}
		return
	}
	t.Fatalf("subtitle missing:\n%s", view)
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

func TestModelUsageTableShowsMixedPriceDetails(t *testing.T) {
	payload := report.Payload{
		Results: []query.Result{
			{
				Key:      map[string]string{"tool": "codex", "model": "gpt-5.5", "provider": "toska"},
				Requests: 1410,
				Usage:    usage.TokenUsage{Input: 199_100_000, Output: 1_100_000, CachedInput: 180_800_000},
				CostUSD:  947.9961,
				Price: &query.Price{Source: "mixed", Components: []query.Price{
					{Source: "custom", InputUSDPerMTok: 5, OutputUSDPerMTok: 40},
					{Source: "official", InputUSDPerMTok: 5, OutputUSDPerMTok: 30},
				}},
			},
		},
	}
	view := stripANSI(RenderWidth(payload, 180))
	if strings.Contains(view, " mixed ") && !strings.Contains(view, "mixed custom+official $5/30..40/M") {
		t.Fatalf("TUI should show compact mixed price details, got: %s", view)
	}
	if !strings.Contains(view, "mixed custom+official $5/30..40/M") {
		t.Fatalf("TUI missing mixed price details: %s", view)
	}
}

func TestModelUsageChartAndTableAreSeparated(t *testing.T) {
	view := RenderWidth(samplePayload(), 140)
	lines := strings.Split(stripANSI(view), "\n")
	tableIndex := -1
	for i, line := range lines {
		if strings.Contains(line, "Model") && strings.Contains(line, "Req") && strings.Contains(line, "Cached") {
			tableIndex = i
			break
		}
	}
	if tableIndex < 2 {
		t.Fatalf("model usage table header missing: %s", view)
	}
	if strings.Trim(lines[tableIndex-1], " │") != "" {
		t.Fatalf("model usage chart and table must be separated by a blank line: %s", view)
	}
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

func TestModelUsageBarStyleUsesSameHueDepthByRank(t *testing.T) {
	first := modelUsageBarStyle(0, 4).GetForeground()
	second := modelUsageBarStyle(1, 4).GetForeground()
	last := modelUsageBarStyle(3, 4).GetForeground()
	if first == second || second == last || first == last {
		t.Fatalf("bar colors should vary by rank within one hue family, got first=%v second=%v last=%v", first, second, last)
	}
	if first != lipgloss.Color("#0782C8") {
		t.Fatalf("highest-usage bar should use the deepest heat shade, got %v", first)
	}
	if last != lipgloss.Color("#4CC2FF") {
		t.Fatalf("fourth bar should step down toward the thread highlight hue, got %v", last)
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

func TestModelUsageTableAddsBreathingRoomAroundCost(t *testing.T) {
	header := stripANSI(modelTableRow("Model", "Req", "Cost", "Price", "Tokens", "Input", "Output", "Cached"))
	reqEnd := strings.Index(header, "Req") + len("Req")
	costStart := strings.Index(header, "Cost")
	costEnd := costStart + len("Cost")
	priceStart := strings.Index(header, "Price")
	if costStart-reqEnd < 3 || priceStart-costEnd < 3 {
		t.Fatalf("model usage table should keep wider spacing around Cost:\n%s", header)
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

func TestModelUsageScrollsWhenProvidersAreMany(t *testing.T) {
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
	for _, expected := range []string{"provider-a", "provider-b", "provider-c", "provider-d"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("model usage chart should show the first visible provider bars including %q:\n%s", expected, view)
		}
	}
	tableHeader := modelTableHeaderNeedle(view)
	chart := view[:strings.Index(view, tableHeader)]
	if strings.Contains(chart, "provider-e") || strings.Contains(chart, "provider-l") {
		t.Fatalf("model usage chart should fold bars after the top four:\n%s", view)
	}
	if !strings.Contains(view, "8 more folded; scroll the table below to view more") {
		t.Fatalf("model usage chart should explain folded bars:\n%s", view)
	}
	if !strings.Contains(view, "┃") {
		t.Fatalf("model usage table overflow should show a scrollbar:\n%s", view)
	}

	m := NewModel(payload)
	m.width = 160
	updated, _ := m.Update(keyMsg("tab"))
	m = updated.(model)
	updated, _ = m.Update(keyMsg("end"))
	m = updated.(model)
	view = stripANSI(m.View())
	tableHeader = modelTableHeaderNeedle(view)
	table := view[strings.Index(view, tableHeader):]
	if !strings.Contains(table, "provider-l") || strings.Contains(table, "provider-a") {
		t.Fatalf("end should scroll model usage table to the last rows:\n%s", view)
	}
}

func modelTableHeaderNeedle(view string) string {
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "Model") && strings.Contains(line, "Req") && strings.Contains(line, "Cost") && strings.Contains(line, "Price") {
			return line
		}
	}
	return ""
}

func TestDashboardShowsCurrentAnalysisContext(t *testing.T) {
	payload := samplePayload()
	payload.Threads = []query.ThreadResult{
		{ID: "thread-a", Name: "Login bug", Tool: "codex", Model: "gpt-5.4", Provider: "openai", Usage: usage.TokenUsage{Input: 10}},
	}
	m := NewModel(payload)
	m.search = "gpt"
	view := stripANSI(m.View())
	for _, expected := range []string{"Sort:", "[Tokens]", "Search: gpt", "Models: 1", "Threads: 1"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("dashboard context missing %q:\n%s", expected, view)
		}
	}
	if strings.Contains(view, "Models: 1/1") || strings.Contains(view, "Threads: 1/1") {
		t.Fatalf("dashboard should show compact model/thread counts:\n%s", view)
	}
	if strings.Contains(view, "Tool: All") {
		t.Fatalf("dashboard should not render a separate Tool context row:\n%s", view)
	}
}

func TestThreadsShowSelectedDetailStrip(t *testing.T) {
	payload := samplePayload()
	payload.Threads = []query.ThreadResult{
		{ID: "thread-a", Name: "Login bug", Tool: "codex", Model: "gpt-5.4", Provider: "openai", Source: "project-a", LastActiveAt: time.Date(2026, 5, 11, 10, 30, 0, 0, time.UTC), Usage: usage.TokenUsage{Input: 10}, CostBreakdown: []query.ThreadCost{{Provider: "openai", USD: 0.1}}},
	}
	view := stripANSI(RenderWidth(payload, 160))
	for _, expected := range []string{"Selected Thread", "ID: thread-a", "Last Active:", "Tokens: 10", "Cost: $0.1000"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("thread detail strip missing %q:\n%s", expected, view)
		}
	}
	if strings.Contains(view, "Source:") || strings.Contains(view, "project-a") || strings.Contains(view, "Split:") {
		t.Fatalf("thread detail strip should omit Source and Split:\n%s", view)
	}
	lines := strings.Split(view, "\n")
	for _, line := range lines {
		if !strings.Contains(line, "Last Active:") {
			continue
		}
		if !strings.Contains(line, "2026-05-11") {
			t.Fatalf("Last Active value should render on the same line:\n%s", view)
		}
		return
	}
	t.Fatalf("Last Active label missing:\n%s", view)
}

func TestThreadsPanelUsesQuarterWidthDetailAndAlignsBottom(t *testing.T) {
	payload := samplePayload()
	payload.Threads = []query.ThreadResult{
		{ID: "thread-a", Name: "Login bug", Tool: "codex", Model: "gpt-5.4", Provider: "openai", Usage: usage.TokenUsage{Input: 10}},
		{ID: "thread-b", Name: "Deploy", Tool: "codex", Model: "gpt-5.5", Provider: "bcb", Usage: usage.TokenUsage{Input: 8}},
	}
	m := NewModel(payload)
	m.width = 180
	panel := stripANSI(m.threadsPanel(m.filteredThreads(), copyFor(LanguageEnglish)))
	for _, line := range strings.Split(panel, "\n") {
		if got, want := runewidth.StringWidth(strings.TrimRight(line, " ")), dashboardOuterWidth(m.width); got > want {
			t.Fatalf("threads panel line should not exceed dashboard width %d, got %d:\n%s", want, got, panel)
		}
	}
	lines := strings.Split(panel, "\n")
	var topLine string
	for _, line := range lines {
		if strings.Contains(line, "╭") && strings.Contains(line, "╮  ╭") {
			topLine = line
			break
		}
	}
	if topLine == "" {
		t.Fatalf("wide threads panel should render side-by-side boxes:\n%s", panel)
	}
	if got, want := runewidth.StringWidth(topLine), dashboardOuterWidth(m.width); got != want {
		t.Fatalf("threads panel top border should align to dashboard width %d, got %d:\n%s", want, got, panel)
	}
	parts := strings.Split(topLine, "  ")
	if len(parts) < 2 {
		t.Fatalf("wide threads panel should keep a two-column gap:\n%s", panel)
	}
	detailWidth := runewidth.StringWidth(parts[len(parts)-1])
	if detailWidth < 43 {
		t.Fatalf("selected detail should use roughly a quarter of dashboard width, got %d:\n%s", detailWidth, panel)
	}
	var bottomLine string
	for _, line := range lines {
		if strings.Contains(line, "╰") && strings.Contains(line, "╯  ╰") {
			bottomLine = line
		}
	}
	if bottomLine == "" {
		t.Fatalf("threads and selected detail boxes should align their bottom borders:\n%s", panel)
	}
	if got, want := runewidth.StringWidth(bottomLine), dashboardOuterWidth(m.width); got != want {
		t.Fatalf("threads panel bottom border should align to dashboard width %d, got %d:\n%s", want, got, panel)
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

func TestSectionTitlesDoNotRepeatSortBadge(t *testing.T) {
	payload := samplePayload()
	payload.Threads = []query.ThreadResult{
		{ID: "thread-a", Name: "Login bug", Tool: "codex", Model: "gpt-5.4", Provider: "openai", Usage: usage.TokenUsage{Input: 10}},
	}
	view := stripANSI(RenderWidth(payload, 160))
	if strings.Contains(view, "Threads [Tokens]") || strings.Contains(view, "Model Usage [Tokens]") {
		t.Fatalf("section titles should not repeat the global sort badge:\n%s", view)
	}
	for _, expected := range []string{"Sort:", "[Tokens]", "Models:", "Threads:", "Search:"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("toolbar metadata missing %q:\n%s", expected, view)
		}
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

func TestThreadViewportHeightShowsSevenRows(t *testing.T) {
	m := NewModel(samplePayload())
	m.width = 160
	if got := m.threadViewportHeight(); got != 7 {
		t.Fatalf("large screens should show 7 thread rows, got %d", got)
	}
	m.width = 80
	if got := m.threadViewportHeight(); got != 7 {
		t.Fatalf("narrow screens should keep 7 thread rows, got %d", got)
	}
}

func TestTUILayoutUsesCompactToolbarAndCards(t *testing.T) {
	m := NewModel(samplePayload())
	toolbar := stripANSI(m.toolbar(copyFor(LanguageEnglish)))
	if got := len(strings.Split(toolbar, "\n")); got != 5 {
		t.Fatalf("toolbar should stay compact, got %d lines:\n%s", got, toolbar)
	}
	if !strings.Contains(toolbar, "──") {
		t.Fatalf("toolbar should separate tabs from metadata:\n%s", toolbar)
	}
	lines := strings.Split(toolbar, "\n")
	if strings.Contains(lines[1], "2026-05-08") || !strings.Contains(lines[3], "2026-05-08") {
		t.Fatalf("toolbar should place the date window on the metadata row:\n%s", toolbar)
	}
	card := stripANSI(cardWithWidth("Requests", "4,906", "↯", blue, 28))
	if got := len(strings.Split(card, "\n")); got > 5 {
		t.Fatalf("summary cards should stay compact, got %d lines:\n%s", got, card)
	}
}

func TestTUISectionRightEdgesAlign(t *testing.T) {
	payload := samplePayload()
	payload.Threads = []query.ThreadResult{{ID: "thread-a", Name: "Login bug", Tool: "codex", Model: "gpt-5.4", Provider: "openai", Usage: usage.TokenUsage{Input: 10}}}
	view := stripANSI(RenderWidth(payload, 180))
	var rightEdges []int
	for _, line := range strings.Split(view, "\n") {
		trimmed := strings.TrimRight(line, " ")
		if (strings.HasSuffix(trimmed, "╮") || strings.HasSuffix(trimmed, "╯")) && !strings.Contains(trimmed, "╮  ╭") && !strings.Contains(trimmed, "╯  ╰") {
			rightEdges = append(rightEdges, runewidth.StringWidth(trimmed))
		}
	}
	if len(rightEdges) < 4 {
		t.Fatalf("expected section borders in rendered view, got %v\n%s", rightEdges, view)
	}
	for _, edge := range rightEdges {
		if edge != rightEdges[0] && edge != rightEdges[2] {
			t.Fatalf("section right edges should align, got %v\n%s", rightEdges, view)
		}
	}
}

func TestThreadRowColumnsAlignHeaderAndContent(t *testing.T) {
	header := stripANSI(threadRow("ID", "Name", "Tool", "Req", "Cost", "Tokens"))
	row := stripANSI(threadRow("019e167b-b…", "Short title", "codex", "297", "$34.9399", "45.5m"))

	for _, label := range []string{"Name", "Tool", "Req", "Cost", "Tokens"} {
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
		return "Short title"
	case "Tool":
		return "codex"
	case "Req":
		return "297"
	case "Cost":
		return "$34.9399"
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
	if !strings.Contains(box, "ID              Name") {
		t.Fatalf("ID and Name columns should keep readable spacing: %s", box)
	}
	if strings.Contains(box, "Events") {
		t.Fatalf("threads compact box should not render a separate Events column: %s", box)
	}
}

func TestThreadsRowsDoNotWrapWhenDetailIsVisible(t *testing.T) {
	payload := samplePayload()
	payload.Threads = []query.ThreadResult{
		{ID: "019e313e-d7f5-7331-ad54-f296dc232d9f", Name: "检查 PR #18，没什么问题就合并，之后继续发布", Tool: "codex", Model: "gpt-5.4", Provider: "openai", Requests: 1082, Usage: usage.TokenUsage{Input: 153_300_000}, CostUSD: 306.0250},
		{ID: "019e2629-2c11-7331-ad54-f296dc232d9f", Name: "Model Usage 不同 provider 统计量是很长的标题", Tool: "codex", Model: "gpt-5.5", Provider: "toska", Requests: 888, Usage: usage.TokenUsage{Input: 129_100_000}, CostUSD: 615.3263},
	}
	m := NewModel(payload)
	m.width = 180
	panel := stripANSI(m.threadsPanel(m.filteredThreads(), copyFor(LanguageEnglish)))
	for _, line := range strings.Split(panel, "\n") {
		if (strings.Contains(line, "153.3m") || strings.Contains(line, "129.1m")) && strings.Contains(line, "codex") {
			if !strings.Contains(line, "019e") {
				t.Fatalf("thread tokens should stay on the same row as ID:\n%s", panel)
			}
		}
		if got, want := runewidth.StringWidth(strings.TrimRight(line, " ")), dashboardOuterWidth(m.width); got > want {
			t.Fatalf("thread panel line should stay within dashboard width %d, got %d:\n%s", want, got, panel)
		}
	}
}

func TestThreadRowAlignmentPolicy(t *testing.T) {
	header := threadRow("ID", "Name", "Tool", "Req", "Cost", "Tokens")
	row := threadRow("019e", "Fix title", "codex", "261", "$31.3324", "41.4m")

	labels := []string{"ID", "Name", "Tool", "Req", "Cost", "Tokens"}
	values := []string{"019e", "Fix title", "codex", "261", "$31.3324", "41.4m"}
	for i, label := range labels {
		headerStart := strings.Index(header, label)
		rowStart := strings.Index(row, values[i])
		if headerStart < 0 || rowStart < 0 {
			t.Fatalf("missing %q/%q in row output:\n%s\n%s", label, values[i], header, row)
		}
		headerColumn := runewidth.StringWidth(header[:headerStart])
		rowColumn := runewidth.StringWidth(row[:rowStart])
		switch label {
		case "Req", "Cost", "Tokens":
			headerEnd := headerColumn + runewidth.StringWidth(label)
			rowEnd := rowColumn + runewidth.StringWidth(values[i])
			if headerEnd != rowEnd {
				t.Fatalf("numeric column %q should right-align at width %d, got %d:\n%s\n%s", label, headerEnd, rowEnd, header, row)
			}
		default:
			if headerColumn != rowColumn {
				t.Fatalf("column %q should align at width %d, got %d:\n%s\n%s", label, headerColumn, rowColumn, header, row)
			}
		}
		if headerColumn < 0 || rowColumn < 0 {
			t.Fatalf("column %q should align at width %d, got %d:\n%s\n%s", label, headerColumn, rowColumn, header, row)
		}
	}
	if strings.Contains(header, "Split") || strings.Contains(header, "Model") || strings.Contains(header, "Provider") || strings.Contains(row, "toska/bcb") {
		t.Fatalf("thread row should omit split/model/provider columns:\n%s\n%s", header, row)
	}
	if strings.Contains(row, "Fix title                          codex") {
		t.Fatalf("thread row should not keep the old wide Name column:\n%s", row)
	}
}

func TestThreadsBoxShowsProviderListAndCostBreakdown(t *testing.T) {
	payload := samplePayload()
	payload.Threads = []query.ThreadResult{
		{
			ID:       "019e2491-5335-7420-91bc-d555ae79337e",
			Name:     "Cost switch",
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
	for _, expected := range []string{"$301.1100", "66.6m"} {
		if !strings.Contains(box, expected) {
			t.Fatalf("threads box should show compact cost/tokens %q:\n%s", expected, box)
		}
	}
	if strings.Contains(box, "Split") || strings.Contains(box, "Model") || strings.Contains(box, "Provider") || strings.Contains(box, "toska/bcb") {
		t.Fatalf("threads list should not render Split/Model/Provider columns:\n%s", box)
	}
	if strings.Contains(box, "bcb,toska") {
		t.Fatalf("threads list should hide provider values while detail shows full values:\n%s", box)
	}
	if strings.Contains(box, "$301.1100+") {
		t.Fatalf("threads box should keep Cost numeric and use Split instead of plus marker:\n%s", box)
	}
}

func TestThreadsListOmitsModelProviderAndDetailShowsFullValues(t *testing.T) {
	payload := samplePayload()
	payload.Threads = []query.ThreadResult{
		{
			ID:       "thread-a",
			Name:     "Mixed provider task",
			Tool:     "codex",
			Model:    "gpt-5.5,gpt-5.4",
			Provider: "openai,toska",
			Usage:    usage.TokenUsage{Input: 10},
		},
	}
	m := NewModel(payload)
	m.width = 180
	list := stripANSI(m.threadsBoxWithWidth(m.filteredThreads(), copyFor(LanguageEnglish), 132, 0))
	if strings.Contains(list, "Model") || strings.Contains(list, "Provider") || strings.Contains(list, "gpt-5.5") || strings.Contains(list, "openai,toska") {
		t.Fatalf("threads list should omit model/provider values:\n%s", list)
	}
	detail := stripANSI(m.threadDetailStripWithWidth(m.filteredThreads(), copyFor(LanguageEnglish), 42, 0))
	for _, expected := range []string{"Model: gpt-5.5,gpt-5.4", "Provider: openai,toska"} {
		if !strings.Contains(detail, expected) {
			t.Fatalf("thread detail should show full model/provider %q:\n%s", expected, detail)
		}
	}
	if strings.Contains(detail, "Model: gpt-5...") || strings.Contains(detail, "Provider: open...") {
		t.Fatalf("thread detail should not truncate full model/provider values:\n%s", detail)
	}
}

func TestThreadDetailShowsFullActiveAndSplitAndOmitsSource(t *testing.T) {
	payload := samplePayload()
	payload.Threads = []query.ThreadResult{
		{
			ID:           "thread-a",
			Name:         "Mixed provider task",
			Tool:         "codex",
			Model:        "gpt-5.4,gpt-5.5",
			Provider:     "openai,toska",
			Source:       "/Users/sosbs/coding/aitok",
			LastActiveAt: time.Date(2026, 5, 17, 22, 12, 34, 0, time.UTC),
			Usage:        usage.TokenUsage{Input: 10},
			CostBreakdown: []query.ThreadCost{
				{Provider: "toska", USD: 1},
				{Provider: "openai", USD: 2},
			},
		},
	}
	m := NewModel(payload)
	detail := stripANSI(m.threadDetailStripWithWidth(m.filteredThreads(), copyFor(LanguageEnglish), 64, 0))
	lastActive := payload.Threads[0].LastActiveAt.In(time.Local).Format("2006-01-02 15:04")
	for _, expected := range []string{"gpt-5.4,gpt-5.5", "openai,toska", lastActive, "Cost: $3.0000 (toska $1.0000 / openai $2.0000)"} {
		if !strings.Contains(detail, expected) {
			t.Fatalf("thread detail should show full value %q:\n%s", expected, detail)
		}
	}
	if strings.Contains(detail, "Source:") || strings.Contains(detail, "Split:") || strings.Contains(detail, "/Users/sosbs") || strings.Contains(detail, "...") {
		t.Fatalf("thread detail should omit Source/Split and avoid truncating full detail values:\n%s", detail)
	}
}

func TestThreadsBoxOmitsSplitColumnWhenCostIsNotSplit(t *testing.T) {
	payload := samplePayload()
	payload.Threads = []query.ThreadResult{
		{ID: "019e167b-b7e8-7743-8bb3-fd9951e5ef2f", Name: "Single provider", Tool: "codex", Model: "gpt-5.5", Provider: "bcb", Requests: 199, Events: 199, Usage: usage.TokenUsage{Input: 28_345_680}, CostUSD: 22.0954},
	}
	m := NewModel(payload)
	m.width = 180
	box := stripANSI(m.threadsBox(m.filteredThreads(), copyFor(LanguageEnglish)))
	for _, expected := range []string{"Cost", "$22.0954"} {
		if !strings.Contains(box, expected) {
			t.Fatalf("threads box should keep cost without split placeholder %q:\n%s", expected, box)
		}
	}
	if strings.Contains(box, "Split") {
		t.Fatalf("threads list should omit Split column:\n%s", box)
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

func TestThreadsBoxAlignsWideCharactersAndKeepsRowsSingleLine(t *testing.T) {
	payload := samplePayload()
	payload.Threads = []query.ThreadResult{
		{ID: "019e167b-b7e8-7743-8bb3-fd9951e5ef2f", Name: "修正日期范围与threads列表很长很长", Tool: "codex", Model: "gpt-5.5", Provider: "bcb", Requests: 199, Events: 199, Usage: usage.TokenUsage{Input: 28_345_680}, CostUSD: 22.0954},
		{ID: "019e1522-e729-70c2-b013-bf66207c6b51", Name: "mini_program_wechat", Tool: "codex", Model: "gpt-5.5", Provider: "bcb", Requests: 61, Events: 61, Usage: usage.TokenUsage{Input: 6_125_217}, CostUSD: 6.7136},
	}
	m := NewModel(payload)
	m.width = 180
	box := m.threadsBox(m.filteredThreads(), copyFor(LanguageEnglish))
	plain := stripANSI(box)
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
	if !strings.Contains(plain, "修正日期范围与threads列表很长很长") {
		t.Fatalf("thread name should use the wider Name column when it fits:\n%s", box)
	}
	if strings.Contains(plain, "\n很长很长") {
		t.Fatalf("thread name should not wrap into a continuation line:\n%s", box)
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
	updated, cmd = m.Update(keyMsg("C"))
	m = updated.(model)
	if cmd == nil || !strings.Contains(m.copyStatus, "thread-b") {
		t.Fatalf("uppercase C should copy the selected thread, status=%q cmd=%v", m.copyStatus, cmd)
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

func TestTabToModelUsageRemovesThreadHighlight(t *testing.T) {
	payload := samplePayload()
	payload.Threads = []query.ThreadResult{
		{ID: "thread-a", Name: "Login bug", Tool: "codex", Model: "gpt-5.4", Provider: "openai", Usage: usage.TokenUsage{Input: 10}},
	}
	m := NewModel(payload)
	updated, _ := m.Update(keyMsg("tab"))
	m = updated.(model)
	if m.focusedPane != "models" {
		t.Fatalf("tab should move focus to model usage, got %q", m.focusedPane)
	}
	modelsFocused := m.threadsBox(m.filteredThreads(), copyFor(LanguageEnglish))
	if !strings.Contains(stripANSI(modelsFocused), "thread-a") {
		t.Fatalf("threads pane should keep the row visible without relying on selected styling:\n%s", modelsFocused)
	}
}

func TestCopyCommandUsesSystemClipboard(t *testing.T) {
	previous := writeClipboard
	defer func() { writeClipboard = previous }()

	var copied string
	writeClipboard = func(value string) error {
		copied = value
		return nil
	}

	cmd := copyToClipboard("thread-123")
	if cmd == nil {
		t.Fatal("copy command should not be nil")
	}
	cmd()
	if copied != "thread-123" {
		t.Fatalf("copy command should write selected thread ID, got %q", copied)
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
