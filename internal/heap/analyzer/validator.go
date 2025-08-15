package analyzer

import (
	"fmt"

	"github.com/mabhi256/jdiag/internal/heap/model"
)

// ValidatorFinal validates reference integrity using context-based dependencies
type ValidatorFinal struct {
	// Analysis context
	ctx *AnalysisContext

	// Helper components
	fieldExtractor *FieldExtractor
	initialized    bool
}

// NewValidatorFinal creates a new reference validator
func NewValidatorFinal() *ValidatorFinal {
	return &ValidatorFinal{
		initialized: false,
	}
}

// Initialize sets up the validator with the analysis context
func (v *ValidatorFinal) Initialize(ctx *AnalysisContext) error {
	if ctx == nil {
		return fmt.Errorf("analysis context is required")
	}

	if err := ctx.Validate(); err != nil {
		return fmt.Errorf("invalid context: %w", err)
	}

	v.ctx = ctx
	v.fieldExtractor = NewFieldExtractor(ctx)
	v.initialized = true

	return nil
}

// ValidateReferences performs comprehensive reference validation
func (v *ValidatorFinal) ValidateReferences() (*ValidationResult, error) {
	if !v.initialized {
		return nil, fmt.Errorf("validator not initialized - call Initialize() first")
	}

	result := &ValidationResult{
		Valid: true,
	}

	fmt.Println("🔍 Phase 11.1: Validating object references...")

	// Build existence maps for validation
	objectExists, stringExists := v.buildExistenceMaps()

	// Validate different types of references
	if err := v.validateAllReferences(result, objectExists, stringExists); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Print validation summary
	v.printValidationSummary(result)

	return result, nil
}

// buildExistenceMaps creates lookup maps for object and string existence
func (v *ValidatorFinal) buildExistenceMaps() (map[model.ID]bool, map[model.ID]bool) {
	objectExists := v.buildObjectExistenceMap()
	stringExists := v.buildStringExistenceMap()
	return objectExists, stringExists
}

// validateAllReferences performs validation on all reference types
func (v *ValidatorFinal) validateAllReferences(result *ValidationResult, objectExists, stringExists map[model.ID]bool) error {
	validationSteps := []struct {
		name string
		fn   func(*ValidationResult, map[model.ID]bool) error
	}{
		{"instance field references", func(r *ValidationResult, oe map[model.ID]bool) error {
			return v.validateInstanceReferences(r, oe)
		}},
		{"object array element references", func(r *ValidationResult, oe map[model.ID]bool) error {
			return v.validateObjectArrayReferences(r, oe)
		}},
		// {"primitive array class references", func(r *ValidationResult, oe map[model.ID]bool) error {
		// 	return v.validatePrimitiveArrayReferences(r, oe)
		// }},
		{"GC root references", func(r *ValidationResult, oe map[model.ID]bool) error {
			return v.validateGCRootReferences(r, oe)
		}},
	}

	for _, step := range validationSteps {
		fmt.Printf("  Validating %s...\n", step.name)
		if err := step.fn(result, objectExists); err != nil {
			return fmt.Errorf("failed to validate %s: %w", step.name, err)
		}
	}

	// Validate class references (uses both object and string existence maps)
	fmt.Println("  Validating class references...")
	if err := v.validateClassReferences(result, objectExists, stringExists); err != nil {
		return fmt.Errorf("failed to validate class references: %w", err)
	}

	return nil
}

// buildObjectExistenceMap creates a fast lookup map for object existence
func (v *ValidatorFinal) buildObjectExistenceMap() map[model.ID]bool {
	objectExists := make(map[model.ID]bool)

	// Add all instances
	for objectID := range v.ctx.InstanceReg.GetAllInstances() {
		objectExists[objectID] = true
	}

	// Add all object arrays
	for objectID := range v.ctx.ArrayReg.GetAllObjectArrays() {
		objectExists[objectID] = true
	}

	// Add all primitive arrays
	for objectID := range v.ctx.ArrayReg.GetAllPrimitiveArrays() {
		objectExists[objectID] = true
	}

	// Add all classes
	for objectID := range v.ctx.ClassDumpReg.GetAllClassDumps() {
		objectExists[objectID] = true
	}

	return objectExists
}

// buildStringExistenceMap creates a fast lookup map for string existence
func (v *ValidatorFinal) buildStringExistenceMap() map[model.ID]bool {
	stringExists := make(map[model.ID]bool)

	for stringID := range v.ctx.StringReg.GetAllStrings() {
		stringExists[stringID] = true
	}

	return stringExists
}

// validateInstanceReferences validates object references in instance fields
func (v *ValidatorFinal) validateInstanceReferences(result *ValidationResult, objectExists map[model.ID]bool) error {
	allInstances := v.ctx.InstanceReg.GetAllInstances()
	for _, instance := range allInstances {
		// Validate class reference
		if !v.validateSingleReference(instance.ClassObjectID, objectExists, result) {
			result.MissingObjects = append(result.MissingObjects, instance.ClassObjectID)
		}

		// Get class definition for field validation
		classDump, hasClass := v.ctx.ClassDumpReg.GetClassDump(instance.ClassObjectID)
		if !hasClass {
			continue // Skip instances without class definitions
		}

		// Extract field references and validate each one
		fieldRefs, err := v.fieldExtractor.ExtractInstanceFieldReferences(instance, classDump)
		if err != nil {
			continue // Skip instances we can't parse
		}

		for _, targetID := range fieldRefs {
			if targetID != 0 && !v.validateSingleReference(targetID, objectExists, result) {
				result.MissingObjects = append(result.MissingObjects, targetID)
			}
		}
	}

	return nil
}

// validateObjectArrayReferences validates references in object array elements
func (v *ValidatorFinal) validateObjectArrayReferences(result *ValidationResult, objectExists map[model.ID]bool) error {
	allObjectArrays := v.ctx.ArrayReg.GetAllObjectArrays()
	for _, array := range allObjectArrays {
		// Validate class reference
		if !v.validateSingleReference(array.ClassID, objectExists, result) {
			result.MissingObjects = append(result.MissingObjects, array.ClassID)
		}

		// Validate each element reference
		for _, elementID := range array.Elements {
			if elementID != 0 && !v.validateSingleReference(elementID, objectExists, result) {
				result.MissingObjects = append(result.MissingObjects, elementID)
			}
		}
	}

	return nil
}

// // validatePrimitiveArrayReferences validates class references in primitive arrays
// func (v *ValidatorFinal) validatePrimitiveArrayReferences(result *ValidationResult, objectExists map[model.ID]bool) error {
// 	allPrimitiveArrays := v.ctx.ArrayReg.GetAllPrimitiveArrays()
// 	for _, array := range allPrimitiveArrays {
// 		// Note: Primitive arrays in HPROF format don't have explicit class object IDs
// 		// The class is implicitly determined by the element type (int[], char[], etc.)
// 		// So there are no class references to validate for primitive arrays.
// 		_ = array // Use array to avoid unused variable warning
// 	}

// 	return nil
// }

// validateGCRootReferences validates references from GC roots to target objects
func (v *ValidatorFinal) validateGCRootReferences(result *ValidationResult, objectExists map[model.ID]bool) error {
	allRoots := v.ctx.RootReg.GetAllRoots()
	for _, root := range allRoots {
		if root.ObjectID != 0 && !v.validateSingleReference(root.ObjectID, objectExists, result) {
			result.MissingObjects = append(result.MissingObjects, root.ObjectID)
		}
	}

	return nil
}

// validateClassReferences validates references within class definitions
func (v *ValidatorFinal) validateClassReferences(result *ValidationResult, objectExists, stringExists map[model.ID]bool) error {
	allClasses := v.ctx.ClassDumpReg.GetAllClassDumps()
	for _, classDump := range allClasses {
		// Validate superclass reference
		if classDump.SuperClassObjectID != 0 {
			if !v.validateSingleReference(classDump.SuperClassObjectID, objectExists, result) {
				result.MissingObjects = append(result.MissingObjects, classDump.SuperClassObjectID)
			}
		}

		// Validate string references (class name, field names, etc.)
		if !v.validateStringReference(classDump.ClassObjectID, stringExists, result) {
			result.MissingObjects = append(result.MissingObjects, classDump.ClassObjectID)
		}

		// Validate instance field name references
		for _, field := range classDump.InstanceFields {
			if !v.validateStringReference(field.NameID, stringExists, result) {
				result.MissingObjects = append(result.MissingObjects, field.NameID)
			}
		}

		// Validate static field references
		for _, staticField := range classDump.StaticFields {
			// Validate field name
			if !v.validateStringReference(staticField.NameID, stringExists, result) {
				result.MissingObjects = append(result.MissingObjects, staticField.NameID)
			}

			// Validate object references in static field values
			if staticField.Type == model.HPROF_NORMAL_OBJECT && len(staticField.Value) >= int(v.ctx.Config.IdentifierSize) {
				objectID := v.extractObjectIDFromStaticField(staticField)
				if objectID != 0 && !v.validateSingleReference(objectID, objectExists, result) {
					result.MissingObjects = append(result.MissingObjects, objectID)
				}
			}
		}
	}

	return nil
}

// validateSingleReference validates a single object reference
func (v *ValidatorFinal) validateSingleReference(objectID model.ID, objectExists map[model.ID]bool, result *ValidationResult) bool {
	result.TotalRefs++

	if objectExists[objectID] {
		result.ValidRefs++
		return true
	}

	result.Valid = false
	return false
}

// validateStringReference validates a single string reference
func (v *ValidatorFinal) validateStringReference(stringID model.ID, stringExists map[model.ID]bool, result *ValidationResult) bool {
	result.TotalRefs++

	if stringExists[stringID] {
		result.ValidRefs++
		return true
	}

	result.Valid = false
	return false
}

// extractObjectIDFromStaticField extracts object ID from static field value
func (v *ValidatorFinal) extractObjectIDFromStaticField(staticField *model.StaticField) model.ID {
	if staticField.Type != model.HPROF_NORMAL_OBJECT || len(staticField.Value) < int(v.ctx.Config.IdentifierSize) {
		return 0
	}

	// Use the field extractor's logic for consistency
	objectID, err := v.fieldExtractor.parseObjectReference(staticField.Value)
	if err != nil {
		return 0
	}

	return objectID
}

// printValidationSummary prints a summary of validation results
func (v *ValidatorFinal) printValidationSummary(result *ValidationResult) {
	fmt.Printf("  Validation completed:\n")
	fmt.Printf("    Total references checked: %d\n", result.TotalRefs)
	fmt.Printf("    Valid references: %d\n", result.ValidRefs)

	if result.TotalRefs > 0 {
		integrity := float64(result.ValidRefs) / float64(result.TotalRefs) * 100
		fmt.Printf("    Reference integrity: %.2f%%\n", integrity)
	}

	if len(result.MissingObjects) > 0 {
		fmt.Printf("    Missing objects: %d\n", len(result.MissingObjects))
	}

	status := "✅"
	if !result.Valid {
		status = "⚠️"
	}
	fmt.Printf("    %s Validation complete\n", status)
}
