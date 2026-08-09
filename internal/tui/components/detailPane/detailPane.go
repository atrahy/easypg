package detailPane

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/atrahy/easypg/internal/sql"
	"github.com/atrahy/easypg/internal/tui/components/columnTile"
	"github.com/atrahy/easypg/internal/tui/components/constraintTile"
	"github.com/atrahy/easypg/internal/tui/components/indexTile"
	"github.com/atrahy/easypg/internal/tui/components/search"
	"github.com/atrahy/easypg/internal/tui/components/sqlTile"
	"github.com/atrahy/easypg/internal/tui/components/tabs"
	"github.com/atrahy/easypg/internal/tui/keys"
	"github.com/charmbracelet/x/ansi"
)

const (
	tabColumn     = "Column"
	tabIndex      = "Index"
	tabConstraint = "Constraints"
	tabSQL        = "SQL"
)

const (
	// inspectorLines is how many wrapped lines the inspector strip shows of the
	// selected row; +1 for its top border.
	inspectorLines  = 3
	inspectorHeight = inspectorLines + 1

	// wrapBreakpoints are the SQL punctuation characters the inspector may wrap
	// on, on top of spaces.
	wrapBreakpoints = ",()"
)

// DetailPane is the right-hand pane: an internal Column/Index/Constraints/SQL
// tab strip — drawn by the caller in the pane's border — over the matching tile.
// The visible tabs adapt to the object type (a view exposes columns + SQL, a
// function only SQL).
//
// Table cells are necessarily truncated, so the tabular tabs sit above an
// inspector strip spelling out the selected row in full (a whole CREATE INDEX
// statement, a FOREIGN KEY clause, a long default expression); it can be folded
// away with "i" to give its height back to the list. The SQL tab scrolls its own
// content instead and always gets the full height.
type DetailPane struct {
	tabs        *tabs.Model
	columns     *columnTile.Model
	indexes     *indexTile.Model
	constraints *constraintTile.Model
	sqlView     *sqlTile.Model

	width, height int

	// inspectorOpen is the (sticky) fold state of the inspector strip.
	inspectorOpen bool

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

		inspectorOpen: true,
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
	if p.isInspectorToggle(msg) {
		p.inspectorOpen = !p.inspectorOpen
		p.updateSize()

		return nil
	}

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
	if p.tabs.ActiveLabel() == tabSQL {
		return p.sqlView.View()
	}

	var body, detail string

	switch p.tabs.ActiveLabel() {
	case tabIndex:
		body, detail = p.indexes.View(), p.indexes.SelectedDetail()
	case tabConstraint:
		body, detail = p.constraints.View(), p.constraints.SelectedDetail()
	default:
		body, detail = p.columns.View(), p.columns.SelectedDetail()
	}

	if !p.inspectorOpen {
		return body
	}

	return lipgloss.JoinVertical(lipgloss.Left, body, p.inspectorView(detail))
}

// Tabs is the pane's tab strip, which it does not draw itself: it is rendered in
// the pane's border (see paneBox), where it costs no row of the tile.
func (p *DetailPane) Tabs() (labels []string, active int) {
	return p.tabs.Visible()
}

// isInspectorToggle reports whether msg is the fold key, pressed on a tab that
// actually has an inspector strip (the key stays inert on the SQL tab rather
// than silently flipping an invisible state).
func (p *DetailPane) isInspectorToggle(msg tea.Msg) bool {
	keyMsg, ok := msg.(tea.KeyPressMsg)

	return ok && key.Matches(keyMsg, keys.Default.Inspector) && p.tabs.ActiveLabel() != tabSQL
}

// InspectorOpen is the fold state of the inspector strip, for the hint line —
// which says what "i" will do next rather than naming the toggle.
func (p *DetailPane) InspectorOpen() bool {
	return p.inspectorOpen
}

// ActiveTab is the label of the visible sub-tab, for the contextual help.
func (p *DetailPane) ActiveTab() string {
	return p.tabs.ActiveLabel()
}

// ActiveTabIsText reports whether the visible tile is a text view (the SQL tab),
// which supports wrapping and horizontal scrolling.
func (p *DetailPane) ActiveTabIsText() bool {
	return p.tabs.ActiveLabel() == tabSQL
}

// ActiveTabIsList reports whether the visible tile is a row list, which can be
// filtered.
func (p *DetailPane) ActiveTabIsList() bool {
	return !p.ActiveTabIsText()
}

// CopyValue is what "y" puts in the clipboard: the whole SQL on the SQL tab, the
// selected row's full text otherwise.
func (p *DetailPane) CopyValue() string {
	switch p.tabs.ActiveLabel() {
	case tabSQL:
		return p.sqlView.Content()
	case tabIndex:
		return p.indexes.SelectedDetail()
	case tabConstraint:
		return p.constraints.SelectedDetail()
	default:
		return p.columns.SelectedDetail()
	}
}

// Position is the active tile's cursor over its row count, for the pane's border
// indicator. The SQL tab reports nothing: it has no cursor, only a scroll
// offset, and the tile already spells that out as a "↕ %" in its status line.
func (p *DetailPane) Position() (current, total int) {
	switch p.tabs.ActiveLabel() {
	case tabIndex:
		return p.indexes.Position()
	case tabConstraint:
		return p.constraints.Position()
	case tabSQL:
		return 0, 0
	default:
		return p.columns.Position()
	}
}

// SelectedName is the highlighted row's name, for the status bar's context path.
// The SQL tab has no rows, so it names nothing.
func (p *DetailPane) SelectedName() string {
	switch p.tabs.ActiveLabel() {
	case tabIndex:
		return p.indexes.SelectedName()
	case tabConstraint:
		return p.constraints.SelectedName()
	case tabSQL:
		return ""
	default:
		return p.columns.SelectedName()
	}
}

// Progress is how far into the active tile we are, as a ratio: the cursor's rank
// on a list, the scroll on the SQL tab. ok is false when there is nothing to be
// positioned in, so the caller can leave the indicator out.
func (p *DetailPane) Progress() (ratio float64, ok bool) {
	if p.tabs.ActiveLabel() == tabSQL {
		if p.sqlView.Content() == "" {
			return 0, false
		}

		return p.sqlView.ScrollPercent(), true
	}

	current, total := p.Position()
	if total == 0 {
		return 0, false
	}

	return float64(current) / float64(total), true
}

// Wrap, CanScrollHorizontally and HorizontalScrollPercent describe the SQL tile
// for the tab's status bar and its key hints; they are meaningless on the
// tabular tabs, which the caller checks with ActiveTabIsText.
func (p *DetailPane) Wrap() bool {
	return p.sqlView.Wrap()
}

func (p *DetailPane) CanScrollHorizontally() bool {
	return p.sqlView.CanScrollHorizontally()
}

func (p *DetailPane) HorizontalScrollPercent() float64 {
	return p.sqlView.HorizontalScrollPercent()
}

// Searching only ever applies to the SQL tab: the tabular tabs are filtered
// instead. The caller checks ActiveTabIsText before driving these.
func (p *DetailPane) Search(query string) int {
	return p.sqlView.Search(query)
}

func (p *DetailPane) CancelSearch() {
	p.sqlView.CancelSearch()
}

func (p *DetailPane) NextMatch() {
	p.sqlView.NextMatch()
}

func (p *DetailPane) PrevMatch() {
	p.sqlView.PrevMatch()
}

func (p *DetailPane) MatchPosition() (current, total int) {
	return p.sqlView.MatchPosition()
}

// activeFilterable is nil on the SQL tab: hiding lines of a statement would make
// it unreadable, so only the row lists are filterable.
func (p *DetailPane) activeFilterable() search.Filterable {
	switch p.tabs.ActiveLabel() {
	case tabIndex:
		return p.indexes
	case tabConstraint:
		return p.constraints
	case tabSQL:
		return nil
	default:
		return p.columns
	}
}

func (p *DetailPane) Filter(query string) int {
	if filterable := p.activeFilterable(); filterable != nil {
		return filterable.Filter(query)
	}

	return 0
}

func (p *DetailPane) ClearFilter() {
	if filterable := p.activeFilterable(); filterable != nil {
		filterable.ClearFilter()
	}
}

func (p *DetailPane) Filtering() bool {
	filterable := p.activeFilterable()

	return filterable != nil && filterable.Filtering()
}

// inspectorView renders the selected row's full text under a separator, wrapped
// over at most inspectorLines lines.
func (p *DetailPane) inspectorView(detail string) string {
	return lipgloss.NewStyle().
		Width(p.width).
		Height(inspectorLines).
		Border(lipgloss.NormalBorder(), true, false, false, false).
		BorderForeground(lipgloss.Color("240")).
		Foreground(lipgloss.Color("245")).
		Render(wrapClamp(detail, p.width, inspectorLines))
}

func (p *DetailPane) SetSize(width, height int) {
	p.width, p.height = width, height
	p.updateSize()
}

// updateSize splits the pane height between the active tile and (for the tabular
// tabs only, when unfolded) the inspector strip. The tab strip takes nothing: it
// is drawn in the pane's border.
func (p *DetailPane) updateSize() {
	strip := 0
	if p.inspectorOpen {
		strip = inspectorHeight
	}

	tileHeight := max(p.height-strip, 0)

	p.columns.SetSize(p.width, tileHeight)
	p.indexes.SetSize(p.width, tileHeight)
	p.constraints.SetSize(p.width, tileHeight)
	p.sqlView.SetSize(p.width, max(p.height, 0))
}

// wrapClamp word-wraps text to width and keeps at most maxLines lines, marking
// the cut with an ellipsis. lipgloss pads a too-short block to the box height
// but never trims a too-long one, hence the manual clamp.
func wrapClamp(text string, width, maxLines int) string {
	if text == "" || width < 1 {
		return ""
	}

	lines := strings.Split(ansi.Wrap(text, width, wrapBreakpoints), "\n")
	if len(lines) <= maxLines {
		return strings.Join(lines, "\n")
	}

	lines = lines[:maxLines]
	lines[maxLines-1] = ansi.Truncate(lines[maxLines-1], max(width-1, 1), "") + "…"

	return strings.Join(lines, "\n")
}
