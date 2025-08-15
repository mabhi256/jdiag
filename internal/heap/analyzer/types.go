package analyzer

import (
	"fmt"

	"github.com/mabhi256/jdiag/internal/heap/model"
)

// ReferenceMap tracks bidirectional object references for graph traversal
// This matches VisualVM's approach but uses in-memory maps instead of file offsets
type ReferenceMap struct {
	// Forward references: ID -> list of objects it references
	ForwardRefs map[model.ID][]model.ID

	// Backward references: ID -> list of objects that reference it
	BackwardRefs map[model.ID][]model.ID

	// Reference counts for quick lookups
	OutgoingCount map[model.ID]int
	IncomingCount map[model.ID]int
}

// NewReferenceMap creates a new bidirectional reference map
func NewReferenceMap() *ReferenceMap {
	return &ReferenceMap{
		ForwardRefs:   make(map[model.ID][]model.ID),
		BackwardRefs:  make(map[model.ID][]model.ID),
		OutgoingCount: make(map[model.ID]int),
		IncomingCount: make(map[model.ID]int),
	}
}

// AddReference adds a reference from source to target object
func (rm *ReferenceMap) AddReference(source, target model.ID) {
	// Add forward reference
	rm.ForwardRefs[source] = append(rm.ForwardRefs[source], target)
	rm.OutgoingCount[source]++

	// Add backward reference
	rm.BackwardRefs[target] = append(rm.BackwardRefs[target], source)
	rm.IncomingCount[target]++
}

// GetReferences returns all objects that the given object references
func (rm *ReferenceMap) GetReferences(objectID model.ID) []model.ID {
	return rm.ForwardRefs[objectID]
}

// GetReferrers returns all objects that reference the given object
func (rm *ReferenceMap) GetReferrers(objectID model.ID) []model.ID {
	return rm.BackwardRefs[objectID]
}

// HasReferences checks if an object has any outgoing references
func (rm *ReferenceMap) HasReferences(objectID model.ID) bool {
	return rm.OutgoingCount[objectID] > 0
}

// HasReferrers checks if an object has any incoming references
func (rm *ReferenceMap) HasReferrers(objectID model.ID) bool {
	return rm.IncomingCount[objectID] > 0
}

// GetStatistics returns comprehensive statistics about the reference map
func (rm *ReferenceMap) GetStatistics() map[string]interface{} {
	stats := make(map[string]interface{})

	totalForwardRefs := 0
	for _, refs := range rm.ForwardRefs {
		totalForwardRefs += len(refs)
	}

	stats["objects_with_references"] = len(rm.ForwardRefs)
	stats["total_forward_references"] = totalForwardRefs
	stats["total_backward_references"] = len(rm.BackwardRefs)

	return stats
}

// ValidationResult represents the result of reference validation
type ValidationResult struct {
	Valid           bool       // Overall validation status
	MissingObjects  []model.ID // Objects referenced but not found
	OrphanedObjects []model.ID // Objects with no references (future use)
	TotalRefs       int        // Total references checked
	ValidRefs       int        // Number of valid references
}

// IsValid returns whether the validation completed successfully (implements AnalysisResult)
func (vr *ValidationResult) IsValid() bool {
	return vr.Valid
}

// GetSummary returns a human-readable summary of the validation results (implements AnalysisResult)
func (vr *ValidationResult) GetSummary() string {
	if vr.TotalRefs == 0 {
		return "No references to validate"
	}

	integrity := float64(vr.ValidRefs) / float64(vr.TotalRefs) * 100

	summary := fmt.Sprintf("Validation Results: %d/%d references valid (%.2f%% integrity)",
		vr.ValidRefs, vr.TotalRefs, integrity)

	if len(vr.MissingObjects) > 0 {
		summary += fmt.Sprintf(", %d missing objects", len(vr.MissingObjects))
	}

	if vr.Valid {
		summary += " - PASSED"
	} else {
		summary += " - FAILED"
	}

	return summary
}

// GetIntegrityPercentage calculates the reference integrity as a percentage
func (vr *ValidationResult) GetIntegrityPercentage() float64 {
	if vr.TotalRefs == 0 {
		return 100.0
	}
	return float64(vr.ValidRefs) / float64(vr.TotalRefs) * 100
}

// ObjectGraph represents the navigable object relationship graph
type ObjectGraph struct {
	References *ReferenceMap     // Bidirectional reference mappings
	Validation *ValidationResult // Reference validation results

	// Quick lookup sets for object existence checks
	ObjectExists map[model.ID]bool // All objects (instances, arrays, classes)
	ClassExists  map[model.ID]bool // Class objects only
	ArrayExists  map[model.ID]bool // Array objects only

	// Statistics
	TotalObjects int // Total number of objects in the graph
	TotalClasses int // Total number of classes
	TotalArrays  int // Total number of arrays
	TotalRefs    int // Total number of references
}

// NewObjectGraph creates a new object graph
func NewObjectGraph() *ObjectGraph {
	return &ObjectGraph{
		ObjectExists: make(map[model.ID]bool),
		ClassExists:  make(map[model.ID]bool),
		ArrayExists:  make(map[model.ID]bool),
	}
}

// IsValid returns whether the graph is valid and ready for analysis (implements AnalysisResult)
func (og *ObjectGraph) IsValid() bool {
	return og.References != nil && og.Validation != nil &&
		len(og.ObjectExists) > 0 && og.TotalObjects > 0
}

// GetSummary returns a human-readable summary of the object graph (implements AnalysisResult)
func (og *ObjectGraph) GetSummary() string {
	if !og.IsValid() {
		return "Object graph is not valid or not ready"
	}

	instanceCount := og.TotalObjects - og.TotalClasses - og.TotalArrays

	summary := fmt.Sprintf("Object Graph: %d total objects (%d instances, %d classes, %d arrays), %d references",
		og.TotalObjects, instanceCount, og.TotalClasses, og.TotalArrays, og.TotalRefs)

	if og.Validation != nil {
		integrity := og.Validation.GetIntegrityPercentage()
		summary += fmt.Sprintf(", %.2f%% reference integrity", integrity)
	}

	return summary
}

// Summary returns a detailed summary of the object graph (legacy method for backward compatibility)
func (og *ObjectGraph) Summary() string {
	return og.GetSummary()
}

// getReferenceIntegrity calculates the reference integrity percentage
func (og *ObjectGraph) getReferenceIntegrity() float64 {
	if og.Validation == nil {
		return 0.0
	}
	return og.Validation.GetIntegrityPercentage()
}

// GetObjectStatistics returns comprehensive statistics about the objects in the graph
func (og *ObjectGraph) GetObjectStatistics() map[string]interface{} {
	stats := make(map[string]interface{})

	stats["total_objects"] = og.TotalObjects
	stats["total_classes"] = og.TotalClasses
	stats["total_arrays"] = og.TotalArrays
	stats["total_instances"] = og.TotalObjects - og.TotalClasses - og.TotalArrays
	stats["total_references"] = og.TotalRefs

	if og.TotalObjects > 0 {
		stats["reference_density"] = float64(og.TotalRefs) / float64(og.TotalObjects)
	}

	if og.Validation != nil {
		stats["reference_integrity"] = og.Validation.GetIntegrityPercentage()
		stats["valid_references"] = og.Validation.ValidRefs
		stats["total_validated_references"] = og.Validation.TotalRefs
		stats["missing_objects_count"] = len(og.Validation.MissingObjects)
	}

	return stats
}

// ContainsObject checks if an object exists in the graph
func (og *ObjectGraph) ContainsObject(objectID model.ID) bool {
	return og.ObjectExists[objectID]
}

// ContainsClass checks if a class exists in the graph
func (og *ObjectGraph) ContainsClass(classID model.ID) bool {
	return og.ClassExists[classID]
}

// ContainsArray checks if an array exists in the graph
func (og *ObjectGraph) ContainsArray(arrayID model.ID) bool {
	return og.ArrayExists[arrayID]
}

// GetReferenceDensity calculates the average number of references per object
func (og *ObjectGraph) GetReferenceDensity() float64 {
	if og.TotalObjects == 0 {
		return 0.0
	}
	return float64(og.TotalRefs) / float64(og.TotalObjects)
}

// Validate performs basic validation of the object graph structure
func (og *ObjectGraph) Validate() error {
	if og.References == nil {
		return fmt.Errorf("reference map is nil")
	}

	if og.ObjectExists == nil {
		return fmt.Errorf("object existence map is nil")
	}

	if og.TotalObjects < 0 {
		return fmt.Errorf("invalid total objects count: %d", og.TotalObjects)
	}

	if og.TotalClasses < 0 {
		return fmt.Errorf("invalid total classes count: %d", og.TotalClasses)
	}

	if og.TotalArrays < 0 {
		return fmt.Errorf("invalid total arrays count: %d", og.TotalArrays)
	}

	if og.TotalRefs < 0 {
		return fmt.Errorf("invalid total references count: %d", og.TotalRefs)
	}

	return nil
}
