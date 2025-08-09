package jmx

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

//go:embed JMXClient.java
var jmxClientSource string

type JMXClient struct {
	pid           int             // Process ID for local attachment
	connectionURL string          // JMX service URL
	tempDir       string          // Temporary directory for generated Java code
	javaPath      string          // Path to Java executable
	activeCmd     *exec.Cmd       // Currently running command (if any)
	cmdMutex      sync.Mutex      // Mutex for active command
	ctx           context.Context // Context for cancellation
	cancel        context.CancelFunc
}

// Interface to allow both regular and debug clients to be used interchangeably
type JMXClientInterface interface {
	QueryMBean(string) (map[string]any, error)
	QueryMBeanPattern(string) ([]map[string]any, error)
	TestConnection() error
	Close() error
}

func NewJMXClient(pid int, url string) (*JMXClient, error) {
	ctx, cancel := context.WithCancel(context.Background())

	client := &JMXClient{
		pid:           pid,
		connectionURL: url,
		ctx:           ctx,
		cancel:        cancel,
	}

	// Find Java executable
	path, err := exec.LookPath("java")
	if err != nil {
		cancel()
		return nil, fmt.Errorf("java executable not found: %w", err)
	}
	client.javaPath = path

	// Create temp directory for JMX client code
	tempDir, err := os.MkdirTemp("", "jdiag-jmx-*")
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	client.tempDir = tempDir

	// Write and compile the embedded Java source
	if err := client.setupJMXClient(); err != nil {
		cancel()
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
		if _, err := os.Stat(toolsJar); err == nil {
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
	// Cancel context to stop any running operations
	c.cancel()

	// Kill active command if running
	c.cmdMutex.Lock()
	if c.activeCmd != nil && c.activeCmd.Process != nil {
		c.activeCmd.Process.Kill()
	}
	c.cmdMutex.Unlock()

	// Remove temp directory
	if c.tempDir != "" {
		return os.RemoveAll(c.tempDir)
	}
	return nil
}

func (c *JMXClient) runJMXCommand(args []string) ([]byte, error) {
	// Check if client is closed
	select {
	case <-c.ctx.Done():
		return nil, fmt.Errorf("JMX client is closed")
	default:
	}

	cmd := exec.CommandContext(c.ctx, c.javaPath, args...)

	// Track active command
	c.cmdMutex.Lock()
	c.activeCmd = cmd
	c.cmdMutex.Unlock()

	// Clean up tracking when done
	defer func() {
		c.cmdMutex.Lock()
		c.activeCmd = nil
		c.cmdMutex.Unlock()
	}()

	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("JMX query failed: %s", string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("failed to execute JMX client: %w", err)
	}

	return output, nil
}

func (c *JMXClient) executeJMXQuery(objectName string, attributes []string) ([]byte, error) {
	args := []string{"-cp", c.tempDir, "JMXClient", "single", objectName}

	if c.pid != 0 {
		args = append(args, strconv.Itoa(c.pid))
	} else {
		args = append(args, c.connectionURL)
	}

	args = append(args, attributes...)
	return c.runJMXCommand(args)
}

func (c *JMXClient) executeJMXQueryPattern(pattern string, attributes []string) ([]byte, error) {
	args := []string{"-cp", c.tempDir, "JMXClient", "pattern", pattern}

	if c.pid != 0 {
		args = append(args, strconv.Itoa(c.pid))
	} else {
		args = append(args, c.connectionURL)
	}

	args = append(args, attributes...)
	return c.runJMXCommand(args)
}

// QueryMBean queries a specific MBean and returns attributes as JSON
func (c *JMXClient) QueryMBean(objectName string) (map[string]any, error) {
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
