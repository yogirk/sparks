package vault

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitFreshVault(t *testing.T) {
	dir := t.TempDir()

	root, err := Init(InitOptions{Path: dir})
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if root != dir {
		t.Errorf("root = %q, want %q", root, dir)
	}

	for _, want := range append([]string{ConfigFilename, InboxFilename}, DefaultDirs...) {
		if _, err := os.Stat(filepath.Join(dir, want)); err != nil {
			t.Errorf("missing %s after init: %v", want, err)
		}
	}
}

func TestInitIdempotentReturnsAlreadyInited(t *testing.T) {
	dir := t.TempDir()
	if _, err := Init(InitOptions{Path: dir}); err != nil {
		t.Fatal(err)
	}
	_, err := Init(InitOptions{Path: dir})
	if !errors.Is(err, ErrAlreadyInited) {
		t.Errorf("second init err = %v, want ErrAlreadyInited", err)
	}
}

func TestInitRepairsMissingDirs(t *testing.T) {
	dir := t.TempDir()
	if _, err := Init(InitOptions{Path: dir}); err != nil {
		t.Fatal(err)
	}
	// Wipe a sub-dir; second init should restore it.
	if err := os.RemoveAll(filepath.Join(dir, "wiki", "entities")); err != nil {
		t.Fatal(err)
	}
	if _, err := Init(InitOptions{Path: dir}); !errors.Is(err, ErrAlreadyInited) {
		t.Fatalf("err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "wiki", "entities")); err != nil {
		t.Errorf("wiki/entities not restored: %v", err)
	}
}

func TestDiscoverWalksUpward(t *testing.T) {
	dir := t.TempDir()
	if _, err := Init(InitOptions{Path: dir}); err != nil {
		t.Fatal(err)
	}
	deep := filepath.Join(dir, "wiki", "entities")
	root, err := Discover(deep)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if root != dir {
		t.Errorf("Discover from %q got %q, want %q", deep, root, dir)
	}
}

func TestDiscoverNotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := Discover(dir)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Discover empty dir err = %v, want ErrNotFound", err)
	}
}

func TestLoadConfigStrictRejectsUnknownKey(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, ConfigFilename)
	body := `[vault]
name = "test"

[collecitons.quotes]
glob = "raw/quotes/**/*.md"
`
	if err := os.WriteFile(cfgPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadConfig(cfgPath)
	if err == nil {
		t.Fatal("LoadConfig accepted unknown key, expected strict failure")
	}
	// go-toml/v2 strict mode reports "fields in the document are missing
	// in the target struct" without naming the offending key. Good enough
	// for V1; future versions may surface key context. We just assert that
	// strict mode fired at all by checking for the canonical phrase.
	if !strings.Contains(err.Error(), "strict") {
		t.Errorf("error %q does not look like a strict-mode failure", err.Error())
	}
}

func TestLoadConfigDefaultsRawMode(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, ConfigFilename)
	if err := os.WriteFile(cfgPath, []byte(`[vault]
name = "x"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Raw.Mode != "append-only" {
		t.Errorf("Raw.Mode = %q, want append-only", cfg.Raw.Mode)
	}
}
