package gc

import "time"

type GCEvent struct {
	ID        int
	Timestamp time.Time
	Type      string // "Young", "Mixed", "Full", "Concurrent"
	Subtype   string // "Normal", "Concurrent Start", etc.
	Cause     string // "G1 Evacuation Pause", "Metadata GC Threshold"

	// [gc] GC(0) Pause Young (Normal) (G1 Evacuation Pause) 65M->42M(256M) 26.506ms
	HeapBefore MemorySize
	HeapAfter  MemorySize
	HeapTotal  MemorySize
	Duration   time.Duration

	// [gc,cpu] GC(0) User=0.02s Sys=0.10s Real=0.03s
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

	// [gc,heap] GC(0)   region size 1024K, 64 young (65536K), 0 survivors (0K)
	RegionSize MemorySize

	// [gc,heap] GC(0) Eden regions: 64->0(62)
	EdenRegionsBefore int
	EdenRegionsAfter  int
	EdenRegionsTarget int
	EdenMemoryBefore  MemorySize
	EdenMemoryAfter   MemorySize

	// [gc,heap] GC(0) Survivor regions: 0->2(2)
	SurvivorRegionsBefore int
	SurvivorRegionsAfter  int
	SurvivorRegionsTarget int
	SurvivorMemoryBefore  MemorySize
	SurvivorMemoryAfter   MemorySize

	// [gc,heap] GC(0) Old regions: 2->42
	OldRegionsBefore int
	OldRegionsAfter  int
	OldMemoryBefore  MemorySize
	OldMemoryAfter   MemorySize

	// Young regions total (Eden + Survivor)
	YoungRegionsBefore int
	YoungRegionsAfter  int
	YoungMemoryBefore  MemorySize
	YoungMemoryAfter   MemorySize

	// [gc,heap] GC(0) Humongous regions: 0->0
	HumongousRegionsBefore int
	HumongousRegionsAfter  int
	HumongousMemoryBefore  MemorySize
	HumongousMemoryAfter   MemorySize

	// [gc,heap] GC(0)  garbage-first heap   total 262144K, used 66610K
	HeapTotalRegions      int
	HeapUsedRegionsBefore int
	HeapUsedRegionsAfter  int

	// [gc,task] GC(0) Using 6 workers of 10 for evacuation
	WorkersUsed      int
	WorkersAvailable int

	// G1GC-specific flags
	ToSpaceExhausted bool

	// [gc,metaspace] GC(0) Metaspace: 138K(320K)->138K(320K) NonClass: 130K(192K)->130K(192K) Class: 8K(128K)->8K(128K)
	// Metaspace: used(committed)->used(committed)
	// Metaspace used 138K, committed 320K, reserved 1114112K
	MetaspaceUsedBefore      MemorySize
	MetaspaceUsedAfter       MemorySize
	MetaspaceCapacityBefore  MemorySize
	MetaspaceCapacityAfter   MemorySize
	MetaspaceCommittedBefore MemorySize
	MetaspaceCommittedAfter  MemorySize
	MetaspaceReserved        MemorySize

	// class space    used 8K, committed 128K, reserved 1048576K
	ClassSpaceUsedBefore     MemorySize
	ClassSpaceUsedAfter      MemorySize
	ClassSpaceCapacityBefore MemorySize
	ClassSpaceCapacityAfter  MemorySize
	ClassSpaceReserved       MemorySize

	// [gc,marking] GC(5) Concurrent Mark Cycle
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

type Analysis struct {
	Critical []PerformanceIssue
	Warning  []PerformanceIssue
	Info     []PerformanceIssue
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
