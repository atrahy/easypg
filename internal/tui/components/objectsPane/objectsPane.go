package objectsPane

import (
	"github.com/atrahy/easypg/internal/sql"
	"github.com/atrahy/easypg/internal/tui/components/tabs"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
// tab strip over a single list showing the objects of the active tab. It owns
// its own table and a unified selection model covering the three kinds, and
// emits ObjectSelectedMsg whenever the selection changes.
type ObjectsPane struct {
	tabs  *tabs.Model
	table table.Model
	width int

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
	cursor := p.table.Cursor()
	if cursor < 0 || cursor >= len(p.current) {
		return Selection{}, false
	}

	return p.current[cursor], true
}

func (p *ObjectsPane) View() string {
	return lipgloss.JoinVertical(lipgloss.Left, p.tabs.View(), p.table.View())
}

func (p *ObjectsPane) SetSize(width, height int) {
	p.width = width

	listHeight := height - 1 // reserve one line for the tab header
	if listHeight < 0 {
		listHeight = 0
	}

	p.table.SetWidth(width)
	p.table.SetHeight(listHeight)
	p.table.SetColumns(getColumns(width, p.tabs.ActiveLabel()))
}

func getColumns(width int, activeTab string) []table.Column {
	if width <= 0 {
		// Fallback until the first SetSize so columns are never zero-width.
		width = 40
	}

	infoTitle := "Type"
	infoWidth := 18

	if activeTab == tabFunction {
		infoTitle = "Arguments"
		infoWidth = 30
	}

	nameWidth := width - infoWidth
	if nameWidth < 0 {
		nameWidth = 0
	}

	return []table.Column{
		{Title: "Name", Width: nameWidth},
		{Title: infoTitle, Width: infoWidth},
	}
}
