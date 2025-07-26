package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var heapCmd = &cobra.Command{
	Use:   "heap",
	Short: "Analyze heap dumps",
}

var heapValidateCmd = &cobra.Command{
	Use:   "validate [hprof-file]",
	Short: "Validate heap dump file",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Validating heap dump: %s\n", args[0])
		// TODO: implement
	},
}

// TODO: add summary, analyze, compare, export, watch commands

func init() {
	rootCmd.AddCommand(heapCmd)

	heapCmd.AddCommand(heapValidateCmd)
}
