package indexTile

import (
	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/atrahy/easypg/internal/sql"
	"github.com/atrahy/easypg/internal/tui/components/search"
	"github.com/atrahy/easypg/internal/tui/components/tableLayout"
	"github.com/atrahy/easypg/internal/tui/keys"
)

// indexSpecs give most of the pane to the definition (a full CREATE INDEX
// statement); the two boolean columns stay at their title width.
var indexSpecs = []tableLayout.Spec{
	{Title: "Name", Min: 16, Weight: 2},
	{Title: "Definition", Min: 20, Weight: 5},
	{Title: "Unique", Min: 6},
	{Title: "Primary", Min: 7},
}

type Model struct {
	table  table.Model
	items  []sql.IndexAttr
	filter search.TableFilter
}

func New() *Model {
	t := table.New(
		table.WithColumns(tableLayout.Fit(0, indexSpecs)),
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

func (m *Model) SetItems(rows []sql.IndexAttr) {
	var items []table.Row

	for _, row := range rows {
		items = append(items, table.Row{row.Name, row.Definition, booleanToString(row.IsUnique), booleanToString(row.IsPrimary)})
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

// SelectedName is the highlighted index's name, for the status bar's context
// path — SelectedDetail returns the whole CREATE INDEX statement.
func (m *Model) SelectedName() string {
	cursor := m.filter.SourceIndex(m.table.Cursor())
	if cursor < 0 || cursor >= len(m.items) {
		return ""
	}

	return m.items[cursor].Name
}

// SelectedDetail returns the highlighted index's full definition, for the detail
// pane's inspector strip.
func (m *Model) SelectedDetail() string {
	cursor := m.filter.SourceIndex(m.table.Cursor())
	if cursor < 0 || cursor >= len(m.items) {
		return ""
	}

	return m.items[cursor].Definition
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
	m.table.SetColumns(tableLayout.Fit(width, indexSpecs))
}

func booleanToString(b bool) string {
	if b {
		return "True"
	}

	return "False"
}
