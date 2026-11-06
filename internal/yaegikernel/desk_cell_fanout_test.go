package yaegikernel

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func fanOutSessionEval(t *testing.T) func(context.Context, string) (SessionResult, error) {
	sess, err := NewSession(NewAllowlist(DefaultSafeStdlibPackagesList()...), nil)
	if err != nil {
		t.Fatal(err)
	}
	return func(ctx context.Context, src string) (SessionResult, error) {
		res, err := sess.Eval(ctx, src)
		return SessionResult{Stdout: res.Stdout}, err
	}
}

// The probe passes on the pinned yaegi (the real cell, an in-process session).
func TestDeskCellFanOutPasses(t *testing.T) {
	got := runDeskCellFanOut(context.Background(), fanOutSessionEval(t))
	if !got.OK || got.Rounds != fanOutRounds || got.Goroutines != fanOutGoroutines || got.Error != "" {
		t.Fatalf("fan-out probe: %+v", got)
	}
}

// A cell panic is reported with its message, as ok=false, not a crash.
func TestDeskCellFanOutReportsAPanic(t *testing.T) {
	got := runDeskCellFanOut(context.Background(), func(context.Context, string) (SessionResult, error) {
		return SessionResult{Error: "panic: call of nil function"}, nil
	})
	if got.OK || !strings.Contains(got.Error, "call of nil function") {
		t.Fatalf("panic not reported: %+v", got)
	}
	got = runDeskCellFanOut(context.Background(), func(context.Context, string) (SessionResult, error) {
		return SessionResult{Stdout: "FANOUT_FAIL panic: call of nil function\n"}, nil
	})
	if got.OK || !strings.Contains(got.Error, "call of nil function") {
		t.Fatalf("recovered panic not reported: %+v", got)
	}
	got = runDeskCellFanOut(context.Background(), func(context.Context, string) (SessionResult, error) {
		return SessionResult{}, errors.New("desk session worker dead")
	})
	if got.OK || !strings.Contains(got.Error, "worker dead") {
		t.Fatalf("transport error not reported: %+v", got)
	}
	got = runDeskCellFanOut(context.Background(), func(context.Context, string) (SessionResult, error) {
		return SessionResult{Stdout: ""}, nil
	})
	if got.OK {
		t.Fatalf("empty output accepted: %+v", got)
	}
}

// The judge refuses a serving worker whose fan-out failed or never ran.
func TestDeskCellSelfTestJudgeRefusesAFailedFanOut(t *testing.T) {
	for name, fo := range map[string]*DeskCellFanOut{"missing": nil, "failed": {Error: "panic: call of nil function"}} {
		rec := DeskCellSelfTestReceipt{SpawnerUID: 1000, Workers: []DeskCellSelfTestWorker{{Role: "research", CellServes: true, FanOut: fo}}}
		if f := strings.Join(JudgeDeskCellSelfTest(rec), "\n"); !strings.Contains(f, "fanout") {
			t.Errorf("%s: judge accepted: %q", name, f)
		}
	}
}
