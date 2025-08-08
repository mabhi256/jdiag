package jmx

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

type Config struct {
	// Target configuration
	PID  int    // Process ID for local monitoring
	Host string // Host for remote monitoring
	Port int    // Port for remote monitoring

	Interval int // Update interval in ms

	// Debug configuration
	Debug        bool   // Enable debug mode
	DebugLogFile string // Path to debug log file
}

func (c *Config) GetInterval() time.Duration {
	return time.Duration(c.Interval) * time.Millisecond
}

func (c *Config) String() string {
	if c.PID != 0 {
		return fmt.Sprintf("PID %d", c.PID)
	}

	if c.Host != "" {
		if c.Port != 0 {
			return fmt.Sprintf("%s:%d", c.Host, c.Port)
		}
		return c.Host
	}

	return "No target specified"
}

type JMXCollector struct {
	config             *Config
	client             *JMXClient
	debugClient        *DebugJMXClient // Wrapper for debug logging
	metrics            *JVMSnapshot
	mu                 sync.RWMutex
	running            bool
	stopChan           chan struct{}
	errChan            chan error
	debugFile          *os.File
	lastEdenUsed       int64
	lastAllocationTime time.Time
}

// DebugJMXClient wraps the original JMXClient to add debug logging
type DebugJMXClient struct {
	originalClient *JMXClient
	debugFile      *os.File
	enabled        bool
}

type DebugLogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	MBeanName string    `json:"mbean_name"`
	QueryType string    `json:"query_type"`
	RawData   any       `json:"raw_data"`
	Error     string    `json:"error,omitempty"`
}

func NewJMXCollector(config *Config) *JMXCollector {
	collector := &JMXCollector{
		config: config,
		metrics: &JVMSnapshot{
			Timestamp: time.Now(),
			Connected: false,
		},
		stopChan: make(chan struct{}),
		errChan:  make(chan error, 1),
	}

	// Initialize debug logging if enabled
	if config.Debug {
		if err := collector.initDebugLogging(); err != nil {
			fmt.Printf("Warning: Failed to initialize debug logging: %v\n", err)
		}
	}

	return collector
}

func (jc *JMXCollector) initDebugLogging() error {
	if jc.config.DebugLogFile == "" {
		timestamp := time.Now().Format("20060102_150405")
		jc.config.DebugLogFile = fmt.Sprintf("jmx_debug_%s.log", timestamp)
	}

	file, err := os.OpenFile(jc.config.DebugLogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open debug log file: %w", err)
	}

	jc.debugFile = file

	// Write debug session header
	header := fmt.Sprintf("=== JMX Debug Session Started at %s ===\n", time.Now().Format(time.RFC3339))
	if _, err := jc.debugFile.WriteString(header); err != nil {
		return fmt.Errorf("failed to write debug header: %w", err)
	}

	return nil
}

// Start metric collection
func (jc *JMXCollector) Start() error {
	if jc.running {
		return fmt.Errorf("collector already running")
	}

	// Create original JMX client
	var err error
	if jc.config.PID != 0 {
		jc.client, err = NewJMXClient(jc.config.PID, "")
	} else {
		// Standard JMX service URL format
		host := jc.config.Host
		port := jc.config.Port
		url := fmt.Sprintf("service:jmx:rmi:///jndi/rmi://%s:%d/jmxrmi", host, port)
		jc.client, err = NewJMXClient(0, url)
	}

	if err != nil {
		return fmt.Errorf("failed to create JMX client: %w", err)
	}

	// Create debug wrapper if debug mode is enabled
	if jc.config.Debug && jc.debugFile != nil {
		jc.debugClient = &DebugJMXClient{
			originalClient: jc.client,
			debugFile:      jc.debugFile,
			enabled:        true,
		}
	}

	jc.running = true

	// Start collection goroutine
	go jc.collectLoop()

	return nil
}

// QueryMBean with automatic debug logging
func (dc *DebugJMXClient) QueryMBean(objectName string) (map[string]any, error) {
	data, err := dc.originalClient.QueryMBean(objectName)

	if dc.enabled {
		dc.logDebugData(objectName, "single", data, err)
	}

	return data, err
}

// QueryMBeanPattern with automatic debug logging
func (dc *DebugJMXClient) QueryMBeanPattern(pattern string) ([]map[string]any, error) {
	data, err := dc.originalClient.QueryMBeanPattern(pattern)

	if dc.enabled {
		// Convert slice to interface{} for logging
		dc.logDebugData(pattern, "pattern", data, err)
	}

	return data, err
}

// TestConnection with debug logging
func (dc *DebugJMXClient) TestConnection() error {
	err := dc.originalClient.TestConnection()

	if dc.enabled {
		dc.logDebugData("java.lang:type=Runtime", "test_connection", map[string]interface{}{"connection_test": true}, err)
	}

	return err
}

// Close method for debug client
func (dc *DebugJMXClient) Close() error {
	return dc.originalClient.Close()
}

func (dc *DebugJMXClient) logDebugData(mbeanName, queryType string, data interface{}, err error) {
	if dc.debugFile == nil {
		return
	}

	entry := DebugLogEntry{
		Timestamp: time.Now(),
		MBeanName: mbeanName,
		QueryType: queryType,
		RawData:   data,
	}

	if err != nil {
		entry.Error = err.Error()
	}

	jsonData, marshalErr := json.MarshalIndent(entry, "", "  ")
	if marshalErr != nil {
		fallbackLog := fmt.Sprintf("[%s] ERROR: Failed to marshal debug data for %s: %v\n",
			entry.Timestamp.Format(time.RFC3339), mbeanName, marshalErr)
		dc.debugFile.WriteString(fallbackLog)
		return
	}

	dc.debugFile.WriteString(string(jsonData) + "\n")
	dc.debugFile.Sync()
}

// getEffectiveClient returns the debug client if available, otherwise the original client
func (jc *JMXCollector) getEffectiveClient() interface {
	QueryMBean(string) (map[string]any, error)
	QueryMBeanPattern(string) ([]map[string]any, error)
	TestConnection() error
	Close() error
} {
	if jc.debugClient != nil {
		return jc.debugClient
	}
	return jc.client
}

// Stop metric collection
func (jc *JMXCollector) Stop() {
	if !jc.running {
		return
	}

	jc.running = false
	close(jc.stopChan)

	if jc.client != nil {
		jc.client.Close()
	}

	if jc.debugFile != nil {
		footer := fmt.Sprintf("=== JMX Debug Session Ended at %s ===\n", time.Now().Format(time.RFC3339))
		jc.debugFile.WriteString(footer)
		jc.debugFile.Close()
		jc.debugFile = nil
	}
}

// Get the current snapshot
func (jc *JMXCollector) GetMetrics() *JVMSnapshot {
	jc.mu.RLock()
	defer jc.mu.RUnlock()

	metricsCopy := *jc.metrics
	return &metricsCopy
}

// Metric collection loop
func (jc *JMXCollector) collectLoop() {
	ticker := time.NewTicker(jc.config.GetInterval())
	defer ticker.Stop()

	for {
		select {
		case <-jc.stopChan:
			return
		case <-ticker.C:
			jc.collectMetrics()
		}
	}
}

// Collect a single set of metrics
// Complete Consolidated JMX Collector - Fetches ALL available attributes

// Collect a single set of metrics
func (jc *JMXCollector) collectMetrics() {
	metrics := &JVMSnapshot{
		Timestamp: time.Now(),
		Connected: false,
	}

	collectors := []func(*JVMSnapshot) error{
		jc.collectMemoryMetrics,
		jc.collectGCMetrics,
		jc.collectThreadMetrics,
		jc.collectSystemMetrics,
	}

	for _, collect := range collectors {
		if err := collect(metrics); err != nil {
			metrics.Error = err
			jc.updateMetrics(metrics)
			return
		}
	}

	metrics.Connected = true
	jc.updateMetrics(metrics)
}

// ===== MEMORY METRICS =====
// Fetch ALL available attributes, then extract what we need
func (jc *JMXCollector) collectMemoryMetrics(metrics *JVMSnapshot) error {
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
func (jc *JMXCollector) calculateAllocationRate(edenUsed int64, metrics *JVMSnapshot) {
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

// ===== GC METRICS =====
// Fetch ALL available attributes for comprehensive GC analysis
func (jc *JMXCollector) collectGCMetrics(metrics *JVMSnapshot) error {
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

// Helper method for G1 phase timings
func (jc *JMXCollector) extractG1PhaseTimings(phases map[string]any, metrics *JVMSnapshot) {
	phaseMap := map[string]*int64{
		"Evacuation Pause": &metrics.G1EvacuationPauseTime,
		"Root Scanning":    &metrics.G1RootScanTime,
		"Update RS":        &metrics.G1UpdateRSTime,
		"Scan RS":          &metrics.G1ScanRSTime,
		"Object Copy":      &metrics.G1ObjectCopyTime,
		"Termination":      &metrics.G1TerminationTime,
	}

	for phaseName, metricPtr := range phaseMap {
		if phaseTime, ok := phases[phaseName].(float64); ok {
			*metricPtr = int64(phaseTime)
		}
	}
}

// Helper method to extract G1 region size
func (jc *JMXCollector) extractG1RegionSize(metrics *JVMSnapshot) {
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

// ===== THREAD METRICS =====
// Fetch ALL available thread-related attributes
func (jc *JMXCollector) collectThreadMetrics(metrics *JVMSnapshot) error {
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
func (jc *JMXCollector) checkForDeadlocks(metrics *JVMSnapshot) {
	client := jc.getEffectiveClient()

	// Try to get deadlock information
	threadMgmt, err := client.QueryMBean("java.lang:type=Threading")
	if err != nil {
		return
	}

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

// ===== SYSTEM METRICS =====
// Fetch ALL available system-related attributes
func (jc *JMXCollector) collectSystemMetrics(metrics *JVMSnapshot) error {
	client := jc.getEffectiveClient()

	// 1. Operating System Metrics - Get ALL available attributes
	osInfo, err := client.QueryMBean("java.lang:type=OperatingSystem")
	if err != nil {
		return fmt.Errorf("failed to query OS metrics: %w", err)
	}

	// CPU metrics
	if processCpu, ok := osInfo["ProcessCpuLoad"].(float64); ok && processCpu >= 0 {
		metrics.ProcessCpuLoad = processCpu
	}
	if systemCpu, ok := osInfo["SystemCpuLoad"].(float64); ok && systemCpu >= 0 {
		metrics.SystemCpuLoad = systemCpu
	}
	if cpuLoad, ok := osInfo["CpuLoad"].(float64); ok && cpuLoad >= 0 {
		metrics.CpuLoad = cpuLoad
	}
	if processCpuTime, ok := osInfo["ProcessCpuTime"].(float64); ok {
		metrics.ProcessCpuTime = int64(processCpuTime)
	}
	if processors, ok := osInfo["AvailableProcessors"].(float64); ok {
		metrics.AvailableProcessors = int64(processors)
	}
	if loadAvg, ok := osInfo["SystemLoadAverage"].(float64); ok && loadAvg >= 0 {
		metrics.SystemLoadAverage = loadAvg
	}

	// Memory metrics
	if totalMem, ok := osInfo["TotalPhysicalMemorySize"].(float64); ok {
		metrics.TotalSystemMemory = int64(totalMem)
	}
	if freeMem, ok := osInfo["FreePhysicalMemorySize"].(float64); ok {
		metrics.FreeSystemMemory = int64(freeMem)
	}
	if totalMemSize, ok := osInfo["TotalMemorySize"].(float64); ok {
		metrics.TotalMemorySize = int64(totalMemSize)
	}
	if freeMemSize, ok := osInfo["FreeMemorySize"].(float64); ok {
		metrics.FreeMemorySize = int64(freeMemSize)
	}
	if virtualMem, ok := osInfo["CommittedVirtualMemorySize"].(float64); ok {
		metrics.ProcessVirtualMemorySize = int64(virtualMem)
	}

	// Swap metrics
	if totalSwap, ok := osInfo["TotalSwapSpaceSize"].(float64); ok {
		metrics.TotalSwapSize = int64(totalSwap)
	}
	if freeSwap, ok := osInfo["FreeSwapSpaceSize"].(float64); ok {
		metrics.FreeSwapSize = int64(freeSwap)
	}

	// File descriptor metrics (Unix/Linux only)
	if openFDs, ok := osInfo["OpenFileDescriptorCount"].(float64); ok {
		metrics.OpenFileDescriptorCount = int64(openFDs)
	}
	if maxFDs, ok := osInfo["MaxFileDescriptorCount"].(float64); ok {
		metrics.MaxFileDescriptorCount = int64(maxFDs)
	}

	// OS details
	if osName, ok := osInfo["Name"].(string); ok {
		metrics.OSName = osName
	}
	if osVersion, ok := osInfo["Version"].(string); ok {
		metrics.OSVersion = osVersion
	}
	if osArch, ok := osInfo["Arch"].(string); ok {
		metrics.OSArch = osArch
	}

	// 2. Try Sun/Oracle-specific OS MBean for additional process memory
	sunOSInfo, err := client.QueryMBean("com.sun.management:type=OperatingSystem")
	if err == nil {
		if privateBytes, ok := sunOSInfo["ProcessPrivateBytes"].(float64); ok {
			metrics.ProcessPrivateMemory = int64(privateBytes)
		}
		if sharedBytes, ok := sunOSInfo["ProcessSharedBytes"].(float64); ok {
			metrics.ProcessSharedMemory = int64(sharedBytes)
		}
		if metrics.ProcessPrivateMemory > 0 || metrics.ProcessSharedMemory > 0 {
			metrics.ProcessResidentSetSize = metrics.ProcessPrivateMemory + metrics.ProcessSharedMemory
		}
	}

	// 3. Runtime and Configuration Metrics - Get ALL available attributes
	runtime, err := client.QueryMBean("java.lang:type=Runtime")
	if err != nil {
		return fmt.Errorf("failed to query runtime metrics: %w", err)
	}

	// Basic JVM info
	if name, ok := runtime["VmName"].(string); ok {
		metrics.JVMName = name
	}
	if vendor, ok := runtime["VmVendor"].(string); ok {
		metrics.JVMVendor = vendor
	}
	if version, ok := runtime["VmVersion"].(string); ok {
		metrics.JVMVersion = version
	}
	if startTime, ok := runtime["StartTime"].(float64); ok {
		metrics.JVMStartTime = time.Unix(int64(startTime/1000), 0)
	}
	if uptime, ok := runtime["Uptime"].(float64); ok {
		metrics.JVMUptime = time.Duration(uptime) * time.Millisecond
	}

	// Process info
	if pid, ok := runtime["Pid"].(float64); ok {
		metrics.ProcessID = int64(pid)
	}
	if processName, ok := runtime["Name"].(string); ok {
		metrics.ProcessName = processName
	}

	// JVM specification info
	if specName, ok := runtime["SpecName"].(string); ok {
		metrics.JVMSpecName = specName
	}
	if specVendor, ok := runtime["SpecVendor"].(string); ok {
		metrics.JVMSpecVendor = specVendor
	}
	if specVersion, ok := runtime["SpecVersion"].(string); ok {
		metrics.JVMSpecVersion = specVersion
	}
	if mgmtSpecVersion, ok := runtime["ManagementSpecVersion"].(string); ok {
		metrics.ManagementSpecVersion = mgmtSpecVersion
	}

	// Paths and configuration
	if classPath, ok := runtime["ClassPath"].(string); ok {
		metrics.JavaClassPath = classPath
	}
	if libPath, ok := runtime["LibraryPath"].(string); ok {
		metrics.JavaLibraryPath = libPath
	}
	if bootClassPath, ok := runtime["BootClassPath"].(string); ok {
		metrics.BootClassPath = bootClassPath
	}
	if bootClassPathSupported, ok := runtime["BootClassPathSupported"].(bool); ok {
		metrics.BootClassPathSupported = bootClassPathSupported
	}

	// JVM Arguments
	if args, ok := runtime["InputArguments"].([]interface{}); ok {
		for _, arg := range args {
			if argStr, ok := arg.(string); ok {
				metrics.JVMArguments = append(metrics.JVMArguments, argStr)
			}
		}
	}

	// System Properties
	if props, ok := runtime["SystemProperties"].(map[string]any); ok {
		metrics.SystemProperties = make(map[string]string)
		metrics.GCSettings = make(map[string]string)

		for key, value := range props {
			if valueStr, ok := value.(string); ok {
				metrics.SystemProperties[key] = valueStr

				// Extract commonly used properties
				switch key {
				case "java.version":
					metrics.JavaVersion = valueStr
				case "java.vendor":
					metrics.JavaVendor = valueStr
				case "java.home":
					metrics.JavaHome = valueStr
				case "user.name":
					metrics.UserName = valueStr
				case "user.home":
					metrics.UserHome = valueStr
				case "os.name":
					if metrics.OSName == "" { // Prefer OS MBean value
						metrics.OSName = valueStr
					}
				case "os.arch":
					if metrics.OSArch == "" { // Prefer OS MBean value
						metrics.OSArch = valueStr
					}
				case "os.version":
					if metrics.OSVersion == "" { // Prefer OS MBean value
						metrics.OSVersion = valueStr
					}
				}

				// Extract GC-related properties
				if strings.HasPrefix(key, "gc.") ||
					strings.Contains(key, "GC") ||
					strings.Contains(key, "UseG1GC") ||
					strings.Contains(key, "UseParallelGC") ||
					strings.Contains(key, "UseConcMarkSweepGC") ||
					strings.Contains(key, "G1HeapRegionSize") {
					metrics.GCSettings[key] = valueStr
				}
			}
		}
	}

	// 4. Compilation Metrics - Get ALL available attributes
	compilation, err := client.QueryMBean("java.lang:type=Compilation")
	if err == nil {
		if totalTime, ok := compilation["TotalCompilationTime"].(float64); ok {
			metrics.TotalCompilationTime = int64(totalTime)
		}
		if name, ok := compilation["Name"].(string); ok {
			metrics.CompilerName = name
		}
		if supported, ok := compilation["CompilationTimeMonitoringSupported"].(bool); ok {
			metrics.CompilationTimeMonitoringSupported = supported
		}
	}

	// 5. Safepoint Metrics (HotSpot-specific) - Get ALL available attributes
	safepointInfo, err := client.QueryMBean("sun.management:type=HotspotRuntime")
	if err == nil {
		if count, ok := safepointInfo["SafepointCount"].(float64); ok {
			metrics.SafepointCount = int64(count)
		}
		if totalTime, ok := safepointInfo["TotalSafepointTime"].(float64); ok {
			metrics.SafepointTime = int64(totalTime)
		}
		if syncTime, ok := safepointInfo["SafepointSyncTime"].(float64); ok {
			metrics.SafepointSyncTime = int64(syncTime)
		}
		if appTime, ok := safepointInfo["ApplicationTime"].(float64); ok {
			metrics.ApplicationTime = int64(appTime)
		}

		if metrics.SafepointTime > 0 && metrics.ApplicationTime > 0 {
			metrics.ApplicationStoppedTime = metrics.SafepointTime
		}
	}

	return nil
}

// ===== HELPER METHODS =====

// Extract pool name from various possible sources in the pool data
func (jc *JMXCollector) extractPoolName(pool map[string]any) string {
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

// Extract GC name from various possible sources in the GC data
func (jc *JMXCollector) extractGCName(gc map[string]any) string {
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

// Helper function to extract total memory from GC info memory maps
func (jc *JMXCollector) extractTotalMemoryFromGCInfo(memoryMap map[string]any) int64 {
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

// --------------

func (jc *JMXCollector) updateMetrics(metrics *JVMSnapshot) {
	jc.mu.Lock()
	defer jc.mu.Unlock()
	jc.metrics = metrics
}

func (jc *JMXCollector) TestConnection() error {
	client := jc.getEffectiveClient()
	return client.TestConnection()
}

// Debug logging method to show available attributes
func (dc *DebugJMXClient) logAvailableAttributes(objectName string, data interface{}, queryType string) {
	if !dc.enabled || dc.debugFile == nil {
		return
	}

	entry := DebugLogEntry{
		Timestamp: time.Now(),
		MBeanName: objectName,
		QueryType: fmt.Sprintf("%s_available_attributes", queryType),
		RawData:   data,
	}

	// If it's a map, also log just the keys for easy scanning
	if dataMap, ok := data.(map[string]any); ok {
		keys := make([]string, 0, len(dataMap))
		for key := range dataMap {
			keys = append(keys, key)
		}
		entry.RawData = map[string]interface{}{
			"available_keys": keys,
			"full_data":      dataMap,
		}
	}

	jsonData, marshalErr := json.MarshalIndent(entry, "", "  ")
	if marshalErr != nil {
		fallbackLog := fmt.Sprintf("[%s] ERROR: Failed to marshal debug data for %s: %v\n",
			entry.Timestamp.Format(time.RFC3339), objectName, marshalErr)
		dc.debugFile.WriteString(fallbackLog)
		return
	}

	dc.debugFile.WriteString(string(jsonData) + "\n")
	dc.debugFile.Sync()
}

// Discover all available attributes for common MBeans
func (jc *JMXCollector) discoverAvailableAttributes() {
	if jc.debugClient == nil {
		return
	}

	client := jc.getEffectiveClient()

	// List of MBeans to discover
	mbeans := []string{
		"java.lang:type=Memory",
		"java.lang:type=Threading",
		"java.lang:type=Runtime",
		"java.lang:type=OperatingSystem",
		"java.lang:type=ClassLoading",
		"java.lang:type=Compilation",
	}

	for _, mbean := range mbeans {
		// Query without specific attributes to get everything available
		data, err := client.QueryMBean(mbean)
		if err == nil {
			jc.debugClient.logAvailableAttributes(mbean, data, "discovery")
		}
	}

	// Discover memory pools
	pools, err := client.QueryMBeanPattern("java.lang:type=MemoryPool,name=*")
	if err == nil && len(pools) > 0 {
		jc.debugClient.logAvailableAttributes("java.lang:type=MemoryPool,name=*", pools, "discovery_pools")
	}

	// Discover GC collectors
	gcs, err := client.QueryMBeanPattern("java.lang:type=GarbageCollector,name=*")
	if err == nil && len(gcs) > 0 {
		jc.debugClient.logAvailableAttributes("java.lang:type=GarbageCollector,name=*", gcs, "discovery_gc")
	}
}
