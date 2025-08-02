package cmd

import (
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/mabhi256/jdiag/internal/gc"
	"github.com/mabhi256/jdiag/internal/tui"
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
		validFormats := []string{"cli", "cli-more", "tui", "html"}
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
		parser := gc.NewParser()
		events, analysis, err := parser.ParseFile(args[0])
		gc.AnalyzeGCLogs(events, analysis)
		if err != nil {
			fmt.Printf("Error parsing GC log: %v\n", err)
			return
		}
		recommendations := gc.GetRecommendations(analysis)

		switch outputFormat {
		case "cli":
			analysis.PrintSummary()
		case "cli-more":
			analysis.PrintDetailed()
			recommendations.Print()
		case "tui":
			tui.StartTUI(events, analysis, recommendations)
		case "html":
			fmt.Println("HTML format not yet implemented")
		default:
			analysis.PrintSummary()
		}
	},
}

// TODO: add compare, export, watch commands

func init() {
	rootCmd.AddCommand(gcCmd)

	gcCmd.AddCommand(gcAnalyzeCmd)

	gcAnalyzeCmd.Flags().StringVarP(&outputFormat, "output", "o", "cli", "Output format")

	// When user types: jdiag gc analyze file.log -o <TAB>
	gcAnalyzeCmd.RegisterFlagCompletionFunc("output", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"cli", "cli-more", "tui", "html"}, cobra.ShellCompDirectiveNoFileComp
	})
}

func completeGCLogFiles(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	// Determine the directory to search and the prefix to match
	dir := "."
	prefix := toComplete

	// If toComplete contains a path separator, extract directory and filename prefix
	if strings.Contains(toComplete, "/") {
		lastSlash := strings.LastIndex(toComplete, "/")
		dir = toComplete[:lastSlash+1] // Include the trailing slash for proper path construction
		prefix = toComplete[lastSlash+1:]

		// Clean up the directory path (remove trailing slash for os.ReadDir)
		dirForRead := strings.TrimSuffix(dir, "/")
		if dirForRead == "" {
			dirForRead = "."
		}
		dir = dirForRead
	}

	// Read the target directory
	files, err := os.ReadDir(dir)
	if err != nil {
		// If we can't read the directory, return no suggestions
		return nil, cobra.ShellCompDirectiveError
	}

	var suggestions []string

	for _, file := range files {
		name := file.Name()

		// Skip hidden files/directories (starting with .)
		if strings.HasPrefix(name, ".") {
			continue
		}

		// Filter by prefix if provided
		if prefix != "" && !strings.HasPrefix(name, prefix) {
			continue
		}

		// Construct the full path for suggestions
		var fullPath string
		if dir == "." {
			fullPath = name
		} else {
			fullPath = dir + "/" + name
		}

		if file.IsDir() {
			// Add directories with trailing slash to indicate they can be traversed
			suggestions = append(suggestions, fullPath+"/")
		} else if isValidGCLogFile(name) {
			// Add valid GC log files
			suggestions = append(suggestions, fullPath)
		}
	}

	// Sort suggestions for better user experience
	slices.Sort(suggestions)

	return suggestions, cobra.ShellCompDirectiveNoFileComp
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
