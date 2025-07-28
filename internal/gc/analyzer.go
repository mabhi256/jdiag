package gc

import (
	"fmt"
	"time"
)

// Performance thresholds
const (
	MinThroughputWarning   = 90.0
	MinThroughputCritical  = 80.0
	LongPauseThreshold     = 100 * time.Millisecond
	CriticalPauseThreshold = 500 * time.Millisecond
	HighAllocRateWarning   = 1000.0 // MB/s
	HighAllocRateCritical  = 5000.0 // MB/s
	HighHeapUtilWarning    = 0.8
	HighHeapUtilCritical   = 0.9
	HighP99PauseInfo       = 50 * time.Millisecond
	HighP99PauseWarning    = 200 * time.Millisecond
	MaxFullGCRateWarning   = 1.0 // per hour
	MaxFullGCRateCritical  = 2.0 // per hour
	ExcellentThroughput    = 99.0
	ExcellentP99Pause      = 10 * time.Millisecond
)

func (log *GCLog) Analyze(outputFormat string) {
	metrics := log.CalculateMetrics()

	log.Status = log.AssessGCHealth(metrics)
	log.PrintReport(metrics, outputFormat)

	issues := metrics.DetectPerformanceIssues()
	PrintRecommendations(issues)
}

func (metrics *GCMetrics) DetectPerformanceIssues() []PerformanceIssue {
	var issues []PerformanceIssue

	// Check throughput
	if metrics.Throughput < MinThroughputWarning {
		severity := "warning"
		if metrics.Throughput < MinThroughputCritical {
			severity = "critical"
		}
		issues = append(issues, PerformanceIssue{
			Type:     "Low Throughput",
			Severity: severity,
			Description: fmt.Sprintf("%.2f%% throughput (target: >%.0f%%)",
				metrics.Throughput, MinThroughputWarning),
			Recommendation: []string{
				"Consider increasing heap size to reduce GC frequency",
				"Review GC algorithm choice (G1GC vs ZGC vs Parallel)",
			},
		})
	}

	// Check for long pauses
	if metrics.MaxPause > LongPauseThreshold {
		severity := "warning"
		if metrics.MaxPause > CriticalPauseThreshold {
			severity = "critical"
		}
		issues = append(issues, PerformanceIssue{
			Type:     "Long GC Pauses",
			Severity: severity,
			Description: fmt.Sprintf("Max pause %v exceeds %v threshold",
				metrics.MaxPause, LongPauseThreshold),
			Recommendation: []string{
				"Tune G1GC parameters: -XX:MaxGCPauseMillis=<target>",
				"Consider low-latency collectors (ZGC/Shenandoah) for pause-sensitive apps",
			},
		})
	}

	// Check for frequent full GCs (with safe division)
	if metrics.FullGCCount > 0 && metrics.TotalRuntime > 0 {
		runtimeHours := metrics.TotalRuntime.Hours()
		if runtimeHours > 0 {
			fullGCRate := float64(metrics.FullGCCount) / runtimeHours
			if fullGCRate > MaxFullGCRateWarning {
				severity := "warning"
				if fullGCRate > MaxFullGCRateCritical {
					severity = "critical"
				}
				issues = append(issues, PerformanceIssue{
					Type:     "Frequent Full GC",
					Severity: severity,
					Description: fmt.Sprintf("%d full GCs in %v (%.2f/hour)",
						metrics.FullGCCount, metrics.TotalRuntime, fullGCRate),
					Recommendation: []string{
						"Increase heap size to avoid memory pressure",
						"Review object lifecycle and potential memory leaks",
					},
				})
			}
		}
	}

	// Check allocation rate
	if metrics.AllocationRate > HighAllocRateWarning {
		severity := "warning"
		if metrics.AllocationRate > HighAllocRateCritical {
			severity = "critical"
		}
		issues = append(issues, PerformanceIssue{
			Type:        "High Allocation Rate",
			Severity:    severity,
			Description: fmt.Sprintf("%.2f MB/s allocation rate", metrics.AllocationRate),
			Recommendation: []string{
				"Review object creation patterns and reduce temporary objects",
				"Consider object pooling for frequently allocated objects",
				"Profile allocation hotspots with tools like async-profiler",
			},
		})
	}

	// Check for memory pressure
	if metrics.AvgHeapUtil > HighHeapUtilWarning {
		severity := "warning"
		if metrics.AvgHeapUtil > HighHeapUtilCritical {
			severity = "critical"
		}
		issues = append(issues, PerformanceIssue{
			Type:     "High Heap Utilization",
			Severity: severity,
			Description: fmt.Sprintf("%.1f%% average heap utilization in recent GCs",
				metrics.AvgHeapUtil*100),
			Recommendation: []string{
				"Increase maximum heap size (-Xmx)",
				"Review memory usage patterns and optimize data structures",
				"Consider implementing memory monitoring and alerting",
			},
		})
	}

	// Check P99 pause times
	if metrics.P99Pause > HighP99PauseInfo {
		severity := "info"
		if metrics.P99Pause > HighP99PauseWarning {
			severity = "warning"
		}
		issues = append(issues, PerformanceIssue{
			Type:        "High P99 Pause Times",
			Severity:    severity,
			Description: fmt.Sprintf("P99 pause time is %v", metrics.P99Pause),
			Recommendation: []string{
				"Fine-tune GC pause target: -XX:MaxGCPauseMillis=<lower_value>",
				"Consider concurrent collection strategies",
			},
		})
	}

	return issues
}

func (log *GCLog) AssessGCHealth(metrics *GCMetrics) string {
	if metrics == nil {
		return "❓ Unknown (Invalid metrics)"
	}

	issues := metrics.DetectPerformanceIssues()

	criticalIssues := 0
	warningIssues := 0

	for _, issue := range issues {
		switch issue.Severity {
		case "critical":
			criticalIssues++
		case "warning":
			warningIssues++
		}
	}

	if criticalIssues > 0 {
		return "🔴 Critical (Immediate attention required)"
	}

	if metrics.FullGCCount > 0 {
		return "⚠️  Needs Attention (Full GC detected)"
	}

	if warningIssues > 2 {
		return "⚠️  Poor (Multiple performance issues)"
	}

	if warningIssues > 0 {
		return "⚠️  Fair (Minor performance issues)"
	}

	if metrics.Throughput > ExcellentThroughput && metrics.P99Pause < ExcellentP99Pause {
		return "✅ Excellent (Optimal performance)"
	}

	return "✅ Good (Performance within acceptable range)"
}
