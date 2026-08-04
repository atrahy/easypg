package objectsPane

import tea "github.com/charmbracelet/bubbletea"

type (
	ObjectSelectedMsg struct{}
)

func objectSelectedEvent() tea.Msg {
	return ObjectSelectedMsg{}
}
