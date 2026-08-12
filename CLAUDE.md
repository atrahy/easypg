# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`easypg` is a terminal UI (TUI) for browsing a PostgreSQL database — a pgAdmin-style schema browser for the terminal, built with Bubble Tea.

## Commands

- Run the app: `task run` (= `XDG_CONFIG_HOME=scripts/xdg go run .`)
- Build + vet: `task check` (or `go build ./... && go vet ./...`)
- Test: `go test ./...`

The connection comes from `$XDG_CONFIG_HOME/easypg/connections.toml` (fallback `~/.config/easypg/`), never from the source. `task run` points that variable at the repo's own test config, `scripts/xdg/easypg/connections.toml`, whose `local` profile is `local_user@localhost/local_db` — so a local Postgres with that user/db (loaded with `scripts/example_schema.sql`) is still what the dev loop expects. `go run .` alone uses your real config instead; `-c <name>` picks a profile.

Passwords are never in that file (the loader *rejects* a `password` key): they live in the system vault via `internal/secrets`, and a profile whose vault entry is missing is prompted for. See [docs/spec/07-connections.md](docs/spec/07-connections.md).

Runtime logs are written to `app.log` in the working directory (stdout/stderr are reserved for the TUI itself, so `log.Printf` is the only way to observe behavior while it's running — `tail -f app.log` in another pane is the way to debug interactively).

## Architecture

Elm-architecture (Model/Update/View) via Bubble Tea, with a tree of nested sub-models rather than one flat model:

```
tui.Model (root)                     internal/tui/tui.go
 └─ definitionTabModel                internal/tui/definitionTab.go
     ├─ schemaTable.SchemaTable       internal/tui/components/schemaTable/
     ├─ tableTable.TableTable         internal/tui/components/tableTable/
     └─ columnTile                    internal/tui/columTile.go
```

- `tui.Model` owns which top-level tab is active (`tabCursor`); today only `definitionTab` is wired up (an `editorTab` constant exists but has no implementation).
- `tui.Model` also owns the **connection**, not just the tabs: it starts on the connection screen (`connectionsScreen.go` + the wizard in `connectionForm.go`) unless startup resolved a single target, connects through a Cmd (`connectCmd` → `connectedMsg` / `connectErrMsg` / `secretNeededMsg`), and rebuilds `definitionTabModel` on every (re)connection — `c` reopens the screen to switch database at runtime. `internal/config` parses the profiles, `internal/secrets` reads/writes the passwords in the system vault.
- `definitionTabModel` is the main screen: three stacked panels (schema list → table list → column list) with `tab`/`shift+tab` cycling focus between them (`focusedTileCursor`, `definitionTabPageTileList`). Only the focused panel's `Update` receives bubbletea key events for navigation.
- Each panel (`schemaTable`, `tableTable`, `columnTile`) wraps a `bubbles/table.Model` and exposes a small hand-rolled interface (`SetItems`, `SetSize`, `View`, `Update`, `GetSelectedItem*`) rather than implementing `tea.Model` directly — `definitionTabModel` calls into these directly instead of routing through `tea.Model.Update` polymorphically.
- Panels are wired together via the standard Bubble Tea Cmd/Msg round-trip, chained three levels deep — the concrete chain isn't visible from any single file, so here it is end to end: moving the cursor in `schemaTable` emits `schemaTable.SchemaCursorUpdateMsg`; `definitionTabModel.Update` catches it and dispatches `fetchTables` (`definitionTabActions.go`), which queries Postgres and returns `tablesList`; that's caught in the same `Update` switch and pushed into `tableTable` via `SetItems`. Moving the cursor there emits `tableTable.TableCursorUpdateMsg`, which triggers `fetchTableAttr` → `tableAttr` → `columnTile.SetItems`. When adding a new panel/drill-down level, extend this same message → cmd → result-message → `SetItems` chain rather than fetching data directly in `View` or on a timer.
- All Postgres access goes through `internal/sql` (package `sql`, shadowing the stdlib name — be careful with imports). Each query type has its own file (`namespace.go`, `tables.go`) with the raw SQL as a const querying `pg_catalog` system tables, a result struct with `db:"..."` tags, and a `(*DBConnection)` method that runs it via `pgx.CollectRows(rows, pgx.RowToStructByName[T])`. `connection.go` has a generic `makeQueryAndCollectRows[T]` helper for this pattern — prefer it for new queries over hand-rolling `conn.Query` + `CollectRows`.
- Sizing is computed top-down: `tui.Model` gets `tea.WindowSizeMsg`, and `definitionTabModel.updateSize` derives each panel's width/height and pushes it down via each panel's `SetSize`.
