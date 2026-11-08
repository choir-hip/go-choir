package apppackage

import (
	"sync/atomic"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
)

// Platform reports what isolation the running platform has. A validator rule
// that depends on it is a capability check, not a switch: the platform either
// has the isolation or lacks it.
type Platform struct {
	// DeskSandbox is true when Phase 6 is in force where the question is
	// asked: the iframe sandbox for the UI, the audited and fuzzed broker, and
	// the worker sandbox (gVisor, required mode, boot selftest passed) wrap
	// every non-first-party desk (R24, R39, R40).
	DeskSandbox bool
}

// IframeSandboxShipped is the UI half of the isolation. The iframe sandbox
// (MA6.1) is the only front-end path for a non-first-party app, so this is a
// build property, not a runtime state: true since MA6.1 landed.
const IframeSandboxShipped = true

// ShippedPlatform is the isolation this build ships. The worker sandbox is in
// the build (MA6.4) and the iframe sandbox is a build property, so the shipped
// platform has DeskSandbox. It is what a HOST-side process answers with
// (corpusd and the proxy validate a release at publish and listing; they run
// no desk worker and cannot see a guest's sandbox). It admits a release to the
// Library; it does not install anything. A guest decides for itself at install
// (CurrentPlatform).
func ShippedPlatform() Platform { return Platform{DeskSandbox: IframeSandboxShipped} }

var platformSource atomic.Pointer[func() Platform]

// SetPlatformSource names where this process reads its platform from. A guest
// runtime sets a source that reads that computer's own worker-sandbox state
// (required mode and a passed boot selftest); a host binary sets
// ShippedPlatform. This is wiring done once at process start by the binary's
// main, not a setting: there is no environment variable or configuration
// that selects it.
func SetPlatformSource(f func() Platform) {
	if f == nil {
		platformSource.Store(nil)
		return
	}
	platformSource.Store(&f)
}

// CurrentPlatform is the isolation the running process has. A process that
// has not named a source (a test, a tool) has none, so a desk from another
// publisher is refused. The validator, the release record and the runtime
// package builder all read this one answer.
func CurrentPlatform() Platform {
	if f := platformSource.Load(); f != nil {
		return (*f)()
	}
	return Platform{}
}

// AdmitsDesksFrom is the one answer to "may a desk from this origin be published
// or installed here": a first-party desk always; any other origin's only when
// the platform has the isolation. The manifest validator and the release record
// both ask it, so they cannot disagree when the sandbox ships.
func (p Platform) AdmitsDesksFrom(origin string) bool {
	return origin == OriginFirstParty || p.DeskSandbox
}

// AdmitsDesksOf is AdmitsDesksFrom for a named package. A release through the
// Library is always origin published, so a first-party package's desk (a slug
// the registry reserves for an in-tree desk, deskmanifest.FirstPartyAppSlug)
// is admitted by its name: the desk row it registers is the image's own
// manifest (appdesk.FirstPartyDesk), never one the release writes, and the
// install admits it only on the owner's own track (agentcore desk_install.go).
// Any other package's desk waits for the sandbox.
func (p Platform) AdmitsDesksOf(origin, pkg string) bool {
	return p.AdmitsDesksFrom(origin) || deskmanifest.FirstPartyAppSlug(pkg)
}
