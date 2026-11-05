package objectgraph

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/kernelservice"
)

// Custody (modular architecture plan MA3.1, section 8).
//
// A kind has one custodian, and the registry does not say who: the manifests
// do. A desk's manifest names the kinds it is the only writer of
// (deskmanifest.Desk.OwnedKinds); a kernel service's registration names its
// own (kernelservice.Manifest.OwnedKinds). DefaultRegistry derives each
// built-in kind's Writer from those claims at load, and the load fails when a
// kind is claimed by two manifests or by none, or a manifest claims a kind the
// registry does not hold. An app desk's kinds are not built-in: a package
// install registers them (RegisterPackageKind), under the desk's own writer.

// ErrCustody refuses a registry load whose manifests do not give every kind
// exactly one custodian.
var ErrCustody = errors.New("objectgraph: custody")

// Claim is one manifest's declaration: who it is and the kinds it writes.
type Claim struct {
	// Custodian is the writer the manifest derives.
	Custodian Writer
	// Source names the manifest, for a refusal.
	Source string
	Kinds  []string
}

// ManifestClaims reads the registered system desks and kernel services. An
// app desk is not read: its kinds register at install.
func ManifestClaims() []Claim {
	var claims []Claim
	for _, name := range deskmanifest.Names() {
		d, ok := deskmanifest.Lookup(name)
		if !ok || d.Kind == deskmanifest.KindApp || len(d.OwnedKinds) == 0 {
			continue
		}
		claims = append(claims, Claim{Custodian: Writer("desk:" + d.Name), Source: "desk manifest " + d.Name, Kinds: d.OwnedKinds})
	}
	for _, m := range kernelservice.All() {
		claims = append(claims, Claim{Custodian: Writer("system:" + m.Name), Source: "kernel service " + m.Name, Kinds: m.OwnedKinds})
	}
	return claims
}

// DeriveCustody returns kinds with Writer set to the one custodian the claims
// give each. It reports every violation at once: a kind with no claim, a kind
// claimed twice (by two manifests or twice by one), a claim of a kind not in
// kinds, and a custodian that is not a valid writer.
func DeriveCustody(kinds []KindRegistration, claims []Claim) ([]KindRegistration, error) {
	registered := make(map[string]bool, len(kinds))
	for _, k := range kinds {
		registered[string(k.Kind)] = true
	}
	claimants := map[string][]string{}
	writers := map[string]Writer{}
	var problems []string
	for _, c := range claims {
		if !ValidKindWriter(c.Custodian) {
			problems = append(problems, fmt.Sprintf("%s derives writer %q, which is not a valid writer", c.Source, c.Custodian))
			continue
		}
		for _, k := range c.Kinds {
			if !registered[k] {
				problems = append(problems, fmt.Sprintf("%s claims %q, which the registry does not hold", c.Source, k))
				continue
			}
			claimants[k] = append(claimants[k], c.Source)
			writers[k] = c.Custodian
		}
	}
	out := make([]KindRegistration, len(kinds))
	for i, k := range kinds {
		who := claimants[string(k.Kind)]
		switch len(who) {
		case 0:
			problems = append(problems, fmt.Sprintf("unclaimed: %s", k.Kind))
		case 1:
			k.Writer = writers[string(k.Kind)]
		default:
			problems = append(problems, fmt.Sprintf("double-claimed: %s by %s", k.Kind, strings.Join(who, ", ")))
		}
		out[i] = k
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return nil, fmt.Errorf("%w: %d problems:\n  %s", ErrCustody, len(problems), strings.Join(problems, "\n  "))
	}
	return out, nil
}

// DeskWriter is the writer of a registered desk, system or app: "desk:" plus
// the name the manifest holds. The reducer resolves the writer of a run's
// desk through it (MA3.2), so no caller spells a desk's writer by hand.
//
// An app desk is registered as "app:<slug>", and a package's kinds name
// desk:<slug> as their writer (RegisterPackageKind), so DeskWriter("app:notes")
// is "desk:notes", the writer of choir.app.notes.*. A bare slug that is no
// system desk is read as the app desk of that name.
func DeskWriter(desk string) (Writer, bool) {
	desk = strings.TrimSpace(desk)
	d, ok := deskmanifest.Lookup(desk)
	if !ok {
		d, ok = deskmanifest.Lookup("app:" + desk)
	}
	if !ok {
		return "", false
	}
	return Writer("desk:" + strings.TrimPrefix(d.Name, "app:")), true
}

// CustodianOf is the writer the registry holds for kind: the desk or service
// whose manifest claims it. A host loop that writes a custodian's kind on its
// behalf (the intake loops writing management's labels and events, the
// support job writing Texture's verdicts) declares this writer, so the loop
// names no desk. Such a declaration is true by construction and so is never a
// crossing; the crossings the counters find are the declarations made from a
// run record, which differ from the kind's custodian.
func CustodianOf(kind ObjectKind) (Writer, bool) {
	reg, err := writeRegistry.LookupKind(kind)
	if err != nil || reg.Writer == "" {
		return "", false
	}
	return reg.Writer, true
}

// FailOnUnknownCustodian panics when kind has no custodian and undeclared
// tracking is on (a test process), so a declaration naming a kind the registry
// does not hold cannot pass silently. Production does nothing.
func FailOnUnknownCustodian(kind ObjectKind) {
	if trackUndeclared.Load() {
		panic(fmt.Sprintf("objectgraph: declaration names kind %q, which has no custodian in the registry", kind))
	}
}
