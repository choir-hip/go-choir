package pkgrelease

import (
	"github.com/yusefmosiah/go-choir/internal/apppackage"
)

// An owner's own update of a base package that draws the shell (T2; decision
// of 2026-11-08 in docs/design/modular-architecture-plan-2026-11-04.md 12.1).
//
// Phase 8 runs Files and the context-menu rounds as in-place updates through
// self-development on the owner's own computer, and the menu renderer
// (menu-default) is T2. R5 stays: a stranger's release must never draw
// Choir's screen. Three gates, none a switch:
//
//  1. publish: corpusd admits a T2 release only on the by-decision route
//     (HostInput.ByDecision), on the owner's own track, for a package the
//     base image provides (apppackage.IsBuiltinPackage). The owner-act route
//     keeps the blanket refusal.
//  2. listing: such a release is filed StateOwnerListed, visible to its owner
//     only and never public.
//  3. install: a T2 record installs only when rec.Track is the installing
//     owner (RequireOwnTrackShell), at every door.
//
// Every layer that reads a record's bytes (host check, build, install
// review) relaxes the shell refusals through OwnShellUpdate, a pure function
// of the record the id hashes. That relaxation is not an authority: the
// install gate is.

// ReasonShellNotYours: a T2 release installs only from the installing
// owner's own track.
const ReasonShellNotYours = "package_shell_not_yours"

// IsShell reports whether the record's manifest summary is tier T2: it
// provides a shell surface (class provider) or holds the ui.shell grant
// (the only privileged grant a Library release can carry here, so level
// privileged). The test is behavioral, never by name.
func (r Record) IsShell() bool {
	return apppackage.Class(r.Manifest.Class) == apppackage.ClassProvider ||
		apppackage.Level(r.Manifest.Level) == apppackage.LevelPrivileged
}

// OwnShellUpdate reports whether the record is on its publisher's own track
// and names a package the base image provides: the only shape in which T2
// content is not refused by the manifest checks.
func (r Record) OwnShellUpdate() bool {
	return r.Track != "" && r.Track == r.Publisher.OwnerID && apppackage.IsBuiltinPackage(r.Package.Name)
}

// RequireOwnTrackShell is the install gate: a T2 record installs only when
// it is on the installing owner's own track. Records are only those corpusd
// filed (VerifyID); a forged record has no id the store holds.
func RequireOwnTrackShell(rec Record, installingOwner string) error {
	if !rec.IsShell() {
		return nil
	}
	if installingOwner == "" || rec.Track != installingOwner || rec.Publisher.OwnerID != installingOwner || !apppackage.IsBuiltinPackage(rec.Package.Name) {
		return refuse(ReasonShellNotYours, "package %s draws part of Choir's own screen (tier T2); it installs only from the installing owner's own track, as an update of a base package", rec.Package.Name)
	}
	return nil
}
