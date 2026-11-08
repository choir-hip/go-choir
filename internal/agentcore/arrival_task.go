package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// Host start on arrival (enforce flip 13.5, F6 S2a).
//
// Capture intake does not run in management's main session: that session's
// label set only grows, and a record carries its sending run's whole set, so
// a story opened from the main run would ask at publication for whatever
// management ever read. When a captures arrival is committed to management's
// inbox, the host mints the management task row and its opening Cast in the
// same CommitInboxRecords batch. The Cast is composed from the arrival alone:
//
//   - its objective is a constant, and no text a model wrote is on it;
//   - its provenance labels are labelsOfRecord of the arrival, never a run's;
//   - it is addressed to the task inbox with a non-system emitter, so the
//     store's wake predicate starts the task exactly as a Delegate Cast does.
//
// The arrival and the task are one atomic batch: there is no window between
// the record and the lease, and a replay finds the captures leased, mints
// nothing and conflicts on the command digest (announceCaptures reads that as
// "already announced"). Nothing runs at boot: a crash closes the open task
// with "interrupted by a restart" and the capture's crash budget spends one.
//
// Failure modes, written before the code (each has a test in
// arrival_task_test.go):
//
//	A1 the arrival commits and the task does not (or the reverse);
//	A2 a replay of the arrival mints a second task;
//	A3 a held arrival, or one with nothing free, mints a task;
//	A4 a disposed or leased capture is leased again, or a task exceeds
//	   MaxTaskCaptures;
//	A5 two records of one event open two tasks;
//	A6 any model-written text, or the labels of a run, reaches the Cast;
//	A7 management's main run is woken for an arrival that started a task;
//	A8 the minted task never starts (a system emitter does not wake);
//	A9 intake off (the owner's settings) still mints tasks;
//	A10 a store read failure loses the arrival (it falls back to the plain
//	    arrival, which wakes management as before).

// managementIntakeObjective is the constant objective of every intake task
// Cast, host-minted or delegated. It is a fixed string in code.
const managementIntakeObjective = "Open, feed or decline every capture these records name."

// intakeTaskCastBody is the Cast body of an intake task: the delegated
// record ids, the leased captures and their titles, all host-supplied.
func intakeTaskCastBody(records, captures, titles []string) string {
	body, _ := json.Marshal(map[string]any{"kind": yaegikernel.TaskKindIntake, "records": records, "captures": captures, "titles": titles})
	return string(body)
}

// hostStartsIntake reports whether a captures record is one the host starts
// a task for: those a task can fully dispose of (plain poll arrivals and
// corroborated, unresolved and released event records). Held records wait on
// their events. An event-aged record (official_single_outlet) needs a second
// source, which a task cannot ask research for (13.6): it stays management's,
// which asks research, then re-delegates with the clean stamp or declines.
func hostStartsIntake(rec types.CommitmentRecord) bool {
	c := rec.Captures
	if c == nil || len(c.Items) == 0 || c.Held {
		return false
	}
	if c.Event != nil {
		switch c.Event.Reason {
		case types.CaptureEventOfficial:
			return false
		}
	}
	return true
}

type arrivalItem struct{ id, title string }

// withIntakeTask returns req with the intake tasks of its captures records
// minted into it, or req unchanged when none is due. An error leaves req
// unchanged for the caller to commit as the plain arrival.
func (rt *Runtime) withIntakeTask(ctx context.Context, req types.CommitInboxRecordsRequest) (types.CommitInboxRecordsRequest, bool, error) {
	if rt == nil || rt.store == nil || len(req.Tasks) > 0 {
		return req, false, nil
	}
	var due []int
	for i, rec := range req.Records {
		if hostStartsIntake(rec) {
			due = append(due, i)
		}
	}
	if len(due) == 0 || !rt.intakeSurfacesEnabled(ctx, req.OwnerID, req.ComputerID) {
		return req, false, nil
	}
	parent := persistentManagementAgentID(req.OwnerID)
	records := append([]types.CommitmentRecord(nil), req.Records...)

	// Records of one world event go to one task (as Delegate's grouping);
	// every other record is its own group.
	var groups [][]int
	byEvent := map[string]int{}
	for _, i := range due {
		if ev := records[i].Captures.Event; ev != nil && strings.TrimSpace(ev.EventID) != "" {
			if g, ok := byEvent[ev.EventID]; ok {
				groups[g] = append(groups[g], i)
				continue
			}
			byEvent[ev.EventID] = len(groups)
		}
		groups = append(groups, []int{i})
	}

	var tasks []types.ManagementTask
	var casts []types.CommitmentRecord
	started := map[int][]string{}
	for _, g := range groups {
		var free []arrivalItem
		var sources []int
		seen := map[string]bool{}
		for _, i := range g {
			contributed := false
			for _, it := range records[i].Captures.Items {
				id := strings.TrimSpace(it.CaptureID)
				if id == "" || seen[id] {
					continue
				}
				ok, err := rt.captureDelegable(ctx, req.OwnerID, req.ComputerID, id)
				if err != nil {
					return req, false, fmt.Errorf("intake task: %w", err)
				}
				if !ok {
					continue
				}
				seen[id] = true
				free = append(free, arrivalItem{id, strings.TrimSpace(it.Title)})
				contributed = true
			}
			if contributed {
				sources = append(sources, i)
			}
		}
		for chunk := 0; len(free) > 0; chunk++ {
			n := min(yaegikernel.MaxTaskCaptures, len(free))
			part := free[:n]
			free = free[n:]
			var leased, titles, delegated []string
			var labels types.LabelSet
			for _, it := range part {
				leased = append(leased, it.id)
				if it.title != "" {
					titles = append(titles, it.title)
				}
			}
			for _, i := range sources {
				delegated = append(delegated, records[i].RecordID)
				labels = labels.Union(labelsOfRecord(records[i]))
			}
			first := records[sources[0]].RecordID
			taskID := managementTaskID("arrival:"+req.CommandID, "intake", chunk)
			inbox := types.ManagementTaskInbox(taskID)
			cast := types.CommitmentRecord{
				SchemaID: types.CommitmentRecordSchemaV1, RecordID: fmt.Sprintf("%s:intake:%d", first, chunk),
				Kind: types.CommitmentKindDirective, Discrepancy: types.DiscrepancyUnresolved,
				Directive: &types.CommitmentDirective{Subtype: types.CommitmentDirectiveCast, Objective: managementIntakeObjective,
					Body: intakeTaskCastBody(delegated, leased, titles)},
				Cast: &types.CommitmentCast{Captures: leased},
				// A host act: no run wrote it, so no run's labels or model are
				// on it. CommittedAt is left to the commit, so a replay hashes
				// the same.
				Provenance:  types.CommitmentProvenance{AgentID: parent, Desk: parent, Origin: types.OriginSystem, Labels: labels.Strings()},
				EmitterKind: types.CommitmentEmitterAgent, ComputerID: req.ComputerID,
				Inbox: inbox, SenderInbox: parent, Addressee: inbox,
				RelatedIDs: delegated, ParentID: delegated[0],
			}
			casts = append(casts, cast)
			tasks = append(tasks, types.ManagementTask{
				TaskID: taskID, OwnerID: req.OwnerID, ComputerID: req.ComputerID, Kind: types.ManagementTaskIntake,
				Objective: managementIntakeObjective, ParentInbox: parent, TaskInbox: inbox,
				ConversationID: types.InboxConversationID(parent, inbox), OpeningRecordID: cast.RecordID,
				DelegatedRecordIDs: delegated, LeasedCaptures: leased, DeadlineSeconds: int(managementTaskDeadline / time.Second),
				State: types.ManagementTaskQueued,
			})
			for _, i := range sources {
				started[i] = append(started[i], taskID)
			}
		}
	}
	if len(tasks) == 0 {
		return req, false, nil
	}
	for i, ids := range started {
		c := *records[i].Captures
		c.StartedTasks = ids
		records[i].Captures = &c
	}
	req.Records = append(records, casts...)
	req.Tasks = tasks
	digest, err := store.ComputeCommitInboxRecordsDigest(req)
	if err != nil {
		return req, false, err
	}
	req.CommandDigest = digest
	return req, true, nil
}
