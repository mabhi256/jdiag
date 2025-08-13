package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mabhi256/jdiag/internal/heap"
	"github.com/mabhi256/jdiag/utils"
	"github.com/spf13/cobra"
)

var heapCmd = &cobra.Command{
	Use: "heap [hprof-file]",
	Short: `Analyze heap dumps (.hprof files only)
The tool automatically validates the file and provides comprehensive analysis including:
- Memory usage overview
- Memory usage by class
- Memory leak detection  
- Object reference analysis
- Dominator tree visualization
- String deduplication analysis`,
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: utils.CompleteFilesByExtension([]string{".hprof"}, true),
	RunE: func(cmd *cobra.Command, args []string) error {
		filename := args[0]

		if _, err := os.Stat(filename); os.IsNotExist(err) {
			return fmt.Errorf("file does not exist: %s", filename)
		}

		// Check file extension (warning only)
		if ext := filepath.Ext(filename); ext != ".hprof" {
			fmt.Printf("Warning: File extension '%s' is not '.hprof', but proceeding anyway...\n", ext)
		}

		return heap.RunHeapAnalysis(filename)
	},
}

func init() {
	rootCmd.AddCommand(heapCmd)
}
