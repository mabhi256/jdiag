package monitor

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type JMXClient struct {
	pid           int    // Process ID for local attachment
	connectionURL string // JMX service URL
	tempDir       string // Temporary directory for generated Java code
	javaPath      string // Path to Java executable
}

//go:embed JMXClient.java
var jmxClientSource string

func NewJMXClient(pid int, url string) (*JMXClient, error) {
	client := &JMXClient{
		pid:           pid,
		connectionURL: url,
	}

	// Find Java executable
	path, err := exec.LookPath("java")
	if err != nil {
		return nil, fmt.Errorf("java executable not found: %w", err)
	}
	client.javaPath = path

	// Create temp directory for JMX client code
	tempDir, err := os.MkdirTemp("", "jdiag-jmx-*")
	if err != nil {
		return nil, fmt.Errorf("failed to creat temp directory: %w", err)
	}
	client.tempDir = tempDir

	// Write and compile the embedded Java source
	if err := client.setupJMXClient(); err != nil {
		os.RemoveAll(tempDir)
		return nil, fmt.Errorf("failed to setup JMX Client: %w", err)
	}

	return client, nil
}

func (j *JMXClient) setupJMXClient() error {
	// Write embedded Java source
	sourceFile := filepath.Join(j.tempDir, "JMXClient.java")
	if err := os.WriteFile(sourceFile, []byte(jmxClientSource), 0644); err != nil {
		return fmt.Errorf("failed to write Java file: %w", err)
	}

	// Add tools.jar for VirtualMachine (Java 8 and below)
	var classpath []string
	if javaHome := os.Getenv("JAVA_HOME"); javaHome != "" {
		toolsJar := filepath.Join(javaHome, "libs", "tools.jar")
		if _, err := os.Stat(toolsJar); err != nil {
			classpath = append(classpath, toolsJar)
		}

		// For Java 9+, check for jdk.attach module
		jmodPath := filepath.Join(javaHome, "jmods", "jdk.attach.jmod")
		if _, err := os.Stat(jmodPath); err == nil {
			// Java 9+ - VirtualMachine should be available via modules
		}
	}

	// Compile the Java source
	args := []string{"javac"}
	if len(classpath) > 0 {
		args = append(args, "-cp", strings.Join(classpath, string(os.PathListSeparator)))
	}
	args = append(args, "-d", j.tempDir, sourceFile)

	cmd := exec.Command(args[0], args[1:]...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("compilation failed: %s", string(output))
	}

	return nil
}

func (c *JMXClient) Close() error {
	if c.tempDir != "" {
		return os.RemoveAll(c.tempDir)
	}
	return nil
}

func (c *JMXClient) executeJMXQuery(objectName string, attributes []string) ([]byte, error) {
	args := []string{"-cp", c.tempDir, "JMXClient", "single", objectName}

	if c.pid != 0 {
		args = append(args, strconv.Itoa(c.pid))
	} else {
		args = append(args, c.connectionURL)
	}

	args = append(args, attributes...)

	cmd := exec.Command(c.javaPath, args...)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("JMX query failed: %s", string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("failed to execute JMX client: %w", err)
	}

	return output, nil
}

func (c *JMXClient) executeJMXQueryPattern(pattern string, attributes []string) ([]byte, error) {
	args := []string{"-cp", c.tempDir, "JMXClient", "pattern", pattern}

	if c.pid != 0 {
		args = append(args, strconv.Itoa(c.pid))
	} else {
		args = append(args, c.connectionURL)
	}

	args = append(args, attributes...)

	cmd := exec.Command(c.javaPath, args...)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("JMX pattern query failed: %s", string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("failed to execute JMX client: %w", err)
	}

	return output, nil
}

// QueryMBean queries a specific MBean and returns attributes as JSON
func (c *JMXClient) QueryMBean(objectName string) (map[string]any, error) {
	// Use our custom JMX client to query the MBean
	output, err := c.executeJMXQuery(objectName, []string{})
	if err != nil {
		return nil, err
	}

	var result map[string]any
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("failed to parse JMX response: %w", err)
	}

	return result, nil
}

// QueryMBeanPattern queries multiple MBeans matching a pattern
func (c *JMXClient) QueryMBeanPattern(pattern string) ([]map[string]any, error) {
	output, err := c.executeJMXQueryPattern(pattern, []string{})
	if err != nil {
		return nil, err
	}

	var result []map[string]any
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("failed to parse JMX response: %w", err)
	}

	return result, nil
}

// TestConnection tests if we can connect to the JMX service
func (c *JMXClient) TestConnection() error {
	// Try to query the Runtime MBean to test connectivity
	_, err := c.QueryMBean("java.lang:type=Runtime")
	return err
}

func EnableJMXForProcess(pid int) error {
	cmd := exec.Command("jcmd", strconv.Itoa(pid), "ManagementAgent.start_local")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to enable JMX for process %d: %w", pid, err)
	}

	outputStr := string(output)
	if strings.Contains(outputStr, "Started") || strings.Contains(outputStr, "successfully") {
		return nil
	}

	return fmt.Errorf("failed to enable JMX: %s", outputStr)
}
