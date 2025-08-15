package registry

import (
	"maps"

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
	allRootObjects map[model.ID]model.HProfTagSubRecord // ObjectID -> Root Type

	// Thread-specific indexes
	threadObjects    map[model.SerialNum]model.GCRootThreadObject // ThreadSerial -> Thread Object
	threadStackRoots map[model.SerialNum][]any                    // ThreadSerial -> Stack roots (Java frames + JNI locals)

	// Statistics
	totalRoots     int
	rootTypeCounts map[model.HProfTagSubRecord]int
}

func NewGCRootRegistry() *GCRootRegistry {
	return &GCRootRegistry{
		allRootObjects:   make(map[model.ID]model.HProfTagSubRecord),
		threadObjects:    make(map[model.SerialNum]model.GCRootThreadObject),
		threadStackRoots: make(map[model.SerialNum][]interface{}),
		rootTypeCounts:   make(map[model.HProfTagSubRecord]int),
	}
}

func (r *GCRootRegistry) AddUnknownRoot(root model.GCRootUnknown) {
	r.unknownRoots = append(r.unknownRoots, root)
	r.allRootObjects[root.ObjectID] = model.HPROF_GC_ROOT_UNKNOWN
	r.totalRoots++
	r.rootTypeCounts[model.HPROF_GC_ROOT_UNKNOWN]++
}

func (r *GCRootRegistry) AddJniGlobalRoot(root model.GCRootJniGlobal) {
	r.jniGlobalRoots = append(r.jniGlobalRoots, root)
	r.allRootObjects[root.ObjectID] = model.HPROF_GC_ROOT_JNI_GLOBAL
	r.totalRoots++
	r.rootTypeCounts[model.HPROF_GC_ROOT_JNI_GLOBAL]++
}

func (r *GCRootRegistry) AddJniLocalRoot(root model.GCRootJniLocal) {
	r.jniLocalRoots = append(r.jniLocalRoots, root)
	r.allRootObjects[root.ObjectID] = model.HPROF_GC_ROOT_JNI_LOCAL
	r.totalRoots++
	r.rootTypeCounts[model.HPROF_GC_ROOT_JNI_LOCAL]++

	// Add to thread stack roots for thread analysis
	r.threadStackRoots[root.ThreadSerialNumber] = append(
		r.threadStackRoots[root.ThreadSerialNumber], root)
}

func (r *GCRootRegistry) AddJavaFrameRoot(root model.GCRootJavaFrame) {
	r.javaFrameRoots = append(r.javaFrameRoots, root)
	r.allRootObjects[root.ObjectID] = model.HPROF_GC_ROOT_JAVA_FRAME
	r.totalRoots++
	r.rootTypeCounts[model.HPROF_GC_ROOT_JAVA_FRAME]++

	// Add to thread stack roots for thread analysis
	r.threadStackRoots[root.ThreadSerialNumber] = append(
		r.threadStackRoots[root.ThreadSerialNumber], root)
}

func (r *GCRootRegistry) AddNativeStackRoot(root model.GCRootNativeStack) {
	r.nativeStackRoots = append(r.nativeStackRoots, root)
	r.allRootObjects[root.ObjectID] = model.HPROF_GC_ROOT_NATIVE_STACK
	r.totalRoots++
	r.rootTypeCounts[model.HPROF_GC_ROOT_NATIVE_STACK]++
}

func (r *GCRootRegistry) AddStickyClassRoot(root model.GCRootStickyClass) {
	r.stickyClassRoots = append(r.stickyClassRoots, root)
	r.allRootObjects[root.ObjectID] = model.HPROF_GC_ROOT_STICKY_CLASS
	r.totalRoots++
	r.rootTypeCounts[model.HPROF_GC_ROOT_STICKY_CLASS]++
}

func (r *GCRootRegistry) AddThreadBlockRoot(root model.GCRootThreadBlock) {
	r.threadBlockRoots = append(r.threadBlockRoots, root)
	r.allRootObjects[root.ObjectID] = model.HPROF_GC_ROOT_THREAD_BLOCK
	r.totalRoots++
	r.rootTypeCounts[model.HPROF_GC_ROOT_THREAD_BLOCK]++
}

func (r *GCRootRegistry) AddMonitorUsedRoot(root model.GCRootMonitorUsed) {
	r.monitorUsedRoots = append(r.monitorUsedRoots, root)
	r.allRootObjects[root.ObjectID] = model.HPROF_GC_ROOT_MONITOR_USED
	r.totalRoots++
	r.rootTypeCounts[model.HPROF_GC_ROOT_MONITOR_USED]++
}

func (r *GCRootRegistry) AddThreadObjectRoot(root model.GCRootThreadObject) {
	r.threadObjectRoots = append(r.threadObjectRoots, root)
	if root.ThreadObjectID != 0 { // Thread object ID may be 0 for JNI-attached threads
		r.allRootObjects[root.ThreadObjectID] = model.HPROF_GC_ROOT_THREAD_OBJ
	}
	r.totalRoots++
	r.rootTypeCounts[model.HPROF_GC_ROOT_THREAD_OBJ]++

	// Index by thread serial number for lookups
	r.threadObjects[root.ThreadSerialNumber] = root
}

func (r *GCRootRegistry) GetTotalRoots() int {
	return r.totalRoots
}

func (r *GCRootRegistry) GetRootTypeCounts() map[model.HProfTagSubRecord]int {
	// Return a copy to prevent modification
	counts := make(map[model.HProfTagSubRecord]int)
	maps.Copy(counts, r.rootTypeCounts)
	return counts
}

func (r *GCRootRegistry) IsRootObject(objectID model.ID) bool {
	_, exists := r.allRootObjects[objectID]
	return exists
}

func (r *GCRootRegistry) GetRootType(objectID model.ID) (model.HProfTagSubRecord, bool) {
	rootType, exists := r.allRootObjects[objectID]
	return rootType, exists
}

func (r *GCRootRegistry) GetThreadObject(threadSerial model.SerialNum) (model.GCRootThreadObject, bool) {
	threadObj, exists := r.threadObjects[threadSerial]
	return threadObj, exists
}

func (r *GCRootRegistry) GetThreadStackRoots(threadSerial model.SerialNum) []interface{} {
	return r.threadStackRoots[threadSerial]
}

func (r *GCRootRegistry) GetAllThreadSerials() []model.SerialNum {
	var serials []model.SerialNum
	for serial := range r.threadObjects {
		serials = append(serials, serial)
	}
	return serials
}

func (r *GCRootRegistry) GetThreadObjectRoots() []model.GCRootThreadObject {
	return r.threadObjectRoots
}

func (r *GCRootRegistry) GetJniGlobalRoots() []model.GCRootJniGlobal {
	return r.jniGlobalRoots
}

func (r *GCRootRegistry) GetStickyClassRoots() []model.GCRootStickyClass {
	return r.stickyClassRoots
}
