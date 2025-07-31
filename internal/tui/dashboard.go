package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mabhi256/jdiag/internal/gc"
)

func (m *Model) RenderDashboard() string {
	if m.metrics == nil {
		return "Loading dashboard..."
	}

	var headerContent []string

	// Add JVM info if available, otherwise add empty line to maintain height
	jvmInfo := ""
	if m.gcLog.JVMVersion != "" {
		jvmInfo = fmt.Sprintf("JVM: %s", m.gcLog.JVMVersion)
		if m.gcLog.HeapMax > 0 {
			jvmInfo += fmt.Sprintf("  Heap: %s", m.gcLog.HeapMax.String())
		}
		if !m.gcLog.StartTime.IsZero() && !m.gcLog.EndTime.IsZero() {
			runtime := m.gcLog.EndTime.Sub(m.gcLog.StartTime)
			jvmInfo += fmt.Sprintf("  Runtime: %s", FormatDuration(runtime))
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

	leftColumn := renderDashboardLeft(m.metrics, leftWidth)
	rightColumn := renderDashboardRight(m.metrics, m.issues, rightWidth)

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

func renderDashboardLeft(metrics *gc.GCMetrics, width int) string {
	sections := []string{
		renderPerformanceOverview(metrics),
		"", // spacing
		renderCollectionBreakdown(metrics, width),
	}

	return strings.Join(sections, "\n")
}

func renderDashboardRight(metrics *gc.GCMetrics, issues *gc.Analysis, width int) string {
	sections := []string{
		renderIssuesSummary(issues, width),
		"", // spacing
		renderMemoryPressure(metrics, width),
	}

	return strings.Join(sections, "\n")
}

func renderPerformanceOverview(metrics *gc.GCMetrics) string {
	title := TitleStyle.Render("Performance Overview")

	var rows []string

	// Throughput - only show if warning/critical
	throughputTarget := 95.0
	if metrics.Throughput < throughputTarget {
		status := "⚠️"
		if metrics.Throughput < 90.0 {
			status = "🔴"
		}
		throughputRow := fmt.Sprintf("%-15s %s %s",
			"Throughput",
			fmt.Sprintf("%.1f%%", metrics.Throughput),
			status)
		rows = append(rows, throughputRow)
	}

	// P99 Pause - only show if warning/critical
	p99Target := 200.0 // ms
	p99Ms := float64(metrics.P99Pause.Nanoseconds()) / 1000000
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
	avgMs := float64(metrics.AvgPause.Nanoseconds()) / 1000000
	avgRow := fmt.Sprintf("%-15s %-12s",
		"Avg Pause",
		fmt.Sprintf("%.1fms", avgMs))
	rows = append(rows, avgRow)

	// Allocation Rate - only show status if high
	allocRow := fmt.Sprintf("%-15s %s",
		"Allocation",
		fmt.Sprintf("%.0f MB/s", metrics.AllocationRate))
	if metrics.AllocationRate > 100 {
		status := "⚠️"
		if metrics.AllocationRate > 500 {
			status = "🔴"
		}
		allocRow += fmt.Sprintf(" %s", status)
	}
	rows = append(rows, allocRow)

	// Total Events and Runtime - simple display
	eventsRow := fmt.Sprintf("%-15s %-12s",
		"Total Events",
		fmt.Sprintf("%d", metrics.TotalEvents))
	rows = append(rows, eventsRow)

	if metrics.TotalRuntime > 0 {
		runtimeRow := fmt.Sprintf("%-15s %-12s",
			"Runtime",
			FormatDuration(metrics.TotalRuntime))
		rows = append(rows, runtimeRow)
	}

	// If no issues, show a simple "All Good" message
	if len(rows) == 3 || (len(rows) == 4 && metrics.TotalRuntime > 0) { // Only basic metrics shown
		if metrics.Throughput >= 95.0 && p99Ms <= 200.0 {
			rows = append([]string{"✅ Performance looks good"}, rows...)
		}
	}

	content := strings.Join(rows, "\n")

	return lipgloss.JoinVertical(lipgloss.Left,
		title,
		"",
		content,
	)
}

func renderCollectionBreakdown(metrics *gc.GCMetrics, width int) string {
	title := TitleStyle.Render("Collection Types")

	total := float64(metrics.TotalEvents)
	if total == 0 {
		return lipgloss.JoinVertical(lipgloss.Left, title, "No GC events")
	}

	barWidth := width - 20 // Reserve space for labels

	// Young GC percentage
	youngPct := float64(metrics.YoungGCCount) / total
	youngBar := CreateProgressBar(youngPct, barWidth, GoodColor)
	youngLine := fmt.Sprintf("Young  %s %d%%", youngBar, int(youngPct*100))

	// Mixed GC percentage
	mixedPct := float64(metrics.MixedGCCount) / total
	mixedBar := CreateProgressBar(mixedPct, barWidth, InfoColor)
	mixedLine := fmt.Sprintf("Mixed  %s %d%%", mixedBar, int(mixedPct*100))

	// Full GC percentage
	fullPct := float64(metrics.FullGCCount) / total
	fullColor := GoodColor
	if metrics.FullGCCount > 0 {
		fullColor = CriticalColor
	}
	fullBar := CreateProgressBar(fullPct, barWidth, fullColor)
	fullLine := fmt.Sprintf("Full   %s %d%%", fullBar, int(fullPct*100))

	content := strings.Join([]string{youngLine, mixedLine, fullLine}, "\n")

	return lipgloss.JoinVertical(lipgloss.Left,
		"",
		title,
		"",
		content,
	)
}

func renderIssuesSummary(issues *gc.Analysis, width int) string {
	title := TitleStyle.Render("Issues Summary")

	criticalCount := len(issues.Critical)
	warningCount := len(issues.Warning)
	infoCount := len(issues.Info)

	var lines []string
	lines = append(lines, "")

	// Issue counts
	if criticalCount > 0 {
		lines = append(lines, CriticalStyle.Render(fmt.Sprintf("Critical: %d", criticalCount)))
	}
	if warningCount > 0 {
		lines = append(lines, WarningStyle.Render(fmt.Sprintf("Warning: %d", warningCount)))
	}
	if infoCount > 0 {
		lines = append(lines, InfoStyle.Render(fmt.Sprintf("ℹ️  Info: %d", infoCount)))
	}

	if len(lines) == 0 {
		lines = append(lines, GoodStyle.Render("✅ No issues detected"))
	} else {
		lines = append(lines, "")

		// Show top issue
		topIssue := getTopIssue(issues)
		if topIssue != nil {
			lines = append(lines, MutedStyle.Render("Top Issue:"))

			issueTitle := TruncateString(topIssue.Type, width-2)
			lines = append(lines, issueTitle)
		}
	}

	content := strings.Join(lines, "\n")

	return lipgloss.JoinVertical(lipgloss.Left,
		title,
		content,
	)
}

func renderMemoryPressure(metrics *gc.GCMetrics, width int) string {
	title := TitleStyle.Render("Memory Pressure")

	var lines []string
	barWidth := width - 20

	// Heap Utilization
	heapTarget := 0.70 // 70%
	heapColor := GoodColor
	if metrics.AvgHeapUtil > 0.90 {
		heapColor = CriticalColor
	} else if metrics.AvgHeapUtil > heapTarget {
		heapColor = WarningColor
	}

	heapBar := CreateProgressBar(metrics.AvgHeapUtil, barWidth, heapColor)
	heapStatus := "✅"
	if metrics.AvgHeapUtil > 0.90 {
		heapStatus = "🔴"
	} else if metrics.AvgHeapUtil > heapTarget {
		heapStatus = "⚠️"
	}

	heapLine := fmt.Sprintf("Heap  %s %.0f%% %s",
		heapBar, metrics.AvgHeapUtil*100, heapStatus)
	lines = append(lines, heapLine)

	// Region Utilization (if available)
	if metrics.AvgRegionUtilization > 0 {
		regionTarget := 0.75 // 75%
		regionColor := GoodColor
		if metrics.AvgRegionUtilization > 0.85 {
			regionColor = CriticalColor
		} else if metrics.AvgRegionUtilization > regionTarget {
			regionColor = WarningColor
		}

		regionBar := CreateProgressBar(metrics.AvgRegionUtilization, barWidth, regionColor)
		regionStatus := "✅"
		if metrics.AvgRegionUtilization > 0.85 {
			regionStatus = "🔴"
		} else if metrics.AvgRegionUtilization > regionTarget {
			regionStatus = "⚠️"
		}

		regionLine := fmt.Sprintf("Regions  %s %.0f%% %s",
			regionBar, metrics.AvgRegionUtilization*100, regionStatus)
		lines = append(lines, regionLine)
	}

	// Allocation Rate indicator
	allocLine := fmt.Sprintf("Alloc Rate: %.0f MB/s", metrics.AllocationRate)
	lines = append(lines, "")
	lines = append(lines, allocLine)

	// Evacuation Failures
	if metrics.EvacuationFailureRate > 0 {
		evacLine := fmt.Sprintf("Evac Failures: %.1f%%", metrics.EvacuationFailureRate*100)
		if metrics.EvacuationFailureRate > 0.01 {
			evacLine = CriticalStyle.Render(evacLine)
		} else {
			evacLine = WarningStyle.Render(evacLine)
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
func getTopIssue(issues *gc.Analysis) *gc.PerformanceIssue {
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
