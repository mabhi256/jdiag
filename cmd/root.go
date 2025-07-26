package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "jdiag",
	Short: "Java diagnostic tool for analyzing GC logs, heap dumps, and thread dumps",
	Long:  `jdiag helps analyze Java application performance through GC logs, heap dumps, and thread dumps.`,
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
