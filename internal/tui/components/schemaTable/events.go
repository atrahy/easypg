package schemaTable

import tea "charm.land/bubbletea/v2"

type (
	SchemaCursorUpdateMsg struct{}
)

func schemaCursorUpdateEvent() tea.Msg {
	return SchemaCursorUpdateMsg{}
}
