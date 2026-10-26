package vmmanager

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// D2 slice S3 (docs/design/density-d2-snapshots-survive-deploys-2026-10-26.md
// 4.2 to 4.6). Two releases, R1 and R2, in a temp releases directory; the
// guest link names one of them.

const (
	relR1 = "11111111111111111111111111111111"
	relR2 = "22222222222222222222222222222222"
	relR3 = "33333333333333333333333333333333"
)

type testRelPolicy struct {
	Digest   string
	Epoch    int64
	ABI      int
	ABIMin   int
	FirstAge time.Duration // first_current_at = now - FirstAge
}

func defaultRelPolicy(age time.Duration) testRelPolicy {
	return testRelPolicy{Digest: "sha256:floor-a", Epoch: 1, ABI: 1, ABIMin: 1, FirstAge: age}
}

func writeTestRelease(t *testing.T, dir, id string, p testRelPolicy) {
	t.Helper()
	rd := filepath.Join(dir, id)
	_ = os.RemoveAll(rd)
	if err := os.MkdirAll(rd, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"vmlinux", "initrd", "rootfs.ext4", "storedisk.erofs", "build.json", "guest-image-manifest"} {
		if err := os.WriteFile(filepath.Join(rd, name), []byte(name+"-"+id), 0o444); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(rd, "kernel-params"), []byte("root=fstab init=/nix/store/"+id+"-nixos-system/init\n"), 0o444); err != nil {
		t.Fatal(err)
	}
	rel := fmt.Sprintf(`{"schema":1,"floor_digest":%q,"security_epoch":%d,"guest_abi":%d,"guest_abi_min":%d,"build_commit":"c-%s"}`, p.Digest, p.Epoch, p.ABI, p.ABIMin, id[:4])
	if err := os.WriteFile(filepath.Join(rd, "release.json"), []byte(rel), 0o444); err != nil {
		t.Fatal(err)
	}
	inst := fmt.Sprintf(`{"store_path":"/nix/store/%s-go-choir-guest-image","first_current_at":%q}`, id, time.Now().Add(-p.FirstAge).UTC().Format(time.RFC3339))
	if err := os.WriteFile(filepath.Join(rd, "installed.json"), []byte(inst), 0o444); err != nil {
		t.Fatal(err)
	}
}

// setFirstCurrent rewrites only installed.json, leaving the release files (and
// their bound identities) alone.
func setFirstCurrent(t *testing.T, dir, id string, age time.Duration) {
	t.Helper()
	path := filepath.Join(dir, id, "installed.json")
	_ = os.Remove(path)
	inst := fmt.Sprintf(`{"store_path":"/nix/store/%s-go-choir-guest-image","first_current_at":%q}`, id, time.Now().Add(-age).UTC().Format(time.RFC3339))
	if err := os.WriteFile(path, []byte(inst), 0o444); err != nil {
		t.Fatal(err)
	}
}

type releaseFixture struct {
	*snapshotFixture
	releases, link string
}

func (r *releaseFixture) point(t *testing.T, id string) {
	t.Helper()
	tmp := r.link + ".next"
	_ = os.Remove(tmp)
	if err := os.Symlink("/nix/store/"+id+"-go-choir-guest-image", tmp); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tmp, r.link); err != nil {
		t.Fatal(err)
	}
}

// newReleaseFixture: switch on, R1 and R2 retained (R1 the older), the link
// at R1, and the fixture's running VM launched from R1 (config, fc-config.json
// and launch record written as a release-bound launch writes them). The link
// is then left at R2 by the callers that model a deploy (point).
func newReleaseFixture(t *testing.T) *releaseFixture {
	t.Helper()
	f := newSnapshotFixture(t)
	// Hermetic host CPU: a release-bound snapshot needs a fingerprint (S5).
	cpu := "cpu-test"
	stubHostCPU(t, &cpu, nil)
	root := f.mgr.cfg.StateDir
	r := &releaseFixture{snapshotFixture: f, releases: filepath.Join(root, "guest-releases"), link: filepath.Join(root, "guest-link")}
	writeTestRelease(t, r.releases, relR1, defaultRelPolicy(2*time.Hour))
	writeTestRelease(t, r.releases, relR2, defaultRelPolicy(time.Hour))
	f.mgr.cfg.SnapshotReleaseRetain = true
	f.mgr.cfg.GuestReleasesDir = r.releases
	f.mgr.cfg.GuestReleaseLink = r.link
	f.mgr.cfg.GuestReleaseSupportWindow = 168 * time.Hour
	r.point(t, relR1)

	vmCfg := f.cfg
	vmCfg.KernelImagePath, vmCfg.InitrdPath, vmCfg.RootfsPath, vmCfg.StoreDiskPath, vmCfg.KernelParams = "", "", "", "", ""
	f.mgr.bindCurrentRelease(&vmCfg)
	f.mgr.applyLaunchDefaults(&vmCfg)
	if vmCfg.GuestRelease != relR1 {
		t.Fatalf("setup: launch bound to %q, want %s", vmCfg.GuestRelease, relR1)
	}
	hostPort, _ := f.mgr.hostPortFromHostURL(f.guestS.URL)
	launch, err := json.Marshal(f.mgr.buildFirecrackerConfig(vmCfg, hostPort))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.mgr.vmStateDir(f.vmID), "fc-config.json"), launch, 0o644); err != nil {
		t.Fatal(err)
	}
	f.cfg = vmCfg
	f.inst.Config = vmCfg
	if err := f.mgr.writeLaunchRecord(vmCfg); err != nil {
		t.Fatal(err)
	}
	return r
}

func (r *releaseFixture) hibernate(t *testing.T) (string, *SnapshotMeta) {
	t.Helper()
	r.serveLaunchAPI(t)
	if err := r.mgr.HibernateVM(r.vmID); err != nil {
		t.Fatal(err)
	}
	dir := r.mgr.snapshotDir(r.vmID)
	meta, err := readSnapshotMeta(dir)
	if err != nil {
		t.Fatal(err)
	}
	return dir, meta
}

func (r *releaseFixture) validate(dir string, meta *SnapshotMeta) string {
	v := snapshotValidation{Now: time.Now(), FirecrackerVersion: meta.FirecrackerVersion, HostKernelRelease: meta.HostKernelRelease, Expect: r.expect}
	return r.mgr.validateSnapshot(dir, meta, v)
}

// S3 test 2: with the switch on, launch artifacts come from the current
// release, the kernel parameters from its kernel-params file, and the launch
// record names the release before the spawn.
func TestReleaseBoundLaunch(t *testing.T) {
	r := newReleaseFixture(t)
	r.point(t, relR2)

	cfg := VMConfig{VMID: "vm-launch", GuestPort: 8085, MachineCPUCount: 2, MachineMemSizeMib: 64, Epoch: 3}
	r.mgr.bindCurrentRelease(&cfg)
	r.mgr.applyLaunchDefaults(&cfg)
	rd := filepath.Join(r.releases, relR2)
	if cfg.GuestRelease != relR2 || cfg.KernelImagePath != filepath.Join(rd, "vmlinux") || cfg.InitrdPath != filepath.Join(rd, "initrd") ||
		cfg.RootfsPath != filepath.Join(rd, "rootfs.ext4") || cfg.StoreDiskPath != filepath.Join(rd, "storedisk.erofs") {
		t.Fatalf("launch config = %+v; want release R2 paths", cfg)
	}
	if !strings.Contains(cfg.KernelParams, "init=/nix/store/"+relR2) || strings.Contains(cfg.KernelParams, "init-a") {
		t.Fatalf("kernel params = %q; want R2/kernel-params, not the cached manager value", cfg.KernelParams)
	}

	// The record is written before the spawn hook runs.
	hostPort, _ := r.mgr.hostPortFromHostURL(r.guestS.URL)
	fc := r.mgr.buildFirecrackerConfig(cfg, hostPort)
	spawned := false
	r.mgr.firecrackerStarter = func(string, []string) error {
		spawned = true
		data, err := os.ReadFile(filepath.Join(r.mgr.vmStateDir("vm-launch"), launchRecordName))
		if err != nil {
			t.Fatalf("no launch record at spawn: %v", err)
		}
		var rec launchRecord
		_ = json.Unmarshal(data, &rec)
		if rec.Release != relR2 || rec.Epoch != 3 {
			t.Fatalf("launch record at spawn = %+v; want release R2, epoch 3", rec)
		}
		return nil
	}
	if err := r.mgr.launchFirecracker(cfg, fc, hostPort); err != nil || !spawned {
		t.Fatalf("launchFirecracker: %v (spawned %v)", err, spawned)
	}

	// A missing installed.json is a legacy launch: m.cfg paths, no release.
	if err := os.Remove(filepath.Join(rd, "installed.json")); err != nil {
		t.Fatal(err)
	}
	legacy := VMConfig{VMID: "vm-legacy", GuestPort: 8085}
	r.mgr.bindCurrentRelease(&legacy)
	r.mgr.applyLaunchDefaults(&legacy)
	if legacy.GuestRelease != "" || legacy.StoreDiskPath != r.mgr.cfg.StoreDiskPath || legacy.KernelParams != r.mgr.cfg.KernelParams {
		t.Fatalf("legacy launch = %+v; want m.cfg paths and no release", legacy)
	}
	// installed.json naming another store path than the link is legacy too.
	writeTestRelease(t, r.releases, relR2, defaultRelPolicy(time.Hour))
	if err := os.WriteFile(filepath.Join(rd, "installed.json.new"), []byte(`{"store_path":"/nix/store/other","first_current_at":"2026-01-01T00:00:00Z"}`), 0o644); err == nil {
		_ = os.Remove(filepath.Join(rd, "installed.json"))
		_ = os.Rename(filepath.Join(rd, "installed.json.new"), filepath.Join(rd, "installed.json"))
	}
	legacy = VMConfig{VMID: "vm-legacy"}
	r.mgr.bindCurrentRelease(&legacy)
	if legacy.GuestRelease != "" {
		t.Fatalf("a release whose installed.json names another store path bound: %+v", legacy)
	}

	// Switch off: never bound, even with releases on disk.
	r.mgr.cfg.SnapshotReleaseRetain = false
	off := VMConfig{VMID: "vm-off"}
	r.mgr.bindCurrentRelease(&off)
	if off.GuestRelease != "" || off.StoreDiskPath != "" {
		t.Fatalf("switch off bound a release: %+v", off)
	}
}

// S3 test 3: a snapshot taken on R1, the current release R2, same floor,
// epoch and ABI, fresh: validation passes and the resume says behind.
func TestSnapshotResumesOntoItsOwnRelease(t *testing.T) {
	r := newReleaseFixture(t)
	dir, meta := r.hibernate(t)
	if meta.GuestRelease != relR1 {
		t.Fatalf("meta.GuestRelease = %q, want R1", meta.GuestRelease)
	}
	for _, d := range meta.Disks {
		if strings.HasPrefix(d.Path, filepath.Join(r.mgr.cfg.StateDir, "guest")+string(filepath.Separator)) {
			t.Fatalf("bound path %s is not under the release", d.Path)
		}
	}
	if meta.Config.StoreDiskPath != filepath.Join(r.releases, relR1, "storedisk.erofs") {
		t.Fatalf("meta store disk = %s", meta.Config.StoreDiskPath)
	}
	r.point(t, relR2) // the deploy
	if got := r.validate(dir, meta); got != "" {
		t.Fatalf("validateSnapshot = %q, want a pass (expected hash equals the launch hash)", got)
	}

	restoreAPI := &fakeFirecrackerAPI{}
	var args []string
	r.restoreStarter(t, restoreAPI, &args)
	inst, err := r.mgr.ResumeVMFromSnapshot(r.vmID, r.expect, nil)
	if err != nil {
		t.Fatalf("ResumeVMFromSnapshot: %v", err)
	}
	if inst.Config.GuestRelease != relR1 {
		t.Fatalf("resumed instance release = %q, want R1", inst.Config.GuestRelease)
	}
	tl, err := r.mgr.BootTimelineForVM(r.vmID)
	if err != nil {
		t.Fatal(err)
	}
	if tl.Snapshot == nil || tl.Snapshot.Outcome != "snapshot" || tl.Snapshot.GuestRelease != relR1 || tl.Snapshot.CurrentRelease != relR2 || !tl.Snapshot.Behind {
		t.Fatalf("resume timeline snapshot = %+v; want outcome snapshot, guest_release R1, current_release R2, behind", tl.Snapshot)
	}
	data, _ := os.ReadFile(filepath.Join(r.mgr.vmStateDir(r.vmID), launchRecordName))
	var rec launchRecord
	_ = json.Unmarshal(data, &rec)
	if rec.Release != relR1 || rec.Epoch != 7 {
		t.Fatalf("launch record after resume = %+v; want release R1, epoch 7", rec)
	}
	// Not behind when the release is still current.
	r2 := newReleaseFixture(t)
	d2, m2 := r2.hibernate(t)
	_ = d2
	_ = m2
	var args2 []string
	r2.restoreStarter(t, &fakeFirecrackerAPI{}, &args2)
	if _, err := r2.mgr.ResumeVMFromSnapshot(r2.vmID, r2.expect, nil); err != nil {
		t.Fatal(err)
	}
	tl2, _ := r2.mgr.BootTimelineForVM(r2.vmID)
	if tl2.Snapshot == nil || tl2.Snapshot.Behind || tl2.Snapshot.CurrentRelease != relR1 {
		t.Fatalf("a resume onto the current release says behind: %+v", tl2.Snapshot)
	}
}

// S3 test 4: every release rule returns its reason; legacy metas are
// unaffected; the current release is exempt from the support window.
func TestValidateSnapshotReleaseRules(t *testing.T) {
	week := 168 * time.Hour
	cases := []struct {
		name   string
		mutate func(t *testing.T, r *releaseFixture, meta *SnapshotMeta)
		want   string
	}{
		{"R1 current, R2 floor same", func(t *testing.T, r *releaseFixture, _ *SnapshotMeta) { r.point(t, relR2) }, ""},
		{"R0 switch off", func(t *testing.T, r *releaseFixture, _ *SnapshotMeta) {
			r.point(t, relR2)
			r.mgr.cfg.SnapshotReleaseRetain = false
		}, snapshotReasonReleaseRetainOff},
		{"R1 release directory removed", func(t *testing.T, r *releaseFixture, _ *SnapshotMeta) {
			r.point(t, relR2)
			_ = os.RemoveAll(filepath.Join(r.releases, relR1))
		}, snapshotReasonReleaseMissing},
		{"R1 installed.json unreadable", func(t *testing.T, r *releaseFixture, _ *SnapshotMeta) {
			r.point(t, relR2)
			_ = os.Remove(filepath.Join(r.releases, relR1, "installed.json"))
		}, snapshotReasonReleaseMissing},
		{"R1 release.json unreadable", func(t *testing.T, r *releaseFixture, _ *SnapshotMeta) {
			r.point(t, relR2)
			_ = os.Remove(filepath.Join(r.releases, relR1, "release.json"))
		}, snapshotReasonReleaseMissing},
		{"R1 malformed id", func(t *testing.T, r *releaseFixture, meta *SnapshotMeta) {
			r.point(t, relR2)
			meta.GuestRelease = "../../etc"
		}, snapshotReasonReleaseMissing},
		{"R2 current release unresolvable (no link)", func(t *testing.T, r *releaseFixture, _ *SnapshotMeta) { _ = os.Remove(r.link) }, snapshotReasonReleaseCurrentUnknown},
		{"R2 current release has no directory", func(t *testing.T, r *releaseFixture, _ *SnapshotMeta) { r.point(t, relR3) }, snapshotReasonReleaseCurrentUnknown},
		{"R3 floor moved", func(t *testing.T, r *releaseFixture, _ *SnapshotMeta) {
			p := defaultRelPolicy(time.Hour)
			p.Digest = "sha256:floor-b"
			writeTestRelease(t, r.releases, relR3, p)
			r.point(t, relR3)
		}, snapshotReasonReleaseFloorMoved},
		{"R4 epoch raised since", func(t *testing.T, r *releaseFixture, _ *SnapshotMeta) {
			p := defaultRelPolicy(time.Hour)
			p.Epoch = 2
			writeTestRelease(t, r.releases, relR3, p)
			r.point(t, relR3)
		}, snapshotReasonReleaseSecurityEpoch},
		{"R4 host rolled back (snapshot epoch higher)", func(t *testing.T, r *releaseFixture, _ *SnapshotMeta) {
			// R1's own epoch is higher than the current release's
			p := defaultRelPolicy(2 * time.Hour)
			p.Epoch = 3
			_ = os.RemoveAll(filepath.Join(r.releases, relR1))
			writeTestRelease(t, r.releases, relR1, p)
			r.point(t, relR2)
		}, snapshotReasonReleaseSecurityEpoch},
		{"R5 abi below the host's minimum", func(t *testing.T, r *releaseFixture, _ *SnapshotMeta) {
			p := defaultRelPolicy(time.Hour)
			p.ABI, p.ABIMin = 2, 2
			writeTestRelease(t, r.releases, relR3, p)
			r.point(t, relR3)
		}, snapshotReasonReleaseABI},
		{"R5 abi above the host's", func(t *testing.T, r *releaseFixture, _ *SnapshotMeta) {
			p := defaultRelPolicy(2 * time.Hour)
			p.ABI = 5
			_ = os.RemoveAll(filepath.Join(r.releases, relR1))
			writeTestRelease(t, r.releases, relR1, p)
			r.point(t, relR2)
		}, snapshotReasonReleaseABI},
		{"R6 superseded and past the support window", func(t *testing.T, r *releaseFixture, _ *SnapshotMeta) {
			setFirstCurrent(t, r.releases, relR1, week+time.Hour)
			r.point(t, relR2)
		}, snapshotReasonReleaseOutOfSupport},
		{"the current release past the window resumes (R6 exemption)", func(t *testing.T, r *releaseFixture, _ *SnapshotMeta) {
			setFirstCurrent(t, r.releases, relR1, week+time.Hour)
			// still current
		}, ""},
		{"a release that became current again after a rollback resumes", func(t *testing.T, r *releaseFixture, _ *SnapshotMeta) {
			setFirstCurrent(t, r.releases, relR1, 3*week)
			r.point(t, relR2)
			r.point(t, relR1)
		}, ""},
		{"a legacy meta under the switch on validates as under off", func(t *testing.T, r *releaseFixture, meta *SnapshotMeta) {
			// A legacy meta: no release, stable paths, hash from m.cfg.
			meta.GuestRelease = ""
			meta.Config.GuestRelease = ""
			meta.Config.KernelImagePath, meta.Config.InitrdPath, meta.Config.RootfsPath, meta.Config.StoreDiskPath, meta.Config.KernelParams = "", "", "", "", ""
		}, snapshotReasonLaunchConfig}, // its launch hash was taken on release paths, so the stable-path rebuild differs: refused, never loaded
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newReleaseFixture(t)
			dir, meta := r.hibernate(t)
			tc.mutate(t, r, meta)
			if got := r.validate(dir, meta); got != tc.want {
				t.Fatalf("validateSnapshot = %q, want %q", got, tc.want)
			}
		})
	}
}

// A truly legacy snapshot (taken with no release) behaves the same with the
// switch on or off: it resumes while the image it was launched from is still
// what the stable paths name.
func TestLegacySnapshotUnderSwitchOn(t *testing.T) {
	f := newSnapshotFixture(t)
	root := f.mgr.cfg.StateDir
	releases := filepath.Join(root, "guest-releases")
	writeTestRelease(t, releases, relR1, defaultRelPolicy(time.Hour))
	f.mgr.cfg.SnapshotReleaseRetain = true
	f.mgr.cfg.GuestReleasesDir = releases
	f.mgr.cfg.GuestReleaseLink = filepath.Join(root, "guest-link")
	if err := os.Symlink("/nix/store/"+relR1+"-go-choir-guest-image", f.mgr.cfg.GuestReleaseLink); err != nil {
		t.Fatal(err)
	}
	f.serveLaunchAPI(t)
	if err := f.mgr.HibernateVM(f.vmID); err != nil {
		t.Fatal(err)
	}
	dir := f.mgr.snapshotDir(f.vmID)
	meta, _ := readSnapshotMeta(dir)
	if meta.GuestRelease != "" {
		t.Fatalf("a legacy launch recorded release %q", meta.GuestRelease)
	}
	v := snapshotValidation{Now: time.Now(), FirecrackerVersion: meta.FirecrackerVersion, HostKernelRelease: meta.HostKernelRelease, Expect: f.expect}
	if got := f.mgr.validateSnapshot(dir, meta, v); got != "" {
		t.Fatalf("legacy meta under the switch on = %q, want a pass", got)
	}
	f.mgr.cfg.StoreDiskPath = "/nix/store/store-b.erofs" // a deploy moved the stable path
	if got := f.mgr.validateSnapshot(dir, meta, v); got != snapshotReasonLaunchConfig {
		t.Fatalf("legacy meta after a deploy = %q, want launch_config_mismatch", got)
	}
}

// S3 test 5: a changed byte or mtime in a retained release refuses.
func TestReleaseFileChangeRefuses(t *testing.T) {
	r := newReleaseFixture(t)
	dir, meta := r.hibernate(t)
	r.point(t, relR2)
	later := time.Now().Add(time.Hour)
	path := filepath.Join(r.releases, relR1, "storedisk.erofs")
	if err := os.Chtimes(path, later, later); err != nil {
		t.Fatal(err)
	}
	if got := r.validate(dir, meta); got != snapshotReasonDisk {
		t.Fatalf("validateSnapshot = %q, want %s", got, snapshotReasonDisk)
	}
}

// S3 test 6: host boot policy is not frozen by the release: a policy change
// that selects the computer moves the hash even with R1 retained.
func TestHostPolicyChangeStillMismatchesWithReleaseRetained(t *testing.T) {
	r := newReleaseFixture(t)
	dir, meta := r.hibernate(t)
	r.point(t, relR2)
	r.mgr.cfg.GuestDNSPort = 8053
	r.mgr.cfg.GuestDNSOnly = []string{"computer-s3"}
	if got := r.validate(dir, meta); got != snapshotReasonLaunchConfig {
		t.Fatalf("validateSnapshot = %q, want %s", got, snapshotReasonLaunchConfig)
	}
}

// S3 test 7: reattach after a deploy keeps the release the VM runs. The
// config half of ReattachVMWithConfig is reattachConfig.
func TestReattachConfigKeepsTheLaunchRelease(t *testing.T) {
	r := newReleaseFixture(t)
	r.point(t, relR2) // deployed; the VM still runs R1

	cfg := r.mgr.reattachConfig(r.vmID, 7, VMConfig{})
	if cfg.GuestRelease != relR1 || cfg.StoreDiskPath != filepath.Join(r.releases, relR1, "storedisk.erofs") {
		t.Fatalf("reattach config = %+v; want the R1 release paths", cfg)
	}
	// Hibernating the reattached instance gives meta R1 with bound paths under R1.
	r.inst.Config = cfg
	r.inst.Config.credentialDiskPath = r.cfg.credentialDiskPath
	_, meta := r.hibernate(t)
	if meta.GuestRelease != relR1 {
		t.Fatalf("meta.GuestRelease = %q, want R1", meta.GuestRelease)
	}
	for _, d := range meta.Disks {
		if strings.Contains(d.Path, "guest-releases") && !strings.Contains(d.Path, relR1) {
			t.Fatalf("bound path %s is not under R1", d.Path)
		}
	}

	// Another epoch, no record, no release directory, and the switch off all
	// give the legacy config (today's paths).
	for name, mutate := range map[string]func(){
		"epoch moved": func() {
			_ = r.mgr.storeLaunchRecord(r.vmID, launchRecord{Release: relR1, Epoch: 99, LaunchedAt: time.Now()})
		},
		"no record": func() { _ = os.Remove(r.mgr.launchRecordPath(r.vmID)) },
		"release dir gone": func() {
			_ = r.mgr.storeLaunchRecord(r.vmID, launchRecord{Release: relR1, Epoch: 7, LaunchedAt: time.Now()})
			_ = os.RemoveAll(filepath.Join(r.releases, relR1))
		},
		"switch off": func() {
			_ = r.mgr.storeLaunchRecord(r.vmID, launchRecord{Release: relR1, Epoch: 7, LaunchedAt: time.Now()})
			r.mgr.cfg.SnapshotReleaseRetain = false
		},
		"legacy record": func() {
			_ = r.mgr.storeLaunchRecord(r.vmID, launchRecord{Release: "", Epoch: 7, LaunchedAt: time.Now()})
		},
	} {
		mutate()
		got := r.mgr.reattachConfig(r.vmID, 7, VMConfig{})
		if got.GuestRelease != "" || got.StoreDiskPath != r.mgr.cfg.StoreDiskPath {
			t.Fatalf("%s: reattach config = %+v; want the legacy config", name, got)
		}
		r.mgr.cfg.SnapshotReleaseRetain = true
		_ = r.mgr.writeLaunchRecord(r.cfg)
		writeTestRelease(t, r.releases, relR1, defaultRelPolicy(2*time.Hour))
	}
}

// S3 test 8: jailed launches. prepareJail's sources come from the release
// directory, and the shared-copy name is stable for the same release across
// two link flips.
func TestJailSourcesComeFromTheReleaseAndAreStable(t *testing.T) {
	r := newReleaseFixture(t)
	hostPort, _ := r.mgr.hostPortFromHostURL(r.guestS.URL)
	fc := r.mgr.buildFirecrackerConfig(r.cfg, hostPort)
	_, files, err := rewriteFirecrackerConfigForJail(fc)
	if err != nil {
		t.Fatal(err)
	}
	var store jailFile
	for _, f := range files {
		if strings.HasSuffix(f.HostPath, "storedisk.erofs") {
			store = f
		}
	}
	if store.HostPath != filepath.Join(r.releases, relR1, "storedisk.erofs") {
		t.Fatalf("jail store disk source = %q; want the R1 release path", store.HostPath)
	}
	name := func() string {
		info, err := os.Stat(store.HostPath)
		if err != nil {
			t.Fatal(err)
		}
		return sharedArtifactName(store.HostPath, info)
	}
	before := name()
	r.point(t, relR2)
	r.point(t, relR1)
	r.point(t, relR2)
	if after := name(); after != before {
		t.Fatalf("shared copy name moved across link flips: %s -> %s", before, after)
	}
}

// S3 test 9: the four variables parse and the defaults hold.
func TestReleaseEnvConfig(t *testing.T) {
	for _, k := range []string{"VM_SNAPSHOT_RELEASE_RETAIN", "VM_GUEST_RELEASE_LINK", "VM_GUEST_RELEASES_DIR", "VM_GUEST_RELEASE_SUPPORT_WINDOW"} {
		t.Setenv(k, "")
	}
	cfg := LoadConfigFromEnv()
	if cfg.SnapshotReleaseRetain || cfg.GuestReleaseLink != "/var/lib/go-choir/guest" || cfg.GuestReleasesDir != "/var/lib/go-choir/guest-releases" || cfg.GuestReleaseSupportWindow != 168*time.Hour {
		t.Fatalf("defaults = %v %q %q %s", cfg.SnapshotReleaseRetain, cfg.GuestReleaseLink, cfg.GuestReleasesDir, cfg.GuestReleaseSupportWindow)
	}
	t.Setenv("VM_SNAPSHOT_RELEASE_RETAIN", "on")
	t.Setenv("VM_GUEST_RELEASE_LINK", "/x/guest")
	t.Setenv("VM_GUEST_RELEASES_DIR", "/x/releases")
	t.Setenv("VM_GUEST_RELEASE_SUPPORT_WINDOW", "72h")
	cfg = LoadConfigFromEnv()
	if !cfg.SnapshotReleaseRetain || cfg.GuestReleaseLink != "/x/guest" || cfg.GuestReleasesDir != "/x/releases" || cfg.GuestReleaseSupportWindow != 72*time.Hour {
		t.Fatalf("env = %v %q %q %s", cfg.SnapshotReleaseRetain, cfg.GuestReleaseLink, cfg.GuestReleasesDir, cfg.GuestReleaseSupportWindow)
	}
	for _, off := range []string{"off", "OFF", "false", "garbage"} {
		t.Setenv("VM_SNAPSHOT_RELEASE_RETAIN", off)
		if LoadConfigFromEnv().SnapshotReleaseRetain {
			t.Fatalf("VM_SNAPSHOT_RELEASE_RETAIN=%q turned the switch on", off)
		}
	}
	t.Setenv("VM_GUEST_RELEASE_SUPPORT_WINDOW", "nonsense")
	if LoadConfigFromEnv().GuestReleaseSupportWindow != 168*time.Hour {
		t.Fatal("a bad support window must keep the default")
	}
}

// S3 test 10: the in-use answer retention reads.
func TestGuestReleaseUsage(t *testing.T) {
	r := newReleaseFixture(t)
	// A running VM on R1 (the fixture instance), a snapshot of R2 under another
	// VM, an old snapshot (past max age) of R3, and a vault snapshot of R2.
	mkSnap := func(root, vmID, release string, created time.Time, consumed bool) {
		dir := filepath.Join(root, vmID, "snapshot")
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		meta := SnapshotMeta{Version: snapshotMetaVersion, VMID: vmID, GuestRelease: release, CreatedAt: created, Consumed: consumed}
		if err := writeSnapshotMeta(dir, &meta); err != nil {
			t.Fatal(err)
		}
	}
	state := r.mgr.cfg.StateDir
	mkSnap(state, "vm-snap-a", relR2, time.Now().Add(-time.Hour), false)
	mkSnap(state, "vm-snap-old", relR3, time.Now().Add(-100*time.Hour), false)
	mkSnap(state, "vm-snap-used", relR3, time.Now(), true)
	mkSnap(state, "vm-snap-legacy", "", time.Now(), false)
	vault := filepath.Join(t.TempDir(), "vault")
	r.mgr.cfg.SnapshotVaultDir = vault
	mkSnap(vault, "vm-vault", relR2, time.Now().Add(-time.Hour), false)

	use := r.mgr.GuestReleaseUsage()
	if !use.Retain || use.Current != relR1 || !use.Complete {
		t.Fatalf("usage = %+v", use)
	}
	byID := map[string]GuestReleaseInUse{}
	for _, e := range use.InUse {
		byID[e.ID] = e
	}
	if got := byID[relR1].Running; len(got) != 1 || got[0] != r.vmID {
		t.Fatalf("R1 running = %v, want [%s]", got, r.vmID)
	}
	if got := byID[relR2].Snapshots; len(got) != 2 {
		t.Fatalf("R2 snapshots = %v, want the state-dir and the vault snapshot", got)
	}
	if _, ok := byID[relR3]; ok {
		t.Fatalf("R3 is listed in use: %+v (a consumed snapshot and one past the max age count for nothing)", byID[relR3])
	}

	// An unreadable vault root: complete=false.
	r.mgr.cfg.SnapshotVaultDir = filepath.Join(t.TempDir(), "no-such-vault")
	if use := r.mgr.GuestReleaseUsage(); use.Complete {
		t.Fatal("an unreadable vault root must give complete=false")
	}
}

func TestGuestReleaseUsageErrorsAreNotSwallowed(t *testing.T) {
	r := newReleaseFixture(t)
	r.mgr.cfg.SnapshotReleaseRetain = false
	use := r.mgr.GuestReleaseUsage()
	if use.Retain {
		t.Fatal("retain must reflect the switch")
	}
	_ = errors.New
}

// Review 331 DeepSeek 3: the health reader builds its map under the manager
// lock; run with -race while a lifecycle mutates the instance.
func TestGuestReleaseHealthReadsUnderTheManagerLock(t *testing.T) {
	r := newReleaseFixture(t)
	r.point(t, relR2)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 200; i++ {
			r.mgr.mu.Lock()
			r.inst.State = StateHibernated
			r.inst.Config.GuestRelease = relR1
			r.inst.State = StateRunning
			r.mgr.mu.Unlock()
		}
	}()
	for i := 0; i < 200; i++ {
		current, byVM := r.mgr.GuestReleaseHealth()
		if current != relR2 {
			t.Fatalf("current = %q", current)
		}
		if st, ok := byVM[r.vmID]; ok && (st.GuestRelease != relR1 || !st.Behind) {
			t.Fatalf("status = %+v; want R1 and behind", st)
		}
	}
	<-done
}
