package registry

import (
	"maps"

	"github.com/mabhi256/jdiag/internal/heap/model"
)

type ThreadInfo struct {
	StartRecord           *model.StartThreadBody
	ThreadName            string // Resolved from string table
	ThreadGroupName       string // Resolved from string table
	ParentThreadGroupName string // Resolved from string table
	IsActive              bool
}

type ThreadRegistry struct {
	threadsBySerial map[model.SerialNum]*ThreadInfo
	threadsByID     map[model.ID]*ThreadInfo

	// Statistics
	startedCount   int
	completedCount int
}

func NewThreadRegistry() *ThreadRegistry {
	return &ThreadRegistry{
		threadsBySerial: make(map[model.SerialNum]*ThreadInfo),
		threadsByID:     make(map[model.ID]*ThreadInfo),
	}
}

func (tr *ThreadRegistry) StartThread(thread *model.StartThreadBody, stringReg *StringRegistry) {
	// Create thread info
	threadInfo := &ThreadInfo{
		StartRecord:           thread,
		ThreadName:            stringReg.GetOrUnresolved(thread.ThreadNameID),
		ThreadGroupName:       stringReg.GetOrUnresolved(thread.ThreadGroupNameID),
		ParentThreadGroupName: stringReg.GetOrUnresolved(thread.ParentThreadGroupNameID),
		IsActive:              true,
	}

	// Store in both maps
	tr.threadsBySerial[thread.ThreadSerialNumber] = threadInfo
	if thread.ThreadObjectID != 0 {
		tr.threadsByID[thread.ThreadObjectID] = threadInfo
	}
	tr.startedCount++
}

func (tr *ThreadRegistry) EndThread(thread *model.EndThreadBody) {
	if threadInfo, exists := tr.threadsBySerial[thread.ThreadSerialNumber]; exists {
		threadInfo.IsActive = false
		tr.startedCount--
		tr.completedCount++
	}
}

func (tr *ThreadRegistry) GetBySerial(serialNum model.SerialNum) (*ThreadInfo, bool) {
	threadInfo, exists := tr.threadsBySerial[serialNum]
	return threadInfo, exists
}

func (tr *ThreadRegistry) GetByObjectID(objectID model.ID) (*ThreadInfo, bool) {
	threadInfo, exists := tr.threadsByID[objectID]
	return threadInfo, exists
}

func (tr *ThreadRegistry) GetAllThreads() map[model.SerialNum]*ThreadInfo {
	result := make(map[model.SerialNum]*ThreadInfo)
	maps.Copy(result, tr.threadsBySerial)
	return result
}

func (tr *ThreadRegistry) GetActiveThreads() []*ThreadInfo {
	var active []*ThreadInfo
	for _, threadInfo := range tr.threadsBySerial {
		if threadInfo.IsActive {
			active = append(active, threadInfo)
		}
	}
	return active
}

func (tr *ThreadRegistry) GetCompletedThreads() []*ThreadInfo {
	var completed []*ThreadInfo
	for _, threadInfo := range tr.threadsBySerial {
		if !threadInfo.IsActive {
			completed = append(completed, threadInfo)
		}
	}
	return completed
}

func (tr *ThreadRegistry) Count() int {
	return len(tr.threadsBySerial)
}
