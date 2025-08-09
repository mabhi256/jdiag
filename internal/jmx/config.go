package jmx

import (
	"fmt"
	"time"
)

type Config struct {
	// Target configuration
	PID  int    // Process ID for local monitoring
	Host string // Remote monitoring
	Port int    // Remote monitoring

	Interval int // ms

	// Debug configuration
	Debug        bool   // Enable debug mode
	DebugLogFile string // Path to debug log file
}

func (c *Config) GetInterval() time.Duration {
	return time.Duration(c.Interval) * time.Millisecond
}

func (c *Config) String() string {
	if c.PID != 0 {
		return fmt.Sprintf("PID %d", c.PID)
	}

	if c.Host != "" {
		if c.Port != 0 {
			return fmt.Sprintf("%s:%d", c.Host, c.Port)
		}
		return c.Host
	}

	return "No target specified"
}
