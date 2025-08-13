package model

// HPROF_GC_ROOT_UNKNOWN
type GCRootUnknown struct {
	ObjectID ID
}

// HPROF_GC_ROOT_JNI_GLOBAL
type GCRootJniGlobal struct {
	ObjectID       ID
	JniGlobalRefID ID
}

// HPROF_GC_ROOT_JNI_LOCAL
type GCRootJniLocal struct {
	ObjectID           ID
	ThreadSerialNumber SerialNum
	FrameNumber        SerialNum
}

// HPROF_GC_ROOT_JAVA_FRAME
type GCRootJavaFrame struct {
	ObjectID           ID
	ThreadSerialNumber SerialNum
	FrameNumber        SerialNum
}

// HPROF_GC_ROOT_NATIVE_STACK
type GCRootNativeStack struct {
	ObjectID           ID
	ThreadSerialNumber SerialNum
}

// HPROF_GC_ROOT_STICKY_CLASS
type GCRootStickyClass struct {
	ObjectID ID
}

// HPROF_GC_ROOT_THREAD_BLOCK
type GCRootThreadBlock struct {
	ObjectID           ID
	ThreadSerialNumber SerialNum
}

// HPROF_GC_ROOT_MONITOR_USED
type GCRootMonitorUsed struct {
	ObjectID ID
}

// HPROF_GC_ROOT_THREAD_OBJ
type GCRootThreadObject struct {
	ThreadObjectID         ID
	ThreadSerialNumber     SerialNum
	StackTraceSerialNumber SerialNum
}

// HPROF_GC_CLASS_DUMP
type ClassDump struct {
	ClassObjectID            ID
	StackTraceSerialNumber   SerialNum
	SuperClassObjectID       ID
	ClassLoaderObjectID      ID
	SignerObjectID           ID
	ProtectionDomainObjectID ID
	Reserved1                ID
	Reserved2                ID
	InstanceSize             uint32
	ConstantPoolSize         uint16
	ConstantPool             []*ConstantPoolEntry
	StaticFieldsCount        uint16
	StaticFields             []*StaticField
	InstanceFieldsCount      uint16
	InstanceFields           []*InstanceField
}

type ConstantPoolEntry struct {
	Index uint16
	Type  HProfTagFieldType
	Value []byte
}

type StaticField struct {
	NameID ID
	Type   HProfTagFieldType
	Value  []byte
}

type InstanceField struct {
	NameID ID
	Type   HProfTagFieldType
}

// HPROF_GC_INSTANCE_DUMP
type GCInstanceDump struct {
	ObjectID               ID
	StackTraceSerialNumber SerialNum
	ClassObjectID          ID
	Size                   uint32
	InstanceData           []byte
}

// HPROF_GC_OBJ_ARRAY_DUMP
type GCObjectArrayDump struct {
	ObjectID               ID
	StackTraceSerialNumber SerialNum
	Size                   uint32
	ClassID                ID
	Elements               []ID
}

// HPROF_GC_PRIM_ARRAY_DUMP
type GCPrimitiveArrayDump struct {
	ObjectID               ID
	StackTraceSerialNumber SerialNum
	Size                   uint32
	Type                   HProfTagFieldType // Other than HPROF_ARRAY_OBJECT and HPROF_NORMAL_OBJECT
	Elements               []byte
}
