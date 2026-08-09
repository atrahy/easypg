# Feature 1 — Definition Tab (visualization)

Status: in progress. See [the index](./00-overview.md) for the overall vision.

Current state (`internal/tui/definitionTab.go` + `internal/sql`):
- Schema pane: list of namespaces ✅
- Table pane: list of tables/views/etc. per schema ✅ (the type — table/view/index/sequence/... — is already fetched on the SQL side but not visually distinguished)
- Columns pane: name/type/default/not null ✅

Remaining:
- Visually distinguish object types in the table pane (icon/color based on `Table.Type`)
- Index pane (`pg_index` + `pg_get_indexdef`)
- Constraints pane (`pg_constraint` + `pg_get_constraintdef`) — `TableAttr.CheckConstraints` already exists in the struct but is not populated by `QueryTableAttr`
- pgAdmin-style "definition" tab (SQL tab): DDL reconstructed from the columns + constraints + indexes for a table, and from `pg_get_viewdef` for a view
- Special case of views: no editable columns pane, just the SQL definition

---

## Design — restructuring (track B, iteration B1→B3)

This iteration covers displaying the **indexes** and **constraints** (B1 + B3) and replaces the visual type distinction with a **split into tabs** (B2 reformulated). It comes with a layout redesign, closer to pgAdmin/lazygit.

**Out of scope for this iteration** (see [roadmap](./03-roadmap.md)):
- "Generated SQL"/DDL view pgAdmin-style (B4) — will be a *toggleable* view (SQL preview of the current resource), not a permanent pane.
- **Functions** support (`pg_proc`) — the Function tab exists but stays empty ("coming soon").
- Table DDL reconstruction.

### Target layout

Navigation on the **left**, detail (larger) on the **right**:

```
┌ nav ────────────────────────┐ ┌ detail ──────────────────────────────┐
│ ┌─────────────────────────┐ │ │ [ Column | Index | Constraints ]     │
│ │ schema (≈1/3 h, compact)│ │ │                                      │
│ │ scroll, N lines (const) │ │ │   <active detail tab content>        │
│ └─────────────────────────┘ │ │                                      │
│ ┌─────────────────────────┐ │ │                                      │
│ │ objects (≈2/3 h)        │ │ │                                      │
│ │ [ Table | View | Func ] │ │ │                                      │
│ │ <active tab list>       │ │ │                                      │
│ └─────────────────────────┘ │ │                                      │
└─────────────────────────────┘ └──────────────────────────────────────┘
```

- **Left column = navigation**
  - top (~1/3 height): compact **schema** pane, ~4-5 visible rows with scroll — height configurable via a constant (`schemaVisibleRows`).
  - bottom (~2/3 height): **objects** pane with internal tabs **Table / View / Function** (Function as a stub).
- **Right column = detail** (the largest pane): internal tabs **Column / Index / Constraints** of the selected object, **adaptive** to its type.

### Keyboard interaction

- `tab` / `shift+tab`: cycle focus between panes `schema → objects → detail` (existing focus mechanism, redefined for 3 panes).
- `[` / `]`: change the internal tab of the focused pane (lazygit-style) — applies to the **objects** pane (Table/View/Function) and the **detail** pane (Column/Index/Constraints).
- `h` / `l` (aliases for pane navigation): **not implemented for now**, noted for later.

### Type-adaptive detail

- **Table** → Column / Index / Constraints tabs.
- **View** → Column tab only (index/constraints hidden).
- **Function** → definition (n/a for this iteration, stub tab).

### Technical breakdown

**B1 — SQL: indexes & constraints**
- New `internal/sql/indexes.go`: `IndexAttr{ Name, Definition, IsPrimary, IsUnique }` + `QueryTableIndexes(oid)` via `pg_index` + `pg_get_indexdef(indexrelid)`, filtered on `indrelid = $1`.
- New `internal/sql/constraints.go`: `ConstraintAttr{ Name, Type, Definition }` (Type = `contype` mapped to check/fk/pk/unique…) + `QueryTableConstraints(oid)` via `pg_constraint` + `pg_get_constraintdef(oid)`, filtered on `conrelid = $1`.
- Both reuse the generic helper `makeQueryAndCollectRows[T]` (`internal/sql/connection.go`).
- `internal/sql/tables.go`: redefine `TableAttr` to `{ Columns []ColumnAttr; Indexes []IndexAttr; Constraints []ConstraintAttr }` (replacing the `[]string` fields declared today but never populated); extend `QueryTableAttr(oid)` to call the 3 queries and populate the struct.

**TUI components**
- New reusable tabs helper `internal/tui/components/tabs/`: a list of labels + an active index, `Next()`/`Prev()` (mapped to `]`/`[`), rendering of the header `[ Column | Index | Constraints ]` with the active one highlighted, support for hidden tabs (for the adaptive behavior). Reused by both the objects pane **and** the detail pane.
- `detailPane` (right): composes `columnTile` + new `indexTile`/`constraintTile` (mirrors of `columnTile`, wrapping `bubbles/table`) + an internal `tabs`; `SetItems(attr, objType)` populates the tiles and sets the visible tabs based on the type.
- `objectsPane` (bottom-left): Table/View/Function tabs; `SetItems([]sql.Table)` **partitions** by `Table.Type` (`table`/`partitioned table` → Table; `view`/`materialized view` → View; other kinds ignored for now); emits an "object selected" event (name + OID + type) on cursor move or tab change.

**Message chain (rewire)**
- schema cursor → `fetchTables(schema)` → `tablesList` → `objectsPane.SetItems(...)` → auto-select the 1st object → object-selected event.
- object-selected (objects cursor **or** objects tab switch) → `fetchTableAttr(oid)` → `tableAttr` → `detailPane.SetItems(attr, objType)`.
- `[`/`]` routed to the focused pane: on objects → new fetch; on detail → simple view change.

**Cleanups along the way**
- Rename `internal/tui/columTile.go` → `columnTile.go`.
- Fix the value-receiver of `goToNextTile`/`goToPrevTile` (absorbed by the new 3-pane focus logic).

---

## Design — layout & readability (iteration B6)

Follow-up on the B1→B4 iteration: the panes were in place but the *content* was
unreadable — columns hardcoded to a fixed width and truncated whatever the
terminal size, and long DDL lines cut off on the right with no way to reach the
rest (no wrap, no horizontal scroll).

### Responsive column widths

Every `bubbles/table` tile declared its columns with magic widths (`{Title:
"Definition", Width: 40}`), so a wide detail pane wasted space while a narrow one
truncated everything. They now declare **intent** instead — a minimum width and a
growth weight — and a shared helper distributes the actual pane width:

- New `internal/tui/components/tableLayout/`: `Spec{Title, Min, Weight}` +
  `Fit(width, specs) []table.Column`. Every column starts at its minimum, the
  leftover width is split proportionally to the weights (rounding remainder to
  the last weighted column), and when the pane is too narrow for the minimums all
  columns shrink proportionally down to a floor of 3.
- `Fit` subtracts the **2 columns of cell padding** `bubbles/table` adds around
  every cell (`table.DefaultStyles` → `Padding(0, 1)`), which the previous
  hand-rolled `width - 20` computations ignored — that alone made the rows
  overflow their pane by `2 × ncols`.
- Used by all five tables: `schemaTable`, `objectsPane` (the second column claims
  a bigger share on the Function tab, where it holds a signature), `columnTile`,
  `indexTile`, `constraintTile` (definition columns carry the biggest weight).

### Inspector strip (detail pane)

A table cell is single-line by construction and `bubbles/table` has no horizontal
scrolling, so the tail of a `CREATE UNIQUE INDEX … USING btree (lower(email))` is
plain unreachable — the wrap/scroll answer used for the SQL tab does not apply
here. The tabular tabs therefore sit above a 3-line **inspector** strip
(separated by a top border) spelling out the **selected row** in full,
word-wrapped, ellipsised only if it overflows 3 lines:

- `columnTile` → `name · type · NOT NULL · DEFAULT …`
- `indexTile` → the full `pg_get_indexdef` statement
- `constraintTile` → `name · full definition`

Each tile exposes `SelectedDetail() string`; `detailPane` owns the wrapping and
the box. The SQL tab has its own scrolling, so it skips the strip and keeps the
full height.

It costs 4 lines of list height permanently, and it earns them mostly on
Index/Constraints (on Column only a long `DEFAULT` overflows). It is therefore
**foldable** with `i` — shown by default, the fold state sticky across tabs and
objects, the key inert on the SQL tab where there is no strip. A fixed height
(rather than one that adapts to whether the selected row is truncated) is
deliberate: a list that changes height as the cursor moves is worse than a
constant 4 lines.

### SQL tab: wrap & horizontal scroll

`sqlTile` wrapped nothing: `bubbles/viewport` cuts lines at the viewport width.

- **Soft wrap is on by default**, computed per line with `ansi.Wrap` (breakpoints
  ` ,()`) and a 4-space **continuation indent**, so a wrapped line stays visually
  distinct from a real SQL line break. Recomputed on resize.
- `w` toggles wrapping off; the viewport then keeps one line per statement and
  **horizontal scrolling** is enabled (`SetHorizontalStep(8)` → `←`/`→`, disabled
  again when wrapping is on since wrapped content never overflows).
- The tile renders its own status line: vertical scroll %, wrap state, and the
  horizontal scroll % — each shown only when there is something off-screen in
  that direction.

### Footer

The status line at the bottom of the tab showed the last error or nothing. It now
falls back to a **contextual key hint** for the focused pane (pane cycling, `[`/`]`
targets, selection), the error still taking precedence in red. SQL-specific keys
(`w`, `←`/`→`) live in the SQL tile's own status line, where they apply.

**Files**: new `internal/tui/components/tableLayout/tableLayout.go`;
`schemaTable`, `objectsPane`, `columnTile`, `indexTile`, `constraintTile`,
`sqlTile`, `detailPane`, `definitionTab.go`; `go.mod` (`charmbracelet/x/ansi`
promoted from indirect to direct dependency).
