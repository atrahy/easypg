# Backlog & technical debt

See [the index](./00-overview.md) for the overall vision.

## Loose ideas (unprioritized backlog)

Not yet explored, just noted so they aren't lost:

- **pgAdmin-style tree navigation (object explorer)**: would replace the Definition Tab's 3 stacked schema/table/column panes with a single collapsible tree (schema > tables/views/... > columns/indexes/constraints). Contradicts the current separate-panes architecture — to be challenged later rather than adopted by default, especially given the planned addition of the index/constraint panes in [Phase 1](./03-roadmap.md).
- ~~**DB passwords in the system keychain**~~ — promoted out of the backlog: it is
  step A2 of [07 — Connections](./07-connections.md). The config file holds no
  secret at all (the loader rejects a `password` key), so it stays committable.

## Technical debt to keep in mind

- `app.log` accumulates with no rotation and no log level
