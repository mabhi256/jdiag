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

	// Humongous object thresholds
	HumongousPercentCritical = 80.0 // 80% of heap in humongous objects
	HumongousPercentWarning  = 50.0 // 50% of heap in humongous objects
	HumongousPercentModerate = 20.0 // 20% of heap in humongous objects

	// Evacuation failure thresholds
	EvacFailureRateCritical = 5.0 // 5% evacuation failure rate
	EvacFailureRateWarning  = 1.0 // 1% evacuation failure rate
)

func (log *GCLog) GetAnalysisData() (*GCLog, *GCMetrics, *Analysis) {
	metrics := log.CalculateMetrics()
	analysis := metrics.DetectPerformanceIssues(log.Events)
	log.Status = log.AssessGCHealth(metrics, analysis)

	return log, metrics, analysis
}

func (metrics *GCMetrics) DetectPerformanceIssues(events []GCEvent) *Analysis {
	var issues []PerformanceIssue

	issues = append(issues, analyzeHumongousObjects(events)...)
	issues = append(issues, analyzeEvacuationFailures(events)...)
	issues = append(issues, analyzeMemoryLeaks(events)...)
	issues = append(issues, analyzeFullGCProblems(metrics)...)
	issues = append(issues, analyzeThroughputAndPauses(metrics, events)...)
	issues = append(issues, analyzeMemoryPressure(metrics)...)
	issues = append(issues, analyzeAllocationPatterns(metrics)...)
	issues = append(issues, analyzeConcurrentMarking(metrics)...)
	issues = append(issues, analyzePrematurePromotion(metrics, events)...)
	issues = append(issues, analyzeCollectionEfficiency(metrics, events)...)
	issues = append(issues, analyzeDetailedPhases(events)...)

	return groupIssuesBySeverity(issues)
}

func analyzeFullGCProblems(metrics *GCMetrics) []PerformanceIssue {
	var issues []PerformanceIssue

	if metrics.FullGCCount == 0 {
		return issues
	}

	severity := "critical"
	var recommendations []string

	if metrics.FullGCCount == 1 {
		recommendations = []string{
			"Check for heap sizing: increase -Xmx by 50-100% if possible",
			"Enable detailed logging: -XX:+PrintGCDetails -XX:+PrintGCTimeStamps",
			"Monitor for one-time events (class loading, permgen, etc.)",
		}
	} else {
		recommendations = []string{
			"Increase heap size by 100-200% immediately",
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
			"Pause times are unacceptable for most applications",
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
				"Increase heap size by at least 50%",
				"Increase evacuation reserve: -XX:G1ReservePercent=15",
				fmt.Sprintf("Optimize region size for allocation rate %.1f MB/s", metrics.AllocationRate),
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

func analyzePrematurePromotion(metrics *GCMetrics, events []GCEvent) []PerformanceIssue {
	var issues []PerformanceIssue

	if len(events) < 10 {
		return issues
	}

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
				"Increase young generation: -XX:G1NewSizePercent=40 -XX:G1MaxNewSizePercent=60",
				"Increase survivor space: -XX:SurvivorRatio=6 (default 8)",
				"Keep objects in young longer: -XX:MaxTenuringThreshold=15",
				"Start concurrent marking earlier: -XX:G1HeapOccupancyPercent=25",
				"Profile object lifecycle: async-profiler -e alloc -d 60 <pid>",
				"Check for short-lived large objects or collections",
				"Review recent code changes for object retention patterns",
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
				"Objects not dying in young generation as expected",
				fmt.Sprintf("Young generation efficiency: %.1f%% (target: >80%%)",
					metrics.YoungCollectionEfficiency*100),
				"Increase young generation size to give objects more time to die",
				"Monitor allocation patterns for optimization opportunities",
				"Consider survivor space tuning if efficiency is low",
				"Profile to identify long-lived temporary objects",
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
				"Increase survivor ratio: -XX:SurvivorRatio=6 (default is 8)",
				"Increase overall young generation: -XX:G1MaxNewSizePercent=50",
				"Monitor survivor utilization patterns",
				"Check for allocation bursts overwhelming survivors",
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
				"Start marking earlier: -XX:G1HeapOccupancyPercent=30",
				"Increase concurrent threads if CPU available: -XX:ConcGCThreads=4",
				"Check for long-lived objects that should be optimized",
				"Review mixed collection targeting: -XX:G1MixedGCLiveThresholdPercent=75",
			},
		})
	}

	return issues
}

func analyzeEvacuationFailures(events []GCEvent) []PerformanceIssue {
	var issues []PerformanceIssue

	evacuationFailures := 0
	evacuationFailureEvents := []int{}

	// Count evacuation failures
	for _, event := range events {
		if event.ToSpaceExhausted || strings.Contains(event.Cause, "Evacuation Failure") {
			evacuationFailures++
			evacuationFailureEvents = append(evacuationFailureEvents, event.ID)
		}
	}

	if evacuationFailures == 0 {
		return issues
	}

	failureRate := float64(evacuationFailures) / float64(len(events)) * 100

	var severity string
	var recommendations []string

	if evacuationFailures > 0 {
		severity = "critical"
		recommendations = []string{
			fmt.Sprintf("EVACUATION FAILURES detected in %d GC events (%.1f%% failure rate)",
				evacuationFailures, failureRate),
			fmt.Sprintf("Failed GC events: %v", evacuationFailureEvents),
			"Evacuation failure means G1GC couldn't move objects to other regions",
			"This causes performance degradation and can trigger Full GC",
			"Heap too full - INCREASE HEAP SIZE immediately: -Xmx<size * 2>",
			"Too many humongous objects - check for large object allocations",
			"Insufficient evacuation reserve - increase: -XX:G1ReservePercent=20",
			"Region fragmentation - consider larger regions: -XX:G1HeapRegionSize=32m",
			"Enable evacuation failure logging: -XX:+TraceClassLoading",
			"Take heap dump after failures to analyze object distribution",
		}

		// Add specific advice based on failure frequency
		if failureRate > 20 {
			recommendations = append(recommendations,
				"CRITICAL: >20% failure rate indicates severe memory pressure")
		}
	}

	if severity != "" {
		issues = append(issues, PerformanceIssue{
			Type:           "Evacuation Failures",
			Severity:       severity,
			Description:    fmt.Sprintf("%d evacuation failures (%.1f%% of collections)", evacuationFailures, failureRate),
			Recommendation: recommendations,
		})
	}

	return issues
}

func analyzeMemoryLeaks(events []GCEvent) []PerformanceIssue {
	var issues []PerformanceIssue

	if len(events) < 5 {
		return issues
	}

	// Always run short-term detection first for immediate issues
	shortTermIssues := detectShortTermMemoryLeaks(events)
	issues = append(issues, shortTermIssues...)

	// Only run long-term analysis if we have enough data AND time
	if len(events) >= MinEventsForTrend {
		startTime := events[0].Timestamp
		endTime := events[len(events)-1].Timestamp
		duration := endTime.Sub(startTime)

		if duration >= MinTimeForTrend {
			trend := detectMemoryLeak(events)
			if trend.LeakSeverity != "none" {
				longTermIssues := analyzeLongTermMemoryLeak(trend)
				issues = append(issues, longTermIssues...)
			}
		}
	}

	return issues
}

func analyzeLongTermMemoryLeak(trend MemoryTrend) []PerformanceIssue {
	var issues []PerformanceIssue
	var recommendations []string

	switch trend.LeakSeverity {
	case "critical":
		recommendations = []string{
			fmt.Sprintf("MEMORY LEAK DETECTED: %.1f MB/hour growth rate", trend.GrowthRateMBPerHour),
			fmt.Sprintf("Baseline memory growing %.1f%% per hour", trend.GrowthRatePercent*100),
			fmt.Sprintf("Heap will be exhausted in approximately %v", trend.ProjectedFullHeapTime),
			"Take heap dump for analysis: jcmd <pid> GC.run_finalization; jcmd <pid> VM.gc; jcmd <pid> VM.dump_heap heap.hprof",
			"Enable heap dump on OOM: -XX:+HeapDumpOnOutOfMemoryError",
			"Analyze with Eclipse MAT or VisualVM",
			"Look for: unclosed resources, static collections, event listeners",
		}

	case "warning":
		recommendations = []string{
			fmt.Sprintf("Suspicious memory growth: %.1f MB/hour (%.1f%% of heap)",
				trend.GrowthRateMBPerHour, trend.GrowthRatePercent*100),
			fmt.Sprintf("Trend confidence: %.1f%% over %v", trend.TrendConfidence*100, trend.SamplePeriod),
			"Take baseline heap dump for comparison",
			"Enable memory tracking: -XX:+PrintGCDetails -XX:+PrintGCApplicationStoppedTime",
			"Profile with async-profiler for allocation hotspots",
			"Review recent code changes for memory usage patterns",
		}
	}

	if len(recommendations) > 0 {
		issues = append(issues, PerformanceIssue{
			Type:           "Long-term Memory Leak",
			Severity:       trend.LeakSeverity,
			Description:    fmt.Sprintf("Memory leak detected: %.2f MB/hour growth", trend.GrowthRateMBPerHour),
			Recommendation: recommendations,
		})
	}

	return issues
}

// Helper functions
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
	return PauseTargetDefault
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

func generateRegionSizeRecommendation(metrics *GCMetrics) string {
	if metrics.MaxPromotionRate > 20 {
		return "Consider larger heap regions for high allocation: -XX:G1HeapRegionSize=32m"
	} else if metrics.MaxPromotionRate > 10 {
		return "Consider medium heap regions: -XX:G1HeapRegionSize=16m"
	}
	return "Current region size likely appropriate"
}

func generateAllocationAdvice(metrics *GCMetrics) string {
	if metrics.YoungCollectionEfficiency < 0.6 {
		return "LOW young generation efficiency - investigate object lifecycle patterns"
	} else if metrics.SurvivorOverflowRate > 0.2 {
		return "Frequent survivor overflow detected - increase survivor space"
	}
	return "Monitor allocation patterns and object lifecycle"
}

func (log *GCLog) AssessGCHealth(metrics *GCMetrics, issues *Analysis) string {
	if metrics == nil {
		return "❓ Unknown"
	}

	criticalCount := len(issues.Critical)
	warningCount := len(issues.Warning)

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

func analyzeHumongousObjects(events []GCEvent) []PerformanceIssue {
	var issues []PerformanceIssue

	if len(events) == 0 {
		return issues
	}

	// Find events with humongous data and analyze the pattern
	var humongousEvents []GCEvent
	maxHumongous := 0
	totalRegions := 0

	for _, event := range events {
		if event.HumongousRegions > 0 {
			humongousEvents = append(humongousEvents, event)
			if event.HumongousRegions > maxHumongous {
				maxHumongous = event.HumongousRegions
			}

			// Calculate total regions from heap total and region size
			if event.HeapTotal > 0 && event.RegionSize > 0 {
				totalRegions = int(event.HeapTotal / event.RegionSize)
			} else if event.HeapTotal > 0 {
				// Fallback: assume 1MB regions (G1 default)
				totalRegions = int(event.HeapTotal / (1024 * 1024))
			}
		}
	}

	if len(humongousEvents) == 0 {
		return issues
	}

	// Calculate percentage of heap consumed by humongous objects
	humongousPercent := 0.0
	if totalRegions > 0 {
		humongousPercent = float64(maxHumongous) / float64(totalRegions) * 100
	} else {
		// Fallback calculation using heap size
		latestEvent := humongousEvents[len(humongousEvents)-1]
		if latestEvent.HeapTotal > 0 {
			// Assume each humongous region is ~1MB
			humongousBytes := int64(maxHumongous * 1024 * 1024)
			humongousPercent = float64(humongousBytes) / float64(latestEvent.HeapTotal) * 100
		}
	}

	// Analyze if humongous objects are being collected
	humongousGrowth := 0
	humongousStatic := 0

	if len(humongousEvents) >= 2 {
		for i := 1; i < len(humongousEvents); i++ {
			prev := humongousEvents[i-1].HumongousRegions
			curr := humongousEvents[i].HumongousRegions

			if curr > prev {
				humongousGrowth++
			} else if curr == prev {
				humongousStatic++
			}
		}
	}

	// Determine severity and recommendations
	var severity string
	var recommendations []string

	// Critical: >50% of heap OR never being collected
	if humongousPercent > HumongousPercentWarning || (humongousStatic > len(humongousEvents)/2 && maxHumongous > 100) {
		severity = "critical"
		recommendations = []string{
			fmt.Sprintf("%d humongous regions consuming %.1f%% of heap", maxHumongous, humongousPercent),
			"Humongous objects (>50% of region size) are not being garbage collected",
			"This indicates a MEMORY LEAK in large object allocation",
			"Take heap dump immediately: jcmd <pid> GC.run_finalization; jcmd <pid> VM.dump_heap leak.hprof",
			"Analyze with Eclipse MAT or VisualVM for large objects",
			"Look for: large arrays, huge strings, oversized collections, cached data",
			"Check recent code for large object allocations that aren't released",
			"Increase heap size by 200-300% as temporary relief: -Xmx<current_size * 3>",
			"Enable OOM dump: -XX:+HeapDumpOnOutOfMemoryError",
		}

		// Add specific leak indicators
		if humongousStatic > humongousGrowth {
			recommendations = append(recommendations,
				fmt.Sprintf("%d humongous regions unchanged across %d GC cycles",
					maxHumongous, humongousStatic))
		}

	} else if humongousPercent > HumongousPercentModerate || maxHumongous > 50 {
		severity = "warning"
		recommendations = []string{
			fmt.Sprintf("Significant humongous object usage: %d regions (%.1f%% of heap)", maxHumongous, humongousPercent),
			"Large objects are consuming significant heap space",
			"Monitor for memory leak patterns",
			"Consider object size optimization or heap size increase",
		}
	}

	// Always report if we have ANY humongous objects with static behavior
	if maxHumongous > 10 && humongousStatic > 0 {
		if severity == "" {
			severity = "info"
			recommendations = []string{
				fmt.Sprintf("Humongous objects detected: %d regions", maxHumongous),
				"Monitor large object allocation patterns",
				"Consider optimizing large data structures",
			}
		}

		recommendations = append(recommendations,
			fmt.Sprintf("%d regions static, %d growing across %d GC events",
				humongousStatic, humongousGrowth, len(humongousEvents)))
	}

	if severity != "" {
		issues = append(issues, PerformanceIssue{
			Type:           "Humongous Objects",
			Severity:       severity,
			Description:    fmt.Sprintf("Large objects consuming %.1f%% of heap (%d regions)", humongousPercent, maxHumongous),
			Recommendation: recommendations,
		})
	}

	return issues
}

func detectShortTermMemoryLeaks(events []GCEvent) []PerformanceIssue {
	var issues []PerformanceIssue

	if len(events) < 5 {
		return issues
	}

	// Collect leak indicators
	var heapAfterGC []int64
	var humongousRegions []int
	fullGCCount := 0
	evacuationFailures := 0

	for _, event := range events {
		if event.HeapAfter > 0 {
			heapAfterGC = append(heapAfterGC, int64(event.HeapAfter))
		}
		if event.HumongousRegions > 0 {
			humongousRegions = append(humongousRegions, event.HumongousRegions)
		}
		if strings.Contains(strings.ToLower(event.Type), "full") {
			fullGCCount++
		}
		if event.ToSpaceExhausted || strings.Contains(event.Cause, "Evacuation Failure") {
			evacuationFailures++
		}
	}

	// Immediate leak indicators
	var leakIndicators []string
	leakScore := 0

	// 1. Multiple Full GCs with no memory recovery
	if fullGCCount >= 3 && len(heapAfterGC) >= 3 {
		lastThree := heapAfterGC[len(heapAfterGC)-3:]
		if lastThree[2] >= lastThree[1] && lastThree[1] >= lastThree[0] {
			leakIndicators = append(leakIndicators,
				fmt.Sprintf("%d Full GCs with no memory recovery", fullGCCount))
			leakScore += 3
		}
	}

	// 2. Humongous objects not being collected (CRITICAL INDICATOR)
	if len(humongousRegions) >= 3 {
		allSame := true
		first := humongousRegions[0]
		for _, count := range humongousRegions[1:] {
			if count < first-5 { // Allow small variation
				allSame = false
				break
			}
		}
		if allSame && first > 20 {
			leakIndicators = append(leakIndicators,
				fmt.Sprintf("%d humongous regions never collected", first))
			leakScore += 4
		}
	}

	// 3. Heap utilization at maximum despite GCs
	if len(heapAfterGC) >= 2 {
		lastEvent := events[len(events)-1]
		if lastEvent.HeapTotal > 0 {
			utilization := float64(lastEvent.HeapAfter) / float64(lastEvent.HeapTotal)
			if utilization > 0.95 {
				leakIndicators = append(leakIndicators,
					fmt.Sprintf("Heap at %.1f%% utilization after GC", utilization*100))
				leakScore += 2
			}
		}
	}

	// 4. Evacuation failures
	if evacuationFailures > 0 {
		leakIndicators = append(leakIndicators,
			fmt.Sprintf("%d evacuation failures", evacuationFailures))
		leakScore += 2
	}

	// 5. Memory growth despite aggressive collection
	if len(heapAfterGC) >= 3 {
		start := heapAfterGC[0]
		end := heapAfterGC[len(heapAfterGC)-1]
		if end > start+50*1024*1024 { // 50MB growth threshold
			leakIndicators = append(leakIndicators,
				fmt.Sprintf("Memory grew from %d to %d MB despite GC", start/1024/1024, end/1024/1024))
			leakScore += 2
		}
	}

	// Determine severity based on leak score
	if leakScore >= 4 {
		recommendations := []string{
			"Multiple indicators suggest objects are not being garbage collected",
		}
		recommendations = append(recommendations, leakIndicators...)
		recommendations = append(recommendations, []string{
			"Take heap dump immediately: jcmd <pid> VM.dump_heap emergency.hprof",
			"Restart application if possible to prevent OutOfMemoryError",
			"Increase heap size as emergency measure: -Xmx<current * 3>",
			"Enable OOM dumps: -XX:+HeapDumpOnOutOfMemoryError",
			"Analyze heap dump for large objects and reference chains",
		}...)

		issues = append(issues, PerformanceIssue{
			Type:           "Severe Memory Leak (Immediate)",
			Severity:       "critical",
			Description:    fmt.Sprintf("IMMEDIATE MEMORY LEAK detected with %d critical indicators", len(leakIndicators)),
			Recommendation: recommendations,
		})
	} else if leakScore >= 2 {
		recommendations := []string{
			"Potential memory leak detected",
		}
		recommendations = append(recommendations, leakIndicators...)
		recommendations = append(recommendations, []string{
			"Take baseline heap dump for analysis",
			"Monitor memory usage over longer period",
			"Check for unclosed resources or static collections",
			"Review recent code changes",
		}...)

		issues = append(issues, PerformanceIssue{
			Type:           "Suspected Memory Leak",
			Severity:       "warning",
			Description:    fmt.Sprintf("Memory leak suspected with %d indicators", len(leakIndicators)),
			Recommendation: recommendations,
		})
	}

	return issues
}

func groupIssuesBySeverity(allIssues []PerformanceIssue) *Analysis {
	var analyzed Analysis

	for _, issue := range allIssues {
		switch issue.Severity {
		case "critical":
			analyzed.Critical = append(analyzed.Critical, issue)
		case "warning":
			analyzed.Warning = append(analyzed.Warning, issue)
		default:
			analyzed.Info = append(analyzed.Info, issue)
		}
	}

	return &analyzed
}
