package agentcore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/yusefmosiah/go-choir/internal/actgate"
	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/trust"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// The act gate and the circuit breaker (modular architecture MA7.8, trust
// draft 5 items 1 and 2, BT4). The decision is internal/actgate's; this file
// is where the runtime consults it and where its outcomes land.
//
// The gate sits on the act path: commitActIntent asks it for every staged
// semantic act after the addressee and disclosure checks and before anything
// is written. The act's class is the record's (trust.ActionOf: the declared
// grants, raised by the class frozen with the record). A material act (I2 and
// up) of a desk that is not first-party (an installed package desk, this
// computer's generated desks included) derives a fresh trust view (at most
// every leaseTrustEvery, feedLeaseTrust) and is autonomous only when the desk's
// lower bound in the class clears tau with no open breaker. Otherwise it goes
// to the owner tap on the disclosure tape (sink act_gate): the act is refused
// now with a fate that names the tap, management's inbox gets a notice, and
// an owner approval lets the same act pass once on its next attempt. Nothing
// blocks or resumes, as with every disclosure: a restart closes the run that
// was refused. The owner's answers are E6 evidence for the desk in the class.
//
// A first-party desk's material acts are counted (act_gate on GET
// /api/runtime/observability, outcome observed) and pass, with no trust
// derivation spent on them: the pre-registered P2 test (MA7.9) decides
// whether the posterior governs first-party desks too.
//
// The breaker: a forged frame from a desk worker (a frame without the slot
// token the host gave it: the worker used authority it was not given) trips
// every class of the desk at once (actgate.Wild). The trip is a trust.breaker
// fact on the tape, it ends the desk's leases through the lease policy's hook
// (lease.Service.RevokeOnFidelity), it notifies management's inbox, and the
// trust view reads it back as E3 fidelity evidence, so the drop survives a
// restart and lifts only through new positive evidence. A breaker row in a
// fresh view (any E3 or E7 the view carries) gates its grant the same way and
// revokes through the lease policy's ObserveTrust.

const (
	// actGateSink is the disclosure tape's sink name for a gated act.
	actGateSink = "act_gate"
	// actGateAgentID is the committer of the gate's and the breaker's notices.
	actGateAgentID = "system:act-gate"
	// breakerCauseForgedFrame is a trip by a forged frame.
	breakerCauseForgedFrame = "forged_frame"
	// actGateTapeLimit bounds the breaker facts the trust view reads.
	actGateTapeLimit = 2000
)

func trustBreakerChannel(computerID string) string {
	return "trust-breaker:" + strings.TrimSpace(computerID)
}

// breakerPayload is the body of a trust.breaker fact.
type breakerPayload struct {
	Desk    string            `json:"desk"`
	Class   trust.ActionClass `json:"class"`
	Cause   string            `json:"cause"`
	RunID   string            `json:"run_id,omitempty"`
	At      string            `json:"at"`
	Revoked int               `json:"revoked"`
	Detail  string            `json:"detail,omitempty"`
}

// actGateEnforced reports whether the gate holds the desk's material acts: an
// installed package desk that is not first-party. A first-party or unknown
// desk is observed.
func actGateEnforced(deskRole string) bool {
	name := strings.TrimSpace(deskRole)
	d, ok := deskmanifest.Lookup(canonicalDeskName(name))
	if !ok {
		d, ok = deskmanifest.Lookup("app:" + name)
	}
	return ok && d.Kind == deskmanifest.KindApp && !d.FirstParty
}

// gateAct is the act gate on the act path. nil lets the act proceed; an error
// is the refusal the cell reads.
func (r *rlmCallReduction) gateAct(ctx context.Context, in yaegikernel.StagedIntent, rec types.CommitmentRecord) error {
	rt := r.rt()
	if rt == nil || rt.actGate == nil || r.rec == nil {
		return nil
	}
	class := trust.ActionOf(rec)
	if !actgate.Material(class) {
		return nil
	}
	desk := strings.TrimSpace(r.rec.AgentProfile)
	if !actGateEnforced(desk) {
		actgate.Note(actgate.Verdict{Decision: "material", Holder: desk, Class: class}, "observed")
		return nil
	}
	owner := strings.TrimSpace(r.scope.OwnerID)
	computer := firstNonEmpty(strings.TrimSpace(r.scope.ComputerID), rt.TextureComputerID())
	rt.feedLeaseTrust(owner, computer)
	v := rt.actGate.Decide(owner, []string{desk, r.scope.DeskAgentID, r.rec.AgentID}, r.scope.ModelID, class, true)
	actgate.Note(v, "enforced")
	if v.Decision != actgate.Gated {
		return nil
	}
	exec := toolregistry.ExecutionContext{RunID: r.scope.RunID, OwnerID: owner, ComputerID: computer, AgentID: r.scope.DeskAgentID, RunRecord: r.rec}
	body := strings.Join(strings.Fields(strings.Join([]string{in.Statement, in.Body, in.Objective, in.Claim, in.Question, in.Answer, in.Precommit}, " ")), " ")
	return rt.actGateTap(ctx, exec, desk, in.Kind, strings.TrimSpace(in.ToDesk), body, r.scope.Labels, v)
}

// actGateDigest identifies one gated act: the same owner, computer, desk,
// class, kind, target and content is the same act, so an approval is of that
// exact act and its retry finds it.
func actGateDigest(owner, computer, desk string, class trust.ActionClass, kind, target, body string) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{
		owner, computer, actgate.Canon(desk), actGateSink, class.String(), kind, target, body,
	}, "\x1f")))
	return hex.EncodeToString(sum[:])
}

// actGateTap sends a gated act to the owner tap: an earlier answer to this
// exact act first (an approval passes once), else a pending fact and a
// notice to management.
func (rt *Runtime) actGateTap(ctx context.Context, exec toolregistry.ExecutionContext, desk, kind, target, body string, labels types.LabelSet, v actgate.Verdict) error {
	owner, computer := exec.OwnerID, exec.ComputerID
	if rt.store == nil || owner == "" {
		return fmt.Errorf("act gated: %s; no store to record the gate, so the act stays closed", v.Reason)
	}
	summary := disclosureBound(kind+" "+body, disclosureSummaryRunes)
	payload := disclosurePayload{
		Digest: actGateDigest(owner, computer, desk, v.Class, kind, target, body), Sink: actGateSink, Target: target,
		Desk: desk, RunID: exec.RunID, Summary: summary, Labels: labels.Strings(), Gate: &v,
		Reason: fmt.Sprintf("a %s act (%s) of desk %s is not autonomous: %s", v.Class.Impact, v.Class, desk, v.Reason),
	}
	rt.disclosureMu.Lock()
	defer rt.disclosureMu.Unlock()
	state, err := rt.disclosureState(ctx, owner, computer, payload.Digest)
	if err != nil {
		return fmt.Errorf("act gated: %s; the tape could not be read (%v), so the act stays closed", payload.Reason, err)
	}
	switch state.phase {
	case disclosurePhaseApproved:
		consumed := payload
		consumed.DisclosureID, consumed.Actor = state.pendingID, "owner"
		if _, err := rt.appendDisclosure(ctx, exec, owner, computer, disclosurePhaseConsumed, consumed); err != nil {
			return fmt.Errorf("act gated: the owner's approval could not be recorded as used (%v), so the act stays closed", err)
		}
		actgate.Note(v, "approved")
		return nil
	case disclosurePhaseDenied:
		return fmt.Errorf("act gated: the owner denied this act (%s)", state.pendingID)
	case disclosurePhasePending:
		return fmt.Errorf("act gated: this act waits on the owner (%s); the owner answers it at /api/disclosures/%s/approve or /deny, and the same act then passes once", state.pendingID, state.pendingID)
	}
	id, err := rt.appendDisclosure(ctx, exec, owner, computer, disclosurePhasePending, payload)
	if err != nil {
		return fmt.Errorf("act gated: %s; the owner tap could not be recorded (%v), so the act stays closed", payload.Reason, err)
	}
	rt.actGateNotice(ctx, owner, computer, "act-gate:"+id,
		fmt.Sprintf("The act gate held a %s act of desk %s (%s to %s): %s. It waits on the owner (%s): approve it at /api/disclosures/%s/approve and the same act passes once, or deny it.",
			v.Class.Impact, desk, kind, firstNonEmpty(target, "no addressee"), v.Reason, id, id),
		map[string]string{"desk": desk, "class": v.Class.String(), "disclosure_id": id})
	return fmt.Errorf("act gated: %s; this act waits on the owner (%s); the owner answers it at /api/disclosures/%s/approve or /deny, and the same act then passes once", payload.Reason, id, id)
}

// tripBreaker is the circuit breaker: the desk's class drops to gated at once
// and its leases in the class end, the trip is a fact on the tape the trust
// view reads back, and management's inbox gets a notice. It returns how many
// leases ended.
func (rt *Runtime) tripBreaker(ctx context.Context, owner, computer, desk string, class trust.ActionClass, cause, runID, detail string) int {
	if rt == nil || rt.actGate == nil || strings.TrimSpace(desk) == "" {
		return 0
	}
	ctx = context.WithoutCancel(ctx)
	at := time.Now().UTC()
	n := rt.actGate.Trip(ctx, desk, class, at)
	actgate.NoteTrip(cause)
	if actgate.IsWild(class) {
		class = actgate.Wild
	}
	owner = strings.TrimSpace(owner)
	computer = firstNonEmpty(strings.TrimSpace(computer), rt.TextureComputerID())
	if rt.store == nil || owner == "" {
		log.Printf("act gate: breaker trip of %s in %s (%s) is not on the tape: no store or owner", desk, class, cause)
		return n
	}
	raw, err := json.Marshal(breakerPayload{Desk: desk, Class: class, Cause: cause, RunID: runID, At: at.Format(time.RFC3339Nano), Revoked: n, Detail: detail})
	if err != nil {
		return n
	}
	id := uuid.New().String()
	ev := types.EventRecord{EventID: id, Timestamp: at, RunID: runID, OwnerID: owner, AgentID: actGateAgentID,
		ChannelID: trustBreakerChannel(computer), Kind: types.EventTrustBreaker, Phase: cause, Payload: raw}
	if err := rt.store.AppendEvent(ctx, &ev); err != nil {
		log.Printf("act gate: record the breaker trip of %s in %s (%s): %v", desk, class, cause, err)
	}
	rt.actGateNotice(ctx, owner, computer, "trust-breaker:"+id,
		fmt.Sprintf("The circuit breaker tripped for desk %s in %s (%s): %d lease(s) ended at once, and its material acts in the class go to the owner tap until new positive evidence. %s",
			desk, class, cause, n, detail),
		map[string]string{"desk": desk, "class": class.String(), "cause": cause})
	return n
}

// actGateNotice commits a notice to management's inbox, as the cost-anomaly
// notice does: a system record that wakes no one. A failure is logged; the
// notice never decides anything.
func (rt *Runtime) actGateNotice(ctx context.Context, owner, computer, recordID, body string, refs map[string]string) {
	if rt == nil || rt.store == nil || owner == "" {
		return
	}
	inbox := persistentManagementAgentID(owner)
	notice := types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: recordID, Kind: types.CommitmentKindDirective,
		Discrepancy: types.DiscrepancyUnresolved,
		Provenance:  types.CommitmentProvenance{AgentID: actGateAgentID, CommittedAt: time.Now().UTC().Format(time.RFC3339Nano)},
		EmitterKind: types.CommitmentEmitterSystem, ComputerID: computer,
		Inbox: inbox, SenderInbox: actGateAgentID, Addressee: inbox,
		Directive:   &types.CommitmentDirective{Subtype: types.CommitmentDirectiveNotice, Body: body},
		SubjectRefs: refs,
	}
	req := types.CommitInboxRecordsRequest{OwnerID: owner, ComputerID: computer, CommandID: recordID, Records: []types.CommitmentRecord{notice}}
	var err error
	if req.CommandDigest, err = store.ComputeCommitInboxRecordsDigest(req); err != nil {
		log.Printf("act gate: notice %s: %v", recordID, err)
		return
	}
	if _, err := rt.store.CommitInboxRecords(ctx, req); err != nil && !errors.Is(err, store.ErrLifecycleCommandConflict) {
		log.Printf("act gate: notice %s: %v", recordID, err)
	}
}

// trustGateInputs adds what the gate and the breaker put on the tape to the
// trust view's inputs: each breaker trip as E3 fidelity evidence about the
// desk in its classes (every class of I1 and up for a wild trip), and each
// owner answer to a gated act as E6 evidence in the act's class.
func (rt *Runtime) trustGateInputs(ctx context.Context, owner, computer string, in *trust.Inputs) error {
	trips, err := rt.store.ListAcknowledgedEventsByChannel(ctx, owner, trustBreakerChannel(computer), actGateTapeLimit)
	if err != nil {
		return fmt.Errorf("breaker trips: %w", err)
	}
	for _, ev := range trips {
		if ev.Kind != types.EventTrustBreaker {
			continue
		}
		var p breakerPayload
		if json.Unmarshal(ev.Payload, &p) != nil || strings.TrimSpace(p.Desk) == "" {
			continue
		}
		var used []string
		for _, c := range actgate.TripClasses(p.Class) {
			used = append(used, c.Grant)
		}
		in.Frames = append(in.Frames, trust.FrameObservation{Desk: p.Desk, RecordID: "breaker:" + ev.EventID, Used: used, At: p.At})
	}
	_, events, err := rt.disclosureFold(ctx, owner, computer)
	if err != nil {
		return fmt.Errorf("gated acts: %w", err)
	}
	for _, ev := range events {
		if ev.Kind != types.EventDisclosure || (ev.Phase != disclosurePhaseApproved && ev.Phase != disclosurePhaseDenied) {
			continue
		}
		var p disclosurePayload
		if json.Unmarshal(ev.Payload, &p) != nil || p.Sink != actGateSink || p.Gate == nil || p.Actor != "owner" {
			continue
		}
		act := "reject"
		if ev.Phase == disclosurePhaseApproved {
			act = "approve"
		}
		in.OwnerActs = append(in.OwnerActs, trust.OwnerAct{Desk: p.Desk, Act: act, Class: p.Gate.Class, At: ev.Timestamp.UTC().Format(time.RFC3339Nano)})
	}
	return nil
}
