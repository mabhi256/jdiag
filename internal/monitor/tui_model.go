package monitor

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mabhi256/jdiag/internal/jmx"
)

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

// Model state management methods
func (m *Model) setError(err string) {
	m.errorMessage = err
	m.showError = true
	m.lastError = fmt.Errorf("%s", err)
}

func (m *Model) clearError() {
	m.errorMessage = ""
	m.showError = false
	m.lastError = nil
}

// Message types
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
