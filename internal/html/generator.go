package html

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mabhi256/jdiag/internal/gc"
	"github.com/mabhi256/jdiag/utils"
)

// Embed template files at compile time
//
//go:embed templates/template.html
var htmlTemplate string

//go:embed templates/styles.css
var cssContent string

//go:embed dist/app.js
var jsContent string

// HTMLReportData contains all data needed for the compact HTML report
type HTMLReportData struct {
	Events      []*gc.GCEvent  `json:"events"`
	Analysis    *gc.GCAnalysis `json:"analysis"`
	Issues      *gc.GCIssues   `json:"issues"`
	GeneratedAt time.Time      `json:"generatedAt"`
	JVMInfo     JVMInfo        `json:"jvmInfo"`
	ChartData   ChartData      `json:"chartData"`
}

type JVMInfo struct {
	Version        string `json:"version"`
	HeapRegionSize string `json:"heapRegionSize"`
	HeapMax        string `json:"heapMax"`
	TotalRuntime   string `json:"totalRuntime"`
	TotalEvents    int    `json:"totalEvents"`
}

type ChartData struct {
	HeapTrends     []TimeSeriesPoint `json:"heapTrends"`
	PauseTrends    []TimeSeriesPoint `json:"pauseTrends"`
	FrequencyData  []FrequencyPoint  `json:"frequencyData"`
	AllocationData []TimeSeriesPoint `json:"allocationData"`
}

type TimeSeriesPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
	Type      string    `json:"type"`
	EventID   int       `json:"eventId"`
}

type FrequencyPoint struct {
	Label      string  `json:"label"`
	Value      float64 `json:"value"`
	Percentage float64 `json:"percentage"`
	Count      int     `json:"count"`
}

func GenerateHTMLReport(events []*gc.GCEvent, analysis *gc.GCAnalysis, issues *gc.GCIssues, outputPath string) (string, error) {
	// Validate inputs
	if err := validateReportData(events, analysis, issues); err != nil {
		return "", fmt.Errorf("invalid report data: %v", err)
	}

	reportData := &HTMLReportData{
		Events:      events,
		Analysis:    analysis,
		Issues:      issues,
		GeneratedAt: time.Now(),
		JVMInfo:     extractJVMInfo(analysis),
		ChartData:   generateChartData(events),
	}

	// Serialize data to JSON for JavaScript
	jsonData, err := json.Marshal(reportData)
	if err != nil {
		return "", fmt.Errorf("failed to marshal report data: %v", err)
	}

	htmlContent := generateSingleFileHTMLContent(string(jsonData))

	// Get safe output path
	absPath, err := GetOutputPath(outputPath)
	if err != nil {
		return "", err
	}
	fmt.Println("path:", outputPath, "absPath:", absPath)
	// Write to file
	if err := os.WriteFile(absPath, []byte(htmlContent), 0644); err != nil {
		return "", fmt.Errorf("failed to write HTML file: %v", err)
	}

	return absPath, nil
}

func extractJVMInfo(analysis *gc.GCAnalysis) JVMInfo {
	return JVMInfo{
		Version:        analysis.JVMVersion,
		HeapRegionSize: analysis.HeapRegionSize.String(),
		HeapMax:        analysis.HeapMax.String(),
		TotalRuntime:   utils.FormatDuration(analysis.TotalRuntime),
		TotalEvents:    analysis.TotalEvents,
	}
}

func generateChartData(events []*gc.GCEvent) ChartData {
	var heapTrends, pauseTrends, allocationTrends []TimeSeriesPoint
	frequencyMap := make(map[string]struct {
		duration time.Duration
		count    int
	})

	// Optimize for performance - limit data points for very large datasets
	sampleRate := 1
	if len(events) > 1000 {
		sampleRate = len(events) / 1000 // Sample down for performance
	}

	for i, event := range events {
		// Skip events based on sample rate for very large datasets
		if i%sampleRate != 0 && len(events) > 1000 {
			continue
		}

		timestamp := event.Timestamp

		// Heap trends - only include events with meaningful heap changes
		if event.HeapAfter.MB() > 0 {
			heapTrends = append(heapTrends, TimeSeriesPoint{
				Timestamp: timestamp,
				Value:     event.HeapAfter.MB(),
				Type:      event.Type,
				EventID:   i,
			})
		}

		// Pause trends - only include events with actual pause times
		if event.Duration > 0 {
			pauseTrends = append(pauseTrends, TimeSeriesPoint{
				Timestamp: timestamp,
				Value:     float64(event.Duration.Milliseconds()),
				Type:      event.Type,
				EventID:   i,
			})
		}

		// Allocation trends - only include events with allocation data
		if event.AllocationRateToEvent > 0 {
			allocationTrends = append(allocationTrends, TimeSeriesPoint{
				Timestamp: timestamp,
				Value:     event.AllocationRateToEvent,
				Type:      event.Type,
				EventID:   i,
			})
		}

		// Frequency data by GC type
		gcType := categorizeGCType(event.Type)
		freq := frequencyMap[gcType]
		freq.duration += event.Duration
		freq.count++
		frequencyMap[gcType] = freq
	}

	// Convert frequency map to slice
	var frequencyData []FrequencyPoint
	var totalDuration time.Duration
	for _, freq := range frequencyMap {
		totalDuration += freq.duration
	}

	for gcType, freq := range frequencyMap {
		if totalDuration > 0 {
			percentage := float64(freq.duration) / float64(totalDuration) * 100
			frequencyData = append(frequencyData, FrequencyPoint{
				Label:      gcType,
				Value:      float64(freq.duration.Milliseconds()),
				Percentage: percentage,
				Count:      freq.count,
			})
		}
	}

	return ChartData{
		HeapTrends:     heapTrends,
		PauseTrends:    pauseTrends,
		FrequencyData:  frequencyData,
		AllocationData: allocationTrends,
	}
}

func categorizeGCType(gcType string) string {
	eventType := strings.ToLower(gcType)
	switch {
	case strings.Contains(eventType, "young"):
		return "Young"
	case strings.Contains(eventType, "mixed"):
		return "Mixed"
	case strings.Contains(eventType, "full"):
		return "Full"
	case strings.Contains(eventType, "concurrent"):
		return "Concurrent"
	default:
		return "Other"
	}
}

// Helper functions for better error handling and data validation
func validateReportData(events []*gc.GCEvent, analysis *gc.GCAnalysis, issues *gc.GCIssues) error {
	if events == nil {
		return fmt.Errorf("events cannot be nil")
	}
	if analysis == nil {
		return fmt.Errorf("analysis cannot be nil")
	}
	if issues == nil {
		return fmt.Errorf("issues cannot be nil")
	}
	if len(events) == 0 {
		return fmt.Errorf("no GC events found")
	}
	return nil
}

// GetOutputPath returns a safe output path, creating directories if needed
func GetOutputPath(path string) (string, error) {
	var outputPath string

	if path != "" {
		outputPath = path
	} else {
		outputPath = GetDefaultOutputPath()
	}

	// Ensure .html extension
	if !strings.HasSuffix(strings.ToLower(outputPath), ".html") {
		outputPath += ".html"
	}

	// Convert to absolute path
	absPath, err := filepath.Abs(outputPath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path for %s: %v", outputPath, err)
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory %s: %v", dir, err)
	}

	return absPath, nil
}

// generateSingleFileHTMLContent creates the single-file HTML with embedded CSS/JS
func generateSingleFileHTMLContent(jsonData string) string {
	// Replace placeholders in the HTML template
	content := htmlTemplate

	content = strings.ReplaceAll(content, "{{CSS_CONTENT}}", cssContent)
	content = strings.ReplaceAll(content, "{{JS_CONTENT}}", jsContent)
	content = strings.ReplaceAll(content, "{{JSON_DATA}}", jsonData)

	return content
}

// GetDefaultOutputPath returns a default HTML output path
func GetDefaultOutputPath() string {
	timestamp := time.Now().Format("20060102_150405")
	return fmt.Sprintf("gc-analysis-%s.html", timestamp)
}
