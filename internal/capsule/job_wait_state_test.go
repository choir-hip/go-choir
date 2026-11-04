package capsule

import (
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// Failure modes written before the code:
//  1. the record drops the host's own reason, so nothing the old text said
//     is lost only when Detail keeps it;
//  2. need is the full peak when the capsule's reservation credit covers
//     part of it (the model would be told it needs more than it waits for);
//  3. other computers' holdings are named (another owner's computer must
//     never reach this guest) or are lost when only their total is known;
//  4. a pressure wait renders headroom figures as if headroom were the cause;
//  5. a pure waiter (choir-delegate wait --park, sleep) is treated as a
//     build and waits for host memory;
//  6. a chained shell command that only starts with a waiter is exempted.

func TestHostMemoryWaitStateRendersTheRefusal(t *testing.T) {
	since := time.Date(2026, 11, 4, 1, 0, 0, 0, time.UTC)
	mw := &yaegikernel.MemoryWait{NeedBytes: 2 << 30, RoomBytes: 600 << 20, Holders: []yaegikernel.MemoryHolder{
		{Kind: "job_lease", Own: true, CapsuleID: "cap-1", JobID: "j-0", Bytes: 1536 << 20},
		{Kind: "job_lease", Class: "batch", Bytes: 3 << 30},
	}}
	w := HostMemoryWaitState("job j-1", mw, "waiting for host memory: host text", since, 5*time.Second, 30*time.Minute)
	if w == nil || w.Kind != types.WaitKindHostMemory || w.Class != types.WaitClassContention {
		t.Fatalf("wait = %+v", w)
	}
	if w.NeedBytes != 2<<30 || !w.HasHave || w.HaveBytes != 600<<20 {
		t.Fatalf("need/have are vmctl's, not re-derived: %+v", w)
	}
	if len(w.Holders) != 2 || w.Holders[0].Who != "job j-0" || w.Holders[1].Who != "another computer" || w.Holders[1].What != "job_lease, batch" {
		t.Fatalf("holders = %+v", w.Holders)
	}
	for _, h := range w.Holders[1:] {
		if strings.Contains(h.Who+h.What, "cap-") {
			t.Fatalf("another computer's holder carries an id: %+v", h)
		}
	}
	if w.Detail != "waiting for host memory: host text" || w.CheckEverySec != 5 || !w.Until.Equal(since.Add(30*time.Minute)) {
		t.Fatalf("detail/cadence/bound = %+v", w)
	}
	if s := w.SentenceAt(since.Add(time.Minute)); !strings.Contains(s, "2048 MiB") || !strings.Contains(s, "another computer") {
		t.Fatalf("sentence = %s", s)
	}
}

func TestHostMemoryWaitStatePressure(t *testing.T) {
	w := HostMemoryWaitState("job j-1", &yaegikernel.MemoryWait{NeedBytes: 1 << 30, PressureHigh: true}, "r", time.Now(), 5*time.Second, time.Minute)
	if w.HasHave || !strings.Contains(w.Have, "pressure") {
		t.Fatalf("pressure wait = %+v", w)
	}
}

func TestHostMemoryWaitStateNilRefusalNoWait(t *testing.T) {
	if w := HostMemoryWaitState("job j", nil, "", time.Now(), time.Second, time.Minute); w != nil {
		t.Fatalf("no refusal must be no wait: %+v", w)
	}
}

func TestPureWaitJob(t *testing.T) {
	yes := [][]string{
		{"choir-delegate", "wait", "--id", "d1", "--park"},
		{"/usr/local/bin/choir-delegate", "wait", "--id", "d1", "--park", "--seconds", "1800"},
		{"choir-delegate", "status"},
		{"sh", "-c", "choir-delegate wait --id d1 --park"},
		{"sleep", "60"},
	}
	for _, argv := range yes {
		if !PureWaitJob(argv) {
			t.Errorf("%q should be a pure waiter", argv)
		}
	}
	no := [][]string{
		{"choir-delegate", "start", "--spec", "/tmp/x"},
		{"sh", "-c", "choir-delegate wait --id d1 --park; go test ./..."},
		{"sh", "-c", "choir-delegate wait --id d1 --park && make"},
		{"sh", "-c", "sleep 5 | go build ./..."},
		{"sh", "-c", "sleep $(go build ./...)"},
		{"go", "test", "./..."},
		{"sleep"},
		{},
	}
	for _, argv := range no {
		if PureWaitJob(argv) {
			t.Errorf("%q must not be exempt from the host memory wait", argv)
		}
	}
}
