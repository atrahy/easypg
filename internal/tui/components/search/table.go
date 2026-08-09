package search

import (
	"strings"

	"github.com/charmbracelet/bubbles/table"
)

// TableFilter hides the rows of a bubbles/table that do not match a query. It
// keeps a snapshot of the full row set (so clearing the filter costs no refetch)
// and the mapping from displayed row back to the pane's own item slice, which
// would otherwise be indexed by the wrong cursor while filtering.
type TableFilter struct {
	rows    []table.Row
	visible []int
	active  bool
}

// Apply narrows the table to the matching rows. An empty query keeps the filter
// mode on while showing everything, which is what an incremental prompt needs
// as the user erases their query.
func (f *TableFilter) Apply(t *table.Model, query string) int {
	if !f.active {
		f.rows = t.Rows()
		f.active = true
	}

	if query == "" {
		f.visible = nil
		t.SetRows(f.rows)
		t.SetCursor(0)

		return len(f.rows)
	}

	kept := make([]table.Row, 0, len(f.rows))
	f.visible = f.visible[:0]

	for i, row := range f.rows {
		if Matches(strings.Join(row, " "), query) {
			kept = append(kept, row)
			f.visible = append(f.visible, i)
		}
	}

	t.SetRows(kept)
	t.SetCursor(0)

	return len(kept)
}

// Clear restores every row and leaves filter mode.
func (f *TableFilter) Clear(t *table.Model) {
	if !f.active {
		return
	}

	t.SetRows(f.rows)
	t.SetCursor(0)
	f.Reset()
}

// Reset drops the snapshot without touching the table, for when the pane's rows
// are replaced wholesale by a new fetch.
func (f *TableFilter) Reset() {
	f.rows, f.visible, f.active = nil, nil, false
}

func (f *TableFilter) Active() bool {
	return f.active
}

// SourceIndex maps a displayed row index back to the pane's item slice; it is
// the identity when no filter narrows the list, and -1 when nothing is selected.
func (f *TableFilter) SourceIndex(displayed int) int {
	if f.visible == nil {
		return displayed
	}

	if displayed < 0 || displayed >= len(f.visible) {
		return -1
	}

	return f.visible[displayed]
}

