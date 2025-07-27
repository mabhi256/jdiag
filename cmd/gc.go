package cmd

import (
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/mabhi256/jdiag/internal/gc"
	"github.com/spf13/cobra"
)

var outputFormat string

var gcCmd = &cobra.Command{
	Use:   "gc",
	Short: "Analyze GC logs",
}

var gcAnalyzeCmd = &cobra.Command{
	Use:               "analyze [gc-log-file]",
	Short:             "Analyze GC log file",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeGCLogFiles,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		// Validate output flag
		validFormats := []string{"cli", "tui", "html"}
		if !slices.Contains(validFormats, outputFormat) {
			return fmt.Errorf("invalid output format: %s. Valid options: %v", outputFormat, validFormats)
		}

		// Validate file argument
		logFile := args[0]
		if !isValidGCLogFile(logFile) {
			return fmt.Errorf("invalid GC log file: %s", logFile)
		}

		// Check if file exists
		_, err := os.Stat(logFile)
		if os.IsNotExist(err) {
			return fmt.Errorf("file does not exist: %s", logFile)
		}

		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Analyzing GC log: %s\n", args[0])
		parser := gc.NewParser()
		log, err := parser.ParseFile(args[0])
		if err != nil {
			fmt.Printf("Error parsing GC log: %v\n", err)
			return
		}

		metrics := log.CalculateMetrics()
		log.PrintSummary(metrics, outputFormat)
		log.AnalyzePerformanceIssues(metrics)
	},
}

// TODO: add summary, analyze, compare, export, watch commands

func init() {
	rootCmd.AddCommand(gcCmd)

	gcCmd.AddCommand(gcAnalyzeCmd)

	gcAnalyzeCmd.Flags().StringVarP(&outputFormat, "output", "o", "cli", "Output format")

	// When user types: jdiag gc analyze file.log -o <TAB>
	gcAnalyzeCmd.RegisterFlagCompletionFunc("output", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"cli", "tui", "html"}, cobra.ShellCompDirectiveNoFileComp
	})
}

func completeGCLogFiles(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	// Get current directory files
	files, _ := os.ReadDir(".")

	var validFiles []string
	for _, file := range files {
		if !file.IsDir() && isValidGCLogFile(file.Name()) {
			validFiles = append(validFiles, file.Name())
		}
	}

	return validFiles, cobra.ShellCompDirectiveNoFileComp
}

func isValidGCLogFile(filename string) bool {
	// Basic .log extension
	if strings.HasSuffix(filename, ".log") {
		return true
	}

	// Rotated logs: .log.0, .log.1, .log.2, etc.
	re := regexp.MustCompile(`\.log\.\d+$`)
	return re.MatchString(filename)
}
