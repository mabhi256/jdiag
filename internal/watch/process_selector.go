package watch

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mabhi256/jdiag/internal/jmx"
	"github.com/mabhi256/jdiag/internal/tui"
)

// processItem represents a Java process in the selection list
type processItem struct {
	process *JavaProcess
}

func (i processItem) FilterValue() string {
	return fmt.Sprintf("%d %s", i.process.PID, i.process.MainClass)
}

func (i processItem) Title() string {
	title := fmt.Sprintf("PID %d: %s", i.process.PID, i.process.MainClass)
	if len(title) > 50 {
		title = title[:47] + "..."
	}
	return title
}

func (i processItem) Description() string {
	return ""
}

// refreshProcessList discovers and populates the process list
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

// Selects a process and switches to monitoring mode
func (m *Model) selectProcess(process *JavaProcess) (tea.Model, tea.Cmd) {
	// Try to enable JMX first
	if err := jmx.EnableJMXForProcess(process.PID); err != nil {
		m.setError(fmt.Sprintf("JMX not enabled for PID %d", process.PID))
		return m, nil
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

	// Reinitialize the collector with new config
	if m.collector != nil {
		m.collector.Stop()
	}
	m.collector = jmx.NewJMXCollector(m.config)

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

// Renders the process selection UI
func (m *Model) renderProcessSelectionView() string {
	header := tui.HeaderStyle.Width(m.width).Render("🔍 Select Java Process to Monitor")

	// Process list
	listView := m.processList.View() // Triggers Title(), Description(), FilterValue()

	// Status bar
	statusText := fmt.Sprintf("Found %d Java processes", len(m.processList.Items()))
	if m.showError {
		statusText = m.errorMessage
	}
	statusView := tui.StatusBarStyle.Width(m.width).Render(statusText)

	separatorLine := strings.Repeat("─", m.width)

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		tui.MutedStyle.Render(separatorLine),
		listView,
		statusView,
	)
}
