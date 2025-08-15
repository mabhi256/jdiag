package analyzer

// AnalysisComponent defines the common interface for all analysis components
type AnalysisComponent interface {
	// Initialize sets up the component with the analysis context
	Initialize(ctx *AnalysisContext) error
}

// ReferenceValidator validates reference integrity across the heap
type ReferenceValidator interface {
	AnalysisComponent

	// ValidateReferences performs comprehensive reference validation
	ValidateReferences() (*ValidationResult, error)
}

// ReferenceResolver builds cross-reference maps for graph navigation
type ReferenceResolver interface {
	AnalysisComponent

	// BuildReferenceMap creates bidirectional reference mappings
	BuildReferenceMap() (*ReferenceMap, error)
}

// ObjectGraphBuilder constructs the complete navigable object graph
type ObjectGraphBuilder interface {
	AnalysisComponent

	// BuildObjectGraph creates the complete object graph from validation and reference data
	BuildObjectGraph(validation *ValidationResult, refMap *ReferenceMap) (*ObjectGraph, error)
}

// FieldProcessor handles extraction and processing of field data from instances
type FieldProcessor interface {
	AnalysisComponent

	// ExtractInstanceFieldReferences extracts object references from instance field data
	ExtractInstanceFieldReferences(instance interface{}, classDump interface{}) ([]interface{}, error)

	// ExtractInstanceFieldValues extracts all field values for detailed analysis
	ExtractInstanceFieldValues(instance interface{}, classDump interface{}) (map[string]interface{}, error)
}

// AnalysisResult represents the outcome of an analysis operation
type AnalysisResult interface {
	// IsValid returns whether the analysis completed successfully
	IsValid() bool

	// GetSummary returns a human-readable summary of the results
	GetSummary() string
}

// Ensure our concrete types implement the result interface
var (
	_ AnalysisResult = (*ValidationResult)(nil)
	_ AnalysisResult = (*ObjectGraph)(nil)
)
