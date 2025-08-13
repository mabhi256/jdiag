package registry

import (
	"fmt"
	"maps"

	"github.com/mabhi256/jdiag/internal/heap/model"
)

// Manages the string table for HPROF parsing
type StringRegistry struct {
	table map[model.ID]string
}

func NewStringRegistry() *StringRegistry {
	return &StringRegistry{
		table: make(map[model.ID]string),
	}
}

func (sr *StringRegistry) Add(id model.ID, text string) {
	sr.table[id] = text
}

func (sr *StringRegistry) Get(id model.ID) (string, bool) {
	text, exists := sr.table[id]
	return text, exists
}

func (sr *StringRegistry) GetOrDefault(id model.ID, defaultVal string) string {
	if text, exists := sr.table[id]; exists {
		return text
	}
	return defaultVal
}

func (sr *StringRegistry) GetOrUnresolved(id model.ID) string {
	if text, exists := sr.table[id]; exists {
		return text
	}
	return fmt.Sprintf("<unresolved:0x%x>", uint64(id))
}

func (sr *StringRegistry) Count() int {
	return len(sr.table)
}

func (sr *StringRegistry) GetAll() map[model.ID]string {
	result := make(map[model.ID]string, len(sr.table))
	maps.Copy(result, sr.table)
	return result
}

func (sr *StringRegistry) Clear() {
	sr.table = make(map[model.ID]string)
}
