# EasyPG — Initial Spec

Spec index. See also:
- [01 — Definition Tab](./01-definition-tab.md)
- [02 — Query Tool](./02-query-tool.md)
- [03 — Roadmap](./03-roadmap.md)
- [04 — Backlog & technical debt](./04-backlog.md)

## Vision

A PostgreSQL management TUI designed as a lightweight alternative to pgAdmin for
everyday use (not exhaustive coverage), with a lazygit-inspired UX: 100%
keyboard navigation, contextual panes with cyclic focus, immediate feedback, low
friction.

## Non-goals (intentionally narrow scope)

- No full pgAdmin coverage (fine-grained role/permission management, monitoring, backup/restore, replication, etc.)
- No spreadsheet-style grid data editing — the tool stays read + query oriented, no UPDATE/INSERT through the UI
- No Windows support for now (macOS/Linux only)

## Architecture decisions

These points were open in the v1 spec; they are settled now to avoid refactoring features that are already planned:

- **Connections**: multi-connection support is baked into the architecture from the start (a registry of named connections via config, the way lazygit handles multiple repos), even though only a single connection is used for now. The hardcoded DSN in `main.go` should be replaced early by this mechanism rather than first patched into a simple config and then re-refactored later.
- **Query tool — sessions**: multi-tab (several queries open in parallel, like browser tabs) is a confirmed need but not a priority for the MVP. The query tool state should therefore be modeled as a list of sessions from the start, even if the MVP only shows one on screen — multi-tab will build on this existing model, not require a refactor.
- **Query history**: persisted to disk (survives a restart). The storage format/location must be chosen when designing the query tool, not bolted on afterwards.
- **Result export (CSV)**: not a priority for the MVP, but the results component must keep the raw rows in memory (not just display-formatted strings) from its inception, so that export is a simple addition later.

## UX reference: lazygit

- Multiple panes, cyclic keyboard focus (`tab` / `shift+tab`) — already in place in `definitionTab`
- No mouse interaction
- Contextual help footer listing the focused pane's shortcuts — hand-written strings today, to be generated from the keymap of [05](./05-keybindings.md)
- High information density, no visual frills
