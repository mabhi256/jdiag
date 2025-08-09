package jmx

import "fmt"

// ===== THREADING METRICS =====
func (jc *JMXPoller) collectThreadingMetrics(metrics *MBeanSnapshot) error {
	client := jc.getEffectiveClient()

	threading, err := client.QueryMBean("java.lang:type=Threading")
	if err != nil {
		return fmt.Errorf("failed to query thread metrics: %w", err)
	}

	// Basic thread counts
	if count, ok := threading["ThreadCount"].(float64); ok {
		metrics.Threading.Count = int64(count)
	}
	if peak, ok := threading["PeakThreadCount"].(float64); ok {
		metrics.Threading.PeakCount = int64(peak)
	}
	if daemon, ok := threading["DaemonThreadCount"].(float64); ok {
		metrics.Threading.DaemonCount = int64(daemon)
	}
	if totalStarted, ok := threading["TotalStartedThreadCount"].(float64); ok {
		metrics.Threading.TotalStartedCount = int64(totalStarted)
	}

	// Current thread metrics
	if cpuTime, ok := threading["CurrentThreadCpuTime"].(float64); ok {
		metrics.Threading.CurrentCPUTime = int64(cpuTime)
	}
	if userTime, ok := threading["CurrentThreadUserTime"].(float64); ok {
		metrics.Threading.CurrentUserTime = int64(userTime)
	}
	if allocatedBytes, ok := threading["CurrentThreadAllocatedBytes"].(float64); ok {
		metrics.Threading.CurrentAllocatedBytes = int64(allocatedBytes)
	}

	// Monitoring capabilities
	if cpuTimeSupported, ok := threading["ThreadCpuTimeSupported"].(bool); ok {
		metrics.Threading.CpuTimeSupported = cpuTimeSupported
	}
	if cpuTimeEnabled, ok := threading["ThreadCpuTimeEnabled"].(bool); ok {
		metrics.Threading.CpuTimeEnabled = cpuTimeEnabled
	}
	if allocMemSupported, ok := threading["ThreadAllocatedMemorySupported"].(bool); ok {
		metrics.Threading.AllocatedMemorySupported = allocMemSupported
	}
	if allocMemEnabled, ok := threading["ThreadAllocatedMemoryEnabled"].(bool); ok {
		metrics.Threading.AllocatedMemoryEnabled = allocMemEnabled
	}
	if contentionSupported, ok := threading["ThreadContentionMonitoringSupported"].(bool); ok {
		metrics.Threading.ContentionMonitoringSupported = contentionSupported
	}
	if contentionEnabled, ok := threading["ThreadContentionMonitoringEnabled"].(bool); ok {
		metrics.Threading.ContentionMonitoringEnabled = contentionEnabled
	}
	if monitorUsageSupported, ok := threading["ObjectMonitorUsageSupported"].(bool); ok {
		metrics.Threading.ObjectMonitorUsageSupported = monitorUsageSupported
	}
	if synchronizerSupported, ok := threading["SynchronizerUsageSupported"].(bool); ok {
		metrics.Threading.SynchronizerUsageSupported = synchronizerSupported
	}

	// Get all thread IDs
	if threadIds, ok := threading["AllThreadIds"].([]interface{}); ok && len(threadIds) > 0 {
		var threadIdList []int64
		for _, tid := range threadIds {
			if id, ok := tid.(float64); ok {
				threadIdList = append(threadIdList, int64(id))
			}
		}
		metrics.Threading.AllThreadIds = threadIdList
	}

	return nil
}

// ===== CLASS LOADING METRICS =====
func (jc *JMXPoller) collectClassLoadingMetrics(metrics *MBeanSnapshot) error {
	client := jc.getEffectiveClient()

	classLoading, err := client.QueryMBean("java.lang:type=ClassLoading")
	if err != nil {
		return fmt.Errorf("failed to query class loading metrics: %w", err)
	}

	if loaded, ok := classLoading["LoadedClassCount"].(float64); ok {
		metrics.ClassLoading.LoadedClassCount = int64(loaded)
	}
	if totalLoaded, ok := classLoading["TotalLoadedClassCount"].(float64); ok {
		metrics.ClassLoading.TotalLoadedClassCount = int64(totalLoaded)
	}
	if unloaded, ok := classLoading["UnloadedClassCount"].(float64); ok {
		metrics.ClassLoading.UnloadedClassCount = int64(unloaded)
	}
	if verbose, ok := classLoading["Verbose"].(bool); ok {
		metrics.ClassLoading.VerboseLogging = verbose
	}

	return nil
}
