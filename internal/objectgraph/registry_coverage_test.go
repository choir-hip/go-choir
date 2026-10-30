package objectgraph

import (
	"fmt"
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

// kindSite is one place in production source that names an object or edge
// kind as a literal.
type kindSite struct {
	Kind string
	Pos  string
}

// scanKindLiterals walks every non-test Go file under internal/ and cmd/ and
// collects the object and edge kinds that production code names as string
// literals in a kind-typed position:
//
//   - a const/var typed ObjectKind or EdgeKind;
//   - a conversion ObjectKind("...") or EdgeKind("...");
//   - an Object{ObjectKind: "..."}, CreateObjectRequest{Kind: "..."},
//     Edge{Kind: "..."} composite literal;
//   - a literal argument in the position of a parameter typed ObjectKind
//     or EdgeKind of any function declared in the scanned tree, matched by
//     name (BuildCanonicalID, BuildEdgeID, platform's makeEdge,
//     store's lifecycleObject, ...).
//
// The registry file itself is skipped: a kind named only there has no
// writer. A kind built at run time from a variable is not visible here; the
// write gate (CheckWrite) refuses it at the store instead.
func scanKindLiterals(t *testing.T) (objects, edges map[string][]string) {
	t.Helper()
	root := filepath.Join("..", "..")
	objects = map[string][]string{}
	edges = map[string][]string{}
	fset := token.NewFileSet()
	typeName := func(e ast.Expr) string {
		switch x := e.(type) {
		case *ast.Ident:
			return x.Name
		case *ast.SelectorExpr:
			return x.Sel.Name
		}
		return ""
	}
	literal := func(e ast.Expr) (string, bool) {
		b, ok := e.(*ast.BasicLit)
		if !ok || b.Kind != token.STRING {
			return "", false
		}
		s, err := strconv.Unquote(b.Value)
		return s, err == nil
	}

	type parsed struct {
		rel  string
		file *ast.File
	}
	var files []parsed
	for _, dir := range []string{"internal", "cmd"} {
		err := filepath.Walk(filepath.Join(root, dir), func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				if name := info.Name(); name == "testdata" || name == "node_modules" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			rel, _ := filepath.Rel(root, path)
			rel = filepath.ToSlash(rel)
			if rel == "internal/objectgraph/registry.go" {
				return nil
			}
			f, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				return fmt.Errorf("parse %s: %w", rel, err)
			}
			files = append(files, parsed{rel: rel, file: f})
			return nil
		})
		if err != nil {
			t.Fatalf("scan %s: %v", dir, err)
		}
	}

	// Pass 1: every declared function's kind-typed parameter positions,
	// keyed "pkg.Func" for functions and "method.Name" for methods, so a
	// same-named function of another package (contentowner.ContentHash)
	// is not mistaken for one that takes a kind.
	kindParams := map[string]map[int]string{}
	for _, pf := range files {
		for _, decl := range pf.file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Type.Params == nil {
				continue
			}
			key := pf.file.Name.Name + "." + fn.Name.Name
			if fn.Recv != nil {
				key = "method." + fn.Name.Name
			}
			idx := 0
			for _, field := range fn.Type.Params.List {
				names := len(field.Names)
				if names == 0 {
					names = 1
				}
				tn := typeName(field.Type)
				for i := 0; i < names; i++ {
					if tn == "ObjectKind" || tn == "EdgeKind" {
						if kindParams[key] == nil {
							kindParams[key] = map[int]string{}
						}
						kindParams[key][idx] = tn
					}
					idx++
				}
			}
		}
	}

	// Pass 2: kind literals.
	for _, pf := range files {
		rel := pf.rel
		imports := map[string]string{}
		for _, imp := range pf.file.Imports {
			path, _ := strconv.Unquote(imp.Path.Value)
			name := path[strings.LastIndex(path, "/")+1:]
			if imp.Name != nil {
				name = imp.Name.Name
			}
			imports[name] = path[strings.LastIndex(path, "/")+1:]
		}
		calleeKey := func(fun ast.Expr) string {
			switch x := fun.(type) {
			case *ast.Ident:
				return pf.file.Name.Name + "." + x.Name
			case *ast.SelectorExpr:
				if id, ok := x.X.(*ast.Ident); ok {
					if pkg, ok := imports[id.Name]; ok {
						return pkg + "." + x.Sel.Name
					}
				}
				return "method." + x.Sel.Name
			}
			return ""
		}
		add := func(tn, kind string, pos token.Pos) {
			m := objects
			if tn == "EdgeKind" {
				m = edges
			}
			m[kind] = append(m[kind], fmt.Sprintf("%s:%d", rel, fset.Position(pos).Line))
		}
		ast.Inspect(pf.file, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.ValueSpec:
				if x.Type == nil {
					return true
				}
				if tn := typeName(x.Type); tn == "ObjectKind" || tn == "EdgeKind" {
					for _, v := range x.Values {
						if s, ok := literal(v); ok {
							add(tn, s, v.Pos())
						}
					}
				}
			case *ast.CallExpr:
				fn := typeName(x.Fun)
				if (fn == "ObjectKind" || fn == "EdgeKind") && len(x.Args) == 1 {
					if s, ok := literal(x.Args[0]); ok {
						add(fn, s, x.Pos())
					}
				}
				for i, tn := range kindParams[calleeKey(x.Fun)] {
					if i < len(x.Args) {
						if s, ok := literal(x.Args[i]); ok {
							add(tn, s, x.Args[i].Pos())
						}
					}
				}
			case *ast.CompositeLit:
				tn := typeName(x.Type)
				for _, el := range x.Elts {
					kv, ok := el.(*ast.KeyValueExpr)
					if !ok {
						continue
					}
					s, ok := literal(kv.Value)
					if !ok {
						continue
					}
					key := typeName(kv.Key)
					switch {
					case tn == "Object" && key == "ObjectKind",
						tn == "CreateObjectRequest" && key == "Kind":
						add("ObjectKind", s, kv.Pos())
					case tn == "Edge" && key == "Kind":
						add("EdgeKind", s, kv.Pos())
					}
				}
			}
			return true
		})
	}
	return objects, edges
}

func sortedKeys(m map[string][]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// TestEveryWrittenKindIsRegistered is H-OG1's standing check: every object
// and edge kind production code names must be in DefaultRegistry, so the
// write gate never refuses a live writer
// (docs/archive/problems/object-graph-registration-and-anchor-gaps-2026-10-14.md).
func TestEveryWrittenKindIsRegistered(t *testing.T) {
	objects, edges := scanKindLiterals(t)
	if len(objects) < 40 || len(edges) < 20 {
		t.Fatalf("scan found %d object kinds and %d edge kinds; the scanner is broken", len(objects), len(edges))
	}
	reg := DefaultRegistry()
	for _, kind := range sortedKeys(objects) {
		if _, err := reg.LookupKind(ObjectKind(kind)); err != nil {
			t.Errorf("object kind %q is written at %s but not registered in DefaultRegistry", kind, strings.Join(objects[kind], ", "))
		}
	}
	for _, kind := range sortedKeys(edges) {
		if _, err := reg.LookupEdge(EdgeKind(kind)); err != nil {
			t.Errorf("edge kind %q is written at %s but not registered in DefaultRegistry", kind, strings.Join(edges[kind], ", "))
		}
	}
	if os.Getenv("OBJECTGRAPH_KIND_INVENTORY") != "" {
		for _, kind := range sortedKeys(objects) {
			t.Logf("object %s %d %s", kind, len(objects[kind]), objects[kind][0])
		}
		for _, kind := range sortedKeys(edges) {
			t.Logf("edge %s %d %s", kind, len(edges[kind]), edges[kind][0])
		}
	}
}

// TestEveryRegisteredKindHasAWriter is H-OG2's standing check: a registered
// object kind that no production code names is dead registration (the
// retired choir.universal_wire_story_cluster was one). Kinds reserved for
// planned work must say so with Reserved.
func TestEveryRegisteredKindHasAWriter(t *testing.T) {
	objects, _ := scanKindLiterals(t)
	for _, reg := range DefaultRegistry().AllKinds() {
		_, named := objects[string(reg.Kind)]
		switch {
		case reg.Reserved && named:
			t.Errorf("object kind %q is marked Reserved but production code names it at %s; drop Reserved", reg.Kind, strings.Join(objects[string(reg.Kind)], ", "))
		case !reg.Reserved && !named:
			t.Errorf("object kind %q is registered but no production code names it; delete the registration or mark it Reserved with a reason", reg.Kind)
		}
	}
}
