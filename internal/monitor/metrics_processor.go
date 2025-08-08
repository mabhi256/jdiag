package monitor

import (
	"time"

	"github.com/mabhi256/jdiag/internal/jmx"
	"github.com/mabhi256/jdiag/utils"
)

// MetricsProcessor orchestrates all metric analysis components
type MetricsProcessor struct {
	dataStore   *HistoricalDataStore
	gcTracker   *GCEventTracker
	alertEngine *AlertEngine

	lastMetrics         *jmx.JVMSnapshot
	startTime           time.Time
	lastAllocationCheck time.Time
	lastHeapUsed        int64
}

// NewMetricsProcessor creates a new metrics processing pipeline
func NewMetricsProcessor() *MetricsProcessor {
	return &MetricsProcessor{
		dataStore:   NewHistoricalDataStore(),
		gcTracker:   NewGCEventTracker(),
		alertEngine: NewAlertEngine(),
		startTime:   time.Now(),
	}
}

// ProcessMetrics takes raw JVM metrics and produces enriched TabState
func (mp *MetricsProcessor) ProcessMetrics(metrics *jmx.JVMSnapshot) *TabState {
	if !metrics.Connected {
		return NewTabState()
	}

	// Update historical data
	mp.updateHistoricalData(metrics)

	// Process GC events
	mp.gcTracker.ProcessGCMetrics(metrics)

	// Calculate trends using utils
	trends := mp.calculateCurrentTrends()

	// Calculate GC overhead
	gcOverhead := mp.calculateGCOverhead(metrics)

	// Generate alerts (now includes GC frequency/pause analysis)
	mp.alertEngine.AnalyzeMetrics(metrics, trends, gcOverhead, mp.gcTracker)

	// Build complete tab state
	tabState := mp.buildTabState(metrics, trends, gcOverhead)

	mp.lastMetrics = metrics
	return tabState
}

// updateHistoricalData adds current metrics to time-series storage
func (mp *MetricsProcessor) updateHistoricalData(metrics *jmx.JVMSnapshot) {
	now := metrics.Timestamp

	// Heap usage percentage
	if metrics.HeapMax > 0 {
		heapPercent := float64(metrics.HeapUsed) / float64(metrics.HeapMax)
		mp.dataStore.AddHeapUsage(now, heapPercent)
	}

	// CPU usage
	mp.dataStore.AddCPUUsage(now, metrics.ProcessCpuLoad)

	// Thread count
	mp.dataStore.AddThreadCount(now, float64(metrics.ThreadCount))

	// Allocation rate
	allocationRate := mp.calculateAllocationRate(metrics)
	mp.dataStore.AddAllocationRate(now, allocationRate)
}

// calculateCurrentTrends computes trend slopes for key metrics
func (mp *MetricsProcessor) calculateCurrentTrends() map[string]float64 {
	trends := make(map[string]float64)

	// Get recent data from storage
	heapData, cpuData, threadData, _ := mp.dataStore.GetRecentData(time.Minute)

	// Calculate trends using utils.LinearRegression
	trends["heap_trend"] = mp.calculateTrendSlope(heapData, time.Minute)
	trends["cpu_trend"] = mp.calculateTrendSlope(cpuData, time.Minute)
	trends["thread_trend"] = mp.calculateTrendSlope(threadData, time.Minute)

	return trends
}

// calculateTrendSlope computes trend slope using utils.LinearRegression
func (mp *MetricsProcessor) calculateTrendSlope(points []utils.HistoryPoint, window time.Duration) float64 {
	if len(points) < 2 {
		return 0
	}

	// Filter points within window
	cutoff := time.Now().Add(-window)
	var windowPoints []utils.HistoryPoint
	for _, point := range points {
		if point.Timestamp.After(cutoff) {
			windowPoints = append(windowPoints, point)
		}
	}

	if len(windowPoints) < 2 {
		return 0
	}

	// Prepare X,Y arrays for LinearRegression
	x := make([]float64, len(windowPoints))
	y := make([]float64, len(windowPoints))

	for i, point := range windowPoints {
		x[i] = float64(i) // Use array index as time
		y[i] = point.Value
	}

	// Use existing utils.LinearRegression
	slope, _ := utils.LinearRegression(x, y)

	// Convert to per-minute rate
	if len(windowPoints) > 1 {
		timeSpan := windowPoints[len(windowPoints)-1].Timestamp.Sub(windowPoints[0].Timestamp)
		if timeSpan.Minutes() > 0 {
			slope = slope * (60.0 / timeSpan.Minutes())
		}
	}

	return slope
}

// calculateGCOverhead computes percentage of time spent in GC
func (mp *MetricsProcessor) calculateGCOverhead(metrics *jmx.JVMSnapshot) float64 {
	uptime := time.Since(mp.startTime)
	totalGCTime := time.Duration(metrics.YoungGCTime+metrics.OldGCTime) * time.Millisecond

	if uptime.Milliseconds() == 0 {
		return 0
	}

	return float64(totalGCTime.Milliseconds()) / float64(uptime.Milliseconds())
}

// calculateAllocationRate estimates memory allocation rate in bytes/second
func (mp *MetricsProcessor) calculateAllocationRate(metrics *jmx.JVMSnapshot) float64 {
	now := metrics.Timestamp

	// Need baseline measurement
	if mp.lastMetrics == nil || mp.lastAllocationCheck.IsZero() {
		mp.lastAllocationCheck = now
		mp.lastHeapUsed = metrics.HeapUsed
		return -1
	}

	timeDiff := now.Sub(mp.lastAllocationCheck).Seconds()
	if timeDiff <= 0 {
		return -1
	}

	// Calculate heap usage change
	heapChange := metrics.HeapUsed - mp.lastHeapUsed

	// Only consider positive changes as allocations
	if heapChange > 0 {
		rate := float64(heapChange) / timeDiff
		mp.lastAllocationCheck = now
		mp.lastHeapUsed = metrics.HeapUsed
		return rate
	}

	// Reset baseline after GC or negative change
	mp.lastAllocationCheck = now
	mp.lastHeapUsed = metrics.HeapUsed
	return 0
}

// buildTabState constructs complete TabState with basic metrics + analytics
func (mp *MetricsProcessor) buildTabState(metrics *jmx.JVMSnapshot, trends map[string]float64, gcOverhead float64) *TabState {
	state := NewTabState()

	// === Memory State ===
	state.Memory.HeapUsed = metrics.HeapUsed
	state.Memory.HeapCommitted = metrics.HeapCommitted
	state.Memory.HeapMax = metrics.HeapMax
	if metrics.HeapMax > 0 {
		state.Memory.HeapUsagePercent = float64(metrics.HeapUsed) / float64(metrics.HeapMax)
	}

	state.Memory.YoungUsed = metrics.YoungUsed
	state.Memory.YoungCommitted = metrics.YoungCommitted
	state.Memory.YoungMax = metrics.YoungMax
	if metrics.YoungMax > 0 {
		state.Memory.YoungUsagePercent = float64(metrics.YoungUsed) / float64(metrics.YoungMax)
	}

	state.Memory.OldUsed = metrics.OldUsed
	state.Memory.OldCommitted = metrics.OldCommitted
	state.Memory.OldMax = metrics.OldMax
	if metrics.OldMax > 0 {
		state.Memory.OldUsagePercent = float64(metrics.OldUsed) / float64(metrics.OldMax)
	}

	state.Memory.NonHeapUsed = metrics.NonHeapUsed
	state.Memory.NonHeapCommitted = metrics.NonHeapCommitted
	state.Memory.NonHeapMax = metrics.NonHeapMax
	if metrics.HeapMax > 0 { // Use HeapMax as reference for non-heap percentage
		state.Memory.NonHeapUsagePercent = float64(metrics.NonHeapUsed) / float64(metrics.HeapMax)
	}

	// Memory analytics
	_, _, _, allocationRates := mp.dataStore.GetRecentData(time.Minute)
	if len(allocationRates) > 0 {
		state.Memory.AllocationRate = allocationRates[len(allocationRates)-1].Value
	}

	if heapTrend, exists := trends["heap_trend"]; exists {
		state.Memory.MemoryTrend = heapTrend
	}

	// === GC State ===
	state.GC.YoungGCCount = metrics.YoungGCCount
	state.GC.YoungGCTime = metrics.YoungGCTime
	if metrics.YoungGCCount > 0 {
		state.GC.YoungGCAvg = float64(metrics.YoungGCTime) / float64(metrics.YoungGCCount)
	}

	state.GC.OldGCCount = metrics.OldGCCount
	state.GC.OldGCTime = metrics.OldGCTime
	if metrics.OldGCCount > 0 {
		state.GC.OldGCAvg = float64(metrics.OldGCTime) / float64(metrics.OldGCCount)
	}

	state.GC.TotalGCCount = metrics.YoungGCCount + metrics.OldGCCount
	state.GC.TotalGCTime = metrics.YoungGCTime + metrics.OldGCTime

	// GC analytics
	state.GC.GCOverhead = gcOverhead
	state.GC.GCFrequency = mp.gcTracker.GetGCFrequency(time.Minute)
	state.GC.RecentGCEvents = mp.gcTracker.GetRecentEvents(10)

	if len(state.GC.RecentGCEvents) > 0 {
		var totalDuration time.Duration
		for _, event := range state.GC.RecentGCEvents {
			totalDuration += event.Duration
		}
		state.GC.AvgGCPauseTime = totalDuration / time.Duration(len(state.GC.RecentGCEvents))
	}

	// === Thread State ===
	state.Threads.CurrentThreadCount = metrics.ThreadCount
	state.Threads.PeakThreadCount = metrics.PeakThreadCount
	state.Threads.LoadedClassCount = metrics.LoadedClassCount
	state.Threads.UnloadedClassCount = metrics.UnloadedClassCount
	state.Threads.TotalLoadedClasses = metrics.LoadedClassCount - metrics.UnloadedClassCount

	// Thread analytics
	if threadTrend, exists := trends["thread_trend"]; exists {
		state.Threads.ThreadCreationRate = threadTrend
	}

	// Check for thread alerts
	alerts := mp.alertEngine.GetAlerts()
	for _, alert := range alerts {
		if alert.MetricName == "thread_contention" {
			state.Threads.ThreadContention = true
			break
		}
	}

	// === System State ===
	state.System.ProcessCpuLoad = metrics.ProcessCpuLoad
	state.System.SystemCpuLoad = metrics.SystemCpuLoad
	state.System.LastUpdateTime = metrics.Timestamp
	state.System.JVMVersion = metrics.JVMVersion
	state.System.JVMVendor = metrics.JVMVendor
	state.System.JVMStartTime = metrics.JVMStartTime
	state.System.JVMUptime = metrics.JVMUptime
	state.System.AvailableProcessors = metrics.AvailableProcessors
	state.System.SystemLoad = metrics.SystemLoadAverage

	// Calculate system memory usage
	if metrics.TotalSystemMemory > 0 {
		state.System.TotalSystemMemory = metrics.TotalSystemMemory
		state.System.FreeSystemMemory = metrics.FreeSystemMemory
		state.System.UsedSystemMemory = metrics.TotalSystemMemory - metrics.FreeSystemMemory
		state.System.SystemMemoryPercent = float64(state.System.UsedSystemMemory) / float64(metrics.TotalSystemMemory)
	}
	// System analytics
	if cpuTrend, exists := trends["cpu_trend"]; exists {
		state.System.CPUTrend = cpuTrend
	}

	return state
}

func (mp *MetricsProcessor) GetAlerts() []PerformanceAlert {
	return mp.alertEngine.GetAlerts()
}

func (mp *MetricsProcessor) GetCurrentTrends() map[string]float64 {
	return mp.calculateCurrentTrends()
}

func (mp *MetricsProcessor) GetSummaryStats() map[string]any {
	stats := make(map[string]any)

	// Session duration
	stats["uptime"] = time.Since(mp.startTime)

	// GC statistics from tracker
	recentEvents := mp.gcTracker.GetRecentEvents(1000) // Get all events
	youngGCs := 0
	oldGCs := 0
	totalGCTime := time.Duration(0)

	for _, event := range recentEvents {
		if event.Generation == "young" {
			youngGCs++
		} else {
			oldGCs++
		}
		totalGCTime += event.Duration
	}

	stats["total_young_gcs"] = youngGCs
	stats["total_old_gcs"] = oldGCs
	stats["total_gc_time"] = totalGCTime

	// Heap usage statistics from historical data
	heapData, _, _, _ := mp.dataStore.GetRecentData(time.Since(mp.startTime))
	if len(heapData) > 0 {
		var min, max, sum float64 = 1.0, 0.0, 0.0
		for _, point := range heapData {
			if point.Value < min {
				min = point.Value
			}
			if point.Value > max {
				max = point.Value
			}
			sum += point.Value
		}

		stats["heap_usage_min"] = min
		stats["heap_usage_max"] = max
		stats["heap_usage_avg"] = sum / float64(len(heapData))
	}

	// Alert counts
	alerts := mp.alertEngine.GetAlerts()
	criticalAlerts := 0
	warningAlerts := 0

	for _, alert := range alerts {
		switch alert.Level {
		case "critical":
			criticalAlerts++
		case "warning":
			warningAlerts++
		}
	}

	stats["critical_alerts"] = criticalAlerts
	stats["warning_alerts"] = warningAlerts

	return stats
}

// GetHistoricalData returns time-series data for charting/analysis
func (mp *MetricsProcessor) GetHistoricalData(window time.Duration) ([]utils.HistoryPoint, []utils.HistoryPoint, []utils.HistoryPoint, []utils.HistoryPoint) {
	return mp.dataStore.GetRecentData(window)
}

// GetGCEvents returns recent GC events
func (mp *MetricsProcessor) GetGCEvents(maxEvents int) []GCEvent {
	return mp.gcTracker.GetRecentEvents(maxEvents)
}

// Reset clears all historical data and starts fresh
func (mp *MetricsProcessor) Reset() {
	mp.dataStore = NewHistoricalDataStore()
	mp.gcTracker = NewGCEventTracker()
	mp.alertEngine = NewAlertEngine()
	mp.lastMetrics = nil
	mp.startTime = time.Now()
	mp.lastAllocationCheck = time.Time{}
	mp.lastHeapUsed = 0
}
