package tableTable

import (
	"github.com/atrahy/easypg/internal/sql"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type TableTable struct {
	data  *[]sql.Table
	table table.Model
}

func NewTableTable() *TableTable {
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

	return &TableTable{
		data:  nil,
		table: t,
	}
}

func (t *TableTable) Update(msg tea.Msg) tea.Cmd {
	currentCursor := t.table.Cursor()

	// Update always return nil
	t.table, _ = t.table.Update(msg)

	newCursor := t.table.Cursor()
	if currentCursor != newCursor {
		return tableCursorUpdateEvent
	}

	return nil
}

func (t *TableTable) View() string {
	return t.table.View()
}

func (t *TableTable) SetItems(rows []sql.Table) tea.Cmd {
	var items []table.Row

	t.data = &rows
	for _, row := range rows {
		items = append(items, table.Row{row.Name})
	}

	t.table.SetRows(items)
	t.table.SetCursor(0)

	return tableCursorUpdateEvent
}

func (t *TableTable) SetSize(width, height int) {
	t.table.SetWidth(width)
	t.table.SetHeight(height)
	t.table.SetColumns(getColumns(width))
}

func (t *TableTable) IsEmpty() bool {
	return t.table.Cursor() == -1 || len(*t.data) == 0
}

func (t *TableTable) GetSelectedItemName() string {
	return (*t.data)[t.table.Cursor()].Name
}

func (t *TableTable) GetSelectedItemOID() string {
	return (*t.data)[t.table.Cursor()].OID
}

func getColumns(width int) []table.Column {
	return []table.Column{
		{Title: "Name", Width: width},
	}
}
