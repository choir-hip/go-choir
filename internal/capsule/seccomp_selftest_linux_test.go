//go:build linux

package capsule

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestMain lets this test binary stand in for the daemon's binary: the
// self-test re-execs its binary as `<bin> capsule-seccomp-selftest <filter>`.
func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == SeccompSelfTestArg {
		os.Exit(RunSeccompSelfTestChild(os.Args[2:]))
	}
	os.Exit(m.Run())
}

// S1: the self-test probes every filter the clone test covers, each from the
// constructor the capsule calls.
func TestSeccompSelfTestCoversEveryCapsuleFilter(t *testing.T) {
	names := map[string]bool{}
	for _, n := range SeccompSelfTestFilterNames {
		names[n] = true
		if _, ok := seccompSelfTestFilter(n); !ok {
			t.Errorf("filter %q has no constructor", n)
		}
	}
	for _, f := range cloneFilters {
		if !names[f.name] {
			t.Errorf("the self-test omits capsule filter %q", f.name)
		}
	}
	a, _ := seccompSelfTestFilterHash("workload")
	b, _ := seccompSelfTestFilterHash("workload")
	c, _ := seccompSelfTestFilterHash("broker")
	if a == "" || a != b || a == c {
		t.Fatalf("filter hash not stable or not distinct: %q %q %q", a, b, c)
	}
}

// End to end through a child process: each filter is loaded, clone3 is
// ENOSYS, a new user namespace is EPERM, and fork/exec still work.
func TestSeccompSelfTestThroughAChild(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	rec := SeccompSelfTest(ctx, os.Args[0])
	if !rec.OK {
		t.Fatalf("seccomp self-test failed: %v\n%+v", rec.Failures, rec)
	}
	if len(rec.Filters) != len(SeccompSelfTestFilterNames) {
		t.Fatalf("filters = %d", len(rec.Filters))
	}
}

// S2: an unknown filter name loads nothing and the child says so.
func TestSeccompSelfTestChildRefusesAnUnknownFilter(t *testing.T) {
	if code := RunSeccompSelfTestChild([]string{"no-such-filter"}); code == 0 {
		t.Fatal("an unknown filter name exited 0")
	}
}
