# Feature 6 — Charm v2 migration (bubbletea / bubbles / lipgloss)

Status: implemented. See [the index](./00-overview.md) for the overall vision.

Charm released v2 of the three libraries the whole TUI stands on, under new
vanity module paths (`charm.land/bubbletea/v2`, `charm.land/bubbles/v2`,
`charm.land/lipgloss/v2`). Most of the port is a find-and-replace on imports;
two changes are structural, and one v2 feature makes a hand-rolled component of
ours redundant.

---

## The two structural changes

### `View()` returns a struct

`tea.Model` is now:

```go
type Model interface {
    Init() Cmd
    Update(Msg) (Model, Cmd)
    View() View
}
```

and `tea.View` carries the terminal-level state that used to be program options
and toggle commands: `AltScreen`, `MouseMode`, `Cursor`, `WindowTitle`,
`ReportFocus`… `tea.WithAltScreen()` is gone; the root view sets
`view.AltScreen = true` on every frame instead.

The consequence for us is not in `tui.Model` — it is in what a *tab* is. A tab
does not own the terminal: it renders a string that the root composes into its
view. It was typed as a `tea.Model` anyway, which now would force it to return a
`tea.View` it has no business producing. Hence a local interface in `tab.go`:

```go
type tab interface {
    Init() tea.Cmd
    Update(tea.Msg) (tab, tea.Cmd)
    View() string
}
```

`definitionTabModel`'s handlers return `(tab, tea.Cmd)`; nothing else about them
changes. The multi-tab work of [C0](./03-roadmap.md) will add tabs to this
interface, not to `tea.Model`.

### Keys are `Code` / `Text` / `Mod`

`tea.KeyMsg` became an *interface*; a press arrives as `tea.KeyPressMsg`, and the
key itself lost `Type`/`Runes` for:

```go
type Key struct {
    Text        string
    Mod         KeyMod
    Code        rune
    ShiftedCode rune
    BaseCode    rune
    IsRepeat    bool
}
```

Every `case tea.KeyMsg` and every `msg.(tea.KeyMsg)` becomes `KeyPressMsg`.
`key.Matches` is now generic over `fmt.Stringer`, so the call sites are
untouched — and matching still goes through `String()`, which returns `Text`
when there is one and a `ctrl+…`-style keystroke otherwise. Our bindings
therefore keep working as written, including the shifted ones (`G`, `R`, `N`,
`H`/`L`, `?`): they arrive as text.

`keys.Synthesize` — the help overlay's trick of *replaying* a binding's key
rather than dispatching it a second way — is the one place that built a key
message by hand, and the only one that had to be rewritten:

```go
var namedKeys = map[string]tea.KeyPressMsg{
    "tab":       {Code: tea.KeyTab},
    "shift+tab": {Code: tea.KeyTab, Mod: tea.ModShift},
    "ctrl+c":    {Code: 'c', Mod: tea.ModCtrl},
    // …
}
// and, for a single rune: {Code: r, Text: string(r)}
```

Setting `Text` is what makes the replay match: `String()` prefers it, so a
synthesized `G` is indistinguishable from a typed one.

> One spelling change to know about: the space bar is now `"space"`, not `" "`.
> No binding of ours uses it.

---

## What the viewport now does for us

`bubbles/v2`'s viewport gained soft wrapping (`SoftWrap`), match highlighting
(`SetHighlights` / `HighlightNext` / `HighlightPrevious`, with `HighlightStyle`
and `SelectedHighlightStyle`) and horizontal scrolling — which is, feature for
feature, what `textView` was hand-rolling in
[B6](./01-definition-tab.md)/[K3](./05-keybindings.md).

`textView` keeps its public API (so `sqlTile`, `detailPane` and the
`Searchable` plumbing see nothing) and becomes a shell over those:

- wrapping is `m.viewport.SoftWrap`, flipped by `w`; the viewport disables
  horizontal scrolling by itself while it is on, so the mode juggling is gone,
- searching computes the match ranges as **byte offsets into the content** —
  smart case still comes from `search.Matches`/`search.CaseSensitive` — and hands
  them to `SetHighlights`; `n`/`N` are `HighlightNext`/`HighlightPrevious`,
- `render`, `wrapLines`, `highlight` and the mirrored `lines []string` are
  deleted, and with them the last user of `search.Cursor`, which is dropped from
  the `search` package.

**Deliberate regression**: wrapped lines lose their 4-space continuation indent.
The viewport wraps internally, and its `LeftGutterFunc` prefixes *every* visual
line rather than only the continuations, so the indent cannot be reproduced
without taking the wrapping back — which is the thing we are giving away.

The current match keeps its own index on our side: the viewport tracks the
selected highlight in an unexported field, and the prompt's `3/17` counter needs
it. Keeping the two in step forces one behavior change — **`/` now anchors at
the top of the content** instead of at the position the search started from.
`SetHighlights` selects "the first match at or after the current scroll
position", where *position* means a post-wrapping line index; reproducing that
rule would mean keeping the wrapping computation we are precisely handing over,
so the view scrolls to the top before highlighting and the first match is match
number 1, by construction. `esc` still restores the pre-search position.

---

## Not adopted (yet)

v2 opens doors this port deliberately leaves closed, since nothing asks for them
today: the real terminal cursor (`view.Cursor`), mouse modes, the window title,
keyboard enhancements (key release events, `IsRepeat`), the native
`ProgressBar`, and the viewport's `LeftGutterFunc` (line numbers on the SQL
tab, one day).

**Files**: `go.mod`/`go.sum`; imports across `main.go`, `internal/tui/**`;
`tab.go` (`CustomModel` → `tab`), `tui.go` (`View() tea.View`), `definitionTab.go`
(handler signatures, `KeyPressMsg`), `keys/keys.go` (`Synthesize`),
`textView` (rewritten on the v2 viewport), `search` (`Cursor` removed),
`searchBar` (`SetWidth`), `sqlTile`/`detailPane`/`helpPane` (`KeyPressMsg`).
