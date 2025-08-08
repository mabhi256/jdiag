package jmx

import "time"

// The current JVM metrics snapshot
type JVMSnapshot struct {
	Timestamp time.Time
	Connected bool
	Error     error

	// ===== BASIC MEMORY METRICS =====
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

	// ===== DETAILED NON-HEAP BREAKDOWN =====
	// Metaspace (replaces PermGen in Java 8+)
	MetaspaceUsed      int64
	MetaspaceCommitted int64
	MetaspaceMax       int64

	// Compressed Class Space
	CompressedClassSpaceUsed      int64
	CompressedClassSpaceCommitted int64
	CompressedClassSpaceMax       int64

	// Code Cache Details
	CodeCacheUsed      int64
	CodeCacheMax       int64
	CodeCacheCommitted int64

	// ===== PEAK MEMORY USAGE =====
	HeapPeakUsed    int64
	YoungPeakUsed   int64
	OldPeakUsed     int64
	NonHeapPeakUsed int64

	// ===== MEMORY AFTER LAST GC =====
	HeapAfterLastGC  int64
	YoungAfterLastGC int64
	OldAfterLastGC   int64

	// ===== MEMORY ALLOCATION TRACKING =====
	AllocationRate      float64 // Bytes allocated per second
	AllocationRateMBSec float64 // MB allocated per second
	EdenAllocationRate  float64 // Eden space allocation rate

	// ===== MEMORY MANAGEMENT =====
	ObjectPendingFinalizationCount int64 // Objects waiting for finalization
	MemoryVerboseLogging           bool  // Whether memory verbose logging is enabled

	// Memory Pool Management
	YoungGenManagers   []string // GC managers for young generation
	OldGenManagers     []string // GC managers for old generation
	InvalidMemoryPools []string // Pools that are no longer valid

	// Memory Pool Thresholds
	HeapThresholdExceeded    bool
	YoungThresholdExceeded   bool
	OldThresholdExceeded     bool
	NonHeapThresholdExceeded bool
	MemoryThresholdCount     int64 // Number of threshold crossings

	// ===== G1GC-SPECIFIC METRICS =====
	G1HeapRegionSize         int64 // Bytes
	G1TotalRegions           int64
	G1UsedRegions            int64
	G1FreeRegions            int64
	G1HumongousRegions       int64
	G1HumongousObjectCount   int64
	G1HumongousWasteBytes    int64 // Bytes wasted in humongous regions
	G1ConcurrentMarkActive   bool
	G1ConcurrentMarkProgress float64
	G1MixedGCLiveThreshold   float64 // Live data threshold for mixed GC
	G1OldGenRegionThreshold  float64 // Old gen threshold for mixed GC

	// G1GC Phase Timings
	G1YoungPauseTime      int64   // ms
	G1MixedPauseTime      int64   // ms
	G1EvacuationPauseTime int64   // ms
	G1RootScanTime        int64   // ms
	G1UpdateRSTime        int64   // Update remembered set time (ms)
	G1ScanRSTime          int64   // ms
	G1ObjectCopyTime      int64   // ms
	G1TerminationTime     int64   // ms
	G1GCOverheadLimit     float64 // percentage

	// ===== GARBAGE COLLECTION METRICS =====
	// Basic GC counts and times
	YoungGCCount int64
	YoungGCTime  int64 // ms
	OldGCCount   int64
	OldGCTime    int64 // ms

	// GC Management
	YoungGCManagedPools []string
	OldGCManagedPools   []string
	InvalidGCs          []string

	// Last GC Details
	LastGCCause        string
	LastGCAction       string // GC action taken
	LastGCName         string // Name of the GC that ran
	LastGCDuration     int64  // ms
	LastGCTimestamp    time.Time
	LastGCEndTime      time.Time
	LastGCId           int64 // GC sequence number
	LastGCMemoryBefore int64
	LastGCMemoryAfter  int64
	LastGCMemoryFreed  int64

	// Collection Efficiency
	YoungGCEfficiency   float64 // Memory freed / time spent (MB/ms)
	OldGCEfficiency     float64
	OverallGCEfficiency float64 // Total efficiency across all GCs

	// ===== THREADING METRICS =====
	// Basic thread counts
	ThreadCount             int64
	PeakThreadCount         int64
	DaemonThreadCount       int64
	TotalStartedThreadCount int64 // Total threads started since JVM start

	// Thread states
	BlockedThreadCount      int64
	WaitingThreadCount      int64
	ThreadTimedWaitingCount int64

	// Thread identification and tracking
	AllThreadIds             []int64  // All current thread IDs
	DeadlockedThreads        []string // Thread names in deadlock
	MonitorDeadlockedThreads []string // Threads deadlocked on object monitors

	// Thread Performance Metrics
	ThreadCPUTime               int64 // Total CPU time used by all threads (nanoseconds)
	ThreadUserTime              int64 // Total user CPU time (nanoseconds)
	CurrentThreadAllocatedBytes int64
	ThreadContentionCount       int64 // Number of times threads blocked on monitors
	ThreadContentionTime        int64 // Total time threads spent blocked (nanoseconds)

	// Thread Monitoring Capabilities
	ThreadCpuTimeSupported              bool // Whether thread CPU time monitoring is supported
	ThreadCpuTimeEnabled                bool
	ThreadAllocatedMemorySupported      bool // Whether thread memory allocation tracking is supported
	ThreadAllocatedMemoryEnabled        bool
	ThreadContentionMonitoringSupported bool
	ThreadContentionMonitoringEnabled   bool
	ObjectMonitorUsageSupported         bool
	SynchronizerUsageSupported          bool

	// ===== CLASS LOADING METRICS =====
	LoadedClassCount           int64 // Currently loaded classes
	TotalLoadedClassCount      int64 // Total classes loaded since JVM start
	UnloadedClassCount         int64 // Total classes unloaded
	ClassLoadingVerboseLogging bool  // Whether class loading verbose logging is enabled

	// ===== CPU METRICS =====
	ProcessCpuLoad      float64 // Process CPU usage (0.0 to 1.0)
	SystemCpuLoad       float64 // System CPU usage (0.0 to 1.0)
	CpuLoad             float64 // Alternative CPU load metric
	ProcessCpuTime      int64   // Total CPU time used by process (nanoseconds)
	AvailableProcessors int64
	SystemLoadAverage   float64

	// ===== SYSTEM MEMORY INFORMATION =====
	// Physical memory
	TotalSystemMemory int64 // Total physical memory
	FreeSystemMemory  int64 // Free physical memory
	TotalMemorySize   int64 // Alternative total memory metric
	FreeMemorySize    int64 // Alternative free memory metric

	// Swap space
	TotalSwapSize int64
	FreeSwapSize  int64

	// Process memory details
	ProcessVirtualMemorySize int64 // VSZ - Virtual memory size
	ProcessResidentSetSize   int64 // RSS - Resident set size
	ProcessPrivateMemory     int64 // Process private memory
	ProcessSharedMemory      int64 // Process shared memory

	// ===== FILE DESCRIPTOR USAGE =====
	// (Unix/Linux systems only)
	OpenFileDescriptorCount int64
	MaxFileDescriptorCount  int64

	// ===== JVM CONFIGURATION AND RUNTIME =====
	// Process identification
	ProcessID   int64
	ProcessName string // usually PID@hostname

	// JVM runtime information
	JVMName      string
	JVMVendor    string
	JVMVersion   string
	JVMStartTime time.Time
	JVMUptime    time.Duration

	// JVM specification information
	JVMSpecName           string
	JVMSpecVendor         string
	JVMSpecVersion        string
	ManagementSpecVersion string

	// JVM configuration
	JVMArguments           []string
	JavaClassPath          string
	JavaLibraryPath        string
	BootClassPath          string
	BootClassPathSupported bool

	// ===== SYSTEM PROPERTIES =====
	SystemProperties map[string]string // All system properties

	// Commonly used properties (extracted for convenience)
	JavaVersion string
	JavaVendor  string
	JavaHome    string
	UserName    string
	UserHome    string
	OSName      string
	OSArch      string
	OSVersion   string

	// GC configuration
	GCName     string            // Current GC algorithm name
	GCSettings map[string]string // GC-related system properties

	// ===== COMPILATION STATISTICS =====
	TotalCompilationTime               int64  // Total JIT compilation time (ms)
	CompilerName                       string // Compiler name
	CompilationTimeMonitoringSupported bool   // Whether compilation time monitoring is supported

	// ===== BUFFER POOL METRICS =====
	// Direct buffers (off-heap)
	DirectBufferCount    int64
	DirectBufferUsed     int64
	DirectBufferCapacity int64

	// Memory-mapped buffers
	MappedBufferCount    int64
	MappedBufferUsed     int64
	MappedBufferCapacity int64

	// ===== SAFEPOINT STATISTICS =====
	// (HotSpot JVM specific)
	SafepointCount         int64 // Number of safepoints reached
	SafepointTime          int64 // Total time spent in safepoints (ms)
	SafepointSyncTime      int64 // Time to reach safepoint (ms)
	ApplicationTime        int64 // Time application was running (ms)
	ApplicationStoppedTime int64 // Total stopped time (ms)
}
