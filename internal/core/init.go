package core

import (
	"errors"

	"github.com/yogirk/sparks/internal/manifest"
	"github.com/yogirk/sparks/internal/vault"
)

// InitVault creates a new vault at path (or repairs missing pieces if one
// already exists), opens the manifest to apply migrations, and returns a
// summary. Idempotent: running it twice on the same path does nothing
// destructive.
func InitVault(path string) (InitResult, error) {
	root, err := vault.Init(vault.InitOptions{Path: path})
	existed := errors.Is(err, vault.ErrAlreadyInited)
	if err != nil && !existed {
		return InitResult{}, err
	}

	v := &vault.Vault{Root: root}
	manifestPath := v.ManifestPath()
	manifestNew := !fileExists(manifestPath)

	db, err := manifest.Open(manifestPath)
	if err != nil {
		return InitResult{}, err
	}
	if err := db.Close(); err != nil {
		return InitResult{}, err
	}

	return InitResult{
		VaultRoot:   root,
		Created:     !existed,
		Existed:     existed,
		ManifestNew: manifestNew,
	}, nil
}
