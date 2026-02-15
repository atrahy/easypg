package tableTable

import tea "github.com/charmbracelet/bubbletea"

type (
	TableCursorUpdateMsg struct{}
)

func tableCursorUpdateEvent() tea.Msg {
	return TableCursorUpdateMsg{}
}
