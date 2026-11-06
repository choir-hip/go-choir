package pkgrelease

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// Updates (slice F1, docs/design/forced-modularity-build-2026-11-04.md
// section 5). The base updates from main; an app updates from the Library,
// on the installer's own terms:
//
//   - manual (the default): a newer listed release of the same track shows
//     "Update available" and installs when the owner presses Update;
//   - automatic: the computer installs the newest release of the track that
//     has been listed for UpdateSoak, at its next load, unless the update
//     widens what the app may do (its extracted uses or its grants), which
//     always waits for the owner's press ("needs approval").
//
// A track change (another publisher) is never an update. The only thing
// that overrides the owner's choice is the platform's break-glass act: a
// yank, with or without a release to force (ForcedRelease on the listing).
// This file is the pure decision; corpusd serves the facts, the guest acts.

// UpdateSoak is how long a release is listed before an automatic installer
// takes it. A constant, not a setting (design D2); it applies only to
// automatic installers and never delays a publish or a manual press.
const UpdateSoak = 24 * time.Hour

// UpdatePolicyKey is the owner preference that holds the update policy.
const UpdatePolicyKey = "packages.update_policy"

// Update policies.
const (
	UpdateManual    = "manual"
	UpdateAutomatic = "automatic"
)

// maxPolicyApps bounds the per-app overrides of one policy.
const maxPolicyApps = 256

// UpdatePolicy is the owner's update setting: a default and per-app
// overrides. The zero value (and an absent or unreadable preference) is
// manual for every app.
type UpdatePolicy struct {
	Default string            `json:"default"`
	Apps    map[string]string `json:"apps,omitempty"`
}

// ParseUpdatePolicy reads a stored or submitted policy strictly: unknown
// fields, an unknown mode or a bad package name refuse it.
func ParseUpdatePolicy(raw []byte) (UpdatePolicy, error) {
	var p UpdatePolicy
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		return UpdatePolicy{}, fmt.Errorf("update policy: %w", err)
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return UpdatePolicy{}, fmt.Errorf("update policy: one object")
	}
	return p, p.Validate()
}

// Validate checks the modes and names.
func (p UpdatePolicy) Validate() error {
	if !validMode(p.Default) {
		return fmt.Errorf("update policy: default %q is not manual or automatic", p.Default)
	}
	if len(p.Apps) > maxPolicyApps {
		return fmt.Errorf("update policy: at most %d apps", maxPolicyApps)
	}
	for name, mode := range p.Apps {
		if !objectgraph.ValidPackageName(name) {
			return fmt.Errorf("update policy: %q is not a package name", name)
		}
		if !validMode(mode) {
			return fmt.Errorf("update policy: %s: %q is not manual or automatic", name, mode)
		}
	}
	return nil
}

func validMode(mode string) bool { return mode == UpdateManual || mode == UpdateAutomatic }

// For is the mode of one app: its override, else the default, else manual.
func (p UpdatePolicy) For(pkg string) string {
	if mode, ok := p.Apps[pkg]; ok && validMode(mode) {
		return mode
	}
	if validMode(p.Default) {
		return p.Default
	}
	return UpdateManual
}

// UpdateSurface is what an update is compared on: the SDK names the module
// uses (extracted by the build, never authored), the ceiling of grants the
// manifest requests, and the behaviour-surface hash of the release.
// UsesKnown is false for an install recorded before uses were kept: its
// uses cannot be compared, so any update of it counts as widening.
type UpdateSurface struct {
	Uses             []string `json:"uses"`
	UsesKnown        bool     `json:"uses_known"`
	Grants           []string `json:"grants"`
	BehaviourSurface string   `json:"behaviour_surface,omitempty"`
}

// UpdateChange is how a candidate differs from the installed release.
type UpdateChange struct {
	// AddedUses and AddedGrants widen what the app may do; either one makes
	// the update need the owner's approval.
	AddedUses   []string `json:"added_uses"`
	AddedGrants []string `json:"added_grants"`
	// UsesUnknown: the installed release's uses were not recorded, so the
	// update is treated as widening.
	UsesUnknown bool `json:"uses_unknown,omitempty"`
	// SurfaceChanged: the behaviour-surface hash moved (any code change).
	SurfaceChanged bool `json:"surface_changed"`
}

// Widening reports whether the change needs the owner's approval.
func (c UpdateChange) Widening() bool {
	return c.UsesUnknown || len(c.AddedUses) > 0 || len(c.AddedGrants) > 0
}

// CompareSurface is next against cur.
func CompareSurface(cur, next UpdateSurface) UpdateChange {
	return UpdateChange{
		AddedUses:      added(cur.Uses, next.Uses),
		AddedGrants:    added(cur.Grants, next.Grants),
		UsesUnknown:    !cur.UsesKnown,
		SurfaceChanged: cur.BehaviourSurface == "" || cur.BehaviourSurface != next.BehaviourSurface,
	}
}

func added(cur, next []string) []string {
	have := map[string]bool{}
	for _, v := range cur {
		have[v] = true
	}
	out := []string{}
	for _, v := range next {
		if !have[v] {
			have[v] = true
			out = append(out, v)
		}
	}
	sort.Strings(out)
	return out
}

// TrackRelease is one release of a track as the update decision reads it:
// its listing's current version and when that version was written.
type TrackRelease struct {
	ReleaseID     string    `json:"release_id"`
	Package       string    `json:"package"`
	Track         string    `json:"track"`
	Seq           int       `json:"seq"`
	State         string    `json:"state"`
	StateReason   string    `json:"state_reason,omitempty"`
	StateActor    string    `json:"state_actor,omitempty"`
	ForcedRelease string    `json:"forced_release,omitempty"`
	Version       int       `json:"version"`
	VersionDigest string    `json:"version_digest,omitempty"`
	ListedAt      time.Time `json:"listed_at"`
}

// Soaked reports whether the release has been listed for UpdateSoak at now.
func (t TrackRelease) Soaked(now time.Time) bool {
	return t.State == StateListed && !t.ListedAt.IsZero() && !now.Before(t.ListedAt.Add(UpdateSoak))
}

// PlatformAct is the platform's break-glass act on an installed release, if
// any: a yank, and a forced target when the yank names one.
func (t TrackRelease) PlatformAct() (yanked bool, forced string) {
	if t.State != StateYanked {
		return false, ""
	}
	return true, t.ForcedRelease
}

// NewerReleases are the listed releases of the installed release's track
// above its seq, newest first. Another track's releases never count.
func NewerReleases(installed TrackRelease, track []TrackRelease) []TrackRelease {
	out := []TrackRelease{}
	for _, r := range track {
		if r.Track != installed.Track || r.Package != installed.Package || r.ReleaseID == installed.ReleaseID {
			continue
		}
		if r.State == StateListed && r.Seq > installed.Seq {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Seq > out[j].Seq })
	return out
}

// NewestSoaked is the newest of newer that has soaked at now.
func NewestSoaked(newer []TrackRelease, now time.Time) (TrackRelease, bool) {
	for _, r := range newer {
		if r.Soaked(now) {
			return r, true
		}
	}
	return TrackRelease{}, false
}

// Update states, as the installed list and the Library show them.
const (
	UpdateStateNone          = "none"
	UpdateStateAvailable     = "available"
	UpdateStateNeedsApproval = "needs_approval"
	UpdateStateUpdated       = "updated"
	UpdateStatePlatformYank  = "removed_by_platform"
	UpdateStatePlatformForce = "updated_by_platform"
)

// AutomaticCandidate decides whether an automatic installer takes a release
// now: the policy is automatic, the release soaked, and the change does not
// widen. It returns the reason when it does not.
func AutomaticCandidate(mode string, release TrackRelease, change UpdateChange, now time.Time) (bool, string) {
	switch {
	case mode != UpdateAutomatic:
		return false, "manual: the owner presses Update"
	case !release.Soaked(now):
		return false, fmt.Sprintf("listed less than %s ago; automatic updates wait for the soak", UpdateSoak)
	case change.Widening():
		return false, "the update widens what the app may do (" + strings.Join(wideningWords(change), "; ") + "); it waits for the owner's approval"
	}
	return true, ""
}

func wideningWords(c UpdateChange) []string {
	var out []string
	if c.UsesUnknown {
		out = append(out, "the installed release's uses were not recorded")
	}
	if len(c.AddedUses) > 0 {
		out = append(out, "new SDK uses: "+strings.Join(c.AddedUses, ", "))
	}
	if len(c.AddedGrants) > 0 {
		out = append(out, "new grants: "+strings.Join(c.AddedGrants, ", "))
	}
	return out
}
