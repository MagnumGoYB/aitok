package tui

import "github.com/charmbracelet/lipgloss"

func card(label, value, icon string, color lipgloss.Color) string {
	return cardWithWidth(label, value, icon, color, 24)
}

func cardWithWidth(label, value, icon string, color lipgloss.Color, width int) string {
	iconStyle := lipgloss.NewStyle().Foreground(color).Bold(true)
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Padding(0, 2).
		Width(width).
		Height(2).
		Render(labelStyle.Render(label) + "\n" + valueStyle.Render(value) + "  " + iconStyle.Render(icon))
}
