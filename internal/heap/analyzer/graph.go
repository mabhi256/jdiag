package analyzer

import (
	"fmt"
)

// GraphBuilder builds the complete object graph using context-based dependencies
type GraphBuilder struct {
	// Analysis context
	ctx *AnalysisContext

	// Initialization state
	initialized bool
}

// NewGraphBuilder creates a new object graph builder
func NewGraphBuilder() *GraphBuilder {
	return &GraphBuilder{
		initialized: false,
	}
}

// Initialize sets up the graph builder with the analysis context
func (gb *GraphBuilder) Initialize(ctx *AnalysisContext) error {
	if ctx == nil {
		return fmt.Errorf("analysis context is required")
	}

	if err := ctx.Validate(); err != nil {
		return fmt.Errorf("invalid context: %w", err)
	}

	gb.ctx = ctx
	gb.initialized = true

	return nil
}

// BuildObjectGraph creates the complete navigable object graph
func (gb *GraphBuilder) BuildObjectGraph(validation *ValidationResult, refMap *ReferenceMap) (*ObjectGraph, error) {
	if !gb.initialized {
		return nil, fmt.Errorf("graph builder not initialized - call Initialize() first")
	}

	if validation == nil {
		return nil, fmt.Errorf("validation result is required")
	}

	if refMap == nil {
		return nil, fmt.Errorf("reference map is required")
	}

	fmt.Println("📊 Phase 11.3: Building navigable object graph...")

	// Create and initialize the graph
	graph := NewObjectGraph()
	graph.References = refMap
	graph.Validation = validation

	// Build the graph in stages
	if err := gb.buildGraphStages(graph); err != nil {
		return nil, fmt.Errorf("failed to build graph: %w", err)
	}

	// Print final summary
	gb.printGraphSummary(graph)

	return graph, nil
}

// buildGraphStages executes all graph building stages
func (gb *GraphBuilder) buildGraphStages(graph *ObjectGraph) error {
	buildStages := []struct {
		name string
		fn   func(*ObjectGraph) error
	}{
		{"object existence maps", gb.buildObjectMaps},
		{"graph statistics", gb.calculateStatistics},
		{"consistency checks", gb.performConsistencyChecks},
	}

	for _, stage := range buildStages {
		fmt.Printf("  %s...\n", stage.name)
		if err := stage.fn(graph); err != nil {
			return fmt.Errorf("failed during %s: %w", stage.name, err)
		}
	}

	return nil
}

// buildObjectMaps builds fast lookup maps for object existence checks
func (gb *GraphBuilder) buildObjectMaps(graph *ObjectGraph) error {
	// Build instance existence map
	if err := gb.buildInstanceMaps(graph); err != nil {
		return fmt.Errorf("failed to build instance maps: %w", err)
	}

	// Build array existence maps
	if err := gb.buildArrayMaps(graph); err != nil {
		return fmt.Errorf("failed to build array maps: %w", err)
	}

	// Build class existence map
	if err := gb.buildClassMaps(graph); err != nil {
		return fmt.Errorf("failed to build class maps: %w", err)
	}

	return nil
}

// buildInstanceMaps builds existence maps for instance objects
func (gb *GraphBuilder) buildInstanceMaps(graph *ObjectGraph) error {
	allInstances := gb.ctx.InstanceReg.GetAllInstances()
	for objectID := range allInstances {
		graph.ObjectExists[objectID] = true
	}

	return nil
}

// buildArrayMaps builds existence maps for array objects
func (gb *GraphBuilder) buildArrayMaps(graph *ObjectGraph) error {
	// Build object array existence map
	allObjectArrays := gb.ctx.ArrayReg.GetAllObjectArrays()
	for objectID := range allObjectArrays {
		graph.ObjectExists[objectID] = true
		graph.ArrayExists[objectID] = true
	}

	// Build primitive array existence map
	allPrimitiveArrays := gb.ctx.ArrayReg.GetAllPrimitiveArrays()
	for objectID := range allPrimitiveArrays {
		graph.ObjectExists[objectID] = true
		graph.ArrayExists[objectID] = true
	}

	return nil
}

// buildClassMaps builds existence maps for class objects
func (gb *GraphBuilder) buildClassMaps(graph *ObjectGraph) error {
	allClasses := gb.ctx.ClassDumpReg.GetAllClassDumps()
	for objectID := range allClasses {
		graph.ObjectExists[objectID] = true
		graph.ClassExists[objectID] = true
	}

	return nil
}

// calculateStatistics computes comprehensive graph statistics
func (gb *GraphBuilder) calculateStatistics(graph *ObjectGraph) error {
	stats := &GraphStatistics{}

	// Count different object types
	stats.TotalObjects = len(graph.ObjectExists)
	stats.TotalClasses = len(graph.ClassExists)
	stats.TotalArrays = len(graph.ArrayExists)

	// Count references
	for _, refs := range graph.References.ForwardRefs {
		stats.TotalRefs += len(refs)
	}

	// Calculate instances (total objects minus classes and arrays)
	stats.TotalInstances = stats.TotalObjects - stats.TotalClasses - stats.TotalArrays

	// Update graph with calculated statistics
	graph.TotalObjects = stats.TotalObjects
	graph.TotalClasses = stats.TotalClasses
	graph.TotalArrays = stats.TotalArrays
	graph.TotalRefs = stats.TotalRefs

	return nil
}

// GraphStatistics holds comprehensive statistics about the object graph
type GraphStatistics struct {
	TotalObjects   int
	TotalInstances int
	TotalClasses   int
	TotalArrays    int
	TotalRefs      int
}

// performConsistencyChecks validates the integrity of the constructed graph
func (gb *GraphBuilder) performConsistencyChecks(graph *ObjectGraph) error {
	checks := []struct {
		name string
		fn   func(*ObjectGraph) error
	}{
		{"reference consistency", gb.checkReferenceConsistency},
		{"object existence consistency", gb.checkObjectExistenceConsistency},
		{"statistical consistency", gb.checkStatisticalConsistency},
	}

	for _, check := range checks {
		if err := check.fn(graph); err != nil {
			return fmt.Errorf("%s failed: %w", check.name, err)
		}
	}

	return nil
}

// checkReferenceConsistency validates that forward and backward references are consistent
func (gb *GraphBuilder) checkReferenceConsistency(graph *ObjectGraph) error {
	// Verify forward/backward reference symmetry
	inconsistencies := 0

	for sourceID, targets := range graph.References.ForwardRefs {
		for _, targetID := range targets {
			// Check if backward reference exists
			found := false
			for _, backRef := range graph.References.BackwardRefs[targetID] {
				if backRef == sourceID {
					found = true
					break
				}
			}
			if !found {
				inconsistencies++
				if inconsistencies <= 5 { // Limit error reporting
					fmt.Printf("    Warning: Missing backward reference %x <- %x\n",
						uint64(targetID), uint64(sourceID))
				}
			}
		}
	}

	if inconsistencies > 0 {
		fmt.Printf("    Found %d reference inconsistencies\n", inconsistencies)
	}

	return nil
}

// checkObjectExistenceConsistency validates that referenced objects exist
func (gb *GraphBuilder) checkObjectExistenceConsistency(graph *ObjectGraph) error {
	missingObjects := 0

	for sourceID, targets := range graph.References.ForwardRefs {
		for _, targetID := range targets {
			if targetID != 0 && !graph.ObjectExists[targetID] {
				missingObjects++
				if missingObjects <= 5 { // Limit error reporting
					fmt.Printf("    Warning: Reference to non-existent object: %x -> %x\n",
						uint64(sourceID), uint64(targetID))
				}
			}
		}
	}

	if missingObjects > 0 {
		fmt.Printf("    Found %d references to missing objects\n", missingObjects)
	}

	return nil
}

// checkStatisticalConsistency validates that calculated statistics make sense
func (gb *GraphBuilder) checkStatisticalConsistency(graph *ObjectGraph) error {
	// Basic sanity checks
	if graph.TotalObjects < 0 {
		return fmt.Errorf("negative total objects: %d", graph.TotalObjects)
	}

	if graph.TotalClasses < 0 {
		return fmt.Errorf("negative total classes: %d", graph.TotalClasses)
	}

	if graph.TotalArrays < 0 {
		return fmt.Errorf("negative total arrays: %d", graph.TotalArrays)
	}

	if graph.TotalRefs < 0 {
		return fmt.Errorf("negative total references: %d", graph.TotalRefs)
	}

	// Logical consistency checks
	totalCounted := graph.TotalClasses + graph.TotalArrays
	totalInstances := graph.TotalObjects - totalCounted

	if totalInstances < 0 {
		fmt.Printf("    Warning: Inconsistent object counts - instances: %d, classes: %d, arrays: %d\n",
			totalInstances, graph.TotalClasses, graph.TotalArrays)
	}

	return nil
}

// printGraphSummary prints a comprehensive summary of the constructed graph
func (gb *GraphBuilder) printGraphSummary(graph *ObjectGraph) {
	fmt.Printf("  Object graph construction completed:\n")
	fmt.Printf("    Total objects: %d\n", graph.TotalObjects)
	fmt.Printf("    Classes: %d\n", graph.TotalClasses)
	fmt.Printf("    Arrays: %d\n", graph.TotalArrays)
	fmt.Printf("    Instances: %d\n", graph.TotalObjects-graph.TotalClasses-graph.TotalArrays)
	fmt.Printf("    Total references: %d\n", graph.TotalRefs)

	// Calculate and display reference density
	if graph.TotalObjects > 0 {
		density := float64(graph.TotalRefs) / float64(graph.TotalObjects)
		fmt.Printf("    Reference density: %.2f refs/object\n", density)
	}

	// Display validation summary if available
	if graph.Validation != nil && graph.Validation.TotalRefs > 0 {
		integrity := float64(graph.Validation.ValidRefs) / float64(graph.Validation.TotalRefs) * 100
		fmt.Printf("    Reference integrity: %.2f%%\n", integrity)
	}

	fmt.Printf("    ✅ Object graph ready for analysis\n")
}
