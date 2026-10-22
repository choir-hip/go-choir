package store

import (
	"bytes"
	"encoding/json"
	"errors"
	"go/ast"
	"go/constant"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// Deploy 1 of F-c (docs/design/choir-event-table-f-c-2026-10-22.md) is inert:
// no production code sets ProjectionOp.Table. The guard is type-based (round
// 1c, DeepSeek): a syntactic check missed elided literals such as
// []computerevent.ProjectionOp{{Table: "choir_events"}}, the style most
// writers use. Every non-test package under internal/ and cmd/ that depends
// on computerevent is type-checked from source against the build's export
// data, and the guard flags, by type:
//   - any composite literal of type ProjectionOp (or *ProjectionOp), written
//     out or elided, keyed or positional, whose Table is not the constant "";
//   - any assignment to the Table field of a ProjectionOp, other than the
//     constant "";
//   - any conversion to ProjectionOp from another type.
//
// The lifecycle fold (F8) is covered like every other writer. Deploy 2 adds
// its writer's file to projectionTableStampers, with the setting that gates
// it.
var projectionTableStampers = map[string]bool{}

const computereventPath = "github.com/yusefmosiah/go-choir/internal/computerevent"

type listedPackage struct {
	ImportPath string
	Dir        string
	Export     string
	GoFiles    []string
	Deps       []string
	Standard   bool
}

var (
	exportDataOnce sync.Once
	exportDataPkgs []listedPackage
	exportDataErr  error
)

// listExportData lists every package under internal/ and cmd/ with its
// dependencies and their export data (go list -export; the build cache
// serves it).
func listExportData(t *testing.T) []listedPackage {
	t.Helper()
	root := repoRoot(t)
	exportDataOnce.Do(func() {
		cmd := exec.Command("go", "list", "-e", "-export", "-deps",
			"-json=ImportPath,Dir,Export,GoFiles,Deps,Standard", "./internal/...", "./cmd/...")
		cmd.Dir = root
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		out, err := cmd.Output()
		if err != nil {
			exportDataErr = errors.New("go list -export: " + err.Error() + ": " + stderr.String())
			return
		}
		dec := json.NewDecoder(bytes.NewReader(out))
		for {
			var p listedPackage
			if err := dec.Decode(&p); errors.Is(err, io.EOF) {
				break
			} else if err != nil {
				exportDataErr = err
				return
			}
			exportDataPkgs = append(exportDataPkgs, p)
		}
	})
	if exportDataErr != nil {
		t.Fatal(exportDataErr)
	}
	return exportDataPkgs
}

// stampChecker type-checks sources against export data with one importer, so
// computerevent's types have one identity everywhere.
type stampChecker struct {
	fset *token.FileSet
	imp  types.Importer
}

func newStampChecker(t *testing.T) *stampChecker {
	t.Helper()
	exports := map[string]string{}
	for _, p := range listExportData(t) {
		if p.Export != "" {
			exports[p.ImportPath] = p.Export
		}
	}
	fset := token.NewFileSet()
	lookup := func(path string) (io.ReadCloser, error) {
		file, ok := exports[path]
		if !ok {
			return nil, errors.New("no export data for " + path)
		}
		return os.Open(file)
	}
	return &stampChecker{fset: fset, imp: importer.ForCompiler(fset, "gc", lookup)}
}

// check type-checks one package. Type errors are tolerated (a cgo file left
// out, say): every expression whose type resolves is still checked.
func (c *stampChecker) check(path string, files []*ast.File) *types.Info {
	info := &types.Info{
		Types:      map[ast.Expr]types.TypeAndValue{},
		Selections: map[*ast.SelectorExpr]*types.Selection{},
	}
	conf := types.Config{Importer: c.imp, Error: func(error) {}, FakeImportC: true}
	_, _ = conf.Check(path, c.fset, files, info)
	return info
}

func isProjectionOp(t types.Type) bool {
	if t == nil {
		return false
	}
	t = types.Unalias(t)
	if p, ok := t.(*types.Pointer); ok {
		t = types.Unalias(p.Elem())
	}
	n, ok := t.(*types.Named)
	return ok && n.Obj().Pkg() != nil && n.Obj().Pkg().Path() == computereventPath && n.Obj().Name() == "ProjectionOp"
}

func projectionOpTableIndex(t types.Type) int {
	t = types.Unalias(t)
	if p, ok := t.(*types.Pointer); ok {
		t = types.Unalias(p.Elem())
	}
	st, ok := t.Underlying().(*types.Struct)
	if !ok {
		return -1
	}
	for i := 0; i < st.NumFields(); i++ {
		if st.Field(i).Name() == "Table" {
			return i
		}
	}
	return -1
}

func isEmptyStringConstant(info *types.Info, expr ast.Expr) bool {
	tv, ok := info.Types[expr]
	return ok && tv.Value != nil && tv.Value.Kind() == constant.String && constant.StringVal(tv.Value) == ""
}

// tableStamps returns the position of every place in files that may set
// ProjectionOp.Table to something other than "".
func (c *stampChecker) tableStamps(files []*ast.File, info *types.Info) []token.Position {
	var found []token.Position
	for _, file := range files {
		ast.Inspect(file, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.CompositeLit:
				tv, ok := info.Types[node]
				if !ok || !isProjectionOp(tv.Type) {
					return true
				}
				idx := projectionOpTableIndex(tv.Type)
				for i, elt := range node.Elts {
					if kv, keyed := elt.(*ast.KeyValueExpr); keyed {
						if key, ok := kv.Key.(*ast.Ident); ok && key.Name == "Table" && !isEmptyStringConstant(info, kv.Value) {
							found = append(found, c.fset.Position(kv.Pos()))
						}
					} else if i == idx && !isEmptyStringConstant(info, elt) {
						found = append(found, c.fset.Position(elt.Pos()))
					}
				}
			case *ast.AssignStmt:
				for i, lhs := range node.Lhs {
					sel, ok := lhs.(*ast.SelectorExpr)
					if !ok {
						continue
					}
					s := info.Selections[sel]
					if s == nil || s.Kind() != types.FieldVal || s.Obj().Name() != "Table" ||
						s.Obj().Pkg() == nil || s.Obj().Pkg().Path() != computereventPath {
						continue
					}
					if node.Tok != token.ASSIGN || len(node.Rhs) != len(node.Lhs) || !isEmptyStringConstant(info, node.Rhs[i]) {
						found = append(found, c.fset.Position(sel.Pos()))
					}
				}
			case *ast.CallExpr:
				// A conversion to ProjectionOp from another struct type could
				// carry a Table value in.
				if fun, ok := info.Types[node.Fun]; ok && fun.IsType() && isProjectionOp(fun.Type) && len(node.Args) == 1 {
					if arg, ok := info.Types[node.Args[0]]; !ok || !isProjectionOp(arg.Type) {
						found = append(found, c.fset.Position(node.Pos()))
					}
				}
			}
			return true
		})
	}
	return found
}

func dependsOn(p listedPackage, path string) bool {
	if p.ImportPath == path {
		return true
	}
	for _, d := range p.Deps {
		if d == path {
			return true
		}
	}
	return false
}

func TestEventTableNothingStampsInDeploy1(t *testing.T) {
	root := repoRoot(t)
	c := newStampChecker(t)
	checked := 0
	var unexpected []string
	for _, p := range listExportData(t) {
		if p.Standard || !strings.HasPrefix(p.Dir, root+string(filepath.Separator)) || !dependsOn(p, computereventPath) {
			continue
		}
		var files []*ast.File
		for _, name := range p.GoFiles {
			file, err := parser.ParseFile(c.fset, filepath.Join(p.Dir, name), nil, 0)
			if err != nil {
				t.Fatalf("parse %s: %v", name, err)
			}
			files = append(files, file)
		}
		info := c.check(p.ImportPath, files)
		checked++
		for _, pos := range c.tableStamps(files, info) {
			rel, _ := filepath.Rel(root, pos.Filename)
			rel = filepath.ToSlash(rel)
			if !projectionTableStampers[rel] {
				unexpected = append(unexpected, rel+":"+strconv.Itoa(pos.Line))
			}
		}
	}
	if checked < 20 {
		t.Fatalf("type-checked only %d packages that depend on computerevent; the guard is not reading the tree", checked)
	}
	sort.Strings(unexpected)
	if len(unexpected) > 0 {
		t.Fatalf("production code sets ProjectionOp.Table; deploy 1 has no writer (the lifecycle fold must never stamp):\n%s", strings.Join(unexpected, "\n"))
	}
}

// The guard itself, on sources that stamp in every form it must catch, and
// on ones that do not stamp, type-checked against the real computerevent.
func TestEventTableStampGuardCatchesEveryForm(t *testing.T) {
	c := newStampChecker(t)
	const header = "package x\n\nimport ce \"" + computereventPath + "\"\n\ntype alias = ce.ProjectionOp\n\ntype other struct{ Table string }\n\nconst stamp = \"choir_events\"\n\nvar table string\n\nfunc use(...any) {}\n\nfunc f() {\n"
	stamps := map[string]string{
		"keyed":                      `op := ce.ProjectionOp{Kind: "object_recorded", Table: "choir_events"}; use(op)`,
		"keyed, a constant":          `op := ce.ProjectionOp{Kind: "object_recorded", Table: stamp}; use(op)`,
		"keyed, a variable":          `op := ce.ProjectionOp{Kind: "object_recorded", Table: table}; use(op)`,
		"pointer":                    `op := &ce.ProjectionOp{Table: "choir_events"}; use(op)`,
		"positional":                 `op := ce.ProjectionOp{"object_recorded", "id", "choir_events", nil}; use(op)`,
		"positional, a variable":     `op := ce.ProjectionOp{"object_recorded", "id", table, nil}; use(op)`,
		"elided keyed":               `ops := []ce.ProjectionOp{{Kind: "object_recorded", CanonicalID: "id", Table: "choir_events"}}; use(ops)`,
		"elided positional":          `ops := []ce.ProjectionOp{{"object_recorded", "id", "choir_events", nil}}; use(ops)`,
		"elided pointer":             `ops := []*ce.ProjectionOp{{Table: "choir_events"}}; use(ops)`,
		"elided in a map":            `ops := map[string]ce.ProjectionOp{"a": {Table: "choir_events"}}; use(ops)`,
		"elided in an array":         `ops := [1]ce.ProjectionOp{{Table: "choir_events"}}; use(ops)`,
		"inside a batch":             `b := ce.ProjectionBatch{Ops: []ce.ProjectionOp{{Table: "choir_events"}}}; use(b)`,
		"through an alias":           `op := alias{Table: "choir_events"}; use(op)`,
		"elided through an alias":    `ops := []alias{{Table: "choir_events"}}; use(ops)`,
		"assigned":                   `var op ce.ProjectionOp; op.Table = "choir_events"; use(op)`,
		"assigned through a pointer": `op := &ce.ProjectionOp{}; op.Table = table; use(op)`,
		"assigned in a slice":        `ops := make([]ce.ProjectionOp, 1); ops[0].Table = stamp; use(ops)`,
		"appended to":                `var op ce.ProjectionOp; op.Table += "x"; use(op)`,
		"tuple assignment":           `var op ce.ProjectionOp; op.Table, op.Kind = "choir_events", "k"; use(op)`,
		"converted":                  `op := ce.ProjectionOp(struct{ Kind, CanonicalID, Table string; Body []byte }{Table: "choir_events"}); use(op)`,
	}
	clean := map[string]string{
		"keyed, no table":           `op := ce.ProjectionOp{Kind: "object_recorded", CanonicalID: "id"}; use(op)`,
		"keyed, empty table":        `op := ce.ProjectionOp{Kind: "object_recorded", Table: ""}; use(op)`,
		"positional, empty table":   `op := ce.ProjectionOp{"object_recorded", "id", "", nil}; use(op)`,
		"elided, no table":          `ops := []ce.ProjectionOp{{Kind: "object_recorded"}}; use(ops)`,
		"assigned empty":            `var op ce.ProjectionOp; op.Table = ""; use(op)`,
		"another type's Table":      `o := other{Table: "choir_events"}; o.Table = table; use(o)`,
		"elided, another type":      `os := []other{{Table: "choir_events"}}; use(os)`,
		"reading the table":         `var op ce.ProjectionOp; t := op.Table; use(t)`,
		"copying a whole op":        `var a, b ce.ProjectionOp; a = b; use(a)`,
		"converting a ProjectionOp": `var a alias; b := ce.ProjectionOp(a); use(b)`,
	}
	run := func(name, body string) []token.Position {
		file, err := parser.ParseFile(c.fset, name+".go", header+body+"\n}\n", 0)
		if err != nil {
			t.Fatalf("%s: parse: %v", name, err)
		}
		files := []*ast.File{file}
		return c.tableStamps(files, c.check("example.com/x/"+strings.ReplaceAll(name, " ", "_"), files))
	}
	for name, body := range stamps {
		if len(run(name, body)) == 0 {
			t.Errorf("%s: the guard missed %s", name, body)
		}
	}
	for name, body := range clean {
		if got := run(name, body); len(got) != 0 {
			t.Errorf("%s: the guard flagged %s at %v", name, body, got)
		}
	}
}

// Round 1c (DeepSeek, finding 3): the dry-run position hook is set only by
// tests. A non-test write to it in this package fails here.
func TestEventTableDryRunHookIsTestOnly(t *testing.T) {
	for name, file := range storeSourceFiles(t, func(string) bool { return true }) {
		ast.Inspect(file, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.AssignStmt:
				for _, lhs := range node.Lhs {
					if sel, ok := lhs.(*ast.SelectorExpr); ok && sel.Sel.Name == "dryRunPositionHook" {
						t.Errorf("%s assigns dryRunPositionHook outside a test", name)
					}
				}
			case *ast.KeyValueExpr:
				if key, ok := node.Key.(*ast.Ident); ok && key.Name == "dryRunPositionHook" {
					t.Errorf("%s sets dryRunPositionHook in a literal outside a test", name)
				}
			}
			return true
		})
	}
}
