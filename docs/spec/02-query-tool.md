# Feature 2 — Query Tool

Status: not started. See [the index](./00-overview.md) for the overall vision and the architecture decisions referenced below.

A separate tab (the `editorTab` stub already exists in `internal/tui/tab.go` but is not wired anywhere) with two panes:

- **Editor pane**: SQL query editing with vim-like bindings (at least normal/insert modes, `hjkl`, `dd`/`yy`) — probably an extended `bubbles/textarea`, or integrating an existing vim mode for bubbletea
- **Results pane**: tabular rendering (likely reusing `bubbles/table` like the other panes) with scroll/pagination for large result sets
- **Execution**: a dedicated shortcut (e.g. `ctrl+enter`), showing SQL errors and execution time
- **Sessions**: modeled internally as a list (see [Architecture decisions](./00-overview.md#architecture-decisions)), even if the MVP shows a single tab
- **Results**: the component keeps the raw rows in memory, not just the formatted display (in preparation for CSV export)
- **History**: persisted to disk, format/location to be defined during design (Phase 2)
