package tui

import (
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/table"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/atrahy/easypg/internal/config"
	"github.com/atrahy/easypg/internal/secrets"
	"github.com/atrahy/easypg/internal/tui/components/paneBox"
	"github.com/atrahy/easypg/internal/tui/components/search"
	"github.com/atrahy/easypg/internal/tui/components/searchBar"
	"github.com/atrahy/easypg/internal/tui/components/tableLayout"
	"github.com/atrahy/easypg/internal/tui/keys"
	"github.com/charmbracelet/x/ansi"
)

// The connection screen is a screen, not a floating overlay like "?": it owns
// the keyboard completely, and the layout behind it is about to be replaced.
// That also keeps the mode state machine inside definitionTabModel, where it
// lives until the multi-tab work moves it (docs/spec/07-connections.md).

// connectRequestMsg asks the root to open a connection. A password means the
// user just typed it; empty means the root should resolve it the usual way.
type connectRequestMsg struct {
	conn     config.Connection
	password string
}

// String keeps the password out of app.log. Every message that reaches a tab is
// logged with %v (definitionTabModel.Update), and a Stringer is what that verb
// calls — so the redaction holds wherever the message travels, rather than
// depending on nobody ever logging it.
func (m connectRequestMsg) String() string {
	return fmt.Sprintf("connectRequestMsg{conn: %s, password: %s}", m.conn.Name, redacted(m.password))
}

func redacted(password string) string {
	if password == "" {
		return "(none)"
	}

	return "(set)"
}

// connectionsClosedMsg is "esc" with a live session behind the screen.
type connectionsClosedMsg struct{}

type connTestResultMsg struct {
	err  error
	took time.Duration
}

// secretForgottenMsg is the outcome of dropping a vault entry.
type secretForgottenMsg struct {
	name string
	err  error
}

type connSavedMsg struct {
	conn     config.Connection
	password string
	err      error
}

func (m connSavedMsg) String() string {
	return fmt.Sprintf("connSavedMsg{conn: %s, password: %s, err: %v}", m.conn.Name, redacted(m.password), m.err)
}

// connMode is which part of the screen holds the keyboard.
type connMode int

const (
	connList connMode = iota
	connFilter
	connSecret
	connForm
)

// connectionSpecs are all the list needs now that the details pane spells the
// rest out: what the profile is called, and how it authenticates. The target
// moved out because a DSN cannot be shown truncated to a cell and still be worth
// reading.
var connectionSpecs = []tableLayout.Spec{
	{Title: "Name", Min: 10, Weight: 3},
	{Title: "Auth", Min: 8, Weight: 1},
}

const (
	// connBlockWidth caps everything drawn on this screen: the two panes side by
	// side, and the lines under them, all span exactly this. Stretching a list of
	// two columns over 200 cells is harder to read, not easier.
	connBlockWidth = 100
	// connListWidth bounds the left pane. The list holds two short columns; the
	// space is worth more to the details pane, where a DSN has to fit.
	connListMinWidth = 16
	connListMaxWidth = 34
	// connGap is the column between the two panes, plus the two borders each of
	// them draws.
	connGap = 1
	connFrame = 2
	// connMessageLines / connHintLines are the fixed heights of the two areas
	// under the panes. Fixed, so nothing above them moves; more than one line, so
	// a driver error and a long hint are read rather than cut.
	connMessageLines = 3
	connHintLines    = 2
	// connMinRows / connMaxRows bound the list's height around its content.
	connMinRows = 3
	connMaxRows = 12
	// headerHeight is what a bubbles/table header costs: the titles plus the
	// border line under them.
	headerHeight = 2
	// formHeight is the wizard's fixed line count: one per field, the password
	// row included whatever the auth mode says.
	formHeight = fieldCount
)

var (
	connHintStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	connPathStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("237"))
	connErrStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("204"))
	connNoticeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	connEmptyStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	connPromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
)

type connectionsScreen struct {
	width, height int

	// dir is where connections.toml is written; path is the file itself, shown
	// under the box so "edit it yourself" is always an available answer.
	dir  string
	path string

	conns []config.Connection

	table  table.Model
	filter search.TableFilter

	prompt *searchBar.Model

	secret    textinput.Model
	secretFor config.Connection
	// secretReason says why the prompt is up when it is not simply the first
	// time — a password the server rejected, most of all.
	secretReason string
	// secretStore is whether the typed password goes to the vault. It defaults to
	// on — that is what auth = "keychain" declares — but a borrowed or shared
	// machine is reason enough to decline it for one connection.
	secretStore bool
	// secretReveal unmasks the field: a password one cannot read is a password
	// one retypes from scratch on the first typo.
	secretReveal bool

	form *connectionForm

	mode connMode

	// canReturn is true when a live session sits behind the screen (it was
	// reached with "c"), so "esc" means "back" rather than nothing.
	canReturn bool

	err    error
	notice string
}

func newConnectionsScreen(dir string, conns config.Connections) *connectionsScreen {
	t := table.New(
		table.WithColumns(tableLayout.Fit(0, connectionSpecs)),
		table.WithFocused(true),
		table.WithKeyMap(keys.TableKeyMap(keys.Default)),
	)

	styles := table.DefaultStyles()
	styles.Header = styles.Header.BorderForeground(lipgloss.Color("240")).Bold(false)
	styles.Selected = styles.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	t.SetStyles(styles)

	secret := textinput.New()
	secret.EchoMode = textinput.EchoPassword

	screen := &connectionsScreen{
		dir:    dir,
		path:   conns.Path,
		conns:  conns.Connections,
		table:  t,
		prompt: searchBar.New(),
		secret: secret,
	}

	screen.setRows()

	return screen
}

// setRows rebuilds the list from the profiles.
//
// The Auth column reports the declared mode and does not probe the vault: asking
// the keychain about every profile just to draw a "stored" marker would pop one
// permission dialog per row on macOS, for information that is decoration next to
// what auth already says.
func (s *connectionsScreen) setRows() {
	rows := make([]table.Row, 0, len(s.conns))

	for _, conn := range s.conns {
		rows = append(rows, table.Row{conn.Name, string(conn.Auth)})
	}

	s.table.SetRows(rows)
	s.updateHeight()
}

// Open (re)enters the screen. canReturn records whether there is a session to go
// back to.
func (s *connectionsScreen) Open(canReturn bool) {
	s.canReturn = canReturn
	s.mode = connList
	s.form = nil
	s.err = nil
}

// PromptSecret asks for the password of a profile whose vault entry is missing —
// the case that makes a committed connections.toml usable on a second machine —
// or whose stored one the server has just rejected, which reason names.
func (s *connectionsScreen) PromptSecret(conn config.Connection, reason string) tea.Cmd {
	s.mode = connSecret
	s.secretFor = conn
	s.secretReason = reason
	s.err = nil
	s.notice = ""
	s.secret.SetValue("")
	s.secretStore = true
	s.secretReveal = false
	s.secret.EchoMode = textinput.EchoPassword
	s.sizeSecret()

	return s.secret.Focus()
}

func (s *connectionsScreen) SetError(err error) {
	s.err = err
	s.notice = ""

	if s.mode == connSecret {
		s.mode = connList
	}
}

// CapturesInput is true while a prompt or the wizard owns the keyboard, so the
// root's global "q" stands down and the letter is typed instead.
func (s *connectionsScreen) CapturesInput() bool {
	return s.mode != connList
}

func (s *connectionsScreen) SetSize(width, height int) {
	s.width, s.height = width, height

	list := s.listWidth()

	s.table.SetWidth(list)
	s.table.SetColumns(tableLayout.Fit(list, connectionSpecs))
	s.prompt.SetWidth(s.blockWidth())
	s.sizeSecret()

	if s.form != nil {
		s.form.SetWidth(s.boxWidth())
	}

	s.updateHeight()
}

// blockWidth is the rendered width of everything this screen draws — the two
// panes with their borders, and every line under them. Keeping it independent of
// the content is what stops a message from moving what sits above it.
func (s *connectionsScreen) blockWidth() int {
	return min(max(s.width-4, 40), connBlockWidth)
}

// listWidth / detailWidth are the *content* widths of the two panes; together
// with their borders and the gap between them they add up to blockWidth.
func (s *connectionsScreen) listWidth() int {
	return min(max(s.blockWidth()*2/5, connListMinWidth), connListMaxWidth)
}

func (s *connectionsScreen) detailWidth() int {
	return s.blockWidth() - s.listWidth() - 2*connFrame - connGap
}

// boxWidth is the content width of a pane spanning the whole block — the wizard,
// which replaces both.
func (s *connectionsScreen) boxWidth() int {
	return s.blockWidth() - connFrame
}

// sizeSecret fits the password field next to its prompt, which names the target
// and is therefore only known once the prompt is opened.
func (s *connectionsScreen) sizeSecret() {
	s.secret.SetWidth(max(s.blockWidth()-ansi.StringWidth(s.secretPrompt())-1, 8))
}

// updateHeight sizes the list around its content, between a floor that keeps the
// box from looking broken and a ceiling that keeps it a box rather than a page.
//
// It counts the profiles, not the visible rows: a box that shrinks under the
// prompt with every keystroke of a filter is harder to read than one that keeps
// its size and empties.
func (s *connectionsScreen) updateHeight() {
	s.table.SetHeight(s.contentHeight())
}

// listHeight counts the header block, which is two lines and not one:
// table.DefaultStyles gives the header a bottom border. Sizing for one hid the
// last profile behind the frame.
func (s *connectionsScreen) listHeight() int {
	return min(max(len(s.conns), connMinRows), connMaxRows) + headerHeight
}

// contentHeight is deliberately the same in both modes: opening the wizard must
// not resize the box the cursor is sitting in.
func (s *connectionsScreen) contentHeight() int {
	return max(s.listHeight(), formHeight)
}

func (s *connectionsScreen) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return s.handleKey(msg)

	case connTestResultMsg:
		if s.form != nil {
			s.form.setTestResult(msg)
		}

	case connSavedMsg:
		return s.handleSaved(msg)

	case secretForgottenMsg:
		s.handleForgotten(msg)
	}

	return nil
}

func (s *connectionsScreen) handleKey(msg tea.KeyPressMsg) tea.Cmd {
	switch s.mode {
	case connFilter:
		return s.handleFilterKey(msg)
	case connSecret:
		return s.handleSecretKey(msg)
	case connForm:
		return s.handleFormKey(msg)
	default:
		return s.handleListKey(msg)
	}
}

func (s *connectionsScreen) handleListKey(msg tea.KeyPressMsg) tea.Cmd {
	s.err = nil
	s.notice = ""

	switch {
	case key.Matches(msg, keys.Default.AcceptSearch):
		return s.connectSelected()

	case key.Matches(msg, keys.Default.NewConn):
		s.openForm()

		return nil

	case key.Matches(msg, keys.Default.ForgetSecret):
		return s.forgetSecret()

	case key.Matches(msg, keys.Default.Search):
		return s.startFilter()

	case key.Matches(msg, keys.Default.Cancel):
		if !s.canReturn {
			return nil
		}

		return func() tea.Msg { return connectionsClosedMsg{} }
	}

	s.table, _ = s.table.Update(msg)

	return nil
}

func (s *connectionsScreen) handleFilterKey(msg tea.KeyPressMsg) tea.Cmd {
	switch {
	case key.Matches(msg, keys.Default.Cancel):
		s.filter.Clear(&s.table)
		s.prompt.Stop()
		s.mode = connList

		return nil

	case key.Matches(msg, keys.Default.AcceptSearch):
		// Confirming keeps the narrowed list: the rows stay hidden until esc.
		s.prompt.Stop()
		s.mode = connList

		return nil
	}

	cmd := s.prompt.Update(msg)

	rows := s.filter.Apply(&s.table, s.prompt.Value())
	s.prompt.SetStatus(filterStatus(rows, s.prompt.Value()))

	return cmd
}

func (s *connectionsScreen) handleSecretKey(msg tea.KeyPressMsg) tea.Cmd {
	switch {
	case key.Matches(msg, keys.Default.Cancel):
		s.secret.Blur()
		s.mode = connList

		return nil

	case key.Matches(msg, keys.Default.RevealSecret):
		s.secretReveal = !s.secretReveal
		s.secret.EchoMode = echoMode(s.secretReveal)

		return nil

	case key.Matches(msg, keys.Default.StoreSecret):
		s.secretStore = !s.secretStore

		return nil

	case key.Matches(msg, keys.Default.AcceptSearch):
		password := s.secret.Value()

		s.secret.Blur()
		s.secret.SetValue("")
		s.mode = connList

		return connectWithPasswordCmd(s.secretFor, password, s.secretStore)
	}

	var cmd tea.Cmd
	s.secret, cmd = s.secret.Update(msg)

	return cmd
}

func (s *connectionsScreen) handleFormKey(msg tea.KeyPressMsg) tea.Cmd {
	switch {
	case key.Matches(msg, keys.Default.Cancel):
		s.form = nil
		s.mode = connList

		return nil

	case key.Matches(msg, keys.Default.SaveConn):
		return s.save()
	}

	return s.form.Update(msg)
}

func (s *connectionsScreen) openForm() {
	s.form = newConnectionForm()
	s.form.SetWidth(s.boxWidth())
	s.mode = connForm
}

func (s *connectionsScreen) startFilter() tea.Cmd {
	s.mode = connFilter

	cmd := s.prompt.Start("/", "filter connections")
	s.prompt.SetStatus(filterStatus(len(s.table.Rows()), ""))

	return cmd
}

func (s *connectionsScreen) connectSelected() tea.Cmd {
	conn, ok := s.selected()
	if !ok {
		return nil
	}

	return connectRequestCmd(conn, "")
}

// forgetSecret drops the selected profile's vault entry. The next connection
// prompts again — which is the way out when the stored password is the wrong one,
// and the way to reuse a name whose old entry would otherwise be inherited.
func (s *connectionsScreen) forgetSecret() tea.Cmd {
	conn, ok := s.selected()
	if !ok || conn.Auth != config.AuthKeychain {
		return nil
	}

	return func() tea.Msg {
		return secretForgottenMsg{name: conn.Name, err: secrets.Delete(conn.Name)}
	}
}

func (s *connectionsScreen) handleForgotten(msg secretForgottenMsg) {
	switch {
	case errors.Is(msg.err, secrets.ErrNotFound):
		s.notice = fmt.Sprintf("nothing was stored for %q", msg.name)
	case msg.err != nil:
		s.err = msg.err
	default:
		s.notice = fmt.Sprintf("password forgotten for %q — it will be asked again", msg.name)
	}
}

// usesVault says whether the highlighted profile reads the vault, which is what
// makes "forget" worth advertising.
func (s *connectionsScreen) usesVault() bool {
	conn, ok := s.selected()

	return ok && conn.Auth == config.AuthKeychain
}

func (s *connectionsScreen) selected() (config.Connection, bool) {
	index := s.filter.SourceIndex(s.table.Cursor())
	if index < 0 || index >= len(s.conns) {
		return config.Connection{}, false
	}

	return s.conns[index], true
}

// save validates the form, then hands the writing to a Cmd: the file write is
// quick, but the vault behind it can stop to ask the user for permission.
func (s *connectionsScreen) save() tea.Cmd {
	conn, err := s.form.connection()
	if err != nil {
		s.form.setError(err)

		return nil
	}

	for _, existing := range s.conns {
		if existing.Name == conn.Name {
			s.form.setError(fmt.Errorf("a connection named %q is already in %s", conn.Name, s.path))

			return nil
		}
	}

	return saveConnectionCmd(s.dir, conn, s.form.password())
}

func (s *connectionsScreen) handleSaved(msg connSavedMsg) tea.Cmd {
	if msg.err != nil {
		if s.form != nil {
			s.form.setError(msg.err)
		}

		return nil
	}

	s.conns = append(s.conns, msg.conn)
	s.filter.Reset()
	s.setRows()
	s.table.SetCursor(len(s.conns) - 1)

	s.form = nil
	s.mode = connList

	return connectRequestCmd(msg.conn, msg.password)
}

func (s *connectionsScreen) View() string {
	if s.width <= 0 || s.height <= 0 {
		return ""
	}

	width := s.blockWidth()

	// The panes, then the lines that describe them — all exactly as wide as the
	// block, so everything shares one left edge and one right edge. The hint
	// wraps inside that width instead of running past the pane it documents, and
	// the message area keeps its three lines the way vim keeps its command line:
	// prompts and errors in one fixed place, so nothing above them ever moves.
	block := lipgloss.JoinVertical(lipgloss.Left,
		s.panesView(),
		s.hintArea(width),
		connPathStyle.Render(ansi.Truncate(s.path, width, "…")),
		s.messageArea(width),
	)

	return lipgloss.Place(s.width, s.height, lipgloss.Center, lipgloss.Center, block)
}

// panesView is the list beside the details of its selected row — or the wizard,
// which takes the whole width. The height is the same in every case, so opening
// one never resizes the other.
func (s *connectionsScreen) panesView() string {
	height := s.contentHeight()

	if s.mode == connForm {
		box := paneBox.Box{Title: "New connection", Width: s.boxWidth(), Height: height, Focused: true}

		return box.Render(s.form.View())
	}

	list := paneBox.Box{Title: "Connections", Width: s.listWidth(), Height: height, Focused: true}
	list.Current, list.Total = tableLayout.Position(s.table)

	details := paneBox.Box{Title: "Details", Width: s.detailWidth(), Height: height}

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		list.Render(s.listView()),
		strings.Repeat(" ", connGap),
		details.Render(s.detailsView()),
	)
}

// detailsView spells out the highlighted profile in full: the point of the pane
// is that nothing about a connection has to be read through a truncated cell —
// least of all a DSN, which is the one part that cannot be guessed from the
// others.
func (s *connectionsScreen) detailsView() string {
	conn, ok := s.selected()
	if !ok {
		return connEmptyStyle.Render("No connection selected.")
	}

	width := s.detailWidth()
	rows := []string{detailRow("Name", conn.Name, width)}

	if conn.DSN != "" {
		rows = append(rows, detailRows("DSN", conn.SafeDSN(), width, s.contentHeight()-3)...)
	} else {
		rows = append(rows,
			detailRow("Host", conn.Host, width),
			detailRow("Port", strconv.Itoa(conn.Port), width),
			detailRow("User", conn.User, width),
			detailRow("Database", conn.DBName, width),
			detailRow("SSL mode", orDefault(conn.SSLMode), width),
		)
	}

	rows = append(rows,
		detailRow("Auth", string(conn.Auth), width),
		detailRow("Password", passwordSource(conn), width),
	)

	return strings.Join(rows, "\n")
}

// passwordSource names the vault entry rather than only the mode: which account
// holds this profile's password is exactly what one needs to know when a name is
// reused or a stored password goes stale.
func passwordSource(conn config.Connection) string {
	if conn.Auth == config.AuthKeychain {
		return "vault · easypg/" + conn.Name
	}

	return secretSource(conn.Auth)
}

func detailRow(label, value string, width int) string {
	return ansi.Truncate(formLabelStyle.Render(label)+value, width, "…")
}

// detailRows is the same, for a value long enough to need the lines below it.
func detailRows(label, value string, width, limit int) []string {
	wrapped := strings.Split(ansi.Wrap(value, max(width-labelWidth, 8), " :/@?&="), "\n")

	if len(wrapped) > limit {
		wrapped = wrapped[:limit]
		wrapped[limit-1] = ansi.Truncate(wrapped[limit-1], width-labelWidth-1, "…")
	}

	rows := make([]string, 0, len(wrapped))

	for i, line := range wrapped {
		title := ""
		if i == 0 {
			title = label
		}

		rows = append(rows, formLabelStyle.Render(title)+line)
	}

	return rows
}

func orDefault(value string) string {
	if value == "" {
		return "(default)"
	}

	return value
}

// hintArea wraps the key hints inside the block instead of letting them run past
// it, over a fixed number of lines so the panes above never move.
func (s *connectionsScreen) hintArea(width int) string {
	lines := wrapMessage(keys.RenderShort(s.hints()), width, connHintStyle, connHintLines)

	for len(lines) < connHintLines {
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}

func (s *connectionsScreen) listView() string {
	if s.mode == connForm {
		return s.form.View()
	}

	if len(s.conns) == 0 {
		return s.emptyView()
	}

	return s.table.View()
}

// emptyView is the first-run state: there is no file yet, and the one thing to
// do about it is one key away.
func (s *connectionsScreen) emptyView() string {
	return connEmptyStyle.Render(strings.Join([]string{
		"No connection yet.",
		"",
		"Press n to create one — it will be written to the file below.",
	}, "\n"))
}

// messageArea is the transient part: the prompt that has the keyboard, or a
// message wrapped over the lines it needs. State belongs in the box above it.
func (s *connectionsScreen) messageArea(width int) string {
	lines := s.messageLines(width)

	for len(lines) < connMessageLines {
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}

func (s *connectionsScreen) messageLines(width int) []string {
	switch s.mode {
	case connFilter:
		return []string{ansi.Truncate(s.prompt.View(), width, "…")}

	case connSecret:
		lines := []string{ansi.Truncate(connPromptStyle.Render(s.secretPrompt())+s.secret.View(), width, "…")}

		// Why the prompt is up, under the prompt itself: "the stored password was
		// refused" is the difference between retyping it and wondering.
		return append(lines, wrapMessage(s.secretReason, width, connErrStyle, connMessageLines-1)...)
	}

	text, style := s.messageText()

	return wrapMessage(text, width, style, connMessageLines)
}

// wrapMessage lays a message over at most limit lines: a pgx error runs long,
// and reading its first line only is how one ends up guessing at the rest.
func wrapMessage(text string, width int, style lipgloss.Style, limit int) []string {
	if text == "" || limit <= 0 {
		return nil
	}

	lines := strings.Split(ansi.Wrap(oneLine(text), width, " -,()"), "\n")

	if len(lines) > limit {
		lines = lines[:limit]
		lines[limit-1] = ansi.Truncate(lines[limit-1], width-1, "…")
	}

	for i, line := range lines {
		lines[i] = style.Render(line)
	}

	return lines
}

// messageText picks what to say, and how: the wizard's own error or
// confirmation while it is open, the screen's otherwise.
func (s *connectionsScreen) messageText() (string, lipgloss.Style) {
	if s.mode == connForm {
		text, isErr := s.form.message()

		switch {
		case text == "":
			return "", connHintStyle
		case isErr:
			return "error: " + text, connErrStyle
		default:
			return text, connNoticeStyle
		}
	}

	if s.err != nil {
		return "error: " + s.err.Error(), connErrStyle
	}

	return s.notice, connNoticeStyle
}

func oneLine(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

func (s *connectionsScreen) secretPrompt() string {
	return fmt.Sprintf("password for %s: ", s.secretFor.Target())
}

func (s *connectionsScreen) hints() []key.Binding {
	switch s.mode {
	case connForm:
		return keys.Default.FormHelp(s.form.hasSecret(), s.form.revealed())
	case connSecret:
		return keys.Default.SecretHelp(s.secretStore, s.secretReveal)
	default:
		return keys.Default.ConnectionsHelp(s.canReturn, s.usesVault())
	}
}

// Select puts the cursor on a profile — what the resolved target now does
// instead of connecting on its own.
func (s *connectionsScreen) Select(name string) {
	for i, conn := range s.conns {
		if conn.Name == name {
			s.table.SetCursor(i)

			return
		}
	}
}

func connectRequestCmd(conn config.Connection, password string) tea.Cmd {
	return func() tea.Msg {
		return connectRequestMsg{conn: conn, password: password}
	}
}

// connectWithPasswordCmd connects with the password just typed, storing it first
// unless the user turned that off — in which case it lives only as long as this
// session, and the prompt comes back next time.
//
// A vault that refuses the write is logged rather than fatal: the user asked to
// connect, and the password they typed is enough to do that now.
func connectWithPasswordCmd(conn config.Connection, password string, store bool) tea.Cmd {
	return func() tea.Msg {
		if store {
			if err := secrets.Set(conn.Name, password); err != nil {
				log.Printf("secrets: cannot store the password of %q: %v", conn.Name, err)
			}
		}

		return connectRequestMsg{conn: conn, password: password}
	}
}

func echoMode(revealed bool) textinput.EchoMode {
	if revealed {
		return textinput.EchoNormal
	}

	return textinput.EchoPassword
}

// saveConnectionCmd appends the profile to connections.toml, then stores its
// password. The order matters: a vault entry for a profile that was never
// written would be an orphan nothing on screen could explain.
func saveConnectionCmd(dir string, conn config.Connection, password string) tea.Cmd {
	return func() tea.Msg {
		if err := config.Append(dir, conn); err != nil {
			return connSavedMsg{err: err}
		}

		if conn.Auth == config.AuthKeychain && password != "" {
			if err := secrets.Set(conn.Name, password); err != nil {
				return connSavedMsg{conn: conn, err: fmt.Errorf("connection saved, but %w", err)}
			}
		}

		return connSavedMsg{conn: conn, password: password}
	}
}
