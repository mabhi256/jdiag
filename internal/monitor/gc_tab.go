package monitor

import (
	"fmt"
	"time"

	"github.com/mabhi256/jdiag/internal/tui"

	"github.com/charmbracelet/lipgloss"
	"github.com/mabhi256/jdiag/utils"
)

func RenderGCTab(state *TabState, tracker *GCEventTracker) string {
	var sections []string

	// Analysis window for calculations
	window := 5 * time.Minute

	// GC statistics summary
	summarySection := renderGCSummary(tracker, window)
	sections = append(sections, summarySection)

	// Most recent GC information
	recentGCSection := renderMostRecentGC(tracker)
	sections = append(sections, recentGCSection)

	// Young Generation GC
	youngGCSection := renderGCSection("Young Generation GC",
		tracker, "young", window)
	sections = append(sections, youngGCSection)

	// Old Generation GC
	oldGCSection := renderGCSection("Old Generation GC",
		tracker, "old", window)
	sections = append(sections, oldGCSection)

	// GC Overhead analysis
	overheadSection := renderGCOverhead(tracker, window)
	sections = append(sections, overheadSection)

	// GC Performance analysis
	performanceSection := renderGCPerformance(tracker, window)
	sections = append(sections, performanceSection)

	// Recent GC events
	recentEvents := tracker.GetRecentEvents(5)
	if len(recentEvents) > 0 {
		recentEventsSection := renderRecentGCEvents(recentEvents)
		sections = append(sections, recentEventsSection)
	}

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// renderGCSummary shows overall GC statistics from tracker
func renderGCSummary(tracker *GCEventTracker, window time.Duration) string {
	totalGCs := tracker.GetTotalGCCount()
	totalTime := time.Duration(tracker.GetTotalGCTime()) * time.Millisecond
	avgPauseTime := tracker.GetAveragePauseTime(window)
	frequency := tracker.GetGCFrequency(window)

	// Calculate overall average if we have data
	overallAvg := tracker.GetOverallGCAverage()

	summaryLines := []string{
		fmt.Sprintf("Total GCs: %d", totalGCs),
		fmt.Sprintf("Total GC Time: %s", totalTime),
	}

	if avgPauseTime > 0 {
		summaryLines = append(summaryLines, fmt.Sprintf("Recent Avg Pause: %s", avgPauseTime))
	} else if overallAvg > 0 {
		summaryLines = append(summaryLines, fmt.Sprintf("Overall Avg Pause: %.1fms", overallAvg))
	}

	if frequency > 0 {
		summaryLines = append(summaryLines, fmt.Sprintf("Recent Frequency: %.1f/min", frequency))
	}

	summaryText := fmt.Sprintf("• %s", summaryLines[0])
	for _, line := range summaryLines[1:] {
		summaryText += "\n" + fmt.Sprintf("• %s", line)
	}

	summary := lipgloss.JoinVertical(lipgloss.Left,
		tui.InfoStyle.Render("GC Summary"),
		tui.MutedStyle.Render(summaryText),
		"", // Empty line for spacing
	)

	return summary
}

// renderMostRecentGC shows information about the most recent GC event
func renderMostRecentGC(tracker *GCEventTracker) string {
	generation, timestamp, duration, collected := tracker.GetMostRecentGCDetails()

	if generation == "none" || timestamp.IsZero() {
		return ""
	}

	var generationIcon string
	var generationColor lipgloss.Color

	if generation == "young" {
		generationIcon = "🐣"
		generationColor = tui.InfoColor
	} else {
		generationIcon = "👵"
		generationColor = tui.WarningColor
	}

	// Determine pause time color
	var pauseColor lipgloss.Color = tui.GoodColor
	if duration > 500*time.Millisecond {
		pauseColor = tui.CriticalColor
	} else if duration > 100*time.Millisecond {
		pauseColor = tui.WarningColor
	}

	timeAgo := time.Since(timestamp)

	recentGCLines := []string{
		fmt.Sprintf("%s %s Generation GC", generationIcon,
			lipgloss.NewStyle().Foreground(generationColor).Render(generation)),
		fmt.Sprintf("Duration: %s",
			lipgloss.NewStyle().Foreground(pauseColor).Render(duration.String())),
		fmt.Sprintf("Occurred: %s ago", utils.FormatDuration(timeAgo)),
	}

	if collected > 0 {
		recentGCLines = append(recentGCLines,
			fmt.Sprintf("Freed: %0.2f MB", utils.MemorySize(collected).MB()))
	}

	recentGCText := ""
	for i, line := range recentGCLines {
		if i == 0 {
			recentGCText = "• " + line
		} else {
			recentGCText += "\n• " + line
		}
	}

	recentGC := lipgloss.JoinVertical(lipgloss.Left,
		tui.InfoStyle.Render("Most Recent GC"),
		tui.MutedStyle.Render(recentGCText),
		"", // Empty line for spacing
	)

	return recentGC
}

// renderGCSection renders individual GC generation statistics using tracker data
func renderGCSection(title string, tracker *GCEventTracker, generation string, window time.Duration) string {
	var count int64
	var totalTime int64
	var avgTime float64

	// Get raw data from tracker
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

	// Get calculated metrics
	frequency := tracker.GetGCFrequencyByGeneration(generation, window)
	youngEff, oldEff, _ := tracker.CalculateEfficiency(window)

	var efficiency float64
	if generation == "young" {
		efficiency = youngEff
	} else {
		efficiency = oldEff
	}

	// Determine color based on frequency and time
	var color lipgloss.Color = tui.GoodColor

	// Color based on average GC time
	if avgTime > 100 { // > 100ms average
		color = tui.WarningColor
	}
	if avgTime > 500 { // > 500ms average
		color = tui.CriticalColor
	}

	// Format the statistics
	titleStyled := tui.InfoStyle.Render(title)

	statsLines := []string{
		fmt.Sprintf("Count: %d", count),
		fmt.Sprintf("Total Time: %s", utils.FormatDuration(time.Duration(totalTime)*time.Millisecond)),
		fmt.Sprintf("Avg Time: %.1fms", avgTime),
	}

	// Add frequency if available
	if frequency > 0 {
		statsLines = append(statsLines, fmt.Sprintf("Frequency: %.1f/min", frequency))
	}

	// Add efficiency if available
	if efficiency > 0 {
		statsLines = append(statsLines, fmt.Sprintf("Efficiency: %.1f%%", efficiency))
	}

	// Add activity level assessment using tracker's calculation
	if count > 0 {
		activityLevel := tracker.GetGCActivityLevel(count, avgTime, frequency)
		statsLines = append(statsLines, fmt.Sprintf("Activity: %s", activityLevel))
	}

	statsText := ""
	for i, line := range statsLines {
		if i == 0 {
			statsText = line
		} else {
			statsText += " | " + line
		}
	}

	section := lipgloss.JoinVertical(lipgloss.Left,
		titleStyled,
		lipgloss.NewStyle().Foreground(color).Render(statsText),
		"", // Empty line for spacing
	)

	return section
}

// renderGCOverhead shows GC overhead analysis using tracker calculations
func renderGCOverhead(tracker *GCEventTracker, window time.Duration) string {
	recentOverhead := tracker.CalculateGCOverhead(window)

	// Also calculate overall overhead for comparison
	totalTime := float64(tracker.GetTotalGCTime())
	var overallOverhead float64
	if tracker.currentSnapshot != nil && !tracker.currentSnapshot.Runtime.StartTime.IsZero() {
		uptime := time.Since(tracker.currentSnapshot.Runtime.StartTime)
		if uptime > 0 {
			overallOverhead = totalTime / float64(uptime.Milliseconds())
		}
	}

	// Use recent overhead if available, otherwise fall back to overall
	displayOverhead := recentOverhead
	if displayOverhead == 0 {
		displayOverhead = overallOverhead
	}

	var overheadColor lipgloss.Color
	var status string

	switch {
	case displayOverhead > 0.20: // 20%
		overheadColor = tui.CriticalColor
		status = "CRITICAL - Application severely impacted"
	case displayOverhead > 0.10: // 10%
		overheadColor = tui.CriticalColor
		status = "HIGH - Performance significantly impacted"
	case displayOverhead > 0.05: // 5%
		overheadColor = tui.WarningColor
		status = "MODERATE - Monitor for performance impact"
	case displayOverhead > 0.02: // 2%
		overheadColor = tui.InfoColor
		status = "LOW - Normal GC overhead"
	default:
		overheadColor = tui.GoodColor
		status = "MINIMAL - Excellent GC performance"
	}

	overheadLines := []string{
		fmt.Sprintf("%.2f%% of time spent in GC", displayOverhead*100),
	}

	if recentOverhead > 0 && overallOverhead > 0 && recentOverhead != overallOverhead {
		overheadLines = append(overheadLines,
			fmt.Sprintf("Recent: %.2f%% | Overall: %.2f%%", recentOverhead*100, overallOverhead*100))
	}

	overheadText := ""
	for i, line := range overheadLines {
		if i == 0 {
			overheadText = line
		} else {
			overheadText += "\n" + line
		}
	}

	overheadSection := lipgloss.JoinVertical(lipgloss.Left,
		tui.InfoStyle.Render("GC Overhead Analysis"),
		lipgloss.NewStyle().Foreground(overheadColor).Render(overheadText),
		tui.MutedStyle.Render(status),
		"", // Empty line for spacing
	)

	return overheadSection
}

// renderGCPerformance shows GC performance analysis using tracker metrics
func renderGCPerformance(tracker *GCEventTracker, window time.Duration) string {
	maxPause := tracker.GetMaxPause(window)
	longPauses := tracker.GetLongPauses(100*time.Millisecond, window)
	_, _, overallEfficiency := tracker.CalculateEfficiency(window)
	pressureLevel := tracker.GetGCPressureLevel(window)

	var performanceLines []string

	if maxPause > 0 {
		pauseColor := tui.GoodColor
		if maxPause > 1*time.Second {
			pauseColor = tui.CriticalColor
		} else if maxPause > 500*time.Millisecond {
			pauseColor = tui.WarningColor
		}

		performanceLines = append(performanceLines,
			fmt.Sprintf("Max Recent Pause: %s",
				lipgloss.NewStyle().Foreground(pauseColor).Render(maxPause.String())))
	}

	if longPauses > 0 {
		performanceLines = append(performanceLines,
			fmt.Sprintf("Long Pauses (>100ms): %d", longPauses))
	}

	if overallEfficiency > 0 {
		efficiencyColor := tui.GoodColor
		if overallEfficiency < 30 {
			efficiencyColor = tui.WarningColor
		}
		if overallEfficiency < 10 {
			efficiencyColor = tui.CriticalColor
		}

		performanceLines = append(performanceLines,
			fmt.Sprintf("Collection Efficiency: %s",
				lipgloss.NewStyle().Foreground(efficiencyColor).Render(fmt.Sprintf("%.1f%%", overallEfficiency))))
	}

	// Add pressure level
	pressureColor := tui.GoodColor
	switch pressureLevel {
	case "critical":
		pressureColor = tui.CriticalColor
	case "high":
		pressureColor = tui.CriticalColor
	case "moderate":
		pressureColor = tui.WarningColor
	case "low":
		pressureColor = tui.InfoColor
	}

	performanceLines = append(performanceLines,
		fmt.Sprintf("GC Pressure: %s",
			lipgloss.NewStyle().Foreground(pressureColor).Render(pressureLevel)))

	if len(performanceLines) == 0 {
		return ""
	}

	performanceText := ""
	for i, line := range performanceLines {
		if i == 0 {
			performanceText = "• " + line
		} else {
			performanceText += "\n• " + line
		}
	}

	performanceSection := lipgloss.JoinVertical(lipgloss.Left,
		tui.InfoStyle.Render("GC Performance Metrics"),
		tui.MutedStyle.Render(performanceText),
		"", // Empty line for spacing
	)

	return performanceSection
}

// renderRecentGCEvents shows recent GC events (events come from tracker)
func renderRecentGCEvents(events []GCEvent) string {
	if len(events) == 0 {
		return ""
	}

	var eventLines []string
	for _, event := range events {
		timeStr := event.Timestamp.Format("15:04:05")
		durationStr := event.Duration.String()

		var generationIcon string
		var durationColor lipgloss.Color = tui.GoodColor

		if event.Generation == "young" {
			generationIcon = "🐣"
		} else {
			generationIcon = "👵"
		}

		// Color duration based on length
		if event.Duration > 500*time.Millisecond {
			durationColor = tui.CriticalColor
		} else if event.Duration > 100*time.Millisecond {
			durationColor = tui.WarningColor
		}

		eventLine := fmt.Sprintf("%s [%s] %s GC - %s",
			timeStr, generationIcon, event.Generation,
			lipgloss.NewStyle().Foreground(durationColor).Render(durationStr))

		if event.Collected > 0 {
			eventLine += fmt.Sprintf(" (freed %0.2f MB)", utils.MemorySize(event.Collected).MB())
		}

		// Add efficiency indicator if we have before/after data
		if event.Before > 0 {
			efficiency := float64(event.Collected) / float64(event.Before) * 100
			eventLine += fmt.Sprintf(" [%.1f%% efficiency]", efficiency)
		}

		eventLines = append(eventLines, eventLine)
	}

	eventsText := ""
	for i, line := range eventLines {
		if i == 0 {
			eventsText = "• " + line
		} else {
			eventsText += "\n• " + line
		}
	}

	recentSection := lipgloss.JoinVertical(lipgloss.Left,
		tui.InfoStyle.Render("Recent GC Events"),
		tui.MutedStyle.Render(eventsText),
		"", // Empty line for spacing
	)

	return recentSection
}
