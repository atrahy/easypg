// Package helpPane is the floating "?" window: the bindings that apply right
// now, grouped in sections. It is a selectable list — "enter" runs the
// highlighted binding — and "/" filters it down to the matching commands,
// section titles being neither selectable nor matchable.
package helpPane

import (
	"strings"

	"github.com/atrahy/easypg/internal/tui/components/search"
	"github.com/atrahy/easypg/internal/tui/keys"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	// The box takes a share of the screen, within bounds that keep it readable
	// on a large terminal and usable on a small one.
	widthRatio, minWidth, maxWidth    = 3, 44, 96
	heightRatio, minHeight, maxHeight = 4, 8, 28

	// frame is the border (2) plus the horizontal padding (2) of the box.
	frame = 4

	// chrome is the border (2) plus the title and hint lines.
	chrome = 4
)

var (
	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("63")).
			Padding(0, 1)

	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63"))
	sectionStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("229"))
	keyStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("229")).Background(lipgloss.Color("57"))
	hintStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

// entry is one line of the list: either a section title (not selectable, not
// matchable) or a binding.
type entry struct {
	title bool
	// text is the styled line; plain is the same line without styling, used when
	// the row is highlighted (nesting styles would break the background).
	text       string
	plain      string
	binding    key.Binding
	searchText string
}

type Model struct {
	entries []entry
	visible []entry

	cursor int
	offset int

	filtered bool

	boxWidth, boxHeight int
}

func New() *Model {
	return &Model{}
}

// SetSections rebuilds the list from the contextual help.
func (m *Model) SetSections(sections []keys.Section) {
	keyWidth := 0
	for _, section := range sections {
		for _, binding := range section.Bindings {
			keyWidth = max(keyWidth, lipgloss.Width(binding.Help().Key))
		}
	}

	m.entries = m.entries[:0]

	for i, section := range sections {
		if i > 0 {
			m.entries = append(m.entries, entry{title: true})
		}

		m.entries = append(m.entries, entry{title: true, text: sectionStyle.Render(section.Title)})

		for _, binding := range section.Bindings {
			help := binding.Help()
			padded := padRight(help.Key, keyWidth)

			m.entries = append(m.entries, entry{
				text:       "  " + keyStyle.Render(padded) + "  " + help.Desc,
				plain:      "  " + padded + "  " + help.Desc,
				binding:    binding,
				searchText: help.Key + " " + help.Desc,
			})
		}
	}

	m.visible = m.entries
	m.cursor, m.offset = 0, 0
	m.moveToSelectable(1)
}

// SetSize derives the floating box's size from the screen's.
func (m *Model) SetSize(screenWidth, screenHeight int) {
	m.boxWidth = clamp(screenWidth*(widthRatio-1)/widthRatio, minWidth, maxWidth)
	m.boxHeight = clamp(screenHeight*(heightRatio-1)/heightRatio, minHeight, maxHeight)

	m.scrollToCursor()
}

func (m *Model) Update(msg tea.Msg) tea.Cmd {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil
	}

	switch {
	case key.Matches(keyMsg, keys.Default.Down):
		m.move(1)
	case key.Matches(keyMsg, keys.Default.Up):
		m.move(-1)
	case key.Matches(keyMsg, keys.Default.HalfPageDown):
		m.move(m.bodyHeight() / 2)
	case key.Matches(keyMsg, keys.Default.HalfPageUp):
		m.move(-m.bodyHeight() / 2)
	case key.Matches(keyMsg, keys.Default.PageDown):
		m.move(m.bodyHeight())
	case key.Matches(keyMsg, keys.Default.PageUp):
		m.move(-m.bodyHeight())
	case key.Matches(keyMsg, keys.Default.Top):
		m.cursor = 0
		m.moveToSelectable(1)
	case key.Matches(keyMsg, keys.Default.Bottom):
		m.cursor = len(m.visible) - 1
		m.moveToSelectable(-1)
	}

	return nil
}

// SelectedBinding is the highlighted entry, for the caller to run on "enter".
func (m *Model) SelectedBinding() (key.Binding, bool) {
	if m.cursor < 0 || m.cursor >= len(m.visible) || m.visible[m.cursor].title {
		return key.Binding{}, false
	}

	return m.visible[m.cursor].binding, true
}

// Filter narrows the list: in a reference list, keeping only the matching
// commands is more useful than jumping between them. Section titles are never
// matched, and one left with nothing under it disappears.
func (m *Model) Filter(query string) int {
	m.filtered = query != ""

	if query == "" {
		m.visible = m.entries
		m.cursor, m.offset = 0, 0
		m.moveToSelectable(1)

		return 0
	}

	filtered := make([]entry, 0, len(m.entries))
	count := 0

	for _, e := range m.entries {
		if e.title {
			// Drop a title that ends up with nothing under it.
			for len(filtered) > 0 && filtered[len(filtered)-1].title {
				filtered = filtered[:len(filtered)-1]
			}

			filtered = append(filtered, e)

			continue
		}

		if search.Matches(e.searchText, query) {
			filtered = append(filtered, e)
			count++
		}
	}

	for len(filtered) > 0 && filtered[len(filtered)-1].title {
		filtered = filtered[:len(filtered)-1]
	}

	m.visible = filtered
	m.cursor, m.offset = 0, 0
	m.moveToSelectable(1)

	return count
}

func (m *Model) ClearFilter() {
	m.Filter("")
}

func (m *Model) Filtering() bool {
	return m.filtered
}

func (m *Model) View() string {
	body := make([]string, 0, m.bodyHeight())
	inner := max(m.boxWidth-frame, 0)

	for i := m.offset; i < len(m.visible) && len(body) < m.bodyHeight(); i++ {
		line := m.visible[i].text

		if i == m.cursor && !m.visible[i].title {
			line = selectedStyle.Width(inner).Render(truncate(m.visible[i].plain, inner))
		}

		body = append(body, truncate(line, inner))
	}

	for len(body) < m.bodyHeight() {
		body = append(body, "")
	}

	hint := keys.RenderShort([]key.Binding{
		keys.Default.AcceptSearch, keys.Default.Search, keys.Default.Cancel,
	})

	return boxStyle.Width(inner).Render(lipgloss.JoinVertical(
		lipgloss.Left,
		titleStyle.Render("Keys"),
		strings.Join(body, "\n"),
		hintStyle.MaxWidth(inner).Render(hint),
	))
}

func (m *Model) bodyHeight() int {
	return max(m.boxHeight-chrome, 1)
}

// move walks the list by delta, then slides onto the next selectable entry in
// that direction so a section title is never highlighted.
func (m *Model) move(delta int) {
	if delta == 0 {
		return
	}

	m.cursor = clamp(m.cursor+delta, 0, max(len(m.visible)-1, 0))

	step := 1
	if delta < 0 {
		step = -1
	}

	m.moveToSelectable(step)
}

// moveToSelectable slides the cursor in the given direction until it lands on a
// binding, then bounces back the other way if it ran off the end.
func (m *Model) moveToSelectable(step int) {
	for m.cursor >= 0 && m.cursor < len(m.visible) && m.visible[m.cursor].title {
		m.cursor += step
	}

	if m.cursor < 0 || m.cursor >= len(m.visible) {
		m.cursor = clamp(m.cursor, 0, max(len(m.visible)-1, 0))

		for m.cursor >= 0 && m.cursor < len(m.visible) && m.visible[m.cursor].title {
			m.cursor -= step
		}
	}

	m.cursor = clamp(m.cursor, 0, max(len(m.visible)-1, 0))
	m.scrollToCursor()
}

// scrollToCursor keeps the highlighted entry inside the visible window.
func (m *Model) scrollToCursor() {
	if m.cursor < m.offset {
		m.offset = m.cursor
	}

	if m.cursor >= m.offset+m.bodyHeight() {
		m.offset = m.cursor - m.bodyHeight() + 1
	}

	m.offset = max(m.offset, 0)
}

func truncate(s string, width int) string {
	return lipgloss.NewStyle().MaxWidth(width).Render(s)
}

func padRight(s string, width int) string {
	if gap := width - lipgloss.Width(s); gap > 0 {
		return s + strings.Repeat(" ", gap)
	}

	return s
}

func clamp(v, low, high int) int {
	return min(max(v, low), high)
}
