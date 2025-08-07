package monitor

import (
	"bufio"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// Find all running Java processes
func DiscoverJavaProcesses() ([]*JavaProcess, error) {
	// Try jps first (preferred method)
	if processes, err := discoverWithJPS(); err == nil {
		return processes, nil
	}

	// Fall back to ps if jps is not available
	return discoverWithPS()
}

// Use jps command to find Java processes
func discoverWithJPS() ([]*JavaProcess, error) {
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
		args := ""
		if len(parts) > 2 {
			args = parts[2]
		}

		// Skip jps itself
		if strings.Contains(mainClass, "sun.tools.jps.Jps") {
			continue
		}

		// Clean up main class name
		mainClass = strings.TrimSuffix(mainClass, ".jar")

		process := &JavaProcess{
			PID:       pid,
			MainClass: mainClass,
			Args:      args,
		}

		// Get additional info
		checkJMXStatus(process)
		getProcessUser(process)

		processes = append(processes, process)
	}

	return processes, nil
}

// Use ps command as fallback
func discoverWithPS() ([]*JavaProcess, error) {
	cmd := exec.Command("ps", "aux")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run ps: %w", err)
	}

	var processes []*JavaProcess
	scanner := bufio.NewScanner(strings.NewReader(string(output)))

	if scanner.Scan() {
		// Skip header line
	}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.Contains(line, "java") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 11 {
			continue
		}

		pid, err := strconv.Atoi(fields[1])
		if err != nil {
			continue
		}

		user := fields[0]

		// Extract command line
		cmdLineStart := strings.Index(line, "java")
		if cmdLineStart == -1 {
			continue
		}

		cmdLine := line[cmdLineStart:]
		mainClass := extractMainClassFromCmdLine(cmdLine)

		process := &JavaProcess{
			PID:       pid,
			MainClass: mainClass,
			User:      user,
			Args:      cmdLine,
		}

		checkJMXStatus(process)

		processes = append(processes, process)
	}

	return processes, nil
}

// extractMainClassFromCmdLine extracts the main class from a java command line
func extractMainClassFromCmdLine(cmdLine string) string {
	parts := strings.Fields(cmdLine)

	// Look for main class after java executable
	for i, part := range parts {
		if strings.HasSuffix(part, "java") && i+1 < len(parts) {
			// Skip JVM options (those starting with -)
			for j := i + 1; j < len(parts); j++ {
				if !strings.HasPrefix(parts[j], "-") {
					mainClass := parts[j]
					// Clean up jar files
					mainClass = strings.TrimSuffix(mainClass, ".jar")
					return mainClass
				}
			}
		}
	}

	return "Unknown"
}

// checkJMXStatus checks if JMX is enabled for the process
func checkJMXStatus(process *JavaProcess) {
	// Try jcmd first
	cmd := exec.Command("jcmd", strconv.Itoa(process.PID), "ManagementAgent.status")
	output, err := cmd.Output()
	if err != nil {
		// jcmd failed, try to check via process arguments
		checkJMXFromArgs(process)
		return
	}

	outputStr := string(output)
	if strings.Contains(outputStr, "Agent not started") {
		process.JMXEnabled = false
		return
	}

	// JMX is enabled if we get here
	process.JMXEnabled = true

	// Try to extract port
	for _, line := range strings.Split(outputStr, "\n") {
		if strings.Contains(line, "jmxremote.port") {
			if idx := strings.Index(line, "="); idx != -1 {
				if port, err := strconv.Atoi(strings.TrimSpace(line[idx+1:])); err == nil {
					process.JMXPort = port
				}
			}
		}
	}
}

// checkJMXFromArgs checks JMX status from process arguments
func checkJMXFromArgs(process *JavaProcess) {
	if strings.Contains(process.Args, "-Dcom.sun.management.jmxremote") {
		process.JMXEnabled = true

		// Try to extract port from arguments
		args := strings.Fields(process.Args)
		for _, arg := range args {
			if strings.HasPrefix(arg, "-Dcom.sun.management.jmxremote.port=") {
				portStr := strings.TrimPrefix(arg, "-Dcom.sun.management.jmxremote.port=")
				if port, err := strconv.Atoi(portStr); err == nil {
					process.JMXPort = port
				}
			}
		}
	}
}

// getProcessUser gets the process owner (used when jps doesn't provide it)
func getProcessUser(process *JavaProcess) {
	if process.User != "" {
		return // Already set
	}

	cmd := exec.Command("ps", "-o", "user=", "-p", strconv.Itoa(process.PID))
	output, err := cmd.Output()
	if err != nil {
		process.User = "unknown"
		return
	}

	user := strings.TrimSpace(string(output))
	if user != "" {
		process.User = user
	} else {
		process.User = "unknown"
	}
}

// GetProcessByPID retrieves information about a specific Java process
func GetProcessByPID(pid int) (*JavaProcess, error) {
	processes, err := DiscoverJavaProcesses()
	if err != nil {
		return nil, err
	}

	for _, proc := range processes {
		if proc.PID == pid {
			return proc, nil
		}
	}

	return nil, fmt.Errorf("java process with PID %d not found", pid)
}

// EnableJMXForProcess enables JMX for a process that doesn't have it
func EnableJMXForProcess(pid int) error {
	cmd := exec.Command("jcmd", strconv.Itoa(pid), "ManagementAgent.start_local")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to enable JMX for process %d: %w", pid, err)
	}

	outputStr := string(output)
	if strings.Contains(outputStr, "Started") || strings.Contains(outputStr, "already started") {
		return nil
	}

	return fmt.Errorf("failed to enable JMX: %s", outputStr)
}

// RefreshProcessInfo updates process information
func RefreshProcessInfo(process *JavaProcess) error {
	updatedProcess, err := GetProcessByPID(process.PID)
	if err != nil {
		return err
	}

	// Update the original process with new information
	process.MainClass = updatedProcess.MainClass
	process.User = updatedProcess.User
	process.JMXEnabled = updatedProcess.JMXEnabled
	process.JMXPort = updatedProcess.JMXPort
	process.Args = updatedProcess.Args

	return nil
}
