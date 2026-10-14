package vmbackend

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// CloudHypervisorBackend launches a computer under Cloud Hypervisor: cold
// boot, stop and metrics only in this slice. Snapshot and restore are not
// offered (Caps().Snapshot is false), so under VM_HIBERNATE_MODE=snapshot a
// Cloud Hypervisor computer takes the kill path and cold boots on wake,
// exactly as a Firecracker computer does in kill mode.
//
// Same guest image, same data disk, same kernel command line as Firecracker.
// Differences, all deliberate:
//   - the tap arrives as an open fd (--net fd=3), so the VMM needs no
//     CAP_NET_ADMIN to attach it (vmmanager still creates the tap);
//   - every disk says image_type=raw, so a guest that writes a QCOW2 header
//     into data.img cannot make the VMM follow a host backing file
//     (CVE-2026-27211 class);
//   - seccomp is always on (--seccomp true); Landlock is opt-in until its
//     rules are proven on the test host;
//   - the serial console goes to stdout (--serial tty --console off), the
//     same console file Firecracker writes.
//
// It runs as the same user as Firecracker today (vmctl's). The jailer work
// replaces that with the shared confinement contract for both backends.
type CloudHypervisorBackend struct {
	// Landlock adds --landlock with rules for every HostPath of the plan.
	Landlock bool
}

// chTapFD is the child's fd number for the tap: the first ExtraFile.
const chTapFD = 3

// CloudHypervisorEventsName is the event-monitor file in the VM state dir.
// It is a file, never a pipe: a pipe whose reader is vmctl would raise
// SIGPIPE in the VMM after a vmctl restart (the Firecracker console
// deadlock of 2026-10-10).
const CloudHypervisorEventsName = "ch-events.json"

func (CloudHypervisorBackend) Name() Name { return CloudHypervisor }

func (b CloudHypervisorBackend) Caps() Caps {
	return Caps{
		ColdBoot:                 true,
		BalloonFreePageReporting: true,
		TapFD:                    true,
		Landlock:                 b.Landlock,
		// core_scheduling=vm is Cloud Hypervisor's default since v52.
		CoreScheduling: true,
		APICounters:    true,
	}
}

func (CloudHypervisorBackend) Version(ctx context.Context, binary string) (string, error) {
	if binary == "" {
		binary = "cloud-hypervisor"
	}
	return versionBanner(ctx, binary)
}

// chDiskArg formats one --disk value.
func chDiskArg(d Drive) string {
	parts := []string{"path=" + d.Path}
	if d.ReadOnly {
		parts = append(parts, "readonly=on")
	}
	parts = append(parts, "image_type=raw", "id="+d.ID)
	return strings.Join(parts, ",")
}

// CloudHypervisorArgs builds the command line. Exported for tests and the
// report.
func (b CloudHypervisorBackend) CloudHypervisorArgs(spec MachineSpec) ([]string, error) {
	if err := spec.Validate(); err != nil {
		return nil, err
	}
	for _, d := range spec.Drives {
		// Cloud Hypervisor's option parser splits on ',' and '='.
		if strings.ContainsAny(d.Path, ",=") || strings.ContainsAny(d.ID, ",=") {
			return nil, fmt.Errorf("vmbackend: cloud-hypervisor disk %q path %q contains ',' or '='", d.ID, d.Path)
		}
	}
	if spec.Net != nil {
		if spec.Net.TapFD == nil {
			return nil, fmt.Errorf("vmbackend: cloud-hypervisor attaches the tap by fd; open it with OpenTap first")
		}
		if spec.Net.MAC == "" {
			return nil, fmt.Errorf("vmbackend: cloud-hypervisor needs the guest MAC")
		}
	}
	if strings.ContainsAny(spec.StateDir, ",=") {
		return nil, fmt.Errorf("vmbackend: state dir %q contains ',' or '='", spec.StateDir)
	}
	if strings.ContainsAny(spec.APISocketPath, ",=") {
		return nil, fmt.Errorf("vmbackend: API socket path %q contains ',' or '='", spec.APISocketPath)
	}
	args := make([]string, 0, 32)
	if spec.APISocketPath != "" {
		args = append(args, "--api-socket", "path="+spec.APISocketPath)
	}
	args = append(args, "--kernel", spec.Kernel)
	if spec.Initrd != "" {
		args = append(args, "--initramfs", spec.Initrd)
	}
	args = append(args,
		"--cmdline", spec.Cmdline,
		"--cpus", fmt.Sprintf("boot=%d", spec.VCPUs),
		"--memory", fmt.Sprintf("size=%dM", spec.MemMiB),
	)
	args = append(args, "--disk")
	for _, d := range spec.Drives {
		args = append(args, chDiskArg(d))
	}
	if spec.Net != nil {
		id := spec.Net.ID
		if id == "" {
			id = "eth0"
		}
		args = append(args, "--net", fmt.Sprintf("fd=%d,mac=%s,id=%s", chTapFD, spec.Net.MAC, id))
	}
	if bl := spec.Balloon; bl != nil {
		v := fmt.Sprintf("size=%dM", bl.AmountMiB)
		if bl.DeflateOnOOM {
			v += ",deflate_on_oom=on"
		}
		if bl.FreePageReporting {
			v += ",free_page_reporting=on"
		}
		// Cloud Hypervisor has no statistics polling interval option;
		// StatsPollingIntervalS is Firecracker-only.
		args = append(args, "--balloon", v)
	}
	args = append(args, "--serial", "tty", "--console", "off", "--seccomp", "true",
		"--event-monitor", "path="+filepath.Join(spec.StateDir, CloudHypervisorEventsName))
	if b.Landlock {
		args = append(args, "--landlock")
		rules := landlockRules(hostPathsFor(spec, ""))
		if len(rules) > 0 {
			args = append(args, "--landlock-rules")
			args = append(args, rules...)
		}
	}
	return args, nil
}

func landlockRules(paths []HostPath) []string {
	seen := map[string]bool{}
	var rules []string
	for _, p := range paths {
		if p.Path == "" || seen[p.Path] || strings.ContainsAny(p.Path, ",=") {
			continue
		}
		seen[p.Path] = true
		access := "r"
		if p.Access == AccessReadWrite {
			access = "rw"
		}
		rules = append(rules, fmt.Sprintf("path=%s,access=%s", p.Path, access))
	}
	sort.Strings(rules)
	return rules
}

func (b CloudHypervisorBackend) PlanLaunch(spec MachineSpec) (LaunchPlan, error) {
	args, err := b.CloudHypervisorArgs(spec)
	if err != nil {
		return LaunchPlan{}, err
	}
	binary := spec.Binary
	if binary == "" {
		binary = "cloud-hypervisor"
	}
	var extra []*os.File
	if spec.Net != nil {
		extra = []*os.File{spec.Net.TapFD}
	}
	digest, err := json.Marshal(struct {
		Binary string   `json:"binary"`
		Args   []string `json:"args"`
	}{binary, args})
	if err != nil {
		return LaunchPlan{}, err
	}
	paths := hostPathsFor(spec, "")
	paths = append(paths,
		HostPath{Path: "/dev/urandom", Access: AccessRead, Purpose: "rng"},
		HostPath{Path: filepath.Join(spec.StateDir, CloudHypervisorEventsName), Access: AccessReadWrite, Purpose: "event-monitor"})
	return LaunchPlan{
		Backend:      CloudHypervisor,
		VMID:         spec.VMID,
		Binary:       binary,
		Args:         args,
		ExtraFiles:   extra,
		HostPaths:    paths,
		APISocket:    spec.APISocketPath,
		ConfigDigest: sha256Hex(digest),
	}, nil
}

// VMIDFromCmdline recognises a Cloud Hypervisor process by an argument whose
// base name contains "cloud-hypervisor" and reads the VM id from the guest
// kernel command line's vm_id= token. Cloud Hypervisor has no --id flag;
// vmmanager always puts vm_id=<id> on the command line it builds.
func (CloudHypervisorBackend) VMIDFromCmdline(args []string) (string, bool) {
	hasCH := false
	for _, a := range args {
		if strings.Contains(filepath.Base(a), "cloud-hypervisor") {
			hasCH = true
			break
		}
	}
	if !hasCH {
		return "", false
	}
	for i, a := range args {
		var cmdline string
		if a == "--cmdline" && i+1 < len(args) {
			cmdline = args[i+1]
		} else if v, ok := strings.CutPrefix(a, "--cmdline="); ok {
			cmdline = v
		} else {
			continue
		}
		for _, tok := range strings.Fields(cmdline) {
			if v, ok := strings.CutPrefix(tok, "vm_id="); ok && v != "" {
				return v, true
			}
		}
		return "", false
	}
	return "", false
}

// Metrics reads /proc and, with an API socket, the VMM's device counters
// (GET /api/v1/vm.counters). A counters failure returns the /proc half with
// the error.
func (CloudHypervisorBackend) Metrics(ctx context.Context, h Handle) (Metrics, error) {
	m, err := ProcMetrics(h.PID)
	m.Backend = CloudHypervisor
	if err != nil || h.APISocket == "" {
		return m, err
	}
	var counters map[string]map[string]uint64
	api := newUnixAPI("cloud-hypervisor", h.APISocket)
	if err := api.do(ctx, CloudHypervisor, http.MethodGet, "/api/v1/vm.counters", nil, &counters); err != nil {
		return m, fmt.Errorf("vmbackend: cloud-hypervisor counters: %w", err)
	}
	m.Counters = flattenCounters(counters)
	return m, nil
}

func flattenCounters(in map[string]map[string]uint64) map[string]uint64 {
	out := make(map[string]uint64)
	for dev, counters := range in {
		for name, v := range counters {
			out[dev+"."+name] = v
		}
	}
	return out
}

// CloudHypervisorEvent is one event-monitor record. Cloud Hypervisor writes
// them as concatenated pretty-printed JSON objects.
type CloudHypervisorEvent struct {
	Timestamp struct {
		Secs  uint64 `json:"secs"`
		Nanos uint32 `json:"nanos"`
	} `json:"timestamp"`
	Source     string            `json:"source"`
	Event      string            `json:"event"`
	Properties map[string]string `json:"properties"`
}

// EndsRealization reports an event after which this VMM must not keep
// running the computer. Cloud Hypervisor handles a guest reset (panic=1,
// reboot=k, a guest reboot) by rebooting the VM inside the same process,
// where Firecracker exits. A silent in-process reboot would re-run the
// guest with the same epoch and an already-consumed credential envelope,
// hiding a crash restart from vmmanager. vmmanager kills the VMM on these
// events so a guest reset becomes a process exit, as on Firecracker.
func (e CloudHypervisorEvent) EndsRealization() bool {
	return e.Source == "vm" && (e.Event == "rebooting" || e.Event == "rebooted")
}

// DecodeCloudHypervisorEvents reads every complete event from the stream.
// A truncated trailing object (the VMM is mid-write) is not an error; it is
// returned on the next read.
func DecodeCloudHypervisorEvents(r io.Reader) ([]CloudHypervisorEvent, error) {
	dec := json.NewDecoder(r)
	var out []CloudHypervisorEvent
	for {
		var ev CloudHypervisorEvent
		err := dec.Decode(&ev)
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return out, nil
		}
		if err != nil {
			return out, fmt.Errorf("vmbackend: cloud-hypervisor event stream: %w", err)
		}
		out = append(out, ev)
	}
}

// Ping answers whether the VMM API is served (GET /api/v1/vmm.ping).
func (CloudHypervisorBackend) Ping(ctx context.Context, h Handle) error {
	if h.APISocket == "" {
		return fmt.Errorf("%w: launched without an API socket", ErrUnsupported)
	}
	return newUnixAPI("cloud-hypervisor", h.APISocket).do(ctx, CloudHypervisor, http.MethodGet, "/api/v1/vmm.ping", nil, nil)
}
