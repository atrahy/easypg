// Package overlay composites a box on top of an already rendered view.
//
// lipgloss v1 can place a box in a space, but not draw one *over* an existing
// render, which is what a floating window needs. Each covered background line is
// therefore cut around the box with ansi.Cut — the same technique
// bubbles/viewport uses for horizontal scrolling — and spliced back together.
package overlay

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// reset closes any style the background line opened before the cut, so it cannot
// bleed into the box (or the box's into the rest of the line).
const reset = "\x1b[0m"

// Composite draws box over background with its top-left corner at (x, y), in
// terminal cells. Box lines falling outside the background are dropped.
func Composite(background, box string, x, y int) string {
	if box == "" {
		return background
	}

	lines := strings.Split(background, "\n")
	boxLines := strings.Split(box, "\n")

	boxWidth := 0
	for _, line := range boxLines {
		boxWidth = max(boxWidth, ansi.StringWidth(line))
	}

	x = max(x, 0)

	for i, boxLine := range boxLines {
		row := y + i
		if row < 0 || row >= len(lines) {
			continue
		}

		lines[row] = spliceLine(lines[row], pad(boxLine, boxWidth), x, boxWidth)
	}

	return strings.Join(lines, "\n")
}

// Center composites box in the middle of background.
func Center(background, box string) string {
	lines := strings.Split(background, "\n")
	boxLines := strings.Split(box, "\n")

	bgWidth := 0
	for _, line := range lines {
		bgWidth = max(bgWidth, ansi.StringWidth(line))
	}

	boxWidth := 0
	for _, line := range boxLines {
		boxWidth = max(boxWidth, ansi.StringWidth(line))
	}

	return Composite(background, box, (bgWidth-boxWidth)/2, (len(lines)-len(boxLines))/2)
}

func spliceLine(background, boxLine string, x, boxWidth int) string {
	width := ansi.StringWidth(background)

	left := ansi.Cut(background, 0, x)
	if width < x {
		// The background line stops before the box starts: pad the gap.
		left += strings.Repeat(" ", x-width)
	}

	var right string
	if width > x+boxWidth {
		right = ansi.Cut(background, x+boxWidth, width)
	}

	return left + reset + boxLine + reset + right
}

func pad(line string, width int) string {
	if gap := width - ansi.StringWidth(line); gap > 0 {
		return line + strings.Repeat(" ", gap)
	}

	return line
}
