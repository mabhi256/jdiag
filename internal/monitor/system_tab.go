package monitor

import (
	"fmt"
	"time"

	"github.com/mabhi256/jdiag/internal/tui"
	"github.com/mabhi256/jdiag/utils"

	"github.com/charmbracelet/lipgloss"
)

// Render renders the system tab view
func RenderSystemTab(state *TabState, config *Config, width int) string {
	var sections []string

	// System overview
	overviewSection := renderSystemOverview(state.System)
	sections = append(sections, overviewSection)

	// CPU usage section
	cpuSection := renderCPUUsage(state.System, width)
	sections = append(sections, cpuSection)

	// System memory section
	if state.System.TotalSystemMemory > 0 {
		systemMemorySection := renderSystemMemory(state.System, width)
		sections = append(sections, systemMemorySection)
	}

	// JVM information section
	jvmInfoSection := renderJVMInfo(state.System)
	sections = append(sections, jvmInfoSection)

	// Connection information section
	connectionSection := renderConnectionInfo(state.System, config)
	sections = append(sections, connectionSection)

	// Performance trends
	if state.System.CPUTrend != 0 {
		trendsSection := renderSystemTrends(state.System)
		sections = append(sections, trendsSection)
	}

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
		statusColor = tui.CriticalColor
		statusIcon = "🔴"
		statusText = "Critical system load"
	case cpuLoad > 0.80 || systemLoad > 0.85:
		statusColor = tui.WarningColor
		statusIcon = "🟡"
		statusText = "High system load"
	case cpuLoad > 0.60 || systemLoad > 0.70:
		statusColor = tui.InfoColor
		statusIcon = "🟠"
		statusText = "Moderate system load"
	default:
		statusColor = tui.GoodColor
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

// renderCPUUsage shows CPU usage with progress bars
func renderCPUUsage(system *SystemState, width int) string {
	// Process CPU usage
	var processCpuColor lipgloss.Color = tui.GoodColor
	if system.ProcessCpuLoad > 0.7 {
		processCpuColor = tui.WarningColor
	}
	if system.ProcessCpuLoad > 0.9 {
		processCpuColor = tui.CriticalColor
	}

	// System CPU usage
	var systemCpuColor lipgloss.Color = tui.GoodColor
	if system.SystemCpuLoad > 0.8 {
		systemCpuColor = tui.WarningColor
	}
	if system.SystemCpuLoad > 0.95 {
		systemCpuColor = tui.CriticalColor
	}

	// Create progress bars
	barWidth := width/3 - 5
	if barWidth < 15 {
		barWidth = 15
	}

	processBar := tui.CreateProgressBar(system.ProcessCpuLoad, barWidth, processCpuColor)
	systemBar := tui.CreateProgressBar(system.SystemCpuLoad, barWidth, systemCpuColor)

	// Format the section
	titleStyled := tui.InfoStyle.Render("CPU Usage")

	processCpuLine := fmt.Sprintf("Process: %s %.1f%%", processBar, system.ProcessCpuLoad*100)
	systemCpuLine := fmt.Sprintf("System:  %s %.1f%%", systemBar, system.SystemCpuLoad*100)

	// Add CPU trend if available
	trendLine := ""
	if system.CPUTrend != 0 {
		trendIcon := "📈"
		if system.CPUTrend < 0 {
			trendIcon = "📉"
		}
		trendLine = fmt.Sprintf("Trend: %s %.1f%%/min", trendIcon, system.CPUTrend*100)
	}

	lines := []string{processCpuLine, systemCpuLine}
	if trendLine != "" {
		lines = append(lines, trendLine)
	}

	section := lipgloss.JoinVertical(lipgloss.Left,
		titleStyled,
		lines[0],
		lines[1],
	)

	if len(lines) > 2 {
		section = lipgloss.JoinVertical(lipgloss.Left, section, tui.MutedStyle.Render(lines[2]))
	}

	return section + "\n"
}

// renderSystemMemory shows system-level memory usage
func renderSystemMemory(system *SystemState, width int) string {
	memoryUsage := system.SystemMemoryPercent

	var color lipgloss.Color = tui.GoodColor
	if memoryUsage > 0.8 {
		color = tui.WarningColor
	}
	if memoryUsage > 0.95 {
		color = tui.CriticalColor
	}

	// Create progress bar
	barWidth := max(width/2-10, 20)

	progressBar := tui.CreateProgressBar(memoryUsage, barWidth, color)
	percentStr := fmt.Sprintf("%.1f%%", memoryUsage*100)

	progressLine := fmt.Sprintf("%s %s", progressBar, percentStr)
	detailLine := fmt.Sprintf("Used: %s | Free: %s | Total: %s",
		utils.MemorySize(system.UsedSystemMemory),
		utils.MemorySize(system.FreeSystemMemory),
		utils.MemorySize(system.TotalSystemMemory))

	section := lipgloss.JoinVertical(lipgloss.Left,
		tui.InfoStyle.Render("System Memory"),
		progressLine,
		tui.MutedStyle.Render(detailLine),
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
		tui.InfoStyle.Render("JVM Information"),
		tui.MutedStyle.Render(jvmText),
		"", // Empty line for spacing
	)

	return section
}

// renderConnectionInfo shows connection and monitoring statistics
func renderConnectionInfo(system *SystemState, config *Config) string {
	var connectionLines []string

	// Target information
	target := config.String()
	if system.ProcessName != "" {
		target = fmt.Sprintf("%s (%s)", target, system.ProcessName)
	}
	connectionLines = append(connectionLines, fmt.Sprintf("Target: %s", target))

	// Monitoring interval
	interval := config.GetInterval().String()
	connectionLines = append(connectionLines, fmt.Sprintf("Interval: %s", interval))

	// Connection uptime
	if system.ConnectionUptime > 0 {
		connectionLines = append(connectionLines,
			fmt.Sprintf("Monitoring: %s", utils.FormatDuration(system.ConnectionUptime)))
	}

	// Update count
	if system.UpdateCount > 0 {
		connectionLines = append(connectionLines,
			fmt.Sprintf("Updates: %d", system.UpdateCount))
	}

	// Last update time
	if !system.LastUpdateTime.IsZero() {
		lastUpdate := time.Since(system.LastUpdateTime)
		connectionLines = append(connectionLines,
			fmt.Sprintf("Last Update: %s ago", utils.FormatDuration(lastUpdate)))
	}

	connectionText := "• " + connectionLines[0]
	for _, line := range connectionLines[1:] {
		connectionText += "\n• " + line
	}

	section := lipgloss.JoinVertical(lipgloss.Left,
		tui.InfoStyle.Render("Connection Info"),
		tui.MutedStyle.Render(connectionText),
		"", // Empty line for spacing
	)

	return section
}

// renderSystemTrends shows system performance trends
func renderSystemTrends(system *SystemState) string {
	var trendLines []string

	if system.CPUTrend != 0 {
		trendIcon := "📈"
		trendColor := tui.InfoColor

		if system.CPUTrend < 0 {
			trendIcon = "📉"
			trendColor = tui.GoodColor
		} else if system.CPUTrend > 0.1 { // Rising more than 10%/min
			trendColor = tui.WarningColor
		}

		trendLine := lipgloss.NewStyle().Foreground(trendColor).Render(
			fmt.Sprintf("%s CPU Trend: %.1f%%/min", trendIcon, system.CPUTrend*100))
		trendLines = append(trendLines, "• "+trendLine)
	}

	if system.SystemLoad > 0 {
		loadColor := tui.GoodColor
		if system.SystemLoad > float64(system.AvailableProcessors) {
			loadColor = tui.WarningColor
		}
		if system.SystemLoad > float64(system.AvailableProcessors*2) {
			loadColor = tui.CriticalColor
		}

		loadLine := lipgloss.NewStyle().Foreground(loadColor).Render(
			fmt.Sprintf("System Load: %.2f", system.SystemLoad))
		trendLines = append(trendLines, "• "+loadLine)
	}

	if len(trendLines) == 0 {
		return ""
	}

	trendsText := trendLines[0]
	for _, line := range trendLines[1:] {
		trendsText += "\n" + line
	}

	section := lipgloss.JoinVertical(lipgloss.Left,
		tui.InfoStyle.Render("System Trends"),
		trendsText,
		"", // Empty line for spacing
	)

	return section
}
