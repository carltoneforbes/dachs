package main

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
)

const maxFileResults = 8
const maxSearchResults = 50

type fileSearchResultMsg struct {
	query   string
	matches []string
}

// debounceSearchMsg is sent after a delay to trigger the actual search.
type debounceSearchMsg struct {
	query string
}

// debounceSearch waits briefly before triggering a search, so rapid keystrokes
// don't fire multiple mdfind/fd processes.
func debounceSearch(query string) tea.Cmd {
	return tea.Tick(150*time.Millisecond, func(t time.Time) tea.Msg {
		return debounceSearchMsg{query: query}
	})
}

// searchFiles finds files matching the query using the best available tool.
func searchFiles(query string) tea.Cmd {
	return func() tea.Msg {
		if len(query) < 2 {
			return fileSearchResultMsg{query: query}
		}

		seen := make(map[string]bool)
		var matches []string

		addResults := func(lines []string) {
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" || seen[line] {
					continue
				}
				if strings.HasPrefix(line, "/System/") || strings.HasPrefix(line, "/Applications/") {
					continue
				}
				if strings.Contains(line, "/Library/") {
					continue
				}
				seen[line] = true
				matches = append(matches, line)
			}
		}

		homeDir := expandPath("~/")

		// fd for hidden files
		if fdPath, err := exec.LookPath("fd"); err == nil {
			fdCmd := exec.Command(fdPath, "--type", "f", "--hidden", "--max-results", "20", "--color", "never",
				"--exclude", "Library", "--exclude", "Movies", "--exclude", "Music",
				"--exclude", "Photos", "--exclude", ".Trash", "--exclude", "node_modules",
				"--exclude", ".git/objects",
				query, homeDir)
			if out, err := fdCmd.Output(); err == nil {
				addResults(strings.Split(strings.TrimSpace(string(out)), "\n"))
			}
		}

		// macOS: Spotlight
		if runtime.GOOS == "darwin" {
			if mdfindPath, err := exec.LookPath("mdfind"); err == nil {
				mdfindCmd := exec.Command(mdfindPath, "-name", query)
				if out, err := mdfindCmd.Output(); err == nil {
					addResults(strings.Split(strings.TrimSpace(string(out)), "\n"))
				}
			}
		}

		// Fallback: find
		if len(matches) == 0 {
			if findPath, err := exec.LookPath("find"); err == nil {
				findCmd := exec.Command(findPath, homeDir,
					"-maxdepth", "5", "-type", "f",
					"-iname", "*"+query+"*",
					"-not", "-path", "*/Library/*",
					"-not", "-path", "*/.git/*",
					"-not", "-path", "*/node_modules/*")
				if out, err := findCmd.Output(); err == nil {
					addResults(strings.Split(strings.TrimSpace(string(out)), "\n"))
				}
			}
		}

		if len(matches) > maxSearchResults {
			matches = matches[:maxSearchResults]
		}

		return fileSearchResultMsg{query: query, matches: matches}
	}
}

// filterAndRankMatches filters cached results for the query, then ranks by
// frecency (history recency + frequency) and file type priority.
func filterAndRankMatches(matches []string, query string, history []string) []string {
	if query == "" {
		return nil
	}
	lower := strings.ToLower(query)

	// Build history lookup: position in history list (0 = most recent)
	historyRank := make(map[string]int)
	for i, h := range history {
		historyRank[h] = len(history) - i // higher = more recent
	}

	type scored struct {
		path  string
		score float64
	}

	var results []scored
	for _, m := range matches {
		if !strings.Contains(strings.ToLower(m), lower) {
			continue
		}

		s := 0.0

		// Frecency: boost files from history (recently/frequently opened)
		if rank, ok := historyRank[m]; ok {
			s += float64(rank) * 10
		}

		// File type priority
		ext := strings.ToLower(filepath.Ext(m))
		switch ext {
		case ".md":
			s += 50 // Markdown files ranked highest
		case ".txt", ".yaml", ".yml", ".toml":
			s += 30
		case ".go", ".ts", ".js", ".py":
			s += 20
		}

		// Boost exact filename match
		base := strings.ToLower(filepath.Base(m))
		if strings.Contains(base, lower) {
			s += 25 // filename match > path-only match
		}
		if base == lower || base == lower+".md" {
			s += 50 // exact match
		}

		results = append(results, scored{path: m, score: s})
	}

	// Sort by score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	var filtered []string
	for _, r := range results {
		filtered = append(filtered, r.path)
		if len(filtered) >= maxFileResults {
			break
		}
	}
	return filtered
}
