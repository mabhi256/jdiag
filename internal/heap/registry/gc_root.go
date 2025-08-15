package registry

import (
	"maps"
	"sync"

	"github.com/mabhi256/jdiag/internal/heap/model"
)

type GCRootRegistry struct {
	// All roots by type
	unknownRoots      []model.GCRootUnknown
	jniGlobalRoots    []model.GCRootJniGlobal
	jniLocalRoots     []model.GCRootJniLocal
	javaFrameRoots    []model.GCRootJavaFrame
	nativeStackRoots  []model.GCRootNativeStack
	stickyClassRoots  []model.GCRootStickyClass
	threadBlockRoots  []model.GCRootThreadBlock
	monitorUsedRoots  []model.GCRootMonitorUsed
	threadObjectRoots []model.GCRootThreadObject

	// Index for fast lookups
	allRootObjects   *BaseRegistry[model.ID, model.HProfTagSubRecord]
	threadObjects    *BaseRegistry[model.SerialNum, model.GCRootThreadObject]
	threadStackRoots map[model.SerialNum][]interface{}

	// Statistics
	totalRoots     int
	rootTypeCounts map[model.HProfTagSubRecord]int
	mu             sync.RWMutex
}

func NewGCRootRegistry() *GCRootRegistry {
	return &GCRootRegistry{
		allRootObjects:   NewBaseRegistry[model.ID, model.HProfTagSubRecord](),
		threadObjects:    NewBaseRegistry[model.SerialNum, model.GCRootThreadObject](),
		threadStackRoots: make(map[model.SerialNum][]interface{}),
		rootTypeCounts:   make(map[model.HProfTagSubRecord]int),
	}
}

func (r *GCRootRegistry) AddUnknownRoot(root model.GCRootUnknown) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.unknownRoots = append(r.unknownRoots, root)
	r.allRootObjects.Add(root.ObjectID, model.HPROF_GC_ROOT_UNKNOWN)
	r.totalRoots++
	r.rootTypeCounts[model.HPROF_GC_ROOT_UNKNOWN]++
}

func (r *GCRootRegistry) AddJniGlobalRoot(root model.GCRootJniGlobal) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.jniGlobalRoots = append(r.jniGlobalRoots, root)
	r.allRootObjects.Add(root.ObjectID, model.HPROF_GC_ROOT_JNI_GLOBAL)
	r.totalRoots++
	r.rootTypeCounts[model.HPROF_GC_ROOT_JNI_GLOBAL]++
}

func (r *GCRootRegistry) AddJniLocalRoot(root model.GCRootJniLocal) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.jniLocalRoots = append(r.jniLocalRoots, root)
	r.allRootObjects.Add(root.ObjectID, model.HPROF_GC_ROOT_JNI_LOCAL)
	r.totalRoots++
	r.rootTypeCounts[model.HPROF_GC_ROOT_JNI_LOCAL]++
	r.threadStackRoots[root.ThreadSerialNumber] = append(r.threadStackRoots[root.ThreadSerialNumber], root)
}

func (r *GCRootRegistry) AddJavaFrameRoot(root model.GCRootJavaFrame) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.javaFrameRoots = append(r.javaFrameRoots, root)
	r.allRootObjects.Add(root.ObjectID, model.HPROF_GC_ROOT_JAVA_FRAME)
	r.totalRoots++
	r.rootTypeCounts[model.HPROF_GC_ROOT_JAVA_FRAME]++
	r.threadStackRoots[root.ThreadSerialNumber] = append(r.threadStackRoots[root.ThreadSerialNumber], root)
}

func (r *GCRootRegistry) AddNativeStackRoot(root model.GCRootNativeStack) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nativeStackRoots = append(r.nativeStackRoots, root)
	r.allRootObjects.Add(root.ObjectID, model.HPROF_GC_ROOT_NATIVE_STACK)
	r.totalRoots++
	r.rootTypeCounts[model.HPROF_GC_ROOT_NATIVE_STACK]++
}

func (r *GCRootRegistry) AddStickyClassRoot(root model.GCRootStickyClass) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stickyClassRoots = append(r.stickyClassRoots, root)
	r.allRootObjects.Add(root.ObjectID, model.HPROF_GC_ROOT_STICKY_CLASS)
	r.totalRoots++
	r.rootTypeCounts[model.HPROF_GC_ROOT_STICKY_CLASS]++
}

func (r *GCRootRegistry) AddThreadBlockRoot(root model.GCRootThreadBlock) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.threadBlockRoots = append(r.threadBlockRoots, root)
	r.allRootObjects.Add(root.ObjectID, model.HPROF_GC_ROOT_THREAD_BLOCK)
	r.totalRoots++
	r.rootTypeCounts[model.HPROF_GC_ROOT_THREAD_BLOCK]++
}

func (r *GCRootRegistry) AddMonitorUsedRoot(root model.GCRootMonitorUsed) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.monitorUsedRoots = append(r.monitorUsedRoots, root)
	r.allRootObjects.Add(root.ObjectID, model.HPROF_GC_ROOT_MONITOR_USED)
	r.totalRoots++
	r.rootTypeCounts[model.HPROF_GC_ROOT_MONITOR_USED]++
}

func (r *GCRootRegistry) AddThreadObjectRoot(root model.GCRootThreadObject) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.threadObjectRoots = append(r.threadObjectRoots, root)
	if root.ThreadObjectID != 0 {
		r.allRootObjects.Add(root.ThreadObjectID, model.HPROF_GC_ROOT_THREAD_OBJ)
	}
	r.totalRoots++
	r.rootTypeCounts[model.HPROF_GC_ROOT_THREAD_OBJ]++
	r.threadObjects.Add(root.ThreadSerialNumber, root)
}

func (r *GCRootRegistry) GetTotalRoots() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.totalRoots
}

func (r *GCRootRegistry) GetRootTypeCounts() map[model.HProfTagSubRecord]int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	counts := make(map[model.HProfTagSubRecord]int)
	maps.Copy(counts, r.rootTypeCounts)
	return counts
}

func (r *GCRootRegistry) IsRootObject(objectID model.ID) bool {
	_, exists := r.allRootObjects.Get(objectID)
	return exists
}

func (r *GCRootRegistry) GetRootType(objectID model.ID) (model.HProfTagSubRecord, bool) {
	return r.allRootObjects.Get(objectID)
}

func (r *GCRootRegistry) GetThreadObject(threadSerial model.SerialNum) (model.GCRootThreadObject, bool) {
	return r.threadObjects.Get(threadSerial)
}

func (r *GCRootRegistry) GetThreadStackRoots(threadSerial model.SerialNum) []interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.threadStackRoots[threadSerial]
}

func (r *GCRootRegistry) GetAllThreadSerials() []model.SerialNum {
	threadObjects := r.threadObjects.GetAll()
	serials := make([]model.SerialNum, 0, len(threadObjects))
	for serial := range threadObjects {
		serials = append(serials, serial)
	}
	return serials
}

func (r *GCRootRegistry) GetThreadObjectRoots() []model.GCRootThreadObject {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.threadObjectRoots
}

func (r *GCRootRegistry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.unknownRoots = nil
	r.jniGlobalRoots = nil
	r.jniLocalRoots = nil
	r.javaFrameRoots = nil
	r.nativeStackRoots = nil
	r.stickyClassRoots = nil
	r.threadBlockRoots = nil
	r.monitorUsedRoots = nil
	r.threadObjectRoots = nil
	r.allRootObjects.Clear()
	r.threadObjects.Clear()
	r.threadStackRoots = make(map[model.SerialNum][]interface{})
	r.totalRoots = 0
	r.rootTypeCounts = make(map[model.HProfTagSubRecord]int)
}

// GCRootInfo represents a unified view of any GC root with its type information
type GCRootInfo struct {
	ObjectID model.ID
	RootType model.HProfTagSubRecord
	Root     interface{} // The actual root struct (GCRootUnknown, GCRootJniGlobal, etc.)
}

// GetAllRoots returns all GC roots regardless of type as a unified collection
func (r *GCRootRegistry) GetAllRoots() []GCRootInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var allRoots []GCRootInfo

	// Add unknown roots
	for _, root := range r.unknownRoots {
		allRoots = append(allRoots, GCRootInfo{
			ObjectID: root.ObjectID,
			RootType: model.HPROF_GC_ROOT_UNKNOWN,
			Root:     root,
		})
	}

	// Add JNI global roots
	for _, root := range r.jniGlobalRoots {
		allRoots = append(allRoots, GCRootInfo{
			ObjectID: root.ObjectID,
			RootType: model.HPROF_GC_ROOT_JNI_GLOBAL,
			Root:     root,
		})
	}

	// Add JNI local roots
	for _, root := range r.jniLocalRoots {
		allRoots = append(allRoots, GCRootInfo{
			ObjectID: root.ObjectID,
			RootType: model.HPROF_GC_ROOT_JNI_LOCAL,
			Root:     root,
		})
	}

	// Add Java frame roots
	for _, root := range r.javaFrameRoots {
		allRoots = append(allRoots, GCRootInfo{
			ObjectID: root.ObjectID,
			RootType: model.HPROF_GC_ROOT_JAVA_FRAME,
			Root:     root,
		})
	}

	// Add native stack roots
	for _, root := range r.nativeStackRoots {
		allRoots = append(allRoots, GCRootInfo{
			ObjectID: root.ObjectID,
			RootType: model.HPROF_GC_ROOT_NATIVE_STACK,
			Root:     root,
		})
	}

	// Add sticky class roots
	for _, root := range r.stickyClassRoots {
		allRoots = append(allRoots, GCRootInfo{
			ObjectID: root.ObjectID,
			RootType: model.HPROF_GC_ROOT_STICKY_CLASS,
			Root:     root,
		})
	}

	// Add thread block roots
	for _, root := range r.threadBlockRoots {
		allRoots = append(allRoots, GCRootInfo{
			ObjectID: root.ObjectID,
			RootType: model.HPROF_GC_ROOT_THREAD_BLOCK,
			Root:     root,
		})
	}

	// Add monitor used roots
	for _, root := range r.monitorUsedRoots {
		allRoots = append(allRoots, GCRootInfo{
			ObjectID: root.ObjectID,
			RootType: model.HPROF_GC_ROOT_MONITOR_USED,
			Root:     root,
		})
	}

	// Add thread object roots
	for _, root := range r.threadObjectRoots {
		allRoots = append(allRoots, GCRootInfo{
			ObjectID: root.ThreadObjectID, // Note: ThreadObjectID, not ObjectID
			RootType: model.HPROF_GC_ROOT_THREAD_OBJ,
			Root:     root,
		})
	}

	return allRoots
}

// GetAllRootObjectIDs returns just the object IDs of all GC roots (convenient for existence checking)
func (r *GCRootRegistry) GetAllRootObjectIDs() []model.ID {
	allRoots := r.GetAllRoots()
	objectIDs := make([]model.ID, 0, len(allRoots))

	for _, root := range allRoots {
		if root.ObjectID != 0 { // Skip null references
			objectIDs = append(objectIDs, root.ObjectID)
		}
	}

	return objectIDs
}

// GetRootsByType returns all roots of a specific type
func (r *GCRootRegistry) GetRootsByType(rootType model.HProfTagSubRecord) []GCRootInfo {
	allRoots := r.GetAllRoots()
	var filteredRoots []GCRootInfo

	for _, root := range allRoots {
		if root.RootType == rootType {
			filteredRoots = append(filteredRoots, root)
		}
	}

	return filteredRoots
}

// Additional convenience getters for individual root types
// (These expose the private slices safely)

func (r *GCRootRegistry) GetUnknownRoots() []model.GCRootUnknown {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]model.GCRootUnknown(nil), r.unknownRoots...)
}

func (r *GCRootRegistry) GetJniGlobalRoots() []model.GCRootJniGlobal {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]model.GCRootJniGlobal(nil), r.jniGlobalRoots...)
}

func (r *GCRootRegistry) GetJniLocalRoots() []model.GCRootJniLocal {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]model.GCRootJniLocal(nil), r.jniLocalRoots...)
}

func (r *GCRootRegistry) GetJavaFrameRoots() []model.GCRootJavaFrame {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]model.GCRootJavaFrame(nil), r.javaFrameRoots...)
}

func (r *GCRootRegistry) GetNativeStackRoots() []model.GCRootNativeStack {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]model.GCRootNativeStack(nil), r.nativeStackRoots...)
}

func (r *GCRootRegistry) GetStickyClassRoots() []model.GCRootStickyClass {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]model.GCRootStickyClass(nil), r.stickyClassRoots...)
}

func (r *GCRootRegistry) GetThreadBlockRoots() []model.GCRootThreadBlock {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]model.GCRootThreadBlock(nil), r.threadBlockRoots...)
}

func (r *GCRootRegistry) GetMonitorUsedRoots() []model.GCRootMonitorUsed {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]model.GCRootMonitorUsed(nil), r.monitorUsedRoots...)
}
