package yaegikernel

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"
)

// The OOM leg of the isolation self-test (docs/problems/
// fp2-step4-deployed-oom-not-drivable-desk-refuses-runaway-2026-10-22.md,
// failure modes O1-O10). These tests pin the judge and the guard that keeps
// the runaway from ever running without a hard budget; the kill itself is
// desk_oom_selftest_linux_test.go.

func goodDeskOOMReceipt() DeskOOMSelfTestReceipt {
	id := DaemonIdentity{PID: 812, StartTicks: 4411, BootID: "2adf3e7d-f8a9-421e-9fb8-005c6573e3da"}
	return DeskOOMSelfTestReceipt{
		Kind:                   "desk_oom_selftest",
		Mode:                   "required",
		Role:                   "research",
		BudgetBytes:            768 << 20,
		MemAvailableBytes:      5 << 30,
		RequiredAvailableBytes: 768<<20 + deskOOMHeadroomBytes,
		Slice:                  DeskOOMCgroup{Path: "/sys/fs/cgroup/choir-desks", MemoryMax: "4294967296", MemoryHigh: "3221225472"},
		Worker: DeskOOMWorker{
			PID:         900,
			Cgroup:      DeskOOMCgroup{Path: "/sys/fs/cgroup/choir-desks/w-1", MemoryMax: "805306368", MemoryHigh: "max", Events: map[string]uint64{"oom_kill": 1, "oom": 1}},
			CellSent:    true,
			CellStarted: true,
			Result:      "transport: EOF",
			Dead:        true,
			ExitCause:   "exited on its own (signal: killed); killed by its memory budget (memory.max 768 MiB, oom_kill 1)",
			OOMKill:     1,
		},
		Sibling:      DeskOOMSibling{Role: "research", PID: 899, BeforeAnswered: true, AfterAnswered: true, OutputBefore: "cell 42", OutputAfter: "cell 42"},
		DaemonBefore: id,
		DaemonAfter:  id,
	}
}

func TestJudgeDeskOOMSelfTestAcceptsAWorkerKilledAtItsBudget(t *testing.T) {
	if f := JudgeDeskOOMSelfTest(goodDeskOOMReceipt()); len(f) != 0 {
		t.Fatalf("a worker killed at its budget with the daemon and sibling alive was refused: %v", f)
	}
}

func TestJudgeDeskOOMSelfTestRefusesEveryShortfall(t *testing.T) {
	cases := map[string]func(*DeskOOMSelfTestReceipt){
		// O2: no oom_kill means something else ended the worker.
		"no oom_kill":      func(r *DeskOOMSelfTestReceipt) { r.Worker.OOMKill = 0; r.Worker.Cgroup.Events["oom_kill"] = 0 },
		"cause not budget": func(r *DeskOOMSelfTestReceipt) { r.Worker.ExitCause = "killed by the host (signal: killed)" },
		"worker alive":     func(r *DeskOOMSelfTestReceipt) { r.Worker.Dead = false },
		"cell not sent":    func(r *DeskOOMSelfTestReceipt) { r.Worker.CellSent = false },
		"cell returned":    func(r *DeskOOMSelfTestReceipt) { r.Worker.Result = "returned" },
		"worker error":     func(r *DeskOOMSelfTestReceipt) { r.Worker.Error = "spawn: no" },
		// O1: the budget must be the hard, numeric manifest value.
		"memory.max unbounded": func(r *DeskOOMSelfTestReceipt) { r.Worker.Cgroup.MemoryMax = "max" },
		"memory.max wrong":     func(r *DeskOOMSelfTestReceipt) { r.Worker.Cgroup.MemoryMax = "1073741824" },
		"no budget":            func(r *DeskOOMSelfTestReceipt) { r.BudgetBytes = 0 },
		"slice unbounded":      func(r *DeskOOMSelfTestReceipt) { r.Slice.MemoryMax = "max" },
		"slice unread":         func(r *DeskOOMSelfTestReceipt) { r.Slice.MemoryMax = "" },
		// O4: the daemon must be the same process.
		"daemon pid changed": func(r *DeskOOMSelfTestReceipt) { r.DaemonAfter.PID = 813 },
		"daemon restarted":   func(r *DeskOOMSelfTestReceipt) { r.DaemonAfter.StartTicks = 9999 },
		"guest rebooted":     func(r *DeskOOMSelfTestReceipt) { r.DaemonAfter.BootID = "other" },
		"daemon unread": func(r *DeskOOMSelfTestReceipt) {
			r.DaemonBefore = DaemonIdentity{Error: "no /proc"}
			r.DaemonAfter = r.DaemonBefore
		},
		// O5: the sibling must still answer.
		"sibling silent after": func(r *DeskOOMSelfTestReceipt) { r.Sibling.AfterAnswered = false },
		"sibling never ran":    func(r *DeskOOMSelfTestReceipt) { r.Sibling.BeforeAnswered = false },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			r := goodDeskOOMReceipt()
			r.Worker.Cgroup.Events = map[string]uint64{"oom_kill": 1, "oom": 1}
			mutate(&r)
			if f := JudgeDeskOOMSelfTest(r); len(f) == 0 {
				t.Fatalf("%s: judged ok", name)
			}
		})
	}
}

// O1: with no desks cgroup the leg spawns nothing and allocates nothing.
func TestDeskOOMSelfTestRefusesWithoutADesksCgroup(t *testing.T) {
	t.Setenv(DeskCgroupRootEnv, "")
	rec := DeskOOMSelfTest(context.Background(), DeskOOMSelfTestConfig{
		Base: DeskSessionWorkerConfig{Bin: "/nonexistent/never-spawned", WorkerArg: "desk-session"},
	})
	if rec.OK {
		t.Fatal("the OOM leg passed with no desks cgroup")
	}
	if rec.Worker.CellSent || rec.Worker.PID != 0 || rec.Sibling.PID != 0 {
		t.Fatalf("the OOM leg spawned or allocated without a budget: %+v", rec)
	}
	if !strings.Contains(strings.Join(rec.Failures, "\n"), "desks cgroup") {
		t.Fatalf("failures do not name the missing desks cgroup: %v", rec.Failures)
	}
	if rec.Role != "research" {
		t.Fatalf("default role = %q, want research (the untrusted-input class)", rec.Role)
	}
}

// A role the manifest gives no worker budget is refused before any spawn.
func TestDeskOOMSelfTestRefusesARoleWithNoBudget(t *testing.T) {
	rec := DeskOOMSelfTest(context.Background(), DeskOOMSelfTestConfig{
		Base: DeskSessionWorkerConfig{Bin: "/nonexistent/never-spawned", WorkerArg: "desk-session", CgroupRoot: t.TempDir()},
		Role: "engineering",
	})
	if rec.OK || rec.Worker.PID != 0 || rec.Worker.CellSent {
		t.Fatalf("a role with no worker budget ran: %+v", rec)
	}
}

// O3: the fixed cell touches every page it allocates, and it is the only
// cell the leg runs (a constant; the config has no source).
func TestDeskOOMSelfTestCellTouchesEveryPage(t *testing.T) {
	if !strings.Contains(deskOOMSelfTestCell, "i += 4096") || !strings.Contains(deskOOMSelfTestCell, "b[i] = 1") {
		t.Fatalf("the runaway cell does not touch its pages:\n%s", deskOOMSelfTestCell)
	}
}

// O4: the daemon's identity is parsed from /proc/self/stat after the last
// ')' (a comm may hold spaces and parentheses).
func TestParseProcStatStartTicks(t *testing.T) {
	stat := "812 (autoputer (x) y) S 1 812 812 0 -1 4194560 100 0 0 0 5 3 0 0 20 0 9 0 4411 1000 200 18446744073709551615"
	got, err := parseProcStatStartTicks(stat)
	if err != nil || got != 4411 {
		t.Fatalf("start ticks = %d, %v; want 4411", got, err)
	}
	if _, err := parseProcStatStartTicks("812 (short) S 1"); err == nil {
		t.Fatal("a truncated stat line parsed")
	}
}

// Review bunch 22 item 2: the runaway is refused when the guest has less
// available memory than the budget plus the headroom, before any worker
// starts; both numbers are in the receipt.
func TestDeskOOMSelfTestRefusesWhenTheGuestIsShortOfMemory(t *testing.T) {
	restore := deskOOMMemAvailable
	deskOOMMemAvailable = func() (uint64, error) { return 1 << 30, nil }
	defer func() { deskOOMMemAvailable = restore }()
	rec := DeskOOMSelfTest(context.Background(), DeskOOMSelfTestConfig{
		Base: DeskSessionWorkerConfig{Bin: "/nonexistent/never-spawned", WorkerArg: "desk-session", CgroupRoot: t.TempDir()},
	})
	if rec.OK || rec.Worker.CellSent || rec.Worker.PID != 0 || rec.Sibling.PID != 0 {
		t.Fatalf("the leg ran with 1 GiB available: %+v", rec)
	}
	if rec.MemAvailableBytes != 1<<30 || rec.RequiredAvailableBytes != 768<<20+deskOOMHeadroomBytes {
		t.Fatalf("receipt numbers: available %d required %d", rec.MemAvailableBytes, rec.RequiredAvailableBytes)
	}
	if !strings.Contains(rec.Worker.Error, "MemAvailable") {
		t.Fatalf("refusal does not name MemAvailable: %q", rec.Worker.Error)
	}
}

// Review bunch 22 item 3: no sibling, no runaway.
func TestDeskOOMSelfTestAbortsWhenTheSiblingCannotStart(t *testing.T) {
	restore := deskOOMMemAvailable
	deskOOMMemAvailable = func() (uint64, error) { return 64 << 30, nil }
	defer func() { deskOOMMemAvailable = restore }()
	rec := DeskOOMSelfTest(context.Background(), DeskOOMSelfTestConfig{
		Base: DeskSessionWorkerConfig{Bin: "/nonexistent/never-spawned", WorkerArg: "desk-session", CgroupRoot: t.TempDir()},
	})
	if rec.OK || rec.Sibling.Error == "" {
		t.Fatalf("sibling spawn did not fail: %+v", rec.Sibling)
	}
	if rec.Worker.PID != 0 || rec.Worker.CellSent {
		t.Fatalf("the runaway started without a live sibling: %+v", rec.Worker)
	}
}

// Review bunch 27 E3: cell_sent is true only when the cell frame is known to
// have been written. Eval's refusals before the write (the worker was already
// dead, the frame would not marshal, the write failed) do not count; every
// error after the write does.
func TestDeskOOMRunawayCellSentIsFalseWhenEvalRefusesBeforeTheWrite(t *testing.T) {
	// A worker that is already dead makes the real Eval answer "desk session
	// worker dead" before it writes anything, with no process or cgroup.
	w := &DeskSessionWorker{dead: true, budget: &workerCgroup{path: t.TempDir()}}
	var out DeskOOMWorker
	runDeskOOMRunaway(context.Background(), w, "print(1)", time.Second, func() bool { return false }, &out)
	if !strings.Contains(out.Result, "desk session worker dead") {
		t.Fatalf("the real Eval did not refuse a dead worker as expected: %q", out.Result)
	}
	if out.CellSent {
		t.Fatalf("cell_sent is true though Eval refused before the frame write: %+v", out)
	}
}

func TestDeskEvalNotSentNamesTheRefusalsBeforeTheWrite(t *testing.T) {
	for _, tc := range []struct {
		err     error
		notSent bool
	}{
		{nil, false},
		{errors.New("desk session worker dead"), true},
		{errors.New("desk eval marshal: json: unsupported value"), true},
		{fmt.Errorf("desk eval write: %w", io.ErrClosedPipe), true},
		// After the write: the read side, the host, the timeout.
		{io.EOF, false},
		{errors.New("desk eval decode: invalid character"), false},
		{errors.New("desk eval: id mismatch got \"a\" want \"b\""), false},
		{context.DeadlineExceeded, false},
		{errForgedFrame, false},
	} {
		if got := deskEvalNotSent(tc.err); got != tc.notSent {
			t.Errorf("deskEvalNotSent(%v) = %v, want %v", tc.err, got, tc.notSent)
		}
	}
}
