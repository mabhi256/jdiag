package monitor

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/mabhi256/jdiag/internal/tui"
	"github.com/mabhi256/jdiag/utils"
)

// Render renders the memory tab view
func RenderMemoryTab(state *TabState, width int) string {
	var sections []string

	// Memory pressure overview
	pressureOverview := renderMemoryPressureOverview(state.Memory)
	sections = append(sections, pressureOverview)

	// Heap Memory Section
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

// renderMemoryPressureOverview shows overall memory pressure status
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

// renderMemorySection renders a memory section with progress bar
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

// renderMemoryTrends shows memory allocation trends and patterns
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
