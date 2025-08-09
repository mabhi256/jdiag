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

	lastMetrics         *jmx.MBeanSnapshot
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
func (mp *MetricsProcessor) ProcessMetrics(metrics *jmx.MBeanSnapshot) *TabState {
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
func (mp *MetricsProcessor) updateHistoricalData(metrics *jmx.MBeanSnapshot) {
	now := metrics.Timestamp

	// Heap usage percentage
	if metrics.Memory.Heap.Max > 0 {
		heapPercent := float64(metrics.Memory.Heap.Used) / float64(metrics.Memory.Heap.Max)
		mp.dataStore.AddHeapUsage(now, heapPercent)
	}

	// CPU usage
	mp.dataStore.AddCPUUsage(now, metrics.OS.ProcessCpuLoad)

	// Thread count
	mp.dataStore.AddThreadCount(now, float64(metrics.Threading.Count))

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
func (mp *MetricsProcessor) calculateGCOverhead(metrics *jmx.MBeanSnapshot) float64 {
	uptime := time.Since(mp.startTime)
	totalGCTime := time.Duration(metrics.GC.YoungGCTime+metrics.GC.OldGCTime) * time.Millisecond

	if uptime.Milliseconds() == 0 {
		return 0
	}

	return float64(totalGCTime.Milliseconds()) / float64(uptime.Milliseconds())
}

// calculateAllocationRate estimates memory allocation rate in bytes/second
func (mp *MetricsProcessor) calculateAllocationRate(metrics *jmx.MBeanSnapshot) float64 {
	now := metrics.Timestamp

	// Need baseline measurement
	if mp.lastMetrics == nil || mp.lastAllocationCheck.IsZero() {
		mp.lastAllocationCheck = now
		mp.lastHeapUsed = metrics.Memory.Heap.Used
		return -1
	}

	timeDiff := now.Sub(mp.lastAllocationCheck).Seconds()
	if timeDiff <= 0 {
		return -1
	}

	// Calculate heap usage change
	heapChange := metrics.Memory.Heap.Used - mp.lastHeapUsed

	// Only consider positive changes as allocations
	if heapChange > 0 {
		rate := float64(heapChange) / timeDiff
		mp.lastAllocationCheck = now
		mp.lastHeapUsed = metrics.Memory.Heap.Used
		return rate
	}

	// Reset baseline after GC or negative change
	mp.lastAllocationCheck = now
	mp.lastHeapUsed = metrics.Memory.Heap.Used
	return 0
}

// Helper functions to derive young/old generation data from memory pools
func (mp *MetricsProcessor) getYoungGenUsage(metrics *jmx.MBeanSnapshot) (used, committed, max int64) {
	// G1 Eden space
	if metrics.Memory.G1Eden.Valid {
		used += metrics.Memory.G1Eden.Usage.Used
		committed += metrics.Memory.G1Eden.Usage.Committed
		if metrics.Memory.G1Eden.Usage.Max > 0 {
			max += metrics.Memory.G1Eden.Usage.Max
		}
	}

	// G1 Survivor space
	if metrics.Memory.G1Survivor.Valid {
		used += metrics.Memory.G1Survivor.Usage.Used
		committed += metrics.Memory.G1Survivor.Usage.Committed
		if metrics.Memory.G1Survivor.Usage.Max > 0 {
			max += metrics.Memory.G1Survivor.Usage.Max
		}
	}

	return used, committed, max
}

func (mp *MetricsProcessor) getOldGenUsage(metrics *jmx.MBeanSnapshot) (used, committed, max int64) {
	// G1 Old Gen space
	if metrics.Memory.G1OldGen.Valid {
		used = metrics.Memory.G1OldGen.Usage.Used
		committed = metrics.Memory.G1OldGen.Usage.Committed
		max = metrics.Memory.G1OldGen.Usage.Max
	}

	return used, committed, max
}

// buildTabState constructs complete TabState with basic metrics + analytics
func (mp *MetricsProcessor) buildTabState(metrics *jmx.MBeanSnapshot, trends map[string]float64, gcOverhead float64) *TabState {
	state := NewTabState()

	// === Memory State ===
	state.Memory.HeapUsed = metrics.Memory.Heap.Used
	state.Memory.HeapCommitted = metrics.Memory.Heap.Committed
	state.Memory.HeapMax = metrics.Memory.Heap.Max
	if metrics.Memory.Heap.Max > 0 {
		state.Memory.HeapUsagePercent = float64(metrics.Memory.Heap.Used) / float64(metrics.Memory.Heap.Max)
	}

	// Calculate young generation metrics from memory pools
	youngUsed, youngCommitted, youngMax := mp.getYoungGenUsage(metrics)
	state.Memory.YoungUsed = youngUsed
	state.Memory.YoungCommitted = youngCommitted
	state.Memory.YoungMax = youngMax
	if youngMax > 0 {
		state.Memory.YoungUsagePercent = float64(youngUsed) / float64(youngMax)
	}

	// Calculate old generation metrics from memory pools
	oldUsed, oldCommitted, oldMax := mp.getOldGenUsage(metrics)
	state.Memory.OldUsed = oldUsed
	state.Memory.OldCommitted = oldCommitted
	state.Memory.OldMax = oldMax
	if oldMax > 0 {
		state.Memory.OldUsagePercent = float64(oldUsed) / float64(oldMax)
	}

	state.Memory.NonHeapUsed = metrics.Memory.NonHeap.Used
	state.Memory.NonHeapCommitted = metrics.Memory.NonHeap.Committed
	state.Memory.NonHeapMax = metrics.Memory.NonHeap.Max
	if metrics.Memory.NonHeap.Max > 0 {
		state.Memory.NonHeapUsagePercent = float64(metrics.Memory.NonHeap.Used) / float64(metrics.Memory.NonHeap.Max)
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
	state.GC.YoungGCCount = metrics.GC.YoungGCCount
	state.GC.YoungGCTime = metrics.GC.YoungGCTime
	if metrics.GC.YoungGCCount > 0 {
		state.GC.YoungGCAvg = float64(metrics.GC.YoungGCTime) / float64(metrics.GC.YoungGCCount)
	}

	state.GC.OldGCCount = metrics.GC.OldGCCount
	state.GC.OldGCTime = metrics.GC.OldGCTime
	if metrics.GC.OldGCCount > 0 {
		state.GC.OldGCAvg = float64(metrics.GC.OldGCTime) / float64(metrics.GC.OldGCCount)
	}

	state.GC.TotalGCCount = metrics.GC.YoungGCCount + metrics.GC.OldGCCount
	state.GC.TotalGCTime = metrics.GC.YoungGCTime + metrics.GC.OldGCTime

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
	state.Threads.CurrentThreadCount = metrics.Threading.Count
	state.Threads.PeakThreadCount = metrics.Threading.PeakCount
	state.Threads.LoadedClassCount = metrics.ClassLoading.LoadedClassCount
	state.Threads.UnloadedClassCount = metrics.ClassLoading.UnloadedClassCount
	state.Threads.TotalLoadedClasses = metrics.ClassLoading.TotalLoadedClassCount

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
	state.System.ProcessCpuLoad = metrics.OS.ProcessCpuLoad
	state.System.SystemCpuLoad = metrics.OS.SystemCpuLoad
	state.System.LastUpdateTime = metrics.Timestamp
	state.System.JVMVersion = metrics.Runtime.VmVersion
	state.System.JVMVendor = metrics.Runtime.VmVendor
	state.System.JVMStartTime = metrics.Runtime.StartTime
	state.System.JVMUptime = metrics.Runtime.Uptime
	state.System.AvailableProcessors = metrics.OS.AvailableProcessors
	state.System.SystemLoad = metrics.OS.SystemLoadAverage

	// Calculate system memory usage
	if metrics.OS.TotalPhysicalMemory > 0 {
		state.System.TotalSystemMemory = metrics.OS.TotalPhysicalMemory
		state.System.FreeSystemMemory = metrics.OS.FreePhysicalMemory
		state.System.UsedSystemMemory = metrics.OS.TotalPhysicalMemory - metrics.OS.FreePhysicalMemory
		state.System.SystemMemoryPercent = float64(state.System.UsedSystemMemory) / float64(metrics.OS.TotalPhysicalMemory)
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
