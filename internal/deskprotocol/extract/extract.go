// Package extract reads the desk protocol surface out of the source tree with
// the standard library's go/types. It adds no module dependency.
package extract

import (
	"fmt"
	"go/ast"
	"go/build"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/deskprotocol"
)

const modulePrefix = "github.com/yusefmosiah/go-choir/"

// Surface extracts the surface of the tree rooted at root. The tree must be a
// checkout of this module; the process works from root as its directory.
func Surface(root string) (*deskprotocol.Surface, error) {
	prev, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	if err := os.Chdir(root); err != nil {
		return nil, err
	}
	defer os.Chdir(prev)
	// A fixed build context: the surface must not depend on the machine.
	ctxt := build.Default
	ctxt.GOOS, ctxt.GOARCH, ctxt.CgoEnabled = "linux", "amd64", false
	old := build.Default
	build.Default = ctxt
	defer func() { build.Default = old }()

	fset := token.NewFileSet()
	e := &extractor{
		root:  root,
		fset:  fset,
		imp:   importer.ForCompiler(fset, "source", nil),
		pkgs:  map[string]*types.Package{},
		short: map[string]string{},
		out: &deskprotocol.Surface{
			Schema:      deskprotocol.SurfaceSchema,
			Version:     deskprotocol.Version,
			Names:       map[string]deskprotocol.Name{},
			Verbs:       map[string]deskprotocol.Verb{},
			Desks:       map[string]deskprotocol.DeskRow{},
			Transitions: map[string][]string{},
		},
	}
	for _, r := range deskprotocol.Roots {
		if err := e.root1(r); err != nil {
			return nil, fmt.Errorf("root %s: %w", r.Name, err)
		}
	}
	for len(e.queue) > 0 {
		o := e.queue[0]
		e.queue = e.queue[1:]
		if err := e.named(o); err != nil {
			return nil, err
		}
	}
	if err := e.verbs(); err != nil {
		return nil, err
	}
	e.desks()
	e.transitions()
	if err := e.out.Seal(); err != nil {
		return nil, err
	}
	return e.out, nil
}

type extractor struct {
	root   string
	fset   *token.FileSet
	imp    types.Importer
	pkgs   map[string]*types.Package // by short name
	short  map[string]string         // short name -> path
	out    *deskprotocol.Surface
	queue  []*types.TypeName
	seen   map[string]bool
	dirs   map[string]deskprotocol.Direction
	openOf map[string]bool
}

func (e *extractor) pkg(short string) (*types.Package, error) {
	if p, ok := e.pkgs[short]; ok {
		return p, nil
	}
	path, ok := deskprotocol.Packages[short]
	if !ok {
		return nil, fmt.Errorf("package %q is not in deskprotocol.Packages", short)
	}
	p, err := e.imp.Import(path)
	if err != nil {
		return nil, fmt.Errorf("load %s: %w", path, err)
	}
	e.pkgs[short] = p
	return p, nil
}

func (e *extractor) qual(p *types.Package) string {
	if prev, ok := e.short[p.Name()]; ok && prev != p.Path() {
		panic("package name collision: " + p.Name() + " " + prev + " " + p.Path())
	}
	e.short[p.Name()] = p.Path()
	return p.Name()
}

func (e *extractor) typeString(t types.Type) string { return types.TypeString(t, e.qual) }

func inModule(p *types.Package) bool {
	return p != nil && strings.HasPrefix(p.Path(), modulePrefix)
}

func (e *extractor) root1(r deskprotocol.Root) error {
	short, ident, ok := strings.Cut(r.Name, ".")
	if !ok {
		return fmt.Errorf("not pkg.Ident")
	}
	if e.seen == nil {
		e.seen = map[string]bool{}
		e.dirs = map[string]deskprotocol.Direction{}
		e.openOf = map[string]bool{}
	}
	if r.FromSource {
		return e.fromSource(short, ident, r)
	}
	p, err := e.pkg(short)
	if err != nil {
		return err
	}
	if prefix, glob := strings.CutSuffix(ident, "*"); glob {
		values := map[string]string{}
		for _, n := range p.Scope().Names() {
			c, ok := p.Scope().Lookup(n).(*types.Const)
			if !ok || !strings.HasPrefix(n, prefix) || !c.Exported() {
				continue
			}
			if _, named := c.Type().(*types.Named); named {
				continue // belongs to its enum
			}
			values[n] = c.Val().ExactString()
		}
		if len(values) == 0 {
			return fmt.Errorf("matches no untyped constant")
		}
		e.out.Names[r.Name] = deskprotocol.Name{Kind: "constgroup", Direction: r.Direction, Open: r.Open, Values: values}
		return nil
	}
	obj := p.Scope().Lookup(ident)
	if obj == nil {
		return fmt.Errorf("does not resolve")
	}
	switch o := obj.(type) {
	case *types.TypeName:
		e.dirs[short+"."+ident] = r.Direction
		e.enqueue(o, r.Direction, r.Open)
	case *types.Const:
		e.out.Names[r.Name] = deskprotocol.Name{Kind: "const", Direction: r.Direction, Quota: r.Quota,
			Underlying: e.typeString(o.Type()), Value: o.Val().ExactString()}
	case *types.Func:
		sig := o.Type().(*types.Signature)
		e.out.Names[r.Name] = deskprotocol.Name{Kind: "func", Direction: r.Direction, Sig: e.sig(sig)}
		e.walkSig(sig, r.Direction)
	default:
		return fmt.Errorf("is a %T, not a type, constant or function", obj)
	}
	return nil
}

func (e *extractor) enqueue(o *types.TypeName, dir deskprotocol.Direction, open bool) {
	key := o.Pkg().Name() + "." + o.Name()
	if open {
		e.openOf[key] = true
	}
	if cur, ok := e.dirs[key]; ok && cur != dir && e.seen[key] {
		e.dirs[key] = deskprotocol.Both
		if n, ok := e.out.Names[key]; ok {
			n.Direction = deskprotocol.Both
			e.out.Names[key] = n
		}
		return
	}
	if e.seen[key] {
		return
	}
	e.seen[key] = true
	e.dirs[key] = dir
	e.queue = append(e.queue, o)
}

func (e *extractor) sig(s *types.Signature) *deskprotocol.Sig {
	out := &deskprotocol.Sig{Params: []string{}, Results: []string{}, Variadic: s.Variadic()}
	for i := 0; i < s.Params().Len(); i++ {
		v := s.Params().At(i)
		t := e.typeString(v.Type())
		out.Params = append(out.Params, t)
		out.Names = append(out.Names, v.Name())
	}
	for i := 0; i < s.Results().Len(); i++ {
		out.Results = append(out.Results, e.typeString(s.Results().At(i).Type()))
	}
	return out
}

func (e *extractor) walkSig(s *types.Signature, dir deskprotocol.Direction) {
	for i := 0; i < s.Params().Len(); i++ {
		e.walk(s.Params().At(i).Type(), dir)
	}
	for i := 0; i < s.Results().Len(); i++ {
		e.walk(s.Results().At(i).Type(), dir)
	}
}

// walk enqueues every in-module named type t references.
func (e *extractor) walk(t types.Type, dir deskprotocol.Direction) {
	switch t := t.(type) {
	case *types.Named:
		if inModule(t.Obj().Pkg()) {
			e.enqueue(t.Obj(), dir, false)
		}
		for i := 0; i < t.TypeArgs().Len(); i++ {
			e.walk(t.TypeArgs().At(i), dir)
		}
	case *types.Alias:
		e.walk(types.Unalias(t), dir)
	case *types.Pointer:
		e.walk(t.Elem(), dir)
	case *types.Slice:
		e.walk(t.Elem(), dir)
	case *types.Array:
		e.walk(t.Elem(), dir)
	case *types.Chan:
		e.walk(t.Elem(), dir)
	case *types.Map:
		e.walk(t.Key(), dir)
		e.walk(t.Elem(), dir)
	case *types.Signature:
		e.walkSig(t, dir)
	case *types.Struct:
		for i := 0; i < t.NumFields(); i++ {
			e.walk(t.Field(i).Type(), dir)
		}
	}
}

func optionalType(t types.Type) bool {
	switch types.Unalias(t).Underlying().(type) {
	case *types.Pointer, *types.Slice, *types.Map, *types.Interface:
		return true
	}
	return false
}

func (e *extractor) named(o *types.TypeName) error {
	key := o.Pkg().Name() + "." + o.Name()
	dir := e.dirs[key]
	n := deskprotocol.Name{Direction: dir, Open: e.openOf[key]}
	switch u := o.Type().Underlying().(type) {
	case *types.Struct:
		n.Kind = "struct"
		n.Fields = map[string]deskprotocol.Field{}
		for i := 0; i < u.NumFields(); i++ {
			f := u.Field(i)
			if !f.Exported() {
				continue
			}
			tag := reflect.StructTag(u.Tag(i)).Get("json")
			if tag == "-" {
				continue
			}
			name, opts, _ := strings.Cut(tag, ",")
			if name == "" {
				name = f.Name()
			}
			omit := false
			for _, opt := range strings.Split(opts, ",") {
				if opt == "omitempty" || opt == "omitzero" {
					omit = true
				}
			}
			n.Fields[f.Name()] = deskprotocol.Field{
				Type: e.typeString(f.Type()), JSON: name,
				Optional: omit || optionalType(f.Type()), Embedded: f.Embedded(),
			}
			e.walk(f.Type(), dir)
		}
	case *types.Interface:
		n.Kind = "interface"
		n.Methods = map[string]*deskprotocol.Sig{}
		for i := 0; i < u.NumMethods(); i++ {
			m := u.Method(i)
			s := m.Type().(*types.Signature)
			n.Methods[m.Name()] = e.sig(s)
			e.walkSig(s, dir)
		}
	case *types.Basic:
		n.Underlying = u.Name()
		values := map[string]string{}
		p := o.Pkg()
		for _, name := range p.Scope().Names() {
			if c, ok := p.Scope().Lookup(name).(*types.Const); ok && c.Exported() && types.Identical(c.Type(), o.Type()) {
				values[name] = c.Val().ExactString()
			}
		}
		if len(values) > 0 {
			n.Kind, n.Values = "enum", values
		} else {
			n.Kind = "type"
		}
	default:
		n.Kind = "type"
		n.Underlying = e.typeString(u)
		e.walk(u, dir)
	}
	e.out.Names[key] = n
	return nil
}

// fromSource reads a function's signature from its declaration, without
// type-checking its package.
func (e *extractor) fromSource(short, ident string, r deskprotocol.Root) error {
	path, ok := deskprotocol.Packages[short]
	if !ok {
		return fmt.Errorf("package %q is not in deskprotocol.Packages", short)
	}
	dir := filepath.Join(e.root, strings.TrimPrefix(path, modulePrefix))
	pkgs, err := parser.ParseDir(e.fset, dir, func(fi os.FileInfo) bool { return !strings.HasSuffix(fi.Name(), "_test.go") }, parser.SkipObjectResolution)
	if err != nil {
		return err
	}
	for _, p := range pkgs {
		for _, f := range p.Files {
			for _, d := range f.Decls {
				fd, ok := d.(*ast.FuncDecl)
				if !ok || fd.Recv != nil || fd.Name.Name != ident {
					continue
				}
				s := &deskprotocol.Sig{Params: []string{}, Results: []string{}}
				for _, fl := range fd.Type.Params.List {
					t := types.ExprString(fl.Type)
					if _, v := fl.Type.(*ast.Ellipsis); v {
						s.Variadic = true
					}
					names := fl.Names
					if len(names) == 0 {
						s.Params, s.Names = append(s.Params, t), append(s.Names, "_")
					}
					for _, nm := range names {
						s.Params, s.Names = append(s.Params, t), append(s.Names, nm.Name)
					}
				}
				if fd.Type.Results != nil {
					for _, fl := range fd.Type.Results.List {
						n := len(fl.Names)
						if n == 0 {
							n = 1
						}
						for i := 0; i < n; i++ {
							s.Results = append(s.Results, types.ExprString(fl.Type))
						}
					}
				}
				e.out.Names[r.Name] = deskprotocol.Name{Kind: "func", Direction: r.Direction, Sig: s}
				return nil
			}
		}
	}
	return fmt.Errorf("function declaration not found in %s", dir)
}

// verbs records the signature of every choir verb any desk holds, plus the
// observation verbs. A verb with no method on the scope is a closure.
func (e *extractor) verbs() error {
	short, ident, _ := strings.Cut(deskprotocol.ChoirScopeType, ".")
	p, err := e.pkg(short)
	if err != nil {
		return err
	}
	obj, ok := p.Scope().Lookup(ident).(*types.TypeName)
	if !ok {
		return fmt.Errorf("%s does not resolve", deskprotocol.ChoirScopeType)
	}
	ms := types.NewMethodSet(types.NewPointer(obj.Type()))
	names := map[string]bool{}
	for _, v := range deskprotocol.ObservationVerbs {
		names[v] = true
	}
	for _, d := range deskmanifest.Names() {
		desk, _ := deskmanifest.Lookup(d)
		if desk.Kind == deskmanifest.KindApp {
			continue
		}
		for _, v := range desk.Verbs {
			names[v] = true
		}
	}
	for v := range names {
		sel := ms.Lookup(p, v)
		if sel == nil {
			e.out.Verbs[v] = deskprotocol.Verb{Closure: true}
			continue
		}
		s := sel.Type().(*types.Signature)
		e.out.Verbs[v] = deskprotocol.Verb{Sig: e.sig(s)}
		e.walkSig(s, deskprotocol.Both)
	}
	for len(e.queue) > 0 { // types the verbs reference
		o := e.queue[0]
		e.queue = e.queue[1:]
		if err := e.named(o); err != nil {
			return err
		}
	}
	return nil
}

func (e *extractor) desks() {
	for _, name := range deskmanifest.Names() {
		d, _ := deskmanifest.Lookup(name)
		if d.Kind == deskmanifest.KindApp {
			continue // registered at run time by a package, not part of the base
		}
		e.out.Desks[name] = DeskRowOf(d)
	}
}

// DeskRowOf is the lock's row for a registry desk: every field
// deskmanifest.Desk.Equal compares except Name, which is the map key.
func DeskRowOf(d deskmanifest.Desk) deskprotocol.DeskRow {
	srt := func(in []string) []string {
		out := append([]string{}, in...)
		sort.Strings(out)
		return out
	}
	return deskprotocol.DeskRow{
		Kind: string(d.Kind), Verbs: srt(d.Verbs),
		CapsuleVerbs: srt(d.CapsuleVerbs), Modules: srt(d.Modules), OwnedKinds: srt(d.OwnedKinds),
		Carrier: string(d.Carrier), Instance: string(d.Instance), Taint: string(d.Taint),
		Sandbox: d.Sandbox.Profile, SandboxUID: d.Sandbox.UID,
		ModelRole: d.ModelRole, Restart: d.Restart, Profile: d.Profile, Prompt: d.Prompt,
		Identity: string(d.Identity), OwnChannel: d.OwnChannel, CellDeadlineNs: int64(d.CellDeadline),
		WorkerMaxBytes: d.Memory.WorkerMaxBytes, WorkerHighBytes: d.Memory.WorkerHighBytes,
		CapsuleRequestedBytes: d.Memory.CapsuleRequestedBytes,
		LeaseKinds:            srt(d.LeaseKinds), LeaseCaptures: d.LeaseCaptures, LeaseGrants: srt(d.LeaseGrants), FirstParty: d.FirstParty,
		Grants: d.GrantStrings(), EndsRun: d.EndsRun,
	}
}

func (e *extractor) transitions() {
	e.out.Enforced = []string{}
	for _, m := range deskprotocol.Machines() {
		if deskprotocol.Enforced(m) {
			e.out.Enforced = append(e.out.Enforced, string(m))
		}
	}
	sort.Strings(e.out.Enforced)
	for _, t := range deskprotocol.Transitions() {
		m := string(t.Machine)
		e.out.Transitions[m] = append(e.out.Transitions[m], t.From+">"+t.To)
	}
}
