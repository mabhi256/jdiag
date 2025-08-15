package analyzer

import (
	"fmt"
	"time"

	"github.com/mabhi256/jdiag/internal/heap/model"
	"github.com/mabhi256/jdiag/internal/heap/registry"
)

// Analyzer coordinates the analysis phases using a context-based approach
type Analyzer struct {
	// Analysis context with all dependencies
	ctx *AnalysisContext

	// Analysis components (using interfaces for better testability)
	validator    ReferenceValidator
	resolver     ReferenceResolver
	graphBuilder ObjectGraphBuilder

	// Analysis results
	ValidationResult *ValidationResult
	ReferenceMap     *ReferenceMap
	ObjectGraph      *ObjectGraph

	// Analysis metadata
	startTime    time.Time
	analysisTime time.Duration
}

// NewAnalyzer creates a new heap analyzer with simplified constructor
func NewAnalyzer(stringReg *registry.StringRegistry, classDumpReg *registry.ClassDumpRegistry,
	instanceReg *registry.InstanceRegistry, arrayReg *registry.ArrayRegistry,
	rootReg *registry.GCRootRegistry, identifierSize uint32) *Analyzer {

	// Create analysis context
	ctx := NewAnalysisContext(stringReg, classDumpReg, instanceReg, arrayReg, rootReg, identifierSize)

	analyzer := &Analyzer{
		ctx: ctx,
	}

	// Initialize components with error handling
	if err := analyzer.initializeComponents(); err != nil {
		// Log error but continue - components will be nil and methods will handle gracefully
		fmt.Printf("Warning: Failed to initialize analysis components: %v\n", err)
	}

	return analyzer
}

// NewAnalyzerWithContext creates an analyzer using an existing context (useful for testing)
func NewAnalyzerWithContext(ctx *AnalysisContext) (*Analyzer, error) {
	if err := ctx.Validate(); err != nil {
		return nil, fmt.Errorf("invalid context: %w", err)
	}

	analyzer := &Analyzer{
		ctx: ctx,
	}

	if err := analyzer.initializeComponents(); err != nil {
		return nil, fmt.Errorf("failed to initialize components: %w", err)
	}

	return analyzer, nil
}

// initializeComponents sets up all analysis components with the context
func (a *Analyzer) initializeComponents() error {
	// Initialize validator
	validator := NewValidatorFinal()
	if err := validator.Initialize(a.ctx); err != nil {
		return fmt.Errorf("failed to initialize validator: %w", err)
	}
	a.validator = validator

	// Initialize resolver
	resolver := NewResolverFinal()
	if err := resolver.Initialize(a.ctx); err != nil {
		return fmt.Errorf("failed to initialize resolver: %w", err)
	}
	a.resolver = resolver

	// Initialize graph builder
	graphBuilder := NewGraphBuilder()
	if err := graphBuilder.Initialize(a.ctx); err != nil {
		return fmt.Errorf("failed to initialize graph builder: %w", err)
	}
	a.graphBuilder = graphBuilder

	return nil
}

// PerformAnalysis executes the complete Phase 11 analysis pipeline
func (a *Analyzer) PerformAnalysis() error {
	if err := a.ctx.Validate(); err != nil {
		return fmt.Errorf("invalid analysis context: %w", err)
	}

	a.startTime = time.Now()
	fmt.Println("🚀 Starting Phase 11: Data Validation & Cross-Reference Resolution")
	fmt.Println()

	// Step 11.1: Reference Validation
	if err := a.performReferenceValidation(); err != nil {
		return fmt.Errorf("reference validation failed: %w", err)
	}
	fmt.Println()

	// Step 11.2: Cross-Reference Resolution
	if err := a.performCrossReferenceResolution(); err != nil {
		return fmt.Errorf("cross-reference resolution failed: %w", err)
	}
	fmt.Println()

	// Step 11.3: Object Graph Construction
	if err := a.performObjectGraphConstruction(); err != nil {
		return fmt.Errorf("object graph construction failed: %w", err)
	}
	fmt.Println()

	// Finalize analysis
	a.finalizeAnalysis()

	return nil
}

// performReferenceValidation executes Step 11.1: Reference Validation
func (a *Analyzer) performReferenceValidation() error {
	if a.validator == nil {
		return fmt.Errorf("validator not initialized")
	}

	validation, err := a.validator.ValidateReferences()
	if err != nil {
		return err
	}

	a.ValidationResult = validation
	return nil
}

// performCrossReferenceResolution executes Step 11.2: Cross-Reference Resolution
func (a *Analyzer) performCrossReferenceResolution() error {
	if a.resolver == nil {
		return fmt.Errorf("resolver not initialized")
	}

	refMap, err := a.resolver.BuildReferenceMap()
	if err != nil {
		return err
	}

	a.ReferenceMap = refMap
	return nil
}

// performObjectGraphConstruction executes Step 11.3: Object Graph Construction
func (a *Analyzer) performObjectGraphConstruction() error {
	if a.graphBuilder == nil {
		return fmt.Errorf("graph builder not initialized")
	}

	graph, err := a.graphBuilder.BuildObjectGraph(a.ValidationResult, a.ReferenceMap)
	if err != nil {
		return err
	}

	a.ObjectGraph = graph
	return nil
}

// finalizeAnalysis completes the analysis and prints summary
func (a *Analyzer) finalizeAnalysis() {
	// Record completion time
	a.analysisTime = time.Since(a.startTime)

	// Print final summary
	a.printFinalSummary()
}

// printFinalSummary prints a comprehensive summary of Phase 11 analysis
func (a *Analyzer) printFinalSummary() {
	fmt.Println("🎯 Phase 11 Analysis Complete")
	fmt.Printf("Analysis time: %v\n", a.analysisTime)
	fmt.Println()

	// Print object graph summary
	if a.ObjectGraph != nil {
		fmt.Println(a.ObjectGraph.Summary())
	}
	fmt.Println()

	// Show success message if validation passed
	if a.ValidationResult != nil && a.ObjectGraph != nil {
		fmt.Println()
		fmt.Printf("🎉 Reference integrity: %.2f%% - validation successful!\n",
			a.ObjectGraph.getReferenceIntegrity())
	}
}

// Legacy method for backward compatibility
func (a *Analyzer) PerformPhase11Analysis() error {
	return a.PerformAnalysis()
}

// === Result Access Methods ===

// GetValidationResult returns the reference validation results
func (a *Analyzer) GetValidationResult() *ValidationResult {
	return a.ValidationResult
}

// GetReferenceMap returns the bidirectional reference map
func (a *Analyzer) GetReferenceMap() *ReferenceMap {
	return a.ReferenceMap
}

// GetObjectGraph returns the complete object graph
func (a *Analyzer) GetObjectGraph() *ObjectGraph {
	return a.ObjectGraph
}

// GetContext returns the analysis context (useful for testing)
func (a *Analyzer) GetContext() *AnalysisContext {
	return a.ctx
}

// === Status and Utility Methods ===

// IsReady checks if the analyzer is ready for advanced analysis phases
func (a *Analyzer) IsReady() bool {
	return a.ValidationResult != nil && a.ReferenceMap != nil && a.ObjectGraph != nil
}

// GetObjectReferences returns all objects referenced by the given object
func (a *Analyzer) GetObjectReferences(objectID model.ID) []model.ID {
	if a.ReferenceMap == nil {
		return nil
	}
	return a.ReferenceMap.GetReferences(objectID)
}

// GetObjectReferrers returns all objects that reference the given object
func (a *Analyzer) GetObjectReferrers(objectID model.ID) []model.ID {
	if a.ReferenceMap == nil {
		return nil
	}
	return a.ReferenceMap.GetReferrers(objectID)
}

// ObjectExists checks if an object exists in the heap
func (a *Analyzer) ObjectExists(objectID model.ID) bool {
	if a.ObjectGraph == nil {
		return false
	}
	return a.ObjectGraph.ObjectExists[objectID]
}

// === Analysis Metadata and Reporting ===

// GetAnalysisMetadata returns metadata about the analysis process
func (a *Analyzer) GetAnalysisMetadata() map[string]interface{} {
	metadata := make(map[string]interface{})

	metadata["analysis_time"] = a.analysisTime
	metadata["start_time"] = a.startTime

	if a.ValidationResult != nil {
		metadata["total_references"] = a.ValidationResult.TotalRefs
		metadata["valid_references"] = a.ValidationResult.ValidRefs
		metadata["reference_integrity"] = float64(a.ValidationResult.ValidRefs) / float64(a.ValidationResult.TotalRefs) * 100
	}

	if a.ObjectGraph != nil {
		metadata["total_objects"] = a.ObjectGraph.TotalObjects
		metadata["total_classes"] = a.ObjectGraph.TotalClasses
		metadata["total_arrays"] = a.ObjectGraph.TotalArrays
		metadata["total_refs_in_graph"] = a.ObjectGraph.TotalRefs
	}

	return metadata
}

// PrintDetailedReport prints a detailed analysis report
func (a *Analyzer) PrintDetailedReport() {
	if !a.IsReady() {
		fmt.Println("❌ Analysis not complete - run PerformAnalysis() first")
		return
	}

	fmt.Println("📊 DETAILED PHASE 11 ANALYSIS REPORT")

	// Analysis metadata
	fmt.Printf("Analysis Duration: %v\n", a.analysisTime)
	fmt.Printf("Analysis Date: %s\n", a.startTime.Format("2006-01-02 15:04:05"))
	fmt.Println()

	// Validation details
	fmt.Println("🔍 REFERENCE VALIDATION")
	if a.ValidationResult != nil {
		fmt.Printf("Total References: %d\n", a.ValidationResult.TotalRefs)
		fmt.Printf("Valid References: %d\n", a.ValidationResult.ValidRefs)
		fmt.Printf("Integrity: %.2f%%\n",
			float64(a.ValidationResult.ValidRefs)/float64(a.ValidationResult.TotalRefs)*100)
	}
	fmt.Println()

	// Object graph details
	fmt.Println("🗺️  OBJECT GRAPH")
	if a.ObjectGraph != nil {
		fmt.Println(a.ObjectGraph.Summary())
	}
	fmt.Println()
}
