package monitor

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mabhi256/jdiag/internal/jmx"
	"github.com/mabhi256/jdiag/utils"
)

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

	if m.processMode {
		return m.handleProcessModeUpdate(msg)
	}

	return m.handleMonitoringModeUpdate(msg)
}

func (m *Model) handleProcessModeUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m.handleWindowResize(msg)

	case TickMsg:
		// Refresh list in process mode
		m.refreshProcessList()
		return m, m.scheduleTick()

	case tea.KeyMsg:
		return m.handleProcessModeKeys(msg)
	}

	return m, nil
}

func (m *Model) handleMonitoringModeUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m.handleWindowResize(msg)

	case TickMsg:
		// Process JMX data in monitor mode
		return m.handleMetricsTick()

	case tea.KeyMsg:
		return m.handleMonitoringModeKeys(msg)
	}

	return m, nil
}

func (m *Model) handleWindowResize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	m.width = msg.Width
	m.height = msg.Height
	m.help.Width = msg.Width

	if m.processMode {
		m.processList.SetWidth(msg.Width)
		m.processList.SetHeight(msg.Height - 4) // Leave space for header and footer
	}

	return m, nil
}

func (m *Model) handleMetricsTick() (tea.Model, tea.Cmd) {
	currentGCFilter := m.tabState.GC.gcChartFilter

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
		m.tabState.GC.gcChartFilter = currentGCFilter
	}

	// Always schedule the next tick
	return m, m.scheduleTick()
}

func (m *Model) handleProcessModeKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Enter):
		if selectedItem := m.processList.SelectedItem(); selectedItem != nil {
			if procItem, ok := selectedItem.(processItem); ok {
				return m.selectProcess(procItem.process)
			}
		}

	case key.Matches(msg, keys.Escape):
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

func (m *Model) handleMonitoringModeKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
		return m, triggerImmediateTick()

	case key.Matches(msg, keys.Reconnect):
		// Reconnect logic
		if m.collector != nil {
			m.collector.Stop()
		}
		m.collector = jmx.NewJMXCollector(m.config)
		if err := m.collector.Start(); err != nil {
			m.setError(fmt.Sprintf("Reconnection failed: %v", err))
		}
		return m, triggerImmediateTick()

	case key.Matches(msg, keys.GCFilter):
		// Only cycle GC filter when on GC tab
		if m.activeTab == TabGC {
			m.tabState.GC.gcChartFilter = m.tabState.GC.gcChartFilter.Next()
		}
		return m, nil
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
