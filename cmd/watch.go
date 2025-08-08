package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mabhi256/jdiag/internal/jmx"
	"github.com/mabhi256/jdiag/internal/monitor"
	"github.com/spf13/cobra"
)

var (
	interval int
	debug    bool
)

var watchCmd = &cobra.Command{
	Use: "watch [PID|HOST:PORT]",
	Short: `Watch provides real-time monitoring of Java application performance metrics including:
- Heap memory usage (young/old generation)
- GC events and frequency  
- Thread count and CPU usage
- Class loading statistics

The tool automatically discovers running Java processes and can enable JMX monitoring
for processes that don't have it enabled.

Examples:
  jdiag watch           				# Interactive process selection
  jdiag watch <TAB>                     # Tab completion with PID and MainClass
  jdiag watch 1234                      # Monitor process ID 1234
  jdiag watch localhost:9999            # Monitor JMX on localhost:9999
  jdiag watch remote.com:8080           # Monitor remote JMX`,
	Args: cobra.MaximumNArgs(1),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		// Already provided (single) argument, don't offer completions
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		// Provide Java process completion
		processes, err := monitor.DiscoverJavaProcesses()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		var completions []string
		for _, proc := range processes {
			option := fmt.Sprintf("%d\t%s", proc.PID, proc.MainClass)
			completions = append(completions, option)
		}

		return completions, cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		config := &jmx.Config{
			Interval: interval,
		}

		if len(args) > 0 {
			arg := args[0]

			// Check if PID
			if pid, err := strconv.Atoi(arg); err == nil && pid > 0 {
				config.PID = pid
			} else if host, port, err := parseHostPort(arg); err == nil {
				// Parse as host:port
				config.Host = host
				config.Port = port
			} else {
				return fmt.Errorf("invalid argument '%s': must be PID or host:port", arg)
			}
		}

		config.Debug = debug
		err := monitor.StartTUI(config)
		if err != nil {
			return fmt.Errorf("unable to start TUI: %w", err)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(watchCmd)

	watchCmd.Flags().IntVarP(&interval, "interval", "i", 1000, "Update interval im ms")
	watchCmd.Flags().BoolVarP(&debug, "debug", "d", false, "Enable debug mode")
}

func parseHostPort(arg string) (string, int, error) {
	before, after, found := strings.Cut(arg, ":")
	if !found {
		return "", -1, fmt.Errorf("invalid host:port format '%s'", arg)
	}

	port, err := strconv.Atoi(strings.TrimSpace(after))
	if err != nil {
		return "", -1, fmt.Errorf("invalid port format '%s'", after)
	}

	return before, port, nil
}
