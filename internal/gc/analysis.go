package gc

import (
	"math"
	"slices"
	"strings"
	"time"
)

// Constants for analysis thresholds
const (
	// Performance targets
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

	// Memory thresholds
	HeapUtilWarning  = 0.8
	HeapUtilCritical = 0.9
	RegionUtilMax    = 0.75

	// Allocation thresholds
	AllocRateModerate = 100.0
	AllocRateHigh     = 500.0
	AllocRateCritical = 2000.0

	// Collection efficiency
	YoungCollectionEff = 0.8
	MixedCollectionEff = 0.4
	EvacFailureMax     = 0.01

	// Phase timing targets
	ObjectCopyTarget    = 30 * time.Millisecond
	RootScanTarget      = 20 * time.Millisecond
	TerminationTarget   = 5 * time.Millisecond
	RefProcessingTarget = 15 * time.Millisecond

	// Leak detection
	LeakGrowthCritical = 5.0
	LeakGrowthWarning  = 1.0
	MinEventsForTrend  = 20
	MinTimeForTrend    = 30 * time.Minute

	// Promotion thresholds
	PromotionRateWarning     = 10.0
	PromotionRateCritical    = 15.0
	OldRegionGrowthCritical  = 3.0
	OldRegionGrowthWarning   = 1.5
	SurvivorOverflowWarning  = 0.2
	SurvivorOverflowCritical = 0.3

	// Humongous thresholds
	HumongousPercentCritical = 80.0
	HumongousPercentWarning  = 50.0
	HumongousPercentModerate = 20.0

	// Leak scoring
	LeakScoreCriticalThresh = 4
	LeakScoreWarningThresh  = 2
	LeakConfidenceThreshold = 0.7
)

const (
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

	// Mixed collection analysis
	MixedCollectionMinYoung = 50  // Minimum young collections before expecting mixed
	ExpectedMixedRatio      = 0.1 // Expected ratio of mixed to young collections
	AllocationBurstThresh   = 10  // % of events that can be bursts before flagging
)

func AnalyzeGCLogs(events []*GCEvent, analysis *GCAnalysis) {
	if len(events) == 0 {
		return
	}

	analysis.TotalEvents = len(events)

	// Initialize time tracking maps
	analysis.GCTypeDurations = make(map[string]time.Duration)
	analysis.GCTypeEventCounts = make(map[string]int)
	analysis.GCCauseDurations = make(map[string]time.Duration)

	// Calculate total runtime, GCLogs are always sorted
	analysis.StartTime = events[0].Timestamp
	analysis.EndTime = events[len(events)-1].Timestamp
	analysis.TotalRuntime = analysis.EndTime.Sub(analysis.StartTime)

	// Pause time collection for percentile calculation
	var durations []time.Duration

	// Tracking variables for single-pass computation
	var totalGCTime time.Duration
	var totalYoungEfficiency, totalMixedEfficiency float64
	var youngEfficiencyCount, mixedEfficiencyCount int
	var totalHeapUtil, heapUtilCount float64
	var totalRegionUtil, regionUtilCount float64
	var allocationEvents []allocationDataPoint
	var promotionEvents []promotionDataPoint
	var humongousEvents []humongousDataPoint
	var memoryTrendPoints []memoryTrendPoint

	// Phase timing accumulators
	var totalObjectCopy, totalRootScan, totalTermination, totalRefProcessing time.Duration
	var objectCopyCount, rootScanCount, terminationCount, refProcessingCount int

	// Consecutive tracking
	consecutiveGrowthSpikes := 0
	currentSpikeCount := 0

	// Previous event for delta calculations
	var prevEvent *GCEvent

	for _, event := range events {
		// ===== CLASSIFY EVENT TYPE =====

		switch event.Type {
		case "Young":
			analysis.YoungGCCount++
		case "Mixed":
			analysis.MixedGCCount++
		case "Full":
			analysis.FullGCCount++
		}

		// ===== GC TIME DISTRIBUTION TRACKING =====

		// Categorize GC type for time tracking
		gcCategory := categorizeGCType(event.Type)

		// Track duration by GC type
		if gcCategory == "Concurrent Mark" {
			analysis.GCTypeDurations[gcCategory] += event.ConcurrentDuration
		} else {
			analysis.GCTypeDurations[gcCategory] += event.Duration
		}
		analysis.GCTypeEventCounts[gcCategory]++

		// Track duration by GC cause
		cause := event.Cause
		if gcCategory != "Concurrent Mark" {
			if cause == "" {
				cause = "Unknown"
			}
			analysis.GCCauseDurations[cause] += event.Duration
		}

		// ===== BASIC METRICS =====
		totalGCTime += event.Duration
		durations = append(durations, event.Duration)

		// ===== ANALYZE INDIVIDUAL EVENT =====

		// Memory pressure analysis
		if event.HeapTotal > 0 {
			event.HeapUtilizationBefore = float64(event.HeapBefore) / float64(event.HeapTotal)
			event.HeapUtilizationAfter = float64(event.HeapAfter) / float64(event.HeapTotal)

			totalHeapUtil += event.HeapUtilizationAfter
			heapUtilCount++

			event.HasMemoryPressure = event.HeapUtilizationBefore > HeapUtilWarning
		}

		// Region utilization
		if event.HeapTotalRegions > 0 && event.HeapUsedRegionsAfter > 0 {
			event.RegionUtilization = float64(event.HeapUsedRegionsAfter) / float64(event.HeapTotalRegions)
			totalRegionUtil += event.RegionUtilization
			regionUtilCount++
		}

		// Collection efficiency
		if event.HeapBefore > 0 && event.HeapAfter > 0 {
			collected := event.HeapBefore - event.HeapAfter
			event.CollectionEfficiency = float64(collected) / float64(event.HeapBefore)

			switch event.Type {
			case "Young":
				totalYoungEfficiency += event.CollectionEfficiency
				youngEfficiencyCount++
			case "Mixed":
				totalMixedEfficiency += event.CollectionEfficiency
				mixedEfficiencyCount++
			}
		}

		// Evacuation failure analysis
		if event.ToSpaceExhausted || strings.Contains(event.Cause, "Evacuation Failure") {
			event.HasEvacuationFailure = true
			analysis.EvacuationFailureCount++
		}

		// Pause time analysis (will compute target after full traversal)
		event.HasHighPauseTime = event.Duration > PausePoor

		// Survivor overflow detection
		if event.SurvivorRegionsAfter > 0 && event.SurvivorRegionsTarget > 0 {
			if event.SurvivorRegionsAfter >= event.SurvivorRegionsTarget {
				event.HasSurvivorOverflow = true
			}
		}

		// Phase timing analysis
		analyzePhaseTimings(event, &totalObjectCopy, &totalRootScan, &totalTermination, &totalRefProcessing,
			&objectCopyCount, &rootScanCount, &terminationCount, &refProcessingCount)

		// ===== ALLOCATION RATE CALCULATION =====
		if prevEvent != nil {
			interval := event.Timestamp.Sub(prevEvent.Timestamp)
			if interval > 0 {
				allocated := event.HeapBefore - prevEvent.HeapAfter
				if allocated > 0 {
					event.AllocationRateToEvent = allocated.MB() / interval.Seconds()

					allocationEvents = append(allocationEvents, allocationDataPoint{
						timestamp: event.Timestamp,
						rate:      event.AllocationRateToEvent,
						interval:  interval,
						allocated: allocated,
					})
				}
			}
		}

		// ===== PROMOTION ANALYSIS =====
		if prevEvent != nil && event.Type == "Young" {
			analyzePromotion(event, &promotionEvents, &consecutiveGrowthSpikes, &currentSpikeCount)
		}

		// ===== HUMONGOUS OBJECT ANALYSIS =====
		if event.HumongousRegionsBefore > 0 || event.HumongousRegionsAfter > 0 {
			analyzeHumongousObject(event, prevEvent, &humongousEvents)
		}

		// ===== MEMORY LEAK TREND DATA =====
		if event.HeapAfter > 0 {
			memoryTrendPoints = append(memoryTrendPoints, memoryTrendPoint{
				timestamp:   event.Timestamp,
				heapAfterMB: event.HeapAfter.MB(),
				heapTotalMB: event.HeapTotal.MB(),
			})
		}

		// ===== CONCURRENT MARK ABORTS =====
		if event.ConcurrentMarkAborted {
			analysis.ConcurrentMarkAbortCount++
		}

		prevEvent = event
	}

	// ===== POST-TRAVERSAL CALCULATIONS =====

	// Basic metrics
	analysis.TotalGCTime = totalGCTime
	if analysis.TotalRuntime > 0 {
		analysis.Throughput = (1.0 - float64(totalGCTime)/float64(analysis.TotalRuntime)) * 100.0
	}

	// Pause time statistics
	if len(durations) > 0 {
		slices.Sort(durations)
		analysis.MinPause = durations[0]
		analysis.MaxPause = durations[len(durations)-1]

		var sum time.Duration
		for _, d := range durations {
			sum += d
		}
		analysis.AvgPause = sum / time.Duration(len(durations))

		analysis.P95Pause = calculatePercentile(durations, 95)
		analysis.P99Pause = calculatePercentile(durations, 99)
		analysis.EstimatedPauseTarget = estimatePauseTarget()

		// Calculate pause target misses and long pauses
		calculatePauseAnalysis(events, analysis)
	}

	// Collection efficiency
	if youngEfficiencyCount > 0 {
		analysis.YoungCollectionEfficiency = totalYoungEfficiency / float64(youngEfficiencyCount)
	}
	if mixedEfficiencyCount > 0 {
		analysis.MixedCollectionEfficiency = totalMixedEfficiency / float64(mixedEfficiencyCount)
	}
	if analysis.YoungGCCount > 0 {
		analysis.MixedToYoungRatio = float64(analysis.MixedGCCount) / float64(analysis.YoungGCCount)
	}

	// Memory utilization
	if heapUtilCount > 0 {
		analysis.AvgHeapUtil = totalHeapUtil / heapUtilCount
	}
	if regionUtilCount > 0 {
		analysis.AvgRegionUtilization = totalRegionUtil / regionUtilCount
	}

	// Evacuation failure rate
	if analysis.TotalEvents > 0 {
		analysis.EvacuationFailureRate = float64(analysis.EvacuationFailureCount) / float64(analysis.TotalEvents)
	}

	// Phase analysis
	analysis.PhaseStats = calculatePhaseStats(totalObjectCopy, totalRootScan, totalTermination, totalRefProcessing,
		objectCopyCount, rootScanCount, terminationCount, refProcessingCount)

	// Allocation rate analysis
	analysis.AllocationRate = calculateAllocationRate(allocationEvents, analysis.TotalRuntime)
	analysis.AllocationBurstCount = calculateAllocationBursts(allocationEvents, analysis.AllocationRate)

	// Promotion analysis
	analysis.PromotionStats = calculatePromotionStats(promotionEvents, analysis.YoungGCCount)
	analysis.AvgPromotionRate = analysis.PromotionStats.AvgPromotionRate
	analysis.MaxPromotionRate = analysis.PromotionStats.MaxPromotionRate
	analysis.AvgOldGrowthRatio = analysis.PromotionStats.AvgOldGrowthRatio
	analysis.MaxOldGrowthRatio = analysis.PromotionStats.MaxOldGrowthRatio
	analysis.SurvivorOverflowRate = analysis.PromotionStats.SurvivorOverflowRate
	analysis.ConsecutiveGrowthSpikes = max(consecutiveGrowthSpikes, currentSpikeCount)

	// Humongous analysis
	analysis.HumongousStats = calculateHumongousStats(humongousEvents)

	// Memory leak analysis
	if len(memoryTrendPoints) >= MinEventsForTrend && analysis.TotalRuntime >= MinTimeForTrend {
		analysis.MemoryTrend = calculateMemoryTrend(memoryTrendPoints, events[0].Timestamp)
	}

	// Concurrent marking analysis
	analysis.ConcurrentMarkingKeepup = assessConcurrentMarkingKeepup(analysis.YoungGCCount, analysis.MixedGCCount)
	analysis.ConcurrentCycleDuration = estimateConcurrentCycleDuration(events)

	// Variance and advanced metrics
	analysis.PauseTimeVariance = calculatePauseVariance(durations, analysis.AvgPause)

	// ===== SET ISSUE FLAGS FOR RECOMMENDATIONS =====
	analysis.setIssueFlags()
}

// categorizeGCType categorizes GC types for time distribution analysis
func categorizeGCType(gcType string) string {
	eventType := strings.ToLower(gcType)
	switch {
	case strings.Contains(eventType, "young"):
		return "Young"
	case strings.Contains(eventType, "mixed"):
		return "Mixed"
	case strings.Contains(eventType, "full"):
		return "Full"
	case strings.Contains(eventType, "concurrent"),
		strings.Contains(eventType, "abort"):
		return "Concurrent Mark"
	default:
		return eventType
	}
}

// Supporting data structures for single-pass analysis
type allocationDataPoint struct {
	timestamp time.Time
	rate      float64
	interval  time.Duration
	allocated MemorySize
}

type promotionDataPoint struct {
	timestamp        time.Time
	promotedRegions  float64
	oldGrowthRatio   float64
	survivorOverflow bool
	efficiency       float64
}

type humongousDataPoint struct {
	timestamp     time.Time
	regionsBefore int
	regionsAfter  int
	heapPercent   float64
	trend         string // "growing", "static", "decreasing"
}

type memoryTrendPoint struct {
	timestamp   time.Time
	heapAfterMB float64
	heapTotalMB float64
}

// Analysis helper functions
func analyzePhaseTimings(event *GCEvent, totalObjectCopy, totalRootScan, totalTermination, totalRefProcessing *time.Duration,
	objectCopyCount, rootScanCount, terminationCount, refProcessingCount *int) {

	if event.ObjectCopyTime > 0 {
		*totalObjectCopy += event.ObjectCopyTime
		*objectCopyCount++
		event.HasSlowObjectCopy = event.ObjectCopyTime > ObjectCopyTarget
	}

	if event.ExtRootScanTime > 0 {
		*totalRootScan += event.ExtRootScanTime
		*rootScanCount++
		event.HasSlowRootScanning = event.ExtRootScanTime > RootScanTarget
	}

	if event.TerminationTime > 0 {
		*totalTermination += event.TerminationTime
		*terminationCount++
		event.HasSlowTermination = event.TerminationTime > TerminationTarget
	}

	if event.ReferenceProcessingTime > 0 {
		*totalRefProcessing += event.ReferenceProcessingTime
		*refProcessingCount++
		event.HasSlowRefProcessing = event.ReferenceProcessingTime > RefProcessingTarget
	}

	event.HasLongPhases = event.HasSlowObjectCopy || event.HasSlowRootScanning ||
		event.HasSlowTermination || event.HasSlowRefProcessing
}

func analyzePromotion(currEvent *GCEvent, promotionEvents *[]promotionDataPoint,
	consecutiveGrowthSpikes *int, currentSpikeCount *int) {

	if currEvent.OldRegionsBefore >= 0 && currEvent.OldRegionsAfter >= currEvent.OldRegionsBefore {
		promoted := float64(currEvent.OldRegionsAfter - currEvent.OldRegionsBefore)

		currEvent.PromotionRate = promoted

		// Track premature promotion
		if promoted > PromotionRateWarning {
			currEvent.IsPrematurePromotion = true
		}

		// Calculate growth ratio for trend analysis
		var growthRatio float64 = 1.0
		if currEvent.OldRegionsBefore > 0 {
			growthRatio = float64(currEvent.OldRegionsAfter) / float64(currEvent.OldRegionsBefore)
		}

		// Track consecutive growth spikes
		if growthRatio > OldRegionGrowthWarning {
			*currentSpikeCount++
		} else {
			if *currentSpikeCount > *consecutiveGrowthSpikes {
				*consecutiveGrowthSpikes = *currentSpikeCount
			}
			*currentSpikeCount = 0
		}

		*promotionEvents = append(*promotionEvents, promotionDataPoint{
			timestamp:        currEvent.Timestamp,
			promotedRegions:  promoted,
			oldGrowthRatio:   growthRatio,
			survivorOverflow: currEvent.HasSurvivorOverflow,
			efficiency:       currEvent.CollectionEfficiency,
		})
	}
}

func analyzeHumongousObject(event *GCEvent, prevEvent *GCEvent, humongousEvents *[]humongousDataPoint) {
	maxRegions := max(event.HumongousRegionsBefore, event.HumongousRegionsAfter)

	var heapPercent float64
	if event.HeapTotal > 0 && event.RegionSize > 0 {
		totalRegions := int(event.HeapTotal / event.RegionSize)
		if totalRegions > 0 {
			heapPercent = float64(maxRegions) / float64(totalRegions) * 100
		}
	}

	var trend string
	if prevEvent != nil {
		prevHumongous := max(prevEvent.HumongousRegionsBefore, prevEvent.HumongousRegionsAfter)
		if maxRegions > prevHumongous {
			trend = "growing"
		} else if maxRegions == prevHumongous {
			trend = "static"
		} else {
			trend = "decreasing"
		}
	}

	if heapPercent > HumongousPercentModerate {
		event.HasHumongousGrowth = true
	}

	*humongousEvents = append(*humongousEvents, humongousDataPoint{
		timestamp:     event.Timestamp,
		regionsBefore: event.HumongousRegionsBefore,
		regionsAfter:  event.HumongousRegionsAfter,
		heapPercent:   heapPercent,
		trend:         trend,
	})
}

// Calculation functions
func calculatePercentile(sortedDurations []time.Duration, percentile int) time.Duration {
	if len(sortedDurations) == 0 {
		return 0
	}

	index := float64(percentile) / 100.0 * float64(len(sortedDurations)-1)
	lower := int(index)
	upper := lower + 1

	if upper >= len(sortedDurations) {
		return sortedDurations[len(sortedDurations)-1]
	}

	weight := index - float64(lower)
	return time.Duration(float64(sortedDurations[lower])*(1-weight) + float64(sortedDurations[upper])*weight)
}

func estimatePauseTarget() time.Duration {
	return getDefaultTargetForAppType("api")
}

func getDefaultTargetForAppType(appType string) time.Duration {
	switch strings.ToLower(appType) {
	case "realtime", "trading", "gaming":
		return 10 * time.Millisecond
	case "web", "api", "microservice":
		return 50 * time.Millisecond
	case "enterprise", "application":
		return 100 * time.Millisecond
	case "batch", "analytics", "etl":
		return 200 * time.Millisecond
	default:
		return 50 * time.Millisecond
	}
}

func calculatePauseAnalysis(events []*GCEvent, analysis *GCAnalysis) {
	pauseTargetMisses := 0
	longPauseCount := 0

	for _, event := range events {

		if event.Duration > analysis.EstimatedPauseTarget {
			event.PauseTargetExceeded = true
			pauseTargetMisses++
		}

		if event.Duration > analysis.EstimatedPauseTarget*2 {
			longPauseCount++
		}
	}

	if analysis.TotalEvents > 0 {
		analysis.PauseTargetMissRate = float64(pauseTargetMisses) / float64(analysis.TotalEvents)
	}
	analysis.LongPauseCount = longPauseCount
}

// Continue with more helper functions...
func calculatePhaseStats(totalObjectCopy, totalRootScan, totalTermination, totalRefProcessing time.Duration,
	objectCopyCount, rootScanCount, terminationCount, refProcessingCount int) PhaseAnalysis {

	stats := PhaseAnalysis{}

	if objectCopyCount > 0 {
		stats.AvgObjectCopyTime = totalObjectCopy / time.Duration(objectCopyCount)
	}
	if rootScanCount > 0 {
		stats.AvgRootScanTime = totalRootScan / time.Duration(rootScanCount)
	}
	if terminationCount > 0 {
		stats.AvgTerminationTime = totalTermination / time.Duration(terminationCount)
	}
	if refProcessingCount > 0 {
		stats.AvgRefProcessingTime = totalRefProcessing / time.Duration(refProcessingCount)
	}

	stats.HasPhaseIssues = stats.AvgObjectCopyTime > ObjectCopyTarget ||
		stats.AvgRootScanTime > RootScanTarget ||
		stats.AvgTerminationTime > TerminationTarget ||
		stats.AvgRefProcessingTime > RefProcessingTarget

	return stats
}

func calculateAllocationRate(events []allocationDataPoint, totalRuntime time.Duration) float64 {
	if len(events) == 0 || totalRuntime == 0 {
		return 0
	}

	var totalAllocated MemorySize
	for _, event := range events {
		totalAllocated += event.allocated
	}

	runtimeSeconds := float64(totalRuntime) / float64(time.Second)
	return totalAllocated.MB() / runtimeSeconds
}

func calculateAllocationBursts(events []allocationDataPoint, avgRate float64) int {
	burstCount := 0
	burstThreshold := avgRate * 3 // 3x average rate

	for _, event := range events {
		if event.rate > burstThreshold {
			burstCount++
		}
	}

	return burstCount
}

func calculatePromotionStats(events []promotionDataPoint, youngGCCount int) PromotionAnalysis {
	stats := PromotionAnalysis{}

	if len(events) == 0 {
		return stats
	}

	var totalPromotion, totalGrowth float64
	var maxPromotion, maxGrowth float64
	overflowCount := 0

	for _, event := range events {
		totalPromotion += event.promotedRegions
		totalGrowth += event.oldGrowthRatio

		if event.promotedRegions > maxPromotion {
			maxPromotion = event.promotedRegions
		}
		if event.oldGrowthRatio > maxGrowth {
			maxGrowth = event.oldGrowthRatio
		}

		if event.survivorOverflow {
			overflowCount++
		}
	}

	stats.TotalPromotionEvents = len(events)
	stats.AvgPromotionRate = totalPromotion / float64(len(events))
	stats.MaxPromotionRate = maxPromotion
	stats.AvgOldGrowthRatio = totalGrowth / float64(len(events))
	stats.MaxOldGrowthRatio = maxGrowth
	stats.SurvivorOverflowCount = overflowCount

	if youngGCCount > 0 {
		stats.SurvivorOverflowRate = float64(overflowCount) / float64(youngGCCount)
	}

	// Calculate premature promotion rate
	prematureCount := 0
	for _, event := range events {
		if event.promotedRegions > PromotionRateWarning {
			prematureCount++
		}
	}

	if len(events) > 0 {
		stats.PrematurePromotionRate = float64(prematureCount) / float64(len(events))
	}

	return stats
}

func calculateHumongousStats(events []humongousDataPoint) HumongousObjectStats {
	stats := HumongousObjectStats{}

	if len(events) == 0 {
		return stats
	}

	maxRegions := 0
	maxHeapPercent := 0.0
	staticCount := 0
	growingCount := 0
	decreasingCount := 0

	for _, event := range events {
		eventMaxRegions := max(event.regionsBefore, event.regionsAfter)
		if eventMaxRegions > maxRegions {
			maxRegions = eventMaxRegions
		}

		if event.heapPercent > maxHeapPercent {
			maxHeapPercent = event.heapPercent
		}

		switch event.trend {
		case "static":
			staticCount++
		case "growing":
			growingCount++
		case "decreasing":
			decreasingCount++
		}
	}

	stats.MaxRegions = maxRegions
	stats.HeapPercentage = maxHeapPercent
	stats.StaticCount = staticCount
	stats.GrowingCount = growingCount
	stats.DecreasingCount = decreasingCount
	stats.TotalEvents = len(events)

	// Determine if this indicates a leak
	stats.IsLeak = (maxHeapPercent > HumongousPercentWarning) ||
		(staticCount > len(events)/2 && maxRegions > 100) ||
		(growingCount > decreasingCount && maxRegions > 50)

	return stats
}

func calculateMemoryTrend(points []memoryTrendPoint, startTime time.Time) MemoryTrend {
	if len(points) < MinEventsForTrend {
		return MemoryTrend{LeakSeverity: "none"}
	}

	var timePoints []float64
	var heapValues []float64

	for _, point := range points {
		timeHours := point.timestamp.Sub(startTime).Hours()
		timePoints = append(timePoints, timeHours)
		heapValues = append(heapValues, point.heapAfterMB)
	}

	slope, correlation := linearRegression(timePoints, heapValues)

	totalHeap := 0.0
	for _, point := range points {
		if point.heapTotalMB > totalHeap {
			totalHeap = point.heapTotalMB
		}
	}

	trend := MemoryTrend{
		GrowthRateMBPerHour: slope,
		GrowthRatePercent:   slope / totalHeap,
		BaselineGrowthRate:  slope,
		TrendConfidence:     correlation * correlation,
		SamplePeriod:        points[len(points)-1].timestamp.Sub(points[0].timestamp),
		EventCount:          len(points),
	}

	// Project time to heap exhaustion
	if slope > 0 && totalHeap > 0 {
		remainingHeap := totalHeap - heapValues[len(heapValues)-1]
		hoursToFull := remainingHeap / slope
		trend.ProjectedFullHeapTime = time.Duration(hoursToFull * float64(time.Hour))
	}

	// Determine severity
	if trend.TrendConfidence > LeakConfidenceThreshold {
		switch {
		case slope > LeakGrowthCritical:
			trend.LeakSeverity = "critical"
		case slope > LeakGrowthWarning:
			trend.LeakSeverity = "warning"
		default:
			trend.LeakSeverity = "none"
		}
	} else {
		trend.LeakSeverity = "none"
	}

	return trend
}

func assessConcurrentMarkingKeepup(youngGCCount, mixedGCCount int) bool {
	if youngGCCount == 0 {
		return true
	}

	const ExpectedMixedRatio = 0.1
	actualRatio := float64(mixedGCCount) / float64(youngGCCount)
	return actualRatio >= ExpectedMixedRatio
}

func estimateConcurrentCycleDuration(events []*GCEvent) time.Duration {
	var mixedCollectionTimestamps []time.Time
	for _, event := range events {
		if strings.Contains(strings.ToLower(event.Type), "mixed") {
			mixedCollectionTimestamps = append(mixedCollectionTimestamps, event.Timestamp)
		}
	}

	if len(mixedCollectionTimestamps) < 2 {
		return 0
	}

	totalInterval := time.Duration(0)
	for i := 1; i < len(mixedCollectionTimestamps); i++ {
		interval := mixedCollectionTimestamps[i].Sub(mixedCollectionTimestamps[i-1])
		totalInterval += interval
	}

	return totalInterval / time.Duration(len(mixedCollectionTimestamps)-1)
}

func calculatePauseVariance(durations []time.Duration, avgPause time.Duration) float64 {
	if len(durations) < 2 {
		return 0
	}

	variance := 0.0
	mean := float64(avgPause.Nanoseconds())

	for _, d := range durations {
		diff := float64(d.Nanoseconds()) - mean
		variance += diff * diff
	}
	variance /= float64(len(durations))

	if mean > 0 {
		return variance / (mean * mean) // Normalized variance
	}
	return 0
}

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

	denominator := n*sumXX - sumX*sumX
	if denominator == 0 {
		return 0, 0
	}

	slope = (n*sumXY - sumX*sumY) / denominator

	numerator := n*sumXY - sumX*sumY
	denominatorCorr := math.Sqrt((n*sumXX - sumX*sumX) * (n*sumYY - sumY*sumY))

	if denominatorCorr == 0 {
		correlation = 0
	} else {
		correlation = numerator / denominatorCorr
	}

	return slope, correlation
}

// Set issue flags based on computed metrics
func (analysis *GCAnalysis) setIssueFlags() {
	// Critical issues
	analysis.HasCriticalMemoryLeak = analysis.MemoryTrend.LeakSeverity == "critical" || analysis.FullGCCount >= 3
	analysis.HasCriticalEvacFailures = analysis.EvacuationFailureRate > 0.05 // 5%
	analysis.HasCriticalThroughput = analysis.Throughput < ThroughputCritical
	analysis.HasCriticalPauseTimes = analysis.MaxPause > PauseCritical
	analysis.HasCriticalPromotion = analysis.MaxOldGrowthRatio > OldRegionGrowthCritical || analysis.AvgPromotionRate > PromotionRateCritical
	analysis.HasCriticalHumongousLeak = analysis.HumongousStats.IsLeak && analysis.HumongousStats.HeapPercentage > HumongousPercentCritical
	analysis.HasCriticalConcurrentMarkAbort = analysis.ConcurrentMarkAbortCount >= 2

	// Warning issues
	analysis.HasWarningMemoryLeak = analysis.MemoryTrend.LeakSeverity == "warning"
	analysis.HasWarningEvacFailures = analysis.EvacuationFailureRate > EvacFailureMax && !analysis.HasCriticalEvacFailures
	analysis.HasWarningThroughput = analysis.Throughput < ThroughputGood && !analysis.HasCriticalThroughput
	analysis.HasWarningPauseTimes = analysis.P99Pause > analysis.EstimatedPauseTarget && !analysis.HasCriticalPauseTimes
	analysis.HasWarningPromotion = (analysis.MaxOldGrowthRatio > OldRegionGrowthWarning || analysis.AvgPromotionRate > PromotionRateWarning) && !analysis.HasCriticalPromotion
	analysis.HasWarningHumongousUsage = analysis.HumongousStats.HeapPercentage > HumongousPercentWarning && !analysis.HasCriticalHumongousLeak
	analysis.HasWarningConcurrentMark = !analysis.ConcurrentMarkingKeepup
	analysis.HasWarningAllocationRate = analysis.AllocationRate > AllocRateHigh
	analysis.HasWarningCollectionEff = analysis.MixedGCCount == 0 && analysis.YoungGCCount > 50

	// Info issues
	analysis.HasInfoAllocationPattern = analysis.AllocationRate > AllocRateModerate && !analysis.HasWarningAllocationRate
	analysis.HasInfoPhaseOptimization = analysis.PhaseStats.HasPhaseIssues
}
