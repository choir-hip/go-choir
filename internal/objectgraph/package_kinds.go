package objectgraph

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// The registry seam for app packages (doctrine I27; P2a of
// docs/design/app-packages-replan-2026-10-24.md).
//
// DefaultRegistry compiles the platform's own kinds. An app package declares
// its data kinds in its manifest, and they reach the write gate through exactly
// one exported function: RegisterPackageKind. The seam is the whole of the
// authority a package has over the registry, so it checks everything a wrong
// package could use to take a kind it does not own:
//
//   - the kind is choir.app.<pkg>.<kind>, in the package's own namespace;
//   - the writer is desk:<pkg>, the package's own desk, and no other (not a
//     built-in desk, not system:runtime which every desk may write through, not
//     the owner: the owner is admitted by OwnerEdits, never named as a writer);
//   - the package is not named like a built-in desk, since desk:texture is
//     Texture whoever declares it;
//   - the kind is not one DefaultRegistry already holds;
//   - a second registration of the same kind must be identical, so a boot that
//     disagrees with an earlier one is an error and never a silent edit.
//
// Registration is append-only for the life of the process; nothing
// deregisters. A binary without the seam replays package objects as inert rows
// (the projector does no registry lookup) and refuses only new writes of them,
// which is why a rollback leaves nothing poisoned.
//
// Nothing calls the seam until P2b's boot registration, and that registers no
// kind until a first-party manifest declares one. The per-computer switch that
// makes package kinds writable is P2b's route (the app_packages.objects
// computer setting); this file holds the checks it relies on.

const packageKindNamespace = "choir.app."

var (
	// ErrPackageKindRefused refuses a package kind registration that breaks one
	// of the seam's rules. The wrapped message names the rule.
	ErrPackageKindRefused = errors.New("objectgraph: package kind refused")
	// ErrPackageKindConflict refuses a re-registration of a package kind whose
	// shape differs from the one already registered.
	ErrPackageKindConflict = errors.New("objectgraph: package kind is registered with a different shape")
)

var (
	// A package name is the app desk's name too (desk:<name>), so the grammar is
	// the intersection of the manifest's name and appDeskWriter: 2 to 32
	// characters, no underscore.
	packageNameGrammar = regexp.MustCompile(`^[a-z][a-z0-9-]{1,31}$`)
	packageKindGrammar = regexp.MustCompile(`^[a-z][a-z0-9_]{0,47}$`)
	packageSchemaRef   = regexp.MustCompile(`^pkg:([a-z][a-z0-9-]{1,31})@([1-9][0-9]{0,8})$`)
)

// ValidPackageName reports whether name may be a package's name for kinds and
// its desk: the grammar, and not the name of a built-in desk.
func ValidPackageName(name string) bool {
	return validPackageName(name) == nil
}

func validPackageName(name string) error {
	if !packageNameGrammar.MatchString(name) {
		return fmt.Errorf("%w: package name %q must match %s", ErrPackageKindRefused, name, packageNameGrammar)
	}
	if BuiltinWriters[Writer("desk:"+name)] {
		return fmt.Errorf("%w: package name %q is a built-in desk; desk:%s would be that desk", ErrPackageKindRefused, name, name)
	}
	return nil
}

// PackageKindNamespace is the prefix every kind of the package carries.
func PackageKindNamespace(pkg string) string { return packageKindNamespace + pkg + "." }

// ValidPackageKindName checks that kind is choir.app.<pkg>.<name> with a name
// matching the kind grammar. It does not check the package name.
func ValidPackageKindName(pkg string, kind ObjectKind) error {
	prefix := PackageKindNamespace(pkg)
	name := strings.TrimPrefix(string(kind), prefix)
	if !strings.HasPrefix(string(kind), prefix) || !packageKindGrammar.MatchString(name) {
		return fmt.Errorf("%w: kind %q is not %s<kind> with a kind name matching %s", ErrPackageKindRefused, kind, prefix, packageKindGrammar)
	}
	return nil
}

// PackageDeskWriter is the one writer of a package's kinds.
func PackageDeskWriter(pkg string) Writer { return Writer("desk:" + pkg) }

// packageKindTable records what the seam registered. The registry holds the
// same registrations for the gate; the table is how the Service and the object
// route find them, and how a caller lists a package's kinds without a copy of
// the registry.
type packageKindTable struct {
	mu    sync.RWMutex
	kinds map[ObjectKind]KindRegistration
}

func newPackageKindTable() *packageKindTable {
	return &packageKindTable{kinds: map[ObjectKind]KindRegistration{}}
}

func (t *packageKindTable) lookup(kind ObjectKind) (KindRegistration, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	reg, ok := t.kinds[kind]
	return reg, ok
}

func (t *packageKindTable) all() []KindRegistration {
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := make([]KindRegistration, 0, len(t.kinds))
	for _, reg := range t.kinds {
		out = append(out, reg)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Kind < out[j].Kind })
	return out
}

func (t *packageKindTable) ofPackage(pkg string) []KindRegistration {
	prefix := PackageKindNamespace(pkg)
	var out []KindRegistration
	for _, reg := range t.all() {
		if strings.HasPrefix(string(reg.Kind), prefix) {
			out = append(out, reg)
		}
	}
	return out
}

// packageKinds is the process's table of registered package kinds.
var packageKinds = newPackageKindTable()

// RegisterPackageKind registers one data kind of the app package pkg with the
// write gate. See the file comment for what it refuses. The caller supplies the
// kind's shape (identity, versioned, owner edits, a schema reference of the form
// pkg:<pkg>@<version>) and the writer desk:<pkg>; Store, Owner and the identity
// default are filled in. An identical re-registration is a no-op.
func RegisterPackageKind(pkg string, k KindRegistration) error {
	return registerPackageKindOn(writeRegistry, packageKinds, DefaultRegistry(), pkg, k)
}

// LookupPackageKind returns the registration of a package kind the seam
// registered. A built-in kind is not found here.
func LookupPackageKind(kind ObjectKind) (KindRegistration, bool) {
	return packageKinds.lookup(kind)
}

// PackageKindsOf lists the kinds the seam registered for pkg, sorted by kind.
// The slice is a copy.
func PackageKindsOf(pkg string) []KindRegistration {
	return packageKinds.ofPackage(pkg)
}

// lookupKind resolves a kind for a Service: its own registry first, then the
// package kinds the seam registered. A Service built without a registry holds
// its own DefaultRegistry, which never contains package kinds.
func (s *Service) lookupKind(kind ObjectKind) (KindRegistration, error) {
	reg, err := s.registry.LookupKind(kind)
	if err == nil {
		return reg, nil
	}
	if pk, ok := packageKinds.lookup(kind); ok {
		return pk, nil
	}
	return KindRegistration{}, err
}

func registerPackageKindOn(r *Registry, tbl *packageKindTable, builtin *Registry, pkg string, k KindRegistration) error {
	if err := validPackageName(pkg); err != nil {
		return err
	}
	refuse := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s", ErrPackageKindRefused, fmt.Sprintf(format, args...))
	}
	if err := ValidPackageKindName(pkg, k.Kind); err != nil {
		return err
	}
	if want := PackageDeskWriter(pkg); k.Writer != want {
		return refuse("kind %q names writer %q; the only writer of a package kind is %q", k.Kind, k.Writer, want)
	}
	if k.Reserved {
		return refuse("kind %q is marked reserved", k.Kind)
	}
	if k.Store != "" && k.Store != StoreTypeDolt {
		return refuse("kind %q names store %q; package kinds are durable (dolt)", k.Kind, k.Store)
	}
	if k.Owner != "" && k.Owner != "package:"+pkg {
		return refuse("kind %q claims owner %q", k.Kind, k.Owner)
	}
	switch k.IdentityMode {
	case "", IdentityContentHash, IdentityExternalKey:
	default:
		return refuse("kind %q names identity mode %q", k.Kind, k.IdentityMode)
	}
	if m := packageSchemaRef.FindStringSubmatch(k.Schema); m == nil || m[1] != pkg {
		return refuse("kind %q schema %q must be pkg:%s@<version>", k.Kind, k.Schema, pkg)
	}
	if builtin != nil {
		if _, err := builtin.LookupKind(k.Kind); err == nil {
			return refuse("kind %q is a built-in kind", k.Kind)
		}
	}
	k.Store = StoreTypeDolt
	k.Owner = "package:" + pkg
	if k.IdentityMode == "" {
		k.IdentityMode = IdentityContentHash
	}

	tbl.mu.Lock()
	defer tbl.mu.Unlock()
	if existing, ok := tbl.kinds[k.Kind]; ok {
		if existing == k {
			return nil
		}
		return fmt.Errorf("%w: %q is registered as %+v, not %+v", ErrPackageKindConflict, k.Kind, existing, k)
	}
	if _, err := r.LookupKind(k.Kind); err == nil {
		return fmt.Errorf("%w: %q is already registered outside the package seam", ErrPackageKindConflict, k.Kind)
	}
	if err := r.RegisterKindChecked(k); err != nil {
		return fmt.Errorf("%w: %v", ErrPackageKindRefused, err)
	}
	tbl.kinds[k.Kind] = k
	return nil
}
