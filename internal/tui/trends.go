package tui

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/mabhi256/jdiag/internal/gc"
)

const (
	ChartMarginWidth = 14
	CausesChartPad   = 30
	FreqChartPad     = 38
)

// Tab names for cleaner code
var trendNames = map[TrendSubTab]string{
	HeapAfterTrend:     "HeapAfter",
	HeapBeforeTrend:    "HeapBefore",
	MemReclaimedTrend:  "MemReclaimed",
	GCDurationTrend:    "GCDuration",
	PauseDurationTrend: "PauseDuration",
	PromotionTrend:     "Promotion",
	FrequencyTrend:     "Collection Freq",
}

func (m *Model) RenderTrends() string {
	if len(m.events) == 0 {
		return MutedStyle.Render("No GC events available for trend analysis.")
	}

	events := m.getRecentEvents()
	if len(events) < 2 {
		return MutedStyle.Render("Insufficient data for trend analysis.\n\nAt least 2 GC events are required.")
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		m.renderTrendsHeader(),
		"",
		m.renderTrendsContent(events))
}

func (m *Model) renderTrendsHeader() string {
	// Build tab line with active/inactive styling
	var tabs []string
	for trend := HeapAfterTrend; trend <= FrequencyTrend; trend++ {
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
	case HeapAfterTrend:
		return m.renderHeapTrends(events, "Heap After GC", "MB",
			func(e *gc.GCEvent) float64 {
				return e.HeapAfter.MB()
			})
	case HeapBeforeTrend:
		return m.renderHeapTrends(events, "Heap Before GC", "MB",
			func(e *gc.GCEvent) float64 {
				return e.HeapBefore.MB()
			})
	case MemReclaimedTrend:
		return m.renderHeapTrends(events, "Memory Reclaimed after GC", "MB",
			func(e *gc.GCEvent) float64 {
				reclaimed := e.HeapBefore - e.HeapAfter
				return reclaimed.MB()
			})
	case GCDurationTrend:
		return m.renderHeapTrends(events, "GC (User + Sys) Duration", "ms",
			func(e *gc.GCEvent) float64 {
				gcDuration := e.UserTime + e.SystemTime
				return float64(gcDuration.Nanoseconds()) / 1e6 // convert to ms
			})
	case PauseDurationTrend:
		return m.renderHeapTrends(events, "GC Pause Duration", "ms",
			func(e *gc.GCEvent) float64 {
				return float64(e.Duration.Nanoseconds()) / 1e6
			})
	case PromotionTrend:
		result := m.renderHeapTrends(events, "Young -> Old Promotions", "MB",
			func(e *gc.GCEvent) float64 {
				return e.RegionSize.Mul(float64(GetPromotedRegions(e))).MB()
			})
		return result + "\n" + MutedStyle.Render("Cannot calculate reliably for Mixed and Full GC")
	case FrequencyTrend:
		return m.renderFrequencyTrends(events)
	default:
		return "Unknown trend view"
	}
}

func (m *Model) renderHeapTrends(
	events []*gc.GCEvent,
	header, unit string,
	f func(*gc.GCEvent) float64,
) string {
	if len(events) == 0 {
		return "No events to display"
	}

	title := TitleStyle.Render(header)

	// Extract data from events
	values := make([]float64, 0)
	timestamps := make([]time.Time, 0)
	gcTypes := make([]string, 0)

	for _, event := range events {
		if strings.Contains(event.Type, "Concurrent") {
			continue
		}
		values = append(values, f(event))
		timestamps = append(timestamps, event.Timestamp)
		gcTypes = append(gcTypes, event.Type)
	}

	if len(values) == 0 {
		return TitleStyle.Render(title) + "\n\nNo data available"
	}

	config := ChartConfig{
		Width:  m.calculateChartWidth(),
		Height: ChartHeight,
		Styles: ChartStyles{
			Muted:    MutedStyle,
			Good:     GoodStyle,
			Info:     InfoStyle,
			Critical: CriticalStyle,
			Warning:  WarningStyle,
		},
	}

	chart := CreatePlot(values, timestamps, gcTypes, unit, config)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		TitleStyle.Render(title),
		"",
		chart)
}

func (m *Model) renderFrequencyTrends(events []*gc.GCEvent) string {
	title := TitleStyle.Render("Collection Time Analysis")

	// Calculate stats by GC type
	stats := make(map[string]struct {
		duration time.Duration
		count    int
	})
	for _, event := range events {
		category := m.categorizeGCType(event.Type)
		s := stats[category]
		if category == "Concurrent Mark" {
			s.duration += event.ConcurrentDuration
		} else {
			s.duration += event.Duration
		}
		s.count++
		stats[category] = s
	}

	if len(stats) == 0 {
		return title + "\n\nNo events to analyze"
	}

	// Calculate total duration
	var totalDuration time.Duration
	for _, stat := range stats {
		totalDuration += stat.duration
	}

	if totalDuration == 0 {
		return title + "\n\nNo duration data available"
	}

	// Create bars for time distribution
	styleMap := map[string]lipgloss.Style{
		"Young":           GoodStyle,
		"Mixed":           InfoStyle,
		"Full":            CriticalStyle,
		"Concurrent Mark": WarningStyle,
		"Other":           MutedStyle,
	}

	var bars []BarData
	for gcType, stat := range stats {
		if stat.count == 0 {
			continue
		}

		percentage := float64(stat.duration) / float64(totalDuration) * 100
		durationMs := float64(stat.duration.Nanoseconds()) / 1e6

		bars = append(bars, BarData{
			Label: gcType, Value: durationMs, Percentage: percentage,
			Style: styleMap[gcType], Suffix: fmt.Sprintf("- %d events", stat.count),
		})
	}

	config := DefaultBarConfig(m.calculateChartWidth() - FreqChartPad)
	config.ValueFormat = "%.1fms"

	chartTitle := fmt.Sprintf("Time Distribution (last %d events, %.1fms total):",
		len(events), float64(totalDuration.Nanoseconds())/1e6)
	sections := []string{CreateHorizontalBarChart(chartTitle, bars, config)}

	// Add frequency analysis
	if len(events) > 1 {
		duration := events[0].Timestamp.Sub(events[len(events)-1].Timestamp)
		if duration > 0 {
			avgInterval := duration / time.Duration(len(events)-1)
			gcPerHour := float64(time.Hour) / float64(avgInterval)
			sections = append(sections, "",
				fmt.Sprintf("Average Interval: %s", FormatDuration(avgInterval)),
				fmt.Sprintf("GC Events/Hour: %.1f", gcPerHour))
		}
	}

	// Add GC causes chart
	if causeChart := m.renderGCCausesChart(events); causeChart != "" {
		sections = append(sections, "", causeChart)
	}

	return lipgloss.JoinVertical(lipgloss.Left, title, "", strings.Join(sections, "\n"))
}

func (m *Model) categorizeGCType(gcType string) string {
	eventType := strings.ToLower(gcType)
	switch {
	case strings.Contains(eventType, "young"):
		return "Young"
	case strings.Contains(eventType, "mixed"):
		return "Mixed"
	case strings.Contains(eventType, "full"):
		return "Full"
	case strings.Contains(eventType, "concurrent"),
		strings.Contains(eventType, "abort"):
		return "Concurrent Mark"
	default:
		return "Other"
	}
}

func (m *Model) renderGCCausesChart(events []*gc.GCEvent) string {
	if len(events) == 0 {
		return ""
	}

	// Group by cause and calculate totals
	causeDurations := make(map[string]time.Duration)
	for _, event := range events {
		cause := event.Cause
		if cause == "" {
			cause = "Unknown"
		}
		causeDurations[cause] += event.Duration
	}

	var totalDuration time.Duration
	for _, duration := range causeDurations {
		totalDuration += duration
	}

	if totalDuration == 0 {
		return "GC Causes (Total Time)\n\nNo duration data available"
	}

	// Create and sort bars
	var bars []BarData
	colors := []lipgloss.Style{GoodStyle, InfoStyle, WarningStyle, CriticalStyle, MutedStyle}

	for cause, duration := range causeDurations {
		percent := float64(duration) / float64(totalDuration) * 100
		durationMs := float64(duration.Nanoseconds()) / 1e6
		bars = append(bars, BarData{
			Label: cause, Value: durationMs, Percentage: percent,
			Style: colors[len(bars)%len(colors)],
		})
	}

	// Sort by duration descending and assign colors
	slices.SortFunc(bars, func(a, b BarData) int {
		if a.Value > b.Value {
			return -1
		}
		if a.Value < b.Value {
			return 1
		}
		return 0
	})

	if len(bars) > len(colors) {
		bars = bars[:len(colors)]
	}
	for i := range bars {
		bars[i].Style = colors[i]
	}

	config := DefaultBarConfig(m.calculateChartWidth() - CausesChartPad)
	config.LabelWidth = 24
	config.ValueFormat = "%.1fms"

	barChart := CreateHorizontalBarChart("GC Causes (Total Time)", bars, config)
	totalMs := float64(totalDuration.Nanoseconds()) / 1e6

	return fmt.Sprintf("%s\n\nTotal GC Time: %.1f ms", barChart, totalMs)
}

func (m *Model) calculateChartWidth() int {
	return max(MinChartWidth, m.width-ChartMarginWidth)
}

func (m *Model) getRecentEvents() []*gc.GCEvent {
	if len(m.events) <= m.trendsState.timeWindow {
		return m.events
	}
	return m.events[len(m.events)-m.trendsState.timeWindow:]
}

func GetPromotedRegions(e *gc.GCEvent) int {
	// For mixed GC, use young regions collected as upper bound
	// and old regions net change as lower bound
	// In mixed - some old regions are also collected
	// Before - 50 young, 20 old
	// GC - 10 young promoted, 40 young freed, 5 young freed
	// After - 0 young, 25 old, but old_1 - old_0 = 5, which is wrong
	switch e.Type {
	case "Young":
		// G1GC can collect old regions opportunistically during young GC
		return max(0, e.OldRegionsAfter-e.OldRegionsBefore)
	default:
		return 0
	}
}
