package monitor

import (
	"fmt"
	"time"

	"github.com/mabhi256/jdiag/internal/tui"

	"github.com/charmbracelet/lipgloss"
	"github.com/mabhi256/jdiag/utils"
)

func RenderGCTab(state *TabState, tracker *GCEventTracker, width int) string {
	var sections []string

	// Analysis window for calculations
	window := 5 * time.Minute

	// Summary section: Summary metrics in a clean grid
	summarySection := renderGCSummaryGrid(tracker, window)
	sections = append(sections, summarySection)

	// Top section: GC Events Chart
	chartSection := renderGCEventsChart(tracker, width, state.GC.gcChartFilter)
	if chartSection != "" {
		sections = append(sections, chartSection)
		sections = append(sections, "")
	}

	// Middle section: Generation stats and recent GC side by side
	middleSection := renderMiddleSection(tracker, window)
	sections = append(sections, middleSection)

	// Bottom section: Performance analysis in organized blocks
	performanceSection := renderPerformanceGrid(tracker, window)
	sections = append(sections, performanceSection)

	// Recent events in a clean list
	recentEvents := tracker.GetRecentEvents(5)
	if len(recentEvents) > 0 {
		eventsSection := renderRecentEventsClean(recentEvents)
		sections = append(sections, eventsSection)
	}

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// renderGCEventsChart creates a time series chart of GC events
func renderGCEventsChart(tracker *GCEventTracker, width int, filter GCChartFilter) string {
	tracker.mu.RLock()
	events := make([]GCEvent, len(tracker.gcEvents))
	copy(events, tracker.gcEvents)
	tracker.mu.RUnlock()

	if len(events) < 2 {
		return ""
	}

	graphWidth := max(width-30, 40)
	graphHeight := 12

	chart := utils.NewChart(graphWidth, graphHeight)

	// Separate events by generation for different colors
	youngEvents := make([]GCEvent, 0)
	oldEvents := make([]GCEvent, 0)

	for _, event := range events {
		switch event.Generation {
		case "young":
			youngEvents = append(youngEvents, event)
		case "old":
			oldEvents = append(oldEvents, event)
		}
	}

	// Add young generation events (main dataset)
	for _, event := range youngEvents {
		var value float64
		switch filter {
		case GCFilterBefore:
			value = utils.MemorySize(event.Before).MB()
		case GCFilterAfter:
			value = utils.MemorySize(event.After).MB()
		case GCFilterCollected:
			value = utils.MemorySize(event.Collected).MB()
		}

		chart.Push(utils.TimePoint{
			Time:  event.Timestamp,
			Value: value,
		})
	}

	// Set young generation style (light green)
	chart.SetStyle(lipgloss.NewStyle().Foreground(tui.GoodColor))

	// Add old generation events as separate dataset
	for _, event := range oldEvents {
		var value float64
		switch filter {
		case GCFilterBefore:
			value = utils.MemorySize(event.Before).MB()
		case GCFilterAfter:
			value = utils.MemorySize(event.After).MB()
		case GCFilterCollected:
			value = utils.MemorySize(event.Collected).MB()
		}

		chart.PushDataSet("old", utils.TimePoint{
			Time:  event.Timestamp,
			Value: value,
		})
	}

	// Set old generation style (orange/warning color)
	chart.SetDataSetStyle("old", lipgloss.NewStyle().Foreground(tui.WarningColor))

	chart.DrawBrailleAll()

	// Create title with current filter prominently displayed
	currentFilter := lipgloss.NewStyle().
		Foreground(tui.InfoColor).
		Bold(true).
		Render(fmt.Sprintf("Showing: %s Memory", filter.String()))
	filterHint := lipgloss.NewStyle().
		Foreground(tui.MutedStyle.GetForeground()).
		Render("[Press 'f' to cycle filters]")

	// Create legend with generation info
	youngLegend := lipgloss.NewStyle().Foreground(tui.GoodColor).Render("🐣 Young Gen")
	oldLegend := lipgloss.NewStyle().Foreground(tui.WarningColor).Render("👵 Old Gen")

	// Build header and legend lines
	legendCol := lipgloss.JoinVertical(lipgloss.Top, currentFilter, filterHint, "", youngLegend, oldLegend)

	// Get the chart view
	chartView := chart.View()
	chartView = lipgloss.JoinHorizontal(lipgloss.Left, chartView, legendCol)

	return lipgloss.JoinVertical(lipgloss.Left, "", chartView)
}

// renderGCSummaryGrid creates a clean, organized summary layout
func renderGCSummaryGrid(tracker *GCEventTracker, window time.Duration) string {
	totalGCs := tracker.GetTotalGCCount()
	totalTime := time.Duration(tracker.GetTotalGCTime()) * time.Millisecond
	avgPauseTime := tracker.GetAveragePauseTime(window)
	frequency := tracker.GetGCFrequency(window)
	overallAvg := tracker.GetOverallGCAverage()

	// Build metrics in a clean line format
	metrics := []string{
		fmt.Sprintf("Total GCs: %d", totalGCs),
		fmt.Sprintf("Total Time: %s", totalTime),
	}

	if avgPauseTime > 0 {
		metrics = append(metrics, fmt.Sprintf("Recent Avg: %s", utils.FormatDuration(avgPauseTime)))
	} else if overallAvg > 0 {
		metrics = append(metrics, fmt.Sprintf("Overall Avg: %.1fms", overallAvg))
	}

	if frequency > 0 {
		metrics = append(metrics, fmt.Sprintf("Frequency: %.1f/min", frequency))
	}

	// Create a clean horizontal layout with proper spacing
	summaryLine := "• " + metrics[0]
	for _, metric := range metrics[1:] {
		summaryLine += "    • " + metric
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		tui.InfoStyle.Render("GC Summary"),
		tui.MutedStyle.Render(summaryLine),
		"")
}

// renderMiddleSection combines generation stats and most recent GC info
func renderMiddleSection(tracker *GCEventTracker, window time.Duration) string {
	// Left side: Generation statistics
	generationStats := renderGenerationColumns(tracker, window)

	// Right side: Most recent GC info
	recentGCInfo := renderMostRecentGCBox(tracker)

	// Combine side by side if we have recent GC info
	if recentGCInfo != "" {
		return lipgloss.JoinHorizontal(lipgloss.Top,
			generationStats,
			"    ",
			recentGCInfo) + "\n"
	}

	return generationStats + "\n"
}

// renderGenerationColumns creates side-by-side generation statistics
func renderGenerationColumns(tracker *GCEventTracker, window time.Duration) string {
	youngStats := buildGenerationColumn(tracker, "young", window)
	oldStats := buildGenerationColumn(tracker, "old", window)

	youngColumn := lipgloss.NewStyle().Width(35).Render(
		lipgloss.JoinVertical(lipgloss.Left,
			tui.InfoStyle.Render("Young Generation"),
			youngStats))

	oldColumn := lipgloss.NewStyle().Width(35).Render(
		lipgloss.JoinVertical(lipgloss.Left,
			tui.InfoStyle.Render("Old Generation"),
			oldStats))

	return lipgloss.JoinHorizontal(lipgloss.Top, youngColumn, "  ", oldColumn)
}

func buildGenerationColumn(tracker *GCEventTracker, generation string, window time.Duration) string {
	var count int64
	var totalTime int64
	var avgTime float64

	if generation == "young" {
		if tracker.currentSnapshot != nil {
			count = tracker.currentSnapshot.GC.YoungGCCount
			totalTime = tracker.currentSnapshot.GC.YoungGCTime
		}
		avgTime = tracker.GetYoungGCAverage()
	} else {
		if tracker.currentSnapshot != nil {
			count = tracker.currentSnapshot.GC.OldGCCount
			totalTime = tracker.currentSnapshot.GC.OldGCTime
		}
		avgTime = tracker.GetOldGCAverage()
	}

	frequency := tracker.GetGCFrequencyByGeneration(generation, window)
	youngEff, oldEff, _ := tracker.CalculateEfficiency(window)

	var efficiency float64
	if generation == "young" {
		efficiency = youngEff
	} else {
		efficiency = oldEff
	}

	var color lipgloss.Color = tui.GoodColor
	if avgTime > 100 {
		color = tui.WarningColor
	}
	if avgTime > 500 {
		color = tui.CriticalColor
	}

	// Create clean metric rows
	lines := []string{
		fmt.Sprintf("Count: %d", count),
		fmt.Sprintf("Total Time: %s", utils.FormatDuration(time.Duration(totalTime)*time.Millisecond)),
		fmt.Sprintf("Avg Time: %.1fms", avgTime),
	}

	if frequency > 0 {
		lines = append(lines, fmt.Sprintf("Frequency: %.1f/min", frequency))
	}

	if efficiency > 0 {
		lines = append(lines, fmt.Sprintf("Efficiency: %.1f%%", efficiency))
	}

	if count > 0 {
		activityLevel := tracker.GetGCActivityLevel(count, avgTime, frequency)
		lines = append(lines, fmt.Sprintf("Activity: %s", activityLevel))
	}

	statsText := ""
	for _, line := range lines {
		statsText += "• " + line + "\n"
	}

	return lipgloss.NewStyle().Foreground(color).Render(statsText)
}

// renderMostRecentGCBox creates a focused box for the most recent GC
func renderMostRecentGCBox(tracker *GCEventTracker) string {
	id, generation, timestamp, duration, collected := tracker.GetMostRecentGCDetails()

	if generation == "none" || timestamp.IsZero() {
		return ""
	}

	var generationIcon string
	var generationColor lipgloss.Color
	var pauseColor lipgloss.Color = tui.GoodColor

	if generation == "young" {
		generationIcon = "🐣"
		generationColor = tui.InfoColor
	} else {
		generationIcon = "👵"
		generationColor = tui.WarningColor
	}

	if duration > 500*time.Millisecond {
		pauseColor = tui.CriticalColor
	} else if duration > 100*time.Millisecond {
		pauseColor = tui.WarningColor
	}

	timeAgo := time.Since(timestamp)

	lines := []string{
		fmt.Sprintf("%s %s Generation",
			generationIcon,
			lipgloss.NewStyle().Foreground(generationColor).Render(generation)),
		fmt.Sprintf("ID: %v", id),
		fmt.Sprintf("Duration: %s",
			lipgloss.NewStyle().Foreground(pauseColor).Render(duration.String())),
		fmt.Sprintf("Occurred: %s ago", utils.FormatDuration(timeAgo)),
	}

	if collected > 0 {
		lines = append(lines, fmt.Sprintf("Freed: %.1f MB", utils.MemorySize(collected).MB()))
	}

	content := ""
	for _, line := range lines {
		content += "• " + line + "\n"
	}

	return lipgloss.NewStyle().Width(30).Render(
		lipgloss.JoinVertical(lipgloss.Left,
			tui.InfoStyle.Render("Most Recent GC"),
			tui.MutedStyle.Render(content)))
}

// renderPerformanceGrid creates organized performance metrics
func renderPerformanceGrid(tracker *GCEventTracker, window time.Duration) string {
	// Left column: Overhead analysis
	overheadColumn := renderOverheadColumn(tracker, window)

	// Right column: Performance metrics
	metricsColumn := renderMetricsColumn(tracker, window)

	return lipgloss.JoinVertical(lipgloss.Left,
		tui.InfoStyle.Render("Performance Analysis"),
		lipgloss.JoinHorizontal(lipgloss.Top,
			overheadColumn,
			"    ",
			metricsColumn),
		"")
}

func renderOverheadColumn(tracker *GCEventTracker, window time.Duration) string {
	recentOverhead := tracker.CalculateGCOverhead(window)
	totalTime := float64(tracker.GetTotalGCTime())
	var overallOverhead float64
	if tracker.currentSnapshot != nil && !tracker.currentSnapshot.Runtime.StartTime.IsZero() {
		uptime := time.Since(tracker.currentSnapshot.Runtime.StartTime)
		if uptime > 0 {
			overallOverhead = totalTime / float64(uptime.Milliseconds())
		}
	}

	displayOverhead := recentOverhead
	if displayOverhead == 0 {
		displayOverhead = overallOverhead
	}

	var overheadColor lipgloss.Color
	var status string

	switch {
	case displayOverhead > 0.20:
		overheadColor = tui.CriticalColor
		status = "CRITICAL"
	case displayOverhead > 0.10:
		overheadColor = tui.CriticalColor
		status = "HIGH"
	case displayOverhead > 0.05:
		overheadColor = tui.WarningColor
		status = "MODERATE"
	case displayOverhead > 0.02:
		overheadColor = tui.InfoColor
		status = "LOW"
	default:
		overheadColor = tui.GoodColor
		status = "MINIMAL"
	}

	lines := []string{
		fmt.Sprintf("GC Overhead: %s",
			lipgloss.NewStyle().Foreground(overheadColor).Render(fmt.Sprintf("%.2f%%", displayOverhead*100))),
		fmt.Sprintf("Status: %s",
			lipgloss.NewStyle().Foreground(overheadColor).Render(status)),
	}

	if recentOverhead > 0 && overallOverhead > 0 && recentOverhead != overallOverhead {
		lines = append(lines, fmt.Sprintf("Recent: %.2f%%", recentOverhead*100))
		lines = append(lines, fmt.Sprintf("Overall: %.2f%%", overallOverhead*100))
	}

	content := ""
	for _, line := range lines {
		content += "• " + line + "\n"
	}

	return lipgloss.NewStyle().Width(40).Render(
		lipgloss.JoinVertical(lipgloss.Left,
			tui.MutedStyle.Render("Overhead Analysis"),
			tui.MutedStyle.Render(content)))
}

func renderMetricsColumn(tracker *GCEventTracker, window time.Duration) string {
	maxPause := tracker.GetMaxPause(window)
	longPauses := tracker.GetLongPauses(100*time.Millisecond, window)
	_, _, overallEfficiency := tracker.CalculateEfficiency(window)
	pressureLevel := tracker.GetGCPressureLevel(window)

	var lines []string

	if maxPause > 0 {
		pauseColor := tui.GoodColor
		if maxPause > 1*time.Second {
			pauseColor = tui.CriticalColor
		} else if maxPause > 500*time.Millisecond {
			pauseColor = tui.WarningColor
		}
		lines = append(lines,
			fmt.Sprintf("Max Pause: %s",
				lipgloss.NewStyle().Foreground(pauseColor).Render(maxPause.String())))
	}

	if longPauses > 0 {
		lines = append(lines, fmt.Sprintf("Long Pauses: %d", longPauses))
	}

	if overallEfficiency > 0 {
		efficiencyColor := tui.GoodColor
		if overallEfficiency < 30 {
			efficiencyColor = tui.WarningColor
		}
		if overallEfficiency < 10 {
			efficiencyColor = tui.CriticalColor
		}
		lines = append(lines,
			fmt.Sprintf("Efficiency: %s",
				lipgloss.NewStyle().Foreground(efficiencyColor).Render(fmt.Sprintf("%.1f%%", overallEfficiency))))
	}

	pressureColor := tui.GoodColor
	switch pressureLevel {
	case "critical", "high":
		pressureColor = tui.CriticalColor
	case "moderate":
		pressureColor = tui.WarningColor
	case "low":
		pressureColor = tui.InfoColor
	}

	lines = append(lines,
		fmt.Sprintf("GC Pressure: %s",
			lipgloss.NewStyle().Foreground(pressureColor).Render(pressureLevel)))

	content := ""
	for _, line := range lines {
		content += "• " + line + "\n"
	}

	return lipgloss.NewStyle().Width(40).Render(
		lipgloss.JoinVertical(lipgloss.Left,
			tui.MutedStyle.Render("Key Metrics"),
			tui.MutedStyle.Render(content)))
}

// renderRecentEventsClean creates a clean, scannable event list
func renderRecentEventsClean(events []GCEvent) string {
	if len(events) == 0 {
		return ""
	}

	lines := []string{
		tui.InfoStyle.Render("Recent GC Events"),
	}

	for _, event := range events {
		timeStr := event.Timestamp.Format("15:04:05")

		var generationIcon string
		var durationColor lipgloss.Color = tui.GoodColor

		if event.Generation == "young" {
			generationIcon = "🐣"
		} else {
			generationIcon = "👵"
		}

		if event.Duration > 500*time.Millisecond {
			durationColor = tui.CriticalColor
		} else if event.Duration > 100*time.Millisecond {
			durationColor = tui.WarningColor
		}

		// Create a clean, readable event line
		eventDetails := []string{
			fmt.Sprintf("[%s] %s %5s - %-4v", timeStr, generationIcon, event.Generation, event.Id),
			fmt.Sprintf("Duration: %5s", lipgloss.NewStyle().Foreground(durationColor).Render(event.Duration.String())),
		}

		if event.Collected > 0 {
			eventDetails = append(eventDetails, fmt.Sprintf("Freed: %.1f MB", utils.MemorySize(event.Collected).MB()))
		}

		if event.Before > 0 {
			efficiency := float64(event.Collected) / float64(event.Before) * 100
			eventDetails = append(eventDetails, fmt.Sprintf("Efficiency: %.1f%%", efficiency))
		}

		eventLine := "• " + eventDetails[0]
		for _, detail := range eventDetails[1:] {
			eventLine += "  |  " + detail
		}

		lines = append(lines, tui.MutedStyle.Render(eventLine))
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}
