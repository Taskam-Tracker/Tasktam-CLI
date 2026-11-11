package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/zalando/go-keyring"
)

func TestDefaultsAndEnvironment(t *testing.T) {
	t.Setenv("TASKAM_CONFIG_DIR", t.TempDir())
	t.Setenv("TASKAM_API", "")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.APIURL != DefaultAPI || cfg.SiteURL != DefaultSite {
		t.Errorf("defaults = %+v", cfg)
	}
	t.Setenv("TASKAM_API", "https://api-tasks.example.com/")
	cfg, _ = Load()
	if cfg.APIURL != "https://api-tasks.example.com" || cfg.SiteURL != "https://tasks.example.com" {
		t.Errorf("env override = %+v", cfg)
	}
}

func TestTokenInKeychain(t *testing.T) {
	keyring.MockInit()
	t.Setenv("TASKAM_CONFIG_DIR", t.TempDir())
	t.Setenv("TASKAM_TOKEN", "")
	cfg, _ := Load()
	where, err := cfg.StoreToken("tk_secret")
	if err != nil || where != "the system keychain" {
		t.Fatalf("where=%q err=%v", where, err)
	}
	if _, err := os.Stat(filepath.Join(os.Getenv("TASKAM_CONFIG_DIR"), "config.json")); err == nil {
		t.Error("the token must not be written to disk when a keychain exists")
	}
	if got := cfg.LoadToken(); got != "tk_secret" {
		t.Errorf("LoadToken = %q", got)
	}
	t.Setenv("TASKAM_TOKEN", "tk_env")
	if got := cfg.LoadToken(); got != "tk_env" {
		t.Errorf("TASKAM_TOKEN should win, got %q", got)
	}
	t.Setenv("TASKAM_TOKEN", "")
	if err := cfg.ClearToken(); err != nil || cfg.LoadToken() != "" {
		t.Errorf("ClearToken left %q (err %v)", cfg.LoadToken(), err)
	}
}

func TestTokenFileFallbackIsPrivate(t *testing.T) {
	keyring.MockInitWithError(keyring.ErrUnsupportedPlatform)
	dir := t.TempDir()
	t.Setenv("TASKAM_CONFIG_DIR", dir)
	t.Setenv("TASKAM_TOKEN", "")
	cfg, _ := Load()
	if _, err := cfg.StoreToken("tk_file"); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("config.json mode = %v, want 0600", info.Mode().Perm())
	}
	reloaded, _ := Load()
	if reloaded.LoadToken() != "tk_file" {
		t.Errorf("token not read back from the file")
	}
}
