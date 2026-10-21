// Package capfloor is the guest daemon's capability floor (SDC SEC-15, the
// first step toward a non-root daemon): the daemon keeps only the
// capabilities the inventory below names, each with the code that needs it,
// and drops every other from its bounding set, so neither it nor any child
// (git, go, node, libreoffice, a desk worker, a capsule's user namespace) can
// hold them again.
//
// The bounding set is per thread, and a cgo binary cannot change every
// thread at once (AllThreadsSyscall refuses with cgo), so Apply drops on one
// locked thread at the top of main and re-executes the binary, marked; the
// new image starts with that thread's set on every thread. This is not a
// non-root daemon: root keeps the fifteen capabilities below. The split that
// would make it non-root is a named residual
// (docs/definitions/choir-launch-fp2-desk-boundary-2026-10-21.md, "Second
// batch", with failure modes K1-K6).
package capfloor

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

// ModeEnv selects the mode. The guest image sets it.
const ModeEnv = "CHOIR_RUNTIME_CAPABILITY_FLOOR"

// appliedEnv marks the re-executed image: it verifies and never re-executes
// again (K3).
const appliedEnv = "CHOIR_RUNTIME_CAPABILITY_FLOOR_APPLIED"

// Mode is what the floor does at start.
type Mode string

const (
	// ModeOff changes nothing.
	ModeOff Mode = "off"
	// ModeObserve logs what enforce would drop; the isolation self-test
	// reports the daemon's set.
	ModeObserve Mode = "observe"
	// ModeEnforce drops, re-executes, and refuses to start if any thread
	// still holds a capability outside the floor.
	ModeEnforce Mode = "enforce"
)

// ModeFromEnv reads the mode: unset or "off" is off; a value it does not know
// is enforce (a typo strengthens the policy).
func ModeFromEnv(getenv func(string) string) Mode {
	if getenv == nil {
		getenv = os.Getenv
	}
	switch v := Mode(strings.ToLower(strings.TrimSpace(getenv(ModeEnv)))); v {
	case "", ModeOff:
		return ModeOff
	case ModeObserve:
		return ModeObserve
	default:
		return ModeEnforce
	}
}

// Capability numbers (linux/capability.h; a stable ABI).
const (
	capChown             = 0
	capDacOverride       = 1
	capDacReadSearch     = 2
	capFowner            = 3
	capFsetid            = 4
	capKill              = 5
	capSetgid            = 6
	capSetuid            = 7
	capSetpcap           = 8
	capLinuxImmutable    = 9
	capNetBindService    = 10
	capNetBroadcast      = 11
	capNetAdmin          = 12
	capNetRaw            = 13
	capIpcLock           = 14
	capIpcOwner          = 15
	capSysModule         = 16
	capSysRawio          = 17
	capSysChroot         = 18
	capSysPtrace         = 19
	capSysPacct          = 20
	capSysAdmin          = 21
	capSysBoot           = 22
	capSysNice           = 23
	capSysResource       = 24
	capSysTime           = 25
	capSysTtyConfig      = 26
	capMknod             = 27
	capLease             = 28
	capAuditWrite        = 29
	capAuditControl      = 30
	capSetfcap           = 31
	capMacOverride       = 32
	capMacAdmin          = 33
	capSyslog            = 34
	capWakeAlarm         = 35
	capBlockSuspend      = 36
	capAuditRead         = 37
	capPerfmon           = 38
	capBPF               = 39
	capCheckpointRestore = 40
	capLastCap           = capCheckpointRestore
)

var capNames = map[int]string{
	capChown: "CAP_CHOWN", capDacOverride: "CAP_DAC_OVERRIDE", capDacReadSearch: "CAP_DAC_READ_SEARCH",
	capFowner: "CAP_FOWNER", capFsetid: "CAP_FSETID", capKill: "CAP_KILL", capSetgid: "CAP_SETGID",
	capSetuid: "CAP_SETUID", capSetpcap: "CAP_SETPCAP", capLinuxImmutable: "CAP_LINUX_IMMUTABLE",
	capNetBindService: "CAP_NET_BIND_SERVICE", capNetBroadcast: "CAP_NET_BROADCAST", capNetAdmin: "CAP_NET_ADMIN",
	capNetRaw: "CAP_NET_RAW", capIpcLock: "CAP_IPC_LOCK", capIpcOwner: "CAP_IPC_OWNER", capSysModule: "CAP_SYS_MODULE",
	capSysRawio: "CAP_SYS_RAWIO", capSysChroot: "CAP_SYS_CHROOT", capSysPtrace: "CAP_SYS_PTRACE",
	capSysPacct: "CAP_SYS_PACCT", capSysAdmin: "CAP_SYS_ADMIN", capSysBoot: "CAP_SYS_BOOT", capSysNice: "CAP_SYS_NICE",
	capSysResource: "CAP_SYS_RESOURCE", capSysTime: "CAP_SYS_TIME", capSysTtyConfig: "CAP_SYS_TTY_CONFIG",
	capMknod: "CAP_MKNOD", capLease: "CAP_LEASE", capAuditWrite: "CAP_AUDIT_WRITE", capAuditControl: "CAP_AUDIT_CONTROL",
	capSetfcap: "CAP_SETFCAP", capMacOverride: "CAP_MAC_OVERRIDE", capMacAdmin: "CAP_MAC_ADMIN", capSyslog: "CAP_SYSLOG",
	capWakeAlarm: "CAP_WAKE_ALARM", capBlockSuspend: "CAP_BLOCK_SUSPEND", capAuditRead: "CAP_AUDIT_READ",
	capPerfmon: "CAP_PERFMON", capBPF: "CAP_BPF", capCheckpointRestore: "CAP_CHECKPOINT_RESTORE",
}

// Name is a capability's name; one the kernel added after this table is
// cap_<n>.
func Name(c int) string {
	if n, ok := capNames[c]; ok {
		return n
	}
	return "cap_" + strconv.Itoa(c)
}

// Kept is the inventory: each capability the daemon keeps and why (K1).
// Everything else is dropped. Changing it is a reviewed change
// (TestKeptCapabilitiesAreTheInventory).
var Kept = map[int]string{
	capSysAdmin:       "capsule executor: overlay and bind mounts, user/mount/network namespaces, cgroup creation (internal/capsule/executor.go, namespace.go); the capsule broker's ambient set (BrokerAmbientCaps)",
	capSetuid:         "desk cell workers and document tools start as their own uids (yaegikernel planCellConfinement, internal/doctool); capsule uid maps; the broker's ambient set",
	capSetgid:         "the same identity switches, for groups; the broker's ambient set",
	capSetpcap:        "the broker's ambient capabilities and its own drop (capsule executor.go:611); this floor's own PR_CAPBSET_DROP",
	capChown:          "cell tmp dirs and document-tool inputs owned by their uid; capsule egress sockets owned by the capsule's mapped uid; the broker's ambient set",
	capDacOverride:    "overlayfs copy-up inside capsules (the broker's ambient set); reading what cells and tools wrote in their 0700 dirs",
	capDacReadSearch:  "searching and reading other uids' 0700 dirs (cell tmp dirs, self-test receipts); overlaps CAP_DAC_OVERRIDE, kept until the enforce probe shows it unused",
	capFowner:         "overlayfs copy-up inside capsules (the broker's ambient set); chmod of files the cell uids own",
	capFsetid:         "overlay copy-up keeping set-group-ID bits in capsule trees; kept until the enforce probe shows it unused",
	capKill:           "signals to desk workers, document tools and capsule processes that run as other uids (process-group and cgroup kills)",
	capNetAdmin:       "a capsule's loopback and egress forwarder (the broker's ambient set with egress, cmd/capsule-broker/egress.go)",
	capNetBindService: "the runtime port comes from the kernel command line (guest_port=) and may be below 1024",
	capSysChroot:      "capsule root setup inside its mount namespace; kept until the enforce probe shows it unused",
	capSysPtrace:      "reading desk workers' /proc status and namespace links from outside (the isolation self-test, desk_cell_selftest_linux.go)",
	capSysResource:    "rlimits and oom_score_adj of children of other uids (desk worker memory limits); kept until the enforce probe shows it unused",
}

func keptMask() uint64 {
	var m uint64
	for c := range Kept {
		m |= 1 << c
	}
	return m
}

// Plan reads a bounding set: extra are the capabilities it holds outside the
// floor (enforce drops them), missing are the kept ones it lacks (something
// else already took them; reported, never added back).
func Plan(bounding uint64) (extra, missing []int) {
	for c := 0; c < 64; c++ {
		has := bounding&(1<<c) != 0
		_, kept := Kept[c]
		switch {
		case has && !kept:
			extra = append(extra, c)
		case !has && kept:
			missing = append(missing, c)
		}
	}
	sort.Ints(missing)
	return extra, missing
}

// Names names capabilities.
func Names(caps []int) []string {
	out := make([]string, 0, len(caps))
	for _, c := range caps {
		out = append(out, Name(c))
	}
	return out
}

// Hex is a capability set as /proc/<pid>/status prints it.
func Hex(set uint64) string { return fmt.Sprintf("%016x", set) }

func parseCapBnd(status string) (uint64, error) {
	for _, line := range strings.Split(status, "\n") {
		if v, ok := strings.CutPrefix(line, "CapBnd:"); ok {
			return strconv.ParseUint(strings.TrimSpace(v), 16, 64)
		}
	}
	return 0, fmt.Errorf("no CapBnd in the status")
}

// Status is the daemon's own capability state, for the isolation self-test.
type Status struct {
	Mode    Mode     `json:"mode"`
	UID     int      `json:"uid"`
	CapBnd  string   `json:"cap_bnd"`
	Extra   []string `json:"extra,omitempty"`
	Missing []string `json:"missing,omitempty"`
	// Floor is the bounding set enforce leaves.
	Floor string `json:"floor"`
	Error string `json:"error,omitempty"`
}

// Snapshot reads the calling process's state. Under enforce, Extra must be
// empty.
func Snapshot() Status {
	s := Status{Mode: ModeFromEnv(nil), UID: os.Geteuid(), Floor: Hex(keptMask())}
	raw, err := os.ReadFile("/proc/self/status")
	if err != nil {
		s.Error = err.Error()
		return s
	}
	bnd, err := parseCapBnd(string(raw))
	if err != nil {
		s.Error = err.Error()
		return s
	}
	s.CapBnd = Hex(bnd)
	extra, missing := Plan(bnd)
	s.Extra, s.Missing = Names(extra), Names(missing)
	return s
}
