package vmmanager

// The layered ring, design step 4 (docs/design/layered-guest-image-
// 2026-10-31.md S2). Failure modes first:
//
//   G1 a computer outside the ring never gets the layered release, whether the
//      ring is empty, names others, or the layered link exists.
//   G2 a ring computer cold boots onto the layered release (base drive last).
//   G3 a ring computer whose layered release is not materialized falls back to
//      the single image rather than failing.
//   G4 a ring computer whose base is missing falls back the same way.
//   G5 removing the computer from the selector returns it to the single
//      image at its next cold boot, and a layered snapshot then refuses wake
//      (the floor moved) instead of resuming across layouts.
//   G6 a layered snapshot of a ring computer wakes onto its own layered
//      release while the computer stays in the ring.
//   G7 the selector takes ComputerIDs only (no class:, no "*"), and an empty
//      value selects nothing.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const ringComputer = "computer-59c902c5d46caecc0a180a8557164e72"

// newRingFixture: R1 single is the fleet's current; R2 layered (its own floor
// digest, as the layered initrd moves it) is the layered link's target; the
// ring names ringComputer, whose fixture VM launched from the layered release.
func newRingFixture(t *testing.T, ring []string) *releaseFixture {
	t.Helper()
	f := newSnapshotFixture(t)
	cpu := "cpu-test"
	stubHostCPU(t, &cpu, nil)
	root := f.mgr.cfg.StateDir
	r := &releaseFixture{snapshotFixture: f, releases: filepath.Join(root, "guest-releases"), link: filepath.Join(root, "guest-link")}
	writeTestRelease(t, r.releases, relR1, defaultRelPolicy(2*time.Hour))
	lp := defaultRelPolicy(time.Hour)
	lp.Digest = "sha256:floor-layered"
	writeTestLayeredRelease(t, r.releases, relR2, baseB1, lp)
	f.mgr.cfg.SnapshotReleaseRetain = true
	f.mgr.cfg.GuestReleasesDir = r.releases
	f.mgr.cfg.GuestReleaseLink = r.link
	f.mgr.cfg.GuestReleaseSupportWindow = 168 * time.Hour
	f.mgr.cfg.GuestLayeredLink = filepath.Join(root, "guest-layered-link")
	f.mgr.cfg.GuestLayeredComputers = ring
	r.point(t, relR1)
	r.pointLayered(t, relR2)

	vmCfg := f.cfg
	vmCfg.ComputerID = ringComputer
	f.expect.ComputerID = ringComputer
	vmCfg.KernelImagePath, vmCfg.InitrdPath, vmCfg.RootfsPath, vmCfg.StoreDiskPath, vmCfg.KernelParams = "", "", "", "", ""
	f.mgr.bindCurrentRelease(&vmCfg)
	f.mgr.applyLaunchDefaults(&vmCfg)
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

func (r *releaseFixture) pointLayered(t *testing.T, id string) {
	t.Helper()
	link := r.mgr.cfg.GuestLayeredLink
	_ = os.Remove(link)
	if err := os.Symlink("/nix/store/"+id+"-go-choir-guest-image", link); err != nil {
		t.Fatal(err)
	}
}

func bindFor(m *Manager, computerID string) VMConfig {
	cfg := VMConfig{VMID: "vm-" + computerID, ComputerID: computerID, GuestPort: 8085}
	m.bindCurrentRelease(&cfg)
	return cfg
}

// G1
func TestLayeredRingNonRingComputerNeverGetsLayered(t *testing.T) {
	for name, ring := range map[string][]string{
		"ring names another computer": {"computer-other"},
		"ring empty":                  nil,
	} {
		t.Run(name, func(t *testing.T) {
			r := newRingFixture(t, ring)
			for _, id := range []string{ringComputer, "computer-x", ""} {
				cfg := bindFor(r.mgr, id)
				if cfg.GuestRelease != relR1 || cfg.BaseDiskPath != "" || cfg.StoreDiskPath != filepath.Join(r.releases, relR1, "storedisk.erofs") {
					t.Fatalf("computer %q: launch = release %q base %q store %q; want the single image %s", id, cfg.GuestRelease, cfg.BaseDiskPath, cfg.StoreDiskPath, relR1)
				}
			}
		})
	}
	t.Run("no layered link configured", func(t *testing.T) {
		r := newRingFixture(t, []string{ringComputer})
		r.mgr.cfg.GuestLayeredLink = ""
		if cfg := bindFor(r.mgr, ringComputer); cfg.GuestRelease != relR1 || cfg.BaseDiskPath != "" {
			t.Fatalf("launch = %+v; want single", cfg)
		}
	})
}

// G2
func TestLayeredRingComputerColdBootsLayered(t *testing.T) {
	r := newRingFixture(t, []string{ringComputer})
	cfg := bindFor(r.mgr, ringComputer)
	if cfg.GuestRelease != relR2 || cfg.StoreDiskPath != filepath.Join(r.releases, relR2, "storedisk-app.erofs") ||
		cfg.BaseDiskPath != filepath.Join(testBasesDir(r.releases), baseB1, "storedisk-base.erofs") {
		t.Fatalf("ring launch = release %q store %q base %q; want layered %s", cfg.GuestRelease, cfg.StoreDiskPath, cfg.BaseDiskPath, relR2)
	}
	r.mgr.applyLaunchDefaults(&cfg)
	hostPort, _ := r.mgr.hostPortFromHostURL(r.guestS.URL)
	if got := strings.Join(driveIDs(r.mgr.buildFirecrackerConfig(cfg, hostPort)), ","); got != "store,data,base" {
		t.Fatalf("ring drives = %s; want store,data,base", got)
	}
	// another computer, same manager, same moment: single.
	if o := bindFor(r.mgr, "computer-x"); o.GuestRelease == relR2 {
		t.Fatal("a non-ring computer launched the layered release")
	}
	// the launch record names the layered link's target.
	rec, ok := r.mgr.readLaunchRecord(r.vmID, r.cfg.Epoch)
	if !ok || rec.Release != relR2 || rec.ImageTarget != "/nix/store/"+relR2+"-go-choir-guest-image" {
		t.Fatalf("launch record = %+v ok=%v; want layered release and target", rec, ok)
	}
}

// G3 and G4
func TestLayeredRingFallsBackToSingleWhenLayeredUnavailable(t *testing.T) {
	for name, mutate := range map[string]func(t *testing.T, r *releaseFixture){
		"layered release not materialized": func(t *testing.T, r *releaseFixture) {
			if err := os.RemoveAll(filepath.Join(r.releases, relR2)); err != nil {
				t.Fatal(err)
			}
		},
		"base missing": func(t *testing.T, r *releaseFixture) {
			if err := os.RemoveAll(filepath.Join(testBasesDir(r.releases), baseB1)); err != nil {
				t.Fatal(err)
			}
		},
		"layered link dangling": func(t *testing.T, r *releaseFixture) {
			_ = os.Remove(r.mgr.cfg.GuestLayeredLink)
		},
		"layered link names a single release": func(t *testing.T, r *releaseFixture) {
			r.pointLayered(t, relR1)
		},
	} {
		t.Run(name, func(t *testing.T) {
			r := newRingFixture(t, []string{ringComputer})
			mutate(t, r)
			cfg := bindFor(r.mgr, ringComputer)
			if cfg.BaseDiskPath != "" || strings.Contains(cfg.StoreDiskPath, "storedisk-app") {
				t.Fatalf("fallback launch is layered: %+v", cfg)
			}
			if cfg.GuestRelease != relR1 || cfg.StoreDiskPath != filepath.Join(r.releases, relR1, "storedisk.erofs") {
				t.Fatalf("fallback launch = release %q store %q; want the single image %s", cfg.GuestRelease, cfg.StoreDiskPath, relR1)
			}
		})
	}
}

// G5 and G6
func TestLayeredRingSnapshotFollowsTheRing(t *testing.T) {
	r := newRingFixture(t, []string{ringComputer})
	dir, meta := r.hibernate(t)
	if meta.GuestRelease != relR2 || meta.Config.BaseDiskPath == "" {
		t.Fatalf("snapshot release %q base %q; want layered %s", meta.GuestRelease, meta.Config.BaseDiskPath, relR2)
	}
	if got := r.validate(dir, meta); got != "" {
		t.Fatalf("ring computer's layered snapshot = %q; want a pass (G6)", got)
	}
	// G5: leave the ring. The next cold boot is single; the layered snapshot
	// refuses rather than resuming across layouts.
	r.mgr.cfg.GuestLayeredComputers = nil
	if cfg := bindFor(r.mgr, ringComputer); cfg.GuestRelease != relR1 || cfg.BaseDiskPath != "" {
		t.Fatalf("after leaving the ring: %+v; want single", cfg)
	}
	if got := r.validate(dir, meta); got != snapshotReasonReleaseFloorMoved {
		t.Fatalf("layered snapshot after leaving the ring = %q; want %s", got, snapshotReasonReleaseFloorMoved)
	}
}

// G7
func TestLayeredRingSelectorTakesComputerIDsOnly(t *testing.T) {
	got := parseLayeredComputers("computer-a, class:test * computer-b\ncomputer-c")
	if strings.Join(got, "|") != "computer-a|computer-b|computer-c" {
		t.Fatalf("parsed = %v; want only plain ComputerIDs", got)
	}
	if len(parseLayeredComputers("")) != 0 || len(parseLayeredComputers("  \n")) != 0 {
		t.Fatal("an empty selector selected something")
	}
	m := &Manager{cfg: ManagerConfig{GuestLayeredLink: "/x", GuestLayeredComputers: []string{"computer-a"}}}
	if !m.layeredSelected("computer-a") || m.layeredSelected("computer-ab") || m.layeredSelected("") || m.layeredSelected("class:test") {
		t.Fatal("layeredSelected is not an exact ComputerID match")
	}
}
