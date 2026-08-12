package config

import (
	"errors"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

// Auth says where a connection's password comes from. It is declared rather than
// guessed: "why is it asking me for a password" must be answerable by reading the
// profile.
type Auth string

const (
	// AuthKeychain reads the password from the system vault (internal/secrets),
	// prompting for it — and storing it — when the vault has no entry yet.
	AuthKeychain Auth = "keychain"
	// AuthPgpass and AuthEnv set no password at all: pgx reads ~/.pgpass
	// ($PGPASSFILE) and $PGPASSWORD natively.
	AuthPgpass Auth = "pgpass"
	AuthEnv    Auth = "env"
	// AuthNone is trust/peer authentication, or a client certificate carried by
	// the DSN.
	AuthNone Auth = "none"
)

// AuthValues is every accepted value, in the order the wizard cycles them.
var AuthValues = []Auth{AuthKeychain, AuthPgpass, AuthEnv, AuthNone}

// SSLModes are the libpq sslmode values, the wizard's choices for that field.
// The empty one means "unset": libpq then applies its own default (prefer).
var SSLModes = []string{"", "disable", "allow", "prefer", "require", "verify-ca", "verify-full"}

// DefaultPort is what a profile that names no port connects to.
const DefaultPort = 5432

// Connection is one named profile, written as a table of its own:
//
//	[staging]
//	host = "db.internal"
//
// The name is therefore the table's key, not a field — hence `toml:"-"`, which
// also makes a stray "name = …" inside the table an unknown key rather than a
// second, contradictory source for it.
//
// The target is declared either as a DSN or as discrete fields; there is no
// password field, and adding one to the file is an error rather than something
// we ignore (see checkForbiddenKeys).
type Connection struct {
	Name    string `toml:"-"`
	DSN     string `toml:"dsn"`
	Host    string `toml:"host"`
	Port    int    `toml:"port"`
	User    string `toml:"user"`
	DBName  string `toml:"dbname"`
	SSLMode string `toml:"sslmode"`
	Auth    Auth   `toml:"auth"`
}

// Connections is connections.toml as a whole, with the profiles in the order the
// file declares them.
//
// The file has no root keys at all — every top-level table is a connection. It
// could afford to lose the last one ("default") because nothing connects on its
// own any more: the screen always opens, so there was nothing left for a default
// to decide. Process-level settings live in config.toml, next door.
type Connections struct {
	Connections []Connection

	// Path is the file this was read from (or would be written to), for the
	// messages and the screen that point at it.
	Path string
}

// LoadConnections reads connections.toml. A missing file is not an error: it is
// an empty registry, which the connection screen turns into its empty state.
func LoadConnections(dir string) (Connections, error) {
	conns := Connections{Path: filepath.Join(dir, ConnectionsFile)}

	var file map[string]Connection

	md, err := toml.DecodeFile(conns.Path, &file)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return conns, nil
		}

		// A value at the root is the one mistake worth naming: it is what an old
		// file (or a habit from another tool) puts there, and the decoder's own
		// "cannot load a string into a struct" says nothing about the shape of
		// this file.
		if key, ok := rootScalar(conns.Path); ok {
			return conns, fmt.Errorf(
				"%s: %q is not a connection — this file holds nothing but [name] tables, one per connection",
				conns.Path, key)
		}

		return conns, fmt.Errorf("%s: %w", conns.Path, err)
	}

	if err := checkForbiddenKeys(conns.Path, md); err != nil {
		return conns, err
	}

	conns.Connections = ordered(file, md)

	for i := range conns.Connections {
		conns.Connections[i].normalize()
	}

	if err := conns.validate(); err != nil {
		return conns, err
	}

	conns.warn()

	return conns, nil
}

// checkForbiddenKeys is what makes connections.toml committable by construction
// rather than by convention: a "password" key fails the load instead of being
// quietly ignored, so a secret cannot end up in the file by accident.
//
// Every other unknown key is reported too — a silently ignored "dbnamee" is a
// profile that connects to the wrong place for a reason nothing on screen names.
func checkForbiddenKeys(path string, md toml.MetaData) error {
	var problems []string

	for _, key := range md.Undecoded() {
		name := key[len(key)-1]

		if name == "password" {
			return fmt.Errorf(
				"%s: %q is not a valid key — secrets live in the system vault, not in this file.\n"+
					"  Set auth = %q and store the password from the app (c → n), or use auth = %q / %q.",
				path, "password", AuthKeychain, AuthPgpass, AuthEnv)
		}

		problems = append(problems, fmt.Sprintf("unknown key %q", key.String()))
	}

	return joinProblems(path, problems)
}

// ordered puts the profiles back in the order the file declares them. A Go map
// has none, and a picker whose rows shuffle between runs is one you cannot learn
// — so the order comes from the document itself, through the decoder's metadata.
func ordered(entries map[string]Connection, md toml.MetaData) []Connection {
	conns := make([]Connection, 0, len(entries))
	seen := make(map[string]bool, len(entries))

	add := func(name string) {
		entry := entries[name]
		entry.Name = name
		seen[name] = true

		conns = append(conns, entry)
	}

	for _, key := range md.Keys() {
		if len(key) == 0 {
			continue
		}

		// Sub-keys report their table first, so the first element is the profile.
		if name := key[0]; !seen[name] {
			if _, ok := entries[name]; ok {
				add(name)
			}
		}
	}

	// Belt and braces: anything the metadata did not report still shows up, in a
	// stable order rather than the map's.
	rest := make([]string, 0, len(entries)-len(seen))

	for name := range entries {
		if !seen[name] {
			rest = append(rest, name)
		}
	}

	sort.Strings(rest)

	for _, name := range rest {
		add(name)
	}

	return conns
}

// normalize fills in what the file may leave out. Auth defaults to the vault:
// the safe reading of an unspecified profile is "it has a password somewhere",
// not "it needs none".
func (c *Connection) normalize() {
	c.Name = strings.TrimSpace(c.Name)
	c.DSN = strings.TrimSpace(c.DSN)

	if c.Auth == "" {
		c.Auth = AuthKeychain
	}

	if c.DSN == "" && c.Port == 0 {
		c.Port = DefaultPort
	}
}

// validate reports every problem in the file at once. Fixing one error, running
// again and finding the next is a poor way to learn a file format.
func (c Connections) validate() error {
	var problems []string

	// Duplicate names need no check here: two tables of the same name is a TOML
	// error, reported by the decoder before this runs.
	for i, conn := range c.Connections {
		label := fmt.Sprintf("connection #%d", i+1)
		if conn.Name != "" {
			label = fmt.Sprintf("connection %q", conn.Name)
		}

		if conn.Name == "" {
			problems = append(problems, label+": a connection needs a name — [<name>]")
		}

		if conn.DSN == "" {
			for _, missing := range conn.missingFields() {
				problems = append(problems, fmt.Sprintf("%s: %s is required (or use dsn)", label, missing))
			}
		}

		if !validAuth(conn.Auth) {
			problems = append(problems, fmt.Sprintf("%s: unknown auth %q (%s)", label, conn.Auth, authList()))
		}

		if conn.Port < 0 || conn.Port > 65535 {
			problems = append(problems, fmt.Sprintf("%s: port %d is out of range", label, conn.Port))
		}
	}

	return joinProblems(c.Path, problems)
}

func (c Connection) missingFields() []string {
	var missing []string

	for _, field := range []struct{ name, value string }{
		{"host", c.Host},
		{"user", c.User},
		{"dbname", c.DBName},
	} {
		if field.value == "" {
			missing = append(missing, field.name)
		}
	}

	return missing
}

// warn logs what is legal but worth knowing about. A password inside a pasted
// DSN is the one hole left in the "no secret in this file" rule — it cannot be
// forbidden without rejecting valid DSNs, so it is the user's call, made once
// they know about it.
func (c Connections) warn() {
	for _, conn := range c.Connections {
		if conn.DSN == "" {
			continue
		}

		if conn.hasFields() {
			log.Printf("config: connection %q sets both dsn and host/user/dbname — the dsn is used, the fields are ignored", conn.Name)
		}

		if dsnHasPassword(conn.DSN) {
			log.Printf("config: connection %q carries a password in its dsn — %s is no longer safe to commit", conn.Name, c.Path)
		}
	}
}

func (c Connection) hasFields() bool {
	return c.Host != "" || c.User != "" || c.DBName != "" || c.SSLMode != ""
}

func dsnHasPassword(dsn string) bool {
	if u, err := url.Parse(dsn); err == nil && u.User != nil {
		if _, set := u.User.Password(); set {
			return true
		}
	}

	// The keyword/value form (host=… password=…), which url.Parse does not read.
	return strings.Contains(strings.ToLower(dsn), "password=")
}

// Find returns a copy of the named profile, or nil.
func (c Connections) Find(name string) *Connection {
	for i := range c.Connections {
		if c.Connections[i].Name == name {
			conn := c.Connections[i]

			return &conn
		}
	}

	return nil
}

// Resolve turns the name asked for on the command line into the profile the
// screen should start on. Nothing is asked for, nothing is resolved: the cursor
// simply starts at the top, since no connection opens on its own.
func (c Connections) Resolve(requested string) (*Connection, error) {
	if requested == "" {
		return nil, nil
	}

	conn := c.Find(requested)
	if conn == nil {
		return nil, fmt.Errorf("unknown connection %q in %s (%s)", requested, c.Path, c.nameList())
	}

	return conn, nil
}

// rootScalar reports a root key holding something other than a table, which is
// what makes this file fail to load as a set of profiles.
func rootScalar(path string) (string, bool) {
	var raw map[string]any

	md, err := toml.DecodeFile(path, &raw)
	if err != nil {
		return "", false
	}

	for _, key := range md.Keys() {
		if len(key) != 1 {
			continue
		}

		if _, isTable := raw[key[0]].(map[string]any); !isTable {
			return key[0], true
		}
	}

	return "", false
}

// Names lists the declared profiles, in file order.
func (c Connections) Names() []string {
	names := make([]string, 0, len(c.Connections))

	for _, conn := range c.Connections {
		names = append(names, conn.Name)
	}

	return names
}

func (c Connections) nameList() string {
	if len(c.Connections) == 0 {
		return "the file declares none"
	}

	return "known: " + strings.Join(c.Names(), ", ")
}

// ConnString is the DSN handed to pgx — never with a password in it. The secret
// is applied out of band (sql.Connect sets it on the parsed config), which keeps
// the escaping question from ever arising and the secret out of anything that
// might get logged.
func (c Connection) ConnString() string {
	if c.DSN != "" {
		return c.DSN
	}

	u := url.URL{
		Scheme: "postgres",
		Host:   c.hostPort(),
		Path:   "/" + c.DBName,
	}

	if c.User != "" {
		u.User = url.User(c.User)
	}

	if c.SSLMode != "" {
		u.RawQuery = url.Values{"sslmode": {c.SSLMode}}.Encode()
	}

	return u.String()
}

func (c Connection) hostPort() string {
	port := c.Port
	if port == 0 {
		port = DefaultPort
	}

	return net.JoinHostPort(c.Host, strconv.Itoa(port))
}

// Target is the human-readable "user@host/dbname" shown in the connection screen
// and the status messages. A DSN is reduced to the same shape, with any password
// it carries redacted — the screen must not be the place a secret leaks onto
// someone's terminal.
func (c Connection) Target() string {
	if c.DSN == "" {
		return fmt.Sprintf("%s@%s/%s", c.User, c.displayHost(), c.DBName)
	}

	u, err := url.Parse(c.DSN)
	if err != nil || u.Host == "" {
		return redactDSN(c.DSN)
	}

	target := u.Host + u.Path
	if user := u.User.Username(); user != "" {
		target = user + "@" + target
	}

	return target
}

// SafeDSN is what the profile hands to the driver, with any password it carries
// blanked. It is what the connection screen shows in full: a DSN one pasted is
// the one thing about a profile that cannot be read from the fields.
func (c Connection) SafeDSN() string {
	if c.DSN == "" {
		return c.ConnString()
	}

	if u, err := url.Parse(c.DSN); err == nil && u.User != nil {
		if _, set := u.User.Password(); set {
			u.User = url.UserPassword(u.User.Username(), "***")

			return u.String()
		}
	}

	return redactDSN(c.DSN)
}

func (c Connection) displayHost() string {
	if c.Port == 0 || c.Port == DefaultPort {
		return c.Host
	}

	return c.hostPort()
}

// redactDSN blanks the password of a keyword/value DSN, the one form Target
// cannot parse into pieces.
func redactDSN(dsn string) string {
	fields := strings.Fields(dsn)

	for i, field := range fields {
		if strings.HasPrefix(strings.ToLower(field), "password=") {
			fields[i] = "password=***"
		}
	}

	return strings.Join(fields, " ")
}

func validAuth(a Auth) bool {
	for _, candidate := range AuthValues {
		if a == candidate {
			return true
		}
	}

	return false
}

func authList() string {
	values := make([]string, 0, len(AuthValues))

	for _, a := range AuthValues {
		values = append(values, string(a))
	}

	return strings.Join(values, " | ")
}

// joinProblems turns the collected complaints into one error naming the file.
func joinProblems(path string, problems []string) error {
	if len(problems) == 0 {
		return nil
	}

	return fmt.Errorf("%s:\n  - %s", path, strings.Join(problems, "\n  - "))
}
