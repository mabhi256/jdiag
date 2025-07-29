package gc

import (
	"fmt"
	"strings"
	"time"
)

func (log *GCLog) PrintReport(metrics *GCMetrics, issues []PerformanceIssue, outputFormat string) {
	if metrics == nil {
		fmt.Println("Error: No metrics available for report")
		return
	}

	switch outputFormat {
	case "cli":
		log.printSummary(metrics)
	case "cli-more":
		log.printDetailed(metrics)
		log.PrintRecommendations(issues)
	case "tui":
		fmt.Println("TUI format not yet implemented")
	case "html":
		fmt.Println("HTML format not yet implemented")
	default:
		fmt.Printf("Unknown output format '%s', using summary format\n\n", outputFormat)
		log.printSummary(metrics)
	}
}

func (log *GCLog) printSummary(metrics *GCMetrics) {
	duration := log.EndTime.Sub(log.StartTime).Round(time.Millisecond)

	// Header
	fmt.Printf("🔍 GC Performance Analysis\n")
	fmt.Printf("Heap Size: %s  |  Events: %d  |  Duration: %v\n",
		log.HeapMax, len(log.Events), duration)
	fmt.Println(strings.Repeat("═", 65))

	// Performance Overview
	fmt.Println("\n📈 PERFORMANCE SUMMARY")
	fmt.Println(strings.Repeat("─", 35))

	throughputIcon, throughputStatus := getThroughputStatusWithIcon(metrics.Throughput)
	gcTimeMs := metrics.TotalGCTime.Milliseconds()

	fmt.Printf("%s Application Throughput: %.1f%% (%s)\n",
		throughputIcon, metrics.Throughput, throughputStatus)
	fmt.Printf("   GC Overhead: %dms of %v total runtime (%.1f%%)\n\n",
		gcTimeMs, metrics.TotalRuntime, 100.0-metrics.Throughput)

	// Pause Time Analysis
	fmt.Println("⏱️  RESPONSE TIME IMPACT")
	fmt.Println(strings.Repeat("─", 35))

	maxPauseMs := float64(metrics.MaxPause.Nanoseconds()) / 1e6
	avgPauseMs := float64(metrics.AvgPause.Nanoseconds()) / 1e6
	p95PauseMs := float64(metrics.P95Pause.Nanoseconds()) / 1e6
	p99PauseMs := float64(metrics.P99Pause.Nanoseconds()) / 1e6

	pauseIcon, pauseAssessment := getPauseAssessmentWithIcon(metrics.MaxPause)

	fmt.Printf("%s Maximum Pause: %.1fms (%s)\n", pauseIcon, maxPauseMs, pauseAssessment)
	fmt.Printf("   Average Pause: %.1fms\n", avgPauseMs)
	fmt.Printf("   95th Percentile: %.1fms\n", p95PauseMs)
	fmt.Printf("   99th Percentile: %.1fms\n\n", p99PauseMs)

	// Collection Breakdown
	fmt.Println("🔄 COLLECTION BREAKDOWN")
	fmt.Println(strings.Repeat("─", 35))

	if metrics.YoungGCCount > 0 {
		avgYoungMs := float64(metrics.TotalGCTime.Nanoseconds()) / float64(metrics.YoungGCCount) / 1e6
		fmt.Printf("✅ Young Generation: %d collections (%.1fms avg)\n",
			metrics.YoungGCCount, avgYoungMs)
	}

	if metrics.MixedGCCount > 0 {
		fmt.Printf("🟡 Mixed Collections: %d collections\n", metrics.MixedGCCount)
	}

	if metrics.FullGCCount > 0 {
		fmt.Printf("🔴 Full GC Events: %d collections", metrics.FullGCCount)
		if metrics.FullGCCount > 0 {
			fmt.Printf(" ⚠️  Memory pressure indicator")
		}
		fmt.Println()
	}

	if metrics.AllocationRate > 0 {
		fmt.Printf("📊 Allocation Rate: %.1f MB/sec\n", metrics.AllocationRate)
	}

	// G1GC-specific metrics if available
	if metrics.YoungCollectionEfficiency > 0 {
		fmt.Printf("⚡ Young Collection Efficiency: %.1f%%\n", metrics.YoungCollectionEfficiency*100)
	}
	if metrics.MixedCollectionEfficiency > 0 {
		fmt.Printf("🔄 Mixed Collection Efficiency: %.1f%%\n", metrics.MixedCollectionEfficiency*100)
	}

	// Status
	fmt.Printf("\n🎯 Overall Assessment: %s\n", log.Status)
}

func (log *GCLog) printDetailed(metrics *GCMetrics) {
	duration := log.EndTime.Sub(log.StartTime)

	fmt.Println("═══════════════════════════════════════════════════════════════════")
	fmt.Println("🔍                  COMPREHENSIVE GC ANALYSIS                       ")
	fmt.Println("═══════════════════════════════════════════════════════════════════")
	fmt.Println()

	// System Configuration
	fmt.Println("⚙️  SYSTEM CONFIGURATION")
	fmt.Println(strings.Repeat("─", 50))
	fmt.Printf("JVM Version:        %s\n", log.JVMVersion)
	fmt.Printf("Maximum Heap Size:  %s\n", log.HeapMax)
	fmt.Printf("Heap Region Size:   %s\n", log.HeapRegionSize)
	fmt.Println()

	// Performance Metrics
	fmt.Println("📊 PERFORMANCE METRICS")
	fmt.Println(strings.Repeat("─", 50))
	fmt.Printf("Application Runtime:    %v\n", metrics.TotalRuntime.Round(time.Millisecond))
	fmt.Printf("Total GC Time:          %v\n", metrics.TotalGCTime.Round(time.Millisecond))
	fmt.Printf("GC Overhead:            %.2f%%\n", 100.0-metrics.Throughput)
	fmt.Printf("Application Throughput: %.2f%%\n", metrics.Throughput)

	if metrics.AllocationRate > 0 {
		fmt.Printf("Allocation Rate:        %.2f MB/sec", metrics.AllocationRate)
		if metrics.AllocationRate > 100 {
			fmt.Printf(" ⚠️  [High - consider optimization]")
		} else if metrics.AllocationRate > 50 {
			fmt.Printf(" [Moderate]")
		}
		fmt.Println()
	}

	// G1GC-specific metrics
	if metrics.YoungCollectionEfficiency > 0 || metrics.MixedCollectionEfficiency > 0 {
		fmt.Println()
		fmt.Println("🔧 G1GC COLLECTION EFFICIENCY")
		fmt.Println(strings.Repeat("─", 50))
		if metrics.YoungCollectionEfficiency > 0 {
			fmt.Printf("Young Generation:       %.1f%% efficiency\n", metrics.YoungCollectionEfficiency*100)
		}
		if metrics.MixedCollectionEfficiency > 0 {
			fmt.Printf("Mixed Collections:      %.1f%% efficiency\n", metrics.MixedCollectionEfficiency*100)
		}
		if metrics.MixedToYoungRatio > 0 {
			fmt.Printf("Mixed/Young Ratio:      %.1f%% mixed collections\n", metrics.MixedToYoungRatio*100)
		}
		if metrics.PauseTimeVariance > 0 {
			fmt.Printf("Pause Time Variance:    %.1f%% coefficient of variation\n", metrics.PauseTimeVariance*100)
		}
		if metrics.PauseTargetMissRate > 0 {
			fmt.Printf("Pause Target Miss Rate: %.1f%% collections exceed target\n", metrics.PauseTargetMissRate*100)
		}
		if metrics.EvacuationFailureRate > 0 {
			fmt.Printf("Evacuation Failures:    %.1f%% of collections\n", metrics.EvacuationFailureRate*100)
		}
	}

	fmt.Println()

	// Detailed Pause Analysis
	fmt.Println("⏱️  PAUSE TIME ANALYSIS")
	fmt.Println(strings.Repeat("─", 50))
	fmt.Printf("Total Collections:     %d\n", metrics.TotalEvents)

	if metrics.TotalEvents > 0 {
		avgFreq := duration.Seconds() / float64(metrics.TotalEvents)
		minPause := float64(metrics.MinPause.Nanoseconds()) / 1e6
		maxPause := float64(metrics.MaxPause.Nanoseconds()) / 1e6
		avgPause := float64(metrics.AvgPause.Nanoseconds()) / 1e6

		fmt.Printf("Collection Frequency:  Every %.1f seconds\n", avgFreq)
		fmt.Printf("Min/Max/Avg Pause:     %.2fms/%.2fms/%.2fms\n", minPause, maxPause, avgPause)

		maxPauseMs := float64(metrics.MaxPause.Nanoseconds()) / 1e6
		if maxPauseMs > 500 {
			fmt.Printf(" 🔴 [Critical - User impact]")
		} else if maxPauseMs > 100 {
			fmt.Printf(" ⚠️  [Warning - Noticeable delay]")
		}
		fmt.Println()

		fmt.Printf("95th Percentile:       %.2fms\n", float64(metrics.P95Pause.Nanoseconds())/1e6)
		fmt.Printf("99th Percentile:       %.2fms\n", float64(metrics.P99Pause.Nanoseconds())/1e6)
	}
	fmt.Println()

	// Collection Type Analysis
	fmt.Println("🔄 COLLECTION TYPE BREAKDOWN")
	fmt.Println(strings.Repeat("─", 50))
	totalEvents := metrics.YoungGCCount + metrics.MixedGCCount + metrics.FullGCCount

	if totalEvents > 0 {
		if metrics.YoungGCCount > 0 {
			youngPct := float64(metrics.YoungGCCount) / float64(totalEvents) * 100
			fmt.Printf("✅ Young Generation:      %3d collections (%.1f%%)\n",
				metrics.YoungGCCount, youngPct)
			fmt.Printf("                          Efficient cleanup of short-lived objects\n")
		}

		if metrics.MixedGCCount > 0 {
			mixedPct := float64(metrics.MixedGCCount) / float64(totalEvents) * 100
			fmt.Printf("🟡 Mixed Collections:     %3d collections (%.1f%%)\n",
				metrics.MixedGCCount, mixedPct)
			fmt.Printf("                          Maintenance of older generation objects\n")
		}

		if metrics.FullGCCount > 0 {
			fullPct := float64(metrics.FullGCCount) / float64(totalEvents) * 100
			fmt.Printf("🔴 Full GC Events:        %3d collections (%.1f%%)",
				metrics.FullGCCount, fullPct)

			if fullPct > 10 {
				fmt.Printf(" [Critical - Frequent memory pressure]")
			} else if fullPct > 5 {
				fmt.Printf(" ⚠️  [Warning - Monitor memory usage]")
			} else {
				fmt.Printf(" [Acceptable frequency]")
			}
			fmt.Println()
			fmt.Printf("                          Emergency cleanup - indicates heap pressure\n")
		}
	} else {
		fmt.Println("No GC events detected in analysis period")
	}
	fmt.Println()

	// G1GC Region Analysis (if available)
	if metrics.AvgRegionUtilization > 0 {
		fmt.Println("🏗️  G1GC REGION ANALYSIS")
		fmt.Println(strings.Repeat("─", 50))
		fmt.Printf("Average Region Utilization: %.1f%%", metrics.AvgRegionUtilization*100)
		if metrics.AvgRegionUtilization > 0.85 {
			fmt.Printf(" ⚠️  [High - may cause evacuation pressure]")
		} else if metrics.AvgRegionUtilization > 0.7 {
			fmt.Printf(" [Moderate]")
		} else {
			fmt.Printf(" [Good]")
		}
		fmt.Println()

		if metrics.AllocationBurstCount > 0 {
			fmt.Printf("Allocation Bursts:          %d detected\n", metrics.AllocationBurstCount)
		}
		fmt.Println()
	}
}

// Clean helper functions for professional output
func getThroughputStatusWithIcon(throughput float64) (string, string) {
	if throughput >= 99 {
		return "✅", "Excellent"
	} else if throughput >= 95 {
		return "✅", "Good"
	} else if throughput >= 90 {
		return "⚠️", "Fair - Monitor"
	} else {
		return "🔴", "Poor - Action needed"
	}
}

func getPauseAssessmentWithIcon(maxPause time.Duration) (string, string) {
	maxPauseMs := float64(maxPause.Nanoseconds()) / 1e6
	if maxPauseMs > 500 {
		return "🔴", "Critical impact"
	} else if maxPauseMs > 100 {
		return "⚠️", "Noticeable delay"
	} else if maxPauseMs > 50 {
		return "⚠️", "Minor impact"
	} else {
		return "✅", "Minimal impact"
	}
}

func (log *GCLog) PrintRecommendations(issues []PerformanceIssue) {
	if len(issues) == 0 {
		fmt.Println("\n💡 RECOMMENDATIONS")
		fmt.Println(strings.Repeat("─", 50))
		fmt.Println("✅ No performance issues detected.")
		fmt.Println("   Current GC configuration appears optimal.")
		fmt.Println("   Continue monitoring for any changes in application behavior.")
		return
	}

	fmt.Println("\n🚀 PERFORMANCE RECOMMENDATIONS")
	fmt.Println(strings.Repeat("─", 50))

	// Group by severity
	critical := []PerformanceIssue{}
	warnings := []PerformanceIssue{}
	info := []PerformanceIssue{}

	for _, issue := range issues {
		switch issue.Severity {
		case "critical":
			critical = append(critical, issue)
		case "warning":
			warnings = append(warnings, issue)
		default:
			info = append(info, issue)
		}
	}

	// Critical issues
	if len(critical) > 0 {
		fmt.Println("\n🚩 CRITICAL ISSUES - Immediate attention required:")
		for _, issue := range critical {
			fmt.Printf("\n🔴 %s\n", issue.Type)
			fmt.Printf("   Issue: %s\n", issue.Description)
			fmt.Println("   Recommended actions:")
			log.printFormattedRecommendations(issue.Recommendation)
		}
	}

	// Warning issues
	if len(warnings) > 0 {
		fmt.Println("\n⚠️  WARNINGS - Address when possible:")
		for _, issue := range warnings {
			fmt.Printf("\n🟡 %s\n", issue.Type)
			fmt.Printf("   Concern: %s\n", issue.Description)
			fmt.Println("   Suggested improvements:")
			log.printFormattedRecommendations(issue.Recommendation)
		}
	}

	// Optimization opportunities
	if len(info) > 0 {
		fmt.Println("\n📈 OPTIMIZATION OPPORTUNITIES:")
		for _, issue := range info {
			fmt.Printf("\n💡 %s\n", issue.Type)
			fmt.Printf("   Note: %s\n", issue.Description)
			log.printFormattedRecommendations(issue.Recommendation)
		}
	}
}

func (log *GCLog) printFormattedRecommendations(recommendations []string) {
	for _, rec := range recommendations {
		trimmed := strings.TrimSpace(rec)

		// Skip empty lines
		if trimmed == "" {
			continue
		}

		fmt.Printf("   • %s\n", trimmed)
	}
}
