// Package search holds the shared pieces of the "/" incremental search: the
// matching rule, the interfaces a pane implements to be searched or filtered,
// and the row filter the list panes embed.
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

