package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type sidebarMode int

const (
	sidebarFiles sidebarMode = iota
	sidebarOutline
	sidebarFavorites
	sidebarHistory
)

type sidebar struct {
	mode     sidebarMode
	visible  bool
	focused  bool
	width    int
	height   int

	// File navigator
	fileList    list.Model
	dir         string
	listInit    bool
	showHidden  bool

	// Outline
	headings []heading
	selected int
	scroll   int

	// Favorites & History
	state       dachsState
	favSelected int
	favScroll   int
	hisSelected int
	hisScroll   int
}

// Styles
var (
	sidebarBorderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#4E4E4E"))

	sidebarTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FD971F")).
				Background(lipgloss.Color("#2D2D2D")).
				Padding(0, 1)

	sidebarItemStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#C1C6B2"))

	sidebarSelectedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#282828")).
				Background(lipgloss.Color("#A6E22E"))

	sidebarDirStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#66D9EF"))

	sidebarH1Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#66D9EF"))
	sidebarH2Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#A6E22E"))
	sidebarH3Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#E6DB74"))
	sidebarH4Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#FD971F"))
	sidebarH5Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#AE81FF"))
	sidebarH6Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#F92672"))
)

func headingStyle(level int) lipgloss.Style {
	switch level {
	case 1:
		return sidebarH1Style
	case 2:
		return sidebarH2Style
	case 3:
		return sidebarH3Style
	case 4:
		return sidebarH4Style
	case 5:
		return sidebarH5Style
	default:
		return sidebarH6Style
	}
}

func newSidebar() sidebar {
	return sidebar{
		mode:  sidebarFiles,
		dir:   defaultRoot(),
		state: loadState(),
	}
}

func sidebarWidth(termWidth int) int {
	w := termWidth / 4
	if w < 20 {
		w = 20
	}
	if w > 40 {
		w = 40
	}
	return w
}

// initFileList creates the file list for the current directory.
func (s *sidebar) initFileList() {
	items := sidebarReadDir(s.dir, s.showHidden)

	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = false
	delegate.SetHeight(1)
	delegate.SetSpacing(0)

	l := list.New(items, delegate, s.width, s.height-2)
	l.Title = shortenPath(s.dir)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)

	s.fileList = l
	s.listInit = true
}

func sidebarReadDir(dir string, showHidden bool) []list.Item {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	type fentry struct {
		name  string
		path  string
		isDir bool
	}

	var dirs, files []fentry
	for _, e := range entries {
		if !showHidden && strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if e.IsDir() {
			dirs = append(dirs, fentry{name: e.Name(), path: filepath.Join(dir, e.Name()), isDir: true})
		} else {
			files = append(files, fentry{name: e.Name(), path: filepath.Join(dir, e.Name())})
		}
	}

	sort.Slice(dirs, func(i, j int) bool { return dirs[i].name < dirs[j].name })
	sort.Slice(files, func(i, j int) bool { return files[i].name < files[j].name })

	var items []list.Item
	parent := filepath.Dir(dir)
	if parent != dir {
		items = append(items, fileItem{name: "..", path: parent, isDir: true})
	}
	for _, d := range dirs {
		items = append(items, fileItem{name: d.name + "/", path: d.path, isDir: true})
	}
	for _, f := range files {
		items = append(items, fileItem{name: f.name, path: f.path})
	}

	return items
}

func (s *sidebar) updateHeadings(content string) {
	s.headings = parseHeadings(content)
	if s.selected >= len(s.headings) {
		s.selected = max(0, len(s.headings)-1)
	}
}

func (s *sidebar) resize(width, height int) {
	s.width = width
	s.height = height
	if s.listInit {
		s.fileList.SetSize(width, height-2)
	}
}

var (
	tabActiveStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A6E22E")).
			Bold(true)

	tabInactiveStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#666666"))

	tabSepStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#4E4E4E"))
)

type sidebarTab struct {
	label    string
	shortcut string
	mode     sidebarMode
}

var sidebarTabs = []sidebarTab{
	{"Files", "^O", sidebarFiles},
	{"Outline", "^T", sidebarOutline},
	{"Favs", "^F", sidebarFavorites},
	{"History", "^Y", sidebarHistory},
}

var shortcutStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#555555"))

func (s *sidebar) renderTabs(w int) string {
	var parts []string
	for i, tab := range sidebarTabs {
		if i > 0 {
			parts = append(parts, tabSepStyle.Render("│"))
		}
		hint := shortcutStyle.Render(" " + tab.shortcut)
		if tab.mode == s.mode {
			parts = append(parts, tabActiveStyle.Render(tab.label)+hint)
		} else {
			parts = append(parts, tabInactiveStyle.Render(tab.label)+hint)
		}
	}

	tabLine := strings.Join(parts, "")
	// Pad to width and add underline
	divider := tabSepStyle.Render(strings.Repeat("─", w))
	return " " + tabLine + "\n" + divider + "\n"
}

// View renders the sidebar content.
func (s *sidebar) View() string {
	if !s.visible {
		return ""
	}

	w := s.width
	h := s.height

	tabs := s.renderTabs(w)
	tabLines := strings.Count(tabs, "\n")
	contentHeight := h - tabLines

	var body string
	switch s.mode {
	case sidebarFiles:
		body = s.renderFileNav(w, contentHeight)
	case sidebarOutline:
		body = s.renderOutline(w, contentHeight)
	case sidebarFavorites:
		body = s.renderSimpleList(w, contentHeight, s.state.Favorites, s.favSelected, s.favScroll, "No favorites yet", "Ctrl+D to add current file")
	case sidebarHistory:
		body = s.renderSimpleList(w, contentHeight, s.state.History, s.hisSelected, s.hisScroll, "No history yet", "")
	}

	return tabs + body
}

func (s *sidebar) renderFileNav(w, h int) string {
	if !s.listInit {
		return ""
	}
	// Resize file list to fit below tabs
	s.fileList.SetSize(w, h)
	return s.fileList.View()
}

func (s *sidebar) renderOutline(w, h int) string {
	var b strings.Builder

	if len(s.headings) == 0 {
		b.WriteString("\n")
		b.WriteString(sidebarItemStyle.Render("  No headings found"))
		lines := strings.Count(b.String(), "\n") + 1
		for i := lines; i < h; i++ {
			b.WriteString("\n")
		}
		return b.String()
	}

	// Calculate visible window
	visibleHeight := h - 1
	if s.selected < s.scroll {
		s.scroll = s.selected
	}
	if s.selected >= s.scroll+visibleHeight {
		s.scroll = s.selected - visibleHeight + 1
	}

	for i := s.scroll; i < len(s.headings) && i < s.scroll+visibleHeight; i++ {
		hd := s.headings[i]
		indent := strings.Repeat("  ", hd.level-1)

		// Truncate text to fit
		maxLen := w - len(indent) - 2
		text := hd.text
		if maxLen > 3 && len(text) > maxLen {
			text = text[:maxLen-1] + "…"
		}

		line := fmt.Sprintf("%s%s", indent, text)

		if i == s.selected && s.focused {
			b.WriteString(sidebarSelectedStyle.Width(w).Render(line))
		} else {
			styled := headingStyle(hd.level).Render(line)
			b.WriteString(styled)
		}
		b.WriteString("\n")
	}

	// Pad remaining height
	lines := strings.Count(b.String(), "\n") + 1
	for i := lines; i < h; i++ {
		b.WriteString("\n")
	}

	return b.String()
}

// cycleSidebarTab moves to the next or previous sidebar tab.
func (s *sidebar) cycleSidebarTab(delta int) {
	modes := []sidebarMode{sidebarFiles, sidebarOutline, sidebarFavorites, sidebarHistory}
	current := 0
	for i, m := range modes {
		if m == s.mode {
			current = i
			break
		}
	}
	next := (current + delta + len(modes)) % len(modes)
	s.mode = modes[next]
}

// Update handles key events when sidebar is focused.
func (s *sidebar) Update(msg tea.KeyMsg) (tea.Cmd, bool) {
	// Tab cycling — [ and ] to switch sidebar tabs
	switch msg.String() {
	case "]":
		s.cycleSidebarTab(1)
		return nil, true
	case "[":
		s.cycleSidebarTab(-1)
		return nil, true
	}

	switch s.mode {
	case sidebarFiles:
		return s.updateFiles(msg)
	case sidebarOutline:
		return s.updateOutline(msg)
	case sidebarFavorites:
		return s.updateSimpleList(msg, s.state.Favorites, &s.favSelected, &s.favScroll)
	case sidebarHistory:
		return s.updateSimpleList(msg, s.state.History, &s.hisSelected, &s.hisScroll)
	}
	return nil, false
}

type jumpToLineMsg struct {
	line int
}

func (s *sidebar) updateFiles(msg tea.KeyMsg) (tea.Cmd, bool) {
	switch msg.String() {
	case "enter":
		if item, ok := s.fileList.SelectedItem().(fileItem); ok {
			if item.isDir {
				s.dir = item.path
				s.initFileList()
				return nil, true
			}
			// Open file — return command, keep sidebar open
			return func() tea.Msg {
				return openFileMsg{path: item.path}
			}, true
		}
		return nil, true

	case "backspace":
		if s.fileList.FilterState() == list.Filtering {
			break
		}
		parent := filepath.Dir(s.dir)
		if parent != s.dir {
			s.dir = parent
			s.initFileList()
		}
		return nil, true

	case "ctrl+h":
		// Toggle hidden files
		s.showHidden = !s.showHidden
		s.initFileList()
		return nil, true

	case "esc":
		s.visible = false
		s.focused = false
		return nil, true
	}

	// Pass to list
	var cmd tea.Cmd
	s.fileList, cmd = s.fileList.Update(msg)
	return cmd, true
}

func (s *sidebar) updateOutline(msg tea.KeyMsg) (tea.Cmd, bool) {
	switch msg.String() {
	case "up", "k":
		if s.selected > 0 {
			s.selected--
		}
		return nil, true

	case "down", "j":
		if s.selected < len(s.headings)-1 {
			s.selected++
		}
		return nil, true

	case "enter":
		if s.selected < len(s.headings) {
			line := s.headings[s.selected].line
			return func() tea.Msg {
				return jumpToLineMsg{line: line}
			}, true
		}
		return nil, true

	case "esc":
		s.visible = false
		s.focused = false
		return nil, true
	}

	return nil, false
}

// renderSimpleList renders a scrollable list of file paths (used for favorites and history).
func (s *sidebar) renderSimpleList(w, h int, items []string, selected, scroll int, emptyMsg, hintMsg string) string {
	var b strings.Builder

	if len(items) == 0 {
		b.WriteString("\n")
		b.WriteString(sidebarItemStyle.Render("  " + emptyMsg))
		if hintMsg != "" {
			b.WriteString("\n")
			b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#666666")).Render("  " + hintMsg))
		}
		lines := strings.Count(b.String(), "\n") + 1
		for i := lines; i < h; i++ {
			b.WriteString("\n")
		}
		return b.String()
	}

	visibleHeight := h - 1
	if selected < scroll {
		scroll = selected
	}
	if selected >= scroll+visibleHeight {
		scroll = selected - visibleHeight + 1
	}

	for i := scroll; i < len(items) && i < scroll+visibleHeight; i++ {
		display := filepath.Base(items[i])
		dir := shortenPath(filepath.Dir(items[i]))
		// Show filename and dim directory
		line := " " + display

		if i == selected && s.focused {
			b.WriteString(sidebarSelectedStyle.Width(w).Render(line))
		} else {
			b.WriteString(sidebarItemStyle.Render(line))
		}
		b.WriteString("\n")

		// Show dir path on next line, dimmed
		if i == selected && s.focused {
			dirLine := " " + dir
			if len(dirLine) > w-1 {
				dirLine = " …" + dir[len(dir)-w+4:]
			}
			b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#666666")).Render(dirLine))
			b.WriteString("\n")
		}
	}

	// Pad remaining
	lines := strings.Count(b.String(), "\n") + 1
	for i := lines; i < h; i++ {
		b.WriteString("\n")
	}

	return b.String()
}

// updateSimpleList handles navigation for favorites/history lists.
func (s *sidebar) updateSimpleList(msg tea.KeyMsg, items []string, selected *int, scroll *int) (tea.Cmd, bool) {
	switch msg.String() {
	case "up", "k":
		if *selected > 0 {
			*selected--
		}
		return nil, true

	case "down", "j":
		if *selected < len(items)-1 {
			*selected++
		}
		return nil, true

	case "enter":
		if *selected < len(items) {
			path := items[*selected]
			return func() tea.Msg {
				return openFileMsg{path: path}
			}, true
		}
		return nil, true

	case "esc":
		s.visible = false
		s.focused = false
		return nil, true
	}

	return nil, false
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
