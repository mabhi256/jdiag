package parser

import (
	"encoding/binary"
	"fmt"
	"strings"

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

/*
* resolveStringFromArrays resolves a String object's actual content using array data
*
* String objects in Java heap dumps reference char[] or byte[] arrays containing
* the actual character data. This function:
*
* 1. Extracts the String object's fields (value array reference, coder, etc.)
* 2. Looks up the referenced array in the array registry
* 3. Converts the array data to a readable string
*
* Handles both legacy (char[]) and modern compact strings (byte[]):
* - Java 8 and earlier: String.value is char[]
* - Java 9+: String.value is byte[], with coder field indicating encoding
*   - coder = 0: Latin1 (1 byte per character)
*   - coder = 1: UTF16 (2 bytes per character)
*
* NOTE: This function requires all objects and arrays to be parsed first,
* so it should be called during post-processing, not during initial parsing.
 */
func resolveStringFromArrays(stringObjectID model.ID, objectReg *registry.InstanceRegistry,
	classDumpReg *registry.ClassDumpRegistry, stringReg *registry.StringRegistry,
	arrayReg *registry.ArrayRegistry, identifierSize uint32) string {

	if stringObjectID == 0 {
		return ""
	}

	// Get the String object instance
	stringInstance, exists := objectReg.GetInstance(stringObjectID)
	if !exists {
		return fmt.Sprintf("<string object not found: 0x%x>", uint64(stringObjectID))
	}

	// Get the String class definition
	stringClassDump, hasClass := classDumpReg.GetClassDump(stringInstance.ClassObjectID)
	if !hasClass {
		return fmt.Sprintf("<string class not found: 0x%x>", uint64(stringInstance.ClassObjectID))
	}

	// Extract String object fields - use the function from instance parser
	// Note: This requires the extractFieldValues function to be exported/accessible
	// For now, we'll implement a simplified version here to avoid circular dependencies

	// Look for the value array field and coder field in the raw instance data
	// This is a simplified approach - in a full implementation, we'd properly parse all fields
	valueArrayID := extractStringValueArrayID(stringInstance, stringClassDump, stringReg, identifierSize)
	if valueArrayID == 0 {
		return fmt.Sprintf("<no value array found in string 0x%x>", uint64(stringObjectID))
	}

	// Try to get the array from array registry
	// First try as char array (traditional strings)
	if stringValue, found := arrayReg.GetCharArray(valueArrayID); found {
		return stringValue
	}

	// Then try as byte array (compact strings)
	if stringValue, found := arrayReg.GetByteArray(valueArrayID); found {
		return stringValue
	}

	return fmt.Sprintf("<array not found: 0x%x>", uint64(valueArrayID))
}

// extractStringValueArrayID extracts the value array ID from a String object
// This is a simplified version that looks for the first object reference field
// A full implementation would properly parse all fields by name
func extractStringValueArrayID(stringInstance *model.GCInstanceDump,
	stringClassDump *model.ClassDump, stringReg *registry.StringRegistry,
	identifierSize uint32) model.ID {

	data := stringInstance.InstanceData
	offset := 0

	// Walk through instance fields looking for object references
	for _, field := range stringClassDump.InstanceFields {
		if offset >= len(data) {
			break
		}

		fieldName := strings.ToLower(stringReg.GetOrUnresolved(field.NameID))
		fieldSize := field.Type.Size(identifierSize)

		// If this is the "value" field and it's an object reference
		if fieldName == "value" && (field.Type == model.HPROF_NORMAL_OBJECT || field.Type == model.HPROF_ARRAY_OBJECT) {
			if fieldSize == 4 {
				return model.ID(binary.BigEndian.Uint32(data[offset:]))
			} else if fieldSize == 8 {
				return model.ID(binary.BigEndian.Uint64(data[offset:]))
			}
		}

		offset += fieldSize
	}

	return 0
}
