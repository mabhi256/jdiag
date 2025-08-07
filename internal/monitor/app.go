package monitor

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mabhi256/jdiag/internal/tui"
	"github.com/mabhi256/jdiag/utils"
)

// processItem represents a Java process in the selection list
type processItem struct {
	process *JavaProcess
}

func (i processItem) FilterValue() string {
	return fmt.Sprintf("%d %s %s", i.process.PID, i.process.MainClass, i.process.User)
}

func (i processItem) Title() string {
	title := fmt.Sprintf("PID %d: %s", i.process.PID, i.process.MainClass)
	if len(title) > 50 {
		title = title[:47] + "..."
	}
	return title
}

func (i processItem) Description() string {
	jmxStatus := "❌ JMX Disabled"
	if i.process.JMXEnabled {
		jmxStatus = "✅ JMX Enabled"
		if i.process.JMXPort > 0 {
			jmxStatus += fmt.Sprintf(" (port %d)", i.process.JMXPort)
		}
	}

	return fmt.Sprintf("User: %s | %s", i.process.User, jmxStatus)
}

// The main TUI model
type Model struct {
	config           *Config
	collector        *JMXCollector
	metricsProcessor *MetricsProcessor
	help             help.Model

	// UI state
	width  int
	height int

	// Tab management
	activeTab TabType
	tabState  *TabState

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

func initialModel(config *Config) *Model {
	// Create process list
	items := []list.Item{}
	processList := list.New(items, list.NewDefaultDelegate(), 0, 0)
	processList.Title = "Java Processes"
	processList.SetShowStatusBar(false)
	processList.SetFilteringEnabled(true)

	m := &Model{
		config:           config,
		collector:        NewJMXCollector(config),
		metricsProcessor: NewMetricsProcessor(),
		help:             help.New(),
		activeTab:        TabMemory,
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

func (m *Model) refreshProcessList() {
	processes, err := DiscoverJavaProcesses()
	if err != nil {
		m.setError(fmt.Sprintf("Failed to discover processes: %v", err))
		return
	}

	// Sort processes by PID
	sort.Slice(processes, func(i, j int) bool {
		return processes[i].PID < processes[j].PID
	})

	items := make([]list.Item, len(processes))
	for i, proc := range processes {
		items[i] = processItem{process: proc}
	}

	m.processList.SetItems(items)
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.help.Width = msg.Width

		if m.processMode {
			m.processList.SetWidth(msg.Width)
			m.processList.SetHeight(msg.Height - 4) // Leave space for header and footer
		}

		return m, nil

	case TickMsg:
		if m.processMode {
			// Refresh list
			m.refreshProcessList()
		} else {
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
		}

		// Always schedule the next tick
		return m, m.scheduleTick()

	case tea.KeyMsg:
		if key.Matches(msg, keys.Quit) {
			if m.collector != nil {
				m.collector.Stop()
			}
			return m, tea.Quit
		}

		// Process mode specific keys
		if m.processMode {
			switch {
			case m.processMode && key.Matches(msg, keys.Enter):
				if selectedItem := m.processList.SelectedItem(); selectedItem != nil {
					if procItem, ok := selectedItem.(processItem); ok {
						return m.selectProcess(procItem.process)
					}
				}

			case m.processMode && key.Matches(msg, keys.Escape):
				// ESC in process mode - go back to monitoring if we had selected a process previously
				if m.selectedProcess != nil {
					m.processMode = false
					// return m, nil // Mode changed, next tick will use metrics interval
				}
			}

			// Handle list navigation in process mode (for arrow keys, filtering, etc.)
			var cmd tea.Cmd
			m.processList, cmd = m.processList.Update(msg)
			return m, cmd
		}

		// Monitoring mode specific keys
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

func (m *Model) selectProcess(process *JavaProcess) (tea.Model, tea.Cmd) {
	if !process.JMXEnabled {
		// Try to enable JMX first
		if err := EnableJMXForProcess(process.PID); err != nil {
			m.setError(fmt.Sprintf("JMX not enabled for PID %d. Try running with sudo or enable JMX on the target application.", process.PID))
			return m, nil
		}

		// Refresh process info after enabling JMX
		if updatedProcess, err := GetProcessByPID(process.PID); err == nil {
			process = updatedProcess
		}
	}

	// Update configuration and switch to monitoring mode
	m.config.PID = process.PID
	m.config.Host = ""
	m.config.Port = 0
	m.selectedProcess = process
	m.processMode = false
	m.clearError()

	// Reset start time for new monitoring session
	m.startTime = time.Now()
	m.updateCount = 0

	// Update system state with process info
	m.tabState.System.ProcessName = process.MainClass
	m.tabState.System.ProcessUser = process.User

	// Reinitialize the collector with new config
	if m.collector != nil {
		m.collector.Stop()
	}
	m.collector = NewJMXCollector(m.config)

	// Reset metrics for new session
	m.metricsProcessor = NewMetricsProcessor()

	// Start monitoring
	if err := m.collector.Start(); err != nil {
		m.setError(fmt.Sprintf("Failed to connect to process %d: %v", process.PID, err))
		m.processMode = true
		return m, nil
	}

	return m, triggerImmediateTick()
}

func (m *Model) reconnect() (tea.Model, tea.Cmd) {
	if m.collector != nil {
		m.collector.Stop()
	}

	m.collector = NewJMXCollector(m.config)
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
	if m.width == 0 {
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
	content := m.renderActiveTab()
	helpView := m.help.View(keys)

	// Calculate available height for content
	usedHeight := lipgloss.Height(header) + lipgloss.Height(tabBar) + lipgloss.Height(helpView) + 2
	contentHeight := m.height - usedHeight

	if contentHeight > 0 {
		content = lipgloss.NewStyle().Height(contentHeight).Render(content)
	}

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

func (m *Model) renderProcessSelectionView() string {
	header := tui.HeaderStyle.Width(m.width).Render("🔍 Select Java Process to Monitor")

	// Process list
	listView := m.processList.View() // Triggers Title(), Description(), FilterValue()

	instructions := []string{
		"Enter: Connect to selected process",
		"e: Enable JMX for selected process",
		"q: Quit",
	}

	instructionStyle := tui.MutedStyle.Width(m.width)
	instructionsView := instructionStyle.Render(strings.Join(instructions, " • "))

	// Status bar
	statusText := fmt.Sprintf("Found %d Java processes", len(m.processList.Items()))
	if m.showError {
		statusText = m.errorMessage
	}
	statusView := tui.StatusBarStyle.Width(m.width).Render(statusText)

	// Calculate available height
	usedHeight := lipgloss.Height(header) + lipgloss.Height(instructionsView) + lipgloss.Height(statusView)
	listHeight := m.height - usedHeight - 2

	if listHeight > 0 {
		m.processList.SetHeight(listHeight)
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		listView,
		instructionsView,
		statusView,
	)
}

func (m *Model) renderHeader() string {
	var title string
	if m.selectedProcess != nil {
		title = fmt.Sprintf("🔍 JDiag Watch - %s (PID: %d)",
			m.selectedProcess.MainClass, m.selectedProcess.PID)
	} else {
		title = fmt.Sprintf("🔍 JDiag Watch - %s", m.config.String())
	}

	var status string
	var statusStyle lipgloss.Style

	if m.connected {
		uptime := time.Since(m.lastUpdate)
		status = fmt.Sprintf("🟢 Connected • Updates: %d • Uptime: %s",
			m.updateCount,
			utils.FormatDuration(uptime))
		statusStyle = tui.GoodStyle
		if m.errorMessage != "" {
			status = fmt.Sprintf("⚠️ Connected (Warning: %s)", m.errorMessage)
			statusStyle = tui.WarningStyle
		}
	} else {
		status = "🔴 Disconnected"
		statusStyle = tui.CriticalStyle
		if m.errorMessage != "" {
			status = fmt.Sprintf("🔴 Error: %s", m.errorMessage)
		}
	}

	timestamp := m.lastUpdate.Format("15:04:05")

	// Create header with title, status, and timestamp
	titleStyle := tui.TitleStyle.Width(m.width / 3)
	statusStyled := statusStyle.Width(m.width / 3).Align(lipgloss.Center)
	timestampStyle := tui.MutedStyle.Width(m.width / 3).Align(lipgloss.Right)

	headerRow := lipgloss.JoinHorizontal(lipgloss.Top,
		titleStyle.Render(title),
		statusStyled.Render(status),
		timestampStyle.Render(timestamp),
	)

	return tui.BoxStyle.Width(m.width).Render(headerRow)
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

func StartTUI(config *Config) error {
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
