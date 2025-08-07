package monitor

import (
	"sync"
	"time"

	"github.com/mabhi256/jdiag/utils"
)

type HistoricalDataStore struct {
	mu sync.RWMutex

	heapUsage       []utils.HistoryPoint
	cpuUsage        []utils.HistoryPoint
	threadCounts    []utils.HistoryPoint
	allocationRates []utils.HistoryPoint

	maxPoints      int
	windowDuration time.Duration
}

func NewHistoricalDataStore() *HistoricalDataStore {
	return &HistoricalDataStore{
		heapUsage:       make([]utils.HistoryPoint, 0),
		cpuUsage:        make([]utils.HistoryPoint, 0),
		threadCounts:    make([]utils.HistoryPoint, 0),
		allocationRates: make([]utils.HistoryPoint, 0),
		maxPoints:       300,
		windowDuration:  5 * time.Minute,
	}
}

func (hds *HistoricalDataStore) AddHeapUsage(timestamp time.Time, percentage float64) {
	hds.mu.Lock()
	defer hds.mu.Unlock()

	hds.heapUsage = append(hds.heapUsage, utils.HistoryPoint{Timestamp: timestamp, Value: percentage})
	hds.trimHistory()
}

func (hds *HistoricalDataStore) AddCPUUsage(timestamp time.Time, cpuLoad float64) {
	hds.mu.Lock()
	defer hds.mu.Unlock()

	hds.cpuUsage = append(hds.cpuUsage, utils.HistoryPoint{Timestamp: timestamp, Value: cpuLoad})
	hds.trimHistory()
}

func (hds *HistoricalDataStore) AddThreadCount(timestamp time.Time, count float64) {
	hds.mu.Lock()
	defer hds.mu.Unlock()

	hds.threadCounts = append(hds.threadCounts, utils.HistoryPoint{Timestamp: timestamp, Value: count})
	hds.trimHistory()
}

func (hds *HistoricalDataStore) AddAllocationRate(timestamp time.Time, rate float64) {
	hds.mu.Lock()
	defer hds.mu.Unlock()

	if rate >= 0 {
		hds.allocationRates = append(hds.allocationRates, utils.HistoryPoint{Timestamp: timestamp, Value: rate})
		hds.trimHistory()
	}
}

func (hds *HistoricalDataStore) GetRecentData(window time.Duration) ([]utils.HistoryPoint, []utils.HistoryPoint, []utils.HistoryPoint, []utils.HistoryPoint) {
	hds.mu.RLock()
	defer hds.mu.RUnlock()

	cutoff := time.Now().Add(-window)

	return hds.filterByTime(hds.heapUsage, cutoff),
		hds.filterByTime(hds.cpuUsage, cutoff),
		hds.filterByTime(hds.threadCounts, cutoff),
		hds.filterByTime(hds.allocationRates, cutoff)
}

func (hds *HistoricalDataStore) filterByTime(points []utils.HistoryPoint, cutoff time.Time) []utils.HistoryPoint {
	var result []utils.HistoryPoint
	for _, point := range points {
		if point.Timestamp.After(cutoff) {
			result = append(result, point)
		}
	}
	return result
}

func (hds *HistoricalDataStore) trimHistory() {
	if len(hds.heapUsage) > hds.maxPoints {
		hds.heapUsage = hds.heapUsage[len(hds.heapUsage)-hds.maxPoints:]
	}
	if len(hds.cpuUsage) > hds.maxPoints {
		hds.cpuUsage = hds.cpuUsage[len(hds.cpuUsage)-hds.maxPoints:]
	}
	if len(hds.threadCounts) > hds.maxPoints {
		hds.threadCounts = hds.threadCounts[len(hds.threadCounts)-hds.maxPoints:]
	}
	if len(hds.allocationRates) > hds.maxPoints {
		hds.allocationRates = hds.allocationRates[len(hds.allocationRates)-hds.maxPoints:]
	}
}
