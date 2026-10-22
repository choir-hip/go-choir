//go:build linux

package agentcore

import (
	"context"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// The route's two halves, off staging. Ways they could mislead (written
// first): the desk-cell half runs a worker that is not the production
// entrypoint (so the staging receipt proves nothing about `autoputer
// desk-session`); the egress half calls a refused port "blocked" (a refusal
// means the packet reached the host) or a timeout "open".

// TestIsolationSelfTestDeskCellsThroughTheRealWorker: the self-test spawns
// the real `autoputer desk-session` binary with the desk_go_eval config, and
// that entrypoint runs the probes after its confinement.
func TestIsolationSelfTestDeskCellsThroughTheRealWorker(t *testing.T) {
	if os.Geteuid() != 0 {
		// Desks are kept apart by distinct uids, which only a root spawner
		// can switch to: under a non-root spawner every desk shares one uid
		// and the cross-desk canary read succeeds. The guest daemon spawns
		// as root; the deployed self-test is the acceptance
		// (docs/problems/fp2-batch1-ci-red-isolation-selftest-and-worker-stdout-race-2026-10-22.md).
		t.Skip("the cross-desk refusals need a root spawner")
	}
	deskTestWorkerBin(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	rec := yaegikernel.DeskCellSelfTest(ctx, deskCellSelfTestConfig("computer-selftest"))
	if !rec.OK {
		t.Fatalf("self-test through the real worker failed: %v", rec.Failures)
	}
	if len(rec.Workers) != len(yaegikernel.DeskCellSelfTestRoles) {
		t.Fatalf("workers = %d", len(rec.Workers))
	}
	for _, w := range rec.Workers {
		if got := w.Probes["socket_inet_stream"]; !strings.HasPrefix(got, "EPERM") {
			t.Errorf("%s: the real worker's socket probe = %q, want EPERM", w.Role, got)
		}
		if w.Status["Seccomp"] != "2" || w.Status["NoNewPrivs"] != "1" {
			t.Errorf("%s: Seccomp=%q NoNewPrivs=%q", w.Role, w.Status["Seccomp"], w.Status["NoNewPrivs"])
		}
	}
}

func TestIsolationSelfTestEgressClassification(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			c.Close()
		}
	}()
	closed, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	closedAddr := closed.Addr().String()
	closed.Close()
	got := probeGuestEgress(context.Background(), []isolationEgressProbe{
		{Name: "open", Addr: ln.Addr().String(), Want: "open"},
		{Name: "refused_is_not_blocked", Addr: closedAddr, Want: "blocked"},
		// TEST-NET-1 is never routed: a timeout or unreachable, never open.
		{Name: "dropped", Addr: "192.0.2.1:9", Want: "blocked"},
	}, 1500*time.Millisecond)
	if !got[0].OK || got[0].Result != "open" {
		t.Errorf("listener: %+v", got[0])
	}
	if got[1].OK || got[1].Result != "refused" {
		t.Errorf("a refused port must not count as blocked: %+v", got[1])
	}
	if got[2].Result == "open" || got[2].Result == "refused" {
		t.Errorf("unroutable address: %+v", got[2])
	}
	host, targets := guestEgressTargets("http://10.200.0.1:8084")
	if host != "10.200.0.1" || len(targets) == 0 {
		t.Fatalf("targets from gateway URL: host=%q n=%d", host, len(targets))
	}
	want := map[string]string{"host_sshd": "10.200.0.1:22", "host_gateway": "10.200.0.1:8084", "other_guest": "10.200.0.6:8085"}
	for _, tg := range targets {
		if addr, ok := want[tg.Name]; ok && tg.Addr != addr {
			t.Errorf("%s = %s, want %s", tg.Name, tg.Addr, addr)
		}
	}
	if h, _ := guestEgressTargets("http://gateway.internal:8084"); h != "" {
		t.Errorf("a name must not become a probe host: %q", h)
	}
}
