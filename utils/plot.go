package utils

import (
	"fmt"
	"slices"
	"strings"
	"time"
)

const (
	ChartHeight     = 14
	YAxisLabelWidth = 7
	MinChartWidth   = 20
	MaxTimeLabels   = 6
	MinLabelSpacing = 10
)

// Abstract styling/rendering concerns
type Renderer interface {
	Render(text string) string
}

type ChartStyles struct {
	Muted    Renderer
	Good     Renderer
	Info     Renderer
	Critical Renderer
	Warning  Renderer
}

// DataPoint represents a single point in the chart
type DataPoint struct {
	Value     float64
	Timestamp time.Time
	Icon      string // Pre-rendered/styled icon
}

// ChartConfig holds configuration for chart rendering
type ChartConfig struct {
	Width  int
	Height int
	Styles ChartStyles
	Legend string // Optional pre-formatted legend
}

// SimpleRenderer provides a basic renderer that just returns the text as-is
type SimpleRenderer struct{}

func (s SimpleRenderer) Render(text string) string {
	return text
}

// CreatePlot creates a line chart with the given data points
func CreatePlot(dataPoints []DataPoint, unit string, config ChartConfig) string {
	if len(dataPoints) == 0 {
		return "No data"
	}

	// Extract values for min/max calculation
	values := make([]float64, len(dataPoints))
	for i, dp := range dataPoints {
		values[i] = dp.Value
	}

	maxVal, minVal := slices.Max(values), slices.Min(values)
	if maxVal == minVal {
		maxVal = minVal + 1 // Avoid division by zero
	}

	width := config.Width - YAxisLabelWidth
	var lines []string

	// Create the chart grid
	chartGrid := make([][]string, config.Height)
	for i := range chartGrid {
		chartGrid[i] = make([]string, width)
		for j := range chartGrid[i] {
			chartGrid[i][j] = " "
		}
	}

	// Calculate data point positions
	chartPoints := make([]struct{ x, y int }, len(dataPoints))
	for i, dp := range dataPoints {
		x := i * (width - 1) / max(1, len(dataPoints)-1)
		if len(dataPoints) == 1 {
			x = width / 2
		}
		// Convert value to y position (inverted since we draw from top to bottom)
		y := int((maxVal-dp.Value)/(maxVal-minVal)*float64(config.Height-1) + 0.5)
		if y >= config.Height {
			y = config.Height - 1
		}
		if y < 0 {
			y = 0
		}
		if x < width {
			chartPoints[i] = struct{ x, y int }{x, y}
		}
	}

	// Draw lines between consecutive points
	if len(chartPoints) > 1 {
		for i := 0; i < len(chartPoints)-1; i++ {
			drawLine(chartGrid, chartPoints[i].x, chartPoints[i].y, chartPoints[i+1].x, chartPoints[i+1].y, width, config.Height, config.Styles.Muted)
		}
	}

	// Place data point markers (this will override line characters at data points)
	for i, point := range chartPoints {
		if point.x < width && point.y < config.Height {
			chartGrid[point.y][point.x] = dataPoints[i].Icon
		}
	}

	// Convert grid to string with Y-axis labels
	for row := 0; row < config.Height; row++ {
		threshold := maxVal - (maxVal-minVal)*float64(row)/float64(config.Height-1)

		// Y-axis label
		var label string
		if unit == "ms" && threshold >= 1000 {
			label = fmt.Sprintf(" %6.2fs", threshold/1000)
		} else {
			label = fmt.Sprintf(" %6.2f%s", threshold, unit)
		}
		lineStr := config.Styles.Muted.Render(label+" ┤") + strings.Join(chartGrid[row], "")
		lines = append(lines, lineStr)
	}

	// Add time axis
	if len(dataPoints) > 0 {
		timestamps := make([]time.Time, len(dataPoints))
		for i, dp := range dataPoints {
			timestamps[i] = dp.Timestamp
		}
		lines = append(lines, createTimeAxis(timestamps, width, config.Styles.Muted)...)
	}

	// Add legend if provided
	if config.Legend != "" {
		lines = append(lines, "", config.Styles.Muted.Render(config.Legend))
	}

	return strings.Join(lines, "\n")
}

// CreateSimplePlot is a convenience function for simple value-only plots
func CreateSimplePlot(values []float64, timestamps []time.Time, unit string, config ChartConfig) string {
	dataPoints := make([]DataPoint, len(values))
	for i, val := range values {
		var ts time.Time
		if i < len(timestamps) {
			ts = timestamps[i]
		}
		dataPoints[i] = DataPoint{
			Value:     val,
			Timestamp: ts,
			Icon:      config.Styles.Good.Render("●"), // Default icon
		}
	}
	return CreatePlot(dataPoints, unit, config)
}

// drawLine draws a line between two points in the chart grid using dots
func drawLine(grid [][]string, x1, y1, x2, y2, width, height int, mutedRenderer Renderer) {
	// Ensure coordinates are within bounds
	if x1 < 0 || x1 >= width || x2 < 0 || x2 >= width ||
		y1 < 0 || y1 >= height || y2 < 0 || y2 >= height {
		return
	}

	dx := abs(x2 - x1)
	dy := abs(y2 - y1)
	sx := 1
	if x1 > x2 {
		sx = -1
	}
	sy := 1
	if y1 > y2 {
		sy = -1
	}
	err := dx - dy

	x, y := x1, y1

	for {
		// Don't overwrite if there's already a data point marker
		if grid[y][x] == " " {
			// Always use dots for line connections
			grid[y][x] = mutedRenderer.Render("·")
		}

		if x == x2 && y == y2 {
			break
		}

		e2 := 2 * err
		if e2 > -dy {
			err -= dy
			x += sx
		}
		if e2 < dx {
			err += dx
			y += sy
		}
	}
}

// createTimeAxis creates time axis labels for the chart
func createTimeAxis(timestamps []time.Time, width int, mutedRenderer Renderer) []string {
	axisLine := strings.Repeat(" ", 10) + "└" + strings.Repeat("─", width)
	timeLine := strings.Repeat(" ", 10)

	numLabels := min(MaxTimeLabels, width/MinLabelSpacing)
	for i := range numLabels {
		timeIndex := i * (len(timestamps) - 1) / max(1, numLabels-1)
		if timeIndex >= len(timestamps) {
			timeIndex = len(timestamps) - 1
		}

		label := timestamps[timeIndex].Format("15:04")
		pos := i * width / max(1, numLabels-1)

		for len(timeLine)-10 < pos {
			timeLine += " "
		}
		timeLine += label
	}

	return []string{mutedRenderer.Render(axisLine), mutedRenderer.Render(timeLine)}
}

// abs returns the absolute value of an integer
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
