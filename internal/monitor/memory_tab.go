package monitor

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/mabhi256/jdiag/internal/tui"
	"github.com/mabhi256/jdiag/utils"
)

func RenderMemoryTab(state *TabState, width int, heapHistory []utils.MultiValueTimePoint) string {
	var sections []string

	// Title
	titleStyled := tui.InfoStyle.Render("Heap Memory (Used vs Committed)")
	sections = append(sections, titleStyled)

	// ntcharts timeseries graph
	graphSection := renderHeapGraph(heapHistory, width)
	sections = append(sections, graphSection)
	sections = append(sections, "")

	// Calculate width for each column (accounting for separator and padding)
	columnWidth := (width - 3) / 2 // -3 for " | " separator

	// Get most recent GC info
	recentGCInfo, isYoungGen := getMostRecentGCInfo(state)

	// Create memory sections with adjusted width
	heapSection := renderMemorySection("Heap Memory",
		state.Memory.HeapUsed,
		state.Memory.HeapCommitted,
		state.Memory.HeapMax,
		state.Memory.HeapUsagePercent,
		columnWidth)

	var youngSection, oldSection string
	if isYoungGen && recentGCInfo != "" {
		youngSection = renderMemorySectionWithGC("Young Gen", recentGCInfo,
			state.Memory.YoungUsed,
			state.Memory.YoungCommitted,
			state.Memory.YoungMax,
			state.Memory.YoungUsagePercent,
			columnWidth)
		oldSection = renderMemorySection("Old Gen",
			state.Memory.OldUsed,
			state.Memory.OldCommitted,
			state.Memory.OldMax,
			state.Memory.OldUsagePercent,
			columnWidth)
	} else if !isYoungGen && recentGCInfo != "" {
		youngSection = renderMemorySection("Young Gen",
			state.Memory.YoungUsed,
			state.Memory.YoungCommitted,
			state.Memory.YoungMax,
			state.Memory.YoungUsagePercent,
			columnWidth)
		oldSection = renderMemorySectionWithGC("Old Gen", recentGCInfo,
			state.Memory.OldUsed,
			state.Memory.OldCommitted,
			state.Memory.OldMax,
			state.Memory.OldUsagePercent,
			columnWidth)
	} else {
		// No recent GC info, render normally
		youngSection = renderMemorySection("Young Gen",
			state.Memory.YoungUsed,
			state.Memory.YoungCommitted,
			state.Memory.YoungMax,
			state.Memory.YoungUsagePercent,
			columnWidth)
		oldSection = renderMemorySection("Old Gen",
			state.Memory.OldUsed,
			state.Memory.OldCommitted,
			state.Memory.OldMax,
			state.Memory.OldUsagePercent,
			columnWidth)
	}

	nonHeapSection := renderMemorySection("Non-Heap Memory",
		state.Memory.NonHeapUsed,
		state.Memory.NonHeapCommitted,
		state.Memory.NonHeapMax,
		state.Memory.NonHeapUsagePercent,
		columnWidth)

	topRow := lipgloss.JoinHorizontal(lipgloss.Top, "  ", heapSection, "     ", youngSection)
	// Create bottom row: non-heap | old
	bottomRow := lipgloss.JoinHorizontal(lipgloss.Top, "  ", nonHeapSection, "    ", oldSection)

	// Add the grid rows to sections
	sections = append(sections, topRow)
	sections = append(sections, bottomRow)

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// getMostRecentGCInfo returns the formatted GC info and whether it was young generation
func getMostRecentGCInfo(state *TabState) (string, bool) {
	if state.GC == nil || len(state.GC.RecentGCEvents) == 0 {
		return "", false
	}

	// Get the most recent GC event
	recentEvent := state.GC.RecentGCEvents[len(state.GC.RecentGCEvents)-1]

	// Determine emoji and generation
	var emoji string
	isYoungGen := recentEvent.Generation == "young"
	if isYoungGen {
		emoji = "🐣"
	} else {
		emoji = "👵"
	}

	// Format timestamp
	timeStr := recentEvent.Timestamp.Format("15:04:05")

	// Format freed memory
	freedStr := utils.MemorySize(recentEvent.Collected).MB()

	// Format GC time
	gcTimeStr := recentEvent.Duration.String()

	// Create the formatted string
	gcInfo := fmt.Sprintf("%s GC @ %s, freed %0.2f MB, %s", emoji, timeStr, freedStr, gcTimeStr)

	return gcInfo, isYoungGen
}

func renderHeapGraph(history []utils.MultiValueTimePoint, width int) string {
	if len(history) < 2 {
		return ""
	}

	graphWidth := max(width-10, 40)
	graphHeight := 10

	chart := utils.NewChart(graphWidth, graphHeight)

	for _, point := range history {
		chart.Push(utils.TimePoint{
			Time:  point.Timestamp,
			Value: point.GetUsedMB(),
		})
	}

	// Set Used memory style (green)
	chart.SetStyle(lipgloss.NewStyle().Foreground(tui.GoodColor))

	// Add Committed memory as named dataset
	for _, point := range history {
		chart.PushDataSet("committed", utils.TimePoint{
			Time:  point.Timestamp,
			Value: point.GetCommittedMB(),
		})
	}
	chart.SetDataSetStyle("committed", lipgloss.NewStyle().Foreground(tui.InfoColor))

	chart.DrawBrailleAll()

	// Create legend
	usedLegend := lipgloss.NewStyle().Foreground(tui.GoodColor).Render("■ Used")
	committedLegend := lipgloss.NewStyle().Foreground(tui.InfoColor).Render("■ Committed")
	legend := lipgloss.JoinHorizontal(lipgloss.Left, usedLegend, "  ", committedLegend)

	// Get the chart view
	chartView := chart.View()

	return lipgloss.JoinVertical(lipgloss.Left, legend, "", chartView)
}

func renderMemorySection(title string, used, committed, max int64, percentage float64, width int) string {
	// Determine color based on usage
	var color lipgloss.Color
	switch {
	case percentage > 0.9:
		color = tui.CriticalColor
	case percentage > 0.7:
		color = tui.WarningColor
	default:
		color = tui.GoodColor
	}

	// Create progress bar
	barWidth := width - 15
	if barWidth < 20 {
		barWidth = 20
	}
	progressBar := tui.CreateProgressBar(percentage, barWidth, color)
	percentStr := fmt.Sprintf("%.1f%%", percentage*100)

	// Build the section
	titleStyled := tui.InfoStyle.Render(title)
	progressLine := fmt.Sprintf("%s %s", progressBar, percentStr)
	detailLine := fmt.Sprintf("Used: %s | Committed: %s | Max: %s",
		utils.MemorySize(used), utils.MemorySize(committed), utils.MemorySize(max))

	section := lipgloss.JoinVertical(lipgloss.Left,
		titleStyled,
		progressLine,
		tui.MutedStyle.Render(detailLine),
		"", // Empty line for spacing
	)

	return section
}

func renderMemorySectionWithGC(title string, gcInfo string, used, committed, max int64, percentage float64, width int) string {
	// Determine color based on usage
	var color lipgloss.Color
	switch {
	case percentage > 0.9:
		color = tui.CriticalColor
	case percentage > 0.7:
		color = tui.WarningColor
	default:
		color = tui.GoodColor
	}

	// Create progress bar
	barWidth := width - 15
	if barWidth < 20 {
		barWidth = 20
	}
	progressBar := tui.CreateProgressBar(percentage, barWidth, color)
	percentStr := fmt.Sprintf("%.1f%%", percentage*100)

	// Build the section with GC info
	titleStyled := tui.InfoStyle.Render(title)
	gcInfoStyled := tui.MutedStyle.Render(fmt.Sprintf("(%s)", gcInfo))
	progressLine := fmt.Sprintf("%s %s", progressBar, percentStr)
	detailLine := fmt.Sprintf("Used: %s | Committed: %s | Max: %s",
		utils.MemorySize(used), utils.MemorySize(committed), utils.MemorySize(max))

	header := lipgloss.JoinHorizontal(lipgloss.Left, titleStyled, gcInfoStyled)
	section := lipgloss.JoinVertical(lipgloss.Left,
		header,
		progressLine,
		tui.MutedStyle.Render(detailLine),
		"", // Empty line for spacing
	)

	return section
}
