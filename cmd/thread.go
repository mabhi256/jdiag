package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var threadCmd = &cobra.Command{
	Use:   "thread",
	Short: "Analyze thread dumps",
}

var threadValidateCmd = &cobra.Command{
	Use:   "validate [hprof-file]",
	Short: "Validate thread dump file",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Validating thread dump: %s\n", args[0])
		// TODO: implement
	},
}

// TODO: add summary, analyze, compare, export, watch commands

func init() {
	rootCmd.AddCommand(threadCmd)

	threadCmd.AddCommand(threadValidateCmd)
}
