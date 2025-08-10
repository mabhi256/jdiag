package monitor

import (
	"fmt"
	"time"

	"github.com/mabhi256/jdiag/internal/tui"

	"github.com/charmbracelet/lipgloss"
	"github.com/mabhi256/jdiag/utils"
)

// Render renders the GC tab view
func RenderGCTab(state *TabState) string {
	var sections []string

	// GC pressure overview
	pressureOverview := renderGCPressureOverview(state.GC)
	sections = append(sections, pressureOverview)

	// GC statistics summary
	summarySection := renderGCSummary(state.GC)
	sections = append(sections, summarySection)

	// Young Generation GC
	youngGCSection := renderGCSection("Young Generation GC",
		state.GC.YoungGCCount,
		state.GC.YoungGCTime,
		state.GC.YoungGCAvg)
	sections = append(sections, youngGCSection)

	// Old Generation GC
	oldGCSection := renderGCSection("Old Generation GC",
		state.GC.OldGCCount,
		state.GC.OldGCTime,
		state.GC.OldGCAvg)
	sections = append(sections, oldGCSection)

	// GC Overhead analysis
	overheadSection := renderGCOverhead(state.GC)
	sections = append(sections, overheadSection)

	// Recent GC events
	if len(state.GC.RecentGCEvents) > 0 {
		recentEventsSection := renderRecentGCEvents(state.GC)
		sections = append(sections, recentEventsSection)
	}

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// renderGCPressureOverview shows overall GC pressure status
func renderGCPressureOverview(gc *GCState) string {
	pressureLevel := gc.GetGCPressureLevel()

	var pressureColor lipgloss.Color
	var pressureIcon string

	switch pressureLevel {
	case "critical":
		pressureColor = tui.CriticalColor
		pressureIcon = "🔴"
	case "high":
		pressureColor = tui.WarningColor
		pressureIcon = "🟡"
	case "moderate":
		pressureColor = tui.InfoColor
		pressureIcon = "🟠"
	default:
		pressureColor = tui.GoodColor
		pressureIcon = "🟢"
	}

	pressureText := fmt.Sprintf("%s GC Pressure: %s", pressureIcon, pressureLevel)

	// Add overhead info
	overheadText := ""
	if gc.GCOverhead > 0 {
		overheadText = fmt.Sprintf(" (%.1f%% overhead)", gc.GCOverhead*100)
	}

	overview := lipgloss.NewStyle().
		Foreground(pressureColor).
		Bold(true).
		Render(pressureText + overheadText)

	return overview + "\n"
}

// renderGCSummary shows overall GC statistics
func renderGCSummary(gc *GCState) string {
	totalGCs := gc.TotalGCCount
	totalTime := time.Duration(gc.TotalGCTime) * time.Millisecond

	var avgPauseTime time.Duration
	if totalGCs > 0 {
		avgPauseTime = totalTime / time.Duration(totalGCs)
	}

	summaryLines := []string{
		fmt.Sprintf("Total GCs: %d", totalGCs),
		fmt.Sprintf("Total GC Time: %s", totalTime),
		fmt.Sprintf("Average Pause: %s", avgPauseTime),
	}

	if gc.GCFrequency > 0 {
		summaryLines = append(summaryLines, fmt.Sprintf("GC Frequency: %.1f/min", gc.GCFrequency))
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

// renderGCSection renders individual GC generation statistics
func renderGCSection(title string, count int64, totalTime int64, avgTime float64) string {
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

	// Add frequency if we have enough data
	if count > 0 {
		// Estimate frequency (would need more context for accurate calculation)
		statsLines = append(statsLines, fmt.Sprintf("Recent Activity: %s",
			getGCActivityLevel(count, avgTime)))
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

// getGCActivityLevel determines activity level based on count and average time
func getGCActivityLevel(count int64, avgTime float64) string {
	switch {
	case avgTime > 500:
		return "High Impact"
	case avgTime > 100:
		return "Moderate Impact"
	case count > 1000:
		return "Frequent"
	case count > 100:
		return "Regular"
	default:
		return "Low"
	}
}

// renderGCOverhead shows GC overhead analysis
func renderGCOverhead(gc *GCState) string {
	overhead := gc.GCOverhead

	var overheadColor lipgloss.Color
	var status string

	switch {
	case overhead > 0.20: // 20%
		overheadColor = tui.CriticalColor
		status = "CRITICAL - Application severely impacted"
	case overhead > 0.10: // 10%
		overheadColor = tui.CriticalColor
		status = "HIGH - Performance significantly impacted"
	case overhead > 0.05: // 5%
		overheadColor = tui.WarningColor
		status = "MODERATE - Monitor for performance impact"
	case overhead > 0.02: // 2%
		overheadColor = tui.InfoColor
		status = "LOW - Normal GC overhead"
	default:
		overheadColor = tui.GoodColor
		status = "MINIMAL - Excellent GC performance"
	}

	overheadSection := lipgloss.JoinVertical(lipgloss.Left,
		tui.InfoStyle.Render("GC Overhead Analysis"),
		lipgloss.NewStyle().Foreground(overheadColor).Render(
			fmt.Sprintf("%.2f%% of total time spent in GC", overhead*100)),
		tui.MutedStyle.Render(status),
		"", // Empty line for spacing
	)

	return overheadSection
}

// renderRecentGCEvents shows recent GC events
func renderRecentGCEvents(gc *GCState) string {
	if len(gc.RecentGCEvents) == 0 {
		return ""
	}

	// Show last few GC events
	maxEvents := 5
	events := gc.RecentGCEvents
	if len(events) > maxEvents {
		events = events[len(events)-maxEvents:]
	}

	var eventLines []string
	for _, event := range events {
		timeStr := event.Timestamp.Format("15:04:05")
		durationStr := event.Duration.String()

		var generationIcon string
		if event.Generation == "young" {
			generationIcon = "🐣"
		} else {
			generationIcon = "👵"
		}

		eventLine := fmt.Sprintf("%s [%s] %s GC - %s",
			timeStr, generationIcon, event.Generation, durationStr)

		if event.Collected > 0 {
			eventLine += fmt.Sprintf(" (freed %v)", utils.MemorySize(event.Collected).MB())
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
