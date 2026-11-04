package apppackage

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// SurfaceDomain versions the behaviour-surface hash's input format.
const SurfaceDomain = "choir.behaviour-surface/v1"

// MaxSurfaceFileBytes bounds each file the hash reads.
const MaxSurfaceFileBytes = 256 << 10

// FileReader reads one file of the package by its package-relative path. The
// path has passed checkPath; a reader must still refuse anything outside the
// package (DirReader does; a git tree reader cannot name one).
type FileReader func(path string) ([]byte, error)

// DirReader reads files under root and refuses a symlink, a non-regular file,
// a path that leaves root, and a file over MaxSurfaceFileBytes.
func DirReader(root string) FileReader {
	return func(p string) ([]byte, error) {
		if err := checkPathAny(p, ".md", ".txt", ".go", ".svelte"); err != nil {
			return nil, err
		}
		cur := root
		for _, part := range strings.Split(p, "/") {
			cur = filepath.Join(cur, part)
			fi, err := os.Lstat(cur)
			if err != nil {
				return nil, err
			}
			if fi.Mode()&os.ModeSymlink != 0 {
				return nil, fmt.Errorf("%q passes through a symlink", p)
			}
		}
		fi, err := os.Lstat(cur)
		if err != nil || !fi.Mode().IsRegular() {
			return nil, fmt.Errorf("%q is not a regular file", p)
		}
		if fi.Size() > MaxSurfaceFileBytes {
			return nil, fmt.Errorf("%q is over %d bytes", p, MaxSurfaceFileBytes)
		}
		return os.ReadFile(cur)
	}
}

// baseImportSet is every desk's yaegi import set before grants add to it: the
// precommitment-records protocol package (choir) and the pure standard library
// packages of internal/yaegikernel.DefaultSafeStdlibPackages (a test holds the
// two together). Nothing with effects is here (R26).
var baseImportSet = []string{
	"bytes", "choir", "encoding/base64", "encoding/hex", "encoding/json", "errors", "fmt",
	"math", "math/big", "math/rand/v2", "regexp", "sort", "strconv", "strings", "time",
	"unicode", "unicode/utf8",
}

// bannedImports are refused for desk code whatever the grants: the capability
// gate of R42, applied to the source the platform builds. "C" is cgo.
var bannedImports = map[string]bool{
	"unsafe": true, "reflect": true, "runtime": true, "runtime/cgo": true, "plugin": true,
	"syscall": true, "os": true, "os/exec": true, "os/signal": true, "net": true,
	"net/http": true, "net/rpc": true, "io/ioutil": true, "embed": true, "C": true,
	"debug/elf": true, "debug/macho": true, "debug/pe": true,
}

// ImportSet is the desk's yaegi import set, derived from its package's grants
// by the host and never authored. No grant implies a package yet, so the set is
// the base set; MA6.6 widens it by grant. It is part of the behaviour-surface
// hash.
func (m Manifest) ImportSet() []string {
	return append([]string{}, baseImportSet...)
}

// CuratedModules is the platform-reviewed list of third-party Go modules a
// desk may import (R42). It starts empty: a non-standard-library import is
// refused until a module is reviewed once for everyone.
func CuratedModules() []string { return nil }

// CheckGoSource refuses a desk source file whose imports leave the desk's
// import set: a banned package, cgo, a package outside the set, or a
// third-party module not on the curated list. It reads imports only (the file
// is never type-checked or run), and refuses a file that does not parse.
func CheckGoSource(name string, src []byte, allowed []string) error {
	// The whole file is parsed: a directive after the import block is scanned,
	// and a file that does not parse is refused (review N1).
	f, err := parser.ParseFile(token.NewFileSet(), name, src, parser.ParseComments)
	if err != nil {
		return invalid("%s: not Go source: %v", name, err)
	}
	ok := map[string]bool{}
	for _, a := range allowed {
		ok[a] = true
	}
	for _, a := range CuratedModules() {
		ok[a] = true
	}
	for _, imp := range f.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			return invalid("%s: bad import %s", name, imp.Path.Value)
		}
		switch {
		case bannedImports[path]:
			return invalid("%s: import %q is banned for desk code", name, path)
		case !ok[path]:
			return invalid("%s: import %q is outside the desk's import set", name, path)
		}
	}
	for _, cg := range f.Comments {
		for _, c := range cg.List {
			if strings.HasPrefix(c.Text, "//go:linkname") || strings.HasPrefix(c.Text, "//go:embed") || strings.HasPrefix(c.Text, "//go:cgo") {
				return invalid("%s: directive %q is banned for desk code", name, strings.Fields(c.Text)[0])
			}
		}
	}
	return nil
}

type surfaceFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type surfaceDesk struct {
	Name      string          `json:"name"`
	Prompt    surfaceFile     `json:"prompt"`
	Code      *surfaceFile    `json:"code"`
	Tools     []surfaceFile   `json:"tools"`
	Schedules []Schedule      `json:"schedules"`
	Accepts   json.RawMessage `json:"accepts"`
}

type surfaceKind struct {
	Kind                string          `json:"kind"`
	Identity            string          `json:"identity"`
	Versioned           bool            `json:"versioned"`
	OwnerEdits          bool            `json:"owner_edits"`
	SchemaVersion       int             `json:"schema_version"`
	ReadsSchemaVersions []int           `json:"reads_schema_versions"`
	Schema              json.RawMessage `json:"schema"`
}

type surfaceApp struct {
	Entry       string   `json:"entry"`
	ABI         string   `json:"abi"`
	RequiresSDK string   `json:"requires_sdk"`
	Aliases     []string `json:"aliases"`
}

type surfaceOpens struct {
	MediaType  string   `json:"media_type"`
	Extensions []string `json:"extensions"`
	Default    bool     `json:"default"`
}

type surfaceDoc struct {
	Domain       string         `json:"domain"`
	App          *surfaceApp    `json:"app"`
	Opens        []surfaceOpens `json:"opens"`
	Desks        []surfaceDesk  `json:"desks"`
	Kinds        []surfaceKind  `json:"kinds"`
	UsesKinds    []string       `json:"uses_kinds"`
	Capabilities []string       `json:"capabilities"`
	ImportSet    []string       `json:"import_set"`
}

// BehaviourSurface is the sha256 (lowercase hex) over what the package may do
// on a computer: the app's entry, abi, SDK range and aliases, the files it
// opens (media types and extensions, with whether it is the default opener);
// each desk's name, prompt file, code and tool files (by
// content), schedules and accepts; every kind with its schema and versions;
// the kinds it reads; the capability ceiling with its arguments (the egress
// hosts among them); and the derived import set. Any change moves the hash,
// and a changed hash is an update the owner approves even when grants are
// unchanged (plan section 5.1, red-team item 2). Authoring order does not
// matter: lists are sorted and JSON is canonical.
//
// It also runs the source gate: desk Go files that import outside the desk's
// set are refused here, so a hash is never issued for a package the gate
// would refuse. files may be nil only when the manifest names no desk file.
func (m Manifest) BehaviourSurface(files FileReader) (string, error) {
	imports := m.ImportSet()
	sort.Strings(imports)
	doc := surfaceDoc{
		Domain: SurfaceDomain, Desks: []surfaceDesk{}, Kinds: []surfaceKind{},
		UsesKinds: []string{}, Capabilities: []string{}, ImportSet: imports,
	}
	read := func(p string, isGo bool) (surfaceFile, error) {
		if files == nil {
			return surfaceFile{}, invalid("%s: no file reader to hash it", p)
		}
		raw, err := files(p)
		if err != nil {
			return surfaceFile{}, invalid("%s: %v", p, err)
		}
		if len(raw) > MaxSurfaceFileBytes {
			return surfaceFile{}, invalid("%s: over %d bytes", p, MaxSurfaceFileBytes)
		}
		if isGo {
			if err := CheckGoSource(p, raw, imports); err != nil {
				return surfaceFile{}, err
			}
		}
		sum := sha256.Sum256(raw)
		return surfaceFile{Path: p, SHA256: hex.EncodeToString(sum[:])}, nil
	}
	if m.App != nil {
		al := append([]string{}, m.App.Aliases...)
		sort.Strings(al)
		doc.App = &surfaceApp{Entry: m.App.Entry, ABI: m.App.ABI, RequiresSDK: m.App.RequiresSDK, Aliases: al}
	}
	doc.Opens = []surfaceOpens{}
	for _, o := range m.Opens {
		ext := append([]string{}, o.Extensions...)
		sort.Strings(ext)
		doc.Opens = append(doc.Opens, surfaceOpens{MediaType: o.MediaType, Extensions: ext, Default: o.IsDefault()})
	}
	sort.Slice(doc.Opens, func(i, j int) bool {
		if doc.Opens[i].MediaType != doc.Opens[j].MediaType {
			return doc.Opens[i].MediaType < doc.Opens[j].MediaType
		}
		return strings.Join(doc.Opens[i].Extensions, ",") < strings.Join(doc.Opens[j].Extensions, ",")
	})
	for _, d := range m.Contributes.Desks {
		sd := surfaceDesk{Name: d.Name, Tools: []surfaceFile{}, Schedules: append([]Schedule{}, d.Schedules...)}
		var err error
		if sd.Prompt, err = read(d.Prompt, false); err != nil {
			return "", err
		}
		if d.Code != "" {
			f, err := read(d.Code, true)
			if err != nil {
				return "", err
			}
			sd.Code = &f
		}
		for _, t := range d.Tools {
			f, err := read(t, true)
			if err != nil {
				return "", err
			}
			sd.Tools = append(sd.Tools, f)
		}
		sort.Slice(sd.Tools, func(i, j int) bool { return sd.Tools[i].Path < sd.Tools[j].Path })
		sort.Slice(sd.Schedules, func(i, j int) bool { return sd.Schedules[i].ID < sd.Schedules[j].ID })
		if sd.Accepts, err = canonicalRaw(d.Accepts, "null"); err != nil {
			return "", err
		}
		doc.Desks = append(doc.Desks, sd)
	}
	sort.Slice(doc.Desks, func(i, j int) bool { return doc.Desks[i].Name < doc.Desks[j].Name })
	for _, k := range m.Kinds {
		schema, err := canonicalRaw(k.Schema, "null")
		if err != nil {
			return "", err
		}
		reads := append([]int{}, k.ReadsSchemaVersions...)
		doc.Kinds = append(doc.Kinds, surfaceKind{
			Kind: k.Kind, Identity: k.Identity, Versioned: k.Versioned, OwnerEdits: k.OwnerEdits,
			SchemaVersion: k.SchemaVersion, ReadsSchemaVersions: reads, Schema: schema,
		})
	}
	sort.Slice(doc.Kinds, func(i, j int) bool { return doc.Kinds[i].Kind < doc.Kinds[j].Kind })
	doc.UsesKinds = append(doc.UsesKinds, m.UsesKinds...)
	sort.Strings(doc.UsesKinds)
	for _, c := range m.Capabilities {
		doc.Capabilities = append(doc.Capabilities, c.key())
	}
	sort.Strings(doc.Capabilities)
	raw, err := json.Marshal(doc)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

// canonicalRaw re-encodes a JSON value with sorted keys and exact numbers. An
// empty input yields def.
func canonicalRaw(raw json.RawMessage, def string) (json.RawMessage, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return json.RawMessage(def), nil
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, invalid("%v", err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, invalid("trailing data after the JSON value")
	}
	out, err := json.Marshal(v) // maps marshal with sorted keys
	return out, err
}
