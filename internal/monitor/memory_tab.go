package monitor

import (
	"fmt"
	"time"

	"github.com/NimbleMarkets/ntcharts/linechart"
	"github.com/charmbracelet/lipgloss"
	"github.com/mabhi256/jdiag/internal/tui"
	"github.com/mabhi256/jdiag/utils"
)

func RenderMemoryTab(state *TabState, width int, heapHistory []utils.MultiValueTimePoint) string {
	var sections []string

	// Memory pressure overview
	pressureOverview := renderMemoryPressureOverview(state.Memory)
	sections = append(sections, pressureOverview)

	// ntcharts timeseries graph
	graphSection := renderEnhancedHeapMemorySection(state.Memory, width, heapHistory)
	sections = append(sections, graphSection)

	heapSection := renderMemorySection("Heap Memory",
		state.Memory.HeapUsed,
		state.Memory.HeapCommitted,
		state.Memory.HeapMax,
		state.Memory.HeapUsagePercent,
		width)

	sections = append(sections, heapSection)

	// Young Generation
	youngSection := renderMemorySection("Young Generation",
		state.Memory.YoungUsed,
		state.Memory.YoungCommitted,
		state.Memory.YoungMax,
		state.Memory.YoungUsagePercent,
		width)
	sections = append(sections, youngSection)

	// Old Generation
	oldSection := renderMemorySection("Old Generation",
		state.Memory.OldUsed,
		state.Memory.OldCommitted,
		state.Memory.OldMax,
		state.Memory.OldUsagePercent,
		width)
	sections = append(sections, oldSection)

	// Non-Heap Memory
	nonHeapSection := renderMemorySection("Non-Heap Memory",
		state.Memory.NonHeapUsed,
		state.Memory.NonHeapCommitted,
		state.Memory.NonHeapMax,
		state.Memory.NonHeapUsagePercent,
		width)
	sections = append(sections, nonHeapSection)

	// Memory allocation rate and trends
	if state.Memory.AllocationRate > 0 {
		trendsSection := renderMemoryTrends(state.Memory)
		sections = append(sections, trendsSection)
	}

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// renderEnhancedHeapMemorySection - UPDATED for multi-value points
func renderEnhancedHeapMemorySection(memory *MemoryState, width int, heapHistory []utils.MultiValueTimePoint) string {
	var sections []string

	// Title
	titleStyled := tui.InfoStyle.Render("Heap Memory (Used vs Committed)")
	sections = append(sections, titleStyled)

	// Multi-series timeseries graph
	if len(heapHistory) > 1 {
		graphSection := renderHeapMemoryMultiSeriesGraph(heapHistory, width)
		sections = append(sections, graphSection)
	} else {
		placeholderGraph := renderPlaceholderGraph(width)
		sections = append(sections, placeholderGraph)
	}

	return lipgloss.JoinVertical(lipgloss.Left, sections...) + "\n"
}

func renderHeapMemoryMultiSeriesGraph(history []utils.MultiValueTimePoint, width int) string {
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

	// Set Committed memory style (blue/info color)
	chart.SetDataSetStyle("committed", lipgloss.NewStyle().Foreground(tui.InfoColor))

	// Draw ALL datasets with braille
	chart.DrawBrailleAll()

	// Create legend
	usedLegend := lipgloss.NewStyle().Foreground(tui.GoodColor).Render("■ Used")
	committedLegend := lipgloss.NewStyle().Foreground(tui.InfoColor).Render("■ Committed")
	legend := lipgloss.JoinHorizontal(lipgloss.Left, usedLegend, "  ", committedLegend)

	// Get the chart view
	chartView := chart.View()

	return lipgloss.JoinVertical(lipgloss.Left, legend, "", chartView)
}

func HourTimeLabelFormatter() linechart.LabelFormatter {
	return func(i int, v float64) string {
		t := time.Unix(int64(v), 0).UTC()
		return t.Format("15:04:05")
	}
}

func renderPlaceholderGraph(width int) string {
	graphWidth := max(width-10, 40)

	// Create an empty chart for placeholder
	tslc := utils.NewChart(graphWidth, 6)

	// Add some placeholder points
	now := time.Now()
	for i := range 5 {
		tslc.Push(utils.TimePoint{
			Time:  now.Add(-time.Duration(i) * time.Minute),
			Value: 0,
		})
	}

	tslc.Draw()
	placeholder := tslc.View()

	// Style as muted/placeholder
	styledPlaceholder := tui.MutedStyle.Render(placeholder)
	message := tui.MutedStyle.Render("Collecting heap memory data (used & committed)...")

	return lipgloss.JoinVertical(lipgloss.Left, styledPlaceholder, message)
}

func renderMemoryPressureOverview(memory *MemoryState) string {
	pressureLevel := memory.GetMemoryPressureLevel()

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

	pressureText := fmt.Sprintf("%s Memory Pressure: %s", pressureIcon, pressureLevel)

	// Add memory trend if available
	trendText := ""
	if memory.MemoryTrend != 0 {
		trendIcon := "📈"
		if memory.MemoryTrend < 0 {
			trendIcon = "📉"
		}
		trendText = fmt.Sprintf(" %s %.1f%%/min", trendIcon, memory.MemoryTrend*100)
	}

	overview := lipgloss.NewStyle().
		Foreground(pressureColor).
		Bold(true).
		Render(pressureText + trendText)

	return overview + "\n"
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
	barWidth := width/2 - 10
	if barWidth < 20 {
		barWidth = 20
	}

	progressBar := tui.CreateProgressBar(percentage, barWidth, color)
	percentStr := fmt.Sprintf("%.1f%%", percentage*100)

	// Build the section
	titleStyled := tui.InfoStyle.Render(title)
	progressLine := fmt.Sprintf("%s %s", progressBar, percentStr)
	detailLine := fmt.Sprintf("Used: %s | Committed: %s | Max: %s", utils.MemorySize(used), utils.MemorySize(committed), utils.MemorySize(max))

	section := lipgloss.JoinVertical(lipgloss.Left,
		titleStyled,
		progressLine,
		tui.MutedStyle.Render(detailLine),
		"", // Empty line for spacing
	)

	return section
}

func renderMemoryTrends(memory *MemoryState) string {
	var trendsInfo []string

	// Allocation rate
	if memory.AllocationRate > 0 {
		allocationStr := fmt.Sprintf("Allocation Rate: %s/sec", utils.MemorySize(memory.AllocationRate))
		trendsInfo = append(trendsInfo, allocationStr)
	}

	// Memory trend
	if memory.MemoryTrend != 0 {
		trendStr := fmt.Sprintf("Memory Trend: %.2f%%/min", memory.MemoryTrend*100)
		if memory.MemoryTrend > 0 {
			trendStr += " ↗️"
		} else {
			trendStr += " ↘️"
		}
		trendsInfo = append(trendsInfo, trendStr)
	}

	if len(trendsInfo) == 0 {
		return ""
	}

	trendsSection := lipgloss.JoinVertical(lipgloss.Left,
		tui.InfoStyle.Render("Memory Trends"),
		tui.MutedStyle.Render(fmt.Sprintf("• %s", trendsInfo[0])),
	)

	if len(trendsInfo) > 1 {
		for _, info := range trendsInfo[1:] {
			trendsSection += "\n" + tui.MutedStyle.Render(fmt.Sprintf("• %s", info))
		}
	}

	return trendsSection + "\n"
}
