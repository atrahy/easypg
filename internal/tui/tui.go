package tui

import (
	"errors"
	"fmt"
	"log"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/atrahy/easypg/internal/config"
	"github.com/atrahy/easypg/internal/secrets"
	"github.com/atrahy/easypg/internal/sql"
	"github.com/atrahy/easypg/internal/tui/keys"
)

// inputCapturer is implemented by tabs that can swallow every key press (a
// search prompt being typed, a modal overlay). The global quit key stands down
// while one does, so "q" typed into a prompt is text, not a command.
type inputCapturer interface {
	CapturesInput() bool
}

// screen is what the root is showing: the connection picker, or the tabs. The
// picker is a screen rather than an overlay because it owns the keyboard
// outright and the layout behind it is about to be replaced.
type screen int

const (
	screenConnections screen = iota
	screenTab
)

// connectedMsg / connectErrMsg / secretNeededMsg are the three outcomes of a
// connection attempt. Connecting is a Cmd rather than a blocking call in main:
// a slow or unreachable host must not freeze the UI, and its error belongs on
// the screen the user is already looking at.
type connectedMsg struct {
	conn config.Connection
	db   *sql.DBConnection
}

type connectErrMsg struct {
	conn config.Connection
	err  error
}

// secretNeededMsg is the vault working and simply holding nothing yet — the
// case that makes a committed connections.toml usable on a second machine.
type secretNeededMsg struct {
	conn config.Connection
}

type Model struct {
	width, height int

	// dir is the config directory: where the wizard writes.
	dir string
	cfg config.Config

	db *sql.DBConnection

	screen      screen
	connections *connectionsScreen

	tabCursor tabCursor

	definitionTab tab
	// editorTab     tab
}

// NewModel starts on the connection screen — always, even with a single profile
// configured. The screen is where one sees what is configured and switches, and
// skipping it whenever it has a single answer means the one launch that *should*
// have shown it is the one where you cannot tell why it did.
//
// target, resolved from -c, only places the cursor.
func NewModel(dir string, cfg config.Config, conns config.Connections, target *config.Connection) Model {
	m := Model{
		dir:         dir,
		cfg:         cfg,
		connections: newConnectionsScreen(dir, conns),
		tabCursor:   definitionTab,
	}

	m.connections.Open(false)

	if target != nil {
		m.connections.Select(target.Name)
	}

	return m
}

func (m Model) Init() tea.Cmd {
	return nil
}

// Close releases the connection once the program has stopped. The root owns the
// connection now (it can swap it at runtime), so main cannot defer a Close on
// something it never opened.
func (m Model) Close() {
	if m.db == nil {
		return
	}

	if err := m.db.Close(); err != nil {
		log.Printf("closing the connection: %v", err)
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.connections.SetSize(msg.Width, msg.Height)

	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, keys.Default.ForceQuit):
			return m, tea.Quit
		case key.Matches(msg, keys.Default.Quit) && !m.capturesInput():
			return m, tea.Quit
		case m.screen == screenTab && key.Matches(msg, keys.Default.Connections) && !m.capturesInput():
			m.openConnections()

			return m, nil
		}

		return m.routeKey(msg)

	case connectRequestMsg:
		return m, connectCmd(msg.conn, msg.password)

	// The screen's own traffic stops here. It carries a password, and everything
	// that reaches a tab is logged verbatim.
	case connSavedMsg, connTestResultMsg, secretForgottenMsg:
		return m, m.connections.Update(msg)

	case connectedMsg:
		return m.connected(msg)

	case connectErrMsg:
		m.openConnections()

		// A password the server refused is the one failure the user can fix right
		// here. Without this, a wrong entry in the vault would be a dead end: it
		// exists, so nothing would ever ask for it again.
		if msg.conn.Auth == config.AuthKeychain && sql.IsAuthError(msg.err) {
			return m, m.connections.PromptSecret(msg.conn, "the stored password was refused — type it again")
		}

		m.connections.SetError(fmt.Errorf("%s: %w", msg.conn.Name, msg.err))

		return m, nil

	case secretNeededMsg:
		m.openConnections()

		return m, m.connections.PromptSecret(msg.conn, "")

	case connectionsClosedMsg:
		if m.db != nil {
			m.screen = screenTab
		}

		return m, nil
	}

	// Everything that is not a key press reaches both: a fetch that was already
	// in flight when the connection screen opened must still land on the tab
	// instead of being dropped, which would leave it showing stale rows.
	cmds = append(cmds, m.connections.Update(msg))

	if m.definitionTab != nil {
		t, cmd := m.definitionTab.Update(msg)
		m.definitionTab = t
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// routeKey gives the key press to whichever screen holds the keyboard — never
// to both.
func (m Model) routeKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.screen == screenConnections || m.definitionTab == nil {
		return m, m.connections.Update(msg)
	}

	t, cmd := m.definitionTab.Update(msg)
	m.definitionTab = t

	return m, cmd
}

// connected swaps the live connection. The definition tab is rebuilt rather than
// pointed at the new database: its schema, object and filter state describes a
// database that is no longer on screen.
func (m Model) connected(msg connectedMsg) (tea.Model, tea.Cmd) {
	previous := m.db

	m.db = msg.db
	m.screen = screenTab

	// The size is pushed before the first render: the tab only learns it from a
	// WindowSizeMsg, and the terminal sends one at startup, not on every rebuild.
	page, _ := newDefinitionTabPage(msg.db).Update(tea.WindowSizeMsg{Width: m.width, Height: m.height})
	m.definitionTab = page

	cmds := []tea.Cmd{page.Init()}

	if previous != nil {
		cmds = append(cmds, closeCmd(previous))
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) openConnections() {
	m.connections.Open(m.db != nil)
	m.screen = screenConnections
}

// View returns the frame *and* the terminal state it wants: since v2 the
// alt-screen is declared here on every render instead of being switched on once
// as a program option.
func (m Model) View() tea.View {
	var page = lipgloss.NewStyle().Width(m.width).Height(m.height)

	content := m.connections.View()

	if m.screen == screenTab && m.definitionTab != nil {
		content = m.getCurrentTab().View()
	}

	view := tea.NewView(page.Render(content))
	view.AltScreen = true

	return view
}

func (m Model) capturesInput() bool {
	if m.screen == screenConnections || m.definitionTab == nil {
		return m.connections.CapturesInput()
	}

	capturer, ok := m.getCurrentTab().(inputCapturer)

	return ok && capturer.CapturesInput()
}

func (m Model) getCurrentTab() tab {
	switch m.tabCursor {
	case definitionTab:
		return m.definitionTab
		// case editorTab:
		// 	m.editorTab.Update()
	}

	return m.definitionTab
}

// connectCmd resolves the password, then opens the connection. An empty password
// means "look it up": the caller only passes one when the user has just typed it.
func connectCmd(conn config.Connection, password string) tea.Cmd {
	return func() tea.Msg {
		if password == "" {
			stored, err := passwordFor(conn)

			switch {
			case errors.Is(err, secrets.ErrNotFound):
				return secretNeededMsg{conn: conn}
			case err != nil:
				return connectErrMsg{conn: conn, err: err}
			}

			password = stored
		}

		db, err := sql.Connect(conn.ConnString(), password)
		if err != nil {
			return connectErrMsg{conn: conn, err: err}
		}

		log.Printf("connected to %q (%s)", conn.Name, conn.Target())

		return connectedMsg{conn: conn, db: db}
	}
}

// passwordFor reads the vault, and only for the profiles that declare it: the
// other modes set no password at all, leaving pgx to read ~/.pgpass or
// $PGPASSWORD itself.
func passwordFor(conn config.Connection) (string, error) {
	if conn.Auth != config.AuthKeychain {
		return "", nil
	}

	return secrets.Get(conn.Name)
}

func closeCmd(db *sql.DBConnection) tea.Cmd {
	return func() tea.Msg {
		if err := db.Close(); err != nil {
			log.Printf("closing the previous connection: %v", err)
		}

		return nil
	}
}
