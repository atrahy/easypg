package schemaTable

import (
	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/atrahy/easypg/internal/sql"
	"github.com/atrahy/easypg/internal/tui/components/search"
	"github.com/atrahy/easypg/internal/tui/components/tableLayout"
	"github.com/atrahy/easypg/internal/tui/keys"
)

// schemaSpecs favor the name in this narrow pane; the description takes what is
// left (often nothing, in which case it collapses to its minimum).
var schemaSpecs = []tableLayout.Spec{
	{Title: "Name", Min: 12, Weight: 2},
	{Title: "Description", Min: 6, Weight: 1},
}

type SchemaTable struct {
	data   *[]sql.Namespace
	table  table.Model
	filter search.TableFilter
}

func NewSchemaTable() *SchemaTable {
	columns := tableLayout.Fit(0, schemaSpecs)

	rows := []table.Row{}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithKeyMap(keys.TableKeyMap(keys.Default)),
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
	s.filter.Reset()

	return schemaCursorUpdateEvent
}

func (s *SchemaTable) Filter(query string) int {
	return s.filter.Apply(&s.table, query)
}

func (s *SchemaTable) ClearFilter() {
	s.filter.Clear(&s.table)
}

func (s *SchemaTable) Filtering() bool {
	return s.filter.Active()
}

// Position is the cursor's place in the list, for the pane's border indicator.
func (s *SchemaTable) Position() (current, total int) {
	return tableLayout.Position(s.table)
}

// SelectionEvent re-emits the selection message, for the moves that do not go
// through Update (a search jumping the cursor, a refresh).
func (s *SchemaTable) SelectionEvent() tea.Cmd {
	return schemaCursorUpdateEvent
}

func (s *SchemaTable) SetSize(width, height int) {
	s.table.SetWidth(width)
	s.table.SetHeight(height)
	s.table.SetColumns(tableLayout.Fit(width, schemaSpecs))
}

// GetSelectedItemName is empty until the first fetch lands, and stays safe if
// the cursor and the data ever disagree (a refresh re-emitting the selection
// before its rows arrive).
func (s *SchemaTable) GetSelectedItemName() string {
	if s.data == nil {
		return ""
	}

	cursor := s.filter.SourceIndex(s.table.Cursor())
	if cursor < 0 || cursor >= len(*s.data) {
		return ""
	}

	return (*s.data)[cursor].Name
}

func nullableToString(s *string) string {
	if s == nil {
		return ""
	}

	return *s
}
