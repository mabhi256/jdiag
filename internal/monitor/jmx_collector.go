// internal/monitor/jmx_collector.go - Simple fix that overrides the client methods
package monitor

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

type JMXCollector struct {
	config      *Config
	client      *JMXClient
	debugClient *DebugJMXClient // Wrapper for debug logging
	metrics     *JVMSnapshot
	mu          sync.RWMutex
	running     bool
	stopChan    chan struct{}
	errChan     chan error
	debugFile   *os.File
}

// DebugJMXClient wraps the original JMXClient to add debug logging
type DebugJMXClient struct {
	originalClient *JMXClient
	debugFile      *os.File
	enabled        bool
}

type DebugLogEntry struct {
	Timestamp time.Time   `json:"timestamp"`
	MBeanName string      `json:"mbean_name"`
	QueryType string      `json:"query_type"`
	RawData   interface{} `json:"raw_data"`
	Error     string      `json:"error,omitempty"`
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
	if jc.config.IsLocalMonitoring() {
		jc.client, err = NewJMXClient(jc.config.PID, "")
	} else {
		jc.client, err = NewJMXClient(0, jc.config.GetJMXConnectionURL())
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
func (dc *DebugJMXClient) QueryMBean(objectName string, attributes []string) (map[string]any, error) {
	data, err := dc.originalClient.QueryMBean(objectName, attributes)

	if dc.enabled {
		dc.logDebugData(objectName, "single", data, err)
	}

	return data, err
}

// QueryMBeanPattern with automatic debug logging
func (dc *DebugJMXClient) QueryMBeanPattern(pattern string, attributes []string) ([]map[string]any, error) {
	data, err := dc.originalClient.QueryMBeanPattern(pattern, attributes)

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
	QueryMBean(string, []string) (map[string]any, error)
	QueryMBeanPattern(string, []string) ([]map[string]any, error)
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
func (jc *JMXCollector) collectMetrics() {
	metrics := &JVMSnapshot{
		Timestamp: time.Now(),
		Connected: false,
	}

	collectors := []func(*JVMSnapshot) error{
		jc.collectMemoryMetrics,
		jc.collectGCMetrics,
		jc.collectThreadMetrics,
		jc.collectOSMetrics,
		jc.collectRuntimeMetrics,
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

// IMPORTANT: Replace all your existing collection methods with these updated versions
// that use the effective client (which will be the debug client if debug is enabled)

func (jc *JMXCollector) collectMemoryMetrics(metrics *JVMSnapshot) error {
	client := jc.getEffectiveClient()

	// Heap memory
	heapMemory, err := client.QueryMBean("java.lang:type=Memory",
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

	// Memory pools
	pools, err := client.QueryMBeanPattern("java.lang:type=MemoryPool,name=*",
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

		poolName := extractPoolName(objectName)
		if poolName == "" {
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

func (jc *JMXCollector) collectGCMetrics(metrics *JVMSnapshot) error {
	client := jc.getEffectiveClient()

	gcs, err := client.QueryMBeanPattern("java.lang:type=GarbageCollector,name=*",
		[]string{"CollectionCount", "CollectionTime"})
	if err != nil {
		return fmt.Errorf("failed to query GC metrics: %w", err)
	}

	for _, gc := range gcs {
		objectName, nameOk := gc["ObjectName"].(string)
		if !nameOk {
			objectName, nameOk = gc["objectName"].(string)
		}
		count, countOk := gc["CollectionCount"].(float64)
		time, timeOk := gc["CollectionTime"].(float64)

		if !nameOk || !countOk || !timeOk {
			continue
		}

		gcName := extractGCName(objectName)
		if gcName == "" {
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

func (jc *JMXCollector) collectThreadMetrics(metrics *JVMSnapshot) error {
	client := jc.getEffectiveClient()

	threading, err := client.QueryMBean("java.lang:type=Threading",
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

	classLoading, err := client.QueryMBean("java.lang:type=ClassLoading",
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

func (jc *JMXCollector) collectOSMetrics(metrics *JVMSnapshot) error {
	client := jc.getEffectiveClient()

	osInfo, err := client.QueryMBean("java.lang:type=OperatingSystem",
		[]string{"ProcessCpuLoad", "SystemCpuLoad", "AvailableProcessors",
			"TotalPhysicalMemorySize", "FreePhysicalMemorySize", "SystemLoadAverage"})
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
	client := jc.getEffectiveClient()

	runtime, err := client.QueryMBean("java.lang:type=Runtime",
		[]string{"Name", "VmName", "VmVendor", "VmVersion", "StartTime", "Uptime"})
	if err != nil {
		return fmt.Errorf("failed to query runtime metrics: %w", err)
	}

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

func (jc *JMXCollector) updateMetrics(metrics *JVMSnapshot) {
	jc.mu.Lock()
	defer jc.mu.Unlock()
	jc.metrics = metrics
}

func (jc *JMXCollector) TestConnection() error {
	client := jc.getEffectiveClient()
	return client.TestConnection()
}

func (jc *JMXCollector) IsRunning() bool {
	return jc.running
}

func (jc *JMXCollector) GetConnectionInfo() map[string]any {
	info := make(map[string]any)

	info["running"] = jc.running
	info["config"] = jc.config.String()
	info["debug_enabled"] = jc.config.Debug
	if jc.config.Debug {
		info["debug_log_file"] = jc.config.DebugLogFile
	}
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

// Helper functions (keep your existing implementations)
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
