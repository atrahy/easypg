# Definition Tab — Manual Test Scenario

Manual, end-to-end test plan for the Definition Tab. It is derived from the
current implementation (`internal/tui/definitionTab.go` + panes) and the example
schema in [`scripts/example_schema.sql`](../../scripts/example_schema.sql).

See [feature spec](./01-definition-tab.md) for the design.

## Setup

```sh
psql "postgres://local_user@localhost:5432/local_db" -f scripts/example_schema.sql
task run
```

Optionally, in another pane, watch the logs: `tail -f app.log`.

## 0. Startup & connection

| Action | Expected |
|---|---|
| Launch the app | Three panes: left column = **schema** (top, compact ~6 rows) stacked over **objects** (bottom); right column = **detail** (large). The schema pane has the **purple border** (initial focus). |
| — | The schema pane lists the namespaces, including `hr` and `sales`. The first schema is auto-selected → objects pre-populated → detail pre-populated (no input required). |

## 1. Pane navigation (`tab` / `shift+tab`)

| Action | Expected |
|---|---|
| `tab` | Focus (purple border) moves schema → **objects**. |
| `tab` again | objects → **detail**. |
| `tab` again | detail → **schema** (wraps). |
| `shift+tab` | Cycles in reverse. |
| Note | **Only the focused pane** reacts to arrow keys and `[` / `]`. |

## 2. Schema pane → cascade (schema → objects → detail)

| Action | Expected |
|---|---|
| Focus schema, `↓` to `hr` | objects reloads with the **tables of `hr`**; the first object is auto-selected and detail updates. |
| `↓` / `↑` over other schemas | On each cursor move: tables refetched + detail reset. |

## 3. Objects pane — internal tabs `Table / View / Function` (`[` / `]`)

Focus on **objects**, schema = `hr`.

| Action | Expected |
|---|---|
| Tab **Table** (default) | 5 tables: `offices, positions, departments, employees, employee_skills`. Columns `Name` / `Type` (= `table`). |
| `↓` to `employees` | detail updates for `hr.employees`. |
| `]` → tab **View** | 1 view: `active_employees` (Type = `view`). |
| `]` → tab **Function** | **Lazy load**: `hire_new` and `employee_count` appear; the second column becomes `Arguments` (wider) and shows the signature. |
| Check log | Functions are fetched **only when the Function tab is opened** (not on every schema move). |

## 4. Detail pane — type-adaptive tabs (`[` / `]`)

Focus on **detail**.

### 4a. Table (`hr.employees`)

Select `employees` (Table tab in objects), then focus detail.

| Tab | Expected |
|---|---|
| **Column** | 11 columns: `id, first_name, …, manager_id, hired_on, salary, is_active` with type / default / not-null. Check defaults: `hired_on = CURRENT_DATE`, `salary = 0`, `is_active = true`. |
| `]` → **Index** | Index list via `pg_get_indexdef`, including: `employees_last_first_idx` (composite), `employees_active_idx` (**partial** `WHERE is_active`), `employees_email_lower_idx` (**unique on expression** `lower(email)`), plus PK/unique. |
| `]` → **Constraints** | PK `id`; **4 FKs** including `manager_id → employees` (**self-referencing**); UNIQUE `email`; CHECK `salary >= 0`. |
| `]` → **SQL** | **Reconstructed DDL**: `CREATE TABLE hr.employees (…)`, then `ALTER TABLE … ADD CONSTRAINT …`, then `CREATE INDEX …` — indexes backing a constraint (PK/unique) are **not** re-emitted. |
| `]` → wrap | Returns to Column. All 4 tabs present. |

### 4b. View (`hr.active_employees`) — **regression for the fixed bug**

objects → tab **View** → select `active_employees`, focus detail.

| Action | Expected |
|---|---|
| Visible tabs | **Only `Column` and `SQL`** (Index/Constraints hidden). |
| Tab **Column** | View columns: `id, first_name, last_name, email, department, office_city, salary`. |
| Tab **SQL** | `CREATE OR REPLACE VIEW hr.active_employees AS SELECT …` — **no more `relation "16635" does not exist` error** (this was the bug). No red error in the footer. |

### 4c. Function (`hr.hire_new`)

objects → tab **Function** → select `hire_new`, focus detail.

| Action | Expected |
|---|---|
| Visible tabs | **`SQL` only**. |
| Tab SQL | The function definition (plpgsql body). |
| Then select a table/view | The detail tab **resets to `Column`** (not stuck on SQL — `cameFromFunction` reset). |

## 5. Cross-cutting cases (schema `sales`)

| Action | Expected |
|---|---|
| Schema `sales`, table `orders`, detail → **Constraints** | FK `employee_id → hr.employees` (**cross-schema**) and CHECK `status IN ('pending','paid','shipped','cancelled')`. |
| `sales`, view `order_summary`, detail → SQL | `CREATE OR REPLACE VIEW …` renders without error (it calls `sales.order_total`). |
| Open **Function** tab on `hr`, then switch to `sales` while Function stays active | Functions **reload for `sales`** (`order_total`), no stale `hr` functions (`ResetFunctions`). |

## 6. Miscellaneous

| Action | Expected |
|---|---|
| `q` or `ctrl+c` | Quits the app cleanly. |
| Select an object whose fetch fails | **Red** footer `error: …`; no crash. |
| Resize the terminal | Layout recomputed (schema ~6 fixed rows, objects takes the rest, detail = wide right column). |

## 7. Layout & readability

### 7a. Responsive columns

| Action | Expected |
|---|---|
| Widen / narrow the terminal | Column widths follow: the detail pane's `Definition` column grows the most, `Unique` / `Primary` / `Not Null` stay at their title width. |
| Very narrow terminal (~60 cols) | Columns shrink proportionally, cells are ellipsised with `…`, and **no row overflows its pane border** (the cell padding is accounted for). |
| objects → Function tab | The `Arguments` column claims a bigger share than `Type` does on the Table/View tabs. |

### 7b. Inspector strip (detail pane)

| Action | Expected |
|---|---|
| Detail → **Column** on `hr.employees` | A 3-line strip under the table, separated by a horizontal line, shows the selected column in full: `salary · numeric(10,2) · NOT NULL · DEFAULT 0`. It follows the cursor. |
| Detail → **Index**, select `employees_email_lower_idx` | The strip shows the **whole** `CREATE UNIQUE INDEX … (lower(email))` statement, wrapped, even though the table cell is truncated. |
| Detail → **Constraints**, select the FK to `hr.employees` | The strip shows `name · FOREIGN KEY (…) REFERENCES …` in full. |
| Detail → **SQL** | **No strip**: the viewport takes the full height and shows its own status line instead. |
| `i` on Column/Index/Constraints | The strip folds away and the list grows by 4 rows; `i` again brings it back. |
| Fold it, then change tab / object / schema | It **stays folded** (sticky state). |
| `i` on the SQL tab | Nothing happens (no strip there). |

### 7c. SQL tab — wrap & horizontal scroll

Detail → **SQL** on a view with long lines (`hr.active_employees`, `sales.order_summary`).

| Action | Expected |
|---|---|
| On arrival | Lines longer than the pane are **wrapped**, continuations indented by 4 spaces; nothing is cut on the right. Status line: `↕ x%  ·  w: wrap on`. |
| `shift+←` / `shift+→` | Nothing happens (wrapped content has no overflow). |
| `w` | Wrapping off: one line per statement, cut on the right. Status line: `w: wrap off  ·  shift+→/L: scroll  ↔ 0%`. |
| `shift+→` (or `L`) a few times | The line scrolls left by 8 columns per press, the end of the statement becomes visible, `↔ %` grows. |
| `shift+←` (or `H`) back to the start | `↔` returns to 0%; scrolling stops at the left edge. |
| `←` / `→` (unshifted) | Change **pane**, never scroll — see [05 — Keybindings](./05-keybindings.md). |
| `w` again | Wrapping back on, horizontal offset reset to 0. |
| Resize while wrapped | The text re-wraps to the new width. |
| Select another object | Back to the top of the content (scroll reset), wrap state preserved. |

### 7d. Footer

| Action | Expected |
|---|---|
| Focus schema | Dim hint generated from the keymap: `tab: next pane · ↑/↓ j/k: move · /: search · ?: help · q: quit`. |
| Focus objects / detail | The hint gains `[/]: tab`. |
| Trigger a fetch error | The **red** error replaces the hint; the footer stays exactly one line (no layout shift). |
| Full keyboard scenarios | See [05 — Keybindings](./05-keybindings.md). |

## Critical checkpoints (what this iteration had to deliver)

- ✅ View SQL renders without error (OID bug)
- ✅ Type-adaptive tabs table / view / function
- ✅ Indexes & constraints populated
- ✅ Reconstructed DDL
- ✅ Lazy function loading + reset on schema change
- ✅ Responsive columns (no fixed width, no pane overflow)
- ✅ Nothing unreachable: inspector strip for the tables, wrap + horizontal scroll for the SQL
