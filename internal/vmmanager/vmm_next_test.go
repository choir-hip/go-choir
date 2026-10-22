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
