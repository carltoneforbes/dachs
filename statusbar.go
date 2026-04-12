package main

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

var statusBarStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#C1C6B2")).
	Background(lipgloss.Color("#353533"))

var statusMsgStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#FD971F")).
	Background(lipgloss.Color("#353533"))

func (m model) renderStatusBar() string {
	w := m.width
	if w == 0 {
		w = 80
	}

	// Status message overrides the bar
	if m.statusMsg != "" {
		msg := " " + m.statusMsg
		gap := w - len(msg)
		if gap > 0 {
			msg += strings.Repeat(" ", gap)
		}
		return statusMsgStyle.Width(w).Render(msg)
	}

	// Left: full file path + modified indicator
	name := "untitled"
	if m.filePath != "" {
		name = shortenPath(m.filePath)
	}
	if m.sidebar.state.isFavorite(m.filePath) {
		name += " *"
	}
	if m.modified {
		name += " [+]"
	}

	// Center: help hint
	hint := "Ctrl+G Help"

	// Right: line position
	line := m.textarea.Line() + 1
	total := m.textarea.LineCount()
	pos := fmt.Sprintf("Ln %d/%d", line, total)

	// Build the bar with spacing
	left := " " + name
	right := pos + " "

	// Calculate gaps to spread left, center, right
	totalContent := len(left) + len(hint) + len(right)
	remainingSpace := w - totalContent
	if remainingSpace < 2 {
		// Not enough room for hint — drop it
		gap := w - len(left) - len(right)
		if gap < 1 {
			gap = 1
		}
		bar := left + strings.Repeat(" ", gap) + right
		return statusBarStyle.Width(w).Render(bar)
	}

	gapLeft := remainingSpace / 2
	gapRight := remainingSpace - gapLeft

	bar := left + strings.Repeat(" ", gapLeft) + hint + strings.Repeat(" ", gapRight) + right
	return statusBarStyle.Width(w).Render(bar)
}
