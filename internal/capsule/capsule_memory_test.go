package capsule

import (
	"os"
	"strings"
	"testing"
)

// Capsule memory per doctrine (SDC RES-04; docs/choir-doctrine.md:406-408):
// memory.high is the request, memory.max twice that, admission counts the
// request, memory.events is feedback. The ways it could fail are Q1-Q10 in
// docs/definitions/choir-launch-fp2-desk-boundary-2026-10-21.md, "Second
// batch"; each test names the one it closes.

func TestCapsuleMemoryPolicyFromEnv(t *testing.T) {
	cases := map[string]CapsuleMemoryPolicy{
		"":            CapsuleMemoryObserve,
		"observe":     CapsuleMemoryObserve,
		"doctrine":    CapsuleMemoryDoctrine,
		" Doctrine\n": CapsuleMemoryDoctrine,
		// Q8: a typo takes the narrower shape, never the wider one.
		"doctrin": CapsuleMemoryObserve,
		"max":     CapsuleMemoryObserve,
	}
	for raw, want := range cases {
		got := CapsuleMemoryPolicyFromEnv(func(k string) string {
			if k != CapsuleMemoryPolicyEnv {
				t.Fatalf("read %s", k)
			}
			return raw
		})
		if got != want {
			t.Errorf("%q: policy %q, want %q", raw, got, want)
		}
	}
}

func TestPlanCapsuleMemory(t *testing.T) {
	const gib = int64(1 << 30)
	cases := []struct {
		name    string
		policy  CapsuleMemoryPolicy
		spec    SpawnSpec
		want    capsuleMemoryPlan
		wantErr string
	}{
		// Observe is the shape before RES-04: admission and memory.max at
		// MemoryMax, no memory.high (Q1: nothing throttles a build).
		{name: "observe", policy: CapsuleMemoryObserve, spec: SpawnSpec{MemoryMax: 2 * gib, MemoryRequest: 2 * gib},
			want: capsuleMemoryPlan{Admit: 2 * gib, Max: 2 * gib}},
		{name: "zero value is observe", spec: SpawnSpec{MemoryMax: 2 * gib, MemoryRequest: gib},
			want: capsuleMemoryPlan{Admit: 2 * gib, Max: 2 * gib}},
		{name: "doctrine", policy: CapsuleMemoryDoctrine, spec: SpawnSpec{MemoryMax: 2 * gib, MemoryRequest: 2 * gib},
			want: capsuleMemoryPlan{Admit: 2 * gib, High: 2 * gib, Max: 4 * gib}},
		{name: "doctrine smaller request", policy: CapsuleMemoryDoctrine, spec: SpawnSpec{MemoryMax: 2 * gib, MemoryRequest: gib / 2},
			want: capsuleMemoryPlan{Admit: gib / 2, High: gib / 2, Max: gib}},
		// Q3: a spec that declares no request is not silently doubled.
		{name: "doctrine without a request keeps the old shape", policy: CapsuleMemoryDoctrine, spec: SpawnSpec{MemoryMax: 2 * gib},
			want: capsuleMemoryPlan{Admit: 2 * gib, Max: 2 * gib}},
		// Q10.
		{name: "negative request", policy: CapsuleMemoryObserve, spec: SpawnSpec{MemoryMax: 2 * gib, MemoryRequest: -1}, wantErr: "request"},
		{name: "overflowing request", policy: CapsuleMemoryDoctrine, spec: SpawnSpec{MemoryMax: 2 * gib, MemoryRequest: 1 << 62}, wantErr: "request"},
		{name: "no max", policy: CapsuleMemoryObserve, spec: SpawnSpec{MemoryRequest: gib}, wantErr: "memory"},
	}
	for _, tc := range cases {
		got, err := planCapsuleMemory(tc.policy, tc.spec)
		if tc.wantErr != "" {
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("%s: err %v, want one naming %q", tc.name, err, tc.wantErr)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: %v", tc.name, err)
			continue
		}
		if got != tc.want {
			t.Errorf("%s: plan %+v, want %+v", tc.name, got, tc.want)
		}
	}
}

// Q9 and the feedback half: the receipt reads memory.events and memory.peak,
// says unknown (-1) for a file it cannot read, and names whether doctrine's
// memory.high would have throttled the capsule.
func TestCapsuleMemoryReceipt(t *testing.T) {
	const mib = int64(1 << 20)
	cg := &fakeMemoryFeedback{
		events: "low 0\nhigh 0\nmax 7\noom 1\noom_kill 1\noom_group_kill 0\n",
		peak:   1843 * mib,
	}
	r := readCapsuleMemoryReceipt(CapsuleMemoryObserve, capsuleMemoryPlan{Admit: 2048 * mib, Max: 2048 * mib}, 1024*mib, cg)
	if r.Peak != 1843*mib || r.EventsHigh != 0 || r.EventsMax != 7 || r.EventsOOM != 1 || r.EventsOOMKill != 1 {
		t.Fatalf("receipt %+v", r)
	}
	if !r.DoctrineWouldThrottle {
		t.Fatal("a peak of 1843 MiB over a 1024 MiB request: doctrine would have throttled")
	}
	line := r.String()
	for _, want := range []string{"policy=observe", "request=1024MiB", "max=2048MiB", "peak=1843MiB", "oom_kill=1", "doctrine_would_throttle=true"} {
		if !strings.Contains(line, want) {
			t.Errorf("receipt line %q lacks %q", line, want)
		}
	}

	r = readCapsuleMemoryReceipt(CapsuleMemoryDoctrine, capsuleMemoryPlan{Admit: 2048 * mib, High: 2048 * mib, Max: 4096 * mib}, 2048*mib, &fakeMemoryFeedback{eventsErr: true, peakErr: true})
	if r.Peak != -1 || r.EventsHigh != -1 || r.EventsOOMKill != -1 || r.DoctrineWouldThrottle {
		t.Fatalf("unreadable files: receipt %+v, want unknowns and no claim", r)
	}
	if !strings.Contains(r.String(), "peak=unknown") || !strings.Contains(r.String(), "high=2048MiB") {
		t.Fatalf("receipt line %q", r.String())
	}
	if r := readCapsuleMemoryReceipt(CapsuleMemoryObserve, capsuleMemoryPlan{Admit: mib, Max: mib}, 0, nil); r.Peak != -1 {
		t.Fatalf("no cgroup: %+v", r)
	}
}

type fakeMemoryFeedback struct {
	events    string
	eventsErr bool
	peak      int64
	peakErr   bool
}

func (f *fakeMemoryFeedback) MemoryEvents() (string, error) {
	if f.eventsErr {
		return "", errFake
	}
	return f.events, nil
}

func (f *fakeMemoryFeedback) MemoryPeak() (int64, error) {
	if f.peakErr {
		return 0, errFake
	}
	return f.peak, nil
}

var errFake = fakeErr("unreadable")

type fakeErr string

func (e fakeErr) Error() string { return string(e) }

// The guest image names the policy rather than relying on the default, so a
// reader of the image sees which shape its capsules get; it ships observe
// until the receipts show builds fit under doctrine's memory.high.
func TestGuestImageNamesTheCapsuleMemoryPolicy(t *testing.T) {
	raw, err := os.ReadFile("../../nix/autoputer-vm.nix")
	if err != nil {
		t.Skipf("nix/autoputer-vm.nix not readable from the package: %v", err)
	}
	if !strings.Contains(string(raw), `CHOIR_CAPSULE_MEMORY_POLICY = "observe";`) {
		t.Fatal("the guest image does not set CHOIR_CAPSULE_MEMORY_POLICY = \"observe\"")
	}
}
