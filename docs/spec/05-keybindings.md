# Feature 5 — Keybindings, help overlay & search

Status: implemented (K1 → K4). See [the index](./00-overview.md) for the overall
vision, and [01 — Definition Tab](./01-definition-tab.md) for the panes these
keys drive.

Goal: a **complete, vim-like, self-documenting keyboard layer**, shared by every
tab and pane, instead of the ad-hoc `switch msg.String()` blocks scattered across
`definitionTab.go` and the components today.

Three user-facing additions:

1. richer **navigation** (`j`/`k`, `h`/`l`, arrows, direct pane jumps),
2. a floating **help overlay** (`?`), contextual to the focused pane,
3. an incremental **search** (`/`) inside the focused pane — including inside the
   help overlay itself.

---

## Design principles

- **Letters are commands, scrolling is `j`/`k`, arrows, `ctrl`, `g`/`G`.** The
  bubbles defaults bind bare `f`, `b`, `u`, `d`, `space` in the viewport; those
  are dropped so the alphabet stays available for app commands (`r`, `y`, `z`,
  `w`, `i`…). Paging is `ctrl+f`/`ctrl+b`/`ctrl+d`/`ctrl+u` + `pgup`/`pgdn`.
- **One declaration per binding.** Every key is a `key.Binding` declared once in
  a central keymap; the footer hint and the `?` overlay are *generated* from it,
  so a key can never be documented in one place and missing in another (the
  footer strings are hand-written today and will drift).
- **Modes intercept before anything else.** While search or the help overlay is
  active, keys go there first — no key may leak to the global handler (today
  `q` quits from `tui.Model.Update` before any routing, which would kill the app
  as soon as you type "q" in a search box).
- **Contextual, not exhaustive.** The footer shows the 4-5 keys that matter for
  the focused pane; `?` shows everything that applies *right now* (global + pane
  + active sub-tab).

---

## Command recap

### Global (any pane)

| Key | Action |
|---|---|
| `?` | Toggle the floating help overlay |
| `/` | Filter a list, search a text view — whichever fits the focused pane (see below) |
| `q` | Quit (closes the overlay/search first if one is open) |
| `ctrl+c` | Quit, always |
| `esc` | Cancel: close the overlay, exit search, unzoom |
| `r` | Refresh the focused pane (re-run its query) |
| `R` | Refresh everything (schemas → objects → detail) |
| `z` | Zoom: the focused pane takes the whole screen, `z`/`esc` restores |
| `y` | Copy the focused pane's current value to the clipboard |

### Pane navigation

Focus **cycles** through the three panes; it does not move geometrically.

| Key | Action |
|---|---|
| `tab` / `l` / `→` | Next pane (schema → objects → detail → schema) |
| `shift+tab` / `h` / `←` | Previous pane |
| `1` / `2` / `3` | Jump straight to schema / objects / detail |

`h`/`l` are aliases of `shift+tab`/`tab` rather than "go left"/"go right": with
schema and objects stacked in the same column, a directional model would make
half the presses no-ops and leave the vertical move keyless.

`j`/`k` deliberately do **not** move between panes: they belong to the list.

### Inside a pane (lists: schema, objects, columns, indexes, constraints)

| Key | Action |
|---|---|
| `j` / `↓`, `k` / `↑` | Move the cursor down / up |
| `ctrl+d` / `ctrl+u` | Half page down / up |
| `ctrl+f` / `pgdn`, `ctrl+b` / `pgup` | Page down / up |
| `g` / `home`, `G` / `end` | First / last row |
| `[` / `]` | Previous / next sub-tab of the pane |

### Inside a pane (SQL tile)

| Key | Action |
|---|---|
| `j`/`k`, `ctrl+d`/`ctrl+u`, `ctrl+f`/`ctrl+b`, `g`/`G` | Same scrolling as lists |
| `w` | Toggle soft wrap |
| `shift+←` / `shift+→` (aliases `H` / `L`) | Horizontal scroll, wrap off only |

> Horizontal scroll moves **off** `←`/`→`, which now change pane. This is the one
> regression of this iteration versus [B6](./01-definition-tab.md); wrap being on
> by default, `←`/`→` were a no-op in the common case anyway.

### Detail pane specifics

| Key | Action |
|---|---|
| `i` | Fold / unfold the inspector strip |
| `[` / `]` | Column ↔ Index ↔ Constraints ↔ SQL |

### Search and filter — one key, `/`

`/` is the single entry point; **what it opens depends on what is focused**,
because the two contents want different things. There is no separate `f`: on a
list it would have duplicated `/`, and on a text view it has no meaning.

| Pane | `/` opens | Behavior |
|---|---|---|
| Row lists (schema, objects, columns, indexes, constraints) | **filter** | Hides the non-matching rows until `esc` |
| Help overlay | **filter** | Keeps only the matching commands (section titles are never matched, and a title left with nothing under it disappears) |
| SQL tile (text) | **search** | Scrolls to the match and **highlights every occurrence**, the current one in a **distinct color**; `n`/`N` walk them |

Filtering a list beats jumping through it — the matches are all on screen at
once. Filtering a *statement* would make it unreadable, hence search there.

| Key | Action |
|---|---|
| `/` | Open the prompt in the footer |
| *(typing)* | Incremental: the pane updates on every keystroke |
| `enter` | Confirm and keep the result (an empty query is the same as cancelling) |
| `esc` | Cancel: drop the filter, or drop the search and restore the position |
| `n` / `N` | Next / previous match — text views only |

Semantics: substring match, **smart case** (case-insensitive unless the query
contains an uppercase letter), wrapping around the end, a `3/17` (search) or
`12 rows` (filter) indicator in the prompt.

### Mode block

The bottom-left corner holds a vim-style mode block, so the current state is
always visible rather than inferred from what the footer happens to say:

| Block | When |
|---|---|
| `NORMAL` (purple) | Keys drive the panes |
| `SEARCH` (orange) | The `/` prompt has the keyboard, on a text view |
| `FILTER` (green) | The `/` prompt has the keyboard, on a list |
| `MATCHES` (orange) | A **confirmed search** still holds matches; `n`/`N` walk them, `esc` clears |
| `FILTERED` (green) | A **confirmed filter** still hides rows; `esc` restores them |
| `HELP` (pink) | The overlay is open and owns the keys |

Confirming a prompt does **not** drop you back to `NORMAL`: the pane is still
narrowed, or still holding matches, and the keys `n`/`N`/`esc` still mean
something — a state that deserves its own name.

`MATCHES` and `FILTERED` are the only two derived from the pane rather than from
a field on the model: a filter belongs to the list it hides rows from, so
switching focus shows the state of the pane you are now on, never a stale one.

The block hugs its label, so the footer text after it shifts with the mode. The
one case that must not move is the prompt, and it does not: `SEARCH` and `FILTER`
are the same width, and the text input is sized against it.

### Help overlay (`?`)

The overlay is a **selectable list**, not a static page: `enter` runs the
highlighted binding.

| Key | Action |
|---|---|
| `?` / `esc` / `q` | Close |
| `j`/`k`, `ctrl+d`/`ctrl+u`, `ctrl+f`/`ctrl+b`, `g`/`G` | Move the selection (section titles are skipped, never selectable) |
| `enter` | Close the overlay and **run the selected binding** |
| `/` | Filter the list to the matching commands |

Content = the bindings that apply right now, in sections: **Global**, **Panes**,
**Focused pane** (its name in the title), **Search & filter**, generated from the
keymap.

Running a binding is implemented by **replaying its key** through the normal
handlers (`keys.Synthesize`), not by a second dispatch table — so the help can
never drift from what the keys actually do. Movement bindings (`j`/`k`, paging)
are listed for reference; selecting one simply closes the overlay, since
scrolling a pane you can no longer see is meaningless.

---

## Conflicts to settle (and how)

| Conflict | Resolution |
|---|---|
| `h`/`l` = pane change vs. viewport horizontal scroll | Horizontal scroll moves to `shift+←`/`shift+→` + `H`/`L`; `h`/`l` cycle panes |
| `←`/`→` = pane change vs. viewport horizontal scroll | Same |
| Bare `f`/`b`/`u`/`d`/`space` paging in the viewport | Removed; `ctrl+`-prefixed only, freeing the letters |
| `q` quits vs. `q` closes the overlay vs. `q` typed in a search | Mode-first routing: search consumes everything but `esc`/`enter`; the overlay consumes `q`; only then does the global handler see it |
| `j`/`k` scroll vs. moving between the stacked left panes | `j`/`k` stay in-pane; `tab` and `1`/`2`/`3` move focus |
| `g` (first row) vs. a future `gg` chord | Single `g` = first row, as bubbles already does; no chords for now |
| `n` (next match) vs. a future "new" command | `n` reserved for search |

---

## Architecture

**New `internal/tui/keys/keys.go`** — one `KeyMap` struct of `key.Binding`
(`bubbles/key`) grouped by section (`Global`, `Focus`, `List`, `Viewport`,
`Detail`, `Search`, `Help`), plus:

- `ShortHelp(context) []key.Binding` → the footer line,
- `FullHelp(context) []Section` → the overlay,

where `context` describes the focused pane and its active sub-tab. `definitionTab`
stops hand-writing its footer strings and renders these instead.

**Mode state machine** — `definitionTabModel` (later: the root `tui.Model`, once
the multi-tab infra of [C0](./03-roadmap.md) lands) gains a `mode` field
(`modeNormal` | `modeSearch`) plus a `helpOpen` flag, rather than a single
three-valued mode: the two are orthogonal, since `/` searches *inside* the open
help. `handleKey` routes search → help → normal, with no fall-through.

`tui.Model`'s global `q` handler moves behind a `CapturesInput()` check that the
tab answers (true while searching or while the help is open), so `q` is text in
a prompt and "close" in the overlay. `ctrl+c` always quits.

**Focus** — replace the linear `focusedTileCursor` + `definitionTabPageTileList`
with an explicit little grid: each pane declares its `(column, row)`, `h`/`l`
move the column (remembering the last row of the left column), `tab` walks a flat
order, digits jump. Keeps the "only the focused pane gets key events" rule.

**New `internal/tui/components/overlay/`** — lipgloss v1 has no compositor, so the
floating box is spliced onto the rendered background line by line with
`ansi.Cut` (the technique `bubbles/viewport` already uses for horizontal
scrolling): `Render(background, box string, x, y int) string`. This is what makes
the help *float* over the layout instead of replacing the screen.

**New `internal/tui/components/searchBar/`** — a `bubbles/textinput` in the footer
line (no extra height), owning the query, the match count and the pre-search
position to restore on `esc`.

**New `internal/tui/components/textView/`** — the scrollable read-only text
component (wrapping, horizontal scroll, search with highlighting) extracted from
`sqlTile`, which becomes a thin wrapper adding its status line.

**New `internal/tui/components/helpPane/`** — a selectable list of its own rather
than a text view: entries are either a section title (skipped by the cursor and
by the filter) or a binding, with its own scroll window and `SelectedBinding()`
for `enter`.

**`Searchable` interface** (`internal/tui/components/search`), implemented by
every pane — `detailPane` forwarding to whichever tile is on screen:

```go
type Searchable interface {
    Search(query string) int // apply, return the match count, move to the 1st match
    CancelSearch()           // drop the search, restore the pre-search position
    NextMatch()
    PrevMatch()
    MatchPosition() (current, total int)
}
```

The same package holds the smart-case matcher and a `Cursor` (match list +
position + origin) that panes embed; `TableCursor` adapts it to a
`bubbles/table`, so the five list panes get search in six lines each.

A parallel `Filterable` interface (`Filter` / `ClearFilter` / `Filtering`) backs
the list panes, implemented by `TableFilter`: it snapshots the full row set
(clearing a filter costs no refetch) **and the mapping from displayed row back to
the pane's item slice** — without it, a filtered list would index its own
`[]ColumnAttr` / `Selection` slice with the wrong cursor and show the wrong
detail.

The two interfaces are **disjoint in practice**: `Searchable` is implemented only
by the text views (`textView`, `sqlTile`, and `detailPane` forwarding to it),
`Filterable` only by the row lists and the help. `searchTarget()` /
`filterTarget()` return nil when the focused pane is not of that kind, and `/`
picks whichever is non-nil.

**Incremental search does not cascade** — while typing, a cursor move in the
schema or objects pane does *not* re-run the downstream queries; the cascade
fires once on `enter`/`esc` and on `n`/`N`. Otherwise every keystroke of a search
would cost a round trip to Postgres.

This split also sidesteps a `bubbles/table` limitation: it renders whole rows
with no per-cell styling hook, so a match inside a row could never be
highlighted. Filtering needs no highlight — every visible row is a match.

**Clipboard** (`y`) — via `atotto/clipboard`, already in the module graph as an
indirect dependency of bubbles; it needs promoting to a direct require. macOS +
Linux only, which matches the [non-goals](./00-overview.md).

---

## Phased breakdown

### K1. Keymap infrastructure + navigation
- **Files**: new `internal/tui/keys/keys.go`; `definitionTab.go` (focus grid,
  footer generated from the keymap); `schemaTable`, `objectsPane`, the four
  detail tiles (apply the shared `table.KeyMap` / `viewport.KeyMap`, move the SQL
  horizontal scroll to `shift+←`/`shift+→`/`H`/`L`).
- ✅ `j`/`k`, `ctrl+d`/`u`/`f`/`b`, `g`/`G` work in every pane; `h`/`l`, `←`/`→`,
  `tab`, `1`/`2`/`3` move focus; the footer is generated, not hand-written.

### K2. Help overlay (`?`)
- **Files**: new `internal/tui/components/overlay/`, new
  `internal/tui/components/helpPane/`; `definitionTab.go` (mode `modeHelp`),
  `tui.go` (global keys behind the mode check).
- ✅ `?` floats a bordered, scrollable box over the layout, listing the keys of
  the focused pane and its sub-tab; `esc`/`q`/`?` closes it.

### K3. Search (`/`)
- **Files**: new `internal/tui/components/searchBar/`; `Searchable` implemented
  by `schemaTable`, `objectsPane`, the detail tiles and `helpPane`;
  `definitionTab.go` (mode `modeSearch`).
- ✅ `/` searches incrementally in the focused pane, `n`/`N` cycle the matches,
  `esc` restores the previous position, the counter shows `3/17`; searching works
  inside the help overlay too.

### K4. Standard extras
- **Files**: `definitionTab.go` + the panes; `go.mod` (`atotto/clipboard` direct).
- `r`/`R` refresh, `z` zoom, `y` copy (selected definition, or the whole DDL on
  the SQL tab), `esc` unified cancel.
- ✅ Each of these keys works and appears in `?`.

---

---

## Manual test checklist

| Action | Expected |
|---|---|
| At rest | The bottom-left block reads `NORMAL` and is exactly as wide as its label; every state leaves one space between the block and the text after it |
| Open a prompt on a narrow terminal | The bar stays on **one line** — prompt + query + counter are sized to fit, and the layout never moves up by a row |
| `j`/`k`, `↑`/`↓` in any pane | The cursor moves; `ctrl+d`/`ctrl+u`, `ctrl+f`/`ctrl+b`, `g`/`G` page and jump |
| `l`/`→` four times | schema → objects → detail → schema: the focus **cycles**, no press is ever a no-op |
| `h`/`←` | The same cycle backwards |
| `?` | A bordered box floats over the layout; its "…pane" section changes with the focused pane, and lists `w`/`shift+→` only when the SQL tab is active |
| `?` then `j`/`k` | The **selection** moves from binding to binding and **skips the section titles** |
| `?` then `enter` on e.g. "3 focus detail" | The overlay closes and the detail pane takes focus — the entry is really executed |
| `?` then `/` then `pane` | Block reads `FILTER`; the list keeps only the matching commands, a section left empty disappears, titles themselves never match |
| `?` then `q` | The overlay closes, the app does **not** quit |
| `/` in the objects pane, type `emp` | Block reads `FILTER`; non-matching rows **disappear**, prompt shows `n rows`; the detail still matches the highlighted row (the filtered index is mapped back) |
| … then `enter` | Block becomes `FILTERED`, footer says `rows hidden · esc: clear filter`; rows stay hidden while you navigate |
| … then `esc` | Every row is back, block returns to `NORMAL`, and the detail follows the restored selection |
| `/` with no match | Prompt shows `no match` |
| `/` then type `q` | The letter is typed into the prompt; the app does **not** quit |
| `/` on the detail **SQL** tab, type a word | Block reads `SEARCH`; every occurrence is highlighted, the current one in a distinct color |
| … then `enter`, then `n`/`N` | Block reads `MATCHES` with `3/17 · n/N: next/prev · esc: clear`; the current-match color moves with each press |
| `/` on the detail **Column/Index/Constraints** tabs | Filters (no highlight) — same key, list behavior |
| `n` with no active search | Nothing happens |
| `z` on the detail pane | The pane takes the whole screen; `z` or `esc` restores the layout |
| `z` then `tab` | The zoom follows the newly focused pane |
| `y` on a schema / object / detail row | Footer confirms `copied to clipboard`; paste gives the schema name, `schema.object`, the row's full text, or the whole DDL on the SQL tab |
| `r` on objects | The object list is refetched (visible in `app.log`) |
| `R` | Everything reloads from the schema list down |
| `w`, `i` | Still work on the detail pane (wrap, inspector) |

## Out of scope / later

- **Multi-key chords** (`gg`, `dd`, `zh`/`zl`): needs a pending-key buffer with a
  timeout; not worth it before the bindings above are in use.
- **User-configurable keymap** (a `[keys]` section in the config file of
  [track A](./03-roadmap.md)): the central `KeyMap` struct is the right seam for
  it, but it stays hardcoded for now.
- Query-tool-specific bindings ([02 — Query Tool](./02-query-tool.md)): the
  editor will need its own insert/normal mode; the keymap must be extended, not
  reinvented, when that lands.
