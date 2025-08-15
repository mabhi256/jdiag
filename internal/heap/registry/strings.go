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

func (r *StringRegistry) AddString(stringID model.ID, value string) {
	r.Add(stringID, value)
}

func (r *StringRegistry) GetString(stringID model.ID) (string, bool) {
	return r.Get(stringID)
}

func (r *StringRegistry) GetAllStrings() map[model.ID]string {
	return r.GetAll()
}

// HasString checks if a string ID exists in the registry
func (r *StringRegistry) HasString(stringID model.ID) bool {
	_, exists := r.Get(stringID)
	return exists
}

// GetOrUnresolved returns the string value or a fallback for unresolved IDs
func (r *StringRegistry) GetOrUnresolved(stringID model.ID) string {
	if str, exists := r.Get(stringID); exists {
		return str
	}
	return fmt.Sprintf("unresolved_string_0x%x", uint64(stringID))
}

func (r *StringRegistry) GetCount() int {
	return r.Count()
}

func (r *StringRegistry) Statistics() map[string]interface{} {
	allStrings := r.GetAllStrings()
	totalLength := 0
	maxLength := 0
	minLength := int(^uint(0) >> 1) // Max int

	for _, str := range allStrings {
		length := len(str)
		totalLength += length
		if length > maxLength {
			maxLength = length
		}
		if length < minLength {
			minLength = length
		}
	}

	count := len(allStrings)
	avgLength := 0.0
	if count > 0 {
		avgLength = float64(totalLength) / float64(count)
	}

	return map[string]interface{}{
		"total_strings":  count,
		"total_length":   totalLength,
		"average_length": avgLength,
		"max_length":     maxLength,
		"min_length":     minLength,
	}
}
