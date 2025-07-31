package tui

import (
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
	issuesState     *IssuesState
	eventsState     *EventsState
	trendsState     *TrendsState
}

type TabType int

const (
	DashboardTab TabType = iota
	MetricsTab
	IssuesTab
	EventsTab
	TrendsTab
)

type IssuesState struct {
	selectedSubTab   IssuesSubTab
	expandedIssues   map[IssueKey]bool
	selectedIssueMap map[IssuesSubTab]int
}

type IssuesSubTab int

const (
	CriticalIssues IssuesSubTab = iota
	WarningIssues
	InfoIssues
)

var issueTypeName = map[IssuesSubTab]string{
	CriticalIssues: "critical",
	WarningIssues:  "warning",
	InfoIssues:     "info",
}

func (ss IssuesSubTab) String() string {
	return issueTypeName[ss]
}

type IssueKey struct {
	SubTab IssuesSubTab
	ID     int
}

type MetricsSubTab int

const (
	GeneralMetrics MetricsSubTab = iota
	TimingMetrics
	MemoryMetrics
	G1GCMetrics
	ConcurrentMetrics
)

type EventsState struct {
	selectedEvent int
	eventFilter   EventFilter
	sortBy        EventSortBy
	searchTerm    string
	showDetails   bool
}

type EventFilter int

const (
	AllEvent EventFilter = iota
	YoungEvent
	MixedEvent
	FullEvent
	ConcurrentEvent
)

type EventSortBy int

const (
	TimeSortEvent EventSortBy = iota
	DurationSortEvent
	TypeSortEvent
)

type TrendsState struct {
	trendSubTab TrendSubTab
	timeWindow  int // number of recent events to show
}

type TrendSubTab int

const (
	PauseTrend TrendSubTab = iota
	HeapTrend
	AllocationTrend
	FrequencyTrend
)

func (m *Model) GetSubTabIssues() []gc.PerformanceIssue {
	subTab := m.issuesState.selectedSubTab

	switch subTab {
	case CriticalIssues:
		return m.issues.Critical
	case WarningIssues:
		return m.issues.Warning
	case InfoIssues:
		return m.issues.Info
	default:
		return []gc.PerformanceIssue{}
	}
}

func (m *Model) GetSelectedIssue() int {
	currentSubTab := m.issuesState.selectedSubTab
	return m.issuesState.selectedIssueMap[currentSubTab]
}

func (m *Model) IsIssueExpanded(id int) bool {
	selectedSubTab := m.issuesState.selectedSubTab
	issueKey := IssueKey{
		SubTab: selectedSubTab,
		ID:     id,
	}
	return m.issuesState.expandedIssues[issueKey]
}
