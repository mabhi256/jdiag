package gc

import (
	"slices"
	"strings"
	"time"
)

func (log *GCLog) CalculateMetrics() *GCMetrics {
	if len(log.Events) == 0 {
		return &GCMetrics{}
	}

	metrics := &GCMetrics{}

	var totalGCTime time.Duration
	var durations []time.Duration
	var youngCount, mixedCount, fullCount int

	for _, event := range log.Events {
		totalGCTime += event.Duration
		durations = append(durations, event.Duration)

		eventType := strings.ToLower(strings.TrimSpace(event.Type))

		// Check for pause types that contain the key words
		if strings.Contains(eventType, "young") {
			youngCount++
		} else if strings.Contains(eventType, "mixed") {
			mixedCount++
		} else if strings.Contains(eventType, "full") {
			fullCount++
		}
	}

	metrics.TotalGCTime = totalGCTime
	metrics.TotalEvents = len(log.Events)
	metrics.TotalRuntime = log.EndTime.Sub(log.StartTime)
	metrics.YoungGCCount = youngCount
	metrics.MixedGCCount = mixedCount
	metrics.FullGCCount = fullCount

	// Calculate runtime and throughput
	if !log.StartTime.IsZero() && !log.EndTime.IsZero() {
		if metrics.TotalRuntime > 0 {
			metrics.Throughput = (1.0 - float64(totalGCTime)/float64(metrics.TotalRuntime)) * 100.0
		}
	}

	// Calculate pause time statistics
	if len(durations) > 0 {
		slices.Sort(durations)

		metrics.MinPause = durations[0]
		metrics.MaxPause = durations[len(durations)-1]

		// Calculate average
		var sum time.Duration
		for _, d := range durations {
			sum += d
		}
		metrics.AvgPause = sum / time.Duration(len(durations))

		// Calculate percentiles
		metrics.P95Pause = calculatePercentile(durations, 95)
		metrics.P99Pause = calculatePercentile(durations, 99)
	}

	// Calculate allocation rate (MB/second) using enhanced before/after data
	if metrics.TotalRuntime > 0 && len(log.Events) > 0 {
		var totalAllocated MemorySize
		for _, event := range log.Events {
			heapBefore := event.HeapBefore
			heapAfter := event.HeapAfter

			// Allocation is roughly heap growth + GC collection
			if heapBefore > heapAfter {
				totalAllocated += heapBefore.Sub(heapAfter)
			}
		}

		runtimeSeconds := float64(metrics.TotalRuntime) / float64(time.Second)
		metrics.AllocationRate = totalAllocated.MB() / runtimeSeconds
	}

	// Memory Pressure - Use last 10 events or all events if fewer than 10
	startIdx := max(metrics.TotalEvents-10, 0)
	recentEvents := log.Events[startIdx:]

	var totalUtilization float64
	validEvents := 0

	for _, event := range recentEvents {
		if event.HeapTotal > 0 {
			utilization := float64(event.HeapAfter) / float64(event.HeapTotal)
			totalUtilization += utilization
			validEvents++
		}
	}
	if validEvents > 0 {
		metrics.AvgHeapUtil = totalUtilization / float64(validEvents)
	}

	metrics.calculateAdvancedMetrics(log.Events)
	metrics.calculatePromotionMetrics(log.Events)
	metrics.calculateEvacuationFailureMetrics(log.Events)

	return metrics
}

func (metrics *GCMetrics) calculateAdvancedMetrics(events []GCEvent) {
	if len(events) == 0 {
		return
	}

	// Young collection efficiency
	youngCollections := 0
	totalYoungEfficiency := 0.0

	// Mixed collection efficiency
	mixedCollections := 0
	totalMixedEfficiency := 0.0

	// Evacuation failure tracking
	evacuationFailures := 0

	// Long pause counting
	pauseTarget := estimateBasicPauseTarget(events)
	longPauseCount := 0
	pauseTargetMisses := 0

	// Region utilization tracking
	totalRegionUtil := 0.0
	regionUtilSamples := 0

	// Allocation burst detection
	allocationBursts := 0

	for i, event := range events {
		eventType := strings.ToLower(strings.TrimSpace(event.Type))

		// Collection efficiency calculation
		if event.HeapBefore > 0 {
			collected := event.HeapBefore - event.HeapAfter
			efficiency := float64(collected) / float64(event.HeapBefore)

			if strings.Contains(eventType, "young") {
				youngCollections++
				totalYoungEfficiency += efficiency
			} else if strings.Contains(eventType, "mixed") {
				mixedCollections++
				totalMixedEfficiency += efficiency
			}
		}

		// Track evacuation failures
		if event.ToSpaceExhausted {
			evacuationFailures++
		}

		// Track pause target misses
		if event.Duration > pauseTarget {
			pauseTargetMisses++
		}

		// Track long pauses (significantly over target)
		if event.Duration > pauseTarget*2 {
			longPauseCount++
		}

		// Track region utilization
		if event.HeapTotalRegions > 0 {
			// Use after values for post-GC utilization
			var usedRegions int
			if event.HeapUsedRegionsAfter > 0 {
				usedRegions = event.HeapUsedRegionsAfter
			}

			utilization := float64(usedRegions) / float64(event.HeapTotalRegions)
			totalRegionUtil += utilization
			regionUtilSamples++
		}

		// Allocation burst heuristic
		if i > 0 {
			prevEvent := events[i-1]
			interval := event.Timestamp.Sub(prevEvent.Timestamp)
			if interval > 0 {
				allocated := event.HeapBefore - prevEvent.HeapAfter
				if allocated > 0 {
					allocationRate := float64(allocated) / interval.Seconds()
					// If allocation rate is 3x higher than average, consider it a burst
					if allocationRate > metrics.AllocationRate*3 {
						allocationBursts++
					}
				}
			}
		}
	}

	// Set collection efficiency metrics
	if youngCollections > 0 {
		metrics.YoungCollectionEfficiency = totalYoungEfficiency / float64(youngCollections)
	}

	if mixedCollections > 0 {
		metrics.MixedCollectionEfficiency = totalMixedEfficiency / float64(mixedCollections)
	}

	if youngCollections > 0 {
		metrics.MixedToYoungRatio = float64(mixedCollections) / float64(youngCollections)
	}

	// Set pause and failure metrics
	if len(events) > 0 {
		metrics.EvacuationFailureRate = float64(evacuationFailures) / float64(len(events))
		metrics.PauseTargetMissRate = float64(pauseTargetMisses) / float64(len(events))
	}

	metrics.LongPauseCount = longPauseCount

	// Set region and allocation metrics
	if regionUtilSamples > 0 {
		metrics.AvgRegionUtilization = totalRegionUtil / float64(regionUtilSamples)
	}

	metrics.RegionExhaustionEvents = evacuationFailures
	metrics.AllocationBurstCount = allocationBursts

	// Calculate pause variance and concurrent metrics
	metrics.PauseTimeVariance = calculatePauseVariance(events)
	metrics.ConcurrentMarkingKeepup = assessConcurrentMarkingKeepup(events)
	metrics.ConcurrentCycleDuration = estimateConcurrentCycleDuration(events)

	// Calculate concurrent cycle frequency
	if metrics.TotalRuntime > 0 {
		// Estimate based on mixed collections frequency
		mixedCollectionFreq := float64(mixedCollections) / metrics.TotalRuntime.Hours()
		metrics.ConcurrentCycleFrequency = mixedCollectionFreq * 0.5
	}
}

func (metrics *GCMetrics) calculatePromotionMetrics(events []GCEvent) {
	var youngCollections []GCEvent
	var mixedCollections []GCEvent

	// Separate young and mixed collections
	for _, event := range events {
		eventType := strings.ToLower(strings.TrimSpace(event.Type))
		if strings.Contains(eventType, "young") {
			youngCollections = append(youngCollections, event)
		} else if strings.Contains(eventType, "mixed") {
			mixedCollections = append(mixedCollections, event)
		}
	}

	if len(youngCollections) < 2 {
		return
	}

	var promotionRates []float64
	var oldRegionGrowths []float64
	var youngEfficiencies []float64
	survivorOverflows := 0
	consecutiveSpikes := 0
	currentSpikeCount := 0

	// Analyze young collection patterns
	for i := 1; i < len(youngCollections); i++ {
		prev := youngCollections[i-1]
		curr := youngCollections[i]

		var prevOldRegions, currOldRegions int
		if prev.OldRegionsAfter > 0 {
			prevOldRegions = prev.OldRegionsAfter
		}
		if curr.OldRegionsBefore > 0 {
			currOldRegions = curr.OldRegionsBefore
		}

		if prevOldRegions > 0 && currOldRegions >= prevOldRegions {
			promoted := float64(currOldRegions - prevOldRegions)
			promotionRates = append(promotionRates, promoted)

			growthRatio := float64(currOldRegions) / float64(prevOldRegions)
			oldRegionGrowths = append(oldRegionGrowths, growthRatio)

			// Track consecutive growth spikes
			const OldRegionGrowthWarning = 1.5
			if growthRatio > OldRegionGrowthWarning {
				currentSpikeCount++
			} else {
				if currentSpikeCount > consecutiveSpikes {
					consecutiveSpikes = currentSpikeCount
				}
				currentSpikeCount = 0
			}
		}

		// Calculate young generation efficiency
		if curr.HeapBefore > 0 && curr.HeapAfter > 0 {
			collected := curr.HeapBefore - curr.HeapAfter
			efficiency := float64(collected) / float64(curr.HeapBefore)
			youngEfficiencies = append(youngEfficiencies, efficiency)
		}

		// Detect survivor overflow (heuristic: rapid heap growth + low young efficiency)
		if len(promotionRates) > 0 && len(youngEfficiencies) > 0 {
			lastPromotion := promotionRates[len(promotionRates)-1]
			lastEfficiency := youngEfficiencies[len(youngEfficiencies)-1]

			// Enhanced heuristic: check survivor region overflow
			survivorOverflow := false

			// Method 1: Check if survivors exceeded target
			if curr.SurvivorRegionsAfter > 0 && curr.SurvivorRegionsTarget > 0 {
				if curr.SurvivorRegionsAfter >= curr.SurvivorRegionsTarget {
					survivorOverflow = true
				}
			}

			// Method 2: Fallback to promotion rate + efficiency heuristic
			if !survivorOverflow {
				promotionThreshold := 5.0
				if prevOldRegions == 0 {
					promotionThreshold = 20.0 // Less precise calculation
				}
				if lastPromotion > promotionThreshold && lastEfficiency < 0.7 {
					survivorOverflow = true
				}
			}

			if survivorOverflow {
				survivorOverflows++
			}
		}
	}

	// Update final consecutive count
	if currentSpikeCount > consecutiveSpikes {
		consecutiveSpikes = currentSpikeCount
	}

	metrics.ConsecutiveGrowthSpikes = consecutiveSpikes

	if len(promotionRates) > 0 {
		metrics.AvgPromotionRate = calculateAverage(promotionRates)
		metrics.MaxPromotionRate = calculateMax(promotionRates)
	}

	if len(oldRegionGrowths) > 0 {
		metrics.AvgOldGrowthRatio = calculateAverage(oldRegionGrowths)
		metrics.MaxOldGrowthRatio = calculateMax(oldRegionGrowths)
	}

	if len(youngCollections) > 0 {
		metrics.SurvivorOverflowRate = float64(survivorOverflows) / float64(len(youngCollections))
	}

	// Calculate promotion efficiency (how much promoted data gets cleaned up)
	if len(mixedCollections) > 0 && len(promotionRates) > 0 {
		totalPromoted := calculateSum(promotionRates)
		totalMixedCleaned := 0.0

		for _, mixed := range mixedCollections {
			if mixed.HeapBefore > 0 && mixed.HeapAfter > 0 {
				cleaned := float64(mixed.HeapBefore - mixed.HeapAfter)
				totalMixedCleaned += cleaned
			}
		}

		if totalPromoted > 0 {
			// Enhanced calculation: use actual region size if available
			regionSizeBytes := float64(1024 * 1024) // Default 1MB regions
			if len(mixedCollections) > 0 && mixedCollections[0].RegionSize > 0 {
				regionSizeBytes = float64(mixedCollections[0].RegionSize.Bytes())
			}

			totalPromotedBytes := totalPromoted * regionSizeBytes
			metrics.PromotionEfficiency = totalMixedCleaned / totalPromotedBytes
		}
	}
}

func (metrics *GCMetrics) calculateEvacuationFailureMetrics(events []GCEvent) {
	evacuationFailures := 0

	for _, event := range events {
		if event.ToSpaceExhausted || strings.Contains(event.Cause, "Evacuation Failure") {
			evacuationFailures++
		}
	}

	if len(events) > 0 {
		metrics.EvacuationFailureRate = float64(evacuationFailures) / float64(len(events))
	}
}

// calculatePercentile calculates the nth percentile of sorted durations
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

// Helper functions for G1GC metrics calculation
func estimateBasicPauseTarget(events []GCEvent) time.Duration {
	if len(events) < 10 {
		return 200 * time.Millisecond // Default G1GC target
	}

	var durations []time.Duration
	for _, event := range events {
		durations = append(durations, event.Duration)
	}

	slices.Sort(durations)

	// Use 90th percentile as estimated target
	p90Index := int(float64(len(durations)) * 0.9)
	return durations[p90Index]
}

func calculatePauseVariance(events []GCEvent) float64 {
	if len(events) < 2 {
		return 0
	}

	// Calculate variance in pause times
	var durations []float64
	for _, event := range events {
		durations = append(durations, float64(event.Duration.Nanoseconds()))
	}

	mean := 0.0
	for _, d := range durations {
		mean += d
	}
	mean /= float64(len(durations))

	variance := 0.0
	for _, d := range durations {
		variance += (d - mean) * (d - mean)
	}
	variance /= float64(len(durations))

	// Return coefficient of variation
	if mean > 0 {
		return (variance / (mean * mean)) // Normalized variance
	}
	return 0
}

func assessConcurrentMarkingKeepup(events []GCEvent) bool {
	youngCollections := 0
	mixedCollections := 0

	for _, event := range events {
		eventType := strings.ToLower(strings.TrimSpace(event.Type))
		if strings.Contains(eventType, "young") {
			youngCollections++
		} else if strings.Contains(eventType, "mixed") {
			mixedCollections++
		}
	}

	if youngCollections == 0 {
		return true // No collections to assess
	}

	// If we have reasonable mixed collection activity, marking is likely keeping up
	const ExpectedMixedRatio = 0.1 // Expect at least 10% mixed collections
	actualRatio := float64(mixedCollections) / float64(youngCollections)

	return actualRatio >= ExpectedMixedRatio
}

func estimateConcurrentCycleDuration(events []GCEvent) time.Duration {
	// Simplified estimation - would need actual concurrent cycle tracking
	// For now, estimate based on interval between mixed collections

	var mixedCollectionTimestamps []time.Time
	for _, event := range events {
		eventType := strings.ToLower(strings.TrimSpace(event.Type))
		if strings.Contains(eventType, "mixed") {
			mixedCollectionTimestamps = append(mixedCollectionTimestamps, event.Timestamp)
		}
	}

	if len(mixedCollectionTimestamps) < 2 {
		return 0
	}

	// Calculate average interval between mixed collections as a proxy
	totalInterval := time.Duration(0)
	for i := 1; i < len(mixedCollectionTimestamps); i++ {
		interval := mixedCollectionTimestamps[i].Sub(mixedCollectionTimestamps[i-1])
		totalInterval += interval
	}

	return totalInterval / time.Duration(len(mixedCollectionTimestamps)-1)
}

// Helper functions
func calculateAverage(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func calculateMax(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	max := values[0]
	for _, v := range values {
		if v > max {
			max = v
		}
	}
	return max
}

func calculateSum(values []float64) float64 {
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum
}
