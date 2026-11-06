// Package appdesk is the join between an app package and its app desk (SDC
// ARCH-03; design docs/design/app-package-carries-app-desk-2026-10-21.md).
//
// A package manifest embeds a desk manifest (Manifest). Install is a tape
// fact: an install event binds the package name and the digest of the desk
// manifest the owner approved. The live desk set, each package kind's one
// writer and the retired kinds are never written to a table: Derive folds
// them from the install and disable events, at boot and on every event, so a
// restore to an earlier head selects the earlier set by replay. Reconcile puts
// a derived set into the daemon's desk manifest, and Boot (required.go) runs
// at every boot: it re-seeds the image's required packages after the log's
// head and reconciles. There is no switch: a derived desk is registered.
//
// A package desk is least-authority by construction: its verbs come from the
// app set deskmanifest.NewAppDesk allows, its taint class is the untrusted
// floor, its uid is in the app range and its own, its kinds are namespaced
// choir.app.<package>.<kind>, and it has no egress until NET-10 adds owner
// grants. The package cannot ask for more; no field clears the floor.
//
// What this package does not do yet: read events from the tape (the package
// manifest and its event kind are S7-0's, on a paused branch), overlay the
// object-graph write gate with the derived kinds (needs ARCH-04 landed), or
// carry grants (NET-10).
package appdesk

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// Manifest is the desk member of an app package manifest: the desk the
// package carries.
type Manifest struct {
	// Package is the package name; the desk is app:<package> and its writer
	// desk:<package>.
	Package string `json:"package"`
	// Verbs are the choir verbs the desk's cells may call, from the app set.
	Verbs []string `json:"verbs"`
	// Taint may only be empty or untrusted_input: a package desk's floor.
	Taint deskmanifest.Taint `json:"taint_class,omitempty"`
	// WorkerMaxBytes is the desk worker's memory.max, at most the app cap.
	WorkerMaxBytes uint64 `json:"worker_max_bytes"`
	// OwnedKinds are the object kinds the desk alone writes, each
	// choir.app.<package>.<kind>.
	OwnedKinds []string `json:"owned_object_kinds"`
	// Egress is the network the desk asks for. Refused while non-empty: owner
	// grants are NET-10's.
	Egress []string `json:"egress_verbs,omitempty"`
	// LeaseKinds and LeaseCaptures are the desk's lease ceiling (MA3.4): the
	// most it may ever request of a custodian. The custodian still decides
	// each lease inside it.
	LeaseKinds    []string `json:"lease_kinds,omitempty"`
	LeaseCaptures bool     `json:"lease_captures,omitempty"`
	// LeaseGrants is the grant part of the ceiling (MA4.7): the first-party
	// typed grants, such as mail.read, the desk may ask their custodian for.
	LeaseGrants []string `json:"lease_grants,omitempty"`
}

// Code is a typed refusal.
type Code string

const (
	CodeReservedName      Code = "reserved_name"
	CodeBadName           Code = "bad_name"
	CodeKindNotNamespaced Code = "kind_not_namespaced"
	CodeVerbNotApp        Code = "verb_not_app"
	CodeTaintBelowFloor   Code = "taint_below_floor"
	CodeMemoryOverCap     Code = "memory_over_cap"
	CodeEgressNotGranted  Code = "egress_not_granted"
	CodeDigestMismatch    Code = "digest_mismatch"
	CodeNameTaken         Code = "name_taken"
	CodeNameRetired       Code = "name_retired"
	CodeKindTaken         Code = "kind_taken"
	CodeNotInstalled      Code = "not_installed"
	CodeBadSequence       Code = "bad_sequence"
	CodeBadAction         Code = "bad_action"
	CodeCapacity          Code = "capacity"
	CodeDesk              Code = "desk_refused"
	// CodeReservedSuffix refuses an installed package named like a candidate;
	// CodeNotCandidate a candidate manifest that is not one; CodeNoCandidate an
	// event that needs a live candidate and finds none; CodeCandidateLive a
	// second candidate for a package that has one.
	CodeReservedSuffix Code = "reserved_suffix"
	CodeNotCandidate   Code = "not_candidate"
	CodeNoCandidate    Code = "no_candidate"
	CodeCandidateLive  Code = "candidate_live"
	// CodeTooLongForCandidate refuses an installed name of 23 to 32 characters:
	// its candidate's desk name would not fit.
	CodeTooLongForCandidate Code = "too_long_for_candidate"
	// CodeRequired refuses a disable of a package the running image requires.
	CodeRequired Code = "required_package"
)

// Refusal is one reason an install or event did not take effect.
type Refusal struct {
	Seq     uint64
	Package string
	Code    Code
	Detail  string
}

// Action is an event's kind.
type Action string

const (
	ActionInstall Action = "install"
	ActionDisable Action = "disable"
	// ActionCandidate installs the package's candidate: the manifest's package
	// is <Package>-candidate. ActionSupplant is acceptance: the installed desk
	// is updated to the candidate's manifest under the installed names and the
	// candidate and its shadow are dropped. ActionDrop drops the candidate
	// without supplanting (rejection, rollback). Event.Package is always the
	// installed package. The candidate has no inbound routing, no schedule
	// and no egress, and writes only its shadow kinds (plan MA4.3, B3).
	ActionCandidate Action = "candidate"
	ActionSupplant  Action = "supplant"
	ActionDrop      Action = "drop_candidate"
	// ActionUpdate replaces an installed package's desk manifest with a later
	// one the owner approved. The event's Seq is the desk's new epoch.
	ActionUpdate Action = "update"
)

// Event is one install or disable fact from the computer's tape.
type Event struct {
	Seq            uint64
	Action         Action
	Package        string
	ManifestDigest string
	Manifest       *Manifest
	// Version is the package version an install, update or candidate event
	// binds (MAJOR.MINOR.PATCH); the required set compares against it.
	Version string
}

// App desk uid range: 62000 and up (deskmanifest.NewAppDesk), one per
// package in install order.
const (
	uidBase = 62000
	uidMax  = 62999
	// memoryCap is deskmanifest.NewAppDesk's app desk cap.
	memoryCap = 512 << 20
)

var (
	packageName = regexp.MustCompile(`^[a-z][a-z0-9-]{1,31}$`) // installed names are further limited to 22 by objectgraph.ValidPackageName
	kindSuffix  = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
)

// reservedNames are the S7-0 package manifest's reserved names (every static
// app id, shell and retired names; feat/s7-0-app-packages
// internal/apppackage/manifest.go). Every desk in the manifest is reserved
// too, checked at call time.
var reservedNames = map[string]bool{
	"files": true, "browser": true, "email": true, "compute-monitor": true,
	"pulse": true, "texture": true, "autopaper": true, "podcast": true,
	"image": true, "audio": true, "video": true, "pdf": true, "epub": true,
	"slides": true, "calendar": true, "content": true, "super-console": true,
	"settings": true,
	"desktop":  true, "shell": true, "package": true, "packages": true,
	"app-packages": true, "system": true, "trace": true, "terminal": true,
	"features": true, "choir": true,
}

// appVerbs is the verb set deskmanifest.NewAppDesk accepts, asked of it with
// a probe so the two can never disagree.
func appVerb(v string) bool {
	_, err := deskmanifest.NewAppDesk(deskmanifest.AppDeskSpec{Name: "app:probe", Verbs: []string{v},
		Taint: deskmanifest.TaintUntrustedInput, UID: uidBase, WorkerMaxBytes: 1 << 20})
	return err == nil
}

// DeskName is the desk a package carries.
func DeskName(pkg string) string { return "app:" + pkg }

// WriterFor is the object-graph writer of a package's kinds (ARCH-04's
// desk:<name> pattern).
func WriterFor(pkg string) string { return "desk:" + pkg }

// PackageOrigin reports whether a desk came from a package. It reads the
// row's kind, which only deskmanifest.NewAppDesk sets to app; there is no
// flag a package can clear.
func PackageOrigin(d deskmanifest.Desk) bool { return d.Kind == deskmanifest.KindApp }

// Validate is pure: every reason the manifest may not be installed. Install
// calls it before it appends anything.
func Validate(m Manifest) []Refusal { return validate(m, false) }

// ValidateCandidate is Validate for a candidate manifest: the package is
// <installed>-candidate, whose installed name is itself valid, and its kinds
// are the shadow kinds choir.app.<installed>-candidate.<kind>.
func ValidateCandidate(m Manifest) []Refusal { return validate(m, true) }

func validate(m Manifest, candidate bool) []Refusal {
	var out []Refusal
	refuse := func(c Code, format string, args ...any) {
		out = append(out, Refusal{Package: m.Package, Code: c, Detail: fmt.Sprintf(format, args...)})
	}
	nameOK := packageName.MatchString(m.Package)
	if !nameOK {
		refuse(CodeBadName, "package name %q must match %s", m.Package, packageName)
	}
	if candidate {
		if !objectgraph.ValidCandidateName(m.Package) {
			refuse(CodeNotCandidate, "candidate name %q must be <installed package>%s", m.Package, objectgraph.CandidateSuffix)
			nameOK = false
		}
	} else if objectgraph.IsCandidateName(m.Package) {
		refuse(CodeReservedSuffix, "package name %q ends with %s, which is reserved for candidates", m.Package, objectgraph.CandidateSuffix)
	} else if nameOK && !objectgraph.ValidPackageName(m.Package) && len(m.Package) > 22 {
		refuse(CodeTooLongForCandidate, "package name %q is %d characters; at most 22 so that %s%s fits a 32-character desk name", m.Package, len(m.Package), m.Package[:3]+"...", objectgraph.CandidateSuffix)
	}
	if _, isDesk := deskmanifest.Lookup(m.Package); reservedNames[m.Package] || isDesk {
		refuse(CodeReservedName, "package name %q is reserved", m.Package)
	}
	// A slug the registry reserves for an in-tree first-party desk (the Mail
	// desk, MA4.7) admits only the manifest the image ships for it.
	if !candidate && deskmanifest.IsReservedAppSlug(m.Package) && !firstParty(m) {
		refuse(CodeReservedName, "package name %q is reserved for the in-tree first-party package's own desk", m.Package)
	}
	prefix := "choir.app." + m.Package + "."
	seen := map[string]bool{}
	for _, k := range m.OwnedKinds {
		if !nameOK || len(k) <= len(prefix) || k[:len(prefix)] != prefix || !kindSuffix.MatchString(k[len(prefix):]) {
			refuse(CodeKindNotNamespaced, "kind %q is not %s<kind>", k, prefix)
			continue
		}
		if seen[k] {
			refuse(CodeKindNotNamespaced, "kind %q is listed twice", k)
		}
		seen[k] = true
	}
	for _, v := range m.Verbs {
		if !appVerb(v) {
			refuse(CodeVerbNotApp, "verb %q is not available to app desks", v)
		}
	}
	if m.Taint != "" && m.Taint != deskmanifest.TaintUntrustedInput {
		refuse(CodeTaintBelowFloor, "taint class %q is weaker than the package floor %s", m.Taint, deskmanifest.TaintUntrustedInput)
	}
	if m.WorkerMaxBytes == 0 || m.WorkerMaxBytes > memoryCap {
		refuse(CodeMemoryOverCap, "worker memory %d must be set and at most %d", m.WorkerMaxBytes, memoryCap)
	}
	if len(m.Egress) > 0 {
		refuse(CodeEgressNotGranted, "egress %v has no owner grant path yet (NET-10)", m.Egress)
	}
	return out
}

// Digest is the sha256 of the manifest's JSON: what the install event binds
// and what the daemon checks before deriving the desk.
func Digest(m Manifest) string {
	raw, err := json.Marshal(m)
	if err != nil {
		// A struct of strings and integers always marshals.
		panic("appdesk: marshal manifest: " + err.Error())
	}
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// Set is the derived desk set.
type Set struct {
	// Desks are the live package desks, sorted by name.
	Desks []deskmanifest.Desk
	// kinds maps a live package kind to its writer; retired a disabled one.
	kinds   map[string]string
	retired map[string]string
	// Epochs maps each live desk name to the Seq of the install or update
	// event that produced its row: the desk epoch (deskmanifest epoch.go).
	Epochs map[string]uint64
	// Candidates maps an installed package to its live candidate package
	// (<pkg>-candidate); the candidate's desk is in Desks like any other.
	Candidates map[string]string
	// Versions maps each live installed package to the version of its last
	// install or update event.
	Versions map[string]string
}

// Writer reports a package kind's writer and whether it may still write
// (false after its package is disabled). ok is false for a kind no package
// declared.
func (s Set) Writer(kind string) (writer string, live, ok bool) {
	if w, found := s.kinds[kind]; found {
		return w, true, true
	}
	if w, found := s.retired[kind]; found {
		return w, false, true
	}
	return "", false, false
}

// Derive folds install and disable events, in seq order, into the desk set.
// It is the only way a package desk comes to exist. An event that cannot take
// effect is skipped with a refusal; the earlier claim always wins.
func Derive(events []Event) (Set, []Refusal) { return DeriveRequired(events, nil) }

// DeriveRequired is Derive with the running image's required set: the desired
// state is the fold of the events union that set (plan MA4.4, B4). A required
// package cannot be disabled, so a disable event for one is refused and the
// package stays installed; what the fold lacks of the set is Desired's to add.
func DeriveRequired(events []Event, required []Required) (Set, []Refusal) {
	isRequired := map[string]bool{}
	for _, r := range required {
		isRequired[r.Package] = true
	}
	ordered := append([]Event(nil), events...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Seq < ordered[j].Seq })

	var refusals []Refusal
	live := map[string]deskmanifest.Desk{}
	retiredPkgs := map[string]bool{}
	set := Set{kinds: map[string]string{}, retired: map[string]string{}, Epochs: map[string]uint64{}, Candidates: map[string]string{}, Versions: map[string]string{}}
	candManifests := map[string]Manifest{}
	uids := map[string]uint32{}
	nextUID := uint32(uidBase)
	for i, ev := range ordered {
		refuse := func(c Code, format string, args ...any) {
			refusals = append(refusals, Refusal{Seq: ev.Seq, Package: ev.Package, Code: c, Detail: fmt.Sprintf(format, args...)})
		}
		if i > 0 && ordered[i-1].Seq == ev.Seq {
			refuse(CodeBadSequence, "seq %d appears twice", ev.Seq)
			continue
		}
		// dropCandidate retires the package's candidate desk and its shadow
		// kinds; it reports whether there was one.
		dropCandidate := func(base string) bool {
			cand, has := set.Candidates[base]
			if !has {
				return false
			}
			if d, ok := live[cand]; ok {
				for _, k := range d.OwnedKinds {
					delete(set.kinds, k)
					set.retired[k] = WriterFor(cand)
				}
				delete(set.Epochs, d.Name)
			}
			delete(live, cand)
			delete(set.Candidates, base)
			delete(candManifests, base)
			return true
		}
		// applyUpdate replaces an installed package's desk with the manifest m
		// (already bound to its event). It reports whether the update took.
		applyUpdate := func(m Manifest) bool {
			prev, ok := live[m.Package]
			if !ok {
				refuse(CodeNotInstalled, "package %s is not installed", m.Package)
				return false
			}
			if rs := Validate(m); len(rs) > 0 {
				for _, r := range rs {
					r.Seq = ev.Seq
					refusals = append(refusals, r)
				}
				return false
			}
			conflict := false
			for _, k := range m.OwnedKinds {
				if w, _, claimed := set.Writer(k); claimed && w != WriterFor(m.Package) {
					refuse(CodeKindTaken, "kind %s already has a writer", k)
					conflict = true
				}
			}
			if conflict {
				return false
			}
			d, err := deskmanifest.NewAppDesk(deskmanifest.AppDeskSpec{Name: DeskName(m.Package), Verbs: m.Verbs,
				Taint: deskmanifest.TaintUntrustedInput, UID: uids[m.Package], WorkerMaxBytes: m.WorkerMaxBytes, OwnedKinds: m.OwnedKinds,
				LeaseKinds: m.LeaseKinds, LeaseCaptures: m.LeaseCaptures, LeaseGrants: m.LeaseGrants, FirstParty: firstParty(m)})
			if err != nil {
				refuse(CodeDesk, "%v", err)
				return false
			}
			// A kind the update drops keeps its data and loses its writer.
			for _, k := range prev.OwnedKinds {
				delete(set.kinds, k)
				set.retired[k] = WriterFor(m.Package)
			}
			for _, k := range m.OwnedKinds {
				delete(set.retired, k)
				set.kinds[k] = WriterFor(m.Package)
			}
			live[m.Package] = d
			set.Epochs[d.Name] = ev.Seq
			set.Versions[m.Package] = ev.Version
			return true
		}
		switch ev.Action {
		case ActionInstall:
			if ev.Manifest == nil || ev.Manifest.Package != ev.Package || Digest(*ev.Manifest) != ev.ManifestDigest {
				refuse(CodeDigestMismatch, "the desk manifest is not the one the install event binds")
				continue
			}
			m := *ev.Manifest
			if rs := Validate(m); len(rs) > 0 {
				for _, r := range rs {
					r.Seq = ev.Seq
					refusals = append(refusals, r)
				}
				continue
			}
			if _, taken := live[m.Package]; taken {
				refuse(CodeNameTaken, "package %s is installed", m.Package)
				continue
			}
			if retiredPkgs[m.Package] {
				refuse(CodeNameRetired, "package %s was disabled; reinstall needs re-approval", m.Package)
				continue
			}
			conflict := false
			for _, k := range m.OwnedKinds {
				if _, _, claimed := set.Writer(k); claimed {
					refuse(CodeKindTaken, "kind %s already has a writer", k)
					conflict = true
				}
			}
			if conflict {
				continue
			}
			if nextUID > uidMax {
				refuse(CodeCapacity, "no app desk uid left")
				continue
			}
			d, err := deskmanifest.NewAppDesk(deskmanifest.AppDeskSpec{Name: DeskName(m.Package), Verbs: m.Verbs,
				Taint: deskmanifest.TaintUntrustedInput, UID: nextUID, WorkerMaxBytes: m.WorkerMaxBytes, OwnedKinds: m.OwnedKinds,
				LeaseKinds: m.LeaseKinds, LeaseCaptures: m.LeaseCaptures, LeaseGrants: m.LeaseGrants, FirstParty: firstParty(m)})
			if err != nil {
				refuse(CodeDesk, "%v", err)
				continue
			}
			uids[m.Package] = nextUID
			nextUID++
			live[m.Package] = d
			set.Epochs[d.Name] = ev.Seq
			set.Versions[m.Package] = ev.Version
			for _, k := range m.OwnedKinds {
				set.kinds[k] = WriterFor(m.Package)
			}
		case ActionUpdate:
			if ev.Manifest == nil || ev.Manifest.Package != ev.Package || Digest(*ev.Manifest) != ev.ManifestDigest {
				refuse(CodeDigestMismatch, "the desk manifest is not the one the update event binds")
				continue
			}
			applyUpdate(*ev.Manifest)
		case ActionCandidate:
			base := ev.Package
			cand := objectgraph.CandidateName(base)
			if _, ok := live[base]; !ok {
				refuse(CodeNotInstalled, "package %s is not installed; a candidate shadows an installed package", base)
				continue
			}
			if _, has := set.Candidates[base]; has {
				refuse(CodeCandidateLive, "package %s already has a candidate", base)
				continue
			}
			if ev.Manifest == nil || ev.Manifest.Package != cand || Digest(*ev.Manifest) != ev.ManifestDigest {
				refuse(CodeDigestMismatch, "the candidate manifest is not %s or not the one the event binds", cand)
				continue
			}
			m := *ev.Manifest
			if rs := ValidateCandidate(m); len(rs) > 0 {
				for _, r := range rs {
					r.Seq = ev.Seq
					refusals = append(refusals, r)
				}
				continue
			}
			// A shadow kind an earlier, dropped candidate of this package held
			// is free again; any other claim stands.
			conflict := false
			for _, k := range m.OwnedKinds {
				if w, liveKind, claimed := set.Writer(k); claimed && (liveKind || w != WriterFor(cand)) {
					refuse(CodeKindTaken, "kind %s already has a writer", k)
					conflict = true
				}
			}
			if conflict {
				continue
			}
			uid, known := uids[cand]
			if !known {
				if nextUID > uidMax {
					refuse(CodeCapacity, "no app desk uid left")
					continue
				}
				uid = nextUID
			}
			d, err := deskmanifest.NewAppDesk(deskmanifest.AppDeskSpec{Name: DeskName(cand), Verbs: m.Verbs,
				Taint: deskmanifest.TaintUntrustedInput, UID: uid, WorkerMaxBytes: m.WorkerMaxBytes, OwnedKinds: m.OwnedKinds,
				LeaseKinds: m.LeaseKinds, LeaseCaptures: m.LeaseCaptures, LeaseGrants: m.LeaseGrants})
			if err != nil {
				refuse(CodeDesk, "%v", err)
				continue
			}
			if !known {
				uids[cand] = uid
				nextUID++
			}
			live[cand] = d
			set.Epochs[d.Name] = ev.Seq
			set.Candidates[base] = cand
			candManifests[base] = m
			for _, k := range m.OwnedKinds {
				delete(set.retired, k)
				set.kinds[k] = WriterFor(cand)
			}
		case ActionDrop:
			if !dropCandidate(ev.Package) {
				refuse(CodeNoCandidate, "package %s has no candidate", ev.Package)
			}
		case ActionSupplant:
			base := ev.Package
			cm, has := candManifests[base]
			if !has {
				refuse(CodeNoCandidate, "package %s has no candidate to accept", base)
				continue
			}
			// The accepted candidate becomes the installed desk's manifest under
			// the installed names: the shadow kinds are renamed to the real ones
			// and the package name drops the suffix. The same validation as any
			// update then applies, so acceptance cannot admit what an update
			// could not.
			cand := objectgraph.CandidateName(base)
			accepted := cm
			accepted.Package = base
			accepted.OwnedKinds = make([]string, len(cm.OwnedKinds))
			for i, k := range cm.OwnedKinds {
				accepted.OwnedKinds[i] = objectgraph.PackageKindNamespace(base) + strings.TrimPrefix(k, objectgraph.PackageKindNamespace(cand))
			}
			// A refused acceptance leaves the candidate as it was.
			if applyUpdate(accepted) {
				dropCandidate(base)
			}
		case ActionDisable:
			d, ok := live[ev.Package]
			if !ok {
				refuse(CodeNotInstalled, "package %s is not installed", ev.Package)
				continue
			}
			if isRequired[ev.Package] {
				refuse(CodeRequired, "package %s is required by the running image and cannot be disabled or uninstalled", ev.Package)
				continue
			}
			dropCandidate(ev.Package)
			delete(live, ev.Package)
			delete(set.Versions, ev.Package)
			delete(set.Epochs, d.Name)
			retiredPkgs[ev.Package] = true
			for _, k := range d.OwnedKinds {
				delete(set.kinds, k)
				set.retired[k] = WriterFor(ev.Package)
			}
		default:
			refuse(CodeBadAction, "unknown action %q", ev.Action)
		}
	}
	for _, d := range live {
		set.Desks = append(set.Desks, d)
	}
	sort.Slice(set.Desks, func(i, j int) bool { return set.Desks[i].Name < set.Desks[j].Name })
	return set, refusals
}

// Reconcile makes the daemon's desk manifest hold exactly the derived set's
// app desks (plan MA4.2, B2 and D3). A desk the registry lacks is registered
// at the epoch of its install event. A desk whose derived epoch is later than
// the live one is updated: the old row stays retained for the activations
// that started under it and is dropped with the last of them, so an update
// that narrows a verb set never cuts a run in flight. An app desk the set no
// longer has is retired. A live desk whose epoch is ahead of the tape's means
// the registry holds a fact the tape does not; that is an error and the row is
// left as it is. Reconcile is idempotent: a second call with the same set
// changes nothing, which is what lets the boot call it after a crash that fell
// between an event and its reconcile.
func Reconcile(set Set) error {
	var errs []error
	want := map[string]bool{}
	for _, d := range set.Desks {
		want[d.Name] = true
		epoch := set.Epochs[d.Name]
		if epoch == 0 {
			errs = append(errs, fmt.Errorf("app desk %s: the derived set carries no epoch", d.Name))
			continue
		}
		have, ok := deskmanifest.Lookup(d.Name)
		switch live := deskmanifest.CurrentEpoch(d.Name); {
		case !ok:
			if err := deskmanifest.RegisterAppAt(d, epoch); err != nil {
				errs = append(errs, err)
			}
		case live == epoch && have.Equal(d):
		case live == epoch:
			errs = append(errs, fmt.Errorf("app desk %s: registered row differs from the derived one at epoch %d", d.Name, epoch))
		case live > epoch:
			errs = append(errs, fmt.Errorf("app desk %s: the registry is at epoch %d, ahead of the tape's %d", d.Name, live, epoch))
		default:
			if err := deskmanifest.UpdateApp(d, epoch); err != nil {
				errs = append(errs, err)
			}
		}
	}
	for _, name := range deskmanifest.Names() {
		if d, ok := deskmanifest.Lookup(name); ok && d.Kind == deskmanifest.KindApp && !want[name] {
			if err := deskmanifest.RetireApp(name); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}
