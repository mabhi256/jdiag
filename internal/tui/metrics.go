package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mabhi256/jdiag/internal/gc"
)

func (m *Model) RenderMetrics() string {
	if m.metrics == nil {
		return "Loading metrics..."
	}

	tabs := renderMetricsSubTabs(m.metricsSubTab)
	content := renderMetricsContent(m.metrics, m.metricsSubTab)

	// Apply scrolling if needed
	contentLines := strings.Split(content, "\n")
	availableHeight := m.height - 4 // Account for tabs

	scrollY := m.scrollPositions[MetricsTab]
	if len(contentLines) > availableHeight {
		maxScrollY := len(contentLines) - availableHeight
		if scrollY > maxScrollY {
			scrollY = maxScrollY
		}
		if scrollY < 0 {
			scrollY = 0
		}

		start := scrollY
		end := min(start+availableHeight, len(contentLines))
		content = strings.Join(contentLines[start:end], "\n")
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		tabs,
		"",
		content,
	)
}

func renderMetricsSubTabs(currentSub MetricsSubTab) string {
	tabs := []string{
		"General", "Timing", "Memory", "G1GC", "Concurrent",
	}

	var rendered []string
	for i, tab := range tabs {
		style := TabInactiveStyle
		if MetricsSubTab(i) == currentSub {
			style = TabActiveStyle
		}
		rendered = append(rendered, style.Render(tab))
	}

	return strings.Join(rendered, "  ")
}

func renderMetricsContent(metrics *gc.GCMetrics, subTab MetricsSubTab) string {
	switch subTab {
	case GeneralMetrics:
		return renderGeneralMetrics(metrics)
	case TimingMetrics:
		return renderTimingMetrics(metrics)
	case MemoryMetrics:
		return renderMemoryMetrics(metrics)
	case G1GCMetrics:
		return renderG1GCMetrics(metrics)
	case ConcurrentMetrics:
		return renderConcurrentMetrics(metrics)
	default:
		return "Unknown metrics tab"
	}
}

func renderGeneralMetrics(metrics *gc.GCMetrics) string {
	var sections []string

	// Performance Section
	sections = append(sections, TitleStyle.Render("General Performance"))

	lines := []string{
		fmt.Sprintf("• Total Events:        %d", metrics.TotalEvents),
		fmt.Sprintf("• Runtime:            %s", FormatDuration(metrics.TotalRuntime)),
		formatMetricWithStatus("• Throughput:", fmt.Sprintf("%.1f%%", metrics.Throughput),
			metrics.Throughput, 95.0, "higher"),
		fmt.Sprintf("• Total GC Time:      %s", FormatDuration(metrics.TotalGCTime)),
	}
	sections = append(sections, strings.Join(lines, "\n"))

	// Collection Breakdown
	sections = append(sections, "")
	sections = append(sections, TitleStyle.Render("Collection Breakdown"))

	total := float64(metrics.TotalEvents)
	collectionLines := []string{
		fmt.Sprintf("• Young GCs:          %d (%.1f%%)",
			metrics.YoungGCCount, float64(metrics.YoungGCCount)/total*100),
		fmt.Sprintf("• Mixed GCs:          %d (%.1f%%)",
			metrics.MixedGCCount, float64(metrics.MixedGCCount)/total*100),
	}

	if metrics.FullGCCount > 0 {
		fullLine := fmt.Sprintf("• Full GCs:           %d (%.1f%%) %s",
			metrics.FullGCCount, float64(metrics.FullGCCount)/total*100,
			CriticalStyle.Render("🔴 Critical"))
		collectionLines = append(collectionLines, fullLine)
	} else {
		collectionLines = append(collectionLines, fmt.Sprintf("• Full GCs:           %d %s",
			metrics.FullGCCount, GoodStyle.Render("✅ Good")))
	}

	sections = append(sections, strings.Join(collectionLines, "\n"))

	// Allocation Statistics
	sections = append(sections, "")
	sections = append(sections, TitleStyle.Render("Allocation Statistics"))

	allocLines := []string{
		formatMetricWithStatus("• Allocation Rate:", fmt.Sprintf("%.1f MB/s", metrics.AllocationRate),
			metrics.AllocationRate, 100.0, "lower"),
		fmt.Sprintf("• Avg Heap Util:      %.1f%%", metrics.AvgHeapUtil*100),
	}

	if metrics.AllocationBurstCount > 0 {
		allocLines = append(allocLines,
			fmt.Sprintf("• Allocation Bursts:  %d", metrics.AllocationBurstCount))
	}

	sections = append(sections, strings.Join(allocLines, "\n"))

	return strings.Join(sections, "\n")
}

func renderTimingMetrics(metrics *gc.GCMetrics) string {
	var sections []string

	// Pause Time Statistics
	sections = append(sections, TitleStyle.Render("Pause Time Statistics"))

	pauseLines := []string{
		fmt.Sprintf("• Average:            %s", FormatDuration(metrics.AvgPause)),
		fmt.Sprintf("• Minimum:            %s", FormatDuration(metrics.MinPause)),
		formatMetricWithStatus("• Maximum:", FormatDuration(metrics.MaxPause),
			float64(metrics.MaxPause.Milliseconds()), 200.0, "lower"),
		formatMetricWithStatus("• P95:", FormatDuration(metrics.P95Pause),
			float64(metrics.P95Pause.Milliseconds()), 100.0, "lower"),
		formatMetricWithStatus("• P99:", FormatDuration(metrics.P99Pause),
			float64(metrics.P99Pause.Milliseconds()), 200.0, "lower"),
	}

	if metrics.PauseTargetMissRate > 0 {
		missStatus := "✅ Good"
		if metrics.PauseTargetMissRate > 0.2 {
			missStatus = CriticalStyle.Render("🔴 High")
		} else if metrics.PauseTargetMissRate > 0.1 {
			missStatus = WarningStyle.Render("⚠️ Elevated")
		}

		pauseLines = append(pauseLines,
			fmt.Sprintf("• Target Miss Rate:   %.1f%% %s",
				metrics.PauseTargetMissRate*100, missStatus))
	}

	if metrics.LongPauseCount > 0 {
		pauseLines = append(pauseLines,
			fmt.Sprintf("• Long Pauses:        %d %s",
				metrics.LongPauseCount, WarningStyle.Render("⚠️")))
	}

	if metrics.PauseTimeVariance > 0 {
		varianceStatus := "✅ Consistent"
		if metrics.PauseTimeVariance > 0.5 {
			varianceStatus = WarningStyle.Render("⚠️ High Variance")
		}
		pauseLines = append(pauseLines,
			fmt.Sprintf("• Pause Variance:     %.3f %s",
				metrics.PauseTimeVariance, varianceStatus))
	}

	sections = append(sections, strings.Join(pauseLines, "\n"))

	return strings.Join(sections, "\n")
}

func renderMemoryMetrics(metrics *gc.GCMetrics) string {
	var sections []string

	// Memory Utilization
	sections = append(sections, TitleStyle.Render("Memory Utilization"))

	memLines := []string{
		formatMetricWithStatus("• Avg Heap Util:", fmt.Sprintf("%.1f%%", metrics.AvgHeapUtil*100),
			metrics.AvgHeapUtil*100, 70.0, "lower"),
	}

	if metrics.AvgRegionUtilization > 0 {
		memLines = append(memLines,
			formatMetricWithStatus("• Avg Region Util:", fmt.Sprintf("%.1f%%", metrics.AvgRegionUtilization*100),
				metrics.AvgRegionUtilization*100, 75.0, "lower"))
	}

	sections = append(sections, strings.Join(memLines, "\n"))

	// Allocation Patterns
	sections = append(sections, "")
	sections = append(sections, TitleStyle.Render("Allocation Patterns"))

	allocLines := []string{
		formatMetricWithStatus("• Allocation Rate:", fmt.Sprintf("%.1f MB/s", metrics.AllocationRate),
			metrics.AllocationRate, 100.0, "lower"),
	}

	if metrics.AllocationBurstCount > 0 {
		allocLines = append(allocLines,
			fmt.Sprintf("• Allocation Bursts:  %d", metrics.AllocationBurstCount))
	}

	sections = append(sections, strings.Join(allocLines, "\n"))

	// Promotion Statistics
	if metrics.AvgPromotionRate > 0 || metrics.MaxPromotionRate > 0 {
		sections = append(sections, "")
		sections = append(sections, TitleStyle.Render("Promotion Statistics"))

		promLines := []string{}

		if metrics.AvgPromotionRate > 0 {
			promLines = append(promLines,
				formatMetricWithStatus("• Avg Promotion:", fmt.Sprintf("%.1f regions/GC", metrics.AvgPromotionRate),
					metrics.AvgPromotionRate, 5.0, "lower"))
		}

		if metrics.MaxPromotionRate > 0 {
			promLines = append(promLines,
				formatMetricWithStatus("• Max Promotion:", fmt.Sprintf("%.1f regions/GC", metrics.MaxPromotionRate),
					metrics.MaxPromotionRate, 10.0, "lower"))
		}

		if metrics.SurvivorOverflowRate > 0 {
			promLines = append(promLines,
				formatMetricWithStatus("• Survivor Overflow:", fmt.Sprintf("%.1f%%", metrics.SurvivorOverflowRate*100),
					metrics.SurvivorOverflowRate*100, 10.0, "lower"))
		}

		if metrics.PromotionEfficiency > 0 {
			promLines = append(promLines,
				formatMetricWithStatus("• Promotion Efficiency:", fmt.Sprintf("%.1f%%", metrics.PromotionEfficiency*100),
					metrics.PromotionEfficiency*100, 50.0, "higher"))
		}

		sections = append(sections, strings.Join(promLines, "\n"))
	}

	return strings.Join(sections, "\n")
}

func renderG1GCMetrics(metrics *gc.GCMetrics) string {
	var sections []string

	// Collection Efficiency
	sections = append(sections, TitleStyle.Render("Collection Efficiency"))

	effLines := []string{}

	if metrics.YoungCollectionEfficiency > 0 {
		effLines = append(effLines,
			formatMetricWithStatus("• Young GC Efficiency:", fmt.Sprintf("%.1f%%", metrics.YoungCollectionEfficiency*100),
				metrics.YoungCollectionEfficiency*100, 80.0, "higher"))
	}

	if metrics.MixedCollectionEfficiency > 0 {
		effLines = append(effLines,
			formatMetricWithStatus("• Mixed GC Efficiency:", fmt.Sprintf("%.1f%%", metrics.MixedCollectionEfficiency*100),
				metrics.MixedCollectionEfficiency*100, 40.0, "higher"))
	}

	if metrics.MixedToYoungRatio > 0 {
		effLines = append(effLines,
			fmt.Sprintf("• Mixed to Young Ratio: %.2f", metrics.MixedToYoungRatio))
	}

	sections = append(sections, strings.Join(effLines, "\n"))

	// Region Statistics
	if metrics.AvgRegionUtilization > 0 || metrics.RegionExhaustionEvents > 0 {
		sections = append(sections, "")
		sections = append(sections, TitleStyle.Render("Region Statistics"))

		regionLines := []string{}

		if metrics.AvgRegionUtilization > 0 {
			regionLines = append(regionLines,
				formatMetricWithStatus("• Avg Region Util:", fmt.Sprintf("%.1f%%", metrics.AvgRegionUtilization*100),
					metrics.AvgRegionUtilization*100, 75.0, "lower"))
		}

		if metrics.RegionExhaustionEvents > 0 {
			regionLines = append(regionLines,
				fmt.Sprintf("• Region Exhaustion:  %d %s",
					metrics.RegionExhaustionEvents, CriticalStyle.Render("🔴")))
		}

		sections = append(sections, strings.Join(regionLines, "\n"))
	}

	// Evacuation Statistics
	if metrics.EvacuationFailureRate > 0 {
		sections = append(sections, "")
		sections = append(sections, TitleStyle.Render("Evacuation Statistics"))

		evacLines := []string{
			formatMetricWithStatus("• Evacuation Failures:", fmt.Sprintf("%.2f%%", metrics.EvacuationFailureRate*100),
				metrics.EvacuationFailureRate*100, 1.0, "lower"),
		}

		sections = append(sections, strings.Join(evacLines, "\n"))
	}

	return strings.Join(sections, "\n")
}

func renderConcurrentMetrics(metrics *gc.GCMetrics) string {
	var sections []string

	// Concurrent Marking
	sections = append(sections, TitleStyle.Render("Concurrent Marking"))

	concLines := []string{}

	keepupStatus := GoodStyle.Render("✅ Keeping Up")
	if !metrics.ConcurrentMarkingKeepup {
		keepupStatus = CriticalStyle.Render("🔴 Falling Behind")
	}
	concLines = append(concLines, fmt.Sprintf("• Marking Keepup:     %s", keepupStatus))

	if metrics.ConcurrentCycleDuration > 0 {
		cycleStatus := "✅ Normal"
		cycleSecs := metrics.ConcurrentCycleDuration.Seconds()
		if cycleSecs > 60 {
			cycleStatus = CriticalStyle.Render("🔴 Too Long")
		} else if cycleSecs > 30 {
			cycleStatus = WarningStyle.Render("⚠️ Long")
		}

		concLines = append(concLines,
			fmt.Sprintf("• Cycle Duration:     %s %s",
				FormatDuration(metrics.ConcurrentCycleDuration), cycleStatus))
	}

	if metrics.ConcurrentCycleFrequency > 0 {
		concLines = append(concLines,
			fmt.Sprintf("• Cycle Frequency:    %.2f/hour", metrics.ConcurrentCycleFrequency))
	}

	if metrics.ConcurrentCycleFailures > 0 {
		concLines = append(concLines,
			fmt.Sprintf("• Cycle Failures:     %d %s",
				metrics.ConcurrentCycleFailures, CriticalStyle.Render("🔴")))
	}

	sections = append(sections, strings.Join(concLines, "\n"))

	return strings.Join(sections, "\n")
}

// Helper function to format a metric with status indicator
func formatMetricWithStatus(label, value string, current, target float64, better string) string {
	var status string

	if better == "higher" {
		if current >= target {
			status = GoodStyle.Render("✅ Good")
		} else if current >= target*0.8 {
			status = WarningStyle.Render("⚠️ Below Target")
		} else {
			status = CriticalStyle.Render("🔴 Poor")
		}
	} else { // lower is better
		if current <= target {
			status = GoodStyle.Render("✅ Good")
		} else if current <= target*1.5 {
			status = WarningStyle.Render("⚠️ High")
		} else {
			status = CriticalStyle.Render("🔴 Critical")
		}
	}

	// Calculate padding for alignment
	labelPadding := max(22-len(label), 1)

	return fmt.Sprintf("%s%s%s %s",
		label,
		strings.Repeat(" ", labelPadding),
		value,
		status)
}
