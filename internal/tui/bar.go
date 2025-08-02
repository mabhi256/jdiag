// horizontalBar.go
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Constants for bar chart configuration
const (
	DefaultLabelWidth  = 16
	DefaultFilledChar  = "█"
	DefaultEmptyChar   = "▱"
	DefaultValueFormat = "%.1f"
	MinBarWidth        = 1
)

// BarData represents a single bar in the chart
type BarData struct {
	Label      string         // Text label for the bar
	Value      float64        // Numeric value to display
	Percentage float64        // Percentage for bar width calculation
	Style      lipgloss.Style // Color/style for the bar
	Suffix     string         // Additional info (e.g., "- 5 events")
}

// HorizontalBarConfig defines chart appearance and behavior
type HorizontalBarConfig struct {
	BarAreaWidth int    // Width of the actual bar area
	LabelWidth   int    // Width reserved for labels
	FilledChar   string // Character for filled portion
	EmptyChar    string // Character for empty portion
	ShowValue    bool   // Whether to show numeric values
	ShowPercent  bool   // Whether to show percentages
	ValueFormat  string // Format string for values (e.g., "%.1fms")
}

// DefaultBarConfig creates a sensible default configuration
func DefaultBarConfig(barAreaWidth int) HorizontalBarConfig {
	return HorizontalBarConfig{
		BarAreaWidth: barAreaWidth,
		LabelWidth:   DefaultLabelWidth,
		FilledChar:   DefaultFilledChar,
		EmptyChar:    DefaultEmptyChar,
		ShowValue:    true,
		ShowPercent:  true,
		ValueFormat:  DefaultValueFormat,
	}
}

// CreateHorizontalBar builds a single horizontal bar representation
func CreateHorizontalBar(data BarData, config HorizontalBarConfig) string {
	// Calculate bar dimensions
	barWidth := max(MinBarWidth, int(data.Percentage*float64(config.BarAreaWidth)/100))
	emptyWidth := max(0, config.BarAreaWidth-barWidth)

	// Create the visual bar
	bar := strings.Repeat(config.FilledChar, barWidth) +
		strings.Repeat(config.EmptyChar, emptyWidth)
	styledBar := data.Style.Render(bar)

	// Build value display text
	var valueParts []string
	if config.ShowValue {
		valueParts = append(valueParts, fmt.Sprintf(config.ValueFormat, data.Value))
	}
	if config.ShowPercent {
		valueParts = append(valueParts, fmt.Sprintf("(%4.1f%%)", data.Percentage))
	}
	if data.Suffix != "" {
		valueParts = append(valueParts, data.Suffix)
	}

	valueDisplay := strings.Join(valueParts, " ")

	// Format the complete line: "Label │████▱▱▱│ Value (Percent) Suffix"
	return fmt.Sprintf("%-*s │%s│ %s",
		config.LabelWidth, data.Label, styledBar, valueDisplay)
}

// CreateHorizontalBarChart builds a complete bar chart with optional title
func CreateHorizontalBarChart(title string, bars []BarData, config HorizontalBarConfig) string {
	var lines []string

	// Add title if provided
	if title != "" {
		lines = append(lines, title, "")
	}

	// Add each bar
	for _, bar := range bars {
		lines = append(lines, CreateHorizontalBar(bar, config))
	}

	return strings.Join(lines, "\n")
}
