package detailPane

import (
	"strings"

	"github.com/atrahy/easypg/internal/sql"
	"github.com/atrahy/easypg/internal/tui/components/columnTile"
	"github.com/atrahy/easypg/internal/tui/components/constraintTile"
	"github.com/atrahy/easypg/internal/tui/components/indexTile"
	"github.com/atrahy/easypg/internal/tui/components/sqlTile"
	"github.com/atrahy/easypg/internal/tui/components/tabs"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	tabColumn     = "Column"
	tabIndex      = "Index"
	tabConstraint = "Constraints"
	tabSQL        = "SQL"
)

// DetailPane is the right-hand pane: an internal Column/Index/Constraints/SQL
// tab strip over the matching tile. The visible tabs adapt to the object type
// (a view exposes columns + SQL, a function only SQL).
type DetailPane struct {
	tabs        *tabs.Model
	columns     *columnTile.Model
	indexes     *indexTile.Model
	constraints *constraintTile.Model
	sqlView     *sqlTile.Model

	// cameFromFunction remembers that the previous content was a function (SQL
	// only), so the next table/view selection resets the active tab to Column
	// instead of staying on the SQL tab.
	cameFromFunction bool
}

func New() *DetailPane {
	return &DetailPane{
		tabs:        tabs.New(tabColumn, tabIndex, tabConstraint, tabSQL),
		columns:     columnTile.New(),
		indexes:     indexTile.New(),
		constraints: constraintTile.New(),
		sqlView:     sqlTile.New(),
	}
}

func (p *DetailPane) SetItems(attr *sql.TableAttr, objType string) {
	if attr == nil {
		p.columns.SetItems(nil)
		p.indexes.SetItems(nil)
		p.constraints.SetItems(nil)
		p.sqlView.SetContent("")
		return
	}

	p.columns.SetItems(attr.Columns)
	p.indexes.SetItems(attr.Indexes)
	p.constraints.SetItems(attr.Constraints)
	p.sqlView.SetContent(attr.DDL)

	if strings.Contains(objType, "view") {
		p.tabs.SetVisible([]string{tabColumn, tabSQL})
	} else {
		p.tabs.SetVisible([]string{tabColumn, tabIndex, tabConstraint, tabSQL})
	}

	// Coming back from a function view (SQL-only) would otherwise leave the
	// active tab stuck on SQL; reset to the first tab for a fresh object.
	if p.cameFromFunction {
		p.tabs.First()
		p.cameFromFunction = false
	}
}

// SetFunctionDef shows a function's definition: only the SQL tab is relevant.
func (p *DetailPane) SetFunctionDef(def string) {
	p.columns.SetItems(nil)
	p.indexes.SetItems(nil)
	p.constraints.SetItems(nil)
	p.sqlView.SetContent(def)
	p.tabs.SetVisible([]string{tabSQL})
	p.cameFromFunction = true
}

func (p *DetailPane) NextTab() {
	p.tabs.Next()
}

func (p *DetailPane) PrevTab() {
	p.tabs.Prev()
}

func (p *DetailPane) Update(msg tea.Msg) tea.Cmd {
	switch p.tabs.ActiveLabel() {
	case tabIndex:
		return p.indexes.Update(msg)
	case tabConstraint:
		return p.constraints.Update(msg)
	case tabSQL:
		return p.sqlView.Update(msg)
	default:
		return p.columns.Update(msg)
	}
}

func (p *DetailPane) View() string {
	var body string

	switch p.tabs.ActiveLabel() {
	case tabIndex:
		body = p.indexes.View()
	case tabConstraint:
		body = p.constraints.View()
	case tabSQL:
		body = p.sqlView.View()
	default:
		body = p.columns.View()
	}

	return lipgloss.JoinVertical(lipgloss.Left, p.tabs.View(), body)
}

func (p *DetailPane) SetSize(width, height int) {
	h := height - 1 // reserve one line for the tab header
	if h < 0 {
		h = 0
	}

	p.columns.SetSize(width, h)
	p.indexes.SetSize(width, h)
	p.constraints.SetSize(width, h)
	p.sqlView.SetSize(width, h)
}
