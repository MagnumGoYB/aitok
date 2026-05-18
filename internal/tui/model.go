package tui

import (
	"io"
	"strings"
	"time"

	"github.com/MagnumGoYB/aitok/internal/query"
	"github.com/MagnumGoYB/aitok/internal/report"
	"github.com/MagnumGoYB/aitok/internal/usage"
	tea "github.com/charmbracelet/bubbletea"
)

const allTools = "all"

const refreshInterval = 5 * time.Second
const copyStatusDuration = 2 * time.Second

type model struct {
	payload      report.Payload
	activeTool   string
	search       string
	searching    bool
	width        int
	height       int
	scrollOffset int
	language     Language
	refresh      func() (report.Payload, error)
	focusedPane  string
	sortBy       query.SortMetric
	threadCursor int
	threadOffset int
	modelCursor  int
	modelOffset  int
	copyStatus   string
	showHelp     bool
}

type refreshResultMsg struct {
	payload report.Payload
	err     error
}

type clearCopyStatusMsg struct{}

func NewModel(payload report.Payload) model {
	return NewModelWithLanguage(payload, LanguageEnglish)
}

func NewModelWithLanguage(payload report.Payload, language Language) model {
	return model{payload: payload, activeTool: allTools, width: 120, language: normalizeLanguage(language), sortBy: normalizePayloadSort(payload.SortBy)}
}

func NewModelWithRefresh(payload report.Payload, language Language, refresh func() (report.Payload, error)) model {
	m := NewModelWithLanguage(payload, language)
	m.refresh = refresh
	return m
}

func Run(out io.Writer, payload report.Payload) error {
	return RunWithLanguage(out, payload, LanguageEnglish)
}

func RunWithLanguage(out io.Writer, payload report.Payload, language Language) error {
	program := tea.NewProgram(NewModelWithLanguage(payload, language), tea.WithOutput(out), tea.WithAltScreen())
	_, err := program.Run()
	return err
}

func RunWithRefresh(out io.Writer, payload report.Payload, language Language, refresh func() (report.Payload, error)) error {
	program := tea.NewProgram(NewModelWithRefresh(payload, language, refresh), tea.WithOutput(out), tea.WithAltScreen())
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
	return m.scheduleRefresh()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case refreshResultMsg:
		if msg.err == nil {
			m.payload = msg.payload
			if m.sortBy == "" {
				m.sortBy = normalizePayloadSort(msg.payload.SortBy)
			}
		}
		return m, m.scheduleRefresh()
	case clearCopyStatusMsg:
		m.copyStatus = ""
		return m, nil
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.clampScrollOffset()
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
				m.ensureThreadVisible()
				m.ensureModelUsageVisible()
			case "ctrl+c":
				return m, tea.Quit
			default:
				if len(key) == 1 {
					m.search += key
					m.ensureThreadVisible()
					m.ensureModelUsageVisible()
				}
			}
			return m, nil
		}
		switch key {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "/":
			m.searching = true
		case "?":
			m.showHelp = !m.showHelp
		case "l":
			m.language = toggleLanguage(m.language)
		case "s":
			m.sortBy = toggleSortMetric(m.sortBy)
			m.ensureThreadVisible()
			m.ensureModelUsageVisible()
		case "esc":
			if m.showHelp {
				m.showHelp = false
				return m, nil
			}
			if m.copyStatus != "" {
				m.copyStatus = ""
				return m, nil
			}
			m.search = ""
			m.searching = false
		case "1":
			m.activeTool = allTools
			m.ensureThreadVisible()
			m.ensureModelUsageVisible()
		case "2":
			m.activeTool = string(usage.ToolClaude)
			m.ensureThreadVisible()
			m.ensureModelUsageVisible()
		case "3":
			m.activeTool = string(usage.ToolCodex)
			m.ensureThreadVisible()
			m.ensureModelUsageVisible()
		case "4":
			m.activeTool = string(usage.ToolGemini)
			m.ensureThreadVisible()
			m.ensureModelUsageVisible()
		case "tab":
			m.toggleFocusedPane()
		case "pgup", "ctrl+u":
			m.movePage(-1)
		case "pgdown", "ctrl+d":
			m.movePage(1)
		case "up", "k":
			if m.focusedPane == "models" && m.canMoveModels() {
				m.moveModelUsageCursor(-1)
			} else if m.canMoveThreads() {
				m.focusedPane = "threads"
				m.moveThreadCursor(-1)
			}
		case "down", "j":
			if m.focusedPane == "models" && m.canMoveModels() {
				m.moveModelUsageCursor(1)
			} else if m.canMoveThreads() {
				m.focusedPane = "threads"
				m.moveThreadCursor(1)
			}
		case "home":
			if m.focusedPane == "models" && m.canMoveModels() {
				m.modelCursor = 0
				m.ensureModelUsageVisible()
			} else if m.canMoveThreads() {
				m.focusedPane = "threads"
				m.threadCursor = 0
				m.ensureThreadVisible()
			}
		case "end":
			if m.focusedPane == "models" && m.canMoveModels() {
				m.modelCursor = len(m.filteredResults()) - 1
				m.ensureModelUsageVisible()
			} else if m.canMoveThreads() {
				m.focusedPane = "threads"
				m.threadCursor = len(m.filteredThreads()) - 1
				m.ensureThreadVisible()
			}
		case "c", "C":
			if m.canMoveThreads() {
				m.focusedPane = "threads"
				id := m.filteredThreads()[m.threadCursor].ID
				m.copyStatus = copyFor(m.language).copyStatusPrefix + ": " + id
				return m, tea.Batch(copyToClipboard(id), clearCopyStatusAfter())
			}
		}
	}
	return m, nil
}

func clearCopyStatusAfter() tea.Cmd {
	return tea.Tick(copyStatusDuration, func(time.Time) tea.Msg {
		return clearCopyStatusMsg{}
	})
}

func (m *model) movePage(direction int) {
	if m.height <= 0 {
		return
	}
	step := m.height - 2
	if step < 1 {
		step = 1
	}
	m.scrollOffset += direction * step
	m.clampScrollOffset()
}

func (m *model) clampScrollOffset() {
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}
	maxOffset := m.maxScrollOffset()
	if m.scrollOffset > maxOffset {
		m.scrollOffset = maxOffset
	}
}

func (m model) maxScrollOffset() int {
	if m.height <= 0 {
		return 0
	}
	lineCount := len(strings.Split(m.fullView(), "\n"))
	maxOffset := lineCount - m.height
	if maxOffset < 0 {
		return 0
	}
	return maxOffset
}

func (m model) canMoveThreads() bool {
	return len(m.filteredThreads()) > 0 && !m.searching
}

func (m model) canMoveModels() bool {
	return len(m.filteredResults()) > 0 && !m.searching
}

func (m *model) toggleFocusedPane() {
	if m.focusedPane == "models" {
		if m.canMoveThreads() {
			m.focusedPane = "threads"
		}
		return
	}
	if m.canMoveModels() {
		m.focusedPane = "models"
		return
	}
	if m.canMoveThreads() {
		m.focusedPane = "threads"
	}
}

func (m model) scheduleRefresh() tea.Cmd {
	if m.refresh == nil {
		return nil
	}
	return tea.Tick(refreshInterval, func(time.Time) tea.Msg {
		payload, err := m.refresh()
		return refreshResultMsg{payload: payload, err: err}
	})
}

func (m *model) moveModelUsageCursor(delta int) {
	results := m.filteredResults()
	if len(results) == 0 {
		m.modelCursor = 0
		m.modelOffset = 0
		return
	}
	m.modelCursor += delta
	if m.modelCursor < 0 {
		m.modelCursor = 0
	}
	if m.modelCursor >= len(results) {
		m.modelCursor = len(results) - 1
	}
	m.ensureModelUsageVisible()
}

func (m *model) ensureModelUsageVisible() {
	results := m.filteredResults()
	if len(results) == 0 {
		m.modelCursor = 0
		m.modelOffset = 0
		return
	}
	if m.modelCursor >= len(results) {
		m.modelCursor = len(results) - 1
	}
	if m.modelCursor < 0 {
		m.modelCursor = 0
	}
	height := m.modelUsageTableRows()
	if m.modelCursor < m.modelOffset {
		m.modelOffset = m.modelCursor
	}
	if m.modelCursor >= m.modelOffset+height {
		m.modelOffset = m.modelCursor - height + 1
	}
	if m.modelOffset < 0 {
		m.modelOffset = 0
	}
	maxOffset := len(results) - height
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.modelOffset > maxOffset {
		m.modelOffset = maxOffset
	}
}

func (m *model) moveThreadCursor(delta int) {
	threads := m.filteredThreads()
	if len(threads) == 0 {
		m.threadCursor = 0
		m.threadOffset = 0
		return
	}
	m.threadCursor += delta
	if m.threadCursor < 0 {
		m.threadCursor = 0
	}
	if m.threadCursor >= len(threads) {
		m.threadCursor = len(threads) - 1
	}
	m.ensureThreadVisible()
}

func (m *model) ensureThreadVisible() {
	threads := m.filteredThreads()
	if len(threads) == 0 {
		m.threadCursor = 0
		m.threadOffset = 0
		return
	}
	if m.threadCursor >= len(threads) {
		m.threadCursor = len(threads) - 1
	}
	if m.threadCursor < 0 {
		m.threadCursor = 0
	}
	height := m.threadViewportHeight()
	if m.threadCursor < m.threadOffset {
		m.threadOffset = m.threadCursor
	}
	if m.threadCursor >= m.threadOffset+height {
		m.threadOffset = m.threadCursor - height + 1
	}
	if m.threadOffset < 0 {
		m.threadOffset = 0
	}
	maxOffset := len(threads) - height
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.threadOffset > maxOffset {
		m.threadOffset = maxOffset
	}
}

func (m model) threadViewportHeight() int {
	if m.width < 100 {
		return 7
	}
	return 7
}
