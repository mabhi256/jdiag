package jmx

import "time"

type MemoryUsage struct {
	Used      int64
	Committed int64
	Max       int64
	Init      int64
}

type ThresholdInfo struct {
	Supported bool
	Threshold int64
	Exceeded  bool
	Count     int64
}

type MemoryPool struct {
	Usage               MemoryUsage
	PeakUsage           MemoryUsage
	Threshold           ThresholdInfo
	CollectionUsage     MemoryUsage
	CollectionThreshold ThresholdInfo
	Valid               bool
	Managers            []string
}

type BufferPool struct {
	Count    int64
	Used     int64
	Capacity int64
}

type Memory struct {
	// Basic heap/non-heap totals
	Heap    MemoryUsage
	NonHeap MemoryUsage

	// Detailed memory pools
	Metaspace            MemoryPool
	CompressedClassSpace MemoryPool
	CodeCache            MemoryUsage // Code cache is spread across multiple pools
	G1Eden               MemoryPool
	G1Survivor           MemoryPool
	G1OldGen             MemoryPool

	// Buffer pools
	DirectBuffers BufferPool
	MappedBuffers BufferPool

	// Memory management
	ObjectPendingFinalizationCount int64
	VerboseLogging                 bool
}

type LastGCInfo struct {
	Duration    int64
	Timestamp   time.Time
	EndTime     time.Time
	Id          int64
	ThreadCount int64

	// Per-pool memory usage before/after GC
	EdenBefore      int64
	EdenAfter       int64
	SurvivorBefore  int64
	SurvivorAfter   int64
	OldBefore       int64
	OldAfter        int64
	MetaspaceBefore int64
	MetaspaceAfter  int64
}

type GarbageCollector struct {
	// Basic GC metrics
	YoungGCCount int64
	YoungGCTime  int64
	OldGCCount   int64
	OldGCTime    int64

	// GC management
	YoungGCManagedPools []string
	OldGCManagedPools   []string
	InvalidGCs          []string

	// Last GC details
	LastGC LastGCInfo
}

type Threading struct {
	// Basic thread counts
	Count             int64
	PeakCount         int64
	DaemonCount       int64
	TotalStartedCount int64

	// Current thread metrics
	CurrentCPUTime        int64
	CurrentUserTime       int64
	CurrentAllocatedBytes int64

	// Thread IDs and deadlock detection
	AllThreadIds             []int64
	DeadlockedThreads        []string
	MonitorDeadlockedThreads []string

	// Monitoring capabilities
	CpuTimeSupported              bool
	CpuTimeEnabled                bool
	AllocatedMemorySupported      bool
	AllocatedMemoryEnabled        bool
	ContentionMonitoringSupported bool
	ContentionMonitoringEnabled   bool
	ObjectMonitorUsageSupported   bool
	SynchronizerUsageSupported    bool
}

type ClassLoading struct {
	LoadedClassCount      int64
	TotalLoadedClassCount int64
	UnloadedClassCount    int64
	VerboseLogging        bool
}

type OperatingSystem struct {
	// System identification
	Name    string
	Version string
	Arch    string

	// CPU metrics
	AvailableProcessors int64
	SystemCpuLoad       float64
	ProcessCpuLoad      float64
	ProcessCpuTime      int64
	SystemLoadAverage   float64

	// Memory metrics
	TotalPhysicalMemory int64
	FreePhysicalMemory  int64
	TotalSwapSpace      int64
	FreeSwapSpace       int64

	// Process memory
	CommittedVirtualMemorySize int64

	// File descriptors (Unix/Linux only)
	OpenFileDescriptorCount int64
	MaxFileDescriptorCount  int64
}

type Runtime struct {
	// Process identification
	ProcessID   int64
	ProcessName string

	// JVM information
	VmName    string
	VmVendor  string
	VmVersion string

	// JVM specification
	SpecName    string
	SpecVendor  string
	SpecVersion string

	// Timing
	StartTime time.Time
	Uptime    time.Duration

	// Configuration
	InputArguments         []string
	SystemProperties       map[string]string
	ClassPath              string
	LibraryPath            string
	BootClassPath          string
	BootClassPathSupported bool
	ManagementSpecVersion  string

	// Commonly used system properties
	JavaVersion string
	JavaVendor  string
	JavaHome    string
	UserName    string
	UserHome    string
	UserDir     string
}

type MBeanSnapshot struct {
	Timestamp time.Time
	Connected bool
	Error     error

	// ===== MBean-organized metrics =====
	GC           GarbageCollector
	Memory       Memory
	Threading    Threading
	ClassLoading ClassLoading
	OS           OperatingSystem
	Runtime      Runtime
}
