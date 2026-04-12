package main

import (
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
)

const version = "0.1.0"

func main() {
	var filePath string
	var startPreview bool
	var rootDir string

	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--preview" || args[i] == "-p":
			startPreview = true
		case args[i] == "--root" || args[i] == "-r":
			if i+1 < len(args) {
				i++
				rootDir = args[i]
			}
		case args[i] == "--version" || args[i] == "-v":
			fmt.Printf("dachs %s\n", version)
			os.Exit(0)
		case args[i] == "--help" || args[i] == "-h":
			printUsage()
			os.Exit(0)
		case strings.HasPrefix(args[i], "-"):
			fmt.Fprintf(os.Stderr, "Unknown flag: %s\nRun 'dachs --help' for usage.\n", args[i])
			os.Exit(1)
		default:
			filePath = args[i]
		}
	}

	if rootDir != "" {
		os.Setenv("DACHS_ROOT", rootDir)
	}

	m, err := newModel(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if startPreview {
		m.startInPreview = true
	}

	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Print(`dachs — A TUI markdown editor

Usage:
  dachs [flags] [file.md]

Flags:
  -p, --preview       Open file in preview mode
  -r, --root <dir>    Set root directory for file navigator
  -v, --version       Print version
  -h, --help          Print this help

Environment:
  DACHS_ROOT          Default root directory for file navigator

Keys (press Ctrl+G in-app for full list):
  Ctrl+S              Save
  Ctrl+O              File navigator sidebar
  Ctrl+T              Heading outline sidebar
  Ctrl+L              Open file by path (fuzzy search)
  Ctrl+P              Toggle markdown preview
  Ctrl+D              Toggle favorite
  Ctrl+G              Help

https://github.com/carltoneforbes/dachs
`)
}
