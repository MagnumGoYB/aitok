package tui

import "github.com/charmbracelet/lipgloss"

var (
	blue   = lipgloss.Color("#27B0FF")
	green  = lipgloss.Color("#31D0AA")
	purple = lipgloss.Color("#A779FF")
	orange = lipgloss.Color("#FFB454")

	titleStyle        = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F5FBFF"))
	subtitleStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("248"))
	activeTabStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#EAF9FF")).Bold(true).Background(lipgloss.Color("#11324A")).Padding(0, 1)
	tabStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Padding(0, 1)
	labelStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("247"))
	valueStyle        = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FDFEFF"))
	sectionStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F5FBFF"))
	mutedStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	helpCompactStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	helpStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
	contextLabelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	contextValueStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	badgeStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("#CDEFFF")).Background(lipgloss.Color("#183548")).Padding(0, 1)
	statusStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#9EE7FF")).Background(lipgloss.Color("#143243")).Padding(0, 1)
	detailLabelStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	detailValueStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	selectedRowStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#062235")).Background(lipgloss.Color("#4CC2FF")).Bold(true)
)
