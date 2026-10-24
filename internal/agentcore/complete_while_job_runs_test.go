package agentcore

import (
	"context"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/capsulejob"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// Failure 23 (docs/problems/engineering-completion-path-cluster-2026-10-20.md):
// a cell that stages Complete while the run's own capsule job runs is
// refused, as Freeze already is, so a round cannot end in the middle of its
// own build. Cells without Complete, runs with no running job and desks
// without capsule jobs are unchanged.
func TestCompleteIsRefusedWhileTheRunsJobRuns(t *testing.T) {
	running := &fakeJobTable{running: true, view: yaegikernel.JobView{ID: "job-build", State: capsulejob.StateRunning, Argv: []string{"sh", "-c", "npm ci"}}}
	idle := &fakeJobTable{}
	complete := []yaegikernel.StagedIntent{{Kind: yaegikernel.IntentComplete, LocalID: "tray-1", Result: yaegikernel.CompleteBlocked}}
	poll := []yaegikernel.StagedIntent{{Kind: yaegikernel.IntentNote, LocalID: "tray-1", ToDesk: "management:m", Body: "still building"}}

	err := refuseCompleteWhileJobRuns(running, "run-1", complete)
	if err == nil {
		t.Fatal("Complete was accepted while the run's job runs")
	}
	for _, want := range []string{"job-build", "still running", "turn ends here", "choir.Stop"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("refusal %q does not say %q", err, want)
		}
	}
	if err := refuseCompleteWhileJobRuns(idle, "run-1", complete); err != nil {
		t.Fatalf("Complete with no running job was refused: %v", err)
	}
	if err := refuseCompleteWhileJobRuns(running, "run-1", poll); err != nil {
		t.Fatalf("a cell without Complete was refused while the job runs: %v", err)
	}
	if err := refuseCompleteWhileJobRuns(nil, "run-1", complete); err != nil {
		t.Fatalf("a desk without capsule jobs was refused: %v", err)
	}
}

// The refusal comes before any side effect: commit returns it without
// recording the cell's fate.
func TestCompleteRefusalLeavesNoCellFate(t *testing.T) {
	rt, st := testRuntime(t)
	rt.jobTable = &fakeJobTable{running: true, view: yaegikernel.JobView{ID: "job-build", State: capsulejob.StateRunning}}
	scope := ReductionScope{
		FromAgentID: "engineering:asg-1", DeskAgentID: "engineering:asg-1", FromRole: "engineering",
		ChannelID: "engineering:asg-1", RunID: "run-1", OwnerID: "owner-1", ComputerID: "computer-1",
		ReturnTo: "management:m", CellID: "cell-1",
	}
	r := &rlmCallReduction{active: true, mb: rt, st: st, ledger: st, scope: scope}
	err := r.commit(context.Background(), []yaegikernel.StagedIntent{{Kind: yaegikernel.IntentComplete, LocalID: "tray-1", Result: yaegikernel.CompleteBlocked}})
	if err == nil || !strings.Contains(err.Error(), "still running") {
		t.Fatalf("commit = %v, want the running-job refusal", err)
	}
	if fate, found, ferr := CellFate(context.Background(), st, "owner-1", "run-1", "cell-1"); ferr != nil || found {
		t.Fatalf("refused cell recorded a fate %q (found %v, err %v)", fate, found, ferr)
	}
}
