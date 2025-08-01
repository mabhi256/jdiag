package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/mabhi256/jdiag/internal/gc"
)

func (m *Model) RenderEvents() string {
	if m.gcLog == nil || len(m.gcLog.Events) == 0 {
		return renderNoEvents()
	}

	filteredEvents := m.getFilteredEvents()
	sortedEvents := m.getSortedEvents(filteredEvents)

	if len(sortedEvents) == 0 {
		return renderNoFilteredEvents(m.eventsState.eventFilter)
	}

	// Calculate layout
	headerHeight := 3 // filter line + table header + separator
	detailsHeight := 0
	if m.eventsState.showDetails {
		detailsHeight = 8 // Details panel height
	}

	availableTableHeight := m.height - headerHeight - detailsHeight - 3 // margins

	header := m.renderEventsHeader(sortedEvents)
	table := m.renderEventsTable(sortedEvents, availableTableHeight)

	content := lipgloss.JoinVertical(lipgloss.Left, header, table)

	if m.eventsState.showDetails && m.eventsState.selectedEvent < len(sortedEvents) {
		details := m.renderEventDetails(sortedEvents[m.eventsState.selectedEvent])
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

	// Table header
	headerLine := fmt.Sprintf("%-6s │ %-8s │ %-14s │ %-9s │ %-20s",
		"ID", "Time", "Type", "Duration", "Heap Before→After")

	separator := strings.Repeat("─", m.width)

	return lipgloss.JoinVertical(lipgloss.Left,
		statusLine,
		"",
		TitleStyle.Render(headerLine),
		MutedStyle.Render(separator))
}

func (m *Model) renderEventsTable(events []gc.GCEvent, maxHeight int) string {
	if len(events) == 0 {
		return "No events to display"
	}

	var lines []string

	for i, event := range events {
		isSelected := i == m.eventsState.selectedEvent
		line := m.renderEventRow(event, isSelected)
		lines = append(lines, line)
	}

	// Apply scrolling to keep selected event visible
	startIdx := 0
	endIdx := len(lines)

	if len(lines) > maxHeight {
		// Center the selected event in the view
		selectedIdx := m.eventsState.selectedEvent
		halfHeight := maxHeight / 2

		startIdx = selectedIdx - halfHeight
		if startIdx < 0 {
			startIdx = 0
		}

		endIdx = startIdx + maxHeight
		if endIdx > len(lines) {
			endIdx = len(lines)
			startIdx = endIdx - maxHeight
			if startIdx < 0 {
				startIdx = 0
			}
		}

		lines = lines[startIdx:endIdx]
	}

	return strings.Join(lines, "\n")
}

func (m *Model) renderEventRow(event gc.GCEvent, isSelected bool) string {
	// Format time
	timeStr := event.Timestamp.Format("15:04:05")

	// Format type
	typeStr := event.Type
	if event.Subtype != "" && event.Subtype != "Normal" {
		typeStr = fmt.Sprintf("%s %s", event.Type, event.Subtype)
	}
	if len(typeStr) > 14 {
		typeStr = TruncateString(typeStr, 14)
	}

	// Format duration
	durationStr := FormatDuration(event.Duration)

	// Format heap changes
	heapStr := fmt.Sprintf("%s → %s (%s)",
		event.HeapBefore.String(),
		event.HeapAfter.String(),
		event.HeapTotal.String())

	// Create the row
	row := fmt.Sprintf("%-6d │ %-8s │ %-14s │ %-9s │ %-20s",
		event.ID,
		timeStr,
		typeStr,
		durationStr,
		heapStr)

	// Apply selection highlighting
	if isSelected {
		return lipgloss.NewStyle().
			Background(InfoColor).
			Foreground(lipgloss.Color("#FFFFFF")).
			Render("▶ " + row)
	}

	// Apply styling based on event type and performance
	style := TextStyle
	if strings.Contains(strings.ToLower(event.Type), "full") {
		style = CriticalStyle
	} else if event.Duration > 200*time.Millisecond {
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

	// Detailed phase breakdown (if available)
	var phaseLines []string
	if event.ObjectCopyTime > 0 {
		phaseLines = append(phaseLines, fmt.Sprintf("Object Copy: %s", FormatDuration(event.ObjectCopyTime)))
	}
	if event.ExtRootScanTime > 0 {
		phaseLines = append(phaseLines, fmt.Sprintf("Root Scan: %s", FormatDuration(event.ExtRootScanTime)))
	}
	if event.UpdateRSTime > 0 {
		phaseLines = append(phaseLines, fmt.Sprintf("Update RS: %s", FormatDuration(event.UpdateRSTime)))
	}
	if event.ScanRSTime > 0 {
		phaseLines = append(phaseLines, fmt.Sprintf("Scan RS: %s", FormatDuration(event.ScanRSTime)))
	}
	if event.TerminationTime > 0 {
		phaseLines = append(phaseLines, fmt.Sprintf("Termination: %s", FormatDuration(event.TerminationTime)))
	}

	phaseLine := ""
	if len(phaseLines) > 0 {
		phaseLine = "Phases: " + strings.Join(phaseLines, "  ")
	}

	// Region information
	regionLine := ""
	if event.EdenRegions > 0 || event.SurvivorRegions > 0 || event.OldRegions > 0 {
		regionLine = fmt.Sprintf("Regions: Eden %d→0, Survivor %d→%d, Old %d",
			event.EdenRegions, event.SurvivorRegions, event.SurvivorRegionsAfter, event.OldRegions)
	}

	// Build the details panel
	lines := []string{
		TitleStyle.Render(title),
		"• " + timingLine,
	}

	if phaseLine != "" {
		lines = append(lines, "• "+phaseLine)
	}

	if regionLine != "" {
		lines = append(lines, "• "+regionLine)
	}

	// Special flags
	if event.ToSpaceExhausted {
		lines = append(lines, CriticalStyle.Render("• ⚠️ To-Space Exhausted"))
	}

	// Worker utilization
	if event.WorkersUsed > 0 && event.WorkersAvailable > 0 {
		utilization := float64(event.WorkersUsed) / float64(event.WorkersAvailable) * 100
		workerLine := fmt.Sprintf("Workers: %d/%d (%.0f%% utilization)",
			event.WorkersUsed, event.WorkersAvailable, utilization)
		lines = append(lines, "• "+workerLine)
	}

	content := strings.Join(lines, "\n")

	// Add border around details
	detailsStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(BorderColor).
		Padding(0, 1).
		Width(m.width - 2)

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

func renderNoFilteredEvents(filter EventFilter) string {
	filterNames := map[EventFilter]string{
		AllEvent:        "all",
		YoungEvent:      "young",
		MixedEvent:      "mixed",
		FullEvent:       "full",
		ConcurrentEvent: "concurrent",
	}

	filterName := filterNames[filter]
	return fmt.Sprintf("No %s GC events found.\n\nPress '←' or '→' to change filter.", filterName)
}
