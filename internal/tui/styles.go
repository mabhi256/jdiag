package tui

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
)

var (
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
		return "🔴 Critical"
	case "warning":
		return "⚠️  Warning"
	case "info":
		return "ℹ️  Info"
	default:
		return "✅ Good"
	}
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
