package tui

import tea "github.com/charmbracelet/bubbletea"

type tabCursor int

const (
	definitionTab tabCursor = iota
	editorTab
)

type CustomModel interface {
	tea.Model

	SetSize(int, int)
}
