package schemaTable

import tea "github.com/charmbracelet/bubbletea"

type (
	SchemaCursorUpdateMsg struct{}
)

func schemaCursorUpdateEvent() tea.Msg {
	return SchemaCursorUpdateMsg{}
}
