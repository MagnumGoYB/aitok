package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type tableAlign int

const (
	alignLeft tableAlign = iota
	alignRight
)

type tableColumn struct {
	value string
	width int
	align tableAlign
}

var modelColumnWidth = 36

func modelTableRow(modelName, tool, req, cost, price, tokens, input, output, cached string) string {
	columns := []tableColumn{
		{value: modelName, width: 24, align: alignLeft},
		{value: tool, width: 8, align: alignLeft},
		{value: req, width: 5, align: alignRight},
		{value: cost, width: 9, align: alignRight},
		{value: tableText(price, 28), width: 28, align: alignLeft},
		{value: tokens, width: 8, align: alignRight},
		{value: input, width: 7, align: alignRight},
		{value: output, width: 7, align: alignRight},
		{value: cached, width: 6, align: alignRight},
	}
	gaps := []int{2, 2, 2, 2, 2, 1, 1, 1}
	return tableRow(columns, gaps)
}

func threadRow(id, name, tool, req, cost, tokens string) string {
	return threadRowWithWidth(id, name, tool, req, cost, tokens, 0)
}

func threadRowWithWidth(id, name, tool, req, cost, tokens string, maxWidth int) string {
	gaps := []int{2, 2, 3, 4, 3}
	nameWidth := 38
	if maxWidth > 0 {
		fixedWidth := 14 + 10 + 5 + 11 + 9
		for _, gap := range gaps {
			fixedWidth += gap
		}
		nameWidth = maxWidth - fixedWidth
		if nameWidth < 10 {
			nameWidth = 10
		}
	}
	columns := []tableColumn{
		{value: id, width: 14, align: alignLeft},
		{value: name, width: nameWidth, align: alignLeft},
		{value: tool, width: 10, align: alignLeft},
		{value: req, width: 5, align: alignRight},
		{value: cost, width: 11, align: alignRight},
		{value: tokens, width: 9, align: alignRight},
	}
	return tableRow(columns, gaps)
}

func tableRow(columns []tableColumn, gaps []int) string {
	parts := make([]string, 0, len(columns)*2-1)
	for i, column := range columns {
		value := tableText(column.value, column.width)
		switch column.align {
		case alignRight:
			parts = append(parts, padLeft(value, column.width))
		default:
			parts = append(parts, padRight(value, column.width))
		}
		if i < len(columns)-1 {
			gap := 1
			if i < len(gaps) {
				gap = gaps[i]
			}
			parts = append(parts, strings.Repeat(" ", gap))
		}
	}
	return strings.Join(parts, "")
}

func threadLine(row string, visibleIndex, offset, visibleHeight, total int, overflow bool) string {
	if !overflow {
		return row
	}
	return row + " " + scrollMarker(visibleIndex, offset, visibleHeight, total)
}

func scrollMarker(visibleIndex, offset, visibleHeight, total int) string {
	if visibleIndex < 0 {
		return " "
	}
	if total <= visibleHeight || visibleHeight <= 0 {
		return " "
	}
	thumbHeight := visibleHeight * visibleHeight / total
	if thumbHeight < 1 {
		thumbHeight = 1
	}
	if thumbHeight > visibleHeight {
		thumbHeight = visibleHeight
	}
	track := visibleHeight - thumbHeight
	start := 0
	if track > 0 {
		maxOffset := total - visibleHeight
		if maxOffset > 0 {
			start = offset * track / maxOffset
		}
	}
	if visibleIndex >= start && visibleIndex < start+thumbHeight {
		return "┃"
	}
	return "│"
}

func padRight(value string, width int) string {
	if padding := width - lipgloss.Width(value); padding > 0 {
		return value + strings.Repeat(" ", padding)
	}
	return value
}

func padLeft(value string, width int) string {
	if padding := width - lipgloss.Width(value); padding > 0 {
		return strings.Repeat(" ", padding) + value
	}
	return value
}

func dashboardWidth(width int) int {
	return clamp(width-4, 72, 180)
}

func clamp(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}
