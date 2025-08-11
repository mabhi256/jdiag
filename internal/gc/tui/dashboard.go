package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mabhi256/jdiag/internal/gc"
	"github.com/mabhi256/jdiag/utils"
)

func (m *Model) RenderDashboard() string {
	var headerContent []string

	// Add JVM info if available, otherwise add empty line to maintain height
	jvmInfo := ""
	if m.analysis.JVMVersion != "" {
		jvmInfo = fmt.Sprintf("JVM: %s", m.analysis.JVMVersion)
		if m.analysis.HeapMax > 0 {
			jvmInfo += fmt.Sprintf("  Heap: %s", m.analysis.HeapMax.String())
		}
		if !m.analysis.StartTime.IsZero() && !m.analysis.EndTime.IsZero() {
			runtime := m.analysis.EndTime.Sub(m.analysis.StartTime)
			jvmInfo += fmt.Sprintf("  Runtime: %s", utils.FormatDuration(runtime))
		}

		headerLine := lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Render(jvmInfo)
		headerContent = append(headerContent, headerLine)
	} else {
		// Add empty line to maintain consistent height when JVM info is not available
		headerContent = append(headerContent, "")
	}

	// Add spacing after JVM info section
	headerContent = append(headerContent, "")

	// Calculate layout - split into two columns
	leftWidth := m.width/2 - 2
	rightWidth := m.width - leftWidth - 6

	leftColumn := renderDashboardLeft(m.analysis, leftWidth)
	rightColumn := renderDashboardRight(m.analysis, m.issues, rightWidth)

	// Join columns horizontally
	columnsContent := lipgloss.JoinHorizontal(
		lipgloss.Top,
		leftColumn,
		"             ", // spacing
		rightColumn,
	)

	headerSection := strings.Join(headerContent, "\n")
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		headerSection,
		columnsContent,
	)

	return content
}

func renderDashboardLeft(analysis *gc.GCAnalysis, width int) string {
	sections := []string{
		renderPerformanceOverview(analysis),
		"", // spacing
		renderCollectionBreakdown(analysis, width),
	}

	return strings.Join(sections, "\n")
}

func renderDashboardRight(analysis *gc.GCAnalysis, issues *gc.GCIssues, width int) string {
	sections := []string{
		renderIssuesSummary(issues, width),
		"", // spacing
		renderMemoryPressure(analysis, width),
	}

	return strings.Join(sections, "\n")
}

func renderPerformanceOverview(analysis *gc.GCAnalysis) string {
	title := utils.TitleStyle.Render("Performance Overview")

	var rows []string

	// Throughput - only show if warning/critical
	throughputTarget := 95.0
	if analysis.Throughput < throughputTarget {
		status := "⚠️"
		if analysis.Throughput < 90.0 {
			status = "🔴"
		}
		throughputRow := fmt.Sprintf("%-15s %s %s",
			"Throughput",
			fmt.Sprintf("%.1f%%", analysis.Throughput),
			status)
		rows = append(rows, throughputRow)
	}

	// P99 Pause - only show if warning/critical
	p99Target := 200.0 // ms
	p99Ms := float64(analysis.P99Pause.Nanoseconds()) / 1000000
	if p99Ms > p99Target {
		status := "⚠️"
		if p99Ms > 500 {
			status = "🔴"
		}
		p99Row := fmt.Sprintf("%-15s %s %s",
			"P99 Pause",
			fmt.Sprintf("%.1fms", p99Ms),
			status)
		rows = append(rows, p99Row)
	}

	// Always show key metrics without status
	avgMs := float64(analysis.AvgPause.Nanoseconds()) / 1000000
	avgRow := fmt.Sprintf("%-15s %-12s",
		"Avg Pause",
		fmt.Sprintf("%.1fms", avgMs))
	rows = append(rows, avgRow)

	// Allocation Rate - only show status if high
	allocRow := fmt.Sprintf("%-15s %s",
		"Allocation",
		fmt.Sprintf("%.0f MB/s", analysis.AllocationRate))
	if analysis.AllocationRate > 100 {
		status := "⚠️"
		if analysis.AllocationRate > 500 {
			status = "🔴"
		}
		allocRow += fmt.Sprintf(" %s", status)
	}
	rows = append(rows, allocRow)

	// Total Events and Runtime - simple display
	eventsRow := fmt.Sprintf("%-15s %-12s",
		"Total Events",
		fmt.Sprintf("%d", analysis.TotalEvents))
	rows = append(rows, eventsRow)

	if analysis.TotalRuntime > 0 {
		runtimeRow := fmt.Sprintf("%-15s %-12s",
			"Runtime",
			utils.FormatDuration(analysis.TotalRuntime))
		rows = append(rows, runtimeRow)
	}

	if len(rows) == 4 {
		rows = append(rows, "")
	}
	if len(rows) == 3 {
		rows = append(rows, "", "")
	}

	content := strings.Join(rows, "\n")

	return lipgloss.JoinVertical(lipgloss.Left,
		title,
		"",
		content,
	)
}

func renderCollectionBreakdown(analysis *gc.GCAnalysis, width int) string {
	title := utils.TitleStyle.Render("Collection Types")

	total := float64(analysis.TotalEvents)
	if total == 0 {
		return lipgloss.JoinVertical(lipgloss.Left, title, "No GC events")
	}

	barWidth := width - 20 // Reserve space for labels

	// Young GC percentage
	youngPct := float64(analysis.YoungGCCount) / total
	youngBar := utils.CreateProgressBar(youngPct, barWidth, utils.GoodColor)
	youngLine := fmt.Sprintf("Young  %s %d%%", youngBar, int(youngPct*100))

	// Mixed GC percentage
	mixedPct := float64(analysis.MixedGCCount) / total
	mixedBar := utils.CreateProgressBar(mixedPct, barWidth, utils.InfoColor)
	mixedLine := fmt.Sprintf("Mixed  %s %d%%", mixedBar, int(mixedPct*100))

	// Full GC percentage
	fullPct := float64(analysis.FullGCCount) / total
	fullColor := utils.GoodColor
	if analysis.FullGCCount > 0 {
		fullColor = utils.CriticalColor
	}
	fullBar := utils.CreateProgressBar(fullPct, barWidth, fullColor)
	fullLine := fmt.Sprintf("Full   %s %d%%", fullBar, int(fullPct*100))

	content := strings.Join([]string{youngLine, mixedLine, fullLine}, "\n")

	return lipgloss.JoinVertical(lipgloss.Left,
		"",
		title,
		"",
		content,
	)
}

func renderIssuesSummary(issues *gc.GCIssues, width int) string {
	title := utils.TitleStyle.Render("Issues Summary")

	criticalCount := len(issues.Critical)
	warningCount := len(issues.Warning)
	infoCount := len(issues.Info)

	var lines []string
	lines = append(lines, "")

	// Issue counts
	if criticalCount > 0 {
		lines = append(lines, utils.CriticalStyle.Render(fmt.Sprintf("Critical: %d", criticalCount)))
	}
	if warningCount > 0 {
		lines = append(lines, utils.WarningStyle.Render(fmt.Sprintf("Warning: %d", warningCount)))
	}
	if infoCount > 0 {
		lines = append(lines, utils.InfoStyle.Render(fmt.Sprintf("ℹ️  Info: %d", infoCount)))
	}

	if len(lines) == 0 {
		lines = append(lines, utils.GoodStyle.Render("✅ No issues detected"))
	} else {
		lines = append(lines, "")

		// Show top issue
		topIssue := getTopIssue(issues)
		if topIssue != nil {
			lines = append(lines, utils.MutedStyle.Render("Top Issue:"))

			issueTitle := utils.TruncateString(topIssue.Type, width-2)
			lines = append(lines, issueTitle)
		}
	}

	content := strings.Join(lines, "\n")

	return lipgloss.JoinVertical(lipgloss.Left,
		title,
		content,
	)
}

func renderMemoryPressure(analysis *gc.GCAnalysis, width int) string {
	title := utils.TitleStyle.Render("Memory Pressure")

	var lines []string
	barWidth := width - 20

	// Heap Utilization
	heapTarget := 0.70 // 70%
	heapColor := utils.GoodColor
	if analysis.AvgHeapUtil > 0.90 {
		heapColor = utils.CriticalColor
	} else if analysis.AvgHeapUtil > heapTarget {
		heapColor = utils.WarningColor
	}

	heapBar := utils.CreateProgressBar(analysis.AvgHeapUtil, barWidth, heapColor)
	heapStatus := "✅"
	if analysis.AvgHeapUtil > 0.90 {
		heapStatus = "🔴"
	} else if analysis.AvgHeapUtil > heapTarget {
		heapStatus = "⚠️"
	}

	heapLine := fmt.Sprintf("Heap     %s %.0f%% %s",
		heapBar, analysis.AvgHeapUtil*100, heapStatus)
	lines = append(lines, heapLine)

	// Region Utilization (if available)
	if analysis.AvgRegionUtilization > 0 {
		regionTarget := 0.75 // 75%
		regionColor := utils.GoodColor
		if analysis.AvgRegionUtilization > 0.85 {
			regionColor = utils.CriticalColor
		} else if analysis.AvgRegionUtilization > regionTarget {
			regionColor = utils.WarningColor
		}

		regionBar := utils.CreateProgressBar(analysis.AvgRegionUtilization, barWidth, regionColor)
		regionStatus := "✅"
		if analysis.AvgRegionUtilization > 0.85 {
			regionStatus = "🔴"
		} else if analysis.AvgRegionUtilization > regionTarget {
			regionStatus = "⚠️"
		}

		regionLine := fmt.Sprintf("Regions  %s %.0f%% %s",
			regionBar, analysis.AvgRegionUtilization*100, regionStatus)
		lines = append(lines, regionLine)
	}

	// Allocation Rate indicator
	allocLine := fmt.Sprintf("Alloc Rate: %.0f MB/s", analysis.AllocationRate)
	lines = append(lines, "")
	lines = append(lines, allocLine)

	// Evacuation Failures
	if analysis.EvacuationFailureRate > 0 {
		evacLine := fmt.Sprintf("Evac Failures: %.1f%%", analysis.EvacuationFailureRate*100)
		if analysis.EvacuationFailureRate > 0.01 {
			evacLine = utils.CriticalStyle.Render(evacLine)
		} else {
			evacLine = utils.WarningStyle.Render(evacLine)
		}
		lines = append(lines, evacLine)
	}

	content := strings.Join(lines, "\n")

	return lipgloss.JoinVertical(lipgloss.Left,
		"",
		title,
		"",
		content,
	)
}

// Helper function to get the most severe issue
func getTopIssue(issues *gc.GCIssues) *gc.PerformanceIssue {
	// Priority: critical > warning > info
	if len(issues.Critical) > 0 {
		return &issues.Critical[0]
	}

	if len(issues.Warning) > 0 {
		return &issues.Warning[0]
	}

	if len(issues.Info) > 0 {
		return &issues.Info[0]
	}

	return nil
}
