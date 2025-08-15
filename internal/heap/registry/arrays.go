package registry

import (
	"fmt"
	"sort"
	"sync"

	"github.com/mabhi256/jdiag/internal/heap/model"
)

type ArrayInfo struct {
	ObjectID    model.ID
	Size        uint32
	Type        string
	ElementType string
}

type ArrayRegistry struct {
	objectArrays    *BaseRegistry[model.ID, *model.GCObjectArrayDump]
	primitiveArrays *BaseRegistry[model.ID, *model.GCPrimitiveArrayDump]
	totalElements   int64
	totalArraySize  uint64
	mu              sync.RWMutex
}

func NewArrayRegistry() *ArrayRegistry {
	return &ArrayRegistry{
		objectArrays:    NewBaseRegistry[model.ID, *model.GCObjectArrayDump](),
		primitiveArrays: NewBaseRegistry[model.ID, *model.GCPrimitiveArrayDump](),
	}
}

func (r *ArrayRegistry) AddObjectArray(array *model.GCObjectArrayDump) {
	r.objectArrays.Add(array.ObjectID, array)
	r.mu.Lock()
	r.totalElements += int64(array.Size)
	r.totalArraySize += uint64(16 + array.Size*8) // Object header + elements
	r.mu.Unlock()
}

func (r *ArrayRegistry) AddPrimitiveArray(array *model.GCPrimitiveArrayDump) {
	r.primitiveArrays.Add(array.ObjectID, array)
	r.mu.Lock()
	r.totalElements += int64(array.Size)
	elementSize := array.Type.Size(8) // Assume 8-byte IDs
	r.totalArraySize += uint64(16 + int(array.Size)*elementSize)
	r.mu.Unlock()
}

func (r *ArrayRegistry) GetObjectArray(arrayID model.ID) (*model.GCObjectArrayDump, bool) {
	return r.objectArrays.Get(arrayID)
}

func (r *ArrayRegistry) GetPrimitiveArray(arrayID model.ID) (*model.GCPrimitiveArrayDump, bool) {
	return r.primitiveArrays.Get(arrayID)
}

func (r *ArrayRegistry) GetAllObjectArrays() map[model.ID]*model.GCObjectArrayDump {
	return r.objectArrays.GetAll()
}

func (r *ArrayRegistry) GetAllPrimitiveArrays() map[model.ID]*model.GCPrimitiveArrayDump {
	return r.primitiveArrays.GetAll()
}

func (r *ArrayRegistry) GetCount() int {
	return r.objectArrays.Count() + r.primitiveArrays.Count()
}

func (r *ArrayRegistry) GetObjectArrayCount() int {
	return r.objectArrays.Count()
}

func (r *ArrayRegistry) GetPrimitiveArrayCount() int {
	return r.primitiveArrays.Count()
}

func (r *ArrayRegistry) GetTotalElements() int64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.totalElements
}

func (r *ArrayRegistry) GetTotalSize() uint64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.totalArraySize
}

func (r *ArrayRegistry) GetCharArray(arrayID model.ID) (string, bool) {
	array, exists := r.primitiveArrays.Get(arrayID)
	if !exists || array.Type != model.HPROF_CHAR {
		return "", false
	}
	return r.convertCharArrayToString(array.Elements), true
}

func (r *ArrayRegistry) GetByteArray(arrayID model.ID) (string, bool) {
	array, exists := r.primitiveArrays.Get(arrayID)
	if !exists || array.Type != model.HPROF_BYTE {
		return "", false
	}
	return r.convertByteArrayToString(array.Elements), true
}

func (r *ArrayRegistry) convertCharArrayToString(elements []byte) string {
	if len(elements)%2 != 0 {
		return "<invalid char array>"
	}
	chars := make([]rune, len(elements)/2)
	for i := 0; i < len(chars); i++ {
		charValue := (uint16(elements[i*2]) << 8) | uint16(elements[i*2+1])
		chars[i] = rune(charValue)
	}
	return string(chars)
}

func (r *ArrayRegistry) convertByteArrayToString(elements []byte) string {
	chars := make([]rune, len(elements))
	for i, b := range elements {
		chars[i] = rune(b)
	}
	return string(chars)
}

func (r *ArrayRegistry) GetLargestArrays(n int) []*ArrayInfo {
	var arrays []*ArrayInfo

	// Add object arrays
	for _, array := range r.objectArrays.GetAll() {
		arrays = append(arrays, &ArrayInfo{
			ObjectID:    array.ObjectID,
			Size:        array.Size,
			Type:        "Object[]",
			ElementType: fmt.Sprintf("Class@0x%x", uint64(array.ClassID)),
		})
	}

	// Add primitive arrays
	for _, array := range r.primitiveArrays.GetAll() {
		arrays = append(arrays, &ArrayInfo{
			ObjectID:    array.ObjectID,
			Size:        array.Size,
			Type:        "Primitive[]",
			ElementType: array.Type.String(),
		})
	}

	// Sort by size
	sort.Slice(arrays, func(i, j int) bool {
		return arrays[i].Size > arrays[j].Size
	})

	if n > len(arrays) {
		n = len(arrays)
	}
	return arrays[:n]
}

func (r *ArrayRegistry) Statistics() map[string]interface{} {
	r.mu.RLock()
	totalElements := r.totalElements
	totalSize := r.totalArraySize
	r.mu.RUnlock()

	objCount := r.objectArrays.Count()
	primCount := r.primitiveArrays.Count()

	avgObjSize := float64(0)
	avgPrimSize := float64(0)

	if objCount > 0 {
		objElements := int64(0)
		for _, array := range r.objectArrays.GetAll() {
			objElements += int64(array.Size)
		}
		avgObjSize = float64(objElements) / float64(objCount)
	}

	if primCount > 0 {
		primElements := int64(0)
		for _, array := range r.primitiveArrays.GetAll() {
			primElements += int64(array.Size)
		}
		avgPrimSize = float64(primElements) / float64(primCount)
	}

	return map[string]interface{}{
		"total_arrays":             objCount + primCount,
		"object_arrays":            objCount,
		"primitive_arrays":         primCount,
		"total_elements":           totalElements,
		"total_size":               totalSize,
		"avg_object_array_size":    avgObjSize,
		"avg_primitive_array_size": avgPrimSize,
	}
}

func (r *ArrayRegistry) Clear() {
	r.objectArrays.Clear()
	r.primitiveArrays.Clear()
	r.mu.Lock()
	r.totalElements = 0
	r.totalArraySize = 0
	r.mu.Unlock()
}
