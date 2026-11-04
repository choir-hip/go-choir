package apppackage

// Platform reports what isolation the running platform has. A validator rule
// that depends on it is a capability check, not a switch: the platform either
// has the isolation or lacks it.
type Platform struct {
	// DeskSandbox is true when Phase 6 has landed: the iframe sandbox for the
	// UI, the audited and fuzzed broker, and the worker sandbox (gVisor) wrap
	// every non-first-party desk (R24, R39, R40).
	DeskSandbox bool
}

// CurrentPlatform is the isolation this build has. MA6.1 to MA6.5 replace this
// body with detection of the sandbox; until then no desk from another
// publisher is published or installed.
func CurrentPlatform() Platform { return Platform{DeskSandbox: false} }

// AdmitsDesksFrom is the one answer to "may a desk from this origin be published
// or installed here": a first-party desk always; any other origin's only when
// the platform has the isolation. The manifest validator and the release record
// both ask it, so they cannot disagree when the sandbox ships.
func (p Platform) AdmitsDesksFrom(origin string) bool {
	return origin == OriginFirstParty || p.DeskSandbox
}
