package tui

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/MagnumGoYB/aitok/internal/query"
	"github.com/MagnumGoYB/aitok/internal/report"
	"github.com/MagnumGoYB/aitok/internal/usage"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const allTools = "all"

type model struct {
	payload    report.Payload
	activeTool string
	search     string
	searching  bool
	width      int
}

func NewModel(payload report.Payload) model {
	return model{payload: payload, activeTool: allTools, width: 120}
}

func Run(out io.Writer, payload report.Payload) error {
	program := tea.NewProgram(NewModel(payload), tea.WithOutput(out))
	_, err := program.Run()
	return err
}

func Render(payload report.Payload) string {
	return NewModel(payload).View()
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
	case tea.KeyMsg:
		key := msg.String()
		if m.searching {
			switch key {
			case "enter", "esc":
				m.searching = false
			case "backspace":
				if len(m.search) > 0 {
					m.search = m.search[:len(m.search)-1]
				}
			case "ctrl+c":
				return m, tea.Quit
			default:
				if len(key) == 1 {
					m.search += key
				}
			}
			return m, nil
		}
		switch key {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "/":
			m.searching = true
		case "esc":
			m.search = ""
			m.searching = false
		case "1":
			m.activeTool = allTools
		case "2":
			m.activeTool = string(usage.ToolClaude)
		case "3":
			m.activeTool = string(usage.ToolCodex)
		case "4":
			m.activeTool = string(usage.ToolGemini)
		}
	}
	return m, nil
}

func (m model) View() string {
	results := m.filteredResults()
	summary := summarize(results)
	var b strings.Builder
	b.WriteString(titleStyle.Render("使用统计"))
	b.WriteString("\n")
	b.WriteString(subtitleStyle.Render("查看 AI 模型的使用情况和成本统计"))
	b.WriteString("\n\n")
	b.WriteString(m.toolbar())
	b.WriteString("\n\n")
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top,
		card("总请求数", formatInt(int64(summary.requests)), "↯", blue),
		card("总成本", report.FormatUSD(summary.cost), "$", green),
		card("总 Token 数", formatInt(summary.total), "▱", purple),
		card("缓存 Token", formatInt(summary.cached), "◉", orange),
	))
	b.WriteString("\n\n")
	b.WriteString(sectionStyle.Render("模型用量"))
	b.WriteString("\n")
	if len(results) == 0 {
		b.WriteString(mutedStyle.Render("No usage events found for this query."))
		b.WriteString("\n")
	} else {
		b.WriteString(m.chart(results, summary.total))
		b.WriteString("\n")
		b.WriteString(m.table(results))
	}
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("1 全部  2 Claude Code  3 Codex  4 Gemini  / 搜索  esc 清空  q 退出"))
	b.WriteString("\n")
	return b.String()
}

func (m model) toolbar() string {
	tabs := []string{
		m.tab(allTools, "全部"),
		m.tab(string(usage.ToolClaude), "Claude Code"),
		m.tab(string(usage.ToolCodex), "Codex"),
		m.tab(string(usage.ToolGemini), "Gemini"),
	}
	search := "Search: " + m.search
	if m.searching {
		search += "▌"
	}
	date := m.payload.Window.Start.Format("2006-01-02")
	if m.payload.Window.Start.IsZero() {
		date = "当日"
	}
	right := mutedStyle.Render("↻ 30s   📅 " + date)
	return toolbarStyle.Render(lipgloss.JoinHorizontal(lipgloss.Center, strings.Join(tabs, "  "), "     ", search, "     ", right))
}

func (m model) tab(value, label string) string {
	if m.activeTool == value {
		return activeTabStyle.Render("[" + label + "]")
	}
	return tabStyle.Render(label)
}

func (m model) chart(results []query.Result, max int64) string {
	if max <= 0 {
		max = 1
	}
	var lines []string
	for _, result := range results {
		total := result.Usage.NormalizedTotal()
		width := int(total * 28 / max)
		if width == 0 && total > 0 {
			width = 1
		}
		lines = append(lines, fmt.Sprintf("%-28s %s %s",
			truncate(resultLabel(result), 28),
			barStyle.Render(strings.Repeat("█", width)),
			mutedStyle.Render(formatInt(total)),
		))
		if len(lines) == 8 {
			break
		}
	}
	return strings.Join(lines, "\n")
}

func (m model) table(results []query.Result) string {
	var b strings.Builder
	b.WriteString(mutedStyle.Render(fmt.Sprintf("%-28s %8s %10s %12s %12s %12s", "Model", "Req", "Cost", "Input", "Output", "Cached")))
	b.WriteString("\n")
	for _, result := range results {
		b.WriteString(fmt.Sprintf("%-28s %8d %10s %12s %12s %12s\n",
			truncate(resultLabel(result), 28),
			result.Requests,
			report.FormatUSD(result.CostUSD),
			compact(result.Usage.Input),
			compact(result.Usage.Output+result.Usage.Reasoning),
			compact(result.Usage.CachedInput+result.Usage.CacheCreation),
		))
		if strings.TrimSpace(b.String()) != "" && strings.Count(b.String(), "\n") > 12 {
			break
		}
	}
	return b.String()
}

func (m model) filteredResults() []query.Result {
	var out []query.Result
	needle := strings.ToLower(strings.TrimSpace(m.search))
	for _, result := range m.payload.Results {
		tool := strings.ToLower(result.Key["tool"])
		if m.activeTool != allTools && tool != m.activeTool {
			continue
		}
		label := strings.ToLower(resultLabel(result))
		if needle != "" && !strings.Contains(label, needle) {
			continue
		}
		out = append(out, result)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Usage.NormalizedTotal() > out[j].Usage.NormalizedTotal()
	})
	return out
}

type totals struct {
	requests int
	cost     float64
	total    int64
	cached   int64
}

func summarize(results []query.Result) totals {
	var out totals
	for _, result := range results {
		out.requests += result.Requests
		out.cost += result.CostUSD
		out.total += result.Usage.NormalizedTotal()
		out.cached += result.Usage.CachedInput + result.Usage.CacheCreation
	}
	return out
}

func resultLabel(result query.Result) string {
	if model := result.Key["model"]; model != "" && model != "unknown" {
		return model
	}
	if tool := result.Key["tool"]; tool != "" {
		return tool
	}
	return formatKey(result.Key)
}

func formatKey(key map[string]string) string {
	parts := make([]string, 0, len(key))
	for k, v := range key {
		parts = append(parts, k+"="+v)
	}
	sort.Strings(parts)
	return strings.Join(parts, ", ")
}

func card(label, value, icon string, color lipgloss.Color) string {
	iconStyle := lipgloss.NewStyle().Foreground(color).Bold(true)
	return cardStyle.Render(labelStyle.Render(label) + "\n\n" + valueStyle.Render(value) + "  " + iconStyle.Render(icon))
}

func formatInt(value int64) string {
	if value < 1000 {
		return fmt.Sprintf("%d", value)
	}
	text := fmt.Sprintf("%d", value)
	var parts []string
	for len(text) > 3 {
		parts = append([]string{text[len(text)-3:]}, parts...)
		text = text[:len(text)-3]
	}
	parts = append([]string{text}, parts...)
	return strings.Join(parts, ",")
}

func compact(value int64) string {
	if value >= 1_000_000 {
		return fmt.Sprintf("%.1fm", float64(value)/1_000_000)
	}
	if value >= 1_000 {
		return fmt.Sprintf("%.1fk", float64(value)/1_000)
	}
	return fmt.Sprintf("%d", value)
}

func truncate(value string, width int) string {
	if len(value) <= width {
		return value
	}
	if width <= 1 {
		return value[:width]
	}
	return value[:width-1] + "…"
}

var (
	blue   = lipgloss.Color("39")
	green  = lipgloss.Color("35")
	purple = lipgloss.Color("99")
	orange = lipgloss.Color("208")

	titleStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15"))
	subtitleStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	toolbarStyle   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("240")).Padding(1, 2)
	activeTabStyle = lipgloss.NewStyle().Foreground(blue).Bold(true).Background(lipgloss.Color("17")).Padding(0, 1)
	tabStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Padding(0, 1)
	cardStyle      = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("240")).Padding(1, 2).Width(24).Height(5).MarginRight(2)
	labelStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	valueStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15"))
	sectionStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15"))
	mutedStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	helpStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	barStyle       = lipgloss.NewStyle().Foreground(blue)
)
