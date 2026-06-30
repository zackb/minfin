// Package env centralizes environment-variable handling and the default file
// locations the minfin binaries share, so paths aren't hardcoded per command.
package env

import (
	"os"
	"path/filepath"
)

// Get returns the value of key, or def when it's unset/empty.
func Get(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// DBPath resolves the sqlite location: $MINFIN_DB if set, else
// $XDG_DATA_HOME/minfin/minfin.db (falling back to ~/.local/share/minfin/minfin.db).
// It creates the parent directory so a fresh install just works.
func DBPath() string {
	if p := os.Getenv("MINFIN_DB"); p != "" {
		ensureDir(p)
		return p
	}
	dir := os.Getenv("XDG_DATA_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".local", "share")
	}
	p := filepath.Join(dir, "minfin", "minfin.db")
	ensureDir(p)
	return p
}

func ensureDir(p string) { _ = os.MkdirAll(filepath.Dir(p), 0o700) }
