package gc

import (
	"fmt"
)

func GetRecommendations(analysis *GCAnalysis) *GCIssues {
	var issues []PerformanceIssue

	// ===== CRITICAL ISSUES =====
	if analysis.HasCriticalMemoryLeak {
		issues = append(issues, getCriticalMemoryLeakRec(analysis))
	}

	if analysis.HasCriticalEvacFailures {
		issues = append(issues, getCriticalEvacFailureRec(analysis))
	}

	if analysis.HasCriticalThroughput {
		issues = append(issues, getCriticalThroughputRec(analysis))
	}

	if analysis.HasCriticalPauseTimes {
		issues = append(issues, getCriticalPauseTimeRec(analysis))
	}

	if analysis.HasCriticalPromotion {
		issues = append(issues, getCriticalPromotionRec(analysis))
	}

	if analysis.HasCriticalHumongousLeak {
		issues = append(issues, getCriticalHumongousRec(analysis))
	}

	if analysis.HasCriticalConcurrentMarkAbort {
		issues = append(issues, getMarkAbortRec(analysis))
	}

	// Full GC is always critical
	if analysis.FullGCCount > 1 {
		issues = append(issues, getFullGCRec(analysis))
	}

	// ===== WARNING ISSUES =====
	if analysis.HasWarningMemoryLeak {
		issues = append(issues, getWarningMemoryLeakRec(analysis))
	}

	if analysis.HasWarningEvacFailures {
		issues = append(issues, getWarningEvacFailureRec(analysis))
	}

	if analysis.HasWarningThroughput {
		issues = append(issues, getWarningThroughputRec(analysis))
	}

	if analysis.HasWarningPauseTimes {
		issues = append(issues, getWarningPauseTimeRec(analysis))
	}

	if analysis.HasWarningPromotion {
		issues = append(issues, getWarningPromotionRec(analysis))
	}

	if analysis.HasWarningHumongousUsage {
		issues = append(issues, getWarningHumongousRec(analysis))
	}

	if analysis.HasWarningConcurrentMark {
		issues = append(issues, getConcurrentMarkingRec(analysis))
	}

	if analysis.HasWarningAllocationRate {
		issues = append(issues, getAllocationRateRec(analysis))
	}

	if analysis.HasWarningCollectionEff {
		issues = append(issues, getCollectionEfficiencyRec(analysis))
	}

	// ===== INFO ISSUES =====
	if analysis.HasInfoAllocationPattern {
		issues = append(issues, getAllocationPatternRec(analysis))
	}

	if analysis.HasInfoPhaseOptimization {
		issues = append(issues, getPhaseOptimizationRec(analysis))
	}

	return groupRecsBySeverity(issues)
}

// ===== CRITICAL RECOMMENDATION GENERATORS =====

func getCriticalMemoryLeakRec(analysis *GCAnalysis) PerformanceIssue {
	var description string
	var recommendations []string

	if analysis.FullGCCount >= 3 {
		description = fmt.Sprintf("SEVERE MEMORY LEAK: %d Full GCs + %.2f MB/hour growth",
			analysis.FullGCCount, analysis.MemoryTrend.GrowthRateMBPerHour)
	} else {
		description = fmt.Sprintf("CRITICAL MEMORY LEAK: %.2f MB/hour growth rate",
			analysis.MemoryTrend.GrowthRateMBPerHour)
	}

	recommendations = []string{
		"IMMEDIATE ACTION REQUIRED - Application will run out of memory",
		fmt.Sprintf("Projected heap exhaustion in: %v", analysis.MemoryTrend.ProjectedFullHeapTime),
		"Take heap dump immediately: jcmd <pid> VM.dump_heap critical-leak.hprof",
		"Restart application if possible to prevent OutOfMemoryError",
		"Increase heap size as emergency measure: -Xmx<current * 3>",
		"Enable OOM dumps: -XX:+HeapDumpOnOutOfMemoryError -XX:HeapDumpPath=/path/to/dumps",
		"Analyze heap dump with Eclipse MAT or VisualVM",
		"Look for: unclosed resources, static collections, event listeners, caches",
		"Check recent code changes for memory retention patterns",
	}

	if analysis.HumongousStats.IsLeak {
		recommendations = append(recommendations,
			fmt.Sprintf("Also investigate humongous objects: %d regions (%.1f%% of heap)",
				analysis.HumongousStats.MaxRegions, analysis.HumongousStats.HeapPercentage))
	}

	return PerformanceIssue{
		Type:           "Memory Leak",
		Severity:       "critical",
		Description:    description,
		Recommendation: recommendations,
	}
}

func getCriticalEvacFailureRec(analysis *GCAnalysis) PerformanceIssue {
	failureRate := analysis.EvacuationFailureRate * 100

	recommendations := []string{
		fmt.Sprintf("EVACUATION FAILURES: %d events (%.1f%% failure rate)",
			analysis.EvacuationFailureCount, failureRate),
		"Evacuation failure means G1GC couldn't move objects to other regions",
		"This causes severe performance degradation and triggers Full GC",
		"IMMEDIATE ACTION: Increase heap size by 100-200%: -Xmx<size * 2>",
		"Increase evacuation reserve: -XX:G1ReservePercent=20",
		"Check for humongous objects causing fragmentation",
		"Consider larger regions: -XX:G1HeapRegionSize=32m",
		"Take heap dump to analyze object distribution",
		"Monitor heap utilization - target <70% to prevent failures",
	}

	if analysis.AvgHeapUtil > 0.9 {
		recommendations = append(recommendations,
			fmt.Sprintf("Critical: Heap utilization %.1f%% - increase heap immediately",
				analysis.AvgHeapUtil*100))
	}

	return PerformanceIssue{
		Type:           "Critical Evacuation Failures",
		Severity:       "critical",
		Description:    fmt.Sprintf("%d evacuation failures (%.1f%% rate)", analysis.EvacuationFailureCount, failureRate),
		Recommendation: recommendations,
	}
}

func getCriticalThroughputRec(analysis *GCAnalysis) PerformanceIssue {
	recommendations := []string{
		fmt.Sprintf("Application throughput %.1f%% is critically low (target: >%.0f%%)",
			analysis.Throughput, ThroughputGood),
		"Primary action: Increase heap size to reduce GC frequency",
		fmt.Sprintf("Recommended heap size: %.0fGB (for allocation rate: %.1f MB/s)",
			calculateRecommendedHeapSize(analysis.AllocationRate), analysis.AllocationRate),
		"Consider G1GC tuning: -XX:G1HeapOccupancyPercent=35",
		"Monitor GC logs for evacuation failures and long pauses",
		"Profile application for allocation hotspots",
	}

	if analysis.FullGCCount > 0 {
		recommendations = append(recommendations,
			fmt.Sprintf("CRITICAL: %d Full GCs detected - heap size insufficient", analysis.FullGCCount))
	}

	return PerformanceIssue{
		Type:           "Critical Throughput Issues",
		Severity:       "critical",
		Description:    fmt.Sprintf("Throughput %.1f%% critically low", analysis.Throughput),
		Recommendation: recommendations,
	}
}

func getCriticalPauseTimeRec(analysis *GCAnalysis) PerformanceIssue {
	recommendations := []string{
		fmt.Sprintf("Maximum pause %v exceeds critical threshold (%v)",
			analysis.MaxPause, PauseCritical),
		"Pause times are unacceptable for most applications",
		fmt.Sprintf("Set pause target: -XX:MaxGCPauseMillis=%d",
			int(PauseAcceptable.Milliseconds())),
		"Reduce young generation: -XX:G1MaxNewSizePercent=20",
		"Increase heap size to reduce memory pressure",
		"Consider low-latency collectors: ZGC (-XX:+UseZGC) or Shenandoah",
		"Profile allocation patterns to reduce GC pressure",
	}

	if analysis.EvacuationFailureCount > 0 {
		recommendations = append(recommendations,
			"Note: Evacuation failures contributing to long pauses")
	}

	return PerformanceIssue{
		Type:           "Critical Pause Times",
		Severity:       "critical",
		Description:    fmt.Sprintf("Maximum pause %v exceeds critical threshold", analysis.MaxPause),
		Recommendation: recommendations,
	}
}

func getCriticalPromotionRec(analysis *GCAnalysis) PerformanceIssue {
	recommendations := []string{
		fmt.Sprintf("PREMATURE PROMOTION: Old gen growing %.1fx per young GC",
			analysis.AvgOldGrowthRatio),
		fmt.Sprintf("%.1f regions promoted per young collection", analysis.AvgPromotionRate),
		"Objects not dying in young generation as expected",
		"Increase young generation: -XX:G1NewSizePercent=40 -XX:G1MaxNewSizePercent=60",
		"Increase survivor space: -XX:SurvivorRatio=6",
		"Keep objects young longer: -XX:MaxTenuringThreshold=15",
		"Start concurrent marking earlier: -XX:G1HeapOccupancyPercent=25",
		"Profile object lifecycle: async-profiler -e alloc -d 60 <pid>",
		"Check for short-lived large objects or collections",
	}

	if analysis.SurvivorOverflowRate > SurvivorOverflowCritical {
		recommendations = append(recommendations,
			fmt.Sprintf("CRITICAL: %.1f%% survivor overflow - increase survivor space immediately",
				analysis.SurvivorOverflowRate*100))
	}

	return PerformanceIssue{
		Type:           "Critical Premature Promotion",
		Severity:       "critical",
		Description:    fmt.Sprintf("Old gen growing %.1fx per young GC", analysis.AvgOldGrowthRatio),
		Recommendation: recommendations,
	}
}

func getCriticalHumongousRec(analysis *GCAnalysis) PerformanceIssue {
	stats := analysis.HumongousStats

	recommendations := []string{
		fmt.Sprintf("HUMONGOUS OBJECT LEAK: %d regions (%.1f%% of heap)",
			stats.MaxRegions, stats.HeapPercentage),
		"Humongous objects are not being garbage collected",
		"This indicates a MEMORY LEAK in large object allocation",
		"Take heap dump: jcmd <pid> VM.dump_heap humongous-leak.hprof",
		"Analyze with Eclipse MAT for large objects and reference chains",
		"Look for: large arrays, huge strings, oversized collections, cached data",
		"Check recent code for large object allocations that aren't released",
		"Increase heap size by 200-300% as temporary relief",
		"Enable OOM dump: -XX:+HeapDumpOnOutOfMemoryError",
	}

	if stats.StaticCount > stats.GrowingCount {
		recommendations = append(recommendations,
			fmt.Sprintf("Static leak pattern: %d unchanged across GC cycles", stats.StaticCount))
	}

	if stats.GrowingCount > stats.DecreasingCount {
		recommendations = append(recommendations,
			fmt.Sprintf("Growing leak pattern: %d growth vs %d cleanup events",
				stats.GrowingCount, stats.DecreasingCount))
	}

	return PerformanceIssue{
		Type:           "Humongous Object Leak",
		Severity:       "critical",
		Description:    fmt.Sprintf("Humongous objects consuming %.1f%% of heap", stats.HeapPercentage),
		Recommendation: recommendations,
	}
}

func getMarkAbortRec(analysis *GCAnalysis) PerformanceIssue {
	recommendations := []string{
		fmt.Sprintf("CONCURRENT MARKING FAILURE: %d concurrent mark cycles aborted",
			analysis.ConcurrentMarkAbortCount),
		"Concurrent marking cannot keep up with allocation rate - forces Full GC",
		"IMMEDIATE ACTION: Double heap size: -Xmx<current * 2>",
		"Start marking earlier: -XX:G1HeapOccupancyPercent=15 (down from 45%)",
		"Increase concurrent threads: -XX:ConcGCThreads=8",
		fmt.Sprintf("Profile allocation hotspots: allocation rate %.1f MB/s needs optimization",
			analysis.AllocationRate),
		"Take heap dump to analyze object lifecycle patterns",
		"Increase young generation: -XX:G1NewSizePercent=40 -XX:G1MaxNewSizePercent=60",
		"Consider ZGC for large heaps: -XX:+UseZGC (avoids concurrent marking)",
	}

	if analysis.EvacuationFailureCount > 0 {
		recommendations = append(recommendations,
			"Note: Concurrent mark aborts are causing evacuation failures")
	}

	return PerformanceIssue{
		Type:           "Critical Concurrent Mark Abort",
		Severity:       "critical",
		Description:    fmt.Sprintf("%d concurrent mark cycles aborted", analysis.ConcurrentMarkAbortCount),
		Recommendation: recommendations,
	}
}

func getFullGCRec(analysis *GCAnalysis) PerformanceIssue {
	var severity string
	var recommendations []string

	if analysis.FullGCCount == 2 {
		severity = "warning"
		recommendations = []string{
			"Single Full GC detected - monitor for recurrence",
			"Check for heap sizing: increase -Xmx by 50-100% if possible",
			"Enable detailed logging: -XX:+PrintGCDetails -XX:+PrintGCTimeStamps",
			"Monitor for one-time events (class loading, permgen, etc.)",
			"Take heap dump if Full GC recurs",
		}
	} else {
		severity = "critical"
		recommendations = []string{
			fmt.Sprintf("%d Full GC events - G1GC performance severely degraded", analysis.FullGCCount),
			"Increase heap size by 100-200% immediately",
			"Check for memory leaks with heap dump analysis",
			"Consider application-level memory optimization",
			"Add monitoring: -XX:+HeapDumpOnOutOfMemoryError",
			"Profile allocation patterns and object lifecycle",
		}
	}

	return PerformanceIssue{
		Type:           "Full GC Events",
		Severity:       severity,
		Description:    fmt.Sprintf("%d Full GC events detected", analysis.FullGCCount),
		Recommendation: recommendations,
	}
}

// ===== WARNING RECOMMENDATION =====

func getWarningMemoryLeakRec(analysis *GCAnalysis) PerformanceIssue {
	recommendations := []string{
		fmt.Sprintf("Suspicious memory growth: %.2f MB/hour", analysis.MemoryTrend.GrowthRateMBPerHour),
		fmt.Sprintf("Trend confidence: %.1f%% over %v",
			analysis.MemoryTrend.TrendConfidence*100, analysis.MemoryTrend.SamplePeriod),
		"Take baseline heap dump for comparison",
		"Enable memory tracking: -XX:+PrintGCDetails -XX:+PrintGCApplicationStoppedTime",
		"Profile with async-profiler for allocation hotspots",
		"Review recent code changes for memory usage patterns",
		"Monitor heap utilization trends",
	}

	return PerformanceIssue{
		Type:           "Suspected Memory Leak",
		Severity:       "warning",
		Description:    fmt.Sprintf("Memory growing %.2f MB/hour", analysis.MemoryTrend.GrowthRateMBPerHour),
		Recommendation: recommendations,
	}
}

func getWarningEvacFailureRec(analysis *GCAnalysis) PerformanceIssue {
	failureRate := analysis.EvacuationFailureRate * 100

	recommendations := []string{
		fmt.Sprintf("Evacuation failures detected: %d events (%.1f%% rate)",
			analysis.EvacuationFailureCount, failureRate),
		"Monitor heap utilization - maintain <80% to prevent failures",
		"Consider increasing heap size by 50%",
		"Increase evacuation reserve: -XX:G1ReservePercent=15",
		"Check for allocation bursts causing temporary pressure",
		getRegionSizeRecommendation(analysis.AllocationRate),
	}

	return PerformanceIssue{
		Type:           "Evacuation Failures",
		Severity:       "warning",
		Description:    fmt.Sprintf("%d evacuation failures", analysis.EvacuationFailureCount),
		Recommendation: recommendations,
	}
}

func getWarningThroughputRec(analysis *GCAnalysis) PerformanceIssue {
	recommendations := []string{
		fmt.Sprintf("Throughput %.1f%% below optimal (target: >%.0f%%)",
			analysis.Throughput, ThroughputGood),
		"Fine-tune pause target: reduce -XX:MaxGCPauseMillis if currently >200ms",
		"Optimize young generation: -XX:G1MaxNewSizePercent=40",
		"Consider heap size increase for better performance",
		"Monitor allocation patterns for optimization opportunities",
	}

	return PerformanceIssue{
		Type:           "Suboptimal Throughput",
		Severity:       "warning",
		Description:    fmt.Sprintf("Throughput %.1f%% has room for improvement", analysis.Throughput),
		Recommendation: recommendations,
	}
}

func getWarningPauseTimeRec(analysis *GCAnalysis) PerformanceIssue {
	recommendations := []string{
		fmt.Sprintf("P99 pause %v exceeds target %v", analysis.P99Pause, analysis.EstimatedPauseTarget),
		fmt.Sprintf("%.1f%% of collections miss pause target", analysis.PauseTargetMissRate*100),
		"Pause time consistency needs improvement",
		fmt.Sprintf("Adjust pause target: -XX:MaxGCPauseMillis=%d",
			int(float64(analysis.EstimatedPauseTarget.Milliseconds())*1.2)),
		"Optimize concurrent marking: -XX:G1HeapOccupancyPercent=30",
		"Consider mixed collection tuning: -XX:G1MixedGCCountTarget=12",
	}

	return PerformanceIssue{
		Type:           "Pause Time Consistency",
		Severity:       "warning",
		Description:    fmt.Sprintf("P99 pause %v exceeds target", analysis.P99Pause),
		Recommendation: recommendations,
	}
}

func getWarningPromotionRec(analysis *GCAnalysis) PerformanceIssue {
	recommendations := []string{
		fmt.Sprintf("High promotion rate: %.1f regions per young GC", analysis.AvgPromotionRate),
		fmt.Sprintf("Old generation growing %.1fx on average", analysis.AvgOldGrowthRatio),
		"Objects not dying in young generation as expected",
		fmt.Sprintf("Young generation efficiency: %.1f%% (target: >80%%)",
			analysis.YoungCollectionEfficiency*100),
		"Increase young generation size to give objects more time to die",
		"Monitor allocation patterns for optimization opportunities",
		"Consider survivor space tuning if efficiency is low",
	}

	return PerformanceIssue{
		Type:           "Premature Promotion Warning",
		Severity:       "warning",
		Description:    fmt.Sprintf("High promotion: %.1f regions per young GC", analysis.AvgPromotionRate),
		Recommendation: recommendations,
	}
}

func getWarningHumongousRec(analysis *GCAnalysis) PerformanceIssue {
	stats := analysis.HumongousStats

	recommendations := []string{
		fmt.Sprintf("Significant humongous object usage: %d regions (%.1f%% of heap)",
			stats.MaxRegions, stats.HeapPercentage),
		"Large objects consuming significant heap space",
		"Monitor for memory leak patterns",
		"Consider object size optimization or heap size increase",
		"Review large object allocation patterns",
	}

	return PerformanceIssue{
		Type:           "High Humongous Object Usage",
		Severity:       "warning",
		Description:    fmt.Sprintf("Humongous objects: %.1f%% of heap", stats.HeapPercentage),
		Recommendation: recommendations,
	}
}

func getConcurrentMarkingRec(analysis *GCAnalysis) PerformanceIssue {
	recommendations := []string{
		fmt.Sprintf("Concurrent marking falling behind allocation rate (%.1f MB/s)",
			analysis.AllocationRate),
		"Start marking earlier: -XX:G1HeapOccupancyPercent=25",
		fmt.Sprintf("Increase concurrent threads: -XX:ConcGCThreads=%d",
			calculateOptimalConcThreads(analysis.AllocationRate)),
		"Increase heap size to provide more marking time",
		"Enable marking diagnostics: -XX:+TraceConcurrentGCollection",
	}

	return PerformanceIssue{
		Type:           "Concurrent Marking Issues",
		Severity:       "warning",
		Description:    "Concurrent marking cannot keep pace with allocation",
		Recommendation: recommendations,
	}
}

func getAllocationRateRec(analysis *GCAnalysis) PerformanceIssue {
	var severity string
	var recommendations []string

	if analysis.AllocationRate > AllocRateCritical {
		severity = "critical"
		recommendations = []string{
			fmt.Sprintf("Very high allocation rate: %.1f MB/s requires specialized tuning",
				analysis.AllocationRate),
			"Use large heap regions: -XX:G1HeapRegionSize=32m",
			"Increase young generation: -XX:G1NewSizePercent=30 -XX:G1MaxNewSizePercent=70",
			"Profile allocation hotspots with async-profiler",
			"Consider object pooling for high-frequency allocations",
		}
	} else {
		severity = "warning"
		recommendations = []string{
			fmt.Sprintf("High allocation rate: %.1f MB/s needs monitoring", analysis.AllocationRate),
			getRegionSizeRecommendation(analysis.AllocationRate),
			"Optimize young generation sizing for allocation pattern",
			"Review object lifecycle and temporary object creation",
		}
	}

	if analysis.AllocationBurstCount > analysis.TotalEvents/10 {
		recommendations = append(recommendations,
			fmt.Sprintf("Note: %d allocation bursts detected - consider batch processing optimization",
				analysis.AllocationBurstCount))
	}

	return PerformanceIssue{
		Type:           "High Allocation Rate",
		Severity:       severity,
		Description:    fmt.Sprintf("Allocation rate %.1f MB/s", analysis.AllocationRate),
		Recommendation: recommendations,
	}
}

func getCollectionEfficiencyRec(analysis *GCAnalysis) PerformanceIssue {
	recommendations := []string{
		fmt.Sprintf("No mixed collections in %d young collections", analysis.YoungGCCount),
		"G1GC is not performing mixed collections - old generation not being cleaned",
		"Lower marking threshold: -XX:G1HeapOccupancyPercent=35",
		"Adjust mixed collection targeting: -XX:G1MixedGCLiveThresholdPercent=75",
		"Verify concurrent marking completes successfully",
	}

	return PerformanceIssue{
		Type:           "Missing Mixed Collections",
		Severity:       "warning",
		Description:    "Old generation not being cleaned - no mixed collections",
		Recommendation: recommendations,
	}
}

// ===== INFO RECOMMENDATION GENERATORS =====

func getAllocationPatternRec(analysis *GCAnalysis) PerformanceIssue {
	recommendations := []string{
		fmt.Sprintf("Moderate allocation rate: %.1f MB/s is manageable", analysis.AllocationRate),
		"Current allocation rate is within normal range",
		"Monitor for allocation bursts or patterns",
		"Consider profiling if allocation rate increases",
	}

	return PerformanceIssue{
		Type:           "Allocation Pattern Analysis",
		Severity:       "info",
		Description:    fmt.Sprintf("Allocation rate %.1f MB/s", analysis.AllocationRate),
		Recommendation: recommendations,
	}
}

func getPhaseOptimizationRec(analysis *GCAnalysis) PerformanceIssue {
	phases := analysis.PhaseStats
	var recommendations []string

	if phases.AvgObjectCopyTime > ObjectCopyTarget {
		recommendations = append(recommendations,
			fmt.Sprintf("Object copy phase averaging %v (target: <%v) - reduce young gen size",
				phases.AvgObjectCopyTime, ObjectCopyTarget))
	}

	if phases.AvgRootScanTime > RootScanTarget {
		recommendations = append(recommendations,
			fmt.Sprintf("Root scanning phase averaging %v (target: <%v) - review JNI usage",
				phases.AvgRootScanTime, RootScanTarget))
	}

	if phases.AvgTerminationTime > TerminationTarget {
		recommendations = append(recommendations,
			fmt.Sprintf("Termination phase averaging %v (target: <%v) - adjust parallel threads",
				phases.AvgTerminationTime, TerminationTarget))
	}

	if phases.AvgRefProcessingTime > RefProcessingTarget {
		recommendations = append(recommendations,
			fmt.Sprintf("Reference processing averaging %v (target: <%v) - enable parallel processing",
				phases.AvgRefProcessingTime, RefProcessingTarget))
	}

	return PerformanceIssue{
		Type:           "GC Phase Optimization",
		Severity:       "info",
		Description:    "Some GC phases can be optimized for better performance",
		Recommendation: recommendations,
	}
}

// ===== HELPER FUNCTIONS =====

func calculateRecommendedHeapSize(allocRate float64) float64 {
	const HeapSizeAllocFactor = 0.6
	const MinRecommendedHeap = 4.0
	const MaxRecommendedHeap = 64.0

	baseGB := allocRate * HeapSizeAllocFactor
	if baseGB < MinRecommendedHeap {
		return MinRecommendedHeap
	}
	if baseGB > MaxRecommendedHeap {
		return MaxRecommendedHeap
	}
	return baseGB
}

func getRegionSizeRecommendation(allocRate float64) string {
	if allocRate > 1000 { // AllocRateCritical/2
		return "Use large regions: -XX:G1HeapRegionSize=32m"
	} else if allocRate > AllocRateHigh {
		return "Use medium regions: -XX:G1HeapRegionSize=16m"
	} else if allocRate < AllocRateModerate {
		return "Use small regions: -XX:G1HeapRegionSize=8m"
	}
	return "Keep default region size: -XX:G1HeapRegionSize=16m"
}

func calculateOptimalConcThreads(allocRate float64) int {
	if allocRate > 1000 { // HighAllocThreads threshold
		return 8
	} else if allocRate > AllocRateHigh {
		return 6
	}
	return 4
}

func groupRecsBySeverity(allIssues []PerformanceIssue) *GCIssues {
	var analysis GCIssues

	for _, issue := range allIssues {
		switch issue.Severity {
		case "critical":
			analysis.Critical = append(analysis.Critical, issue)
		case "warning":
			analysis.Warning = append(analysis.Warning, issue)
		default:
			analysis.Info = append(analysis.Info, issue)
		}
	}

	return &analysis
}

// Main entry point for the optimized analysis
func AnalyzeAndRecommend(events []*GCEvent, analysis *GCAnalysis) *GCIssues {
	AnalyzeGCLogs(events, analysis)

	return GetRecommendations(analysis)
}
