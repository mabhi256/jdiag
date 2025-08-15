package registry

import (
	"github.com/mabhi256/jdiag/internal/heap/model"
)

// ObjectRegistry manages parsed object instances from INSTANCE_DUMP records
type ObjectRegistry struct {
	instances       map[model.ID]*model.GCInstanceDump
	threadInstances map[model.ID]*ThreadInstanceData // Special tracking for Thread objects
	totalInstances  int
	totalSize       uint64 // Total shallow size of all instances
}

// ThreadInstanceData represents parsed Thread object fields
type ThreadInstanceData struct {
	ObjectID          model.ID
	InstanceDump      *model.GCInstanceDump
	ThreadID          int64  // The tid field - actual thread ID
	Name              string // Resolved thread name
	Priority          int32
	Daemon            bool
	ThreadStatus      int32
	ThreadGroup       model.ID
	ParsedFieldValues map[string]interface{} // All parsed field values
}

// NewObjectRegistry creates a new object registry
func NewObjectRegistry() *ObjectRegistry {
	return &ObjectRegistry{
		instances:       make(map[model.ID]*model.GCInstanceDump),
		threadInstances: make(map[model.ID]*ThreadInstanceData),
	}
}

// AddInstance adds a parsed instance to the registry
func (r *ObjectRegistry) AddInstance(instance *model.GCInstanceDump) {
	r.instances[instance.ObjectID] = instance
	r.totalInstances++
	r.totalSize += uint64(instance.Size)
}

// AddThreadInstance adds a parsed Thread object instance with extracted field data
func (r *ObjectRegistry) AddThreadInstance(threadData *ThreadInstanceData) {
	r.threadInstances[threadData.ObjectID] = threadData
	// Also add to regular instances
	r.AddInstance(threadData.InstanceDump)
}

// GetInstance retrieves an instance by object ID
func (r *ObjectRegistry) GetInstance(objectID model.ID) (*model.GCInstanceDump, bool) {
	instance, exists := r.instances[objectID]
	return instance, exists
}

// GetThreadInstance retrieves Thread instance data by object ID
func (r *ObjectRegistry) GetThreadInstance(objectID model.ID) (*ThreadInstanceData, bool) {
	threadData, exists := r.threadInstances[objectID]
	return threadData, exists
}

// GetAllInstances returns all object instances
func (r *ObjectRegistry) GetAllInstances() map[model.ID]*model.GCInstanceDump {
	return r.instances
}

// GetAllThreadInstances returns all Thread object instances
func (r *ObjectRegistry) GetAllThreadInstances() map[model.ID]*ThreadInstanceData {
	return r.threadInstances
}

// GetCount returns the total number of instances
func (r *ObjectRegistry) GetCount() int {
	return r.totalInstances
}

// GetThreadCount returns the number of Thread instances
func (r *ObjectRegistry) GetThreadCount() int {
	return len(r.threadInstances)
}

// GetTotalSize returns the total shallow size of all instances
func (r *ObjectRegistry) GetTotalSize() uint64 {
	return r.totalSize
}

// GetInstancesByClass returns all instances of a specific class
func (r *ObjectRegistry) GetInstancesByClass(classObjectID model.ID) []*model.GCInstanceDump {
	var instances []*model.GCInstanceDump
	for _, instance := range r.instances {
		if instance.ClassObjectID == classObjectID {
			instances = append(instances, instance)
		}
	}
	return instances
}

// GetInstanceClassCounts returns a map of class ID to instance count
func (r *ObjectRegistry) GetInstanceClassCounts() map[model.ID]int {
	classCounts := make(map[model.ID]int)
	for _, instance := range r.instances {
		classCounts[instance.ClassObjectID]++
	}
	return classCounts
}

// GetClassSizeTotals returns a map of class ID to total shallow size
func (r *ObjectRegistry) GetClassSizeTotals() map[model.ID]uint64 {
	classSizes := make(map[model.ID]uint64)
	for _, instance := range r.instances {
		classSizes[instance.ClassObjectID] += uint64(instance.Size)
	}
	return classSizes
}

// Statistics returns summary statistics about the object registry
func (r *ObjectRegistry) Statistics() map[string]interface{} {
	return map[string]interface{}{
		"total_instances":  r.totalInstances,
		"total_size":       r.totalSize,
		"thread_instances": len(r.threadInstances),
		"unique_classes":   len(r.GetInstanceClassCounts()),
		"average_size":     float64(r.totalSize) / float64(r.totalInstances),
	}
}
