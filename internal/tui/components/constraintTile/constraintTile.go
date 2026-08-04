package constraintTile

import (
	"github.com/atrahy/easypg/internal/sql"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Model struct {
	table table.Model
}

func New() *Model {
	columns := []table.Column{
		{Title: "Name", Width: 24},
		{Title: "Type", Width: 14},
		{Title: "Definition", Width: 40},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows([]table.Row{}),
		table.WithFocused(true),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.Bold(true)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	t.SetStyles(s)

	return &Model{table: t}
}

func (m *Model) SetItems(rows []sql.ConstraintAttr) {
	var items []table.Row

	for _, row := range rows {
		items = append(items, table.Row{row.Name, row.Type, row.Definition})
	}

	m.table.SetRows(items)
	m.table.SetCursor(0)
}

func (m *Model) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd

	m.table, cmd = m.table.Update(msg)

	return cmd
}

func (m *Model) View() string {
	return m.table.View()
}

func (m *Model) SetSize(width, height int) {
	m.table.SetWidth(width)
	m.table.SetHeight(height)
}
