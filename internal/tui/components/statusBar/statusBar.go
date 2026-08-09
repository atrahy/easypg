// Package statusBar draws the segmented bar at the bottom of a tab, in the Charm
// house style (the one soft-serve uses): coloured blocks laid side by side, each
// answering one question, the middle one stretching to fill the line.
//
// It holds state only — the mode, where you are, how far into it — never
// anything transient: prompts, errors and confirmations belong to the message
// line below, which is what lets the bar stand still while you type.
package statusBar

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

const (
	// valueBg / valueFg dress the stretching middle segment; infoBg and helpBg
	// the two fixed ones on the right. The mode block brings its own color.
	modeFg  = "232"
	valueBg = "235"
	valueFg = "252"
	infoBg  = "62"
	infoFg  = "230"
	helpBg  = "237"
	helpFg  = "243"

	// padding is the one cell each segment keeps on either side of its text.
	padding = 2
)

// Bar is one rendering of the status line. Every field is a finished string: the
// caller decides what "where you are" means, this decides how it looks.
type Bar struct {
	// Mode is the vim-style state block, with the color that goes with it.
	Mode, ModeColor string
	// Context is what the focused pane points at. It is the segment that gives
	// up its width first, since it is the only one that can be shortened without
	// becoming meaningless.
	Context string
	// Info is the position indicator ("☰ 33%"), empty when the pane has nothing
	// to be positioned in.
	Info string
	// Help names the key that opens everything else ("? Help").
	Help string

	Width int
}

// View renders the bar on exactly one line. On a narrow terminal the optional
// segments drop out right to left — help first, then the position — so the mode
// and the context, the two that answer "where am I", survive longest.
func (b Bar) View() string {
	if b.Width <= 0 {
		return ""
	}

	mode := b.segment(b.Mode, modeFg, b.ModeColor, true)
	info := b.segment(b.Info, infoFg, infoBg, false)
	help := b.segment(b.Help, helpFg, helpBg, false)

	for _, drop := range []*string{&help, &info} {
		if lipgloss.Width(mode)+lipgloss.Width(info)+lipgloss.Width(help) <= b.Width {
			break
		}

		*drop = ""
	}

	rest := b.Width - lipgloss.Width(mode) - lipgloss.Width(info) - lipgloss.Width(help)

	return lipgloss.JoinHorizontal(lipgloss.Left, mode, b.contextSegment(rest), info, help)
}

// contextSegment fills whatever the fixed segments left, so the bar always spans
// the full width — a half-painted bar reads as a rendering bug.
func (b Bar) contextSegment(width int) string {
	if width <= 0 {
		return ""
	}

	// Below the padding there is no room for text at all; anything left in would
	// wrap inside the block and cost the layout a row.
	text := ""
	if width > padding {
		text = ansi.Truncate(b.Context, width-padding, "…")
	}

	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(valueFg)).
		Background(lipgloss.Color(valueBg)).
		Padding(0, 1).
		Width(width).
		MaxWidth(width).
		Render(text)
}

func (b Bar) segment(text, fg, bg string, bold bool) string {
	if strings.TrimSpace(text) == "" {
		return ""
	}

	return lipgloss.NewStyle().
		Bold(bold).
		Foreground(lipgloss.Color(fg)).
		Background(lipgloss.Color(bg)).
		Padding(0, 1).
		Render(text)
}
