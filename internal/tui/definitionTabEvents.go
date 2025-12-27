package tui

import tea "github.com/charmbracelet/bubbletea"

type schemaCursorUpdateMsg struct{}

func schemaCursorUpdateEvent() tea.Msg {
	return schemaCursorUpdateMsg{}
}
