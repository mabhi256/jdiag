package watch

import (
	"fmt"
	"strings"

	"github.com/mabhi256/jdiag/internal/tui"
)

func (m *Model) applyScrolling(content string, viewportHeight int) string {
	lines := strings.Split(content, "\n")
	totalLines := len(lines)

	// No scrolling needed if content fits
	if totalLines <= viewportHeight {
		return content
	}

	// Get current scroll position for active tab
	scrollPos := m.scrollPositions[m.activeTab]

	// Ensure scroll position is valid
	maxScroll := totalLines - viewportHeight
	if scrollPos > maxScroll {
		scrollPos = maxScroll
		m.scrollPositions[m.activeTab] = scrollPos
	}
	if scrollPos < 0 {
		scrollPos = 0
		m.scrollPositions[m.activeTab] = scrollPos
	}

	// Extract visible lines
	endPos := scrollPos + viewportHeight
	visibleLines := lines[scrollPos:endPos]

	// Add scroll indicator if content is scrolled
	if scrollPos > 0 || endPos < totalLines {
		// Replace last line with scroll indicator
		scrollInfo := fmt.Sprintf("%s (Line %d-%d of %d) %s",
			tui.MutedStyle.Render("▲"),
			scrollPos+1,
			endPos,
			totalLines,
			tui.MutedStyle.Render("▼"))

		if len(visibleLines) > 0 {
			visibleLines[len(visibleLines)-1] = scrollInfo
		}
	}

	return strings.Join(visibleLines, "\n")
}

func (m *Model) scrollUp(lines int) {
	currentPos := m.scrollPositions[m.activeTab]
	newPos := max(currentPos-lines, 0)
	m.scrollPositions[m.activeTab] = newPos
}

func (m *Model) scrollDown(lines int) {
	currentPos := m.scrollPositions[m.activeTab]
	m.scrollPositions[m.activeTab] = currentPos + lines
	// Max scroll validation happens in applyScrolling()
}
