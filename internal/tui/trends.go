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
		PauseTrend:      "Pause Times",
		HeapTrend:       "Heap Usage",
		AllocationTrend: "Allocation Rate",
		FrequencyTrend:  "Collection Freq",
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
	case AllocationTrend:
		return m.renderAllocationTrends(events)
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
	var labels []string
	var maxPause float64

	for _, event := range events {
		pauseMs := float64(event.Duration.Nanoseconds()) / 1000000
		pauses = append(pauses, pauseMs)
		maxPause = math.Max(maxPause, pauseMs)
		labels = append(labels, event.Timestamp.Format("15:04"))
	}

	// Create ASCII chart
	chartHeight := 12
	chartWidth := max(10, m.width-10) // Ensure minimum width
	chart := m.createLineChart(pauses, labels, chartWidth, chartHeight, "ms")

	// Calculate statistics
	avg := average(pauses)
	p95 := percentile(pauses, 0.95)
	p99 := percentile(pauses, 0.99)

	stats := fmt.Sprintf("Avg: %.1fms  P95: %.1fms  P99: %.1fms  Max: %.1fms",
		avg, p95, p99, maxPause)

	// Trend analysis
	trend := analyzeTrend(pauses)
	trendText := fmt.Sprintf("Trend: %s", trend)

	return lipgloss.JoinVertical(lipgloss.Left,
		title,
		"",
		chart,
		"",
		InfoStyle.Render(stats),
		MutedStyle.Render(trendText))
}

func (m *Model) renderHeapTrends(events []*gc.GCEvent) string {
	title := TitleStyle.Render("Heap Usage Pattern")

	// Extract heap utilization data
	var beforeUtil, afterUtil []float64
	var labels []string

	for _, event := range events {
		if event.HeapTotal.Bytes() > 0 {
			beforePct := float64(event.HeapBefore.Bytes()) / float64(event.HeapTotal.Bytes()) * 100
			afterPct := float64(event.HeapAfter.Bytes()) / float64(event.HeapTotal.Bytes()) * 100

			beforeUtil = append(beforeUtil, beforePct)
			afterUtil = append(afterUtil, afterPct)
			labels = append(labels, event.Timestamp.Format("15:04"))
		}
	}

	if len(beforeUtil) == 0 {
		return "No heap utilization data available"
	}

	// Create heap usage chart
	chartHeight := 6
	chartWidth := max(10, m.width-15) // Ensure minimum width

	beforeChart := m.createAreaChart(beforeUtil, labels, chartWidth, chartHeight, "Before GC")
	afterChart := m.createAreaChart(afterUtil, labels, chartWidth, chartHeight, "After GC")

	// Statistics
	avgBefore := average(beforeUtil)
	avgAfter := average(afterUtil)
	efficiency := (avgBefore - avgAfter) / avgBefore * 100

	stats := fmt.Sprintf("Avg Before: %.1f%%  Avg After: %.1f%%  Efficiency: %.1f%%",
		avgBefore, avgAfter, efficiency)

	return lipgloss.JoinVertical(lipgloss.Left,
		title,
		"",
		InfoStyle.Render("Before GC:"),
		beforeChart,
		"",
		InfoStyle.Render("After GC:"),
		afterChart,
		"",
		InfoStyle.Render(stats))
}

func (m *Model) renderAllocationTrends(events []*gc.GCEvent) string {
	title := TitleStyle.Render("Allocation Rate Over Time")

	// Calculate allocation rates between GC events
	var rates []float64
	var labels []string

	for i := 1; i < len(events); i++ {
		prev := events[i-1]
		curr := events[i]

		timeDiff := curr.Timestamp.Sub(prev.Timestamp)
		if timeDiff > 0 {
			// Allocation = heap growth + collected memory
			allocated := curr.HeapBefore.Bytes() - prev.HeapAfter.Bytes()
			if allocated > 0 {
				rateMBPerSec := float64(allocated) / timeDiff.Seconds() / (1024 * 1024)
				rates = append(rates, rateMBPerSec)
				labels = append(labels, curr.Timestamp.Format("15:04"))
			}
		}
	}

	if len(rates) == 0 {
		return "Insufficient data for allocation rate calculation"
	}

	// Create chart
	chartHeight := 10
	chartWidth := max(10, m.width-15) // Ensure minimum width
	chart := m.createLineChart(rates, labels, chartWidth, chartHeight, "MB/s")

	// Statistics
	avgRate := average(rates)
	maxRate := maxOf(rates)
	minRate := minOf(rates)

	stats := fmt.Sprintf("Avg: %.1f MB/s  Min: %.1f MB/s  Max: %.1f MB/s",
		avgRate, minRate, maxRate)

	// Classification
	var classification string
	switch {
	case avgRate > 500:
		classification = CriticalStyle.Render("🔴 Very High")
	case avgRate > 100:
		classification = WarningStyle.Render("⚠️ High")
	default:
		classification = GoodStyle.Render("✅ Normal")
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		title,
		"",
		chart,
		"",
		InfoStyle.Render(stats),
		fmt.Sprintf("Classification: %s", classification))
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

	// Calculate bar width - ensure we have enough width for the bars
	minWidth := 50 // Minimum width needed for meaningful bars
	if m.width < minWidth {
		return fmt.Sprintf("Terminal too narrow (need at least %d chars)", minWidth)
	}

	barAreaWidth := m.width - 30 // Reserve space for labels
	if barAreaWidth <= 0 {
		barAreaWidth = 20 // Fallback minimum
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("Event Distribution (last %d events):", total))
	lines = append(lines, "")

	// Young GC bar
	if youngCount > 0 {
		barWidth := max(0, int(youngPct*float64(barAreaWidth)/100))
		emptyWidth := max(0, barAreaWidth-barWidth)
		youngBar := strings.Repeat("█", barWidth) + strings.Repeat("░", emptyWidth)
		lines = append(lines, fmt.Sprintf("Young   │%s│ %d (%4.1f%%)",
			GoodStyle.Render(youngBar), youngCount, youngPct))
	}

	// Mixed GC bar
	if mixedCount > 0 {
		barWidth := max(0, int(mixedPct*float64(barAreaWidth)/100))
		emptyWidth := max(0, barAreaWidth-barWidth)
		mixedBar := strings.Repeat("█", barWidth) + strings.Repeat("░", emptyWidth)
		lines = append(lines, fmt.Sprintf("Mixed   │%s│ %d (%4.1f%%)",
			InfoStyle.Render(mixedBar), mixedCount, mixedPct))
	}

	// Full GC bar
	if fullCount > 0 {
		barWidth := max(0, int(fullPct*float64(barAreaWidth)/100))
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

func (m *Model) createLineChart(values []float64, labels []string, width, height int, unit string) string {
	if len(values) == 0 {
		return "No data"
	}

	// Ensure minimum dimensions
	width = max(10, width)
	height = max(3, height)

	maxVal := maxOf(values)
	minVal := minOf(values)
	if maxVal == minVal {
		maxVal = minVal + 1 // Avoid division by zero
	}

	var lines []string

	// Chart area
	for row := 0; row < height; row++ {
		line := ""
		threshold := maxVal - (maxVal-minVal)*float64(row)/float64(height-1)

		// Y-axis label
		label := fmt.Sprintf("%6.0f%s", threshold, unit)
		line += MutedStyle.Render(label + " ┤")

		// Data points
		for i, val := range values {
			if i >= width {
				break
			}

			if val >= threshold {
				line += "█"
			} else if len(values) > 1 && i > 0 && values[i-1] >= threshold {
				line += "╲"
			} else if len(values) > 1 && i < len(values)-1 && values[i+1] >= threshold {
				line += "╱"
			} else {
				line += " "
			}
		}

		lines = append(lines, line)
	}

	// Time axis
	if len(labels) > 0 {
		axisLine := strings.Repeat(" ", 9) + "└" + strings.Repeat("─", width)
		lines = append(lines, MutedStyle.Render(axisLine))

		// Time labels (show every few labels to avoid crowding)
		timeLine := strings.Repeat(" ", 10)
		step := max(1, len(labels)/8) // Show ~8 time labels
		for i := 0; i < len(labels) && i < width; i += step {
			if i > 0 {
				// Ensure we don't have negative spacing
				spacing := max(0, step-len(labels[i]))
				timeLine += strings.Repeat(" ", spacing)
			}
			timeLine += labels[i]
		}
		lines = append(lines, MutedStyle.Render(timeLine))
	}

	return strings.Join(lines, "\n")
}

func (m *Model) createAreaChart(values []float64, labels []string, width, height int, title string) string {
	if len(values) == 0 {
		return "No data"
	}

	// Ensure minimum dimensions
	width = max(10, width)
	height = max(3, height)

	maxVal := 100.0 // Percentage chart

	var lines []string

	// Chart title
	lines = append(lines, MutedStyle.Render(title+":"))

	// Chart area - simplified area chart
	for row := 0; row < height; row++ {
		line := ""
		threshold := maxVal - maxVal*float64(row)/float64(height-1)

		// Y-axis label
		label := fmt.Sprintf("%3.0f%%", threshold)
		line += MutedStyle.Render(label + " ┤")

		// Data area
		for i, val := range values {
			if i >= width {
				break
			}

			if val >= threshold {
				if val >= 90 {
					line += CriticalStyle.Render("█")
				} else if val >= 70 {
					line += WarningStyle.Render("█")
				} else {
					line += GoodStyle.Render("█")
				}
			} else {
				line += "░"
			}
		}

		lines = append(lines, line)
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

func analyzeTrend(values []float64) string {
	if len(values) < 3 {
		return "Insufficient data"
	}

	// Simple linear trend analysis
	n := len(values)
	sumX, sumY, sumXY, sumX2 := 0.0, 0.0, 0.0, 0.0

	for i, v := range values {
		x := float64(i)
		sumX += x
		sumY += v
		sumXY += x * v
		sumX2 += x * x
	}

	slope := (float64(n)*sumXY - sumX*sumY) / (float64(n)*sumX2 - sumX*sumX)

	switch {
	case slope > 1:
		return CriticalStyle.Render("📈 Strongly Increasing")
	case slope > 0.1:
		return WarningStyle.Render("📈 Increasing")
	case slope < -1:
		return GoodStyle.Render("📉 Strongly Decreasing")
	case slope < -0.1:
		return InfoStyle.Render("📉 Decreasing")
	default:
		return MutedStyle.Render("➡️ Stable")
	}
}

func renderNoTrendsData() string {
	return MutedStyle.Render("No GC events available for trend analysis.")
}

func renderInsufficientTrendsData() string {
	return MutedStyle.Render("Insufficient data for trend analysis.\n\nAt least 2 GC events are required.")
}
