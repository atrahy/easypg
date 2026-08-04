package sqlTile

import (
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// Model is a read-only, scrollable text tile used to show generated SQL/DDL.
type Model struct {
	viewport viewport.Model
}

func New() *Model {
	return &Model{viewport: viewport.New(0, 0)}
}

func (m *Model) SetContent(content string) {
	m.viewport.SetContent(content)
	m.viewport.GotoTop()
}

func (m *Model) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd

	m.viewport, cmd = m.viewport.Update(msg)

	return cmd
}

func (m *Model) View() string {
	return m.viewport.View()
}

func (m *Model) SetSize(width, height int) {
	m.viewport.Width = width
	m.viewport.Height = height
}
