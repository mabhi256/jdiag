package analyzer

import (
	"fmt"

	"github.com/mabhi256/jdiag/internal/heap/model"
)

// ResolverFinal builds cross-reference maps using context-based dependencies
type ResolverFinal struct {
	// Analysis context
	ctx *AnalysisContext

	// Helper components
	fieldExtractor *FieldExtractor
	initialized    bool
}

// NewResolverFinal creates a new cross-reference resolver
func NewResolverFinal() *ResolverFinal {
	return &ResolverFinal{
		initialized: false,
	}
}

// Initialize sets up the resolver with the analysis context
func (r *ResolverFinal) Initialize(ctx *AnalysisContext) error {
	if ctx == nil {
		return fmt.Errorf("analysis context is required")
	}

	if err := ctx.Validate(); err != nil {
		return fmt.Errorf("invalid context: %w", err)
	}

	r.ctx = ctx
	r.fieldExtractor = NewFieldExtractor(ctx)
	r.initialized = true

	return nil
}

// BuildReferenceMap creates bidirectional reference maps for graph navigation
func (r *ResolverFinal) BuildReferenceMap() (*ReferenceMap, error) {
	if !r.initialized {
		return nil, fmt.Errorf("resolver not initialized - call Initialize() first")
	}

	fmt.Println("🔗 Phase 11.2: Building cross-reference maps (VisualVM-compatible)...")

	refMap := NewReferenceMap()

	// Build references using a structured approach
	if err := r.buildAllReferences(refMap); err != nil {
		return nil, fmt.Errorf("failed to build references: %w", err)
	}

	// Print summary
	r.printReferenceSummary(refMap)

	return refMap, nil
}

// buildAllReferences builds all types of references in the heap
func (r *ResolverFinal) buildAllReferences(refMap *ReferenceMap) error {
	referenceBuilders := []struct {
		name string
		fn   func(*ReferenceMap) error
	}{
		{"instance references", r.buildInstanceReferences},
		{"object array references", r.buildObjectArrayReferences},
		// {"primitive array references", r.buildPrimitiveArrayReferences},
		{"GC root references", r.buildGCRootReferences},
		{"class references", r.buildClassReferences},
	}

	for _, builder := range referenceBuilders {
		fmt.Printf("  Building %s...\n", builder.name)
		if err := builder.fn(refMap); err != nil {
			return fmt.Errorf("failed to build %s: %w", builder.name, err)
		}
	}

	return nil
}

// buildInstanceReferences builds references from object instances to other objects
func (r *ResolverFinal) buildInstanceReferences(refMap *ReferenceMap) error {
	allInstances := r.ctx.InstanceReg.GetAllInstances()
	for instanceID, instance := range allInstances {
		// Add class reference (instance -> class)
		if instance.ClassObjectID != 0 {
			refMap.AddReference(instanceID, instance.ClassObjectID)
		}

		// Get class definition for field layout
		classDump, hasClass := r.ctx.ClassDumpReg.GetClassDump(instance.ClassObjectID)
		if !hasClass {
			continue // Skip instances without class definitions
		}

		// Extract and process field references
		fieldRefs, err := r.fieldExtractor.ExtractInstanceFieldReferences(instance, classDump)
		if err != nil {
			continue // Skip instances we can't parse
		}

		// Add each field reference (instance -> referenced object)
		for _, targetID := range fieldRefs {
			if targetID != 0 {
				refMap.AddReference(instanceID, targetID)
			}
		}
	}

	return nil
}

// buildObjectArrayReferences builds references from object arrays to their elements
func (r *ResolverFinal) buildObjectArrayReferences(refMap *ReferenceMap) error {
	allObjectArrays := r.ctx.ArrayReg.GetAllObjectArrays()
	for arrayID, array := range allObjectArrays {
		// Add class reference (array -> array class)
		if array.ClassID != 0 {
			refMap.AddReference(arrayID, array.ClassID)
		}

		// Add element references (array -> each element)
		for _, elementID := range array.Elements {
			if elementID != 0 {
				refMap.AddReference(arrayID, elementID)
			}
		}
	}

	return nil
}

// // buildPrimitiveArrayReferences builds references from primitive arrays to their class objects
// func (r *ResolverFinal) buildPrimitiveArrayReferences(refMap *ReferenceMap) error {
// 	allPrimitiveArrays := r.ctx.ArrayReg.GetAllPrimitiveArrays()
// 	for _, array := range allPrimitiveArrays {
// 		// Note: Primitive arrays in HPROF format don't store explicit class object IDs
// 		// The class is implicitly determined by the element type (int[], char[], etc.)
// 		// Some heap analysis tools create synthetic class objects for primitive array types,
// 		// but this is not standard in the HPROF format.

// 		// For now, we skip class references for primitive arrays as they don't
// 		// have explicit class object IDs in the heap dump.
// 		_ = array // Use array to avoid unused variable warning
// 	}

// 	return nil
// }

// buildGCRootReferences builds references from GC roots to referenced objects
func (r *ResolverFinal) buildGCRootReferences(refMap *ReferenceMap) error {
	allRoots := r.ctx.RootReg.GetAllRoots()
	for _, root := range allRoots {
		// GC roots reference their target objects
		if root.ObjectID != 0 {
			// Use a special ID for GC roots (they don't have object IDs)
			// In practice, you might want to handle this differently
			// For now, we'll add the reference without a source ID
			// This maintains the original behavior
			refMap.AddReference(0, root.ObjectID) // 0 represents "GC root space"
		}
	}

	return nil
}

// buildClassReferences builds references from classes to other objects
func (r *ResolverFinal) buildClassReferences(refMap *ReferenceMap) error {
	allClasses := r.ctx.ClassDumpReg.GetAllClassDumps()
	for classID, classDump := range allClasses {
		// Add superclass reference (class -> superclass)
		if classDump.SuperClassObjectID != 0 {
			refMap.AddReference(classID, classDump.SuperClassObjectID)
		}

		// Add static field object references
		for _, staticField := range classDump.StaticFields {
			if staticField.Type == model.HPROF_NORMAL_OBJECT {
				objectID := r.extractObjectIDFromStaticField(staticField)
				if objectID != 0 {
					refMap.AddReference(classID, objectID)
				}
			}
		}

		// Note: String references (class names, field names) are not added to the object graph
		// as they represent metadata rather than heap object relationships
	}

	return nil
}

// extractObjectIDFromStaticField extracts object ID from static field value
func (r *ResolverFinal) extractObjectIDFromStaticField(staticField *model.StaticField) model.ID {
	if staticField.Type != model.HPROF_NORMAL_OBJECT || len(staticField.Value) < int(r.ctx.Config.IdentifierSize) {
		return 0
	}

	// Use consistent parsing logic with field extractor
	objectID, err := r.fieldExtractor.parseObjectReference(staticField.Value)
	if err != nil {
		return 0
	}

	return objectID
}

// printReferenceSummary prints a comprehensive summary of the reference mapping results
func (r *ResolverFinal) printReferenceSummary(refMap *ReferenceMap) {
	stats := r.calculateReferenceStatistics(refMap)

	fmt.Printf("  Cross-reference mapping completed:\n")
	fmt.Printf("    Objects with outgoing references: %d\n", stats.ObjectsWithReferences)
	fmt.Printf("    Total forward references: %d\n", stats.TotalForwardRefs)
	fmt.Printf("    Total backward references: %d\n", stats.TotalBackwardRefs)

	if stats.ObjectsWithReferences > 0 {
		fmt.Printf("    Average references per object: %.2f\n", stats.AvgReferencesPerObject)
	}

	if stats.MaxOutgoing > 0 {
		fmt.Printf("    Most outgoing refs: Object 0x%x (%d references)\n",
			uint64(stats.MaxOutgoingID), stats.MaxOutgoing)
	}

	if stats.MaxIncoming > 0 {
		fmt.Printf("    Most incoming refs: Object 0x%x (%d referrers)\n",
			uint64(stats.MaxIncomingID), stats.MaxIncoming)
	}

	fmt.Printf("    ✅ Reference mapping complete\n")
}

// ReferenceStatistics holds statistics about the reference mapping
type ReferenceStatistics struct {
	ObjectsWithReferences  int
	TotalForwardRefs       int
	TotalBackwardRefs      int
	AvgReferencesPerObject float64
	MaxOutgoing            int
	MaxIncoming            int
	MaxOutgoingID          model.ID
	MaxIncomingID          model.ID
}

// calculateReferenceStatistics computes comprehensive statistics about the reference map
func (r *ResolverFinal) calculateReferenceStatistics(refMap *ReferenceMap) *ReferenceStatistics {
	stats := &ReferenceStatistics{
		ObjectsWithReferences: len(refMap.ForwardRefs),
	}

	// Calculate totals
	for _, refs := range refMap.ForwardRefs {
		stats.TotalForwardRefs += len(refs)
	}

	for _, refs := range refMap.BackwardRefs {
		stats.TotalBackwardRefs += len(refs)
	}

	// Calculate average
	if stats.ObjectsWithReferences > 0 {
		stats.AvgReferencesPerObject = float64(stats.TotalForwardRefs) / float64(stats.ObjectsWithReferences)
	}

	// Find maximums
	for objID, count := range refMap.OutgoingCount {
		if count > stats.MaxOutgoing {
			stats.MaxOutgoing = count
			stats.MaxOutgoingID = objID
		}
	}

	for objID, count := range refMap.IncomingCount {
		if count > stats.MaxIncoming {
			stats.MaxIncoming = count
			stats.MaxIncomingID = objID
		}
	}

	return stats
}
