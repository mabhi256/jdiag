package jmx

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

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

type SnapshotLogEntry struct {
	Timestamp time.Time      `json:"timestamp"`
	Connected bool           `json:"connected"`
	Error     string         `json:"error,omitempty"`
	Snapshot  *MBeanSnapshot `json:"snapshot"`
}

func (jc *JMXPoller) initDebugLogging() error {
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

	// Initialize snapshot debug logging
	if err := jc.initSnapshotDebugLogging(); err != nil {
		return fmt.Errorf("failed to initialize snapshot debug logging: %w", err)
	}

	return nil
}

func (jc *JMXPoller) initSnapshotDebugLogging() error {
	timestamp := time.Now().Format("20060102_150405")
	snapshotLogFile := fmt.Sprintf("jmx_snapshots_%s.log", timestamp)

	file, err := os.OpenFile(snapshotLogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open snapshot debug log file: %w", err)
	}

	jc.snapshotDebugFile = file

	// Write snapshot debug session header
	header := fmt.Sprintf("=== JMX Snapshot Debug Session Started at %s ===\n", time.Now().Format(time.RFC3339))
	if _, err := jc.snapshotDebugFile.WriteString(header); err != nil {
		return fmt.Errorf("failed to write snapshot debug header: %w", err)
	}

	return nil
}

// QueryMBean implementation for DebugJMXClient
func (dc *DebugJMXClient) QueryMBean(objectName string) (map[string]any, error) {
	result, err := dc.originalClient.QueryMBean(objectName)

	if dc.enabled && dc.debugFile != nil {
		dc.logQueryResult(objectName, "QueryMBean", result, err)
	}

	return result, err
}

// QueryMBeanPattern implementation for DebugJMXClient
func (dc *DebugJMXClient) QueryMBeanPattern(pattern string) ([]map[string]any, error) {
	result, err := dc.originalClient.QueryMBeanPattern(pattern)

	if dc.enabled && dc.debugFile != nil {
		dc.logQueryResult(pattern, "QueryMBeanPattern", result, err)
	}

	return result, err
}

// TestConnection implementation for DebugJMXClient
func (dc *DebugJMXClient) TestConnection() error {
	return dc.originalClient.TestConnection()
}

// Close implementation for DebugJMXClient
func (dc *DebugJMXClient) Close() error {
	return dc.originalClient.Close()
}

func (dc *DebugJMXClient) logQueryResult(objectName, queryType string, data any, err error) {
	entry := DebugLogEntry{
		Timestamp: time.Now(),
		MBeanName: objectName,
		QueryType: queryType,
		RawData:   data,
	}

	if err != nil {
		entry.Error = err.Error()
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

// Log the complete parsed MBeanSnapshot
func (jc *JMXPoller) logParsedSnapshot(snapshot *MBeanSnapshot) {
	if jc.snapshotDebugFile == nil {
		return
	}

	entry := SnapshotLogEntry{
		Timestamp: time.Now(),
		Connected: snapshot.Connected,
		Snapshot:  snapshot,
	}

	if snapshot.Error != nil {
		entry.Error = snapshot.Error.Error()
	}

	jsonData, marshalErr := json.MarshalIndent(entry, "", "  ")
	if marshalErr != nil {
		fallbackLog := fmt.Sprintf("[%s] ERROR: Failed to marshal snapshot data: %v\n",
			entry.Timestamp.Format(time.RFC3339), marshalErr)
		jc.snapshotDebugFile.WriteString(fallbackLog)
		return
	}

	jc.snapshotDebugFile.WriteString(string(jsonData) + "\n")
	jc.snapshotDebugFile.Sync()
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
func (jc *JMXPoller) discoverAvailableAttributes() {
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
