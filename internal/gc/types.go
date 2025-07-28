package gc

import "time"

type GCEvent struct {
	ID        int
	Timestamp time.Time
	Type      string // "Young", "Mixed", "Full", "Concurrent"
	Subtype   string // "Normal", "Concurrent Start", etc.
	Cause     string // "G1 Evacuation Pause", "Metadata GC Threshold"
	Duration  time.Duration

	// Heap data from summary line: "9M->2M(16M)"
	HeapBefore MemorySize
	HeapAfter  MemorySize
	HeapTotal  MemorySize

	// CPU data: "User=0.00s Sys=0.00s Real=0.01s"
	UserTime   time.Duration
	SystemTime time.Duration
	RealTime   time.Duration
}

type GCLog struct {
	// Configuration from init lines
	JVMVersion     string
	HeapRegionSize MemorySize
	HeapMax        MemorySize

	// GC
	Events    []GCEvent
	StartTime time.Time
	EndTime   time.Time

	Status string
}

type GCMetrics struct {
	TotalEvents  int
	YoungGCCount int
	MixedGCCount int
	FullGCCount  int

	Throughput     float64 // percentage of time NOT spent in GC
	AvgHeapUtil    float64
	AllocationRate float64

	TotalRuntime time.Duration
	TotalGCTime  time.Duration
	AvgPause     time.Duration
	MinPause     time.Duration
	MaxPause     time.Duration
	P95Pause     time.Duration
	P99Pause     time.Duration
}

type PerformanceIssue struct {
	Type           string
	Severity       string // "warning", "critical", "info"
	Description    string
	Recommendation []string
}
