package parser

import (
	"fmt"

	"github.com/mabhi256/jdiag/internal/heap/model"
	"github.com/mabhi256/jdiag/internal/heap/registry"
)

/*
* parseObjectArrayDump parses a HPROF_GC_OBJ_ARRAY_DUMP sub-record:
*
* OBJ_ARRAY_DUMP contains an object array with references to other objects:
*
* Format:
* 	id    						Array object ID
* 	u4    						Stack trace serial number
* 	u4    						Array length (number of elements)
* 	id    						Array class object ID
* 	[id]*                       Array elements (object references)
*
* Object arrays contain references to other heap objects. Each element
* is an object ID that may be 0 (null) or point to another object.
*
* Examples:
* - String[] arrays
* - Object[] arrays
* - Custom class arrays like Person[]
*
* The array class ID points to the array class definition, which will
* have a name like "[Ljava/lang/String;" for String arrays.
 */
func parseObjectArrayDump(reader *BinaryReader, arrayReg *registry.ArrayRegistry) error {
	array := &model.GCObjectArrayDump{}

	// Parse array header
	var err error
	array.ObjectID, err = reader.ReadID()
	if err != nil {
		return fmt.Errorf("failed to read array object ID: %w", err)
	}

	stackTraceSerial, err := reader.ReadU4()
	if err != nil {
		return fmt.Errorf("failed to read stack trace serial: %w", err)
	}
	array.StackTraceSerialNumber = model.SerialNum(stackTraceSerial)

	array.Size, err = reader.ReadU4()
	if err != nil {
		return fmt.Errorf("failed to read array length: %w", err)
	}

	array.ClassID, err = reader.ReadID()
	if err != nil {
		return fmt.Errorf("failed to read array class ID: %w", err)
	}

	// Read array elements (object references)
	array.Elements = make([]model.ID, array.Size)
	for i := uint32(0); i < array.Size; i++ {
		elementID, err := reader.ReadID()
		if err != nil {
			return fmt.Errorf("failed to read array element %d: %w", i, err)
		}
		array.Elements[i] = elementID
	}

	// Add to registry
	arrayReg.AddObjectArray(array)
	return nil
}

/*
* parsePrimitiveArrayDump parses a HPROF_GC_PRIM_ARRAY_DUMP sub-record:
*
* PRIM_ARRAY_DUMP contains a primitive array with actual data values:
*
* Format:
* 	id    						Array object ID
* 	u4    						Stack trace serial number
* 	u4    						Array length (number of elements)
* 	u1    						Element type (see HProfTagFieldType)
* 	[u1]*                       Array elements (primitive data)
*
* Primitive arrays contain actual primitive values packed in binary format.
* The element type determines how to interpret the data:
*
* - HPROF_BOOLEAN (4): 1 byte per element
* - HPROF_CHAR (5): 2 bytes per element (UTF-16)
* - HPROF_FLOAT (6): 4 bytes per element
* - HPROF_DOUBLE (7): 8 bytes per element
* - HPROF_BYTE (8): 1 byte per element
* - HPROF_SHORT (9): 2 bytes per element
* - HPROF_INT (10): 4 bytes per element
* - HPROF_LONG (11): 8 bytes per element
*
* Examples:
* - char[] arrays (used by String objects)
* - byte[] arrays (used by compact String objects in Java 9+)
* - int[] arrays for numeric data
*
* This is crucial for String resolution since String objects reference
* char[] or byte[] arrays containing the actual character data.
 */
func parsePrimitiveArrayDump(reader *BinaryReader, arrayReg *registry.ArrayRegistry) error {
	array := &model.GCPrimitiveArrayDump{}

	// Parse array header
	var err error
	array.ObjectID, err = reader.ReadID()
	if err != nil {
		return fmt.Errorf("failed to read array object ID: %w", err)
	}

	stackTraceSerial, err := reader.ReadU4()
	if err != nil {
		return fmt.Errorf("failed to read stack trace serial: %w", err)
	}
	array.StackTraceSerialNumber = model.SerialNum(stackTraceSerial)

	array.Size, err = reader.ReadU4()
	if err != nil {
		return fmt.Errorf("failed to read array length: %w", err)
	}

	elementTypeRaw, err := reader.ReadU1()
	if err != nil {
		return fmt.Errorf("failed to read element type: %w", err)
	}
	array.Type = model.HProfTagFieldType(elementTypeRaw)

	// Calculate element size
	elementSize := array.Type.Size(reader.Header().IdentifierSize)
	if elementSize == 0 {
		return fmt.Errorf("unknown primitive array element type: 0x%02x", elementTypeRaw)
	}

	// Read array elements as raw bytes
	totalSize := int(array.Size) * elementSize
	array.Elements = make([]byte, totalSize)
	err = reader.ReadBytes(array.Elements)
	if err != nil {
		return fmt.Errorf("failed to read array elements: %w", err)
	}

	// Add to registry
	arrayReg.AddPrimitiveArray(array)
	return nil
}
