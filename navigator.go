package main

import (
	"os"
	"path/filepath"
	"strings"
)

type fileItem struct {
	name  string
	path  string
	isDir bool
}

func (f fileItem) Title() string       { return f.name }
func (f fileItem) Description() string { return "" }
func (f fileItem) FilterValue() string { return f.name }

// defaultRoot returns the default root directory for the file navigator.
// Priority: $DACHS_ROOT env var > --root flag > current working directory.
func defaultRoot() string {
	if root := os.Getenv("DACHS_ROOT"); root != "" {
		return expandPath(root)
	}
	cwd, err := os.Getwd()
	if err != nil {
		home, _ := os.UserHomeDir()
		return home
	}
	return cwd
}

func expandPath(p string) string {
	if strings.HasPrefix(p, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, p[2:])
	}
	return p
}

func shortenPath(p string) string {
	home, _ := os.UserHomeDir()
	if strings.HasPrefix(p, home) {
		return "~" + p[len(home):]
	}
	return p
}

type openFileMsg struct {
	path string
}
