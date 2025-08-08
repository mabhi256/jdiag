package monitor

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/mabhi256/jdiag/internal/jmx"
)

type AlertEngine struct {
	mu sync.RWMutex

	alerts []PerformanceAlert

	// Thresholds
	heapWarningThreshold  float64
	heapCriticalThreshold float64
	gcOverheadWarning     float64
	gcOverheadCritical    float64
	cpuWarningThreshold   float64
	cpuCriticalThreshold  float64
	trendThreshold        float64
}

func NewAlertEngine() *AlertEngine {
	return &AlertEngine{
		alerts:                make([]PerformanceAlert, 0),
		heapWarningThreshold:  0.75,
		heapCriticalThreshold: 0.90,
		gcOverheadWarning:     0.05,
		gcOverheadCritical:    0.10,
		cpuWarningThreshold:   0.70,
		cpuCriticalThreshold:  0.90,
		trendThreshold:        0.1,
	}
}

func (ae *AlertEngine) AnalyzeMetrics(metrics *jmx.JVMSnapshot, trends map[string]float64, gcOverhead float64, gcTracker *GCEventTracker) {
	ae.mu.Lock()
	defer ae.mu.Unlock()

	ae.alerts = nil // Clear previous alerts

	// Heap usage analysis
	if metrics.HeapMax > 0 {
		heapPercent := float64(metrics.HeapUsed) / float64(metrics.HeapMax)

		if heapPercent >= ae.heapCriticalThreshold {
			ae.addAlert("critical", "Critical Heap Usage",
				fmt.Sprintf("Heap usage is at %.1f%%, approaching maximum capacity", heapPercent*100),
				heapPercent, ae.heapCriticalThreshold, "heap_usage")
		} else if heapPercent >= ae.heapWarningThreshold {
			ae.addAlert("warning", "High Heap Usage",
				fmt.Sprintf("Heap usage is at %.1f%%, consider investigating memory leaks", heapPercent*100),
				heapPercent, ae.heapWarningThreshold, "heap_usage")
		}
	}

	// GC overhead analysis
	if gcOverhead >= ae.gcOverheadCritical {
		ae.addAlert("critical", "Excessive GC Overhead",
			fmt.Sprintf("GC is consuming %.1f%% of total time, performance severely impacted", gcOverhead*100),
			gcOverhead, ae.gcOverheadCritical, "gc_overhead")
	} else if gcOverhead >= ae.gcOverheadWarning {
		ae.addAlert("warning", "High GC Overhead",
			fmt.Sprintf("GC is consuming %.1f%% of total time, monitor for performance impact", gcOverhead*100),
			gcOverhead, ae.gcOverheadWarning, "gc_overhead")
	}

	// CPU usage analysis
	if metrics.ProcessCpuLoad >= ae.cpuCriticalThreshold {
		ae.addAlert("critical", "Critical CPU Usage",
			fmt.Sprintf("Process CPU usage is at %.1f%%, system may be overloaded", metrics.ProcessCpuLoad*100),
			metrics.ProcessCpuLoad, ae.cpuCriticalThreshold, "cpu_usage")
	} else if metrics.ProcessCpuLoad >= ae.cpuWarningThreshold {
		ae.addAlert("warning", "High CPU Usage",
			fmt.Sprintf("Process CPU usage is at %.1f%% monitor system load", metrics.ProcessCpuLoad*100),
			metrics.ProcessCpuLoad, ae.cpuWarningThreshold, "cpu_usage")
	}

	// Trend analysis
	if heapTrend, exists := trends["heap_trend"]; exists && heapTrend > ae.trendThreshold {
		ae.addAlert("warning", "Rising Heap Usage Trend",
			fmt.Sprintf("Heap usage is increasing at %.1f%% per minute", heapTrend*100),
			heapTrend, ae.trendThreshold, "heap_trend")
	}

	if cpuTrend, exists := trends["cpu_trend"]; exists && cpuTrend > ae.trendThreshold {
		ae.addAlert("warning", "Rising CPU Usage Trend",
			fmt.Sprintf("CPU usage is increasing at %.1f%% per minute", cpuTrend*100),
			cpuTrend, ae.trendThreshold, "cpu_trend")
	}

	if threadTrend, exists := trends["thread_trend"]; exists && threadTrend > 10 {
		ae.addAlert("warning", "Rising Thread Count",
			fmt.Sprintf("Thread count is increasing at %.1f threads per minute", threadTrend),
			threadTrend, 10, "thread_trend")
	}

	// GC frequency and pause analysis (the missing piece!)
	ae.analyzeGCFrequencyAndPauses(gcTracker)
}

func (ae *AlertEngine) addAlert(level, title, description string, value, threshold float64, metricName string) {
	alert := PerformanceAlert{
		Level:       level,
		Title:       title,
		Description: description,
		Timestamp:   time.Now(),
		Value:       value,
		Threshold:   threshold,
		MetricName:  metricName,
	}
	ae.alerts = append(ae.alerts, alert)
}

func (ae *AlertEngine) GetAlerts() []PerformanceAlert {
	ae.mu.RLock()
	defer ae.mu.RUnlock()

	alerts := make([]PerformanceAlert, len(ae.alerts))
	copy(alerts, ae.alerts)

	// Sort by severity
	sort.Slice(alerts, func(i, j int) bool {
		levelOrder := map[string]int{"critical": 0, "warning": 1, "info": 2}
		return levelOrder[alerts[i].Level] < levelOrder[alerts[j].Level]
	})

	return alerts
}

func (ae *AlertEngine) analyzeGCFrequencyAndPauses(gcTracker *GCEventTracker) {
	// Analyze recent GC frequency
	recentWindow := time.Minute
	cutoff := time.Now().Add(-recentWindow)

	youngCount := 0
	oldCount := 0
	recentEvents := gcTracker.GetRecentEvents(1000) // Get lots of events to filter

	for _, event := range recentEvents {
		if event.Timestamp.After(cutoff) {
			if event.Generation == "young" {
				youngCount++
			} else {
				oldCount++
			}
		}
	}

	// Alert on excessive GC frequency
	if youngCount > 60 { // More than 1 per second
		ae.addAlert("warning", "Frequent Young GC",
			fmt.Sprintf("%d young generation GCs in the last minute", youngCount),
			float64(youngCount), 60, "young_gc_frequency")
	}

	if oldCount > 5 { // More than 5 old GCs per minute
		ae.addAlert("critical", "Frequent Old GC",
			fmt.Sprintf("%d old generation GCs in the last minute, possible memory pressure", oldCount),
			float64(oldCount), 5, "old_gc_frequency")
	}

	// Analyze GC pause times
	recentPauses := ae.getRecentGCPauses(gcTracker, recentWindow)
	if len(recentPauses) > 0 {
		avgPause := ae.calculateAveragePause(recentPauses)
		if avgPause > 500*time.Millisecond { // More than 500ms average
			ae.addAlert("critical", "Long GC Pauses",
				fmt.Sprintf("Average GC pause time is %s", avgPause),
				float64(avgPause.Milliseconds()), 500, "gc_pause_time")
		} else if avgPause > 100*time.Millisecond { // More than 100ms average
			ae.addAlert("warning", "Elevated GC Pauses",
				fmt.Sprintf("Average GC pause time is %s", avgPause),
				float64(avgPause.Milliseconds()), 100, "gc_pause_time")
		}
	}
}

func (ae *AlertEngine) getRecentGCPauses(gcTracker *GCEventTracker, window time.Duration) []GCEvent {
	cutoff := time.Now().Add(-window)
	var recentEvents []GCEvent

	allEvents := gcTracker.GetRecentEvents(1000) // Get lots of events
	for _, event := range allEvents {
		if event.Timestamp.After(cutoff) {
			recentEvents = append(recentEvents, event)
		}
	}

	return recentEvents
}

// calculateAveragePause calculates the average pause time for a set of GC events
func (ae *AlertEngine) calculateAveragePause(events []GCEvent) time.Duration {
	if len(events) == 0 {
		return 0
	}

	var totalDuration time.Duration
	for _, event := range events {
		totalDuration += event.Duration
	}

	return totalDuration / time.Duration(len(events))
}
