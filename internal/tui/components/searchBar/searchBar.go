// Package searchBar is the "/" (search) and "f" (filter) prompt. It lives in the
// footer line, so opening one never changes the layout's height.
package searchBar

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// statusReserve is the room kept on the right for the indicator. It is a fixed
// width even though "3/17", "12 rows" and "no match" differ in length: the text
// input must not resize under the user as the status changes.
const statusReserve = 10

var statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

type Model struct {
	input textinput.Model
	width int

	// status is the right-hand indicator ("3/17", "12 rows", "no match"),
	// computed by the caller since it depends on the prompt's meaning.
	status string
}

func New() *Model {
	return &Model{input: textinput.New()}
}

// Start clears the previous query, sets the prompt and takes the keyboard.
func (m *Model) Start(prompt, placeholder string) tea.Cmd {
	m.input.Prompt = prompt
	m.input.Placeholder = placeholder
	m.input.SetValue("")
	m.status = ""
	m.layout()

	return m.input.Focus()
}

func (m *Model) Stop() {
	m.input.Blur()
}

func (m *Model) Value() string {
	return m.input.Value()
}

func (m *Model) SetStatus(status string) {
	m.status = status
}

func (m *Model) SetWidth(width int) {
	m.width = width
	m.layout()
}

// layout sizes the text input so prompt + value + status always fit on one line.
// Overflowing would be more than cosmetic: the root view wraps a too-long line,
// which pushes the whole layout up by a row.
func (m *Model) layout() {
	// -1 for the cursor cell the input keeps past its value, -2 for the gap
	// before the status.
	available := m.width - lipgloss.Width(m.input.Prompt) - statusReserve - 3

	m.input.Width = max(available, 1)
}

func (m *Model) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd

	m.input, cmd = m.input.Update(msg)

	return cmd
}

func (m *Model) View() string {
	view := m.input.View()

	if m.status != "" {
		view += "  " + statusStyle.Render(m.status)
	}

	if m.width <= 0 {
		return view
	}

	// Belt and braces: whatever the input decides to render, the bar stays on
	// one line.
	return lipgloss.NewStyle().MaxWidth(m.width).Render(view)
}
