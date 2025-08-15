package registry

import (
	"github.com/mabhi256/jdiag/internal/heap/model"
)

type ClassDumpRegistry struct {
	*BaseRegistry[model.ID, *model.GCClassDump]
}

func NewClassDumpRegistry() *ClassDumpRegistry {
	return &ClassDumpRegistry{
		BaseRegistry: NewBaseRegistry[model.ID, *model.GCClassDump](),
	}
}

func (r *ClassDumpRegistry) AddClassDump(classDump *model.GCClassDump) {
	r.Add(classDump.ClassObjectID, classDump)
}

func (r *ClassDumpRegistry) GetClassDump(classObjectID model.ID) (*model.GCClassDump, bool) {
	return r.Get(classObjectID)
}

func (r *ClassDumpRegistry) GetAllClassDumps() map[model.ID]*model.GCClassDump {
	return r.GetAll()
}

func (r *ClassDumpRegistry) GetCount() int {
	return r.Count()
}
