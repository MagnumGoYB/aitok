package tui

import (
	"fmt"
	"io"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sosbs/aitok/internal/report"
)

type model struct {
	payload report.Payload
}

func Run(out io.Writer, payload report.Payload) error {
	program := tea.NewProgram(model{payload: payload}, tea.WithOutput(out))
	_, err := program.Run()
	return err
}

func Render(payload report.Payload) string {
	return model{payload: payload}.View()
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m model) View() string {
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39")).Render("aitok token usage")
	subtitle := fmt.Sprintf("%s to %s", m.payload.Window.Start.Format("2006-01-02 15:04"), m.payload.Window.End.Format("2006-01-02 15:04"))
	var b strings.Builder
	b.WriteString(title)
	b.WriteString("\n")
	b.WriteString(subtitle)
	b.WriteString("\n\n")
	if len(m.payload.Results) == 0 {
		b.WriteString("No usage events found for this query.\n")
	} else {
		for _, result := range m.payload.Results {
			b.WriteString(fmt.Sprintf("%s  events=%d total=%d input=%d output=%d cached=%d reasoning=%d\n",
				formatKey(result.Key),
				result.Events,
				result.Usage.NormalizedTotal(),
				result.Usage.Input,
				result.Usage.Output,
				result.Usage.CachedInput,
				result.Usage.Reasoning,
			))
		}
	}
	b.WriteString("\nPress q to quit.\n")
	return b.String()
}

func formatKey(key map[string]string) string {
	parts := make([]string, 0, len(key))
	for k, v := range key {
		parts = append(parts, k+"="+v)
	}
	return strings.Join(parts, ", ")
}
