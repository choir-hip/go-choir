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
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/yusefmosiah/go-choir/internal/actgate"
	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/disclosure"
	"github.com/yusefmosiah/go-choir/internal/lease"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// The disclosure path (MA3.6, R15; plan section 7.2). A run carries the union
// of the labels of everything it read (run_labels.go). When it tries a sink,
// a place its data leaves its custody, three steps decide, in order:
//
//  1. the kernel rules table (internal/disclosure): label class by sink to
//     allow, deny or ask. Most decisions end here;
//  2. for an ask, the System 1 kernel service answers one typed question,
//     "is this disclosure within what the owner's request implies", with a
//     full distribution over yes, no and unsure. The distribution is recorded
//     as a precommit-kind record carrying the run's labels, and the RULE
//     (disclosure.FromDistribution) decides from it: System 1 never decides
//     alone;
//  3. a distribution the rule does not clear escalates to the owner tap: a
//     disclosure.decision fact on the tape (phase pending) that the owner
//     answers through POST /api/disclosures/{id}/approve or .../deny. The
//     sink is refused now, with a fate saying it waits on the owner. Nothing
//     blocks and nothing resumes: an approval lets the next attempt of the
//     same disclosure pass once (phase consumed), and a restart closes the
//     run that was refused like any other.
//
// The custodian-model step (the custodian desk's own cell decides) is a named
// residual: it needs a cell decision wired into the sink, and no custodian
// cell exists for the kinds that ask today.
//
// Enforcement mirrors the writer and reader gates, per desk class and not by a
// setting. A built-in desk's decision is recorded and counted
// (disclosure_gate on GET /api/runtime/observability) and a deny or an ask
// does not block yet, and an ask does not spend a System 1 call on it; MA3.7
// flips it, in code. An installed package desk (app:*) is enforced now.
//
// A model call is not a sink (owner ruling R15). It is logged with the
// resolved provider and the run's labels (toolregistry.WithLabelSource).

const (
	disclosurePhasePending  = "pending"
	disclosurePhaseAllowed  = "allowed"
	disclosurePhaseApproved = "approved"
	disclosurePhaseDenied   = "denied"
	disclosurePhaseConsumed = "consumed"
	disclosurePhaseRefused  = "refused"

	// disclosureSummaryRunes bounds what is recorded of the disclosure.
	disclosureSummaryRunes = 300
	// disclosureRequestRunes bounds the owner's request given to System 1.
	disclosureRequestRunes = 600

	disclosureHostRequestRef = system1HostRefPrefix + "request"
	disclosureHostSinkRef    = system1HostRefPrefix + "disclosure"
)

// disclosureChannel is the tape channel of a computer's disclosure facts.
func disclosureChannel(computerID string) string {
	return "disclosure:" + strings.TrimSpace(computerID)
}

// disclosureCall is one sink attempt. Every field is the host's: the run's
// identity comes from the run record, the target and the summary from the
// verb the cell called.
type disclosureCall struct {
	exec     toolregistry.ExecutionContext
	deskRole string
	sink     disclosure.Sink
	// target is where the data goes: a desk, a URL or provider, a document,
	// a package.
	target string
	// summary is what leaves, bounded.
	summary string
	// mailReply is set by the mail custodian's own reply path, and only when it
	// replies in the owner's own thread (mail_send only). The gate names no
	// desk: the custodian's path is the one that sets it.
	mailReply bool
	// kind is the message kind of a desk message (cast, report, reply), for
	// the sites tally only.
	kind string
}

// disclosurePayload is the body of a disclosure.decision event.
type disclosurePayload struct {
	// DisclosureID is the pending event an owner answer, a consumption or an
	// allowance refers to.
	DisclosureID string              `json:"disclosure_id,omitempty"`
	Digest       string              `json:"digest"`
	Sink         string              `json:"sink"`
	Target       string              `json:"target,omitempty"`
	Desk         string              `json:"desk,omitempty"`
	RunID        string              `json:"run_id,omitempty"`
	Summary      string              `json:"summary,omitempty"`
	Labels       []string            `json:"labels,omitempty"`
	Verdict      *disclosure.Verdict `json:"verdict,omitempty"`
	// Distribution is System 1's full answer to the typed question.
	Distribution   map[string]float64 `json:"distribution,omitempty"`
	Rule           string             `json:"rule,omitempty"`
	System1Receipt string             `json:"system1_receipt,omitempty"`
	RecordID       string             `json:"record_id,omitempty"`
	Reason         string             `json:"reason,omitempty"`
	Actor          string             `json:"actor,omitempty"`
	// Gate is the act gate's verdict when the fact is a gated act (sink
	// act_gate, act_gate.go, MA7.8) rather than a disclosure.
	Gate *actgate.Verdict `json:"gate,omitempty"`
}

// packageDesk reports whether the desk is an installed package desk, the
// class the gate enforces now.
func packageDesk(deskRole string) bool {
	d, ok := deskmanifest.Lookup(canonicalDeskName(strings.TrimSpace(deskRole)))
	if !ok {
		d, ok = deskmanifest.Lookup("app:" + strings.TrimSpace(deskRole))
	}
	return ok && d.Kind == deskmanifest.KindApp
}

// labelObjectKinds are the object kinds a label's kind names, for custody and
// lease lookups. A label's kind is the product's word (document, story,
// capture); the registry's is the object kind.
func labelObjectKinds(kind string) []objectgraph.ObjectKind { return objectgraph.LabelKinds(kind) }

// messageAdmitted reports whether the desk a message goes to is admitted for a
// label: the custodian of the label's kind, a holder of a live lease that
// names the kind, or a holder of a live disclosure grant bound to the sending
// run (runID) whose grant names the label (lease.GrantTextureBrief).
func (rt *Runtime) messageAdmitted(owner, runID, toDesk, label string) bool {
	if !strings.HasPrefix(label, types.LabelOwnerPrivatePrefix) {
		return true
	}
	kind := strings.TrimPrefix(label, types.LabelOwnerPrivatePrefix)
	to, ok := resolveMessageDesk(toDesk)
	if !ok {
		return false
	}
	writer, _ := objectgraph.DeskWriter(to)
	for _, k := range labelObjectKinds(kind) {
		if custodian, found := objectgraph.CustodianOf(k); found && custodian == writer {
			return true
		}
	}
	if rt != nil && rt.leases != nil {
		if rt.leases.AdmitsMessageLabel(writer, runID, label) {
			return true
		}
		// The lease is held by the recipient's run, not the sender's, so it is
		// matched on the recipient desk, the owner, the expiry and the
		// run-or-durable rule (enforce flip section 6.5).
		for _, k := range labelObjectKinds(kind) {
			if rt.leases.AdmitsKindForRecipient(owner, writer, k) {
				return true
			}
		}
	}
	return false
}

// egressDisclosure names the destination and the content of an egress call a
// cell composed: a URL it fetches, a provider endpoint with its params, a
// search query. The other host calls read stored objects or fixed registry
// sources and carry nothing the run composed outward.
func egressDisclosure(action yaegikernel.BrokerAction, payload json.RawMessage) (target, summary string, ok bool) {
	switch action {
	case yaegikernel.ActionRead:
		var p yaegikernel.ReadPayload
		if json.Unmarshal(payload, &p) != nil {
			return "", "", false
		}
		return p.URL, "fetch " + p.URL, true
	case yaegikernel.ActionSourceAPI:
		var p yaegikernel.SourceAPIPayload
		if json.Unmarshal(payload, &p) != nil {
			return "", "", false
		}
		params, _ := json.Marshal(p.Params)
		return p.Provider + " " + p.Endpoint, fmt.Sprintf("source API %s %s %s", p.Provider, p.Endpoint, params), true
	case yaegikernel.ActionSearchStart:
		var p yaegikernel.SearchStartPayload
		if json.Unmarshal(payload, &p) != nil {
			return "", "", false
		}
		return "search providers", "search query " + p.Query, true
	}
	return "", "", false
}

// gateEgressCall is the egress sink's gate step: deskEgress runs it for the
// host calls it dispatches, and the async search dispatch runs it before it
// starts a search, which never reaches deskEgress.
func (rt *Runtime) gateEgressCall(ctx context.Context, deskRole string, exec toolregistry.ExecutionContext, action yaegikernel.BrokerAction, payload json.RawMessage) error {
	target, summary, ok := egressDisclosure(action, payload)
	if !ok {
		return nil
	}
	return rt.gateDisclosure(ctx, disclosureCall{exec: exec, deskRole: deskRole, sink: disclosure.SinkEgress, target: target, summary: summary})
}

// messageTargetDesk is the desk a desk message goes to, without its instance
// suffix; other sinks have none (the tally names the sink).
func messageTargetDesk(call disclosureCall) string {
	if call.sink != disclosure.SinkDeskMessage {
		return ""
	}
	// resolveMessageDesk keeps a package desk's app:<name> whole; the cut is
	// only the fallback for a target it cannot resolve.
	if to, ok := resolveMessageDesk(call.target); ok {
		return to
	}
	head, _, _ := strings.Cut(strings.TrimSpace(call.target), ":")
	return canonicalDeskName(head)
}

// ownerFlowRoles are the built-in desks the owner flow pairs along the owner's
// delegation chain (owner ruling 2026-11-06, extended by the enforce flip,
// section 6.2): the owner asks Texture, Texture casts management, management
// casts engineering and its own task role, and the reports come back. Every
// end is resolved through the registry, never by a literal name: Texture by
// the custodian of the owner's documents, the others by their sole-holder
// grants, a task role by its manifest Parent. An end that is not a system desk
// (a grant moved to a package desk) resolves to nothing, and a pair with an
// empty end is never covered.
type ownerFlowRoles struct {
	texture, research, admitter, performer string
	// parentOf names the parent desk of a role (deskmanifest.ParentOf); nil
	// covers no role.
	parentOf func(role string) (string, bool)
}

// ownerFlowRolesFrom builds the roles from a sole-holder lookup. A holder
// that is not a persistent system desk is dropped.
func ownerFlowRolesFrom(holder func(grant string) (deskmanifest.Desk, bool), texture string) ownerFlowRoles {
	system := func(grant string) string {
		if d, ok := holder(grant); ok && d.Kind == deskmanifest.KindSystem {
			return d.Name
		}
		return ""
	}
	return ownerFlowRoles{
		texture:   texture,
		research:  system(deskmanifest.GrantResearchSpawn),
		admitter:  system(deskmanifest.GrantAdmitAssignment),
		performer: system(deskmanifest.GrantPerformAssignment),
	}
}

// currentOwnerFlowRoles reads the registry now.
func currentOwnerFlowRoles() ownerFlowRoles {
	texture := ""
	if w, ok := objectgraph.CustodianOf("choir.texture_document"); ok {
		if d, found := deskmanifest.Lookup(strings.TrimPrefix(string(w), "desk:")); found && d.Kind == deskmanifest.KindSystem {
			texture = d.Name
		}
	}
	r := ownerFlowRolesFrom(deskmanifest.GrantHolder, texture)
	r.parentOf = func(role string) (string, bool) {
		p, ok := deskmanifest.ParentOf(role)
		return p.Name, ok
	}
	return r
}

// pair reports whether a message from one desk to another is on the chain.
func (r ownerFlowRoles) pair(from, to string) bool {
	if from == "" || to == "" {
		return false
	}
	if from == to {
		// A message whose two ends are the same built-in system desk leaves no
		// custody: an intake task's outward message is stamped with its parent
		// (outwardSender), so its report reads as management to management.
		// Only the system desks the roles resolved to count; a package desk, a
		// role and an unknown name never pair.
		return from == r.texture || from == r.research || from == r.admitter || from == r.performer
	}
	on := func(a, b string) bool {
		switch {
		case a == "" || b == "":
			return false
		case a == r.texture && (b == r.research || b == r.admitter):
			return true
		case a == r.admitter && b == r.performer:
			return true
		case a == r.admitter && r.parentOf != nil:
			// management to its own task role: the role's parent is the
			// admitter, by the manifest's grant holder.
			p, ok := r.parentOf(b)
			return ok && p == r.admitter
		}
		return false
	}
	return on(from, to) || on(to, from)
}

// ownerDeskPair reports whether a desk message runs between two built-in desks
// on the owner's delegation chain, in either direction (disclosure.OwnerFlow):
// Texture and research, Texture and management, management and engineering,
// management and its task role. The registry names every end, so no desk name
// is keyed here, and a package desk is never an end.
func ownerDeskPair(from, to string) bool {
	// The Delegate path spells the task's inbox management-task:<id> while the
	// manifest's role is management_task, and agentprofile.Canonical folds the
	// hyphen only for the verifier role. The registry spells every desk and
	// role with underscores, so the fold here is narrow: a hyphenated name
	// that is no registered role still resolves to nothing.
	fold := func(name string) string {
		return strings.ReplaceAll(canonicalDeskName(strings.TrimSpace(name)), "-", "_")
	}
	return currentOwnerFlowRoles().pair(fold(from), fold(to))
}

// disclosureFacts computes the admission facts of a sink attempt.
func (rt *Runtime) disclosureFacts(ctx context.Context, call disclosureCall) disclosure.Facts {
	switch call.sink {
	case disclosure.SinkDeskMessage:
		owner, runID := strings.TrimSpace(call.exec.OwnerID), strings.TrimSpace(call.exec.RunID)
		return disclosure.Facts{
			AdmittedFor: func(label string) bool { return rt.messageAdmitted(owner, runID, call.target, label) },
			OwnerDesks:  ownerDeskPair(canonicalDeskName(strings.TrimSpace(call.deskRole)), messageTargetDesk(call)),
		}
	case disclosure.SinkPublication:
		// The document custodian's publish of a publish-auto story whose spec
		// traces to the owner (publish_auto_admission.go).
		return disclosure.Facts{AdmittedFor: rt.publishAutoAdmission(ctx, call)}
	case disclosure.SinkMailSend:
		// Only the mail custodian replying in the owner's own thread is
		// admitted; any other mail leaves the owner's mailbox on an ask.
		admitted := call.mailReply
		return disclosure.Facts{AdmittedFor: func(string) bool { return admitted }}
	}
	return disclosure.Facts{}
}

// GateDisclosure is the exported entry for a sink the run reaches outside
// this package (a story's publication in the Texture owner). rec is the run's
// record; a refusal is the fate the caller shows.
func (rt *Runtime) GateDisclosure(ctx context.Context, rec types.RunRecord, sink disclosure.Sink, target, summary string) error {
	return rt.gateDisclosure(ctx, disclosureCall{
		exec: toolregistry.ExecutionContext{
			RunID: rec.RunID, OwnerID: rec.OwnerID, ComputerID: rec.ComputerID, AgentID: rec.AgentID, RunRecord: &rec,
		},
		deskRole: rec.AgentProfile, sink: sink, target: target, summary: summary,
	})
}

// gateDisclosure decides one sink attempt: nil lets it proceed, an error is
// the refusal the run reads.
func (rt *Runtime) gateDisclosure(ctx context.Context, call disclosureCall) error {
	if rt == nil {
		return nil
	}
	owner, computer, runID := strings.TrimSpace(call.exec.OwnerID), strings.TrimSpace(call.exec.ComputerID), strings.TrimSpace(call.exec.RunID)
	if computer == "" {
		computer = strings.TrimSpace(rt.cfg.ComputerID)
	}
	enforcedDesk := packageDesk(call.deskRole) || disclosure.CurrentMode() == disclosure.ModeEnforce
	labels := rt.runLabelSet(runID)
	if rt.store != nil && owner != "" && runID != "" {
		stored, err := rt.store.RunLabels(ctx, owner, computer, runID)
		switch {
		case err == nil:
			labels = labels.Union(stored)
		case errors.Is(err, store.ErrNotFound):
		case enforcedDesk:
			// The run's labels cannot be read: an enforced desk's sink stays
			// closed rather than reading an empty set.
			disclosure.NoteUnreadable(call.sink, "enforced")
			return fmt.Errorf("disclosure refused: the labels of run %s could not be read (%v), so the %s sink stays closed", runID, err, call.sink)
		default:
			log.Printf("disclosure: labels of run %s unreadable: %v", runID, err)
			disclosure.NoteUnreadable(call.sink, "observed")
		}
	}
	verdict := disclosure.Decide(labels, call.sink, rt.disclosureFacts(ctx, call))
	if verdict.Decision == disclosure.Allow {
		disclosure.Note(verdict, "allowed")
		return nil
	}
	disclosure.NoteSite(verdict, disclosure.Site{
		Sender: canonicalDeskName(strings.TrimSpace(call.deskRole)), Recipient: messageTargetDesk(call), Kind: call.kind,
	})
	if !enforcedDesk {
		// A built-in desk observes: counted, logged once, not blocked, and no
		// System 1 call is spent on it.
		disclosure.Note(verdict, "observed")
		return nil
	}
	disclosure.Note(verdict, "enforced")
	if rt.store == nil || owner == "" {
		return fmt.Errorf("disclosure refused: %s; no store to record the decision, so the sink stays closed", verdict.Reason())
	}
	payload := disclosurePayload{
		Digest: disclosureDigest(owner, computer, call, labels), Sink: string(call.sink), Target: call.target,
		Desk: call.deskRole, RunID: runID, Summary: disclosureBound(call.summary, disclosureSummaryRunes),
		Labels: labels.Strings(), Verdict: &verdict, Reason: verdict.Reason(),
	}
	if verdict.Decision == disclosure.Deny {
		payload.Rule = string(disclosure.Deny)
		if _, err := rt.appendDisclosure(ctx, call.exec, owner, computer, disclosurePhaseRefused, payload); err != nil {
			log.Printf("disclosure: record the refusal of %s on run %s: %v", call.sink, runID, err)
		}
		return fmt.Errorf("disclosure refused: %s", verdict.Reason())
	}
	return rt.disclosureAsk(ctx, call, owner, computer, labels, payload)
}

// disclosureAsk runs the ask path: an earlier answer to this exact disclosure
// first, then System 1 and the rule, then the owner tap.
func (rt *Runtime) disclosureAsk(ctx context.Context, call disclosureCall, owner, computer string, labels types.LabelSet, payload disclosurePayload) error {
	// standing answers from an earlier step on the tape. The mutex covers the
	// check and the consume, not the judge call: a slow judge must not
	// serialize every gated attempt.
	standing := func() (bool, error) {
		rt.disclosureMu.Lock()
		defer rt.disclosureMu.Unlock()
		state, err := rt.disclosureState(ctx, owner, computer, payload.Digest)
		if err != nil {
			return true, fmt.Errorf("disclosure refused: %s; the tape could not be read (%v), so the sink stays closed", payload.Reason, err)
		}
		switch state.phase {
		case disclosurePhaseApproved:
			// The owner approved this exact disclosure: it passes once.
			consumed := payload
			consumed.DisclosureID = state.pendingID
			consumed.Actor = "owner"
			if _, err := rt.appendDisclosure(ctx, call.exec, owner, computer, disclosurePhaseConsumed, consumed); err != nil {
				return true, fmt.Errorf("disclosure refused: the owner's approval could not be recorded as used (%v), so the sink stays closed", err)
			}
			return true, nil
		case disclosurePhaseDenied:
			return true, fmt.Errorf("disclosure refused: the owner denied this disclosure (%s) of %s to %s", state.pendingID, call.sink, call.target)
		case disclosurePhasePending:
			return true, fmt.Errorf("disclosure refused: this disclosure waits on the owner (%s); the owner answers it at /api/disclosures/%s/approve or /deny, and the same attempt then passes once", state.pendingID, state.pendingID)
		}
		return false, nil
	}
	if done, err := standing(); done {
		return err
	}

	dist, receipt, askErr := rt.disclosureJudge(ctx, call, owner, computer, labels, payload)
	payload.Distribution, payload.System1Receipt = dist, receipt
	if askErr != nil {
		payload.Reason += "; System 1 did not answer: " + askErr.Error()
	}
	// The distribution is a precommit-kind record carrying the run's labels,
	// whether or not the rule clears it.
	if recordID, err := rt.recordDisclosurePrecommit(ctx, call, owner, computer, labels, dist, payload.Digest); err != nil {
		log.Printf("disclosure: record the precommit of %s on run %s: %v", call.sink, call.exec.RunID, err)
	} else {
		payload.RecordID = recordID
	}
	outcome := disclosure.FromDistribution(dist)
	payload.Rule = string(outcome)
	// Another attempt may have asked, or the owner answered, while the judge
	// ran: the tape is re-read before this one writes.
	if done, err := standing(); done {
		return err
	}
	rt.disclosureMu.Lock()
	defer rt.disclosureMu.Unlock()
	if outcome == disclosure.OutcomeAllow {
		if _, err := rt.appendDisclosure(ctx, call.exec, owner, computer, disclosurePhaseAllowed, payload); err != nil {
			return fmt.Errorf("disclosure refused: the rule's allowance could not be recorded (%v), so the sink stays closed", err)
		}
		return nil
	}
	id, err := rt.appendDisclosure(ctx, call.exec, owner, computer, disclosurePhasePending, payload)
	if err != nil {
		return fmt.Errorf("disclosure refused: the owner tap could not be recorded (%v), so the sink stays closed", err)
	}
	return fmt.Errorf("disclosure refused: %s; System 1's answer does not clear the rule (P(yes) >= %.1f and P(no) <= %.1f), so this waits on the owner (%s); the owner answers it at /api/disclosures/%s/approve or /deny, and the same attempt then passes once",
		payload.Reason, disclosure.MinYes, disclosure.MaxNo, id, id)
}

// disclosureJudge asks System 1 the typed question and returns the full
// distribution it gave. The evidence is the owner's request and the
// disclosure, both composed by the host.
func (rt *Runtime) disclosureJudge(ctx context.Context, call disclosureCall, owner, computer string, labels types.LabelSet, payload disclosurePayload) (map[string]float64, string, error) {
	if rt.candidateJudge == nil {
		return nil, "", fmt.Errorf("no decision model is bound on this computer")
	}
	request := ""
	if call.exec.RunRecord != nil {
		request = call.exec.RunRecord.Prompt
	}
	if strings.TrimSpace(request) == "" && call.exec.RunID != "" {
		if rec, err := rt.getRunForComputer(ctx, owner, call.exec.RunID); err == nil {
			request = rec.Prompt
		}
	}
	text := fmt.Sprintf("desk %s will send to %s (%s): %s. It carries data labelled %s.",
		call.deskRole, call.sink, call.target, payload.Summary, strings.Join(labels, ", "))
	questions := []yaegikernel.System1Question{{
		ID: disclosure.QuestionID, Text: disclosure.Question, Choices: disclosure.Choices,
		ContextRefs: []string{disclosureHostRequestRef, disclosureHostSinkRef},
	}}
	valid, err := yaegikernel.ValidateSystem1HostQuestions(questions)
	if err != nil {
		return nil, "", err
	}
	raw, err := rt.System1().Ask(ctx, call.exec, owner, computer, system1Ask{
		questions: valid, labels: labels,
		hostText: map[string]string{
			disclosureHostRequestRef: "The owner's request: " + disclosureBound(request, disclosureRequestRunes),
			disclosureHostSinkRef:    text,
		},
	})
	if err != nil {
		return nil, "", err
	}
	var result system1Result
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, "", err
	}
	for _, a := range result.Answers {
		if a.ID == disclosure.QuestionID {
			if a.Error != "" {
				return nil, result.ReceiptRef, fmt.Errorf("%s", a.Error)
			}
			return a.Probabilities, result.ReceiptRef, nil
		}
	}
	return nil, result.ReceiptRef, fmt.Errorf("System 1 gave no answer to the disclosure question")
}

// recordDisclosurePrecommit writes the typed question and System 1's
// distribution as a precommit-kind record on the commitment ledger, stamped
// with the run's labels. The kernel service is its committer and resolver.
func (rt *Runtime) recordDisclosurePrecommit(ctx context.Context, call disclosureCall, owner, computer string, labels types.LabelSet, dist map[string]float64, digest string) (string, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	recordID := call.exec.RunID + ":disclosure:" + digest[:12]
	claim := fmt.Sprintf("the disclosure of %s to %s is within what the owner's request implies", call.sink, call.target)
	rec := types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: recordID, Kind: types.CommitmentKindPrecommit,
		Discrepancy: types.DiscrepancyUnresolved,
		Precommit: &types.CommitmentPrecommit{
			Question: claim, Distribution: dist, Resolver: disclosureService,
		},
		Provenance: types.CommitmentProvenance{
			AgentID: disclosureService, ModelID: "system1", Desk: call.deskRole, RunID: call.exec.RunID,
			Origin: types.OriginSystem, CommittedAt: now, Labels: labels.Strings(),
		},
		Prediction: types.CommitmentPrediction{
			Hypothesis: claim, CommittedAt: now,
			Questions: []types.TypedQuestion{{Question: claim, Answers: disclosure.Choices, Probabilities: dist}},
		},
		EvidenceRefs: []string{"disclosure:" + digest},
		Addressee:    disclosureService,
	}
	id, err := rt.store.AppendCommitmentRecord(ctx, owner, computer, rec)
	if err != nil {
		return "", err
	}
	return id, nil
}

// disclosureService names the kernel's disclosure path as the committer of
// its own records.
const disclosureService = "system:disclosure"

// disclosureDigest identifies one disclosure: the same owner, computer, desk,
// sink, target and summary under the same labels is the same disclosure, so
// an approval is of that exact thing and a retry finds it.
func disclosureDigest(owner, computer string, call disclosureCall, labels types.LabelSet) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{
		owner, computer, canonicalDeskName(strings.TrimSpace(call.deskRole)), string(call.sink), call.target, call.summary,
		strings.Join(labels, ","),
	}, "\x1f")))
	return hex.EncodeToString(sum[:])
}

func disclosureBound(text string, runes int) string {
	text = strings.Join(strings.Fields(text), " ")
	if utf8.RuneCountInString(text) > runes {
		return string([]rune(text)[:runes]) + "…"
	}
	return text
}

// appendDisclosure writes one disclosure.decision fact on the computer's
// disclosure channel and returns its event id.
func (rt *Runtime) appendDisclosure(ctx context.Context, exec toolregistry.ExecutionContext, owner, computer, phase string, payload disclosurePayload) (string, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	id := uuid.New().String()
	ev := types.EventRecord{
		EventID: id, Timestamp: time.Now().UTC(), RunID: firstNonEmpty(exec.RunID, payload.RunID), OwnerID: owner, AgentID: exec.AgentID,
		ChannelID: disclosureChannel(computer), Kind: types.EventDisclosure, Phase: phase, Payload: raw,
	}
	if err := rt.store.AppendEvent(ctx, &ev); err != nil {
		return "", err
	}
	return id, nil
}

// resolveMessageDesk names the registered desk an address reaches: the full
// name when it is a desk ("app:notes"), else the desk before a subject
// ("texture:doc-1" is the texture desk). ok is false for an address that is no
// desk (the owner, a system inbox).
func resolveMessageDesk(to string) (name string, ok bool) {
	to = strings.TrimSpace(to)
	if _, found := objectgraph.DeskWriter(canonicalDeskName(to)); found {
		return canonicalDeskName(to), true
	}
	head, _, _ := strings.Cut(to, ":")
	if _, found := objectgraph.DeskWriter(canonicalDeskName(head)); found {
		return canonicalDeskName(head), true
	}
	return "", false
}

// gateMessageDisclosure is the desk-message sink of the disclosure path: an
// addressed act (Cast, Ask, Report, Reply, Feed, Decline, Escalate) from a
// desk whose run carries labels. The target desk is admitted for a label when
// it is the custodian of its kind or holds a live lease for it
// (messageAdmitted). A message to the owner leaves no custody and is not a
// sink.
func (r *rlmCallReduction) gateMessageDisclosure(ctx context.Context, in yaegikernel.StagedIntent) error {
	switch in.Kind {
	case yaegikernel.IntentCast, yaegikernel.IntentAsk, yaegikernel.IntentReport, yaegikernel.IntentReply,
		yaegikernel.IntentFeed, yaegikernel.IntentDecline, yaegikernel.IntentEscalate:
	case yaegikernel.IntentResolve, yaegikernel.IntentDisagreement, yaegikernel.IntentCancel:
		// Addressed acts that are not envelopes (MA3.7b, MA3.6 N1): they are
		// gated when they name a desk, and a retract also reaches the desk
		// the retracted act addressed.
	default:
		return nil
	}
	if in.LocalID != "" {
		if r.gatedMessages[in.LocalID] {
			return nil
		}
		if r.gatedMessages == nil {
			r.gatedMessages = map[string]bool{}
		}
		r.gatedMessages[in.LocalID] = true
	}
	body := strings.Join([]string{in.Statement, in.Body, in.Objective, in.Claim, in.Question, in.Packet, in.Resolve, in.Disagreement}, " ")
	to := in.ToDesk
	if in.Kind == yaegikernel.IntentCancel && strings.TrimSpace(to) == "" {
		to = r.retractAddressee(ctx, in)
	}
	return r.gateMessage(ctx, in.Kind, to, body)
}

// retractAddressee is the desk the retracted act addressed, which a retract
// stands down (the intent itself is unaddressed). A record that cannot be
// read names none: the gate does not guess a sink.
func (r *rlmCallReduction) retractAddressee(ctx context.Context, in yaegikernel.StagedIntent) string {
	rt := r.rt()
	if rt == nil || rt.store == nil || strings.TrimSpace(in.TargetRef) == "" {
		return ""
	}
	computer := firstNonEmpty(strings.TrimSpace(r.scope.ComputerID), rt.TextureComputerID())
	target, err := rt.store.GetCommitmentRecord(ctx, strings.TrimSpace(r.scope.OwnerID), computer, strings.TrimSpace(in.TargetRef))
	if err != nil || target == nil {
		return ""
	}
	return strings.TrimSpace(target.Addressee)
}

// gateMessage gates one message of kind to the address to.
func (r *rlmCallReduction) gateMessage(ctx context.Context, kind, to, body string) error {
	rt := r.rt()
	to = strings.TrimSpace(to)
	if rt == nil || r.rec == nil || to == "" {
		return nil
	}
	head, _, _ := strings.Cut(to, ":")
	if d := strings.ToLower(strings.TrimSpace(head)); d == "owner" || d == "user" || d == "" {
		return nil
	}
	r.issueCustodianBrief(ctx, to)
	return rt.gateDisclosure(ctx, disclosureCall{
		exec: toolregistry.ExecutionContext{
			RunID: r.scope.RunID, OwnerID: r.scope.OwnerID, ComputerID: r.scope.ComputerID, AgentID: r.scope.DeskAgentID, RunRecord: r.rec,
		},
		deskRole: r.rec.AgentProfile, sink: disclosure.SinkDeskMessage, target: to, kind: kind,
		summary: kind + " " + disclosureBound(body, disclosureSummaryRunes),
	})
}

// issueCustodianBrief mints the Texture custodian's texture.brief grant when a
// Texture run messages research (lease.IssueTextureBrief): the brief carries
// the owner-private labels of the run's documents, and the custodian admits
// research for them, for messages from this run only, until the run ends. The
// sender is the run record's desk and the run the reduction's, both the
// host's; the lease service refuses any other sender or holder. Any other
// message mints nothing. A failure is logged and the gate decides without the
// grant, which asks.
func (r *rlmCallReduction) issueCustodianBrief(ctx context.Context, to string) {
	rt := r.rt()
	if rt == nil || rt.leases == nil || r.rec == nil {
		return
	}
	sender := canonicalDeskName(strings.TrimSpace(r.rec.AgentProfile))
	senderWriter, ok := objectgraph.DeskWriter(sender)
	if !ok || senderWriter != objectgraph.WriterTexture {
		return
	}
	holder, ok := resolveMessageDesk(to)
	if !ok {
		return
	}
	holderWriter, ok := objectgraph.DeskWriter(holder)
	if !ok || !lease.GrantHeldBy(lease.GrantTextureBrief, holderWriter) {
		return
	}
	computer := firstNonEmpty(strings.TrimSpace(r.scope.ComputerID), strings.TrimSpace(rt.cfg.ComputerID))
	if _, err := rt.leases.IssueTextureBrief(ctx, r.scope.OwnerID, computer, sender, r.scope.RunID, r.scope.DeskAgentID, holder); err != nil {
		log.Printf("disclosure: texture.brief lease for run %s to %s: %v", r.scope.RunID, holder, err)
	}
}
