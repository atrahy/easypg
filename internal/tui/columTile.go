package tui

import (
	"github.com/atrahy/easypg/internal/sql"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type columnTile struct {
	Columns table.Model
}

// type item struct {
// 	title string
// }

// func (i item) Title() string       { return i.title }
// func (i item) Description() string { return i.title }
// func (i item) FilterValue() string { return i.title }

func newColumnTile() *columnTile {
	columns := []table.Column{
		{Title: "Name", Width: 20},
		{Title: "Type", Width: 10},
		{Title: "Default", Width: 20},
		{Title: "Not Null", Width: 8},
	}

	rows := []table.Row{}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(7),
	)

	s := table.DefaultStyles()

	s.Header = s.Header.
		// BorderStyle(lipgloss.NormalBorder()).
		// Border(lipgloss.NormalBorder(), true).
		// BorderForeground(lipgloss.Color("240")).
		Bold(true)

	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	t.SetStyles(s)

	// l := list.New(nil, list.NewDefaultDelegate(), 0, 0)
	// // l.SetShowFilter(false)
	// l.SetShowHelp(false)
	// l.SetShowStatusBar(false)
	// l.SetShowTitle(false)

	// return &schemaTile{l}

	return &columnTile{t}
}

func booleanToString(b bool) string {
	if b {
		return "True"
	}

	return "False"
}

func ptrToString(s *string) string {
	if s == nil {
		return ""
	}

	return *s
}

func (c *columnTile) SetItems(rows []sql.ColumnAttr) tea.Cmd {
	var items []table.Row

	for _, row := range rows {
		items = append(items, table.Row{row.Name, row.Type, ptrToString(row.DefaultExpr), booleanToString(row.NotNullable)})
	}

	c.Columns.SetRows(items)
	c.Columns.SetCursor(0)

	return nil
}

func (c *columnTile) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd

	c.Columns, cmd = c.Columns.Update(msg)

	return cmd
}

func (c *columnTile) View() string {
	return c.Columns.View()
}

// func (c *columnTile) GetSelectedItemName() string {
// 	return n.List.SelectedItem().(item).title
// }

// func (c *columnTile) GetSelectedItemID() int {
// 	return n.List.Cursor()
// }

// func (c *columnTile) Cursor() int {
// 	return n.List.Cursor()
// }

func (c *columnTile) SetSize(width, height int) {
	c.Columns.SetWidth(width)
	c.Columns.SetHeight(height)
}
