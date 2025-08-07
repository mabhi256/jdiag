package monitor

import (
	"fmt"
	"sync"
	"time"
)

type TabType int

const (
	TabMemory TabType = iota
	TabGC
	TabThreads
	TabSystem
)

func (t TabType) String() string {
	switch t {
	case TabMemory:
		return "Memory"
	case TabGC:
		return "GC"
	case TabThreads:
		return "Threads"
	case TabSystem:
		return "System"
	default:
		return "Unknown"
	}
}

func GetAllTabs() []TabType {
	return []TabType{TabMemory, TabGC, TabThreads, TabSystem}
}

// A discovered Java process
type JavaProcess struct {
	PID        int    `json:"pid"`
	MainClass  string `json:"mainClass"`
	User       string `json:"user"`
	JMXEnabled bool   `json:"jmxEnabled"`
	JMXPort    int    `json:"jmxPort,omitempty"`
	Args       string `json:"args,omitempty"`
}

// The current JVM metrics snapshot
type JVMSnapshot struct {
	Timestamp time.Time
	Connected bool
	Error     error

	// Heap memory
	HeapUsed      int64
	HeapCommitted int64
	HeapMax       int64

	// Generation-specific memory
	YoungUsed      int64
	YoungCommitted int64
	YoungMax       int64

	OldUsed      int64
	OldCommitted int64
	OldMax       int64

	// Non-heap memory
	NonHeapUsed      int64
	NonHeapCommitted int64
	NonHeapMax       int64

	// Garbage collection
	YoungGCCount int64
	YoungGCTime  int64 // milliseconds
	OldGCCount   int64
	OldGCTime    int64 // milliseconds

	// Threading
	ThreadCount     int64
	PeakThreadCount int64

	// Class loading
	LoadedClassCount   int64
	UnloadedClassCount int64

	// CPU
	ProcessCpuLoad      float64
	SystemCpuLoad       float64
	AvailableProcessors int64

	// JVM Runtime Information
	JVMName      string
	JVMVendor    string
	JVMVersion   string
	JVMStartTime time.Time
	JVMUptime    time.Duration

	// System Memory Information
	TotalSystemMemory int64
	FreeSystemMemory  int64
	SystemLoadAverage float64
}

type GCEvent struct {
	Timestamp  time.Time
	Generation string // "young" or "old"
	Duration   time.Duration
	Before     int64 // Memory before GC
	After      int64 // Memory after GC
	Collected  int64 // Amount collected
}

type PerformanceAlert struct {
	Level       string // "info", "warning", "critical"
	Title       string
	Description string
	Timestamp   time.Time
	Value       float64
	Threshold   float64
	MetricName  string
}

type MemoryState struct {
	HeapUsed         int64
	HeapCommitted    int64
	HeapMax          int64
	HeapUsagePercent float64

	YoungUsed         int64
	YoungCommitted    int64
	YoungMax          int64
	YoungUsagePercent float64

	OldUsed         int64
	OldCommitted    int64
	OldMax          int64
	OldUsagePercent float64

	NonHeapUsed         int64
	NonHeapCommitted    int64
	NonHeapMax          int64
	NonHeapUsagePercent float64

	// Memory trends and alerts
	AllocationRate  float64 // bytes per second
	MemoryPressure  string  // "low", "moderate", "high", "critical"
	LastMemoryAlert *PerformanceAlert
	MemoryTrend     float64 // percentage change per minute
}

func (ms *MemoryState) GetMemoryPressureLevel() string {
	maxUsage := ms.HeapUsagePercent
	if ms.OldUsagePercent > maxUsage {
		maxUsage = ms.OldUsagePercent
	}

	switch {
	case maxUsage > 0.90:
		return "critical"
	case maxUsage > 0.75:
		return "high"
	case maxUsage > 0.50:
		return "moderate"
	default:
		return "low"
	}
}

type GCState struct {
	YoungGCCount int64
	YoungGCTime  int64 // milliseconds
	YoungGCAvg   float64

	OldGCCount int64
	OldGCTime  int64 // milliseconds
	OldGCAvg   float64

	TotalGCCount int64
	TotalGCTime  int64

	// GC performance metrics
	GCOverhead      float64 // percentage of total time
	GCFrequency     float64 // GCs per minute
	LastGCEvent     *GCEvent
	GCPressureLevel string // "low", "moderate", "high", "critical"
	RecentGCEvents  []GCEvent
	AvgGCPauseTime  time.Duration
}

func (gs *GCState) GetGCPressureLevel() string {
	switch {
	case gs.GCOverhead > 0.10: // 10% overhead
		return "critical"
	case gs.GCOverhead > 0.05: // 5% overhead
		return "high"
	case gs.GCFrequency > 60: // More than 1 GC per second
		return "moderate"
	default:
		return "low"
	}
}

type ThreadState struct {
	CurrentThreadCount int64
	PeakThreadCount    int64
	DaemonThreadCount  int64

	LoadedClassCount   int64
	UnloadedClassCount int64
	TotalLoadedClasses int64

	// Thread performance metrics
	ThreadCreationRate float64 // threads created per minute
	ThreadContention   bool    // whether thread contention is detected
	DeadlockedThreads  int64
	BlockedThreadCount int64
	WaitingThreadCount int64

	// Class loading metrics
	ClassLoadingRate   float64 // classes loaded per minute
	ClassUnloadingRate float64 // classes unloaded per minute
}

type SystemState struct {
	ProcessCpuLoad float64
	SystemCpuLoad  float64

	// System memory (not JVM heap)
	TotalSystemMemory   int64
	FreeSystemMemory    int64
	UsedSystemMemory    int64
	SystemMemoryPercent float64

	// JVM uptime and info
	JVMUptime    time.Duration
	JVMStartTime time.Time
	JVMVersion   string
	JVMVendor    string

	// Process info
	ProcessName string
	ProcessUser string
	ProcessArgs []string

	// Performance indicators
	CPUTrend            float64 // percentage change per minute
	SystemLoad          float64
	AvailableProcessors int64

	// Connection metrics
	ConnectionUptime time.Duration
	UpdateCount      int64
	LastUpdateTime   time.Time
}

type TabState struct {
	Memory  *MemoryState
	GC      *GCState
	Threads *ThreadState
	System  *SystemState
}

func NewTabState() *TabState {
	return &TabState{
		Memory: &MemoryState{
			MemoryPressure: "low",
		},
		GC: &GCState{
			GCPressureLevel: "low",
			RecentGCEvents:  make([]GCEvent, 0),
		},
		Threads: &ThreadState{},
		System:  &SystemState{},
	}
}

type Config struct {
	// Target configuration
	PID  int    // Process ID for local monitoring
	Host string // Host for remote monitoring
	Port int    // Port for remote monitoring

	Interval int // Update interval in milliseconds
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

type JMXClient struct {
	connectionURL string // JMX service URL
	pid           int    // Process ID for local attachment
	tempDir       string // Temporary directory for generated Java code
	javaPath      string // Path to Java executable
}

type JMXCollector struct {
	config   *Config
	client   *JMXClient
	metrics  *JVMSnapshot
	mu       sync.RWMutex
	running  bool
	stopChan chan struct{}
	errChan  chan error
}
