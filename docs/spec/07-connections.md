# Feature 7 — Connections: config, secrets & selection

Status: designed, not implemented. See [the index](./00-overview.md) for the
overall vision and [03 — Roadmap](./03-roadmap.md) for the track this replaces.

The v1 roadmap's track A was three small steps: read a TOML file, drop the
hardcoded DSN, and leave a list-shaped struct behind as an extension point. That
is the shape of a config *file*, not of a connection *layer* — and every CLI that
handles several targets (`aws`, `k9s`, lazygit outside a repo) ships two things
this misses:

- a way to **add** a target without hand-editing a file,
- an **intermediate view** to pick one, defaulting to the obvious choice.

This iteration builds that layer instead, and supersedes A1→A3.

The constraint that drives the file format: **`connections.toml` must be
committable.** A user who wants to version or share their profiles must be able
to, without ever leaking a secret. That is not a documentation promise here, it
is enforced by the loader (see below).

---

## On disk

XDG, with `~/.config` as the fallback when `$XDG_CONFIG_HOME` is unset:

```
$XDG_CONFIG_HOME/easypg/
├── config.toml        # general app config — reserved, nothing to configure yet
└── connections.toml   # the profiles. No secrets, ever. Mode 0644.
```

Two files rather than two sections of one, on the `.aws/config` model: the
connections file is the one a user edits often, diffs, and may commit; the
general one will hold process-level preferences (theme, log level, default tab)
that have nothing to do with any database. `config.toml` is *read* from the start
(absent = defaults, never an error) so that adding a key later needs no new
plumbing — it just has no keys yet.

Secrets live in the **system vault**, never in either file — see
[Secrets](#secrets).

`$XDG_CONFIG_HOME` is the only way to point the layer elsewhere — there is **no
`--config` flag**. The env var already does the job, standard-conformant and with
no code of ours, and it is what the [test config](#the-repos-test-config) below
rides on. A flag can be added later if a use case turns up that the env var does
not cover.

The directory not existing is the normal first run, not an error: the connection
screen opens in its empty state and the wizard creates it.

### `connections.toml`

```toml
[local]
host    = "localhost"
port    = 5432             # optional, default 5432
user    = "local_user"
dbname  = "local_db"
sslmode = "disable"        # optional, libpq default (prefer)
auth    = "none"           # keychain | pgpass | env | none

[staging]
dsn  = "postgres://app@db.internal:5432/app_prod?sslmode=require"
auth = "keychain"
```

**Every top-level table is a connection, and the file holds nothing else** — no
root keys, no `[[connections]]` array of tables (the one TOML construct most
people have never written), no `connections.` prefix over each name. The prefix
only earns its keep when a file mixes profiles with something else, and this one
cannot: process-level settings live in `config.toml`, next door. The last root
key, `default`, disappeared with the auto-connection it used to pick — the screen
always opens, so there was nothing left for it to decide.

A value at the root is therefore an error, and one worth naming: it is what an
old file, or a habit from another tool, leaves behind, and the decoder's own
"cannot load a string into a struct" says nothing about the shape of this file.

The name being the table's key has two consequences beyond looks: there is no
`name = …` field (a stray one is an unknown key, hence an error), and a duplicate
profile is a duplicate table, caught by the decoder before any of our own
validation runs.

The price is that a Go map has no order, while the picker's rows must follow the
file: a list that shuffles between runs cannot be learned. The order is therefore
recovered from the decoder's metadata, which reports the keys as the document
declares them.

> Why TOML at all: it is what hand-edited CLI config settled on (Cargo, starship,
> alacritty — which *left* YAML for it — helix, ruff), it is specified and typed
> (`port = 5432` is an integer), comments are first-class, and `BurntSushi/toml`
> carries no dependencies. YAML was the alternative, being what lazygit and k9s
> use, but it brings indentation traps and a Go story that got worse when
> `go-yaml/yaml` was archived in 2025. INI — the AWS-CLI look, `[conn name]` —
> has no specification at all: every library disagrees on comments, escaping and
> repeated sections, and everything is a string.

A profile declares its target **either** as a `dsn` **or** as discrete fields; if
both are present the `dsn` wins and the fields are ignored (with a line in
`app.log` — silently dropping half a profile is how a user ends up connected to
the wrong database). The discrete form is what the wizard writes and what reads
best in review; the `dsn` form is what one pastes from a provider's dashboard, and
it is also the only way to reach a libpq parameter we do not model.

**There is no `password` key, and there never will be.** The loader does not
ignore one — it **fails** on it, with a message pointing at `auth` and the vault:

```
error: ~/.config/easypg/connections.toml, connection "staging": "password" is not
       a valid key — secrets live in the system vault, not in this file.
       Set auth = "keychain" and store it from the app (c → n), or use
       auth = "pgpass" / "env".
```

Rejecting it is what makes the file committable *by construction*: a user cannot
end up with a secret in it by accident, and a reviewer never has to check. The one
hole left open is a password embedded in a pasted `dsn` — that we cannot forbid
without rejecting valid DSNs, so it is logged as a warning on load and left to the
user's judgment.

Validation is otherwise ordinary, and reports **every** problem at once rather
than the first: a name is required and must be unique, a profile needs a `dsn` or
at least `host`/`user`/`dbname`, `auth` must be one of the four values, and
`default` must name an existing profile.

---

## Secrets

`zalando/go-keyring` — macOS Keychain and the Linux Secret Service, which is
exactly the platform support the [non-goals](./00-overview.md) commit to. Service
`easypg`, account = the profile name.

Naming entries after the profile means a name can outlive the profile that
created it, so the app owns the whole life cycle rather than sending the user to
Keychain Access:

- **Writing overwrites.** Re-creating a deleted profile under the same name
  replaces its old entry instead of inheriting it.
- **`f` forgets** the highlighted profile's entry (listed in the hint line only
  when the profile reads the vault). That clears an orphan, and it stops keeping
  a password one no longer wants kept.
- **A refused password re-prompts.** A connection that fails on `28P01` /
  `28000` while the profile is on `keychain` reopens the prompt — "the stored
  password was refused — type it again" — and the new one overwrites. Without
  this, a *wrong* entry is a dead end: it exists, so nothing would ever ask
  again, and the only fix would be outside the app.
- The wizard **refuses to save `keychain` with an empty password**, which would
  otherwise write a profile pointing at an entry nothing put there — or, worse,
  one that silently inherits a stale entry left by an older profile of that name.

The `auth` field says where the credential comes from, and it is **declared, not
guessed** — "why is it asking me for a password" must be answerable by reading the
profile:

| `auth` | Where the password comes from |
|---|---|
| `keychain` (default) | The vault. No entry → the app **prompts** for it, masked, and stores it unless told not to |
| `pgpass` | Nowhere, from our side: pgx reads `~/.pgpass` / `$PGPASSFILE` natively |
| `env` | Nowhere: pgx reads `$PGPASSWORD` natively |
| `none` | No password at all — trust, peer, or a client certificate carried by the DSN |

The prompt on a missing entry is not a fallback, it is the point: it is what makes
a committed `connections.toml` usable on a second machine. Clone the file, launch,
type the password once, and the vault holds it from then on.

Two toggles live on that prompt, because both answers are legitimate:

- `ctrl+r` **unmasks** the field. A password one cannot read is a password one
  retypes from scratch at the first typo, and the mask protects against a
  shoulder, not against the person typing.
- `ctrl+s` turns **remembering off** (it is on by default — that is what
  `auth = "keychain"` declares). On a borrowed or shared machine the password
  then lives only as long as the session, and the prompt comes back next time.
  Its state is spelled out in the hint line, `remember: on` / `off`, so it is
  read before `enter` rather than discovered after.

When the vault itself is unreachable (headless Linux with no Secret Service, a CI
box), the error names the situation and the two ways out (`auth = "pgpass"` or
`"env"`) instead of surfacing a `dbus` failure.

**The password never enters a connection string.** `sql.Connect` becomes
`Connect(dsn, password string)`: it `pgx.ParseConfig`s the DSN, sets
`cfg.Password` when the caller has one, and calls `pgx.ConnectConfig`. This
sidesteps the whole escaping question (a `@` or a `/` in a password, URL-encoded
or libpq-quoted) — there is no string to escape — and it keeps the secret out of
anything that might get logged. An empty password means "set nothing", which is
precisely what `pgpass` / `env` / `none` need.

---

## Which connection, and when

**The connection screen always opens.** Not only when the choice is ambiguous:
the screen is where one sees what is configured and switches, and a screen that
skips itself whenever the config has a single answer is a screen you never learn.
The launch that *should* have shown it — a profile silently taken from a stale
`default`, a `-c` typo that resolved to something else — would be exactly the one
where nothing tells you why you are looking at that database.

All that is left of resolution is the **cursor**: `--connection <name>` (alias
`-c`) starts the screen on that profile, anything else starts it on the first
row, and connecting is one `enter`. An unknown `-c` name is still an error at
startup rather than a silently ignored flag.

No file, or a file with zero profiles, lands on the same screen in its empty
state, which is a one-line invitation to press `n`. That replaces the "generate a
commented starter file and exit" idea: with a wizard on `n`, writing a file the
user must then find and edit is a detour, and the app can create it itself with
the profile the user just described.

---

## The repo's test config

The hardcoded DSN had one virtue: `task run` worked on a fresh clone with no
setup beyond a local Postgres. Its removal must not cost that, and it must not
buy it back by making the dev loop depend on whatever the contributor happens to
have in `~/.config`. So the repo ships its own config home, and `task run` points
`$XDG_CONFIG_HOME` at it:

```yaml
run:
  desc: Run the project against the repo's test config
  cmds:
    - XDG_CONFIG_HOME=scripts/xdg go run .
```

Hence the `easypg/` level in the path — the env var names a *config home*, and
the app looks for its own directory inside it. The file therefore lives at
`scripts/xdg/easypg/connections.toml`, one directory away from the
[`example_schema.sql`](../../scripts/example_schema.sql) the
[test scenario](./01-definition-tab-test.md) already loads:

```toml
# Test config — used by `task run` (XDG_CONFIG_HOME=scripts/xdg).
# Committed on purpose: it holds no secret, which is the whole point of the
# format. Plain `easypg` reads your real ~/.config/easypg instead.

default = "local"

[[connections]]
name   = "local"          # scripts/example_schema.sql lives here
host   = "localhost"
user   = "local_user"
dbname = "local_db"
auth   = "none"           # trust/peer on a local socket — no vault entry needed

[[connections]]
name   = "down"           # deliberately unreachable: exercises the error paths
host   = "127.0.0.1"
port   = 5433
user   = "nobody"
dbname = "nothing"
auth   = "none"
```

It earns its keep three times over: `task run` connects with no prompt (`default`
+ `auth = "none"`), `c` has a second entry to switch to and a connection failure
to display, and the comment block at the top is the fixture for the
"[appending preserves what is above](#writing-the-file)" check — the wizard
writes *there* under that env var, so the path is testable without touching the
contributor's real config. `git checkout scripts/xdg/` undoes the experiment.

The checklist below still needs hand-made variants (no `default`, a `password`
key, a duplicate name); those are a scratch directory and `XDG_CONFIG_HOME`, not
more files in the repo.

---

## The connection screen

```
        ╭─ Connections ──────────────────────────────────╮
        │ Name       Target                    Auth      │
        │ local      local_user@localhost/local_db  none │
        │ staging    app@db.internal/app_prod   keychain │
        │ prod       app@db.prod/app_prod       keychain │
        ╰──────────────────────────────────────── 2/3 ───╯
        enter: connect · n: new · /: filter · q: quit
        ~/.config/easypg/connections.toml
        error: failed to connect to `host=db.prod user=app`:
        dial error: connection refused
```

A `bubbles/table` behind a [`paneBox`](./01-definition-tab.md#design--pane-chrome-iteration-b7)
with columns laid out by [`tableLayout.Fit`](./01-definition-tab.md#responsive-column-widths),
so it inherits the chrome, the responsive widths, the `x/total` position, and
`TableFilter` — `/` filters the list exactly as it does in every other list pane.

The third column reports the **declared** `auth`, and the screen does **not**
probe the vault to say whether a password is actually stored. Asking the keychain
about every profile just to draw a marker pops one permission dialog per row on
macOS — a heavy price for decoration next to what `auth` already says, and paid
at the very moment the user is only trying to see their list. (It also settles the
padlock question: no glyph, hence no `🔒`, which measures two cells in some
terminals and one in others — the lesson the [status bar](./05-keybindings.md#status-bar)
already paid for once with `☰`.)

### Geometry that never moves

Everything on this screen is laid out inside a **block of fixed width** (wider
than the box, which is centred in it), and the box keeps **one height for both
modes** — the list's and the wizard's. Nothing the user does resizes anything:

- Under the box, in this order: the **hint line**, the **path**, then the message
  area. All three start at the **box's own left edge**, not the block's, so the
  screen reads as one column and the keys that drive the pane sit directly under
  it. They may still run wider than the box to the right — which is what a block
  wider than the frame buys.
- The **message area** is last and is 3 lines, always drawn, blank when there is
  nothing to say — the way vim keeps its command line at the bottom. It holds the
  `/` prompt, the password prompt, or a message **wrapped** across its lines: a
  pgx error runs long, and reading its first line only is how one ends up
  guessing at the rest. Beyond 3 lines it is cut.
- The **box height** is `max(list, wizard)`. Switching to the wizard with `n`
  must not resize the frame the cursor is sitting in.
- The wizard draws **one line per field, always**, including the password row —
  which is why changing `auth` from `keychain` to `pgpass` does not make the pane
  jump (see below).

A line that appears with an error moves the box it belongs to, and that reads as
the layout breaking rather than as something having gone wrong.

| Key | Action |
|---|---|
| `enter` | Connect to the highlighted profile |
| `n` | Open the wizard (new profile) |
| `f` | Forget the stored password of the highlighted profile (vault-backed profiles only) |
| `/` | Filter the list |
| `j`/`k`, `g`/`G`, paging | The shared list bindings |
| `q` / `ctrl+c` | Quit — at startup there is nothing behind it |
| `esc` | Back to the current session, when reached from `c` (see below) |

It is a **screen, not a floating overlay**, unlike `?`. The help overlay is
contextual to the pane behind it and must show it; this one owns the keyboard
completely, and the layout behind it is about to be replaced. It also keeps the
mode state machine where it lives today — inside `definitionTabModel` — instead of
requiring the root to override the tab's status bar, which is a seam
[C0](./03-roadmap.md) is going to move anyway. Its hint line is generated from
the keymap (`KeyMap.ConnectionsHelp()`), the way the overlay's is.

### Switching at runtime — `c`

`c` (a free letter, and the initial of what it opens) re-enters the screen from a
live session: pick another profile, `enter`, and the app connects to it. `esc`
returns without touching the current connection.

Reconnecting **rebuilds the definition tab** rather than swapping a pointer inside
it: `newDefinitionTabPage(db)` + its `Init()`, and the old `*pgx.Conn` is closed
once the new one is up. Schema, object, filter and zoom state are deliberately
lost — they describe a database that is no longer on screen, and carrying them
over would show a cursor pointing at a schema the new target may not have.

Connecting is a `tea.Cmd` (`connectedMsg` / `connectErrMsg`), not a blocking call
in `main`: a slow or unreachable host must not freeze the UI, and its error
belongs on the screen the user is already looking at — the first half of what
[E4](./03-roadmap.md) asks for.

---

## The wizard

`n` opens a form — the only way to create a profile, since we deliberately ship no
`easypg conn add` subcommand: this is a TUI, and the screen that lists the
profiles is where a user looks for "add one".

```
╭─ New connection ───────────────────────────╮
│ Name      ▏staging                         │
│ Host      ▏db.internal                     │
│ Port      ▏5432                            │
│ User      ▏app                             │
│ Database  ▏app_prod                        │
│ SSL mode  ▏require                         │
│ Auth      ▏keychain                        │
│ Password  ▏••••••••   (→ vault, not saved  │
│                          to the file)      │
╰ tab: field · ctrl+t: test · ctrl+s: save ──╯
  ctrl+r: show password · esc: cancel
```

A list of `bubbles/textinput`s (`tab` / `shift+tab` between fields — safe, since
the form captures every key), the password one masked and unmasked by `ctrl+r`.
`sslmode` and `auth` cycle through their allowed values rather than accepting
free text.

The password row is **always on screen**, even under the modes that store nothing
with us — the cursor skips it, and it spells out where the password will come
from instead:

| `auth` | The row reads |
|---|---|
| `keychain` | the masked field, `→ vault, never in the file` |
| `pgpass` | `read from ~/.pgpass ($PGPASSFILE) by the driver` |
| `env` | `read from $PGPASSWORD by the driver` |
| `none` | `none — trust, peer, or a client certificate` |

That answers "then how does it authenticate?" where the question is asked rather
than in this document, and it is what keeps the pane from resizing as the choice
changes.

Two orthogonal actions, no cleverness about the order:

- `ctrl+t` (or `F2`) **tests** the current values — a real `sql.Connect`,
  reporting `ok` and the round-trip time, or the driver's error.
- `enter` **saves**: validate, append the profile to `connections.toml`, store
  the password in the vault, then select and connect to it. Enter submits, as in
  any form — the fields are walked with `tab`, so the key is free, and it is the
  one nobody has to be taught.

Saving does not require a successful test. A profile for a host that is down, or
behind a tunnel that is not up yet, is a perfectly legitimate thing to write; a
gate there would just teach users to work around it.

### Writing the file

The profile is **appended** as text — a `[<name>]` table built with
`strconv.Quote`d values, the key bare when it can be and quoted when the name
holds a space or a dot — not re-encoded from a decoded document. Round-tripping
through a TOML encoder would drop the user's comments and reflow their file, on
the one file this design invites them to hand-edit and commit. Appending touches
nothing above the last line.

The corollary: the wizard **creates**, it does not edit or delete. Changing a
profile means editing the file (which is why the screen shows its path); a
delete/edit path would need the round-trip we just refused, and can come later if
it is ever asked for.

---

## Keymap & modes

New in `internal/tui/keys`:

- `Connections` — `c`, "connections", in the **Global** section of `FullHelp`, so
  the screen is discoverable from `?` like everything else.
- `ConnectionsHelp()` and `FormHelp()` — the hint lines of the two screens,
  relabeling the shared keys where they mean something else (`enter` connects,
  `esc` goes back), the same single-declaration rule the overlay follows.
- Form bindings: `NextField` / `PrevField` (`tab` / `shift+tab`), `SaveConn`
  (`enter`), `TestConn` (`ctrl+t` / `F2`), `RevealSecret` (`ctrl+r` / `F3`, in the
  wizard and on the prompt), `StoreSecret` (`ctrl+s` / `F4`, on the prompt, where
  nothing is being written to a file and the key is free).

**Why not `cmd+…` on macOS.** A terminal cannot deliver it: the Command modifier
is not part of the escape sequences terminals send, and the one way to receive it
— the Kitty keyboard protocol's `super` modifier, which bubbletea does model as
`tea.ModSuper` — is implemented by Ghostty, Kitty and WezTerm but not by
Terminal.app, iTerm2 or alacritty. A `cmd+s` binding would therefore work on some
machines and be silently dead on others. The portable second key is a **function
key**, so each action carries an F-alias; it doubles as the way out on terminals
where `ctrl+s` still means XOFF.

Two new mode blocks for the [status bar](./05-keybindings.md#status-bar)
vocabulary, used by the screens' own chrome since they draw no bar of their own:
`CONN` (the picker owns the keys) and `FORM` (the wizard does, insert-like).

The root model's `capturesInput` check must now consider **its own** state, not
only the active tab's: while the picker or the form is up, `q` is a command in one
and a character in the other, and the tab is not even being rendered.

---

## Deliberately out of scope

- **Editing or deleting a profile from the TUI** — see above; the file is the
  editor.
- **Any actual key in `config.toml`** — the file is read and empty on purpose.
- **Remembering the last connection used** across restarts. That is state, not
  config, and it belongs in `$XDG_STATE_HOME/easypg/` next to the query history
  [C1](./03-roadmap.md) has to place anyway. Deciding both at once beats deciding
  this one twice.
- **A connection pool** — still one `*pgx.Conn` behind a mutex
  ([connection.go](../../internal/sql/connection.go)); the Query Tool is what will
  force that question.
- **A `[keys]` section** — [05](./05-keybindings.md) already parks user-configurable
  keybindings on this file's arrival; it stays parked.

---

## Phased breakdown

### A1. Config files & loader
- **Files**: new `internal/config/paths.go` (XDG resolution), `config.go` (general
  config, absent = defaults), `connections.go` (`Connection`, decode, validate);
  `go.mod` (`github.com/BurntSushi/toml` — a decoder is all we need, since we
  never marshal).
- ✅ A hand-written `connections.toml` loads; every validation error is reported at
  once with the file path and the profile name; a `password` key is **rejected**.

### A2. Secrets
- **Files**: new `internal/secrets/keyring.go` (`Get`/`Set` over
  `zalando/go-keyring`); `internal/sql/connection.go`
  (`Connect(dsn, password)` via `ParseConfig` + `ConnectConfig`);
  `internal/config/connections.go` (`Connection.DSN()`, `auth` resolution).
- ✅ A profile with `auth = "keychain"` connects using a secret stored in the
  vault, and the password appears in no connection string; `pgpass` / `env` /
  `none` connect without us setting one.

### A3. Resolution, flags, and the end of the hardcoded DSN
- **Files**: `main.go` (`--connection` / `-c`, config load, **no** `pgUrlString`,
  no blocking connect); `internal/tui/tui.go` (`NewModel(cfg, connections)`,
  `screen` state, async connect via `connectedMsg` / `connectErrMsg`, definition
  tab built on connect); new `scripts/xdg/easypg/connections.toml`;
  `Taskfile.yml` (`run` sets `XDG_CONFIG_HOME=scripts/xdg`); `CLAUDE.md` (the
  paragraph describing the hardcoded DSN, and the dev setup that replaces it).
- ✅ No DSN in the source; `task run` still works on a fresh clone; `easypg` and
  `easypg -c staging` open the screen with the right row selected; an unreachable
  host shows an error instead of hanging or exiting.

### A4. Connection screen
- **Files**: new `internal/tui/connectionsScreen.go` (table + `paneBox` +
  `tableLayout` + `TableFilter`); `internal/tui/keys/keys.go` (`Connections`,
  `ConnectionsHelp`).
- ✅ The screen opens at startup, every profile visible; `enter` connects; `/`
  filters; an empty file shows the empty state; an error never moves the box.

### A5. Wizard
- **Files**: new `internal/tui/connectionForm.go`; new
  `internal/config/write.go` (append-only block writer, creating the directory
  and the file if needed).
- ✅ `n` → fill → `ctrl+t` reports ok/error → `ctrl+s` writes the block (comments
  above untouched), stores the secret, and connects.

### A6. Runtime switch & missing-secret prompt
- **Files**: `internal/tui/tui.go`, `internal/tui/connectionsScreen.go`.
- ✅ `c` reopens the screen from a live session, `enter` switches database (the
  definition tab reloads, the old connection is closed), `esc` returns untouched;
  a `keychain` profile with no stored entry prompts for the password and offers to
  store it.

---

## Manual test checklist

| Action | Expected |
|---|---|
| `task run` on a fresh clone | The screen opens on `scripts/xdg`'s profiles, cursor on the first; `enter` connects — the real `~/.config/easypg` is untouched |
| `XDG_CONFIG_HOME=/tmp/x easypg` | Reads `/tmp/x/easypg/`, leaving the real config alone; writes go there too |
| No config directory at all | The connection screen opens in its empty state, naming the file it will create; `n` works, `q` quits |
| One profile in the file | The screen still opens, cursor on it — never a silent connection |
| Three profiles | All three are **visible** in the box (none hidden behind the frame); `enter` opens the highlighted one |
| `easypg -c prod` | The cursor starts on `prod` |
| `easypg -c typo` | Startup error naming the unknown profile and listing the valid names |
| A leftover `default = "local"` at the root | Startup error saying this file holds nothing but `[name]` tables |
| A `password = "..."` key in the file | Startup **fails** with the message pointing at `auth` / the vault; the app does not connect |
| A `dsn` containing a password | Connects, and `app.log` carries a warning about the file being unsafe to commit |
| Both `dsn` and `host`/`user` on one profile | The `dsn` is used; `app.log` says the fields were ignored |
| Two `[local]` tables | The decoder's own duplicate-key error, naming the line |
| Two profiles each missing a field | Both problems reported in one message, with the profile names |
| Profiles reordered in the file | The picker's rows follow the file, not a shuffled map |
| `auth = "keychain"` with nothing in the vault | Masked prompt; the password connects and is stored — a relaunch no longer asks |
| Same, then delete the vault entry | It asks again (and nothing was left in the file) |
| `ctrl+r` on the prompt | The password becomes readable, the hint flips to `hide password`, `ctrl+r` masks it again |
| `ctrl+s` on the prompt, then `enter` | The hint reads `remember: off`; it connects, and the next launch asks again |
| A stored password the server refuses | The prompt reopens, saying it was refused; the new one overwrites and connects |
| `f` on a `keychain` profile | `password forgotten` — the next connection asks again |
| `f` on a `pgpass` / `env` / `none` profile | Nothing happens, and the key is absent from the hint line |
| Delete a profile from the file, re-create it with the same name | The wizard's password overwrites the old vault entry — no manual keychain trip |
| `ctrl+s` in the wizard with `auth = "keychain"` and an empty password | Refused, with a message pointing at the other three modes; nothing written |
| An unreachable host, then the error | The box does **not** move: the message line was already there, blank |
| A multi-line driver error | Collapsed to one truncated line — the layout never grows |
| `app.log` after typing a password | No occurrence of it: the messages that carry one redact it (`password: (set)`) |
| `auth = "pgpass"` with a matching `~/.pgpass` line | Connects with no prompt |
| `auth = "none"` on a trust/peer local socket | Connects with no prompt |
| Vault unreachable (Linux, no Secret Service) | An error naming the vault and suggesting `pgpass` / `env`, not a raw dbus error |
| An unreachable host | The error appears on the screen; the app neither hangs nor exits |
| `n`, fill everything, `ctrl+t` | `ok` + the round-trip time, or the driver's error, in the form |
| `n`, a host that is down, `ctrl+s` | Saved and selected anyway; the connect error is shown |
| `ctrl+s` with an empty name | Field-level error, nothing written |
| A file with comments above the profiles, then `ctrl+s` | The comments and the formatting above are byte-identical; the new block sits at the end |
| `ctrl+s` with `auth = "keychain"` | The block written has **no** `password` key, and the vault has the entry |
| `esc` in the form | Back to the list, nothing written |
| `c` from a live session, `esc` | Same schema, same object, same cursor — nothing reconnected |
| `c`, pick another profile, `enter` | The definition tab reloads on the new database from the schema list down; `app.log` shows the old connection closed |
| `?` on any pane | Lists `c` under **Global** |
| `q` in the picker vs. in the form's name field | Quits in the first, types a letter in the second |
