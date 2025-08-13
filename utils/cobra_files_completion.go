package utils

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/spf13/cobra"
)

func CompleteFilesByExtension(extensions []string, isRotated bool) func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		dir := filepath.Dir(toComplete)
		prefix := filepath.Base(toComplete)

		// If no path separator, we're completing in current directory
		if !strings.Contains(toComplete, "/") {
			dir = "."
			prefix = toComplete
		}

		files, err := os.ReadDir(dir)
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}

		var suggestions []string
		for _, file := range files {
			name := file.Name()

			// Skip hidden files and non-matching prefixes
			if strings.HasPrefix(name, ".") || !strings.HasPrefix(name, prefix) {
				continue
			}

			// Build the suggestion path
			suggestion := name
			if dir != "." {
				suggestion = filepath.Join(dir, name)
			}

			if file.IsDir() {
				suggestions = append(suggestions, suggestion+"/")
			} else if isValidFileExtension(name, extensions, isRotated) {
				suggestions = append(suggestions, suggestion)
			}
		}

		slices.Sort(suggestions)
		return suggestions, cobra.ShellCompDirectiveNoFileComp
	}
}

func isValidFileExtension(filename string, extensions []string, includeRotated bool) bool {
	for _, ext := range extensions {
		// Basic extension check
		if strings.HasSuffix(filename, ext) {
			return true
		}

		// Rotated files: .ext.0, .ext.1, .ext.2, etc.
		if includeRotated {
			pattern := regexp.QuoteMeta(ext) + `\.\d+$`
			if matched, _ := regexp.MatchString(pattern, filename); matched {
				return true
			}
		}
	}
	return false
}
