package watch

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/mabhi256/jdiag/utils"
)

// Render renders the threads tab view
func RenderThreadsTab(state *TabState, width int, classHistory []utils.TimeMap, threadHistory []utils.TimeMap) string {
	var sections []string

	chartsSection := renderThreadsCharts(state.Threads, width, classHistory, threadHistory)
	sections = append(sections, chartsSection)

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

// Show thread and class charts side by side
func renderThreadsCharts(threads *ThreadState, width int, classHistory []utils.TimeMap, threadHistory []utils.TimeMap) string {
	chartWidth := width - 20

	threadsChartSection := renderThreadChart(threads, chartWidth, threadHistory)
	classChartSection := renderClassChart(threads, chartWidth, classHistory)

	chartsRow := lipgloss.JoinVertical(lipgloss.Top, "", threadsChartSection, "", classChartSection)

	return chartsRow + "\n"
}

func renderThreadChart(threads *ThreadState, width int, threadHistory []utils.TimeMap) string {
	valuesText := fmt.Sprintf("Current: %d | Peak: %d | Daemon: %d | Total Started: %d",
		threads.CurrentThreadCount,
		threads.PeakThreadCount,
		threads.DaemonThreadCount,
		threads.TotalStartedCount)

	var chartView string

	graphWidth := max(width-10, 30)
	graphHeight := 8

	chart := utils.NewChart(graphWidth, graphHeight)

	for _, point := range threadHistory {
		currentCount := point.GetOrDefault("current_count", 0)
		chart.Push(utils.TimePoint{
			Time:  point.Timestamp,
			Value: float64(currentCount),
		})
	}

	// Set total loaded style (blue/info)
	chart.SetStyle(lipgloss.NewStyle().Foreground(utils.InfoColor))

	for _, point := range threadHistory {
		daemonCount := point.GetOrDefault("daemon_count", 0)
		chart.PushDataSet("daemon", utils.TimePoint{
			Time:  point.Timestamp,
			Value: float64(daemonCount),
		})
	}
	chart.SetDataSetStyle("daemon", lipgloss.NewStyle().Foreground(utils.WarningColor))

	chart.DrawBrailleAll()

	// Create legend
	currentLegend := lipgloss.NewStyle().Foreground(utils.InfoColor).Render("■ Current")
	daemonLegend := lipgloss.NewStyle().Foreground(utils.WarningColor).Render("■ Daemon")
	legend := lipgloss.JoinHorizontal(lipgloss.Left, currentLegend, "  ", daemonLegend)

	chartView = lipgloss.JoinHorizontal(lipgloss.Left, chart.View(), "", legend)

	section := lipgloss.JoinVertical(lipgloss.Left,
		utils.InfoStyle.Render("Threads"),
		chartView,
		utils.MutedStyle.Render(valuesText),
		"", // Empty line for spacing
	)

	return section
}

// renderClassChart renders the class loading chart
func renderClassChart(threads *ThreadState, width int, classHistory []utils.TimeMap) string {
	valuesText := fmt.Sprintf("Loaded: %d | Unloaded: %d | Currently Loaded: %d",
		threads.LoadedClassCount,
		threads.UnloadedClassCount,
		threads.TotalLoadedClasses)

	var chartView string

	graphWidth := max(width-10, 30)
	graphHeight := 8

	chart := utils.NewChart(graphWidth, graphHeight)

	for _, point := range classHistory {
		totalLoaded := point.GetOrDefault("total_loaded", 0)
		chart.Push(utils.TimePoint{
			Time:  point.Timestamp,
			Value: float64(totalLoaded),
		})
	}

	// Set current threads style (green)
	chart.SetStyle(lipgloss.NewStyle().Foreground(utils.GoodColor))

	for _, point := range classHistory {
		unloadedCount := point.GetOrDefault("unloaded_count", 0)
		chart.PushDataSet("unloaded", utils.TimePoint{
			Time:  point.Timestamp,
			Value: float64(unloadedCount),
		})
	}
	chart.SetDataSetStyle("unloaded", lipgloss.NewStyle().Foreground(utils.InfoColor))

	chart.DrawBrailleAll()

	// Create legend
	loadedLegend := lipgloss.NewStyle().Foreground(utils.GoodColor).Render("■ Total Loaded")
	unloadedLegend := lipgloss.NewStyle().Foreground(utils.InfoColor).Render("■ Unloaded")
	legend := lipgloss.JoinHorizontal(lipgloss.Left, loadedLegend, "  ", unloadedLegend)

	chartView = lipgloss.JoinHorizontal(lipgloss.Left, chart.View(), "", legend)

	section := lipgloss.JoinVertical(lipgloss.Left,
		utils.InfoStyle.Render("Classes"),
		chartView,
		utils.MutedStyle.Render(valuesText),
		"", // Empty line for spacing
	)

	return section
}

// renderThreadPerformance shows thread performance metrics
func renderThreadPerformance(threads *ThreadState) string {
	var performanceLines []string

	if threads.ThreadCreationRate > 0 {
		creationColor := utils.GoodColor
		if threads.ThreadCreationRate > 10 { // More than 10 threads/min
			creationColor = utils.WarningColor
		}
		if threads.ThreadCreationRate > 30 { // More than 30 threads/min
			creationColor = utils.CriticalColor
		}

		creationLine := lipgloss.NewStyle().Foreground(creationColor).Render(
			fmt.Sprintf("Thread Creation Rate: %.1f/min", threads.ThreadCreationRate))
		performanceLines = append(performanceLines, "• "+creationLine)
	}

	if threads.ThreadContention {
		contentionLine := lipgloss.NewStyle().Foreground(utils.WarningColor).Render(
			"Thread Contention: Detected")
		performanceLines = append(performanceLines, "• "+contentionLine)
	}

	if threads.DeadlockedThreads > 0 {
		deadlockLine := lipgloss.NewStyle().Foreground(utils.CriticalColor).Render(
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
		utils.InfoStyle.Render("Thread Performance"),
		performanceText,
		"", // Empty line for spacing
	)

	return section
}

func renderThreadStates(threads *ThreadState) string {
	runningThreads := threads.CurrentThreadCount - threads.BlockedThreadCount - threads.WaitingThreadCount

	var stateLines []string

	if runningThreads > 0 {
		stateLines = append(stateLines,
			fmt.Sprintf("Running: %d", runningThreads))
	}

	if threads.BlockedThreadCount > 0 {
		blockedColor := utils.WarningColor
		if threads.BlockedThreadCount > threads.CurrentThreadCount/4 { // More than 25% blocked
			blockedColor = utils.CriticalColor
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
		utils.InfoStyle.Render("Thread States"),
		statesText,
		"", // Empty line for spacing
	)

	return section
}
