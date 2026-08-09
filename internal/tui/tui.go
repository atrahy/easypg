package tui

import (
	"github.com/atrahy/easypg/internal/sql"
	"github.com/atrahy/easypg/internal/tui/keys"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// inputCapturer is implemented by tabs that can swallow every key press (a
// search prompt being typed, a modal overlay). The global quit key stands down
// while one does, so "q" typed into a prompt is text, not a command.
type inputCapturer interface {
	CapturesInput() bool
}

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

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Default.ForceQuit):
			return m, tea.Quit
		case key.Matches(msg, keys.Default.Quit) && !m.capturesInput():
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

func (m Model) capturesInput() bool {
	capturer, ok := m.getCurrentTab().(inputCapturer)

	return ok && capturer.CapturesInput()
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
