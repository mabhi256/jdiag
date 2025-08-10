package monitor

import (
	"sync"
	"time"

	"github.com/mabhi256/jdiag/utils"
)

type HistoricalDataStore struct {
	mu sync.RWMutex

	// Heap memory using multi-value points
	heapMemory []utils.MultiValueTimePoint

	// Single value metrics (keeping existing)
	cpuUsage     []utils.TimePoint
	threadCounts []utils.TimePoint

	maxPoints      int
	windowDuration time.Duration
}

func NewHistoricalDataStore() *HistoricalDataStore {
	return &HistoricalDataStore{
		heapMemory:     make([]utils.MultiValueTimePoint, 0),
		cpuUsage:       make([]utils.TimePoint, 0),
		threadCounts:   make([]utils.TimePoint, 0),
		maxPoints:      300,
		windowDuration: 5 * time.Minute,
	}
}

// AddHeapMemory stores complete heap memory information in a multi-value point
func (hds *HistoricalDataStore) AddHeapMemory(timestamp time.Time, used, committed, max int64) {
	hds.mu.Lock()
	defer hds.mu.Unlock()

	point := utils.NewMultiValuePoint(timestamp)
	point.SetMemoryValues(used, committed, max)

	hds.heapMemory = append(hds.heapMemory, *point)
	hds.trimHistory()
}

// Backwards compatibility - converts percentage to multi-value point
func (hds *HistoricalDataStore) AddHeapUsage(timestamp time.Time, percentage float64) {
	hds.mu.Lock()
	defer hds.mu.Unlock()

	point := utils.NewMultiValuePoint(timestamp)
	point.Set("usage_percent", percentage)

	hds.heapMemory = append(hds.heapMemory, *point)
	hds.trimHistory()
}

func (hds *HistoricalDataStore) AddCPUUsage(timestamp time.Time, cpuLoad float64) {
	hds.mu.Lock()
	defer hds.mu.Unlock()

	hds.cpuUsage = append(hds.cpuUsage, utils.TimePoint{Time: timestamp, Value: cpuLoad})
	hds.trimHistory()
}

func (hds *HistoricalDataStore) AddThreadCount(timestamp time.Time, count float64) {
	hds.mu.Lock()
	defer hds.mu.Unlock()

	hds.threadCounts = append(hds.threadCounts, utils.TimePoint{Time: timestamp, Value: count})
	hds.trimHistory()
}

// GetRecentHeapMemory returns multi-value heap memory data
func (hds *HistoricalDataStore) GetRecentHeapMemory(window time.Duration) []utils.MultiValueTimePoint {
	hds.mu.RLock()
	defer hds.mu.RUnlock()

	cutoff := time.Now().Add(-window)
	return hds.filterMultiValueByTime(hds.heapMemory, cutoff)
}

func (hds *HistoricalDataStore) GetRecentData(window time.Duration) ([]utils.TimePoint, []utils.TimePoint, []utils.TimePoint) {
	hds.mu.RLock()
	defer hds.mu.RUnlock()

	cutoff := time.Now().Add(-window)

	// Convert multi-value heap memory points to simple history points for backwards compatibility
	heapUsagePoints := make([]utils.TimePoint, 0)
	for _, memPoint := range hds.filterMultiValueByTime(hds.heapMemory, cutoff) {
		heapUsagePoints = append(heapUsagePoints, utils.TimePoint{
			Time:  memPoint.Timestamp,
			Value: memPoint.GetUsagePercent(),
		})
	}

	return heapUsagePoints,
		hds.filterByTime(hds.cpuUsage, cutoff),
		hds.filterByTime(hds.threadCounts, cutoff)
}

// GetRecentDataField returns a specific field from multi-value heap memory as simple TimePoint array
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

func (hds *HistoricalDataStore) filterMultiValueByTime(points []utils.MultiValueTimePoint, cutoff time.Time) []utils.MultiValueTimePoint {
	var result []utils.MultiValueTimePoint
	for _, point := range points {
		if point.Timestamp.After(cutoff) {
			result = append(result, point)
		}
	}
	return result
}

func (hds *HistoricalDataStore) filterByTime(points []utils.TimePoint, cutoff time.Time) []utils.TimePoint {
	var result []utils.TimePoint
	for _, point := range points {
		if point.Time.After(cutoff) {
			result = append(result, point)
		}
	}
	return result
}

func (hds *HistoricalDataStore) trimHistory() {
	if len(hds.heapMemory) > hds.maxPoints {
		hds.heapMemory = hds.heapMemory[len(hds.heapMemory)-hds.maxPoints:]
	}
	if len(hds.cpuUsage) > hds.maxPoints {
		hds.cpuUsage = hds.cpuUsage[len(hds.cpuUsage)-hds.maxPoints:]
	}
	if len(hds.threadCounts) > hds.maxPoints {
		hds.threadCounts = hds.threadCounts[len(hds.threadCounts)-hds.maxPoints:]
	}
}
