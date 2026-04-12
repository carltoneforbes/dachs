package main

import (
	"os/exec"
	"runtime"
	"strings"

	tea "charm.land/bubbletea/v2"
)

const maxFileResults = 8
const maxSearchResults = 50

type fileSearchResultMsg struct {
	query   string
	matches []string
}

// searchFiles finds files matching the query using the best available tool.
// macOS: mdfind (Spotlight) + fd for hidden dirs
// Linux/other: fd, falling back to find
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

		// Try fd for hidden files (works on all platforms)
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

		// macOS: also use Spotlight for indexed files
		if runtime.GOOS == "darwin" {
			if mdfindPath, err := exec.LookPath("mdfind"); err == nil {
				mdfindCmd := exec.Command(mdfindPath, "-name", query)
				if out, err := mdfindCmd.Output(); err == nil {
					addResults(strings.Split(strings.TrimSpace(string(out)), "\n"))
				}
			}
		}

		// Fallback: if no results yet, try find
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

// filterMatches filters cached search results client-side for the current query.
func filterMatches(matches []string, query string) []string {
	if query == "" {
		return nil
	}
	lower := strings.ToLower(query)
	var filtered []string
	for _, m := range matches {
		if strings.Contains(strings.ToLower(m), lower) {
			filtered = append(filtered, m)
			if len(filtered) >= maxFileResults {
				break
			}
		}
	}
	return filtered
}
