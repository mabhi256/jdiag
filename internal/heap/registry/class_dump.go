package registry

import (
	"github.com/mabhi256/jdiag/internal/heap/model"
)

type ClassDumpRegistry struct {
	*BaseRegistry[model.ID, *model.ClassDump]
}

func NewClassDumpRegistry() *ClassDumpRegistry {
	return &ClassDumpRegistry{
		BaseRegistry: NewBaseRegistry[model.ID, *model.ClassDump](),
	}
}

func (r *ClassDumpRegistry) AddClassDump(classDump *model.ClassDump) {
	r.Add(classDump.ClassObjectID, classDump)
}

func (r *ClassDumpRegistry) GetClassDump(classObjectID model.ID) (*model.ClassDump, bool) {
	return r.Get(classObjectID)
}

func (r *ClassDumpRegistry) GetAllClassDumps() map[model.ID]*model.ClassDump {
	return r.GetAll()
}

func (r *ClassDumpRegistry) GetCount() int {
	return r.Count()
}
