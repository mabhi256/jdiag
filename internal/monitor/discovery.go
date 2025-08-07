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
	// Skip processes with unavailable information
	if strings.Contains(mainClass, "-- process information unavailable") ||
		strings.HasPrefix(mainClass, "--") {
		return true
	}

	// Skip if main class is empty or just whitespace
	if strings.TrimSpace(mainClass) == "" {
		return true
	}

	// Skip VSCode extensions
	if strings.Contains(mainClass, ".vscode") &&
		strings.Contains(mainClass, "extensions") {
		return true
	}

	// Skip jps itself
	if strings.Contains(mainClass, "sun.tools.jps.Jps") {
		return true
	}

	// Skip Eclipse launcher (common IDE process)
	if strings.Contains(mainClass, "org.eclipse.equinox.launcher") {
		return true
	}

	return false
}
