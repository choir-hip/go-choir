package store

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/deskprotocol"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The store reads the desk protocol's transition table for the lifecycle
// checks below. These tests pin that reading to the old inline checks (frozen
// here as oracles) and to every transition the store performs, so the table
// cannot omit a legal move and wedge live work, nor allow a new one.

var (
	allWorkStatuses = []types.WorkItemStatus{"", "bogus", types.WorkItemOpen, types.WorkItemCompleted, types.WorkItemCancelled, types.WorkItemRefused}
	allTrajStatuses = []types.TrajectoryStatus{"", "bogus", types.TrajectoryLive, types.TrajectorySettled, types.TrajectoryCancelled}
	allCapsule      = []types.EngineeringCapsuleDisposition{"", "bogus", types.EngineeringCapsuleUnbound, types.EngineeringCapsuleActive,
		types.EngineeringCapsuleFreezeRequested, types.EngineeringCapsuleFrozen, types.EngineeringCapsuleRevokeRequested, types.EngineeringCapsuleRevoked}
)

// legacyWorkItemTerminal is workItemTerminal before the table.
func legacyWorkItemTerminal(status types.WorkItemStatus) bool {
	return status == types.WorkItemCompleted || status == types.WorkItemCancelled || status == types.WorkItemRefused
}

// legacyValidEngineeringCapsuleTransition is validEngineeringCapsuleTransition
// before the table.
func legacyValidEngineeringCapsuleTransition(current, next types.EngineeringCapsuleDisposition, intentRef, ackRef string) bool {
	switch next {
	case types.EngineeringCapsuleFreezeRequested:
		return current == types.EngineeringCapsuleActive && intentRef != "" && ackRef == ""
	case types.EngineeringCapsuleFrozen:
		return current == types.EngineeringCapsuleFreezeRequested && intentRef != "" && ackRef != ""
	case types.EngineeringCapsuleRevokeRequested:
		return (current == types.EngineeringCapsuleUnbound || current == types.EngineeringCapsuleActive || current == types.EngineeringCapsuleFreezeRequested || current == types.EngineeringCapsuleFrozen) && intentRef != "" && ackRef == ""
	case types.EngineeringCapsuleRevoked:
		return current == types.EngineeringCapsuleRevokeRequested && intentRef != "" && strings.HasPrefix(ackRef, "capsule-revoke:sha256:") &&
			types.ValidSHA256Digest(strings.TrimPrefix(ackRef, "capsule-revoke:"))
	default:
		return false
	}
}

func TestTableReadingEqualsTheInlineChecksItReplaced(t *testing.T) {
	for _, st := range allWorkStatuses {
		if got, want := workItemTerminal(st), legacyWorkItemTerminal(st); got != want {
			t.Errorf("workItemTerminal(%q) = %v, was %v", st, got, want)
		}
	}
	// The trajectory checks were `Status != TrajectoryLive` for both settle
	// and cancel.
	for _, st := range allTrajStatuses {
		for _, next := range []types.TrajectoryStatus{types.TrajectorySettled, types.TrajectoryCancelled} {
			if got, want := trajectoryMayMoveTo(st, next), st == types.TrajectoryLive; got != want {
				t.Errorf("trajectoryMayMoveTo(%q,%q) = %v, was %v", st, next, got, want)
			}
		}
	}
	validAck := "capsule-revoke:sha256:" + strings.Repeat("a", 64)
	for _, cur := range allCapsule {
		for _, next := range allCapsule {
			for _, intent := range []string{"", "i"} {
				for _, ack := range []string{"", "a", validAck, "capsule-revoke:sha256:zz"} {
					got := validEngineeringCapsuleTransition(cur, next, intent, ack)
					want := legacyValidEngineeringCapsuleTransition(cur, next, intent, ack)
					if got != want {
						t.Errorf("capsule %q -> %q intent=%q ack=%q: %v, was %v", cur, next, intent, ack, got, want)
					}
				}
			}
		}
	}
}

// Every state in the table is a value of the types constant set, and every
// constant of those sets is in the table (as a from or a to).
func TestTableStatesAreTheTypesConstants(t *testing.T) {
	want := map[deskprotocol.Machine][]string{
		deskprotocol.MachineWorkItem:   {string(types.WorkItemOpen), string(types.WorkItemCompleted), string(types.WorkItemCancelled), string(types.WorkItemRefused)},
		deskprotocol.MachineTrajectory: {string(types.TrajectoryLive), string(types.TrajectorySettled), string(types.TrajectoryCancelled)},
		deskprotocol.MachineEngineeringAssignment: {string(types.EngineeringAssignmentOpen), string(types.EngineeringAssignmentBound),
			string(types.EngineeringAssignmentCompleted), string(types.EngineeringAssignmentFailed), string(types.EngineeringAssignmentCancelled)},
		deskprotocol.MachineEngineeringCapsule: {string(types.EngineeringCapsuleUnbound), string(types.EngineeringCapsuleActive),
			string(types.EngineeringCapsuleFreezeRequested), string(types.EngineeringCapsuleFrozen), string(types.EngineeringCapsuleRevokeRequested), string(types.EngineeringCapsuleRevoked)},
	}
	for m, states := range want {
		in := map[string]bool{}
		for _, tr := range deskprotocol.Transitions() {
			if tr.Machine == m {
				if tr.From != "" {
					in[tr.From] = true
				}
				in[tr.To] = true
			}
		}
		set := map[string]bool{}
		for _, s := range states {
			set[s] = true
			if !in[s] {
				t.Errorf("%s: constant %q is in no row of the table", m, s)
			}
		}
		for s := range in {
			if !set[s] {
				t.Errorf("%s: table state %q is not a types constant", m, s)
			}
		}
	}
	if !deskprotocol.Terminal(deskprotocol.MachineEngineeringAssignment, string(types.EngineeringAssignmentCompleted)) ||
		!types.EngineeringAssignmentCompleted.Terminal() {
		t.Error("assignment terminal states disagree with Disposition.Terminal")
	}
	for _, d := range []types.EngineeringAssignmentDisposition{types.EngineeringAssignmentOpen, types.EngineeringAssignmentBound,
		types.EngineeringAssignmentCompleted, types.EngineeringAssignmentFailed, types.EngineeringAssignmentCancelled} {
		if d.Terminal() != deskprotocol.Terminal(deskprotocol.MachineEngineeringAssignment, string(d)) {
			t.Errorf("assignment %q: Disposition.Terminal=%v, table terminal=%v", d, d.Terminal(), !d.Terminal())
		}
	}
}

func mustAllow(t *testing.T, m deskprotocol.Machine, from, to string) {
	t.Helper()
	if !deskprotocol.Allowed(m, from, to) {
		t.Errorf("the store performed %s %s -> %s and the table does not allow it", m, from, to)
	}
}

// Drive the store's real lifecycle paths and check every move they make is a
// row of the table.
func TestStoreLifecyclePathsMakeOnlyTableTransitions(t *testing.T) {
	ctx := context.Background()
	observe := func(s *Store, start types.StartLifecycleRequest) (types.TrajectoryStatus, types.WorkItemStatus) {
		snap, err := s.GetLifecycleSnapshot(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID)
		if err != nil {
			t.Fatalf("snapshot: %v", err)
		}
		return snap.Trajectory.Status, snap.WorkItems[0].Status
	}

	t.Run("work settles and the trajectory settles", func(t *testing.T) {
		s := openTestStore(t)
		start := lifecycleStartFixture()
		if _, err := s.StartLifecycle(ctx, start); err != nil {
			t.Fatal(err)
		}
		run := lifecycleRunFixture(start, "run-table-settle", types.RunPending)
		if err := s.CreateRun(ctx, run); err != nil {
			t.Fatal(err)
		}
		tr0, w0 := observe(s, start)
		settleInitialLifecycleWork(t, s, start, "command-table-work")
		_, w1 := observe(s, start)
		mustAllow(t, deskprotocol.MachineWorkItem, string(w0), string(w1))
		run.State = types.RunCompleted
		run.UpdatedAt = time.Now().UTC()
		run.FinishedAt = &run.UpdatedAt
		if err := s.UpdateRun(ctx, run); err != nil {
			t.Fatal(err)
		}
		tr1, _ := observe(s, start)
		if tr1 != types.TrajectorySettled {
			t.Fatalf("trajectory %s, want settled", tr1)
		}
		mustAllow(t, deskprotocol.MachineTrajectory, string(tr0), string(tr1))
	})

	t.Run("trajectory cancelled cancels its work", func(t *testing.T) {
		s := openTestStore(t)
		start := lifecycleStartFixture()
		if _, err := s.StartLifecycle(ctx, start); err != nil {
			t.Fatal(err)
		}
		snap, _ := s.GetLifecycleSnapshot(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID)
		tr0, w0 := observe(s, start)
		cancel := types.CancelLifecycleRequest{
			OwnerID: start.OwnerID, ComputerID: start.ComputerID, CommandID: "command-table-cancel", TrajectoryID: start.TrajectoryID,
			ExpectedLifecycleVersion: snap.Trajectory.LifecycleVersion, ExpectedHeadRevisionID: snap.HeadRevision.RevisionID, Reason: "table",
		}
		cancel.CommandDigest, _ = ComputeCancelLifecycleDigest(cancel)
		if _, err := s.CancelLifecycleTrajectory(ctx, cancel); err != nil {
			t.Fatal(err)
		}
		tr1, w1 := observe(s, start)
		mustAllow(t, deskprotocol.MachineTrajectory, string(tr0), string(tr1))
		mustAllow(t, deskprotocol.MachineWorkItem, string(w0), string(w1))
		// And nothing leaves a cancelled trajectory or its work.
		if _, err := s.CancelLifecycleTrajectory(ctx, cancel); err != nil {
			t.Logf("replay of the cancel: %v", err)
		}
		tr2, w2 := observe(s, start)
		if tr2 != tr1 || w2 != w1 {
			t.Fatalf("a terminal state moved: %s/%s -> %s/%s", tr1, w1, tr2, w2)
		}
	})

	t.Run("a rejected update refuses its producer work", func(t *testing.T) {
		s := openTestStore(t)
		start := lifecycleStartFixture()
		start.InitialDocument.DocID = "document-table-reject"
		start.InitialRevision.RevisionID = "revision-table-reject-v0"
		start.SubjectRefs["doc_id"] = start.InitialDocument.DocID
		start.Agent.AgentID = "texture:" + start.InitialDocument.DocID
		start.Agent.ChannelID = start.InitialDocument.DocID
		start.StartRequestDigest, _ = ComputeStartLifecycleRequestDigest(start)
		if _, err := s.StartLifecycle(ctx, start); err != nil {
			t.Fatal(err)
		}
		queue := queueLifecycleUpdateFixture(t, s, start, "command-table-queue")
		queue.UpdateID, queue.ProducerUpdateID = "update-table", "producer-update-table"
		queue.ProducerAgentID = start.Agent.AgentID
		queue.WorkDisposition = types.WorkItemCompleted
		queue.WorkItemID = start.InitialWork.WorkItemID
		queue.CommandDigest, _ = ComputeQueueLifecycleUpdateDigest(queue)
		if _, err := s.QueueLifecycleUpdate(ctx, queue); err != nil {
			t.Fatal(err)
		}
		_, w0 := observe(s, start)
		apply := types.ApplyLifecycleUpdateRequest(queue)
		apply.CommandID, apply.CommandDigest = "command-table-apply", ""
		apply.Disposition = types.UpdateRejected
		apply.DispositionRef = "evidence://table/1"
		apply.Reason = "table"
		apply.WorkDisposition = types.WorkItemRefused
		apply.CommandDigest, _ = ComputeApplyLifecycleUpdateDigest(apply)
		if _, err := s.ApplyLifecycleUpdate(ctx, apply); err != nil {
			t.Fatal(err)
		}
		_, w1 := observe(s, start)
		mustAllow(t, deskprotocol.MachineWorkItem, string(w0), string(w1))
	})
}

// Every place in the runtime that assigns a lifecycle status or disposition
// names a destination the table has a row into, so a legal transition cannot
// be missing from the table's destinations.
func TestEveryAssignedLifecycleDestinationIsInTheTable(t *testing.T) {
	prefixes := map[string]deskprotocol.Machine{
		"WorkItem":              deskprotocol.MachineWorkItem,
		"Trajectory":            deskprotocol.MachineTrajectory,
		"EngineeringAssignment": deskprotocol.MachineEngineeringAssignment,
		"EngineeringCapsule":    deskprotocol.MachineEngineeringCapsule,
	}
	// constant name -> state value, from the types package source.
	values := typesConstValues(t)
	to := map[deskprotocol.Machine]map[string]bool{}
	for _, tr := range deskprotocol.Transitions() {
		if to[tr.Machine] == nil {
			to[tr.Machine] = map[string]bool{}
		}
		to[tr.Machine][tr.To] = true
	}
	found := 0
	for _, dir := range []string{".", "../textureowner", "../agentcore", "../autoputer"} {
		files, _ := filepath.Glob(filepath.Join(dir, "*.go"))
		for _, f := range files {
			if strings.HasSuffix(f, "_test.go") {
				continue
			}
			fset := token.NewFileSet()
			af, err := parser.ParseFile(fset, f, nil, parser.SkipObjectResolution)
			if err != nil {
				t.Fatal(err)
			}
			ast.Inspect(af, func(n ast.Node) bool {
				as, ok := n.(*ast.AssignStmt)
				if !ok || as.Tok != token.ASSIGN || len(as.Lhs) != len(as.Rhs) {
					return true
				}
				for i, l := range as.Lhs {
					sel, ok := l.(*ast.SelectorExpr)
					if !ok || (sel.Sel.Name != "Status" && sel.Sel.Name != "Disposition" && sel.Sel.Name != "CapsuleDisposition") {
						continue
					}
					r, ok := as.Rhs[i].(*ast.SelectorExpr)
					if !ok {
						continue
					}
					if x, ok := r.X.(*ast.Ident); !ok || x.Name != "types" {
						continue
					}
					for p, m := range prefixes {
						if !strings.HasPrefix(r.Sel.Name, p) {
							continue
						}
						// "EngineeringAssignmentX" constants that are other types
						// (kinds, results) do not end up in values.
						v, known := values[m][r.Sel.Name]
						if !known {
							continue
						}
						found++
						if !to[m][v] {
							t.Errorf("%s assigns %s (%q) of machine %s, which no table row moves to", f, r.Sel.Name, v, m)
						}
					}
				}
				return true
			})
		}
	}
	if found < 12 {
		t.Fatalf("the scan found only %d assignments; it has stopped seeing the code", found)
	}
}

// typesConstValues reads the state constants out of the types package source.
func typesConstValues(t *testing.T) map[deskprotocol.Machine]map[string]string {
	t.Helper()
	typeToMachine := map[string]deskprotocol.Machine{
		"WorkItemStatus": deskprotocol.MachineWorkItem, "TrajectoryStatus": deskprotocol.MachineTrajectory,
		"EngineeringAssignmentDisposition": deskprotocol.MachineEngineeringAssignment,
		"EngineeringCapsuleDisposition":    deskprotocol.MachineEngineeringCapsule,
	}
	out := map[deskprotocol.Machine]map[string]string{}
	for _, m := range typeToMachine {
		out[m] = map[string]string{}
	}
	files, _ := filepath.Glob("../types/*.go")
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		af, err := parser.ParseFile(token.NewFileSet(), f, src, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range af.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok || gd.Tok != token.CONST {
				continue
			}
			for _, sp := range gd.Specs {
				vs := sp.(*ast.ValueSpec)
				id, ok := vs.Type.(*ast.Ident)
				if !ok || len(vs.Values) != 1 {
					continue
				}
				m, ok := typeToMachine[id.Name]
				lit, isLit := vs.Values[0].(*ast.BasicLit)
				if !ok || !isLit {
					continue
				}
				out[m][vs.Names[0].Name] = strings.Trim(lit.Value, `"`)
			}
		}
	}
	return out
}
