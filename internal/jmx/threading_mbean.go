package jmx

import (
	"fmt"
)

// ===== THREAD METRICS =====
// Fetch ALL available thread-related attributes
func (jc *JMXPoller) collectThreadMetrics(metrics *JVMSnapshot) error {
	client := jc.getEffectiveClient()

	// 1. Threading - Get ALL available attributes
	threading, err := client.QueryMBean("java.lang:type=Threading")
	if err != nil {
		return fmt.Errorf("failed to query thread metrics: %w", err)
	}

	// Basic thread counts
	if count, ok := threading["ThreadCount"].(float64); ok {
		metrics.ThreadCount = int64(count)
	}
	if peak, ok := threading["PeakThreadCount"].(float64); ok {
		metrics.PeakThreadCount = int64(peak)
	}
	if daemon, ok := threading["DaemonThreadCount"].(float64); ok {
		metrics.DaemonThreadCount = int64(daemon)
	}
	if totalStarted, ok := threading["TotalStartedThreadCount"].(float64); ok {
		metrics.TotalStartedThreadCount = int64(totalStarted)
	}

	// Thread performance metrics
	if cpuTime, ok := threading["CurrentThreadCpuTime"].(float64); ok {
		metrics.ThreadCPUTime = int64(cpuTime)
	}
	if userTime, ok := threading["CurrentThreadUserTime"].(float64); ok {
		metrics.ThreadUserTime = int64(userTime)
	}
	if allocatedBytes, ok := threading["CurrentThreadAllocatedBytes"].(float64); ok {
		metrics.CurrentThreadAllocatedBytes = int64(allocatedBytes)
	}

	// Thread monitoring capabilities
	if cpuTimeSupported, ok := threading["ThreadCpuTimeSupported"].(bool); ok {
		metrics.ThreadCpuTimeSupported = cpuTimeSupported
	}
	if cpuTimeEnabled, ok := threading["ThreadCpuTimeEnabled"].(bool); ok {
		metrics.ThreadCpuTimeEnabled = cpuTimeEnabled
	}
	if allocMemSupported, ok := threading["ThreadAllocatedMemorySupported"].(bool); ok {
		metrics.ThreadAllocatedMemorySupported = allocMemSupported
	}
	if allocMemEnabled, ok := threading["ThreadAllocatedMemoryEnabled"].(bool); ok {
		metrics.ThreadAllocatedMemoryEnabled = allocMemEnabled
	}
	if contentionSupported, ok := threading["ThreadContentionMonitoringSupported"].(bool); ok {
		metrics.ThreadContentionMonitoringSupported = contentionSupported
	}
	if contentionEnabled, ok := threading["ThreadContentionMonitoringEnabled"].(bool); ok {
		metrics.ThreadContentionMonitoringEnabled = contentionEnabled
	}
	if monitorUsageSupported, ok := threading["ObjectMonitorUsageSupported"].(bool); ok {
		metrics.ObjectMonitorUsageSupported = monitorUsageSupported
	}
	if synchronizerSupported, ok := threading["SynchronizerUsageSupported"].(bool); ok {
		metrics.SynchronizerUsageSupported = synchronizerSupported
	}

	// Get all thread IDs for detailed analysis
	if threadIds, ok := threading["AllThreadIds"].([]interface{}); ok && len(threadIds) > 0 {
		var threadIdList []int64
		for _, tid := range threadIds {
			if id, ok := tid.(float64); ok {
				threadIdList = append(threadIdList, int64(id))
			}
		}
		metrics.AllThreadIds = threadIdList
	}

	// Deadlock detection
	jc.checkForDeadlocks(metrics)

	// 2. Class Loading Metrics - Get ALL available attributes
	classLoading, err := client.QueryMBean("java.lang:type=ClassLoading")
	if err != nil {
		return fmt.Errorf("failed to query class loading metrics: %w", err)
	}

	if loaded, ok := classLoading["LoadedClassCount"].(float64); ok {
		metrics.LoadedClassCount = int64(loaded)
	}
	if totalLoaded, ok := classLoading["TotalLoadedClassCount"].(float64); ok {
		metrics.TotalLoadedClassCount = int64(totalLoaded)
	}
	if unloaded, ok := classLoading["UnloadedClassCount"].(float64); ok {
		metrics.UnloadedClassCount = int64(unloaded)
	}
	if verbose, ok := classLoading["Verbose"].(bool); ok {
		metrics.ClassLoadingVerboseLogging = verbose
	}

	return nil
}

// Helper method to check for deadlocks
func (jc *JMXPoller) checkForDeadlocks(metrics *JVMSnapshot) {
	client := jc.getEffectiveClient()

	// Try to get deadlock information
	threadMgmt, err := client.QueryMBean("java.lang:type=Threading")
	if err != nil {
		return
	}

	jc.extractDeadlockInfo(threadMgmt, metrics)
}

func (jc *JMXPoller) extractDeadlockInfo(threadMgmt map[string]any, metrics *JVMSnapshot) {
	// Check for deadlocked threads
	if deadlocks, ok := threadMgmt["findDeadlockedThreads"].([]interface{}); ok && len(deadlocks) > 0 {
		for _, threadID := range deadlocks {
			if id, ok := threadID.(float64); ok {
				threadName := fmt.Sprintf("Thread-%d", int64(id))
				metrics.DeadlockedThreads = append(metrics.DeadlockedThreads, threadName)
			}
		}
	}

	// Check for monitor deadlocked threads
	if monitorDeadlocks, ok := threadMgmt["findMonitorDeadlockedThreads"].([]interface{}); ok && len(monitorDeadlocks) > 0 {
		for _, threadID := range monitorDeadlocks {
			if id, ok := threadID.(float64); ok {
				threadName := fmt.Sprintf("MonitorThread-%d", int64(id))
				metrics.MonitorDeadlockedThreads = append(metrics.MonitorDeadlockedThreads, threadName)
			}
		}
	}
}
