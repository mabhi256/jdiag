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
func ParseHeapDumpSegment(reader *BinaryReader, length uint32, rootReg *registry.GCRootRegistry) (int, map[model.HProfTagSubRecord]int, error) {
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

		subRecordType, err := reader.ReadU1()
		if err != nil {
			return 0, nil, fmt.Errorf("failed to read sub-record type at offset %d (remaining: %d): %w",
				beforeSubRecord, remaining, err)
		}

		subRecordCount++
		subRecordCountMap[model.HProfTagSubRecord(subRecordType)]++

		// Parse or skip the specific sub-record type
		err = parseSubRecord(reader, model.HProfTagSubRecord(subRecordType), rootReg)
		if err != nil {
			return 0, nil, fmt.Errorf("failed to parse sub-record %s at offset %d: %w",
				model.HProfTagSubRecord(subRecordType), beforeSubRecord, err)
		}

		afterSubRecord := reader.BytesRead()
		bytesConsumed := afterSubRecord - beforeSubRecord

		// Verify we haven't exceeded segment boundary
		if afterSubRecord > segmentEnd {
			return 0, nil, fmt.Errorf("sub-record %s exceeded segment boundary: at %d, segment ends at %d",
				model.HProfTagSubRecord(subRecordType), afterSubRecord, segmentEnd)
		}

		// Safety check for infinite loops
		if bytesConsumed <= 0 {
			return 0, nil, fmt.Errorf("no progress made parsing sub-record %s at offset %d",
				model.HProfTagSubRecord(subRecordType), beforeSubRecord)
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

// parseSubRecord parses a specific heap dump sub-record (Phase 7: GC roots implemented)
func parseSubRecord(reader *BinaryReader, subRecordType model.HProfTagSubRecord, rootReg *registry.GCRootRegistry) error {
	startPos := reader.BytesRead() - 1 // -1 because we already read the type byte

	switch subRecordType {
	// GC Root records - fixed size, already working
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

	// Complex records - corrected skipping logic
	case model.HPROF_GC_CLASS_DUMP:
		return skipClassDump(reader, startPos)
	case model.HPROF_GC_INSTANCE_DUMP:
		return skipInstanceDump(reader, startPos)
	case model.HPROF_GC_OBJ_ARRAY_DUMP:
		return skipObjectArrayDump(reader, startPos)
	case model.HPROF_GC_PRIM_ARRAY_DUMP:
		return skipPrimitiveArrayDump(reader, startPos)

	default:
		return fmt.Errorf("unknown sub-record type: 0x%02x at offset %d", subRecordType, startPos)
	}
}

// skipClassDump skips a CLASS_DUMP sub-record by parsing its structure
func skipClassDump(reader *BinaryReader, startPos int64) error {
	// CLASS_DUMP format:
	// id    class_object_id
	// u4    stack_trace_serial_number
	// id    super_class_object_id
	// id    class_loader_object_id
	// id    signers_object_id
	// id    protection_domain_object_id
	// id    reserved
	// id    reserved
	// u4    instance_size
	// u2    constant_pool_size
	// [constant_pool_entry]*
	// u2    static_fields_count
	// [static_field]*
	// u2    instance_fields_count
	// [instance_field]*

	// Skip header (8 IDs + 1 u4)

	// Skip class_object_id (ID)
	if err := reader.Skip(int(reader.Header().IdentifierSize)); err != nil {
		return fmt.Errorf("failed to skip class_object_id: %w", err)
	}

	// Skip stack_trace_serial_number (u4)
	if err := reader.Skip(4); err != nil {
		return fmt.Errorf("failed to skip stack_trace_serial_number: %w", err)
	}

	// Skip remaining header fields (6 more IDs + instance_size)
	remainingHeaderSize := 6*int(reader.Header().IdentifierSize) + 4
	if err := reader.Skip(remainingHeaderSize); err != nil {
		return fmt.Errorf("failed to skip remaining header: %w", err)
	}

	// Read and skip constant pool
	constantPoolSize, err := reader.ReadU2()
	if err != nil {
		return fmt.Errorf("failed to read constant pool size: %w", err)
	}

	for i := uint16(0); i < constantPoolSize; i++ {
		// beforeEntry := reader.BytesRead()

		// Skip constant pool index (u2)
		if err := reader.Skip(2); err != nil {
			return fmt.Errorf("failed to skip CP entry %d index: %w", i, err)
		}

		// Read value type (u1)
		valueType, err := reader.ReadU1()
		if err != nil {
			return fmt.Errorf("failed to read CP entry %d type: %w", i, err)
		}

		// Skip value based on type
		valueSize := model.HProfTagFieldType(valueType).Size(reader.Header().IdentifierSize)
		if valueSize == 0 {
			return fmt.Errorf("unknown constant pool value type: 0x%02x at entry %d", valueType, i)
		}

		if err := reader.Skip(valueSize); err != nil {
			return fmt.Errorf("failed to skip CP entry %d value: %w", i, err)
		}
	}

	// Read and skip static fields
	staticFieldsCount, err := reader.ReadU2()
	if err != nil {
		return fmt.Errorf("failed to read static fields count: %w", err)
	}

	for i := uint16(0); i < staticFieldsCount; i++ {
		beforeField := reader.BytesRead()

		// Skip name ID
		if err := reader.Skip(int(reader.Header().IdentifierSize)); err != nil {
			return fmt.Errorf("failed to skip static field %d name ID: %w", i, err)
		}

		// Read field type
		fieldType, err := reader.ReadU1()
		if err != nil {
			return fmt.Errorf("failed to read static field %d type: %w", i, err)
		}

		// Check field type validity
		fieldSize := model.HProfTagFieldType(fieldType).Size(reader.Header().IdentifierSize)
		if fieldSize == 0 {
			return fmt.Errorf("unknown static field type: 0x%02x at field %d (offset %d)", fieldType, i, beforeField)
		}

		// Skip field value
		if err := reader.Skip(fieldSize); err != nil {
			return fmt.Errorf("failed to skip static field %d value: %w", i, err)
		}
	}

	// Read and skip instance fields
	instanceFieldsCount, err := reader.ReadU2()
	if err != nil {
		return fmt.Errorf("failed to read instance fields count: %w", err)
	}

	for i := uint16(0); i < instanceFieldsCount; i++ {
		// Skip name ID
		if err := reader.Skip(int(reader.Header().IdentifierSize)); err != nil {
			return fmt.Errorf("failed to skip instance field %d name ID: %w", i, err)
		}

		// Read field type (but don't skip value - instance fields don't store values)
		_, err := reader.ReadU1()
		if err != nil {
			return fmt.Errorf("failed to read instance field %d type: %w", i, err)
		}
	}

	return nil
}

// skipInstanceDump skips an INSTANCE_DUMP sub-record
func skipInstanceDump(reader *BinaryReader, startPos int64) error {
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
func skipObjectArrayDump(reader *BinaryReader, startPos int64) error {
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
func skipPrimitiveArrayDump(reader *BinaryReader, startPos int64) error {
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
