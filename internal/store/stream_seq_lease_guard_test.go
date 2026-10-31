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

// Source guards for the stream_seq lease
// (docs/problems/stream-seq-high-water-mark-design-2026-10-23.md, section 3
// "The untrust marker: every writer, every swap point" and section 6 "Writer
// fence"). Ways they can fail, written before the code:
//  1. A new function renames a workspace into the live store path, or opens a
//     fresh workspace for a live path, and does not write the marker: a lease
//     of another lineage is trusted after the swap. The list below is the
//     decision; a new swap point fails here until someone makes it.
//  2. A caller outside the lease writer mints the capability that admits a
//     lease write under every gate mode.
//  3. Open writes the receipt before it reads it, so receipt 3 hides receipt 2.
//  4. A test seam is assigned by production code.

// leaseSwapPoints are the functions that move a workspace into a live store or
// workspace path (or open a fresh one for it). Each must write the marker.
var leaseSwapPoints = map[string]string{
	"open.go:ReplaceWorkspace":               "internal/store: the fresh workspace OpenFresh created",
	"rematerialize.go:RematerializeFromTape": "internal/agentcore: the staged workspace after Close, before the flip",
}

// leaseSwapExempt are functions that match the swap-point shape and are not
// one: they build or probe a disposable workspace that never becomes a
// computer's live store, or delegate to a swap point that marks. Each carries
// its reason.
var leaseSwapExempt = map[string]string{
	"replay_completeness.go:openProbeReplayStore": "agentcore: a disposable replay-probe store in a temp dir",
	"restore_snapshot.go:openStagedStore":         "agentcore: opens or installs the staged store in the rebuild's staging dir; RematerializeFromTape marks the staged workspace before the flip",
	"workspace_replace.go:ReplaceWorkspace":       "agentcore: delegates to Store.ReplaceWorkspace, which marks the fresh workspace",
}

func sourceFilesIn(t *testing.T, dir string) map[string]*ast.File {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]*ast.File{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := filepath.Join(dir, name)
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		out[path] = file
	}
	return out
}

// selectorCalls lists the "pkg.Name" and "x.Name" selector calls in a node.
func selectorCalls(node ast.Node) map[string]bool {
	calls := map[string]bool{}
	ast.Inspect(node, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
			if id, ok := sel.X.(*ast.Ident); ok {
				calls[id.Name+"."+sel.Sel.Name] = true
			}
			calls["."+sel.Sel.Name] = true
		}
		if id, ok := call.Fun.(*ast.Ident); ok {
			calls[id.Name] = true
		}
		return true
	})
	return calls
}

// Test 7. Every function in internal/store or internal/agentcore that renames into a store or workspace path, or opens a
// fresh workspace for one, writes the lease untrust marker, or is a named
// exemption.
func TestEverySwapPointWritesTheLeaseUntrustMarker(t *testing.T) {
	var bad []string
	seenSwap := map[string]bool{}
	for _, dir := range []string{".", "../agentcore"} {
		for path, file := range sourceFilesIn(t, dir) {
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}
				calls := selectorCalls(fn.Body)
				moves := calls["os.Rename"] || calls[".OpenFresh"] || calls["OpenFresh"]
				touchesWorkspace := mentionsIdent(fn.Body, "TextureWorkspacePath") || mentionsIdent(fn.Body, "deriveTextureWorkspacePath") ||
					mentionsIdent(fn.Body, "OpenFresh")
				name := filepath.Base(path) + ":" + fn.Name.Name
				if _, swap := leaseSwapPoints[name]; swap {
					seenSwap[name] = true
					if !calls["storeschema.WriteLeaseUntrusted"] {
						bad = append(bad, path+" "+name+" is a swap point and does not call storeschema.WriteLeaseUntrusted")
					}
					continue
				}
				if !moves || !touchesWorkspace {
					continue
				}
				if _, exempt := leaseSwapExempt[name]; exempt {
					continue
				}
				if calls["storeschema.WriteLeaseUntrusted"] {
					continue
				}
				bad = append(bad, path+" "+name+" moves a workspace and is neither a marked swap point nor a named exemption")
			}
		}
	}
	for name := range leaseSwapPoints {
		if !seenSwap[name] {
			bad = append(bad, "swap point "+name+" no longer exists: update the list")
		}
	}
	sort.Strings(bad)
	if len(bad) > 0 {
		t.Fatalf("lease untrust marker guard:\n%s", strings.Join(bad, "\n"))
	}
}

// Test 29. The only non-test caller of WithStreamSeqLeaseCapability is
// internal/store/stream_seq_lease.go.
func TestOnlyTheLeaseWriterMintsTheCapability(t *testing.T) {
	root := filepath.Join("..", "..")
	var bad []string
	allowed := filepath.Join("..", "..", "internal", "store", "stream_seq_lease.go")
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "vendor", ".direnv", "scratchpad":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		raw, readErr := os.ReadFile(path)
		if readErr != nil || !strings.Contains(string(raw), "WithStreamSeqLeaseCapability") {
			return nil
		}
		file, parseErr := parser.ParseFile(token.NewFileSet(), path, raw, 0)
		if parseErr != nil {
			return nil
		}
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			name := ""
			switch fun := call.Fun.(type) {
			case *ast.SelectorExpr:
				name = fun.Sel.Name
			case *ast.Ident:
				name = fun.Name
			}
			if name == "WithStreamSeqLeaseCapability" && filepath.Clean(path) != filepath.Clean(allowed) {
				bad = append(bad, path)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(allowed); err != nil {
		t.Fatalf("the lease writer %s is missing: %v", allowed, err)
	}
	if len(bad) > 0 {
		t.Fatalf("non-test callers of WithStreamSeqLeaseCapability outside internal/store/stream_seq_lease.go:\n%s", strings.Join(bad, "\n"))
	}
}

// Test 8 (ordering). In Open, the receipt is read before it is written, and
// the untrust marker is written before the receipt.
func TestOpenReadsTheReceiptThenWritesTheMarkerThenTheReceipt(t *testing.T) {
	file := storeSourceFiles(t, func(name string) bool { return name == "open.go" })["open.go"]
	var open *ast.FuncDecl
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Name.Name == "openStore" {
			open = fn
		}
	}
	if open == nil {
		t.Fatal("Open not found")
	}
	positions := map[string]token.Pos{}
	ast.Inspect(open.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if id, ok := sel.X.(*ast.Ident); ok && id.Name == "storeschema" {
			if _, seen := positions[sel.Sel.Name]; !seen {
				positions[sel.Sel.Name] = call.Pos()
			}
		}
		return true
	})
	read, wrote, marker := positions["Read"], positions["Write"], positions["WriteLeaseUntrusted"]
	if read == 0 || wrote == 0 || marker == 0 {
		t.Fatalf("Open must call storeschema.Read, storeschema.WriteLeaseUntrusted and storeschema.Write (positions %v)", positions)
	}
	if !(read < marker && marker < wrote) {
		t.Fatalf("Open's order must be Read < WriteLeaseUntrusted < Write; got read=%d marker=%d write=%d", read, marker, wrote)
	}
}

// leaseTestSeams are the package-level and field seams only tests may assign.
var leaseTestSeams = map[string]bool{
	"leaseWriteHook": true, "leaseScanHook": true,
	"leaseResumeHook": true, "reopenBeforeAdoptHook": true,
}

// seamAssignments names every assignment to a seam in file: a selector on the
// left (s.leaseWriteHook = ...), a bare identifier (a package-level var such
// as leaseResumeHook = ...), or a literal key.
func seamAssignments(file *ast.File, seams map[string]bool) []string {
	var found []string
	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.AssignStmt:
			for _, lhs := range node.Lhs {
				if sel, ok := lhs.(*ast.SelectorExpr); ok && seams[sel.Sel.Name] {
					found = append(found, sel.Sel.Name)
				}
				if id, ok := lhs.(*ast.Ident); ok && seams[id.Name] {
					found = append(found, id.Name)
				}
			}
		case *ast.KeyValueExpr:
			if key, ok := node.Key.(*ast.Ident); ok && seams[key.Name] {
				found = append(found, key.Name+" (literal)")
			}
		}
		return true
	})
	return found
}

// The lease test seams are assigned by tests only.
func TestLeaseTestSeamsAreTestOnly(t *testing.T) {
	for name, file := range storeSourceFiles(t, func(string) bool { return true }) {
		for _, seam := range seamAssignments(file, leaseTestSeams) {
			t.Errorf("%s assigns %s outside a test", name, seam)
		}
	}
}

// The guard itself catches a bare-identifier package-var seam, a field seam
// and a literal key (bunch 180, Devin, low: the first version saw only
// selectors and literal keys, so leaseResumeHook and reopenBeforeAdoptHook
// slipped past).
func TestLeaseTestSeamGuardSeesBareIdentifiers(t *testing.T) {
	src := `package store
func f(s *Store) {
	leaseResumeHook = nil
	reopenBeforeAdoptHook = func(string) {}
	s.leaseWriteHook = nil
	_ = Store{leaseScanHook: nil}
	hook := leaseResumeHook
	_ = hook
}`
	file, err := parser.ParseFile(token.NewFileSet(), "seam.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	got := seamAssignments(file, leaseTestSeams)
	want := map[string]bool{"leaseResumeHook": true, "reopenBeforeAdoptHook": true, "leaseWriteHook": true, "leaseScanHook (literal)": true}
	if len(got) != len(want) {
		t.Fatalf("seamAssignments = %v, want exactly %v", got, want)
	}
	for _, g := range got {
		if !want[g] {
			t.Fatalf("seamAssignments = %v, want exactly %v", got, want)
		}
	}
}

// The allocator is the only code that takes a stream_seq: no other function
// in the package increments maxStreamSeq.
func TestOnlyTheAllocatorTakesAStreamSeq(t *testing.T) {
	allowed := map[string]bool{"allocateStreamSeqLocked": true, "completeSeedLocked": true}
	for name, file := range storeSourceFiles(t, func(string) bool { return true }) {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				switch node := n.(type) {
				case *ast.IncDecStmt:
					if sel, ok := node.X.(*ast.SelectorExpr); ok && sel.Sel.Name == "maxStreamSeq" && !allowed[fn.Name.Name] {
						t.Errorf("%s %s increments maxStreamSeq", name, fn.Name.Name)
					}
				case *ast.AssignStmt:
					for _, lhs := range node.Lhs {
						if sel, ok := lhs.(*ast.SelectorExpr); ok && sel.Sel.Name == "maxStreamSeq" && !allowed[fn.Name.Name] {
							t.Errorf("%s %s assigns maxStreamSeq", name, fn.Name.Name)
						}
					}
				}
				return true
			})
		}
	}
}
