package ui

import (
	"strings"

	"github.com/charmbracelet/glamour"
)

// help uses styleHelpBorder from styles.go (amber ops-console frame).

const helpMarkdown = `# repo-cleanup-tui · keybindings

## Browse
| Key | Action |
|-----|--------|
| **j** / **↓** | Next row |
| **u** / **↑** | Previous row |
| **[** / **]** | Page up / down (20 rows) |
| **s** | Toggle sort (size / inactive) |
| **f** | Cycle inactivity filter |
| **k** | Toggle safe-only (lockfile) |
| **d** | Toggle dirty-only |
| **g** | Toggle git columns |
| **r** | Rescan workspace |
| **x** | Cleanup preview |
| **/** | Search by path or branch |
| **c** | Clear search |
| **w** | Switch workspace (quick path) |
| **m** | Workspace manager (add/edit/remove) |
| **?** | Toggle this help |

## Workspace manager
| Key | Action |
|-----|--------|
| **j** / **↓** | Next workspace |
| **u** / **↑** | Previous workspace |
| **enter** | Set active workspace, save, return and rescan |
| **a** | Add workspace path |
| **i** | Edit ignore dirs for selection |
| **delete** / **backspace** | Remove workspace (keeps at least one) |
| **esc** / **q** | Back to list or browse |

## Search
| Key | Action |
|-----|--------|
| **enter** | Apply and return to browse |
| **esc** / **q** | Cancel to browse |

## Workspace
| Key | Action |
|-----|--------|
| **enter** | Set workspace and rescan |
| **esc** / **q** | Cancel to browse |

## Cleanup preview
| Key | Action |
|-----|--------|
| **p** | Dry-run (no delete) |
| **y** | Continue to confirm |
| **n** | Cancel to browse |

## Confirm
| Key | Action |
|-----|--------|
| **enter** | Delete node_modules after typing token |
| **esc** / **q** | Cancel to browse |
`

// RenderHelp renders markdown help for the given terminal width.
func RenderHelp(width int) string {
	if width < 30 {
		width = 80
	}
	innerW := width - 4
	if innerW < 40 {
		innerW = 72
	}
	r, err := glamour.NewTermRenderer(
		glamour.WithWordWrap(innerW),
		glamour.WithStandardStyle("dark"),
	)
	if err != nil {
		return styleHelpBorder.Render(fallbackHelpText())
	}
	out, err := r.Render(helpMarkdown)
	if err != nil {
		return styleHelpBorder.Render(fallbackHelpText())
	}
	return styleHelpBorder.Render(strings.TrimRight(out, "\n"))
}

func fallbackHelpText() string {
	return `repo-cleanup-tui keybindings

  j / down       next row
  u / up         previous row
  [ / ]          page up / down (20 rows)
  s              sort size/inactive
  f              inactive filter
  k              safe-only
  d              dirty-only
  g              git columns
  r              rescan
  x              cleanup preview
  /              search
  c              clear search
  w              workspace path
  m              workspace manager
  ?              help
  p              dry-run
  y              confirm step
  esc / q        back or quit`
}
