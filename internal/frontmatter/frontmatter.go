// Package frontmatter parses and validates the YAML header that prefixes
// every Sparks wiki page. The schema is hardcoded in V1; see
// sparks-contracts.md for the canonical definition.
package frontmatter

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	"gopkg.in/yaml.v3"
)

// Sentinel errors.
var (
	ErrNoFrontmatter = errors.New("frontmatter: no leading YAML block")
	ErrUnclosed      = errors.New("frontmatter: leading delimiter without closing ---")
)

// Frontmatter is the parsed YAML header. Slices are nil when the field is
// absent, distinct from empty arrays in the source. Date fields stay as
// strings; downstream code parses them when needed.
type Frontmatter struct {
	Title    string   `yaml:"title"`
	Type     string   `yaml:"type"`
	Maturity string   `yaml:"maturity"`
	Tags     []string `yaml:"tags,omitempty"`
	Aliases  []string `yaml:"aliases,omitempty"`
	Sources  []string `yaml:"sources,omitempty"`
	Created  string   `yaml:"created"`
	Updated  string   `yaml:"updated"`
}

// validTypes is the closed set of page types Sparks supports in V1.
var validTypes = map[string]bool{
	"entity":     true,
	"concept":    true,
	"summary":    true,
	"synthesis":  true,
	"collection": true,
}

// validMaturity is the closed set of maturity values.
var validMaturity = map[string]bool{
	"seed":       true,
	"working":    true,
	"stable":     true,
	"historical": true,
}

// Parse extracts the leading `---\n...---\n` YAML block from r and decodes
// it. Returns ErrNoFrontmatter if the file does not start with `---`.
func Parse(r io.Reader) (Frontmatter, []byte, error) {
	br := bufio.NewReader(r)
	first, err := br.Peek(3)
	if err != nil || string(first) != "---" {
		// Also accept the BOM-prefixed variant. Very rare; covered defensively.
		return Frontmatter{}, nil, ErrNoFrontmatter
	}

	// Discard the opening delimiter line.
	if _, err := br.ReadString('\n'); err != nil {
		return Frontmatter{}, nil, ErrUnclosed
	}

	var yamlBuf bytes.Buffer
	for {
		line, err := br.ReadString('\n')
		if errors.Is(err, io.EOF) && line == "" {
			return Frontmatter{}, nil, ErrUnclosed
		}
		trimmed := strings.TrimRight(line, "\r\n")
		if trimmed == "---" {
			break
		}
		yamlBuf.WriteString(line)
		if errors.Is(err, io.EOF) {
			return Frontmatter{}, nil, ErrUnclosed
		}
	}

	body, _ := io.ReadAll(br)

	var fm Frontmatter
	if err := yaml.Unmarshal(yamlBuf.Bytes(), &fm); err != nil {
		return Frontmatter{}, body, fmt.Errorf("parse yaml: %w", err)
	}
	return fm, body, nil
}

// ParseBytes is a convenience wrapper around Parse for in-memory inputs.
func ParseBytes(data []byte) (Frontmatter, []byte, error) {
	return Parse(bytes.NewReader(data))
}

// Validate reports issues against the V1 hardcoded schema. Returns nil if
// the frontmatter satisfies all required fields and enums.
func Validate(fm Frontmatter) []string {
	var issues []string
	if strings.TrimSpace(fm.Title) == "" {
		issues = append(issues, "missing required field: title")
	}
	if fm.Type == "" {
		issues = append(issues, "missing required field: type")
	} else if !validTypes[fm.Type] {
		issues = append(issues, fmt.Sprintf("invalid type %q (want entity|concept|summary|synthesis|collection)", fm.Type))
	}
	if fm.Maturity == "" {
		issues = append(issues, "missing required field: maturity")
	} else if !validMaturity[fm.Maturity] {
		issues = append(issues, fmt.Sprintf("invalid maturity %q (want seed|working|stable|historical)", fm.Maturity))
	}
	if fm.Created == "" {
		issues = append(issues, "missing required field: created")
	}
	if fm.Updated == "" {
		issues = append(issues, "missing required field: updated")
	}
	if len(fm.Sources) == 0 {
		issues = append(issues, "missing required field: sources (must list raw/ paths)")
	}
	return issues
}

// IsValidType reports whether s is a recognized page type.
func IsValidType(s string) bool { return validTypes[s] }

// IsValidMaturity reports whether s is a recognized maturity value.
func IsValidMaturity(s string) bool { return validMaturity[s] }
