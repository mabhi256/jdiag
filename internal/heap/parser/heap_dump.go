package parser

import (
	"fmt"

	"github.com/mabhi256/jdiag/internal/heap/model"
	"github.com/mabhi256/jdiag/internal/heap/registry"
)

/*
* ParseHeapDumpSegment parses a HPROF_HEAP_DUMP_SEGMENT record:
*
* HEAP_DUMP_SEGMENT contains a series of sub-records that describe the heap state:
* 	[sub-record]*		A sequence of heap dump sub-records
*
* Each sub-record has this format:
* 	u1    				Sub-record tag (see HProfTagSubRecord)
* 	[data]				Sub-record specific data (variable length)
*
* Sub-record types:
* - GC_ROOT_*: Various types of garbage collection roots
* - GC_CLASS_DUMP: Complete class definition with fields and methods
* - GC_INSTANCE_DUMP: Object instance with field values
* - GC_OBJ_ARRAY_DUMP: Object array with references
* - GC_PRIM_ARRAY_DUMP: Primitive array with values
*
* The segment ends when we've consumed exactly 'length' bytes.
 */
func ParseHeapDumpSegment(reader *BinaryReader, length uint32,
	rootReg *registry.GCRootRegistry, classDumpReg *registry.ClassDumpRegistry,
) (int, map[model.HProfTagSubRecord]int, error) {
	if length == 0 {
		return 0, make(map[model.HProfTagSubRecord]int), nil
	}

	segmentStart := reader.BytesRead()
	segmentEnd := segmentStart + int64(length)
	subRecordCount := 0
	subRecordCountMap := make(map[model.HProfTagSubRecord]int)

	// Parse sub-records until we reach the segment end
	for reader.BytesRead() < segmentEnd {
		beforeSubRecord := reader.BytesRead()
		remaining := segmentEnd - beforeSubRecord

		if remaining <= 0 {
			break
		}

		subRecordRaw, err := reader.ReadU1()
		if err != nil {
			return 0, nil, fmt.Errorf("failed to read sub-record type at offset %d (remaining: %d): %w",
				beforeSubRecord, remaining, err)
		}

		subRecordType := model.HProfTagSubRecord(subRecordRaw)

		subRecordCount++
		subRecordCountMap[subRecordType]++

		// Parse or skip the specific sub-record type
		err = parseSubRecord(reader, subRecordType, rootReg, classDumpReg)
		if err != nil {
			return 0, nil, fmt.Errorf("failed to parse sub-record %s at offset %d: %w",
				subRecordType, beforeSubRecord, err)
		}

		afterSubRecord := reader.BytesRead()
		bytesConsumed := afterSubRecord - beforeSubRecord

		// Verify we haven't exceeded segment boundary
		if afterSubRecord > segmentEnd {
			return 0, nil, fmt.Errorf("sub-record %s exceeded segment boundary: at %d, segment ends at %d",
				subRecordType, afterSubRecord, segmentEnd)
		}

		// Safety check for infinite loops
		if bytesConsumed <= 0 {
			return 0, nil, fmt.Errorf("no progress made parsing sub-record %s at offset %d",
				subRecordType, beforeSubRecord)
		}
	}

	finalPosition := reader.BytesRead()

	// Handle remaining bytes
	remaining := segmentEnd - finalPosition
	if remaining > 0 {
		// fmt.Printf("  [DEBUG] Skipping %d remaining bytes\n", remaining)
		err := reader.Skip(int(remaining))
		if err != nil {
			return 0, nil, fmt.Errorf("failed to skip remaining %d bytes: %w", remaining, err)
		}
	} else if remaining < 0 {
		return 0, nil, fmt.Errorf("read %d bytes beyond segment boundary", -remaining)
	}

	return subRecordCount, subRecordCountMap, nil
}

/*
ParseHeapDumpEnd parses a HPROF_HEAP_DUMP_END record:

HEAP_DUMP_END marks the end of a heap dump sequence.
It has no body - just the record header.
*/
func ParseHeapDumpEnd(length uint32) error {
	if length != 0 {
		return fmt.Errorf("HEAP_DUMP_END should have zero length, got %d", length)
	}
	return nil
}

// parseSubRecord parses a specific heap dump sub-record
func parseSubRecord(reader *BinaryReader, subRecordType model.HProfTagSubRecord,
	rootReg *registry.GCRootRegistry, classDumpReg *registry.ClassDumpRegistry,
) error {
	startPos := reader.BytesRead() - 1

	switch subRecordType {
	case model.HPROF_GC_ROOT_UNKNOWN:
		return parseGCRootUnknown(reader, rootReg)
	case model.HPROF_GC_ROOT_JNI_GLOBAL:
		return parseGCRootJniGlobal(reader, rootReg)
	case model.HPROF_GC_ROOT_JNI_LOCAL:
		return parseGCRootJniLocal(reader, rootReg)
	case model.HPROF_GC_ROOT_JAVA_FRAME:
		return parseGCRootJavaFrame(reader, rootReg)
	case model.HPROF_GC_ROOT_NATIVE_STACK:
		return parseGCRootNativeStack(reader, rootReg)
	case model.HPROF_GC_ROOT_STICKY_CLASS:
		return parseGCRootStickyClass(reader, rootReg)
	case model.HPROF_GC_ROOT_THREAD_BLOCK:
		return parseGCRootThreadBlock(reader, rootReg)
	case model.HPROF_GC_ROOT_MONITOR_USED:
		return parseGCRootMonitorUsed(reader, rootReg)
	case model.HPROF_GC_ROOT_THREAD_OBJ:
		return parseGCRootThreadObj(reader, rootReg)
	case model.HPROF_GC_CLASS_DUMP:
		return parseClassDump(reader, classDumpReg)
	case model.HPROF_GC_INSTANCE_DUMP:
		return skipInstanceDump(reader)
	case model.HPROF_GC_OBJ_ARRAY_DUMP:
		return skipObjectArrayDump(reader)
	case model.HPROF_GC_PRIM_ARRAY_DUMP:
		return skipPrimitiveArrayDump(reader)

	default:
		return fmt.Errorf("unknown sub-record type: 0x%02x at offset %d", subRecordType, startPos)
	}
}

// skipInstanceDump skips an INSTANCE_DUMP sub-record
func skipInstanceDump(reader *BinaryReader) error {
	// INSTANCE_DUMP format:
	// id    object_id
	// u4    stack_trace_serial_number
	// id    class_object_id
	// u4    instance_data_size
	// [u1]* instance_data

	// Skip header (2 IDs + 2 u4s)

	// Skip object_id (ID)
	if err := reader.Skip(int(reader.Header().IdentifierSize)); err != nil {
		return fmt.Errorf("failed to skip object_id: %w", err)
	}

	// Skip stack_trace_serial_number (u4)
	if err := reader.Skip(4); err != nil {
		return fmt.Errorf("failed to skip stack_trace_serial_number: %w", err)
	}

	// Skip class_object_id (ID)
	if err := reader.Skip(int(reader.Header().IdentifierSize)); err != nil {
		return fmt.Errorf("failed to skip class_object_id: %w", err)
	}

	// Read instance_data_size (u4)
	dataSize, err := reader.ReadU4()
	if err != nil {
		return fmt.Errorf("failed to read instance data size: %w", err)
	}

	// Skip instance data
	if err := reader.Skip(int(dataSize)); err != nil {
		return fmt.Errorf("failed to skip instance data: %w", err)
	}

	return nil
}

// skipObjectArrayDump skips an OBJ_ARRAY_DUMP sub-record
func skipObjectArrayDump(reader *BinaryReader) error {
	// OBJ_ARRAY_DUMP format:
	// id    array_object_id
	// u4    stack_trace_serial_number
	// u4    array_length
	// id    array_class_id
	// [id]* elements

	// Skip header (2 IDs + 2 u4s)

	// Skip array_object_id (ID)
	if err := reader.Skip(int(reader.Header().IdentifierSize)); err != nil {
		return fmt.Errorf("failed to skip array_object_id: %w", err)
	}

	// Skip stack_trace_serial_number (u4)
	if err := reader.Skip(4); err != nil {
		return fmt.Errorf("failed to skip stack_trace_serial_number: %w", err)
	}

	// Read array_length (u4)
	arrayLength, err := reader.ReadU4()
	if err != nil {
		return fmt.Errorf("failed to read array length: %w", err)
	}

	// Skip array_class_id (ID)
	if err := reader.Skip(int(reader.Header().IdentifierSize)); err != nil {
		return fmt.Errorf("failed to skip array_class_id: %w", err)
	}

	// fmt.Printf("        [DEBUG] Object array length: %d elements\n", arrayLength)

	// Skip array elements (each is an ID)
	elementsSize := int(arrayLength) * int(reader.Header().IdentifierSize)
	if err := reader.Skip(elementsSize); err != nil {
		return fmt.Errorf("failed to skip array elements: %w", err)
	}

	return nil
}

// skipPrimitiveArrayDump skips a PRIM_ARRAY_DUMP sub-record
func skipPrimitiveArrayDump(reader *BinaryReader) error {
	// PRIM_ARRAY_DUMP format:
	// id    array_object_id
	// u4    stack_trace_serial_number
	// u4    array_length
	// u1    element_type
	// [u1]* elements

	// Skip header (1 ID + 2 u4s)

	// Format: id array_object_id, u4 stack_trace_serial_number, u4 array_length, u1 element_type, [u1]* elements

	// Skip array_object_id (ID)
	if err := reader.Skip(int(reader.Header().IdentifierSize)); err != nil {
		return fmt.Errorf("failed to skip array_object_id: %w", err)
	}

	// Skip stack_trace_serial_number (u4)
	if err := reader.Skip(4); err != nil {
		return fmt.Errorf("failed to skip stack_trace_serial_number: %w", err)
	}

	// Read array_length (u4)
	arrayLength, err := reader.ReadU4()
	if err != nil {
		return fmt.Errorf("failed to read array length: %w", err)
	}

	// Read element_type (u1)
	elementType, err := reader.ReadU1()
	if err != nil {
		return fmt.Errorf("failed to read element type: %w", err)
	}

	// Calculate element size
	elementSize := model.HProfTagFieldType(elementType).Size(reader.Header().IdentifierSize)
	if elementSize == 0 {
		return fmt.Errorf("unknown element type: 0x%02x", elementType)
	}

	// Skip array elements
	elementsSize := int(arrayLength) * elementSize
	if err := reader.Skip(elementsSize); err != nil {
		return fmt.Errorf("failed to skip array elements: %w", err)
	}

	return nil
}
