package watch

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mabhi256/jdiag/internal/jmx"
	"github.com/mabhi256/jdiag/internal/tui"
	"github.com/mabhi256/jdiag/utils"
)

func StartTUI(config *jmx.Config) error {
	model := initialModel(config)

	program := tea.NewProgram(
		model,
		tea.WithAltScreen(),       // Use alternate screen buffer
		tea.WithMouseCellMotion(), // Enable mouse support
	)

	// Run the program
	if _, err := program.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	return nil
}

func (m *Model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	// Add error overlay if needed
	if m.showError {
		errorBox := tui.ErrorStyle.Render(m.errorMessage)
		errorOverlay := lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, errorBox)
		return errorOverlay
	}

	if m.processMode {
		return m.renderProcessSelectionView()
	}

	return m.renderMonitorView()
}

func (m *Model) renderMonitorView() string {
	header := m.renderHeader()
	tabBar := m.renderTabBar()
	helpView := m.help.View(keys)

	// Calculate available height for content area
	headerHeight := lipgloss.Height(header)
	tabBarHeight := lipgloss.Height(tabBar)
	helpHeight := lipgloss.Height(helpView)

	contentHeight := m.height - headerHeight - tabBarHeight - helpHeight
	contentHeight = max(contentHeight, 1)

	fullContent := m.renderActiveTab()
	scrolledContent := m.applyScrolling(fullContent, contentHeight)
	content := lipgloss.NewStyle().Height(contentHeight).Render(scrolledContent)

	return lipgloss.JoinVertical(lipgloss.Left, header, tabBar, content, helpView)
}

func (m *Model) renderActiveTab() string {
	if !m.connected {
		if m.errorMessage != "" {
			return tui.CriticalStyle.Render(fmt.Sprintf("Connection error: %s", m.errorMessage))
		}
		return tui.CriticalStyle.Render("No connection to JVM")
	}

	switch m.activeTab {
	case TabMemory:
		heapHistory := m.GetHistoricalHeapMemory(5 * time.Minute)
		return RenderMemoryTab(m.tabState, m.width, heapHistory)
	case TabGC:
		return RenderGCTab(m.tabState, m.metricsProcessor.gcTracker, m.width)
	case TabThreads:
		classHistory := m.GetHistoricalClassCount(5 * time.Minute)
		threadHistory := m.GetHistoricaThreadCount(5 * time.Minute)
		return RenderThreadsTab(m.tabState, m.width, classHistory, threadHistory)
	case TabSystem:
		systemHistory := m.GetHistoricalSystemUsage(5 * time.Minute)
		return RenderSystemTab(m.tabState, m.config, m.width, systemHistory)
	default:
		return tui.CriticalStyle.Render("Unknown tab")
	}
}

func (m *Model) renderHeader() string {
	title := m.getTitle()
	status := m.getStatus()

	headerLine := title + " • " + status
	separatorLine := strings.Repeat("─", m.width)

	return lipgloss.JoinVertical(lipgloss.Left,
		tui.HeaderStyle.Width(m.width).Render(headerLine),
		tui.MutedStyle.Render(separatorLine),
	)
}

func (m *Model) getTitle() string {
	var title string
	if m.selectedProcess != nil {
		title = fmt.Sprintf("🔍 JDiag Watch - %s (PID: %d)",
			m.selectedProcess.MainClass, m.selectedProcess.PID)
	} else {
		title = fmt.Sprintf("🔍 JDiag Watch - %s", m.config.String())
	}

	return title
}

func (m *Model) getStatus() string {
	var status string
	if m.connected {
		uptime := m.tabState.System.JVMUptime
		status = tui.GoodStyle.Render(fmt.Sprintf("🟢 Connected • Uptime: %s", utils.FormatDuration(uptime)))
		if m.errorMessage != "" {
			status = tui.WarningStyle.Render("⚠️ Connected (Warning)")
		}
	} else {
		status = tui.CriticalStyle.Render("🔴 Disconnected")
		if m.errorMessage != "" {
			status = tui.CriticalStyle.Render("🔴 Error")
		}
	}

	return status
}

func (m *Model) renderTabBar() string {
	var tabs []string

	for _, tab := range GetAllTabs() {
		if tab == m.activeTab {
			tabs = append(tabs, tui.TabActiveStyle.Render(tab.String()))
		} else {
			tabs = append(tabs, tui.TabInactiveStyle.Render(tab.String()))
		}
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, tabs...)
}

func (m *Model) Init() tea.Cmd {
	if !m.processMode {
		// Start the collector for direct monitoring
		if err := m.collector.Start(); err != nil {
			m.lastError = err
		}
	}

	return triggerImmediateTick()
}
