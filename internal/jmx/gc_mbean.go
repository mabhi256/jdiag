package jmx

import (
	"fmt"
	"strings"
	"time"
)

// ===== GC METRICS =====
// Fetch ALL available attributes for comprehensive GC analysis
func (jc *JMXPoller) collectGCMetrics(metrics *JVMSnapshot) error {
	client := jc.getEffectiveClient()

	// 1. Basic GC Metrics - Get ALL available attributes
	gcs, err := client.QueryMBeanPattern("java.lang:type=GarbageCollector,name=*")
	if err != nil {
		return fmt.Errorf("failed to query GC metrics: %w", err)
	}

	var lastGCTime time.Time
	for _, gc := range gcs {
		gcName := jc.extractGCName(gc)
		if gcName == "" {
			continue
		}

		count, countOk := gc["CollectionCount"].(float64)
		gcTime, timeOk := gc["CollectionTime"].(float64)

		if !countOk || !timeOk {
			continue
		}

		if isYoungGenGC(gcName) {
			metrics.YoungGCCount += int64(count)
			metrics.YoungGCTime += int64(gcTime)
		} else if isOldGenGC(gcName) {
			metrics.OldGCCount += int64(count)
			metrics.OldGCTime += int64(gcTime)
		}

		// Extract Memory Pool Names managed by this GC
		if poolNames, ok := gc["MemoryPoolNames"].([]interface{}); ok {
			var managedPools []string
			for _, pool := range poolNames {
				if poolName, ok := pool.(string); ok {
					managedPools = append(managedPools, poolName)
				}
			}
			if isYoungGenGC(gcName) {
				metrics.YoungGCManagedPools = managedPools
			} else if isOldGenGC(gcName) {
				metrics.OldGCManagedPools = managedPools
			}
		}

		// Extract Last GC Info for detailed analysis
		if lastGcInfo, ok := gc["LastGcInfo"].(map[string]any); ok && lastGcInfo != nil {
			if startTime, ok := lastGcInfo["startTime"].(float64); ok {
				gcStartTime := time.Unix(int64(startTime/1000), 0)
				if gcStartTime.After(lastGCTime) {
					lastGCTime = gcStartTime
					metrics.LastGCTimestamp = gcStartTime

					// Extract all available GC info fields
					if gcCause, ok := lastGcInfo["GcCause"].(string); ok {
						metrics.LastGCCause = gcCause
					}
					if gcAction, ok := lastGcInfo["GcAction"].(string); ok {
						metrics.LastGCAction = gcAction
					}
					if gcName, ok := lastGcInfo["GcName"].(string); ok {
						metrics.LastGCName = gcName
					}
					if duration, ok := lastGcInfo["duration"].(float64); ok {
						metrics.LastGCDuration = int64(duration)
					}
					if endTime, ok := lastGcInfo["endTime"].(float64); ok {
						metrics.LastGCEndTime = time.Unix(int64(endTime/1000), 0)
					}
					if id, ok := lastGcInfo["id"].(float64); ok {
						metrics.LastGCId = int64(id)
					}

					if memBefore, ok := lastGcInfo["memoryUsageBeforeGc"].(map[string]any); ok {
						metrics.LastGCMemoryBefore = jc.extractTotalMemoryFromGCInfo(memBefore)
					}
					if memAfter, ok := lastGcInfo["memoryUsageAfterGc"].(map[string]any); ok {
						metrics.LastGCMemoryAfter = jc.extractTotalMemoryFromGCInfo(memAfter)
					}

					if metrics.LastGCMemoryBefore > 0 && metrics.LastGCMemoryAfter > 0 {
						metrics.LastGCMemoryFreed = metrics.LastGCMemoryBefore - metrics.LastGCMemoryAfter
					}

					// Extract G1 phase timings if available
					if phases, ok := lastGcInfo["phases"].(map[string]any); ok {
						jc.extractG1PhaseTimings(phases, metrics)
					}
				}
			}
		}

		// GC validity
		if valid, ok := gc["Valid"].(bool); ok && !valid {
			metrics.InvalidGCs = append(metrics.InvalidGCs, gcName)
		}
	}

	// 2. G1GC-Specific Metrics (try multiple patterns)
	g1Patterns := []string{
		"com.sun.management:type=HotSpotGarbageCollector,name=G1 Young Generation",
		"java.lang:type=GarbageCollector,name=G1 Young Generation",
	}

	for _, pattern := range g1Patterns {
		g1Info, err := client.QueryMBean(pattern)
		if err == nil {
			// Extract G1-specific metrics if available
			if lastGcInfo, ok := g1Info["LastGcInfo"].(map[string]any); ok && lastGcInfo != nil {
				if phases, ok := lastGcInfo["phases"].(map[string]any); ok {
					jc.extractG1PhaseTimings(phases, metrics)
				}
			}
			break
		}
	}

	// Try to extract G1 region size from various sources
	jc.extractG1RegionSize(metrics)

	// 3. Calculate GC Efficiency Metrics
	if metrics.YoungGCTime > 0 && metrics.LastGCMemoryFreed > 0 {
		freedMB := float64(metrics.LastGCMemoryFreed) / (1024 * 1024)
		timeMs := float64(metrics.YoungGCTime)
		if timeMs > 0 {
			metrics.YoungGCEfficiency = freedMB / timeMs
		}
	}

	if metrics.OldGCTime > 0 && metrics.LastGCMemoryFreed > 0 {
		freedMB := float64(metrics.LastGCMemoryFreed) / (1024 * 1024)
		timeMs := float64(metrics.OldGCTime)
		if timeMs > 0 {
			metrics.OldGCEfficiency = freedMB / timeMs
		}
	}

	totalTime := metrics.YoungGCTime + metrics.OldGCTime
	if totalTime > 0 && metrics.LastGCMemoryFreed > 0 {
		freedMB := float64(metrics.LastGCMemoryFreed) / (1024 * 1024)
		timeMs := float64(totalTime)
		metrics.OverallGCEfficiency = freedMB / timeMs
	}

	return nil
}

// Extract GC name from various possible sources in the GC data
func (jc *JMXPoller) extractGCName(gc map[string]any) string {
	// Try multiple possible fields for GC name
	nameFields := []string{"Name", "name"}
	for _, field := range nameFields {
		if name, ok := gc[field].(string); ok && name != "" {
			return name
		}
	}

	// Extract from ObjectName
	objectNameFields := []string{"ObjectName", "objectName"}
	for _, field := range objectNameFields {
		if objectName, ok := gc[field].(string); ok {
			return extractGCName(objectName)
		}
	}

	return ""
}

func extractGCName(objectName string) string {
	if objectName == "" {
		return ""
	}
	if idx := strings.Index(objectName, "name="); idx != -1 {
		name := objectName[idx+5:]
		if commaIdx := strings.Index(name, ","); commaIdx != -1 {
			name = name[:commaIdx]
		}
		return name
	}
	return objectName
}

// Helper function to extract total memory from GC info memory maps
func (jc *JMXPoller) extractTotalMemoryFromGCInfo(memoryMap map[string]any) int64 {
	var total int64
	for _, poolInfo := range memoryMap {
		if poolData, ok := poolInfo.(map[string]any); ok {
			if used, ok := poolData["used"].(float64); ok {
				total += int64(used)
			}
		}
	}
	return total
}

// Helper method to extract G1 phase timings
func (jc *JMXPoller) extractG1PhaseTimings(phases map[string]any, metrics *JVMSnapshot) {
	phaseMap := map[string]*int64{
		"Root Scanning":       &metrics.G1RootScanTime,
		"Object Copy":         &metrics.G1ObjectCopyTime,
		"Termination":         &metrics.G1TerminationTime,
		"Other":               &metrics.G1OtherTime,
		"Choose CSet":         &metrics.G1ChooseCSetTime,
		"ref_proc":            &metrics.G1RefProcTime,
		"ref_enq":             &metrics.G1RefEnqTime,
		"redirty_cards":       &metrics.G1RedirtyCardsTime,
		"code_root_purge":     &metrics.G1CodeRootPurgeTime,
		"string_dedup":        &metrics.G1StringDedupTime,
		"young_free_cset":     &metrics.G1YoungFreeCSetTime,
		"non_young_free_cset": &metrics.G1NonYoungFreeCSetTime,
		"Evacuation Pause":    &metrics.G1EvacuationPauseTime,
		"Update RS":           &metrics.G1UpdateRSTime,
		"Scan RS":             &metrics.G1ScanRSTime,
	}

	for phaseName, metricPtr := range phaseMap {
		if phaseTime, ok := phases[phaseName].(float64); ok {
			*metricPtr = int64(phaseTime)
		}
	}
}

// Helper method to extract G1 region size
func (jc *JMXPoller) extractG1RegionSize(metrics *JVMSnapshot) {
	client := jc.getEffectiveClient()

	// Try to get from Runtime system properties
	runtime, err := client.QueryMBean("java.lang:type=Runtime")
	if err == nil {
		if props, ok := runtime["SystemProperties"].(map[string]any); ok {
			if regionSize, exists := props["G1HeapRegionSize"]; exists {
				if size, ok := regionSize.(float64); ok {
					metrics.G1HeapRegionSize = int64(size)
				}
			}
		}
	}

	// Calculate derived G1 metrics
	if metrics.G1HeapRegionSize > 0 && metrics.HeapMax > 0 {
		metrics.G1TotalRegions = metrics.HeapMax / metrics.G1HeapRegionSize
		if metrics.HeapUsed > 0 {
			metrics.G1UsedRegions = (metrics.HeapUsed + metrics.G1HeapRegionSize - 1) / metrics.G1HeapRegionSize
			metrics.G1FreeRegions = metrics.G1TotalRegions - metrics.G1UsedRegions
		}
	}
}
