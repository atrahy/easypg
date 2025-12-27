package tui

import (
	"github.com/atrahy/easypg/internal/sql"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Model struct {
	width, height int

	db *sql.DBConnection

	tabCursor tabCursor

	definitionTab tea.Model
	// editorTab     CustomModel
}

func NewModel(db *sql.DBConnection) Model {
	return Model{
		db: db,

		tabCursor:     definitionTab,
		definitionTab: newDefinitionTabPage(db),
	}
}

func (m Model) Init() tea.Cmd {
	var cmds []tea.Cmd

	cmd := m.definitionTab.Init()
	cmds = append(cmds, cmd)

	return tea.Batch(cmds...)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// m.definitionTab.SetSize(m.width, m.height)
		// t, cmd := m.definitionTab.Update(msg)
		// m.definitionTab = t
		// cmds = append(cmds, cmd)
		// t, cmd = m.editorTab.Update(msg)
		// m.definitionTab = t
		// cmds = append(cmds, cmd)

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		}
	}

	currentTab := m.getCurrentTab()
	t, cmd := currentTab.Update(msg)
	m.definitionTab = t
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	var page = lipgloss.NewStyle().Width(m.width).Height(m.height)

	currentTab := m.getCurrentTab()

	return page.Render(currentTab.View())
}

func (m Model) getCurrentTab() tea.Model {
	switch m.tabCursor {
	case definitionTab:
		return m.definitionTab
		// case editorTab:
		// 	m.editorTab.Update()
	}

	return m.definitionTab
}
