package analyzer

import (
	"fmt"

	"github.com/mabhi256/jdiag/internal/heap/registry"
)

// AnalysisContext bundles common dependencies used across all analysis components
type AnalysisContext struct {
	// Core registries from parsing phases
	StringReg    *registry.StringRegistry
	ClassDumpReg *registry.ClassDumpRegistry
	InstanceReg  *registry.InstanceRegistry
	ArrayReg     *registry.ArrayRegistry
	RootReg      *registry.GCRootRegistry

	// Configuration
	Config *AnalysisConfig
}

// AnalysisConfig holds configuration parameters for the analysis
type AnalysisConfig struct {
	IdentifierSize uint32
}

// NewAnalysisContext creates a new analysis context with the provided registries and configuration
func NewAnalysisContext(stringReg *registry.StringRegistry, classDumpReg *registry.ClassDumpRegistry,
	instanceReg *registry.InstanceRegistry, arrayReg *registry.ArrayRegistry,
	rootReg *registry.GCRootRegistry, identifierSize uint32) *AnalysisContext {

	return &AnalysisContext{
		StringReg:    stringReg,
		ClassDumpReg: classDumpReg,
		InstanceReg:  instanceReg,
		ArrayReg:     arrayReg,
		RootReg:      rootReg,
		Config: &AnalysisConfig{
			IdentifierSize: identifierSize,
		},
	}
}

// Validate ensures the context is properly initialized
func (ctx *AnalysisContext) Validate() error {
	if ctx.StringReg == nil {
		return fmt.Errorf("string registry is required")
	}
	if ctx.ClassDumpReg == nil {
		return fmt.Errorf("class dump registry is required")
	}
	if ctx.InstanceReg == nil {
		return fmt.Errorf("instance registry is required")
	}
	if ctx.ArrayReg == nil {
		return fmt.Errorf("array registry is required")
	}
	if ctx.RootReg == nil {
		return fmt.Errorf("GC root registry is required")
	}
	if ctx.Config == nil {
		return fmt.Errorf("analysis config is required")
	}
	if ctx.Config.IdentifierSize == 0 {
		return fmt.Errorf("identifier size must be greater than 0")
	}
	return nil
}

// Clone creates a copy of the context (useful for testing)
func (ctx *AnalysisContext) Clone() *AnalysisContext {
	return &AnalysisContext{
		StringReg:    ctx.StringReg,
		ClassDumpReg: ctx.ClassDumpReg,
		InstanceReg:  ctx.InstanceReg,
		ArrayReg:     ctx.ArrayReg,
		RootReg:      ctx.RootReg,
		Config: &AnalysisConfig{
			IdentifierSize: ctx.Config.IdentifierSize,
		},
	}
}
