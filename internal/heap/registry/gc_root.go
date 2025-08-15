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
