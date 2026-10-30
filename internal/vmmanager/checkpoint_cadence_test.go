package vmmanager

import (
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"
)

// The automatic checkpoint cadence per computer (X1 slice 3,
// docs/archive/design/apply-checkpoint-without-replay-2026-10-26.md, F6; review 329).
// The guest compiles the cadence off. The host names the computers that run
// it, with the values, as kernel boot arguments, so FP0 turns it on for one
// computer, reads last_bind_millis, and only then widens it. Written before the
// code. Failure modes guarded:
//  1. a computer nobody selected getting the cadence (an empty list selects
//     none; an unselected launch's boot args are byte-identical to today's);
//  2. the selection missing from one of the two boot paths (microvm store disk
//     and legacy rootfs);
//  3. a bad value (zero, negative, unparsable) reaching the guest, or no value
//     at all, so a selected computer runs the unmeasured defaults by accident;
//  4. the node-b list naming every computer, a class, or the paper;
//  5. the guest boot script copying more than the two values, or copying
//     anything unvalidated into the daemon's environment file.

const (
	cadenceEventsArg = "choir.checkpoint_every_events=2500"
	cadenceEveryArg  = "choir.checkpoint_every=60m"
)

func TestCheckpointCadenceBootArgsPerComputer(t *testing.T) {
	for _, microvm := range []bool{false, true} {
		selected := balloonTestVMConfig()
		selected.ComputerID = "computer-selected"
		other := balloonTestVMConfig()
		other.VMID = "vm-other"
		other.ComputerID = "computer-other"
		mutate := func(c *ManagerConfig) {}
		if microvm {
			selected.StoreDiskPath = "/opt/go-choir/guest/store.erofs"
			other.StoreDiskPath = "/opt/go-choir/guest/store.erofs"
			mutate = func(c *ManagerConfig) { c.KernelParams = "root=fstab init=/nix/store/init-a" }
		}
		base := balloonTestManager(t, mutate)
		want := bootArgsOf(t, base.buildFirecrackerConfig(other, 9001))
		if strings.Contains(want, "checkpoint_every") {
			t.Fatalf("microvm=%t: default boot args carry the cadence: %s", microvm, want)
		}
		if got := bootArgsOf(t, base.buildFirecrackerConfig(selected, 9001)); strings.Contains(got, "checkpoint_every") {
			t.Fatalf("microvm=%t: an empty list selected a computer: %s", microvm, got)
		}
		mgr := balloonTestManager(t, func(c *ManagerConfig) {
			mutate(c)
			c.CheckpointCadenceOnly = []string{"computer-selected"}
			c.CheckpointCadenceEveryEvents = 2500
			c.CheckpointCadenceEvery = "60m"
		})
		if got := bootArgsOf(t, mgr.buildFirecrackerConfig(other, 9001)); got != want {
			t.Fatalf("microvm=%t: an unselected computer's boot args changed:\n got %s\nwant %s", microvm, got, want)
		}
		args := " " + bootArgsOf(t, mgr.buildFirecrackerConfig(selected, 9001)) + " "
		for _, arg := range []string{cadenceEventsArg, cadenceEveryArg} {
			if !strings.Contains(args, " "+arg+" ") {
				t.Fatalf("microvm=%t: selected boot args lack %s: %s", microvm, arg, args)
			}
		}
	}
}

func TestCheckpointCadenceSelectionWithoutValidValuesSelectsNothing(t *testing.T) {
	selected := balloonTestVMConfig()
	selected.ComputerID = "computer-selected"
	for _, tc := range []struct {
		name   string
		events int
		every  string
	}{
		{"no values", 0, ""},
		{"negative events", -1, "60m"},
		{"unparsable duration", 2500, "soon"},
		{"negative duration", 2500, "-5m"},
		{"zero duration with zero events", 0, "0s"},
	} {
		mgr := balloonTestManager(t, func(c *ManagerConfig) {
			c.CheckpointCadenceOnly = []string{"computer-selected"}
			c.CheckpointCadenceEveryEvents = tc.events
			c.CheckpointCadenceEvery = tc.every
		})
		if got := bootArgsOf(t, mgr.buildFirecrackerConfig(selected, 9001)); strings.Contains(got, "checkpoint_every") {
			t.Fatalf("%s: a selection with no usable value reached the guest: %s", tc.name, got)
		}
	}
	// One usable trigger is enough: the other stays off in the guest.
	mgr := balloonTestManager(t, func(c *ManagerConfig) {
		c.CheckpointCadenceOnly = []string{"computer-selected"}
		c.CheckpointCadenceEveryEvents = 500
	})
	got := " " + bootArgsOf(t, mgr.buildFirecrackerConfig(selected, 9001)) + " "
	if !strings.Contains(got, " choir.checkpoint_every_events=500 ") || strings.Contains(got, "choir.checkpoint_every=") {
		t.Fatalf("events-only selection: %s", got)
	}
}

func TestCheckpointCadenceConfigFromEnv(t *testing.T) {
	for _, k := range []string{"VM_CHECKPOINT_CADENCE_ONLY", "VM_CHECKPOINT_CADENCE_EVERY_EVENTS", "VM_CHECKPOINT_CADENCE_EVERY"} {
		t.Setenv(k, "")
	}
	cfg := LoadConfigFromEnv()
	if len(cfg.CheckpointCadenceOnly) != 0 {
		t.Fatalf("default must select none: %v", cfg.CheckpointCadenceOnly)
	}
	t.Setenv("VM_CHECKPOINT_CADENCE_ONLY", "computer-1, computer-2")
	cfg = LoadConfigFromEnv()
	if !reflect.DeepEqual(cfg.CheckpointCadenceOnly, []string{"computer-1", "computer-2"}) {
		t.Fatalf("selector = %v", cfg.CheckpointCadenceOnly)
	}
	if cfg.CheckpointCadenceEveryEvents != 2500 || cfg.CheckpointCadenceEvery != "60m" {
		t.Fatalf("selected values default to the design's 2500 / 60m, got %d / %q", cfg.CheckpointCadenceEveryEvents, cfg.CheckpointCadenceEvery)
	}
	t.Setenv("VM_CHECKPOINT_CADENCE_EVERY_EVENTS", "100")
	t.Setenv("VM_CHECKPOINT_CADENCE_EVERY", "10m")
	cfg = LoadConfigFromEnv()
	if cfg.CheckpointCadenceEveryEvents != 100 || cfg.CheckpointCadenceEvery != "10m" {
		t.Fatalf("values = %d / %q", cfg.CheckpointCadenceEveryEvents, cfg.CheckpointCadenceEvery)
	}
}

// node-b names the selector once, never selects every computer, a class, or
// the paper; FP0 names the first computer.
func TestNodeBNamesTheCheckpointCadenceSelector(t *testing.T) {
	raw, err := os.ReadFile("../../nix/node-b.nix")
	if err != nil {
		t.Skipf("nix/node-b.nix not readable from the package: %v", err)
	}
	m := regexp.MustCompile(`"VM_CHECKPOINT_CADENCE_ONLY=([^"]*)"`).FindAllStringSubmatch(string(raw), -1)
	if len(m) != 1 {
		t.Fatalf("node-b sets VM_CHECKPOINT_CADENCE_ONLY %d times, want once", len(m))
	}
	// Rings decide reach (docs/design/progressive-delivery-2026-10-26.md,
	// section 3): R0 is d@e, R1 is class:test. Not "*" yet.
	toks := parseComputerSelector(m[0][1])
	has := func(want string) bool {
		for _, tok := range toks {
			if tok == want {
				return true
			}
		}
		return false
	}
	if !has("computer-59c902c5d46caecc0a180a8557164e72") || !has("class:test") {
		t.Fatalf("node-b cadence ring = %q, want d@e (R0) and class:test (R1)", toks)
	}
	if has("*") {
		t.Fatalf("node-b cadence ring = %q: \"*\" is a later ring", toks)
	}
}

// The guest copies exactly the two validated values into the daemon's
// environment file.
func TestGuestImageCopiesOnlyValidatedCadenceValues(t *testing.T) {
	raw, err := os.ReadFile("../../nix/autoputer-vm.nix")
	if err != nil {
		t.Skipf("nix/autoputer-vm.nix not readable from the package: %v", err)
	}
	img := string(raw)
	for _, want := range []string{
		`choir.checkpoint_every_events=*)`, `CHOIR_CHECKPOINT_EVERY_EVENTS=`,
		`choir.checkpoint_every=*)`, `CHOIR_CHECKPOINT_EVERY=`,
	} {
		if !strings.Contains(img, want) {
			t.Fatalf("the guest boot script lacks %q", want)
		}
	}
	// Validation is in the script: digits only for the count, digits and one
	// unit for the duration.
	if !strings.Contains(img, `*[!0-9]*)`) {
		t.Fatal("the guest boot script does not reject a non-numeric event count")
	}
}

// Naming a computer moves its launch-config hash: a snapshot taken without the
// cadence is refused and the computer cold boots with it (the same link the
// tunnel selector relies on), and a computer the list does not name resumes.
func TestCheckpointCadenceSelectionRefusesAnUnselectedSnapshot(t *testing.T) {
	for _, tc := range []struct {
		name string
		only []string
		want string
	}{
		{"no selector", nil, ""},
		{"another computer", []string{"computer-other"}, ""},
		{"this computer id", []string{"computer-s3"}, snapshotReasonLaunchConfig},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newSnapshotFixture(t)
			f.serveLaunchAPI(t)
			if err := f.mgr.HibernateVM(f.vmID); err != nil {
				t.Fatal(err)
			}
			dir := f.mgr.snapshotDir(f.vmID)
			meta, err := readSnapshotMeta(dir)
			if err != nil {
				t.Fatal(err)
			}
			f.mgr.cfg.CheckpointCadenceOnly = tc.only
			f.mgr.cfg.CheckpointCadenceEveryEvents = 2500
			f.mgr.cfg.CheckpointCadenceEvery = "60m"
			v := snapshotValidation{Now: time.Now(), FirecrackerVersion: meta.FirecrackerVersion, HostKernelRelease: meta.HostKernelRelease, Expect: f.expect}
			if got := f.mgr.validateSnapshot(dir, meta, v); got != tc.want {
				t.Fatalf("validateSnapshot = %q, want %q", got, tc.want)
			}
		})
	}
}

// S3 (docs/problems/paper-never-checkpointed-2026-10-27.md; owner,
// 2026-10-28: "rings decide reach, the code doesn't"). The bind is the
// hash-only witness, milliseconds under the hold, so the code no longer
// refuses a wildcard, a class, or a protected-class computer; the host list is
// the ring. This supersedes review 336's id-only rule, which existed because
// the live row extract's hold was unmeasured on the paper.
func TestCheckpointCadenceSelectorFollowsTheRing(t *testing.T) {
	launch := func(only []string, class string) string {
		cfg := balloonTestVMConfig()
		cfg.ComputerID = "computer-selected"
		cfg.WarmnessClass = class
		mgr := balloonTestManager(t, func(c *ManagerConfig) {
			c.CheckpointCadenceOnly = only
			c.CheckpointCadenceEveryEvents = 2500
			c.CheckpointCadenceEvery = "60m"
		})
		return bootArgsOf(t, mgr.buildFirecrackerConfig(cfg, 9001))
	}
	for _, tc := range []struct {
		name  string
		only  []string
		class string
		want  bool
	}{
		{"a named computer", []string{"computer-selected"}, "", true},
		{"another computer named", []string{"computer-other"}, "", false},
		{"wildcard", []string{"*"}, "", true},
		{"class test", []string{"class:test"}, "test", true},
		{"class test, another class", []string{"class:test"}, "standard", false},
		{"class critical_protected", []string{"class:critical_protected"}, "critical_protected", true},
		{"named critical_protected computer", []string{"computer-selected"}, "critical_protected", true},
		{"named public_platform computer", []string{"computer-selected"}, "public_platform", true},
		{"padded class token", []string{" class:test "}, "test", true},
		{"empty list", nil, "critical_protected", false},
	} {
		got := strings.Contains(launch(tc.only, tc.class), "choir.checkpoint_every")
		if got != tc.want {
			t.Fatalf("%s: cadence boot args present = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// The host config load keeps every selector token the generic selector takes.
func TestCheckpointCadenceConfigKeepsRingTokens(t *testing.T) {
	t.Setenv("VM_CHECKPOINT_CADENCE_ONLY", "computer-1, class:test, computer-2")
	t.Setenv("VM_CHECKPOINT_CADENCE_EVERY_EVENTS", "")
	t.Setenv("VM_CHECKPOINT_CADENCE_EVERY", "")
	if got := LoadConfigFromEnv().CheckpointCadenceOnly; !reflect.DeepEqual(got, []string{"computer-1", "class:test", "computer-2"}) {
		t.Fatalf("selector = %v, want every token", got)
	}
}
