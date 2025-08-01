package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mabhi256/jdiag/internal/gc"
)

func (m *Model) RenderEvents() string {
	if m.gcLog == nil || len(m.gcLog.Events) == 0 {
		return renderNoEvents()
	}

	filteredEvents := m.getFilteredEvents()
	sortedEvents := m.getSortedEvents(filteredEvents)

	// Calculate layout
	headerHeight := 5  // filter line + table header + separator
	detailsHeight := 7 // Fixed details panel height (4 lines + border)

	availableTableHeight := m.height - headerHeight - detailsHeight - 3 // margins

	header := m.renderEventsHeader(sortedEvents)
	// if len(sortedEvents) == 0 {
	// 	return renderNoFilteredEvents(m.eventsState.eventFilter)
	// }

	table := m.renderEventsTable(sortedEvents, availableTableHeight)

	content := lipgloss.JoinVertical(lipgloss.Left, header, table)

	// Always show details at bottom with selected event
	if len(sortedEvents) > 0 {
		selectedIdx := m.eventsState.selectedEvent
		if selectedIdx >= len(sortedEvents) {
			selectedIdx = 0
		}
		details := m.renderEventDetails(sortedEvents[selectedIdx])
		content = lipgloss.JoinVertical(lipgloss.Left, content, "", details)
	}

	return content
}

func (m *Model) renderEventsHeader(events []gc.GCEvent) string {
	// Filter and sort status
	filterStyle := TabInactiveStyle
	if m.eventsState.eventFilter != AllEvent {
		filterStyle = TabActiveStyle
	}

	// Convert enum to display string
	filterNames := map[EventFilter]string{
		AllEvent:        "All",
		YoungEvent:      "Young",
		MixedEvent:      "Mixed",
		FullEvent:       "Full",
		ConcurrentEvent: "Concurrent",
	}

	sortNames := map[EventSortBy]string{
		TimeSortEvent:     "Time",
		DurationSortEvent: "Duration",
		TypeSortEvent:     "Type",
	}

	filterText := fmt.Sprintf("Filter: %s", filterNames[m.eventsState.eventFilter])
	sortText := fmt.Sprintf("Sort: %s", sortNames[m.eventsState.sortBy])
	countText := fmt.Sprintf("%d/%d events", len(events), len(m.gcLog.Events))

	statusLine := fmt.Sprintf("%s | %s | %s",
		filterStyle.Render(filterText),
		MutedStyle.Render(sortText),
		MutedStyle.Render(countText))

	return lipgloss.JoinVertical(lipgloss.Left,
		statusLine,
		"",
	)
}

func (m *Model) renderEventsTable(events []gc.GCEvent, maxHeight int) string {
	if len(events) == 0 {
		return "No events to display"
	}

	// Table header
	headerLine := fmt.Sprintf(" %-6s │ %-8s │ %-22s │ %-9s │ %-20s",
		"ID", "Time", "Type", "Duration", "Heap Before→After")

	separator := strings.Repeat("─", m.width)

	var lines []string

	for i, event := range events {
		isSelected := i == m.eventsState.selectedEvent
		line := m.renderEventRow(event, isSelected)
		lines = append(lines, line)
	}

	// Apply scrolling to keep selected event visible
	startIdx := 0
	if len(lines) > maxHeight {
		// Center the selected event in the view
		selectedIdx := m.eventsState.selectedEvent
		halfHeight := maxHeight / 2

		startIdx = max(selectedIdx-halfHeight, 0)

		endIdx := startIdx + maxHeight
		if endIdx > len(lines) {
			endIdx = len(lines)
			startIdx = max(endIdx-maxHeight, 0)
		}

		lines = lines[startIdx:endIdx]
	}

	table := strings.Join(lines, "\n")

	return lipgloss.JoinVertical(lipgloss.Left,
		TitleStyle.Render(headerLine),
		MutedStyle.Render(separator),
		table,
	)
}

type eventIssues struct {
	critical []string
	warning  []string
}

func (m *Model) analyzeEventIssues(event gc.GCEvent) eventIssues {
	var issues eventIssues

	// Critical issues
	if event.Duration > gc.PauseCritical {
		issues.critical = append(issues.critical, "PAUSE")
	}
	if strings.Contains(strings.ToLower(event.Type), "full") {
		issues.critical = append(issues.critical, "FULL")
	}
	if event.ToSpaceExhausted {
		issues.critical = append(issues.critical, "EVAC")
	}

	// Calculate heap utilization
	if event.HeapTotal > 0 {
		heapUtil := float64(event.HeapAfter) / float64(event.HeapTotal)
		if heapUtil > gc.HeapUtilCritical {
			issues.critical = append(issues.critical, "HEAP")
		} else if heapUtil > gc.HeapUtilWarning {
			issues.warning = append(issues.warning, "heap")
		}
	}

	// Warning issues
	if event.Duration > gc.PausePoor && event.Duration <= gc.PauseCritical {
		issues.warning = append(issues.warning, "pause")
	}

	// Phase analysis
	if event.ObjectCopyTime > gc.ObjectCopyTarget {
		issues.warning = append(issues.warning, "copy")
	}
	if event.ExtRootScanTime > gc.RootScanTarget {
		issues.warning = append(issues.warning, "roots")
	}
	if event.TerminationTime > gc.TerminationTarget {
		issues.warning = append(issues.warning, "term")
	}

	// Worker utilization
	if event.WorkersUsed > 0 && event.WorkersAvailable > 0 {
		utilization := float64(event.WorkersUsed) / float64(event.WorkersAvailable)
		if utilization < 0.5 {
			issues.warning = append(issues.warning, "workers")
		}
	}

	return issues
}

func (m *Model) renderEventRow(event gc.GCEvent, isSelected bool) string {
	// Format time
	timeStr := event.Timestamp.Format("15:04:05")

	// Format type
	typeStr := event.Type
	if event.Subtype != "" && event.Subtype != "Normal" {
		typeStr = fmt.Sprintf("%s %s", event.Type, event.Subtype)
	}

	// Format duration
	durationStr := FormatDuration(event.Duration)

	const heapFieldWidth = 4     // 3 digits + 1 suffix (K/M/G/T)
	const durationFieldWidth = 9 // 123.45 ms

	// Format heap changes
	heapStr := fmt.Sprintf("%*s → %-*s (%s)",
		heapFieldWidth, event.HeapBefore.String(),
		heapFieldWidth, event.HeapAfter.String(),
		event.HeapTotal.String())

	// Create the row
	row := fmt.Sprintf("%-6d │ %-8s │ %-22s │ %*s │ %-20s",
		event.ID,
		timeStr,
		typeStr,
		durationFieldWidth, durationStr,
		heapStr)

	// Apply selection highlighting
	if isSelected {
		return lipgloss.NewStyle().
			Background(InfoColor).
			Foreground(lipgloss.Color("#FFFFFF")).
			Render("▶ " + row)
	}

	// Analyze issues for row-level styling
	issues := m.analyzeEventIssues(event)
	style := TextStyle
	if len(issues.critical) > 0 {
		style = CriticalStyle
	} else if len(issues.warning) > 0 {
		style = WarningStyle
	}

	return style.Render("  " + row)
}

func (m *Model) renderEventDetails(event gc.GCEvent) string {
	title := fmt.Sprintf("GC(%d) %s (%s)", event.ID, event.Type, event.Cause)

	// Basic timing info
	userMs := float64(event.UserTime.Nanoseconds()) / 1000000
	sysMs := float64(event.SystemTime.Nanoseconds()) / 1000000
	realMs := float64(event.RealTime.Nanoseconds()) / 1000000

	timingLine := fmt.Sprintf("Duration: %s  User: %.2fms  Sys: %.2fms  Real: %.2fms",
		FormatDuration(event.Duration), userMs, sysMs, realMs)

	// Region information
	regionLine := ""
	if event.EdenRegionsBefore > 0 || event.SurvivorRegionsBefore > 0 || event.OldRegionsBefore > 0 {
		regionLine = fmt.Sprintf("Regions: Eden %d→%d, Survivor %d→%d, Old %d→%d",
			event.EdenRegionsBefore, event.EdenRegionsAfter,
			event.SurvivorRegionsBefore, event.SurvivorRegionsAfter,
			event.OldRegionsBefore, event.OldRegionsAfter)
	}

	// Worker utilization
	workerLine := ""
	if event.WorkersUsed > 0 && event.WorkersAvailable > 0 {
		utilization := float64(event.WorkersUsed) / float64(event.WorkersAvailable) * 100
		workerLine = fmt.Sprintf("Workers: %d/%d (%.0f%% utilization)",
			event.WorkersUsed, event.WorkersAvailable, utilization)
	}

	// Build exactly 5 lines of content
	lines := []string{
		TitleStyle.Render(title),
		timingLine,
		regionLine,
		workerLine,
	}

	content := strings.Join(lines, "\n")

	// Fixed size border
	detailsStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(BorderColor).
		Padding(0, 1).
		Width(m.width - 2).
		Height(4) // Fixed height

	return detailsStyle.Render(content)
}

func (m *Model) getSortedEvents(events []gc.GCEvent) []gc.GCEvent {
	// Make a copy to avoid modifying the original
	sorted := make([]gc.GCEvent, len(events))
	copy(sorted, events)

	switch m.eventsState.sortBy {
	case TimeSortEvent:
		// Default order (chronological) - reverse for newest first
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].Timestamp.After(sorted[j].Timestamp)
		})
	case DurationSortEvent:
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].Duration > sorted[j].Duration
		})
	case TypeSortEvent:
		sort.Slice(sorted, func(i, j int) bool {
			if sorted[i].Type == sorted[j].Type {
				return sorted[i].Timestamp.After(sorted[j].Timestamp)
			}
			return sorted[i].Type < sorted[j].Type
		})
	}

	return sorted
}

func renderNoEvents() string {
	return MutedStyle.Render("No GC events found in the log.")
}
