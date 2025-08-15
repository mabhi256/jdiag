package parser

import (
	"encoding/binary"
	"fmt"
	"strings"

	"github.com/mabhi256/jdiag/internal/heap/model"
	"github.com/mabhi256/jdiag/internal/heap/registry"
)

/*
* parseInstanceDump parses a HPROF_GC_INSTANCE_DUMP sub-record:
*
* INSTANCE_DUMP contains an object instance with its field values:
*
* Format:
* 	id    						Object ID
* 	u4    						Stack trace serial number
* 	id    						Class object ID
* 	u4    						Instance data size in bytes
* 	[u1]*                       Instance field data (raw bytes)
*
* To extract field values, we need to:
* 1. Look up the class definition using the class object ID
* 2. Get the instance field definitions from the class dump
* 3. Parse the instance data according to field types and order
* 4. Handle inheritance by walking up the class hierarchy
*
* Special handling for java.lang.Thread objects:
* - Extract thread-specific fields (name, priority, daemon, etc.)
* - Handle both legacy and modern Java Thread structures
* - Store parsed Thread data for later analysis
 */
func parseInstanceDump(reader *BinaryReader, objectReg *registry.ObjectRegistry,
	classDumpReg *registry.ClassDumpRegistry, stringReg *registry.StringRegistry,
	arrayReg *registry.ArrayRegistry) error {

	instance := &model.GCInstanceDump{}

	// Parse instance header
	var err error
	instance.ObjectID, err = reader.ReadID()
	if err != nil {
		return fmt.Errorf("failed to read object ID: %w", err)
	}

	stackTraceSerial, err := reader.ReadU4()
	if err != nil {
		return fmt.Errorf("failed to read stack trace serial: %w", err)
	}
	instance.StackTraceSerialNumber = model.SerialNum(stackTraceSerial)

	instance.ClassObjectID, err = reader.ReadID()
	if err != nil {
		return fmt.Errorf("failed to read class object ID: %w", err)
	}

	instance.Size, err = reader.ReadU4()
	if err != nil {
		return fmt.Errorf("failed to read instance data size: %w", err)
	}

	// Read instance data
	instance.InstanceData = make([]byte, instance.Size)
	err = reader.ReadBytes(instance.InstanceData)
	if err != nil {
		return fmt.Errorf("failed to read instance data: %w", err)
	}

	// Check if this is a Thread object and handle specially
	if isThreadObject(instance, classDumpReg, stringReg) {
		threadData := parseThreadInstance(instance, classDumpReg, stringReg, objectReg, arrayReg, reader.Header().IdentifierSize)
		objectReg.AddThreadInstance(threadData)
	} else {
		// Add regular instance to registry
		objectReg.AddInstance(instance)
	}

	return nil
}

// isThreadObject checks if an instance is a Thread object by examining its class
func isThreadObject(instance *model.GCInstanceDump, classDumpReg *registry.ClassDumpRegistry, stringReg *registry.StringRegistry) bool {
	classDump, hasClass := classDumpReg.GetClassDump(instance.ClassObjectID)
	if !hasClass {
		return false
	}

	return hasThreadFields(classDump, stringReg)
}

// hasThreadFields checks if a class has typical Thread fields
func hasThreadFields(classDump *model.ClassDump, stringReg *registry.StringRegistry) bool {
	// Look for Thread-specific field names
	threadFieldNames := []string{"tid", "name", "eetop", "interrupted"}
	foundFields := 0

	for _, field := range classDump.InstanceFields {
		fieldName := strings.ToLower(stringReg.GetOrUnresolved(field.NameID))
		for _, threadField := range threadFieldNames {
			if fieldName == threadField {
				foundFields++
				break
			}
		}
	}

	// Consider it a Thread if we find at least 3 of the key Thread fields
	return foundFields >= 3
}

// parseThreadInstance parses a Thread object instance and extracts thread-specific data
func parseThreadInstance(instance *model.GCInstanceDump, classDumpReg *registry.ClassDumpRegistry,
	stringReg *registry.StringRegistry, objectReg *registry.ObjectRegistry,
	arrayReg *registry.ArrayRegistry, identifierSize uint32) *registry.ThreadInstanceData { // Phase 10: Add array registry

	threadData := &registry.ThreadInstanceData{
		ObjectID:     instance.ObjectID,
		InstanceDump: instance,
		Name:         fmt.Sprintf("Thread@0x%x", uint64(instance.ObjectID)), // Default name, will be updated with TID
		Priority:     5,                                                     // Default priority
		Daemon:       false,                                                 // Default daemon status
		ThreadID:     0,                                                     // Will be extracted from tid field
	}

	// Extract field values from instance data
	classDump, hasClass := classDumpReg.GetClassDump(instance.ClassObjectID)
	if !hasClass {
		return threadData
	}

	fieldValues, err := extractFieldValues(instance, classDump, classDumpReg, stringReg, identifierSize)
	if err != nil {
		// Return default thread data if field extraction fails
		return threadData
	}

	threadData.ParsedFieldValues = fieldValues

	// Extract thread fields from the main Thread object
	extractThreadFields(threadData, fieldValues, stringReg, arrayReg)

	// Check for holder object (Java 19+ virtual threads)
	if holderID, hasHolder := fieldValues["holder"]; hasHolder {
		if holderObjID, ok := holderID.(model.ID); ok && holderObjID != 0 {
			extractHolderFields(threadData, holderObjID, objectReg, classDumpReg, stringReg, arrayReg, identifierSize)
		}
	}

	// NOTE: Full string resolution will be implemented in post-processing
	// since arrays and String objects may be parsed in any order

	return threadData
}

// extractThreadFields extracts basic thread fields from field values
func extractThreadFields(threadData *registry.ThreadInstanceData, fieldValues map[string]interface{},
	stringReg *registry.StringRegistry, arrayReg *registry.ArrayRegistry) { // Phase 10: Add array registry
	for fieldName, value := range fieldValues {
		lowerFieldName := strings.ToLower(fieldName)

		switch {
		case lowerFieldName == "tid":
			if tid, ok := value.(int64); ok {
				threadData.ThreadID = tid
				// Update name to include TID
				threadData.Name = fmt.Sprintf("Thread-%d", tid)
			}
		case lowerFieldName == "name":
			if nameID, ok := value.(model.ID); ok && nameID != 0 {
				// Phase 10: Now we can actually resolve String objects!
				// Note: We need to defer this until after all objects are parsed
				// For now, store the name ID and show a placeholder
				if threadData.ThreadID != 0 {
					threadData.Name = fmt.Sprintf("Thread-%d@String[0x%x]", threadData.ThreadID, uint64(nameID))
				} else {
					threadData.Name = fmt.Sprintf("Thread@String[0x%x]", uint64(nameID))
				}
				// TODO: Add post-processing step to resolve actual string content
			}
		case lowerFieldName == "priority":
			if priority, ok := value.(int32); ok {
				threadData.Priority = priority
			} else if priority, ok := value.(int8); ok {
				threadData.Priority = int32(priority)
			}
		case lowerFieldName == "daemon":
			if daemon, ok := value.(bool); ok {
				threadData.Daemon = daemon
			}
		case lowerFieldName == "threadstatus" || strings.Contains(lowerFieldName, "status"):
			if status, ok := value.(int32); ok {
				threadData.ThreadStatus = status
			}
		case strings.Contains(lowerFieldName, "group"):
			if groupID, ok := value.(model.ID); ok {
				threadData.ThreadGroup = groupID
			}
		}
	}
}

// extractHolderFields extracts fields from holder object (modern Java virtual threads)
func extractHolderFields(threadData *registry.ThreadInstanceData, holderObjID model.ID,
	objectReg *registry.ObjectRegistry, classDumpReg *registry.ClassDumpRegistry,
	stringReg *registry.StringRegistry, arrayReg *registry.ArrayRegistry, identifierSize uint32) { // Phase 10: Add array registry

	// Get the holder object
	holderInstance, exists := objectReg.GetInstance(holderObjID)
	if !exists {
		return
	}

	// Get holder class
	holderClass, hasClass := classDumpReg.GetClassDump(holderInstance.ClassObjectID)
	if !hasClass {
		return
	}

	// Extract holder fields
	holderFields, err := extractFieldValues(holderInstance, holderClass, classDumpReg, stringReg, identifierSize)
	if err != nil {
		return
	}

	// Check for priority and daemon in holder
	for holderFieldName, holderValue := range holderFields {
		lowerHolderField := strings.ToLower(holderFieldName)

		switch {
		case lowerHolderField == "priority":
			if priority, ok := holderValue.(int32); ok {
				threadData.Priority = priority
			} else if priority, ok := holderValue.(int8); ok {
				threadData.Priority = int32(priority)
			}
		case lowerHolderField == "daemon":
			if daemon, ok := holderValue.(bool); ok {
				threadData.Daemon = daemon
			}
		case strings.Contains(lowerHolderField, "group"):
			if groupID, ok := holderValue.(model.ID); ok {
				threadData.ThreadGroup = groupID
			}
		}
	}
}

// extractFieldValues extracts field values from instance data using class definition
func extractFieldValues(instance *model.GCInstanceDump, classDump *model.ClassDump,
	classDumpReg *registry.ClassDumpRegistry, stringReg *registry.StringRegistry,
	identifierSize uint32) (map[string]interface{}, error) {

	fieldValues := make(map[string]interface{})
	data := instance.InstanceData
	offset := 0

	// Build complete field layout by walking inheritance hierarchy
	allFields, err := buildCompleteFieldLayout(classDump, classDumpReg, stringReg)
	if err != nil {
		return nil, fmt.Errorf("failed to build field layout: %w", err)
	}

	// Parse field values in order
	for _, fieldInfo := range allFields {
		if offset >= len(data) {
			break // Ran out of data
		}

		value, bytesRead, err := parseFieldValue(data[offset:], fieldInfo.Field.Type, identifierSize)
		if err != nil {
			return nil, fmt.Errorf("failed to parse field %s: %w", fieldInfo.Name, err)
		}

		fieldValues[fieldInfo.Name] = value
		offset += bytesRead
	}

	return fieldValues, nil
}

// FieldInfo represents a field with its resolved name
type FieldInfo struct {
	Name  string
	Field *model.InstanceField
}

// buildCompleteFieldLayout builds the complete field layout including inherited fields
func buildCompleteFieldLayout(classDump *model.ClassDump,
	classDumpReg *registry.ClassDumpRegistry, stringReg *registry.StringRegistry) ([]*FieldInfo, error) {

	var allFields []*FieldInfo
	seenFields := make(map[string]bool) // Track seen field names to avoid duplicates

	// Walk up the inheritance hierarchy
	currentClass := classDump
	for currentClass != nil {
		// Add instance fields from current class
		for _, field := range currentClass.InstanceFields {
			// Resolve the actual field name from string registry
			fieldName := stringReg.GetOrUnresolved(field.NameID)

			// Skip if we've already seen this field name
			if seenFields[fieldName] {
				continue
			}
			seenFields[fieldName] = true

			fieldInfo := &FieldInfo{
				Name:  fieldName,
				Field: field,
			}
			allFields = append(allFields, fieldInfo)
		}

		// Move to superclass
		if currentClass.SuperClassObjectID == 0 {
			break // Reached java.lang.Object
		}

		var hasSuper bool
		currentClass, hasSuper = classDumpReg.GetClassDump(currentClass.SuperClassObjectID)
		if !hasSuper {
			break // Superclass not found in dump
		}
	}

	return allFields, nil
}

// parseFieldValue parses a single field value from byte data
func parseFieldValue(data []byte, fieldType model.HProfTagFieldType, identifierSize uint32) (interface{}, int, error) {
	switch fieldType {
	case model.HPROF_BOOLEAN:
		if len(data) < 1 {
			return nil, 0, fmt.Errorf("insufficient data for boolean")
		}
		return data[0] != 0, 1, nil

	case model.HPROF_CHAR:
		if len(data) < 2 {
			return nil, 0, fmt.Errorf("insufficient data for char")
		}
		return binary.BigEndian.Uint16(data), 2, nil

	case model.HPROF_FLOAT:
		if len(data) < 4 {
			return nil, 0, fmt.Errorf("insufficient data for float")
		}
		return binary.BigEndian.Uint32(data), 4, nil

	case model.HPROF_DOUBLE:
		if len(data) < 8 {
			return nil, 0, fmt.Errorf("insufficient data for double")
		}
		return binary.BigEndian.Uint64(data), 8, nil

	case model.HPROF_BYTE:
		if len(data) < 1 {
			return nil, 0, fmt.Errorf("insufficient data for byte")
		}
		return int8(data[0]), 1, nil

	case model.HPROF_SHORT:
		if len(data) < 2 {
			return nil, 0, fmt.Errorf("insufficient data for short")
		}
		return int16(binary.BigEndian.Uint16(data)), 2, nil

	case model.HPROF_INT:
		if len(data) < 4 {
			return nil, 0, fmt.Errorf("insufficient data for int")
		}
		return int32(binary.BigEndian.Uint32(data)), 4, nil

	case model.HPROF_LONG:
		if len(data) < 8 {
			return nil, 0, fmt.Errorf("insufficient data for long")
		}
		return int64(binary.BigEndian.Uint64(data)), 8, nil

	case model.HPROF_NORMAL_OBJECT, model.HPROF_ARRAY_OBJECT:
		// Object references - size depends on identifier size
		idSize := int(identifierSize)
		if len(data) < idSize {
			return nil, 0, fmt.Errorf("insufficient data for object reference")
		}

		var objectID model.ID
		if identifierSize == 4 {
			objectID = model.ID(binary.BigEndian.Uint32(data))
		} else {
			objectID = model.ID(binary.BigEndian.Uint64(data))
		}
		return objectID, idSize, nil

	default:
		return nil, 0, fmt.Errorf("unknown field type: 0x%02x", fieldType)
	}
}
