package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/atrahy/easypg/internal/config"
	"github.com/atrahy/easypg/internal/sql"
	"github.com/atrahy/easypg/internal/tui/keys"
	"github.com/charmbracelet/x/ansi"
)

// The wizard is the only way to create a profile: this is a TUI, and the screen
// that lists the connections is where one looks for "add one". It writes the
// profile to connections.toml and the password to the system vault — never the
// other way round.

// fieldKind tells how a field is edited: typed, typed but masked, or cycled
// through a closed set of values (sslmode, auth), which have no free-text form
// worth allowing.
type fieldKind int

const (
	kindText fieldKind = iota
	kindPassword
	kindChoice
)

// Field order, top to bottom.
const (
	fieldName = iota
	fieldHost
	fieldPort
	fieldUser
	fieldDBName
	fieldSSLMode
	fieldAuth
	fieldSecret
	fieldCount
)

const (
	// labelWidth is the fixed width of the label column, so the inputs line up.
	labelWidth = 10
	// cursorCell is the column a textinput keeps past its value for the cursor.
	cursorCell = 1
	// secretNote trails the password field: what happens to what you type is
	// worth saying where you type it.
	secretNote = "  → vault, never in the file"
)

var (
	formLabelStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Width(labelWidth)
	formChoiceStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("229"))
	formNoteStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

type formField struct {
	label string
	kind  fieldKind

	input textinput.Model

	choices []string
	choice  int
}

func (f formField) value() string {
	switch f.kind {
	case kindChoice:
		return f.choices[f.choice]
	case kindPassword:
		// Not trimmed: leading or trailing spaces are part of a password.
		return f.input.Value()
	default:
		return strings.TrimSpace(f.input.Value())
	}
}

type connectionForm struct {
	fields []formField
	focus  int

	width int

	// reveal unmasks the password field, so a typo can be seen rather than
	// guessed at.
	reveal bool

	err    error
	notice string
}

// hasSecret / revealed describe the password field to the hint line: the key
// that unmasks it is only worth listing when there is one to type into.
func (f *connectionForm) hasSecret() bool {
	return f.fields[fieldAuth].value() == string(config.AuthKeychain)
}

func (f *connectionForm) revealed() bool {
	return f.reveal
}

func newConnectionForm() *connectionForm {
	fields := make([]formField, fieldCount)

	fields[fieldName] = textField("Name", "staging")
	fields[fieldHost] = textField("Host", "db.internal")
	fields[fieldPort] = textField("Port", strconv.Itoa(config.DefaultPort))
	fields[fieldUser] = textField("User", "app")
	fields[fieldDBName] = textField("Database", "app_prod")
	fields[fieldSSLMode] = choiceField("SSL mode", config.SSLModes)
	fields[fieldAuth] = choiceField("Auth", authChoices())
	fields[fieldSecret] = textField("Password", "")
	fields[fieldSecret].kind = kindPassword
	fields[fieldSecret].input.EchoMode = textinput.EchoPassword

	form := &connectionForm{fields: fields}
	form.fields[0].input.Focus()

	return form
}

func textField(label, placeholder string) formField {
	input := textinput.New()
	input.Placeholder = placeholder

	return formField{label: label, kind: kindText, input: input}
}

func choiceField(label string, choices []string) formField {
	return formField{label: label, kind: kindChoice, choices: choices}
}

func authChoices() []string {
	choices := make([]string, 0, len(config.AuthValues))

	for _, a := range config.AuthValues {
		choices = append(choices, string(a))
	}

	return choices
}

// SetWidth fits every row inside the pane. A textinput renders its prompt, then
// its value padded to the width set here, then the cell it keeps for the cursor
// past the end — so the width to hand it is what is left of the line once the
// label, the prompt, that cell and any trailing note are accounted for. Getting
// this off by one is enough to push the row past the frame, which then draws its
// borders on a different column than its rows.
func (f *connectionForm) SetWidth(width int) {
	f.width = width

	for i := range f.fields {
		if f.fields[i].kind == kindChoice {
			continue
		}

		available := width - labelWidth - ansi.StringWidth(f.fields[i].input.Prompt) - cursorCell

		if i == fieldSecret {
			available -= ansi.StringWidth(secretNote)
		}

		f.fields[i].input.SetWidth(max(available, 8))
	}
}

// editable is false for the password field under the authentication modes that
// do not use one: the row stays on screen (removing it would resize the pane as
// the auth choice changes) but the cursor skips it, and it explains where the
// password comes from instead of offering a box that would be ignored.
func (f *connectionForm) editable(index int) bool {
	return index != fieldSecret || f.hasSecret()
}

func (f *connectionForm) auth() config.Auth {
	return config.Auth(f.fields[fieldAuth].value())
}

func (f *connectionForm) Update(msg tea.Msg) tea.Cmd {
	press, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return nil
	}

	f.notice = ""

	switch {
	case key.Matches(press, keys.Default.NextField):
		f.move(1)

		return nil

	case key.Matches(press, keys.Default.PrevField):
		f.move(-1)

		return nil

	case key.Matches(press, keys.Default.RevealSecret):
		f.reveal = !f.reveal
		f.fields[fieldSecret].input.EchoMode = echoMode(f.reveal)

		return nil

	case key.Matches(press, keys.Default.TestConn):
		return f.test()
	}

	field := &f.fields[f.focus]

	if field.kind == kindChoice {
		f.cycle(field, press)

		return nil
	}

	var cmd tea.Cmd
	field.input, cmd = field.input.Update(msg)

	return cmd
}

// cycle walks a closed-set field with the arrows. Left/right are free here: the
// form owns every key while it is open, so they are not the pane-cycling ones.
func (f *connectionForm) cycle(field *formField, press tea.KeyPressMsg) {
	switch press.String() {
	case "right", "l", "space":
		field.choice = (field.choice + 1) % len(field.choices)
	case "left", "h":
		field.choice = (field.choice - 1 + len(field.choices)) % len(field.choices)
	}
}

func (f *connectionForm) move(delta int) {
	f.fields[f.focus].input.Blur()

	for i := 0; i < len(f.fields); i++ {
		f.focus = (f.focus + delta + len(f.fields)) % len(f.fields)

		if f.editable(f.focus) {
			break
		}
	}

	if f.fields[f.focus].kind != kindChoice {
		f.fields[f.focus].input.Focus()
	}
}

// connection assembles what the fields describe, or says what is missing. The
// wizard only ever writes the discrete form — a DSN is something one pastes into
// the file, not something a form helps with.
func (f *connectionForm) connection() (config.Connection, error) {
	conn := config.Connection{
		Name:    f.fields[fieldName].value(),
		Host:    f.fields[fieldHost].value(),
		User:    f.fields[fieldUser].value(),
		DBName:  f.fields[fieldDBName].value(),
		SSLMode: f.fields[fieldSSLMode].value(),
		Auth:    config.Auth(f.fields[fieldAuth].value()),
	}

	var missing []string

	for _, required := range []struct{ label, value string }{
		{"name", conn.Name},
		{"host", conn.Host},
		{"user", conn.User},
		{"database", conn.DBName},
	} {
		if required.value == "" {
			missing = append(missing, required.label)
		}
	}

	if len(missing) > 0 {
		return conn, fmt.Errorf("%s: required", strings.Join(missing, ", "))
	}

	conn.Port = config.DefaultPort

	if port := f.fields[fieldPort].value(); port != "" {
		number, err := strconv.Atoi(port)
		if err != nil || number <= 0 || number > 65535 {
			return conn, fmt.Errorf("port %q is not a number between 1 and 65535", port)
		}

		conn.Port = number
	}

	// Saving auth = "keychain" with an empty field would write a profile that
	// reads a vault entry nothing ever put there — and, worse, one that silently
	// inherits a stale entry left by an older profile of the same name.
	if conn.Auth == config.AuthKeychain && f.password() == "" {
		return conn, fmt.Errorf("password: required with auth = %q — or pick pgpass / env / none", config.AuthKeychain)
	}

	return conn, nil
}

// password is what the vault should hold — empty for every mode that does not
// use one, so nothing is ever stored for a profile that would not read it.
func (f *connectionForm) password() string {
	if f.fields[fieldAuth].value() != string(config.AuthKeychain) {
		return ""
	}

	return f.fields[fieldSecret].value()
}

func (f *connectionForm) test() tea.Cmd {
	conn, err := f.connection()
	if err != nil {
		f.err = err

		return nil
	}

	f.err = nil
	f.notice = "testing…"

	return testConnectionCmd(conn, f.password())
}

func (f *connectionForm) setTestResult(msg connTestResultMsg) {
	if msg.err != nil {
		f.notice = ""
		f.err = msg.err

		return
	}

	f.err = nil
	f.notice = fmt.Sprintf("ok — connected in %s", msg.took.Round(time.Millisecond))
}

func (f *connectionForm) setError(err error) {
	f.notice = ""
	f.err = err
}

// View is exactly one line per field, always — including the password one. Its
// height is what the screen sizes the box on, and a pane that grows and shrinks
// as the auth choice changes is a pane that moves under the cursor.
func (f *connectionForm) View() string {
	lines := make([]string, 0, len(f.fields))

	for i, field := range f.fields {
		line := formLabelStyle.Render(field.label) + f.fieldView(i, field)

		// Belt and braces: the pane pads its content but does not cut it, so a row
		// one cell too long would take the border with it.
		if f.width > 0 {
			line = ansi.Truncate(line, f.width, "…")
		}

		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

func (f *connectionForm) fieldView(index int, field formField) string {
	if field.kind == kindChoice {
		return f.choiceView(index, field)
	}

	if index == fieldSecret {
		if !f.hasSecret() {
			return formNoteStyle.Render(secretSource(f.auth()))
		}

		return field.input.View() + formNoteStyle.Render(secretNote)
	}

	return field.input.View()
}

// secretSource says where the password comes from under the modes that do not
// store one with us. It is the answer to "then how does it authenticate?", put
// where the question is asked rather than in the documentation.
func secretSource(auth config.Auth) string {
	switch auth {
	case config.AuthPgpass:
		return "read from ~/.pgpass ($PGPASSFILE) by the driver"
	case config.AuthEnv:
		return "read from $PGPASSWORD by the driver"
	default:
		return "none — trust, peer, or a client certificate"
	}
}

// choiceView marks the focused closed-set field with the arrows that change it,
// so nothing on screen looks like a box one should type into.
func (f *connectionForm) choiceView(index int, field formField) string {
	value := field.choices[field.choice]
	if value == "" {
		value = "(default)"
	}

	if f.focus != index {
		return formNoteStyle.Render(value)
	}

	return formNoteStyle.Render("‹ ") + formChoiceStyle.Render(value) + formNoteStyle.Render(" ›")
}

// message hands the form's error or confirmation to the screen, which owns the
// lines under the box. Rendering it inside would make the pane taller the moment
// something went wrong — the one moment the layout must hold still.
func (f *connectionForm) message() (text string, isErr bool) {
	if f.err != nil {
		return f.err.Error(), true
	}

	return f.notice, false
}

// testConnectionCmd opens a throwaway connection and closes it. It is a Cmd and
// not an inline call because a host that does not answer takes as long as its
// timeout, and the form must stay responsive while it waits.
func testConnectionCmd(conn config.Connection, password string) tea.Cmd {
	return func() tea.Msg {
		started := time.Now()

		db, err := sql.Connect(conn.ConnString(), password)
		if err != nil {
			return connTestResultMsg{err: err}
		}

		db.Close()

		return connTestResultMsg{took: time.Since(started)}
	}
}
