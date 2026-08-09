// Package paneBox draws the frame around a pane: its shortcut, its name and its
// tab strip in the top edge, its current/total position in the bottom one —
// lazygit-style, so none of them costs a line of content on a layout where every
// pane is already tight.
//
// lipgloss v1 has no border-title support, so the frame is built here instead of
// through Style.Border: the content is rendered borderless and the four sides
// are added around it, the horizontal ones splicing their label into the fill.
package paneBox

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

const (
	// focusColor marks the focused pane — its border, its name, its active tab
	// and its shortcut badge. Everything else stays quiet: three permanently lit
	// badges would compete with the border for "where am I".
	focusColor = "63"
	// activeTabColor / activeTabBg highlight the active tab of the focused pane,
	// reusing the selected-row colors of the tables.
	activeTabColor = "229"
	activeTabBg    = "57"
	// idleColor / statusColor are the unfocused name and the dim details (the
	// inactive tabs, the separators, the position indicator).
	idleColor   = "245"
	statusColor = "240"

	contextSeparator = " · "
	tabSeparator     = "|"
)

// Box is the frame of one pane. Width and Height describe the *content* box, so
// the rendered result is 2 cells wider and 2 lines taller — the same geometry as
// the plain bordered style it replaces.
type Box struct {
	// Title is the pane's name, e.g. "Objects".
	Title string
	// Context qualifies the name for the panes that have no tabs to show
	// instead, e.g. "public" → "Schema · public".
	Context string
	// Tabs is the pane's internal tab strip (already filtered to the tabs that
	// currently apply) and the position of the active one within it.
	Tabs      []string
	ActiveTab int
	// Shortcut is the key that focuses this pane, rendered as "[2]". It comes
	// from the keymap, so it can never advertise a key that does something else.
	Shortcut string
	// Current/Total is the bottom-edge position. It is hidden when Total is 0,
	// since "0/0" says nothing an empty pane does not already say.
	Current, Total int

	Width, Height int
	Focused       bool
}

// Render frames content. Content taller than Height is cut rather than allowed
// to push the bottom edge out of the layout.
func (b Box) Render(content string) string {
	if b.Width <= 0 || b.Height <= 0 {
		return ""
	}

	border := lipgloss.RoundedBorder()
	side := b.borderStyle()

	body := lipgloss.NewStyle().
		Width(b.Width).
		Height(b.Height).
		MaxHeight(b.Height).
		Render(content)

	lines := strings.Split(body, "\n")

	rows := make([]string, 0, len(lines)+2)
	rows = append(rows, b.edge(border.TopLeft, border.Top, border.TopRight, b.label(), lipgloss.Left))

	for _, line := range lines {
		rows = append(rows, side.Render(border.Left)+line+side.Render(border.Right))
	}

	rows = append(rows, b.edge(border.BottomLeft, border.Bottom, border.BottomRight, b.position(), lipgloss.Right))

	return strings.Join(rows, "\n")
}

// edge builds one horizontal border line with label spliced into it, hugging
// either side. A label that does not fit is dropped whole: a half-written title
// reads as corrupted output rather than as a shortened one.
func (b Box) edge(left, fill, right, label string, align lipgloss.Position) string {
	style := b.borderStyle()

	labelWidth := ansi.StringWidth(label)
	if label == "" || labelWidth+2 > b.Width {
		return style.Render(left + strings.Repeat(fill, b.Width) + right)
	}

	lead, trail := 1, b.Width-labelWidth-1
	if align == lipgloss.Right {
		lead, trail = trail, lead
	}

	return style.Render(left+strings.Repeat(fill, lead)) +
		label +
		style.Render(strings.Repeat(fill, trail)+right)
}

// label is the top-edge caption: the first of the candidates that fits.
func (b Box) label() string {
	for _, candidate := range b.labelCandidates() {
		label := pad(candidate)
		if ansi.StringWidth(label)+2 <= b.Width {
			return label
		}
	}

	return ""
}

// labelCandidates lists the caption from the fullest form down to the barest, so
// a cramped pane gives up the least useful part first. A tabbed pane drops its
// name before its tabs — the badge already identifies the pane, while the tabs
// are the only place the "[" / "]" targets are visible — and falls back to the
// active tab alone before showing nothing but the badge.
func (b Box) labelCandidates() []string {
	badge := b.badgeView()
	name := b.nameStyle().Render(b.Title)

	if len(b.Tabs) > 0 {
		return []string{
			join(badge, name, b.borderStyle().Render("─"), b.tabsView(b.Tabs)),
			join(badge, b.tabsView(b.Tabs)),
			join(badge, b.tabsView(b.activeTabOnly())),
			badge,
		}
	}

	if b.Context != "" {
		return []string{
			join(badge, name+b.dimStyle().Render(contextSeparator)+b.contextStyle().Render(b.Context)),
			join(badge, name),
			badge,
		}
	}

	return []string{join(badge, name), badge}
}

// tabsView renders a tab strip: the active one highlighted (as a selected row
// is), the others dim. Both variants are padded the same way, so the caption
// keeps its width when the focus moves.
func (b Box) tabsView(labels []string) string {
	parts := make([]string, 0, len(labels))
	active := b.ActiveTab

	if len(labels) == 1 {
		active = 0
	}

	for i, label := range labels {
		if i == active {
			parts = append(parts, b.activeTabStyle().Render(label))
			continue
		}

		parts = append(parts, b.inactiveTabStyle().Render(label))
	}

	return strings.Join(parts, b.dimStyle().Render(tabSeparator))
}

func (b Box) activeTabOnly() []string {
	if b.ActiveTab < 0 || b.ActiveTab >= len(b.Tabs) {
		return nil
	}

	return []string{b.Tabs[b.ActiveTab]}
}

func (b Box) badgeView() string {
	if b.Shortcut == "" {
		return ""
	}

	return b.badgeStyle().Render("[" + b.Shortcut + "]")
}

// position is the bottom-edge indicator, e.g. " 3/17 ".
func (b Box) position() string {
	if b.Total <= 0 {
		return ""
	}

	return pad(b.dimStyle().Render(fmt.Sprintf("%d/%d", b.Current, b.Total)))
}

// pad surrounds a caption with the spaces separating it from the border fill; an
// empty caption stays empty, so the edge is drawn plain.
func pad(label string) string {
	if label == "" {
		return ""
	}

	return " " + label + " "
}

// join assembles the caption's parts, skipping the empty ones (no badge, no
// title) so they leave no double space behind.
func join(parts ...string) string {
	kept := make([]string, 0, len(parts))

	for _, part := range parts {
		if part != "" {
			kept = append(kept, part)
		}
	}

	return strings.Join(kept, " ")
}

func (b Box) borderStyle() lipgloss.Style {
	if b.Focused {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(focusColor))
	}

	return lipgloss.NewStyle()
}

func (b Box) nameStyle() lipgloss.Style {
	if b.Focused {
		return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(focusColor))
	}

	return lipgloss.NewStyle().Foreground(lipgloss.Color(idleColor))
}

func (b Box) badgeStyle() lipgloss.Style {
	if b.Focused {
		return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(focusColor))
	}

	return lipgloss.NewStyle().Foreground(lipgloss.Color(statusColor))
}

func (b Box) contextStyle() lipgloss.Style {
	if b.Focused {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(idleColor))
	}

	return lipgloss.NewStyle().Foreground(lipgloss.Color(statusColor))
}

// activeTabStyle marks the active tab with the selected-row colors, but only on
// the focused pane: an unfocused pane keeps its tab readable without claiming
// the eye, since its "[" / "]" are not the ones the keyboard is driving.
func (b Box) activeTabStyle() lipgloss.Style {
	base := lipgloss.NewStyle().Bold(true).Padding(0, 1)

	if b.Focused {
		return base.Foreground(lipgloss.Color(activeTabColor)).Background(lipgloss.Color(activeTabBg))
	}

	return base.Foreground(lipgloss.Color(idleColor))
}

func (b Box) inactiveTabStyle() lipgloss.Style {
	return lipgloss.NewStyle().Padding(0, 1).Foreground(lipgloss.Color(statusColor))
}

func (b Box) dimStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(statusColor))
}
