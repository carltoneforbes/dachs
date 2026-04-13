package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type appMode int

const (
	modeEdit appMode = iota
	modePreview
	modeHelp
	modeConfirmQuit
	modeGoToFile
)

type model struct {
	textarea textarea.Model
	preview  viewport.Model
	sidebar  sidebar

	mode           appMode
	filePath       string
	modified       bool
	lastSaved      string
	startInPreview bool
	pathInput       textinput.Model
	fileMatches     []string
	matchIsDir      []bool   // parallel to fileMatches: true if entry is a directory
	cachedResults   []string // fallback: broad results from mdfind
	cachedQuery     string
	matchSelected   int
	lastQuery       string
	fileIndex       *FileIndex
	indexReady      bool
	browsing        bool     // true when drilling into a directory from search
	browseDir       string   // current directory in browse mode
	width          int
	height         int
	statusMsg      string
}

type clearStatusMsg struct{}

func clearStatusAfter(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return clearStatusMsg{}
	})
}

func newModel(filePath string) (model, error) {
	ta := textarea.New()
	ta.Placeholder = ""
	ta.Focus()
	ta.CharLimit = 0
	ta.ShowLineNumbers = true
	ta.EndOfBufferCharacter = ' '

	sb := newSidebar()

	pi := textinput.New()
	pi.Placeholder = "Enter file path..."
	pi.CharLimit = 500
	pi.SetWidth(60)

	m := model{
		textarea:  ta,
		sidebar:   sb,
		pathInput: pi,
		mode:      modeEdit,
	}

	if filePath != "" {
		absPath, err := filepath.Abs(filePath)
		if err != nil {
			return m, err
		}
		m.filePath = absPath
		m.sidebar.dir = filepath.Dir(absPath)
		m.sidebar.state.addHistory(absPath)
		saveState(m.sidebar.state)

		content, err := os.ReadFile(absPath)
		if err != nil {
			if os.IsNotExist(err) {
				m.lastSaved = ""
			} else {
				return m, err
			}
		} else {
			s := strings.TrimRight(string(content), "\n")
			m.textarea.SetValue(s)
			m.textarea.MoveToBegin()
			m.lastSaved = m.textarea.Value()
		}
	} else {
		// No file arg — start with sidebar open in file nav
		m.sidebar.visible = true
		m.sidebar.focused = true
		m.sidebar.mode = sidebarFiles
	}

	return m, nil
}

func (m model) Init() tea.Cmd {
	cmds := []tea.Cmd{textarea.Blink, buildFileIndex(), refreshFileIndex()}
	if m.filePath != "" {
		cmds = append(cmds, watchFile(m.filePath))
	}
	return tea.Batch(cmds...)
}

// mainWidth returns the width available for the main content area.
func (m model) mainWidth() int {
	if m.sidebar.visible {
		return m.width - m.sidebar.width - 1 // -1 for separator
	}
	return m.width
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		sw := sidebarWidth(msg.Width)
		m.sidebar.resize(sw, msg.Height-1)

		mw := m.mainWidth()
		m.textarea.SetWidth(mw)
		m.textarea.SetHeight(msg.Height - 1)

		// Initialize sidebar file list on first resize if starting with sidebar open
		if m.sidebar.visible && !m.sidebar.listInit {
			m.sidebar.initFileList()
		}

		// Start in preview mode if flag was set
		if m.startInPreview && m.filePath != "" {
			m.startInPreview = false
			m.mode = modePreview
			m.preview = newPreviewViewport(mw, msg.Height)
			rendered := renderMarkdownPreview(m.textarea.Value(), mw)
			m.preview.SetContent(rendered)
		}

		if m.mode == modePreview {
			m.preview.SetWidth(mw)
			m.preview.SetHeight(msg.Height - 2)
		}
		return m, nil

	case clearStatusMsg:
		m.statusMsg = ""
		return m, nil

	case fileIndexReadyMsg:
		m.fileIndex = msg.index
		m.indexReady = true
		// If user is mid-search, re-search with the new index
		if m.mode == modeGoToFile && len(m.pathInput.Value()) >= 2 {
			m.applySearchResults(searchFileIndex(m.fileIndex, m.pathInput.Value(), m.sidebar.state.History))
			m.matchSelected = 0
		}
		return m, nil

	case fileIndexRefreshMsg:
		// Rebuild index in background; old index stays active during rebuild
		return m, tea.Batch(buildFileIndex(), refreshFileIndex())

	case debounceSearchMsg:
		// Fallback path: only used when index is not ready
		if !m.indexReady && msg.query == m.pathInput.Value() && len(msg.query) >= 2 {
			return m, searchFiles(msg.query)
		}
		return m, nil

	case fileSearchResultMsg:
		// Fallback path: only used when index is not ready
		if !m.indexReady {
			m.cachedResults = msg.matches
			m.cachedQuery = msg.query
			current := m.pathInput.Value()
			m.fileMatches = filterAndRankMatches(m.cachedResults, current, m.sidebar.state.History)
			m.matchSelected = 0
		}
		return m, nil

	case fileChangedMsg:
		return handleFileChanged(m, msg)

	case fileWatchErrMsg:
		return m, nil

	case openFileMsg:
		m, cmd := openFile(m, msg.path)
		// Record in history
		m.sidebar.state.addHistory(msg.path)
		saveState(m.sidebar.state)
		// Update outline if sidebar is showing it
		if m.sidebar.mode == sidebarOutline {
			m.sidebar.updateHeadings(m.textarea.Value())
		}
		return m, cmd

	case jumpToLineMsg:
		// Jump the editor cursor to the specified line
		current := m.textarea.Line()
		target := msg.line
		if target > current {
			for i := 0; i < target-current; i++ {
				m.textarea.CursorDown()
			}
		} else {
			for i := 0; i < current-target; i++ {
				m.textarea.CursorUp()
			}
		}
		m.sidebar.focused = false
		m.textarea.Focus()
		return m, nil

	case tea.KeyMsg:
		// Confirm quit — always handle first
		if m.mode == modeConfirmQuit {
			switch msg.String() {
			case "y", "Y":
				return m, tea.Quit
			case "n", "N", "esc":
				m.mode = modeEdit
				m.statusMsg = ""
				return m, nil
			}
			return m, nil
		}

		// Help — any key returns
		if m.mode == modeHelp {
			m.mode = modeEdit
			return m, nil
		}

		// Go-to-file mode
		if m.mode == modeGoToFile {
			switch msg.String() {
			case "enter":
				if len(m.fileMatches) > 0 && m.matchSelected < len(m.fileMatches) {
					path := m.fileMatches[m.matchSelected]
					isDir := m.matchSelected < len(m.matchIsDir) && m.matchIsDir[m.matchSelected]
					if isDir {
						// Enter directory — switch to browse mode
						m.loadBrowseDir(path)
						return m, nil
					}
					// Open file
					m.mode = modeEdit
					m.pathInput.SetValue("")
					m.fileMatches = nil
					m.matchIsDir = nil
					m.browsing = false
					m.textarea.Focus()
					return m, func() tea.Msg {
						return openFileMsg{path: path}
					}
				} else if path := m.pathInput.Value(); path != "" {
					path = expandPath(strings.TrimSpace(path))
					absPath, _ := filepath.Abs(path)
					m.mode = modeEdit
					m.pathInput.SetValue("")
					m.fileMatches = nil
					m.matchIsDir = nil
					m.browsing = false
					m.textarea.Focus()
					return m, func() tea.Msg {
						return openFileMsg{path: absPath}
					}
				}
				return m, nil

			case "right":
				// Drill into selected directory
				if len(m.fileMatches) > 0 && m.matchSelected < len(m.matchIsDir) && m.matchIsDir[m.matchSelected] {
					m.loadBrowseDir(m.fileMatches[m.matchSelected])
				}
				return m, nil

			case "left":
				// Go up to parent directory (in browse mode)
				if m.browsing {
					parent := filepath.Dir(m.browseDir)
					if parent != m.browseDir {
						m.loadBrowseDir(parent)
					}
				}
				return m, nil

			case "up":
				if m.matchSelected > 0 {
					m.matchSelected--
				}
				return m, nil
			case "down":
				if m.matchSelected < len(m.fileMatches)-1 {
					m.matchSelected++
				}
				return m, nil
			case "esc":
				if m.browsing {
					// Exit browse mode back to search
					m.browsing = false
					m.fileMatches = nil
					m.matchIsDir = nil
					m.pathInput.SetValue("")
					m.lastQuery = ""
					return m, nil
				}
				m.mode = modeEdit
				m.pathInput.SetValue("")
				m.fileMatches = nil
				m.matchIsDir = nil
				m.browsing = false
				m.textarea.Focus()
				return m, nil
			}

			// Pass to text input
			var cmd tea.Cmd
			m.pathInput, cmd = m.pathInput.Update(msg)

			// Update search when input changes
			query := m.pathInput.Value()
			if query != m.lastQuery {
				m.lastQuery = query

				if m.browsing {
					// In browse mode: filter current directory contents
					m.filterBrowseDir(query)
				} else if len(query) < 2 {
					m.fileMatches = nil
					m.matchIsDir = nil
				} else if m.indexReady {
					m.applySearchResults(searchFileIndex(m.fileIndex, query, m.sidebar.state.History))
				} else if m.cachedQuery != "" && strings.HasPrefix(strings.ToLower(query), strings.ToLower(m.cachedQuery)) {
					m.fileMatches = filterAndRankMatches(m.cachedResults, query, m.sidebar.state.History)
					m.matchIsDir = nil
					m.matchSelected = 0
				} else {
					cmd = tea.Batch(cmd, debounceSearch(query))
				}
			}
			return m, cmd
		}

		// Global shortcuts (work regardless of focus)
		switch msg.String() {
		case "ctrl+s":
			return saveFile(m)
		case "ctrl+q", "ctrl+c":
			if m.modified {
				m.mode = modeConfirmQuit
				m.statusMsg = "Unsaved changes! Quit anyway? (y/n)"
				return m, nil
			}
			return m, tea.Quit
		case "ctrl+g":
			m.mode = modeHelp
			return m, nil
		case "ctrl+l":
			m.mode = modeGoToFile
			m.pathInput.SetValue("")
			m.fileMatches = nil
			m.cachedResults = nil
			m.cachedQuery = ""
			m.lastQuery = ""
			pw := m.width*3/5 - 6
			if pw < 34 {
				pw = 34
			}
			m.pathInput.SetWidth(pw)
			m.pathInput.Focus()
			m.textarea.Blur()
			return m, nil
		case "ctrl+o":
			return m.toggleSidebar(sidebarFiles)
		case "ctrl+t":
			return m.toggleSidebar(sidebarOutline)
		case "ctrl+f":
			return m.toggleSidebar(sidebarFavorites)
		case "ctrl+d":
			// Toggle favorite on current file
			if m.filePath != "" {
				added := m.sidebar.state.toggleFavorite(m.filePath)
				saveState(m.sidebar.state)
				if added {
					m.statusMsg = "Added to favorites"
				} else {
					m.statusMsg = "Removed from favorites"
				}
				return m, clearStatusAfter(2 * time.Second)
			}
			return m, nil
		case "ctrl+y":
			return m.toggleSidebar(sidebarHistory)
		case "tab":
			if m.sidebar.visible {
				m.sidebar.focused = !m.sidebar.focused
				if m.sidebar.focused {
					m.textarea.Blur()
				} else {
					m.textarea.Focus()
				}
				return m, nil
			}
		case "ctrl+p":
			if m.mode == modePreview {
				m.mode = modeEdit
				mw := m.mainWidth()
				m.textarea.SetWidth(mw)
				m.textarea.SetHeight(m.height - 1)
				m.textarea.Focus()
			} else {
				m.mode = modePreview
				mw := m.mainWidth()
				m.preview = newPreviewViewport(mw, m.height)
				rendered := renderMarkdownPreview(m.textarea.Value(), mw)
				m.preview.SetContent(rendered)
			}
			return m, nil
		case "ctrl+r":
			return reloadFile(m)
		}

		// Route to sidebar if focused
		if m.sidebar.visible && m.sidebar.focused {
			cmd, handled := m.sidebar.Update(msg)
			if handled {
				return m, cmd
			}
		}

		// Route to main content
		if m.mode == modePreview {
			return updatePreview(m, msg)
		}
	}

	// Pass everything else to textarea (editor mode, main focused)
	if m.mode == modeEdit && !m.sidebar.focused {
		var cmd tea.Cmd
		m.textarea, cmd = m.textarea.Update(msg)
		m.modified = m.textarea.Value() != m.lastSaved
		return m, cmd
	}

	return m, nil
}

func (m *model) applySearchResults(results []SearchResult) {
	m.fileMatches = make([]string, len(results))
	m.matchIsDir = make([]bool, len(results))
	for i, r := range results {
		m.fileMatches[i] = r.Path
		m.matchIsDir[i] = r.IsDir
	}
	m.matchSelected = 0
}

// naturalLess sorts case-insensitively with letters before symbols/numbers.
func naturalLess(a, b string) bool {
	return strings.ToLower(a) < strings.ToLower(b)
}

func (m *model) loadBrowseDir(dir string) {
	m.browsing = true
	m.browseDir = dir
	m.pathInput.SetValue("")
	m.lastQuery = ""

	entries, err := os.ReadDir(dir)
	if err != nil {
		m.fileMatches = nil
		m.matchIsDir = nil
		return
	}

	// Separate dirs and files, excluding hidden
	type entry struct {
		name  string
		path  string
		isDir bool
	}
	var dirs, files []entry
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if e.IsDir() {
			dirs = append(dirs, entry{e.Name(), filepath.Join(dir, e.Name()), true})
		} else {
			files = append(files, entry{e.Name(), filepath.Join(dir, e.Name()), false})
		}
	}

	sort.Slice(dirs, func(i, j int) bool { return naturalLess(dirs[i].name, dirs[j].name) })
	sort.Slice(files, func(i, j int) bool { return naturalLess(files[i].name, files[j].name) })

	var paths []string
	var isDir []bool

	// Parent directory
	parent := filepath.Dir(dir)
	if parent != dir {
		paths = append(paths, parent)
		isDir = append(isDir, true)
	}

	for _, d := range dirs {
		paths = append(paths, d.path)
		isDir = append(isDir, true)
	}
	for _, f := range files {
		paths = append(paths, f.path)
		isDir = append(isDir, false)
	}

	m.fileMatches = paths
	m.matchIsDir = isDir
	m.matchSelected = 0
}

func (m *model) filterBrowseDir(query string) {
	entries, err := os.ReadDir(m.browseDir)
	if err != nil {
		return
	}

	lower := strings.ToLower(query)

	type entry struct {
		name string
		path string
	}
	var dirs, files []entry

	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if !strings.Contains(strings.ToLower(e.Name()), lower) {
			continue
		}
		if e.IsDir() {
			dirs = append(dirs, entry{e.Name(), filepath.Join(m.browseDir, e.Name())})
		} else {
			files = append(files, entry{e.Name(), filepath.Join(m.browseDir, e.Name())})
		}
	}

	sort.Slice(dirs, func(i, j int) bool { return naturalLess(dirs[i].name, dirs[j].name) })
	sort.Slice(files, func(i, j int) bool { return naturalLess(files[i].name, files[j].name) })

	var paths []string
	var isDir []bool
	for _, d := range dirs {
		paths = append(paths, d.path)
		isDir = append(isDir, true)
	}
	for _, f := range files {
		paths = append(paths, f.path)
		isDir = append(isDir, false)
	}

	m.fileMatches = paths
	m.matchIsDir = isDir
	m.matchSelected = 0
}

func (m model) toggleSidebar(mode sidebarMode) (tea.Model, tea.Cmd) {
	if m.sidebar.visible && m.sidebar.mode == mode {
		// Same mode — hide sidebar
		m.sidebar.visible = false
		m.sidebar.focused = false
		m.textarea.Focus()
	} else {
		// Show sidebar in requested mode
		m.sidebar.visible = true
		m.sidebar.focused = true
		m.sidebar.mode = mode
		m.textarea.Blur()

		if mode == sidebarFiles && !m.sidebar.listInit {
			m.sidebar.initFileList()
		}
		if mode == sidebarOutline {
			m.sidebar.updateHeadings(m.textarea.Value())
		}
	}

	// Resize main content for new layout
	mw := m.mainWidth()
	m.textarea.SetWidth(mw)
	m.textarea.SetHeight(m.height - 1)
	if m.mode == modePreview {
		m.preview.SetWidth(mw)
		m.preview.SetHeight(m.height - 2)
	}

	return m, nil
}

var separatorStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#4E4E4E"))

var goToFileBarStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#A6E22E")).
	Background(lipgloss.Color("#353533"))

func (m model) View() tea.View {
	var content string
	if m.width == 0 {
		content = "Loading..."
	} else if m.mode == modeHelp {
		content = renderHelp(m.width, m.height)
	} else {
		// Render main content
		var main string
		mw := m.mainWidth()

		switch m.mode {
		case modePreview:
			title := previewTitleStyle.Width(mw).Render("Preview: " + filepath.Base(m.filePath))
			main = title + "\n" + m.preview.View()
		default:
			main = m.textarea.View()
		}

		// Combine with sidebar if visible
		if m.sidebar.visible {
			sb := m.sidebar.View()
			sep := renderSeparator(m.height - 1)
			content = joinPanes(sb, sep, main, m.sidebar.width, mw, m.height-1)
		} else {
			content = main
		}

		content += "\n" + m.renderStatusBar()

		// Overlay the go-to-file popup on top of content
		if m.mode == modeGoToFile {
			content = m.overlayGoToFile(content)
		}
	}

	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

var (
	popupBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#A6E22E")).
				Background(lipgloss.Color("#1E1E1E")).
				Padding(0, 1)

	popupTitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A6E22E")).
			Bold(true)

	popupMatchStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#C1C6B2"))

	popupSelectedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#1E1E1E")).
				Background(lipgloss.Color("#A6E22E"))

	popupDimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666"))
)

func (m model) overlayGoToFile(base string) string {
	// Build popup content
	popupWidth := m.width * 3 / 5
	if popupWidth < 40 {
		popupWidth = 40
	}
	if popupWidth > m.width-4 {
		popupWidth = m.width - 4
	}
	innerWidth := popupWidth - 4 // account for border + padding

	var lines []string

	if m.browsing {
		// Browse mode header
		lines = append(lines, popupTitleStyle.Render("Browse: "+shortenPath(m.browseDir)))
		if m.pathInput.Value() != "" {
			lines = append(lines, " Filter: "+m.pathInput.View())
		} else {
			lines = append(lines, popupDimStyle.Render(" ← parent  → enter dir  type to filter"))
		}
	} else {
		lines = append(lines, popupTitleStyle.Render("Open File"))
		lines = append(lines, m.pathInput.View())
	}
	lines = append(lines, popupDimStyle.Render(strings.Repeat("─", innerWidth)))

	if len(m.fileMatches) > 0 {
		// In browse mode, show all results with scrolling viewport
		// In search mode, results are already capped at maxFileResults
		maxVisible := m.height - 8 // leave room for popup chrome
		if maxVisible < 5 {
			maxVisible = 5
		}
		if !m.browsing && maxVisible > maxFileResults {
			maxVisible = maxFileResults
		}

		// Calculate scroll offset to keep selected item visible
		scrollOffset := 0
		if m.matchSelected >= maxVisible {
			scrollOffset = m.matchSelected - maxVisible + 1
		}

		end := scrollOffset + maxVisible
		if end > len(m.fileMatches) {
			end = len(m.fileMatches)
		}

		// Scroll indicator
		if scrollOffset > 0 {
			lines = append(lines, popupDimStyle.Render(fmt.Sprintf(" ↑ %d more", scrollOffset)))
		}

		for i := scrollOffset; i < end; i++ {
			match := m.fileMatches[i]
			isDir := i < len(m.matchIsDir) && m.matchIsDir[i]
			display := filepath.Base(match)
			if display == ".." || match == filepath.Dir(m.browseDir) {
				display = ".."
			}
			if isDir && display != ".." {
				display += "/"
			}

			// In search mode, show path context dimmed
			if !m.browsing {
				dir := shortenPath(filepath.Dir(match))
				if len(dir)+len(display)+4 <= innerWidth {
					display = display + "  " + popupDimStyle.Render(dir)
				}
			}

			if i == m.matchSelected {
				lines = append(lines, popupSelectedStyle.Width(innerWidth).Render(" "+display))
			} else if isDir {
				lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("#66D9EF")).Render(" "+display))
			} else {
				lines = append(lines, popupMatchStyle.Render(" "+display))
			}
		}

		// Scroll indicator
		if end < len(m.fileMatches) {
			lines = append(lines, popupDimStyle.Render(fmt.Sprintf(" ↓ %d more", len(m.fileMatches)-end)))
		}
	} else if m.browsing {
		lines = append(lines, popupDimStyle.Render(" Empty directory"))
	} else if len(m.pathInput.Value()) >= 2 {
		lines = append(lines, popupDimStyle.Render(" Searching..."))
	} else {
		lines = append(lines, popupDimStyle.Render(" Type to search  →/Enter to browse"))
	}

	popup := popupBorderStyle.Width(innerWidth).Render(strings.Join(lines, "\n"))

	// Overlay popup onto the base content
	baseLines := strings.Split(base, "\n")
	popupLines := strings.Split(popup, "\n")

	// Position: centered horizontally, near top (row 2)
	startRow := 2
	leftPad := (m.width - lipgloss.Width(popupLines[0])) / 2
	if leftPad < 0 {
		leftPad = 0
	}

	for i, pLine := range popupLines {
		row := startRow + i
		if row >= len(baseLines) {
			break
		}
		// Replace the portion of the base line with the popup line
		baseLine := baseLines[row]
		padded := strings.Repeat(" ", leftPad) + pLine

		// Ensure we fill the full width
		baseVisible := visibleLen(baseLine)
		paddedVisible := visibleLen(padded)
		if paddedVisible < baseVisible {
			padded += strings.Repeat(" ", baseVisible-paddedVisible)
		}

		baseLines[row] = padded
	}

	return strings.Join(baseLines, "\n")
}


func renderSeparator(height int) string {
	lines := make([]string, height)
	for i := range lines {
		lines[i] = separatorStyle.Render("│")
	}
	return strings.Join(lines, "\n")
}

func joinPanes(left, sep, right string, leftWidth, rightWidth, height int) string {
	leftLines := strings.Split(left, "\n")
	sepLines := strings.Split(sep, "\n")
	rightLines := strings.Split(right, "\n")

	// Pad to height
	for len(leftLines) < height {
		leftLines = append(leftLines, "")
	}
	for len(sepLines) < height {
		sepLines = append(sepLines, "│")
	}
	for len(rightLines) < height {
		rightLines = append(rightLines, "")
	}

	var b strings.Builder
	for i := 0; i < height; i++ {
		l := leftLines[i]
		// Pad left pane to width
		lVisible := visibleLen(l)
		if lVisible < leftWidth {
			l += strings.Repeat(" ", leftWidth-lVisible)
		}

		r := rightLines[i]

		b.WriteString(l)
		if i < len(sepLines) {
			b.WriteString(sepLines[i])
		}
		b.WriteString(r)
		if i < height-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}

// visibleLen returns the visible length of a string, ignoring ANSI escape codes.
func visibleLen(s string) int {
	n := 0
	inEsc := false
	for _, r := range s {
		if inEsc {
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || r == '~' {
				inEsc = false
			}
			continue
		}
		if r == '\x1b' {
			inEsc = true
			continue
		}
		n++
	}
	return n
}
