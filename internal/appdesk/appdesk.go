// Package appdesk is the join between an app package and its app desk (SDC
// ARCH-03; design docs/design/app-package-carries-app-desk-2026-10-21.md).
//
// A package manifest embeds a desk manifest (Manifest). Install is a tape
// fact: an install event binds the package name and the digest of the desk
// manifest the owner approved. The live desk set, each package kind's one
// writer and the retired kinds are never written to a table: Derive folds
// them from the install and disable events, at boot and on every event, so a
// restore to an earlier head selects the earlier set by replay. Activate puts
// a derived set into the daemon's desk manifest; it does nothing unless
// CHOIR_APP_DESKS=on.
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
	"os"
	"regexp"
	"sort"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
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
)

// Event is one install or disable fact from the computer's tape.
type Event struct {
	Seq            uint64
	Action         Action
	Package        string
	ManifestDigest string
	Manifest       *Manifest
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
	packageName = regexp.MustCompile(`^[a-z][a-z0-9-]{1,31}$`)
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
func Validate(m Manifest) []Refusal {
	var out []Refusal
	refuse := func(c Code, format string, args ...any) {
		out = append(out, Refusal{Package: m.Package, Code: c, Detail: fmt.Sprintf(format, args...)})
	}
	nameOK := packageName.MatchString(m.Package)
	if !nameOK {
		refuse(CodeBadName, "package name %q must match %s", m.Package, packageName)
	}
	if _, isDesk := deskmanifest.Lookup(m.Package); reservedNames[m.Package] || isDesk {
		refuse(CodeReservedName, "package name %q is reserved", m.Package)
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
func Derive(events []Event) (Set, []Refusal) {
	ordered := append([]Event(nil), events...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Seq < ordered[j].Seq })

	var refusals []Refusal
	live := map[string]deskmanifest.Desk{}
	retiredPkgs := map[string]bool{}
	set := Set{kinds: map[string]string{}, retired: map[string]string{}}
	nextUID := uint32(uidBase)
	for i, ev := range ordered {
		refuse := func(c Code, format string, args ...any) {
			refusals = append(refusals, Refusal{Seq: ev.Seq, Package: ev.Package, Code: c, Detail: fmt.Sprintf(format, args...)})
		}
		if i > 0 && ordered[i-1].Seq == ev.Seq {
			refuse(CodeBadSequence, "seq %d appears twice", ev.Seq)
			continue
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
				Taint: deskmanifest.TaintUntrustedInput, UID: nextUID, WorkerMaxBytes: m.WorkerMaxBytes, OwnedKinds: m.OwnedKinds})
			if err != nil {
				refuse(CodeDesk, "%v", err)
				continue
			}
			nextUID++
			live[m.Package] = d
			for _, k := range m.OwnedKinds {
				set.kinds[k] = WriterFor(m.Package)
			}
		case ActionDisable:
			d, ok := live[ev.Package]
			if !ok {
				refuse(CodeNotInstalled, "package %s is not installed", ev.Package)
				continue
			}
			delete(live, ev.Package)
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

// Mode gates activation. Off by default: new red behavior ships off.
type Mode string

const (
	ModeOff Mode = "off"
	ModeOn  Mode = "on"
	// EnvMode is read once by whoever activates; only "on" turns it on.
	EnvMode = "CHOIR_APP_DESKS"
)

// ModeFromEnv reads EnvMode. Anything but "on" is off.
func ModeFromEnv(getenv func(string) string) Mode {
	if getenv == nil {
		getenv = os.Getenv
	}
	if getenv(EnvMode) == string(ModeOn) {
		return ModeOn
	}
	return ModeOff
}

// Activate makes the daemon's desk manifest hold exactly the derived set's
// app desks: it registers the new ones and retires the app desks the set no
// longer has. A live registration whose row differs from the derived one is
// an error and is left as it is. With mode off it does nothing.
func Activate(mode Mode, set Set) error {
	if mode != ModeOn {
		return nil
	}
	var errs []error
	want := map[string]bool{}
	for _, d := range set.Desks {
		want[d.Name] = true
		if have, ok := deskmanifest.Lookup(d.Name); ok {
			if !have.Equal(d) {
				errs = append(errs, fmt.Errorf("app desk %s: registered row differs from the derived one", d.Name))
			}
			continue
		}
		if err := deskmanifest.RegisterApp(d); err != nil {
			errs = append(errs, err)
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
