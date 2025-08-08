package monitor

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mabhi256/jdiag/internal/jmx"
	"github.com/mabhi256/jdiag/internal/tui"
	"github.com/mabhi256/jdiag/utils"
)

// The main TUI model
type Model struct {
	config           *jmx.Config
	collector        *jmx.JMXPoller
	metricsProcessor *MetricsProcessor
	help             help.Model

	// UI state
	width  int
	height int

	// Tab management
	activeTab TabType
	tabState  *TabState
	// Scrolling support
	scrollPositions map[TabType]int // Per-tab scroll positions

	// Process selection state
	processMode     bool
	processList     list.Model
	selectedProcess *JavaProcess

	// Error state
	lastError    error
	errorMessage string
	showError    bool

	// Status tracking
	connected   bool
	lastUpdate  time.Time
	updateCount int64
	startTime   time.Time
}

func initialModel(config *jmx.Config) *Model {
	// Create process list
	items := []list.Item{}
	processList := list.New(items, list.NewDefaultDelegate(), 0, 0)
	processList.Title = "Java Processes"
	processList.SetShowStatusBar(false)
	processList.SetFilteringEnabled(true)

	m := &Model{
		config:           config,
		collector:        jmx.NewJMXCollector(config),
		metricsProcessor: NewMetricsProcessor(),
		help:             help.New(),
		activeTab:        TabMemory,
		scrollPositions:  make(map[TabType]int),
		tabState:         NewTabState(),
		processList:      processList,
		processMode:      config.PID == 0 && config.Host == "", // Start in process mode if no target specified
		connected:        false,
		startTime:        time.Now(),
	}

	if m.processMode {
		m.refreshProcessList()
	}

	return m
}

type TickMsg time.Time

func (m *Model) scheduleTick() tea.Cmd {
	interval := m.config.GetInterval() // Default metrics interval
	if m.processMode {
		interval = 5 * time.Second // Process refresh interval
	}

	return tea.Tick(interval, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

func triggerImmediateTick() tea.Cmd {
	return func() tea.Msg { return TickMsg(time.Now()) }
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle global keys first (before mode-specific logic)
	if msg, ok := msg.(tea.KeyMsg); ok {
		if key.Matches(msg, keys.Quit) {
			if m.collector != nil {
				m.collector.Stop()
			}
			return m, tea.Quit
		}
	}

	// Handles ALL process mode logic
	if m.processMode {
		return m.handleProcessModeUpdate(msg)
	}

	// Monitoring mode specific keys
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.help.Width = msg.Width

		return m, nil

	case TickMsg:
		// Metrics collection and processing
		metrics := m.collector.GetMetrics()
		m.tabState = m.metricsProcessor.ProcessMetrics(metrics)

		// Track connection status
		if metrics.Connected && !m.connected {
			m.connected = true
			m.clearError()
		} else if !metrics.Connected && m.connected {
			m.connected = false
			if metrics.Error != nil {
				m.setError(fmt.Sprintf("Connection lost: %v", metrics.Error))
			} else {
				m.setError("Connection lost to JVM")
			}
		}

		if metrics.Connected {
			m.lastUpdate = time.Now()
			m.updateCount++
			m.tabState.System.ConnectionUptime = time.Since(m.startTime)
			m.tabState.System.UpdateCount = m.updateCount
		}

		// Always schedule the next tick
		return m, m.scheduleTick()

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Tab):
			m.activeTab = m.nextTab()
			return m, nil

		case key.Matches(msg, keys.Left):
			m.activeTab = m.prevTab()
			return m, nil

		case key.Matches(msg, keys.Right):
			m.activeTab = m.nextTab()
			return m, nil

		case key.Matches(msg, keys.Up):
			m.scrollUp(1)
			return m, nil

		case key.Matches(msg, keys.Down):
			m.scrollDown(1)
			return m, nil

		case key.Matches(msg, keys.PageUp):
			m.scrollUp(10)
			return m, nil

		case key.Matches(msg, keys.PageDown):
			m.scrollDown(10)
			return m, nil

		case key.Matches(msg, keys.SelectProcess):
			m.processMode = true
			m.refreshProcessList()
			// return m, m.doProcessTick() // Start process ticking

		case key.Matches(msg, keys.Reconnect):
			return m.reconnect()

		case key.Matches(msg, keys.Refresh):
			// Trigger immediate tick
			return m, triggerImmediateTick()
		}
	}

	return m, nil
}

func (m *Model) nextTab() TabType {
	// utils.CycleEnumPtr(&m.activeTab, 1, TabSystem)
	return utils.GetNextEnum(m.activeTab, TabSystem)
}

func (m *Model) prevTab() TabType {
	// utils.CycleEnumPtr(&m.activeTab, -1, TabSystem)
	return utils.GetPrevEnum(m.activeTab, TabSystem)
}

func (m *Model) reconnect() (tea.Model, tea.Cmd) {
	if m.collector != nil {
		m.collector.Stop()
	}

	m.collector = jmx.NewJMXCollector(m.config)
	if err := m.collector.Start(); err != nil {
		m.setError(fmt.Sprintf("Reconnection failed: %v", err))
	} else {
		m.clearError()
		m.connected = true
		// Reset start time on reconnect
		m.startTime = time.Now()
	}

	// Get immediate data after reconnect
	return m, triggerImmediateTick()
}

func (m *Model) setError(message string) {
	m.errorMessage = message
	m.showError = true
}

func (m *Model) clearError() {
	m.errorMessage = ""
	m.showError = false
}

func (m *Model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	if m.processMode {
		return m.renderProcessSelectionView()
	}

	// Add error overlay if needed
	if m.showError {
		errorBox := tui.ErrorStyle.Render(m.errorMessage)
		errorOverlay := lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, errorBox)
		return errorOverlay
	}

	header := m.renderHeader()
	tabBar := m.renderTabBar()
	helpView := m.help.View(keys)

	// Calculate available height for content area
	headerHeight := lipgloss.Height(header)
	tabBarHeight := lipgloss.Height(tabBar)
	helpHeight := lipgloss.Height(helpView)

	contentHeight := m.height - headerHeight - tabBarHeight - helpHeight
	if contentHeight < 1 {
		contentHeight = 1 // Minimum content height
	}

	// Generate full content
	fullContent := m.renderActiveTab()

	// Apply scrolling to content
	scrolledContent := m.applyScrolling(fullContent, contentHeight)

	// Apply height constraint to scrolled content
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
		return RenderMemoryTab(m.tabState, m.width)
	case TabGC:
		return RenderGCTab(m.tabState)
	case TabThreads:
		return RenderThreadsTab(m.tabState, m.width)
	case TabSystem:
		return RenderSystemTab(m.tabState, m.config, m.width)
	default:
		return tui.CriticalStyle.Render("Unknown tab")
	}
}

func (m *Model) renderHeader() string {
	// Build title
	var title string
	if m.selectedProcess != nil {
		title = fmt.Sprintf("🔍 JDiag Watch - %s (PID: %d)",
			m.selectedProcess.MainClass, m.selectedProcess.PID)
	} else {
		title = fmt.Sprintf("🔍 JDiag Watch - %s", m.config.String())
	}

	// Build status
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

	// Single header line
	headerLine := title + " • " + status
	separatorLine := strings.Repeat("─", m.width)

	return lipgloss.JoinVertical(lipgloss.Left,
		tui.HeaderStyle.Width(m.width).Render(headerLine),
		tui.MutedStyle.Render(separatorLine),
	)
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

func (m *Model) Init() tea.Cmd {
	if !m.processMode {
		// Start the collector for direct monitoring
		if err := m.collector.Start(); err != nil {
			m.lastError = err
		}
	}

	return triggerImmediateTick()
}
