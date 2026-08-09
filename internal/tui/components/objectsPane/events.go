package objectsPane

import tea "charm.land/bubbletea/v2"

type (
	ObjectSelectedMsg struct{}
)

func objectSelectedEvent() tea.Msg {
	return ObjectSelectedMsg{}
}
