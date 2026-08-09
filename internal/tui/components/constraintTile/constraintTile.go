package constraintTile

import (
	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/atrahy/easypg/internal/sql"
	"github.com/atrahy/easypg/internal/tui/components/search"
	"github.com/atrahy/easypg/internal/tui/components/tableLayout"
	"github.com/atrahy/easypg/internal/tui/keys"
)

// constraintSpecs give most of the pane to the definition (a full FOREIGN
// KEY/CHECK clause), the type column holding short labels only.
var constraintSpecs = []tableLayout.Spec{
	{Title: "Name", Min: 16, Weight: 2},
	{Title: "Type", Min: 10, Weight: 1},
	{Title: "Definition", Min: 20, Weight: 5},
}

type Model struct {
	table  table.Model
	items  []sql.ConstraintAttr
	filter search.TableFilter
}

func New() *Model {
	t := table.New(
		table.WithColumns(tableLayout.Fit(0, constraintSpecs)),
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

func (m *Model) SetItems(rows []sql.ConstraintAttr) {
	var items []table.Row

	for _, row := range rows {
		items = append(items, table.Row{row.Name, row.Type, row.Definition})
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

// SelectedName is the highlighted constraint's name, for the status bar's
// context path.
func (m *Model) SelectedName() string {
	cursor := m.filter.SourceIndex(m.table.Cursor())
	if cursor < 0 || cursor >= len(m.items) {
		return ""
	}

	return m.items[cursor].Name
}

// SelectedDetail returns the highlighted constraint's full definition, for the
// detail pane's inspector strip.
func (m *Model) SelectedDetail() string {
	cursor := m.filter.SourceIndex(m.table.Cursor())
	if cursor < 0 || cursor >= len(m.items) {
		return ""
	}

	con := m.items[cursor]

	return con.Name + "  ·  " + con.Definition
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
	m.table.SetColumns(tableLayout.Fit(width, constraintSpecs))
}
