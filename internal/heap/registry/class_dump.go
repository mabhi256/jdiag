package registry

import (
	"maps"

	"github.com/mabhi256/jdiag/internal/heap/model"
)

type ClassDumpRegistry struct {
	classDumps map[model.ID]*model.ClassDump // ClassObjectID -> ClassDump
	totalCount int
}

func NewClassDumpRegistry() *ClassDumpRegistry {
	return &ClassDumpRegistry{
		classDumps: make(map[model.ID]*model.ClassDump),
	}
}

func (r *ClassDumpRegistry) AddClassDump(classDump *model.ClassDump) {
	r.classDumps[classDump.ClassObjectID] = classDump
	r.totalCount++
}

func (r *ClassDumpRegistry) GetClassDump(classObjectID model.ID) (*model.ClassDump, bool) {
	classDump, exists := r.classDumps[classObjectID]
	return classDump, exists
}

func (r *ClassDumpRegistry) GetAllClassDumps() map[model.ID]*model.ClassDump {
	// Return copy to prevent modification
	result := make(map[model.ID]*model.ClassDump)
	maps.Copy(result, r.classDumps)
	return result
}

func (r *ClassDumpRegistry) GetCount() int {
	return r.totalCount
}
