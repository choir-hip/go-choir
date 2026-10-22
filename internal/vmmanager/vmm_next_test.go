package vmmanager

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// NET-08 / RES-16, P3 of the FP8 station: a second Firecracker build is
// chosen per computer at each launch, so 1.17 (and data-drive discard) can
// be proven on one test computer while every other computer launches the
// pinned build. Failure modes guarded:
//  1. the next build reaching a computer nobody selected (an empty
//     selector selects none; next needs an explicit list);
//  2. a missing or unreadable next binary failing boots (it is refused at
//     load and every launch uses the pinned build);
//  3. a next binary whose jail directory name differs from the pinned one
//     (the jail path would not match the jailer's);
//  4. discard sent to a binary that does not know it (gated on the version
//     of the binary this computer launches, not the host's pinned one);
//  5. a snapshot taken under one build loaded by the other (the snapshot
//     records the build the VM ran, persisted across vmctl restarts);
//  6. a computer selected for discard but launching the pinned 1.16 build.

func fakeFirecrackerAt(t *testing.T, dir, banner string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "firecracker")
	if err := os.WriteFile(path, []byte("#!/bin/sh\necho '"+banner+"'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "jailer"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadConfigFromEnv_FirecrackerNext(t *testing.T) {
	root := t.TempDir()
	stable := fakeFirecrackerAt(t, filepath.Join(root, "fc-1.16.1", "bin"), "Firecracker v1.16.1")
	next := fakeFirecrackerAt(t, filepath.Join(root, "fc-1.17.0", "bin"), "Firecracker v1.17.0")
	t.Setenv("VM_FIRECRACKER_BIN", stable)

	cfg := LoadConfigFromEnv()
	if cfg.FirecrackerNext.BinPath != "" {
		t.Fatalf("next must be off by default: %+v", cfg.FirecrackerNext)
	}

	t.Setenv("VM_FIRECRACKER_NEXT_BIN", next)
	if cfg := LoadConfigFromEnv(); cfg.FirecrackerNext.BinPath != "" {
		t.Fatalf("a next binary with no selector must stay off: %+v", cfg.FirecrackerNext)
	}

	t.Setenv("VM_FIRECRACKER_NEXT_ONLY", "class:test computer-x")
	t.Setenv("VM_DATA_DISCARD", "1")
	t.Setenv("VM_DATA_DISCARD_ONLY", "class:test")
	cfg = LoadConfigFromEnv()
	n := cfg.FirecrackerNext
	if n.BinPath != next || n.JailerBinPath != filepath.Join(filepath.Dir(next), "jailer") || !reflect.DeepEqual(n.Only, []string{"class:test", "computer-x"}) {
		t.Fatalf("next = %+v", n)
	}
	if !n.DataDiscard {
		t.Fatal("a 1.17 next build with VM_DATA_DISCARD on must allow discard")
	}
	if cfg.DataDiscard {
		t.Fatal("the pinned 1.16.2 build must not allow discard")
	}
	if !reflect.DeepEqual(cfg.DataDiscardOnly, []string{"class:test"}) {
		t.Fatalf("DataDiscardOnly = %q", cfg.DataDiscardOnly)
	}

	t.Setenv("VM_FIRECRACKER_NEXT_BIN", filepath.Join(root, "missing", "firecracker"))
	if cfg := LoadConfigFromEnv(); cfg.FirecrackerNext.BinPath != "" {
		t.Fatal("a missing next binary must be refused at load")
	}

	odd := filepath.Join(root, "odd", "fc")
	if err := os.MkdirAll(filepath.Dir(odd), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(odd, []byte("#!/bin/sh\necho 'Firecracker v1.17.0'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VM_FIRECRACKER_NEXT_BIN", odd)
	if cfg := LoadConfigFromEnv(); cfg.FirecrackerNext.BinPath != "" {
		t.Fatal("a next binary with another file name must be refused (the jail dir is named after it)")
	}
}

func nextTestManager(t *testing.T, mutate func(*ManagerConfig)) *Manager {
	t.Helper()
	return balloonTestManager(t, func(c *ManagerConfig) {
		c.FirecrackerBinPath = "/nix/store/a-firecracker-1.16.2/bin/firecracker"
		c.FirecrackerNext = FirecrackerNextConfig{
			BinPath:       "/nix/store/b-firecracker-1.17.0/bin/firecracker",
			JailerBinPath: "/nix/store/b-firecracker-1.17.0/bin/jailer",
			Only:          []string{"class:test"},
			DataDiscard:   true,
		}
		c.Jailer.BinPath = "/nix/store/a-firecracker-1.16.2/bin/jailer"
		if mutate != nil {
			mutate(c)
		}
	})
}

func TestVMMForSelectsNextBuildPerComputer(t *testing.T) {
	mgr := nextTestManager(t, nil)
	test := balloonTestVMConfig()
	test.WarmnessClass = "test"
	primary := balloonTestVMConfig()
	primary.VMID = "vm-primary"
	primary.WarmnessClass = "primary"

	if b := mgr.vmmFor(test); !b.Next || b.Firecracker != mgr.cfg.FirecrackerNext.BinPath || b.Jailer != mgr.cfg.FirecrackerNext.JailerBinPath {
		t.Fatalf("test computer = %+v, want the next build", b)
	}
	if b := mgr.vmmFor(primary); b.Next || b.Firecracker != mgr.cfg.FirecrackerBinPath || b.Jailer != mgr.cfg.Jailer.BinPath {
		t.Fatalf("primary computer = %+v, want the pinned build", b)
	}
	off := nextTestManager(t, func(c *ManagerConfig) { c.FirecrackerNext = FirecrackerNextConfig{} })
	if b := off.vmmFor(test); b.Next {
		t.Fatal("no next build configured must launch the pinned build")
	}
}

func TestDataDiscardFollowsTheLaunchedBuild(t *testing.T) {
	mgr := nextTestManager(t, func(c *ManagerConfig) { c.DataDiscardOnly = []string{"class:test", "vm-primary"} })
	test := balloonTestVMConfig()
	test.WarmnessClass = "test"
	primary := balloonTestVMConfig()
	primary.VMID = "vm-primary"
	primary.WarmnessClass = "primary"

	if d := driveOf(t, mgr.buildFirecrackerConfig(test, 9001), "data"); d["discard"] != true || d["io_engine"] != "Sync" {
		t.Fatalf("selected test computer on 1.17 must get discard: %#v", d)
	}
	// Selected for discard, but it launches the pinned 1.16 build.
	if d := driveOf(t, mgr.buildFirecrackerConfig(primary, 9002), "data"); d["discard"] != nil || d["io_engine"] != nil {
		t.Fatalf("a computer on the pinned build must not get discard: %#v", d)
	}
	// On the next build, but not selected for discard.
	narrow := nextTestManager(t, func(c *ManagerConfig) { c.DataDiscardOnly = []string{"computer-other"} })
	if d := driveOf(t, narrow.buildFirecrackerConfig(test, 9001), "data"); d["discard"] != nil {
		t.Fatalf("discard must follow VM_DATA_DISCARD_ONLY: %#v", d)
	}
	// An empty discard selector keeps today's meaning: every computer whose
	// build allows it.
	all := nextTestManager(t, nil)
	if d := driveOf(t, all.buildFirecrackerConfig(test, 9001), "data"); d["discard"] != true {
		t.Fatalf("empty VM_DATA_DISCARD_ONLY must not narrow: %#v", d)
	}
}

func TestLaunchedVMMBuildIsDurable(t *testing.T) {
	mgr := nextTestManager(t, nil)
	if err := os.MkdirAll(mgr.vmStateDir("vm-a"), 0o750); err != nil {
		t.Fatal(err)
	}
	if got := mgr.launchedVMM("vm-a"); got.Firecracker != mgr.cfg.FirecrackerBinPath || got.Next {
		t.Fatalf("a VM with no record (launched before this change) = %+v, want the pinned build", got)
	}
	b := vmmBuild{Firecracker: mgr.cfg.FirecrackerNext.BinPath, Jailer: mgr.cfg.FirecrackerNext.JailerBinPath, Next: true}
	mgr.recordLaunchedVMM("vm-a", b)
	if got := mgr.launchedVMM("vm-a"); got != b {
		t.Fatalf("launchedVMM = %+v, want %+v", got, b)
	}
	// A restarted vmctl reads the record from the VM's state dir.
	restarted := NewManager(mgr.cfg)
	if got := restarted.launchedVMM("vm-a"); got != b {
		t.Fatalf("after restart launchedVMM = %+v, want %+v", got, b)
	}
	// A brand-new VM is recorded before bootVM creates its state dir.
	mgr.recordLaunchedVMM("vm-new", b)
	if got := NewManager(mgr.cfg).launchedVMM("vm-new"); got != b {
		t.Fatalf("a new VM's build was not durable: %+v", got)
	}
}

// Selection on a pinned 1.17 build with no next build
// (docs/problems/firecracker-1-17-for-every-computer-2026-10-22.md): a
// VM_DATA_DISCARD_ONLY that names one computer gives discard to that
// computer only, and every other computer's launch config is byte-identical
// to a discard-off host, so a version change alone moves no launch-config
// hash.
func TestPinnedDiscardBuildNamesOnlyTheSelectedComputer(t *testing.T) {
	pinned := fakeFirecrackerAt(t, filepath.Join(t.TempDir(), "fc-1.17.0", "bin"), "Firecracker v1.17.0")
	t.Setenv("VM_FIRECRACKER_BIN", pinned)
	t.Setenv("VM_FIRECRACKER_NEXT_BIN", "")
	t.Setenv("VM_FIRECRACKER_NEXT_ONLY", "")
	t.Setenv("VM_DATA_DISCARD", "1")
	t.Setenv("VM_DATA_DISCARD_ONLY", "computer-trial")
	loaded := LoadConfigFromEnv()
	if loaded.FirecrackerNext.BinPath != "" {
		t.Fatalf("no next build is configured: %+v", loaded.FirecrackerNext)
	}
	if !loaded.DataDiscard {
		t.Fatal("a pinned 1.17 build with VM_DATA_DISCARD on must allow discard")
	}
	if !reflect.DeepEqual(loaded.DataDiscardOnly, []string{"computer-trial"}) {
		t.Fatalf("DataDiscardOnly = %q", loaded.DataDiscardOnly)
	}

	mgr := balloonTestManager(t, func(c *ManagerConfig) {
		c.FirecrackerBinPath = pinned
		c.DataDiscard = loaded.DataDiscard
		c.DataDiscardOnly = loaded.DataDiscardOnly
	})
	// Same state dir: drive paths are in the config.
	off := balloonTestManager(t, func(c *ManagerConfig) {
		c.FirecrackerBinPath = pinned
		c.StateDir = mgr.cfg.StateDir
	})

	trial := balloonTestVMConfig()
	trial.ComputerID = "computer-trial"
	trial.WarmnessClass = "test"
	if b := mgr.vmmFor(trial); b.Next || b.Firecracker != pinned {
		t.Fatalf("the trial computer must launch the pinned build: %+v", b)
	}
	if d := driveOf(t, mgr.buildFirecrackerConfig(trial, 9001), "data"); d["discard"] != true || d["io_engine"] != "Sync" {
		t.Fatalf("the named computer must keep discard on the pinned build: %#v", d)
	}

	for _, other := range []VMConfig{
		func() VMConfig {
			c := balloonTestVMConfig()
			c.VMID = "vm-paper"
			c.ComputerID = "computer-paper"
			c.WarmnessClass = "critical_protected"
			return c
		}(),
		func() VMConfig {
			c := balloonTestVMConfig()
			c.VMID = "vm-test"
			c.ComputerID = "computer-test"
			c.WarmnessClass = "test"
			return c
		}(),
	} {
		got, err := launchConfigHash(mgr.buildFirecrackerConfig(other, 9002))
		if err != nil {
			t.Fatal(err)
		}
		want, err := launchConfigHash(off.buildFirecrackerConfig(other, 9002))
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("%s: launch config differs from a discard-off host (%s vs %s)", other.ComputerID, got, want)
		}
	}
}

// The pin change after deploy D4: D4 launches every computer on the next
// build (1.17, VM_FIRECRACKER_NEXT_ONLY=*) with VM_DATA_DISCARD_ONLY=*, over
// a pinned 1.16.2. Making 1.17 the pinned build and dropping the next build
// must not change any computer's launch config or Firecracker banner, or
// every snapshot taken under D4 would be refused a second time.
func TestPinned117KeepsTheLaunchConfigOfTheNextBuild(t *testing.T) {
	root := t.TempDir()
	old := fakeFirecrackerAt(t, filepath.Join(root, "fc-1.16.2", "bin"), "Firecracker v1.16.2")
	next := fakeFirecrackerAt(t, filepath.Join(root, "fc-1.17.0", "bin"), "Firecracker v1.17.0")

	load := func(pinned, nextBin, nextOnly string) ManagerConfig {
		t.Setenv("VM_FIRECRACKER_BIN", pinned)
		t.Setenv("VM_FIRECRACKER_NEXT_BIN", nextBin)
		t.Setenv("VM_FIRECRACKER_NEXT_ONLY", nextOnly)
		t.Setenv("VM_DATA_DISCARD", "1")
		t.Setenv("VM_DATA_DISCARD_ONLY", "*")
		return LoadConfigFromEnv()
	}
	d4Env := load(old, next, "*")
	pinEnv := load(next, "", "")
	if d4Env.FirecrackerNext.BinPath != next || !d4Env.FirecrackerNext.DataDiscard || d4Env.DataDiscard {
		t.Fatalf("D4 shape: next=%+v pinned discard=%v", d4Env.FirecrackerNext, d4Env.DataDiscard)
	}
	if pinEnv.FirecrackerNext.BinPath != "" || !pinEnv.DataDiscard {
		t.Fatalf("pinned shape: next=%+v pinned discard=%v", pinEnv.FirecrackerNext, pinEnv.DataDiscard)
	}

	d4 := balloonTestManager(t, func(c *ManagerConfig) {
		c.FirecrackerBinPath = old
		c.FirecrackerNext = d4Env.FirecrackerNext
		c.DataDiscard = d4Env.DataDiscard
		c.DataDiscardOnly = d4Env.DataDiscardOnly
	})
	pin := balloonTestManager(t, func(c *ManagerConfig) {
		c.StateDir = d4.cfg.StateDir // drive paths are in the config
		c.FirecrackerBinPath = next
		c.DataDiscard = pinEnv.DataDiscard
		c.DataDiscardOnly = pinEnv.DataDiscardOnly
	})

	for _, class := range []string{"critical_protected", "public_platform", "test", ""} {
		cfg := balloonTestVMConfig()
		cfg.VMID = "vm-" + class
		cfg.ComputerID = "computer-" + class
		cfg.WarmnessClass = class
		before, after := d4.vmmFor(cfg), pin.vmmFor(cfg)
		if !before.Next || before.Firecracker != next || after.Next || after.Firecracker != next {
			t.Fatalf("class %q: D4 build %+v, pinned build %+v; want the same 1.17 binary", class, before, after)
		}
		if b, a := firecrackerBuildVersion(before.Firecracker), firecrackerBuildVersion(after.Firecracker); b != a || a != "Firecracker v1.17.0" {
			t.Fatalf("class %q: banner %q before, %q after", class, b, a)
		}
		beforeCfg := d4.buildFirecrackerConfig(cfg, 9001)
		if d := driveOf(t, beforeCfg, "data"); d["discard"] != true || d["io_engine"] != "Sync" {
			t.Fatalf("class %q: D4 data drive %#v, want discard", class, d)
		}
		hb, err := launchConfigHash(beforeCfg)
		if err != nil {
			t.Fatal(err)
		}
		ha, err := launchConfigHash(pin.buildFirecrackerConfig(cfg, 9001))
		if err != nil {
			t.Fatal(err)
		}
		if hb != ha {
			t.Fatalf("class %q: launch config hash %s under D4, %s pinned: every D4 snapshot would be refused", class, hb, ha)
		}
	}
}
