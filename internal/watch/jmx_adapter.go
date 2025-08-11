package watch

import (
	"time"

	"github.com/mabhi256/jdiag/internal/jmx"
	"github.com/mabhi256/jdiag/utils"
)

type MetricsProcessor struct {
	dataStore *HistoricalDataStore
	gcTracker *GCEventTracker

	lastMetrics *jmx.MBeanSnapshot
	startTime   time.Time
}

func NewMetricsProcessor() *MetricsProcessor {
	return &MetricsProcessor{
		dataStore: NewHistoricalDataStore(),
		gcTracker: NewGCEventTracker(),
		startTime: time.Now(),
	}
}

func (mp *MetricsProcessor) ProcessMetrics(metrics *jmx.MBeanSnapshot) *TabState {
	if !metrics.Connected {
		return NewTabState()
	}

	mp.updateHistoricalData(metrics)

	mp.gcTracker.ProcessGCMetrics(metrics)

	gcOverhead := mp.calculateGCOverhead(metrics)

	tabState := mp.buildTabState(metrics, gcOverhead)

	mp.lastMetrics = metrics
	return tabState
}

func (mp *MetricsProcessor) updateHistoricalData(metrics *jmx.MBeanSnapshot) {
	now := metrics.Timestamp

	mp.dataStore.AddHeapMemory(now, &metrics.Memory.Heap)
	mp.dataStore.AddThreadCount(now, &metrics.Threading)
	mp.dataStore.AddClassCount(now, &metrics.ClassLoading)
	mp.dataStore.AddSystemUsage(now, &metrics.OS)

}

func (m *Model) GetHistoricalHeapMemory(window time.Duration) []utils.TimeMap {
	return m.metricsProcessor.dataStore.GetRecentHistory(window, func(hds *HistoricalDataStore) []utils.TimeMap {
		return hds.heapMemory
	})
}

func (m *Model) GetHistoricalClassCount(window time.Duration) []utils.TimeMap {
	return m.metricsProcessor.dataStore.GetRecentHistory(window, func(hds *HistoricalDataStore) []utils.TimeMap {
		return hds.classCounts
	})
}

func (m *Model) GetHistoricaThreadCount(window time.Duration) []utils.TimeMap {
	return m.metricsProcessor.dataStore.GetRecentHistory(window, func(hds *HistoricalDataStore) []utils.TimeMap {
		return hds.threadCounts
	})
}

func (m *Model) GetHistoricalSystemUsage(window time.Duration) []utils.TimeMap {
	return m.metricsProcessor.dataStore.GetRecentHistory(window, func(hds *HistoricalDataStore) []utils.TimeMap {
		return hds.systemUsage
	})
}

// calculateGCOverhead
func (mp *MetricsProcessor) calculateGCOverhead(metrics *jmx.MBeanSnapshot) float64 {
	uptime := time.Since(mp.startTime)
	totalGCTime := time.Duration(metrics.GC.YoungGCTime+metrics.GC.OldGCTime) * time.Millisecond

	if uptime.Milliseconds() == 0 {
		return 0
	}

	return float64(totalGCTime.Milliseconds()) / float64(uptime.Milliseconds())
}

// buildTabState - UPDATED to use multi-value trends
func (mp *MetricsProcessor) buildTabState(metrics *jmx.MBeanSnapshot, gcOverhead float64) *TabState {
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
	} else {
		state.Memory.YoungUsagePercent = float64(youngUsed) / float64(youngCommitted)
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
	} else {
		state.Memory.NonHeapUsagePercent = float64(metrics.Memory.NonHeap.Used) / float64(metrics.Memory.NonHeap.Committed)
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
	state.Threads.DaemonThreadCount = metrics.Threading.DaemonCount
	state.Threads.TotalStartedCount = metrics.Threading.TotalStartedCount

	state.Threads.LoadedClassCount = metrics.ClassLoading.LoadedClassCount
	state.Threads.UnloadedClassCount = metrics.ClassLoading.UnloadedClassCount
	state.Threads.TotalLoadedClasses = metrics.ClassLoading.TotalLoadedClassCount

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
		state.System.TotalSwap = metrics.OS.TotalSwapSpace
		state.System.FreeSwap = metrics.OS.FreeSwapSpace
		state.System.UsedSwap = metrics.OS.TotalSwapSpace - metrics.OS.FreeSwapSpace
		state.System.SwapPercent = float64(state.System.UsedSwap) / float64(metrics.OS.TotalSwapSpace)
	}

	return state
}

// Helper functions
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
