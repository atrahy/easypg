// Package tableLayout computes responsive column widths for the bubbles/table
// based tiles. Every tile used to hardcode its widths, so content was truncated
// on narrow panes and the wide panes wasted space; instead each tile declares a
// minimum width and a growth weight and lets Fit distribute the pane width.
package tableLayout

import "github.com/charmbracelet/bubbles/table"

const (
	// cellPadding is the horizontal padding bubbles/table adds around every
	// cell (table.DefaultStyles → Padding(0, 1)); it is not part of the column
	// width, so it must be subtracted before distributing the pane width.
	cellPadding = 2

	// minColumnWidth keeps a column readable (and the "…" ellipsis visible)
	// even when the pane is far too narrow for every column.
	minColumnWidth = 3
)

// Spec describes one column: its title, the width below which it stops being
// readable, and its share of the leftover width (Weight 0 = fixed at Min).
type Spec struct {
	Title  string
	Min    int
	Weight int
}

// Fit turns specs into concrete columns filling width exactly: every column
// starts at its minimum, then the leftover space is split proportionally to the
// weights. When width is too small for the minimums, all columns shrink
// proportionally instead (down to minColumnWidth).
func Fit(width int, specs []Spec) []table.Column {
	widths := make([]int, len(specs))

	totalMin, totalWeight := 0, 0
	for i, s := range specs {
		widths[i] = max(s.Min, minColumnWidth)
		totalMin += widths[i]
		totalWeight += s.Weight
	}

	available := width - cellPadding*len(specs)

	switch {
	case available < totalMin:
		shrink(widths, available, totalMin)
	case totalWeight > 0:
		grow(widths, specs, available-totalMin, totalWeight)
	}

	columns := make([]table.Column, len(specs))
	for i, s := range specs {
		columns[i] = table.Column{Title: s.Title, Width: widths[i]}
	}

	return columns
}

// Position is the 1-based cursor position and row count of a table, for the
// position indicator a pane draws in its bottom border. Rows hidden by a filter
// are excluded, since the indicator describes what is on screen; an empty table
// reports 0/0, which callers render as no indicator at all.
func Position(t table.Model) (current, total int) {
	total = len(t.Rows())
	if total == 0 {
		return 0, 0
	}

	return min(t.Cursor()+1, total), total
}

// grow hands the leftover width to the weighted columns, the rounding remainder
// going to the last one so the columns add up to the full pane width.
func grow(widths []int, specs []Spec, extra, totalWeight int) {
	handed, last := 0, -1

	for i, s := range specs {
		if s.Weight == 0 {
			continue
		}

		share := extra * s.Weight / totalWeight
		widths[i] += share
		handed += share
		last = i
	}

	if last >= 0 {
		widths[last] += extra - handed
	}
}

// shrink scales every column down to fit an undersized pane, keeping each one
// at least minColumnWidth wide (the table truncates with "…" beyond that).
func shrink(widths []int, available, totalMin int) {
	if available < minColumnWidth*len(widths) {
		for i := range widths {
			widths[i] = minColumnWidth
		}

		return
	}

	for i, w := range widths {
		widths[i] = max(w*available/totalMin, minColumnWidth)
	}
}
