package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	tea "charm.land/bubbletea/v2"
)

// ---------------------------------------------------------------------------
// Scoring constants — multi-tier ranking for file search results
// ---------------------------------------------------------------------------

const (
	scoreExactName   = 140 // filename matches query exactly
	scoreNamePrefix  = 118 // filename starts with query
	scoreAllTokens   = 102 // every query token found in entry tokens
	scoreTokenPrefix = 88  // query tokens are prefixes of entry tokens
	scoreSubstring   = 70  // query appears as substring of name
	scoreSubsequence = 44  // query chars appear in order within name

	boostMarkdown = 50 // .md files
	boostText     = 30 // .txt, .yaml, .yml, .toml
	boostCode     = 20 // .go, .ts, .js, .py

	penaltyDir = -10 // directories ranked below files

	maxIndexEntries  = 500_000
	maxPrefixLen     = 12
	maxIndexSearchResults = 8
	refreshInterval  = 5 * time.Minute
)

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// IndexEntry represents a single file or directory in the index.
type IndexEntry struct {
	Name     string   // filename only
	Path     string   // full absolute path
	IsDir    bool
	Tokens   []string // lowercased tokenized name
	NormName string   // lowercased name
	NormPath string   // lowercased full path
}

// FileIndex is the in-memory index built by walking the home directory.
type FileIndex struct {
	Entries   []IndexEntry
	PrefixMap map[string][]int // prefix (up to 12 chars) -> entry indices
	BuiltAt   time.Time
	Ready     bool
}

// ---------------------------------------------------------------------------
// Bubble Tea messages
// ---------------------------------------------------------------------------

type fileIndexReadyMsg struct {
	index *FileIndex
}

type fileIndexRefreshMsg struct{}

// ---------------------------------------------------------------------------
// Excluded directory names — skipped during the walk
// ---------------------------------------------------------------------------

var excludedDirs = map[string]bool{
	"Library":     true,
	"Movies":      true,
	"Music":       true,
	"Photos":      true,
	".Trash":      true,
	"node_modules": true,
	"__pycache__": true,
	".venv":       true,
	"dist":        true,
	"build":       true,
	".next":       true,
	".cache":      true,
	".tmp":        true,
}

// ---------------------------------------------------------------------------
// buildFileIndex returns a tea.Cmd that walks ~/ in a goroutine, builds the
// index, and sends fileIndexReadyMsg when complete.
// ---------------------------------------------------------------------------

func buildFileIndex() tea.Cmd {
	return func() tea.Msg {
		home := expandPath("~/")
		entries := make([]IndexEntry, 0, 50_000)

		filepath.WalkDir(home, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return filepath.SkipDir
			}

			name := d.Name()

			// Skip excluded directories
			if d.IsDir() {
				if excludedDirs[name] {
					return filepath.SkipDir
				}
				// Special case: skip .git/objects (deep object store) but allow
				// .git itself so config/hooks are indexable
				if name == "objects" {
					parent := filepath.Base(filepath.Dir(path))
					if parent == ".git" {
						return filepath.SkipDir
					}
				}
			}

			// Enforce max entries
			if len(entries) >= maxIndexEntries {
				return filepath.SkipAll
			}

			tokens := tokenizeName(name)
			normName := strings.ToLower(name)
			normPath := strings.ToLower(path)

			entries = append(entries, IndexEntry{
				Name:     name,
				Path:     path,
				IsDir:    d.IsDir(),
				Tokens:   tokens,
				NormName: normName,
				NormPath: normPath,
			})

			return nil
		})

		prefixMap := buildPrefixMap(entries)

		idx := &FileIndex{
			Entries:   entries,
			PrefixMap: prefixMap,
			BuiltAt:   time.Now(),
			Ready:     true,
		}
		return fileIndexReadyMsg{index: idx}
	}
}

// ---------------------------------------------------------------------------
// refreshFileIndex waits for the refresh interval, then signals the model
// to rebuild the index.
// ---------------------------------------------------------------------------

func refreshFileIndex() tea.Cmd {
	return tea.Tick(refreshInterval, func(t time.Time) tea.Msg {
		return fileIndexRefreshMsg{}
	})
}

// ---------------------------------------------------------------------------
// tokenizeName splits a filename into lowercase tokens by camelCase
// boundaries, punctuation delimiters (-, _, .), and treats the file
// extension as a separate token.
//
// Examples:
//   "CamelCaseFile.md"      -> ["camel", "case", "file", "md"]
//   "my-project_notes.txt"  -> ["my", "project", "notes", "txt"]
//   "README.md"             -> ["readme", "md"]
// ---------------------------------------------------------------------------

func tokenizeName(name string) []string {
	var tokens []string
	var current strings.Builder

	flush := func() {
		if current.Len() > 0 {
			tokens = append(tokens, strings.ToLower(current.String()))
			current.Reset()
		}
	}

	runes := []rune(name)
	for i := 0; i < len(runes); i++ {
		r := runes[i]

		// Delimiters: split and discard the delimiter character
		if r == '-' || r == '_' || r == '.' || r == ' ' {
			flush()
			continue
		}

		// CamelCase boundary: an uppercase letter following a lowercase letter,
		// or an uppercase letter followed by a lowercase letter in a run of
		// uppercase (e.g., "HTMLParser" -> "html", "parser").
		if unicode.IsUpper(r) {
			if current.Len() > 0 {
				// Previous char was lowercase — simple camel boundary
				prevRune := runes[i-1]
				if unicode.IsLower(prevRune) || unicode.IsDigit(prevRune) {
					flush()
				} else if unicode.IsUpper(prevRune) && i+1 < len(runes) && unicode.IsLower(runes[i+1]) {
					// Transition like "ML" -> "P" in "HTMLParser"
					flush()
				}
			}
			current.WriteRune(r)
		} else {
			current.WriteRune(r)
		}
	}
	flush()

	return tokens
}

// ---------------------------------------------------------------------------
// buildPrefixMap builds a map from every prefix (length 1..12) of every
// token in every entry to the list of entry indices that contain that prefix.
// ---------------------------------------------------------------------------

func buildPrefixMap(entries []IndexEntry) map[string][]int {
	pm := make(map[string][]int, len(entries)*4)

	for idx, entry := range entries {
		// Track which prefixes we have already added for this entry
		// to avoid duplicate index entries from repeated tokens.
		seen := make(map[string]bool)

		for _, token := range entry.Tokens {
			limit := len(token)
			if limit > maxPrefixLen {
				limit = maxPrefixLen
			}
			for plen := 1; plen <= limit; plen++ {
				prefix := token[:plen]
				if !seen[prefix] {
					seen[prefix] = true
					pm[prefix] = append(pm[prefix], idx)
				}
			}
		}
	}

	return pm
}

// ---------------------------------------------------------------------------
// searchFileIndex is the main search entry point. It returns up to 8 file
// paths, ranked by a multi-tier scoring system.
//
// If the query contains "/" or starts with "~", path-based substring matching
// is used. Otherwise, token-based prefix-map lookup with intersection and
// scoring is applied.
// ---------------------------------------------------------------------------

// SearchResult holds a single search result with metadata.
type SearchResult struct {
	Path  string
	IsDir bool
	Score int
}

func searchFileIndex(idx *FileIndex, query string, history []string) []SearchResult {
	if idx == nil || !idx.Ready || len(query) == 0 {
		return nil
	}

	normQuery := strings.ToLower(strings.TrimSpace(query))
	if len(normQuery) == 0 {
		return nil
	}

	// Path-based search when query looks like a path
	if strings.Contains(normQuery, "/") || strings.HasPrefix(normQuery, "~") {
		return searchByPath(idx, normQuery, history)
	}

	return searchByTokens(idx, normQuery, history)
}

// searchByPath does a simple substring match against the full normalized path.
func searchByPath(idx *FileIndex, normQuery string, history []string) []SearchResult {
	var results []SearchResult

	for _, entry := range idx.Entries {
		if strings.Contains(entry.NormPath, normQuery) {
			s := scoreEntry(entry, normQuery, tokenizeName(normQuery), history)
			results = append(results, SearchResult{Path: entry.Path, IsDir: entry.IsDir, Score: s})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if len(results) > maxIndexSearchResults {
		results = results[:maxIndexSearchResults]
	}
	return results
}

// searchByTokens uses the prefix map to find candidates, then scores and ranks.
func searchByTokens(idx *FileIndex, normQuery string, history []string) []SearchResult {
	queryTokens := tokenizeName(normQuery)
	if len(queryTokens) == 0 {
		// Single token that did not split — use the query itself
		queryTokens = []string{normQuery}
	}

	// Gather candidate sets from prefix map, one per query token
	candidateSets := make([][]int, 0, len(queryTokens))
	for _, qt := range queryTokens {
		key := qt
		if len(key) > maxPrefixLen {
			key = key[:maxPrefixLen]
		}
		if indices, ok := idx.PrefixMap[key]; ok {
			candidateSets = append(candidateSets, indices)
		} else {
			// No prefix match for this token — fall back to linear scan
			candidateSets = nil
			break
		}
	}

	var candidates []int

	if len(candidateSets) > 0 {
		candidates = intersectSorted(candidateSets)
	} else {
		// Fallback: linear scan for substring/subsequence matches
		for i, entry := range idx.Entries {
			if strings.Contains(entry.NormName, normQuery) || isSubsequence(normQuery, entry.NormName) {
				candidates = append(candidates, i)
				if len(candidates) > 10_000 {
					break
				}
			}
		}
	}

	// Score all candidates
	results := make([]SearchResult, 0, len(candidates))
	for _, ci := range candidates {
		entry := idx.Entries[ci]
		s := scoreEntry(entry, normQuery, queryTokens, history)
		if s > 0 {
			results = append(results, SearchResult{Path: entry.Path, IsDir: entry.IsDir, Score: s})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		return len(results[i].Path) < len(results[j].Path)
	})

	if len(results) > maxIndexSearchResults {
		results = results[:maxIndexSearchResults]
	}
	return results
}

// ---------------------------------------------------------------------------
// intersectSorted finds entry indices that appear in ALL candidate sets.
// Sorts sets by length (smallest first), then iterates the shortest set and
// checks membership in the others.
// ---------------------------------------------------------------------------

func intersectSorted(sets [][]int) []int {
	if len(sets) == 0 {
		return nil
	}
	if len(sets) == 1 {
		return sets[0]
	}

	// Sort sets by length, smallest first
	sort.Slice(sets, func(i, j int) bool {
		return len(sets[i]) < len(sets[j])
	})

	// Build lookup sets for all but the smallest
	lookups := make([]map[int]bool, len(sets)-1)
	for i := 1; i < len(sets); i++ {
		m := make(map[int]bool, len(sets[i]))
		for _, idx := range sets[i] {
			m[idx] = true
		}
		lookups[i-1] = m
	}

	// Iterate smallest set, keep entries present in all others
	var result []int
	for _, idx := range sets[0] {
		inAll := true
		for _, lk := range lookups {
			if !lk[idx] {
				inAll = false
				break
			}
		}
		if inAll {
			result = append(result, idx)
		}
	}

	return result
}

// ---------------------------------------------------------------------------
// scoreEntry computes a composite score for a single entry against the query.
// ---------------------------------------------------------------------------

func scoreEntry(entry IndexEntry, normQuery string, queryTokens []string, history []string) int {
	score := 0

	// --- Tier matching (take the highest tier that matches) ---

	tierScore := 0

	// Exact name match
	if entry.NormName == normQuery {
		tierScore = scoreExactName
	}

	// Name starts with query
	if tierScore < scoreNamePrefix && strings.HasPrefix(entry.NormName, normQuery) {
		tierScore = scoreNamePrefix
	}

	// All query tokens found as exact tokens in entry
	if tierScore < scoreAllTokens && allTokensMatch(entry.Tokens, queryTokens) {
		tierScore = scoreAllTokens
	}

	// Token prefix match — every query token is a prefix of some entry token
	if tierScore < scoreTokenPrefix && allTokenPrefixes(entry.Tokens, queryTokens) {
		tierScore = scoreTokenPrefix
	}

	// Substring match in name
	if tierScore < scoreSubstring && strings.Contains(entry.NormName, normQuery) {
		tierScore = scoreSubstring
	}

	// Subsequence match in name
	if tierScore < scoreSubsequence && isSubsequence(normQuery, entry.NormName) {
		tierScore = scoreSubsequence
	}

	// No match at all — skip this entry
	if tierScore == 0 {
		return 0
	}

	score += tierScore

	// --- History frecency boost ---
	// Index 0 in history = most recent, give highest boost
	for i, h := range history {
		if h == entry.Path {
			score += max(1, 50-i*2) // recent files get up to +50
			break
		}
	}

	// --- File type boost ---
	ext := strings.ToLower(filepath.Ext(entry.Name))
	switch ext {
	case ".md":
		score += boostMarkdown
	case ".txt", ".yaml", ".yml", ".toml":
		score += boostText
	case ".go", ".ts", ".js", ".py":
		score += boostCode
	}

	// --- Length penalty: prefer shorter names ---
	nameLen := len(entry.NormName)
	queryLen := len(normQuery)
	lengthBonus := 20 - (nameLen - queryLen)
	if lengthBonus < 0 {
		lengthBonus = 0
	}
	score += lengthBonus

	// --- Directory penalty ---
	if entry.IsDir {
		score += penaltyDir
	}

	return score
}

// allTokensMatch returns true if every query token appears as an exact token
// in the entry's token list.
func allTokensMatch(entryTokens, queryTokens []string) bool {
	if len(queryTokens) == 0 {
		return false
	}
	set := make(map[string]bool, len(entryTokens))
	for _, t := range entryTokens {
		set[t] = true
	}
	for _, qt := range queryTokens {
		if !set[qt] {
			return false
		}
	}
	return true
}

// allTokenPrefixes returns true if every query token is a prefix of at least
// one entry token.
func allTokenPrefixes(entryTokens, queryTokens []string) bool {
	if len(queryTokens) == 0 {
		return false
	}
	for _, qt := range queryTokens {
		found := false
		for _, et := range entryTokens {
			if strings.HasPrefix(et, qt) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------------------
// isSubsequence checks whether needle is a subsequence of haystack.
// Characters must appear in order but need not be contiguous.
// ---------------------------------------------------------------------------

func isSubsequence(needle, haystack string) bool {
	if len(needle) == 0 {
		return true
	}
	ni := 0
	needleRunes := []rune(needle)
	for _, r := range haystack {
		if r == needleRunes[ni] {
			ni++
			if ni == len(needleRunes) {
				return true
			}
		}
	}
	return false
}

