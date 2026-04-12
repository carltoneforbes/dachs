package main

import "strings"

type heading struct {
	level int    // 1-6
	text  string // heading text without #s
	line  int    // 0-indexed line number in file
}

func parseHeadings(content string) []heading {
	var headings []heading
	lines := strings.Split(content, "\n")
	inCodeBlock := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Track fenced code blocks
		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
			continue
		}
		if inCodeBlock {
			continue
		}

		// Match ATX headings
		if !strings.HasPrefix(trimmed, "#") {
			continue
		}

		level := 0
		for _, ch := range trimmed {
			if ch == '#' {
				level++
			} else {
				break
			}
		}
		if level < 1 || level > 6 {
			continue
		}
		// Must have a space after the #s (or be just #s at end of line)
		rest := trimmed[level:]
		if len(rest) > 0 && rest[0] != ' ' {
			continue
		}
		text := strings.TrimSpace(rest)
		if text == "" {
			continue
		}

		headings = append(headings, heading{
			level: level,
			text:  text,
			line:  i,
		})
	}

	return headings
}
