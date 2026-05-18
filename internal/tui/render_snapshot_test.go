package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/MagnumGoYB/aitok/internal/query"
	"github.com/MagnumGoYB/aitok/internal/report"
	"github.com/MagnumGoYB/aitok/internal/usage"
	"github.com/mattn/go-runewidth"
)

func TestDashboardRenderHardeningAcrossFixedWidths(t *testing.T) {
	for _, width := range []int{100, 120, 160} {
		t.Run(fmt.Sprintf("width_%d", width), func(t *testing.T) {
			view := stripANSI(RenderWidth(hardeningPayload(), width))
			assertDashboardHeaderContract(t, view)
			assertDashboardSectionWidths(t, view, width)
			assertModelUsageFoldContract(t, view)
			if width >= 132 {
				assertWideThreadsPanelContract(t, view, width)
			}
		})
	}
}

func assertDashboardHeaderContract(t *testing.T, view string) {
	t.Helper()
	lines := strings.Split(view, "\n")
	if len(lines) < 4 || strings.TrimSpace(lines[0]) != "" || strings.TrimSpace(lines[1]) != "" {
		t.Fatalf("dashboard title should keep top breathing room:\n%s", view)
	}
	if !strings.HasPrefix(lines[2], "  Usage Dashboard") {
		t.Fatalf("dashboard title should keep left padding and stay after top padding:\n%s", view)
	}
	titleLine := ""
	subtitleLine := ""
	for _, line := range lines {
		if strings.Contains(line, "Usage Dashboard") {
			titleLine = line
		}
		if strings.Contains(line, "Monitor AI model usage and estimated cost") {
			subtitleLine = line
			break
		}
	}
	if titleLine == "" || subtitleLine == "" || !strings.Contains(titleLine, "? help") {
		t.Fatalf("compact help should stay in the top-right header notice:\n%s", view)
	}
	if strings.Contains(view, "Threads [Tokens]") || strings.Contains(view, "Model Usage [Tokens]") || strings.Contains(view, "[Tokens/Cost]") {
		t.Fatalf("section titles should not repeat global sort badges:\n%s", view)
	}
}

func assertDashboardSectionWidths(t *testing.T, view string, terminalWidth int) {
	t.Helper()
	maxWidth := dashboardOuterWidth(terminalWidth)
	for _, line := range strings.Split(view, "\n") {
		if got := runewidth.StringWidth(strings.TrimRight(line, " ")); got > maxWidth {
			t.Fatalf("rendered line should stay within dashboard width %d, got %d:\n%s", maxWidth, got, view)
		}
	}
}

func assertWideThreadsPanelContract(t *testing.T, view string, terminalWidth int) {
	t.Helper()
	lines := strings.Split(view, "\n")
	wantWidth := dashboardOuterWidth(terminalWidth)
	foundTop := false
	foundBottom := false
	for _, line := range lines {
		trimmed := strings.TrimRight(line, " ")
		switch {
		case strings.Contains(trimmed, "╭") && strings.Contains(trimmed, "╮  ╭"):
			foundTop = true
			if got := runewidth.StringWidth(trimmed); got != wantWidth {
				t.Fatalf("threads/detail top border should align to dashboard width %d, got %d:\n%s", wantWidth, got, view)
			}
		case strings.Contains(trimmed, "╰") && strings.Contains(trimmed, "╯  ╰"):
			foundBottom = true
			if got := runewidth.StringWidth(trimmed); got != wantWidth {
				t.Fatalf("threads/detail bottom border should align to dashboard width %d, got %d:\n%s", wantWidth, got, view)
			}
		case strings.Contains(trimmed, "019e") && (strings.Contains(trimmed, "$306.0250") || strings.Contains(trimmed, "$615.3263")):
			if !strings.Contains(trimmed, "codex") {
				t.Fatalf("thread row should remain single-line with tool/cost/tokens visible:\n%s", view)
			}
		}
	}
	if !foundTop || !foundBottom {
		t.Fatalf("wide dashboard should render side-by-side Threads and Selected Thread boxes:\n%s", view)
	}
	for _, expected := range []string{
		"Selected Thread",
		"Model: gpt-5.4,gpt-5.5",
		"Provider: openai,toska",
		"Cost: $306.0250 (toska $299.7700 /",
		"openai $6.2550)",
	} {
		if !strings.Contains(view, expected) {
			t.Fatalf("selected thread detail missing %q:\n%s", expected, view)
		}
	}
	if strings.Contains(view, "Split:") || strings.Contains(view, "Source:") {
		t.Fatalf("selected thread detail should not reintroduce Split or Source rows:\n%s", view)
	}
}

func assertModelUsageFoldContract(t *testing.T, view string) {
	t.Helper()
	if !strings.Contains(view, "8 more folded; scroll the table below to view more") {
		t.Fatalf("model usage should fold bars after the top four and explain table scrolling:\n%s", view)
	}
	tableHeader := modelTableHeaderNeedle(view)
	if tableHeader == "" {
		t.Fatalf("model usage table header missing:\n%s", view)
	}
	chart := view[:strings.Index(view, tableHeader)]
	for _, expected := range []string{"provider-a", "provider-b", "provider-c", "provider-d"} {
		if !strings.Contains(chart, expected) {
			t.Fatalf("model usage chart should include top four providers, missing %q:\n%s", expected, view)
		}
	}
	if strings.Contains(chart, "provider-e") || strings.Contains(chart, "provider-l") {
		t.Fatalf("model usage chart should not render folded provider bars:\n%s", view)
	}
	if !strings.Contains(view, "┃") {
		t.Fatalf("model usage table should show a scrollbar when rows overflow:\n%s", view)
	}
}

func hardeningPayload() report.Payload {
	loc := time.FixedZone("CST", 8*60*60)
	payload := report.Payload{
		Period: query.PeriodThisWeek,
		Window: query.Window{
			Start: time.Date(2026, 5, 11, 0, 0, 0, 0, loc),
			End:   time.Date(2026, 5, 18, 0, 0, 0, 0, loc),
		},
	}
	for i := 0; i < 12; i++ {
		provider := fmt.Sprintf("provider-%c", rune('a'+i))
		payload.Results = append(payload.Results, query.Result{
			Key:      map[string]string{"tool": "codex", "model": "gpt-5.5", "provider": provider},
			Requests: 100 - i,
			Usage:    usage.TokenUsage{Input: int64(12-i) * 1_000_000},
			CostUSD:  float64(12-i) * 10,
		})
	}
	payload.Threads = []query.ThreadResult{
		{
			ID:       "019e313e-d7f5-7331-ad54-f296dc232d9f",
			Name:     "检查 PR #18 并继续发布",
			Tool:     "codex",
			Model:    "gpt-5.4,gpt-5.5",
			Provider: "openai,toska",
			Requests: 1082,
			Usage:    usage.TokenUsage{Input: 153_300_000},
			CostUSD:  306.0250,
			CostBreakdown: []query.ThreadCost{
				{Provider: "toska", USD: 299.7700},
				{Provider: "openai", USD: 6.2550},
			},
			LastActiveAt: time.Date(2026, 5, 18, 9, 30, 0, 0, loc),
		},
		{
			ID:       "019e2629-2c11-7331-ad54-f296dc232d9f",
			Name:     "Model Usage 不同 provider 统计量长标题",
			Tool:     "codex",
			Model:    "gpt-5.5",
			Provider: "toska",
			Requests: 888,
			Usage:    usage.TokenUsage{Input: 129_100_000},
			CostUSD:  615.3263,
		},
	}
	return payload
}
