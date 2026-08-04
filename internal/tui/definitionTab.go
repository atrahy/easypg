package tui

import (
	"log"

	"github.com/atrahy/easypg/internal/sql"
	"github.com/atrahy/easypg/internal/tui/components/detailPane"
	"github.com/atrahy/easypg/internal/tui/components/objectsPane"
	"github.com/atrahy/easypg/internal/tui/components/schemaTable"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// schemaVisibleRows is the (content) row count of the compact schema pane in the
// left navigation column; the objects pane takes the remaining height.
const schemaVisibleRows = 6

var definitionTabPageTileList = []string{
	"schema",
	"objects",
	"detail",
}

type definitionTabModel struct {
	width, height int

	leftWidth, rightWidth int
	schemaHeight          int
	objectsHeight         int
	detailHeight          int

	focusedTileCursor int

	lastErr error

	// currentSchema is the schema whose objects are shown; functionsFetched
	// tracks whether its functions have been lazily loaded yet (they are only
	// fetched when the Function tab is opened, not on every schema move).
	currentSchema    string
	functionsFetched bool

	schemaTile  *schemaTable.SchemaTable
	objectsPane *objectsPane.ObjectsPane
	detailPane  *detailPane.DetailPane

	db *sql.DBConnection
}

func newDefinitionTabPage(db *sql.DBConnection) definitionTabModel {
	return definitionTabModel{
		db: db,

		schemaTile:  schemaTable.NewSchemaTable(),
		objectsPane: objectsPane.New(),
		detailPane:  detailPane.New(),
	}
}

func (t definitionTabModel) Init() tea.Cmd {
	return t.fetchNamespaces
}

// Panels drill into each other via a cursor/selection -> fetch -> result -> SetItems chain:
//   schemaTable.SchemaCursorUpdateMsg -> fetchTables/fetchFunctions -> tablesList/functionsList -> objectsPane.Set{Tables,Functions}
//   objectsPane.ObjectSelectedMsg     -> fetchTableAttr/fetchFunctionDef -> tableAttr/functionDef -> detailPane.Set{Items,FunctionDef}
// Add new drill-down levels by extending this same chain.
func (t definitionTabModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	log.Printf("msg received: %T : %v", msg, msg)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		t.width, t.height = msg.Width, msg.Height
		t.updateSize()

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

	case tea.KeyMsg:
		switch msg.String() {
		case "tab":
			t.focusedTileCursor = nextTile(t.focusedTileCursor)
		case "shift+tab":
			t.focusedTileCursor = prevTile(t.focusedTileCursor)
		case "]":
			cmds = append(cmds, t.focusedNextInternalTab())
		case "[":
			cmds = append(cmds, t.focusedPrevInternalTab())
		}
	}

	switch definitionTabPageTileList[t.focusedTileCursor] {
	case "schema":
		cmds = append(cmds, t.schemaTile.Update(msg))
	case "objects":
		cmds = append(cmds, t.objectsPane.Update(msg))
	case "detail":
		cmds = append(cmds, t.detailPane.Update(msg))
	}

	return t, tea.Batch(cmds...)
}

func (t definitionTabModel) View() string {
	left := lipgloss.JoinVertical(
		lipgloss.Top,
		t.styleFor("schema", t.leftWidth, t.schemaHeight).Render(t.schemaTile.View()),
		t.styleFor("objects", t.leftWidth, t.objectsHeight).Render(t.objectsPane.View()),
	)
	right := t.styleFor("detail", t.rightWidth, t.detailHeight).Render(t.detailPane.View())

	body := lipgloss.JoinHorizontal(lipgloss.Left, left, right)

	return lipgloss.JoinVertical(lipgloss.Left, body, t.footerView())
}

// footerView renders a single status line: the last fetch error in red, or an
// empty line so the layout height stays stable.
func (t definitionTabModel) footerView() string {
	if t.lastErr == nil {
		return ""
	}

	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("196")).
		Render("error: " + t.lastErr.Error())
}

// updateSize splits the screen into a left navigation column (schema stacked over
// objects) and a wider right detail column. Each pane draws a rounded border, so
// its content box is the total minus 2 in each dimension.
func (t *definitionTabModel) updateSize() {
	leftTotal := t.width / 3
	rightTotal := t.width - leftTotal

	t.leftWidth = clampZero(leftTotal - 2)
	t.rightWidth = clampZero(rightTotal - 2)

	// One line is reserved at the bottom for the status/error footer.
	usableHeight := t.height - 1

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
	switch definitionTabPageTileList[t.focusedTileCursor] {
	case "objects":
		return tea.Batch(t.objectsPane.NextTab(), t.maybeFetchFunctions())
	case "detail":
		t.detailPane.NextTab()
	}

	return nil
}

func (t *definitionTabModel) focusedPrevInternalTab() tea.Cmd {
	switch definitionTabPageTileList[t.focusedTileCursor] {
	case "objects":
		return tea.Batch(t.objectsPane.PrevTab(), t.maybeFetchFunctions())
	case "detail":
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

func (t definitionTabModel) styleFor(name string, width, height int) lipgloss.Style {
	base := lipgloss.NewStyle().
		Width(width).
		Height(height).
		Border(lipgloss.RoundedBorder(), true)

	if name == definitionTabPageTileList[t.focusedTileCursor] {
		return base.BorderForeground(lipgloss.Color("63"))
	}

	return base
}

func nextTile(cursor int) int {
	return (cursor + 1) % len(definitionTabPageTileList)
}

func prevTile(cursor int) int {
	return (cursor - 1 + len(definitionTabPageTileList)) % len(definitionTabPageTileList)
}

func clampZero(v int) int {
	if v < 0 {
		return 0
	}

	return v
}
