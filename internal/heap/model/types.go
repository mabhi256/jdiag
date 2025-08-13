package model

import (
	"encoding/binary"
	"fmt"
	"time"
)

type ID uint64        // Represents a memory address, 4 or 8 bytes depending on platform
type SerialNum uint32 // u4, just a counter

type HProfTagRecord byte

const (
	// top-level records
	HPROF_UTF8             HProfTagRecord = 0x01
	HPROF_LOAD_CLASS                      = 0x02
	HPROF_UNLOAD_CLASS                    = 0x03
	HPROF_FRAME                           = 0x04
	HPROF_TRACE                           = 0x05
	HPROF_ALLOC_SITES                     = 0x06
	HPROF_HEAP_SUMMARY                    = 0x07
	HPROF_START_THREAD                    = 0x0A
	HPROF_END_THREAD                      = 0x0B
	HPROF_HEAP_DUMP                       = 0x0C
	HPROF_CPU_SAMPLES                     = 0x0D
	HPROF_CONTROL_SETTINGS                = 0x0E

	// 1.0.2 record types
	HPROF_HEAP_DUMP_SEGMENT = 0x1C
	HPROF_HEAP_DUMP_END     = 0x2C
)

func (h HProfTagRecord) String() string {
	switch h {
	case HPROF_UTF8:
		return "UTF8"
	case HPROF_LOAD_CLASS:
		return "LOAD_CLASS"
	case HPROF_UNLOAD_CLASS:
		return "UNLOAD_CLASS"
	case HPROF_FRAME:
		return "STACK_FRAME"
	case HPROF_TRACE:
		return "STACK_TRACE"
	case HPROF_ALLOC_SITES:
		return "ALLOC_SITES"
	case HPROF_HEAP_SUMMARY:
		return "HEAP_SUMMARY"
	case HPROF_START_THREAD:
		return "START_THREAD"
	case HPROF_END_THREAD:
		return "END_THREAD"
	case HPROF_HEAP_DUMP:
		return "HEAP_DUMP"
	case HPROF_CPU_SAMPLES:
		return "CPU_SAMPLES"
	case HPROF_CONTROL_SETTINGS:
		return "CONTROL_SETTINGS"
	case HPROF_HEAP_DUMP_SEGMENT:
		return "HEAP_DUMP_SEGMENT"
	case HPROF_HEAP_DUMP_END:
		return "HEAP_DUMP_END"
	default:
		return fmt.Sprintf("HProfTagRecord(0x%02X)", byte(h))
	}
}

type HProfTagFieldType byte

const (
	HPROF_ARRAY_OBJECT  HProfTagFieldType = 0x01
	HPROF_NORMAL_OBJECT                   = 0x02
	HPROF_BOOLEAN                         = 0x04
	HPROF_CHAR                            = 0x05
	HPROF_FLOAT                           = 0x06
	HPROF_DOUBLE                          = 0x07
	HPROF_BYTE                            = 0x08
	HPROF_SHORT                           = 0x09
	HPROF_INT                             = 0x0A
	HPROF_LONG                            = 0x0B
)

func (ft HProfTagFieldType) Size(identifierSize uint32) int {
	switch ft {
	case HPROF_BOOLEAN, HPROF_BYTE:
		return 1
	case HPROF_CHAR, HPROF_SHORT:
		return 2
	case HPROF_INT, HPROF_FLOAT:
		return 4
	case HPROF_LONG, HPROF_DOUBLE:
		return 8
	case HPROF_NORMAL_OBJECT, HPROF_ARRAY_OBJECT:
		return int(identifierSize)
	default:
		return 0
	}
}

type HProfTagSubRecord byte

const (
	HPROF_GC_ROOT_UNKNOWN      HProfTagSubRecord = 0xFF
	HPROF_GC_ROOT_JNI_GLOBAL                     = 0x01
	HPROF_GC_ROOT_JNI_LOCAL                      = 0x02
	HPROF_GC_ROOT_JAVA_FRAME                     = 0x03
	HPROF_GC_ROOT_NATIVE_STACK                   = 0x04
	HPROF_GC_ROOT_STICKY_CLASS                   = 0x05
	HPROF_GC_ROOT_THREAD_BLOCK                   = 0x06
	HPROF_GC_ROOT_MONITOR_USED                   = 0x07
	HPROF_GC_ROOT_THREAD_OBJ                     = 0x08
	HPROF_GC_CLASS_DUMP                          = 0x20
	HPROF_GC_INSTANCE_DUMP                       = 0x21
	HPROF_GC_OBJ_ARRAY_DUMP                      = 0x22
	HPROF_GC_PRIM_ARRAY_DUMP                     = 0x23
)

func ReadID(data []byte, offset int, identifierSize uint32) (ID, int) {
	if identifierSize == 4 {
		return ID(binary.BigEndian.Uint32(data[offset:])), offset + 4
	} else {
		return ID(binary.BigEndian.Uint64(data[offset:])), offset + 8
	}
}

const (
	EmptyFrame int32 = -1
)

// Flag constants for various record types
const (
	ALLOC_TYPE = 0x0001 // incremental vs complete
	ALLOC_SORT = 0x0002 // sorted by allocation vs live
	ALLOC_GC   = 0x0004 // force GC

	CONTROL_ALLOC_TRACES = 0x00000001 // Allocation traces on/off
	CONTROL_CPU_SAMPLING = 0x00000002 // CPU sampling on/off
)

type HprofHeader struct {
	Format         string    // Typically "JAVA PROFILE 1.0.2"
	IdentifierSize uint32    // u4 size of object IDs
	Timestamp      time.Time // u4 + u4, stored as uint64, milliseconds since 0:00 GMT, 1/1/70
}

type HprofRecord struct {
	Type       HProfTagRecord // u1 tag
	TimeOffset uint32         // u4 - microseconds since header timestamp
	Length     uint32         // u4 bytes remaining (excludes tag+length)
	Data       []byte         // variable length body
}
