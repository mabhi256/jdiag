package monitor

import (
	"sync"
	"time"

	"github.com/mabhi256/jdiag/internal/jmx"
)

type GCEventTracker struct {
	mu sync.RWMutex

	gcEvents       []GCEvent
	lastGCCounts   map[string]int64
	lastGCTimes    map[string]int64
	windowDuration time.Duration
}

func NewGCEventTracker() *GCEventTracker {
	return &GCEventTracker{
		gcEvents:       make([]GCEvent, 0),
		lastGCCounts:   make(map[string]int64),
		lastGCTimes:    make(map[string]int64),
		windowDuration: 5 * time.Minute,
	}
}

func (get *GCEventTracker) ProcessGCMetrics(metrics *jmx.JVMSnapshot) {
	get.mu.Lock()
	defer get.mu.Unlock()

	// Young generation GC
	if lastCount, exists := get.lastGCCounts["young"]; exists {
		if metrics.YoungGCCount > lastCount {
			newEvents := metrics.YoungGCCount - lastCount
			totalNewTime := metrics.YoungGCTime - get.lastGCTimes["young"]
			avgDuration := time.Duration(float64(totalNewTime)/float64(newEvents)) * time.Millisecond

			for i := int64(0); i < newEvents; i++ {
				get.gcEvents = append(get.gcEvents, GCEvent{
					Timestamp:  metrics.Timestamp,
					Generation: "young",
					Duration:   avgDuration,
					Before:     metrics.YoungUsed,
					After:      metrics.YoungUsed,
					Collected:  0,
				})
			}
		}
	}

	// Old generation GC
	if lastCount, exists := get.lastGCCounts["old"]; exists {
		if metrics.OldGCCount > lastCount {
			newEvents := metrics.OldGCCount - lastCount
			totalNewTime := metrics.OldGCTime - get.lastGCTimes["old"]
			avgDuration := time.Duration(float64(totalNewTime)/float64(newEvents)) * time.Millisecond

			for i := int64(0); i < newEvents; i++ {
				get.gcEvents = append(get.gcEvents, GCEvent{
					Timestamp:  metrics.Timestamp,
					Generation: "old",
					Duration:   avgDuration,
					Before:     metrics.OldUsed,
					After:      metrics.OldUsed,
					Collected:  0,
				})
			}
		}
	}

	// Update last known values
	get.lastGCCounts["young"] = metrics.YoungGCCount
	get.lastGCCounts["old"] = metrics.OldGCCount
	get.lastGCTimes["young"] = metrics.YoungGCTime
	get.lastGCTimes["old"] = metrics.OldGCTime

	// Cleanup old events
	get.cleanupOldEvents()
}

func (get *GCEventTracker) GetRecentEvents(maxEvents int) []GCEvent {
	get.mu.RLock()
	defer get.mu.RUnlock()

	events := get.gcEvents
	if len(events) > maxEvents {
		events = events[len(events)-maxEvents:]
	}

	result := make([]GCEvent, len(events))
	copy(result, events)
	return result
}

func (get *GCEventTracker) GetGCFrequency(window time.Duration) float64 {
	get.mu.RLock()
	defer get.mu.RUnlock()

	cutoff := time.Now().Add(-window)
	count := 0
	for _, event := range get.gcEvents {
		if event.Timestamp.After(cutoff) {
			count++
		}
	}
	return float64(count) / window.Minutes()
}

func (get *GCEventTracker) cleanupOldEvents() {
	cutoff := time.Now().Add(-get.windowDuration)
	var recentEvents []GCEvent
	for _, event := range get.gcEvents {
		if event.Timestamp.After(cutoff) {
			recentEvents = append(recentEvents, event)
		}
	}
	get.gcEvents = recentEvents
}
