//go:build linux

package updater

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"time"
)

const kernelProbeHelperEnv = "CHOIR_KERNEL_CAPABILITY_HELPER"

func ProbeKernelCapabilities(ctx context.Context) (KernelCapabilityProbe, error) {
	kernelRelease, err := readTrimmed("/proc/sys/kernel/osrelease")
	if err != nil {
		return KernelCapabilityProbe{}, err
	}
	bootID, err := readTrimmed("/proc/sys/kernel/random/boot_id")
	if err != nil {
		return KernelCapabilityProbe{}, err
	}
	bootParameters, err := readTrimmed("/proc/cmdline")
	if err != nil {
		return KernelCapabilityProbe{}, err
	}
	var stat unix.Statfs_t
	if err := unix.Statfs("/sys/fs/cgroup", &stat); err != nil || uint64(stat.Type) != uint64(unix.CGROUP2_SUPER_MAGIC) {
		return KernelCapabilityProbe{}, fmt.Errorf("kernel capability probe: unified cgroup v2 is unavailable")
	}
	if err := probeCgroupController(); err != nil {
		return KernelCapabilityProbe{}, err
	}
	overlayDigest, err := loadedOverlayModuleDigest(kernelRelease)
	if err != nil {
		return KernelCapabilityProbe{}, err
	}

	checks := []struct {
		helper     string
		capability string
	}{
		{helper: "namespaces", capability: "user_namespace,pid_namespace,mount_namespace,network_namespace,uts_namespace,ipc_namespace"},
		{helper: "overlay", capability: "overlayfs_loaded_and_mountable"},
		{helper: "seccomp", capability: "seccomp_filter_enforced"},
		{helper: "landlock", capability: "landlock_enforcing"},
	}
	for _, check := range checks {
		helper, capability := check.helper, check.capability
		if err := runKernelProbeHelper(ctx, helper); err != nil {
			return KernelCapabilityProbe{}, fmt.Errorf("kernel capability probe: %s: %w", capability, err)
		}
	}
	contract, err := computerevent.CanonicalJSON(map[string]any{"version": 1, "mandatory": mandatoryKernelCapabilities})
	if err != nil {
		return KernelCapabilityProbe{}, err
	}
	capabilities := make(map[string]KernelCapabilityObservation, len(mandatoryKernelCapabilities))
	for _, name := range mandatoryKernelCapabilities {
		evidence := strings.Join([]string{"kernel-capability-v1", name, kernelRelease, bootID, bootParameters, overlayDigest}, "\x00")
		capabilities[name] = KernelCapabilityObservation{Supported: true, Enforced: true, ObservationRef: "sha256:" + computerevent.DigestBytes([]byte(evidence))}
	}
	return KernelCapabilityProbe{
		KernelRelease: kernelRelease, BootID: bootID, BootParameters: bootParameters,
		CgroupFilesystem: "cgroup2", OverlayModuleDigest: overlayDigest,
		ObservedAt:   time.Now().UTC().Format(time.RFC3339Nano),
		Capabilities: capabilities, ContractDigest: computerevent.DigestBytes(contract),
	}, nil
}

func RunKernelCapabilityProbeWriter(ctx context.Context) (bool, int) {
	output := strings.TrimSpace(os.Getenv("CHOIR_KERNEL_CAPABILITY_PROBE_OUTPUT"))
	if output == "" {
		return false, 0
	}
	if !filepath.IsAbs(output) {
		_, _ = fmt.Fprintln(os.Stderr, "kernel capability probe output must be absolute")
		return true, 1
	}
	probe, err := ProbeKernelCapabilities(ctx)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		return true, 1
	}
	raw, err := computerevent.CanonicalJSON(probe)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		return true, 1
	}
	temporary := output + ".tmp"
	if err := os.WriteFile(temporary, raw, 0o400); err == nil {
		err = os.Rename(temporary, output)
	}
	_ = os.Remove(temporary)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		return true, 1
	}
	return true, 0
}

func RunKernelCapabilityProbeHelper() (bool, int) {
	helper := strings.TrimSpace(os.Getenv(kernelProbeHelperEnv))
	if helper == "" {
		return false, 0
	}
	var err error
	switch helper {
	case "namespaces":
		// Successful exec under the requested clone flags is the probe.
	case "overlay":
		err = probeOverlayMount()
	case "seccomp":
		err = probeSeccompEnforcement()
	case "landlock":
		err = probeLandlockEnforcement()
	default:
		err = fmt.Errorf("unknown helper")
	}
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		return true, 1
	}
	return true, 0
}

func runKernelProbeHelper(ctx context.Context, helper string) error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	command := func(flags uintptr, mapRoot bool) *exec.Cmd {
		cmd := exec.CommandContext(ctx, executable)
		cmd.Env = append(os.Environ(), kernelProbeHelperEnv+"="+helper)
		if flags != 0 {
			attr := &syscall.SysProcAttr{Cloneflags: flags}
			if mapRoot {
				attr.UidMappings = []syscall.SysProcIDMap{{ContainerID: 0, HostID: os.Geteuid(), Size: 1}}
				attr.GidMappings = []syscall.SysProcIDMap{{ContainerID: 0, HostID: os.Getegid(), Size: 1}}
				attr.GidMappingsEnableSetgroups = false
			}
			cmd.SysProcAttr = attr
		}
		return cmd
	}
	run := func(cmd *exec.Cmd) error {
		output, err := cmd.CombinedOutput()
		if err != nil {
			if detail := strings.TrimSpace(string(output)); detail != "" {
				return fmt.Errorf("%w: %s", err, detail)
			}
		}
		return err
	}
	switch helper {
	case "namespaces":
		combined := uintptr(unix.CLONE_NEWUSER | unix.CLONE_NEWPID | unix.CLONE_NEWNS | unix.CLONE_NEWNET | unix.CLONE_NEWUTS | unix.CLONE_NEWIPC)
		if err := run(command(combined, true)); err != nil {
			probes := []struct {
				name    string
				flag    uintptr
				mapRoot bool
			}{
				{name: "user", flag: unix.CLONE_NEWUSER, mapRoot: true},
				{name: "pid", flag: unix.CLONE_NEWPID},
				{name: "mount", flag: unix.CLONE_NEWNS},
				{name: "network", flag: unix.CLONE_NEWNET},
				{name: "uts", flag: unix.CLONE_NEWUTS},
				{name: "ipc", flag: unix.CLONE_NEWIPC},
			}
			results := make([]string, 0, len(probes))
			for _, probe := range probes {
				probeErr := run(command(probe.flag, probe.mapRoot))
				if probeErr == nil {
					results = append(results, probe.name+"=ok")
				} else {
					results = append(results, probe.name+"="+probeErr.Error())
				}
			}
			return fmt.Errorf("combined namespaces: %w (individual: %s)", err, strings.Join(results, ", "))
		}
		return nil
	case "overlay":
		return run(command(unix.CLONE_NEWNS, false))
	default:
		return run(command(0, false))
	}
}

func probeCgroupController() error {
	raw, err := os.ReadFile("/proc/self/cgroup")
	if err != nil {
		return err
	}
	var relative string
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		if strings.HasPrefix(line, "0::") {
			relative = strings.TrimPrefix(line, "0::")
			break
		}
	}
	if relative == "" || strings.Contains(relative, "..") {
		return fmt.Errorf("kernel capability probe: current cgroup v2 path unavailable")
	}
	parent := filepath.Join("/sys/fs/cgroup", relative)
	path := filepath.Join(parent, fmt.Sprintf("choir-capability-probe-%d", os.Getpid()))
	if err := os.Mkdir(path, 0o755); err != nil {
		return fmt.Errorf("kernel capability probe: cgroup v2 controller creation: %w", err)
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("kernel capability probe: cgroup v2 cleanup: %w", err)
	}
	return nil
}

func probeOverlayMount() error {
	if err := unix.Mount("", "/", "", unix.MS_REC|unix.MS_PRIVATE, ""); err != nil {
		return err
	}
	root, err := os.MkdirTemp("", "choir-overlay-probe-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(root)
	lower, upper, work, merged := filepath.Join(root, "lower"), filepath.Join(root, "upper"), filepath.Join(root, "work"), filepath.Join(root, "merged")
	for _, dir := range []string{lower, upper, work, merged} {
		if err := os.Mkdir(dir, 0o700); err != nil {
			return err
		}
	}
	if err := os.WriteFile(filepath.Join(lower, "proof"), []byte("overlay\n"), 0o600); err != nil {
		return err
	}
	options := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", lower, upper, work)
	if err := unix.Mount("overlay", merged, "overlay", 0, options); err != nil {
		return err
	}
	defer unix.Unmount(merged, 0)
	raw, err := os.ReadFile(filepath.Join(merged, "proof"))
	if err != nil || string(raw) != "overlay\n" {
		return fmt.Errorf("overlay readback failed")
	}
	return nil
}

func probeSeccompEnforcement() error {
	if err := capsule.LoadWorkloadFilter(); err != nil {
		return err
	}
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_STREAM, 0)
	if fd >= 0 {
		unix.Close(fd)
	}
	if !errors.Is(err, unix.EPERM) {
		return fmt.Errorf("seccomp AF_INET denial returned %v", err)
	}
	return nil
}

func probeLandlockEnforcement() error {
	allowed, err := os.MkdirTemp("/tmp", "choir-landlock-probe-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(allowed)
	if err := capsule.NewWorkloadLandlock(allowed).Apply(); err != nil {
		return err
	}
	if _, err := os.ReadFile("/etc/hostname"); !errors.Is(err, os.ErrPermission) {
		return fmt.Errorf("Landlock denial returned %v", err)
	}
	return nil
}

func loadedOverlayModuleDigest(kernelRelease string) (string, error) {
	if _, err := os.Stat("/sys/module/overlay"); err != nil {
		return "", fmt.Errorf("kernel capability probe: overlay module is not loaded: %w", err)
	}
	patterns := []string{
		filepath.Join("/run/current-system/kernel-modules/lib/modules", kernelRelease, "kernel/fs/overlayfs/overlay.ko*"),
		filepath.Join("/lib/modules", kernelRelease, "kernel/fs/overlayfs/overlay.ko*"),
	}
	for _, pattern := range patterns {
		matches, _ := filepath.Glob(pattern)
		for _, match := range matches {
			digest, err := fileSHA256(match)
			if err == nil {
				return digest, nil
			}
		}
	}
	return "", fmt.Errorf("kernel capability probe: loaded overlay module artifact unavailable")
}

func readTrimmed(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("kernel capability probe: read %s: %w", path, err)
	}
	value := strings.TrimSpace(string(raw))
	if value == "" {
		return "", fmt.Errorf("kernel capability probe: empty %s", path)
	}
	return value, nil
}
