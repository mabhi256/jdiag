package gc

import (
	"fmt"
	"time"
)

func (log *GCLog) PrintSummary(metrics *GCMetrics, outputFormat string) {
	switch outputFormat {
	case "cli":
		log.printCLISummary(metrics)
	case "tui":
	case "html":
	}
}

func (log *GCLog) printCLISummary(metrics *GCMetrics) {
	fmt.Printf("=== GC Log Summary ===\n")
	fmt.Printf("JVM Version: %s\n", log.JVMVersion)
	fmt.Printf("Heap Region Size: %s\n", log.HeapRegionSize)
	fmt.Printf("Maximum Heap Size: %s\n", log.HeapMax)
	fmt.Printf("Total Events: %d\n", log.TotalEvents)
	fmt.Printf("Time Range: %s to %s\n", log.StartTime.Format(time.RFC3339), log.EndTime.Format(time.RFC3339))
	fmt.Printf("Total Runtime: %v\n", metrics.TotalRuntime)

	fmt.Printf("\n=== Performance Metrics ===\n")
	fmt.Printf("Throughput: %.2f%%\n", metrics.Throughput)
	fmt.Printf("Total GC Time: %v\n", metrics.TotalGCTime)
	fmt.Printf("Average Pause: %v\n", metrics.AvgPause)
	fmt.Printf("Min Pause: %v\n", metrics.MinPause)
	fmt.Printf("Max Pause: %v\n", metrics.MaxPause)
	fmt.Printf("P95 Pause: %v\n", metrics.P95Pause)
	fmt.Printf("P99 Pause: %v\n", metrics.P99Pause)
	fmt.Printf("Allocation Rate: %.2f MB/s\n", metrics.AllocationRate)

	fmt.Printf("\n=== GC Type Breakdown ===\n")
	fmt.Printf("Young GC: %d\n", metrics.YoungGCCount)
	fmt.Printf("Mixed GC: %d\n", metrics.MixedGCCount)
	fmt.Printf("Full GC: %d\n", metrics.FullGCCount)
}
