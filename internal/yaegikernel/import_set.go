package yaegikernel

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/gomodule"
)

// A desk's code-mode import set (plan MA6.6, ruling R26;
// docs/design/ma66-code-mode-import-sets-2026-11-06.md). It is derived by the
// host from the desk's manifest row, never authored:
//
//   - Packages: the pure base every desk gets, the precommitment-records
//     protocol package (choir) and the desk standard-library set
//     (gomodule.Stdlib). Nothing in it reaches the OS: no file, process,
//     socket, environment or interpreter internals (gomodule's list and its
//     banned trees). No grant adds a package today; every effect a grant
//     carries reaches a cell as a choir verb.
//   - Choir: the choir symbols the desk's cells may call, the observation
//     tier every cell has plus the verbs the desk's grants produce
//     (deskmanifest resolve). This is the effectful part of the set, and the
//     part that differs between desks.
//   - GoImports: a package desk's verified Go module imports (MA6.5), carried
//     beside the base by the module bundle, not by the package list.
//
// The worker re-derives the set from its own row and refuses a carried
// package list that leaves it (DeskAllowlist), so the daemon's list can only
// narrow the set, never widen it.

// choirPackage is the import path of the prebound precommitment-records
// protocol package.
const choirPackage = "choir"

// ImportSet is one desk's derived import set.
type ImportSet struct {
	Desk      string   `json:"desk"`
	Packages  []string `json:"packages"`
	Choir     []string `json:"choir"`
	GoImports []string `json:"go_imports,omitempty"`
}

// basePackages is every desk's package set: choir and the desk stdlib set,
// sorted.
func basePackages() []string {
	out := append([]string{choirPackage}, gomodule.Stdlib()...)
	sort.Strings(out)
	return out
}

// DeskImportSet derives the import set of the desk (or run role) named role.
// A role the manifest does not name gets the base packages and the
// observation tier only, as its choir scope does (NewChoirScope).
func DeskImportSet(role string, goImports []string) ImportSet {
	desk := normalizeDeskRole(role)
	scope := &ChoirScope{desk: desk, readOnly: scopeReadOnly(role, desk)}
	exports := scope.ChoirExports()[choirPackage+"/"+choirPackage]
	choir := make([]string, 0, len(exports))
	for name := range exports {
		choir = append(choir, name)
	}
	sort.Strings(choir)
	set := ImportSet{Desk: role, Packages: basePackages(), Choir: choir}
	if len(goImports) > 0 {
		set.GoImports = append([]string{}, goImports...)
		sort.Strings(set.GoImports)
	}
	return set
}

// Digest is the sha256 (lowercase hex) of the set's canonical JSON. Equal
// rows on one platform version give equal digests; the order grants, verbs
// or imports were written in does not matter.
func (s ImportSet) Digest() string {
	raw, _ := json.Marshal(s)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// DeskAllowedPackages is the package list the daemon carries to a desk's
// worker: the package part of DeskImportSet(role), without binding the choir
// surface. No grant adds a package (design section 2), so it is the base for
// every role; the role stays in the signature because the set is the desk's.
func DeskAllowedPackages(role string) []string {
	_ = role
	return basePackages()
}

// DeskAllowlist is the worker's import allowlist: the carried package list,
// which must lie inside the desk's derived set, or the derived set itself
// when nothing was carried. A carried package outside the set (a package
// gomodule does not name, or one the desk's row does not give) refuses the
// worker: the static import check is only as narrow as this list
// (buildFilteredSymbols binds a stdlib symbol table for every listed path).
func DeskAllowlist(role string, carried []string) (*Allowlist, error) {
	base := DeskAllowedPackages(role)
	if len(carried) == 0 {
		return NewAllowlist(base...), nil
	}
	in := make(map[string]bool, len(base))
	for _, p := range base {
		in[p] = true
	}
	var out []string
	for _, p := range carried {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if !in[p] {
			return nil, fmt.Errorf("import set: package %q is outside desk %q's import set", p, role)
		}
		out = append(out, p)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("import set: the carried package list for desk %q is blank", role)
	}
	return NewAllowlist(out...), nil
}
