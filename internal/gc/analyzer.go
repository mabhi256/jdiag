package gc

import (
	"fmt"
	"math"
	"strings"
	"time"
)

const (
	// Core performance targets
	ThroughputExcellent = 99.0
	ThroughputGood      = 95.0
	ThroughputPoor      = 90.0
	ThroughputCritical  = 80.0

	// Pause time targets
	PauseExcellent     = 10 * time.Millisecond
	PauseGood          = 50 * time.Millisecond
	PauseAcceptable    = 100 * time.Millisecond
	PausePoor          = 200 * time.Millisecond
	PauseCritical      = 500 * time.Millisecond
	PauseTargetDefault = 200 * time.Millisecond

	// Memory pressure
	HeapUtilGood     = 0.7  // 70% - healthy utilization
	HeapUtilWarning  = 0.8  // 80% - start monitoring
	HeapUtilCritical = 0.9  // 90% - immediate action needed
	RegionUtilMax    = 0.75 // 75% - optimal for G1 region management

	// Allocation thresholds
	AllocRateModerate = 100.0  // MB/s - typical web app
	AllocRateHigh     = 500.0  // MB/s - high-throughput app
	AllocRateCritical = 2000.0 // MB/s - requires special tuning

	// Collection efficiency targets
	YoungCollectionEff = 0.8  // 80% of young gen should be collected
	MixedCollectionEff = 0.4  // 40% of selected old regions
	EvacFailureMax     = 0.01 // 1% max evacuation failure rate

	// Timing thresholds for detailed phases
	ObjectCopyTarget    = 30 * time.Millisecond
	RootScanTarget      = 20 * time.Millisecond
	TerminationTarget   = 5 * time.Millisecond
	RefProcessingTarget = 15 * time.Millisecond

	// Leak detection thresholds
	LeakGrowthCritical = 5.0 // MB/hour - definitely a leak
	LeakGrowthWarning  = 1.0 // MB/hour - suspicious growth
	LeakGrowthMinimal  = 0.1 // MB/hour - normal application growth

	// Minimum data points for reliable trend analysis
	MinEventsForTrend = 20
	MinTimeForTrend   = 30 * time.Minute

	// Memory pressure indicators
	BaselineGrowthCritical = 0.05 // 5% per hour of total heap
	BaselineGrowthWarning  = 0.02 // 2% per hour of total heap

	// Premature promotion thresholds
	PromotionRateCritical    = 15.0 // regions per young GC
	PromotionRateWarning     = 8.0  // regions per young GC
	OldRegionGrowthCritical  = 3.0  // 3x growth per young GC is severe
	OldRegionGrowthWarning   = 1.5  // 1.5x growth is concerning
	SurvivorOverflowCritical = 0.3  // 30% of young collections overflow survivors
)

type MemoryTrend struct {
	GrowthRateMBPerHour   float64       // Raw memory growth rate
	GrowthRatePercent     float64       // Growth as % of heap per hour
	BaselineGrowthRate    float64       // Growth of post-GC baseline
	TrendConfidence       float64       // R-squared correlation (0-1)
	ProjectedFullHeapTime time.Duration // Time until heap exhaustion
	LeakSeverity          string        // none, warning, critical
	SamplePeriod          time.Duration // Duration of analysis
	EventCount            int           // Number of GC events analyzed
}

func (log *GCLog) Analyze(outputFormat string) {
	metrics := log.CalculateMetrics()
	issues := metrics.DetectPerformanceIssues(log.Events)
	log.Status = log.AssessGCHealth(metrics, issues)
	log.PrintReport(metrics, issues, outputFormat)
}

func (metrics *GCMetrics) DetectPerformanceIssues(events []GCEvent) []PerformanceIssue {
	var issues []PerformanceIssue

	issues = append(issues, analyzeMemoryLeaks(metrics, events)...)
	issues = append(issues, analyzeFullGCProblems(metrics)...)
	issues = append(issues, analyzeThroughputAndPauses(metrics, events)...)
	issues = append(issues, analyzeMemoryPressure(metrics)...)
	issues = append(issues, analyzeAllocationPatterns(metrics)...)
	issues = append(issues, analyzeConcurrentMarking(metrics)...)
	issues = append(issues, analyzePrematurePromotion(metrics, events)...)
	issues = append(issues, analyzeCollectionEfficiency(metrics, events)...)
	issues = append(issues, analyzeDetailedPhases(events)...)

	return issues
}

func analyzeFullGCProblems(metrics *GCMetrics) []PerformanceIssue {
	var issues []PerformanceIssue

	if metrics.FullGCCount == 0 {
		return issues
	}

	// Calculate rate if we have runtime data
	// var rateInfo string
	// if metrics.TotalRuntime > time.Hour {
	// 	rate := float64(metrics.FullGCCount) / metrics.TotalRuntime.Hours()
	// 	rateInfo = fmt.Sprintf(" (%.1f/hour)", rate)
	// }

	severity := "critical"
	var recommendations []string

	if metrics.FullGCCount == 1 {
		recommendations = []string{
			"Check for heap sizing: increase -Xmx by 50-100%% if possible",
			"Enable detailed logging: -XX:+PrintGCDetails -XX:+PrintGCTimeStamps",
			"Monitor for one-time events (class loading, permgen, etc.)",
		}
	} else {
		recommendations = []string{
			"URGENT: Increase heap size by 100-200%% immediately",
			"Check for memory leaks with heap dump analysis",
			"Consider application-level memory optimization",
			"Add monitoring: -XX:+HeapDumpOnOutOfMemoryError -XX:HeapDumpPath=/path/to/dumps",
		}
	}

	issues = append(issues, PerformanceIssue{
		Type:           "Full GC Events",
		Severity:       severity,
		Description:    fmt.Sprintf("%d Full GC events detected - G1GC performance severely degraded", metrics.FullGCCount),
		Recommendation: recommendations,
	})

	return issues
}

func analyzeThroughputAndPauses(metrics *GCMetrics, events []GCEvent) []PerformanceIssue {
	var issues []PerformanceIssue

	if metrics.Throughput < ThroughputGood {
		severity := "warning"
		if metrics.Throughput < ThroughputCritical {
			severity = "critical"
		}

		var recommendations []string
		if metrics.Throughput < ThroughputPoor {
			recommendations = []string{
				fmt.Sprintf("Throughput %.1f%% is below acceptable threshold (target: >%.0f%%)", metrics.Throughput, ThroughputGood),
				"Primary action: Increase heap size (-Xmx) to reduce GC frequency",
				fmt.Sprintf("Target heap size: %.0fGB (current allocation rate: %.1f MB/s)",
					calculateRecommendedHeapSize(metrics.AllocationRate), metrics.AllocationRate),
				"Consider G1GC tuning: -XX:G1HeapOccupancyPercent=35 (start marking earlier)",
			}
		} else {
			recommendations = []string{
				fmt.Sprintf("Throughput %.1f%% has room for improvement", metrics.Throughput),
				"Fine-tune pause target: reduce -XX:MaxGCPauseMillis if currently >200ms",
				"Optimize young generation: -XX:G1MaxNewSizePercent=40",
			}
		}

		issues = append(issues, PerformanceIssue{
			Type:           "Throughput Issues",
			Severity:       severity,
			Description:    fmt.Sprintf("Application throughput %.1f%% (target: >%.0f%%)", metrics.Throughput, ThroughputGood),
			Recommendation: recommendations,
		})
	}

	pauseIssue := analyzePausePerformance(metrics, events)
	if pauseIssue.Type != "" {
		issues = append(issues, pauseIssue)
	}

	return issues
}

func analyzePausePerformance(metrics *GCMetrics, events []GCEvent) PerformanceIssue {
	if len(events) < 5 {
		return PerformanceIssue{}
	}

	// Determine pause target
	pauseTarget := estimatePauseTarget(events)
	if pauseTarget == 0 {
		pauseTarget = PauseTargetDefault
	}

	var severity string
	var recommendations []string
	var description string

	// Prioritize the most severe pause issue
	if metrics.MaxPause > PauseCritical {
		severity = "critical"
		description = fmt.Sprintf("Maximum pause %v exceeds critical threshold (%v)", metrics.MaxPause, PauseCritical)
		recommendations = []string{
			"CRITICAL: Pause times are unacceptable for most applications",
			fmt.Sprintf("Immediate action: Set -XX:MaxGCPauseMillis=%d", int(PauseAcceptable.Milliseconds())),
			"Reduce young generation: -XX:G1MaxNewSizePercent=20",
			"Consider low-latency collectors: ZGC (-XX:+UseZGC) or Shenandoah",
		}
	} else if metrics.P99Pause > pauseTarget {
		severity = "warning"
		description = fmt.Sprintf("P99 pause %v exceeds target %v, %.1f%% of collections miss target",
			metrics.P99Pause, pauseTarget, metrics.PauseTargetMissRate*100)
		recommendations = []string{
			"Pause time consistency needs improvement",
			fmt.Sprintf("Adjust pause target: -XX:MaxGCPauseMillis=%d", int(float64(pauseTarget.Milliseconds())*1.2)),
			"Optimize concurrent marking: -XX:G1HeapOccupancyPercent=30",
			"Consider mixed collection tuning: -XX:G1MixedGCCountTarget=12",
		}
	} else if metrics.MaxPause > PauseGood {
		severity = "info"
		description = fmt.Sprintf("Maximum pause %v is acceptable but could be improved", metrics.MaxPause)
		recommendations = []string{
			"Pause times are acceptable but have optimization potential",
			"Fine-tune for better consistency: monitor allocation rate patterns",
			"Consider incremental improvements to pause target",
		}
	}

	if severity != "" {
		return PerformanceIssue{
			Type:           "Pause Time Performance",
			Severity:       severity,
			Description:    description,
			Recommendation: recommendations,
		}
	}

	return PerformanceIssue{}
}

func analyzeMemoryPressure(metrics *GCMetrics) []PerformanceIssue {
	var issues []PerformanceIssue

	heapPressure := metrics.AvgHeapUtil
	regionPressure := metrics.AvgRegionUtilization
	evacFailures := metrics.EvacuationFailureRate

	if heapPressure > HeapUtilWarning || regionPressure > RegionUtilMax || evacFailures > 0 {
		severity := "warning"
		if heapPressure > HeapUtilCritical || evacFailures > EvacFailureMax*2 {
			severity = "critical"
		}

		var recommendations []string
		if evacFailures > 0 {
			recommendations = []string{
				fmt.Sprintf("Memory pressure critical: %.1f%% heap, %.1f%% region utilization, %.1f%% evacuation failures",
					heapPressure*100, regionPressure*100, evacFailures*100),
				"URGENT: Increase heap size by at least 50%",
				"Increase evacuation reserve: -XX:G1ReservePercent=15",
				fmt.Sprintf("Optimize region size for allocation rate %.1f MB/s:", metrics.AllocationRate),
				getRegionSizeRecommendation(metrics.AllocationRate),
			}
		} else {
			recommendations = []string{
				fmt.Sprintf("Memory pressure detected: %.1f%% heap utilization (target: <%.0f%%)",
					heapPressure*100, HeapUtilWarning*100),
				"Increase heap size to reduce memory pressure",
				"Target heap utilization: 60-70% for optimal G1 performance",
				"Monitor allocation patterns for optimization opportunities",
			}
		}

		issues = append(issues, PerformanceIssue{
			Type:           "Memory Pressure",
			Severity:       severity,
			Description:    "High memory utilization affecting GC performance",
			Recommendation: recommendations,
		})
	}

	return issues
}

func analyzeAllocationPatterns(metrics *GCMetrics) []PerformanceIssue {
	var issues []PerformanceIssue

	if metrics.AllocationRate > AllocRateModerate {
		severity := "info"
		if metrics.AllocationRate > AllocRateHigh {
			severity = "warning"
		}
		if metrics.AllocationRate > AllocRateCritical {
			severity = "critical"
		}

		var recommendations []string
		if metrics.AllocationRate > AllocRateCritical {
			recommendations = []string{
				fmt.Sprintf("Very high allocation rate: %.1f MB/s requires specialized tuning", metrics.AllocationRate),
				"Use large heap regions: -XX:G1HeapRegionSize=32m",
				"Increase young generation: -XX:G1NewSizePercent=30 -XX:G1MaxNewSizePercent=70",
				"Profile allocation hotspots with async-profiler",
				"Consider object pooling for high-frequency allocations",
			}
		} else if metrics.AllocationRate > AllocRateHigh {
			recommendations = []string{
				fmt.Sprintf("High allocation rate: %.1f MB/s needs monitoring", metrics.AllocationRate),
				getRegionSizeRecommendation(metrics.AllocationRate),
				"Optimize young generation sizing for allocation pattern",
				"Review object lifecycle and temporary object creation",
			}
		} else {
			recommendations = []string{
				fmt.Sprintf("Moderate allocation rate: %.1f MB/s is typical", metrics.AllocationRate),
				"Current allocation rate is manageable",
				"Monitor for allocation bursts or patterns",
			}
		}

		// Add allocation burst information if relevant
		if metrics.AllocationBurstCount > metrics.TotalEvents/10 {
			recommendations = append(recommendations,
				fmt.Sprintf("Note: %d allocation bursts detected - consider batch processing optimization", metrics.AllocationBurstCount))
		}

		issues = append(issues, PerformanceIssue{
			Type:           "Allocation Patterns",
			Severity:       severity,
			Description:    fmt.Sprintf("Allocation rate %.1f MB/s", metrics.AllocationRate),
			Recommendation: recommendations,
		})
	}

	return issues
}

func analyzeConcurrentMarking(metrics *GCMetrics) []PerformanceIssue {
	var issues []PerformanceIssue

	if !metrics.ConcurrentMarkingKeepup {
		issues = append(issues, PerformanceIssue{
			Type:     "Concurrent Marking Issues",
			Severity: "critical",
			Description: fmt.Sprintf("Concurrent marking falling behind allocation rate (%.1f MB/s)",
				metrics.AllocationRate),
			Recommendation: []string{
				"Concurrent marking cannot keep pace with allocation",
				"Start marking earlier: -XX:G1HeapOccupancyPercent=25",
				fmt.Sprintf("Increase concurrent threads: -XX:ConcGCThreads=%d",
					calculateOptimalConcThreads(metrics.AllocationRate)),
				"Increase heap size to provide more marking time",
				"Enable marking diagnostics: -XX:+TraceConcurrentGCollection",
			},
		})
	}

	if metrics.ConcurrentCycleDuration > 30*time.Second {
		issues = append(issues, PerformanceIssue{
			Type:     "Long Concurrent Cycles",
			Severity: "warning",
			Description: fmt.Sprintf("Concurrent cycles taking %v (target: <30s)",
				metrics.ConcurrentCycleDuration),
			Recommendation: []string{
				"Concurrent marking cycles are too long",
				"Optimize concurrent thread count for workload",
				"Check for memory pressure causing slow progress",
				"Consider incremental marking improvements",
			},
		})
	}

	return issues
}

func analyzeCollectionEfficiency(metrics *GCMetrics, events []GCEvent) []PerformanceIssue {
	var issues []PerformanceIssue

	mixedCollections := countMixedCollections(events)
	youngCollections := countYoungCollections(events)

	// Check for missing mixed collections
	if mixedCollections == 0 && youngCollections > 50 {
		issues = append(issues, PerformanceIssue{
			Type:     "Missing Mixed Collections",
			Severity: "warning",
			Description: fmt.Sprintf("No mixed collections in %d young collections - old generation not being cleaned",
				youngCollections),
			Recommendation: []string{
				"G1GC is not performing mixed collections",
				"Lower marking threshold: -XX:G1HeapOccupancyPercent=35",
				"Adjust mixed collection targeting: -XX:G1MixedGCLiveThresholdPercent=75",
				"Verify concurrent marking completes successfully",
			},
		})
	}

	// Check mixed collection efficiency
	if metrics.MixedCollectionEfficiency > 0 && metrics.MixedCollectionEfficiency < MixedCollectionEff {
		issues = append(issues, PerformanceIssue{
			Type:     "Inefficient Mixed Collections",
			Severity: "warning",
			Description: fmt.Sprintf("Mixed collections only %.1f%% efficient (target: >%.0f%%)",
				metrics.MixedCollectionEfficiency*100, MixedCollectionEff*100),
			Recommendation: []string{
				"Mixed collections not reclaiming enough old generation space",
				"Increase regions per collection: -XX:G1OldCSetRegionThreshold=15",
				"Spread work over more collections: -XX:G1MixedGCCountTarget=12",
				"More aggressive cleanup: -XX:G1MixedGCLiveThresholdPercent=65",
			},
		})
	}

	return issues
}

func calculateRecommendedHeapSize(allocRate float64) float64 {
	// Rule of thumb: heap should handle 10-15 minutes of allocation
	baseGB := allocRate * 0.6 // 10 minutes of allocation in GB
	if baseGB < 4 {
		return 4 // Minimum 4GB
	}
	if baseGB > 64 {
		return 64 // Practical maximum for most applications
	}
	return baseGB
}

func getRegionSizeRecommendation(allocRate float64) string {
	if allocRate > 1000 {
		return "Use large regions: -XX:G1HeapRegionSize=32m"
	} else if allocRate > 500 {
		return "Use medium regions: -XX:G1HeapRegionSize=16m"
	} else if allocRate < 100 {
		return "Use small regions: -XX:G1HeapRegionSize=8m"
	}
	return "Keep default region size: -XX:G1HeapRegionSize=16m"
}

func calculateOptimalConcThreads(allocRate float64) int {
	if allocRate > 1000 {
		return 8
	} else if allocRate > 500 {
		return 6
	}
	return 4
}

func analyzeDetailedPhases(events []GCEvent) []PerformanceIssue {
	var issues []PerformanceIssue

	if len(events) < 10 {
		return issues
	}

	phases := map[string]struct {
		extractor func(GCEvent) time.Duration
		target    time.Duration
		issue     string
		fix       string
	}{
		"Object Copy": {
			func(e GCEvent) time.Duration { return e.ObjectCopyTime },
			ObjectCopyTarget,
			"Slow object evacuation during GC",
			"Reduce young gen size: -XX:G1MaxNewSizePercent=30",
		},
		"Root Scanning": {
			func(e GCEvent) time.Duration { return e.ExtRootScanTime },
			RootScanTarget,
			"Slow root scanning phase",
			"Review JNI usage and thread count",
		},
		"Termination": {
			func(e GCEvent) time.Duration { return e.TerminationTime },
			TerminationTarget,
			"Work imbalance between GC threads",
			"Adjust parallel threads: -XX:ParallelGCThreads=<cores*0.75>",
		},
		"Reference Processing": {
			func(e GCEvent) time.Duration { return e.ReferenceProcessingTime },
			RefProcessingTarget,
			"Slow reference processing",
			"Enable parallel ref processing: -XX:+ParallelRefProcEnabled",
		},
	}

	for phaseName, phase := range phases {
		avgTime := calculateAveragePhaseTime(events, phase.extractor)
		if avgTime > phase.target {
			issues = append(issues, PerformanceIssue{
				Type:        fmt.Sprintf("Inefficient %s Phase", phaseName),
				Severity:    "info",
				Description: fmt.Sprintf("%s averaging %v (target: <%v)", phaseName, avgTime.Round(time.Millisecond), phase.target),
				Recommendation: []string{
					phase.issue,
					phase.fix,
				},
			})
		}
	}

	return issues
}

func (log *GCLog) AssessGCHealth(metrics *GCMetrics, issues []PerformanceIssue) string {
	if metrics == nil {
		return "❓ Unknown"
	}

	criticalCount := 0
	warningCount := 0

	for _, issue := range issues {
		switch issue.Severity {
		case "critical":
			criticalCount++
		case "warning":
			warningCount++
		}
	}

	if criticalCount > 0 {
		return "🔴 Critical"
	}

	if metrics.FullGCCount > 0 {
		return "⚠️  Poor"
	}

	if warningCount > 2 {
		return "⚠️  Fair"
	}

	if warningCount > 0 {
		return "⚠️  Good"
	}

	if metrics.Throughput > ThroughputExcellent && metrics.P99Pause < PauseExcellent {
		return "✅ Excellent"
	}

	return "✅ Good"
}

func countMixedCollections(events []GCEvent) int {
	count := 0
	for _, event := range events {
		if strings.Contains(event.Type, "Mixed") {
			count++
		}
	}
	return count
}

func countYoungCollections(events []GCEvent) int {
	count := 0
	for _, event := range events {
		if strings.Contains(event.Type, "Young") {
			count++
		}
	}
	return count
}

func estimatePauseTarget(events []GCEvent) time.Duration {
	// First try to extract from JVM flags if we had that data
	// (this would require parsing the GC log header or command line)

	// For now, use default
	return PauseTargetDefault

	// // Calculate actual pause distribution to determine appropriate target
	// var durations []time.Duration
	// for _, event := range events {
	// 	durations = append(durations, event.Duration)
	// }

	// slices.Sort(durations)
	// p95 := durations[int(float64(len(durations))*0.95)]

	// // Determine target based on application profile:
	// switch {
	// case p95 < 5*time.Millisecond:
	// 	// Ultra-low latency app - target very aggressive
	// 	return 10 * time.Millisecond

	// case p95 < 50*time.Millisecond:
	// 	// Low latency app (web services, trading) - target reasonable headroom
	// 	return 50 * time.Millisecond

	// case p95 < 200*time.Millisecond:
	// 	// Standard app - target acceptable for most use cases
	// 	return 200 * time.Millisecond

	// default:
	// 	// High latency tolerance - focus on throughput
	// 	return 500 * time.Millisecond
	// }
}

func calculateAveragePhaseTime(events []GCEvent, timeExtractor func(GCEvent) time.Duration) time.Duration {
	var total time.Duration
	count := 0

	for _, event := range events {
		phaseTime := timeExtractor(event)
		if phaseTime > 0 {
			total += phaseTime
			count++
		}
	}

	if count == 0 {
		return 0
	}
	return total / time.Duration(count)
}

func analyzeMemoryLeaks(metrics *GCMetrics, events []GCEvent) []PerformanceIssue {
	var issues []PerformanceIssue

	trend := detectMemoryLeak(events)
	if trend.LeakSeverity == "none" {
		return issues
	}

	severity := trend.LeakSeverity
	var recommendations []string

	switch severity {
	case "critical":
		recommendations = []string{
			fmt.Sprintf("MEMORY LEAK DETECTED: %.1f MB/hour growth rate", trend.GrowthRateMBPerHour),
			fmt.Sprintf("Baseline memory growing %.1f%% per hour", trend.GrowthRatePercent*100),
			fmt.Sprintf("Heap will be exhausted in approximately %v", trend.ProjectedFullHeapTime),
			"IMMEDIATE ACTION REQUIRED:",
			"1. Take heap dump for analysis: jcmd <pid> GC.run_finalization; jcmd <pid> VM.gc; jcmd <pid> VM.dump_heap heap.hprof",
			"2. Enable heap dump on OOM: -XX:+HeapDumpOnOutOfMemoryError",
			"3. Analyze with Eclipse MAT or VisualVM",
			"4. Look for: unclosed resources, static collections, event listeners",
		}

	case "warning":
		recommendations = []string{
			fmt.Sprintf("Suspicious memory growth: %.1f MB/hour (%.1f%% of heap)",
				trend.GrowthRateMBPerHour, trend.GrowthRatePercent*100),
			fmt.Sprintf("Trend confidence: %.1f%% over %v", trend.TrendConfidence*100, trend.SamplePeriod),
			"Monitor and investigate:",
			"1. Take baseline heap dump for comparison",
			"2. Enable memory tracking: -XX:+PrintGCDetails -XX:+PrintGCApplicationStoppedTime",
			"3. Profile with async-profiler for allocation hotspots",
			"4. Review recent code changes for memory usage patterns",
		}
	}

	issues = append(issues, PerformanceIssue{
		Type:           "Memory Leak Detection",
		Severity:       severity,
		Description:    fmt.Sprintf("Memory leak detected: %.2f MB/hour growth", trend.GrowthRateMBPerHour),
		Recommendation: recommendations,
	})

	return issues
}

func detectMemoryLeak(events []GCEvent) MemoryTrend {
	if len(events) < MinEventsForTrend {
		return MemoryTrend{LeakSeverity: "none"}
	}

	// Extract post-GC heap usage and timestamps
	var timePoints []float64
	var heapUsages []float64
	var baselineUsages []float64

	startTime := events[0].Timestamp

	for _, event := range events {
		// Skip events without heap data
		if event.HeapBefore == 0 || event.HeapAfter == 0 {
			continue
		}

		timeHours := event.Timestamp.Sub(startTime).Hours()
		timePoints = append(timePoints, timeHours)
		heapUsages = append(heapUsages, float64(event.HeapAfter)/1024/1024) // Convert to MB

		// Track baseline (post-GC) usage - this is key for leak detection
		baselineUsages = append(baselineUsages, float64(event.HeapAfter)/1024/1024)
	}

	if len(timePoints) < MinEventsForTrend {
		return MemoryTrend{LeakSeverity: "none"}
	}

	samplePeriod := time.Duration(timePoints[len(timePoints)-1] * float64(time.Hour))
	if samplePeriod < MinTimeForTrend {
		return MemoryTrend{LeakSeverity: "none"}
	}

	// Calculate linear regression for baseline memory growth
	slope, correlation := linearRegression(timePoints, baselineUsages)

	// Estimate total heap size from maximum heap usage
	maxHeap := 0.0
	totalHeap := 0.0
	for i := range events {
		if i < len(heapUsages) {
			if heapUsages[i] > maxHeap {
				maxHeap = heapUsages[i]
			}
			// Estimate total heap as ~1.3x max usage (typical headroom)
			totalHeap = maxHeap * 1.3
		}
	}

	trend := MemoryTrend{
		GrowthRateMBPerHour: slope,
		GrowthRatePercent:   slope / totalHeap, // Growth as percentage of total heap
		BaselineGrowthRate:  slope,
		TrendConfidence:     correlation * correlation, // R-squared
		SamplePeriod:        samplePeriod,
		EventCount:          len(timePoints),
	}

	// Project time to heap exhaustion
	if slope > 0 && totalHeap > 0 {
		remainingHeap := totalHeap - baselineUsages[len(baselineUsages)-1]
		hoursToFull := remainingHeap / slope
		trend.ProjectedFullHeapTime = time.Duration(hoursToFull * float64(time.Hour))
	}

	// Determine severity based on growth rate and confidence
	if trend.TrendConfidence > 0.7 { // Only flag if we're confident in the trend
		switch {
		case slope > LeakGrowthCritical || trend.GrowthRatePercent > BaselineGrowthCritical:
			trend.LeakSeverity = "critical"
		case slope > LeakGrowthWarning || trend.GrowthRatePercent > BaselineGrowthWarning:
			trend.LeakSeverity = "warning"
		default:
			trend.LeakSeverity = "none"
		}
	} else {
		trend.LeakSeverity = "none"
	}

	return trend
}

// Simple linear regression to calculate slope and correlation
func linearRegression(x, y []float64) (slope, correlation float64) {
	if len(x) != len(y) || len(x) < 2 {
		return 0, 0
	}

	n := float64(len(x))
	var sumX, sumY, sumXY, sumXX, sumYY float64

	for i := 0; i < len(x); i++ {
		sumX += x[i]
		sumY += y[i]
		sumXY += x[i] * y[i]
		sumXX += x[i] * x[i]
		sumYY += y[i] * y[i]
	}

	// Calculate slope (growth rate)
	denominator := n*sumXX - sumX*sumX
	if denominator == 0 {
		return 0, 0
	}

	slope = (n*sumXY - sumX*sumY) / denominator

	// Calculate correlation coefficient
	numerator := n*sumXY - sumX*sumY
	denominatorCorr := math.Sqrt((n*sumXX - sumX*sumX) * (n*sumYY - sumY*sumY))

	if denominatorCorr == 0 {
		correlation = 0
	} else {
		correlation = numerator / denominatorCorr
	}

	return slope, correlation
}

func analyzePrematurePromotion(metrics *GCMetrics, events []GCEvent) []PerformanceIssue {
	var issues []PerformanceIssue

	if len(events) < 10 {
		return issues
	}

	// Use the metrics parameter directly instead of calling a separate function
	// All promotion metrics are now calculated as part of the main metrics

	// Check for critical premature promotion
	if metrics.MaxOldGrowthRatio > OldRegionGrowthCritical ||
		metrics.AvgPromotionRate > PromotionRateCritical {

		issues = append(issues, PerformanceIssue{
			Type:     "Premature Promotion - Critical",
			Severity: "critical",
			Description: fmt.Sprintf(
				"SEVERE premature promotion: Old generation growing %.1fx per young GC (max: %.1fx), "+
					"%.1f regions promoted per collection",
				metrics.AvgOldGrowthRatio, metrics.MaxOldGrowthRatio,
				metrics.AvgPromotionRate),
			Recommendation: []string{
				"IMMEDIATE ACTIONS (restart required):",
				"1. Increase young generation: -XX:G1NewSizePercent=40 -XX:G1MaxNewSizePercent=60",
				"2. Increase survivor space: -XX:SurvivorRatio=6 (default 8)",
				"3. Keep objects in young longer: -XX:MaxTenuringThreshold=15",
				"4. Start concurrent marking earlier: -XX:G1HeapOccupancyPercent=25",
				"",
				"INVESTIGATION (while running):",
				"5. Profile object lifecycle: async-profiler -e alloc -d 60 <pid>",
				"6. Check for short-lived large objects or collections",
				"7. Review recent code changes for object retention patterns",
				generateRegionSizeRecommendation(metrics),
			},
		})
	} else if metrics.MaxOldGrowthRatio > OldRegionGrowthWarning ||
		metrics.AvgPromotionRate > PromotionRateWarning {

		issues = append(issues, PerformanceIssue{
			Type:     "Premature Promotion - Warning",
			Severity: "warning",
			Description: fmt.Sprintf(
				"High promotion rate: %.1f regions per young GC, old generation growing %.1fx on average",
				metrics.AvgPromotionRate, metrics.AvgOldGrowthRatio),
			Recommendation: []string{
				"⚠️  Objects not dying in young generation as expected",
				fmt.Sprintf("Young generation efficiency: %.1f%% (target: >80%%)",
					metrics.YoungCollectionEfficiency*100),
				"",
				"TUNING RECOMMENDATIONS:",
				"1. Increase young generation size to give objects more time to die",
				"2. Monitor allocation patterns for optimization opportunities",
				"3. Consider survivor space tuning if efficiency is low",
				"4. Profile to identify long-lived temporary objects",
				generateAllocationAdvice(metrics),
			},
		})
	}

	// Check for survivor space issues
	if metrics.SurvivorOverflowRate > SurvivorOverflowCritical {
		issues = append(issues, PerformanceIssue{
			Type:     "Survivor Space Overflow",
			Severity: "warning",
			Description: fmt.Sprintf(
				"Survivor spaces overflowing in %.1f%% of young collections",
				metrics.SurvivorOverflowRate*100),
			Recommendation: []string{
				"Survivor spaces too small - objects forced into old generation",
				"1. Increase survivor ratio: -XX:SurvivorRatio=6 (default is 8)",
				"2. Increase overall young generation: -XX:G1MaxNewSizePercent=50",
				"3. Monitor survivor utilization patterns",
				"4. Check for allocation bursts overwhelming survivors",
			},
		})
	}

	// Check concurrent marking effectiveness
	if metrics.PromotionEfficiency < 0.5 && metrics.MixedGCCount > 0 {
		issues = append(issues, PerformanceIssue{
			Type:     "Ineffective Concurrent Marking",
			Severity: "warning",
			Description: fmt.Sprintf(
				"Only %.1f%% of promoted objects are cleaned up by concurrent marking",
				metrics.PromotionEfficiency*100),
			Recommendation: []string{
				"Concurrent marking not effectively cleaning promoted objects",
				"1. Start marking earlier: -XX:G1HeapOccupancyPercent=30",
				"2. Increase concurrent threads if CPU available: -XX:ConcGCThreads=4",
				"3. Check for long-lived objects that should be optimized",
				"4. Review mixed collection targeting: -XX:G1MixedGCLiveThresholdPercent=75",
			},
		})
	}

	return issues
}

func generateRegionSizeRecommendation(metrics *GCMetrics) string {
	if metrics.MaxPromotionRate > 20 {
		return "8. Consider larger heap regions for high allocation: -XX:G1HeapRegionSize=32m"
	} else if metrics.MaxPromotionRate > 10 {
		return "8. Consider medium heap regions: -XX:G1HeapRegionSize=16m"
	}
	return "8. Current region size likely appropriate"
}

func generateAllocationAdvice(metrics *GCMetrics) string {
	if metrics.YoungCollectionEfficiency < 0.6 {
		return "5. LOW young generation efficiency - investigate object lifecycle patterns"
	} else if metrics.SurvivorOverflowRate > 0.2 {
		return "5. Frequent survivor overflow detected - increase survivor space"
	}
	return "5. Monitor allocation patterns and object lifecycle"
}
