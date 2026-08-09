// Package search holds the shared pieces of the "/" incremental search: the
// matching rule, a cursor tracking the matches of a pane, and the interface a
// pane implements to be searchable.
package search

import (
	"strings"
	"unicode"
)

// Searchable is implemented by every pane that "/" can drive. Search is
// incremental (called on every keystroke) and returns the match count; the pane
// remembers where the search started so CancelSearch can restore it.
type Searchable interface {
	Search(query string) int
	CancelSearch()
	NextMatch()
	PrevMatch()
	MatchPosition() (current, total int)
}

// Filterable is implemented by the row lists, where "f" hides the non-matching
// rows instead of jumping between them. Text views are not filterable: hiding
// lines of a SQL statement would make it unreadable.
type Filterable interface {
	Filter(query string) int
	ClearFilter()
	Filtering() bool
}

// Matches reports whether haystack contains query, with smart case: the search
// is case-insensitive unless the query itself contains an uppercase letter.
func Matches(haystack, query string) bool {
	if query == "" {
		return false
	}

	if hasUpper(query) {
		return strings.Contains(haystack, query)
	}

	return strings.Contains(strings.ToLower(haystack), strings.ToLower(query))
}

// CaseSensitive reports how the smart case rule resolves for query, for callers
// that need to reproduce the matching themselves (highlighting).
func CaseSensitive(query string) bool {
	return hasUpper(query)
}

func hasUpper(s string) bool {
	for _, r := range s {
		if unicode.IsUpper(r) {
			return true
		}
	}

	return false
}

// Cursor tracks the matching indices of a search over an indexed sequence (list
// rows, or text lines) plus the position within them. Panes embed it and only
// translate the returned index into their own scrolling.
type Cursor struct {
	matches []int
	current int

	// origin is the position the pane was at when the search started, restored
	// by Cancel.
	origin int
	active bool
}

// Apply recomputes the matches for query over items, starting the search at
// position. It returns the position to move to and the match count; an empty
// query resets the cursor and leaves the position untouched.
func (c *Cursor) Apply(items []string, query string, position int) (target, count int) {
	if query == "" {
		c.matches, c.current = nil, 0

		return position, 0
	}

	if !c.active {
		c.origin, c.active = position, true
	}

	c.matches = c.matches[:0]
	for i, item := range items {
		if Matches(item, query) {
			c.matches = append(c.matches, i)
		}
	}

	if len(c.matches) == 0 {
		return c.origin, 0
	}

	// Land on the first match at or after where the search started, wrapping
	// around, so typing does not jump backwards through the list.
	c.current = 0
	for i, m := range c.matches {
		if m >= c.origin {
			c.current = i
			break
		}
	}

	return c.matches[c.current], len(c.matches)
}

// Next / Prev walk the matches, wrapping around; ok is false when there is
// nothing to walk.
func (c *Cursor) Next() (int, bool) {
	if len(c.matches) == 0 {
		return 0, false
	}

	c.current = (c.current + 1) % len(c.matches)

	return c.matches[c.current], true
}

func (c *Cursor) Prev() (int, bool) {
	if len(c.matches) == 0 {
		return 0, false
	}

	c.current = (c.current - 1 + len(c.matches)) % len(c.matches)

	return c.matches[c.current], true
}

// Cancel drops the search and reports the position it started from.
func (c *Cursor) Cancel() (int, bool) {
	origin, active := c.origin, c.active

	c.Reset()

	return origin, active
}

func (c *Cursor) Reset() {
	c.matches, c.current, c.origin, c.active = nil, 0, 0, false
}

// Position is the 1-based rank of the current match and the total, for the
// "3/17" counter in the search prompt.
func (c *Cursor) Position() (current, total int) {
	if len(c.matches) == 0 {
		return 0, 0
	}

	return c.current + 1, len(c.matches)
}

// Indices are the matching positions, for callers that highlight them.
func (c *Cursor) Indices() []int {
	return c.matches
}

// Current is the index of the match the cursor sits on, for callers that style
// it apart from the others.
func (c *Cursor) Current() (int, bool) {
	if len(c.matches) == 0 {
		return 0, false
	}

	return c.matches[c.current], true
}
