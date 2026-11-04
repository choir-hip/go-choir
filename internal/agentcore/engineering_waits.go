package agentcore

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Engineering waits, one record per assignment
// (docs/problems/selfdev-waits-untyped-and-invisible-2026-11-04.md).
//
// Every wait an engineering assignment spends time in, its admission's
// (the slot, the queue, an open operation, host memory, the platform
// update) and its build jobs' (host memory), is one types.WaitState here:
// the assignment's current wait and the time it has waited this attempt,
// by kind. Management reads it in its cell frame (ActingPack.Waits);
// Texture is told once per wait that has lasted waitAnnounceAfter, and
// once when it ends, through the engineering progress relay's record shape
// and switch (types.NewEngineeringWaitRelay); when the assignment ends,
// its minutes waited by kind are put on the ledger for management.
//
// The tracker is memory: a restart ends it. A crash closes the assignment
// anyway (the restart rule); a planned restart loses the attempt's tally
// so far, and its waits begin again.

// waitAnnounceAfter is how long a wait lasts before Texture is told: a
// passing wait (one admission retry, a few seconds of a job) is not the
// owner's business.
const waitAnnounceAfter = time.Minute

// waitTrackerIdleForget is how long an entry with no current wait is kept
// after its last change when its assignment's end was never seen.
const waitTrackerIdleForget = 6 * time.Hour

type spawnMark struct {
	took time.Duration
	at   time.Time
}

type assignmentWaitTracker struct {
	mu      sync.Mutex
	entries map[string]*assignmentWaitEntry
	// spawns is how long each attempt's spawn saga took (the round receipt's
	// capsule spawn, round_receipt.go); closeAssignmentWaits puts it on the
	// ledger and drops it.
	spawns map[string]spawnMark
}

type assignmentWaitEntry struct {
	ownerID, computerID, assignmentID string
	attempt                           uint64
	textureInbox                      string
	current                           *types.WaitState
	// episode counts the attempt's waits; announced says Texture was told
	// of the current one.
	episode   int
	announced bool
	// startRec is the current wait's start relay, built once, so a retry
	// re-sends the same bytes under its fixed id (review r1 DeepSeek F1).
	startRec *types.CommitmentRecord
	tally    types.WaitTally
	// jobTally is the part of tally that build jobs waited (a wait whose
	// subject is not the assignment's admission): the round receipt takes
	// it from the engineer's window, the rest from cast to bind.
	jobTally types.WaitTally
	updated  time.Time
}

func assignmentWaitKey(assignmentID string, attempt uint64) string {
	return strings.TrimSpace(assignmentID) + "#" + strconv.FormatUint(attempt, 10)
}

func (rt *Runtime) assignmentWaitNow() time.Time { return rt.engineeringAdmissionNow() }

// waitRelayPlan is a relay to mint after the tracker's lock is released.
type waitRelayPlan struct {
	ownerID, computerID string
	rec                 types.CommitmentRecord
}

// observeAssignmentWait records that the assignment waits now; build makes
// the wait, given when it began (the current wait's start when the kind
// is unchanged). It returns the relay to mint, if the wait just crossed
// waitAnnounceAfter.
func (rt *Runtime) observeAssignmentWait(ctx context.Context, assignment types.EngineeringAssignment, build func(since time.Time) *types.WaitState) {
	if rt == nil || assignment.Disposition.Terminal() {
		// An ended assignment waits for nothing; a late event must not
		// bring its entry back (review r1 DeepSeek F2).
		return
	}
	now := rt.assignmentWaitNow()
	binding := assignment.Binding
	key := assignmentWaitKey(assignment.AssignmentID, binding.Attempt)
	textureInbox, _ := types.ConversationCounterpart(binding.ParentConversationID, binding.ParentAgentID)
	t := &rt.assignmentWaits
	var plans []waitRelayPlan
	t.mu.Lock()
	if t.entries == nil {
		t.entries = map[string]*assignmentWaitEntry{}
	}
	e := t.entries[key]
	if e == nil {
		e = &assignmentWaitEntry{ownerID: binding.OwnerID, computerID: binding.ComputerID, assignmentID: assignment.AssignmentID, attempt: binding.Attempt}
		t.entries[key] = e
	}
	if textureInbox != "" {
		e.textureInbox = textureInbox
	}
	since := now
	if e.current != nil {
		since = e.current.Since
	}
	w := build(since)
	if !w.Active() {
		t.mu.Unlock()
		return
	}
	if e.current != nil && e.current.Kind != w.Kind {
		// A new cause is a new wait: the old one ends here.
		plans = append(plans, e.endLocked(now, "it now waits for "+w.Kind.Label())...)
		w.Since = now
		if w.Until.After(now) && !since.IsZero() {
			w.Until = now.Add(w.Until.Sub(since))
		}
	}
	if e.current == nil {
		e.episode++
		e.announced, e.startRec = false, nil
	}
	e.current, e.updated = w.Clone(), now
	if !e.announced && now.Sub(e.current.Since) >= waitAnnounceAfter && e.textureInbox != "" {
		if e.startRec == nil {
			if rec, ok := types.NewEngineeringWaitRelay(agentprofile.Engineering+":"+e.assignmentID, e.textureInbox, e.computerID, e.attempt, e.episodeID(),
				types.WaitRelayStart, e.current.TextureLine(now), now, map[string]string{"wait_kind": string(e.current.Kind)}); ok {
				e.startRec = &rec
			}
		}
		if e.startRec != nil {
			e.announced = true
			plans = append(plans, waitRelayPlan{ownerID: e.ownerID, computerID: e.computerID, rec: *e.startRec})
		}
	}
	t.mu.Unlock()
	rt.mintWaitRelays(ctx, plans, key)
}

// episodeID names the current wait: its ordinal and its start, so a
// tracker a planned restart reset never reuses a committed id.
func (e *assignmentWaitEntry) episodeID() string {
	since := int64(0)
	if e.current != nil {
		since = e.current.Since.Unix()
	}
	return fmt.Sprintf("%d-%d", e.episode, since)
}

// endLocked ends the entry's current wait: its time goes to the tally, and
// an announced wait gets its closing line. The caller holds the lock.
func (e *assignmentWaitEntry) endLocked(now time.Time, outcome string) []waitRelayPlan {
	if e.current == nil {
		return nil
	}
	w := e.current
	episode := e.episodeID()
	waited := now.Sub(w.Since)
	e.tally.Add(w.Kind, waited)
	if strings.HasPrefix(w.Subject, "job ") {
		e.jobTally.Add(w.Kind, waited)
	}
	e.current, e.updated = nil, now
	if !e.announced || e.textureInbox == "" {
		return nil
	}
	e.announced, e.startRec = false, nil
	body := fmt.Sprintf("Waited %d min for %s", max(int((waited+30*time.Second)/time.Minute), 1), firstNonEmpty(w.Resource, w.Kind.Label()))
	if outcome != "" {
		body += "; " + outcome
	}
	body += "."
	refs := map[string]string{"wait_kind": string(w.Kind)}
	for kind, seconds := range e.tally.Seconds {
		refs["waited_s:"+kind] = strconv.FormatInt(seconds, 10)
	}
	rec, ok := types.NewEngineeringWaitRelay(agentprofile.Engineering+":"+e.assignmentID, e.textureInbox, e.computerID, e.attempt, episode,
		types.WaitRelayEnd, body, now, refs)
	if !ok {
		return nil
	}
	return []waitRelayPlan{{ownerID: e.ownerID, computerID: e.computerID, rec: rec}}
}

// endAssignmentWait ends the assignment attempt's current wait, if any:
// its request was admitted, or its job started or ended.
func (rt *Runtime) endAssignmentWait(ctx context.Context, assignmentID string, attempt uint64, outcome string) {
	rt.endAssignmentWaitOf(ctx, assignmentID, attempt, "", outcome)
}

// endAdmissionWait ends the attempt's wait only when it is its admission's
// (subject "assignment …"): a job's wait that began first is not ended by
// the round's start (review r1 DeepSeek F6).
func (rt *Runtime) endAdmissionWait(ctx context.Context, assignmentID string, attempt uint64, outcome string) {
	rt.endAssignmentWaitOf(ctx, assignmentID, attempt, "assignment ", outcome)
}

func (rt *Runtime) endAssignmentWaitOf(ctx context.Context, assignmentID string, attempt uint64, subjectPrefix, outcome string) {
	if rt == nil {
		return
	}
	now := rt.assignmentWaitNow()
	key := assignmentWaitKey(assignmentID, attempt)
	t := &rt.assignmentWaits
	t.mu.Lock()
	var plans []waitRelayPlan
	if e := t.entries[key]; e != nil && (subjectPrefix == "" || (e.current != nil && strings.HasPrefix(e.current.Subject, subjectPrefix))) {
		plans = e.endLocked(now, outcome)
	}
	t.mu.Unlock()
	rt.mintWaitRelays(ctx, plans, key)
}

// assignmentWaitSummaryRecordID is the one record of an assignment
// attempt's minutes waited by kind.
func assignmentWaitSummaryRecordID(assignmentID string, attempt uint64) string {
	return fmt.Sprintf("engineering-waits:%s:%d", strings.TrimSpace(assignmentID), attempt)
}

// closeAssignmentWaits ends the attempt's tracking when the assignment
// ends: a current wait ends, and the attempt's minutes waited by kind go on
// the ledger for management, beside the assignment's report. Observational:
// a ledger failure is logged.
func (rt *Runtime) closeAssignmentWaits(ctx context.Context, assignment types.EngineeringAssignment) {
	if rt == nil {
		return
	}
	now := rt.assignmentWaitNow()
	key := assignmentWaitKey(assignment.AssignmentID, assignment.Binding.Attempt)
	t := &rt.assignmentWaits
	t.mu.Lock()
	e := t.entries[key]
	var plans []waitRelayPlan
	var tally, jobTally types.WaitTally
	if e != nil {
		plans = e.endLocked(now, "the assignment ended ("+string(assignment.Disposition)+")")
		tally, jobTally = e.tally.Clone(), e.jobTally.Clone()
		delete(t.entries, key)
	}
	spawn := t.spawns[key].took
	delete(t.spawns, key)
	t.mu.Unlock()
	rt.mintWaitRelays(ctx, plans, "")
	if rt.store == nil || (e == nil && spawn <= 0) {
		return
	}
	stamp := now.UTC().Format(time.RFC3339Nano)
	refs := map[string]string{"assignment_id": assignment.AssignmentID}
	for kind, seconds := range tally.Seconds {
		refs["waited_s:"+kind] = strconv.FormatInt(seconds, 10)
	}
	summary := tally.Summary()
	// The round receipt reads the spawn and the job waits from here
	// (types.BuildRoundReceipt); written for every ended attempt, waited or
	// not, so a round with no wait still has a spawn.
	refs[types.RoundRefSpawnS] = strconv.FormatInt(int64(spawn.Round(time.Second)/time.Second), 10)
	refs[types.RoundRefJobWaitedS] = strconv.FormatInt(int64(jobTally.Total()/time.Second), 10)
	if summary == "" {
		summary = "did not wait"
	}
	rec := types.CommitmentRecord{
		SchemaID:    types.CommitmentRecordSchemaV1,
		RecordID:    assignmentWaitSummaryRecordID(assignment.AssignmentID, assignment.Binding.Attempt),
		Discrepancy: types.DiscrepancyUnresolved,
		Prediction: types.CommitmentPrediction{
			Hypothesis:  fmt.Sprintf("engineering assignment %s (attempt %d) %s.", assignment.AssignmentID, assignment.Binding.Attempt, summary),
			CommittedAt: stamp,
		},
		Observation: types.CommitmentObservation{Excerpt: summary, SourceRef: assignment.AssignmentID, ObservedAt: stamp},
		Provenance:  types.CommitmentProvenance{AgentID: engineeringHoldObserverAgentID, ContextRef: assignment.Binding.TrajectoryID, CommittedAt: stamp},
		Addressee:   persistentManagementAgentID(assignment.Binding.OwnerID),
		SubjectRefs: refs, EvidenceRefs: []string{assignment.AssignmentID},
	}
	// Detached: the cancel paths that close are the ones whose context is
	// most likely done (review r1 DeepSeek F4).
	sctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()
	if _, err := rt.store.AppendCommitmentRecord(sctx, assignment.Binding.OwnerID, assignment.Binding.ComputerID, rec); err != nil {
		log.Printf("runtime: engineering waits: summary record for %s: %v", assignment.AssignmentID, err)
	}
}

// markAssignmentSpawn records how long the attempt's spawn saga took.
func (rt *Runtime) markAssignmentSpawn(assignmentID string, attempt uint64, took time.Duration) {
	if rt == nil || took <= 0 {
		return
	}
	t := &rt.assignmentWaits
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.spawns == nil {
		t.spawns = map[string]spawnMark{}
	}
	now := rt.assignmentWaitNow()
	// A mark that lands after its attempt closed is never read: forget it
	// with the idle entries (review r1 devin F1).
	for k, m := range t.spawns {
		if now.Sub(m.at) > waitTrackerIdleForget {
			delete(t.spawns, k)
		}
	}
	t.spawns[assignmentWaitKey(assignmentID, attempt)] = spawnMark{took: took, at: now}
}

// mintWaitRelays commits the relays into Texture's inbox when the
// computer's engineering.progress_to_texture setting is on. A start relay
// that fails to commit is un-announced (key), so the next check tries again.
func (rt *Runtime) mintWaitRelays(ctx context.Context, plans []waitRelayPlan, key string) {
	if rt == nil || rt.store == nil || len(plans) == 0 {
		return
	}
	unannounce := func(p waitRelayPlan) {
		if key == "" || p.rec.SubjectRefs["wait_phase"] != types.WaitRelayStart {
			return
		}
		t := &rt.assignmentWaits
		t.mu.Lock()
		if e := t.entries[key]; e != nil {
			e.announced = false
		}
		t.mu.Unlock()
	}
	for _, p := range plans {
		if !rt.computerSettingOn(context.WithoutCancel(ctx), p.ownerID, p.computerID, store.SettingEngineeringProgressToTexture) {
			if p.rec.SubjectRefs["wait_phase"] == types.WaitRelayEnd {
				log.Printf("runtime: engineering waits: end relay %s not minted: engineering.progress_to_texture is off", p.rec.RecordID)
			}
			// Not told, so not announced: an end line never follows a start
			// that was not minted (review r1 F9).
			unannounce(p)
			continue
		}
		req := types.CommitInboxRecordsRequest{OwnerID: p.ownerID, ComputerID: p.computerID, CommandID: "wait-relay:" + p.rec.RecordID,
			Records: []types.CommitmentRecord{p.rec}}
		digest, err := store.ComputeCommitInboxRecordsDigest(req)
		if err == nil {
			req.CommandDigest = digest
			cctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
			_, err = rt.store.CommitInboxRecords(cctx, req)
			cancel()
		}
		if err != nil {
			// A start is tried again at the next check. A failed end is
			// logged and lost: Texture's line stays at its last "Waiting"
			// until the round's own records supersede it (accepted, review
			// r1 F3; the ledger summary at the assignment's end still
			// carries the minutes).
			log.Printf("runtime: engineering waits: relay %s: %v", p.rec.RecordID, err)
			unannounce(p)
		}
	}
}

// assignmentWaitViews is management's view of the computer's assignments
// that wait or have waited this attempt, by assignment id.
func (rt *Runtime) assignmentWaitViews(ownerID, computerID string) []types.AssignmentWaitView {
	if rt == nil {
		return nil
	}
	now := rt.assignmentWaitNow()
	t := &rt.assignmentWaits
	t.mu.Lock()
	defer t.mu.Unlock()
	var out []types.AssignmentWaitView
	for key, e := range t.entries {
		// A live wait is re-observed at least every admission retry (at
		// most a minute) or job check (seconds); an entry not touched for
		// waitTrackerIdleForget lost its end (review r1 F5) and is dropped.
		if now.Sub(e.updated) > waitTrackerIdleForget {
			delete(t.entries, key)
			continue
		}
		if e.ownerID != ownerID || e.computerID != computerID {
			continue
		}
		v := types.AssignmentWaitView{AssignmentID: e.assignmentID, Attempt: e.attempt,
			WaitedMinutes: e.tally.Minutes(), WaitedSummary: e.tally.Summary()}
		if e.current != nil {
			v.Wait = e.current.Clone()
			v.Wait.Stamp(now)
		}
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].AssignmentID != out[j].AssignmentID {
			return out[i].AssignmentID < out[j].AssignmentID
		}
		return out[i].Attempt < out[j].Attempt
	})
	return out
}

// observeAdmissionWait records a deferred admission's wait. checkEvery is
// the delay to the next retry.
func (rt *Runtime) observeAdmissionWait(ctx context.Context, assignment types.EngineeringAssignment, busy *engineeringAdmissionBusy, now time.Time, checkEvery time.Duration) {
	if busy == nil {
		return
	}
	rt.observeAssignmentWait(ctx, assignment, func(since time.Time) *types.WaitState {
		return admissionBusyWaitState(assignment, busy, since, now, checkEvery)
	})
}

// admissionBusyWaitState is a busy refusal as a typed wait.
func admissionBusyWaitState(assignment types.EngineeringAssignment, busy *engineeringAdmissionBusy, since, now time.Time, checkEvery time.Duration) *types.WaitState {
	if busy == nil {
		return nil
	}
	if since.IsZero() {
		since = now
	}
	w := &types.WaitState{Class: types.WaitClassPolicy, Subject: "assignment " + assignment.AssignmentID, Since: since.UTC(),
		CheckEverySec: max(int(checkEvery/time.Second), min(int(checkEvery), 1))}
	other := func(what string) []types.WaitHolder {
		return []types.WaitHolder{{Who: "assignment " + busy.BlockedBy, What: what}}
	}
	switch busy.Why {
	case engineeringWaitHostMemory:
		w.Kind, w.Class, w.Resource = types.WaitKindHostMemory, types.WaitClassContention, "host memory"
		w.Need, w.Detail = "a build capsule's reservation", busy.HostMemoryReason
		w.Until = since.Add(engineeringHostMemoryMaxWait).UTC()
	case "live":
		w.Kind, w.Resource = types.WaitKindEngineeringSlot, "the computer's one engineering slot"
		w.Holders = other("live")
	case "queued_ahead":
		w.Kind, w.Resource = types.WaitKindQueuedAhead, "its turn in the engineering queue"
		w.Holders = other("queued first")
	case "candidate_pending":
		w.Kind, w.Resource, w.Detail = types.WaitKindCandidatePending, "the decision on its trajectory's newest candidate", busy.BlockedBy
	case "operation_open":
		w.Kind, w.Resource = types.WaitKindSelfdevOperation, "self-development operation "+busy.OperationID+" to settle"
		holder := types.WaitHolder{Who: "operation " + busy.OperationID, What: busy.OperationState}
		if created, err := time.Parse(time.RFC3339Nano, busy.OperationCreatedAt); err == nil {
			holder.Since = created
		}
		w.Holders = []types.WaitHolder{holder}
		w.Detail = engineeringHoldSettlingActs(busy.OperationID, busy.OperationState)
	case "carry_unreadable":
		w.Kind, w.Resource, w.Detail = types.WaitKindReadError, "state it could not read", busy.BlockedBy
	case engineeringWaitRefresh:
		w.Kind, w.Resource = types.WaitKindPlatformUpdate, "the platform update to "+firstNonEmpty(busy.RefreshTarget, "the new build")
		w.Detail = "the computer takes the update between rounds; the round starts after it, or on the current build at the bound"
		w.Until = since.Add(refreshHoldMax).UTC()
	case engineeringWaitSourceSettling:
		w.Kind, w.Resource, w.Detail = types.WaitKindSourceSettling, "the source to settle on the booted base", busy.BlockedBy
	default:
		w.Kind, w.Resource, w.Detail = types.WaitKind(firstNonEmpty(busy.Why, "admission")), firstNonEmpty(busy.BlockedBy, "admission"), busy.BlockedBy
	}
	return w
}

// handleCapsuleJobWait is the executor's job-wait handler: a build job of
// an engineering assignment began waiting, was re-checked, or stopped
// waiting.
func (rt *Runtime) handleCapsuleJobWait(ev capsule.JobWaitEvent) {
	if rt == nil || rt.store == nil {
		return
	}
	ownerID, computerID, inbox := ev.Notify[jobNotifyOwner], ev.Notify[jobNotifyComputer], ev.Notify[jobNotifyInbox]
	if ownerID == "" || computerID == "" || !strings.HasPrefix(inbox, agentprofile.Engineering+":") {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	assignment, ok := rt.jobWaitAssignment(ctx, ownerID, computerID, inbox)
	if !ok {
		return
	}
	if ev.Ended {
		outcome := "the job started"
		if ev.Outcome != "" && ev.Outcome != "started" {
			outcome = "the job ended " + ev.Outcome + " without starting"
		}
		rt.endAssignmentWait(ctx, assignment.AssignmentID, assignment.Binding.Attempt, outcome)
		return
	}
	if !ev.Wait.Active() {
		return
	}
	rt.observeAssignmentWait(ctx, assignment, func(since time.Time) *types.WaitState {
		w := ev.Wait.Clone()
		if !since.IsZero() && since.Before(w.Since) {
			w.Since = since
		}
		return w
	})
}

// jobWaitAssignment resolves a job's engineering inbox to its assignment,
// from the tracker when it already follows it.
func (rt *Runtime) jobWaitAssignment(ctx context.Context, ownerID, computerID, inbox string) (types.EngineeringAssignment, bool) {
	assignmentID := strings.TrimPrefix(inbox, agentprofile.Engineering+":")
	t := &rt.assignmentWaits
	t.mu.Lock()
	var found *assignmentWaitEntry
	for _, e := range t.entries {
		// The newest attempt of the assignment (review r1 F2): an older
		// attempt's entry never takes a later attempt's job waits.
		if e.assignmentID == assignmentID && e.ownerID == ownerID && e.computerID == computerID && (found == nil || e.attempt > found.attempt) {
			found = e
		}
	}
	if found != nil {
		a := types.EngineeringAssignment{AssignmentID: found.assignmentID}
		a.Binding.OwnerID, a.Binding.ComputerID, a.Binding.Attempt = found.ownerID, found.computerID, found.attempt
		t.mu.Unlock()
		// The entry keeps its Texture inbox; a binding without a parent
		// conversation leaves it as it is.
		return a, true
	}
	t.mu.Unlock()
	assignment, err := rt.store.EngineeringAssignmentForInbox(ctx, ownerID, computerID, inbox)
	if err != nil {
		log.Printf("runtime: engineering waits: job wait of %s: %v", inbox, err)
		return types.EngineeringAssignment{}, false
	}
	if assignment.Disposition.Terminal() {
		return types.EngineeringAssignment{}, false
	}
	return assignment, true
}
