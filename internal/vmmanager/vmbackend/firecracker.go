package vmbackend

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// FirecrackerBackend formats today's Firecracker launch from a MachineSpec.
// Its config JSON must stay byte-identical to vmmanager.buildFirecrackerConfig
// for the same inputs: launchConfigHash pins committed snapshots to those
// bytes, so any drift turns every hibernated computer's wake into a crash
// boot (launch_config_mismatch). firecracker_test.go holds the golden
// captured from vmmanager at 980ffaf4e.
type FirecrackerBackend struct{}

// FirecrackerConfigName is the --config-file name in the VM state dir.
const FirecrackerConfigName = "fc-config.json"

func (FirecrackerBackend) Name() Name { return Firecracker }

func (FirecrackerBackend) Caps() Caps {
	return Caps{
		ColdBoot:                 true,
		Snapshot:                 true,
		BalloonFreePageReporting: true,
	}
}

func (FirecrackerBackend) Version(ctx context.Context, binary string) (string, error) {
	if binary == "" {
		binary = "firecracker"
	}
	return versionBanner(ctx, binary)
}

// FirecrackerConfig builds the --config-file object. Exported for the
// migration's equivalence test against vmmanager.buildFirecrackerConfig.
func FirecrackerConfig(spec MachineSpec) map[string]interface{} {
	drives := make([]map[string]interface{}, 0, len(spec.Drives))
	for _, d := range spec.Drives {
		drives = append(drives, map[string]interface{}{
			"drive_id":       d.ID,
			"path_on_host":   d.Path,
			"is_root_device": d.Root,
			"is_read_only":   d.ReadOnly,
		})
	}
	bootSource := map[string]interface{}{
		"kernel_image_path": spec.Kernel,
		"boot_args":         spec.Cmdline,
	}
	if spec.Initrd != "" {
		bootSource["initrd_path"] = spec.Initrd
	}
	cfg := map[string]interface{}{
		"boot-source": bootSource,
		"drives":      drives,
		"machine-config": map[string]interface{}{
			"vcpu_count":   spec.VCPUs,
			"mem_size_mib": spec.MemMiB,
		},
	}
	if spec.Net != nil {
		cfg["network-interfaces"] = []map[string]interface{}{
			{
				"iface_id":      spec.Net.ID,
				"guest_mac":     spec.Net.MAC,
				"host_dev_name": spec.Net.TapName,
			},
		}
	}
	if b := spec.Balloon; b != nil {
		cfg["balloon"] = map[string]interface{}{
			"amount_mib":               b.AmountMiB,
			"deflate_on_oom":           b.DeflateOnOOM,
			"stats_polling_interval_s": b.StatsPollingIntervalS,
			"free_page_reporting":      b.FreePageReporting,
		}
	}
	return cfg
}

func (FirecrackerBackend) PlanLaunch(spec MachineSpec) (LaunchPlan, error) {
	if err := spec.Validate(); err != nil {
		return LaunchPlan{}, err
	}
	if spec.Net != nil && spec.Net.TapName == "" {
		return LaunchPlan{}, fmt.Errorf("vmbackend: firecracker attaches taps by name; tap name is empty")
	}
	data, err := json.Marshal(FirecrackerConfig(spec))
	if err != nil {
		return LaunchPlan{}, err
	}
	configPath := filepath.Join(spec.StateDir, FirecrackerConfigName)
	var args []string
	if spec.APISocketPath != "" {
		args = []string{"--api-sock", spec.APISocketPath, "--enable-pci", "--id", spec.VMID, "--config-file", configPath}
	} else {
		args = []string{"--no-api", "--enable-pci", "--id", spec.VMID, "--config-file", configPath}
	}
	binary := spec.Binary
	if binary == "" {
		binary = "firecracker"
	}
	return LaunchPlan{
		Backend:      Firecracker,
		VMID:         spec.VMID,
		Binary:       binary,
		Args:         args,
		WriteFiles:   []PlannedFile{{Path: configPath, Data: data, Mode: 0o644}},
		HostPaths:    hostPathsFor(spec, configPath),
		APISocket:    spec.APISocketPath,
		ConfigDigest: sha256Hex(data),
	}, nil
}

// VMIDFromCmdline mirrors vmmanager.firecrackerCmdlineBytesMatchVM: any
// argument whose base name contains "firecracker", plus --id. (vmmanager's
// LiveFirecrackerVMs checks argv[0] only; the permissive form is the one the
// boot and destroy guards use, and a false "live" refuses a destroy where a
// false "dead" would delete a running computer's state.)
func (FirecrackerBackend) VMIDFromCmdline(args []string) (string, bool) {
	if len(args) == 0 {
		return "", false
	}
	hasFC := false
	for _, a := range args {
		if strings.Contains(filepath.Base(a), "firecracker") {
			hasFC = true
			break
		}
	}
	if !hasFC {
		return "", false
	}
	for i, a := range args {
		if a == "--id" && i+1 < len(args) {
			if id := strings.TrimSpace(args[i+1]); id != "" {
				return id, true
			}
		}
		if v, ok := strings.CutPrefix(a, "--id="); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v), true
		}
	}
	return "", false
}

func (FirecrackerBackend) Metrics(_ context.Context, h Handle) (Metrics, error) {
	m, err := ProcMetrics(h.PID)
	m.Backend = Firecracker
	return m, err
}

// --- Snapshotter (the S3 path; vmmanager's fcapi.go does this today) ----

const (
	fcSnapshotStateName = "vmstate"
	fcSnapshotMemName   = "mem"
)

func (FirecrackerBackend) api(h Handle) (*unixAPI, error) {
	if h.APISocket == "" {
		return nil, fmt.Errorf("%w: firecracker launched with --no-api", ErrUnsupported)
	}
	return newUnixAPI("firecracker", h.APISocket), nil
}

func (b FirecrackerBackend) Pause(ctx context.Context, h Handle) error {
	api, err := b.api(h)
	if err != nil {
		return err
	}
	return api.do(ctx, Firecracker, http.MethodPatch, "/vm", map[string]string{"state": "Paused"}, nil)
}

func (b FirecrackerBackend) CreateSnapshot(ctx context.Context, h Handle, dir string) (SnapshotFiles, error) {
	api, err := b.api(h)
	if err != nil {
		return SnapshotFiles{}, err
	}
	state, mem := filepath.Join(dir, fcSnapshotStateName), filepath.Join(dir, fcSnapshotMemName)
	if err := api.do(ctx, Firecracker, http.MethodPut, "/snapshot/create", map[string]string{
		"snapshot_type": "Full",
		"snapshot_path": state,
		"mem_file_path": mem,
	}, nil); err != nil {
		return SnapshotFiles{}, err
	}
	return SnapshotFiles{Dir: dir, Files: []string{state, mem}}, nil
}

func (FirecrackerBackend) PlanRestore(spec MachineSpec, dir string) (LaunchPlan, error) {
	if spec.APISocketPath == "" {
		return LaunchPlan{}, fmt.Errorf("%w: firecracker restore needs an API socket", ErrUnsupported)
	}
	binary := spec.Binary
	if binary == "" {
		binary = "firecracker"
	}
	return LaunchPlan{
		Backend:   Firecracker,
		VMID:      spec.VMID,
		Binary:    binary,
		Args:      []string{"--api-sock", spec.APISocketPath, "--enable-pci", "--id", spec.VMID},
		HostPaths: append(hostPathsFor(spec, ""), HostPath{Path: dir, Access: AccessRead, Purpose: "snapshot"}),
		APISocket: spec.APISocketPath,
	}, nil
}

func (b FirecrackerBackend) CompleteRestore(ctx context.Context, h Handle, dir string) error {
	api, err := b.api(h)
	if err != nil {
		return err
	}
	if err := waitReady(ctx, h.APISocket, func(c context.Context) error {
		return api.do(c, Firecracker, http.MethodGet, "/", nil, nil)
	}); err != nil {
		return err
	}
	return api.do(ctx, Firecracker, http.MethodPut, "/snapshot/load", map[string]any{
		"snapshot_path": filepath.Join(dir, fcSnapshotStateName),
		"mem_backend": map[string]string{
			"backend_type": "File",
			"backend_path": filepath.Join(dir, fcSnapshotMemName),
		},
		"resume_vm": true,
	}, nil)
}

// --- shared helpers ------------------------------------------------------

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// versionBanner runs "<binary> --version" and returns its first line.
func versionBanner(ctx context.Context, binary string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, binary, "--version").Output()
	if err != nil {
		return "", fmt.Errorf("vmbackend: %s --version: %w", binary, err)
	}
	line := strings.TrimSpace(string(out))
	if i := strings.IndexByte(line, '\n'); i >= 0 {
		line = strings.TrimSpace(line[:i])
	}
	if line == "" {
		return "", fmt.Errorf("vmbackend: %s --version printed nothing", binary)
	}
	return line, nil
}

// hostPathsFor lists the host paths a launch opens.
func hostPathsFor(spec MachineSpec, configPath string) []HostPath {
	paths := []HostPath{{Path: "/dev/kvm", Access: AccessReadWrite, Purpose: "kvm"}}
	paths = append(paths, HostPath{Path: spec.Kernel, Access: AccessRead, Purpose: "kernel"})
	if spec.Initrd != "" {
		paths = append(paths, HostPath{Path: spec.Initrd, Access: AccessRead, Purpose: "initrd"})
	}
	for _, d := range spec.Drives {
		access := AccessReadWrite
		if d.ReadOnly {
			access = AccessRead
		}
		paths = append(paths, HostPath{Path: d.Path, Access: access, Purpose: "drive:" + d.ID})
	}
	if configPath != "" {
		paths = append(paths, HostPath{Path: configPath, Access: AccessRead, Purpose: "config"})
	}
	if spec.APISocketPath != "" {
		paths = append(paths, HostPath{Path: filepath.Dir(spec.APISocketPath), Access: AccessReadWrite, Purpose: "api-socket"})
	}
	if spec.Net != nil && spec.Net.TapFD == nil {
		paths = append(paths, HostPath{Path: "/dev/net/tun", Access: AccessReadWrite, Purpose: "tap:" + spec.Net.TapName})
	}
	return paths
}
