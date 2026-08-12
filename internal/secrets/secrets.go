// Package secrets keeps connection passwords in the operating system's vault —
// the macOS Keychain, the Linux Secret Service — which is exactly the platform
// support the project commits to (no Windows, see docs/spec/00-overview.md).
//
// It is the other half of the rule that connections.toml holds no secret: the
// file names a profile, the vault holds what that profile needs to authenticate.
package secrets

import (
	"errors"
	"fmt"

	"github.com/zalando/go-keyring"
)

// service is the vault entry's service name; the account is the profile name, so
// renaming a profile orphans its secret and costs one re-entry.
const service = "easypg"

// ErrNotFound means the vault works and simply holds nothing for this profile —
// the case the app answers with a prompt, not with an error. It is what makes a
// committed connections.toml usable on a second machine.
var ErrNotFound = errors.New("no password stored for this connection")

func Get(profile string) (string, error) {
	password, err := keyring.Get(service, profile)

	switch {
	case errors.Is(err, keyring.ErrNotFound):
		return "", ErrNotFound
	case err != nil:
		return "", unavailable(err)
	}

	return password, nil
}

// Set overwrites whatever the vault held for that profile. That is what makes a
// name reusable: re-creating a deleted profile replaces its old entry instead of
// inheriting it.
func Set(profile, password string) error {
	if err := keyring.Set(service, profile, password); err != nil {
		return unavailable(err)
	}

	return nil
}

// Delete drops the entry, so the app never needs the system's own vault UI to be
// opened for it: an orphan left by a renamed profile, or a password one simply
// wants to stop keeping, goes from here.
func Delete(profile string) error {
	err := keyring.Delete(service, profile)

	switch {
	case errors.Is(err, keyring.ErrNotFound):
		return ErrNotFound
	case err != nil:
		return unavailable(err)
	}

	return nil
}

// unavailable names the situation and the way out. A raw dbus or Security.framework
// error tells a user nothing about what they can do instead.
func unavailable(err error) error {
	return fmt.Errorf(
		"system vault unavailable (%w) — set auth = \"pgpass\" or auth = \"env\" on this connection "+
			"to use ~/.pgpass or $PGPASSWORD instead", err)
}
