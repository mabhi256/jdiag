package heap

import (
	"fmt"

	"github.com/mabhi256/jdiag/internal/heap/analyzer"
	"github.com/mabhi256/jdiag/internal/heap/parser"
)

// RunHeapAnalysis performs the complete heap analysis using the refactored analyzer
func RunHeapAnalysis(filename string) error {
	parser, err := parser.NewParser(filename)
	if err != nil {
		return fmt.Errorf("failed to create parser: %w", err)
	}
	defer parser.Close()

	if err := parser.ParseHprof(); err != nil {
		return fmt.Errorf("failed to parse hprof file: %w", err)
	}

	// Create analyzer using the same interface - now with improved internal structure
	heapAnalyzer := analyzer.NewAnalyzer(
		parser.GetStringRegistry(),
		parser.GetClassDumpRegistry(),
		parser.GetObjectRegistry(),
		parser.GetArrayRegistry(),
		parser.GetGCRootRegistry(),
		parser.GetHeader().IdentifierSize,
	)

	// Perform analysis - the external interface remains the same
	if err := heapAnalyzer.PerformAnalysis(); err != nil {
		return fmt.Errorf("analysis failed: %w", err)
	}

	// Demonstrate analyzer capabilities with improved error handling
	demonstrateAnalyzerCapabilities(heapAnalyzer)

	return nil
}

// RunHeapAnalysisWithContext demonstrates using the new context-based approach for advanced scenarios
func RunHeapAnalysisWithContext(filename string) error {
	parser, err := parser.NewParser(filename)
	if err != nil {
		return fmt.Errorf("failed to create parser: %w", err)
	}
	defer parser.Close()

	if err := parser.ParseHprof(); err != nil {
		return fmt.Errorf("failed to parse hprof file: %w", err)
	}

	// Create analysis context first (useful for testing and advanced configuration)
	ctx := analyzer.NewAnalysisContext(
		parser.GetStringRegistry(),
		parser.GetClassDumpRegistry(),
		parser.GetObjectRegistry(),
		parser.GetArrayRegistry(),
		parser.GetGCRootRegistry(),
		parser.GetHeader().IdentifierSize,
	)

	// Create analyzer with context (alternative approach)
	heapAnalyzer, err := analyzer.NewAnalyzerWithContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to create analyzer: %w", err)
	}

	// Perform analysis
	if err := heapAnalyzer.PerformAnalysis(); err != nil {
		return fmt.Errorf("analysis failed: %w", err)
	}

	// Print detailed report using new capabilities
	heapAnalyzer.PrintDetailedReport()

	return nil
}

// demonstrateAnalyzerCapabilities showcases the refactored analyzer's enhanced capabilities
func demonstrateAnalyzerCapabilities(analyzer *analyzer.Analyzer) {
	fmt.Println()
	fmt.Println("🛠️  ENHANCED ANALYZER CAPABILITIES DEMO")

	// Check if analyzer is ready (improved status checking)
	if !analyzer.IsReady() {
		fmt.Println("❌ Analyzer not ready for advanced operations")
		return
	}

	// Show object existence checking with improved error handling
	fmt.Println("1. Object Existence Checking:")
	objectGraph := analyzer.GetObjectGraph()
	if objectGraph != nil && objectGraph.IsValid() {
		existingObjects := 0
		for _, exists := range objectGraph.ObjectExists {
			if exists {
				existingObjects++
			}
		}
		fmt.Printf("   Can check existence of %d objects\n", existingObjects)
		fmt.Printf("   Graph integrity: %s\n", objectGraph.GetSummary())
	}

	// Show reference navigation with enhanced statistics
	fmt.Println("\n2. Enhanced Reference Navigation:")
	refMap := analyzer.GetReferenceMap()
	if refMap != nil {
		stats := refMap.GetStatistics()
		fmt.Printf("   Objects with references: %v\n", stats["objects_with_references"])
		fmt.Printf("   Total forward references: %v\n", stats["total_forward_references"])

		// Show example of reference navigation for first few objects
		count := 0
		for objID, refs := range refMap.ForwardRefs {
			if count >= 3 {
				break
			}
			if len(refs) > 0 {
				fmt.Printf("   Example: Object 0x%x → %d references\n",
					uint64(objID), len(refs))
				count++
			}
		}
	}

	// Show enhanced validation results
	fmt.Println("\n3. Enhanced Data Validation:")
	validation := analyzer.GetValidationResult()
	if validation != nil {
		fmt.Printf("   %s\n", validation.GetSummary())
		fmt.Printf("   Reference integrity: %.2f%%\n", validation.GetIntegrityPercentage())
	}

	// Show readiness for future phases (unchanged)
	fmt.Println("\n4. Ready for Advanced Analysis:")
	fmt.Println("   ✅ Reachability analysis (Phase 12)")
	fmt.Println("   ✅ Memory usage calculation (Phase 13)")
	fmt.Println("   ✅ Leak detection (Phase 16)")
	fmt.Println("   ✅ Reference path analysis (Phase 19)")

	// Show enhanced metadata with better organization
	fmt.Println("\n5. Enhanced Analysis Metadata:")
	metadata := analyzer.GetAnalysisMetadata()

	// Group metadata by category for better readability
	categories := map[string][]string{
		"Timing":     {"analysis_time", "start_time"},
		"References": {"total_references", "valid_references", "reference_integrity"},
		"Objects":    {"total_objects", "total_classes", "total_arrays", "total_refs_in_graph"},
	}

	for category, keys := range categories {
		fmt.Printf("   %s:\n", category)
		for _, key := range keys {
			if value, exists := metadata[key]; exists {
				fmt.Printf("     %s: %v\n", key, value)
			}
		}
	}

	// Show context information (new capability)
	fmt.Println("\n6. Analysis Context Info:")
	ctx := analyzer.GetContext()
	if ctx != nil && ctx.Config != nil {
		fmt.Printf("   Identifier size: %d bytes\n", ctx.Config.IdentifierSize)

		// Show registry statistics
		if ctx.StringReg != nil {
			allStrings := ctx.StringReg.GetAllStrings()
			fmt.Printf("   String registry: %d entries\n", len(allStrings))
		}

		if ctx.InstanceReg != nil {
			allInstances := ctx.InstanceReg.GetAllInstances()
			fmt.Printf("   Instance registry: %d entries\n", len(allInstances))
		}

		// Show comprehensive GC root information
		if ctx.RootReg != nil {
			allRoots := ctx.RootReg.GetAllRoots()
			fmt.Printf("   GC roots: %d total\n", len(allRoots))

			// Show breakdown by root type
			rootTypeCounts := ctx.RootReg.GetRootTypeCounts()
			for rootType, count := range rootTypeCounts {
				fmt.Printf("     %s: %d\n", rootType, count)
			}
		}
	}

	fmt.Println("\n✨ Refactored analyzer provides improved maintainability and testability!")
}
