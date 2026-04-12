package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
)

func saveFile(m model) (tea.Model, tea.Cmd) {
	if m.filePath == "" {
		m.statusMsg = "No file path specified"
		return m, clearStatusAfter(3 * time.Second)
	}

	content := m.textarea.Value()
	// Ensure trailing newline (POSIX)
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}

	if err := os.WriteFile(m.filePath, []byte(content), 0644); err != nil {
		m.statusMsg = fmt.Sprintf("Error saving: %v", err)
		return m, clearStatusAfter(3 * time.Second)
	}

	m.lastSaved = m.textarea.Value()
	m.modified = false
	m.statusMsg = "Saved"
	return m, clearStatusAfter(3 * time.Second)
}

func openFile(m model, path string) (model, tea.Cmd) {
	content, err := os.ReadFile(path)
	if err != nil {
		m.statusMsg = fmt.Sprintf("Error opening: %v", err)
		m.mode = modeEdit
		return m, clearStatusAfter(3 * time.Second)
	}

	s := strings.TrimRight(string(content), "\n")
	m.textarea.SetValue(s)
	m.textarea.MoveToBegin()
	m.lastSaved = m.textarea.Value()
	m.filePath = path
	m.modified = false
	m.mode = modeEdit
	m.statusMsg = "Opened " + filepath.Base(path)
	return m, tea.Batch(clearStatusAfter(3*time.Second), watchFile(path))
}

func reloadFile(m model) (tea.Model, tea.Cmd) {
	if m.filePath == "" {
		m.statusMsg = "No file to reload"
		return m, clearStatusAfter(3 * time.Second)
	}

	content, err := os.ReadFile(m.filePath)
	if err != nil {
		m.statusMsg = fmt.Sprintf("Error reloading: %v", err)
		return m, clearStatusAfter(3 * time.Second)
	}

	s := strings.TrimRight(string(content), "\n")
	m.textarea.SetValue(s)
	m.lastSaved = m.textarea.Value()
	m.modified = false
	m.statusMsg = "Reloaded from disk"
	return m, clearStatusAfter(3 * time.Second)
}
