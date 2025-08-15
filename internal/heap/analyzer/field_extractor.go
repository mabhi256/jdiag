package analyzer

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/mabhi256/jdiag/internal/heap/model"
)

// FieldExtractor handles parsing instance field values from binary data using context-based dependencies
type FieldExtractor struct {
	// Analysis context
	ctx *AnalysisContext

	// Initialization state
	initialized bool
}

// NewFieldExtractor creates a new field extractor using the analysis context
func NewFieldExtractor(ctx *AnalysisContext) *FieldExtractor {
	extractor := &FieldExtractor{
		ctx:         ctx,
		initialized: ctx != nil,
	}

	return extractor
}

// Initialize sets up the field extractor (implements AnalysisComponent interface)
func (fe *FieldExtractor) Initialize(ctx *AnalysisContext) error {
	if ctx == nil {
		return fmt.Errorf("analysis context is required")
	}

	if err := ctx.Validate(); err != nil {
		return fmt.Errorf("invalid context: %w", err)
	}

	fe.ctx = ctx
	fe.initialized = true

	return nil
}

// FieldInfo represents a field with its metadata and layout information
type FieldInfo struct {
	Name        string
	Type        model.HProfTagFieldType
	IsReference bool
	Size        int
	Offset      int
}

// ExtractInstanceFieldReferences extracts object references from instance field data
func (fe *FieldExtractor) ExtractInstanceFieldReferences(instance *model.GCInstanceDump,
	classDump *model.GCClassDump) ([]model.ID, error) {

	if !fe.initialized {
		return nil, fmt.Errorf("field extractor not initialized")
	}

	// Get all instance fields including inherited ones
	allFields, err := fe.getAllInstanceFields(classDump)
	if err != nil {
		return nil, fmt.Errorf("failed to get instance fields: %w", err)
	}

	// Parse field values and extract object references
	references, err := fe.parseFieldReferences(instance.InstanceData, allFields)
	if err != nil {
		return nil, fmt.Errorf("failed to parse field references: %w", err)
	}

	return references, nil
}

// ExtractInstanceFieldValues extracts all field values (for detailed analysis)
func (fe *FieldExtractor) ExtractInstanceFieldValues(instance *model.GCInstanceDump,
	classDump *model.GCClassDump) (map[string]interface{}, error) {

	if !fe.initialized {
		return nil, fmt.Errorf("field extractor not initialized")
	}

	// Get all instance fields including inherited ones
	allFields, err := fe.getAllInstanceFields(classDump)
	if err != nil {
		return nil, fmt.Errorf("failed to get instance fields: %w", err)
	}

	// Parse all field values
	fieldValues, err := fe.parseAllFieldValues(instance.InstanceData, allFields)
	if err != nil {
		return nil, fmt.Errorf("failed to parse field values: %w", err)
	}

	return fieldValues, nil
}

// getAllInstanceFields gets all instance fields including inherited ones using proper inheritance order
func (fe *FieldExtractor) getAllInstanceFields(classDump *model.GCClassDump) ([]FieldInfo, error) {
	var allFields []FieldInfo

	// Build inheritance chain from current class to root
	inheritanceChain, err := fe.buildInheritanceChain(classDump)
	if err != nil {
		return nil, fmt.Errorf("failed to build inheritance chain: %w", err)
	}

	// Add fields in correct order: ROOT CLASS FIRST, then subclasses
	// Reverse the inheritance chain so root class comes first
	for i := len(inheritanceChain) - 1; i >= 0; i-- {
		classFields := fe.getClassInstanceFields(inheritanceChain[i])
		allFields = append(allFields, classFields...)
	}

	// Calculate field offsets based on field order
	fe.calculateFieldOffsets(allFields)

	return allFields, nil
}

// buildInheritanceChain builds the complete inheritance chain for a class
func (fe *FieldExtractor) buildInheritanceChain(classDump *model.GCClassDump) ([]*model.GCClassDump, error) {
	var inheritanceChain []*model.GCClassDump
	currentClass := classDump

	// Traverse up the inheritance hierarchy
	for currentClass != nil {
		inheritanceChain = append(inheritanceChain, currentClass)

		// Move to superclass
		if currentClass.SuperClassObjectID == 0 {
			break // Reached root class (Object)
		}

		superClass, hasSuperClass := fe.ctx.ClassDumpReg.GetClassDump(currentClass.SuperClassObjectID)
		if !hasSuperClass {
			// Can't follow inheritance chain further - this could indicate missing class data
			break
		}
		currentClass = superClass
	}

	return inheritanceChain, nil
}

// getClassInstanceFields extracts instance fields from a single class
func (fe *FieldExtractor) getClassInstanceFields(classDump *model.GCClassDump) []FieldInfo {
	var fields []FieldInfo

	for _, field := range classDump.InstanceFields {
		fieldName := fe.ctx.StringReg.GetOrUnresolved(field.NameID)
		fieldType := field.Type

		fieldInfo := FieldInfo{
			Name:        fieldName,
			Type:        fieldType,
			IsReference: fe.isReferenceType(fieldType),
			Size:        fieldType.Size(fe.ctx.Config.IdentifierSize),
		}

		fields = append(fields, fieldInfo)
	}

	return fields
}

// calculateFieldOffsets calculates the byte offset for each field in instance data
func (fe *FieldExtractor) calculateFieldOffsets(fields []FieldInfo) {
	offset := 0
	for i := range fields {
		fields[i].Offset = offset
		offset += fields[i].Size
	}
}

// isReferenceType checks if a field type represents an object reference
func (fe *FieldExtractor) isReferenceType(fieldType model.HProfTagFieldType) bool {
	return fieldType == model.HPROF_NORMAL_OBJECT || fieldType == model.HPROF_ARRAY_OBJECT
}

// parseFieldReferences parses instance data and extracts only object references
func (fe *FieldExtractor) parseFieldReferences(instanceData []byte, fields []FieldInfo) ([]model.ID, error) {
	var references []model.ID

	for _, field := range fields {
		if field.IsReference {
			// Check data bounds
			if field.Offset+field.Size > len(instanceData) {
				// Data truncated - skip remaining fields
				break
			}

			objectID, err := fe.parseObjectReference(instanceData[field.Offset : field.Offset+field.Size])
			if err != nil {
				continue // Skip malformed references
			}

			if objectID != 0 { // Non-null reference
				references = append(references, objectID)
			}
		}
	}

	return references, nil
}

// parseAllFieldValues parses instance data and extracts all field values with error handling
func (fe *FieldExtractor) parseAllFieldValues(instanceData []byte, fields []FieldInfo) (map[string]interface{}, error) {
	fieldValues := make(map[string]interface{})

	for _, field := range fields {
		if field.Offset+field.Size > len(instanceData) {
			// Data truncated - skip remaining fields
			break
		}

		fieldData := instanceData[field.Offset : field.Offset+field.Size]
		value, err := fe.parseFieldValue(fieldData, field.Type)
		if err != nil {
			// Store error information for debugging
			fieldValues[field.Name] = fmt.Sprintf("parse_error: %v", err)
			continue
		}

		fieldValues[field.Name] = value
	}

	return fieldValues, nil
}

// parseObjectReference extracts an object ID from reference field data
func (fe *FieldExtractor) parseObjectReference(data []byte) (model.ID, error) {
	identifierSize := int(fe.ctx.Config.IdentifierSize)

	if len(data) != identifierSize {
		return 0, fmt.Errorf("invalid reference data size: expected %d, got %d",
			identifierSize, len(data))
	}

	var objectID model.ID

	// Parse big-endian object ID based on identifier size
	switch identifierSize {
	case 4:
		objectID = model.ID(binary.BigEndian.Uint32(data))
	case 8:
		objectID = model.ID(binary.BigEndian.Uint64(data))
	default:
		return 0, fmt.Errorf("unsupported identifier size: %d", identifierSize)
	}

	return objectID, nil
}

// parseFieldValue parses a field value according to its type with comprehensive type support
func (fe *FieldExtractor) parseFieldValue(data []byte, fieldType model.HProfTagFieldType) (interface{}, error) {
	switch fieldType {
	case model.HPROF_BOOLEAN:
		if len(data) < 1 {
			return nil, fmt.Errorf("insufficient data for boolean field")
		}
		return data[0] != 0, nil

	case model.HPROF_BYTE:
		if len(data) < 1 {
			return nil, fmt.Errorf("insufficient data for byte field")
		}
		return int8(data[0]), nil

	case model.HPROF_CHAR:
		if len(data) < 2 {
			return nil, fmt.Errorf("insufficient data for char field")
		}
		return int32(binary.BigEndian.Uint16(data)), nil // Char as Unicode code point

	case model.HPROF_SHORT:
		if len(data) < 2 {
			return nil, fmt.Errorf("insufficient data for short field")
		}
		return int16(binary.BigEndian.Uint16(data)), nil

	case model.HPROF_INT:
		if len(data) < 4 {
			return nil, fmt.Errorf("insufficient data for int field")
		}
		return int32(binary.BigEndian.Uint32(data)), nil

	case model.HPROF_LONG:
		if len(data) < 8 {
			return nil, fmt.Errorf("insufficient data for long field")
		}
		return int64(binary.BigEndian.Uint64(data)), nil

	case model.HPROF_FLOAT:
		if len(data) < 4 {
			return nil, fmt.Errorf("insufficient data for float field")
		}
		bits := binary.BigEndian.Uint32(data)
		return math.Float32frombits(bits), nil

	case model.HPROF_DOUBLE:
		if len(data) < 8 {
			return nil, fmt.Errorf("insufficient data for double field")
		}
		bits := binary.BigEndian.Uint64(data)
		return math.Float64frombits(bits), nil

	case model.HPROF_NORMAL_OBJECT, model.HPROF_ARRAY_OBJECT:
		// Return object ID for references
		return fe.parseObjectReference(data)

	default:
		return nil, fmt.Errorf("unknown field type: %v", fieldType)
	}
}

// GetFieldLayout returns the complete field layout for a class (useful for debugging)
func (fe *FieldExtractor) GetFieldLayout(classDump *model.GCClassDump) ([]FieldInfo, error) {
	if !fe.initialized {
		return nil, fmt.Errorf("field extractor not initialized")
	}

	return fe.getAllInstanceFields(classDump)
}

// ValidateFieldData validates that instance data is consistent with field layout
func (fe *FieldExtractor) ValidateFieldData(instance *model.GCInstanceDump, classDump *model.GCClassDump) error {
	if !fe.initialized {
		return fmt.Errorf("field extractor not initialized")
	}

	fields, err := fe.getAllInstanceFields(classDump)
	if err != nil {
		return fmt.Errorf("failed to get field layout: %w", err)
	}

	// Calculate expected data size
	expectedSize := 0
	if len(fields) > 0 {
		lastField := fields[len(fields)-1]
		expectedSize = lastField.Offset + lastField.Size
	}

	actualSize := len(instance.InstanceData)

	if actualSize < expectedSize {
		return fmt.Errorf("instance data truncated: expected at least %d bytes, got %d",
			expectedSize, actualSize)
	}

	return nil
}
