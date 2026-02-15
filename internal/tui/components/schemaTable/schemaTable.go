package schemaTable

import (
	"github.com/atrahy/easypg/internal/sql"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type SchemaTable struct {
	data  *[]sql.Namespace
	table table.Model
}

func NewSchemaTable() *SchemaTable {
	columns := getColumns(0)

	rows := []table.Row{}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
	)

	s := table.DefaultStyles()

	s.Header = s.Header.
		BorderForeground(lipgloss.Color("240")).
		Bold(false)

	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	t.SetStyles(s)

	return &SchemaTable{
		data:  nil,
		table: t,
	}
}

func (s *SchemaTable) Update(msg tea.Msg) tea.Cmd {
	currentCursor := s.table.Cursor()

	// Update always return nil
	s.table, _ = s.table.Update(msg)

	newCursor := s.table.Cursor()
	if currentCursor != newCursor {
		return schemaCursorUpdateEvent
	}

	return nil
}

func (s *SchemaTable) View() string {
	return s.table.View()
}

func (s *SchemaTable) SetItems(rows []sql.Namespace) tea.Cmd {
	var items []table.Row

	s.data = &rows

	for _, row := range rows {
		items = append(items, table.Row{row.Name, nullableToString(row.Description)})
	}

	s.table.SetRows(items)
	s.table.SetCursor(0)

	return schemaCursorUpdateEvent
}

func (s *SchemaTable) SetSize(width, height int) {
	s.table.SetWidth(width)
	s.table.SetHeight(height)
	s.table.SetColumns(getColumns(width))
}

func (s *SchemaTable) GetSelectedItemName() string {
	return (*s.data)[s.table.Cursor()].Name
}

func getColumns(width int) []table.Column {
	return []table.Column{
		{Title: "Name", Width: 20},
		{Title: "Description", Width: width - 20},
	}
}

func nullableToString(s *string) string {
	if s == nil {
		return ""
	}

	return *s
}
