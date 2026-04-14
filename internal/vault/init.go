package vault

import (
	"fmt"
	"os"
	"path/filepath"
)

// DefaultDirs is the set of directories created on `sparks init`.
// Order is stable; init walks this slice and creates each missing dir.
var DefaultDirs = []string{
	"raw",
	"raw/inbox",
	"raw/archive",
	"wiki",
	"wiki/entities",
	"wiki/concepts",
	"wiki/summaries",
	"wiki/synthesis",
	"wiki/collections",
}

// DefaultConfigTOML is the contents of a fresh sparks.toml. The {{name}}
// token is replaced with the vault directory's basename at init time.
const DefaultConfigTOML = `# Sparks vault config. See https://github.com/yogirk/sparks for the full spec.

[vault]
name = "{{name}}"

[raw]
mode = "append-only"   # V1 supports only this mode

[git]
auto_commit = true     # commit after ingest --finalize and collections regen
# Commit identity follows your git config. Set a vault-local identity with
# ` + "`" + `git config --local user.email "..."` + "`" + ` if you want vault commits to use a
# different name/email than your usual git identity.

# Collection source globs (optional - defaults shown).
# Collection types, extractor logic, and output filenames are hardcoded in
# the binary; only the input globs are user-overridable. Omit a section to
# use the default.
#
# [collections.quotes]
# glob = "raw/quotes/**/*.md"
#
# [collections.bookmarks]
# glob = "raw/weblinks/**/*.md"
#
# [collections.media]
# glob = "raw/media/**/*.md"
#
# [collections.ideas]
# glob = "raw/ideas/**/*.md"
`

// DefaultInboxMD is the seed contents of a fresh inbox.md.
//
// The trailing `---` is load-bearing: status counts entries that follow the
// first `---` separator, so a fresh inbox with only header text reports
// 0 pending captures rather than mistaking the seed comment for a capture.
const DefaultInboxMD = `# Inbox

Drop captures below the line, separated by ` + "`---`" + ` on its own line.
The first line of an entry may be a date (YYYY-MM-DD) for the capture date;
otherwise today's date is used.

Run ` + "`sparks ingest --prepare`" + ` and let your agent process pending entries.

---

`

// InitOptions configures vault creation.
type InitOptions struct {
	Path string // target directory; created if missing
	Name string // vault name; defaults to filepath.Base(Path)
}

// Init creates a new vault at opts.Path. If a sparks.toml already exists at
// the target, returns ErrAlreadyInited so callers can decide whether to
// no-op or refresh missing sub-pieces.
func Init(opts InitOptions) (string, error) {
	if opts.Path == "" {
		return "", fmt.Errorf("vault.Init: empty path")
	}
	abs, err := filepath.Abs(opts.Path)
	if err != nil {
		return "", err
	}
	if err := ensureDir(abs); err != nil {
		return "", err
	}

	configPath := filepath.Join(abs, ConfigFilename)
	if fileExists(configPath) {
		// Repair missing dirs and inbox even if config exists. Idempotent.
		if err := ensureLayout(abs); err != nil {
			return abs, err
		}
		if err := ensureInbox(abs); err != nil {
			return abs, err
		}
		return abs, ErrAlreadyInited
	}

	name := opts.Name
	if name == "" {
		name = filepath.Base(abs)
	}
	configBody := []byte(replaceName(DefaultConfigTOML, name))
	if err := os.WriteFile(configPath, configBody, 0o644); err != nil {
		return abs, fmt.Errorf("write config: %w", err)
	}
	if err := ensureLayout(abs); err != nil {
		return abs, err
	}
	if err := ensureInbox(abs); err != nil {
		return abs, err
	}
	return abs, nil
}

func ensureLayout(root string) error {
	for _, d := range DefaultDirs {
		if err := ensureDir(filepath.Join(root, d)); err != nil {
			return err
		}
	}
	return nil
}

func ensureInbox(root string) error {
	path := filepath.Join(root, InboxFilename)
	if fileExists(path) {
		return nil
	}
	if err := os.WriteFile(path, []byte(DefaultInboxMD), 0o644); err != nil {
		return fmt.Errorf("write inbox: %w", err)
	}
	return nil
}

// replaceName substitutes {{name}} in the config template. We avoid
// text/template here because the value never contains template syntax and
// the substitution is single-use.
func replaceName(template, name string) string {
	const token = "{{name}}"
	out := make([]byte, 0, len(template))
	rest := template
	for {
		idx := indexOf(rest, token)
		if idx < 0 {
			out = append(out, rest...)
			break
		}
		out = append(out, rest[:idx]...)
		out = append(out, name...)
		rest = rest[idx+len(token):]
	}
	return string(out)
}

// indexOf is a tiny replacement for strings.Index to keep this file dep-free.
func indexOf(haystack, needle string) int {
	if len(needle) == 0 {
		return 0
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
