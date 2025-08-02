package tui

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/mabhi256/jdiag/internal/gc"
)

func (m *Model) RenderTrends() string {
	if len(m.events) == 0 {
		return renderNoTrendsData()
	}

	// Get recent events based on time window
	events := m.getRecentEvents()
	if len(events) < 2 {
		return renderInsufficientTrendsData()
	}

	header := m.renderTrendsHeader()
	content := m.renderTrendsContent(events)

	return lipgloss.JoinVertical(lipgloss.Left, header, "", content)
}

func (m *Model) renderTrendsHeader() string {
	trendNames := map[TrendSubTab]string{
		PauseTrend:     "Pause Times",
		HeapTrend:      "Heap Usage",
		FrequencyTrend: "Collection Freq",
	}

	var tabs []string
	for trend := PauseTrend; trend <= FrequencyTrend; trend++ {
		style := TabInactiveStyle
		if trend == m.trendsState.trendSubTab {
			style = TabActiveStyle
		}
		tabs = append(tabs, style.Render(trendNames[trend]))
	}

	tabLine := strings.Join(tabs, "  ")
	infoLine := MutedStyle.Render(fmt.Sprintf("Showing last %d events", m.trendsState.timeWindow))

	return lipgloss.JoinVertical(lipgloss.Left, tabLine, infoLine)
}

func (m *Model) renderTrendsContent(events []*gc.GCEvent) string {
	switch m.trendsState.trendSubTab {
	case PauseTrend:
		return m.renderPauseTrends(events)
	case HeapTrend:
		return m.renderHeapTrends(events)
	case FrequencyTrend:
		return m.renderFrequencyTrends(events)
	default:
		return "Unknown trend view"
	}
}

func (m *Model) renderPauseTrends(events []*gc.GCEvent) string {
	title := TitleStyle.Render("Pause Times Over Time")

	// Extract pause times
	var pauses []float64
	var timestamps []time.Time
	var maxPause float64

	for _, event := range events {
		pauseMs := float64(event.Duration.Nanoseconds()) / 1000000
		pauses = append(pauses, pauseMs)
		timestamps = append(timestamps, event.Timestamp)
		maxPause = math.Max(maxPause, pauseMs)
	}

	// Create ASCII chart with full width utilization
	chartHeight := 12
	chartWidth := m.calculateChartWidth()
	chart := m.createImprovedLineChart(pauses, timestamps, chartWidth, chartHeight, "ms", "Pause Time")

	// Calculate statistics
	avg := average(pauses)
	p95 := percentile(pauses, 0.95)
	p99 := percentile(pauses, 0.99)

	stats := fmt.Sprintf("Avg: %.1fms  P95: %.1fms  P99: %.1fms  Max: %.1fms",
		avg, p95, p99, maxPause)

	return lipgloss.JoinVertical(lipgloss.Left,
		title,
		"",
		chart,
		"",
		InfoStyle.Render(stats))
}

func (m *Model) renderHeapTrends(events []*gc.GCEvent) string {
	title := TitleStyle.Render("Heap Usage Over Time")

	// Extract heap usage data as MemorySize
	var beforeHeap, afterHeap []gc.MemorySize
	var timestamps []time.Time

	for _, event := range events {
		if event.HeapTotal.Bytes() > 0 {
			beforeHeap = append(beforeHeap, event.HeapBefore)
			afterHeap = append(afterHeap, event.HeapAfter)
			timestamps = append(timestamps, event.Timestamp)
		}
	}

	if len(beforeHeap) == 0 {
		return "No heap utilization data available"
	}

	// Create heap usage chart
	chartHeight := 15
	chartWidth := m.calculateChartWidth()
	chart := m.createHeapBarsChart(beforeHeap, afterHeap, timestamps, chartWidth, chartHeight)

	// Statistics
	avgBefore := averageMemorySize(beforeHeap)
	avgAfter := averageMemorySize(afterHeap)
	avgFreed := avgBefore.Sub(avgAfter)
	efficiency := avgFreed.Ratio(avgBefore) * 100

	stats := fmt.Sprintf("Avg Before: %s  Avg After: %s  Avg Freed: %s  Efficiency: %.1f%%",
		avgBefore.String(), avgAfter.String(), avgFreed.String(), efficiency)

	// Legend
	legend := fmt.Sprintf("%s Before GC  %s After GC  %s GC Event",
		CriticalStyle.Render("█"),
		GoodStyle.Render("█"),
		MutedStyle.Render("│"))

	return lipgloss.JoinVertical(lipgloss.Left,
		title,
		"",
		legend,
		"",
		chart,
		"",
		InfoStyle.Render(stats))
}

// Create heap bars chart where full height = before, colored portion = after
func (m *Model) createHeapBarsChart(beforeHeap, afterHeap []gc.MemorySize, timestamps []time.Time, width, height int) string {
	if len(beforeHeap) == 0 || len(afterHeap) == 0 {
		return "No data"
	}

	maxVal := maxMemorySize(beforeHeap)
	if maxVal == 0 {
		maxVal = gc.MemorySize(1) // Avoid division by zero
	}

	var lines []string

	// Chart area
	for row := range height {
		line := ""
		// Calculate the heap threshold for this row (from top to bottom)
		threshold := maxVal.Mul(float64(height-row) / float64(height))

		// Y-axis label
		label := fmt.Sprintf("%6s", threshold.String())
		line += MutedStyle.Render(label + " ┤")

		// Data points spread across full width
		for col := range width {
			// Map column to data point index
			dataIndex := int(float64(col) * float64(len(beforeHeap)-1) / float64(width-1))
			if dataIndex >= len(beforeHeap) {
				dataIndex = len(beforeHeap) - 1
			}

			beforeVal := beforeHeap[dataIndex]
			afterVal := afterHeap[dataIndex]

			// Show heap usage as overlapping bars: before (CriticalStyle) with after (GoodStyle) overlaid
			var char string
			if beforeVal >= threshold {
				// We're within the "before" heap usage area
				if afterVal >= threshold {
					// After GC still uses this memory - show in GoodStyle (overlaid)
					char = GoodStyle.Render("█")
				} else {
					// This memory was freed by GC - show in CriticalStyle
					char = CriticalStyle.Render("█")
				}
			} else {
				// Above the heap usage - empty space
				char = " "
			}

			line += char
		}

		lines = append(lines, line)
	}

	// Time axis
	if len(timestamps) > 0 {
		axisLine := strings.Repeat(" ", 9) + "└" + strings.Repeat("─", width)
		lines = append(lines, MutedStyle.Render(axisLine))

		// Time labels spread across width
		timeLine := strings.Repeat(" ", 10)
		numLabels := min(6, width/10)

		for i := 0; i < numLabels; i++ {
			timeIndex := int(float64(i) * float64(len(timestamps)-1) / float64(numLabels-1))
			if timeIndex >= len(timestamps) {
				timeIndex = len(timestamps) - 1
			}

			label := timestamps[timeIndex].Format("15:04")

			// Calculate position for this label
			pos := int(float64(i) * float64(width) / float64(numLabels-1))

			// Add spacing to reach the correct position
			for len(timeLine)-10 < pos {
				timeLine += " "
			}
			timeLine += label
		}
		lines = append(lines, MutedStyle.Render(timeLine))
	}

	return strings.Join(lines, "\n")
}

// Helper function to calculate average of MemorySize values
func averageMemorySize(values []gc.MemorySize) gc.MemorySize {
	if len(values) == 0 {
		return 0
	}
	sum := gc.MemorySize(0)
	for _, v := range values {
		sum = sum.Add(v)
	}
	return sum.Div(float64(len(values)))
}

// Helper function to find maximum MemorySize
func maxMemorySize(values []gc.MemorySize) gc.MemorySize {
	if len(values) == 0 {
		return 0
	}
	maxVal := values[0]
	for _, v := range values {
		if v > maxVal {
			maxVal = v
		}
	}
	return maxVal
}

func (m *Model) renderFrequencyTrends(events []*gc.GCEvent) string {
	title := TitleStyle.Render("Collection Frequency Analysis")

	// Group events by type and calculate frequencies
	youngCount := 0
	mixedCount := 0
	fullCount := 0

	for _, event := range events {
		switch {
		case strings.Contains(strings.ToLower(event.Type), "young"):
			youngCount++
		case strings.Contains(strings.ToLower(event.Type), "mixed"):
			mixedCount++
		case strings.Contains(strings.ToLower(event.Type), "full"):
			fullCount++
		}
	}

	total := len(events)
	if total == 0 {
		return "No events to analyze"
	}

	// Calculate percentages
	youngPct := float64(youngCount) / float64(total) * 100
	mixedPct := float64(mixedCount) / float64(total) * 100
	fullPct := float64(fullCount) / float64(total) * 100

	// Calculate bar width with better space utilization
	barAreaWidth := m.calculateChartWidth() - 10 // Reserve space for labels and bars

	var lines []string
	lines = append(lines, fmt.Sprintf("Event Distribution (last %d events):", total))
	lines = append(lines, "")

	// Young GC bar
	if youngCount > 0 {
		barWidth := max(1, int(youngPct*float64(barAreaWidth)/100))
		emptyWidth := max(0, barAreaWidth-barWidth)
		youngBar := strings.Repeat("█", barWidth) + strings.Repeat("░", emptyWidth)
		lines = append(lines, fmt.Sprintf("Young   │%s│ %d (%4.1f%%)",
			GoodStyle.Render(youngBar), youngCount, youngPct))
	}

	// Mixed GC bar
	if mixedCount > 0 {
		barWidth := max(1, int(mixedPct*float64(barAreaWidth)/100))
		emptyWidth := max(0, barAreaWidth-barWidth)
		mixedBar := strings.Repeat("█", barWidth) + strings.Repeat("░", emptyWidth)
		lines = append(lines, fmt.Sprintf("Mixed   │%s│ %d (%4.1f%%)",
			InfoStyle.Render(mixedBar), mixedCount, mixedPct))
	}

	// Full GC bar
	if fullCount > 0 {
		barWidth := max(1, int(fullPct*float64(barAreaWidth)/100))
		emptyWidth := max(0, barAreaWidth-barWidth)
		fullBar := strings.Repeat("█", barWidth) + strings.Repeat("░", emptyWidth)
		lines = append(lines, fmt.Sprintf("Full    │%s│ %d (%4.1f%%)",
			CriticalStyle.Render(fullBar), fullCount, fullPct))
	}

	lines = append(lines, "")

	// Time-based frequency analysis
	if len(events) > 1 {
		duration := events[0].Timestamp.Sub(events[len(events)-1].Timestamp)
		if duration > 0 {
			avgInterval := duration / time.Duration(len(events)-1)
			gcPerHour := float64(time.Hour) / float64(avgInterval)

			lines = append(lines, fmt.Sprintf("Average Interval: %s", FormatDuration(avgInterval)))
			lines = append(lines, fmt.Sprintf("GC Events/Hour: %.1f", gcPerHour))

			// Health assessment
			var health string
			switch {
			case fullCount > 0:
				health = CriticalStyle.Render("🔴 Full GCs detected - heap tuning needed")
			case gcPerHour > 1000:
				health = CriticalStyle.Render("🔴 Very high GC frequency")
			case gcPerHour > 360: // Every 10 seconds
				health = WarningStyle.Render("⚠️ High GC frequency")
			default:
				health = GoodStyle.Render("✅ Normal GC frequency")
			}

			lines = append(lines, "")
			lines = append(lines, fmt.Sprintf("Assessment: %s", health))
		}
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		title,
		"",
		strings.Join(lines, "\n"))
}

// Calculate usable chart width accounting for margins and borders
func (m *Model) calculateChartWidth() int {
	// Account for Y-axis labels (8 chars), border chars (2), and some margin (4)
	minWidth := 20
	usableWidth := m.width - 14 // Y-axis + margins
	return max(minWidth, usableWidth)
}

// Improved line chart that spreads data across full width
func (m *Model) createImprovedLineChart(values []float64, timestamps []time.Time, width, height int, unit, title string) string {
	if len(values) == 0 {
		return "No data"
	}

	maxVal := maxOf(values)
	minVal := minOf(values)
	if maxVal == minVal {
		maxVal = minVal + 1 // Avoid division by zero
	}

	var lines []string
	lines = append(lines, MutedStyle.Render(title+":"))

	// Chart area
	for row := 0; row < height; row++ {
		line := ""
		threshold := maxVal - (maxVal-minVal)*float64(row)/float64(height-1)

		// Y-axis label with better formatting
		label := fmt.Sprintf("%6.0f%s", threshold, unit)
		line += MutedStyle.Render(label + " ┤")

		// Spread data points across full width
		for col := 0; col < width; col++ {
			// Map column to data point index
			dataIndex := int(float64(col) * float64(len(values)-1) / float64(width-1))
			if dataIndex >= len(values) {
				dataIndex = len(values) - 1
			}

			val := values[dataIndex]

			// Color code based on value relative to max
			var char string
			if val >= threshold {
				if val >= maxVal*0.9 {
					char = CriticalStyle.Render("█")
				} else if val >= maxVal*0.7 {
					char = WarningStyle.Render("█")
				} else {
					char = InfoStyle.Render("█")
				}
			} else {
				char = "░"
			}
			line += char
		}

		lines = append(lines, line)
	}

	// Time axis
	if len(timestamps) > 0 {
		axisLine := strings.Repeat(" ", 9) + "└" + strings.Repeat("─", width)
		lines = append(lines, MutedStyle.Render(axisLine))

		// Time labels spread across width
		timeLine := strings.Repeat(" ", 10)
		numLabels := min(8, width/8) // Show reasonable number of time labels

		for i := 0; i < numLabels; i++ {
			timeIndex := int(float64(i) * float64(len(timestamps)-1) / float64(numLabels-1))
			if timeIndex >= len(timestamps) {
				timeIndex = len(timestamps) - 1
			}

			label := timestamps[timeIndex].Format("15:04")

			// Calculate position for this label
			pos := int(float64(i) * float64(width) / float64(numLabels-1))

			// Add spacing to reach the correct position
			for len(timeLine)-10 < pos {
				timeLine += " "
			}
			timeLine += label
		}
		lines = append(lines, MutedStyle.Render(timeLine))
	}

	return strings.Join(lines, "\n")
}

// Combined heap chart showing before/after on same timeline
func (m *Model) createCombinedHeapChart(beforeUtil, afterUtil []float64, timestamps []time.Time, width, height int) string {
	if len(beforeUtil) == 0 || len(afterUtil) == 0 {
		return "No data"
	}

	maxVal := 100.0 // Percentage chart
	var lines []string

	// Calculate how many events we can show based on width
	// Reserve some space between bars
	availableWidth := width
	numEvents := len(beforeUtil)

	// If we have more events than can fit, sample them evenly
	eventIndices := make([]int, 0)
	if numEvents <= availableWidth {
		for i := 0; i < numEvents; i++ {
			eventIndices = append(eventIndices, i)
		}
	} else {
		// Sample events evenly across the dataset
		for i := 0; i < availableWidth; i++ {
			idx := int(float64(i) * float64(numEvents-1) / float64(availableWidth-1))
			eventIndices = append(eventIndices, idx)
		}
	}

	// Chart area
	for row := 0; row < height; row++ {
		line := ""
		threshold := maxVal - maxVal*float64(row)/float64(height-1)

		// Y-axis label
		label := fmt.Sprintf("%3.0f%%", threshold)
		line += MutedStyle.Render(label + " ┤")

		// Draw bars for each event
		for col := 0; col < len(eventIndices) && col < width; col++ {
			eventIdx := eventIndices[col]
			beforeVal := beforeUtil[eventIdx]
			afterVal := afterUtil[eventIdx]

			var char string
			if beforeVal >= threshold {
				// We're within the "before GC" range
				if afterVal >= threshold {
					// After GC level - this portion remains after GC (colored)
					char = GoodStyle.Render("█")
				} else {
					// This portion was freed by GC (empty/lighter)
					char = MutedStyle.Render("░")
				}
			} else {
				// Below the before GC usage level
				char = " "
			}

			line += char
		}

		// Fill remaining width with spaces if needed
		for len(line) < width+6 {
			line += " "
		}

		lines = append(lines, line)
	}

	// Time axis
	if len(timestamps) > 0 {
		axisLine := strings.Repeat(" ", 6) + "└" + strings.Repeat("─", len(eventIndices))
		lines = append(lines, MutedStyle.Render(axisLine))

		// Time labels - show labels for some of the events
		timeLine := strings.Repeat(" ", 7)
		numLabels := 6
		if len(eventIndices) < numLabels {
			numLabels = len(eventIndices)
		}

		for i := 0; i < numLabels; i++ {
			labelIdx := int(float64(i) * float64(len(eventIndices)-1) / float64(numLabels-1))
			if labelIdx >= len(eventIndices) {
				labelIdx = len(eventIndices) - 1
			}

			eventIdx := eventIndices[labelIdx]
			if eventIdx < len(timestamps) {
				label := timestamps[eventIdx].Format("15:04")

				// Calculate position
				pos := int(float64(i) * float64(len(eventIndices)) / float64(numLabels-1))

				// Add spacing
				for len(timeLine)-7 < pos {
					timeLine += " "
				}
				timeLine += label
			}
		}
		lines = append(lines, MutedStyle.Render(timeLine))
	}

	return strings.Join(lines, "\n")
}

func (m *Model) getRecentEvents() []*gc.GCEvent {
	if len(m.events) <= m.trendsState.timeWindow {
		return m.events
	}

	// Return the most recent events
	start := len(m.events) - m.trendsState.timeWindow
	return m.events[start:]
}

// Helper functions
func average(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func maxOf(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	maxVal := values[0]
	for _, v := range values {
		if v > maxVal {
			maxVal = v
		}
	}
	return maxVal
}

func minOf(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	minVal := values[0]
	for _, v := range values {
		if v < minVal {
			minVal = v
		}
	}
	return minVal
}

func percentile(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}

	// Simple percentile calculation
	sorted := make([]float64, len(values))
	copy(sorted, values)

	// Basic bubble sort for small datasets
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	index := int(float64(len(sorted)) * p)
	if index >= len(sorted) {
		index = len(sorted) - 1
	}

	return sorted[index]
}

func renderNoTrendsData() string {
	return MutedStyle.Render("No GC events available for trend analysis.")
}

func renderInsufficientTrendsData() string {
	return MutedStyle.Render("Insufficient data for trend analysis.\n\nAt least 2 GC events are required.")
}
