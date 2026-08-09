# Development Roadmap

Actionable version of the roadmap. See [the index](./00-overview.md), [01 — Definition Tab](./01-definition-tab.md), [02 — Query Tool](./02-query-tool.md) and [04 — Backlog](./04-backlog.md) for context.

Each task lists the **files touched** and a **done criterion** (✅ = observable).

## Two independent tracks (any order / parallelizable)

Tracks **A (Config)** and **B (Definition Tab)** touch disjoint files — `main.go` + a new `internal/config` on one side; `internal/sql` + `internal/tui` on the other. Neither blocks the other: doing one before the other imposes no rework. The v1 "foundations first" sequencing is therefore dropped in favor of these two efforts carried out in parallel.

Phases **C → E** (Query Tool then polish) come next and are themselves sequential.

---

## Track A — Config & connection registry

### A1. Config package
- **Files**: new `internal/config/config.go`; add a TOML dependency (e.g. `github.com/BurntSushi/toml`) to `go.mod`.
- A `Config` struct with a **list** of named connections (`[]Connection{Name, DSN | host/port/user/db/sslmode}`), read from `~/.config/easypg/config.toml`. Clear error message if the file is missing.
- ✅ The app reads the config at startup; the first connection (or a default) is used.

### A2. Remove the hardcoded DSN
- **Files**: `main.go` (remove the `pgUrlString` const, build the DSN from the selected connection; `sql.Connect` already takes a string, unchanged).
- ✅ No more hardcoded DSN string; startup goes through the config.

### A3. Multi-connection extension point
- The registry is already modeled as a list; the UI only exposes one connection for now. Document the future connection selector (the way lazygit handles multiple repos).
- ✅ "List of connections" model in place, ready for a later UI selector.
- *Related backlog*: passwords in the system keychain (`go-keyring`, macOS/Linux) — see [04 — Backlog](./04-backlog.md), out of scope for this track.

---

## Track B — Finish the Definition Tab

### B1. Index & constraint queries
- **Files**: `internal/sql/tables.go` (or new `internal/sql/indexes.go` / `constraints.go`).
- Add `QueryTableIndexes(oid)` via `pg_index` + `pg_get_indexdef`, and `QueryTableConstraints(oid)` via `pg_constraint` + `pg_get_constraintdef`. **Standardize on the generic helper `makeQueryAndCollectRows[T]`** (today only `QueryTableAttr` uses it). Finally populate `TableAttr.Indexes` / `TableAttr.CheckConstraints`, currently declared but left `nil`.
- ✅ `QueryTableAttr` returns populated indexes + constraints for a table (verifiable via log or test).

### B2. Distinguish object types
- **Files**: `internal/tui/components/tableTable/tableTable.go`.
- Display / colorize `Table.Type` (already fetched on the SQL side but not visually distinguished) — an icon or color per `relkind` (table / view / materialized view / index / sequence…).
- ✅ Table vs view vs sequence visually distinguishable in the table pane.

### B3. Index + constraint pane(s)
- **Files**: new components (or an extension of the columns pane) + `definitionTab.go` / `definitionTabActions.go` to extend the `tableAttr → SetItems` chain.
- ✅ Selecting a table shows its indexes and constraints.

### B4. "SQL" / DDL view (pgAdmin-style)
- **Files**: `internal/sql` (new `QueryViewDef` via `pg_get_viewdef`; DDL reconstruction from columns + constraints + indexes for tables); `definitionTab.go` (finally wire the `"view"` tile — today an orphan focus target, with no `SetItems` and no `View` case).
- Views case: no editable column drill-down, just the SQL definition.
- ✅ The view pane shows the selected object's DDL; views show their SQL definition.

### B6. Layout & readability
- **Files**: new `internal/tui/components/tableLayout/`; all the table-based tiles + `sqlTile` / `detailPane` / `definitionTab.go`.
- Responsive column widths (min + weight, cell padding accounted for) instead of hardcoded widths; foldable (`i`) inspector strip showing the selected row in full under the tabular tabs; wrap (default) / horizontal scroll toggle in the SQL tab; contextual key hints in the footer. See [01 — Definition Tab](./01-definition-tab.md#design--layout--readability-iteration-b6).
- ✅ No column is truncated by a fixed width any more, and no part of a long DDL line is unreachable.

### B5. Opportunistic refactor (optional)
- Introduce a common `Panel` / `Tile` interface (`SetItems` / `SetSize` / `View` / `Update` / `GetSelected…`) to unify `schemaTable`, `tableTable` and `columnTile`. Rename the file `columTile.go` → `columnTile.go` (typo) and align `columnTile` with the two other components (sub-package, event emission, getters). Fix the value-receiver bug in `goToNextTile` / `goToPrevTile`.
- ✅ The 3 panes share an interface; no more orphan tile.

---

## Track K — Keyboard layer (keymap, help overlay, search)

Detailed in [05 — Keybindings](./05-keybindings.md). Touches `internal/tui` only, so
it is independent of track A; it does overlap track B's files (`definitionTab.go`
and the panes), so it is best done **after** B rather than in parallel with it.

- **K1.** Central keymap (`internal/tui/keys`) + navigation: `j`/`k`, `ctrl+d/u/f/b`, `g`/`G` in every pane; `h`/`l`, `←`/`→`, `tab`, `1`/`2`/`3` for focus; footer generated from the keymap. ✅ No hand-written key strings left.
- **K2.** Floating help overlay `?`, contextual to the focused pane, as a **selectable list** whose `enter` runs the highlighted binding (+ a reusable `overlay` compositing helper). ✅ `?` floats over the layout, executes a command, and closes with `esc`/`q`.
- **K3.** Incremental `/`: filters a row list or the help, searches (with highlight + `n`/`N`) a text view. ✅ One key, the right behavior per pane, with a counter.
- **K4.** Standard extras: `r`/`R` refresh, `z` zoom, `y` copy to clipboard, unified `esc`, vim-style mode block (`NORMAL`/`SEARCH`/`FILTER`/`MATCHES`/`FILTERED`/`HELP`). ✅ Each key works and is listed in `?`.

⚠️ Prerequisite fix carried by K2: `tui.Model.Update` handles `q` / `ctrl+c` before any routing, so a `q` typed into a search box would quit the app — global keys must move behind the mode check.

---

## Phase C — Query Tool MVP

> Depends on real tab infrastructure (C0), which is not functional today.

### C0. Multi-tab infrastructure (prerequisite)
- **Files**: `internal/tui/tui.go`, `internal/tui/tab.go`.
- Today `tabCursor` never changes and `Model.Update` / `getCurrentTab` are hardwired to `definitionTab` (`editorTab` + the `CustomModel` interface = dead scaffolding). Make the dispatch generic: a settable tab slot, a switch keybinding (e.g. `1`/`2` or `ctrl+tab`), routing of `Update` / `View` / `SetSize` to the active tab via `CustomModel`. Wire up `editorTab`.
- ✅ You can switch Definition ↔ Query with the keyboard; each tab receives its events and its size.

### C1. History storage decision (design, before coding)
- Lock down the format and location of the persistent history (e.g. `~/.local/state/easypg/history.jsonl`, or SQLite) **now**, so the query tool isn't redesigned around it later. Record the decision in this file.
- ✅ Decision written down, history data structure defined.

### C2. Session model (list)
- **Files**: new `internal/tui/editorTab.go`.
- State = `[]querySession` (only one shown in the MVP); each session = { editor, results, last error, execution time }. **Keep the raw rows** (`[][]any`, not just formatted strings) to prepare for CSV export.
- ✅ Session structure in place, one active session.

### C3. Editor pane (basic, no vim)
- **Files**: `internal/tui/editorTab.go` (+ a dedicated component).
- `bubbles/textarea` for SQL editing + an execution shortcut (e.g. `ctrl+r`).
- ✅ You type a query and run it.

### C4. Execution + results pane
- **Files**: `internal/sql` (new `RunQuery(sql) (cols, rows, err)` — an arbitrary query not typed into a struct, a new pattern vs the current `QueryXxx`); results rendered via `bubbles/table`.
- ⚠️ The connection is a single `*pgx.Conn` (no pool): a long query blocks the UI → consider a pool or async execution with cancel.
- ✅ A `SELECT` shows tabular results + the execution time; an SQL error is shown **without crashing** (in contrast with the current `os.Exit(1)` in the fetch cmds).

---

## Phase D — Advanced Query Tool

- **D1.** Multi-tab: create / close / navigate between sessions (builds on the session list from C2).
- **D2.** Vim-like bindings in the editor (normal / insert modes, `hjkl`, `dd` / `yy`).
- **D3.** Persistent history: write / read (format locked in C1), `↑` / `↓` navigation in the editor.
- **D4.** Large results: pagination / streaming (pgx cursor, `LIMIT`/`OFFSET` or incremental fetch).

---

## Phase E — Export & UX polish (lazygit-style)

- **E1.** CSV export of results (raw rows already available since C2).
- **E2.** Contextual help footer (`bubbles/help` + keymaps, shortcuts depending on the focused pane) — nonexistent today.
- **E3.** A `theme` / `styles` package centralizing the colors currently duplicated across 3 components (`fg 229` / `bg 57` / border `63` / header `240`).
- **E4.** Connection robustness: replace the `os.Exit(1)` in the fetch cmds with error surfacing in the UI; handle reconnection (all the more useful with track A's multi-connection support).

---

## Cross-cutting technical debt (to address opportunistically)

- `os.Exit(1)` in `definitionTabActions.go` → surface errors in the UI rather than crashing.
- `columTile.go` misspelled; `columnTile` inconsistent with the two other components.
- Orphan `"view"` tile in `definitionTabPageTileList` (resolved by B4).
- `app.log` with no rotation and no log level (see [04 — Backlog](./04-backlog.md)).
- A single `*pgx.Conn` vs a pool (impacts the Query Tool, phase C).
