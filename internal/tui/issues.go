package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mabhi256/jdiag/internal/gc"
)

func (m *Model) RenderIssues() string {
	totalIssues := len(m.issues.Critical) + len(m.issues.Warning) + len(m.issues.Info)
	if totalIssues == 0 {
		return renderNoIssues()
	}

	subTab := m.issuesState.selectedSubTab
	subTabIssues := m.GetSubTabIssues()

	header := renderIssuesHeader(m.issues, subTab)
	content := m.renderIssuesList(subTabIssues)

	// Apply scrolling logic (same as before)
	contentLines := strings.Split(content, "\n")
	availableHeight := m.height - 4

	if len(contentLines) > availableHeight {
		selectedStartLine := m.calculateSelectedStartLine(subTabIssues)

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

func renderIssuesHeader(issues *gc.GCIssues, subTab IssuesSubTab) string {
	criticalCount := len(issues.Critical)
	warningCount := len(issues.Warning)
	infoCount := len(issues.Info)

	counts := []string{}
	criticalStyle := TabInactiveStyle
	if subTab == CriticalIssues {
		criticalStyle = TabActiveStyle
	}
	counts = append(counts, criticalStyle.Render(fmt.Sprintf("🔴 Critical: %d", criticalCount)))

	warningStyle := TabInactiveStyle
	if subTab == WarningIssues {
		warningStyle = TabActiveStyle
	}
	counts = append(counts, warningStyle.Render(fmt.Sprintf("⚠️  Warning: %d", warningCount)))

	infoStyle := TabInactiveStyle
	if subTab == InfoIssues {
		infoStyle = TabActiveStyle
	}
	counts = append(counts, infoStyle.Render(fmt.Sprintf("ℹ️  Info: %d", infoCount)))

	header := strings.Join(counts, "  ")

	return header
}

func renderNoIssues() string {
	return GoodStyle.Render("✅ No performance issues detected!\n\nYour G1GC configuration appears to be working well.")
}

func renderNoSubTabIssues(subTab IssuesSubTab) string {
	return fmt.Sprintf("No %s issues found.\n\n", subTab)
}

func (m *Model) renderIssuesList(issues []gc.PerformanceIssue) string {
	var lines []string

	selectedSubTab := m.issuesState.selectedSubTab
	if len(issues) == 0 {
		return renderNoSubTabIssues(selectedSubTab)
	}

	for i, issue := range issues {
		isSelected := i == m.GetSelectedIssue()
		isExpanded := m.IsIssueExpanded(i)

		issueLines := renderIssueItem(issue, isSelected, isExpanded, m.width)
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
	// if len(descLine) > width-2 {
	// 	descLine = TruncateString(descLine, width-2)
	// }
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
func (m *Model) calculateSelectedStartLine(issues []gc.PerformanceIssue) int {
	lineCount := 0
	selectedIssue := m.GetSelectedIssue()

	for i := 0; i < selectedIssue && i < len(issues); i++ {
		// Each issue has at least 4 lines (title, description, expand control, spacing)
		lineCount += 4

		// Add lines for expanded recommendations
		if m.IsIssueExpanded(selectedIssue) && len(issues[i].Recommendation) > 0 {
			lineCount += 2 // Header and spacing
			for _, rec := range issues[i].Recommendation {
				wrapped := WrapText(rec, 72) // Approximate width for calculation
				lineCount += len(wrapped)
			}
		}
	}

	return lineCount
}
