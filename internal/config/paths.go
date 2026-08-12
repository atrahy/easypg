// Package config reads the application's configuration: the general settings
// (config.toml) and the connection registry (connections.toml), both under the
// XDG config directory.
//
// The registry never holds a secret — the loader rejects a "password" key
// outright — so connections.toml is a file a user can version and share. Where
// passwords actually live is internal/secrets. See docs/spec/07-connections.md.
package config

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	// appDir is our directory inside the XDG config home.
	appDir = "easypg"

	// ConfigFile holds process-level settings; ConnectionsFile the profiles.
	// They are two files rather than two sections of one because only the second
	// is meant to be edited often, diffed and committed.
	ConfigFile      = "config.toml"
	ConnectionsFile = "connections.toml"
)

// Dir is $XDG_CONFIG_HOME/easypg, falling back to ~/.config/easypg. Pointing the
// whole layer elsewhere is done through the environment variable — there is no
// --config flag, since the variable already does the job with no code of ours
// (it is what Taskfile's "run" uses to reach scripts/xdg).
//
// The directory not existing is not an error: that is the first run, and the
// connection screen offers to create it.
func Dir() (string, error) {
	if base := os.Getenv("XDG_CONFIG_HOME"); base != "" {
		return filepath.Join(base, appDir), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot locate the home directory (set $XDG_CONFIG_HOME): %w", err)
	}

	return filepath.Join(home, ".config", appDir), nil
}
