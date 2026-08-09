package tui

import (
	"fmt"
	"log"

	"github.com/atotto/clipboard"
	"github.com/atrahy/easypg/internal/sql"
	"github.com/atrahy/easypg/internal/tui/components/detailPane"
	"github.com/atrahy/easypg/internal/tui/components/helpPane"
	"github.com/atrahy/easypg/internal/tui/components/objectsPane"
	"github.com/atrahy/easypg/internal/tui/components/overlay"
	"github.com/atrahy/easypg/internal/tui/components/paneBox"
	"github.com/atrahy/easypg/internal/tui/components/schemaTable"
	"github.com/atrahy/easypg/internal/tui/components/search"
	"github.com/atrahy/easypg/internal/tui/components/searchBar"
	"github.com/atrahy/easypg/internal/tui/keys"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// schemaVisibleRows is the (content) row count of the compact schema pane in the
// left navigation column; the objects pane takes the remaining height.
const schemaVisibleRows = 6

// promptBlockWidth is how much of the footer the mode block takes while a prompt
// is open — "SEARCH" and "FILTER" are both 6 cells, plus the block's padding and
// the gap before the prompt. The text input is sized against it so the footer
// never wraps onto a second line.
const promptBlockWidth = 6 + 2 + 1

// pane identifies a focusable pane. Focus cycles through them in this order —
// tab/shift+tab, and h/l (or the arrows) as aliases — while 1/2/3 jump straight
// to one. Cycling rather than moving geometrically means no key press is ever a
// no-op, which matters with the schema and objects panes stacked in one column.
type pane int

const (
	paneSchema pane = iota
	paneObjects
	paneDetail
)

var paneNames = [...]string{"Schema", "Objects", "Detail"}

// paneShortcuts is the key each pane advertises as a "[n]" badge in its top
// border. Read from the keymap rather than hardcoded, so the badge can never
// promise a key that does something else.
var paneShortcuts = [...]key.Binding{
	keys.Default.PaneSchema,
	keys.Default.PaneObjects,
	keys.Default.PaneDetail,
}

// mode is the *prompt* state: while one is open it owns the keyboard, so no key
// may reach the panes or the global handler. What "/" opens depends on the
// focused pane — a filter on a list or the help, a search on a text view; see
// docs/spec/05-keybindings.md.
//
// The states that outlive the prompt (a filter still hiding rows, a search still
// holding matches) are not modes here: they belong to the pane, and the mode
// block derives them from it so switching focus can never show a stale one.
type mode int

const (
	modeNormal mode = iota
	modeSearch
	modeFilter
)

type definitionTabModel struct {
	width, height int

	leftWidth, rightWidth int
	schemaHeight          int
	objectsHeight         int
	detailHeight          int
	zoomWidth, zoomHeight int

	focus pane

	mode     mode
	helpOpen bool
	zoomed   bool

	lastErr error
	// notice is a transient confirmation (a copy landing), cleared on the next
	// key press.
	notice string

	// currentSchema is the schema whose objects are shown; functionsFetched
	// tracks whether its functions have been lazily loaded yet (they are only
	// fetched when the Function tab is opened, not on every schema move).
	currentSchema    string
	functionsFetched bool

	schemaTile  *schemaTable.SchemaTable
	objectsPane *objectsPane.ObjectsPane
	detailPane  *detailPane.DetailPane
	helpPane    *helpPane.Model
	searchBar   *searchBar.Model

	db *sql.DBConnection
}

func newDefinitionTabPage(db *sql.DBConnection) definitionTabModel {
	return definitionTabModel{
		db: db,

		schemaTile:  schemaTable.NewSchemaTable(),
		objectsPane: objectsPane.New(),
		detailPane:  detailPane.New(),
		helpPane:    helpPane.New(),
		searchBar:   searchBar.New(),
	}
}

func (t definitionTabModel) Init() tea.Cmd {
	return t.fetchNamespaces
}

// CapturesInput tells the root model that this tab is swallowing every key (a
// search being typed, or the help overlay being open), so its global q/quit
// handler must stand down — otherwise typing "q" in the prompt would quit.
func (t definitionTabModel) CapturesInput() bool {
	return t.mode != modeNormal || t.helpOpen
}

// Panels drill into each other via a cursor/selection -> fetch -> result -> SetItems chain:
//
//	schemaTable.SchemaCursorUpdateMsg -> fetchTables/fetchFunctions -> tablesList/functionsList -> objectsPane.Set{Tables,Functions}
//	objectsPane.ObjectSelectedMsg     -> fetchTableAttr/fetchFunctionDef -> tableAttr/functionDef -> detailPane.Set{Items,FunctionDef}
//
// Add new drill-down levels by extending this same chain.
func (t definitionTabModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	log.Printf("msg received: %T : %v", msg, msg)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		t.width, t.height = msg.Width, msg.Height
		t.updateSize()

	case tea.KeyMsg:
		return t.handleKey(msg)

	case schemaList:
		t.lastErr = nil
		cmds = append(cmds, t.schemaTile.SetItems(msg.schemas))

	case schemaTable.SchemaCursorUpdateMsg:
		schema := t.schemaTile.GetSelectedItemName()
		log.Printf("schemaCursorUpdate: name :%s", schema)

		t.currentSchema = schema
		t.functionsFetched = false
		t.objectsPane.ResetFunctions()

		cmds = append(cmds, t.fetchTables(schema))
		// Functions are fetched lazily; only load them now if the Function tab
		// is already the active one.
		cmds = append(cmds, t.maybeFetchFunctions())

	case tablesList:
		t.lastErr = nil
		cmds = append(cmds, t.objectsPane.SetTables(msg.tables))

	case functionsList:
		t.lastErr = nil
		cmds = append(cmds, t.objectsPane.SetFunctions(msg.functions))

	case objectsPane.ObjectSelectedMsg:
		sel, ok := t.objectsPane.GetSelection()
		if !ok {
			log.Printf("objectSelected: nothing selectable, clearing detail")
			t.detailPane.SetItems(nil, "")
			break
		}

		log.Printf("objectSelected: oid=%s kind=%s func=%t", sel.OID, sel.Kind, sel.IsFunction)
		if sel.IsFunction {
			cmds = append(cmds, t.fetchFunctionDef(sel.OID))
		} else {
			cmds = append(cmds, t.fetchTableAttr(sel))
		}

	case tableAttr:
		t.lastErr = nil
		t.detailPane.SetItems(msg.tableAttr, msg.objType)

	case functionDef:
		t.lastErr = nil
		t.detailPane.SetFunctionDef(msg.def)

	case fetchErrMsg:
		log.Printf("fetch error: %v", msg.err)
		t.lastErr = msg.err
	}

	return t, tea.Batch(cmds...)
}

// handleKey routes a key press by mode: the search prompt first, then the help
// overlay, then the normal bindings. Nothing falls through between them.
func (t definitionTabModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	t.notice = ""

	switch {
	case t.mode != modeNormal:
		return t.handlePromptKey(msg)
	case t.helpOpen:
		return t.handleHelpKey(msg)
	default:
		return t.handleNormalKey(msg)
	}
}

func (t definitionTabModel) handleNormalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Default.Help):
		t.openHelp()

	case key.Matches(msg, keys.Default.Search):
		// The command is built before returning: a return statement's operands
		// are not guaranteed to be evaluated before the model is copied.
		cmd := t.startPromptForFocus()

		return t, cmd

	case key.Matches(msg, keys.Default.Quit, keys.Default.ForceQuit):
		// Only reachable when replayed from the help overlay: the root model
		// handles these keys before routing when they are actually typed.
		return t, tea.Quit

	case key.Matches(msg, keys.Default.Cancel):
		// One escape for everything that can be "on": a zoom, a confirmed
		// search, an active filter.
		t.clearSearch()
		t.clearFilter()

		if t.zoomed {
			t.zoomed = false
			t.updateSize()
		}

		return t, t.selectionEvent()

	case key.Matches(msg, keys.Default.NextMatch):
		if target := t.searchTarget(); target != nil {
			target.NextMatch()
		}

	case key.Matches(msg, keys.Default.PrevMatch):
		if target := t.searchTarget(); target != nil {
			target.PrevMatch()
		}

	case key.Matches(msg, keys.Default.NextPane):
		t.setFocus((t.focus + 1) % pane(len(paneNames)))

	case key.Matches(msg, keys.Default.PrevPane):
		t.setFocus((t.focus - 1 + pane(len(paneNames))) % pane(len(paneNames)))

	case key.Matches(msg, keys.Default.PaneSchema):
		t.setFocus(paneSchema)

	case key.Matches(msg, keys.Default.PaneObjects):
		t.setFocus(paneObjects)

	case key.Matches(msg, keys.Default.PaneDetail):
		t.setFocus(paneDetail)

	case key.Matches(msg, keys.Default.NextTab):
		cmd := t.focusedNextInternalTab()

		return t, cmd

	case key.Matches(msg, keys.Default.PrevTab):
		cmd := t.focusedPrevInternalTab()

		return t, cmd

	case key.Matches(msg, keys.Default.Zoom):
		t.zoomed = !t.zoomed
		t.updateSize()

	case key.Matches(msg, keys.Default.Copy):
		t.copySelection()

	case key.Matches(msg, keys.Default.Refresh):
		cmd := t.refreshFocused()

		return t, cmd

	case key.Matches(msg, keys.Default.RefreshAll):
		return t, t.fetchNamespaces

	default:
		return t, t.updateFocusedPane(msg)
	}

	return t, nil
}

// handlePromptKey feeds the "/" or "f" prompt and re-applies it on every
// keystroke. The cascading fetches (a schema move reloading the objects) are
// deliberately *not* triggered while typing — only on confirm/cancel — so an
// incremental search does not fire a query per character.
func (t definitionTabModel) handlePromptKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	filtering := t.mode == modeFilter

	switch {
	case key.Matches(msg, keys.Default.AcceptSearch):
		t.mode = modeNormal
		t.searchBar.Stop()

		// Confirming an empty prompt is the same as never having opened it.
		if t.searchBar.Value() == "" {
			if filtering {
				t.clearFilter()
			} else {
				t.clearSearch()
			}
		}

		return t, t.selectionEvent()

	case key.Matches(msg, keys.Default.Cancel):
		t.mode = modeNormal
		t.searchBar.Stop()

		if filtering {
			t.clearFilter()
		} else {
			t.clearSearch()
		}

		return t, t.selectionEvent()
	}

	cmd := t.searchBar.Update(msg)
	query := t.searchBar.Value()

	if filtering {
		if target := t.filterTarget(); target != nil {
			t.searchBar.SetStatus(filterStatus(target.Filter(query), query))
		}

		return t, cmd
	}

	if target := t.searchTarget(); target != nil {
		target.Search(query)
		t.searchBar.SetStatus(matchStatus(target, query))
	}

	return t, cmd
}

// matchStatus is the right-hand indicator of the search prompt.
func matchStatus(target search.Searchable, query string) string {
	if query == "" {
		return ""
	}

	current, total := target.MatchPosition()
	if total == 0 {
		return "no match"
	}

	return fmt.Sprintf("%d/%d", current, total)
}

func filterStatus(rows int, query string) string {
	if query == "" {
		return ""
	}

	if rows == 0 {
		return "no match"
	}

	return fmt.Sprintf("%d rows", rows)
}

// clearSearch / clearFilter undo a confirmed search or filter, restoring what
// the pane showed before it. The "is one active?" state lives in the panes
// themselves, so switching focus can never leave a stale indicator behind.
func (t *definitionTabModel) clearSearch() {
	if target := t.searchTarget(); target != nil {
		target.CancelSearch()
	}
}

func (t *definitionTabModel) clearFilter() {
	if target := t.filterTarget(); target != nil {
		target.ClearFilter()
	}
}

// handleHelpKey drives the overlay: it closes on esc/q/?, runs the highlighted
// binding on enter, and otherwise scrolls or filters its own list.
func (t definitionTabModel) handleHelpKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Default.Help, keys.Default.Cancel, keys.Default.Quit):
		t.closeHelp()

		return t, nil

	case key.Matches(msg, keys.Default.AcceptSearch):
		return t.runSelectedBinding()

	case key.Matches(msg, keys.Default.Search):
		// The help is a list, so "/" narrows it — same rule as everywhere else.
		cmd := t.startPromptForFocus()

		return t, cmd
	}

	return t, t.helpPane.Update(msg)
}

// runSelectedBinding closes the overlay and replays the highlighted binding's
// key, so the help executes commands through the very handlers it documents
// instead of a parallel dispatch table.
func (t definitionTabModel) runSelectedBinding() (tea.Model, tea.Cmd) {
	binding, ok := t.helpPane.SelectedBinding()
	if !ok {
		return t, nil
	}

	t.closeHelp()

	replay, ok := keys.Synthesize(binding)
	if !ok {
		return t, nil
	}

	return t.handleNormalKey(replay)
}

func (t *definitionTabModel) closeHelp() {
	t.helpPane.ClearFilter()
	t.helpOpen = false
}

func (t *definitionTabModel) openHelp() {
	t.helpPane.SetSections(keys.Default.FullHelp(t.helpContext()))
	t.helpPane.SetSize(t.width, t.height-1)
	t.helpOpen = true
}

// startPromptForFocus is what "/" does: it opens the prompt that fits what is
// focused — a filter on a row list or the help, a search on a text view.
func (t *definitionTabModel) startPromptForFocus() tea.Cmd {
	if t.searchTarget() != nil {
		return t.startPrompt(modeSearch)
	}

	if t.filterTarget() != nil {
		return t.startPrompt(modeFilter)
	}

	return nil
}

// startPrompt opens the footer prompt in search or filter mode.
func (t *definitionTabModel) startPrompt(target mode) tea.Cmd {
	t.mode = target
	t.searchBar.SetWidth(clampZero(t.width - promptBlockWidth))

	if target == modeFilter {
		return t.searchBar.Start("filter: ", "hide non-matching rows")
	}

	return t.searchBar.Start("/", "search, then n/N")
}

// helpContext describes what has focus so the overlay only lists applicable keys.
func (t definitionTabModel) helpContext() keys.Context {
	ctx := keys.Context{Pane: paneNames[t.focus]}

	switch t.focus {
	case paneObjects:
		ctx.HasTabs = true
	case paneDetail:
		ctx.HasTabs = true
		ctx.IsDetail = true
		ctx.IsText = t.detailPane.ActiveTabIsText()
	}

	ctx.IsList = t.focusedIsList()

	return ctx
}

// focusedIsList reports whether the focused pane shows rows (and can therefore
// be filtered) rather than text.
func (t definitionTabModel) focusedIsList() bool {
	if t.focus == paneDetail {
		return t.detailPane.ActiveTabIsList()
	}

	return true
}

// filterTarget is what a filter narrows: the overlay when it is open, otherwise
// the focused pane when it shows rows — nil on a text view, which cannot hide
// lines without becoming unreadable.
func (t definitionTabModel) filterTarget() search.Filterable {
	if t.helpOpen {
		return t.helpPane
	}

	switch t.focus {
	case paneObjects:
		return t.objectsPane
	case paneDetail:
		if t.detailPane.ActiveTabIsList() {
			return t.detailPane
		}

		return nil
	default:
		return t.schemaTile
	}
}

func (t *definitionTabModel) setFocus(target pane) {
	t.focus = target

	// A zoomed pane must resize when the focus moves to another one.
	if t.zoomed {
		t.updateSize()
	}
}

// searchTarget is what a search drives, and nil when the focused pane is not a
// text view — a row list is filtered instead, and the help is a list too.
func (t definitionTabModel) searchTarget() search.Searchable {
	if t.helpOpen {
		return nil
	}

	if t.focus == paneDetail && t.detailPane.ActiveTabIsText() {
		return t.detailPane
	}

	return nil
}

func (t definitionTabModel) updateFocusedPane(msg tea.Msg) tea.Cmd {
	switch t.focus {
	case paneObjects:
		return t.objectsPane.Update(msg)
	case paneDetail:
		return t.detailPane.Update(msg)
	default:
		return t.schemaTile.Update(msg)
	}
}

// selectionEvent re-emits the focused pane's selection message, for the cursor
// moves that bypass Update (search jumps) and must still cascade downstream.
func (t definitionTabModel) selectionEvent() tea.Cmd {
	if t.helpOpen {
		return nil
	}

	switch t.focus {
	case paneSchema:
		return t.schemaTile.SelectionEvent()
	case paneObjects:
		return t.objectsPane.SelectionEvent()
	default:
		return nil
	}
}

// refreshFocused re-runs the query behind the focused pane; deeper panes follow
// through the usual cascade.
func (t *definitionTabModel) refreshFocused() tea.Cmd {
	switch t.focus {
	case paneObjects:
		t.functionsFetched = false
		return tea.Batch(t.fetchTables(t.currentSchema), t.maybeFetchFunctions())
	case paneDetail:
		return t.objectsPane.SelectionEvent()
	default:
		return t.fetchNamespaces
	}
}

// copySelection puts the focused pane's current value in the system clipboard.
func (t *definitionTabModel) copySelection() {
	var value string

	switch t.focus {
	case paneObjects:
		if sel, ok := t.objectsPane.GetSelection(); ok {
			value = sel.Schema + "." + sel.Name
		}
	case paneDetail:
		value = t.detailPane.CopyValue()
	default:
		value = t.schemaTile.GetSelectedItemName()
	}

	if value == "" {
		t.notice = "nothing to copy"
		return
	}

	if err := clipboard.WriteAll(value); err != nil {
		log.Printf("clipboard write failed: %v", err)
		t.lastErr = err

		return
	}

	t.notice = "copied to clipboard"
}

func (t definitionTabModel) View() string {
	body := t.layoutView()

	if t.helpOpen {
		body = overlay.Center(body, t.helpPane.View())
	}

	// The footer is clamped to exactly one line here rather than trusted to be
	// one: the root view wraps anything wider than the screen, and a footer
	// spilling onto a second line shifts the whole layout up.
	footer := lipgloss.NewStyle().MaxWidth(t.width).MaxHeight(1).Render(t.footerView())

	return lipgloss.JoinVertical(lipgloss.Left, body, footer)
}

// layoutView draws the three panes side by side, or the focused one alone when
// zoomed.
func (t definitionTabModel) layoutView() string {
	if t.zoomed {
		return t.boxFor(t.focus, t.zoomWidth, t.zoomHeight).Render(t.focusedView())
	}

	left := lipgloss.JoinVertical(
		lipgloss.Top,
		t.boxFor(paneSchema, t.leftWidth, t.schemaHeight).Render(t.schemaTile.View()),
		t.boxFor(paneObjects, t.leftWidth, t.objectsHeight).Render(t.objectsPane.View()),
	)
	right := t.boxFor(paneDetail, t.rightWidth, t.detailHeight).Render(t.detailPane.View())

	return lipgloss.JoinHorizontal(lipgloss.Left, left, right)
}

func (t definitionTabModel) focusedView() string {
	switch t.focus {
	case paneObjects:
		return t.objectsPane.View()
	case paneDetail:
		return t.detailPane.View()
	default:
		return t.schemaTile.View()
	}
}

// footerView renders the status line: a vim-style mode block, then the search
// prompt, a transient notice, the last fetch error, or the focused pane's key
// hints. Always exactly one line, so the layout height stays stable.
func (t definitionTabModel) footerView() string {
	block := t.modeBlockView()

	return lipgloss.JoinHorizontal(
		lipgloss.Left,
		block,
		t.footerContent(clampZero(t.width-lipgloss.Width(block))),
	)
}

// modeBlockView is the vim-like indicator in the bottom-left corner. A confirmed
// search or filter gets its own state rather than falling back to NORMAL: the
// pane is still narrowed (or still holding matches for n/N), and that has to be
// visible. Those two are derived from the pane itself, so moving the focus can
// never leave a stale label behind.
//
// The block hugs its label, so what follows shifts when the mode changes; only
// the prompt is protected from that, since the two modes that show one are the
// same width.
func (t definitionTabModel) modeBlockView() string {
	label, color := "NORMAL", "63"

	switch {
	case t.mode == modeSearch:
		label, color = "SEARCH", "214"
	case t.mode == modeFilter:
		label, color = "FILTER", "42"
	case t.filterActive():
		label, color = "FILTERED", "42"
	case t.matchesActive():
		label, color = "MATCHES", "214"
	case t.helpOpen:
		label, color = "HELP", "205"
	}

	return lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("232")).
		Background(lipgloss.Color(color)).
		Padding(0, 1).
		Render(label)
}

func (t definitionTabModel) footerContent(width int) string {
	content, color := "", "240"

	switch {
	case t.mode != modeNormal:
		// Same leading gap as the other states: the prompt must not sit flush
		// against the block.
		return " " + t.searchBar.View()

	case t.notice != "":
		content, color = t.notice, "42"

	case t.lastErr != nil:
		content, color = "error: "+t.lastErr.Error(), "196"

	case t.searchState() != "":
		// The mode block says which state we are in; this says how to work with
		// it and how to leave it.
		content, color = t.searchState(), "214"
		if t.filterActive() {
			color = "42"
		}

	default:
		content = keys.RenderShort(keys.Default.ShortHelp(t.helpContext()))
	}

	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(color)).
		MaxWidth(width).
		Render(" " + content)
}


// filterActive / matchesActive report the states that outlive their prompt, read
// from the pane so they can never go stale.
func (t definitionTabModel) filterActive() bool {
	target := t.filterTarget()

	return target != nil && target.Filtering()
}

func (t definitionTabModel) matchesActive() bool {
	target := t.searchTarget()
	if target == nil {
		return false
	}

	_, total := target.MatchPosition()

	return total > 0
}

// searchState spells out what the mode block is showing, and how to get out of
// it.
func (t definitionTabModel) searchState() string {
	if t.matchesActive() {
		current, total := t.searchTarget().MatchPosition()

		return fmt.Sprintf("%d/%d  ·  n/N: next/prev  ·  esc: clear", current, total)
	}

	if t.filterActive() {
		return "rows hidden  ·  esc: clear filter"
	}

	return ""
}

// updateSize splits the screen into a left navigation column (schema stacked over
// objects) and a wider right detail column. Each pane draws a rounded border, so
// its content box is the total minus 2 in each dimension.
func (t *definitionTabModel) updateSize() {
	// One line is reserved at the bottom for the status/error/search footer.
	usableHeight := t.height - 1

	t.helpPane.SetSize(t.width, usableHeight)
	t.searchBar.SetWidth(clampZero(t.width - promptBlockWidth))

	if t.zoomed {
		t.zoomWidth = clampZero(t.width - 2)
		t.zoomHeight = clampZero(usableHeight - 2)

		switch t.focus {
		case paneObjects:
			t.objectsPane.SetSize(t.zoomWidth, t.zoomHeight)
		case paneDetail:
			t.detailPane.SetSize(t.zoomWidth, t.zoomHeight)
		default:
			t.schemaTile.SetSize(t.zoomWidth, t.zoomHeight)
		}

		return
	}

	leftTotal := t.width / 3
	rightTotal := t.width - leftTotal

	t.leftWidth = clampZero(leftTotal - 2)
	t.rightWidth = clampZero(rightTotal - 2)

	// +1 for the table's header row inside the compact schema pane.
	t.schemaHeight = schemaVisibleRows + 1
	// The two stacked bordered panes must fit the usable height:
	// (schemaHeight+2) + (objectsHeight+2) = usableHeight.
	t.objectsHeight = clampZero(usableHeight - 4 - t.schemaHeight)
	t.detailHeight = clampZero(usableHeight - 2)

	t.schemaTile.SetSize(t.leftWidth, t.schemaHeight)
	t.objectsPane.SetSize(t.leftWidth, t.objectsHeight)
	t.detailPane.SetSize(t.rightWidth, t.detailHeight)
}

// focusedNextInternalTab / focusedPrevInternalTab route the [ / ] keys to the
// focused pane's internal tab strip. Objects reload their list (hence a Cmd)
// and lazily fetch functions when the Function tab is opened; detail just
// switches the visible tile. Schema has no internal tabs.
func (t *definitionTabModel) focusedNextInternalTab() tea.Cmd {
	switch t.focus {
	case paneObjects:
		return tea.Batch(t.objectsPane.NextTab(), t.maybeFetchFunctions())
	case paneDetail:
		t.detailPane.NextTab()
	}

	return nil
}

func (t *definitionTabModel) focusedPrevInternalTab() tea.Cmd {
	switch t.focus {
	case paneObjects:
		return tea.Batch(t.objectsPane.PrevTab(), t.maybeFetchFunctions())
	case paneDetail:
		t.detailPane.PrevTab()
	}

	return nil
}

// maybeFetchFunctions loads the current schema's functions on demand: only when
// the Function tab is active and they have not been fetched yet for this schema.
func (t *definitionTabModel) maybeFetchFunctions() tea.Cmd {
	if !t.objectsPane.ActiveTabIsFunction() || t.functionsFetched {
		return nil
	}

	t.functionsFetched = true

	return t.fetchFunctions(t.currentSchema)
}

// boxFor is the frame of a pane: its shortcut and title in the top border, its
// cursor position in the bottom one. It costs the same 2 columns and 2 lines the
// plain border did, so the sizing above is unaffected.
func (t definitionTabModel) boxFor(target pane, width, height int) paneBox.Box {
	current, total := t.panePosition(target)
	labels, active := t.paneTabs(target)

	return paneBox.Box{
		Title:     paneNames[target],
		Context:   t.paneContext(target),
		Tabs:      labels,
		ActiveTab: active,
		Shortcut:  paneShortcuts[target].Help().Key,
		Current:   current,
		Total:     total,
		Width:     width,
		Height:    height,
		Focused:   target == t.focus,
	}
}

// paneTabs is the tab strip a pane draws in its border. The two panes that have
// one show it there instead of on a line of their own; the schema pane has none.
func (t definitionTabModel) paneTabs(target pane) (labels []string, active int) {
	switch target {
	case paneObjects:
		return t.objectsPane.Tabs()
	case paneDetail:
		return t.detailPane.Tabs()
	default:
		return nil, 0
	}
}

// paneContext qualifies the name of the pane that has no tabs to show instead:
// the schema pane says which schema everything below it is about.
func (t definitionTabModel) paneContext(target pane) string {
	if target == paneSchema {
		return t.currentSchema
	}

	return ""
}

// panePosition is the "x/total" a pane shows in its bottom border.
func (t definitionTabModel) panePosition(target pane) (current, total int) {
	switch target {
	case paneObjects:
		return t.objectsPane.Position()
	case paneDetail:
		return t.detailPane.Position()
	default:
		return t.schemaTile.Position()
	}
}

func clampZero(v int) int {
	if v < 0 {
		return 0
	}

	return v
}
