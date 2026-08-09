package columnTile

import (
	"strings"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/atrahy/easypg/internal/sql"
	"github.com/atrahy/easypg/internal/tui/components/search"
	"github.com/atrahy/easypg/internal/tui/components/tableLayout"
	"github.com/atrahy/easypg/internal/tui/keys"
)

// columnSpecs let the name/type/default columns grow with the pane; "Not Null"
// only ever holds True/False so it stays at its title width.
var columnSpecs = []tableLayout.Spec{
	{Title: "Name", Min: 12, Weight: 3},
	{Title: "Type", Min: 10, Weight: 2},
	{Title: "Default", Min: 10, Weight: 4},
	{Title: "Not Null", Min: 8},
}

type Model struct {
	table  table.Model
	items  []sql.ColumnAttr
	filter search.TableFilter
}

func New() *Model {
	t := table.New(
		table.WithColumns(tableLayout.Fit(0, columnSpecs)),
		table.WithRows([]table.Row{}),
		table.WithFocused(true),
		table.WithKeyMap(keys.TableKeyMap(keys.Default)),
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

func (m *Model) SetItems(rows []sql.ColumnAttr) {
	var items []table.Row

	for _, row := range rows {
		items = append(items, table.Row{row.Name, row.Type, ptrToString(row.DefaultExpr), booleanToString(row.NotNullable)})
	}

	m.items = rows
	m.table.SetRows(items)
	m.table.SetCursor(0)
	m.filter.Reset()
}

func (m *Model) Filter(query string) int {
	return m.filter.Apply(&m.table, query)
}

func (m *Model) ClearFilter() {
	m.filter.Clear(&m.table)
}

func (m *Model) Filtering() bool {
	return m.filter.Active()
}

// Position is the cursor's place in the list, for the detail pane's border
// indicator.
func (m *Model) Position() (current, total int) {
	return tableLayout.Position(m.table)
}

// SelectedDetail describes the highlighted column in full, for the detail pane's
// inspector strip: the table cells are truncated, this is not.
func (m *Model) SelectedDetail() string {
	cursor := m.filter.SourceIndex(m.table.Cursor())
	if cursor < 0 || cursor >= len(m.items) {
		return ""
	}

	col := m.items[cursor]

	parts := []string{col.Name, col.Type}
	if col.NotNullable {
		parts = append(parts, "NOT NULL")
	}

	if col.DefaultExpr != nil {
		parts = append(parts, "DEFAULT "+*col.DefaultExpr)
	}

	return strings.Join(parts, "  ·  ")
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
	m.table.SetColumns(tableLayout.Fit(width, columnSpecs))
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
