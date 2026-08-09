package sqlTile

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/atrahy/easypg/internal/tui/components/textView"
	"github.com/atrahy/easypg/internal/tui/keys"
)

// Model is a read-only, scrollable text tile used to show generated SQL/DDL.
// Long statements (view definitions, function bodies, CHECK expressions) either
// soft-wrap with an indent — the default — or stay on one line and scroll
// horizontally, toggled with "w".
//
// It draws nothing but the statement: its position, its wrap state and the keys
// that move it are reported by the tab's status bar and message line, so the
// tile spends none of its height saying what the screen already says elsewhere.
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
	return m.text.View()
}

func (m *Model) SetSize(width, height int) {
	m.text.SetSize(width, height)
}

// ScrollPercent, Wrap, CanScrollHorizontally and HorizontalScrollPercent are
// what the tab needs to describe this tile in its own status line.
func (m *Model) ScrollPercent() float64 {
	return m.text.ScrollPercent()
}

func (m *Model) Wrap() bool {
	return m.text.Wrap()
}

func (m *Model) CanScrollHorizontally() bool {
	return m.text.CanScrollHorizontally()
}

func (m *Model) HorizontalScrollPercent() float64 {
	return m.text.HorizontalScrollPercent()
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
