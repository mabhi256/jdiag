package monitor

import (
	"bufio"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// Use jps command to find Java processes
func DiscoverJavaProcesses() ([]*JavaProcess, error) {
	cmd := exec.Command("jps", "-l", "-v")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run jps: %w (ensure Java development tools are installed)", err)
	}

	var processes []*JavaProcess
	scanner := bufio.NewScanner(strings.NewReader(string(output)))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, " ", 3)
		if len(parts) < 2 {
			continue
		}

		pid, err := strconv.Atoi(parts[0])
		if err != nil {
			continue
		}

		mainClass := parts[1]

		// Skip unwanted processes
		if shouldSkipProcess(mainClass) {
			continue
		}

		args := ""
		if len(parts) > 2 {
			args = parts[2]
		}

		// Clean up main class name
		mainClass = strings.TrimSuffix(mainClass, ".jar")

		process := &JavaProcess{
			PID:       pid,
			MainClass: mainClass,
			Args:      args,
		}

		processes = append(processes, process)
	}

	return processes, nil
}

func shouldSkipProcess(mainClass string) bool {
	mainClass = strings.TrimSpace(mainClass)

	// Skip if empty
	if mainClass == "" {
		return true
	}

	// Skip if starts with --
	if strings.HasPrefix(mainClass, "--") {
		return true
	}

	// Skip VSCode extensions
	if strings.Contains(mainClass, ".vscode") && strings.Contains(mainClass, "extensions") {
		return true
	}

	// Skip common tools and unavailable processes
	skipPatterns := []string{
		"sun.tools.jps.Jps",
		"jcmd",
		"JMXClient",
		"-- process information unavailable",
		"org.eclipse.equinox.launcher",
	}

	for _, pattern := range skipPatterns {
		if strings.Contains(mainClass, pattern) {
			return true
		}
	}

	return false
}
