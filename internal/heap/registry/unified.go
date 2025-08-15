package registry

import (
	"maps"
	"sync"
)

// Statistics represents common statistics across all registries
type Statistics struct {
	TotalCount int
	TotalSize  uint64
	Details    map[string]interface{}
}

// BaseRegistry provides common functionality for simple key-value registries
type BaseRegistry[K comparable, V any] struct {
	data       map[K]V
	totalCount int
	totalSize  uint64
	mu         sync.RWMutex // For future thread safety if needed
}

// NewBaseRegistry creates a new base registry
func NewBaseRegistry[K comparable, V any]() *BaseRegistry[K, V] {
	return &BaseRegistry[K, V]{
		data: make(map[K]V),
	}
}

// Add adds an item to the registry
func (r *BaseRegistry[K, V]) Add(key K, value V) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.data[key] = value
	r.totalCount++
}

// Get retrieves an item from the registry
func (r *BaseRegistry[K, V]) Get(key K) (V, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	value, exists := r.data[key]
	return value, exists
}

// GetAll returns all items (copy to prevent external modification)
func (r *BaseRegistry[K, V]) GetAll() map[K]V {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make(map[K]V, len(r.data))
	maps.Copy(result, r.data)
	return result
}

// Count returns the total number of items
func (r *BaseRegistry[K, V]) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.totalCount
}

// Clear removes all items
func (r *BaseRegistry[K, V]) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.data = make(map[K]V)
	r.totalCount = 0
	r.totalSize = 0
}

// UpdateSize updates the total size tracking
func (r *BaseRegistry[K, V]) UpdateSize(delta uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.totalSize += delta
}

// GetSize returns total size
func (r *BaseRegistry[K, V]) GetSize() uint64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.totalSize
}

type HeapRegistries struct {
	Strings    *StringRegistry
	Classes    *ClassRegistry
	Stack      *StackRegistry
	GCRoots    *GCRootRegistry
	ClassDumps *ClassDumpRegistry
	Objects    *InstanceRegistry
	Arrays     *ArrayRegistry
}

func NewHeapRegistries() *HeapRegistries {
	return &HeapRegistries{
		Strings:    NewStringRegistry(),
		Classes:    NewClassRegistry(),
		Stack:      NewStackRegistry(),
		GCRoots:    NewGCRootRegistry(),
		ClassDumps: NewClassDumpRegistry(),
		Objects:    NewInstanceRegistry(),
		Arrays:     NewArrayRegistry(),
	}
}

// GetOverallStatistics returns combined statistics across all registries
func (h *HeapRegistries) GetOverallStatistics() Statistics {
	return Statistics{
		TotalCount: h.Strings.Count() + h.Classes.Count() + h.Stack.CountFrames() +
			h.Stack.CountTraces() + h.GCRoots.GetTotalRoots() +
			h.ClassDumps.GetCount() + h.Objects.GetCount() + h.Arrays.GetCount(),
		Details: map[string]interface{}{
			"strings":     h.Strings.Count(),
			"classes":     h.Classes.Count(),
			"frames":      h.Stack.CountFrames(),
			"traces":      h.Stack.CountTraces(),
			"gc_roots":    h.GCRoots.GetTotalRoots(),
			"class_dumps": h.ClassDumps.GetCount(),
			"objects":     h.Objects.GetCount(),
			"arrays":      h.Arrays.GetCount(),
		},
	}
}

// Clear all registries
func (h *HeapRegistries) Clear() {
	h.Strings.Clear()
	h.Classes.Clear()
	h.Stack.Clear()
	h.GCRoots.Clear()
	h.ClassDumps.Clear()
	h.Objects.Clear()
	h.Arrays.Clear()
}

// GetMemoryStatistics returns memory-related statistics
func (h *HeapRegistries) GetMemoryStatistics() map[string]any {
	objectStats := h.Objects.Statistics()
	arrayStats := h.Arrays.Statistics()

	return map[string]any{
		"total_object_size": objectStats["total_size"],
		"total_array_size":  arrayStats["total_size"],
		"object_count":      objectStats["total_instances"],
		"array_count":       arrayStats["total_arrays"],
		"thread_count":      objectStats["thread_instances"],
	}
}
