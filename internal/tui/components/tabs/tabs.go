package tabs

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Model is a reusable tab header ("[ A | B | C ]") with an active tab and an
// adaptive visibility mask so callers can hide tabs depending on context
// (e.g. a view has no index/constraint tabs).
type Model struct {
	labels  []string
	visible []int // indices into labels, in label order
	active  int   // index into labels; always one of visible
}

func New(labels ...string) *Model {
	visible := make([]int, len(labels))
	for i := range labels {
		visible[i] = i
	}

	return &Model{labels: labels, visible: visible, active: 0}
}

func (m *Model) activeVisiblePos() int {
	for pos, idx := range m.visible {
		if idx == m.active {
			return pos
		}
	}

	return 0
}

func (m *Model) Next() {
	if len(m.visible) == 0 {
		return
	}

	pos := (m.activeVisiblePos() + 1) % len(m.visible)
	m.active = m.visible[pos]
}

func (m *Model) Prev() {
	if len(m.visible) == 0 {
		return
	}

	pos := (m.activeVisiblePos() - 1 + len(m.visible)) % len(m.visible)
	m.active = m.visible[pos]
}

func (m *Model) Active() int {
	return m.active
}

// First selects the first visible tab.
func (m *Model) First() {
	if len(m.visible) > 0 {
		m.active = m.visible[0]
	}
}

func (m *Model) ActiveLabel() string {
	if m.active < 0 || m.active >= len(m.labels) {
		return ""
	}

	return m.labels[m.active]
}

// SetVisible recomputes which tabs are selectable/shown. If the current active
// tab becomes hidden, the active falls back to the first visible tab.
func (m *Model) SetVisible(labels []string) {
	want := make(map[string]bool, len(labels))
	for _, l := range labels {
		want[l] = true
	}

	m.visible = m.visible[:0]
	for i, l := range m.labels {
		if want[l] {
			m.visible = append(m.visible, i)
		}
	}

	for _, idx := range m.visible {
		if idx == m.active {
			return
		}
	}

	if len(m.visible) > 0 {
		m.active = m.visible[0]
	}
}

func (m *Model) View() string {
	activeStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Padding(0, 1)
	normalStyle := lipgloss.NewStyle().Padding(0, 1)

	var parts []string
	for _, idx := range m.visible {
		if idx == m.active {
			parts = append(parts, activeStyle.Render(m.labels[idx]))
		} else {
			parts = append(parts, normalStyle.Render(m.labels[idx]))
		}
	}

	return strings.Join(parts, "")
}
