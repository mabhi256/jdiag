package monitor

import (
	"sync"
	"time"

	"github.com/mabhi256/jdiag/internal/jmx"
	"github.com/mabhi256/jdiag/utils"
)

type HistoricalDataStore struct {
	mu sync.RWMutex

	heapMemory   []utils.TimeMap
	threadCounts []utils.TimeMap
	classCounts  []utils.TimeMap
	systemUsage  []utils.TimeMap

	windowDuration time.Duration
}

func NewHistoricalDataStore() *HistoricalDataStore {
	return &HistoricalDataStore{
		heapMemory:     make([]utils.TimeMap, 0),
		threadCounts:   make([]utils.TimeMap, 0),
		classCounts:    make([]utils.TimeMap, 0),
		systemUsage:    make([]utils.TimeMap, 0),
		windowDuration: 5 * time.Minute,
	}
}

func (hds *HistoricalDataStore) AddHeapMemory(timestamp time.Time, entry *jmx.MemoryUsage) {
	hds.mu.Lock()
	defer hds.mu.Unlock()

	point := utils.NewTimeMap(timestamp)

	point.Values["used_mb"] = utils.MemorySize(entry.Used).MB()
	point.Values["committed_mb"] = utils.MemorySize(entry.Committed).MB()

	if entry.Max > 0 {
		point.Values["max_mb"] = utils.MemorySize(entry.Max).MB()
		point.Values["usage_percent"] = float64(entry.Used) / float64(entry.Max)
	}

	hds.heapMemory = append(hds.heapMemory, *point)
}

func (hds *HistoricalDataStore) AddThreadCount(timestamp time.Time, entry *jmx.Threading) {
	hds.mu.Lock()
	defer hds.mu.Unlock()

	point := utils.NewTimeMap(timestamp)

	point.Values["current_count"] = float64(entry.Count)
	point.Values["daemon_count"] = float64(entry.DaemonCount)

	hds.threadCounts = append(hds.threadCounts, *point)
}

func (hds *HistoricalDataStore) AddClassCount(timestamp time.Time, entry *jmx.ClassLoading) {
	hds.mu.Lock()
	defer hds.mu.Unlock()

	point := utils.NewTimeMap(timestamp)

	point.Values["total_loaded"] = float64(entry.TotalLoadedClassCount)
	point.Values["unloaded_count"] = float64(entry.UnloadedClassCount)

	hds.classCounts = append(hds.classCounts, *point)
}

func (hds *HistoricalDataStore) AddSystemUsage(timestamp time.Time, entry *jmx.OperatingSystem) {
	hds.mu.Lock()
	defer hds.mu.Unlock()

	point := utils.NewTimeMap(timestamp)

	point.Values["process_cpu"] = float64(entry.ProcessCpuLoad)
	point.Values["system_cpu"] = float64(entry.SystemCpuLoad)
	point.Values["ram"] = utils.MemorySize(entry.TotalPhysicalMemory - entry.FreePhysicalMemory).GB()
	point.Values["swap"] = utils.MemorySize(entry.TotalSwapSpace - entry.FreeSwapSpace).GB()

	hds.systemUsage = append(hds.systemUsage, *point)
}

func (hds *HistoricalDataStore) GetRecentHistory(window time.Duration, f func(*HistoricalDataStore) []utils.TimeMap) []utils.TimeMap {
	hds.mu.RLock()
	defer hds.mu.RUnlock()

	cutoff := time.Now().Add(-window)
	return hds.filterMultiValueByTime(f(hds), cutoff)
}

func (hds *HistoricalDataStore) GetRecentDataField(window time.Duration, fieldName string) []utils.TimePoint {
	hds.mu.RLock()
	defer hds.mu.RUnlock()

	cutoff := time.Now().Add(-window)
	filteredData := hds.filterMultiValueByTime(hds.heapMemory, cutoff)

	result := make([]utils.TimePoint, len(filteredData))
	for i, point := range filteredData {
		result[i] = utils.TimePoint{
			Time:  point.Timestamp,
			Value: point.GetOrDefault(fieldName, 0),
		}
	}

	return result
}

func (hds *HistoricalDataStore) filterMultiValueByTime(points []utils.TimeMap, cutoff time.Time) []utils.TimeMap {
	var result []utils.TimeMap
	for _, point := range points {
		if point.Timestamp.After(cutoff) {
			result = append(result, point)
		}
	}
	return result
}
