package jmx

import (
	"fmt"
	"strings"
	"time"
)

// ===== MEMORY METRICS =====
// Fetch ALL available attributes, then extract what we need
func (jc *JMXPoller) collectMemoryMetrics(metrics *JVMSnapshot) error {
	client := jc.getEffectiveClient()

	// 1. Basic Memory - Get ALL available attributes (no attribute list = get everything)
	heapMemory, err := client.QueryMBean("java.lang:type=Memory")
	if err != nil {
		return fmt.Errorf("failed to query heap memory: %w", err)
	}

	// Extract heap usage
	if heapUsage, ok := heapMemory["HeapMemoryUsage"].(map[string]any); ok {
		if used, ok := heapUsage["used"].(float64); ok {
			metrics.HeapUsed = int64(used)
		}
		if committed, ok := heapUsage["committed"].(float64); ok {
			metrics.HeapCommitted = int64(committed)
		}
		if max, ok := heapUsage["max"].(float64); ok && max > 0 {
			metrics.HeapMax = int64(max)
		}
	}

	// Extract non-heap usage
	if nonHeapUsage, ok := heapMemory["NonHeapMemoryUsage"].(map[string]any); ok {
		if used, ok := nonHeapUsage["used"].(float64); ok {
			metrics.NonHeapUsed = int64(used)
		}
		if committed, ok := nonHeapUsage["committed"].(float64); ok {
			metrics.NonHeapCommitted = int64(committed)
		}
		if max, ok := nonHeapUsage["max"].(float64); ok && max > 0 {
			metrics.NonHeapMax = int64(max)
		}
	}

	// Extract additional memory metrics
	if pendingFinal, ok := heapMemory["ObjectPendingFinalizationCount"].(float64); ok {
		metrics.ObjectPendingFinalizationCount = int64(pendingFinal)
	}
	if verbose, ok := heapMemory["Verbose"].(bool); ok {
		metrics.MemoryVerboseLogging = verbose
	}

	// 2. Memory Pools - Get ALL available attributes
	pools, err := client.QueryMBeanPattern("java.lang:type=MemoryPool,name=*")
	if err != nil {
		return fmt.Errorf("failed to query memory pools: %w", err)
	}

	var totalThresholdCount int64
	for _, pool := range pools {
		poolType, typeOk := pool["Type"].(string)
		usage, usageOk := pool["Usage"].(map[string]any)

		if !typeOk || !usageOk {
			continue
		}

		// Extract pool name from multiple possible sources
		poolName := jc.extractPoolName(pool)
		if poolName == "" {
			continue
		}

		// Current Usage
		used, usedOk := usage["used"].(float64)
		committed, committedOk := usage["committed"].(float64)
		max, maxOk := usage["max"].(float64)

		if !usedOk || !committedOk {
			continue
		}

		if poolType == "HEAP" {
			if isYoungGenPool(poolName) {
				metrics.YoungUsed += int64(used)
				metrics.YoungCommitted += int64(committed)
				if maxOk && max > 0 {
					metrics.YoungMax += int64(max)
				}
				// Handle Eden allocation rate tracking
				if strings.Contains(strings.ToLower(poolName), "eden") {
					jc.calculateAllocationRate(int64(used), metrics)
				}
			} else if isOldGenPool(poolName) {
				metrics.OldUsed += int64(used)
				metrics.OldCommitted += int64(committed)
				if maxOk && max > 0 {
					metrics.OldMax += int64(max)
				}
			}
		} else if poolType == "NON_HEAP" {
			// Handle different types of non-heap pools
			if strings.Contains(strings.ToLower(poolName), "code") {
				metrics.CodeCacheUsed += int64(used)
				metrics.CodeCacheCommitted += int64(committed)
				if maxOk && max > 0 {
					metrics.CodeCacheMax += int64(max)
				}
			} else if strings.Contains(strings.ToLower(poolName), "metaspace") {
				metrics.MetaspaceUsed = int64(used)
				metrics.MetaspaceCommitted = int64(committed)
				if maxOk && max > 0 {
					metrics.MetaspaceMax = int64(max)
				}
			} else if strings.Contains(strings.ToLower(poolName), "compressed") {
				metrics.CompressedClassSpaceUsed = int64(used)
				metrics.CompressedClassSpaceCommitted = int64(committed)
				if maxOk && max > 0 {
					metrics.CompressedClassSpaceMax = int64(max)
				}
			}
		}

		// Peak Usage
		if peakUsage, ok := pool["PeakUsage"].(map[string]any); ok {
			if peakUsed, ok := peakUsage["used"].(float64); ok {
				if poolType == "HEAP" {
					if isYoungGenPool(poolName) {
						metrics.YoungPeakUsed = int64(peakUsed)
					} else if isOldGenPool(poolName) {
						metrics.OldPeakUsed = int64(peakUsed)
					}
					metrics.HeapPeakUsed += int64(peakUsed)
				} else if poolType == "NON_HEAP" {
					metrics.NonHeapPeakUsed += int64(peakUsed)
				}
			}
		}

		// Post-GC Usage (CollectionUsage)
		if collectionUsage, ok := pool["CollectionUsage"].(map[string]any); ok && collectionUsage != nil {
			if collUsed, ok := collectionUsage["used"].(float64); ok {
				if poolType == "HEAP" {
					if isYoungGenPool(poolName) {
						metrics.YoungAfterLastGC += int64(collUsed)
					} else if isOldGenPool(poolName) {
						metrics.OldAfterLastGC += int64(collUsed)
					}
					metrics.HeapAfterLastGC += int64(collUsed)
				}
			}
		}

		// Memory Threshold Metrics
		if exceeded, ok := pool["UsageThresholdExceeded"].(bool); ok {
			if isYoungGenPool(poolName) {
				metrics.YoungThresholdExceeded = exceeded
			} else if isOldGenPool(poolName) {
				metrics.OldThresholdExceeded = exceeded
			} else if strings.Contains(strings.ToLower(poolName), "heap") {
				metrics.HeapThresholdExceeded = exceeded
			} else {
				metrics.NonHeapThresholdExceeded = exceeded
			}
		}
		if count, ok := pool["UsageThresholdCount"].(float64); ok {
			totalThresholdCount += int64(count)
		}

		// Memory Manager Names
		if managers, ok := pool["MemoryManagerNames"].([]interface{}); ok {
			var managerNames []string
			for _, mgr := range managers {
				if name, ok := mgr.(string); ok {
					managerNames = append(managerNames, name)
				}
			}
			if poolType == "HEAP" && isYoungGenPool(poolName) {
				metrics.YoungGenManagers = managerNames
			} else if poolType == "HEAP" && isOldGenPool(poolName) {
				metrics.OldGenManagers = managerNames
			}
		}

		// Pool validity
		if valid, ok := pool["Valid"].(bool); ok && !valid {
			metrics.InvalidMemoryPools = append(metrics.InvalidMemoryPools, poolName)
		}
	}
	metrics.MemoryThresholdCount = totalThresholdCount

	// 3. Buffer Pools - Get ALL available attributes
	bufferPools, err := client.QueryMBeanPattern("java.nio:type=BufferPool,name=*")
	if err == nil {
		for _, pool := range bufferPools {
			poolName := jc.extractPoolName(pool)

			if count, ok := pool["Count"].(float64); ok {
				if used, ok := pool["MemoryUsed"].(float64); ok {
					if capacity, ok := pool["TotalCapacity"].(float64); ok {
						if strings.Contains(strings.ToLower(poolName), "direct") {
							metrics.DirectBufferCount = int64(count)
							metrics.DirectBufferUsed = int64(used)
							metrics.DirectBufferCapacity = int64(capacity)
						} else if strings.Contains(strings.ToLower(poolName), "mapped") {
							metrics.MappedBufferCount = int64(count)
							metrics.MappedBufferUsed = int64(used)
							metrics.MappedBufferCapacity = int64(capacity)
						}
					}
				}
			}
		}
	}

	return nil
}

// Helper method for allocation rate calculation
func (jc *JMXPoller) calculateAllocationRate(edenUsed int64, metrics *JVMSnapshot) {
	if jc.lastEdenUsed > 0 && jc.lastAllocationTime.After(time.Time{}) {
		timeDiff := metrics.Timestamp.Sub(jc.lastAllocationTime).Seconds()
		if timeDiff > 0 {
			var allocated int64
			if edenUsed >= jc.lastEdenUsed {
				allocated = edenUsed - jc.lastEdenUsed
			} else {
				allocated = jc.lastEdenUsed + edenUsed
			}

			if allocated > 0 {
				metrics.AllocationRate = float64(allocated) / timeDiff
				metrics.AllocationRateMBSec = metrics.AllocationRate / (1024 * 1024)
				metrics.EdenAllocationRate = metrics.AllocationRate
			}
		}
	}
	jc.lastEdenUsed = edenUsed
	jc.lastAllocationTime = metrics.Timestamp
}

// Extract pool name from various possible sources in the pool data
func (jc *JMXPoller) extractPoolName(pool map[string]any) string {
	// Try multiple possible fields for pool name
	nameFields := []string{"Name", "name"}
	for _, field := range nameFields {
		if name, ok := pool[field].(string); ok && name != "" {
			return name
		}
	}

	// Extract from ObjectName
	objectNameFields := []string{"ObjectName", "objectName"}
	for _, field := range objectNameFields {
		if objectName, ok := pool[field].(string); ok {
			return extractPoolName(objectName)
		}
	}

	return ""
}

// Existing helper functions (unchanged)
func extractPoolName(objectName string) string {
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

func isYoungGenPool(poolName string) bool {
	lowerName := strings.ToLower(poolName)
	return strings.Contains(lowerName, "eden") ||
		strings.Contains(lowerName, "survivor") ||
		strings.Contains(lowerName, "young") ||
		strings.Contains(lowerName, "nursery") ||
		strings.Contains(lowerName, "s0") ||
		strings.Contains(lowerName, "s1") ||
		strings.Contains(lowerName, "g1 eden") ||
		strings.Contains(lowerName, "g1 survivor")
}

func isOldGenPool(poolName string) bool {
	lowerName := strings.ToLower(poolName)
	return strings.Contains(lowerName, "old") ||
		strings.Contains(lowerName, "tenured") ||
		strings.Contains(lowerName, "cms") ||
		strings.Contains(lowerName, "g1 old") ||
		strings.Contains(lowerName, "g1 old gen")
}

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
