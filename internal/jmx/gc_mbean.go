package jmx

import (
	"fmt"
	"strings"
	"time"
)

func (jc *JMXPoller) collectGCMetrics(metrics *MBeanSnapshot) error {
	client := jc.getEffectiveClient()

	// Query all GC collectors
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

		// Basic GC metrics - raw counts and times
		count, countOk := gc["CollectionCount"].(float64)
		gcTime, timeOk := gc["CollectionTime"].(float64)

		if !countOk || !timeOk {
			continue
		}

		// Classify GC type and store raw data
		if isYoungGenGC(gcName) {
			metrics.GC.YoungGCCount = int64(count)
			metrics.GC.YoungGCTime = int64(gcTime)
		} else if isOldGenGC(gcName) {
			metrics.GC.OldGCCount = int64(count)
			metrics.GC.OldGCTime = int64(gcTime)
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
				metrics.GC.YoungGCManagedPools = managedPools
			} else if isOldGenGC(gcName) {
				metrics.GC.OldGCManagedPools = managedPools
			}
		}

		// Extract LastGcInfo - RAW DATA ONLY
		if lastGcInfo, ok := gc["LastGcInfo"].(map[string]any); ok && lastGcInfo != nil {
			if startTime, ok := lastGcInfo["startTime"].(float64); ok {
				gcStartTime := time.Unix(int64(startTime/1000), 0)
				if gcStartTime.After(lastGCTime) {
					lastGCTime = gcStartTime

					// Build LastGCInfo structure
					var lastGC LastGCInfo
					lastGC.Timestamp = gcStartTime

					// Raw GC timing data
					if duration, ok := lastGcInfo["duration"].(float64); ok {
						lastGC.Duration = int64(duration)
					}
					if endTime, ok := lastGcInfo["endTime"].(float64); ok {
						lastGC.EndTime = time.Unix(int64(endTime/1000), 0)
					}
					if id, ok := lastGcInfo["id"].(float64); ok {
						lastGC.Id = int64(id)
					}
					if threadCount, ok := lastGcInfo["GcThreadCount"].(float64); ok {
						lastGC.ThreadCount = int64(threadCount)
					}

					// Extract per-pool memory usage before/after GC
					if memBefore, ok := lastGcInfo["memoryUsageBeforeGc"].(map[string]any); ok {
						jc.extractPerPoolMemoryUsage(memBefore, &lastGC, "before")
					}
					if memAfter, ok := lastGcInfo["memoryUsageAfterGc"].(map[string]any); ok {
						jc.extractPerPoolMemoryUsage(memAfter, &lastGC, "after")
					}

					metrics.GC.LastGC = lastGC
				}
			}
		}

		// GC validity check
		if valid, ok := gc["Valid"].(bool); ok && !valid {
			metrics.GC.InvalidGCs = append(metrics.GC.InvalidGCs, gcName)
		}
	}

	return nil
}

// Extract per-pool memory usage from GC info
func (jc *JMXPoller) extractPerPoolMemoryUsage(memoryMap map[string]any, lastGC *LastGCInfo, timing string) {
	for poolName, poolInfo := range memoryMap {
		if poolData, ok := poolInfo.(map[string]any); ok {
			var poolValue map[string]any
			if valueMap, ok := poolData["value"].(map[string]any); ok {
				poolValue = valueMap
			} else {
				poolValue = poolData
			}

			if used, ok := poolValue["used"].(float64); ok {
				// Store memory usage by pool name and timing
				switch timing {
				case "before":
					switch {
					case strings.Contains(strings.ToLower(poolName), "eden"):
						lastGC.EdenBefore = int64(used)
					case strings.Contains(strings.ToLower(poolName), "survivor"):
						lastGC.SurvivorBefore = int64(used)
					case strings.Contains(strings.ToLower(poolName), "old"):
						lastGC.OldBefore = int64(used)
					case strings.Contains(strings.ToLower(poolName), "metaspace"):
						lastGC.MetaspaceBefore = int64(used)
					}
				case "after":
					switch {
					case strings.Contains(strings.ToLower(poolName), "eden"):
						lastGC.EdenAfter = int64(used)
					case strings.Contains(strings.ToLower(poolName), "survivor"):
						lastGC.SurvivorAfter = int64(used)
					case strings.Contains(strings.ToLower(poolName), "old"):
						lastGC.OldAfter = int64(used)
					case strings.Contains(strings.ToLower(poolName), "metaspace"):
						lastGC.MetaspaceAfter = int64(used)
					}
				}
			}
		}
	}
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

// GC type classification helpers
func isYoungGenGC(gcName string) bool {
	lowerName := strings.ToLower(gcName)
	return strings.Contains(lowerName, "young") ||
		strings.Contains(lowerName, "copy") ||
		strings.Contains(lowerName, "scavenge") ||
		strings.Contains(lowerName, "parnew") ||
		strings.Contains(lowerName, "ps scavenge") ||
		strings.Contains(lowerName, "g1 young") ||
		strings.Contains(lowerName, "g1 young generation") ||
		(strings.Contains(lowerName, "g1") &&
			!strings.Contains(lowerName, "old") &&
			!strings.Contains(lowerName, "mixed"))
}

func isOldGenGC(gcName string) bool {
	lowerName := strings.ToLower(gcName)
	return strings.Contains(lowerName, "old") ||
		strings.Contains(lowerName, "cms") ||
		strings.Contains(lowerName, "marksweep") ||
		strings.Contains(lowerName, "parallel old") ||
		strings.Contains(lowerName, "ps marksweep") ||
		strings.Contains(lowerName, "g1 old") ||
		strings.Contains(lowerName, "g1 old generation") ||
		strings.Contains(lowerName, "g1 mixed") ||
		strings.Contains(lowerName, "g1 concurrent")
}
