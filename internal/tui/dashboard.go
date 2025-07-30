package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mabhi256/jdiag/internal/gc"
)

// RenderDashboard renders the dashboard view
func RenderDashboard(gcLog *gc.GCLog, metrics *gc.GCMetrics, issues *gc.Analysis, width, height int) string {
	if metrics == nil {
		return "Loading dashboard..."
	}

	jvmInfo := ""
	if gcLog != nil {
		jvmInfo = fmt.Sprintf("JVM: %s", gcLog.JVMVersion)
		if gcLog.HeapMax > 0 {
			jvmInfo += fmt.Sprintf("  Heap: %s", gcLog.HeapMax.String())
		}
		if !gcLog.StartTime.IsZero() && !gcLog.EndTime.IsZero() {
			runtime := gcLog.EndTime.Sub(gcLog.StartTime)
			jvmInfo += fmt.Sprintf("  Runtime: %s", FormatDuration(runtime))
		}
	}

	headerLine := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Render(jvmInfo)

	// Calculate layout - split into two columns
	leftWidth := width/2 - 2
	rightWidth := width - leftWidth - 4

	leftColumn := renderDashboardLeft(metrics, leftWidth)
	rightColumn := renderDashboardRight(metrics, issues, rightWidth)

	// Join columns horizontally
	columnsContent := lipgloss.JoinHorizontal(
		lipgloss.Top,
		leftColumn,
		"  ", // spacing
		rightColumn,
	)

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		headerLine,
		columnsContent,
	)

	return content
}

func renderDashboardLeft(metrics *gc.GCMetrics, width int) string {
	sections := []string{
		renderPerformanceOverview(metrics, width),
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

func renderPerformanceOverview(metrics *gc.GCMetrics, width int) string {
	title := TitleStyle.Render("Performance Overview")

	var lines []string

	// Throughput
	throughputTarget := 95.0
	throughputStatus := "✅"
	if metrics.Throughput < 90.0 {
		throughputStatus = "🔴"
	} else if metrics.Throughput < throughputTarget {
		throughputStatus = "⚠️"
	}

	throughputLine := fmt.Sprintf("• Throughput: %.1f%% (target >%.0f%%) %s",
		metrics.Throughput, throughputTarget, throughputStatus)
	lines = append(lines, throughputLine)

	// P99 Pause
	p99Target := 200.0 // ms
	p99Status := "✅"
	p99Ms := float64(metrics.P99Pause.Nanoseconds()) / 1000000
	if p99Ms > 500 {
		p99Status = "🔴"
	} else if p99Ms > p99Target {
		p99Status = "⚠️"
	}

	p99Line := fmt.Sprintf("• P99 Pause: %.1fms (target <%.0fms) %s",
		p99Ms, p99Target, p99Status)
	lines = append(lines, p99Line)

	// Average Pause
	avgMs := float64(metrics.AvgPause.Nanoseconds()) / 1000000
	avgLine := fmt.Sprintf("• Avg Pause: %.1fms", avgMs)
	lines = append(lines, avgLine)

	// Allocation Rate
	allocStatus := "ℹ️"
	allocDescription := "normal"
	if metrics.AllocationRate > 500 {
		allocStatus = "🔴"
		allocDescription = "very high"
	} else if metrics.AllocationRate > 100 {
		allocStatus = "⚠️"
		allocDescription = "high"
	}

	allocLine := fmt.Sprintf("• Allocation: %.0f MB/s (%s) %s",
		metrics.AllocationRate, allocDescription, allocStatus)
	lines = append(lines, allocLine)

	// Total Events
	eventsLine := fmt.Sprintf("• Total Events: %d", metrics.TotalEvents)
	lines = append(lines, eventsLine)

	// Runtime
	if metrics.TotalRuntime > 0 {
		runtimeLine := fmt.Sprintf("• Runtime: %s", FormatDuration(metrics.TotalRuntime))
		lines = append(lines, runtimeLine)
	}

	content := strings.Join(lines, "\n")

	return lipgloss.JoinVertical(lipgloss.Left,
		title,
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
	youngLine := fmt.Sprintf("Young    %s %d%%", youngBar, int(youngPct*100))

	// Mixed GC percentage
	mixedPct := float64(metrics.MixedGCCount) / total
	mixedBar := CreateProgressBar(mixedPct, barWidth, InfoColor)
	mixedLine := fmt.Sprintf("Mixed    %s %d%%", mixedBar, int(mixedPct*100))

	// Full GC percentage
	fullPct := float64(metrics.FullGCCount) / total
	fullColor := GoodColor
	if metrics.FullGCCount > 0 {
		fullColor = CriticalColor
	}
	fullBar := CreateProgressBar(fullPct, barWidth, fullColor)
	fullLine := fmt.Sprintf("Full     %s %d%%", fullBar, int(fullPct*100))

	content := strings.Join([]string{youngLine, mixedLine, fullLine}, "\n")

	return lipgloss.JoinVertical(lipgloss.Left,
		title,
		content,
	)
}

func renderIssuesSummary(issues *gc.Analysis, width int) string {
	title := TitleStyle.Render("Issues Summary")

	criticalCount := len(issues.Critical)
	warningCount := len(issues.Warning)
	infoCount := len(issues.Info)

	var lines []string

	// Issue counts
	if criticalCount > 0 {
		lines = append(lines, CriticalStyle.Render(fmt.Sprintf("🔴 Critical: %d", criticalCount)))
	}
	if warningCount > 0 {
		lines = append(lines, WarningStyle.Render(fmt.Sprintf("⚠️  Warning: %d", warningCount)))
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
			severity := GetSeverityIcon(topIssue.Severity)
			lines = append(lines, issueTitle)
			lines = append(lines, severity)
			lines = append(lines, "")
			lines = append(lines, MutedStyle.Render("→ View Details [Tab 3]"))
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

	heapLine := fmt.Sprintf("Heap     %s %.0f%% %s",
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
		title,
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
