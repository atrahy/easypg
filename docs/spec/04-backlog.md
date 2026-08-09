# Backlog & technical debt

See [the index](./00-overview.md) for the overall vision.

## Loose ideas (unprioritized backlog)

Not yet explored, just noted so they aren't lost:

- **pgAdmin-style tree navigation (object explorer)**: would replace the Definition Tab's 3 stacked schema/table/column panes with a single collapsible tree (schema > tables/views/... > columns/indexes/constraints). Contradicts the current separate-panes architecture — to be challenged later rather than adopted by default, especially given the planned addition of the index/constraint panes in [Phase 1](./03-roadmap.md).
- **DB passwords in the system keychain** (macOS Keychain / Linux Secret Service — no Windows for now, see non-goals) rather than in cleartext in the connections config file. This attaches naturally to the [Phase 0](./03-roadmap.md) work on the connection registry: the config file would reference a connection, and the password would live in the keychain (e.g. the `zalando/go-keyring` lib, whose use would be restricted to macOS/Linux here).

## Technical debt to keep in mind

- `app.log` accumulates with no rotation and no log level
