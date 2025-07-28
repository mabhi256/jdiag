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

		switch eventType {
		case "young":
			youngCount++
		case "mixed":
			mixedCount++
		case "full":
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

	// Calculate allocation rate (MB/second)
	if metrics.TotalRuntime > 0 && len(log.Events) > 0 {
		var totalAllocated MemorySize
		for _, event := range log.Events {
			// Allocation is roughly heap growth + GC collection
			if event.HeapBefore > event.HeapAfter {
				totalAllocated += event.HeapBefore.Sub(event.HeapAfter)
			}
		}

		runtimeSeconds := float64(metrics.TotalRuntime) / float64(time.Second)
		metrics.AllocationRate = totalAllocated.MB() / runtimeSeconds
	}

	// Memory Pressure
	// Use last 10 events or all events if fewer than 10
	startIdx := metrics.TotalEvents - 10
	if startIdx < 0 {
		startIdx = 0
	}
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

	if validEvents == 0 {
		metrics.AvgHeapUtil = 0
	}

	metrics.AvgHeapUtil = totalUtilization / float64(validEvents)

	return metrics
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
