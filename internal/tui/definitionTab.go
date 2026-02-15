package tui

import (
	"log"

	"github.com/atrahy/easypg/internal/sql"
	"github.com/atrahy/easypg/internal/tui/components/schemaTable"
	"github.com/atrahy/easypg/internal/tui/components/tableTable"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var definitionTabPageTileList = []string{
	"schema",
	"table",
	"column",
	"view",
}

type definitionTabModel struct {
	width, height                 int
	sideTileWidth, sideTileHeigth int
	viewTileWidth, viewTileHeight int

	focusedTileCursor int

	// tableAttr *sql.TableAttr

	schemaTile *schemaTable.SchemaTable
	tableTile  *tableTable.TableTable
	columnTile *columnTile

	viewTile int

	db *sql.DBConnection
}

func newDefinitionTabPage(db *sql.DBConnection) definitionTabModel {
	return definitionTabModel{
		db: db,

		schemaTile: schemaTable.NewSchemaTable(),
		tableTile:  tableTable.NewTableTable(),
		columnTile: newColumnTile(),
	}
}

func (t definitionTabModel) Init() tea.Cmd {
	return t.fetchNamespaces
}

func (t definitionTabModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	log.Printf("msg received: %T : %v", msg, msg)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		t.width, t.height = msg.Width, msg.Height
		t.sideTileWidth, t.sideTileHeigth, t.viewTileWidth, t.viewTileHeight = t.updateSize(t.width, t.height)

	case schemaList:
		schemaList := schemaList(msg)

		setItemCmd := t.schemaTile.SetItems(schemaList.schemas)
		cmds = append(cmds, setItemCmd)

	case schemaTable.SchemaCursorUpdateMsg:
		log.Printf("schemaCursorUpdate: name :%s", t.schemaTile.GetSelectedItemName())
		cmds = append(cmds, t.fetchTables(t.schemaTile.GetSelectedItemName()))

	case tablesList:
		tableList := tablesList(msg)

		setItemCmd := t.tableTile.SetItems(tableList.tables)
		cmds = append(cmds, setItemCmd)

	case tableTable.TableCursorUpdateMsg:
		if t.tableTile.IsEmpty() {
			log.Printf("tableCursorUpdate: table is empty")
			cmds = append(cmds, t.fetchTableAttr("-1"))
			break
		}

		log.Printf("tableCursorUpdate: name :%s", t.tableTile.GetSelectedItemName())
		cmds = append(cmds, t.fetchTableAttr(t.tableTile.GetSelectedItemOID()))

	case tableAttr:
		t.columnTile.SetItems(tableAttr(msg).tableAttr.Columns)

	case tea.KeyMsg:
		switch msg.String() {
		case "tab":
			t.focusedTileCursor = t.goToNextTile()
		case "shift+tab":
			t.focusedTileCursor = t.goToPrevTile()
		}
	}

	switch definitionTabPageTileList[t.focusedTileCursor] {
	case "schema":
		cmd := t.schemaTile.Update(msg)
		cmds = append(cmds, cmd)
	case "table":
		cmd := t.tableTile.Update(msg)
		cmds = append(cmds, cmd)
	case "column":
		t.columnTile.Columns.Focus()
		cmd := t.columnTile.Update(msg)
		cmds = append(cmds, cmd)
	}

	return t, tea.Batch(cmds...)
}

func (t definitionTabModel) View() string {
	sideTileStyle := lipgloss.NewStyle().Width(t.sideTileWidth).Height(t.sideTileHeigth)
	// viewTileStyle := lipgloss.NewStyle().Width(t.viewTileWidth).Height(t.viewTileHeight)

	return lipgloss.JoinHorizontal(
		lipgloss.Left,
		lipgloss.JoinVertical(
			lipgloss.Top,
			sideTileStyle.Inherit(t.applyTileStyle("schema")).Render(t.schemaTile.View()),
			sideTileStyle.Inherit(t.applyTileStyle("table")).Render(t.tableTile.View()),
			sideTileStyle.Inherit(t.applyTileStyle("column")).Render(t.columnTile.View()),
		),
		// viewTileStyle.Inherit(t.applyTileStyle("view")).Render(""),
	)
}

func (t definitionTabModel) updateSize(width, height int) (sideWidth int, sideHeight int, viewWidth int, viewHeight int) {
	var sideTileWidth, sideTileHeigth, viewTileWidth, viewTileHeight int

	// No view panel for now so bigger panel
	sideTileWidth = width / 2
	// sideTileWidth = width / 3
	sideTileHeigth = height / 3
	viewTileWidth = width - sideTileWidth
	viewTileHeight = sideTileHeigth * 3

	// TODO: Should probably move elsewhere with better style integration
	// Currently remove 2 because of the border
	// Could get it dynamicaly with lipgloss.RoundedBorder().Get[Dir]Size()
	sideTileWidth = sideTileWidth - 2
	sideTileHeigth = sideTileHeigth - 2
	viewTileWidth = viewTileWidth - 2
	viewTileHeight = viewTileHeight - 2

	t.schemaTile.SetSize(sideTileWidth, sideTileHeigth)
	t.tableTile.SetSize(sideTileWidth, sideTileHeigth)
	t.columnTile.SetSize(sideTileWidth, sideTileHeigth)

	return sideTileWidth, sideTileHeigth, viewTileWidth, viewTileHeight
}

func (t definitionTabModel) goToNextTile() int {
	if t.focusedTileCursor < len(definitionTabPageTileList)-1 {
		t.focusedTileCursor++
		return t.focusedTileCursor
	}

	return 0
}

func (t definitionTabModel) goToPrevTile() int {
	if t.focusedTileCursor > 0 {
		t.focusedTileCursor--
		return t.focusedTileCursor
	}

	return len(definitionTabPageTileList) - 1
}

func (t definitionTabModel) applyTileStyle(compare string) lipgloss.Style {
	focusedStyle := lipgloss.NewStyle().Border(lipgloss.RoundedBorder(), true).BorderForeground(lipgloss.Color("63"))
	normalStyle := lipgloss.NewStyle().Border(lipgloss.RoundedBorder(), true)

	if compare == definitionTabPageTileList[t.focusedTileCursor] {
		return focusedStyle
	}

	return normalStyle
}
