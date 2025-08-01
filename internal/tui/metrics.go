package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mabhi256/jdiag/internal/gc"
)

func (m *Model) RenderMetrics() string {
	tabs := renderMetricsSubTabs(m.metricsSubTab)
	content := renderMetricsContent(m.analysis, m.metricsSubTab)

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

	return lipgloss.JoinVertical(lipgloss.Left, tabs, "", content)
}

func renderMetricsSubTabs(currentSub MetricsSubTab) string {
	tabs := []string{"General", "Timing", "Memory", "G1GC", "Concurrent"}
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

func renderMetricsContent(analysis *gc.GCAnalysis, subTab MetricsSubTab) string {
	switch subTab {
	case GeneralMetrics:
		return renderGeneralMetrics(analysis)
	case TimingMetrics:
		return renderTimingMetrics(analysis)
	case MemoryMetrics:
		return renderMemoryMetrics(analysis)
	case G1GCMetrics:
		return renderG1GCAnalysis(analysis)
	case ConcurrentMetrics:
		return renderConcurrentMetrics(analysis)
	default:
		return "Unknown metrics tab"
	}
}

func renderSection(title string, lines []string) string {
	if len(lines) == 0 {
		return ""
	}

	// Calculate optimal width for alignment
	maxWidth := 0
	for _, line := range lines {
		if idx := strings.Index(line, ":"); idx != -1 {
			maxWidth = max(maxWidth, idx+1)
		}
	}

	// Format lines with consistent spacing
	var formatted []string
	for _, line := range lines {
		if idx := strings.Index(line, ":"); idx != -1 {
			label := line[:idx+1]
			rest := line[idx+1:]
			padding := maxWidth - len(label) + 2
			formatted = append(formatted, label+strings.Repeat(" ", padding)+strings.TrimSpace(rest))
		} else {
			formatted = append(formatted, line)
		}
	}

	return TitleStyle.Render(title) + "\n" + strings.Join(formatted, "\n")
}

func getStatusIndicator(current, warning, critical float64) string {
	if warning > critical {
		// Higher is better (e.g., throughput: warning=90%, critical=80%)
		if current <= critical {
			return CriticalStyle.Render("🔴 Critical")
		} else if current <= warning {
			return WarningStyle.Render("⚠️ Below Target")
		}
	} else {
		// Lower is better (e.g., pause time: warning=100ms, critical=500ms)
		if current >= critical {
			return CriticalStyle.Render("🔴 Critical")
		} else if current >= warning {
			return WarningStyle.Render("⚠️ High")
		}
	}
	return "" // Good - no indicator needed
}

func renderGeneralMetrics(analysis *gc.GCAnalysis) string {
	total := float64(analysis.TotalEvents)

	// Performance Section
	throughputStatus := getStatusIndicator(analysis.Throughput, gc.ThroughputPoor, gc.ThroughputCritical)
	throughputStr := fmt.Sprintf("• Throughput: %.1f%%", analysis.Throughput)
	if throughputStatus != "" {
		throughputStr += " " + throughputStatus
	}

	perf := []string{
		fmt.Sprintf("• Total Events: %d", analysis.TotalEvents),
		fmt.Sprintf("• Runtime: %s", FormatDuration(analysis.TotalRuntime)),
		throughputStr,
		fmt.Sprintf("• Total GC Time: %s", FormatDuration(analysis.TotalGCTime)),
	}

	// Collection Breakdown
	collection := []string{
		fmt.Sprintf("• Young GCs: %d (%.1f%%)", analysis.YoungGCCount, float64(analysis.YoungGCCount)/total*100),
		fmt.Sprintf("• Mixed GCs: %d (%.1f%%)", analysis.MixedGCCount, float64(analysis.MixedGCCount)/total*100),
	}
	if analysis.FullGCCount > 0 {
		collection = append(collection, fmt.Sprintf("• Full GCs: %d (%.1f%%) %s", analysis.FullGCCount, float64(analysis.FullGCCount)/total*100, CriticalStyle.Render("🔴 Critical")))
	} else {
		collection = append(collection, fmt.Sprintf("• Full GCs: %d (%.1f%%)", analysis.FullGCCount, float64(analysis.FullGCCount)/total*100))
	}

	// Allocation Statistics
	allocRateStatus := getStatusIndicator(analysis.AllocationRate, gc.AllocRateHigh, gc.AllocRateCritical)
	allocRateStr := fmt.Sprintf("• Allocation Rate: %.1f MB/s", analysis.AllocationRate)
	if allocRateStatus != "" {
		allocRateStr += " " + allocRateStatus
	}

	alloc := []string{
		allocRateStr,
		fmt.Sprintf("• Avg Heap Util: %.1f%%", analysis.AvgHeapUtil*100),
	}
	if analysis.AllocationBurstCount > 0 {
		alloc = append(alloc, fmt.Sprintf("• Allocation Bursts: %d", analysis.AllocationBurstCount))
	}

	return strings.Join([]string{
		renderSection("General Performance", perf),
		renderSection("Collection Breakdown", collection),
		renderSection("Allocation Statistics", alloc),
	}, "\n\n")
}

func renderTimingMetrics(metrics *gc.GCAnalysis) string {
	// Maximum pause with status
	maxPauseStatus := getStatusIndicator(float64(metrics.MaxPause.Milliseconds()), float64(gc.PausePoor.Milliseconds()), float64(gc.PauseCritical.Milliseconds()))
	maxPauseStr := fmt.Sprintf("• Maximum: %s", FormatDuration(metrics.MaxPause))
	if maxPauseStatus != "" {
		maxPauseStr += " " + maxPauseStatus
	}

	// P95 pause with status
	p95PauseStatus := getStatusIndicator(float64(metrics.P95Pause.Milliseconds()), float64(gc.PauseAcceptable.Milliseconds()), float64(gc.PausePoor.Milliseconds()))
	p95PauseStr := fmt.Sprintf("• P95: %s", FormatDuration(metrics.P95Pause))
	if p95PauseStatus != "" {
		p95PauseStr += " " + p95PauseStatus
	}

	// P99 pause with status
	p99PauseStatus := getStatusIndicator(float64(metrics.P99Pause.Milliseconds()), float64(gc.PausePoor.Milliseconds()), float64(gc.PauseCritical.Milliseconds()))
	p99PauseStr := fmt.Sprintf("• P99: %s", FormatDuration(metrics.P99Pause))
	if p99PauseStatus != "" {
		p99PauseStr += " " + p99PauseStatus
	}

	lines := []string{
		fmt.Sprintf("• Average: %s", FormatDuration(metrics.AvgPause)),
		fmt.Sprintf("• Minimum: %s", FormatDuration(metrics.MinPause)),
		maxPauseStr,
		p95PauseStr,
		p99PauseStr,
	}

	if metrics.PauseTargetMissRate > 0 {
		var status string
		if metrics.PauseTargetMissRate > 0.2 {
			status = CriticalStyle.Render("🔴 High")
		} else if metrics.PauseTargetMissRate > 0.1 {
			status = WarningStyle.Render("⚠️ Elevated")
		}

		targetMissStr := fmt.Sprintf("• Target Miss Rate: %.1f%%", metrics.PauseTargetMissRate*100)
		if status != "" {
			targetMissStr += " " + status
		}
		lines = append(lines, targetMissStr)
	}

	if metrics.LongPauseCount > 0 {
		lines = append(lines, fmt.Sprintf("• Long Pauses: %d %s", metrics.LongPauseCount, WarningStyle.Render("⚠️")))
	}

	if metrics.PauseTimeVariance > 0 {
		var status string
		if metrics.PauseTimeVariance > gc.PauseVarianceCritical {
			status = CriticalStyle.Render("🔴 Very High Variance")
		} else if metrics.PauseTimeVariance > gc.PauseVarianceWarning {
			status = WarningStyle.Render("⚠️ High Variance")
		}

		varianceStr := fmt.Sprintf("• Pause Variance: %.3f", metrics.PauseTimeVariance)
		if status != "" {
			varianceStr += " " + status
		}
		lines = append(lines, varianceStr)
	}

	return renderSection("Pause Time Statistics", lines)
}

func renderMemoryMetrics(metrics *gc.GCAnalysis) string {
	var sections []string

	// Memory Utilization
	heapUtilStatus := getStatusIndicator(metrics.AvgHeapUtil*100, gc.HeapUtilWarning*100, gc.HeapUtilCritical*100)
	heapUtilStr := fmt.Sprintf("• Avg Heap Util: %.1f%%", metrics.AvgHeapUtil*100)
	if heapUtilStatus != "" {
		heapUtilStr += " " + heapUtilStatus
	}

	mem := []string{heapUtilStr}

	if metrics.AvgRegionUtilization > 0 {
		regionUtilStatus := getStatusIndicator(metrics.AvgRegionUtilization*100, gc.RegionUtilWarning*100, gc.RegionUtilCritical*100)
		regionUtilStr := fmt.Sprintf("• Avg Region Util: %.1f%%", metrics.AvgRegionUtilization*100)
		if regionUtilStatus != "" {
			regionUtilStr += " " + regionUtilStatus
		}
		mem = append(mem, regionUtilStr)
	}
	sections = append(sections, renderSection("Memory Utilization", mem))

	// Allocation Patterns
	allocRateStatus := getStatusIndicator(metrics.AllocationRate, gc.AllocRateHigh, gc.AllocRateCritical)
	allocRateStr := fmt.Sprintf("• Allocation Rate: %.1f MB/s", metrics.AllocationRate)
	if allocRateStatus != "" {
		allocRateStr += " " + allocRateStatus
	}

	alloc := []string{allocRateStr}
	if metrics.AllocationBurstCount > 0 {
		alloc = append(alloc, fmt.Sprintf("• Allocation Bursts: %d", metrics.AllocationBurstCount))
	}
	sections = append(sections, renderSection("Allocation Patterns", alloc))

	// Promotion Statistics
	if metrics.AvgPromotionRate > 0 || metrics.MaxPromotionRate > 0 {
		var prom []string

		if metrics.AvgPromotionRate > 0 {
			avgPromStatus := getStatusIndicator(metrics.AvgPromotionRate, gc.PromotionRateWarning, gc.PromotionRateCritical)
			avgPromStr := fmt.Sprintf("• Avg Promotion: %.1f regions/GC", metrics.AvgPromotionRate)
			if avgPromStatus != "" {
				avgPromStr += " " + avgPromStatus
			}
			prom = append(prom, avgPromStr)
		}

		if metrics.MaxPromotionRate > 0 {
			maxPromStatus := getStatusIndicator(metrics.MaxPromotionRate, gc.PromotionRateWarning, gc.PromotionRateCritical)
			maxPromStr := fmt.Sprintf("• Max Promotion: %.1f regions/GC", metrics.MaxPromotionRate)
			if maxPromStatus != "" {
				maxPromStr += " " + maxPromStatus
			}
			prom = append(prom, maxPromStr)
		}

		if metrics.SurvivorOverflowRate > 0 {
			survivorStatus := getStatusIndicator(metrics.SurvivorOverflowRate*100, gc.SurvivorOverflowWarning*100, gc.SurvivorOverflowCritical*100)
			survivorStr := fmt.Sprintf("• Survivor Overflow: %.1f%%", metrics.SurvivorOverflowRate*100)
			if survivorStatus != "" {
				survivorStr += " " + survivorStatus
			}
			prom = append(prom, survivorStr)
		}

		if metrics.PromotionEfficiency > 0 {
			efficiencyStatus := getStatusIndicator(metrics.PromotionEfficiency*100, gc.PromotionEfficiencyWarning*100, gc.PromotionEfficiencyCritical*100)
			efficiencyStr := fmt.Sprintf("• Promotion Efficiency: %.1f%%", metrics.PromotionEfficiency*100)
			if efficiencyStatus != "" {
				efficiencyStr += " " + efficiencyStatus
			}
			prom = append(prom, efficiencyStr)
		}

		sections = append(sections, renderSection("Promotion Statistics", prom))
	}

	return strings.Join(sections, "\n\n")
}

func renderG1GCAnalysis(metrics *gc.GCAnalysis) string {
	var sections []string

	// Collection Efficiency
	var eff []string
	if metrics.YoungCollectionEfficiency > 0 {
		youngEffStatus := getStatusIndicator(metrics.YoungCollectionEfficiency*100, gc.YoungCollectionEffWarning*100, gc.YoungCollectionEff*100/2)
		youngEffStr := fmt.Sprintf("• Young GC Efficiency: %.1f%%", metrics.YoungCollectionEfficiency*100)
		if youngEffStatus != "" {
			youngEffStr += " " + youngEffStatus
		}
		eff = append(eff, youngEffStr)
	}

	if metrics.MixedCollectionEfficiency > 0 {
		mixedEffStatus := getStatusIndicator(metrics.MixedCollectionEfficiency*100, gc.MixedCollectionEffWarning*100, gc.MixedCollectionEff*100/2)
		mixedEffStr := fmt.Sprintf("• Mixed GC Efficiency: %.1f%%", metrics.MixedCollectionEfficiency*100)
		if mixedEffStatus != "" {
			mixedEffStr += " " + mixedEffStatus
		}
		eff = append(eff, mixedEffStr)
	}

	if metrics.MixedToYoungRatio > 0 {
		eff = append(eff, fmt.Sprintf("• Mixed to Young Ratio: %.2f", metrics.MixedToYoungRatio))
	}
	sections = append(sections, renderSection("Collection Efficiency", eff))

	// Region Statistics
	if metrics.AvgRegionUtilization > 0 || metrics.RegionExhaustionEvents > 0 {
		var region []string

		if metrics.AvgRegionUtilization > 0 {
			regionUtilStatus := getStatusIndicator(metrics.AvgRegionUtilization*100, gc.RegionUtilWarning*100, gc.RegionUtilCritical*100)
			regionUtilStr := fmt.Sprintf("• Avg Region Util: %.1f%%", metrics.AvgRegionUtilization*100)
			if regionUtilStatus != "" {
				regionUtilStr += " " + regionUtilStatus
			}
			region = append(region, regionUtilStr)
		}

		if metrics.RegionExhaustionEvents > 0 {
			region = append(region, fmt.Sprintf("• Region Exhaustion: %d %s", metrics.RegionExhaustionEvents, CriticalStyle.Render("🔴")))
		}
		sections = append(sections, renderSection("Region Statistics", region))
	}

	// Evacuation Statistics
	if metrics.EvacuationFailureRate > 0 {
		evacFailStatus := getStatusIndicator(metrics.EvacuationFailureRate*100, gc.EvacFailureRateWarning, gc.EvacFailureRateCritical)
		evacFailStr := fmt.Sprintf("• Evacuation Failures: %.2f%%", metrics.EvacuationFailureRate*100)
		if evacFailStatus != "" {
			evacFailStr += " " + evacFailStatus
		}

		evac := []string{evacFailStr}
		sections = append(sections, renderSection("Evacuation Statistics", evac))
	}

	return strings.Join(sections, "\n\n")
}

func renderConcurrentMetrics(metrics *gc.GCAnalysis) string {
	lines := []string{}

	if !metrics.ConcurrentMarkingKeepup {
		lines = append(lines, fmt.Sprintf("• Marking Keepup: %s", CriticalStyle.Render("🔴 Falling Behind")))
	}

	if metrics.ConcurrentCycleDuration > 0 {
		var status string
		if metrics.ConcurrentCycleDuration > gc.ConcurrentCycleCritical {
			status = CriticalStyle.Render("🔴 Too Long")
		} else if metrics.ConcurrentCycleDuration > gc.ConcurrentCycleWarning {
			status = WarningStyle.Render("⚠️ Long")
		}

		cycleDurationStr := fmt.Sprintf("• Cycle Duration: %s", FormatDuration(metrics.ConcurrentCycleDuration))
		if status != "" {
			cycleDurationStr += " " + status
		}
		lines = append(lines, cycleDurationStr)
	}

	if metrics.ConcurrentCycleFrequency > 0 {
		lines = append(lines, fmt.Sprintf("• Cycle Frequency: %.2f/hour", metrics.ConcurrentCycleFrequency))
	}

	if metrics.ConcurrentCycleFailures > 0 {
		lines = append(lines, fmt.Sprintf("• Cycle Failures: %d %s", metrics.ConcurrentCycleFailures, CriticalStyle.Render("🔴")))
	}

	// If no issues to show, show basic concurrent status
	if len(lines) == 0 {
		lines = append(lines, "• Concurrent marking: No issues detected")
	}

	return renderSection("Concurrent Marking", lines)
}
