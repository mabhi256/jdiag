// poller.go
package jmx

import (
	"fmt"
	"os"
	"sync"
	"time"
)

type JMXPoller struct {
	config            *Config
	client            *JMXClient
	debugClient       *DebugJMXClient // Wrapper for debug logging
	metrics           *MBeanSnapshot
	mu                sync.RWMutex
	running           bool
	stopChan          chan struct{}
	errChan           chan error
	debugFile         *os.File // Raw JMX debug logging
	snapshotDebugFile *os.File // Parsed snapshot debug logging
}

func NewJMXCollector(config *Config) *JMXPoller {
	collector := &JMXPoller{
		config: config,
		metrics: &MBeanSnapshot{
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

// Start metric collection
func (jc *JMXPoller) Start() error {
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

		// Run attribute discovery if debug mode is enabled
		go func() {
			time.Sleep(1 * time.Second) // Give the client time to initialize
			jc.discoverAvailableAttributes()
		}()
	}

	jc.running = true

	// Start collection goroutine
	go jc.collectLoop()

	return nil
}

func (jc *JMXPoller) getEffectiveClient() JMXClientInterface {
	if jc.debugClient != nil {
		return jc.debugClient
	}
	return jc.client
}

// Stop metric collection
func (jc *JMXPoller) Stop() {
	if !jc.running {
		return
	}

	jc.running = false
	close(jc.stopChan)

	if jc.client != nil {
		jc.client.Close()
	}

	// Close debug files
	if jc.debugFile != nil {
		footer := fmt.Sprintf("=== JMX Debug Session Ended at %s ===\n", time.Now().Format(time.RFC3339))
		jc.debugFile.WriteString(footer)
		jc.debugFile.Close()
		jc.debugFile = nil
	}

	if jc.snapshotDebugFile != nil {
		footer := fmt.Sprintf("=== JMX Snapshot Debug Session Ended at %s ===\n", time.Now().Format(time.RFC3339))
		jc.snapshotDebugFile.WriteString(footer)
		jc.snapshotDebugFile.Close()
		jc.snapshotDebugFile = nil
	}
}

// Get the current snapshot
func (jc *JMXPoller) GetMetrics() *MBeanSnapshot {
	jc.mu.RLock()
	defer jc.mu.RUnlock()
	metricsCopy := *jc.metrics
	return &metricsCopy
}

// Metric collection loop
func (jc *JMXPoller) collectLoop() {
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
func (jc *JMXPoller) collectMetrics() {
	metrics := &MBeanSnapshot{
		Timestamp: time.Now(),
		Connected: false,
	}

	collectors := []func(*MBeanSnapshot) error{
		jc.collectMemoryMetrics,
		jc.collectMemoryPools,
		jc.collectBufferPools,
		jc.collectGCMetrics,
		jc.collectThreadingMetrics,
		jc.collectClassLoadingMetrics,
		jc.collectOperatingSystemMetrics,
		jc.collectRuntimeMetrics,
	}

	for _, collect := range collectors {
		if err := collect(metrics); err != nil {
			metrics.Error = err
			jc.updateMetrics(metrics)

			// Log failed snapshot if debug mode is enabled
			if jc.config.Debug {
				jc.logParsedSnapshot(metrics)
			}
			return
		}
	}

	metrics.Connected = true
	jc.updateMetrics(metrics)

	// Log successful snapshot if debug mode is enabled
	if jc.config.Debug {
		jc.logParsedSnapshot(metrics)
	}
}

func (jc *JMXPoller) updateMetrics(metrics *MBeanSnapshot) {
	jc.mu.Lock()
	defer jc.mu.Unlock()
	jc.metrics = metrics
}

func (jc *JMXPoller) TestConnection() error {
	client := jc.getEffectiveClient()
	return client.TestConnection()
}
