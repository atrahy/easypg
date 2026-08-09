// Package textView is the scrollable read-only text component behind the SQL
// tile: soft wrapping, horizontal scrolling when wrapping is off, and "/" search
// with highlighted matches.
//
// Since the v2 migration ([06](docs/spec/06-charm-v2.md)) the viewport does all
// three natively, so what is left here is the bookkeeping it keeps to itself:
// the match ranges and which one the cursor is on, for the prompt's "3/17".
package textView

import (
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/atrahy/easypg/internal/tui/components/search"
	"github.com/atrahy/easypg/internal/tui/keys"
	"github.com/charmbracelet/x/ansi"
)

// horizontalStep is how many columns the horizontal scroll keys move when
// wrapping is off; the viewport cuts long lines instead of wrapping them, so
// without this the right-hand side of a long line is unreachable. It is inert
// while soft wrap is on, which the viewport enforces itself.
const horizontalStep = 8

var (
	// Every match is highlighted; the one the cursor sits on gets a distinct
	// color, so n/N visibly walks the matches.
	highlightStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("232")).
			Background(lipgloss.Color("214"))

	currentHighlightStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("232")).
				Background(lipgloss.Color("46")).
				Bold(true)
)

type Model struct {
	viewport viewport.Model

	content     string
	longestLine int

	// matches are the byte ranges of the current query in content. The viewport
	// highlights them and holds the selected one in an unexported field, so the
	// index is mirrored here for the "3/17" counter.
	matches [][]int
	current int

	// origin is the scroll offset the search started from, restored by
	// CancelSearch; active tells whether there is one to restore.
	origin int
	active bool
}

func New() *Model {
	vp := viewport.New()
	vp.KeyMap = keys.ViewportKeyMap(keys.Default)
	vp.SoftWrap = true
	vp.HighlightStyle = highlightStyle
	vp.SelectedHighlightStyle = currentHighlightStyle
	vp.SetHorizontalStep(horizontalStep)

	return &Model{viewport: vp}
}

func (m *Model) SetContent(content string) {
	m.content = content

	m.longestLine = 0
	for _, line := range strings.Split(content, "\n") {
		m.longestLine = max(m.longestLine, ansi.StringWidth(line))
	}

	m.dropSearch()

	m.viewport.SetContent(content)
	m.viewport.GotoTop()
	m.viewport.SetXOffset(0)
}

func (m *Model) SetSize(width, height int) {
	m.viewport.SetWidth(width)
	m.viewport.SetHeight(max(height, 0))
}

func (m *Model) Update(msg tea.Msg) tea.Cmd {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		// The viewport has no top/bottom bindings of its own.
		switch {
		case key.Matches(keyMsg, keys.Default.Top):
			m.viewport.GotoTop()
			return nil
		case key.Matches(keyMsg, keys.Default.Bottom):
			m.viewport.GotoBottom()
			return nil
		}
	}

	var cmd tea.Cmd

	m.viewport, cmd = m.viewport.Update(msg)

	return cmd
}

func (m *Model) View() string {
	return m.viewport.View()
}

func (m *Model) Wrap() bool {
	return m.viewport.SoftWrap
}

// ToggleWrap flips soft wrapping and resets the horizontal offset, which is
// meaningless once the content wraps.
func (m *Model) ToggleWrap() {
	m.viewport.SoftWrap = !m.viewport.SoftWrap
	m.viewport.SetXOffset(0)
}

// CanScrollHorizontally reports whether anything is currently off-screen to the
// right, so callers only advertise the keys when they do something.
func (m *Model) CanScrollHorizontally() bool {
	return !m.viewport.SoftWrap && m.longestLine > m.viewport.Width()
}

func (m *Model) ScrollPercent() float64 {
	return m.viewport.ScrollPercent()
}

func (m *Model) HorizontalScrollPercent() float64 {
	return m.viewport.HorizontalScrollPercent()
}

// Scrollable reports whether the content is taller than the viewport.
func (m *Model) Scrollable() bool {
	return m.viewport.TotalLineCount() > m.viewport.VisibleLineCount()
}

func (m *Model) Width() int {
	return m.viewport.Width()
}

// Search highlights every occurrence of query and moves to the first one.
//
// It deliberately anchors at the top of the content: the viewport selects the
// first match at or after the current scroll position, and its notion of "line"
// is post-wrapping, which we no longer compute — scrolling to the top first is
// what keeps the counter below in step with what is highlighted.
func (m *Model) Search(query string) int {
	if query == "" {
		m.clearMatches()

		return 0
	}

	if !m.active {
		m.origin, m.active = m.viewport.YOffset(), true
	}

	m.matches, m.current = matchRanges(m.content, query), 0

	if len(m.matches) == 0 {
		m.viewport.ClearHighlights()
		m.viewport.SetYOffset(m.origin)

		return 0
	}

	m.viewport.GotoTop()
	m.viewport.SetHighlights(m.matches)

	return len(m.matches)
}

func (m *Model) CancelSearch() {
	origin, active := m.origin, m.active

	m.dropSearch()

	if active {
		m.viewport.SetYOffset(origin)
	}
}

func (m *Model) NextMatch() {
	if len(m.matches) == 0 {
		return
	}

	m.current = (m.current + 1) % len(m.matches)
	m.viewport.HighlightNext()
}

func (m *Model) PrevMatch() {
	if len(m.matches) == 0 {
		return
	}

	m.current = (m.current - 1 + len(m.matches)) % len(m.matches)
	m.viewport.HighlightPrevious()
}

func (m *Model) MatchPosition() (current, total int) {
	if len(m.matches) == 0 {
		return 0, 0
	}

	return m.current + 1, len(m.matches)
}

// clearMatches drops the highlights but keeps the origin, so a query emptied
// mid-search can still be cancelled back to where it started.
func (m *Model) clearMatches() {
	m.matches, m.current = nil, 0
	m.viewport.ClearHighlights()
}

func (m *Model) dropSearch() {
	m.clearMatches()
	m.origin, m.active = 0, false
}

// matchRanges lists the byte ranges of query in content, honoring the smart case
// rule — the viewport highlights by byte offset into the content it was given.
func matchRanges(content, query string) [][]int {
	haystack, needle := content, query
	if !search.CaseSensitive(query) {
		haystack, needle = strings.ToLower(content), strings.ToLower(query)
	}

	// Lowercasing changes the byte length of a few exotic runes, which would
	// shift every offset; skip the search rather than highlight the wrong spans.
	if needle == "" || len(haystack) != len(content) || len(needle) != len(query) {
		return nil
	}

	var ranges [][]int

	for offset := 0; ; {
		idx := strings.Index(haystack[offset:], needle)
		if idx < 0 {
			return ranges
		}

		start := offset + idx
		ranges = append(ranges, []int{start, start + len(needle)})
		offset = start + len(needle)
	}
}
