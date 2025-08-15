package registry

import (
	"fmt"

	"github.com/mabhi256/jdiag/internal/heap/model"
)

type StringRegistry struct {
	*BaseRegistry[model.ID, string]
}

func NewStringRegistry() *StringRegistry {
	return &StringRegistry{
		BaseRegistry: NewBaseRegistry[model.ID, string](),
	}
}

func (r *StringRegistry) GetOrDefault(id model.ID, defaultVal string) string {
	if text, exists := r.Get(id); exists {
		return text
	}
	return defaultVal
}

func (r *StringRegistry) GetOrUnresolved(id model.ID) string {
	if text, exists := r.Get(id); exists {
		return text
	}
	return fmt.Sprintf("<unresolved:0x%x>", uint64(id))
}
