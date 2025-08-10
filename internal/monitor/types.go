package monitor

import (
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
	MemoryPressure  string // "low", "moderate", "high", "critical"
	LastMemoryAlert *PerformanceAlert
}

type GCState struct {
	YoungGCCount int64
	YoungGCTime  int64 // ms
	YoungGCAvg   float64

	OldGCCount int64
	OldGCTime  int64 // ms
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
	TotalStartedCount  int64

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
	TotalSwap           int64
	FreeSwap            int64
	UsedSwap            int64
	SwapPercent         float64

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
