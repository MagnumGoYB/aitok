package tui

import (
	"strings"

	"github.com/MagnumGoYB/aitok/internal/query"
	"github.com/MagnumGoYB/aitok/internal/usage"
	"github.com/charmbracelet/lipgloss"
)

type dashboardSections struct {
	header     string
	toolbar    string
	cards      string
	threads    string
	modelUsage string
}

func (m model) View() string {
	return m.viewportView()
}

func (m model) viewportView() string {
	view := m.fullView()
	if m.height <= 0 {
		return view
	}
	lines := strings.Split(view, "\n")
	maxOffset := len(lines) - m.height
	if maxOffset < 0 {
		maxOffset = 0
	}
	offset := m.scrollOffset
	if offset < 0 {
		offset = 0
	}
	if offset > maxOffset {
		offset = maxOffset
	}
	end := offset + m.height
	if end > len(lines) {
		end = len(lines)
	}
	return strings.Join(lines[offset:end], "\n")
}

func (m model) fullView() string {
	if m.width <= 0 {
		m.width = 120
	}
	results := m.filteredResults()
	threads := m.filteredThreads()
	summary := summarize(results)
	copy := copyFor(m.language)
	sections := dashboardSections{
		header:     m.header(copy),
		toolbar:    m.toolbar(copy),
		cards:      m.cards(summary, copy),
		modelUsage: m.modelUsageBox(results, summary.total, copy),
	}
	if len(threads) > 0 {
		sections.threads = m.threadsPanel(threads, copy)
	}

	var b strings.Builder
	b.WriteString(sections.header)
	b.WriteString("\n")
	b.WriteString(sections.toolbar)
	b.WriteString("\n")
	b.WriteString(sections.cards)
	b.WriteString("\n")
	if sections.threads != "" {
		b.WriteString(sections.threads)
		b.WriteString("\n")
	}
	b.WriteString(sections.modelUsage)
	b.WriteString("\n")
	return b.String()
}

func (m model) toolbar(copy localizedCopy) string {
	tabs := []string{
		m.tab(allTools, copy.all),
		m.tab(string(usage.ToolClaude), "Claude Code"),
		m.tab(string(usage.ToolCodex), "Codex"),
		m.tab(string(usage.ToolGemini), "Gemini"),
		m.tab(string(usage.ToolReasonix), "Reasonix"),
	}
	top := strings.Join(tabs, "  ")
	separator := toolbarSeparator(dashboardWidth(m.width) - 6)
	content := lipgloss.JoinVertical(lipgloss.Left, top, separator, m.toolbarMeta(copy))
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("238")).
		Padding(0, 2).
		Width(dashboardWidth(m.width)).
		Render(content)
}

func toolbarSeparator(width int) string {
	if width < 1 {
		width = 1
	}
	return mutedStyle.Render(strings.Repeat("─", width))
}

func (m model) toolbarMeta(copy localizedCopy) string {
	ctx := m.currentViewContext()
	search := "-"
	if ctx.search != "" {
		search = ctx.search
	}
	parts := []string{
		contextLabelStyle.Render(copy.contextSort+": ") + badgeStyle.Render(copy.sortBadge(ctx.sortBy)),
		contextLabelStyle.Render(copy.contextResults+":") + " " + contextValueStyle.Render(formatInt(int64(ctx.shownResults))),
		contextLabelStyle.Render(copy.contextThreads+":") + " " + contextValueStyle.Render(formatInt(int64(ctx.shownThreads))),
		contextLabelStyle.Render(copy.contextSearch+":") + " " + contextValueStyle.Render(search),
		mutedStyle.Render(m.windowLabel(copy)),
	}
	if m.searching {
		parts[len(parts)-1] += "▌"
	}
	return strings.Join(parts, "   ")
}

func (m model) header(copy localizedCopy) string {
	indent := strings.Repeat(" ", 2)
	notice := m.headerNotice(copy)
	titleLine := placeHeaderRight(indent+titleStyle.Render(copy.title), notice, dashboardWidth(m.width))
	subtitleLine := indent + subtitleStyle.Render(copy.subtitle)
	return lipgloss.NewStyle().Width(dashboardWidth(m.width)).Render("\n\n" + titleLine + "\n" + subtitleLine)
}

func (m model) headerNotice(copy localizedCopy) string {
	switch {
	case m.copyStatus != "":
		return statusStyle.Render(m.copyStatus)
	case m.showHelp:
		return helpStyle.Render(copy.helpInline)
	default:
		return helpCompactStyle.Render(copy.helpCompact)
	}
}

func placeHeaderRight(left, right string, width int) string {
	if right == "" {
		return left
	}
	rightWidth := lipgloss.Width(right)
	available := width - lipgloss.Width(left)
	if available <= 2 {
		return left
	}
	if rightWidth > available-2 {
		right = tableText(right, available-2)
		rightWidth = lipgloss.Width(right)
	}
	gap := width - lipgloss.Width(left) - rightWidth
	if gap < 2 {
		gap = 2
	}
	return left + strings.Repeat(" ", gap) + right
}

func (m model) windowLabel(copy localizedCopy) string {
	start := m.payload.Window.Start
	if start.IsZero() {
		return copy.today
	}
	zone := start.Location().String()
	if zone == "Local" {
		zone, _ = start.Zone()
	}
	if m.payload.Period == query.PeriodToday {
		return start.Format("2006-01-02") + " " + zone
	}
	return start.Format("2006-01-02 15:04") + " ~ " + m.payload.Window.End.In(start.Location()).Format("2006-01-02 15:04") + " " + zone
}

func (m model) cards(summary totals, copy localizedCopy) string {
	cardWidths := []int{24, 24, 24, 24}
	if m.width >= 132 {
		available := dashboardWidth(m.width) - 12
		base := available / 4
		remainder := available % 4
		for i := range cardWidths {
			cardWidths[i] = base
			if i < remainder {
				cardWidths[i]++
			}
		}
	}
	cards := []string{
		cardWithWidth(copy.requests, formatInt(int64(summary.requests)), "↯", blue, cardWidths[0]),
		cardWithWidth(copy.cost, tuiFormatCost(summary.cost, summary.currency), tuiCurrencyIcon(summary.currency), green, cardWidths[1]),
		cardWithWidth(copy.totalTokens, formatInt(summary.total), "▱", purple, cardWidths[2]),
		cardWithWidth(copy.cachedTokens, formatInt(summary.cached), "◉", orange, cardWidths[3]),
	}
	if dashboardWidth(m.width) < 108 {
		return lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.JoinHorizontal(lipgloss.Top, cards[0], cards[1]),
			lipgloss.JoinHorizontal(lipgloss.Top, cards[2], cards[3]),
		)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, cards[0], "  ", cards[1], "  ", cards[2], "  ", cards[3])
}

func (m model) tab(value, label string) string {
	if m.activeTool == value {
		return activeTabStyle.Render(label)
	}
	return tabStyle.Render(label)
}
