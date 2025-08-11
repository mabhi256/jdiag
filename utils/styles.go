package utils

import (
	"fmt"
	"math"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
)

var (
	CriticalColor = lipgloss.Color("#CC3333") // Dark red
	WarningColor  = lipgloss.Color("#FF8800") // Orange
	GoodColor     = lipgloss.Color("#228B22") // Forest green
	InfoColor     = lipgloss.Color("#4682B4") // Steel blue
	TextColor     = lipgloss.Color("#CCCCCC") // Light gray
	MutedColor    = lipgloss.Color("#888888") // Medium gray
	BorderColor   = lipgloss.Color("#666666") // Dark gray

	CriticalLightColor = lipgloss.Color("#FF6666") // Lighter red
	WarningLightColor  = lipgloss.Color("#FFAA44") // Lighter orange
	GoodLightColor     = lipgloss.Color("#66BB66") // Lighter green
	InfoLightColor     = lipgloss.Color("#88AACC") // Lighter blue
)

var (
	CriticalStyle = lipgloss.NewStyle().Foreground(CriticalColor).Bold(true)
	WarningStyle  = lipgloss.NewStyle().Foreground(WarningColor).Bold(true)
	GoodStyle     = lipgloss.NewStyle().Foreground(GoodColor).Bold(true)
	InfoStyle     = lipgloss.NewStyle().Foreground(InfoColor)
	MutedStyle    = lipgloss.NewStyle().Foreground(MutedColor)
	TextStyle     = lipgloss.NewStyle().Foreground(TextColor)

	CriticalLightStyle = lipgloss.NewStyle().Foreground(CriticalLightColor)
	WarningLightStyle  = lipgloss.NewStyle().Foreground(WarningLightColor)
	GoodLightStyle     = lipgloss.NewStyle().Foreground(GoodLightColor)
	InfoLightStyle     = lipgloss.NewStyle().Foreground(InfoLightColor)
)

var (
	TabActiveStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(InfoColor).
			Padding(0, 1).
			Bold(true)

	TabInactiveStyle = lipgloss.NewStyle().
				Foreground(MutedColor).
				Padding(0, 1)
)

var (
	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(BorderColor).
			Padding(1, 2)

	TitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Bold(true).
			Padding(0, 1)

	HeaderStyle = lipgloss.NewStyle().
			Foreground(TextColor).
			Background(lipgloss.Color("#1a1a1a")).
			Bold(true).
			Padding(0, 1)

	StatusBarStyle = lipgloss.NewStyle().
			Foreground(TextColor).
			Background(MutedColor).
			Padding(0, 1)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(CriticalColor).
			Background(lipgloss.Color("#1a1a1a")).
			Bold(true).
			Padding(1, 2).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(CriticalColor)

	HelpBarStyle = lipgloss.NewStyle().
			Foreground(MutedColor).
			Background(lipgloss.Color("#1a1a1a")).
			Width(0). // Will be set dynamically
			Padding(0, 1)
)

type TerminalCapabilities struct {
	SupportsUnicode bool
	SupportsColor   bool
	Width           int
}

var termCaps *TerminalCapabilities

func init() {
	termCaps = detectTerminalCapabilities()
}

func detectTerminalCapabilities() *TerminalCapabilities {
	caps := &TerminalCapabilities{
		SupportsUnicode: true, // Default to true, fallback if needed
		SupportsColor:   true, // Most modern terminals support color
		Width:           80,   // Default width
	}

	// Check TERM environment variable
	term := os.Getenv("TERM")
	if strings.Contains(term, "xterm") || strings.Contains(term, "color") {
		caps.SupportsColor = true
	}

	// Test unicode support by checking if we can measure unicode characters properly
	testStr := "█░"
	if utf8.RuneCountInString(testStr) != len([]rune(testStr)) {
		caps.SupportsUnicode = false
	}

	return caps
}

type ProgressBarConfig struct {
	Width     int
	FillChar  string
	EmptyChar string
	UseColor  bool
}

func GetProgressBarConfig(width int) ProgressBarConfig {
	config := ProgressBarConfig{
		Width:    width,
		UseColor: termCaps.SupportsColor,
	}

	if termCaps.SupportsUnicode {
		config.FillChar = "█"
		config.EmptyChar = "░"
	} else {
		config.FillChar = "#"
		config.EmptyChar = "-"
	}

	return config
}

func CreateProgressBar(percentage float64, width int, color lipgloss.Color) string {
	if width < 4 {
		return fmt.Sprintf("%.0f%%", percentage*100)
	}

	config := GetProgressBarConfig(width)

	// Calculate filled portion
	filled := int(math.Round(percentage * float64(config.Width)))
	if filled > config.Width {
		filled = config.Width
	}
	if filled < 0 {
		filled = 0
	}

	// Build bar
	bar := strings.Repeat(config.FillChar, filled) +
		strings.Repeat(config.EmptyChar, config.Width-filled)

	if config.UseColor && color != "" {
		style := lipgloss.NewStyle().Foreground(color)
		bar = style.Render(bar)
	}

	return bar
}

func CreateProgressBarWithLabel(percentage float64, width int, color lipgloss.Color, label string) string {
	if width < 10 {
		return fmt.Sprintf("%.0f%%", percentage*100)
	}

	// Reserve space for label
	labelSpace := len(label) + 1
	barWidth := width - labelSpace

	if barWidth < 4 {
		return label
	}

	bar := CreateProgressBar(percentage, barWidth, color)
	return fmt.Sprintf("%s %s", bar, label)
}

func CreateTargetProgressBar(current, target float64, width int, better string) string {
	if width < 15 {
		return fmt.Sprintf("%.1f/%.1f", current, target)
	}

	// Calculate performance ratio
	var performance float64
	if better == "higher" {
		performance = current / target
	} else {
		performance = target / current
	}

	// Determine status and color based on performance
	var color lipgloss.Color
	var status string
	switch {
	case performance >= 1.0:
		color, status = GoodColor, "✅"
	case performance >= 0.8:
		color, status = WarningColor, "⚠️"
	default:
		color, status = CriticalColor, "🔴"
	}

	// Clamp percentage for progress bar display
	percentage := math.Min(performance, 1.0)

	label := fmt.Sprintf("%.1f (target: %.1f) %s", current, target, status)
	barWidth := width - len(label) - 1

	if barWidth < 4 {
		return label
	}

	bar := CreateProgressBar(percentage, barWidth, color)
	return fmt.Sprintf("%s %s", bar, label)
}

func GetSeverityStyle(severity string) lipgloss.Style {
	switch strings.ToLower(severity) {
	case "critical":
		return CriticalStyle
	case "warning":
		return WarningStyle
	case "info":
		return InfoStyle
	default:
		return GoodStyle
	}
}

func GetSeverityIcon(severity string) string {
	switch strings.ToLower(severity) {
	case "critical":
		return "🔴"
	case "warning":
		return "⚠️"
	case "info":
		return "ℹ️"
	default:
		return "✅"
	}
}

// GetSeverityIconWithText returns icon with text for severity level
func GetSeverityIconWithText(severity string) string {
	switch strings.ToLower(severity) {
	case "critical":
		return "🔴 Critical"
	case "warning":
		return "⚠️  Warning"
	case "info":
		return "ℹ️  Info"
	default:
		return "✅ Good"
	}
}

func GetMemoryPressureStyle(pressure string) lipgloss.Style {
	switch pressure {
	case "critical":
		return CriticalStyle
	case "high":
		return WarningStyle
	case "moderate":
		return InfoStyle
	default:
		return GoodStyle
	}
}

func GetMemoryPressureIcon(level string) string {
	switch level {
	case "critical":
		return "🔴"
	case "high":
		return "🟡"
	case "moderate":
		return "🟠"
	default:
		return "🟢"
	}
}

func GetTrendIcon(trend float64) string {
	if trend > 0.05 { // Rising more than 5%
		return "📈"
	} else if trend < -0.05 { // Falling more than 5%
		return "📉"
	}
	return "➡️" // Stable
}

func CreateStatusIndicator(status, text string, color lipgloss.Color) string {
	var icon string
	switch status {
	case "connected":
		icon = "🟢"
	case "disconnected":
		icon = "🔴"
	case "warning":
		icon = "🟡"
	case "error":
		icon = "❌"
	default:
		icon = "⚫"
	}

	style := lipgloss.NewStyle().Foreground(color).Bold(true)
	return style.Render(fmt.Sprintf("%s %s", icon, text))
}

// CreateMetricDisplay creates a formatted metric display
func CreateMetricDisplay(name, value, unit string, color lipgloss.Color) string {
	nameStyle := InfoStyle
	valueStyle := lipgloss.NewStyle().Foreground(color).Bold(true)
	unitStyle := MutedStyle

	return fmt.Sprintf("%s: %s%s",
		nameStyle.Render(name),
		valueStyle.Render(value),
		unitStyle.Render(unit))
}

// CreateSparkline creates a simple sparkline chart
func CreateSparkline(values []float64, width int) string {
	if len(values) == 0 || width <= 0 {
		return ""
	}

	// Find min and max values
	min, max := values[0], values[0]
	for _, v := range values {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}

	// Avoid division by zero
	if max == min {
		return strings.Repeat("─", width)
	}

	// Create sparkline characters
	chars := []string{"▁", "▂", "▃", "▄", "▅", "▆", "▇", "█"}

	var result strings.Builder
	for i := 0; i < width && i < len(values); i++ {
		// Normalize value to 0-1 range
		normalized := (values[i] - min) / (max - min)

		// Map to character index
		charIndex := int(normalized * float64(len(chars)-1))
		if charIndex >= len(chars) {
			charIndex = len(chars) - 1
		}

		result.WriteString(chars[charIndex])
	}

	return result.String()
}

// CreateGauge creates a gauge-style progress indicator
func CreateGauge(value, min, max float64, width int, color lipgloss.Color) string {
	if max <= min || width <= 0 {
		return ""
	}

	// Normalize value to 0-1 range
	normalized := (value - min) / (max - min)
	if normalized < 0 {
		normalized = 0
	}
	if normalized > 1 {
		normalized = 1
	}

	return CreateProgressBar(normalized, width, color)
}

// Responsive width calculations
func CalculateBarWidth(totalWidth, sections int) int {
	sectionWidth := totalWidth / sections
	barWidth := sectionWidth - 10 // Leave space for labels
	if barWidth < 10 {
		barWidth = 10
	}
	return barWidth
}

// Table-like formatting for aligned data
func FormatKeyValue(key, value string, keyWidth int) string {
	keyStyled := InfoStyle.Width(keyWidth).Render(key + ":")
	valueStyled := TextStyle.Render(value)
	return lipgloss.JoinHorizontal(lipgloss.Left, keyStyled, " ", valueStyled)
}

// Multi-column layout helper
func CreateColumns(content []string, totalWidth int) string {
	if len(content) == 0 {
		return ""
	}

	if len(content) == 1 {
		return content[0]
	}

	columnWidth := totalWidth / len(content)
	var columns []string

	for _, c := range content {
		column := lipgloss.NewStyle().
			Width(columnWidth).
			Render(c)
		columns = append(columns, column)
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, columns...)
}

// TruncateString truncates a string to fit within maxWidth
func TruncateString(s string, maxWidth int) string {
	if len(s) <= maxWidth {
		return s
	}
	if maxWidth < 4 {
		return strings.Repeat(".", maxWidth)
	}
	return s[:maxWidth-3] + "..."
}

// SanitizeString removes control characters and ensures safe display
func SanitizeString(s string) string {
	// Remove any control characters that might mess up the display
	var result []rune
	for _, r := range s {
		if r >= 32 && r != 127 { // Printable ASCII characters
			result = append(result, r)
		}
	}
	return string(result)
}

// PadRight pads a string to the right to reach the specified width
func PadRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

// WrapText wraps text to fit within specified width
func WrapText(text string, width int) []string {
	if width < 10 {
		return []string{text}
	}

	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{""}
	}

	var lines []string
	var currentLine []string
	currentLength := 0

	for _, word := range words {
		// If adding this word would exceed width, start new line
		if currentLength+len(word)+len(currentLine) > width && len(currentLine) > 0 {
			lines = append(lines, strings.Join(currentLine, " "))
			currentLine = []string{word}
			currentLength = len(word)
		} else {
			currentLine = append(currentLine, word)
			currentLength += len(word)
		}
	}

	// Add the last line
	if len(currentLine) > 0 {
		lines = append(lines, strings.Join(currentLine, " "))
	}

	return lines
}
