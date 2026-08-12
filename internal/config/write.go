package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Append adds a profile at the end of connections.toml, creating the directory
// and the file if needed.
//
// It appends *text* rather than re-encoding a decoded document: this is the one
// file the design invites users to hand-edit and commit, and a round trip
// through a TOML encoder would drop their comments and reflow their formatting.
// Appending cannot touch anything above the last line.
//
// The corollary is that this creates only — editing or deleting a profile means
// editing the file, which is why the connection screen shows its path.
func Append(dir string, c Connection) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("cannot create %s: %w", dir, err)
	}

	path := filepath.Join(dir, ConnectionsFile)

	existing, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("cannot read %s: %w", path, err)
	}

	// The file is world-readable on purpose: it holds no secret, which is the
	// whole point of the format.
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("cannot write %s: %w", path, err)
	}
	defer file.Close()

	if _, err := file.WriteString(block(existing, c)); err != nil {
		return fmt.Errorf("cannot write %s: %w", path, err)
	}

	return nil
}

// block renders the [[connections]] entry, prefixed with whatever separation the
// existing content needs (a missing final newline, then a blank line).
func block(existing []byte, c Connection) string {
	var b strings.Builder

	if len(existing) > 0 {
		if !strings.HasSuffix(string(existing), "\n") {
			b.WriteString("\n")
		}

		b.WriteString("\n")
	}

	b.WriteString("[" + tableKey(c.Name) + "]\n")

	if c.DSN != "" {
		b.WriteString(entry("dsn", c.DSN))
	} else {
		b.WriteString(entry("host", c.Host))

		if c.Port != 0 && c.Port != DefaultPort {
			b.WriteString("port    = " + strconv.Itoa(c.Port) + "\n")
		}

		b.WriteString(entry("user", c.User))
		b.WriteString(entry("dbname", c.DBName))

		if c.SSLMode != "" {
			b.WriteString(entry("sslmode", c.SSLMode))
		}
	}

	b.WriteString(entry("auth", string(c.Auth)))

	return b.String()
}

// entry writes one key, quoted the TOML way. There is no "password" case here,
// and there must never be one: the loader rejects that key on the way back in.
func entry(key, value string) string {
	return fmt.Sprintf("%-7s = %s\n", key, strconv.Quote(value))
}

// tableKey renders a profile name as the table key it becomes. Bare when it can
// be — that is the form everyone writes by hand — quoted when the name holds
// anything a bare key may not (a space, a dot, an accent).
func tableKey(name string) string {
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-':
		default:
			return strconv.Quote(name)
		}
	}

	if name == "" {
		return strconv.Quote(name)
	}

	return name
}
