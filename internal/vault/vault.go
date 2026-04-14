// Package vault discovers a Sparks vault root and loads its config.
//
// A vault is a directory containing a sparks.toml file at its root. Vault
// discovery walks upward from a starting path until sparks.toml is found
// or the filesystem root is reached. This mirrors how git locates .git/.
package vault

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

// ConfigFilename is the name of the per-vault config file at vault root.
const ConfigFilename = "sparks.toml"

// ManifestFilename is the SQLite manifest path relative to vault root.
const ManifestFilename = "sparks.db"

// InboxFilename is the capture-interface markdown path relative to vault root.
const InboxFilename = "inbox.md"

// Sentinel errors.
var (
	ErrNotFound      = errors.New("vault not found")
	ErrAlreadyInited = errors.New("vault already initialized")
)

// Vault represents an opened Sparks vault.
type Vault struct {
	Root   string  // absolute path to the vault root
	Config *Config // parsed sparks.toml
}

// Config is the parsed sparks.toml.
type Config struct {
	Vault       VaultSection                  `toml:"vault"`
	Raw         RawSection                    `toml:"raw"`
	Git         GitSection                    `toml:"git"`
	Collections map[string]CollectionOverride `toml:"collections"`
}

type VaultSection struct {
	Name string `toml:"name"`
}

type RawSection struct {
	Mode string `toml:"mode"` // V1: only "append-only"
}

type GitSection struct {
	AutoCommit bool `toml:"auto_commit"`
}

// CollectionOverride lets the user override a collection's source glob.
// Other collection knobs (extractor, output filename) are not configurable.
type CollectionOverride struct {
	Glob string `toml:"glob"`
}

// Discover walks upward from start looking for ConfigFilename. Returns the
// vault root or ErrNotFound. start may be relative; the returned root is absolute.
func Discover(start string) (string, error) {
	abs, err := filepath.Abs(start)
	if err != nil {
		return "", fmt.Errorf("vault.Discover: %w", err)
	}
	dir := abs
	for {
		candidate := filepath.Join(dir, ConfigFilename)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", ErrNotFound
		}
		dir = parent
	}
}

// Open discovers the vault containing path and parses its config.
func Open(path string) (*Vault, error) {
	root, err := Discover(path)
	if err != nil {
		return nil, err
	}
	cfg, err := LoadConfig(filepath.Join(root, ConfigFilename))
	if err != nil {
		return nil, fmt.Errorf("vault.Open: %w", err)
	}
	return &Vault{Root: root, Config: cfg}, nil
}

// LoadConfig parses sparks.toml strictly: unknown keys are errors, not silent
// no-ops. This catches typos like [collecitons.quotes] before they ship.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	cfg := &Config{}
	dec := toml.NewDecoder(bytesReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if cfg.Raw.Mode == "" {
		cfg.Raw.Mode = "append-only"
	}
	return cfg, nil
}

// ManifestPath returns the absolute path to sparks.db.
func (v *Vault) ManifestPath() string {
	return filepath.Join(v.Root, ManifestFilename)
}

// InboxPath returns the absolute path to inbox.md.
func (v *Vault) InboxPath() string {
	return filepath.Join(v.Root, InboxFilename)
}

// IsVaultRoot reports whether path contains a sparks.toml at its top level.
func IsVaultRoot(path string) bool {
	info, err := os.Stat(filepath.Join(path, ConfigFilename))
	return err == nil && !info.IsDir()
}

// fileExists is a small helper used during init.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// dirExists reports whether path is a directory.
func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// ensureDir mkdir-ps a directory if missing.
func ensureDir(path string) error {
	if dirExists(path) {
		return nil
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", path, err)
	}
	return nil
}

