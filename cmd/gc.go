package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"slices"
	"strings"

	"github.com/mabhi256/jdiag/internal/gc"
	"github.com/mabhi256/jdiag/internal/gc/html"
	"github.com/mabhi256/jdiag/internal/gc/tui"
	"github.com/mabhi256/jdiag/utils"
	"github.com/spf13/cobra"
)

var (
	output string
)

var gcCmd = &cobra.Command{
	Use:   "gc",
	Short: "Analyze Java garbage collection logs",
}

func isHtmlFile() bool {
	return strings.HasSuffix(output, ".html")
}

var gcAnalyzeCmd = &cobra.Command{
	Use: "analyze [gc-log-file]",
	Short: `Analyze a Java GC log file.

This command parses GC log files and provides detailed analysis
including pause times, throughput metrics, heap utilization, and tuning recommendations.

Output Formats:
  cli       Basic summary with key metrics (default)
  cli-more  Detailed analysis with recommendations
  tui       Interactive terminal interface for exploration
  html      Generate HTML report and open in browser
  file.html Save HTML report to specific file

Examples:
  jdiag gc analyze app.log					# Basic analysis with summary output
  jdiag gc analyze app.log -o cli-more		# Detailed command-line output with recommendations
  jdiag gc analyze app.log -o tui			# Interactive terminal interface
  jdiag gc analyze app.log -o html			# Generate HTML report
  jdiag gc analyze app.log -o report.html	# Save HTML report to specific file`,
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: utils.CompleteFilesByExtension([]string{".log"}, true),
	PreRunE: func(cmd *cobra.Command, args []string) error {
		validFormats := []string{"cli", "cli-more", "tui", "html"}

		if !slices.Contains(validFormats, output) && !isHtmlFile() {
			return fmt.Errorf("invalid output format: %s. Valid options: %v or *.html", output, validFormats)
		}

		// Check if file exists
		logFile := args[0]
		if _, err := os.Stat(logFile); os.IsNotExist(err) {
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

		switch {
		case output == "cli":
			analysis.PrintSummary()
		case output == "cli-more":
			analysis.PrintDetailed()
			recommendations.Print()
		case output == "tui":
			tui.StartTUI(events, analysis, recommendations)
		case output == "html" || isHtmlFile():
			// Generate HTML report and return absolute path of the output
			var absPath string
			var err error
			if isHtmlFile() {
				absPath, err = html.GenerateHTMLReport(events, analysis, recommendations, output)
			} else {
				absPath, err = html.GenerateHTMLReport(events, analysis, recommendations, "")
			}
			if err != nil {
				fmt.Printf("Error generating HTML report: %v\n", err)
				return
			}

			fmt.Printf("HTML report generated: %s\n", makeClickableLink(absPath))

			// Auto-open the file in default browser
			if err := openInBrowser(absPath); err != nil {
				fmt.Printf("Note: Could not automatically open browser: %v\n", err)
			}
		default:
			analysis.PrintSummary()
		}
	},
}

// TODO: add compare command

func init() {
	rootCmd.AddCommand(gcCmd)

	gcCmd.AddCommand(gcAnalyzeCmd)

	gcAnalyzeCmd.Flags().StringVarP(&output, "output", "o", "cli", "Output format")

	// When user types: jdiag gc analyze file.log -o <TAB>
	gcAnalyzeCmd.RegisterFlagCompletionFunc("output", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"cli", "cli-more", "tui", "html"}, cobra.ShellCompDirectiveNoFileComp
	})
}

func makeClickableLink(filePath string) string {
	fileURL := "file://" + filePath

	// Use OSC 8 escape sequence for clickable links
	// Format: \033]8;;URL\033\\TEXT\033]8;;\033\\
	return fmt.Sprintf("\033]8;;%s\033\\%s\033]8;;\033\\", fileURL, filePath)
}

func openInBrowser(filePath string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin": // macOS
		cmd = exec.Command("open", filePath)
	case "linux":
		cmd = exec.Command("xdg-open", filePath)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", filePath)
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}

	return cmd.Start()
}
