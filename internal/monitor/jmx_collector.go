package monitor

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

type JMXCollector struct {
	config   *Config
	client   *JMXClient
	metrics  *JVMSnapshot
	mu       sync.RWMutex
	running  bool
	stopChan chan struct{}
	errChan  chan error
}

func NewJMXCollector(config *Config) *JMXCollector {
	return &JMXCollector{
		config: config,
		metrics: &JVMSnapshot{
			Timestamp: time.Now(),
			Connected: false,
		},
		stopChan: make(chan struct{}),
		errChan:  make(chan error, 1),
	}
}

// Start metric collection
func (jc *JMXCollector) Start() error {
	if jc.running {
		return fmt.Errorf("collector already running")
	}

	// Create JMX client
	var err error
	if jc.config.IsLocalMonitoring() {
		jc.client, err = NewJMXClient(jc.config.PID, "")
	} else {
		jc.client, err = NewJMXClient(0, jc.config.GetJMXConnectionURL())
	}

	if err != nil {
		return fmt.Errorf("failed to create JMX client: %w", err)
	}

	jc.running = true

	// Start collection goroutine
	go jc.collectLoop()

	return nil
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
}

// Get the current snapshot
func (jc *JMXCollector) GetMetrics() *JVMSnapshot {
	jc.mu.RLock()
	defer jc.mu.RUnlock()

	// Return a copy to avoid race conditions
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
func (jc *JMXCollector) collectMetrics() {
	metrics := &JVMSnapshot{
		Timestamp: time.Now(),
		Connected: false,
	}

	// Define all collection functions
	collectors := []func(*JVMSnapshot) error{
		jc.collectMemoryMetrics,
		jc.collectGCMetrics,
		jc.collectThreadMetrics,
		jc.collectOSMetrics,
		jc.collectRuntimeMetrics,
	}

	// Execute all collectors
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

// Collect memory-related metrics
func (jc *JMXCollector) collectMemoryMetrics(metrics *JVMSnapshot) error {
	// Heap memory
	heapMemory, err := jc.client.QueryMBean("java.lang:type=Memory",
		[]string{"HeapMemoryUsage", "NonHeapMemoryUsage"})
	if err != nil {
		return fmt.Errorf("failed to query heap memory: %w", err)
	}

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

	// Memory pools for generation-specific metrics
	pools, err := jc.client.QueryMBeanPattern("java.lang:type=MemoryPool,name=*",
		[]string{"Usage", "Type"})
	if err != nil {
		return fmt.Errorf("failed to query memory pools: %w", err)
	}

	for _, pool := range pools {
		poolType, typeOk := pool["Type"].(string)
		usage, usageOk := pool["Usage"].(map[string]any)

		if !typeOk || !usageOk || poolType != "HEAP" {
			continue
		}

		objectName, nameOk := pool["ObjectName"].(string)
		if !nameOk {
			// Try lowercase objectName (this is what the Java client actually uses)
			objectName, nameOk = pool["objectName"].(string)
		}
		if !nameOk {
			continue
		}

		used, usedOk := usage["used"].(float64)
		committed, committedOk := usage["committed"].(float64)
		max, maxOk := usage["max"].(float64)

		if !usedOk || !committedOk {
			continue
		}

		// Categorize by generation based on pool name
		poolName := extractPoolName(objectName)

		// If ObjectName extraction failed, try alternative methods
		if poolName == "" {
			// Try to get name from other fields that might be available
			if name, exists := pool["Name"].(string); exists {
				poolName = name
			} else if name, exists := pool["name"].(string); exists {
				poolName = name
			}
		}

		if isYoungGenPool(poolName) {
			metrics.YoungUsed += int64(used)
			metrics.YoungCommitted += int64(committed)
			if maxOk && max > 0 {
				metrics.YoungMax += int64(max)
			}
		} else if isOldGenPool(poolName) {
			metrics.OldUsed += int64(used)
			metrics.OldCommitted += int64(committed)
			if maxOk && max > 0 {
				metrics.OldMax += int64(max)
			}
		}
	}

	return nil
}

// Collect garbage collection metrics
func (jc *JMXCollector) collectGCMetrics(metrics *JVMSnapshot) error {
	gcs, err := jc.client.QueryMBeanPattern("java.lang:type=GarbageCollector,name=*",
		[]string{"CollectionCount", "CollectionTime"})
	if err != nil {
		return fmt.Errorf("failed to query GC metrics: %w", err)
	}

	for _, gc := range gcs {
		objectName, nameOk := gc["ObjectName"].(string)
		if !nameOk {
			// Try lowercase objectName (this is what the Java client actually uses)
			objectName, nameOk = gc["objectName"].(string)
		}
		count, countOk := gc["CollectionCount"].(float64)
		time, timeOk := gc["CollectionTime"].(float64)

		if !nameOk || !countOk || !timeOk {
			continue
		}

		gcName := extractGCName(objectName)

		// If ObjectName extraction failed, try alternative methods
		if gcName == "" {
			// Try to get name from other fields that might be available
			if name, exists := gc["Name"].(string); exists {
				gcName = name
			} else if name, exists := gc["name"].(string); exists {
				gcName = name
			}
		}

		if isYoungGenGC(gcName) {
			metrics.YoungGCCount += int64(count)
			metrics.YoungGCTime += int64(time)
		} else if isOldGenGC(gcName) {
			metrics.OldGCCount += int64(count)
			metrics.OldGCTime += int64(time)
		}
	}

	return nil
}

// Collect thread-related metrics
func (jc *JMXCollector) collectThreadMetrics(metrics *JVMSnapshot) error {
	// Thread metrics
	threading, err := jc.client.QueryMBean("java.lang:type=Threading",
		[]string{"ThreadCount", "PeakThreadCount", "DaemonThreadCount"})
	if err != nil {
		return fmt.Errorf("failed to query thread metrics: %w", err)
	}

	if count, ok := threading["ThreadCount"].(float64); ok {
		metrics.ThreadCount = int64(count)
	}
	if peak, ok := threading["PeakThreadCount"].(float64); ok {
		metrics.PeakThreadCount = int64(peak)
	}

	// Class loading metrics
	classLoading, err := jc.client.QueryMBean("java.lang:type=ClassLoading",
		[]string{"LoadedClassCount", "UnloadedClassCount", "TotalLoadedClassCount"})
	if err != nil {
		return fmt.Errorf("failed to query class loading metrics: %w", err)
	}

	if loaded, ok := classLoading["TotalLoadedClassCount"].(float64); ok {
		metrics.LoadedClassCount = int64(loaded)
	}
	if unloaded, ok := classLoading["UnloadedClassCount"].(float64); ok {
		metrics.UnloadedClassCount = int64(unloaded)
	}

	return nil
}

// Collect OS metrics
func (jc *JMXCollector) collectOSMetrics(metrics *JVMSnapshot) error {
	osInfo, err := jc.client.QueryMBean("java.lang:type=OperatingSystem",
		[]string{"ProcessCpuLoad", "SystemCpuLoad", "AvailableProcessors", "TotalPhysicalMemorySize", "FreePhysicalMemorySize", "SystemLoadAverage"})
	if err != nil {
		return fmt.Errorf("failed to query CPU metrics: %w", err)
	}

	if processCpu, ok := osInfo["ProcessCpuLoad"].(float64); ok && processCpu >= 0 {
		metrics.ProcessCpuLoad = processCpu
	}
	if systemCpu, ok := osInfo["SystemCpuLoad"].(float64); ok && systemCpu >= 0 {
		metrics.SystemCpuLoad = systemCpu
	}
	if processors, ok := osInfo["AvailableProcessors"].(float64); ok {
		metrics.AvailableProcessors = int64(processors)
	}
	if totalMem, ok := osInfo["TotalPhysicalMemorySize"].(float64); ok {
		metrics.TotalSystemMemory = int64(totalMem)
	}
	if freeMem, ok := osInfo["FreePhysicalMemorySize"].(float64); ok {
		metrics.FreeSystemMemory = int64(freeMem)
	}
	if loadAvg, ok := osInfo["SystemLoadAverage"].(float64); ok && loadAvg >= 0 {
		metrics.SystemLoadAverage = loadAvg
	}

	return nil
}

func (jc *JMXCollector) collectRuntimeMetrics(metrics *JVMSnapshot) error {
	// Runtime information
	runtime, err := jc.client.QueryMBean("java.lang:type=Runtime",
		[]string{"Name", "VmName", "VmVendor", "VmVersion", "StartTime", "Uptime"})
	if err != nil {
		return fmt.Errorf("failed to query runtime metrics: %w", err)
	}

	// Extract runtime information
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

	return nil
}

// Updates the current metrics
func (jc *JMXCollector) updateMetrics(metrics *JVMSnapshot) {
	jc.mu.Lock()
	defer jc.mu.Unlock()
	jc.metrics = metrics
}

// Helper functions for categorizing memory pools and GC collectors

func extractPoolName(objectName string) string {
	// If objectName is empty, return empty
	if objectName == "" {
		return ""
	}

	// Extract name from "java.lang:type=MemoryPool,name=PoolName"
	if idx := strings.Index(objectName, "name="); idx != -1 {
		name := objectName[idx+5:]
		// Remove any trailing parameters
		if commaIdx := strings.Index(name, ","); commaIdx != -1 {
			name = name[:commaIdx]
		}
		return name
	}

	// Fallback: return the whole objectName
	return objectName
}

func extractGCName(objectName string) string {
	// If objectName is empty, return empty
	if objectName == "" {
		return ""
	}

	// Extract name from "java.lang:type=GarbageCollector,name=CollectorName"
	if idx := strings.Index(objectName, "name="); idx != -1 {
		name := objectName[idx+5:]
		// Remove any trailing parameters
		if commaIdx := strings.Index(name, ","); commaIdx != -1 {
			name = name[:commaIdx]
		}
		return name
	}

	// Fallback: return the whole objectName
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
		// G1GC specific pool names
		strings.Contains(lowerName, "g1 eden") ||
		strings.Contains(lowerName, "g1 survivor")
}

func isOldGenPool(poolName string) bool {
	lowerName := strings.ToLower(poolName)
	return strings.Contains(lowerName, "old") ||
		strings.Contains(lowerName, "tenured") ||
		strings.Contains(lowerName, "cms") ||
		// G1GC specific pool names
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
		// G1GC specific collector names
		strings.Contains(lowerName, "g1 young") ||
		strings.Contains(lowerName, "g1 young generation") ||
		// Sometimes the name might just be "g1" for young collections
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
		// G1GC specific collector names
		strings.Contains(lowerName, "g1 old") ||
		strings.Contains(lowerName, "g1 old generation") ||
		strings.Contains(lowerName, "g1 mixed") ||
		strings.Contains(lowerName, "g1 concurrent")
}

// Test the JMX connection
func (jc *JMXCollector) TestConnection() error {
	if jc.client == nil {
		return fmt.Errorf("no JMX client initialized")
	}

	return jc.client.TestConnection()
}

// Checks if the collector is currently running
func (jc *JMXCollector) IsRunning() bool {
	return jc.running
}

// Get info about the current connection
func (jc *JMXCollector) GetConnectionInfo() map[string]any {
	info := make(map[string]any)

	info["running"] = jc.running
	info["config"] = jc.config.String()
	info["target_type"] = "unknown"

	if jc.config.IsLocalMonitoring() {
		info["target_type"] = "local_pid"
		info["pid"] = jc.config.PID
	} else if jc.config.IsRemoteMonitoring() {
		info["target_type"] = "remote_jmx"
		info["host"] = jc.config.Host
		info["port"] = jc.config.Port
	}

	jc.mu.RLock()
	if jc.metrics != nil {
		info["connected"] = jc.metrics.Connected
		info["last_update"] = jc.metrics.Timestamp
		if jc.metrics.Error != nil {
			info["last_error"] = jc.metrics.Error.Error()
		}
	}
	jc.mu.RUnlock()

	return info
}

func (c *Config) GetJMXConnectionURL() string {
	if c.Host == "" || c.Port == 0 {
		return ""
	}

	// Standard JMX service URL format
	url := fmt.Sprintf("service:jmx:rmi:///jndi/rmi://%s:%d/jmxrmi", c.Host, c.Port)
	return url
}

func (c *Config) IsRemoteMonitoring() bool {
	return c.Host != "" && c.Port != 0
}

func (c *Config) IsLocalMonitoring() bool {
	return c.PID != 0
}
