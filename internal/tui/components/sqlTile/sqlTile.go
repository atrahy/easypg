package sqlTile

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/atrahy/easypg/internal/tui/components/textView"
	"github.com/atrahy/easypg/internal/tui/keys"
)

// statusHeight is the line the tile reserves for its own status/keys hint.
const statusHeight = 1

// Model is a read-only, scrollable text tile used to show generated SQL/DDL.
// Long statements (view definitions, function bodies, CHECK expressions) either
// soft-wrap with an indent — the default — or stay on one line and scroll
// horizontally, toggled with "w".
type Model struct {
	text    *textView.Model
	content string
}

func New() *Model {
	return &Model{text: textView.New()}
}

func (m *Model) SetContent(content string) {
	m.content = content
	m.text.SetContent(content)
}

// Content is the raw SQL, for the copy-to-clipboard command.
func (m *Model) Content() string {
	return m.content
}

func (m *Model) Update(msg tea.Msg) tea.Cmd {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok && key.Matches(keyMsg, keys.Default.Wrap) {
		m.text.ToggleWrap()

		return nil
	}

	return m.text.Update(msg)
}

func (m *Model) View() string {
	return lipgloss.JoinVertical(lipgloss.Left, m.text.View(), m.statusView())
}

func (m *Model) SetSize(width, height int) {
	m.text.SetSize(width, max(height-statusHeight, 0))
}

func (m *Model) Search(query string) int {
	return m.text.Search(query)
}

func (m *Model) CancelSearch() {
	m.text.CancelSearch()
}

func (m *Model) NextMatch() {
	m.text.NextMatch()
}

func (m *Model) PrevMatch() {
	m.text.PrevMatch()
}

func (m *Model) MatchPosition() (current, total int) {
	return m.text.MatchPosition()
}

// statusView is the tile's own footer: scroll position plus the keys that only
// apply here (wrap toggle, horizontal scroll). Positions are only shown when
// there is actually something off-screen in that direction.
func (m *Model) statusView() string {
	var parts []string

	if m.text.Scrollable() {
		parts = append(parts, fmt.Sprintf("↕ %d%%", percent(m.text.ScrollPercent())))
	}

	if m.text.Wrap() {
		parts = append(parts, "w: wrap on")
	} else {
		parts = append(parts, "w: wrap off")

		if m.text.CanScrollHorizontally() {
			hint := keys.Default.ScrollHorizontalHint().Help()

			parts = append(parts, fmt.Sprintf("%s: %s  ↔ %d%%", hint.Key, hint.Desc, percent(m.text.HorizontalScrollPercent())))
		}
	}

	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		MaxWidth(m.text.Width()).
		Render(strings.Join(parts, "  ·  "))
}

func percent(ratio float64) int {
	return min(max(int(ratio*100), 0), 100)
}
