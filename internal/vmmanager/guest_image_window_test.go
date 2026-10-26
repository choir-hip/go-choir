package vmmanager

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// D2 slice S1b (docs/design/density-d2-snapshots-survive-deploys-2026-10-26.md
// 4.4, H1 and H6; docs/problems/guest-image-switch-window-kernel-params-and-
// bound-identity-2026-10-26.md). These tests run with the D2 switch off: no
// release directories exist.

// H1, S1b test 1: kernel parameters are read at each launch, so a launch
// after the NixOS switch repointed the guest link (and before vmctl
// restarted) does not pair the new kernel and store disk with the old init=
// path.
func TestKernelParamsReadAtEachLaunch(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "kernel-params")
	if err := os.WriteFile(file, []byte("root=fstab init=/nix/store/init-a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := DefaultManagerConfig()
	cfg.StateDir = dir
	cfg.KernelImagePath = "/guest/vmlinux"
	cfg.StoreDiskPath = "/guest/storedisk.erofs"
	cfg.KernelParams = "root=fstab init=/nix/store/init-cached"
	cfg.KernelParamsFile = file
	mgr := NewManager(cfg)

	launch := func() (VMConfig, string) {
		vmCfg := VMConfig{VMID: "vm-h1", GuestPort: 8085, MachineCPUCount: 2, MachineMemSizeMib: 64}
		mgr.applyLaunchDefaults(&vmCfg)
		return vmCfg, bootArgsOf(t, mgr.buildFirecrackerConfig(vmCfg, 9001))
	}

	vmCfg, args := launch()
	if vmCfg.KernelParams != "root=fstab init=/nix/store/init-a" || !strings.HasPrefix(args, "root=fstab init=/nix/store/init-a ") {
		t.Fatalf("launch 1 kernel params = %q, boot args %q; want the file's contents, trimmed", vmCfg.KernelParams, args)
	}

	// The switch lands: the file now names the new image. No vmctl restart.
	if err := os.WriteFile(file, []byte("root=fstab init=/nix/store/init-b"), 0o644); err != nil {
		t.Fatal(err)
	}
	vmCfg, args = launch()
	if vmCfg.KernelParams != "root=fstab init=/nix/store/init-b" || !strings.Contains(args, "init=/nix/store/init-b") || strings.Contains(args, "init-a") {
		t.Fatalf("launch 2 kernel params = %q, boot args %q; want the new contents without a restart", vmCfg.KernelParams, args)
	}

	// A launch that arrives with blank params (refreshConfigForCurrentDeploy)
	// is filled from the file too, at the boot-args step as well.
	blank := VMConfig{VMID: "vm-h1", GuestPort: 8085, MachineCPUCount: 2, MachineMemSizeMib: 64, StoreDiskPath: "/guest/storedisk.erofs"}
	if args := bootArgsOf(t, mgr.buildFirecrackerConfig(blank, 9001)); !strings.Contains(args, "init=/nix/store/init-b") {
		t.Fatalf("boot args for blank params = %q; want the file's contents", args)
	}

	// Unreadable or empty file: the cached value.
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	vmCfg, args = launch()
	if vmCfg.KernelParams != "root=fstab init=/nix/store/init-cached" || !strings.Contains(args, "init-cached") {
		t.Fatalf("unreadable file: kernel params = %q, boot args %q; want the cached value", vmCfg.KernelParams, args)
	}
	if err := os.WriteFile(file, []byte(" \n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if vmCfg, _ = launch(); vmCfg.KernelParams != "root=fstab init=/nix/store/init-cached" {
		t.Fatalf("empty file: kernel params = %q; want the cached value", vmCfg.KernelParams)
	}

	// No file configured: today's behaviour.
	mgr.cfg.KernelParamsFile = ""
	if vmCfg, _ = launch(); vmCfg.KernelParams != "root=fstab init=/nix/store/init-cached" {
		t.Fatalf("no file configured: kernel params = %q; want the cached value", vmCfg.KernelParams)
	}
}

func TestWindowEnvConfig(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "kernel-params")
	if err := os.WriteFile(file, []byte("root=fstab init=/x"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VM_KERNEL_PARAMS_FILE", file)
	t.Setenv("VM_GUEST_RELEASE_LINK", "")
	cfg := LoadConfigFromEnv()
	if cfg.KernelParamsFile != file || cfg.KernelParams != "root=fstab init=/x" {
		t.Fatalf("kernel params file = %q, params = %q", cfg.KernelParamsFile, cfg.KernelParams)
	}
	if cfg.GuestReleaseLink != "/var/lib/go-choir/guest" {
		t.Fatalf("default guest release link = %q", cfg.GuestReleaseLink)
	}
	t.Setenv("VM_GUEST_RELEASE_LINK", "/run/guest-link")
	if cfg := LoadConfigFromEnv(); cfg.GuestReleaseLink != "/run/guest-link" {
		t.Fatalf("guest release link = %q", cfg.GuestReleaseLink)
	}
}

// windowFixture is the snapshot fixture with a stable guest link, as on Node
// B: link -> image A at launch.
type windowFixture struct {
	*snapshotFixture
	link, imageA, imageB string
}

func newWindowFixture(t *testing.T) *windowFixture {
	t.Helper()
	f := newSnapshotFixture(t)
	root := filepath.Join(f.mgr.cfg.StateDir, "images")
	w := &windowFixture{snapshotFixture: f, link: filepath.Join(root, "guest"), imageA: filepath.Join(root, "image-a"), imageB: filepath.Join(root, "image-b")}
	for _, d := range []string{w.imageA, w.imageB} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(w.imageA, w.link); err != nil {
		t.Fatal(err)
	}
	f.mgr.cfg.GuestReleaseLink = w.link
	return w
}

func (w *windowFixture) repoint(t *testing.T, to string) {
	t.Helper()
	// Atomic, as the activation script does (mv -Tf).
	tmp := w.link + ".next"
	if err := os.Symlink(to, tmp); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tmp, w.link); err != nil {
		t.Fatal(err)
	}
}

func (w *windowFixture) launchRecord(t *testing.T) launchRecord {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(w.mgr.vmStateDir(w.vmID), launchRecordName))
	if err != nil {
		t.Fatalf("launch record: %v", err)
	}
	var rec launchRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		t.Fatal(err)
	}
	return rec
}

// Every launch writes the launch record before the spawn, with the link's
// target read once, switch on or off. Direct and jailed launches share it.
func TestLaunchWritesLaunchRecordBeforeSpawn(t *testing.T) {
	w := newWindowFixture(t)
	hostPort, _ := w.mgr.hostPortFromHostURL(w.guestS.URL)
	fc := w.mgr.buildFirecrackerConfig(w.cfg, hostPort)
	spawned := false
	wantTarget := w.imageA
	w.mgr.firecrackerStarter = func(string, []string) error {
		spawned = true
		rec := w.launchRecord(t)
		if rec.ImageTarget != wantTarget || rec.Epoch != 7 || rec.Release != "" || rec.LaunchedAt.IsZero() {
			t.Fatalf("launch record at spawn = %+v; want image_target %s, epoch 7, legacy release", rec, wantTarget)
		}
		return nil
	}
	if err := w.mgr.launchFirecracker(w.cfg, fc, hostPort); err != nil {
		t.Fatalf("launchFirecracker: %v", err)
	}
	if !spawned {
		t.Fatal("the fake spawn never ran")
	}

	// A second launch after the link moved rewrites the record.
	w.repoint(t, w.imageB)
	wantTarget = w.imageB
	if err := w.mgr.launchFirecracker(w.cfg, fc, hostPort); err != nil {
		t.Fatal(err)
	}
	if rec := w.launchRecord(t); rec.ImageTarget != w.imageB {
		t.Fatalf("launch record after relaunch = %+v; want image_target %s", rec, w.imageB)
	}

	// A configured link that cannot be read at launch records a sentinel that
	// never equals a real target, so a snapshot of that guest refuses at wake
	// (review 318 DeepSeek 6) instead of turning the guard off.
	w.mgr.cfg.GuestReleaseLink = filepath.Join(w.mgr.cfg.StateDir, "no-such-link")
	wantTarget = launchTargetUnreadable
	if err := w.mgr.launchFirecracker(w.cfg, fc, hostPort); err != nil {
		t.Fatal(err)
	}
	if rec := w.launchRecord(t); rec.ImageTarget != launchTargetUnreadable {
		t.Fatalf("launch record with an unreadable link = %+v; want the unreadable sentinel", rec)
	}
}

// H6, S1b test 2: launch with the link at A, the switch repoints it to B,
// hibernate, wake: refused, cold boot. Without the repoint it resumes.
func TestSnapshotRefusedWhenImageMovedSinceLaunch(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(t *testing.T, w *windowFixture)
		want   string
	}{
		{"no repoint resumes", func(*testing.T, *windowFixture) {}, ""},
		{"repointed before the hibernate", func(t *testing.T, w *windowFixture) { w.repoint(t, w.imageB) }, snapshotReasonImageMoved},
		{"link unreadable at wake (review 313 F4)", func(t *testing.T, w *windowFixture) {
			if err := os.Remove(w.link); err != nil {
				t.Fatal(err)
			}
		}, snapshotReasonImageMoved},
		{"link replaced by a directory at wake", func(t *testing.T, w *windowFixture) {
			if err := os.Remove(w.link); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(w.link, 0o755); err != nil {
				t.Fatal(err)
			}
		}, snapshotReasonImageMoved},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := newWindowFixture(t)
			if err := w.mgr.writeLaunchRecord(w.cfg); err != nil {
				t.Fatal(err)
			}
			if tc.want != "" && strings.Contains(tc.name, "before the hibernate") {
				// The switch lands between launch and hibernate: the bound
				// path (the stable link) now names the new image, so every
				// other check passes. Only the launch record knows.
				tc.mutate(t, w)
			}
			w.serveLaunchAPI(t)
			if err := w.mgr.HibernateVM(w.vmID); err != nil {
				t.Fatal(err)
			}
			dir := w.mgr.snapshotDir(w.vmID)
			meta, err := readSnapshotMeta(dir)
			if err != nil {
				t.Fatal(err)
			}
			if meta.LaunchImageTarget != w.imageA {
				t.Fatalf("meta.LaunchImageTarget = %q, want %q (the launch's target, not the link now)", meta.LaunchImageTarget, w.imageA)
			}
			if !strings.Contains(tc.name, "before the hibernate") {
				tc.mutate(t, w)
			}
			v := snapshotValidation{Now: time.Now(), FirecrackerVersion: meta.FirecrackerVersion, HostKernelRelease: meta.HostKernelRelease, Expect: w.expect}
			if got := w.mgr.validateSnapshot(dir, meta, v); got != tc.want {
				t.Fatalf("validateSnapshot = %q, want %q", got, tc.want)
			}
		})
	}
}

// The refusal end to end: the wake falls back cold and names the reason.
func TestResumeAfterImageMovedFallsBackCold(t *testing.T) {
	w := newWindowFixture(t)
	if err := w.mgr.writeLaunchRecord(w.cfg); err != nil {
		t.Fatal(err)
	}
	w.serveLaunchAPI(t)
	if err := w.mgr.HibernateVM(w.vmID); err != nil {
		t.Fatal(err)
	}
	w.repoint(t, w.imageB)
	started := false
	w.mgr.firecrackerStarter = func(string, []string) error { started = true; return nil }
	_, err := w.mgr.ResumeVMFromSnapshot(w.vmID, w.expect, nil)
	var fb *SnapshotFallbackError
	if !errors.As(err, &fb) || fb.Reason != snapshotReasonImageMoved {
		t.Fatalf("resume = %v, want an %s fallback", err, snapshotReasonImageMoved)
	}
	if started {
		t.Fatal("a refused snapshot must never start Firecracker")
	}
	if _, err := os.Stat(w.mgr.snapshotDir(w.vmID)); !os.IsNotExist(err) {
		t.Fatal("a refused snapshot is discarded")
	}
}

// A resume keeps the original launch target: the next hibernate of the
// resumed computer must carry it, so the window is guarded across cycles.
func TestResumeRewritesLaunchRecordWithOriginalTarget(t *testing.T) {
	w := newWindowFixture(t)
	if err := w.mgr.writeLaunchRecord(w.cfg); err != nil {
		t.Fatal(err)
	}
	w.serveLaunchAPI(t)
	if err := w.mgr.HibernateVM(w.vmID); err != nil {
		t.Fatal(err)
	}
	// A stale record, as after a vmctl restart that lost it.
	if err := os.Remove(filepath.Join(w.mgr.vmStateDir(w.vmID), launchRecordName)); err != nil {
		t.Fatal(err)
	}
	restoreAPI := &fakeFirecrackerAPI{}
	var restoreArgs []string
	w.restoreStarter(t, restoreAPI, &restoreArgs)
	if _, err := w.mgr.ResumeVMFromSnapshot(w.vmID, w.expect, nil); err != nil {
		t.Fatalf("ResumeVMFromSnapshot: %v", err)
	}
	if rec := w.launchRecord(t); rec.ImageTarget != w.imageA || rec.Epoch != 7 {
		t.Fatalf("launch record after resume = %+v; want image_target %s, epoch 7", rec, w.imageA)
	}
}

// S1b test 3: a meta without launch_image_target validates exactly as today,
// even when the link has moved (nothing recorded, nothing to compare).
func TestSnapshotWithoutLaunchTargetValidatesAsToday(t *testing.T) {
	w := newWindowFixture(t)
	// No launch record: the VM was launched by a vmctl before this repair.
	w.serveLaunchAPI(t)
	if err := w.mgr.HibernateVM(w.vmID); err != nil {
		t.Fatal(err)
	}
	dir := w.mgr.snapshotDir(w.vmID)
	meta, err := readSnapshotMeta(dir)
	if err != nil {
		t.Fatal(err)
	}
	if meta.LaunchImageTarget != "" {
		t.Fatalf("meta.LaunchImageTarget = %q without a launch record", meta.LaunchImageTarget)
	}
	raw, _ := json.Marshal(meta)
	if strings.Contains(string(raw), "launch_image_target") {
		t.Fatalf("an empty launch target must not be serialized (old vmctl compatibility): %s", raw)
	}
	w.repoint(t, w.imageB)
	v := snapshotValidation{Now: time.Now(), FirecrackerVersion: meta.FirecrackerVersion, HostKernelRelease: meta.HostKernelRelease, Expect: w.expect}
	if got := w.mgr.validateSnapshot(dir, meta, v); got != "" {
		t.Fatalf("validateSnapshot = %q, want a pass: a meta without a launch target validates as today", got)
	}

	// A stale record of another realization is not trusted either.
	if err := w.mgr.writeLaunchRecord(VMConfig{VMID: w.vmID, Epoch: 99}); err != nil {
		t.Fatal(err)
	}
	if rec, ok := w.mgr.readLaunchRecord(w.vmID, 7); ok {
		t.Fatalf("a record of epoch 99 was read for epoch 7: %+v", rec)
	}
}

// Review 318 DeepSeek 2 (H1 on the other cold boots): a cold ResumeVM and a
// RecoverVM rebuild from the stored config, whose kernel parameters were
// filled at the original launch. After a NixOS switch (and before the vmctl
// restart) they must read the new file, as a fresh boot and a refresh do.
func TestColdResumeAndRecoverReadKernelParamsAtLaunch(t *testing.T) {
	for _, tc := range []struct {
		name string
		boot func(f *snapshotFixture) error
	}{
		{"ResumeVM", func(f *snapshotFixture) error {
			f.mgr.mu.Lock()
			f.inst.State = StateHibernated
			f.mgr.mu.Unlock()
			_, err := f.mgr.ResumeVM(f.vmID)
			return err
		}},
		// bootVM needs resize2fs; the recovery's config is what is under test.
		{"RecoverVMWithConfig", func(f *snapshotFixture) error {
			cfg := f.mgr.recoverConfig(f.inst.Config, VMConfig{}, f.vmID)
			hostPort, _ := f.mgr.hostPortFromHostURL(f.guestS.URL)
			raw, err := json.Marshal(f.mgr.buildFirecrackerConfig(cfg, hostPort))
			if err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(f.mgr.vmStateDir(f.vmID), "fc-config.json"), raw, 0o644)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newSnapshotFixture(t)
			file := filepath.Join(f.mgr.cfg.StateDir, "kernel-params")
			f.mgr.cfg.KernelParamsFile = file
			f.mgr.cfg.BootReadyTimeout = 300 * time.Millisecond
			// The stored config carries the parameters read at its launch.
			f.inst.Config.KernelParams = "root=fstab init=/nix/store/init-a"
			if err := os.WriteFile(file, []byte("root=fstab init=/nix/store/init-b"), 0o644); err != nil {
				t.Fatal(err)
			}
			f.mgr.firecrackerStarter = func(string, []string) error { return nil }
			if err := tc.boot(f); err != nil {
				t.Logf("boot returned: %v", err)
			}
			raw, err := os.ReadFile(filepath.Join(f.mgr.vmStateDir(f.vmID), "fc-config.json"))
			if err != nil {
				t.Fatalf("no launch config written: %v", err)
			}
			var fc map[string]interface{}
			if err := json.Unmarshal(raw, &fc); err != nil {
				t.Fatal(err)
			}
			args := bootArgsOf(t, fc)
			if !strings.Contains(args, "init=/nix/store/init-b") || strings.Contains(args, "init-a") {
				t.Fatalf("%s booted with %q; want the kernel parameters read at launch (init-b)", tc.name, args)
			}
		})
	}
}

// Review 318 DeepSeek 6: a launch whose record cannot be written spawns nothing.
func TestLaunchRecordWriteFailureStopsTheLaunch(t *testing.T) {
	w := newWindowFixture(t)
	hostPort, _ := w.mgr.hostPortFromHostURL(w.guestS.URL)
	fc := w.mgr.buildFirecrackerConfig(w.cfg, hostPort)
	// A directory where the record's temp file goes: the write fails.
	if err := os.MkdirAll(w.mgr.launchRecordPath(w.vmID)+".tmp", 0o755); err != nil {
		t.Fatal(err)
	}
	spawned := false
	w.mgr.firecrackerStarter = func(string, []string) error { spawned = true; return nil }
	if err := w.mgr.launchFirecracker(w.cfg, fc, hostPort); err == nil || spawned {
		t.Fatalf("launchFirecracker = %v, spawned %v; want an error and no spawn", err, spawned)
	}
}

// Review 318 DeepSeek 4: a resume whose launch record cannot be rewritten
// falls back to a cold boot and does not consume the snapshot.
func TestResumeFallsBackWhenTheLaunchRecordCannotBeWritten(t *testing.T) {
	w := newWindowFixture(t)
	if err := w.mgr.writeLaunchRecord(w.cfg); err != nil {
		t.Fatal(err)
	}
	w.serveLaunchAPI(t)
	if err := w.mgr.HibernateVM(w.vmID); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(w.mgr.launchRecordPath(w.vmID)+".tmp", 0o755); err != nil {
		t.Fatal(err)
	}
	started := false
	w.mgr.firecrackerStarter = func(string, []string) error { started = true; return nil }
	_, err := w.mgr.ResumeVMFromSnapshot(w.vmID, w.expect, nil)
	var fb *SnapshotFallbackError
	if !errors.As(err, &fb) || fb.Reason != snapshotReasonLaunchRecord || started {
		t.Fatalf("resume = %v (started %v); want a %s fallback and no spawn", err, started, snapshotReasonLaunchRecord)
	}
}
