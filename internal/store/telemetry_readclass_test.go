package store

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// P0-4 / design section 13: every non-test call of a Store.ListEvents* method
// outside this package is named here with its read class. A new call site
// fails this test until someone decides whether it needs a tape that includes
// the pending telemetry window (Consistent, the default, because it decides
// from events or cites them) or may lag by at most the window (Relaxed, a
// polled UI endpoint that marks its context with WithReadConsistency).
//
// Key: path from the repository root, then the method called.
var listEventsCallSiteClass = map[string]ReadConsistency{
	"internal/apihandler/costs.go ListEventsByOwner":                            ReadRelaxed,    // polled cost estimate
	"internal/textureowner/texture.go ListEventsByOwner":                        ReadRelaxed,    // polled owner events page
	"internal/agentcore/api_run_events.go ListEventsAfter":                      ReadRelaxed,    // polled run events page
	"internal/textureowner/texture_workflow_verifier.go ListEventsByTrajectory": ReadConsistent, // reads tool.result as evidence
	"internal/agentcore/run_acceptance.go ListEventsByTrajectory":               ReadConsistent, // cites events by id
	"internal/agentcore/live_ws.go ListEventsByOwnerAfter":                      ReadConsistent, // one-time catch-up after reconnect
	"internal/agentcore/api.go ListEvents":                                      ReadConsistent, // desk tooling reads run events
	"internal/agentcore/run_memory.go ListEvents":                               ReadConsistent, // compaction serializes the run ledger
	// F10 (event-table design section 1.2): the channel read covers
	// acknowledged kinds only; a second caller fails here until it adds a
	// channel index or a union.
	"internal/agentcore/system1_verb.go ListAcknowledgedEventsByChannel": ReadConsistent, // replay lookup and the System 1 call caps
}

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("repo root not found at %s: %v", root, err)
	}
	return root
}

// forEachNonTestGoFile parses every non-test .go file under internal/ and
// cmd/ (not vendor, not testdata) and calls fn with the repo-relative path.
func forEachNonTestGoFile(t *testing.T, fn func(rel string, file *ast.File, fset *token.FileSet)) {
	t.Helper()
	root := repoRoot(t)
	for _, top := range []string{"internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join(root, top), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if name := d.Name(); name == "vendor" || name == "testdata" || name == "node_modules" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			fset := token.NewFileSet()
			file, perr := parser.ParseFile(fset, path, nil, 0)
			if perr != nil {
				return nil // a file that does not parse is not a call site
			}
			rel, _ := filepath.Rel(root, path)
			fn(filepath.ToSlash(rel), file, fset)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestListEventsCallSitesAreClassified(t *testing.T) {
	found := map[string]bool{}
	forEachNonTestGoFile(t, func(rel string, file *ast.File, _ *token.FileSet) {
		if strings.HasPrefix(rel, "internal/store/") {
			return
		}
		relaxed := false
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			switch {
			case sel.Sel.Name == "WithReadConsistency":
				relaxed = true
			case strings.HasPrefix(sel.Sel.Name, "ListEvents"), sel.Sel.Name == "ListAcknowledgedEventsByChannel":
				found[rel+" "+sel.Sel.Name] = true
			}
			return true
		})
		// A Relaxed site marks its context; a Consistent one must not.
		for key, class := range listEventsCallSiteClass {
			if !strings.HasPrefix(key, rel+" ") {
				continue
			}
			if class == ReadRelaxed && !relaxed {
				t.Errorf("%s is classified Relaxed but does not call WithReadConsistency", rel)
			}
			if class == ReadConsistent && relaxed {
				t.Errorf("%s is classified Consistent but calls WithReadConsistency", rel)
			}
		}
	})
	var unclassified, stale []string
	for key := range found {
		if _, ok := listEventsCallSiteClass[key]; !ok {
			unclassified = append(unclassified, key)
		}
	}
	for key := range listEventsCallSiteClass {
		if !found[key] {
			stale = append(stale, key)
		}
	}
	sort.Strings(unclassified)
	sort.Strings(stale)
	if len(unclassified) > 0 {
		t.Fatalf("ListEvents* call sites with no read class (add each to listEventsCallSiteClass: Consistent unless it is a polled UI endpoint): %v", unclassified)
	}
	if len(stale) > 0 {
		t.Fatalf("listEventsCallSiteClass names call sites that no longer exist: %v", stale)
	}
}
