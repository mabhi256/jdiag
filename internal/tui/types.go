package tui

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/mabhi256/jdiag/internal/gc"
)

type Model struct {
	// Data
	gcLog   *gc.GCLog
	metrics *gc.GCMetrics
	issues  *gc.Analysis

	// UI State
	currentTab TabType
	width      int
	height     int

	scrollPositions map[TabType]int
	metricsSubTab   MetricsSubTab
	issuesFilter    string
	expandedIssues  map[int]bool
	selectedIssue   int

	// Key bindings
	keys KeyMap
}

type TabType int

const (
	DashboardTab TabType = iota
	MetricsTab
	IssuesTab
	EventsTab
	TrendsTab
)

type MetricsSubTab int

const (
	GeneralMetrics MetricsSubTab = iota
	TimingMetrics
	MemoryMetrics
	G1GCMetrics
	ConcurrentMetrics
)

type KeyMap struct {
	Tab1  key.Binding
	Tab2  key.Binding
	Tab3  key.Binding
	Left  key.Binding
	Right key.Binding
	Up    key.Binding
	Down  key.Binding
	Enter key.Binding
	Quit  key.Binding
}

func k(keys []string, help, desc string) key.Binding {
	return key.NewBinding(
		key.WithKeys(keys...),
		key.WithHelp(help, desc),
	)
}

func DefaultKeyMap() KeyMap {
	return KeyMap{
		Tab1:  k([]string{"1"}, "1", "dashboard"),
		Tab2:  k([]string{"2"}, "2", "metrics"),
		Tab3:  k([]string{"3"}, "3", "issues"),
		Left:  k([]string{"left", "h"}, "←/h", "prev tab"),
		Right: k([]string{"right", "l"}, "→/l", "next tab"),
		Up:    k([]string{"up", "k"}, "↑/k", "up"),
		Down:  k([]string{"down", "j"}, "↓/j", "down"),
		Enter: k([]string{"enter"}, "enter", "expand"),
		Quit:  k([]string{"q", "ctrl+c"}, "q", "quit"),
	}
}
