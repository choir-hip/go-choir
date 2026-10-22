package store

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// Source guards for deploy 1 of F-c
// (docs/design/choir-event-table-f-c-2026-10-22.md section 5).

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

// Deploy 1 is inert: no production code stamps ProjectionOp.Table. The
// lifecycle fold (F8) and every other writer leave it empty. Deploy 2 adds
// its writer to this list, with the setting that gates it.
var projectionTableStampers = map[string]bool{}

// projectionOpTableIndex is Table's position in ProjectionOp, which an
// unkeyed literal fills positionally (TestProjectionOpFieldOrder pins it).
const projectionOpTableIndex = 2

// projectionTableStamps returns every place in file that may set
// ProjectionOp.Table: a keyed literal naming Table, an unkeyed literal whose
// Table position is not the empty string (round 1b, Sonnet: the keyed check
// alone passed a positional literal), and any assignment to a .Table field.
func projectionTableStamps(rel string, file *ast.File, fset *token.FileSet) []string {
	var found []string
	at := func(pos token.Pos) string { return rel + ":" + strconv.Itoa(fset.Position(pos).Line) }
	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.CompositeLit:
			typeName := ""
			switch typ := node.Type.(type) {
			case *ast.Ident:
				typeName = typ.Name
			case *ast.SelectorExpr:
				typeName = typ.Sel.Name
			}
			if typeName != "ProjectionOp" || len(node.Elts) == 0 {
				return true
			}
			if _, keyed := node.Elts[0].(*ast.KeyValueExpr); !keyed {
				if len(node.Elts) > projectionOpTableIndex {
					if lit, ok := node.Elts[projectionOpTableIndex].(*ast.BasicLit); !ok || lit.Value != `""` {
						found = append(found, at(node.Pos()))
					}
				}
				return true
			}
			for _, elt := range node.Elts {
				if kv, ok := elt.(*ast.KeyValueExpr); ok {
					if key, ok := kv.Key.(*ast.Ident); ok && key.Name == "Table" {
						found = append(found, at(kv.Pos()))
					}
				}
			}
		case *ast.AssignStmt:
			for _, lhs := range node.Lhs {
				if sel, ok := lhs.(*ast.SelectorExpr); ok && sel.Sel.Name == "Table" {
					found = append(found, at(sel.Pos()))
				}
			}
		}
		return true
	})
	return found
}

func TestEventTableNothingStampsInDeploy1(t *testing.T) {
	var unexpected []string
	forEachNonTestGoFile(t, func(rel string, file *ast.File, fset *token.FileSet) {
		for _, site := range projectionTableStamps(rel, file, fset) {
			if !projectionTableStampers[rel] {
				unexpected = append(unexpected, site)
			}
		}
	})
	if len(unexpected) > 0 {
		t.Fatalf("production code stamps ProjectionOp.Table; deploy 1 has no writer (the lifecycle fold must never stamp):\n%s", strings.Join(unexpected, "\n"))
	}
}

// The guard itself, on sources that stamp in each way it must catch, and
// on ones that do not stamp.
func TestEventTableStampGuardCatchesEveryForm(t *testing.T) {
	stamps := map[string]string{
		"keyed":           `op := computerevent.ProjectionOp{Kind: "object_recorded", Table: "choir_events"}`,
		"unkeyed":         `op := computerevent.ProjectionOp{"object_recorded", "id", "choir_events", nil}`,
		"unkeyed by name": `op := computerevent.ProjectionOp{"object_recorded", "id", table, nil}`,
		"local type":      `op := ProjectionOp{"object_recorded", "id", choirEventsTable, nil}`,
		"assigned":        `op.Table = "choir_events"`,
	}
	clean := map[string]string{
		"keyed, no table":      `op := computerevent.ProjectionOp{Kind: "object_recorded", CanonicalID: "id"}`,
		"unkeyed, empty table": `op := computerevent.ProjectionOp{"object_recorded", "id", "", nil}`,
		"other type":           `fp := ObjectGraphFingerprint{Table: "choir_events"}`,
	}
	parse := func(stmt string) (*ast.File, *token.FileSet) {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, "x.go", "package x\nfunc f() {\n"+stmt+"\n_ = op\n}\n", 0)
		if err != nil {
			t.Fatalf("parse %q: %v", stmt, err)
		}
		return file, fset
	}
	for name, stmt := range stamps {
		file, fset := parse(stmt)
		if len(projectionTableStamps("x.go", file, fset)) == 0 {
			t.Errorf("%s: the guard missed %s", name, stmt)
		}
	}
	for name, stmt := range clean {
		file, fset := parse(strings.Replace(stmt, "fp :=", "op :=", 1))
		if got := projectionTableStamps("x.go", file, fset); len(got) != 0 {
			t.Errorf("%s: the guard flagged %s at %v", name, stmt, got)
		}
	}
}

func TestProjectionOpFieldOrder(t *testing.T) {
	field := reflect.TypeOf(computerevent.ProjectionOp{}).Field(projectionOpTableIndex)
	if field.Name != "Table" {
		t.Fatalf("ProjectionOp field %d is %s, not Table: update projectionOpTableIndex", projectionOpTableIndex, field.Name)
	}
}

// F2: every function in this package that reads choir.event objects is
// union-aware, a writer, or the acknowledged-only channel read. A new reader
// of ogKindEvent fails here until someone decides which.
var ogKindEventFunctions = map[string]string{
	"AppendEvent":                     "writer; both seeds read both tables",
	"AppendEventOG":                   "writer",
	"buildEventObject":                "writer",
	"recheckTelemetryOp":              "writer (the sealer's second look)",
	"ListEventsAfter":                 "union",
	"ListEventsByOwner":               "union",
	"ListEventsByOwnerAfter":          "union",
	"ListEventsByTrajectory":          "union",
	"ListAcknowledgedEventsByChannel": "og_objects only: acknowledged kinds never leave it (F10)",
	"projectChoirEvent":               "the projector",
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
	files := storeSourceFiles(t, func(name string) bool { return name == "store.go" })
	for _, decl := range files["store.go"].Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		switch funcName(fn) {
		case "Open", "OpenFresh", "bootstrap", "ensureColumns", "backfillDerivedRuntimeState":
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
	for _, m := range runtimeColumnMigrations {
		if m.table == choirEventsTable {
			t.Fatal("deploy 1 adds no column migration to choir_events: its DDL is frozen (section 2.3)")
		}
	}
}
