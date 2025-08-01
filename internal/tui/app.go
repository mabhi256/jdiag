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
	selecttedIssuesTab := getFirstNonEmptyFilter(issues)
	selectedIssue := make(map[IssuesSubTab]int)
	selectedIssue[CriticalIssues] = 0
	selectedIssue[WarningIssues] = 0
	selectedIssue[InfoIssues] = 0

	return &Model{
		currentTab:      DashboardTab,
		gcLog:           gcLog,
		metrics:         metrics,
		issues:          issues,
		scrollPositions: make(map[TabType]int),
		metricsSubTab:   GeneralMetrics,
		issuesState: &IssuesState{
			selectedSubTab:   selecttedIssuesTab,
			expandedIssues:   make(map[IssueKey]bool),
			selectedIssueMap: selectedIssue,
		},
		eventsState: &EventsState{
			selectedEvent: 0,
			eventFilter:   AllEvent,
			sortBy:        TimeSortEvent,
			searchTerm:    "",
			showDetails:   true,
		},
		trendsState: &TrendsState{
			trendSubTab: PauseTrend,
			timeWindow:  100,
		},
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
				m.currentTab = EventsTab
			case EventsTab:
				m.currentTab = TrendsTab
			case TrendsTab:
				m.currentTab = DashboardTab
			}

		case "1":
			m.currentTab = DashboardTab
		case "2":
			m.currentTab = MetricsTab
		case "3":
			m.currentTab = IssuesTab
		case "4":
			m.currentTab = EventsTab
		case "5":
			m.currentTab = TrendsTab

		case "left", "h":
			return m.handleHorizontalNavigation(-1)
		case "right", "l":
			return m.handleHorizontalNavigation(1)

		default:
			// Forward to tab-specific handlers for up/down and other keys
			return m.handleTabSpecificKeys(msg)
		}
	}

	return m, nil
}

func (m *Model) handleHorizontalNavigation(direction int) (tea.Model, tea.Cmd) {
	switch m.currentTab {
	case MetricsTab:
		CycleEnumPtr(&m.metricsSubTab, direction, ConcurrentMetrics)
	case IssuesTab:
		CycleEnumPtr(&m.issuesState.selectedSubTab, direction, InfoIssues)
	case TrendsTab:
		CycleEnumPtr(&m.trendsState.trendSubTab, direction, FrequencyTrend)
	default:
		return m, nil
	}

	m.scrollPositions[m.currentTab] = 0

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
	case EventsTab:
		return m.handleEventsKeys(msg)
	case TrendsTab:
		return m.handleTrendsKeys(msg)
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
	state := m.issuesState
	selectedIssueMap := state.selectedIssueMap
	expandedIssues := state.expandedIssues
	currentSubTab := state.selectedSubTab

	subTabIssues := m.GetSubTabIssues()
	selectedIssue := selectedIssueMap[currentSubTab]

	switch msg.String() {
	case "up", "k":
		if selectedIssue > 0 {
			selectedIssueMap[currentSubTab] = selectedIssue - 1
		}
	case "down", "j":
		if selectedIssue < len(subTabIssues)-1 {
			selectedIssueMap[currentSubTab] = selectedIssue + 1
		}
	case "enter", " ":
		key := IssueKey{
			SubTab: currentSubTab,
			ID:     selectedIssue,
		}
		expandedIssues[key] = !expandedIssues[key]
	}
	return m, nil
}

func (m *Model) handleEventsKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	filteredEvents := m.getFilteredEvents()

	switch msg.String() {
	case "up", "k":
		if m.eventsState.selectedEvent > 0 {
			m.eventsState.selectedEvent--
		}
	case "down", "j":
		if m.eventsState.selectedEvent < len(filteredEvents)-1 {
			m.eventsState.selectedEvent++
		}
	case "f":
		CycleEnumPtr(&m.eventsState.eventFilter, 1, ConcurrentEvent)
	case "s":
		CycleEnumPtr(&m.eventsState.sortBy, 1, TypeSortEvent)
	}
	return m, nil
}

func (m *Model) handleTrendsKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.scrollPositions[TrendsTab] > 0 {
			m.scrollPositions[TrendsTab]--
		}
	case "down", "j":
		m.scrollPositions[TrendsTab]++
	case "+":
		// Increase time window
		if m.trendsState.timeWindow < 1000 {
			m.trendsState.timeWindow += 50
		}
	case "-":
		// Decrease time window
		if m.trendsState.timeWindow > 50 {
			m.trendsState.timeWindow -= 50
		}
	}
	return m, nil
}

func (m *Model) getFilteredEvents() []gc.GCEvent {
	if m.gcLog == nil {
		return []gc.GCEvent{}
	}

	events := m.gcLog.Events
	if m.eventsState.eventFilter == AllEvent {
		return events
	}

	var filtered []gc.GCEvent
	for _, event := range events {
		switch m.eventsState.eventFilter {
		case YoungEvent:
			if strings.Contains(strings.ToLower(event.Type), "young") {
				filtered = append(filtered, event)
			}
		case MixedEvent:
			if strings.Contains(strings.ToLower(event.Type), "mixed") {
				filtered = append(filtered, event)
			}
		case FullEvent:
			if strings.Contains(strings.ToLower(event.Type), "full") {
				filtered = append(filtered, event)
			}
		case ConcurrentEvent:
			if strings.Contains(strings.ToLower(event.Type), "concurrent") {
				filtered = append(filtered, event)
			}
		}
	}
	return filtered
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
		content = m.RenderIssues()
	case EventsTab:
		content = m.RenderEvents()
	case TrendsTab:
		content = m.RenderTrends()
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

	tabIcons := []string{"📋", "⏱️", "❗", "📍", "📈"}
	tabNames := []string{"Dashboard", "Metrics", "Issues", "Events", "Trends"}

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

	tabLine := strings.Join(tabs, "")

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
	case EventsTab:
		tabSpecific = "↑↓:nav • f:filter • s:sort"
	case TrendsTab:
		tabSpecific = "↑↓:scroll • ←/→:view • +/-:timespan"
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

func getFirstNonEmptyFilter(issues *gc.Analysis) IssuesSubTab {
	if len(issues.Critical) > 0 {
		return CriticalIssues
	}
	if len(issues.Warning) > 0 {
		return WarningIssues
	}
	if len(issues.Info) > 0 {
		return InfoIssues
	}
	return CriticalIssues // fallback
}
