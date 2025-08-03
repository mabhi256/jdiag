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

	// G1GC detailed timing
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

	// ===== ANALYSIS FLAGS (computed during traversal) =====

	// Performance issue flags
	HasEvacuationFailure  bool
	HasHighPauseTime      bool
	HasMemoryPressure     bool
	HasSurvivorOverflow   bool
	HasHumongousGrowth    bool
	IsPrematurePromotion  bool
	HasLongPhases         bool
	ConcurrentMarkAborted bool
	HasAllocationBurst    bool // unused

	// Computed metrics for this event
	CollectionEfficiency  float64 // Amount collected / heap before
	HeapUtilizationBefore float64 // Heap used / heap total (before GC)
	HeapUtilizationAfter  float64 // Heap used / heap total (after GC)
	RegionUtilization     float64 // Used regions / total regions
	PromotionRate         float64 // Regions promoted to old gen
	AllocationRateToEvent float64 // MB/s since last event
	PauseTargetExceeded   bool    // Duration > estimated pause target

	// Phase analysis flags
	HasSlowObjectCopy    bool
	HasSlowRootScanning  bool
	HasSlowTermination   bool
	HasSlowRefProcessing bool
}

type GCAnalysis struct {
	// ===== BASIC INFO ====
	JVMVersion     string
	HeapRegionSize MemorySize
	HeapMax        MemorySize
	TotalEvents    int
	YoungGCCount   int
	MixedGCCount   int
	FullGCCount    int

	StartTime    time.Time
	EndTime      time.Time
	Status       string
	TotalRuntime time.Duration
	TotalGCTime  time.Duration

	// ===== PERFORMANCE METRICS =====
	Throughput     float64 // percentage of time NOT spent in GC
	AvgHeapUtil    float64
	AllocationRate float64

	// Pause time metrics
	AvgPause time.Duration
	MinPause time.Duration
	MaxPause time.Duration
	P95Pause time.Duration
	P99Pause time.Duration

	// ===== G1GC SPECIFIC METRICS =====

	// Collection efficiency
	YoungCollectionEfficiency float64
	MixedCollectionEfficiency float64
	MixedToYoungRatio         float64

	// Pause analysis
	PauseTimeVariance    float64
	PauseTargetMissRate  float64
	LongPauseCount       int
	EstimatedPauseTarget time.Duration

	// Region management
	AvgRegionUtilization   float64
	RegionExhaustionEvents int
	EvacuationFailureRate  float64
	EvacuationFailureCount int

	// Concurrent marking
	ConcurrentMarkingKeepup  bool
	ConcurrentCycleDuration  time.Duration
	ConcurrentCycleFrequency float64
	ConcurrentCycleFailures  int
	ConcurrentMarkAbortCount int

	// Allocation patterns
	AllocationBurstCount    int
	AvgPromotionRate        float64
	MaxPromotionRate        float64
	AvgOldGrowthRatio       float64
	MaxOldGrowthRatio       float64
	SurvivorOverflowRate    float64
	PromotionEfficiency     float64
	ConsecutiveGrowthSpikes int

	// ===== TIME DISTRIBUTION ANALYSIS =====

	// GC Type time distributions
	GCTypeDurations   map[string]time.Duration // "Young", "Mixed", "Full", "Concurrent Mark", "Other"
	GCTypeEventCounts map[string]int           // Event counts by type

	// GC Cause time distributions
	GCCauseDurations map[string]time.Duration // Total time spent per cause

	// ===== AGGREGATE ANALYSIS RESULTS =====

	// Humongous object analysis
	HumongousStats HumongousObjectStats

	// Memory leak analysis
	MemoryTrend          MemoryTrend
	MemoryLeakIndicators []string
	LeakScore            int

	// Promotion analysis
	PromotionStats PromotionAnalysis

	// Phase timing analysis
	PhaseStats PhaseAnalysis

	// ===== ISSUE FLAGS FOR RECOMMENDATIONS =====

	// Critical issues
	HasCriticalMemoryLeak          bool
	HasCriticalEvacFailures        bool
	HasCriticalThroughput          bool
	HasCriticalPauseTimes          bool
	HasCriticalPromotion           bool
	HasCriticalHumongousLeak       bool
	HasCriticalConcurrentMarkAbort bool

	// Warning issues
	HasWarningMemoryLeak     bool
	HasWarningEvacFailures   bool
	HasWarningThroughput     bool
	HasWarningPauseTimes     bool
	HasWarningPromotion      bool
	HasWarningHumongousUsage bool
	HasWarningConcurrentMark bool
	HasWarningAllocationRate bool
	HasWarningCollectionEff  bool

	// Info issues
	HasInfoAllocationPattern bool
	HasInfoPhaseOptimization bool
}

type HumongousObjectStats struct {
	MaxRegions      int
	HeapPercentage  float64
	StaticCount     int
	GrowingCount    int
	DecreasingCount int
	IsLeak          bool
	TotalEvents     int
}

type PromotionAnalysis struct {
	TotalPromotionEvents   int
	AvgPromotionRate       float64
	MaxPromotionRate       float64
	AvgOldGrowthRatio      float64
	MaxOldGrowthRatio      float64
	SurvivorOverflowCount  int
	SurvivorOverflowRate   float64
	PromotionEfficiency    float64
	ConsecutiveSpikes      int
	PrematurePromotionRate float64
}

type PhaseAnalysis struct {
	AvgObjectCopyTime    time.Duration
	AvgRootScanTime      time.Duration
	AvgTerminationTime   time.Duration
	AvgRefProcessingTime time.Duration

	SlowObjectCopyCount    int
	SlowRootScanCount      int
	SlowTerminationCount   int
	SlowRefProcessingCount int

	HasPhaseIssues bool
}

type MemoryTrend struct {
	GrowthRateMBPerHour   float64
	GrowthRatePercent     float64
	BaselineGrowthRate    float64
	TrendConfidence       float64
	ProjectedFullHeapTime time.Duration
	LeakSeverity          string
	SamplePeriod          time.Duration
	EventCount            int
}

type PerformanceIssue struct {
	Type           string
	Severity       string // "warning", "critical", "info"
	Description    string
	Recommendation []string
}

type GCIssues struct {
	Critical []PerformanceIssue
	Warning  []PerformanceIssue
	Info     []PerformanceIssue
}
