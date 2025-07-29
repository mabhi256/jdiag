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

	// G1GC timing fields (Microsoft GC Toolkit patterns)
	PreEvacuateTime         time.Duration
	PostEvacuateTime        time.Duration
	ExtRootScanTime         time.Duration
	UpdateRSTime            time.Duration
	ScanRSTime              time.Duration
	CodeRootScanTime        time.Duration
	ObjectCopyTime          time.Duration
	TerminationTime         time.Duration
	WorkerOtherTime         time.Duration
	ReferenceProcessingTime time.Duration
	EvacuationFailureTime   time.Duration

	// G1GC region information
	RegionSize            MemorySize
	YoungRegions          int
	YoungRegionsMemory    MemorySize
	SurvivorRegions       int
	SurvivorRegionsMemory MemorySize
	EdenRegions           int
	SurvivorRegionsAfter  int
	OldRegions            int
	HumongousRegions      int
	HeapTotalRegions      int
	HeapUsedRegions       int

	// Worker thread information
	WorkersUsed      int
	WorkersAvailable int

	// G1GC-specific flags
	ToSpaceExhausted bool

	// Metaspace information
	MetaspaceUsed      MemorySize
	MetaspaceCapacity  MemorySize
	MetaspaceCommitted MemorySize
	MetaspaceReserved  MemorySize
	ClassSpaceUsed     MemorySize
	ClassSpaceCapacity MemorySize

	// Concurrent marking details
	ConcurrentPhase    string
	ConcurrentDuration time.Duration
	ConcurrentCycleId  int
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

	// G1GC collection analysis
	YoungCollectionEfficiency float64
	MixedCollectionEfficiency float64
	MixedToYoungRatio         float64
	ConcurrentCycleDuration   time.Duration
	ConcurrentCycleFrequency  float64

	// G1GC pause time analysis
	PauseTimeVariance   float64
	PauseTargetMissRate float64
	LongPauseCount      int

	// G1GC region analysis
	AvgRegionUtilization   float64
	RegionExhaustionEvents int
	EvacuationFailureRate  float64

	// G1GC concurrent marking
	ConcurrentMarkingKeepup  bool
	ConcurrentCycleFailures  int
	IHOPTriggeredCollections int

	// G1GC allocation patterns
	AllocationBurstCount    int
	AvgPromotionRate        float64 // regions promoted per young GC
	MaxPromotionRate        float64
	AvgOldGrowthRatio       float64
	MaxOldGrowthRatio       float64
	SurvivorOverflowRate    float64 // % of collections with survivor overflow
	PromotionEfficiency     float64 // how much promoted data survives concurrent mark
	ConsecutiveGrowthSpikes int     // consecutive high-growth young GCs
}

type PerformanceIssue struct {
	Type           string
	Severity       string // "warning", "critical", "info"
	Description    string
	Recommendation []string
}

type MemoryTrend struct {
	GrowthRateMBPerHour   float64       // Raw memory growth rate
	GrowthRatePercent     float64       // Growth as % of heap per hour
	BaselineGrowthRate    float64       // Growth of post-GC baseline
	TrendConfidence       float64       // R-squared correlation (0-1)
	ProjectedFullHeapTime time.Duration // Time until heap exhaustion
	LeakSeverity          string        // none, warning, critical
	SamplePeriod          time.Duration // Duration of analysis
	EventCount            int           // Number of GC events analyzed
}
