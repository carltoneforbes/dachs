package main

import (
	"strings"

	"charm.land/lipgloss/v2"
)

var (
	helpTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FD971F")).
			MarginBottom(1)

	helpKeyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A6E22E")).
			Width(18)

	helpDescStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#C1C6B2"))

	helpFooterStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#75715E")).
			MarginTop(1)
)

type shortcut struct {
	key  string
	desc string
}

var shortcuts = []shortcut{
	{"Ctrl+S", "Save file"},
	{"Ctrl+R", "Reload file from disk"},
	{"Ctrl+O", "File navigator sidebar"},
	{"Ctrl+T", "Heading outline sidebar"},
	{"Ctrl+F", "Favorites sidebar"},
	{"Ctrl+Y", "History sidebar"},
	{"Ctrl+D", "Toggle favorite on current file"},
	{"Ctrl+L", "Open file by path"},
	{"Ctrl+P", "Toggle markdown preview"},
	{"Tab", "Switch focus: sidebar / editor"},
	{"Ctrl+Z", "Undo"},
	{"Ctrl+Y", "Redo"},
	{"Ctrl+G", "Toggle this help screen"},
	{"Ctrl+Q", "Quit (prompts if unsaved)"},
	{"", ""},
	{"Arrows", "Move cursor"},
	{"Alt+Left/Right", "Move by word"},
	{"Home / End", "Start / end of line"},
	{"PgUp / PgDn", "Scroll viewport"},
	{"", ""},
	{"--- Sidebar ---", ""},
	{"[ / ]", "Cycle sidebar tabs"},
	{"Enter", "Open file / jump to heading"},
	{"Backspace", "Go up one directory"},
	{"Ctrl+H", "Toggle hidden files"},
	{"j / k", "Navigate items"},
	{"Esc", "Close sidebar"},
}

func renderHelp(width, height int) string {
	var b strings.Builder

	b.WriteString(helpTitleStyle.Render("Dachs - Keyboard Shortcuts"))
	b.WriteString("\n")
	b.WriteString(helpFooterStyle.Render("  Usage: dachs [--preview|-p] [file.md]"))
	b.WriteString("\n\n")

	for _, s := range shortcuts {
		if s.key == "" {
			b.WriteString("\n")
			continue
		}
		line := helpKeyStyle.Render(s.key) + helpDescStyle.Render(s.desc)
		b.WriteString(line + "\n")
	}

	b.WriteString("\n")
	b.WriteString(helpFooterStyle.Render("Press any key to return to editor"))

	// Center vertically
	content := b.String()
	contentLines := strings.Count(content, "\n") + 1
	topPad := (height - contentLines) / 3
	if topPad < 1 {
		topPad = 1
	}

	// Center horizontally with left padding
	leftPad := (width - 40) / 3
	if leftPad < 2 {
		leftPad = 2
	}

	padded := strings.Repeat("\n", topPad)
	for _, line := range strings.Split(content, "\n") {
		padded += strings.Repeat(" ", leftPad) + line + "\n"
	}

	return padded
}
