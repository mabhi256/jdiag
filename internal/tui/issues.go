package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mabhi256/jdiag/internal/gc"
)

func RenderIssues(issues *gc.Analysis, filterSev string, selectedIndex int, expandedItems map[int]bool, width, height int) string {
	totalIssues := len(issues.Critical) + len(issues.Warning) + len(issues.Info)
	if totalIssues == 0 {
		return renderNoIssues()
	}

	filteredIssues := getFilteredIssuesFromAnalyzed(issues, filterSev)

	if len(filteredIssues) == 0 {
		return renderNoFilteredIssues(filterSev)
	}

	header := renderIssuesHeader(issues, filterSev)
	content := renderIssuesList(filteredIssues, selectedIndex, expandedItems, width)

	// Apply scrolling logic (same as before)
	contentLines := strings.Split(content, "\n")
	availableHeight := height - 4

	if len(contentLines) > availableHeight {
		selectedStartLine := calculateSelectedStartLine(filteredIssues, selectedIndex, expandedItems)

		scrollY := 0
		if selectedStartLine >= availableHeight {
			scrollY = selectedStartLine - availableHeight/2
		}

		if scrollY+availableHeight > len(contentLines) {
			scrollY = len(contentLines) - availableHeight
		}
		if scrollY < 0 {
			scrollY = 0
		}

		start := scrollY
		end := min(start+availableHeight, len(contentLines))
		content = strings.Join(contentLines[start:end], "\n")
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		"",
		content,
	)
}

func getFilteredIssuesFromAnalyzed(issues *gc.Analysis, filterSev string) []gc.PerformanceIssue {
	switch filterSev {
	case "critical":
		return issues.Critical
	case "warning":
		return issues.Warning
	case "info":
		return issues.Info
	default:
		return []gc.PerformanceIssue{}
	}
}

func renderIssuesHeader(issues *gc.Analysis, filterSev string) string {
	criticalCount := len(issues.Critical)
	warningCount := len(issues.Warning)
	infoCount := len(issues.Info)

	counts := []string{}
	if criticalCount > 0 {
		style := TabInactiveStyle
		if filterSev == "critical" {
			style = TabActiveStyle
		}
		counts = append(counts, style.Render(fmt.Sprintf("🔴 Critical: %d", criticalCount)))
	}

	if warningCount > 0 {
		style := TabInactiveStyle
		if filterSev == "warning" {
			style = TabActiveStyle
		}
		counts = append(counts, style.Render(fmt.Sprintf("⚠️  Warning: %d", warningCount)))
	}

	if infoCount > 0 {
		style := TabInactiveStyle
		if filterSev == "info" {
			style = TabActiveStyle
		}
		counts = append(counts, style.Render(fmt.Sprintf("ℹ️  Info: %d", infoCount)))
	}

	if len(counts) == 0 {
		return GoodStyle.Render("✅ No issues detected!")
	}

	header := strings.Join(counts, "  ")

	filteredCount := len(getFilteredIssuesFromAnalyzed(issues, filterSev))
	header += MutedStyle.Render(fmt.Sprintf("     │ Showing: %d issues", filteredCount))

	return header
}

func getFilteredIssues(issues []gc.PerformanceIssue, filterSev string) []gc.PerformanceIssue {
	if filterSev == "all" {
		return issues
	}

	var filtered []gc.PerformanceIssue
	for _, issue := range issues {
		if strings.ToLower(issue.Severity) == filterSev {
			filtered = append(filtered, issue)
		}
	}
	return filtered
}

func renderNoIssues() string {
	return GoodStyle.Render("✅ No performance issues detected!\n\nYour G1GC configuration appears to be working well.")
}

func renderNoFilteredIssues(filterSev string) string {
	return fmt.Sprintf("No %s issues found.\n\nPress 'a' to show all issues.", filterSev)
}

func renderIssuesList(issues []gc.PerformanceIssue, selectedIndex int, expandedItems map[int]bool, width int) string {
	var lines []string

	for i, issue := range issues {
		isSelected := i == selectedIndex
		isExpanded := expandedItems[i]

		issueLines := renderIssueItem(issue, isSelected, isExpanded, width)
		lines = append(lines, issueLines...)
		lines = append(lines, "") // Spacing between issues
	}

	return strings.Join(lines, "\n")
}

func renderIssueItem(issue gc.PerformanceIssue, isSelected, isExpanded bool, width int) []string {
	var lines []string

	icon := GetSeverityIcon(issue.Severity)
	style := GetSeverityStyle(issue.Severity)

	// Selection indicator
	selector := " "
	if isSelected {
		selector = "▶"
	}

	// Expansion indicator
	expandIcon := "[+]"
	if isExpanded {
		expandIcon = "[-]"
	}

	// Main issue line
	titleLine := fmt.Sprintf("%s %s %s", selector, icon, issue.Type)
	if isSelected {
		titleLine = lipgloss.NewStyle().
			Background(InfoColor).
			Foreground(lipgloss.Color("#FFFFFF")).
			Render(titleLine)
	} else {
		titleLine = style.Render(titleLine)
	}

	lines = append(lines, titleLine)

	// Description line
	descLine := fmt.Sprintf("  ├─ %s", issue.Description)
	if len(descLine) > width-2 {
		descLine = TruncateString(descLine, width-2)
	}
	lines = append(lines, MutedStyle.Render(descLine))

	// Expansion control
	expandLine := fmt.Sprintf("  └─ %s Show Recommendations", expandIcon)
	if isSelected {
		expandLine = InfoStyle.Render(expandLine)
	} else {
		expandLine = MutedStyle.Render(expandLine)
	}
	lines = append(lines, expandLine)

	// Expanded recommendations
	if isExpanded && len(issue.Recommendation) > 0 {
		lines = append(lines, "") // Spacing
		lines = append(lines, InfoStyle.Render("     Recommendations:"))

		for _, rec := range issue.Recommendation {
			bullet := "✓"
			if strings.Contains(strings.ToLower(rec), "critical") ||
				strings.Contains(strings.ToLower(rec), "immediate") {
				bullet = "⚠️"
			}

			// Wrap long recommendations
			wrapped := WrapText(rec, width-8)
			for j, line := range wrapped {
				prefix := fmt.Sprintf("     %s ", bullet)
				if j > 0 {
					prefix = "       " // Indent continuation lines
				}
				lines = append(lines, TextStyle.Render(prefix+line))
			}
		}
	}

	return lines
}

// Calculate the starting line number for the selected issue
func calculateSelectedStartLine(issues []gc.PerformanceIssue, selectedIndex int, expandedItems map[int]bool) int {
	lineCount := 0

	for i := 0; i < selectedIndex && i < len(issues); i++ {
		// Each issue has at least 4 lines (title, description, expand control, spacing)
		lineCount += 4

		// Add lines for expanded recommendations
		if expandedItems[i] && len(issues[i].Recommendation) > 0 {
			lineCount += 2 // Header and spacing
			for _, rec := range issues[i].Recommendation {
				wrapped := WrapText(rec, 72) // Approximate width for calculation
				lineCount += len(wrapped)
			}
		}
	}

	return lineCount
}
