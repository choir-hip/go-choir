package yaegikernel

import (
	"os"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
)

// Desk cell confinement (F4,
// docs/problems/desk-boundary-residuals-2026-10-21.md).
//
// Management, Texture and research run model-authored Go in a desk session
// worker. Doctrine I18: Yaegi is not the security boundary, so the worker
// process is confined as if the interpreter had already been escaped:
//
//   - spawner (a root guest runtime): the worker runs as its desk's own
//     unprivileged user (no supplementary groups, so no capabilities after
//     exec) in a private, empty network namespace and IPC namespace, with
//     its own 0700 tmp dir as cwd and TMPDIR;
//   - worker, before it serves a cell: no_new_privs, Landlock (read-only on
//     the allowed root, read-write on its tmp dir, nothing else; no TCP bind
//     or connect), and a deny-by-default seccomp filter that admits no
//     socket creation, no exec, no fork (threads only), no ptrace, and
//     signals only to itself.
//
// The worker needs nothing else: every effect is a frame on the inherited
// fd 3 socket that the host answers.

// CellConfinementMode is how strictly a desk worker is confined.
type CellConfinementMode string

const (
	// CellConfinementAuto (the zero value) confines as far as the spawner
	// can: a root spawner always switches identity and network namespace
	// (a cell never runs as root); a non-root spawner (dev, CI) loads
	// no_new_privs and seccomp, and Landlock where the kernel has it.
	CellConfinementAuto CellConfinementMode = ""
	// CellConfinementRequired (the guest image) additionally makes Landlock
	// V5 mandatory and refuses to spawn when the identity switch is not
	// possible. Nothing falls back to an unconfined cell.
	CellConfinementRequired CellConfinementMode = "required"
)

// DeskCellConfinementEnv selects the mode for the guest runtime: "required"
// in the guest image; unset or "auto" otherwise. Any other value reads as
// required, so a typo strengthens the policy. There is no opt-out.
const DeskCellConfinementEnv = "CHOIR_DESK_CELL_CONFINEMENT"

// DeskCellConfinementModeFromEnv reads DeskCellConfinementEnv.
func DeskCellConfinementModeFromEnv(getenv func(string) string) CellConfinementMode {
	if getenv == nil {
		getenv = os.Getenv
	}
	switch strings.ToLower(strings.TrimSpace(getenv(DeskCellConfinementEnv))) {
	case "", "auto":
		return CellConfinementAuto
	}
	return CellConfinementRequired
}

// CellConfinement is the confinement a desk worker spawn applies.
type CellConfinement struct {
	Mode CellConfinementMode
	// UID overrides the role's identity (DeskCellIdentity). Tests only; a
	// non-zero value, and never root.
	UID uint32
}

// DeskCellIdentity is the numeric uid and gid a desk's cells run as, read from
// the desk manifest (internal/deskmanifest): one per desk, so a cell of one
// desk cannot signal, trace or read another desk's worker. A management task
// shares management's, a research child research's. A role the manifest does
// not give a desk-cell sandbox has none, and its spawn is refused. The identity
// is numeric on purpose: the worker never depends on a passwd entry, so there
// is no "user missing from the image" state to fall back from; the guest image
// declares matching users only so operators see names (nix/autoputer-vm.nix).
func DeskCellIdentity(role string) (uint32, bool) {
	d, ok := deskmanifest.Lookup(role)
	if !ok || d.Carrier != deskmanifest.CarrierDeskWorker || d.Sandbox.Profile != "desk_cell" || d.Sandbox.UID == 0 {
		return 0, false
	}
	return d.Sandbox.UID, true
}

// cellSandboxLevel is the worker-side half of the contract, passed in the
// worker's environment.
type cellSandboxLevel string

const (
	cellSandboxBestEffort cellSandboxLevel = "best_effort"
	cellSandboxRequired   cellSandboxLevel = "required"
)

// Worker environment keys (after the session prefix).
const (
	cellSandboxEnv = "SANDBOX"
	cellTmpDirEnv  = "CELL_TMPDIR"
	cellUIDEnv     = "CELL_UID"
)

func sessionEnvPrefix(prefix string) string {
	if prefix == "" {
		return "CHOIR_DESK_SESSION_"
	}
	return prefix
}
