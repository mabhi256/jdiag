package jmx

import (
	"fmt"
	"strings"
)

func (jc *JMXPoller) collectGCMetrics(metrics *MBeanSnapshot) error {
	client := jc.getEffectiveClient()

	// Query all GC collectors
	gcs, err := client.QueryMBeanPattern("java.lang:type=GarbageCollector,name=*")
	if err != nil {
		return fmt.Errorf("failed to query GC metrics: %w", err)
	}

	// Process each GC collector separately
	for _, gc := range gcs {
		gcName := gc["Name"].(string)
		if gcName == "" {
			continue
		}

		count, countOk := gc["CollectionCount"].(float64)
		gcTime, timeOk := gc["CollectionTime"].(float64)

		if !countOk || !timeOk {
			continue
		}

		// Extract Memory Pool Names managed by this GC
		var managedPools []string
		if poolNames, ok := gc["MemoryPoolNames"].([]interface{}); ok {
			for _, pool := range poolNames {
				if poolName, ok := pool.(string); ok {
					managedPools = append(managedPools, poolName)
				}
			}
		}

		if isYoungGenGC(gcName) {
			metrics.GC.YoungGCCount = int64(count)
			metrics.GC.YoungGCTime = int64(gcTime)
			metrics.GC.YoungGCManagedPools = managedPools

			if lastGcInfo, ok := gc["LastGcInfo"].(map[string]any); ok && lastGcInfo != nil {
				metrics.GC.LastYoungGC = jc.extractLastGCInfo(lastGcInfo)
			}
		} else if isOldGenGC(gcName) {
			metrics.GC.OldGCCount = int64(count)
			metrics.GC.OldGCTime = int64(gcTime)
			metrics.GC.OldGCManagedPools = managedPools

			// Extract LastGcInfo for Old GC - RAW DATA ONLY
			if lastGcInfo, ok := gc["LastGcInfo"].(map[string]any); ok && lastGcInfo != nil {
				metrics.GC.LastOldGC = jc.extractLastGCInfo(lastGcInfo)
			}
		}

		// GC validity check
		if valid, ok := gc["Valid"].(bool); ok && !valid {
			metrics.GC.InvalidGCs = append(metrics.GC.InvalidGCs, gcName)
		}
	}

	return nil
}

func (jc *JMXPoller) extractLastGCInfo(lastGcInfo map[string]any) LastGCInfo {
	var lastGC LastGCInfo

	// Extract raw timing data (milliseconds since JVM startup)
	if startTime, ok := lastGcInfo["startTime"].(float64); ok {
		lastGC.StartTime = int64(startTime)
	}
	if endTime, ok := lastGcInfo["endTime"].(float64); ok {
		lastGC.EndTime = int64(endTime)
	}
	if duration, ok := lastGcInfo["duration"].(float64); ok {
		lastGC.Duration = int64(duration)
	}
	if id, ok := lastGcInfo["id"].(float64); ok {
		lastGC.Id = int64(id)
	}
	if threadCount, ok := lastGcInfo["GcThreadCount"].(float64); ok {
		lastGC.GcThreadCount = int64(threadCount)
	}

	if memBefore, ok := lastGcInfo["memoryUsageBeforeGc"].(map[string]any); ok {
		lastGC.MemoryUsageBeforeGc = jc.extractMemoryUsageMap(memBefore)
		jc.extractPerPoolMemoryUsage(memBefore, &lastGC, "before")
	}
	if memAfter, ok := lastGcInfo["memoryUsageAfterGc"].(map[string]any); ok {
		lastGC.MemoryUsageAfterGc = jc.extractMemoryUsageMap(memAfter)
		jc.extractPerPoolMemoryUsage(memAfter, &lastGC, "after")
	}

	return lastGC
}

func (jc *JMXPoller) extractMemoryUsageMap(memoryMap map[string]any) map[string]MemoryUsage {
	result := make(map[string]MemoryUsage)

	for poolName, poolInfo := range memoryMap {
		if poolData, ok := poolInfo.(map[string]any); ok {
			var poolValue map[string]any
			if valueMap, ok := poolData["value"].(map[string]any); ok {
				poolValue = valueMap
			} else {
				poolValue = poolData
			}

			var memUsage MemoryUsage
			if used, ok := poolValue["used"].(float64); ok {
				memUsage.Used = int64(used)
			}
			if committed, ok := poolValue["committed"].(float64); ok {
				memUsage.Committed = int64(committed)
			}
			if max, ok := poolValue["max"].(float64); ok {
				memUsage.Max = int64(max)
			}
			if init, ok := poolValue["init"].(float64); ok {
				memUsage.Init = int64(init)
			}

			result[poolName] = memUsage
		}
	}

	return result
}

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

// func (jc *JMXPoller) extractGCName(gc map[string]any) string {
// 	// Try multiple possible fields for GC name
// 	nameFields := []string{"Name", "name"}
// 	for _, field := range nameFields {
// 		if name, ok := gc[field].(string); ok && name != "" {
// 			return name
// 		}
// 	}

// 	// Extract from ObjectName
// 	objectNameFields := []string{"ObjectName", "objectName"}
// 	for _, field := range objectNameFields {
// 		if objectName, ok := gc[field].(string); ok {
// 			return extractGCName(objectName)
// 		}
// 	}

// 	return ""
// }

// func extractGCName(objectName string) string {
// 	if objectName == "" {
// 		return ""
// 	}
// 	if idx := strings.Index(objectName, "name="); idx != -1 {
// 		name := objectName[idx+5:]
// 		if commaIdx := strings.Index(name, ","); commaIdx != -1 {
// 			name = name[:commaIdx]
// 		}
// 		return name
// 	}
// 	return objectName
// }

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
