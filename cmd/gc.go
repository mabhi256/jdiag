package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var gcCmd = &cobra.Command{
	Use:   "gc",
	Short: "Analyze GC logs",
}

var gcValidateCmd = &cobra.Command{
	Use:   "validate [gc-log-file]",
	Short: "Validate GC log file",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Validating GC log: %s\n", args[0])
		// TODO: implement
	},
}

// TODO: add summary, analyze, compare, export, watch commands

func init() {
	rootCmd.AddCommand(gcCmd)

	gcCmd.AddCommand(gcValidateCmd)
}
