package gomodule

import (
	"bytes"
	"encoding/json"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing/fstest"
)

// ModuleSource is one module's whole content as a release carries it: every
// file, by its path relative to the module root.
type ModuleSource struct {
	Module Module
	Files  map[string][]byte
}

// BundleFile is one file of a bundled module, by its path relative to the
// module root.
type BundleFile struct {
	Name string `json:"name"`
	Data []byte `json:"data"`
}

// BundleModule is one module of a bundle with its whole content, so the
// worker can re-hash it against its h1: (review r1, D2).
type BundleModule struct {
	Path    string       `json:"path"`
	Version string       `json:"version"`
	Sum     string       `json:"sum"`
	Files   []BundleFile `json:"files"`
}

// Bundle is what a desk worker loads: the whole content of every module the
// desk's closure reaches (with their curated hashes) and the packages the
// desk's cells may import. Lists are sorted, so the encoding is canonical. The
// worker re-runs every check (Load) and exposes to the interpreter only the
// non-test Go files of the packages the imports reach.
type Bundle struct {
	Modules []BundleModule `json:"modules"`
	Imports []string       `json:"imports"`
}

// Empty reports a bundle that loads nothing.
func (b Bundle) Empty() bool { return len(b.Imports) == 0 && len(b.Modules) == 0 }

// nonGoInputs are the build inputs a package directory may not hold: Go
// would compile or link them (cgo's C, C++, Objective-C, Fortran and SWIG
// inputs, assembly, prebuilt objects), yaegi cannot interpret them, and each
// is a way around the import check.
var nonGoInputs = map[string]bool{
	".s": true, ".S": true, ".sx": true, ".c": true, ".cc": true, ".cpp": true, ".cxx": true,
	".h": true, ".hh": true, ".hpp": true, ".hxx": true, ".m": true, ".mm": true, ".f": true, ".F": true,
	".for": true, ".f90": true, ".syso": true, ".swig": true, ".swigcxx": true,
}

var goFileName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*\.go$`)

// ignoredDir reports a directory the go tool never builds from: testdata and
// any element starting with _ or a dot.
func ignoredDir(dir string) bool {
	if dir == "." {
		return false
	}
	for _, e := range strings.Split(dir, "/") {
		if e == "testdata" || strings.HasPrefix(e, "_") || strings.HasPrefix(e, ".") {
			return true
		}
	}
	return false
}

// pkgFiles is one package directory: its non-test Go files and whether it
// holds a non-Go build input.
type pkgFiles struct {
	module  string
	files   map[string][]byte
	nonGo   string
	present bool
}

// index maps every package import path of the modules to its files.
func index(mods []ModuleSource) map[string]*pkgFiles {
	out := map[string]*pkgFiles{}
	for _, m := range mods {
		for rel, data := range m.Files {
			dir, name := path.Split(rel)
			dir = strings.TrimSuffix(dir, "/")
			if dir == "" {
				dir = "."
			}
			if ignoredDir(dir) {
				continue
			}
			imp := m.Module.Path
			if dir != "." {
				imp += "/" + dir
			}
			p := out[imp]
			if p == nil {
				p = &pkgFiles{module: m.Module.Path, files: map[string][]byte{}}
				out[imp] = p
			}
			switch ext := path.Ext(name); {
			case ext == ".go" && !strings.HasSuffix(name, "_test.go") && !strings.HasPrefix(name, "_") && !strings.HasPrefix(name, "."):
				p.files[name] = data
				p.present = true
			case nonGoInputs[ext]:
				p.nonGo = name
			}
		}
	}
	return out
}

// moduleOf is the declared module providing import path p (longest prefix).
func moduleOf(mods []Module, p string) (Module, bool) {
	var best Module
	found := false
	for _, m := range mods {
		if (p == m.Path || strings.HasPrefix(p, m.Path+"/")) && len(m.Path) > len(best.Path) {
			best, found = m, true
		}
	}
	return best, found
}

// internalParent is the tree an internal package may be imported from, or ""
// when p is not internal.
func internalParent(p string) string {
	if strings.HasSuffix(p, "/internal") {
		return strings.TrimSuffix(p, "/internal")
	}
	if i := strings.LastIndex(p, "/internal/"); i >= 0 {
		return p[:i]
	}
	return ""
}

func isInternal(p string) bool { return internalParent(p) != "" || strings.HasPrefix(p, "internal/") }

// checkFile parses one Go file (the whole file, so a directive anywhere is
// seen) and returns its imports, refusing cgo, banned directives and imports
// outside the desk set. It never type-checks or runs the code.
func checkFile(chain, pkg, name string, src []byte) ([]string, error) {
	where := pkg + "/" + name
	f, err := parser.ParseFile(token.NewFileSet(), where, src, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return nil, refuse(CodeParse, "%s: not Go source: %v", where, err)
	}
	if f.Name.Name == "choir" {
		return nil, refuse(CodeReservedName, "%s (%s): package name choir is the desk's host verbs", where, chain)
	}
	for _, cg := range f.Comments {
		for _, c := range cg.List {
			for _, d := range []string{"//go:linkname", "//go:embed", "//go:cgo_"} {
				if strings.HasPrefix(c.Text, d) {
					return nil, refuse(CodeDirective, "%s (%s): directive %s is forbidden", where, chain, strings.Fields(c.Text)[0])
				}
			}
		}
	}
	var out []string
	for _, imp := range f.Imports {
		p, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			return nil, refuse(CodeParse, "%s: bad import %s", where, imp.Path.Value)
		}
		switch {
		case p == "C":
			return nil, refuse(CodeCgo, "%s (%s) imports \"C\": cgo is forbidden", where, chain)
		case strings.HasPrefix(p, ".") || strings.HasPrefix(p, "/"):
			return nil, refuse(CodeUnresolved, "%s (%s): relative import %q", where, chain, p)
		}
		if r, ok := BannedReason(p); ok {
			return nil, refuse(CodeBannedImport, "%s (%s) imports %q: %s", where, chain, p, r)
		}
		if IsStdlibPath(p) && !InStdlib(p) {
			return nil, refuse(CodeOutsideSet, "%s (%s) imports %q, which is outside the desk import set", where, chain, p)
		}
		out = append(out, p)
	}
	return out, nil
}

// walk checks the closure from entries over pkgs and returns the reached
// packages in visit order. resolve says whether a non-standard import is a
// package the walk can reach.
func walk(entries []string, pkgs map[string]*pkgFiles, mods []Module) ([]string, error) {
	type item struct{ pkg, chain string }
	var queue []item
	seen := map[string]bool{}
	for _, e := range entries {
		if !seen[e] {
			seen[e] = true
			queue = append(queue, item{e, e})
		}
	}
	var order []string
	total := 0
	for len(queue) > 0 {
		it := queue[0]
		queue = queue[1:]
		if _, ok := moduleOf(mods, it.pkg); !ok {
			return nil, refuse(CodeUnresolved, "%s: no declared module provides %q", it.chain, it.pkg)
		}
		if path.Base(it.pkg) == "choir" {
			// A cell's import whose last element is choir is bound to the
			// host verbs (yaegikernel aliasChoirImports), never to a module.
			return nil, refuse(CodeReservedName, "%s: package %q ends in choir, the desk's host verbs", it.chain, it.pkg)
		}
		p := pkgs[it.pkg]
		if p == nil || !p.present {
			return nil, refuse(CodeUnresolved, "%s: package %q has no Go source in its module", it.chain, it.pkg)
		}
		if p.nonGo != "" {
			return nil, refuse(CodeNonGoSource, "%s: package %q holds %s, a non-Go build input", it.chain, it.pkg, p.nonGo)
		}
		order = append(order, it.pkg)
		if len(order) > MaxClosurePackages {
			return nil, refuse(CodeTooLarge, "the closure reaches more than %d packages", MaxClosurePackages)
		}
		names := make([]string, 0, len(p.files))
		for n := range p.files {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, n := range names {
			if !goFileName.MatchString(n) {
				return nil, refuse(CodeBadBundle, "%s: file name %q", it.pkg, n)
			}
			total += len(p.files[n])
			if total > MaxBundleBytes {
				return nil, refuse(CodeTooLarge, "the closure's Go source is over %d bytes", MaxBundleBytes)
			}
			imps, err := checkFile(it.chain, it.pkg, n, p.files[n])
			if err != nil {
				return nil, err
			}
			for _, q := range imps {
				if IsStdlibPath(q) {
					continue
				}
				if parent := internalParent(q); parent != "" && it.pkg != parent && !strings.HasPrefix(it.pkg, parent+"/") {
					return nil, refuse(CodeInternal, "%s imports %q, an internal package of another tree", it.chain, q)
				}
				if !seen[q] {
					seen[q] = true
					queue = append(queue, item{q, it.chain + " -> " + q})
				}
			}
		}
	}
	return order, nil
}

// Verify is the curated-list half of the gate: no more than MaxModules, no
// module path twice or inside another's (a prefix would make the module that
// provides a package ambiguous), each on the curated list at the same version,
// within the size caps, and hashing to its curated h1: from these bytes. It
// returns the modules with their sums, sorted by path.
func Verify(curated []Module, mods []ModuleSource) ([]Module, error) {
	if len(mods) > MaxModules {
		return nil, refuse(CodeTooLarge, "%d modules; at most %d", len(mods), MaxModules)
	}
	cur := map[string]Module{}
	for _, c := range curated {
		cur[c.Path] = c
	}
	var declared []Module
	for _, m := range mods {
		if err := CheckPath(m.Module.Path); err != nil {
			return nil, refuse(CodeBadRef, "%v", err)
		}
		c, ok := cur[m.Module.Path]
		switch {
		case !ok:
			return nil, refuse(CodeNotCurated, "%s is not on the curated list", m.Module)
		case c.Version != m.Module.Version:
			return nil, refuse(CodeNotCurated, "%s: the curated list has %s", m.Module, c)
		}
		for n := range m.Files {
			if n == "" || path.Clean(n) != n || strings.HasPrefix(n, "/") || strings.HasPrefix(n, "../") || n == ".." || strings.ContainsAny(n, "\\\x00") {
				return nil, refuse(CodeBadBundle, "%s: file name %q is not a clean relative path", m.Module, n)
			}
		}
		if len(m.Files) > MaxModuleFiles {
			return nil, refuse(CodeTooLarge, "%s has %d files; at most %d", m.Module, len(m.Files), MaxModuleFiles)
		}
		size := 0
		for _, d := range m.Files {
			size += len(d)
		}
		if size > MaxModuleBytes {
			return nil, refuse(CodeTooLarge, "%s is %d bytes; at most %d", m.Module, size, MaxModuleBytes)
		}
		sum, err := Hash1(m.Module.String(), m.Files)
		if err != nil {
			return nil, err
		}
		if sum != c.Sum {
			return nil, refuse(CodeSumMismatch, "%s hashes to %s; the curated list (the Go checksum database) has %s", m.Module, sum, c.Sum)
		}
		declared = append(declared, Module{Path: c.Path, Version: c.Version, Sum: c.Sum})
	}
	sort.Slice(declared, func(i, j int) bool { return declared[i].Path < declared[j].Path })
	for i := 1; i < len(declared); i++ {
		if declared[i-1].Path == declared[i].Path {
			return nil, refuse(CodeDuplicateMod, "%s is declared twice", declared[i].Path)
		}
	}
	for _, a := range declared {
		for _, b := range declared {
			if strings.HasPrefix(b.Path, a.Path+"/") {
				return nil, refuse(CodeOverlap, "module %s lies inside module %s", b.Path, a.Path)
			}
		}
	}
	return declared, nil
}

// checkImports refuses a cell import list that is too long, repeated,
// malformed, internal or reserved.
func checkImports(imports []string) ([]string, error) {
	if len(imports) > MaxImports {
		return nil, refuse(CodeTooLarge, "%d imports; at most %d", len(imports), MaxImports)
	}
	entries := append([]string{}, imports...)
	sort.Strings(entries)
	for i, e := range entries {
		if i > 0 && entries[i-1] == e {
			return nil, refuse(CodeBadRef, "import %q is named twice", e)
		}
		if err := CheckPath(e); err != nil {
			return nil, refuse(CodeUnresolved, "import %v", err)
		}
		if isInternal(e) {
			return nil, refuse(CodeInternal, "a cell may not import %q, an internal package", e)
		}
	}
	return entries, nil
}

// Resolve is the publish-time gate (R42) for one desk: Verify, then the
// import closure from imports (the packages the desk's cells may import) is
// walked and checked. It returns the bundle the desk worker loads: the whole
// content of every module the closure reaches, at most MaxBundleBytes. An
// empty imports list yields an empty bundle (the modules are still verified).
func Resolve(curated []Module, mods []ModuleSource, imports []string) (Bundle, error) {
	declared, err := Verify(curated, mods)
	if err != nil {
		return Bundle{}, err
	}
	entries, err := checkImports(imports)
	if err != nil {
		return Bundle{}, err
	}
	if len(entries) == 0 {
		return Bundle{Modules: []BundleModule{}, Imports: []string{}}, nil
	}
	order, err := walk(entries, index(mods), declared)
	if err != nil {
		return Bundle{}, err
	}
	reached := map[string]bool{}
	for _, p := range order {
		m, _ := moduleOf(declared, p)
		reached[m.Path] = true
	}
	b := Bundle{Imports: entries}
	total := 0
	for _, d := range declared {
		if !reached[d.Path] {
			continue
		}
		bm := BundleModule{Path: d.Path, Version: d.Version, Sum: d.Sum}
		for _, m := range mods {
			if m.Module.Path != d.Path {
				continue
			}
			names := make([]string, 0, len(m.Files))
			for n := range m.Files {
				names = append(names, n)
			}
			sort.Strings(names)
			for _, n := range names {
				total += len(m.Files[n])
				bm.Files = append(bm.Files, BundleFile{Name: n, Data: m.Files[n]})
			}
		}
		b.Modules = append(b.Modules, bm)
	}
	if total > MaxBundleBytes {
		return Bundle{}, refuse(CodeTooLarge, "the modules the closure reaches are %d bytes; a desk's bundle is at most %d", total, MaxBundleBytes)
	}
	return b, nil
}

// Loaded is a bundle that passed every check: the interpreter's source
// filesystem and the packages in it.
type Loaded struct {
	// FS is a read-only GOPATH tree (src/<import path>/<file>) for yaegi's
	// SourcecodeFilesystem, with GoPath ".": the non-test Go files of the
	// packages the imports reach, and nothing else.
	FS fs.FS
	// Imports are the packages the desk's cells may import.
	Imports []string
	// Packages are every package in FS.
	Packages []string
}

// Load is the worker's check of a bundle (it trusts only what it checks):
// every module is re-hashed against its h1: and must be on curated at that
// version and sum; the imports and the whole closure are re-walked; every
// module carried must be reached; the total is within MaxBundleBytes.
func (b Bundle) Load(curated []Module) (Loaded, error) {
	var mods []ModuleSource
	total := 0
	for i, m := range b.Modules {
		if i > 0 && b.Modules[i-1].Path >= m.Path {
			return Loaded{}, refuse(CodeBadBundle, "modules are not sorted and unique")
		}
		ms := ModuleSource{Module: Module{Path: m.Path, Version: m.Version}, Files: map[string][]byte{}}
		for j, f := range m.Files {
			if j > 0 && m.Files[j-1].Name >= f.Name {
				return Loaded{}, refuse(CodeBadBundle, "%s: files are not sorted and unique", m.Path)
			}
			total += len(f.Data)
			ms.Files[f.Name] = f.Data
		}
		mods = append(mods, ms)
	}
	if total > MaxBundleBytes {
		return Loaded{}, refuse(CodeTooLarge, "bundle content is %d bytes; at most %d", total, MaxBundleBytes)
	}
	declared, err := Verify(curated, mods)
	if err != nil {
		return Loaded{}, err
	}
	for i, d := range declared {
		if d.Sum != b.Modules[i].Sum {
			return Loaded{}, refuse(CodeSumMismatch, "%s: the bundle names %s; the curated list has %s", d, b.Modules[i].Sum, d.Sum)
		}
	}
	entries, err := checkImports(b.Imports)
	if err != nil {
		return Loaded{}, err
	}
	for i := range entries {
		if entries[i] != b.Imports[i] {
			return Loaded{}, refuse(CodeBadBundle, "imports are not sorted and unique")
		}
	}
	if len(entries) == 0 {
		if len(mods) > 0 {
			return Loaded{}, refuse(CodeBadBundle, "the bundle carries modules and no imports")
		}
		return Loaded{FS: fstest.MapFS{}}, nil
	}
	pkgs := index(mods)
	order, err := walk(entries, pkgs, declared)
	if err != nil {
		return Loaded{}, err
	}
	reached := map[string]bool{}
	out := fstest.MapFS{}
	sort.Strings(order)
	for _, p := range order {
		m, _ := moduleOf(declared, p)
		reached[m.Path] = true
		for name, data := range pkgs[p].files {
			out["src/"+p+"/"+name] = &fstest.MapFile{Data: append([]byte{}, data...), Mode: 0o444}
		}
	}
	for _, d := range declared {
		if !reached[d.Path] {
			return Loaded{}, refuse(CodeBadBundle, "the bundle carries %s, which its imports do not reach", d)
		}
	}
	return Loaded{FS: out, Imports: entries, Packages: order}, nil
}

// MaxEncodedBundleBytes bounds an encoded bundle (base64 inflates the content
// by a third, plus the JSON framing).
const MaxEncodedBundleBytes = MaxBundleBytes*4/3 + 64<<10

// Encode is the canonical encoding (sorted lists, JSON).
func (b Bundle) Encode() ([]byte, error) {
	if b.Imports == nil {
		b.Imports = []string{}
	}
	if b.Modules == nil {
		b.Modules = []BundleModule{}
	}
	return json.Marshal(b)
}

// DecodeBundle decodes strictly (no unknown field, one value, bounded). The
// caller then Loads it, which runs every check.
func DecodeBundle(raw []byte) (Bundle, error) {
	if len(raw) > MaxEncodedBundleBytes {
		return Bundle{}, refuse(CodeTooLarge, "encoded bundle is %d bytes; at most %d", len(raw), MaxEncodedBundleBytes)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var b Bundle
	if err := dec.Decode(&b); err != nil {
		return Bundle{}, refuse(CodeBadBundle, "%v", err)
	}
	if _, err := dec.Token(); err != io.EOF {
		return Bundle{}, refuse(CodeBadBundle, "trailing data after the bundle")
	}
	return b, nil
}
