package store

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// Source guards for deploy 1 of F-c
// (docs/archive/design/choir-event-table-f-c-2026-10-22.md section 5).

// storeSourceFiles parses this package's non-test sources.
func storeSourceFiles(t *testing.T, match func(name string) bool) map[string]*ast.File {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]*ast.File{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") || !match(name) {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(".", name), nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		out[name] = file
	}
	return out
}

func funcName(fn *ast.FuncDecl) string {
	return fn.Name.Name
}

func mentionsIdent(node ast.Node, name string) bool {
	found := false
	ast.Inspect(node, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && id.Name == name {
			found = true
		}
		return !found
	})
	return found
}

func stringLiterals(node ast.Node) []string {
	var out []string
	ast.Inspect(node, func(n ast.Node) bool {
		if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING {
			if v, err := strconv.Unquote(lit.Value); err == nil {
				out = append(out, v)
			}
		}
		return true
	})
	return out
}

// F2: every function in this package that reads choir.event objects is
// union-aware, a writer, or the acknowledged-only channel read. A new reader
// of ogKindEvent fails here until someone decides which.
var ogKindEventFunctions = map[string]string{
	"allocateEventSeqLocked":          "writer; the one allocator of seq and stream_seq: both seeds read both tables",
	"leaseCrossCheckTimed":            "writer; the lease's staleness check: bounded newest-objects window of og_objects, then choir_events (exactTableStreamSeqMax is its exact table half)",
	"exactOGStreamSeqMax":             "writer; the exact seed's og half: a keyset scan of the whole event kind, metadata only; the table half is exactTableStreamSeqMax",
	"AppendEventOG":                   "writer",
	"buildEventObject":                "writer",
	"recheckTelemetryOp":              "writer (the sealer's second look)",
	"ListEventsAfter":                 "union",
	"ListEventsByOwner":               "union",
	"ListEventsByOwnerAfter":          "union",
	"ListEventsByTrajectory":          "union",
	"ListAcknowledgedEventsByChannel": "og_objects only: acknowledged kinds never leave it (F10)",
	"projectChoirEvent":               "the projector",
	"decodeChoirEventRow":             "the projector (choir_events and the telemetry partitions)",
}

func TestEventTableReadersAreUnionAware(t *testing.T) {
	var bad []string
	for name, file := range storeSourceFiles(t, func(string) bool { return true }) {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			if mentionsIdent(fn.Body, "ogKindEvent") {
				if _, listed := ogKindEventFunctions[funcName(fn)]; !listed {
					bad = append(bad, name+" "+funcName(fn)+" reads ogKindEvent")
				}
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				lit, ok := n.(*ast.CompositeLit)
				if !ok {
					return true
				}
				sel, ok := lit.Type.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "ListFilter" {
					return true
				}
				hasKind := false
				for _, elt := range lit.Elts {
					if kv, ok := elt.(*ast.KeyValueExpr); ok {
						if key, ok := kv.Key.(*ast.Ident); ok && key.Name == "Kind" {
							hasKind = true
						}
					}
				}
				if !hasKind {
					bad = append(bad, name+" "+funcName(fn)+" lists objects of every kind")
				}
				return true
			})
		}
	}
	sort.Strings(bad)
	if len(bad) > 0 {
		t.Fatalf("event readers outside the union-aware list:\n%s", strings.Join(bad, "\n"))
	}
}

// F11: the fences read objectRowTables. The V1→V2 migration machinery is
// the one named exception: no choir_events row can be V1, and the full fence
// covers it.
var v1ToV2MigrationFunctions = map[string]bool{
	"planOGMigration":        true,
	"ogResolveLiveCollision": true,
	"applyOGMigration":       true,
	"revertOGMigration":      true,
}

func TestEventTableVocabFencesReadTheList(t *testing.T) {
	files := storeSourceFiles(t, func(name string) bool { return strings.HasPrefix(name, "vocab_") })
	sawFull, sawNarrowed := false, false
	for name, file := range files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			for _, lit := range stringLiterals(fn.Body) {
				if strings.Contains(lit, "FROM og_objects") && !v1ToV2MigrationFunctions[funcName(fn)] {
					t.Fatalf("%s %s reads og_objects by name; read objectRowTables (or name the function as V1→V2 machinery)", name, funcName(fn))
				}
			}
			switch funcName(fn) {
			case "servingOGRoleFields":
				sawFull = true
				if !mentionsIdent(fn.Body, "objectRowTables") {
					t.Fatal("the full fence does not read objectRowTables")
				}
			case "vocabReplayStaysFenced":
				sawNarrowed = true
				if !mentionsIdent(fn.Body, "choirEventsTable") {
					t.Fatal("the narrowed fence does not re-read stamped rows from choir_events")
				}
			}
		}
	}
	if !sawFull || !sawNarrowed {
		t.Fatalf("fence functions not found (full %t, narrowed %t)", sawFull, sawNarrowed)
	}
	if len(objectRowTables) != 2 || objectRowTables[0] != "og_objects" || objectRowTables[1] != choirEventsTable {
		t.Fatalf("objectRowTables = %v", objectRowTables)
	}
}

// F19: the boot path's only new statement is the table's DDL inside
// schemaDDL. No boot step reads, scans or fills the table, and no resumer
// is added.
func TestEventTableBootPathAddsOnlyTheDDL(t *testing.T) {
	if n := strings.Count(schemaDDL, "CREATE TABLE IF NOT EXISTS choir_events"); n != 1 {
		t.Fatalf("schemaDDL creates choir_events %d times", n)
	}
	files := storeSourceFiles(t, func(name string) bool { return name == "open.go" })
	for _, decl := range files["open.go"].Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		switch funcName(fn) {
		case "Open", "OpenFresh", "bootstrap":
		default:
			continue
		}
		if mentionsIdent(fn.Body, "choirEventsTable") {
			t.Fatalf("boot step %s names choirEventsTable", funcName(fn))
		}
		for _, lit := range stringLiterals(fn.Body) {
			if strings.Contains(lit, "choir_events") {
				t.Fatalf("boot step %s runs SQL on choir_events: %q", funcName(fn), lit)
			}
		}
	}
	// Deploy 2: Open's one statement on choir_events is the floor probe, one
	// row by primary key (review finding A), never a scan.
	if eventTableAnyRowSQL != `SELECT 1 FROM choir_events LIMIT 1` {
		t.Fatalf("Open's floor probe is %q; it must read one row", eventTableAnyRowSQL)
	}
	if !mentionsIdent(files["open.go"], "ensureEventTableFloor") {
		t.Fatal("Open no longer floors a workspace whose choir_events has rows")
	}
}
