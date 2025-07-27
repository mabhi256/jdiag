package gc

import (
	"fmt"
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
	metrics.YoungGCCount = youngCount
	metrics.MixedGCCount = mixedCount
	metrics.FullGCCount = fullCount

	// Calculate runtime and throughput
	if !log.StartTime.IsZero() && !log.EndTime.IsZero() {
		metrics.TotalRuntime = log.EndTime.Sub(log.StartTime)
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

func (log *GCLog) AnalyzePerformanceIssues(metrics *GCMetrics) {
	fmt.Printf("\n=== Performance Analysis ===\n")

	// Check throughput
	if metrics.Throughput < 90.0 {
		fmt.Printf("⚠️  LOW THROUGHPUT: %.2f%% (target: >90%%)\n", metrics.Throughput)
	} else {
		fmt.Printf("✅ Good throughput: %.2f%%\n", metrics.Throughput)
	}

	// Check for long pauses
	longPauseThreshold := 100 * time.Millisecond
	if metrics.MaxPause > longPauseThreshold {
		fmt.Printf("⚠️  LONG GC PAUSES: Max pause %v exceeds %v threshold\n",
			metrics.MaxPause, longPauseThreshold)
	}

	// Check for frequent full GCs
	if metrics.FullGCCount > 0 {
		fullGCRate := float64(metrics.FullGCCount) / float64(metrics.TotalRuntime.Hours())
		if fullGCRate > 1.0 { // More than 1 full GC per hour
			fmt.Printf("⚠️  FREQUENT FULL GC: %d full GCs in %v (%.2f/hour)\n",
				metrics.FullGCCount, metrics.TotalRuntime, fullGCRate)
		}
	}

	// Check allocation rate
	if metrics.AllocationRate > 1000.0 { // > 1GB/s
		fmt.Printf("⚠️  HIGH ALLOCATION RATE: %.2f MB/s\n", metrics.AllocationRate)
	}

	// Check for memory pressure
	events := log.Events
	if len(events) > 0 {
		recentEvents := events[max(0, len(events)-10):] // Last 10 events
		var avgHeapUtil float64
		for _, event := range recentEvents {
			if event.HeapTotal > 0 {
				utilization := float64(event.HeapAfter) / float64(event.HeapTotal)
				avgHeapUtil += utilization
			}
		}
		avgHeapUtil /= float64(len(recentEvents))

		if avgHeapUtil > 0.8 { // > 80% heap utilization
			fmt.Printf("⚠️  HIGH HEAP UTILIZATION: %.1f%% average in recent GCs\n",
				avgHeapUtil*100)
		}
	}
}
