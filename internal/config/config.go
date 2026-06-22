// Package config resolves the NYT API key from (in priority order) an explicit
// flag value, the NYT_API_KEY environment variable, or a JSON config file.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EnvVar is the primary environment variable checked for the API key.
const EnvVar = "NYT_API_KEY"

// EnvVars lists, in priority order, the environment variables checked for the
// API key. NYT_KEY matches the "Key" field shown on the developer dashboard.
var EnvVars = []string{"NYT_API_KEY", "NYT_KEY"}

// Config is the on-disk configuration file format.
type Config struct {
	APIKey string `json:"api_key"`
}

// ConfigPath returns the path to the config file, honoring $NYT_CONFIG and
// $XDG_CONFIG_HOME, defaulting to ~/.config/nyt/config.json.
func ConfigPath() string {
	if p := os.Getenv("NYT_CONFIG"); p != "" {
		return p
	}
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "nyt", "config.json")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "nyt", "config.json")
}

// Load reads the config file if present. A missing file is not an error.
func Load() (*Config, error) {
	path := ConfigPath()
	if path == "" {
		return &Config{}, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, fmt.Errorf("reading config %s: %w", path, err)
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("parsing config %s: %w", path, err)
	}
	return &c, nil
}

// ResolveAPIKey applies the precedence flag > env > file. It returns the key and
// the source it came from, or an error if none is set.
func ResolveAPIKey(flagVal string) (key, source string, err error) {
	if v := strings.TrimSpace(flagVal); v != "" {
		return v, "--api-key flag", nil
	}
	LoadDotEnv()
	for _, name := range EnvVars {
		if v := strings.TrimSpace(os.Getenv(name)); v != "" {
			return v, name + " env var", nil
		}
	}
	cfg, err := Load()
	if err != nil {
		return "", "", err
	}
	if v := strings.TrimSpace(cfg.APIKey); v != "" {
		return v, ConfigPath(), nil
	}
	return "", "", fmt.Errorf(
		"no NYT API key found — pass --api-key, set %s (or NYT_KEY) in your environment or .env, "+
			"or run `nyt config set-key <KEY>`\nget a key at https://developer.nytimes.com/get-started",
		EnvVar)
}

// Save writes the config file, creating parent directories as needed.
func Save(c *Config) (string, error) {
	path := ConfigPath()
	if path == "" {
		return "", fmt.Errorf("could not determine config path")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		return "", err
	}
	return path, nil
}
