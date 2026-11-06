package gomodule

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// ErrRefused is the sentinel every refusal of this package matches.
var ErrRefused = errors.New("go module refused")

// Refusal codes: a caller maps them without parsing prose.
const (
	CodeBadRef        = "bad_module_ref"
	CodeNotCurated    = "not_curated"
	CodeSumMismatch   = "sum_mismatch"
	CodeBannedImport  = "banned_import"
	CodeOutsideSet    = "outside_import_set"
	CodeUnresolved    = "unresolved_import"
	CodeInternal      = "internal_import"
	CodeCgo           = "cgo"
	CodeDirective     = "directive"
	CodeNonGoSource   = "non_go_source"
	CodeParse         = "parse"
	CodeTooLarge      = "too_large"
	CodeBadBundle     = "bad_bundle"
	CodeStrayFile     = "stray_file"
	CodeDuplicateMod  = "duplicate_module"
	CodeCuratedFormat = "curated_list_format"
	CodeOverlap       = "overlapping_modules"
	CodeReservedName  = "reserved_name"
)

// Refusal is one typed refusal: Code is stable, Detail names the module, the
// package chain and the import.
type Refusal struct {
	Code   string
	Detail string
}

func (r *Refusal) Error() string { return "go module refused: " + r.Code + ": " + r.Detail }

// Is lets errors.Is(err, ErrRefused) match every refusal.
func (r *Refusal) Is(target error) bool { return target == ErrRefused }

func refuse(code, format string, args ...any) *Refusal {
	return &Refusal{Code: code, Detail: fmt.Sprintf(format, args...)}
}

// CodeOf is the refusal code of err, or "" when it is not a Refusal.
func CodeOf(err error) string {
	var r *Refusal
	if errors.As(err, &r) {
		return r.Code
	}
	return ""
}

// Caps (design section 7; open question 9's size cap, taken conservatively).
const (
	// MaxModules bounds the modules one package declares.
	MaxModules = 8
	// MaxImports bounds the packages one desk's cells may import.
	MaxImports = 16
	// MaxClosurePackages bounds the packages a desk's closure reaches.
	MaxClosurePackages = 64
	// MaxBundleBytes bounds a desk's bundle: the whole content of the
	// modules its closure reaches (the worker re-hashes them).
	MaxBundleBytes = 512 << 10
	// MaxModuleBytes bounds one module's whole content in a release.
	MaxModuleBytes = 2 << 20
	// MaxModuleFiles bounds one module's file count.
	MaxModuleFiles = 1024
)

// Module is one module version; Sum is its go.sum h1: hash (empty in a
// manifest reference, set on the curated list).
type Module struct {
	Path    string `json:"path"`
	Version string `json:"version"`
	Sum     string `json:"sum,omitempty"`
}

// String is path@version.
func (m Module) String() string { return m.Path + "@" + m.Version }

// Dir is the module's directory in a release (under the package's gomod/
// root) and in the module cache: escaped path @ escaped version.
func (m Module) Dir() string { return Escape(m.Path) + "@" + Escape(m.Version) }

var (
	pathElem = regexp.MustCompile(`^[A-Za-z0-9_~.-]+$`)
	semver   = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z.-]+)?(\+incompatible)?$`)
	h1Sum    = regexp.MustCompile(`^h1:[A-Za-z0-9+/]{43}=$`)
)

// CheckPath refuses a module or package path that is not a plain module
// path: slash-separated elements of letters, digits and _~.-, none empty or
// starting with a dot or dash, the first holding a dot (a domain).
func CheckPath(p string) error {
	if p == "" || len(p) > 256 || strings.HasPrefix(p, "/") || strings.HasSuffix(p, "/") {
		return fmt.Errorf("%q is not a module path", p)
	}
	for i, e := range strings.Split(p, "/") {
		if !pathElem.MatchString(e) || strings.HasPrefix(e, ".") || strings.HasPrefix(e, "-") || strings.HasSuffix(e, ".") {
			return fmt.Errorf("%q is not a module path", p)
		}
		if i == 0 && !strings.Contains(e, ".") {
			return fmt.Errorf("%q is not a module path: the first element is not a domain", p)
		}
	}
	return nil
}

// ParseRef parses a manifest's module reference, path@version.
func ParseRef(ref string) (Module, error) {
	i := strings.LastIndexByte(ref, '@')
	if i <= 0 {
		return Module{}, refuse(CodeBadRef, "%q is not path@version", ref)
	}
	m := Module{Path: ref[:i], Version: ref[i+1:]}
	if err := CheckPath(m.Path); err != nil {
		return Module{}, refuse(CodeBadRef, "%v", err)
	}
	if !semver.MatchString(m.Version) {
		return Module{}, refuse(CodeBadRef, "%q: %q is not a semantic version", ref, m.Version)
	}
	return m, nil
}

// Escape is the module cache's case escaping: an upper-case letter becomes
// '!' and its lower case, so two paths differing in case never share a
// directory on a case-insensitive filesystem.
func Escape(s string) string {
	var b strings.Builder
	for _, r := range s {
		if 'A' <= r && r <= 'Z' {
			b.WriteByte('!')
			b.WriteRune(r + ('a' - 'A'))
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// Hash1 is the Go checksum database's h1: hash of a module (dirhash.Hash1):
// the sha256 of the lines "<sha256 hex>  <prefix>/<name>\n" over the module's
// files sorted by name, base64. prefix is path@version (unescaped).
func Hash1(prefix string, files map[string][]byte) (string, error) {
	names := make([]string, 0, len(files))
	for n := range files {
		if strings.Contains(n, "\n") {
			return "", refuse(CodeBadBundle, "file name %q has a newline", n)
		}
		names = append(names, prefix+"/"+n)
	}
	sort.Strings(names)
	h := sha256.New()
	for _, full := range names {
		sum := sha256.Sum256(files[full[len(prefix)+1:]])
		fmt.Fprintf(h, "%x  %s\n", sum, full)
	}
	return "h1:" + base64.StdEncoding.EncodeToString(h.Sum(nil)), nil
}

// curatedRaw is the platform-reviewed list (R42): one module version per
// line, "path version h1:hash", the hash as go.sum and sum.golang.org record
// it. An addition is a reviewed code change, and
// scripts/check-curated-go-modules re-checks every line against GOSUMDB.
//
//go:embed curated.txt
var curatedRaw []byte

// Curated is the checked-in curated list.
func Curated() ([]Module, error) { return ParseCurated(curatedRaw) }

// ParseCurated parses a curated list. It refuses a malformed line, a bad
// path, version or hash, and a module path listed twice (one version per
// path: a version bump replaces the line).
func ParseCurated(raw []byte) ([]Module, error) {
	var out []Module
	seen := map[string]bool{}
	sc := bufio.NewScanner(bytes.NewReader(raw))
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Fields(line)
		if len(f) != 3 {
			return nil, refuse(CodeCuratedFormat, "line %d: want \"path version h1:hash\"", n)
		}
		m, err := ParseRef(f[0] + "@" + f[1])
		if err != nil {
			return nil, refuse(CodeCuratedFormat, "line %d: %v", n, err)
		}
		if !h1Sum.MatchString(f[2]) {
			return nil, refuse(CodeCuratedFormat, "line %d: %q is not an h1: hash", n, f[2])
		}
		if seen[m.Path] {
			return nil, refuse(CodeCuratedFormat, "line %d: %s is listed twice", n, m.Path)
		}
		seen[m.Path] = true
		m.Sum = f[2]
		out = append(out, m)
	}
	if err := sc.Err(); err != nil {
		return nil, refuse(CodeCuratedFormat, "%v", err)
	}
	return out, nil
}
