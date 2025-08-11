package watch

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

	// JVM start time for timestamp conversion
	jvmStartTime time.Time

	// Current raw data (for calculations)
	currentSnapshot *jmx.MBeanSnapshot
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

	// Store current snapshot for calculations
	get.currentSnapshot = metrics

	// Set JVM start time for timestamp conversion
	if !metrics.Runtime.StartTime.IsZero() {
		get.jvmStartTime = metrics.Runtime.StartTime
	}

	// Calculate current memory usage for fallback scenarios
	youngUsed := metrics.Memory.G1Eden.Usage.Used + metrics.Memory.G1Survivor.Usage.Used
	oldUsed := metrics.Memory.G1OldGen.Usage.Used

	// Process Young Generation GC events
	get.processGenerationGC("young", metrics.GC.YoungGCCount, metrics.GC.YoungGCTime,
		metrics.GC.LastYoungGC, youngUsed, metrics.Timestamp)

	// Process Old Generation GC events
	get.processGenerationGC("old", metrics.GC.OldGCCount, metrics.GC.OldGCTime,
		metrics.GC.LastOldGC, oldUsed, metrics.Timestamp)

	// Update last known values
	get.lastGCCounts["young"] = metrics.GC.YoungGCCount
	get.lastGCCounts["old"] = metrics.GC.OldGCCount
	get.lastGCTimes["young"] = metrics.GC.YoungGCTime
	get.lastGCTimes["old"] = metrics.GC.OldGCTime

	// Cleanup old events
	get.cleanupOldEvents()
}

// processGenerationGC handles GC event processing for a specific generation
func (get *GCEventTracker) processGenerationGC(generation string, currentCount, currentTime int64,
	lastGCInfo jmx.LastGCInfo, fallbackUsed int64, timestamp time.Time) {

	lastCount, exists := get.lastGCCounts[generation]
	if !exists {
		return // First run, no previous data to compare
	}

	if currentCount <= lastCount {
		return // No new GC events
	}

	// Calculate metrics for new GC events
	newEvents := currentCount - lastCount
	totalNewTime := currentTime - get.lastGCTimes[generation]
	avgDuration := time.Duration(float64(totalNewTime)/float64(newEvents)) * time.Millisecond

	// Determine memory before/after values and actual timestamp
	var beforeMem, afterMem, collected int64
	var eventTimestamp time.Time

	if lastGCInfo.IsValid() && get.isRecentGC(lastGCInfo) {
		// Use actual GC data if available and recent
		beforeMem, afterMem, collected = get.extractMemoryFromGCInfo(lastGCInfo, generation)
		eventTimestamp = get.convertJVMTimestamp(lastGCInfo.EndTime)
	} else {
		// Fallback to current usage (less accurate but better than nothing)
		beforeMem = fallbackUsed
		afterMem = fallbackUsed
		collected = 0
		eventTimestamp = timestamp
	}

	// If we have exact timing for a single event, use precise duration
	actualDuration := avgDuration
	if lastGCInfo.IsValid() && newEvents == 1 {
		actualDuration = time.Duration(lastGCInfo.Duration) * time.Millisecond
	}

	// Create GC events for each new collection
	for range newEvents {
		get.gcEvents = append(get.gcEvents, GCEvent{
			Id:         lastGCInfo.Id,
			Timestamp:  eventTimestamp,
			Generation: generation,
			Duration:   actualDuration,
			Before:     beforeMem,
			After:      afterMem,
			Collected:  collected,
		})
	}
}

// ===== ALL DERIVED DATA CALCULATIONS =====

// GetTotalGCCount returns total GC count across all generations
func (get *GCEventTracker) GetTotalGCCount() int64 {
	get.mu.RLock()
	defer get.mu.RUnlock()

	if get.currentSnapshot == nil {
		return 0
	}
	return get.currentSnapshot.GC.YoungGCCount + get.currentSnapshot.GC.OldGCCount
}

// GetTotalGCTime returns total GC time across all generations
func (get *GCEventTracker) GetTotalGCTime() int64 {
	get.mu.RLock()
	defer get.mu.RUnlock()

	if get.currentSnapshot == nil {
		return 0
	}
	return get.currentSnapshot.GC.YoungGCTime + get.currentSnapshot.GC.OldGCTime
}

// GetYoungGCAverage returns average young GC time
func (get *GCEventTracker) GetYoungGCAverage() float64 {
	get.mu.RLock()
	defer get.mu.RUnlock()

	if get.currentSnapshot == nil || get.currentSnapshot.GC.YoungGCCount == 0 {
		return 0
	}
	return float64(get.currentSnapshot.GC.YoungGCTime) / float64(get.currentSnapshot.GC.YoungGCCount)
}

// GetOldGCAverage returns average old GC time
func (get *GCEventTracker) GetOldGCAverage() float64 {
	get.mu.RLock()
	defer get.mu.RUnlock()

	if get.currentSnapshot == nil || get.currentSnapshot.GC.OldGCCount == 0 {
		return 0
	}
	return float64(get.currentSnapshot.GC.OldGCTime) / float64(get.currentSnapshot.GC.OldGCCount)
}

// GetOverallGCAverage returns average GC time across all generations
func (get *GCEventTracker) GetOverallGCAverage() float64 {
	totalCount := get.GetTotalGCCount()
	if totalCount == 0 {
		return 0
	}
	return float64(get.GetTotalGCTime()) / float64(totalCount)
}

// GetMostRecentGCInfo determines which GC happened most recently using raw endTime comparison
func (get *GCEventTracker) GetMostRecentGCInfo() (jmx.LastGCInfo, string) {
	get.mu.RLock()
	defer get.mu.RUnlock()

	if get.currentSnapshot == nil {
		return jmx.LastGCInfo{}, "none"
	}

	youngGC := get.currentSnapshot.GC.LastYoungGC
	oldGC := get.currentSnapshot.GC.LastOldGC

	// Compare raw endTime values to determine which happened most recently
	if !youngGC.IsValid() && !oldGC.IsValid() {
		return jmx.LastGCInfo{}, "none"
	}

	if !youngGC.IsValid() {
		return oldGC, "old"
	}

	if !oldGC.IsValid() {
		return youngGC, "young"
	}

	// Both are valid, compare raw endTime (milliseconds since JVM startup)
	if youngGC.EndTime > oldGC.EndTime {
		return youngGC, "young"
	}

	return oldGC, "old"
}

// GetMostRecentGCDetails returns processed details about the most recent GC
func (get *GCEventTracker) GetMostRecentGCDetails() (id int64, generation string, timestamp time.Time, duration time.Duration, collected int64) {
	gcInfo, gcType := get.GetMostRecentGCInfo()
	if !gcInfo.IsValid() {
		return -1, "none", time.Time{}, 0, 0
	}

	timestamp = get.convertJVMTimestamp(gcInfo.EndTime)
	duration = time.Duration(gcInfo.Duration) * time.Millisecond

	// Calculate collected amount based on generation
	switch gcType {
	case "young":
		edenCollected := gcInfo.EdenBefore - gcInfo.EdenAfter
		survivorCollected := gcInfo.SurvivorBefore - gcInfo.SurvivorAfter
		collected = edenCollected + survivorCollected
	case "old":
		collected = gcInfo.OldBefore - gcInfo.OldAfter
	default:
		// Total collection across all pools
		totalBefore := gcInfo.EdenBefore + gcInfo.SurvivorBefore + gcInfo.OldBefore
		totalAfter := gcInfo.EdenAfter + gcInfo.SurvivorAfter + gcInfo.OldAfter
		collected = totalBefore - totalAfter
	}

	// Ensure collected is not negative
	if collected < 0 {
		collected = 0
	}

	return gcInfo.Id, gcType, timestamp, duration, collected
}

// GetGCPressureLevel calculates the current GC pressure level
func (get *GCEventTracker) GetGCPressureLevel(window time.Duration) string {
	overhead := get.CalculateGCOverhead(window)
	maxPause := get.GetMaxPause(window)
	frequency := get.GetGCFrequency(window)
	longPauses := get.GetLongPauses(100*time.Millisecond, window)

	switch {
	case overhead > 0.20: // 20% overhead
		return "critical"
	case overhead > 0.10: // 10% overhead
		return "high"
	case maxPause > 1*time.Second: // Long pauses
		return "high"
	case overhead > 0.05: // 5% overhead
		return "moderate"
	case frequency > 60: // More than 1 GC per second
		return "moderate"
	case longPauses > 5: // Multiple long pauses
		return "moderate"
	default:
		return "low"
	}
}

// GetGCActivityLevel determines activity level based on count and average time
func (get *GCEventTracker) GetGCActivityLevel(count int64, avgTime, frequency float64) string {
	switch {
	case avgTime > 500:
		return "High Impact"
	case avgTime > 100:
		return "Moderate Impact"
	case frequency > 120: // More than 2 per second
		return "Very Frequent"
	case frequency > 60: // More than 1 per second
		return "Frequent"
	case count > 1000:
		return "Regular"
	case count > 100:
		return "Normal"
	default:
		return "Low"
	}
}

// convertJVMTimestamp converts JVM startup milliseconds to actual time
func (get *GCEventTracker) convertJVMTimestamp(jvmMillis int64) time.Time {
	if get.jvmStartTime.IsZero() {
		return time.Now() // fallback if we don't have JVM start time
	}
	return get.jvmStartTime.Add(time.Duration(jvmMillis) * time.Millisecond)
}

// extractMemoryFromGCInfo extracts the appropriate memory values from raw GC info
func (get *GCEventTracker) extractMemoryFromGCInfo(gcInfo jmx.LastGCInfo, generation string) (before, after, collected int64) {
	switch generation {
	case "young":
		before = gcInfo.EdenBefore + gcInfo.SurvivorBefore
		after = gcInfo.EdenAfter + gcInfo.SurvivorAfter
		collected = (gcInfo.EdenBefore - gcInfo.EdenAfter) + (gcInfo.SurvivorBefore - gcInfo.SurvivorAfter)
	case "old":
		before = gcInfo.OldBefore
		after = gcInfo.OldAfter
		collected = gcInfo.OldBefore - gcInfo.OldAfter
	default:
		// Fallback - calculate total from all pools
		before = gcInfo.EdenBefore + gcInfo.SurvivorBefore + gcInfo.OldBefore
		after = gcInfo.EdenAfter + gcInfo.SurvivorAfter + gcInfo.OldAfter
		collected = before - after
	}

	// Ensure collected is not negative
	if collected < 0 {
		collected = 0
	}

	return before, after, collected
}

// isRecentGC checks if the GC info represents a recent collection
func (get *GCEventTracker) isRecentGC(gcInfo jmx.LastGCInfo) bool {
	if !gcInfo.IsValid() {
		return false
	}

	// Convert JVM timestamp to actual time and check if recent
	actualTime := get.convertJVMTimestamp(gcInfo.EndTime)
	return time.Since(actualTime) < 30*time.Second
}

// ===== TIME-WINDOW BASED CALCULATIONS =====

// GetRecentEvents returns recent GC events
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

// GetGCFrequency returns overall GC frequency for the given window
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

// GetGCFrequencyByGeneration returns GC frequency for a specific generation
func (get *GCEventTracker) GetGCFrequencyByGeneration(generation string, window time.Duration) float64 {
	get.mu.RLock()
	defer get.mu.RUnlock()

	cutoff := time.Now().Add(-window)
	count := 0
	for _, event := range get.gcEvents {
		if event.Generation == generation && event.Timestamp.After(cutoff) {
			count++
		}
	}
	return float64(count) / window.Minutes()
}

// GetAveragePauseTime returns the average pause time for recent GCs
func (get *GCEventTracker) GetAveragePauseTime(window time.Duration) time.Duration {
	get.mu.RLock()
	defer get.mu.RUnlock()

	cutoff := time.Now().Add(-window)
	var totalDuration time.Duration
	count := 0

	for _, event := range get.gcEvents {
		if event.Timestamp.After(cutoff) {
			totalDuration += event.Duration
			count++
		}
	}

	if count == 0 {
		return 0
	}

	return totalDuration / time.Duration(count)
}

// GetTotalGCTime returns total time spent in GC for the given window
func (get *GCEventTracker) GetTotalGCTimeWindow(window time.Duration) time.Duration {
	get.mu.RLock()
	defer get.mu.RUnlock()

	cutoff := time.Now().Add(-window)
	var totalDuration time.Duration

	for _, event := range get.gcEvents {
		if event.Timestamp.After(cutoff) {
			totalDuration += event.Duration
		}
	}

	return totalDuration
}

// GetLongPauses returns count of GC pauses longer than threshold in the given window
func (get *GCEventTracker) GetLongPauses(threshold, window time.Duration) int64 {
	get.mu.RLock()
	defer get.mu.RUnlock()

	cutoff := time.Now().Add(-window)
	count := int64(0)

	for _, event := range get.gcEvents {
		if event.Timestamp.After(cutoff) && event.Duration > threshold {
			count++
		}
	}

	return count
}

// GetMaxPause returns the longest pause in the given window
func (get *GCEventTracker) GetMaxPause(window time.Duration) time.Duration {
	get.mu.RLock()
	defer get.mu.RUnlock()

	cutoff := time.Now().Add(-window)
	var maxPause time.Duration

	for _, event := range get.gcEvents {
		if event.Timestamp.After(cutoff) && event.Duration > maxPause {
			maxPause = event.Duration
		}
	}

	return maxPause
}

func (get *GCEventTracker) CalculateGCOverhead(window time.Duration) float64 {
	totalGCTime := get.GetTotalGCTimeWindow(window)
	if totalGCTime == 0 {
		return 0.0
	}

	return float64(totalGCTime) / float64(window)
}

// CalculateEfficiency calculates collection efficiency metrics by generation
func (get *GCEventTracker) CalculateEfficiency(window time.Duration) (young, old, overall float64) {
	get.mu.RLock()
	defer get.mu.RUnlock()

	cutoff := time.Now().Add(-window)

	var youngCollected, youngBefore, oldCollected, oldBefore, totalCollected, totalBefore int64

	for _, event := range get.gcEvents {
		if event.Timestamp.After(cutoff) {
			totalCollected += event.Collected
			totalBefore += event.Before

			if event.Generation == "young" {
				youngCollected += event.Collected
				youngBefore += event.Before
			} else if event.Generation == "old" {
				oldCollected += event.Collected
				oldBefore += event.Before
			}
		}
	}

	// Calculate efficiency percentages
	if youngBefore > 0 {
		young = float64(youngCollected) / float64(youngBefore) * 100
	}

	if oldBefore > 0 {
		old = float64(oldCollected) / float64(oldBefore) * 100
	}

	if totalBefore > 0 {
		overall = float64(totalCollected) / float64(totalBefore) * 100
	}

	return young, old, overall
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
