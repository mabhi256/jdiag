package heap

import (
	"fmt"

	"github.com/mabhi256/jdiag/internal/heap/parser"
)

func RunHeapAnalysis(filename string) error {

	parser, err := parser.NewParser(filename)
	if err != nil {
		return fmt.Errorf("failed to create parser: %w", err)
	}

	if err := parser.ParseHprof(); err != nil {
		parser.Close()
		return fmt.Errorf("failed to parse header: %w", err)
	}

	return nil
}
