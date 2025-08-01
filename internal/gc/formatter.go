package gc

import (
	"fmt"
	"strings"
	"time"
)

func (analysis *GCAnalysis) PrintSummary() {
	duration := analysis.EndTime.Sub(analysis.StartTime).Round(time.Millisecond)

	// Header
	fmt.Printf("🔍 GC Performance Analysis\n")
	fmt.Printf("Heap Size: %s  |  Events: %d  |  Duration: %v\n",
		analysis.HeapMax, analysis.TotalEvents, duration)
	fmt.Println(strings.Repeat("═", 65))

	// Performance Overview
	fmt.Println("\n📈 PERFORMANCE SUMMARY")
	fmt.Println(strings.Repeat("─", 35))

	throughputIcon, throughputStatus := getThroughputStatusWithIcon(analysis.Throughput)
	gcTimeMs := analysis.TotalGCTime.Milliseconds()

	fmt.Printf("%s Application Throughput: %.1f%% (%s)\n",
		throughputIcon, analysis.Throughput, throughputStatus)
	fmt.Printf("   GC Overhead: %dms of %v total runtime (%.1f%%)\n\n",
		gcTimeMs, analysis.TotalRuntime, 100.0-analysis.Throughput)

	// Pause Time Analysis
	fmt.Println("⏱️  RESPONSE TIME IMPACT")
	fmt.Println(strings.Repeat("─", 35))

	maxPauseMs := float64(analysis.MaxPause.Nanoseconds()) / 1e6
	avgPauseMs := float64(analysis.AvgPause.Nanoseconds()) / 1e6
	p95PauseMs := float64(analysis.P95Pause.Nanoseconds()) / 1e6
	p99PauseMs := float64(analysis.P99Pause.Nanoseconds()) / 1e6

	pauseIcon, pauseAssessment := getPauseAssessmentWithIcon(analysis.MaxPause)

	fmt.Printf("%s Maximum Pause: %.1fms (%s)\n", pauseIcon, maxPauseMs, pauseAssessment)
	fmt.Printf("   Average Pause: %.1fms\n", avgPauseMs)
	fmt.Printf("   95th Percentile: %.1fms\n", p95PauseMs)
	fmt.Printf("   99th Percentile: %.1fms\n\n", p99PauseMs)

	// Collection Breakdown
	fmt.Println("🔄 COLLECTION BREAKDOWN")
	fmt.Println(strings.Repeat("─", 35))

	if analysis.YoungGCCount > 0 {
		avgYoungMs := float64(analysis.TotalGCTime.Nanoseconds()) / float64(analysis.YoungGCCount) / 1e6
		fmt.Printf("✅ Young Generation: %d collections (%.1fms avg)\n",
			analysis.YoungGCCount, avgYoungMs)
	}

	if analysis.MixedGCCount > 0 {
		fmt.Printf("🟡 Mixed Collections: %d collections\n", analysis.MixedGCCount)
	}

	if analysis.FullGCCount > 0 {
		fmt.Printf("🔴 Full GC Events: %d collections", analysis.FullGCCount)
		if analysis.FullGCCount > 0 {
			fmt.Printf(" ⚠️  Memory pressure indicator")
		}
		fmt.Println()
	}

	if analysis.AllocationRate > 0 {
		fmt.Printf("📊 Allocation Rate: %.1f MB/sec\n", analysis.AllocationRate)
	}

	// G1GC-specific metrics if available
	if analysis.YoungCollectionEfficiency > 0 {
		fmt.Printf("⚡ Young Collection Efficiency: %.1f%%\n", analysis.YoungCollectionEfficiency*100)
	}
	if analysis.MixedCollectionEfficiency > 0 {
		fmt.Printf("🔄 Mixed Collection Efficiency: %.1f%%\n", analysis.MixedCollectionEfficiency*100)
	}
}

func (analysis *GCAnalysis) PrintDetailed() {
	duration := analysis.EndTime.Sub(analysis.StartTime)

	fmt.Println("═══════════════════════════════════════════════════════════════════")
	fmt.Println("🔍                  COMPREHENSIVE GC ANALYSIS                       ")
	fmt.Println("═══════════════════════════════════════════════════════════════════")
	fmt.Println()

	// System Configuration
	fmt.Println("⚙️  SYSTEM CONFIGURATION")
	fmt.Println(strings.Repeat("─", 50))
	fmt.Printf("JVM Version:        %s\n", analysis.JVMVersion)
	fmt.Printf("Maximum Heap Size:  %s\n", analysis.HeapMax)
	fmt.Printf("Heap Region Size:   %s\n", analysis.HeapRegionSize)
	fmt.Println()

	// Performance Metrics
	fmt.Println("📊 PERFORMANCE METRICS")
	fmt.Println(strings.Repeat("─", 50))
	fmt.Printf("Application Runtime:    %v\n", analysis.TotalRuntime.Round(time.Millisecond))
	fmt.Printf("Total GC Time:          %v\n", analysis.TotalGCTime.Round(time.Millisecond))
	fmt.Printf("GC Overhead:            %.2f%%\n", 100.0-analysis.Throughput)
	fmt.Printf("Application Throughput: %.2f%%\n", analysis.Throughput)

	if analysis.AllocationRate > 0 {
		fmt.Printf("Allocation Rate:        %.2f MB/sec", analysis.AllocationRate)
		if analysis.AllocationRate > 100 {
			fmt.Printf(" ⚠️  [High - consider optimization]")
		} else if analysis.AllocationRate > 50 {
			fmt.Printf(" [Moderate]")
		}
		fmt.Println()
	}

	// G1GC-specific metrics
	if analysis.YoungCollectionEfficiency > 0 || analysis.MixedCollectionEfficiency > 0 {
		fmt.Println()
		fmt.Println("🔧 G1GC COLLECTION EFFICIENCY")
		fmt.Println(strings.Repeat("─", 50))
		if analysis.YoungCollectionEfficiency > 0 {
			fmt.Printf("Young Generation:       %.1f%% efficiency\n", analysis.YoungCollectionEfficiency*100)
		}
		if analysis.MixedCollectionEfficiency > 0 {
			fmt.Printf("Mixed Collections:      %.1f%% efficiency\n", analysis.MixedCollectionEfficiency*100)
		}
		if analysis.MixedToYoungRatio > 0 {
			fmt.Printf("Mixed/Young Ratio:      %.1f%% mixed collections\n", analysis.MixedToYoungRatio*100)
		}
		if analysis.PauseTimeVariance > 0 {
			fmt.Printf("Pause Time Variance:    %.1f%% coefficient of variation\n", analysis.PauseTimeVariance*100)
		}
		if analysis.PauseTargetMissRate > 0 {
			fmt.Printf("Pause Target Miss Rate: %.1f%% collections exceed target\n", analysis.PauseTargetMissRate*100)
		}
		if analysis.EvacuationFailureRate > 0 {
			fmt.Printf("Evacuation Failures:    %.1f%% of collections\n", analysis.EvacuationFailureRate*100)
		}
	}

	fmt.Println()

	// Detailed Pause Analysis
	fmt.Println("⏱️  PAUSE TIME ANALYSIS")
	fmt.Println(strings.Repeat("─", 50))
	fmt.Printf("Total Collections:     %d\n", analysis.TotalEvents)

	if analysis.TotalEvents > 0 {
		avgFreq := duration.Seconds() / float64(analysis.TotalEvents)
		minPause := float64(analysis.MinPause.Nanoseconds()) / 1e6
		maxPause := float64(analysis.MaxPause.Nanoseconds()) / 1e6
		avgPause := float64(analysis.AvgPause.Nanoseconds()) / 1e6

		fmt.Printf("Collection Frequency:  Every %.1f seconds\n", avgFreq)
		fmt.Printf("Min/Max/Avg Pause:     %.2fms/%.2fms/%.2fms\n", minPause, maxPause, avgPause)

		maxPauseMs := float64(analysis.MaxPause.Nanoseconds()) / 1e6
		if maxPauseMs > 500 {
			fmt.Printf(" 🔴 [Critical - User impact]")
		} else if maxPauseMs > 100 {
			fmt.Printf(" ⚠️  [Warning - Noticeable delay]")
		}
		fmt.Println()

		fmt.Printf("95th Percentile:       %.2fms\n", float64(analysis.P95Pause.Nanoseconds())/1e6)
		fmt.Printf("99th Percentile:       %.2fms\n", float64(analysis.P99Pause.Nanoseconds())/1e6)
	}
	fmt.Println()

	// Collection Type Analysis
	fmt.Println("🔄 COLLECTION TYPE BREAKDOWN")
	fmt.Println(strings.Repeat("─", 50))
	totalEvents := analysis.YoungGCCount + analysis.MixedGCCount + analysis.FullGCCount

	if totalEvents > 0 {
		if analysis.YoungGCCount > 0 {
			youngPct := float64(analysis.YoungGCCount) / float64(totalEvents) * 100
			fmt.Printf("✅ Young Generation:      %3d collections (%.1f%%)\n",
				analysis.YoungGCCount, youngPct)
			fmt.Printf("                          Efficient cleanup of short-lived objects\n")
		}

		if analysis.MixedGCCount > 0 {
			mixedPct := float64(analysis.MixedGCCount) / float64(totalEvents) * 100
			fmt.Printf("🟡 Mixed Collections:     %3d collections (%.1f%%)\n",
				analysis.MixedGCCount, mixedPct)
			fmt.Printf("                          Maintenance of older generation objects\n")
		}

		if analysis.FullGCCount > 0 {
			fullPct := float64(analysis.FullGCCount) / float64(totalEvents) * 100
			fmt.Printf("🔴 Full GC Events:        %3d collections (%.1f%%)",
				analysis.FullGCCount, fullPct)

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
	if analysis.AvgRegionUtilization > 0 {
		fmt.Println("🏗️  G1GC REGION ANALYSIS")
		fmt.Println(strings.Repeat("─", 50))
		fmt.Printf("Average Region Utilization: %.1f%%", analysis.AvgRegionUtilization*100)
		if analysis.AvgRegionUtilization > 0.85 {
			fmt.Printf(" ⚠️  [High - may cause evacuation pressure]")
		} else if analysis.AvgRegionUtilization > 0.7 {
			fmt.Printf(" [Moderate]")
		} else {
			fmt.Printf(" [Good]")
		}
		fmt.Println()

		if analysis.AllocationBurstCount > 0 {
			fmt.Printf("Allocation Bursts:          %d detected\n", analysis.AllocationBurstCount)
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

func (issues *GCIssues) Print() {
	totalIssues := len(issues.Critical) + len(issues.Warning) + len(issues.Info)

	if totalIssues == 0 {
		fmt.Println("\n💡 RECOMMENDATIONS")
		fmt.Println(strings.Repeat("─", 50))
		fmt.Println("✅ No performance issues detected.")
		fmt.Println("   Current GC configuration appears optimal.")
		fmt.Println("   Continue monitoring for any changes in application behavior.")
		return
	}

	fmt.Println("\n🚀 PERFORMANCE RECOMMENDATIONS")
	fmt.Println(strings.Repeat("─", 50))

	// Critical issues
	if len(issues.Critical) > 0 {
		fmt.Println("\n🚩 CRITICAL ISSUES - Immediate attention required:")
		for _, issue := range issues.Critical {
			fmt.Printf("\n🔴 %s\n", issue.Type)
			fmt.Printf("   Issue: %s\n", issue.Description)
			fmt.Println("   Recommended actions:")
			printFormattedRecommendations(issue.Recommendation)
		}
	}

	// Warning issues
	if len(issues.Warning) > 0 {
		fmt.Println("\n⚠️  WARNINGS - Address when possible:")
		for _, issue := range issues.Warning {
			fmt.Printf("\n🟡 %s\n", issue.Type)
			fmt.Printf("   Concern: %s\n", issue.Description)
			fmt.Println("   Suggested improvements:")
			printFormattedRecommendations(issue.Recommendation)
		}
	}

	// Optimization opportunities
	if len(issues.Info) > 0 {
		fmt.Println("\n📈 OPTIMIZATION OPPORTUNITIES:")
		for _, issue := range issues.Info {
			fmt.Printf("\n💡 %s\n", issue.Type)
			fmt.Printf("   Note: %s\n", issue.Description)
			printFormattedRecommendations(issue.Recommendation)
		}
	}
}

func printFormattedRecommendations(recommendations []string) {
	for _, rec := range recommendations {
		trimmed := strings.TrimSpace(rec)

		// Skip empty lines
		if trimmed == "" {
			continue
		}

		fmt.Printf("   • %s\n", trimmed)
	}
}
