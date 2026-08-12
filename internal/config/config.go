package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config is the general application configuration (config.toml).
//
// It deliberately has no keys yet: there is nothing about the process worth
// configuring today. The file is read from the start anyway, so that adding a
// key later (theme, log level, default tab) is one struct field rather than a
// new loading path — and unknown keys are ignored here, since a config written
// for a newer version must not stop an older one from starting.
type Config struct{}

// Load reads config.toml. An absent file is the normal case, not an error.
func Load(dir string) (Config, error) {
	var cfg Config

	path := filepath.Join(dir, ConfigFile)

	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}

		return cfg, fmt.Errorf("%s: %w", path, err)
	}

	return cfg, nil
}
