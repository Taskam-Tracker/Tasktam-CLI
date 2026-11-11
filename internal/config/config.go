// Package config holds where Taskam lives and the signed-in token.
//
// The token goes to the OS keychain (macOS Keychain, Windows Credential Manager, Secret Service on Linux). On
// machines without one (headless servers, CI) it falls back to the config file, readable only by the user.
// TASKAM_TOKEN and TASKAM_API override both, for scripts.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/zalando/go-keyring"
)

const (
	DefaultAPI  = "https://api-taskam.moeein.me"
	DefaultSite = "https://taskam.moeein.me"

	keyringService = "taskam-cli"
)

type Config struct {
	APIURL  string `json:"api_url,omitempty"`
	SiteURL string `json:"site_url,omitempty"`
	// Token is only stored here when no OS keychain is available.
	Token string `json:"token,omitempty"`
}

// Dir is the configuration directory (TASKAM_CONFIG_DIR, else e.g. ~/Library/Application Support/taskam).
func Dir() (string, error) {
	if dir := os.Getenv("TASKAM_CONFIG_DIR"); dir != "" {
		return dir, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "taskam"), nil
}

func path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// Load reads the config file (missing is fine) and applies defaults and environment overrides.
func Load() (*Config, error) {
	cfg := &Config{}
	p, err := path()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return nil, err
	default:
		if err := json.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("%s is not valid JSON: %w", p, err)
		}
	}
	if v := os.Getenv("TASKAM_API"); v != "" {
		cfg.APIURL = v
	}
	if cfg.APIURL == "" {
		cfg.APIURL = DefaultAPI
	}
	if cfg.SiteURL == "" {
		cfg.SiteURL = siteFor(cfg.APIURL)
	}
	cfg.APIURL = strings.TrimRight(cfg.APIURL, "/")
	cfg.SiteURL = strings.TrimRight(cfg.SiteURL, "/")
	return cfg, nil
}

// siteFor guesses the web app from the API host: api-<x> → <x>, api.<x> → <x>.
func siteFor(api string) string {
	if strings.TrimRight(api, "/") == DefaultAPI {
		return DefaultSite
	}
	switch {
	case strings.Contains(api, "://api-"):
		return strings.Replace(api, "://api-", "://", 1)
	case strings.Contains(api, "://api."):
		return strings.Replace(api, "://api.", "://", 1)
	case strings.Contains(api, "localhost:8000"):
		return strings.Replace(api, "localhost:8000", "localhost:3000", 1)
	}
	return api
}

// Save writes the config file with owner-only permissions.
func (c *Config) Save() error {
	p, err := path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

// account names the keychain entry per server, so several servers can be signed in at once.
func (c *Config) account() string { return c.APIURL }

// LoadToken returns the token for the configured server: TASKAM_TOKEN, else keychain, else config file.
func (c *Config) LoadToken() string {
	if v := os.Getenv("TASKAM_TOKEN"); v != "" {
		return v
	}
	if secret, err := keyring.Get(keyringService, c.account()); err == nil && secret != "" {
		return secret
	}
	return c.Token
}

// StoreToken saves the token in the keychain, or (without one) in the config file. It reports where.
func (c *Config) StoreToken(token string) (string, error) {
	if err := keyring.Set(keyringService, c.account(), token); err == nil {
		if c.Token != "" { // an older file copy must not linger
			c.Token = ""
			if err := c.Save(); err != nil {
				return "", err
			}
		}
		return "the system keychain", nil
	}
	c.Token = token
	if err := c.Save(); err != nil {
		return "", err
	}
	p, _ := path()
	return p + " (no system keychain available)", nil
}

// ClearToken removes the token from every place it may be stored.
func (c *Config) ClearToken() error {
	// Not found, or no keychain on this machine: either way nothing is left there.
	_ = keyring.Delete(keyringService, c.account())
	if c.Token != "" {
		c.Token = ""
		return c.Save()
	}
	return nil
}
