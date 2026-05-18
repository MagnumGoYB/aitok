package tui

import (
	"fmt"
	"strings"

	"github.com/MagnumGoYB/aitok/internal/query"
	"github.com/charmbracelet/lipgloss"
)

func (m model) threadsBox(threads []query.ThreadResult, copy localizedCopy) string {
	return m.threadsBoxWithWidth(threads, copy, dashboardWidth(m.width), 0)
}

func (m model) threadsBoxWithWidth(threads []query.ThreadResult, copy localizedCopy, width int, minHeight int) string {
	contentWidth := panelContentWidth(width)
	height := m.threadViewportHeight()
	if m.threadCursor >= len(threads) {
		m.threadCursor = len(threads) - 1
	}
	if m.threadCursor < 0 {
		m.threadCursor = 0
	}
	if m.threadOffset > len(threads)-1 {
		m.threadOffset = len(threads) - 1
	}
	if m.threadOffset < 0 {
		m.threadOffset = 0
	}
	end := m.threadOffset + height
	if end > len(threads) {
		end = len(threads)
	}
	overflow := len(threads) > height
	rowWidth := contentWidth - 4
	if overflow {
		rowWidth -= 2
	}
	header := mutedStyle.Render(threadLine(threadRowWithWidth(copy.headerID, copy.headerName, copy.headerTool, copy.headerReq, copy.headerCost, copy.headerTokens, rowWidth), -1, m.threadOffset, height, len(threads), overflow))
	var lines []string
	lines = append(lines, sectionStyle.Render(copy.threads))
	lines = append(lines, header)
	for i := m.threadOffset; i < end; i++ {
		thread := threads[i]
		line := threadRowWithWidth(
			thread.ID,
			thread.Name,
			thread.Tool,
			fmt.Sprint(thread.Requests),
			tuiThreadCost(thread),
			compact(thread.Usage.NormalizedTotal()),
			rowWidth,
		)
		line = threadLine(line, i-m.threadOffset, m.threadOffset, height, len(threads), overflow)
		if m.focusedPane != "models" && i == m.threadCursor {
			line = selectedRowStyle.Render(line)
		}
		lines = append(lines, line)
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("238")).
		Padding(1, 2).
		Width(contentWidth).
		Height(minHeight).
		Render(strings.Join(lines, "\n"))
}

func (m model) threadDetailStrip(threads []query.ThreadResult, copy localizedCopy) string {
	return m.threadDetailStripWithWidth(threads, copy, dashboardWidth(m.width), 0)
}

func (m model) threadDetailStripWithWidth(threads []query.ThreadResult, copy localizedCopy, width int, minHeight int) string {
	if len(threads) == 0 || m.threadCursor < 0 || m.threadCursor >= len(threads) {
		return ""
	}
	detail := selectedThreadDetail(threads[m.threadCursor])
	contentWidth := panelContentWidth(width)
	valueWidth := contentWidth - 10
	if valueWidth < 12 {
		valueWidth = 12
	}
	lines := []string{
		sectionStyle.Render(copy.threadDetail),
		detailLabelStyle.Render("ID:") + " " + detailValueStyle.Render(tableText(detail.id, valueWidth)),
		detailLabelStyle.Render(copy.headerModel+":") + " " + detailValueStyle.Render(fallbackValue(detail.model)),
		detailLabelStyle.Render(copy.headerProvider+":") + " " + detailValueStyle.Render(fallbackValue(detail.provider)),
		detailLabelStyle.Render(copy.threadLastActive+":") + " " + detailValueStyle.Render(detail.lastActive),
		detailLabelStyle.Render(copy.threadTokens+":") + " " + detailValueStyle.Render(tableText(detail.tokens, valueWidth)),
		detailLabelStyle.Render(copy.headerCost+":") + " " + detailValueStyle.Render(detail.split),
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("238")).
		Padding(1, 2).
		Width(contentWidth).
		Height(minHeight).
		Render(strings.Join(lines, "\n"))
}

func (m model) threadsPanel(threads []query.ThreadResult, copy localizedCopy) string {
	width := dashboardOuterWidth(m.width)
	if width < 132 {
		return m.threadsBoxWithWidth(threads, copy, width, 0) + "\n" + m.threadDetailStripWithWidth(threads, copy, width, 0)
	}
	gap := 2
	detailWidth := width * 22 / 100
	if detailWidth < 44 {
		detailWidth = 44
	}
	listWidth := width - detailWidth - gap
	if listWidth < 84 {
		listWidth = 84
		detailWidth = width - listWidth - gap
	}
	panelHeight := m.threadViewportHeight() + 4
	list := m.threadsBoxWithWidth(threads, copy, listWidth, panelHeight)
	detail := m.threadDetailStripWithWidth(threads, copy, detailWidth, panelHeight)
	actualWidth := renderedWidth(list) + gap + renderedWidth(detail)
	if delta := width - actualWidth; delta > 0 {
		detailWidth += delta
		detail = m.threadDetailStripWithWidth(threads, copy, detailWidth, panelHeight)
	}
	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		list,
		strings.Repeat(" ", gap),
		detail,
	)
}

func renderedWidth(value string) int {
	width := 0
	for _, line := range strings.Split(value, "\n") {
		if lineWidth := lipgloss.Width(line); lineWidth > width {
			width = lineWidth
		}
	}
	return width
}

func panelContentWidth(outerWidth int) int {
	contentWidth := outerWidth - 2
	if contentWidth < 1 {
		return 1
	}
	return contentWidth
}

func dashboardOuterWidth(width int) int {
	return dashboardWidth(width) + 2
}

func fallbackValue(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}
