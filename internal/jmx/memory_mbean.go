package jmx

import (
	"fmt"
	"strings"
)

// ===== MEMORY METRICS =====
func (jc *JMXPoller) collectMemoryMetrics(metrics *MBeanSnapshot) error {
	client := jc.getEffectiveClient()

	// 1. Basic Memory - Get heap and non-heap totals
	heapMemory, err := client.QueryMBean("java.lang:type=Memory")
	if err != nil {
		return fmt.Errorf("failed to query heap memory: %w", err)
	}

	// Extract heap usage
	if heapUsage, ok := heapMemory["HeapMemoryUsage"].(map[string]any); ok {
		metrics.Memory.Heap = extractMemoryUsage(heapUsage)
	}

	// Extract non-heap usage
	if nonHeapUsage, ok := heapMemory["NonHeapMemoryUsage"].(map[string]any); ok {
		metrics.Memory.NonHeap = extractMemoryUsage(nonHeapUsage)
	}

	// Extract additional memory metrics
	if pendingFinal, ok := heapMemory["ObjectPendingFinalizationCount"].(float64); ok {
		metrics.Memory.ObjectPendingFinalizationCount = int64(pendingFinal)
	}
	if verbose, ok := heapMemory["Verbose"].(bool); ok {
		metrics.Memory.VerboseLogging = verbose
	}

	// 2. Memory Pools
	if err := jc.collectMemoryPools(metrics); err != nil {
		return err
	}

	// 3. Buffer Pools
	if err := jc.collectBufferPools(metrics); err != nil {
		return err
	}

	return nil
}

// Collect memory pool information
func (jc *JMXPoller) collectMemoryPools(metrics *MBeanSnapshot) error {
	client := jc.getEffectiveClient()

	pools, err := client.QueryMBeanPattern("java.lang:type=MemoryPool,name=*")
	if err != nil {
		return fmt.Errorf("failed to query memory pools: %w", err)
	}

	for _, pool := range pools {
		// poolType, typeOk := pool["Type"].(string)
		poolName, nameOk := pool["Name"].(string)

		if !nameOk || poolName == "" {
			continue
		}

		// Extract current usage
		var usage MemoryUsage
		if usageData, ok := pool["Usage"].(map[string]any); ok {
			usage = extractMemoryUsage(usageData)
		} else {
			continue
		}

		// Extract peak usage
		var peakUsage MemoryUsage
		if peakData, ok := pool["PeakUsage"].(map[string]any); ok {
			peakUsage = extractMemoryUsage(peakData)
		}

		// Extract collection usage
		var collectionUsage MemoryUsage
		var collectionThreshold ThresholdInfo
		if collData, ok := pool["CollectionUsage"].(map[string]any); ok && collData != nil {
			collectionUsage = extractMemoryUsage(collData)
		}
		collectionThreshold = extractThresholdInfo(pool, "CollectionUsage")

		// Extract usage threshold
		threshold := extractThresholdInfo(pool, "Usage")

		// Extract managers
		var managers []string
		if mgrs, ok := pool["MemoryManagerNames"].([]interface{}); ok {
			for _, mgr := range mgrs {
				if name, ok := mgr.(string); ok {
					managers = append(managers, name)
				}
			}
		}

		// Check validity
		valid := true
		if v, ok := pool["Valid"].(bool); ok {
			valid = v
		}

		// Create memory pool structure
		memPool := MemoryPool{
			Usage:               usage,
			PeakUsage:           peakUsage,
			Threshold:           threshold,
			CollectionUsage:     collectionUsage,
			CollectionThreshold: collectionThreshold,
			Valid:               valid,
			Managers:            managers,
		}

		// Store in appropriate location and aggregate
		lowerPoolName := strings.ToLower(poolName)
		switch {
		case strings.Contains(lowerPoolName, "metaspace"):
			metrics.Memory.Metaspace = memPool
		case strings.Contains(lowerPoolName, "compressed class space"):
			metrics.Memory.CompressedClassSpace = memPool
		case strings.Contains(lowerPoolName, "g1 eden"):
			metrics.Memory.G1Eden = memPool
		case strings.Contains(lowerPoolName, "g1 survivor"):
			metrics.Memory.G1Survivor = memPool
		case strings.Contains(lowerPoolName, "g1 old"):
			metrics.Memory.G1OldGen = memPool
		case strings.Contains(lowerPoolName, "code"):
		}
	}

	return nil
}

func (jc *JMXPoller) collectBufferPools(metrics *MBeanSnapshot) error {
	client := jc.getEffectiveClient()

	bufferPools, err := client.QueryMBeanPattern("java.nio:type=BufferPool,name=*")
	if err != nil {
		return nil // Buffer pools are optional
	}

	for _, pool := range bufferPools {
		poolName, _ := pool["Name"].(string)

		var bufferPool BufferPool
		if count, ok := pool["Count"].(float64); ok {
			bufferPool.Count = int64(count)
		}
		if used, ok := pool["MemoryUsed"].(float64); ok {
			bufferPool.Used = int64(used)
		}
		if capacity, ok := pool["TotalCapacity"].(float64); ok {
			bufferPool.Capacity = int64(capacity)
		}

		lowerPoolName := strings.ToLower(poolName)
		if strings.Contains(lowerPoolName, "direct") {
			metrics.Memory.DirectBuffers = bufferPool
		} else if strings.Contains(lowerPoolName, "mapped") {
			metrics.Memory.MappedBuffers = bufferPool
		}
	}

	return nil
}

func extractMemoryUsage(data map[string]any) MemoryUsage {
	var usage MemoryUsage
	if used, ok := data["used"].(float64); ok {
		usage.Used = int64(used)
	}
	if committed, ok := data["committed"].(float64); ok {
		usage.Committed = int64(committed)
	}
	if max, ok := data["max"].(float64); ok {
		usage.Max = int64(max)
	}
	if init, ok := data["init"].(float64); ok {
		usage.Init = int64(init)
	}
	return usage
}

func extractThresholdInfo(pool map[string]any, prefix string) ThresholdInfo {
	var threshold ThresholdInfo

	if supported, ok := pool[prefix+"ThresholdSupported"].(bool); ok {
		threshold.Supported = supported
	}
	if thresh, ok := pool[prefix+"Threshold"].(float64); ok {
		threshold.Threshold = int64(thresh)
	}
	if exceeded, ok := pool[prefix+"ThresholdExceeded"].(bool); ok {
		threshold.Exceeded = exceeded
	}
	if count, ok := pool[prefix+"ThresholdCount"].(float64); ok {
		threshold.Count = int64(count)
	}

	return threshold
}
