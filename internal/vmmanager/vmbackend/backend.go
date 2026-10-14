// Package vmbackend is the seam between vmmanager's hypervisor-neutral VM
// lifecycle and the virtual machine monitor (VMM) that runs one computer.
//
// Status (2026-10-14): nothing imports this package yet. It is the interface,
// a Firecracker backend that reproduces today's launch config byte for byte,
// and a first Cloud Hypervisor backend (cold boot, stop, metrics). vmmanager
// moves onto it in the migration steps of
// docs/reports/vm-backend-interface-2026-10-14.md, after the Firecracker
// jailer and the admission/reclaim branches land.
//
// What stays in vmmanager (hypervisor-neutral): state directories, data and
// credential disks, taps and iptables, epochs, health probes, boot
// timelines, snapshot budget and metadata, destruction receipts, OOM
// scores. What a Backend owns: turning a MachineSpec into a command line
// and config files, recognising its own processes, its control API
// (snapshot, balloon, counters), and its version.
//
// Rules that keep the seam small:
//   - Capabilities, not type switches: callers ask Caps(), and a capability
//     a backend reports must be implemented (conformance tests check both
//     directions).
//   - A computer's backend is recorded per computer (LaunchRecord) and
//     changes only while the computer is stopped (CheckSwitch). Snapshots
//     are never portable between backends.
//   - One registry of live VMMs across backends (Registry.LiveVMs), so
//     process reconcile sees every VMM, not only Firecracker.
package vmbackend

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Name identifies a backend. It is recorded per computer and in snapshot
// metadata, so the strings are durable: never rename one.
type Name string

const (
	// Firecracker is the production backend and the default for every
	// computer with no launch record.
	Firecracker Name = "firecracker"
	// CloudHypervisor is the second backend (cold boot only in this slice).
	CloudHypervisor Name = "cloud-hypervisor"

	// Default is the backend of a computer that never recorded one.
	Default = Firecracker
)

// KnownNames lists every backend name this build understands.
func KnownNames() []Name { return []Name{Firecracker, CloudHypervisor} }

// ParseName maps a configured string to a Name. Empty means Default.
// Unknown names are an error, never a silent fallback.
func ParseName(s string) (Name, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return Default, nil
	}
	for _, n := range KnownNames() {
		if s == string(n) {
			return n, nil
		}
	}
	return "", fmt.Errorf("vmbackend: unknown backend %q (known: %s, %s)", s, Firecracker, CloudHypervisor)
}

// ErrUnsupported is returned by an operation the backend does not offer.
// Callers check Caps() first; this error is the backstop.
var ErrUnsupported = errors.New("vmbackend: operation not supported by this backend")

// Caps is what a backend's Choir implementation supports: not everything
// the VMM can do upstream, only what is wired here. The interface report
// carries the upstream matrix.
type Caps struct {
	// ColdBoot: launch from kernel, initrd and disks. Every backend must.
	ColdBoot bool
	// Snapshot: full memory snapshot of a paused VM and restore into a fresh
	// process (the S3 hibernate path). Requires the Snapshotter interface.
	Snapshot bool
	// DiffSnapshot: incremental snapshots.
	DiffSnapshot bool
	// CoWRestore: several VMs restored from one snapshot share its pages.
	CoWRestore bool
	// LiveMigrate: move a running VM to another host.
	LiveMigrate bool
	// BalloonFreePageReporting: a launch-time balloon that reports freed
	// guest pages so host RSS follows guest use.
	BalloonFreePageReporting bool
	// BalloonResize: change the balloon target at run time. Requires the
	// MemoryController interface.
	BalloonResize bool
	// VirtioMem: hotplug memory leases.
	VirtioMem bool
	// Discard: virtio-blk discard reaches the host file.
	Discard bool
	// TapFD: the VMM accepts an already-open tap file descriptor, so it
	// needs no CAP_NET_ADMIN to attach networking.
	TapFD bool
	// Landlock: the VMM can restrict its own file access.
	Landlock bool
	// CoreScheduling: the VMM sets a core-scheduling cookie for its vCPUs.
	CoreScheduling bool
	// APICounters: Metrics includes per-device counters from the VMM's API.
	APICounters bool
}

// Drive is one virtio-blk disk, in guest order. The guest mounts by order
// (/dev/vda store, /dev/vdb data) and by label (CHOIR_CRED), so order is part
// of the contract every backend must keep.
type Drive struct {
	ID       string
	Path     string
	ReadOnly bool
	// Root marks the legacy rootfs drive (no store disk).
	Root bool
}

// NetIface is the computer's one network interface. vmmanager creates the
// tap, its /30 and its iptables rules before launch; the backend only
// attaches it.
type NetIface struct {
	ID      string
	TapName string
	MAC     string
	// TapFD is an open handle on the tap (IFF_TAP|IFF_NO_PI|IFF_VNET_HDR),
	// for backends with Caps().TapFD. The launch passes it to the child;
	// the caller closes its copy after Start.
	TapFD *os.File
}

// Balloon is the launch-time balloon device.
type Balloon struct {
	AmountMiB             int
	DeflateOnOOM          bool
	FreePageReporting     bool
	StatsPollingIntervalS int
}

// MachineSpec is the backend-neutral description of one realization.
// vmmanager builds it (kernel command line, drive list, tap) exactly as it
// builds the Firecracker config today; a backend only formats it.
type MachineSpec struct {
	VMID       string
	ComputerID string
	// StateDir is the per-VM state directory (<VM_STATE_DIR>/<vmID>).
	// Backends write their config files here.
	StateDir string
	// Binary is the VMM executable (VM_FIRECRACKER_BIN, VM_CLOUD_HYPERVISOR_BIN).
	Binary  string
	Kernel  string
	Initrd  string
	Cmdline string
	VCPUs   int
	MemMiB  int
	Drives  []Drive
	Net     *NetIface
	Balloon *Balloon
	// APISocketPath, when set, asks the VMM to serve its control API there.
	// vmmanager prepares the directory and enforces the sun_path limit;
	// empty means no API (Firecracker's --no-api).
	APISocketPath string
}

// Validate refuses a spec no backend should launch.
func (s MachineSpec) Validate() error {
	switch {
	case strings.TrimSpace(s.VMID) == "" || strings.ContainsRune(s.VMID, '/') || s.VMID == "." || s.VMID == "..":
		return fmt.Errorf("vmbackend: invalid VM ID %q", s.VMID)
	case s.StateDir == "" || !filepath.IsAbs(s.StateDir):
		return fmt.Errorf("vmbackend: state dir must be absolute, got %q", s.StateDir)
	case s.Kernel == "":
		return fmt.Errorf("vmbackend: kernel is required")
	case s.VCPUs <= 0:
		return fmt.Errorf("vmbackend: vcpus must be positive, got %d", s.VCPUs)
	case s.MemMiB <= 0:
		return fmt.Errorf("vmbackend: memory must be positive, got %d MiB", s.MemMiB)
	case len(s.Drives) == 0:
		return fmt.Errorf("vmbackend: at least one drive is required")
	}
	seen := map[string]bool{}
	for _, d := range s.Drives {
		if d.ID == "" || d.Path == "" {
			return fmt.Errorf("vmbackend: drive needs an id and a path: %+v", d)
		}
		if seen[d.ID] {
			return fmt.Errorf("vmbackend: duplicate drive id %q", d.ID)
		}
		seen[d.ID] = true
	}
	return nil
}

// Access is how the VMM uses a host path.
type Access string

const (
	AccessRead      Access = "r"
	AccessReadWrite Access = "rw"
)

// HostPath is one host path the VMM opens. A launch plan lists all of them so
// a confinement wrapper (the Firecracker jailer, a systemd cage, Cloud
// Hypervisor's Landlock rules) can expose exactly these and nothing else.
type HostPath struct {
	Path    string
	Access  Access
	Purpose string
}

// PlannedFile is a file the launch writes before exec (Firecracker's
// --config-file).
type PlannedFile struct {
	Path string
	Data []byte
	Mode os.FileMode
}

// LaunchPlan is a complete, inspectable VMM launch: argv, files, fds. It is
// pure data so tests can check it without KVM, and so a confinement
// wrapper can rewrite it (jail paths, prepend a jailer) before Start.
type LaunchPlan struct {
	Backend Name
	VMID    string
	Binary  string
	Args    []string
	// Env nil inherits vmctl's environment (Firecracker today).
	Env        []string
	WriteFiles []PlannedFile
	// ExtraFiles become the child's fds 3, 4, ...; Args name them by number.
	ExtraFiles []*os.File
	HostPaths  []HostPath
	APISocket  string
	// ConfigDigest pins the launch description for snapshot identity. For
	// Firecracker it equals vmmanager's launchConfigHash of fc-config.json.
	ConfigDigest string
}

// Handle names a running VMM process.
type Handle struct {
	Backend   Name
	VMID      string
	PID       int
	APISocket string
}

// Metrics is one observation of a VMM process.
type Metrics struct {
	Backend Name
	PID     int
	// RSSBytes is the VMM's resident set, which includes guest memory the
	// guest has touched and not returned.
	RSSBytes  int64
	UserCPU   time.Duration
	SystemCPU time.Duration
	// Counters are the VMM's own device counters ("<device>.<counter>"),
	// present when Caps().APICounters and the API socket answered.
	Counters map[string]uint64
	At       time.Time
}

// Backend is one VMM. Implementations hold no per-VM state: everything per
// VM is in the MachineSpec or the Handle, so one Backend serves every
// computer on the host.
type Backend interface {
	Name() Name
	Caps() Caps
	// Version is the VMM build's own banner, pinned into boot timelines and
	// snapshot identity. An empty version must refuse snapshot use.
	Version(ctx context.Context, binary string) (string, error)
	// PlanLaunch formats a cold boot. It does no I/O.
	PlanLaunch(spec MachineSpec) (LaunchPlan, error)
	// VMIDFromCmdline recognises this backend's VMM process for a VM from
	// /proc/<pid>/cmdline (split on NUL). ok is false for any other process.
	VMIDFromCmdline(args []string) (vmID string, ok bool)
	// Metrics observes a running VMM.
	Metrics(ctx context.Context, h Handle) (Metrics, error)
}

// SnapshotFiles are the files of one captured snapshot.
type SnapshotFiles struct {
	Dir   string
	Files []string
}

// Snapshotter is implemented by backends with Caps().Snapshot.
type Snapshotter interface {
	// Pause stops the vCPUs.
	Pause(ctx context.Context, h Handle) error
	// CreateSnapshot writes a full snapshot of a paused VM into dir.
	CreateSnapshot(ctx context.Context, h Handle, dir string) (SnapshotFiles, error)
	// PlanRestore formats the launch of a fresh VMM that will restore dir.
	PlanRestore(spec MachineSpec, dir string) (LaunchPlan, error)
	// CompleteRestore loads (if the backend loads over its API) and
	// resumes the restored VM.
	CompleteRestore(ctx context.Context, h Handle, dir string) error
}

// MemoryController is implemented by backends with Caps().BalloonResize.
type MemoryController interface {
	SetBalloonTargetMiB(ctx context.Context, h Handle, mib int) error
}
