package objectsPane

import (
	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/atrahy/easypg/internal/sql"
	"github.com/atrahy/easypg/internal/tui/components/search"
	"github.com/atrahy/easypg/internal/tui/components/tableLayout"
	"github.com/atrahy/easypg/internal/tui/components/tabs"
	"github.com/atrahy/easypg/internal/tui/keys"
)

const (
	tabTable    = "Table"
	tabView     = "View"
	tabFunction = "Function"
)

// Selection identifies the currently highlighted object, whatever its kind.
type Selection struct {
	OID        string
	Schema     string
	Name       string
	Kind       string // Table.Type value, or "function"
	IsFunction bool
}

// ObjectsPane is the bottom-left navigation pane: an internal Table/View/Function
// tab strip — drawn by the caller in the pane's border — over a single list
// showing the objects of the active tab. It owns
// its own table and a unified selection model covering the three kinds, and
// emits ObjectSelectedMsg whenever the selection changes.
type ObjectsPane struct {
	tabs   *tabs.Model
	table  table.Model
	width  int
	filter search.TableFilter

	tables  []sql.Table
	views   []sql.Table
	funcs   []sql.Function
	current []Selection // parallel to the displayed rows
}

func New() *ObjectsPane {
	t := table.New(
		table.WithColumns(getColumns(0, tabTable)),
		table.WithRows([]table.Row{}),
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

	return &ObjectsPane{
		tabs:  tabs.New(tabTable, tabView, tabFunction),
		table: t,
	}
}

// SetTables partitions rows by Table.Type into the Table and View tabs.
func (p *ObjectsPane) SetTables(rows []sql.Table) tea.Cmd {
	p.tables = p.tables[:0]
	p.views = p.views[:0]

	for _, r := range rows {
		switch r.Type {
		case "table", "partitioned table":
			p.tables = append(p.tables, r)
		case "view", "materialized view":
			p.views = append(p.views, r)
		}
	}

	return p.loadActive()
}

// SetFunctions stores the schema's functions; it only reselects (and thus
// triggers a detail refresh) when the Function tab is the active one.
func (p *ObjectsPane) SetFunctions(rows []sql.Function) tea.Cmd {
	p.funcs = rows

	if p.tabs.ActiveLabel() == tabFunction {
		return p.loadActive()
	}

	return nil
}

// ResetFunctions drops functions from a previous schema so the Function tab
// never shows stale objects before the new schema's functions are fetched.
func (p *ObjectsPane) ResetFunctions() {
	p.funcs = nil
}

// ActiveTabIsFunction reports whether the Function tab is currently selected,
// used by the caller to lazily fetch functions only when needed.
func (p *ObjectsPane) ActiveTabIsFunction() bool {
	return p.tabs.ActiveLabel() == tabFunction
}

func (p *ObjectsPane) loadActive() tea.Cmd {
	p.current = p.current[:0]

	var rows []table.Row

	switch p.tabs.ActiveLabel() {
	case tabFunction:
		for _, f := range p.funcs {
			p.current = append(p.current, Selection{OID: f.OID, Schema: f.Schema, Name: f.Name, Kind: "function", IsFunction: true})
			rows = append(rows, table.Row{f.Name, f.Arguments})
		}
	case tabView:
		for _, v := range p.views {
			p.current = append(p.current, Selection{OID: v.OID, Schema: v.Schema, Name: v.Name, Kind: v.Type})
			rows = append(rows, table.Row{v.Name, v.Type})
		}
	default:
		for _, t := range p.tables {
			p.current = append(p.current, Selection{OID: t.OID, Schema: t.Schema, Name: t.Name, Kind: t.Type})
			rows = append(rows, table.Row{t.Name, t.Type})
		}
	}

	p.table.SetColumns(getColumns(p.width, p.tabs.ActiveLabel()))
	p.table.SetRows(rows)
	p.table.SetCursor(0)
	p.filter.Reset()

	return objectSelectedEvent
}

func (p *ObjectsPane) Filter(query string) int {
	return p.filter.Apply(&p.table, query)
}

func (p *ObjectsPane) ClearFilter() {
	p.filter.Clear(&p.table)
}

func (p *ObjectsPane) Filtering() bool {
	return p.filter.Active()
}

// Position is the cursor's place in the active tab's list, for the pane's border
// indicator.
func (p *ObjectsPane) Position() (current, total int) {
	return tableLayout.Position(p.table)
}

// SelectionEvent re-emits the selection message, for the moves that do not go
// through Update (a search jumping the cursor, a refresh).
func (p *ObjectsPane) SelectionEvent() tea.Cmd {
	return objectSelectedEvent
}

func (p *ObjectsPane) NextTab() tea.Cmd {
	p.tabs.Next()
	return p.loadActive()
}

func (p *ObjectsPane) PrevTab() tea.Cmd {
	p.tabs.Prev()
	return p.loadActive()
}

func (p *ObjectsPane) Update(msg tea.Msg) tea.Cmd {
	before := p.table.Cursor()

	p.table, _ = p.table.Update(msg)

	if p.table.Cursor() != before {
		return objectSelectedEvent
	}

	return nil
}

// GetSelection returns the highlighted object, or ok=false when the active tab
// has no objects.
func (p *ObjectsPane) GetSelection() (Selection, bool) {
	cursor := p.filter.SourceIndex(p.table.Cursor())
	if cursor < 0 || cursor >= len(p.current) {
		return Selection{}, false
	}

	return p.current[cursor], true
}

func (p *ObjectsPane) View() string {
	return p.table.View()
}

// Tabs is the pane's tab strip, which it does not draw itself: it is rendered in
// the pane's border (see paneBox), where it costs no row of the list.
func (p *ObjectsPane) Tabs() (labels []string, active int) {
	return p.tabs.Visible()
}

func (p *ObjectsPane) SetSize(width, height int) {
	p.width = width

	p.table.SetWidth(width)
	p.table.SetHeight(max(height, 0))
	p.table.SetColumns(getColumns(width, p.tabs.ActiveLabel()))
}

// getColumns keeps the object name as wide as possible; the second column shows
// the kind for tables/views and the (much longer) signature for functions, so it
// only claims a real share of the width on the Function tab.
func getColumns(width int, activeTab string) []table.Column {
	info := tableLayout.Spec{Title: "Type", Min: 8, Weight: 1}
	if activeTab == tabFunction {
		info = tableLayout.Spec{Title: "Arguments", Min: 10, Weight: 2}
	}

	return tableLayout.Fit(width, []tableLayout.Spec{
		{Title: "Name", Min: 12, Weight: 3},
		info,
	})
}
