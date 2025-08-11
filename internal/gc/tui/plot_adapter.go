package tui

import (
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/mabhi256/jdiag/utils"
)

// LipglossRenderer wraps a lipgloss.Style to implement the utils.Renderer interface
type LipglossRenderer struct {
	Style lipgloss.Style
}

func (lr LipglossRenderer) Render(text string) string {
	return lr.Style.Render(text)
}

// GCIconMapper provides mapping from GC types to styled icons
type GCIconMapper struct {
	Styles utils.ChartStyles
}

// GetStyledIcon returns a styled icon for the given GC type
func (g GCIconMapper) GetStyledIcon(gcType string) string {
	switch strings.ToLower(gcType) {
	case "young":
		return g.Styles.Good.Render("●")
	case "mixed":
		return g.Styles.Info.Render("▲")
	case "full":
		return g.Styles.Critical.Render("■")
	case "concurrent mark abort":
		return g.Styles.Warning.Render("◆")
	default:
		return g.Styles.Muted.Render("•")
	}
}

// CreateGCLegend creates a legend string for garbage collection types
func CreateGCLegend(styles utils.ChartStyles) string {
	mapper := GCIconMapper{Styles: styles}
	return "Legend: " + mapper.GetStyledIcon("young") + " Young " +
		mapper.GetStyledIcon("mixed") + " Mixed " +
		mapper.GetStyledIcon("full") + " Full"
}

// CreateChartStyles creates chart styles using your existing TUI styles
func CreateChartStyles() utils.ChartStyles {
	return utils.ChartStyles{
		Muted:    LipglossRenderer{utils.MutedStyle},
		Good:     LipglossRenderer{utils.GoodStyle},
		Info:     LipglossRenderer{utils.InfoStyle},
		Critical: LipglossRenderer{utils.CriticalStyle},
		Warning:  LipglossRenderer{utils.WarningStyle},
	}
}

// CreatePlotFromGCData creates a plot specifically for GC data with proper styling and legend
func CreatePlotFromGCData(values []float64, timestamps []time.Time, gcTypes []string, unit string, width, height int) string {
	styles := CreateChartStyles()
	mapper := GCIconMapper{Styles: styles}

	// Convert to DataPoints with styled icons
	dataPoints := make([]utils.DataPoint, len(values))
	for i, val := range values {
		var ts time.Time
		if i < len(timestamps) {
			ts = timestamps[i]
		}
		var gcType string
		if i < len(gcTypes) {
			gcType = gcTypes[i]
		}

		dataPoints[i] = utils.DataPoint{
			Value:     val,
			Timestamp: ts,
			Icon:      mapper.GetStyledIcon(gcType),
		}
	}

	config := utils.ChartConfig{
		Width:  width,
		Height: height,
		Styles: styles,
		Legend: CreateGCLegend(styles),
	}

	return utils.CreatePlot(dataPoints, unit, config)
}

// CreateSimplePlot creates a basic plot with default styling (backward compatibility)
func CreateSimplePlot(values []float64, timestamps []time.Time, unit string, width, height int) string {
	styles := CreateChartStyles()
	config := utils.ChartConfig{
		Width:  width,
		Height: height,
		Styles: styles,
		Legend: "", // No legend for simple plots
	}
	return utils.CreateSimplePlot(values, timestamps, unit, config)
}
