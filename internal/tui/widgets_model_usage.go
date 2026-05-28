package tui

import (
	"fmt"
	"strings"

	"github.com/MagnumGoYB/aitok/internal/query"
	"github.com/charmbracelet/lipgloss"
)

func (m model) modelUsageBox(results []query.Result, total int64, copy localizedCopy) string {
	var lines []string
	lines = append(lines, sectionStyle.Render(copy.modelUsage))
	if len(results) == 0 {
		message := copy.empty
		if len(m.payload.Results) > 0 {
			message = copy.emptyFiltered
		}
		lines = append(lines, mutedStyle.Render(message))
	} else {
		lines = append(lines, strings.Split(m.chart(results, total, copy), "\n")...)
		lines = append(lines, "")
		lines = append(lines, m.tableLines(results, copy)...)
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Padding(1, 2).
		Width(dashboardWidth(m.width)).
		Render(strings.Join(lines, "\n"))
}

func (m model) chart(results []query.Result, max int64, copy localizedCopy) string {
	maxValue := float64(max)
	if normalizePayloadSort(m.sortBy) == query.SortByCost {
		maxValue = 0
		for _, result := range results {
			if result.CostUSD > maxValue {
				maxValue = result.CostUSD
			}
		}
	}
	if maxValue <= 0 {
		maxValue = 1
	}
	height := m.modelUsageChartRows(len(results))
	end := height
	if end > len(results) {
		end = len(results)
	}
	var lines []string
	for i := 0; i < end; i++ {
		result := results[i]
		value := m.modelUsageMetricValue(result)
		labelValue := m.modelUsageMetricLabel(result)
		label := padRight(tableText(resultLabel(result), modelColumnWidth), modelColumnWidth)
		line := fmt.Sprintf("%s  %s %s",
			label,
			modelUsageBarStyle(i, minInt(len(results), 6)).Render(metricBar(value, maxValue, 28)),
			mutedStyle.Render(labelValue),
		)
		lines = append(lines, line)
	}
	if hidden := len(results) - end; hidden > 0 {
		lines = append(lines, mutedStyle.Render(fmt.Sprintf(copy.modelUsageHidden, hidden)))
	}
	return strings.Join(lines, "\n")
}

func (m model) modelUsageMetricValue(result query.Result) float64 {
	if normalizePayloadSort(m.sortBy) == query.SortByCost {
		return result.CostUSD
	}
	return float64(result.Usage.NormalizedTotal())
}

func (m model) modelUsageMetricLabel(result query.Result) string {
	if normalizePayloadSort(m.sortBy) == query.SortByCost {
		return tuiFormatCost(result.CostUSD, resultCurrency2(result))
	}
	return formatInt(result.Usage.NormalizedTotal())
}

func (m model) table(results []query.Result, copy localizedCopy) string {
	return strings.Join(m.tableLines(results, copy), "\n") + "\n"
}

func (m model) tableLines(results []query.Result, copy localizedCopy) []string {
	height := m.modelUsageTableRows()
	if m.modelOffset >= len(results) {
		m.modelOffset = len(results) - 1
	}
	if m.modelOffset < 0 {
		m.modelOffset = 0
	}
	end := m.modelOffset + height
	if end > len(results) {
		end = len(results)
	}
	overflow := len(results) > height
	header := mutedStyle.Render(modelUsageTableLine(modelTableRow(copy.headerModel, copy.headerReq, copy.headerCost, copy.headerPrice, copy.headerTokens, copy.headerInput, copy.headerOutput, copy.headerCached), -1, m.modelOffset, height, len(results), overflow))
	lines := []string{header}
	for i := m.modelOffset; i < end; i++ {
		result := results[i]
		line := modelUsageTableLine(modelTableRow(
			resultLabel(result),
			fmt.Sprint(result.Requests),
			tuiFormatCost(result.CostUSD, resultCurrency2(result)),
			priceLabel(result.Price, result.PriceSource),
			compact(result.Usage.NormalizedTotal()),
			compact(result.Usage.Input),
			compact(result.Usage.Output),
			compact(result.Usage.CachedInput+result.Usage.CacheCreation),
		), i-m.modelOffset, m.modelOffset, height, len(results), overflow)
		if m.focusedPane == "models" && i == m.modelCursor {
			line = selectedRowStyle.Render(line)
		}
		lines = append(lines, line)
	}
	return lines
}

func (m model) modelUsageTableRows() int {
	return 5
}

func (m model) modelUsageChartRows(total int) int {
	rows := 4
	if total < rows {
		return total
	}
	return rows
}

func modelUsageTableLine(row string, visibleIndex, offset, visibleHeight, total int, overflow bool) string {
	return threadLine(row, visibleIndex, offset, visibleHeight, total, overflow)
}

func tokenBar(total, max int64, width int) string {
	return metricBar(float64(total), float64(max), width)
}

func metricBar(value, max float64, width int) string {
	if value <= 0 || max <= 0 || width <= 0 {
		return ""
	}
	units := int((value*float64(width*8) + max/2) / max)
	if units < 1 {
		units = 1
	}
	maxUnits := width * 8
	if units > maxUnits {
		units = maxUnits
	}
	full := units / 8
	remainder := units % 8
	bar := strings.Repeat("█", full)
	if remainder > 0 {
		bar += string([]rune("▏▎▍▌▋▊▉")[remainder-1])
	}
	return bar
}

func modelUsageBarStyle(index, total int) lipgloss.Style {
	if total <= 0 {
		total = 1
	}
	palette := []lipgloss.Color{
		lipgloss.Color("#0782C8"),
		lipgloss.Color("#1598D8"),
		lipgloss.Color("#27AEE8"),
		lipgloss.Color("#4CC2FF"),
		lipgloss.Color("#8AD8FF"),
		lipgloss.Color("#C7EEFF"),
	}
	if total == 1 {
		return lipgloss.NewStyle().Foreground(palette[0])
	}
	if index < 0 {
		index = 0
	}
	return lipgloss.NewStyle().Foreground(palette[index%len(palette)])
}
