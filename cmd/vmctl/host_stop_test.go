package main

import (
	"errors"
	"testing"
)

// S5 busy hibernate: when vmctl's exit is a planned host stop
// (docs/problems/paper-never-checkpointed-2026-10-27.md, slice S5). Written
// before the code. Ways it can fail:
//   - E1 (owner 2026-10-28: on by default, no off switch) a host shutdown
//     needs configuration to hibernate;
//   - E2 a vmctl restart for a deploy (system running) hibernates every
//     computer instead of leaving it running for reattach;
//   - E3 an unreadable system state is taken as a shutdown;
//   - E4 a host shutdown, or VMCTL_STOP_MANAGED_ON_EXIT, does not
//     hibernate.
func TestDecideHostStop(t *testing.T) {
	state := func(s string, err error) func() (string, error) {
		return func() (string, error) { return s, err }
	}
	for _, tc := range []struct {
		name       string
		stopOnExit bool
		state      func() (string, error)
		want       bool
		why        string
	}{
		{"E2 deploy restart", false, state("running", nil), false, ""},
		{"E2 degraded host restart", false, state("degraded\n", nil), false, ""},
		{"E3 unreadable", false, state("", errors.New("no bus")), false, ""},
		{"E4 host shutdown", false, state("stopping\n", nil), true, hostStopWhyShutdown},
		{"E4 stop on exit", true, state("running", nil), true, hostStopWhyStopOnExit},
	} {
		got := decideHostStop(tc.stopOnExit, tc.state)
		if got.Hibernate != tc.want || got.Why != tc.why {
			t.Fatalf("%s: %+v, want hibernate=%v why=%q", tc.name, got, tc.want, tc.why)
		}
	}
}
