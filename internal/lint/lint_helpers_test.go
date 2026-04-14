package lint

import "github.com/yogirk/sparks/internal/vault"

// realVault is the helper the tests that call check*(v, ...) need. We
// construct a minimal vault.Vault with just Root set, which is all the
// checks read. Keeping this in a separate _test.go file means test
// dependency on the vault package stays visible rather than hidden
// inside the stub.
func realVault(root string) *vault.Vault {
	return &vault.Vault{Root: root, Config: &vault.Config{}}
}
