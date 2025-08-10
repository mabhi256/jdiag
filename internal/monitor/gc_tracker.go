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

func (get *GCEventTracker) ProcessGCMetrics(metrics *jmx.MBeanSnapshot) {
	get.mu.Lock()
	defer get.mu.Unlock()

	// Extract derived memory usage from Memory.Heap and generation-specific pools
	// heapUsed := metrics.Memory.Heap.Used

	// Calculate young generation usage from G1Eden + G1Survivor
	youngUsed := metrics.Memory.G1Eden.Usage.Used + metrics.Memory.G1Survivor.Usage.Used

	// Calculate old generation usage from G1OldGen
	oldUsed := metrics.Memory.G1OldGen.Usage.Used

	// Young generation GC processing
	if lastCount, exists := get.lastGCCounts["young"]; exists {
		if metrics.GC.YoungGCCount > lastCount {
			newEvents := metrics.GC.YoungGCCount - lastCount
			totalNewTime := metrics.GC.YoungGCTime - get.lastGCTimes["young"]
			avgDuration := time.Duration(float64(totalNewTime)/float64(newEvents)) * time.Millisecond

			// Extract before/after data from LastGC if available and young gen
			var beforeMem, afterMem int64
			if get.isLastGCYoungGen(metrics) {
				beforeMem = metrics.GC.LastGC.EdenBefore + metrics.GC.LastGC.SurvivorBefore
				afterMem = metrics.GC.LastGC.EdenAfter + metrics.GC.LastGC.SurvivorAfter
			} else {
				// Fallback to current usage
				beforeMem = youngUsed
				afterMem = youngUsed
			}

			for range newEvents {
				get.gcEvents = append(get.gcEvents, GCEvent{
					Timestamp:  metrics.Timestamp,
					Generation: "young",
					Duration:   avgDuration,
					Before:     beforeMem,
					After:      afterMem,
					Collected:  beforeMem - afterMem,
				})
			}
		}
	}

	// Old generation GC processing
	if lastCount, exists := get.lastGCCounts["old"]; exists {
		if metrics.GC.OldGCCount > lastCount {
			newEvents := metrics.GC.OldGCCount - lastCount
			totalNewTime := metrics.GC.OldGCTime - get.lastGCTimes["old"]
			avgDuration := time.Duration(float64(totalNewTime)/float64(newEvents)) * time.Millisecond

			// Extract before/after data from LastGC if available and old gen
			var beforeMem, afterMem int64
			if !get.isLastGCYoungGen(metrics) {
				beforeMem = metrics.GC.LastGC.OldBefore
				afterMem = metrics.GC.LastGC.OldAfter
			} else {
				// Fallback to current usage
				beforeMem = oldUsed
				afterMem = oldUsed
			}

			for range newEvents {
				get.gcEvents = append(get.gcEvents, GCEvent{
					Timestamp:  metrics.Timestamp,
					Generation: "old",
					Duration:   avgDuration,
					Before:     beforeMem,
					After:      afterMem,
					Collected:  beforeMem - afterMem,
				})
			}
		}
	}

	// Update last known values
	get.lastGCCounts["young"] = metrics.GC.YoungGCCount
	get.lastGCCounts["old"] = metrics.GC.OldGCCount
	get.lastGCTimes["young"] = metrics.GC.YoungGCTime
	get.lastGCTimes["old"] = metrics.GC.OldGCTime

	// Cleanup old events
	get.cleanupOldEvents()
}

// isLastGCYoungGen determines if the last GC was a young generation GC
// based on which pools had significant changes
func (get *GCEventTracker) isLastGCYoungGen(metrics *jmx.MBeanSnapshot) bool {
	lastGC := metrics.GC.LastGC

	// Check if eden or survivor had significant changes (indicating young gen GC)
	edenChange := lastGC.EdenBefore - lastGC.EdenAfter
	survivorChange := lastGC.SurvivorBefore - lastGC.SurvivorAfter
	oldChange := lastGC.OldBefore - lastGC.OldAfter

	// Young gen GC typically clears most of eden/survivor but may not touch old
	if edenChange > 0 || survivorChange > 0 {
		return true
	}

	// If old gen had significant changes but eden/survivor didn't, it's likely old gen GC
	if oldChange > 0 && edenChange <= 0 && survivorChange <= 0 {
		return false
	}

	// Default to young gen for ambiguous cases
	return true
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
