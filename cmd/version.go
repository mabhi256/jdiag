package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	// This will be set by goreleaser
	version = "dev"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("jdiag version %s\n", version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
