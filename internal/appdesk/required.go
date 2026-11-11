package appdesk

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// The required set (plan MA4.4, section 6 B4).
//
// The desired state of a computer's package desks is the fold of the install
// events union the required set of the running base image. The required set is
// image configuration: it ships with the base build (required.json, embedded
// here), it is not a tape fact. At every boot, Boot compares the two and, for a
// required package the fold lacks (a restore to a head before it was
// installed) or holds at an older version than the image's, appends a new
// install or update event after the restored head, then registers the result.
// That is an install, not a resumer of work: it touches no open obligation,
// which the boot closes as "interrupted by a restart" in its own pass. A
// required package cannot be disabled or uninstalled (DeriveRequired).
//
// Each Phase 4 slice that turns a built-in desk into a package adds that
// package to required.json: the set is the management, research, engineering
// and Texture packages and the prompt router. The Mail package is not required.
// The first-party autoradio desk is (docs/design/autoradio-desk-fast-turns-2026-11-11.md):
// the app's voice turns call it on every computer.

// Required is one package the image requires: its desk manifest and the
// version the image ships.
type Required struct {
	Package  string   `json:"package"`
	Version  string   `json:"version"`
	Manifest Manifest `json:"manifest"`
}

//go:embed required.json
var requiredJSON []byte

type requiredFile struct {
	Packages []Required `json:"packages"`
}

// ImageRequired is the required set the running build ships.
func ImageRequired() ([]Required, error) { return parseRequired(requiredJSON) }

func parseRequired(raw []byte) ([]Required, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var f requiredFile
	if err := dec.Decode(&f); err != nil {
		return nil, fmt.Errorf("appdesk: required set: %w", err)
	}
	seen := map[string]bool{}
	for _, r := range f.Packages {
		if r.Manifest.Package != r.Package {
			return nil, fmt.Errorf("appdesk: required package %q carries the manifest of %q", r.Package, r.Manifest.Package)
		}
		if _, ok := parseVersion(r.Version); !ok {
			return nil, fmt.Errorf("appdesk: required package %s: version %q is not MAJOR.MINOR.PATCH", r.Package, r.Version)
		}
		if seen[r.Package] {
			return nil, fmt.Errorf("appdesk: required package %s is listed twice", r.Package)
		}
		seen[r.Package] = true
		if rs := Validate(r.Manifest); len(rs) > 0 {
			return nil, fmt.Errorf("appdesk: required package %s: %s: %s", r.Package, rs[0].Code, rs[0].Detail)
		}
	}
	sort.Slice(f.Packages, func(i, j int) bool { return f.Packages[i].Package < f.Packages[j].Package })
	return f.Packages, nil
}

func parseVersion(v string) ([3]int, bool) {
	var out [3]int
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return out, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 || (len(p) > 1 && p[0] == '0') {
			return out, false
		}
		out[i] = n
	}
	return out, true
}

// versionLess reports whether a is an older version than b.
// VersionLess reports whether a is an older version than b.
func VersionLess(a, b string) bool { return versionLess(a, b) }

func versionLess(a, b string) bool {
	av, aok := parseVersion(a)
	bv, bok := parseVersion(b)
	if !aok {
		return true // an unversioned install is older than any version
	}
	if !bok {
		return false
	}
	for i := range av {
		if av[i] != bv[i] {
			return av[i] < bv[i]
		}
	}
	return false
}

// Desired returns the events the required set adds to the fold of events: an
// install for a required package the fold lacks and an update for one held at
// an older version. The events carry no sequence; the log assigns it, so they
// come after the head the log has (a restore to an earlier head gets them
// appended after that head). It is pure and idempotent: with the result
// appended it returns nothing.
func Desired(events []Event, required []Required) []Event {
	set, _ := DeriveRequired(events, required)
	var add []Event
	for _, r := range required {
		m := r.Manifest
		ev := Event{Package: r.Package, ManifestDigest: Digest(m), Manifest: &m, Version: r.Version}
		have, live := set.Versions[r.Package]
		switch {
		case !live:
			ev.Action = ActionInstall
		case versionLess(have, r.Version):
			ev.Action = ActionUpdate
		default:
			continue
		}
		add = append(add, ev)
	}
	return add
}

// Log is the durable event log boot reads and appends to (the store's
// desk_package_events table).
type Log interface {
	Events(ctx context.Context) ([]Event, error)
	// Append stores ev and returns the sequence the log gave it.
	Append(ctx context.Context, ev Event) (uint64, error)
}

// Boot is the boot-time reconcile: it re-seeds the required set after the
// log's head, folds the log with the required set, and registers the result in
// the desk registry (Reconcile). A refusal in the fold is returned with the
// derived set so the caller can report it; the desks the fold justifies are
// registered regardless. An error is a log or registry failure.
func Boot(ctx context.Context, log Log, required []Required) (Set, []Refusal, error) {
	events, err := log.Events(ctx)
	if err != nil {
		return Set{}, nil, fmt.Errorf("appdesk boot: read the log: %w", err)
	}
	for _, ev := range Desired(events, required) {
		seq, err := log.Append(ctx, ev)
		if err != nil {
			return Set{}, nil, fmt.Errorf("appdesk boot: re-seed %s: %w", ev.Package, err)
		}
		ev.Seq = seq
		events = append(events, ev)
	}
	set, refusals := DeriveRequired(events, required)
	// A required package that the fold still lacks was refused (its install
	// conflicts with an earlier fact); say so rather than boot without it.
	var errs []error
	for _, r := range required {
		if _, live := set.Versions[r.Package]; !live {
			errs = append(errs, fmt.Errorf("required package %s is not installed after the re-seed", r.Package))
		}
	}
	if err := Reconcile(set); err != nil {
		errs = append(errs, err)
	}
	return set, refusals, errors.Join(errs...)
}

// EncodeEvent is the log's body for an event; the sequence is the log's.
func EncodeEvent(ev Event) (json.RawMessage, error) {
	return json.Marshal(struct {
		Action         Action    `json:"action"`
		Package        string    `json:"package"`
		ManifestDigest string    `json:"manifest_digest,omitempty"`
		Manifest       *Manifest `json:"manifest,omitempty"`
		Version        string    `json:"version,omitempty"`
		Reapproved     bool      `json:"reapproved,omitempty"`
	}{ev.Action, ev.Package, ev.ManifestDigest, ev.Manifest, ev.Version, ev.Reapproved})
}

// DecodeEvent reads a log body back at its sequence. Unknown fields are
// refused: the log is ours.
func DecodeEvent(seq uint64, body json.RawMessage) (Event, error) {
	var in struct {
		Action         Action    `json:"action"`
		Package        string    `json:"package"`
		ManifestDigest string    `json:"manifest_digest"`
		Manifest       *Manifest `json:"manifest"`
		Version        string    `json:"version"`
		Reapproved     bool      `json:"reapproved"`
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		return Event{}, fmt.Errorf("appdesk: decode event %d: %w", seq, err)
	}
	return Event{Seq: seq, Action: in.Action, Package: in.Package, ManifestDigest: in.ManifestDigest, Manifest: in.Manifest, Version: in.Version, Reapproved: in.Reapproved}, nil
}
