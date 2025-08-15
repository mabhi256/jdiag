package registry

import (
	"sync"

	"github.com/mabhi256/jdiag/internal/heap/model"
)

type ThreadInstanceData struct {
	ObjectID          model.ID
	InstanceDump      *model.GCInstanceDump
	ThreadID          int64
	Name              string
	Priority          int32
	Daemon            bool
	ThreadStatus      int32
	ThreadGroup       model.ID
	ParsedFieldValues map[string]interface{}
}

type InstanceRegistry struct {
	instances       *BaseRegistry[model.ID, *model.GCInstanceDump]
	threadInstances *BaseRegistry[model.ID, *ThreadInstanceData]
	totalSize       uint64
	mu              sync.RWMutex
}

func NewInstanceRegistry() *InstanceRegistry {
	return &InstanceRegistry{
		instances:       NewBaseRegistry[model.ID, *model.GCInstanceDump](),
		threadInstances: NewBaseRegistry[model.ID, *ThreadInstanceData](),
	}
}

func (r *InstanceRegistry) AddInstance(instance *model.GCInstanceDump) {
	r.instances.Add(instance.ObjectID, instance)
	r.mu.Lock()
	r.totalSize += uint64(instance.Size)
	r.mu.Unlock()
}

func (r *InstanceRegistry) AddThreadInstance(threadData *ThreadInstanceData) {
	r.threadInstances.Add(threadData.ObjectID, threadData)
	r.AddInstance(threadData.InstanceDump)
}

func (r *InstanceRegistry) GetInstance(objectID model.ID) (*model.GCInstanceDump, bool) {
	return r.instances.Get(objectID)
}

func (r *InstanceRegistry) GetThreadInstance(objectID model.ID) (*ThreadInstanceData, bool) {
	return r.threadInstances.Get(objectID)
}

func (r *InstanceRegistry) GetAllInstances() map[model.ID]*model.GCInstanceDump {
	return r.instances.GetAll()
}

func (r *InstanceRegistry) GetAllThreadInstances() map[model.ID]*ThreadInstanceData {
	return r.threadInstances.GetAll()
}

func (r *InstanceRegistry) GetCount() int {
	return r.instances.Count()
}

func (r *InstanceRegistry) GetThreadCount() int {
	return r.threadInstances.Count()
}

func (r *InstanceRegistry) GetTotalSize() uint64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.totalSize
}

func (r *InstanceRegistry) GetInstancesByClass(classObjectID model.ID) []*model.GCInstanceDump {
	instances := r.instances.GetAll()
	var result []*model.GCInstanceDump
	for _, instance := range instances {
		if instance.ClassObjectID == classObjectID {
			result = append(result, instance)
		}
	}
	return result
}

func (r *InstanceRegistry) GetInstanceClassCounts() map[model.ID]int {
	instances := r.instances.GetAll()
	classCounts := make(map[model.ID]int)
	for _, instance := range instances {
		classCounts[instance.ClassObjectID]++
	}
	return classCounts
}

func (r *InstanceRegistry) Statistics() map[string]interface{} {
	r.mu.RLock()
	totalSize := r.totalSize
	r.mu.RUnlock()

	totalInstances := r.instances.Count()
	avgSize := float64(0)
	if totalInstances > 0 {
		avgSize = float64(totalSize) / float64(totalInstances)
	}

	return map[string]interface{}{
		"total_instances":  totalInstances,
		"total_size":       totalSize,
		"thread_instances": r.threadInstances.Count(),
		"unique_classes":   len(r.GetInstanceClassCounts()),
		"average_size":     avgSize,
	}
}

func (r *InstanceRegistry) Clear() {
	r.instances.Clear()
	r.threadInstances.Clear()
	r.mu.Lock()
	r.totalSize = 0
	r.mu.Unlock()
}
