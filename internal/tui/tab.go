package tui

import tea "charm.land/bubbletea/v2"

type tabCursor int

const (
	definitionTab tabCursor = iota
	editorTab
)

// tab is a screen owned by the root model. It is deliberately *not* a tea.Model:
// since v2 a tea.Model returns a tea.View, which carries terminal-level state
// (alt-screen, cursor, window title) that belongs to the root alone. A tab
// renders a string, and the root composes it into the frame it returns.
type tab interface {
	Init() tea.Cmd
	Update(tea.Msg) (tab, tea.Cmd)
	View() string
}
