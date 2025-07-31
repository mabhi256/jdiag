package tui

import (
	"fmt"
	"strings"

	"github.com/mabhi256/jdiag/internal/gc"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const PageSize = 10 // Number of lines to scroll per page

func initialModel(gcLog *gc.GCLog, metrics *gc.GCMetrics, issues *gc.Analysis) *Model {
	return &Model{
		currentTab:      DashboardTab,
		gcLog:           gcLog,
		metrics:         metrics,
		issues:          issues,
		keys:            DefaultKeyMap(),
		scrollPositions: make(map[TabType]int),
		expandedIssues:  make(map[int]bool),
		issuesFilter:    getFirstNonEmptyFilter(issues),
		selectedIssue:   0,
		metricsSubTab:   GeneralMetrics,
	}
}

func (m *Model) Init() tea.Cmd {
	return nil
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "tab":
			// Cycle through tabs: Dashboard -> Metrics -> Issues -> Dashboard
			switch m.currentTab {
			case DashboardTab:
				m.currentTab = MetricsTab
			case MetricsTab:
				m.currentTab = IssuesTab
			case IssuesTab:
				m.currentTab = DashboardTab
			}

		case "1":
			m.currentTab = DashboardTab
		case "2":
			m.currentTab = MetricsTab
		case "3":
			m.currentTab = IssuesTab

		case "left", "h":
			return m.handleLeftNavigation()
		case "right", "l":
			return m.handleRightNavigation()

		default:
			// Forward to tab-specific handlers for up/down and other keys
			return m.handleTabSpecificKeys(msg)
		}
	}

	return m, nil
}

func (m *Model) handleLeftNavigation() (tea.Model, tea.Cmd) {
	switch m.currentTab {
	case MetricsTab:
		// Handle metrics sub-tabs
		if m.metricsSubTab > GeneralMetrics {
			m.metricsSubTab--
			m.scrollPositions[MetricsTab] = 0
		}
	case IssuesTab:
		// Cycle through non-empty filters backwards
		filters := getAvailableFilters(m.issues)
		if len(filters) > 1 {
			currentIndex := getFilterIndex(m.issuesFilter, filters)
			newIndex := (currentIndex + len(filters) - 1) % len(filters)
			m.issuesFilter = filters[newIndex]
			m.selectedIssue = 0
		}
	}
	return m, nil
}

func (m *Model) handleRightNavigation() (tea.Model, tea.Cmd) {
	switch m.currentTab {
	case MetricsTab:
		// Handle metrics sub-tabs
		if m.metricsSubTab < ConcurrentMetrics {
			m.metricsSubTab++
			m.scrollPositions[MetricsTab] = 0
		}
	case IssuesTab:
		// Cycle through non-empty filters forwards
		filters := getAvailableFilters(m.issues)
		if len(filters) > 1 {
			currentIndex := getFilterIndex(m.issuesFilter, filters)
			newIndex := (currentIndex + 1) % len(filters)
			m.issuesFilter = filters[newIndex]
			m.selectedIssue = 0
		}
	}
	return m, nil
}

func (m *Model) handleTabSpecificKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.currentTab {
	case DashboardTab:
		return m.handleDashboardKeys(msg)

	case MetricsTab:
		return m.handleMetricsKeys(msg)

	case IssuesTab:
		return m.handleIssuesKeys(msg)
	}

	return m, nil
}

func (m *Model) handleDashboardKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.scrollPositions[DashboardTab] > 0 {
			m.scrollPositions[DashboardTab]--
		}
	case "down", "j":
		m.scrollPositions[DashboardTab]++
	}
	return m, nil
}

func (m *Model) handleMetricsKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.scrollPositions[MetricsTab] > 0 {
			m.scrollPositions[MetricsTab]--
		}
	case "down", "j":
		// Will be bounded in rendering
		m.scrollPositions[MetricsTab]++
	}
	return m, nil
}

func (m *Model) handleIssuesKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	filteredIssues := m.getFilteredIssues()

	switch msg.String() {
	case "up", "k":
		if m.selectedIssue > 0 {
			m.selectedIssue--
		}
	case "down", "j":
		if m.selectedIssue < len(filteredIssues)-1 {
			m.selectedIssue++
		}
	case "enter", " ":
		m.expandedIssues[m.selectedIssue] = !m.expandedIssues[m.selectedIssue]
	}
	return m, nil
}

func (m *Model) getFilteredIssues() []gc.PerformanceIssue {
	switch m.issuesFilter {
	case "critical":
		return m.issues.Critical
	case "warning":
		return m.issues.Warning
	case "info":
		return m.issues.Info
	default:
		return []gc.PerformanceIssue{}
	}
}

func (m *Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	var content string

	// Calculate available height for content (header + content + shortcuts)
	headerHeight := 2 // tab line + border
	shortcutsHeight := 1
	contentHeight := m.height - headerHeight - shortcutsHeight

	// Render current Tab
	switch m.currentTab {
	case DashboardTab:
		content = m.RenderDashboard()

	case MetricsTab:
		content = m.RenderMetrics()

	case IssuesTab:
		content = RenderIssues(m.issues, m.issuesFilter, m.selectedIssue, m.expandedIssues, m.width, contentHeight)
	}

	// Create a style that ensures content takes up exactly the available height
	contentStyle := lipgloss.NewStyle().
		Height(contentHeight).
		Width(m.width)
	content = contentStyle.Render(content)

	// Build the full Tab
	header := m.renderHeader()
	shortcuts := m.renderFooter()

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		content,
		shortcuts,
	)
}

func (m *Model) renderHeader() string {
	// Enhanced tab navigation with better visual indicators
	tabs := []string{}

	tabIcons := []string{"📊", "📈", "⚠️"}
	tabNames := []string{"Dashboard", "Metrics", "Issues"}

	for i, name := range tabNames {
		style := TabInactiveStyle
		indicator := " "

		if TabType(i) == m.currentTab {
			style = TabActiveStyle
			indicator = "●" // Active indicator
		}

		tabText := fmt.Sprintf("%s %s %s [%d]", indicator, tabIcons[i], name, i+1)
		tabs = append(tabs, style.Render(tabText))
	}

	tabLine := strings.Join(tabs, "  ")

	border := strings.Repeat("─", m.width)

	headerContent := []string{
		tabLine,
	}

	headerContent = append(headerContent, border)

	return lipgloss.JoinVertical(lipgloss.Left, headerContent...)
}

func GetShortcuts(currentTab TabType) string {
	base := "q:quit • tab:cycle • 1-3:tabs"

	var tabSpecific string
	switch currentTab {
	case DashboardTab:
		tabSpecific = "↑↓:scroll"

	case MetricsTab:
		tabSpecific = "↑↓:scroll • ←/→:metrics"

	case IssuesTab:
		tabSpecific = "↑↓:nav • ←/→:filter • space/enter:expand"
	}

	if tabSpecific != "" {
		return base + " • " + tabSpecific
	}
	return base
}

func (m *Model) renderFooter() string {
	shortcuts := GetShortcuts(m.currentTab)

	return HelpBarStyle.Width(m.width).Render(shortcuts)
}

func StartTUI(gcLog *gc.GCLog, metrics *gc.GCMetrics, issues *gc.Analysis) error {
	model := initialModel(gcLog, metrics, issues)

	program := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	_, err := program.Run()
	return err
}

func getAvailableFilters(issues *gc.Analysis) []string {
	var filters []string
	if len(issues.Critical) > 0 {
		filters = append(filters, "critical")
	}
	if len(issues.Warning) > 0 {
		filters = append(filters, "warning")
	}
	if len(issues.Info) > 0 {
		filters = append(filters, "info")
	}
	return filters
}

func getFirstNonEmptyFilter(issues *gc.Analysis) string {
	if len(issues.Critical) > 0 {
		return "critical"
	}
	if len(issues.Warning) > 0 {
		return "warning"
	}
	if len(issues.Info) > 0 {
		return "info"
	}
	return "critical" // fallback
}

func getFilterIndex(filter string, filters []string) int {
	for i, f := range filters {
		if f == filter {
			return i
		}
	}
	return 0
}
