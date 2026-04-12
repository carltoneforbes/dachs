package main

import (
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/glamour"
)

var previewTitleStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color("#FD971F")).
	Background(lipgloss.Color("#353533")).
	Padding(0, 1)

func newPreviewViewport(width, height int) viewport.Model {
	vp := viewport.New(
		viewport.WithWidth(width),
		viewport.WithHeight(height-2), // Reserve for title + status
	)
	return vp
}

func renderMarkdownPreview(content string, width int) string {
	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(width-4),
	)
	if err != nil {
		return content
	}

	rendered, err := renderer.Render(content)
	if err != nil {
		return content
	}

	return rendered
}

func updatePreview(m model, msg tea.KeyMsg) (model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+p", "esc", "q":
		m.mode = modeEdit
		return m, nil
	}

	// Pass to viewport for scrolling
	var cmd tea.Cmd
	m.preview, cmd = m.preview.Update(msg)
	return m, cmd
}
