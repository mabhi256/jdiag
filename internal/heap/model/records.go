package model

// Body of a HPROF_UTF8 record
type UTF8Body struct {
	StringID ID
	Text     string
}

// Body of a HPROF_LOAD_CLASS record
type LoadClassBody struct {
	ClassSerialNumber      SerialNum // u4
	ObjectID               ID
	StackTraceSerialNumber SerialNum
	ClassNameID            ID // It is a pointer to a UTF8 string
}

// Body of a HPROF_UNLOAD_CLASS record
type UnloadClassBody struct {
	ClassSerialNumber SerialNum
}

// Body of a HPROF_FRAME record
type FrameBody struct {
	StackFrameID      ID
	MethodNameID      ID // References UTF8
	MethodSignatureID ID // References UTF8
	SourceFileNameID  ID // References UTF8
	ClassSerialNumber SerialNum
	LineNumber        int32 // signed int because it can be negative
	// >0: normal line, -1: unknown, -2: compiled method, -3: native method
}

// Body of a HPROF_TRACE record
type TraceBody struct {
	StackTraceSerialNumber SerialNum
	ThreadSerialNumber     SerialNum // Thread that produced this trace
	NumFrames              uint32
	StackFrameIDs          []ID
}

// A single allocation site within HPROF_ALLOC_SITES
type AllocSite struct {
	IsArray uint8 // 0: normal object, 2: object array, 4: boolean array, 5: char array,
	// 6: float array, 7: double array, 8: byte array, 9: short array, 10 int array, 11: long array
	ClassSerialNumber      SerialNum
	StackTraceSerialNumber SerialNum
	BytesAlive             uint32
	InstancesAlive         uint32
	BytesAlloc             uint32
	InstancesAlloc         uint32
}

// Body of a HPROF_ALLOC_SITES record
type AllocSiteGroup struct {
	Flags               uint16
	CutoffRatio         uint32
	TotalLiveBytes      uint32
	TotalLiveInstances  uint32
	TotalBytesAlloc     uint64
	TotalInstancesAlloc uint64
	NumAllocSites       uint32
	Sites               []AllocSite
}

// AllocSiteGroup flag helpers
func (asg *AllocSiteGroup) IsIncremental() bool {
	return (asg.Flags & ALLOC_TYPE) != 0
}

func (asg *AllocSiteGroup) IsSortedByAllocation() bool {
	return (asg.Flags & ALLOC_SORT) != 0
}

func (asg *AllocSiteGroup) ForcedGC() bool {
	return (asg.Flags & ALLOC_GC) != 0
}

// Body of a HPROF_START_THREAD record
type StartThreadBody struct {
	ThreadSerialNumber      SerialNum
	ThreadObjectID          ID
	StackTraceSerialNumber  SerialNum
	ThreadNameID            ID // References UTF8
	ThreadGroupNameID       ID
	ParentThreadGroupNameID ID
}

// Body of a HPROF_END_THREAD record
type EndThreadBody struct {
	ThreadSerialNumber SerialNum
}

// Body of a HPROF_HEAP_SUMMARY record
type HeapSummary struct {
	LiveBytes      uint32
	LiveInstances  uint32
	BytesAlloc     uint64
	InstancesAlloc uint64
}

// Trace information within CPU_SAMPLES
type CPUTraceInfo struct {
	NumSamples             uint32
	StackTraceSerialNumber uint32 // References a HPROF_TRACE record
}

// Body of a HPROF_CPU_SAMPLES record
type CPUSample struct {
	TotalSamples uint32
	NumTraces    uint32
	Traces       []CPUTraceInfo
}

// Body of a HPROF_CONTROL_SETTINGS record
type ControlSettings struct {
	Flags           uint32 // Bit flags for various settings
	StackTraceDepth uint16
}

// ControlSettings flag helpers
func (cs *ControlSettings) IsAllocTracesEnabled() bool {
	return (cs.Flags & CONTROL_ALLOC_TRACES) != 0
}

func (cs *ControlSettings) IsCPUSamplingEnabled() bool {
	return (cs.Flags & CONTROL_CPU_SAMPLING) != 0
}
