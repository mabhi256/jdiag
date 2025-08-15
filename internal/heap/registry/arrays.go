package registry

import (
	"fmt"

	"github.com/mabhi256/jdiag/internal/heap/model"
)

// ArrayRegistry manages parsed array instances from array dump records
type ArrayRegistry struct {
	objectArrays    map[model.ID]*model.GCObjectArrayDump
	primitiveArrays map[model.ID]*model.GCPrimitiveArrayDump
	totalArrays     int
	totalElements   int64
	totalArraySize  uint64 // Total memory used by arrays
}

// NewArrayRegistry creates a new array registry
func NewArrayRegistry() *ArrayRegistry {
	return &ArrayRegistry{
		objectArrays:    make(map[model.ID]*model.GCObjectArrayDump),
		primitiveArrays: make(map[model.ID]*model.GCPrimitiveArrayDump),
	}
}

// AddObjectArray adds a parsed object array to the registry
func (r *ArrayRegistry) AddObjectArray(array *model.GCObjectArrayDump) {
	r.objectArrays[array.ObjectID] = array
	r.totalArrays++
	r.totalElements += int64(array.Size)

	// Calculate array size: header + (elements * reference size)
	// Assume 8-byte references for now - should get from header
	arraySize := uint64(16 + array.Size*8) // Object header + array elements
	r.totalArraySize += arraySize
}

// AddPrimitiveArray adds a parsed primitive array to the registry
func (r *ArrayRegistry) AddPrimitiveArray(array *model.GCPrimitiveArrayDump) {
	r.primitiveArrays[array.ObjectID] = array
	r.totalArrays++
	r.totalElements += int64(array.Size)

	// Calculate array size: header + elements
	elementSize := array.Type.Size(8)                     // Assume 8-byte IDs for now
	arraySize := uint64(16 + int(array.Size)*elementSize) // Object header + array data
	r.totalArraySize += arraySize
}

// GetObjectArray retrieves an object array by ID
func (r *ArrayRegistry) GetObjectArray(arrayID model.ID) (*model.GCObjectArrayDump, bool) {
	array, exists := r.objectArrays[arrayID]
	return array, exists
}

// GetPrimitiveArray retrieves a primitive array by ID
func (r *ArrayRegistry) GetPrimitiveArray(arrayID model.ID) (*model.GCPrimitiveArrayDump, bool) {
	array, exists := r.primitiveArrays[arrayID]
	return array, exists
}

// GetAllObjectArrays returns all object arrays
func (r *ArrayRegistry) GetAllObjectArrays() map[model.ID]*model.GCObjectArrayDump {
	return r.objectArrays
}

// GetAllPrimitiveArrays returns all primitive arrays
func (r *ArrayRegistry) GetAllPrimitiveArrays() map[model.ID]*model.GCPrimitiveArrayDump {
	return r.primitiveArrays
}

// GetCount returns the total number of arrays
func (r *ArrayRegistry) GetCount() int {
	return r.totalArrays
}

// GetObjectArrayCount returns the number of object arrays
func (r *ArrayRegistry) GetObjectArrayCount() int {
	return len(r.objectArrays)
}

// GetPrimitiveArrayCount returns the number of primitive arrays
func (r *ArrayRegistry) GetPrimitiveArrayCount() int {
	return len(r.primitiveArrays)
}

// GetTotalElements returns the total number of array elements
func (r *ArrayRegistry) GetTotalElements() int64 {
	return r.totalElements
}

// GetTotalSize returns the total memory used by arrays
func (r *ArrayRegistry) GetTotalSize() uint64 {
	return r.totalArraySize
}

// GetCharArray retrieves a char array and converts it to string
func (r *ArrayRegistry) GetCharArray(arrayID model.ID) (string, bool) {
	array, exists := r.primitiveArrays[arrayID]
	if !exists {
		return "", false
	}

	// Check if this is a char array (type 5 = HPROF_CHAR)
	if array.Type != model.HPROF_CHAR {
		return "", false
	}

	return r.convertCharArrayToString(array.Elements), true
}

// GetByteArray retrieves a byte array and converts it to string (for compact strings)
func (r *ArrayRegistry) GetByteArray(arrayID model.ID) (string, bool) {
	array, exists := r.primitiveArrays[arrayID]
	if !exists {
		return "", false
	}

	// Check if this is a byte array (type 8 = HPROF_BYTE)
	if array.Type != model.HPROF_BYTE {
		return "", false
	}

	return r.convertByteArrayToString(array.Elements), true
}

// convertCharArrayToString converts char array bytes to string
func (r *ArrayRegistry) convertCharArrayToString(elements []byte) string {
	if len(elements)%2 != 0 {
		return "<invalid char array>"
	}

	chars := make([]rune, len(elements)/2)
	for i := 0; i < len(chars); i++ {
		// Read 2-byte char in big-endian format
		charValue := (uint16(elements[i*2]) << 8) | uint16(elements[i*2+1])
		chars[i] = rune(charValue)
	}

	return string(chars)
}

// convertByteArrayToString converts byte array to string (Latin1 encoding)
func (r *ArrayRegistry) convertByteArrayToString(elements []byte) string {
	chars := make([]rune, len(elements))
	for i, b := range elements {
		chars[i] = rune(b)
	}
	return string(chars)
}

// GetArraysByClass returns arrays grouped by their class ID
func (r *ArrayRegistry) GetArraysByClass() map[model.ID]int {
	classCounts := make(map[model.ID]int)

	for _, array := range r.objectArrays {
		classCounts[array.ClassID]++
	}

	// For primitive arrays, we could group by element type
	// but they don't have class IDs in the same way

	return classCounts
}

// GetLargestArrays returns the N largest arrays by element count
func (r *ArrayRegistry) GetLargestArrays(n int) []*ArrayInfo {
	var arrays []*ArrayInfo

	// Add object arrays
	for _, array := range r.objectArrays {
		arrays = append(arrays, &ArrayInfo{
			ObjectID:    array.ObjectID,
			Size:        array.Size,
			Type:        "Object[]",
			ElementType: fmt.Sprintf("Class@0x%x", uint64(array.ClassID)),
		})
	}

	// Add primitive arrays
	for _, array := range r.primitiveArrays {
		arrays = append(arrays, &ArrayInfo{
			ObjectID:    array.ObjectID,
			Size:        array.Size,
			Type:        "Primitive[]",
			ElementType: array.Type.String(),
		})
	}

	// Sort by size (simple bubble sort for small N)
	for i := 0; i < len(arrays)-1; i++ {
		for j := 0; j < len(arrays)-1-i; j++ {
			if arrays[j].Size < arrays[j+1].Size {
				arrays[j], arrays[j+1] = arrays[j+1], arrays[j]
			}
		}
	}

	// Return top N
	if n > len(arrays) {
		n = len(arrays)
	}
	return arrays[:n]
}

// ArrayInfo represents summary information about an array
type ArrayInfo struct {
	ObjectID    model.ID
	Size        uint32
	Type        string
	ElementType string
}

// Statistics returns summary statistics about the array registry
func (r *ArrayRegistry) Statistics() map[string]interface{} {
	avgObjectArraySize := float64(0)
	avgPrimitiveArraySize := float64(0)

	if len(r.objectArrays) > 0 {
		totalObjElements := int64(0)
		for _, array := range r.objectArrays {
			totalObjElements += int64(array.Size)
		}
		avgObjectArraySize = float64(totalObjElements) / float64(len(r.objectArrays))
	}

	if len(r.primitiveArrays) > 0 {
		totalPrimElements := int64(0)
		for _, array := range r.primitiveArrays {
			totalPrimElements += int64(array.Size)
		}
		avgPrimitiveArraySize = float64(totalPrimElements) / float64(len(r.primitiveArrays))
	}

	return map[string]interface{}{
		"total_arrays":             r.totalArrays,
		"object_arrays":            len(r.objectArrays),
		"primitive_arrays":         len(r.primitiveArrays),
		"total_elements":           r.totalElements,
		"total_size":               r.totalArraySize,
		"avg_object_array_size":    avgObjectArraySize,
		"avg_primitive_array_size": avgPrimitiveArraySize,
	}
}
