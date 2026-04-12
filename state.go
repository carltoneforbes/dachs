package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const maxHistoryItems = 50

type dachsState struct {
	Favorites []string `json:"favorites"`
	History   []string `json:"history"`
}

func stateFilePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "dachs", "state.json")
}

func loadState() dachsState {
	data, err := os.ReadFile(stateFilePath())
	if err != nil {
		return dachsState{}
	}
	var s dachsState
	json.Unmarshal(data, &s)
	return s
}

func saveState(s dachsState) {
	path := stateFilePath()
	os.MkdirAll(filepath.Dir(path), 0755)
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(path, data, 0644)
}

func (s *dachsState) toggleFavorite(path string) bool {
	for i, f := range s.Favorites {
		if f == path {
			s.Favorites = append(s.Favorites[:i], s.Favorites[i+1:]...)
			return false // removed
		}
	}
	s.Favorites = append(s.Favorites, path)
	return true // added
}

func (s *dachsState) isFavorite(path string) bool {
	for _, f := range s.Favorites {
		if f == path {
			return true
		}
	}
	return false
}

func (s *dachsState) addHistory(path string) {
	// Remove if already present (move to top)
	for i, h := range s.History {
		if h == path {
			s.History = append(s.History[:i], s.History[i+1:]...)
			break
		}
	}
	// Prepend
	s.History = append([]string{path}, s.History...)
	// Trim
	if len(s.History) > maxHistoryItems {
		s.History = s.History[:maxHistoryItems]
	}
}
