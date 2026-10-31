package vmmanager

// Layered guest image, design step 2 (docs/design/layered-guest-image-
// 2026-10-31.md 1 and 7, S1). vmctl reads both release layouts while the
// fleet stays on the single image. Failure modes first:
//
//   F1 wrong drive order: the data drive is found by position (/dev/vdb), so
//      the base drive comes after data and credential, never before.
//   F2 the base missing at launch: no half launch (the release does not load,
//      the launch is legacy, no base drive and no release id).
//   F3 the base missing at wake: the snapshot refuses (release_missing).
//   F4 the base replaced under its path: the snapshot refuses (disk).
//   F5 a single-image release changed in any way: no base drive, the same
//      drives, the same bound paths, the same launch hash inputs.
//   F6 an old-layout snapshot waking after the change still resumes.
//   F7 a layered snapshot resumes onto its OWN base after the current release
//      moved to another base.
//   F8 an old vmctl must refuse a layered release: the release directory has
//      no storedisk.erofs (the app image is named storedisk-app.erofs).
//   F9 a malformed or ambiguous layered release never loads.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	baseB1 = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	baseB2 = "cccccccccccccccccccccccccccccccc"
)

func testBasesDir(releases string) string {
	return filepath.Join(filepath.Dir(releases), "guest-bases")
}

// writeTestBase materializes a base the way node-b-guest-release-materialize
// does: guest-bases/<id>/storedisk-base.erofs.
func writeTestBase(t *testing.T, releases, baseID string) string {
	t.Helper()
	d := filepath.Join(testBasesDir(releases), baseID)
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(d, "storedisk-base.erofs")
	_ = os.Remove(p)
	if err := os.WriteFile(p, []byte("base-"+baseID), 0o444); err != nil {
		t.Fatal(err)
	}
	return p
}

// writeTestLayeredRelease rewrites a release as layered: no storedisk.erofs,
// storedisk-app.erofs and layers.json instead.
func writeTestLayeredRelease(t *testing.T, releases, id, baseID string, p testRelPolicy) {
	t.Helper()
	writeTestRelease(t, releases, id, p)
	rd := filepath.Join(releases, id)
	if err := os.Remove(filepath.Join(rd, "storedisk.erofs")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rd, "storedisk-app.erofs"), []byte("app-"+id), 0o444); err != nil {
		t.Fatal(err)
	}
	layers := fmt.Sprintf(`{"schema":1,"layout":"layered","base_id":%q,"base_store_path":"/nix/store/%s-guest-store-base.erofs"}`, baseID, baseID)
	if err := os.WriteFile(filepath.Join(rd, "layers.json"), []byte(layers), 0o444); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(testBasesDir(releases), baseID, "storedisk-base.erofs")); err != nil {
		writeTestBase(t, releases, baseID)
	}
}

func driveIDs(fc map[string]interface{}) []string {
	var ids []string
	for _, d := range fc["drives"].([]map[string]interface{}) {
		ids = append(ids, d["drive_id"].(string))
	}
	return ids
}

// newLayeredFixture: R1 layered on B1 is current; the fixture VM launched from it.
func newLayeredFixture(t *testing.T) *releaseFixture {
	t.Helper()
	f := newSnapshotFixture(t)
	cpu := "cpu-test"
	stubHostCPU(t, &cpu, nil)
	root := f.mgr.cfg.StateDir
	r := &releaseFixture{snapshotFixture: f, releases: filepath.Join(root, "guest-releases"), link: filepath.Join(root, "guest-link")}
	writeTestLayeredRelease(t, r.releases, relR1, baseB1, defaultRelPolicy(2*time.Hour))
	writeTestLayeredRelease(t, r.releases, relR2, baseB1, defaultRelPolicy(time.Hour))
	f.mgr.cfg.SnapshotReleaseRetain = true
	f.mgr.cfg.GuestReleasesDir = r.releases
	f.mgr.cfg.GuestReleaseLink = r.link
	f.mgr.cfg.GuestReleaseSupportWindow = 168 * time.Hour
	r.point(t, relR1)
	vmCfg := f.cfg
	vmCfg.KernelImagePath, vmCfg.InitrdPath, vmCfg.RootfsPath, vmCfg.StoreDiskPath, vmCfg.KernelParams = "", "", "", "", ""
	f.mgr.bindCurrentRelease(&vmCfg)
	f.mgr.applyLaunchDefaults(&vmCfg)
	if vmCfg.GuestRelease != relR1 || vmCfg.BaseDiskPath == "" {
		t.Fatalf("setup: layered launch not bound: %+v", vmCfg)
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

// F1: drive order, layered.
func TestLayeredDriveOrderBaseIsLast(t *testing.T) {
	r := newLayeredFixture(t)
	hostPort, _ := r.mgr.hostPortFromHostURL(r.guestS.URL)
	cfg := r.cfg
	rd := filepath.Join(r.releases, relR1)
	if cfg.StoreDiskPath != filepath.Join(rd, "storedisk-app.erofs") || cfg.BaseDiskPath != filepath.Join(testBasesDir(r.releases), baseB1, "storedisk-base.erofs") {
		t.Fatalf("layered launch config = store %q base %q", cfg.StoreDiskPath, cfg.BaseDiskPath)
	}
	cfg.credentialDiskPath = ""
	if got := strings.Join(driveIDs(r.mgr.buildFirecrackerConfig(cfg, hostPort)), ","); got != "store,data,base" {
		t.Fatalf("layered drives = %s; want store,data,base", got)
	}
	cfg.credentialDiskPath = filepath.Join(r.mgr.vmStateDir(cfg.VMID), "credential.img")
	fc := r.mgr.buildFirecrackerConfig(cfg, hostPort)
	if got := strings.Join(driveIDs(fc), ","); got != "store,data,credential,base" {
		t.Fatalf("layered drives with a credential disk = %s; want store,data,credential,base (data stays /dev/vdb, base last)", got)
	}
	last := fc["drives"].([]map[string]interface{})[3]
	if last["is_read_only"] != true || last["is_root_device"] != false || last["path_on_host"] != cfg.BaseDiskPath {
		t.Fatalf("base drive = %+v; want read-only, not root, at the base path", last)
	}
}

// F5: a single-image release is unchanged in every way.
func TestSingleImageReleaseIsUnchanged(t *testing.T) {
	r := newReleaseFixture(t)
	hostPort, _ := r.mgr.hostPortFromHostURL(r.guestS.URL)
	cfg := r.cfg
	rel, err := r.mgr.releaseByID(relR1)
	if err != nil || rel.BaseID != "" || rel.BaseDisk != "" || rel.Layered {
		t.Fatalf("single release loaded as %+v (%v); want no base", rel, err)
	}
	if cfg.BaseDiskPath != "" {
		t.Fatalf("single launch carries a base: %q", cfg.BaseDiskPath)
	}
	cfg.credentialDiskPath = "/cred.img"
	fc := r.mgr.buildFirecrackerConfig(cfg, hostPort)
	if got := strings.Join(driveIDs(fc), ","); got != "store,data,credential" {
		t.Fatalf("single-image drives = %s; want store,data,credential exactly as before", got)
	}
	want := []string{filepath.Join(r.mgr.vmStateDir(cfg.VMID), "data.img"), "/cred.img", cfg.RootfsPath, cfg.StoreDiskPath, cfg.KernelImagePath, cfg.InitrdPath}
	got := r.mgr.snapshotBoundPaths(cfg.VMID, cfg)
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("single-image bound paths = %v; want %v", got, want)
	}
	// A base directory on disk changes nothing for a single release.
	writeTestBase(t, r.releases, baseB1)
	if again := r.mgr.snapshotBoundPaths(cfg.VMID, cfg); strings.Join(again, "|") != strings.Join(want, "|") {
		t.Fatalf("bound paths moved with a base on disk: %v", again)
	}
}

// F2: a layered current release whose base is missing never half-launches.
func TestLayeredBaseMissingAtLaunchIsNotALayeredLaunch(t *testing.T) {
	r := newLayeredFixture(t)
	if err := os.RemoveAll(filepath.Join(testBasesDir(r.releases), baseB1)); err != nil {
		t.Fatal(err)
	}
	if _, err := r.mgr.releaseByID(relR1); err == nil {
		t.Fatal("a layered release with no base loaded")
	}
	cfg := VMConfig{VMID: "vm-nobase", GuestPort: 8085}
	r.mgr.bindCurrentRelease(&cfg)
	if cfg.GuestRelease != "" || cfg.BaseDiskPath != "" {
		t.Fatalf("a launch bound to a release whose base is gone: %+v", cfg)
	}
	hostPort, _ := r.mgr.hostPortFromHostURL(r.guestS.URL)
	r.mgr.applyLaunchDefaults(&cfg)
	for _, id := range driveIDs(r.mgr.buildFirecrackerConfig(cfg, hostPort)) {
		if id == "base" {
			t.Fatal("legacy launch grew a base drive")
		}
	}
}

// F3 and F4: the base at wake.
func TestLayeredSnapshotBaseMissingOrReplacedRefusesWake(t *testing.T) {
	for name, tc := range map[string]struct {
		mutate func(t *testing.T, r *releaseFixture)
		want   string
	}{
		"base deleted": {func(t *testing.T, r *releaseFixture) {
			if err := os.RemoveAll(filepath.Join(testBasesDir(r.releases), baseB1)); err != nil {
				t.Fatal(err)
			}
		}, snapshotReasonReleaseMissing},
		"base replaced": {func(t *testing.T, r *releaseFixture) {
			later := time.Now().Add(time.Hour)
			if err := os.Chtimes(filepath.Join(testBasesDir(r.releases), baseB1, "storedisk-base.erofs"), later, later); err != nil {
				t.Fatal(err)
			}
		}, snapshotReasonDisk},
		"layers.json now names another base": {func(t *testing.T, r *releaseFixture) {
			writeTestLayeredRelease(t, r.releases, relR1, baseB2, defaultRelPolicy(2*time.Hour))
		}, ""}, // any refusal; the reason is the bound-path identity or the launch hash
	} {
		t.Run(name, func(t *testing.T) {
			r := newLayeredFixture(t)
			dir, meta := r.hibernate(t)
			if meta.Config.BaseDiskPath == "" {
				t.Fatal("meta carries no base path")
			}
			bound := false
			for _, d := range meta.Disks {
				if d.Path == meta.Config.BaseDiskPath {
					bound = true
				}
			}
			if !bound {
				t.Fatalf("the base is not among the snapshot-bound disks: %+v", meta.Disks)
			}
			r.point(t, relR2)
			if got := r.validate(dir, meta); got != "" {
				t.Fatalf("untouched layered snapshot = %q, want a pass", got)
			}
			tc.mutate(t, r)
			got := r.validate(dir, meta)
			if tc.want != "" && got != tc.want {
				t.Fatalf("validateSnapshot = %q, want %s", got, tc.want)
			}
			if tc.want == "" && got == "" {
				t.Fatal("a snapshot whose release changed base still passed")
			}
		})
	}
}

// F7: a layered snapshot resumes onto its own base though current moved to
// another base.
func TestLayeredSnapshotResumesOntoItsOwnBase(t *testing.T) {
	r := newLayeredFixture(t)
	dir, meta := r.hibernate(t)
	writeTestLayeredRelease(t, r.releases, relR2, baseB2, defaultRelPolicy(time.Hour)) // a base moved
	r.point(t, relR2)
	if got := r.validate(dir, meta); got != "" {
		t.Fatalf("validateSnapshot = %q, want a pass onto B1", got)
	}
	var args []string
	r.restoreStarter(t, &fakeFirecrackerAPI{}, &args)
	inst, err := r.mgr.ResumeVMFromSnapshot(r.vmID, r.expect, nil)
	if err != nil {
		t.Fatalf("ResumeVMFromSnapshot: %v", err)
	}
	if inst.Config.GuestRelease != relR1 || !strings.Contains(inst.Config.BaseDiskPath, baseB1) || strings.Contains(inst.Config.BaseDiskPath, baseB2) {
		t.Fatalf("resumed config = release %s base %s; want R1 on B1", inst.Config.GuestRelease, inst.Config.BaseDiskPath)
	}
}

// F6: a snapshot taken on a single-image release (the old layout, no base
// field) resumes after a layered release became current, onto one drive.
func TestOldLayoutSnapshotWakesAfterLayeredReleaseAppears(t *testing.T) {
	r := newReleaseFixture(t)
	dir, meta := r.hibernate(t)
	raw, _ := json.Marshal(meta)
	if strings.Contains(string(raw), "BaseDiskPath") {
		t.Fatalf("a single-image snapshot meta writes the base field: %s", raw)
	}
	writeTestLayeredRelease(t, r.releases, relR2, baseB1, defaultRelPolicy(time.Hour))
	r.point(t, relR2)
	if got := r.validate(dir, meta); got != "" {
		t.Fatalf("old-layout snapshot = %q after a layered release became current, want a pass", got)
	}
	var args []string
	r.restoreStarter(t, &fakeFirecrackerAPI{}, &args)
	inst, err := r.mgr.ResumeVMFromSnapshot(r.vmID, r.expect, nil)
	if err != nil {
		t.Fatalf("ResumeVMFromSnapshot: %v", err)
	}
	if inst.Config.GuestRelease != relR1 || inst.Config.BaseDiskPath != "" {
		t.Fatalf("resumed config = %+v; want R1 with no base", inst.Config)
	}
}

// F8: an old vmctl's releaseByID requires <release>/storedisk.erofs as a
// regular file; a layered release must not have it. The materializer test in
// scripts/test-node-b-guest-release-retention.sh asserts the same on the real
// directory.
func TestLayeredReleaseHasNoStorediskErofs(t *testing.T) {
	r := newLayeredFixture(t)
	if _, err := os.Stat(filepath.Join(r.releases, relR1, "storedisk.erofs")); !os.IsNotExist(err) {
		t.Fatalf("a layered release directory has storedisk.erofs (%v): an old vmctl would half-launch it", err)
	}
	// A release carrying both layouts is ambiguous and never loads.
	if err := os.WriteFile(filepath.Join(r.releases, relR1, "storedisk.erofs"), []byte("x"), 0o444); err != nil {
		t.Fatal(err)
	}
	if _, err := r.mgr.releaseByID(relR1); err == nil {
		t.Fatal("a release with both storedisk.erofs and layers.json loaded")
	}
}

// F9.
func TestLayeredReleaseMalformedNeverLoads(t *testing.T) {
	for name, layers := range map[string]string{
		"bad json":        `{`,
		"wrong schema":    `{"schema":2,"layout":"layered","base_id":"` + baseB1 + `"}`,
		"wrong layout":    `{"schema":1,"layout":"stacked","base_id":"` + baseB1 + `"}`,
		"bad base id":     `{"schema":1,"layout":"layered","base_id":"../../etc"}`,
		"empty base id":   `{"schema":1,"layout":"layered","base_id":""}`,
		"uppercase id":    `{"schema":1,"layout":"layered","base_id":"BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB"}`,
		"base not a file": `{"schema":1,"layout":"layered","base_id":"` + baseB2 + `"}`,
	} {
		t.Run(name, func(t *testing.T) {
			r := newLayeredFixture(t)
			if name == "base not a file" {
				if err := os.MkdirAll(filepath.Join(testBasesDir(r.releases), baseB2, "storedisk-base.erofs"), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			p := filepath.Join(r.releases, relR1, "layers.json")
			_ = os.Remove(p)
			if err := os.WriteFile(p, []byte(layers), 0o444); err != nil {
				t.Fatal(err)
			}
			if rel, err := r.mgr.releaseByID(relR1); err == nil {
				t.Fatalf("malformed layers.json loaded: %+v", rel)
			}
		})
	}
}

// A base symlink is not a base (a swapped link must not redirect a wake).
func TestLayeredBaseSymlinkRefused(t *testing.T) {
	r := newLayeredFixture(t)
	p := filepath.Join(testBasesDir(r.releases), baseB1, "storedisk-base.erofs")
	other := filepath.Join(t.TempDir(), "other")
	if err := os.WriteFile(other, []byte("x"), 0o444); err != nil {
		t.Fatal(err)
	}
	_ = os.Remove(p)
	if err := os.Symlink(other, p); err != nil {
		t.Fatal(err)
	}
	if _, err := r.mgr.releaseByID(relR1); err == nil {
		t.Fatal("a symlinked base loaded")
	}
}

// A refresh or wake rebuild never carries a base forward.
func TestRefreshClearsBase(t *testing.T) {
	cfg := refreshConfigForCurrentDeploy(VMConfig{StoreDiskPath: "/a", BaseDiskPath: "/b"}, ManagerConfig{StoreDiskPath: "/x"})
	if cfg.BaseDiskPath != "" {
		t.Fatalf("refresh kept the base: %+v", cfg)
	}
	cfg = refreshConfigForCurrentDeploy(VMConfig{StoreDiskPath: "/a", BaseDiskPath: "/b"}, ManagerConfig{})
	if cfg.BaseDiskPath != "" {
		t.Fatalf("refresh without a store disk kept the base: %+v", cfg)
	}
}

// Usage: a running or hibernated layered release is reported by release id,
// and the base id it names is reported so retention can keep the base.
func TestGuestReleaseUsageNamesBases(t *testing.T) {
	r := newLayeredFixture(t)
	r.hibernate(t)
	use := r.mgr.GuestReleaseUsage()
	found := false
	for _, e := range use.InUse {
		if e.ID == relR1 {
			found = true
			if e.BaseID != baseB1 {
				t.Fatalf("in_use entry %+v; want base_id %s", e, baseB1)
			}
		}
	}
	if !found {
		t.Fatalf("usage lacks R1: %+v", use)
	}
}
