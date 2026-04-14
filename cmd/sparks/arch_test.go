// Architecture guard. Enforces the CLI/MCP thin-adapter rule from the
// design doc (decision A1 in /plan-eng-review, 2026-04-14):
//
//	cmd/sparks/ holds cobra adapters that parse flags, call internal/core,
//	and format output. They MUST NOT touch SQLite, the filesystem, git, or
//	YAML directly. All business logic lives in internal/* packages.
//
// If this test fails, move the offending logic into an internal package and
// have the cobra command call it.
package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

// Imports forbidden in cmd/sparks/. These are implementation details that
// belong behind internal/* abstractions.
var forbiddenImports = []string{
	"modernc.org/sqlite",
	"database/sql",
	"gopkg.in/yaml.v3",
	"github.com/pelletier/go-toml/v2",
	"github.com/mark3labs/mcp-go",
	"os/exec", // git operations live in internal/git/
}

// Imports allowed unconditionally.
var allowedImportPrefixes = []string{
	"github.com/yogirk/sparks/internal/",
	"github.com/spf13/cobra",
	"github.com/spf13/pflag",
}

func TestNoForbiddenImportsInCommands(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("no .go files found in cmd/sparks/")
	}

	fset := token.NewFileSet()
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, file, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", file, err)
		}
		for _, imp := range f.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			for _, forbidden := range forbiddenImports {
				if path == forbidden || strings.HasPrefix(path, forbidden+"/") {
					t.Errorf("%s imports %q, forbidden in cmd/sparks/. "+
						"Move logic into internal/* and call it.", file, path)
				}
			}
		}
	}
}

// TestNoLargeFunctionsInCommands keeps cobra adapters thin. Any handler
// over the line budget is a sign business logic is leaking into the CLI.
// Threshold is generous on purpose; trim early if it starts to drift.
func TestNoLargeFunctionsInCommands(t *testing.T) {
	const lineBudget = 50

	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}

	fset := token.NewFileSet()
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") || file == "main.go" {
			continue
		}
		f, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", file, err)
		}
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			start := fset.Position(fn.Body.Lbrace).Line
			end := fset.Position(fn.Body.Rbrace).Line
			lines := end - start
			if lines > lineBudget {
				t.Errorf("%s: function %s is %d lines (budget %d). "+
					"Push logic into internal/core and keep adapters thin.",
					file, fn.Name.Name, lines, lineBudget)
			}
		}
	}
}
