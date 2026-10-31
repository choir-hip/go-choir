package tapefollow

// A follower must never become a writer (docs/design/branching-tape-2026-10-31.md):
//
//  R1  Follower code sends a request that could change the computer: any
//      method but GET, or a path of the tape's write side (append, pin,
//      checkpoints, lifecycle).
//  R2  Two syncers share one store (the app and a CLI follow), racing their
//      applies.
//  R3  A store opened for one computer reads another's follow.json.
//
// R1 is checked over the source of every file that makes the follower's
// requests: this package and the Mac app's syncer.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

var followerSources = []string{".", "../../cmd/desktop"}

func TestFollowerSendsOnlyGET(t *testing.T) {
	writeMethods := map[string]bool{"MethodPost": true, "MethodPut": true, "MethodPatch": true, "MethodDelete": true}
	writeLiterals := []string{"POST", "PUT", "PATCH", "DELETE"}
	writePaths := []string{"/events/append", "/events/pin", "/checkpoints", "/lifecycle/", "/files/chunks", "/snapshot-root"}
	files := 0
	for _, dir := range followerSources {
		matches, _ := filepath.Glob(filepath.Join(dir, "*.go"))
		for _, path := range matches {
			if strings.HasSuffix(path, "_test.go") {
				continue
			}
			if dir != "." && !strings.Contains(filepath.Base(path), "sync") {
				continue // the app's other files proxy the renderer; only its syncer is follower code
			}
			files++
			f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
			if err != nil {
				t.Fatal(err)
			}
			ast.Inspect(f, func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.SelectorExpr:
					if id, ok := x.X.(*ast.Ident); ok && id.Name == "http" && writeMethods[x.Sel.Name] {
						t.Errorf("%s uses http.%s: a follower only reads (R1)", path, x.Sel.Name)
					}
				case *ast.BasicLit:
					if x.Kind != token.STRING {
						return true
					}
					v, _ := strconv.Unquote(x.Value)
					for _, m := range writeLiterals {
						if v == m {
							t.Errorf("%s names method %q (R1)", path, v)
						}
					}
					for _, p := range writePaths {
						if strings.Contains(v, p) {
							t.Errorf("%s names write path %q (R1)", path, v)
						}
					}
				}
				return true
			})
		}
	}
	if files < 4 {
		t.Fatalf("checked %d follower source files; the list is stale", files)
	}
}

func TestStoreLockAndIdentity(t *testing.T) {
	dir := t.TempDir()
	a, err := Open(dir, "computer-a", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Open(dir, "computer-a", nil); err == nil {
		t.Fatal("a second syncer opened a held store (R2)")
	}
	if err := writeState(dir, State{FormatVersion: stateFormat, ComputerID: "computer-a", Branch: Branch}); err != nil {
		t.Fatal(err)
	}
	_ = a.Close()
	if _, err := Open(dir, "computer-b", nil); err == nil {
		t.Fatal("a store of computer-a opened as computer-b (R3)")
	}
	for _, bad := range []string{"", "..", "a/b", " x"} {
		if _, err := Open(t.TempDir(), bad, nil); err == nil {
			t.Fatalf("computer id %q accepted", bad)
		}
	}
	b, err := Open(dir, "computer-a", nil)
	if err != nil {
		t.Fatalf("reopen after close: %v", err)
	}
	_ = b.Close()
	if _, err := os.Stat(filepath.Join(dir, StateName)); err != nil {
		t.Fatal(err)
	}
}
