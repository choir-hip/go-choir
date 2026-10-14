package vmbackend

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Failure modes for per-computer recording and switching (report section 7):
//
//	S1 a computer without a record (every computer today) reads as anything
//	   but Firecracker, so its next boot moves backend silently;
//	S2 a corrupt or unknown record is guessed, so a computer boots on a
//	   backend it never ran on;
//	S3 a backend change while running, pending, hibernated or failed turns
//	   into a snapshot load on the wrong VMM or a crash boot nobody asked
//	   for;
//	S4 a host that never opted in, or a production computer, gets a
//	   non-default backend;
//	S5 a typo in VM_BACKENDS_ENABLED silently disables a backend.

func TestLaunchRecordMissingIsImplicitFirecracker(t *testing.T) {
	rec, err := ReadLaunchRecord(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if rec.Backend != Firecracker || !rec.Implicit {
		t.Fatalf("S1: missing record read as %+v", rec)
	}
}

func TestLaunchRecordRoundTrip(t *testing.T) {
	dir := t.TempDir()
	at := time.Date(2026, 10, 14, 20, 0, 0, 0, time.UTC)
	in := LaunchRecord{Backend: CloudHypervisor, BackendVersion: "cloud-hypervisor v52.0", Epoch: 9, RecordedAt: at, Implicit: true}
	if err := WriteLaunchRecord(dir, in); err != nil {
		t.Fatal(err)
	}
	out, err := ReadLaunchRecord(dir)
	if err != nil {
		t.Fatal(err)
	}
	if out.Backend != CloudHypervisor || out.BackendVersion != in.BackendVersion || out.Epoch != 9 || !out.RecordedAt.Equal(at) || out.Implicit {
		t.Fatalf("round trip %+v", out)
	}
	info, err := os.Stat(LaunchRecordPath(dir))
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("record mode %v %v", info, err)
	}
	if _, err := os.Stat(LaunchRecordPath(dir) + ".tmp"); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("temp file left behind")
	}
	// Overwrite: the latest cold boot wins.
	if err := WriteLaunchRecord(dir, LaunchRecord{Backend: Firecracker}); err != nil {
		t.Fatal(err)
	}
	if out, _ := ReadLaunchRecord(dir); out.Backend != Firecracker || out.RecordedAt.IsZero() {
		t.Fatalf("overwrite %+v", out)
	}
}

func TestLaunchRecordRefusesGuessing(t *testing.T) {
	for name, body := range map[string]string{
		"corrupt":         `{"backend":`,
		"unknown backend": `{"version":1,"backend":"qemu"}`,
		"empty backend":   `{"version":1,"backend":""}`,
		"future version":  `{"version":2,"backend":"firecracker"}`,
	} {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, LaunchRecordName), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		if rec, err := ReadLaunchRecord(dir); err == nil {
			t.Errorf("S2 %s: read as %+v", name, rec)
		}
	}
	if err := WriteLaunchRecord(t.TempDir(), LaunchRecord{Backend: "qemu"}); err == nil {
		t.Fatal("wrote an unknown backend")
	}
	if err := WriteLaunchRecord(t.TempDir(), LaunchRecord{}); err == nil {
		t.Fatal("wrote an empty backend")
	}
}

func TestCheckSwitchOnlyFromStopped(t *testing.T) {
	for _, phase := range []Phase{PhaseStopped, PhasePending, PhaseRunning, PhaseHibernated, PhaseFailed} {
		if err := CheckSwitch(Firecracker, Firecracker, phase); err != nil {
			t.Errorf("staying on firecracker in %s refused: %v", phase, err)
		}
		if err := CheckSwitch("", Firecracker, phase); err != nil {
			t.Errorf("implicit firecracker staying put in %s refused: %v", phase, err)
		}
		err := CheckSwitch(Firecracker, CloudHypervisor, phase)
		if phase == PhaseStopped {
			if err != nil {
				t.Errorf("switch from stopped refused: %v", err)
			}
			continue
		}
		if !errors.Is(err, ErrSwitchNotStopped) {
			t.Errorf("S3: switch in phase %s: %v, want ErrSwitchNotStopped", phase, err)
		}
		if err := CheckSwitch(CloudHypervisor, Firecracker, phase); !errors.Is(err, ErrSwitchNotStopped) {
			t.Errorf("S3: switch back in phase %s: %v", phase, err)
		}
	}
	if err := CheckSwitch(Firecracker, "qemu", PhaseStopped); err == nil {
		t.Fatal("switch to unknown backend allowed")
	}
}

func TestPolicyDefaultIsFirecrackerOnly(t *testing.T) {
	p, err := ParsePolicy("")
	if err != nil {
		t.Fatal(err)
	}
	if !p.Enabled(Firecracker) || p.Enabled(CloudHypervisor) {
		t.Fatal("default policy must enable firecracker only")
	}
	if n, err := p.Select("", false); err != nil || n != Firecracker {
		t.Fatalf("default selection %q %v", n, err)
	}
	if _, err := p.Select(CloudHypervisor, true); !errors.Is(err, ErrBackendNotEnabled) {
		t.Fatalf("S4: CH on a host that never opted in: %v", err)
	}
}

func TestPolicySelectTestComputersOnly(t *testing.T) {
	p, err := ParsePolicy(" firecracker , cloud-hypervisor ")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Select(CloudHypervisor, false); !errors.Is(err, ErrBackendTestOnly) {
		t.Fatalf("S4: CH for a production computer: %v", err)
	}
	if n, err := p.Select(CloudHypervisor, true); err != nil || n != CloudHypervisor {
		t.Fatalf("CH for a test computer: %q %v", n, err)
	}
	if n, err := p.Select(Firecracker, false); err != nil || n != Firecracker {
		t.Fatalf("firecracker for production: %q %v", n, err)
	}
	if _, err := p.Select("qemu", true); err == nil {
		t.Fatal("unknown backend selected")
	}
	chOnly, err := ParsePolicy("cloud-hypervisor")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := chOnly.Select("", false); !errors.Is(err, ErrBackendNotEnabled) {
		t.Fatalf("default on a CH-only host: %v", err)
	}
}

func TestPolicyRefusesTypos(t *testing.T) {
	for _, v := range []string{"firecracker,cloud_hypervisor", "fc", ",", " , "} {
		if _, err := ParsePolicy(v); err == nil {
			t.Errorf("S5: VM_BACKENDS_ENABLED=%q accepted", v)
		}
	}
}

func TestParseName(t *testing.T) {
	for in, want := range map[string]Name{"": Firecracker, " Firecracker ": Firecracker, "cloud-hypervisor": CloudHypervisor} {
		if got, err := ParseName(in); err != nil || got != want {
			t.Errorf("ParseName(%q) = %q %v", in, got, err)
		}
	}
	if _, err := ParseName("ch"); err == nil {
		t.Error("abbreviations must not parse")
	}
}
