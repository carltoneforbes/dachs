package main

import (
	"os"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/fsnotify/fsnotify"
)

type fileChangedMsg struct {
	content string
}

type fileWatchErrMsg struct {
	err error
}

// watchFile creates a Cmd that blocks until the file changes on disk,
// then returns a fileChangedMsg with the new content. After handling the
// message, re-issue this Cmd to keep watching.
func watchFile(path string) tea.Cmd {
	return func() tea.Msg {
		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			return fileWatchErrMsg{err}
		}
		defer watcher.Close()

		if err := watcher.Add(path); err != nil {
			return fileWatchErrMsg{err}
		}

		// Debounce: wait for writes to settle
		for {
			select {
			case event := <-watcher.Events:
				if event.Op&(fsnotify.Write|fsnotify.Create) != 0 {
					// Small delay for writes to finish (editors do rename+write)
					time.Sleep(100 * time.Millisecond)
					content, err := os.ReadFile(path)
					if err != nil {
						return fileWatchErrMsg{err}
					}
					return fileChangedMsg{content: string(content)}
				}
			case err := <-watcher.Errors:
				if err != nil {
					return fileWatchErrMsg{err}
				}
			}
		}
	}
}

func handleFileChanged(m model, msg fileChangedMsg) (model, tea.Cmd) {
	newContent := strings.TrimRight(msg.content, "\n")

	// If the new content matches what we last saved, this was our own write — ignore
	if newContent == m.lastSaved {
		return m, watchFile(m.filePath)
	}

	// External change detected
	if !m.modified {
		// Buffer is clean — silently reload
		m.textarea.SetValue(newContent)
		m.lastSaved = newContent
		m.statusMsg = "Reloaded (external change)"
		return m, tea.Batch(clearStatusAfter(3*time.Second), watchFile(m.filePath))
	}

	// Buffer has unsaved changes — show conflict
	m.statusMsg = "File changed on disk! Ctrl+R to reload, Ctrl+S to overwrite"
	return m, watchFile(m.filePath)
}
