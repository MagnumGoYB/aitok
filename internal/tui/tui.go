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

type Language string

const (
	LanguageEnglish Language = "en"
	LanguageChinese Language = "zh-CN"
)

type model struct {
	payload    report.Payload
	activeTool string
	search     string
	searching  bool
	width      int
	language   Language
}

func NewModel(payload report.Payload) model {
	return NewModelWithLanguage(payload, LanguageEnglish)
}

func NewModelWithLanguage(payload report.Payload, language Language) model {
	return model{payload: payload, activeTool: allTools, width: 120, language: normalizeLanguage(language)}
}

func Run(out io.Writer, payload report.Payload) error {
	return RunWithLanguage(out, payload, LanguageEnglish)
}

func RunWithLanguage(out io.Writer, payload report.Payload, language Language) error {
	program := tea.NewProgram(NewModelWithLanguage(payload, language), tea.WithOutput(out), tea.WithAltScreen())
	_, err := program.Run()
	return err
}

func Render(payload report.Payload) string {
	return NewModel(payload).View()
}

func RenderWidth(payload report.Payload, width int) string {
	return RenderWidthWithLanguage(payload, width, LanguageEnglish)
}

func RenderWidthWithLanguage(payload report.Payload, width int, language Language) string {
	m := NewModelWithLanguage(payload, language)
	if width > 0 {
		m.width = width
	}
	return m.View()
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
		case "l":
			m.language = toggleLanguage(m.language)
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
	if m.width <= 0 {
		m.width = 120
	}
	results := m.filteredResults()
	summary := summarize(results)
	copy := copyFor(m.language)
	var b strings.Builder
	b.WriteString(titleStyle.Render(copy.title))
	b.WriteString("\n")
	b.WriteString(subtitleStyle.Render(copy.subtitle))
	b.WriteString("\n\n")
	b.WriteString(m.toolbar(copy))
	b.WriteString("\n\n")
	b.WriteString(m.cards(summary, copy))
	b.WriteString("\n\n")
	b.WriteString(sectionStyle.Render(copy.modelUsage))
	b.WriteString("\n")
	if len(results) == 0 {
		b.WriteString(mutedStyle.Render(copy.empty))
		b.WriteString("\n")
	} else {
		b.WriteString(m.chart(results, summary.total))
		b.WriteString("\n\n")
		b.WriteString(m.table(results))
	}
	b.WriteString("\n")
	b.WriteString(helpStyle.Render(copy.help))
	b.WriteString("\n")
	return b.String()
}

func (m model) toolbar(copy localizedCopy) string {
	tabs := []string{
		m.tab(allTools, copy.all),
		m.tab(string(usage.ToolClaude), "Claude Code"),
		m.tab(string(usage.ToolCodex), "Codex"),
		m.tab(string(usage.ToolGemini), "Gemini"),
	}
	search := copy.search + ": " + m.search
	if m.searching {
		search += "▌"
	}
	date := m.payload.Window.Start.Format("2006-01-02")
	if m.payload.Window.Start.IsZero() {
		date = copy.today
	}
	right := mutedStyle.Render("📅 " + date)
	content := lipgloss.JoinHorizontal(lipgloss.Center, strings.Join(tabs, "  "), "     ", search, "     ", right)
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Padding(1, 2).
		Width(clamp(m.width-4, 72, 180)).
		Render(content)
}

func (m model) cards(summary totals, copy localizedCopy) string {
	cardWidth := 24
	if m.width >= 132 {
		cardWidth = (m.width - 14) / 4
	}
	cards := []string{
		cardWithWidth(copy.requests, formatInt(int64(summary.requests)), "↯", blue, cardWidth),
		cardWithWidth(copy.cost, report.FormatUSD(summary.cost), "$", green, cardWidth),
		cardWithWidth(copy.totalTokens, formatInt(summary.total), "▱", purple, cardWidth),
		cardWithWidth(copy.cachedTokens, formatInt(summary.cached), "◉", orange, cardWidth),
	}
	if m.width < 96 {
		return lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.JoinHorizontal(lipgloss.Top, cards[0], cards[1]),
			lipgloss.JoinHorizontal(lipgloss.Top, cards[2], cards[3]),
		)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, cards...)
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
		lines = append(lines, fmt.Sprintf("%-*s  %s %s",
			modelColumnWidth,
			truncate(resultLabel(result), modelColumnWidth),
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
	b.WriteString(mutedStyle.Render(fmt.Sprintf("%-*s  %8s %10s %12s %12s %12s", modelColumnWidth, "Model", "Req", "Cost", "Input", "Output", "Cached")))
	b.WriteString("\n")
	for _, result := range results {
		b.WriteString(fmt.Sprintf("%-*s  %8d %10s %12s %12s %12s\n",
			modelColumnWidth,
			truncate(resultLabel(result), modelColumnWidth),
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
		if provider := result.Key["provider"]; provider != "" && provider != "unknown" {
			return fmt.Sprintf("%s (%s)", model, provider)
		}
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

type localizedCopy struct {
	title        string
	subtitle     string
	all          string
	search       string
	today        string
	requests     string
	cost         string
	totalTokens  string
	cachedTokens string
	modelUsage   string
	empty        string
	help         string
}

func copyFor(language Language) localizedCopy {
	if normalizeLanguage(language) == LanguageChinese {
		return localizedCopy{
			title:        "使用统计",
			subtitle:     "查看 AI 模型的使用情况和成本统计",
			all:          "全部",
			search:       "Search",
			today:        "当日",
			requests:     "总请求数",
			cost:         "总成本",
			totalTokens:  "总 Token 数",
			cachedTokens: "缓存 Token",
			modelUsage:   "模型用量",
			empty:        "当前查询没有找到用量事件。",
			help:         "1 全部  2 Claude Code  3 Codex  4 Gemini  / 搜索  esc 清空  l 语言  q 退出",
		}
	}
	return localizedCopy{
		title:        "Usage Dashboard",
		subtitle:     "Monitor AI model usage and estimated cost",
		all:          "All",
		search:       "Search",
		today:        "Today",
		requests:     "Requests",
		cost:         "Estimated Cost",
		totalTokens:  "Total Tokens",
		cachedTokens: "Cached Tokens",
		modelUsage:   "Model Usage",
		empty:        "No usage events found for this query.",
		help:         "1 All  2 Claude Code  3 Codex  4 Gemini  / search  esc clear  l language  q quit",
	}
}

func normalizeLanguage(language Language) Language {
	switch Language(strings.ToLower(string(language))) {
	case "zh", "zh-cn", "cn":
		return LanguageChinese
	default:
		return LanguageEnglish
	}
}

func toggleLanguage(language Language) Language {
	if normalizeLanguage(language) == LanguageChinese {
		return LanguageEnglish
	}
	return LanguageChinese
}

func card(label, value, icon string, color lipgloss.Color) string {
	return cardWithWidth(label, value, icon, color, 24)
}

func cardWithWidth(label, value, icon string, color lipgloss.Color, width int) string {
	iconStyle := lipgloss.NewStyle().Foreground(color).Bold(true)
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Padding(1, 2).
		Width(width).
		Height(5).
		MarginRight(2).
		Render(labelStyle.Render(label) + "\n\n" + valueStyle.Render(value) + "  " + iconStyle.Render(icon))
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

func clamp(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

var (
	modelColumnWidth = 32

	blue   = lipgloss.Color("39")
	green  = lipgloss.Color("35")
	purple = lipgloss.Color("99")
	orange = lipgloss.Color("208")

	titleStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15"))
	subtitleStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	activeTabStyle = lipgloss.NewStyle().Foreground(blue).Bold(true).Background(lipgloss.Color("17")).Padding(0, 1)
	tabStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Padding(0, 1)
	labelStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	valueStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15"))
	sectionStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15"))
	mutedStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	helpStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	barStyle       = lipgloss.NewStyle().Foreground(blue)
)
