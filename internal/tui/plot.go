package tui

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// Chart-related constants
const (
	ChartHeight     = 14
	YAxisLabelWidth = 7
	MinChartWidth   = 20
	MaxTimeLabels   = 6
	MinLabelSpacing = 10
)

// ChartConfig holds configuration for chart rendering
type ChartConfig struct {
	Width  int
	Height int
	Styles ChartStyles
}

// ChartStyles holds the styling information for charts
type ChartStyles struct {
	Muted    lipgloss.Style
	Good     lipgloss.Style
	Info     lipgloss.Style
	Critical lipgloss.Style
	Warning  lipgloss.Style
}

// CreatePlot creates a line chart with the given data points
func CreatePlot(values []float64, timestamps []time.Time, gcTypes []string, unit string, config ChartConfig) string {
	if len(values) == 0 {
		return "No data"
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
	dataPoints := make([]struct{ x, y int }, len(values))
	for i, val := range values {
		x := i * (width - 1) / max(1, len(values)-1)
		if len(values) == 1 {
			x = width / 2
		}
		// Convert value to y position (inverted since we draw from top to bottom)
		y := int((maxVal-val)/(maxVal-minVal)*float64(config.Height-1) + 0.5)
		if y >= config.Height {
			y = config.Height - 1
		}
		if y < 0 {
			y = 0
		}
		if x < width {
			dataPoints[i] = struct{ x, y int }{x, y}
		}
	}

	// Draw lines between consecutive points
	if len(dataPoints) > 1 {
		for i := 0; i < len(dataPoints)-1; i++ {
			drawLine(chartGrid, dataPoints[i].x, dataPoints[i].y, dataPoints[i+1].x, dataPoints[i+1].y, width, config.Height, config.Styles.Muted)
		}
	}

	// Place data point markers (this will override line characters at data points)
	for i, point := range dataPoints {
		if point.x < width && point.y < config.Height {
			chartGrid[point.y][point.x] = getGCIcon(gcTypes[i], config.Styles)
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

	// Add time axis and legend
	if len(timestamps) > 0 {
		lines = append(lines, createTimeAxis(timestamps, width, config.Styles.Muted)...)
	}

	legend := "Legend: " + config.Styles.Good.Render("● Young") + " " +
		config.Styles.Info.Render("▲ Mixed") + " " +
		config.Styles.Critical.Render("■ Full")
	lines = append(lines, "", config.Styles.Muted.Render(legend))

	return strings.Join(lines, "\n")
}

// drawLine draws a line between two points in the chart grid using dots
func drawLine(grid [][]string, x1, y1, x2, y2, width, height int, mutedStyle lipgloss.Style) {
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
			grid[y][x] = mutedStyle.Render("·")
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

// getGCIcon returns a styled icon for the given GC type
func getGCIcon(gcType string, styles ChartStyles) string {
	switch strings.ToLower(gcType) {
	case "young":
		return styles.Good.Render("●")
	case "mixed":
		return styles.Info.Render("▲")
	case "full":
		return styles.Critical.Render("■")
	case "concurrent mark abort":
		return styles.Warning.Render("◆")
	default:
		return styles.Muted.Render("•")
	}
}

// createTimeAxis creates time axis labels for the chart
func createTimeAxis(timestamps []time.Time, width int, mutedStyle lipgloss.Style) []string {
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

	return []string{mutedStyle.Render(axisLine), mutedStyle.Render(timeLine)}
}

// abs returns the absolute value of an integer
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
