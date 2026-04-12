# Dachs

A TUI markdown editor for the terminal, built with the [Charm](https://charm.sh) ecosystem.

Dachs (German for dachshund, sounds like "docs") combines a markdown editor, file navigator, heading outline, Glamour-powered preview, and fuzzy file search in one lightweight terminal app.

## Features

- **Multiline editor** with line numbers, undo/redo, word movement, and selection (via [Bubbles textarea](https://github.com/charmbracelet/bubbles))
- **Sidebar** with four tabs:
  - **Files** — navigate directories, toggle hidden files
  - **Outline** — heading tree with jump-to navigation
  - **Favorites** — bookmark files you access often
  - **History** — recently opened files
- **Markdown preview** powered by [Glamour](https://github.com/charmbracelet/glamour) (the same renderer behind [Glow](https://github.com/charmbracelet/glow))
- **Fuzzy file search** (Ctrl+L) — search your entire disk using Spotlight (macOS) and [fd](https://github.com/sharkdp/fd)
- **Live reload** — automatically picks up external changes to the open file
- **Favorites & history** — persisted across sessions

## Install

### From source

Requires [Go](https://go.dev) 1.21+.

```bash
git clone https://github.com/carltoneforbes/dachs.git
cd dachs
make install
```

### Go install

```bash
go install github.com/carltoneforbes/dachs@latest
```

> **macOS note:** Go binaries need ad-hoc code signing on Apple Silicon. The Makefile handles this automatically. If you use `go install`, run `codesign --force --sign - $(which dachs)` afterward.

## Usage

```bash
# Open a file
dachs README.md

# Open in preview mode
dachs --preview README.md
dachs -p README.md

# Open file navigator (no file arg)
dachs

# Set default root directory
dachs --root ~/Documents
DACHS_ROOT=~/notes dachs
```

## Key Bindings

Press **Ctrl+G** in-app for the full list.

### Editor

| Key | Action |
|-----|--------|
| Ctrl+S | Save |
| Ctrl+Z / Ctrl+Y | Undo / Redo |
| Ctrl+P | Toggle markdown preview |
| Ctrl+L | Fuzzy file search |
| Ctrl+D | Toggle favorite on current file |
| Ctrl+G | Help screen |
| Ctrl+Q | Quit |

### Sidebar

| Key | Action |
|-----|--------|
| Ctrl+O | Files tab |
| Ctrl+T | Outline tab |
| Ctrl+F | Favorites tab |
| Ctrl+Y | History tab |
| Tab | Switch focus between sidebar and editor |
| [ / ] | Cycle sidebar tabs |
| Enter | Open file / jump to heading |
| Ctrl+H | Toggle hidden files (Files tab) |
| Esc | Close sidebar |

## Configuration

| Env var | Description |
|---------|-------------|
| `DACHS_ROOT` | Default root directory for the file navigator |

Favorites and history are persisted in `~/.config/dachs/state.json`.

## Dependencies

- **Required:** [Go](https://go.dev) 1.21+
- **Recommended:** [fd](https://github.com/sharkdp/fd) for fast file search (falls back to `find`)
- macOS Spotlight (`mdfind`) is used automatically when available

## Built With

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) — TUI framework
- [Bubbles](https://github.com/charmbracelet/bubbles) — textarea, viewport, list components
- [Glamour](https://github.com/charmbracelet/glamour) — markdown rendering
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) — terminal styling

## License

[MIT](LICENSE)
