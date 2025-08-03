package tui

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mabhi256/jdiag/internal/gc"
	"github.com/mabhi256/jdiag/utils"
)

func (m *Model) RenderEvents() string {
	if len(m.events) == 0 {
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

func (m *Model) renderEventsHeader(events []*gc.GCEvent) string {
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
		ConcurrentAbort: "Concurrent",
	}

	sortNames := map[EventSortBy]string{
		TimeSortEvent:     "Time",
		DurationSortEvent: "Duration",
		TypeSortEvent:     "Type",
	}

	filterText := fmt.Sprintf("Filter: %s", filterNames[m.eventsState.eventFilter])
	sortText := fmt.Sprintf("Sort: %s", sortNames[m.eventsState.sortBy])
	countText := fmt.Sprintf("%d/%d events", len(events), len(m.events))

	statusLine := fmt.Sprintf("%s | %s | %s",
		filterStyle.Render(filterText),
		MutedStyle.Render(sortText),
		MutedStyle.Render(countText))

	return lipgloss.JoinVertical(lipgloss.Left,
		statusLine,
		"",
	)
}

func (m *Model) renderEventsTable(events []*gc.GCEvent, maxHeight int) string {
	if len(events) == 0 {
		return "No events to display"
	}

	// Table header
	headerLine := fmt.Sprintf(" %-6s │ %-8s │ %-24s │ %-9s │ %-20s",
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

func (m *Model) renderEventRow(event *gc.GCEvent, isSelected bool) string {
	// Format time
	timeStr := event.Timestamp.Format("15:04:05")

	// Format type
	typeStr := event.Type
	if event.Subtype != "" && event.Subtype != "Normal" {
		typeStr = fmt.Sprintf("%s %s", event.Type, event.Subtype)
	}

	var durationStr string
	const heapFieldWidth = 4     // 3 digits + 1 suffix (K/M/G/T)
	const durationFieldWidth = 9 // 123.45 ms
	heapStr := ""

	// Format duration & heap
	if event.ConcurrentDuration != 0 {
		durationStr = utils.FormatDuration(event.ConcurrentDuration)
	} else {
		durationStr = utils.FormatDuration(event.Duration)

		// Format heap changes
		heapStr = fmt.Sprintf("%*s → %-*s (%s)",
			heapFieldWidth, event.HeapBefore.String(),
			heapFieldWidth, event.HeapAfter.String(),
			event.HeapTotal.String())
	}

	// Create the row
	row := fmt.Sprintf("%-6d │ %-8s │ %-24s │ %*s │ %-20s",
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

func (m *Model) analyzeEventIssues(event *gc.GCEvent) eventIssues {
	var issues eventIssues

	// ===== CRITICAL ISSUES =====

	// Pause time issues
	if event.HasHighPauseTime || event.Duration > gc.PauseCritical {
		issues.critical = append(issues.critical, "PAUSE")
	}

	// Full GC - always critical
	if strings.Contains(strings.ToLower(event.Type), "full") {
		issues.critical = append(issues.critical, "FULL")
	}

	// Evacuation failure - critical
	if event.HasEvacuationFailure || event.ToSpaceExhausted {
		issues.critical = append(issues.critical, "EVAC")
	}

	// Memory pressure - critical heap utilization
	if event.HasMemoryPressure || (event.HeapTotal > 0 && float64(event.HeapAfter)/float64(event.HeapTotal) > gc.HeapUtilCritical) {
		issues.critical = append(issues.critical, "HEAP")
	}

	// Concurrent mark aborted
	if event.ConcurrentMarkAborted {
		issues.critical = append(issues.critical, "MARK")
	}

	// Pause target significantly exceeded
	if event.PauseTargetExceeded && event.Duration > gc.PauseCritical {
		issues.critical = append(issues.critical, "TARGET")
	}

	// ===== WARNING ISSUES =====

	// Warning level pause times
	if event.Duration > gc.PausePoor && event.Duration <= gc.PauseCritical {
		issues.warning = append(issues.warning, "pause")
	}

	// Warning level heap utilization
	if event.HeapTotal > 0 {
		heapUtil := float64(event.HeapAfter) / float64(event.HeapTotal)
		if heapUtil > gc.HeapUtilWarning && heapUtil <= gc.HeapUtilCritical {
			issues.warning = append(issues.warning, "heap")
		}
	}

	// Survivor overflow
	if event.HasSurvivorOverflow {
		issues.warning = append(issues.warning, "survivor")
	}

	// Premature promotion
	if event.IsPrematurePromotion {
		issues.warning = append(issues.warning, "promotion")
	}

	// Humongous object growth
	if event.HasHumongousGrowth {
		issues.warning = append(issues.warning, "humongous")
	}

	// Allocation burst
	if event.HasAllocationBurst {
		issues.warning = append(issues.warning, "alloc")
	}

	// ===== PHASE TIMING ISSUES =====

	// Individual phase issues (can be warning or critical based on severity)
	if event.HasSlowObjectCopy || event.ObjectCopyTime > gc.ObjectCopyTarget {
		if event.ObjectCopyTime > gc.ObjectCopyTarget*2 {
			issues.critical = append(issues.critical, "COPY")
		} else {
			issues.warning = append(issues.warning, "copy")
		}
	}

	if event.HasSlowRootScanning || event.ExtRootScanTime > gc.RootScanTarget {
		if event.ExtRootScanTime > gc.RootScanTarget*2 {
			issues.critical = append(issues.critical, "ROOTS")
		} else {
			issues.warning = append(issues.warning, "roots")
		}
	}

	if event.HasSlowTermination || event.TerminationTime > gc.TerminationTarget {
		if event.TerminationTime > gc.TerminationTarget*2 {
			issues.critical = append(issues.critical, "TERM")
		} else {
			issues.warning = append(issues.warning, "term")
		}
	}

	if event.HasSlowRefProcessing || event.ReferenceProcessingTime > gc.RefProcessingTarget {
		if event.ReferenceProcessingTime > gc.RefProcessingTarget*2 {
			issues.critical = append(issues.critical, "REFS")
		} else {
			issues.warning = append(issues.warning, "refs")
		}
	}

	// General long phases flag
	if event.HasLongPhases && len(issues.critical) == 0 && len(issues.warning) == 0 {
		issues.warning = append(issues.warning, "phases")
	}

	// Worker utilization
	if event.WorkersUsed > 0 && event.WorkersAvailable > 0 {
		utilization := float64(event.WorkersUsed) / float64(event.WorkersAvailable)
		if utilization < 0.3 {
			issues.critical = append(issues.critical, "WORKERS")
		} else if utilization < 0.5 {
			issues.warning = append(issues.warning, "workers")
		}
	}

	return issues
}

func (m *Model) renderEventDetails(event *gc.GCEvent) string {
	title := fmt.Sprintf("GC(%d) %s (%s)", event.ID, event.Type, event.Cause)

	// Basic timing info
	userMs := float64(event.UserTime.Nanoseconds()) / 1000000
	sysMs := float64(event.SystemTime.Nanoseconds()) / 1000000
	realMs := float64(event.RealTime.Nanoseconds()) / 1000000

	timingLine := ""
	if event.ConcurrentDuration == 0 {
		timingLine = fmt.Sprintf("Duration: %s  User: %.2fms  Sys: %.2fms  Real: %.2fms",
			utils.FormatDuration(event.Duration), userMs, sysMs, realMs)
	}

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

	// Analyze issues for this event
	issues := m.analyzeEventIssues(event)
	issuesLine := ""

	if len(issues.critical) > 0 || len(issues.warning) > 0 {
		var issueParts []string

		// Add critical issues in red
		if len(issues.critical) > 0 {
			criticalText := fmt.Sprintf("Critical: %s", strings.Join(issues.critical, ", "))
			issueParts = append(issueParts, CriticalStyle.Render(criticalText))
		}

		// Add warning issues in orange
		if len(issues.warning) > 0 {
			warningText := fmt.Sprintf("Warning: %s", strings.Join(issues.warning, ", "))
			issueParts = append(issueParts, WarningStyle.Render(warningText))
		}

		issuesLine = fmt.Sprintf("Issues: %s", strings.Join(issueParts, " | "))
	}

	// Build content lines - filter out empty lines
	lines := []string{
		TitleStyle.Render(title),
		timingLine,
	}

	if regionLine != "" {
		lines = append(lines, regionLine)
	}

	if workerLine != "" {
		lines = append(lines, workerLine)
	}

	if issuesLine != "" {
		lines = append(lines, issuesLine)
	}

	content := strings.Join(lines, "\n")

	// Fixed size border
	detailsStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(BorderColor).
		Padding(0, 1).
		Width(m.width - 2).
		Height(5)

	return detailsStyle.Render(content)
}

func (m *Model) getSortedEvents(events []*gc.GCEvent) []*gc.GCEvent {
	// Make a copy to avoid modifying the original
	sorted := make([]*gc.GCEvent, len(events))
	copy(sorted, events)

	switch m.eventsState.sortBy {
	case TimeSortEvent:
		// Default order (chronological) - reverse for newest first
		slices.Reverse(sorted)
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
