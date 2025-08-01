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
	HeapUtilWarning  = 0.8  // 80% - start monitoring
	HeapUtilCritical = 0.9  // 90% - immediate action needed
	RegionUtilMax    = 0.75 // 75% - optimal for G1 region management

	// Allocation thresholds
	AllocRateModerate = 100.0 // MB/s
	AllocRateHigh     = 500.0
	AllocRateCritical = 2000.0

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
	LeakGrowthCritical = 5.0 // MB/hour
	LeakGrowthWarning  = 1.0
	LeakGrowthMinimal  = 0.1

	// Minimum data points for reliable trend analysis
	MinEventsForTrend = 20
	MinTimeForTrend   = 30 * time.Minute

	BaselineGrowthCritical = 0.05 // 5% per hour of total heap
	BaselineGrowthWarning  = 0.02

	PromotionRateWarning     = 10.0 // regions per young GC
	PromotionRateCritical    = 15.0
	OldRegionGrowthCritical  = 3.0 // 3x growth per young GC
	OldRegionGrowthWarning   = 1.5
	SurvivorOverflowWarning  = 0.2
	SurvivorOverflowCritical = 0.3 // 30% of young collections overflow survivors

	HumongousPercentCritical = 80.0 // 80% of heap in humongous objects
	HumongousPercentWarning  = 50.0
	HumongousPercentModerate = 20.0

	EvacFailureRateCritical = 5.0 // 5% evacuation failure rate
	EvacFailureRateWarning  = 1.0

	ConcurrentCycleWarning  = 20 * time.Second // Warning cycle duration
	ConcurrentCycleCritical = 60 * time.Second

	RegionUtilWarning  = 0.85 // 85%
	RegionUtilCritical = 0.95

	YoungCollectionEffWarning = 0.6 // 60% - suboptimal young collection
	MixedCollectionEffWarning = 0.25

	PromotionEfficiencyWarning  = 0.3 // 30% - low promotion efficiency
	PromotionEfficiencyCritical = 0.1

	PauseVarianceWarning  = 0.3
	PauseVarianceCritical = 0.7

	// Additional constants moved from magic numbers
	EvacFailureHighRate           = 20.0 // 20% failure rate is critical
	HumongousRegionsStaticThresh  = 100  // Static humongous regions threshold
	HumongousRegionsGrowingThresh = 50   // Growing humongous regions threshold
	HeapUtilPostGCCritical        = 0.95 // 95% heap utilization after GC
	MemoryPressureCritical        = 95.0 // 95% memory pressure at failures

	// Heap size calculation constants
	HeapSizeAllocFactor = 0.6  // Factor for allocation rate based heap sizing
	MinRecommendedHeap  = 4.0  // GB - minimum recommended heap
	MaxRecommendedHeap  = 64.0 // GB - maximum recommended heap

	// Thread calculation constants
	HighAllocThreads    = 8 // Threads for >1000 MB/s allocation
	MediumAllocThreads  = 6 // Threads for >500 MB/s allocation
	DefaultAllocThreads = 4 // Default thread count

	// Pause target adjustment
	PauseTargetAdjustment = 1.2 // 20% increase for pause target

	// Leak detection confidence threshold
	LeakConfidenceThreshold = 0.7 // 70% confidence required

	// Leak scoring constants
	LeakScorePerIndicator   = 1 // Base score per indicator
	LeakScoreFullGC         = 3 // Additional score for Full GCs
	LeakScoreHumongous      = 4 // Additional score for humongous leaks
	LeakScoreEvacFailure    = 2 // Additional score for evacuation failures
	LeakScoreHighHeapUtil   = 2 // Additional score for high heap utilization
	LeakScoreCriticalThresh = 4 // Threshold for critical leak
	LeakScoreWarningThresh  = 2 // Threshold for warning leak

	// Mixed collection analysis
	MixedCollectionMinYoung = 50  // Minimum young collections before expecting mixed
	ExpectedMixedRatio      = 0.1 // Expected ratio of mixed to young collections
	AllocationBurstThresh   = 10  // % of events that can be bursts before flagging

)

type AnalysisState struct {
	// Use metrics instead of calculating redundantly
	Metrics *GCMetrics

	// Cached calculations specific to analysis
	EvacFailureCount int
	FullGCCount      int
	HumongousStats   HumongousObjectStats
	MemoryTrend      MemoryTrend

	// Event categorization
	YoungCollections int
	MixedCollections int

	// Processed for reuse
	Events         []GCEvent
	TotalEvents    int
	AnalysisPeriod time.Duration
}

type HumongousObjectStats struct {
	MaxRegions      int
	HeapPercentage  float64
	StaticCount     int
	GrowingCount    int
	DecreasingCount int
	IsLeak          bool
}

func (log *GCLog) GetAnalysisData() (*GCLog, *GCMetrics, *Analysis) {
	metrics := log.CalculateMetrics()

	// Create shared analysis state to avoid redundant calculations
	state := buildAnalysisState(log.Events, metrics)

	analysis := detectPerformanceIssuesOptimized(metrics, state)
	log.Status = log.AssessGCHealth(metrics, analysis)

	return log, metrics, analysis
}

func buildAnalysisState(events []GCEvent, metrics *GCMetrics) *AnalysisState {
	state := &AnalysisState{
		Events:      events,
		TotalEvents: len(events),
		Metrics:     metrics, // Use calculated metrics instead of recalculating
	}

	if len(events) == 0 {
		return state
	}

	state.AnalysisPeriod = events[len(events)-1].Timestamp.Sub(events[0].Timestamp)

	// Count event types and failures once (these are not in base metrics)
	for _, event := range events {
		if strings.Contains(strings.ToLower(event.Type), "full") {
			state.FullGCCount++
		}
		if event.ToSpaceExhausted || strings.Contains(event.Cause, "Evacuation Failure") {
			state.EvacFailureCount++
		}
		if strings.Contains(event.Type, "Mixed") {
			state.MixedCollections++
		}
		if strings.Contains(event.Type, "Young") {
			state.YoungCollections++
		}
	}

	// Analyze humongous objects once
	state.HumongousStats = analyzeHumongousObjectsOnce(events)

	// Detect memory trends once (only if enough data)
	if len(events) >= MinEventsForTrend && state.AnalysisPeriod >= MinTimeForTrend {
		state.MemoryTrend = detectMemoryLeak(events)
	} else {
		state.MemoryTrend = MemoryTrend{LeakSeverity: "none"}
	}

	return state
}

func detectPerformanceIssuesOptimized(metrics *GCMetrics, state *AnalysisState) *Analysis {
	var issues []PerformanceIssue

	// Each analyzer now uses shared state to avoid redundant calculations
	issues = append(issues, analyzeFullGCProblemsOptimized(state)...)
	issues = append(issues, analyzeEvacuationFailuresOptimized(state)...)
	issues = append(issues, analyzeHumongousObjectsOptimized(state)...)
	issues = append(issues, analyzeThroughputAndPauses(metrics, state.Events)...)
	issues = append(issues, analyzeMemoryPressureOptimized(metrics, state)...)
	issues = append(issues, analyzeAllocationPatternsOptimized(metrics, state)...)
	issues = append(issues, analyzeConcurrentMarking(metrics)...)
	issues = append(issues, analyzePrematurePromotion(metrics, state.Events)...)
	issues = append(issues, analyzeCollectionEfficiencyOptimized(metrics, state)...)
	issues = append(issues, analyzeDetailedPhases(state.Events)...)

	// Memory leak detection now uses results from other analyzers
	issues = append(issues, analyzeMemoryLeaksOptimized(state)...)

	return groupIssuesBySeverity(issues)
}

func analyzeFullGCProblemsOptimized(state *AnalysisState) []PerformanceIssue {
	var issues []PerformanceIssue

	if state.FullGCCount == 0 {
		return issues
	}

	severity := "critical"
	var recommendations []string

	if state.FullGCCount == 1 {
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
		Description:    fmt.Sprintf("%d Full GC events detected - G1GC performance severely degraded", state.FullGCCount),
		Recommendation: recommendations,
	})

	return issues
}

func analyzeEvacuationFailuresOptimized(state *AnalysisState) []PerformanceIssue {
	var issues []PerformanceIssue

	if state.EvacFailureCount == 0 {
		return issues
	}

	failureRate := float64(state.EvacFailureCount) / float64(state.TotalEvents) * 100

	var evacuationFailureEvents []int
	var failureMemoryPressure []float64

	// Collect detailed failure information
	for _, event := range state.Events {
		if event.ToSpaceExhausted || strings.Contains(event.Cause, "Evacuation Failure") {
			evacuationFailureEvents = append(evacuationFailureEvents, event.ID)

			if event.HeapTotal > 0 && event.HeapBefore > 0 {
				pressure := float64(event.HeapBefore) / float64(event.HeapTotal)
				failureMemoryPressure = append(failureMemoryPressure, pressure)
			}
		}
	}

	severity := "critical"
	recommendations := []string{
		fmt.Sprintf("EVACUATION FAILURES detected in %d GC events (%.1f%% failure rate)",
			state.EvacFailureCount, failureRate),
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
	if failureRate > EvacFailureHighRate {
		recommendations = append(recommendations,
			fmt.Sprintf("CRITICAL: >%.0f%% failure rate indicates severe memory pressure", EvacFailureHighRate))
	}

	// Add memory pressure analysis from failure events
	if len(failureMemoryPressure) > 0 {
		avgPressure := calculateAverage(failureMemoryPressure) * 100
		maxPressure := calculateMax(failureMemoryPressure) * 100
		recommendations = append(recommendations,
			fmt.Sprintf("Memory pressure at failures: avg %.1f%%, max %.1f%%", avgPressure, maxPressure))

		if avgPressure > MemoryPressureCritical {
			recommendations = append(recommendations,
				fmt.Sprintf("Failures occur at >%.0f%% heap utilization - increase heap size immediately", MemoryPressureCritical))
		}
	}

	issues = append(issues, PerformanceIssue{
		Type:           "Evacuation Failures",
		Severity:       severity,
		Description:    fmt.Sprintf("%d evacuation failures (%.1f%% of collections)", state.EvacFailureCount, failureRate),
		Recommendation: recommendations,
	})

	return issues
}

func analyzeHumongousObjectsOnce(events []GCEvent) HumongousObjectStats {
	stats := HumongousObjectStats{}

	if len(events) == 0 {
		return stats
	}

	var humongousEvents []GCEvent
	maxHumongousBefore := 0
	maxHumongousAfter := 0
	totalRegions := 0

	for _, event := range events {
		humongousBefore := event.HumongousRegionsBefore
		humongousAfter := event.HumongousRegionsAfter

		if humongousBefore > 0 || humongousAfter > 0 {
			humongousEvents = append(humongousEvents, event)
			if humongousBefore > maxHumongousBefore {
				maxHumongousBefore = humongousBefore
			}
			if humongousAfter > maxHumongousAfter {
				maxHumongousAfter = humongousAfter
			}

			if event.HeapTotal > 0 && event.RegionSize > 0 {
				totalRegions = int(event.HeapTotal / event.RegionSize)
			} else if event.HeapTotal > 0 {
				totalRegions = int(event.HeapTotal / (1024 * 1024))
			}
		}
	}

	if len(humongousEvents) == 0 {
		return stats
	}

	stats.MaxRegions = max(maxHumongousBefore, maxHumongousAfter)

	// Calculate percentage of heap consumed
	if totalRegions > 0 {
		stats.HeapPercentage = float64(stats.MaxRegions) / float64(totalRegions) * 100
	} else {
		latestEvent := humongousEvents[len(humongousEvents)-1]
		if latestEvent.HeapTotal > 0 {
			humongousBytes := int64(stats.MaxRegions * 1024 * 1024)
			stats.HeapPercentage = float64(humongousBytes) / float64(latestEvent.HeapTotal) * 100
		}
	}

	// Analyze collection patterns
	if len(humongousEvents) >= 2 {
		for i := 1; i < len(humongousEvents); i++ {
			prev := humongousEvents[i-1]
			curr := humongousEvents[i]

			var prevHumongous, currHumongous int
			if prev.HumongousRegionsAfter > 0 {
				prevHumongous = prev.HumongousRegionsAfter
			}
			if curr.HumongousRegionsBefore > 0 {
				currHumongous = curr.HumongousRegionsBefore
			} else if curr.HumongousRegionsAfter > 0 {
				currHumongous = curr.HumongousRegionsAfter
			}

			if currHumongous > prevHumongous {
				stats.GrowingCount++
			} else if currHumongous == prevHumongous {
				stats.StaticCount++
			} else {
				stats.DecreasingCount++
			}
		}
	}

	// Determine if this indicates a leak
	stats.IsLeak = (stats.HeapPercentage > HumongousPercentWarning) ||
		(stats.StaticCount > len(humongousEvents)/2 && stats.MaxRegions > HumongousRegionsStaticThresh) ||
		(stats.GrowingCount > stats.DecreasingCount && stats.MaxRegions > HumongousRegionsGrowingThresh)

	return stats
}

func analyzeHumongousObjectsOptimized(state *AnalysisState) []PerformanceIssue {
	var issues []PerformanceIssue

	stats := state.HumongousStats
	if stats.MaxRegions == 0 {
		return issues
	}

	var severity string
	var recommendations []string

	// Determine severity based on pre-calculated stats
	if stats.HeapPercentage > HumongousPercentWarning ||
		(stats.StaticCount > len(state.Events)/2 && stats.MaxRegions > HumongousRegionsStaticThresh) ||
		(stats.GrowingCount > stats.DecreasingCount && stats.MaxRegions > HumongousRegionsGrowingThresh) {
		severity = "critical"
		recommendations = []string{
			fmt.Sprintf("%d humongous regions consuming %.1f%% of heap", stats.MaxRegions, stats.HeapPercentage),
			"Humongous objects (>50% of region size) are not being garbage collected",
			"This indicates a MEMORY LEAK in large object allocation",
			"Take heap dump immediately: jcmd <pid> GC.run_finalization; jcmd <pid> VM.dump_heap leak.hprof",
			"Analyze with Eclipse MAT or VisualVM for large objects",
			"Look for: large arrays, huge strings, oversized collections, cached data",
			"Check recent code for large object allocations that aren't released",
			"Increase heap size by 200-300% as temporary relief: -Xmx<current_size * 3>",
			"Enable OOM dump: -XX:+HeapDumpOnOutOfMemoryError",
		}

		if stats.StaticCount > stats.GrowingCount {
			recommendations = append(recommendations,
				fmt.Sprintf("%d humongous regions unchanged across GC cycles (static: %d, growing: %d, decreasing: %d)",
					stats.MaxRegions, stats.StaticCount, stats.GrowingCount, stats.DecreasingCount))
		}
		if stats.GrowingCount > stats.DecreasingCount {
			recommendations = append(recommendations,
				fmt.Sprintf("Humongous objects consistently growing: %d growth vs %d cleanup events",
					stats.GrowingCount, stats.DecreasingCount))
		}

	} else if stats.HeapPercentage > HumongousPercentModerate || stats.MaxRegions > HumongousRegionsGrowingThresh {
		severity = "warning"
		recommendations = []string{
			fmt.Sprintf("Significant humongous object usage: %d regions (%.1f%% of heap)", stats.MaxRegions, stats.HeapPercentage),
			"Large objects are consuming significant heap space",
			"Monitor for memory leak patterns",
			"Consider object size optimization or heap size increase",
		}
	}

	if severity != "" {
		issues = append(issues, PerformanceIssue{
			Type:           "Humongous Objects",
			Severity:       severity,
			Description:    fmt.Sprintf("Large objects consuming %.1f%% of heap (%d regions)", stats.HeapPercentage, stats.MaxRegions),
			Recommendation: recommendations,
		})
	}

	return issues
}

func analyzeMemoryPressureOptimized(metrics *GCMetrics, state *AnalysisState) []PerformanceIssue {
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

		if regionPressure > RegionUtilWarning {
			recommendations = append(recommendations,
				fmt.Sprintf("High region utilization (%.1f%%) may cause allocation failures", regionPressure*100))
			recommendations = append(recommendations,
				"Consider increasing region size or total heap to reduce region pressure")
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

func analyzeAllocationPatternsOptimized(metrics *GCMetrics, state *AnalysisState) []PerformanceIssue {
	var issues []PerformanceIssue

	allocRate := metrics.AllocationRate
	if allocRate > AllocRateModerate {
		severity := "info"
		if allocRate > AllocRateHigh {
			severity = "warning"
		}
		if allocRate > AllocRateCritical {
			severity = "critical"
		}

		var recommendations []string
		if allocRate > AllocRateCritical {
			recommendations = []string{
				fmt.Sprintf("Very high allocation rate: %.1f MB/s requires specialized tuning", allocRate),
				"Use large heap regions: -XX:G1HeapRegionSize=32m",
				"Increase young generation: -XX:G1NewSizePercent=30 -XX:G1MaxNewSizePercent=70",
				"Profile allocation hotspots with async-profiler",
				"Consider object pooling for high-frequency allocations",
			}
		} else if allocRate > AllocRateHigh {
			recommendations = []string{
				fmt.Sprintf("High allocation rate: %.1f MB/s needs monitoring", allocRate),
				getRegionSizeRecommendation(allocRate),
				"Optimize young generation sizing for allocation pattern",
				"Review object lifecycle and temporary object creation",
			}
		} else {
			recommendations = []string{
				fmt.Sprintf("Moderate allocation rate: %.1f MB/s is typical", allocRate),
				"Current allocation rate is manageable",
				"Monitor for allocation bursts or patterns",
			}
		}

		// Add allocation burst information if relevant
		if metrics.AllocationBurstCount > state.TotalEvents/AllocationBurstThresh {
			recommendations = append(recommendations,
				fmt.Sprintf("Note: %d allocation bursts detected - consider batch processing optimization", metrics.AllocationBurstCount))
		}

		issues = append(issues, PerformanceIssue{
			Type:           "Allocation Patterns",
			Severity:       severity,
			Description:    fmt.Sprintf("Allocation rate %.1f MB/s", allocRate),
			Recommendation: recommendations,
		})
	}

	return issues
}

func analyzeCollectionEfficiencyOptimized(metrics *GCMetrics, state *AnalysisState) []PerformanceIssue {
	var issues []PerformanceIssue

	// Check for missing mixed collections using pre-counted values
	if state.MixedCollections == 0 && state.YoungCollections > MixedCollectionMinYoung {
		issues = append(issues, PerformanceIssue{
			Type:     "Missing Mixed Collections",
			Severity: "warning",
			Description: fmt.Sprintf("No mixed collections in %d young collections - old generation not being cleaned",
				state.YoungCollections),
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

func analyzeMemoryLeaksOptimized(state *AnalysisState) []PerformanceIssue {
	var issues []PerformanceIssue

	// Use results from other analyzers instead of re-analyzing
	leakIndicators := buildLeakIndicators(state)
	leakScore := calculateLeakScore(leakIndicators, state)

	// Add long-term trend analysis if available
	if state.MemoryTrend.LeakSeverity != "none" {
		longTermIssues := analyzeLongTermMemoryLeak(state.MemoryTrend)
		issues = append(issues, longTermIssues...)
	}

	// Determine severity based on combined indicators
	if leakScore >= LeakScoreCriticalThresh {
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
	} else if leakScore >= LeakScoreWarningThresh {
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

func buildLeakIndicators(state *AnalysisState) []string {
	var indicators []string

	// Use pre-analyzed results instead of re-processing events
	if state.FullGCCount >= 3 {
		indicators = append(indicators,
			fmt.Sprintf("%d Full GCs detected", state.FullGCCount))
	}

	if state.HumongousStats.IsLeak {
		indicators = append(indicators,
			fmt.Sprintf("%d humongous regions not being collected", state.HumongousStats.MaxRegions))
	}

	if state.Metrics.AvgHeapUtil > HeapUtilPostGCCritical {
		indicators = append(indicators,
			fmt.Sprintf("Heap at %.1f%% utilization after GC", state.Metrics.AvgHeapUtil*100))
	}

	if state.EvacFailureCount > 0 {
		indicators = append(indicators,
			fmt.Sprintf("%d evacuation failures", state.EvacFailureCount))
	}

	return indicators
}

func calculateLeakScore(indicators []string, state *AnalysisState) int {
	score := 0

	// Score based on indicators
	score += len(indicators) * LeakScorePerIndicator

	// Additional scoring based on severity
	if state.FullGCCount >= 3 {
		score += LeakScoreFullGC
	}
	if state.HumongousStats.IsLeak {
		score += LeakScoreHumongous
	}
	if state.EvacFailureCount > 0 {
		score += LeakScoreEvacFailure
	}
	if state.Metrics.AvgHeapUtil > HeapUtilPostGCCritical {
		score += LeakScoreHighHeapUtil
	}

	return score
}

// Remaining functions stay the same as they don't have redundancy issues
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

	pauseTarget := estimatePauseTarget(events)
	if pauseTarget == 0 {
		pauseTarget = PauseTargetDefault
	}

	var severity string
	var recommendations []string
	var description string

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
			fmt.Sprintf("Adjust pause target: -XX:MaxGCPauseMillis=%d", int(float64(pauseTarget.Milliseconds())*PauseTargetAdjustment)),
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

	if metrics.ConcurrentCycleDuration > ConcurrentCycleWarning {
		issues = append(issues, PerformanceIssue{
			Type:     "Long Concurrent Cycles",
			Severity: "warning",
			Description: fmt.Sprintf("Concurrent cycles taking %v (target: <%v)",
				metrics.ConcurrentCycleDuration, ConcurrentCycleWarning),
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

	if metrics.MaxOldGrowthRatio > OldRegionGrowthCritical ||
		metrics.AvgPromotionRate > PromotionRateCritical {

		issues = append(issues, PerformanceIssue{
			Type:     "Premature Promotion",
			Severity: "critical",
			Description: fmt.Sprintf(
				"Old gen growing %.1fx per young GC (max: %.1fx), "+
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

	if metrics.PromotionEfficiency < PromotionEfficiencyWarning && metrics.MixedGCCount > 0 {
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

// Helper functions remain the same
func calculateRecommendedHeapSize(allocRate float64) float64 {
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
	if allocRate > AllocRateCritical/2 { // 1000 MB/s
		return "Use large regions: -XX:G1HeapRegionSize=32m"
	} else if allocRate > AllocRateHigh {
		return "Use medium regions: -XX:G1HeapRegionSize=16m"
	} else if allocRate < AllocRateModerate {
		return "Use small regions: -XX:G1HeapRegionSize=8m"
	}
	return "Keep default region size: -XX:G1HeapRegionSize=16m"
}

func calculateOptimalConcThreads(allocRate float64) int {
	if allocRate > AllocRateCritical/2 { // 1000 MB/s
		return HighAllocThreads
	} else if allocRate > AllocRateHigh {
		return MediumAllocThreads
	}
	return DefaultAllocThreads
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
	if metrics.MaxPromotionRate > PromotionRateCritical+5 { // 20
		return "Consider larger heap regions for high allocation: -XX:G1HeapRegionSize=32m"
	} else if metrics.MaxPromotionRate > PromotionRateWarning {
		return "Consider medium heap regions: -XX:G1HeapRegionSize=16m"
	}
	return "Current region size likely appropriate"
}

func generateAllocationAdvice(metrics *GCMetrics) string {
	if metrics.YoungCollectionEfficiency < YoungCollectionEffWarning {
		return "LOW young generation efficiency - investigate object lifecycle patterns"
	} else if metrics.SurvivorOverflowRate > SurvivorOverflowWarning {
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
	var baselineUsages []float64

	startTime := events[0].Timestamp

	for _, event := range events {
		// Skip events without heap data
		if event.HeapBefore == 0 || event.HeapAfter == 0 {
			continue
		}

		timeHours := event.Timestamp.Sub(startTime).Hours()
		timePoints = append(timePoints, timeHours)
		heapUsageMB := float64(event.HeapAfter) / 1024 / 1024
		// Track baseline (post-GC) usage - this is key for leak detection
		baselineUsages = append(baselineUsages, heapUsageMB)
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

	totalHeap := 0.0
	for _, event := range events {
		if event.HeapTotal > 0 {
			totalHeapMB := float64(event.HeapTotal) / 1024 / 1024
			if totalHeapMB > totalHeap {
				totalHeap = totalHeapMB
			}
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

	// Project time to heap exhaustion using actual heap total
	if slope > 0 && totalHeap > 0 {
		remainingHeap := totalHeap - baselineUsages[len(baselineUsages)-1]
		hoursToFull := remainingHeap / slope
		trend.ProjectedFullHeapTime = time.Duration(hoursToFull * float64(time.Hour))
	}

	// Determine severity based on growth rate and confidence
	if trend.TrendConfidence > LeakConfidenceThreshold { // Only flag if we're confident in the trend
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
