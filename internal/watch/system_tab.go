package watch

import (
	"fmt"

	"github.com/mabhi256/jdiag/internal/jmx"
	"github.com/mabhi256/jdiag/utils"

	"github.com/charmbracelet/lipgloss"
)

// Render renders the system tab view
func RenderSystemTab(state *TabState, config *jmx.Config, width int, systemHistory []utils.TimeMap) string {
	var sections []string

	// System overview
	overviewSection := renderSystemOverview(state.System)
	sections = append(sections, overviewSection)

	// Charts section (CPU and Memory charts side by side)
	chartsSection := renderSystemCharts(state.System, width, systemHistory)
	sections = append(sections, chartsSection)

	// JVM information section
	jvmInfoSection := renderJVMInfo(state.System)
	sections = append(sections, jvmInfoSection)

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// renderSystemOverview shows high-level system status
func renderSystemOverview(system *SystemState) string {
	var statusColor lipgloss.Color
	var statusIcon string
	var statusText string

	// Determine overall system health
	cpuLoad := system.ProcessCpuLoad
	systemLoad := system.SystemCpuLoad

	switch {
	case cpuLoad > 0.95 || systemLoad > 0.95:
		statusColor = utils.CriticalColor
		statusIcon = "🔴"
		statusText = "Critical system load"
	case cpuLoad > 0.80 || systemLoad > 0.85:
		statusColor = utils.WarningColor
		statusIcon = "🟡"
		statusText = "High system load"
	case cpuLoad > 0.60 || systemLoad > 0.70:
		statusColor = utils.InfoColor
		statusIcon = "🟠"
		statusText = "Moderate system load"
	default:
		statusColor = utils.GoodColor
		statusIcon = "🟢"
		statusText = "Normal system load"
	}

	// Add uptime info
	uptimeText := ""
	if system.JVMUptime > 0 {
		uptimeText = fmt.Sprintf(" (uptime: %s)", utils.FormatDuration(system.JVMUptime))
	}

	overview := lipgloss.NewStyle().
		Foreground(statusColor).
		Bold(true).
		Render(fmt.Sprintf("%s %s%s", statusIcon, statusText, uptimeText))

	return overview + "\n"
}

// renderSystemCharts shows CPU and Memory charts side by side
func renderSystemCharts(system *SystemState, width int, systemHistory []utils.TimeMap) string {
	chartWidth := width - 20

	cpuChartSection := renderCPUChart(system, chartWidth, systemHistory)
	memoryChartSection := renderMemoryChart(system, chartWidth, systemHistory)

	chartsRow := lipgloss.JoinVertical(lipgloss.Top, "", cpuChartSection, "", memoryChartSection)

	return chartsRow + "\n"
}

// renderCPUChart renders the CPU usage chart
func renderCPUChart(system *SystemState, width int, systemHistory []utils.TimeMap) string {
	valuesText := fmt.Sprintf("Process: %.1f%% | System: %.1f%%",
		system.ProcessCpuLoad*100,
		system.SystemCpuLoad*100)

	if system.SystemLoad > 0 {
		valuesText += fmt.Sprintf(" | Load: %.2f", system.SystemLoad)
	}

	var chartView string

	graphWidth := max(width-10, 30)
	graphHeight := 8

	chart := utils.NewChart(graphWidth, graphHeight)

	// Add process CPU data
	for _, point := range systemHistory {
		processCpu := point.GetOrDefault("process_cpu", 0)
		chart.Push(utils.TimePoint{
			Time:  point.Timestamp,
			Value: processCpu * 100, // Convert to percentage
		})
	}

	// Set process CPU style (blue/info)
	chart.SetStyle(lipgloss.NewStyle().Foreground(utils.InfoColor))

	// Add system CPU data as second dataset
	for _, point := range systemHistory {
		systemCpu := point.GetOrDefault("system_cpu", 0)
		chart.PushDataSet("system", utils.TimePoint{
			Time:  point.Timestamp,
			Value: systemCpu * 100, // Convert to percentage
		})
	}
	chart.SetDataSetStyle("system", lipgloss.NewStyle().Foreground(utils.WarningColor))

	chart.DrawBrailleAll()

	// Create legend
	processLegend := lipgloss.NewStyle().Foreground(utils.InfoColor).Render("■ Process CPU")
	systemLegend := lipgloss.NewStyle().Foreground(utils.WarningColor).Render("■ System CPU")
	legend := lipgloss.JoinHorizontal(lipgloss.Left, processLegend, "  ", systemLegend)

	chartView = lipgloss.JoinHorizontal(lipgloss.Left, chart.View(), "", legend)

	section := lipgloss.JoinVertical(lipgloss.Left,
		utils.InfoStyle.Render("CPU Usage"),
		chartView,
		utils.MutedStyle.Render(valuesText),
		"", // Empty line for spacing
	)

	return section
}

// renderMemoryChart renders the RAM and Swap usage chart
func renderMemoryChart(system *SystemState, width int, systemHistory []utils.TimeMap) string {
	valuesText := fmt.Sprintf("RAM: %s (%.1f%%) | Swap: %s (%.1f%%)",
		utils.MemorySize(system.UsedSystemMemory),
		system.SystemMemoryPercent*100,
		utils.MemorySize(system.UsedSwap),
		system.SwapPercent*100)

	var chartView string

	graphWidth := max(width-10, 30)
	graphHeight := 8

	chart := utils.NewChart(graphWidth, graphHeight)

	// Add RAM usage data
	for _, point := range systemHistory {
		ramUsage := point.GetOrDefault("ram", 0)
		chart.Push(utils.TimePoint{
			Time:  point.Timestamp,
			Value: ramUsage, // Already in GB
		})
	}

	// Set RAM style (green)
	chart.SetStyle(lipgloss.NewStyle().Foreground(utils.GoodColor))

	// Add Swap usage data as second dataset
	for _, point := range systemHistory {
		swapUsage := point.GetOrDefault("swap", 0)
		chart.PushDataSet("swap", utils.TimePoint{
			Time:  point.Timestamp,
			Value: swapUsage, // Already in GB
		})
	}
	chart.SetDataSetStyle("swap", lipgloss.NewStyle().Foreground(utils.CriticalColor))

	chart.DrawBrailleAll()

	// Create legend
	ramLegend := lipgloss.NewStyle().Foreground(utils.GoodColor).Render("■ RAM (GB)")
	swapLegend := lipgloss.NewStyle().Foreground(utils.CriticalColor).Render("■ Swap (GB)")
	legend := lipgloss.JoinHorizontal(lipgloss.Left, ramLegend, "  ", swapLegend)

	chartView = lipgloss.JoinHorizontal(lipgloss.Left, chart.View(), "", legend)

	section := lipgloss.JoinVertical(lipgloss.Left,
		utils.InfoStyle.Render("System Memory"),
		chartView,
		utils.MutedStyle.Render(valuesText),
		"", // Empty line for spacing
	)

	return section
}

// renderJVMInfo shows JVM version and runtime information
func renderJVMInfo(system *SystemState) string {
	var jvmLines []string

	if system.JVMVersion != "" {
		jvmLines = append(jvmLines, fmt.Sprintf("Version: %s", system.JVMVersion))
	}

	if system.JVMVendor != "" {
		jvmLines = append(jvmLines, fmt.Sprintf("Vendor: %s", system.JVMVendor))
	}

	if !system.JVMStartTime.IsZero() {
		startTimeStr := system.JVMStartTime.Format("2006-01-02 15:04:05")
		jvmLines = append(jvmLines, fmt.Sprintf("Started: %s", startTimeStr))
	}

	if system.JVMUptime > 0 {
		jvmLines = append(jvmLines, fmt.Sprintf("Uptime: %s", utils.FormatDuration(system.JVMUptime)))
	}

	if system.AvailableProcessors > 0 {
		jvmLines = append(jvmLines, fmt.Sprintf("Available CPUs: %d", system.AvailableProcessors))
	}

	if len(jvmLines) == 0 {
		return ""
	}

	jvmText := "• " + jvmLines[0]
	for _, line := range jvmLines[1:] {
		jvmText += "\n• " + line
	}

	section := lipgloss.JoinVertical(lipgloss.Left,
		utils.InfoStyle.Render("JVM Information"),
		utils.MutedStyle.Render(jvmText),
		"", // Empty line for spacing
	)

	return section
}
