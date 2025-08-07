package monitor

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/mabhi256/jdiag/internal/tui"
)

// Render renders the threads tab view
func RenderThreadsTab(state *TabState, width int) string {
	var sections []string

	// Thread overview
	overviewSection := renderThreadOverview(state.Threads)
	sections = append(sections, overviewSection)

	// Thread counts section
	threadCountsSection := renderThreadCounts(state.Threads, width)
	sections = append(sections, threadCountsSection)

	// Class loading section
	classLoadingSection := renderClassLoading(state.Threads)
	sections = append(sections, classLoadingSection)

	// Thread performance metrics
	if state.Threads.ThreadCreationRate > 0 || state.Threads.ThreadContention {
		performanceSection := renderThreadPerformance(state.Threads)
		sections = append(sections, performanceSection)
	}

	// Thread state breakdown (if available)
	if state.Threads.BlockedThreadCount > 0 || state.Threads.WaitingThreadCount > 0 {
		stateSection := renderThreadStates(state.Threads)
		sections = append(sections, stateSection)
	}

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// renderThreadOverview shows high-level thread status
func renderThreadOverview(threads *ThreadState) string {
	var statusColor lipgloss.Color
	var statusIcon string
	var statusText string

	threadUtilization := float64(threads.CurrentThreadCount) / float64(threads.PeakThreadCount)
	if threads.PeakThreadCount == 0 {
		threadUtilization = 0
	}

	switch {
	case threads.DeadlockedThreads > 0:
		statusColor = tui.CriticalColor
		statusIcon = "🔴"
		statusText = fmt.Sprintf("DEADLOCK DETECTED (%d threads)", threads.DeadlockedThreads)
	case threads.ThreadContention:
		statusColor = tui.WarningColor
		statusIcon = "🟡"
		statusText = "Thread contention detected"
	case threadUtilization > 0.9:
		statusColor = tui.WarningColor
		statusIcon = "🟡"
		statusText = "High thread utilization"
	case threads.CurrentThreadCount > 1000:
		statusColor = tui.InfoColor
		statusIcon = "🟠"
		statusText = "High thread count"
	default:
		statusColor = tui.GoodColor
		statusIcon = "🟢"
		statusText = "Normal thread activity"
	}

	overview := lipgloss.NewStyle().
		Foreground(statusColor).
		Bold(true).
		Render(fmt.Sprintf("%s %s", statusIcon, statusText))

	return overview + "\n"
}

// renderThreadCounts shows thread count statistics
func renderThreadCounts(threads *ThreadState, width int) string {
	// Create progress bar for thread usage
	var threadProgress float64
	if threads.PeakThreadCount > 0 {
		threadProgress = float64(threads.CurrentThreadCount) / float64(threads.PeakThreadCount)
	}

	var color lipgloss.Color = tui.GoodColor
	if threadProgress > 0.8 {
		color = tui.WarningColor
	}
	if threadProgress > 0.95 {
		color = tui.CriticalColor
	}

	barWidth := width/2 - 10
	if barWidth < 20 {
		barWidth = 20
	}

	progressBar := tui.CreateProgressBar(threadProgress, barWidth, color)
	percentStr := fmt.Sprintf("%.1f%% of peak", threadProgress*100)

	progressLine := fmt.Sprintf("%s %s", progressBar, percentStr)
	detailLine := fmt.Sprintf("Current: %d | Peak: %d | Daemon: %d",
		threads.CurrentThreadCount,
		threads.PeakThreadCount,
		threads.DaemonThreadCount)

	section := lipgloss.JoinVertical(lipgloss.Left,
		tui.InfoStyle.Render("Thread Count"),
		progressLine,
		tui.MutedStyle.Render(detailLine),
		"", // Empty line for spacing
	)

	return section
}

// renderClassLoading shows class loading statistics
func renderClassLoading(threads *ThreadState) string {
	classStats := []string{
		fmt.Sprintf("Loaded: %d", threads.LoadedClassCount),
		fmt.Sprintf("Unloaded: %d", threads.UnloadedClassCount),
		fmt.Sprintf("Currently Loaded: %d", threads.TotalLoadedClasses),
	}

	// Add loading rates if available
	if threads.ClassLoadingRate > 0 {
		classStats = append(classStats, fmt.Sprintf("Loading Rate: %.1f/min", threads.ClassLoadingRate))
	}

	if threads.ClassUnloadingRate > 0 {
		classStats = append(classStats, fmt.Sprintf("Unloading Rate: %.1f/min", threads.ClassUnloadingRate))
	}

	statsText := "• " + classStats[0]
	for _, stat := range classStats[1:] {
		statsText += "\n• " + stat
	}

	section := lipgloss.JoinVertical(lipgloss.Left,
		tui.InfoStyle.Render("Class Loading"),
		tui.MutedStyle.Render(statsText),
		"", // Empty line for spacing
	)

	return section
}

// renderThreadPerformance shows thread performance metrics
func renderThreadPerformance(threads *ThreadState) string {
	var performanceLines []string

	if threads.ThreadCreationRate > 0 {
		creationColor := tui.GoodColor
		if threads.ThreadCreationRate > 10 { // More than 10 threads/min
			creationColor = tui.WarningColor
		}
		if threads.ThreadCreationRate > 30 { // More than 30 threads/min
			creationColor = tui.CriticalColor
		}

		creationLine := lipgloss.NewStyle().Foreground(creationColor).Render(
			fmt.Sprintf("Thread Creation Rate: %.1f/min", threads.ThreadCreationRate))
		performanceLines = append(performanceLines, "• "+creationLine)
	}

	if threads.ThreadContention {
		contentionLine := lipgloss.NewStyle().Foreground(tui.WarningColor).Render(
			"Thread Contention: Detected")
		performanceLines = append(performanceLines, "• "+contentionLine)
	}

	if threads.DeadlockedThreads > 0 {
		deadlockLine := lipgloss.NewStyle().Foreground(tui.CriticalColor).Render(
			fmt.Sprintf("Deadlocked Threads: %d", threads.DeadlockedThreads))
		performanceLines = append(performanceLines, "• "+deadlockLine)
	}

	if len(performanceLines) == 0 {
		return ""
	}

	performanceText := performanceLines[0]
	for _, line := range performanceLines[1:] {
		performanceText += "\n" + line
	}

	section := lipgloss.JoinVertical(lipgloss.Left,
		tui.InfoStyle.Render("Thread Performance"),
		performanceText,
		"", // Empty line for spacing
	)

	return section
}

// renderThreadStates shows breakdown of thread states
func renderThreadStates(threads *ThreadState) string {
	runningThreads := threads.CurrentThreadCount - threads.BlockedThreadCount - threads.WaitingThreadCount

	var stateLines []string

	if runningThreads > 0 {
		stateLines = append(stateLines,
			fmt.Sprintf("Running: %d", runningThreads))
	}

	if threads.BlockedThreadCount > 0 {
		blockedColor := tui.WarningColor
		if threads.BlockedThreadCount > threads.CurrentThreadCount/4 { // More than 25% blocked
			blockedColor = tui.CriticalColor
		}

		blockedLine := lipgloss.NewStyle().Foreground(blockedColor).Render(
			fmt.Sprintf("Blocked: %d", threads.BlockedThreadCount))
		stateLines = append(stateLines, blockedLine)
	}

	if threads.WaitingThreadCount > 0 {
		stateLines = append(stateLines,
			fmt.Sprintf("Waiting: %d", threads.WaitingThreadCount))
	}

	if len(stateLines) == 0 {
		return ""
	}

	statesText := "• " + stateLines[0]
	for _, line := range stateLines[1:] {
		statesText += "\n• " + line
	}

	section := lipgloss.JoinVertical(lipgloss.Left,
		tui.InfoStyle.Render("Thread States"),
		statesText,
		"", // Empty line for spacing
	)

	return section
}
