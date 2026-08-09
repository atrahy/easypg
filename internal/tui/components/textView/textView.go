// Package textView is the scrollable read-only text component shared by the SQL
// tile and the help overlay: soft wrapping, horizontal scrolling when wrapping
// is off, and "/" search with highlighted matches.
package textView

import (
	"strings"

	"github.com/atrahy/easypg/internal/tui/components/search"
	"github.com/atrahy/easypg/internal/tui/keys"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

const (
	// horizontalStep is how many columns the horizontal scroll keys move when
	// wrapping is off; the viewport cuts long lines instead of wrapping them, so
	// without this the right-hand side of a long line is unreachable.
	horizontalStep = 8

	// continuationIndent prefixes the extra lines produced by wrapping, so a
	// wrapped line stays visually distinct from a real line break.
	continuationIndent = "    "

	// wrapBreakpoints are the punctuation characters a wrapped line may be
	// broken on, on top of spaces (chosen for SQL).
	wrapBreakpoints = ",()"
)

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
	wrap        bool
	longestLine int

	// lines is the rendered content (after wrapping), which is what search and
	// scrolling work on.
	lines  []string
	query  string
	cursor search.Cursor
}

func New() *Model {
	vp := viewport.New(0, 0)
	vp.KeyMap = keys.ViewportKeyMap(keys.Default)

	m := &Model{viewport: vp, wrap: true}
	m.applyWrapMode()

	return m
}

func (m *Model) SetContent(content string) {
	m.content = content

	m.longestLine = 0
	for _, line := range strings.Split(content, "\n") {
		m.longestLine = max(m.longestLine, ansi.StringWidth(line))
	}

	m.cursor.Reset()
	m.query = ""

	m.render()
	m.viewport.GotoTop()
	m.viewport.SetXOffset(0)
}

func (m *Model) SetSize(width, height int) {
	m.viewport.Width = width
	m.viewport.Height = max(height, 0)

	// Wrapping is width-dependent, so the content has to be rebuilt on resize.
	m.render()
}

func (m *Model) Update(msg tea.Msg) tea.Cmd {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
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
	return m.wrap
}

// ToggleWrap flips soft wrapping and resets the horizontal offset, which is
// meaningless once the content wraps.
func (m *Model) ToggleWrap() {
	m.wrap = !m.wrap
	m.applyWrapMode()
	m.render()
	m.viewport.SetXOffset(0)
}

// CanScrollHorizontally reports whether anything is currently off-screen to the
// right, so callers only advertise the keys when they do something.
func (m *Model) CanScrollHorizontally() bool {
	return !m.wrap && m.longestLine > m.viewport.Width
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
	return m.viewport.Width
}

// Search moves to the first matching line and highlights every match.
func (m *Model) Search(query string) int {
	m.query = query

	target, count := m.cursor.Apply(m.lines, query, m.viewport.YOffset)

	m.render()
	m.revealLine(target)

	return count
}

func (m *Model) CancelSearch() {
	origin, active := m.cursor.Cancel()
	m.query = ""

	m.render()

	if active {
		m.viewport.SetYOffset(origin)
	}
}

func (m *Model) NextMatch() {
	if target, ok := m.cursor.Next(); ok {
		// Re-rendered so the "current match" color moves with the cursor.
		m.render()
		m.revealLine(target)
	}
}

func (m *Model) PrevMatch() {
	if target, ok := m.cursor.Prev(); ok {
		m.render()
		m.revealLine(target)
	}
}

func (m *Model) MatchPosition() (current, total int) {
	return m.cursor.Position()
}

// revealLine scrolls only when the line is off-screen, so walking matches does
// not shuffle the view around for nothing.
func (m *Model) revealLine(line int) {
	if line < m.viewport.YOffset || line >= m.viewport.YOffset+m.viewport.Height {
		m.viewport.SetYOffset(line)
	}
}

// applyWrapMode enables the viewport's horizontal scrolling only when wrapping
// is off; wrapped content never overflows, so the keys would be a no-op there.
func (m *Model) applyWrapMode() {
	if m.wrap {
		m.viewport.SetHorizontalStep(0)
		return
	}

	m.viewport.SetHorizontalStep(horizontalStep)
}

// render rebuilds the displayed lines from the raw content: wrapping first (so
// line indices match what is on screen, including for search), then match
// highlighting.
func (m *Model) render() {
	if m.wrap {
		m.lines = wrapLines(m.content, m.viewport.Width)
	} else {
		m.lines = strings.Split(m.content, "\n")
	}

	if m.query == "" {
		m.viewport.SetContent(strings.Join(m.lines, "\n"))
		return
	}

	current, hasCurrent := m.cursor.Current()

	shown := make([]string, len(m.lines))

	for i, line := range m.lines {
		style := highlightStyle
		if hasCurrent && i == current {
			style = currentHighlightStyle
		}

		shown[i] = highlight(line, m.query, style)
	}

	m.viewport.SetContent(strings.Join(shown, "\n"))
}

// wrapLines soft-wraps each line to width, indenting the continuations. It wraps
// line by line (rather than handing the whole block to ansi.Wrap) so the indent
// applies only to the breaks wrapping introduced.
func wrapLines(content string, width int) []string {
	lines := strings.Split(content, "\n")

	limit := width - len(continuationIndent)
	if limit < 1 {
		return lines
	}

	wrapped := make([]string, 0, len(lines))

	for _, line := range lines {
		if ansi.StringWidth(line) <= width {
			wrapped = append(wrapped, line)
			continue
		}

		for i, segment := range strings.Split(ansi.Wrap(line, limit, wrapBreakpoints), "\n") {
			if i > 0 {
				segment = continuationIndent + segment
			}

			wrapped = append(wrapped, segment)
		}
	}

	return wrapped
}

// highlight styles every occurrence of query in line with the given style,
// honoring the same smart case rule as the search itself.
func highlight(line, query string, style lipgloss.Style) string {
	if !search.Matches(line, query) {
		return line
	}

	haystack, needle := line, query
	if !search.CaseSensitive(query) {
		haystack, needle = strings.ToLower(line), strings.ToLower(query)
	}

	// Byte offsets are shared between line and haystack; lowercasing changes the
	// byte length for a few exotic runes, in which case highlighting is skipped
	// rather than slicing at the wrong offsets.
	if len(haystack) != len(line) || len(needle) != len(query) {
		return line
	}

	var b strings.Builder

	for offset := 0; ; {
		idx := strings.Index(haystack[offset:], needle)
		if idx < 0 {
			b.WriteString(line[offset:])
			break
		}

		start := offset + idx
		b.WriteString(line[offset:start])
		b.WriteString(style.Render(line[start : start+len(needle)]))
		offset = start + len(needle)
	}

	return b.String()
}
