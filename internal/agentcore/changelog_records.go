package agentcore

import (
	"context"
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Emit helpers for the changelog and op-ed records (autopaper step 0
// contracts 2 and 3, H1 notes 5.5). Each commits one silent system record
// with a deterministic id, so a replay mints nothing. Lanes C1 and C2 call
// these; none of the records wakes a reader.

// changelogRecordRequest builds the command for one changelog event. The
// level is set here from the kind, never by the caller.
func changelogRecordRequest(ownerID, computerID string, body types.CommitmentChangelog) (types.CommitInboxRecordsRequest, error) {
	ownerID, computerID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID)
	if ownerID == "" || computerID == "" {
		return types.CommitInboxRecordsRequest{}, fmt.Errorf("changelog event needs an owner and a computer")
	}
	level, ok := types.ChangelogLevelOf(body.Kind)
	if !ok {
		return types.CommitInboxRecordsRequest{}, fmt.Errorf("changelog kind %q is not a known kind", body.Kind)
	}
	body.Level = level
	recordID := types.ChangelogRecordID(body.EventID)
	if body.TapeRef != nil {
		ref := *body.TapeRef
		ref.ComputerID, ref.RecordID = computerID, recordID
		body.TapeRef = &ref
	}
	rec := types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: recordID,
		Kind: types.CommitmentKindReport, Discrepancy: types.DiscrepancyUnresolved,
		Provenance:  types.CommitmentProvenance{AgentID: types.ChangelogSystemAgentID},
		EmitterKind: types.CommitmentEmitterSystem, ComputerID: computerID,
		Inbox: types.ChangelogInbox(ownerID), SenderInbox: types.ChangelogSenderInbox(body.Origin), Addressee: types.ChangelogInbox(ownerID),
		SubjectRefs: map[string]string{"changelog_kind": string(body.Kind)},
		Changelog:   &body,
	}
	if err := types.ValidateChangelogRecord(rec); err != nil {
		return types.CommitInboxRecordsRequest{}, err
	}
	return finishSilentRecordRequest(ownerID, computerID, rec)
}

// opedRecordRequest builds the command for one op-ed record on management's
// inbox. The id and sender inbox are the deterministic ones of the kind.
func opedRecordRequest(ownerID, computerID string, body types.CommitmentOped, subject map[string]string) (types.CommitInboxRecordsRequest, error) {
	ownerID, computerID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID)
	if ownerID == "" || computerID == "" {
		return types.CommitInboxRecordsRequest{}, fmt.Errorf("op-ed record needs an owner and a computer")
	}
	var recordID, sender string
	switch body.Kind {
	case types.OpedRecordArrival:
		if body.Arrival != nil {
			recordID, sender = types.OpedArrivalRecordID(body.Arrival.VersionID), types.OpedSenderInbox(body.Arrival.VersionID)
		}
	case types.OpedRecordVerdict:
		if body.Verdict != nil {
			recordID, sender = types.OpedVerdictRecordID(body.Verdict.VersionID, body.Verdict.Run), types.OpedSenderInbox(body.Verdict.VersionID)
		}
	case types.OpedRecordWakeDecision:
		if body.WakeDecision != nil {
			recordID, sender = types.OpedWakeDecisionRecordID(body.WakeDecision.StoryDocID, body.WakeDecision.N), types.OpedWakeSenderInbox(body.WakeDecision.StoryDocID)
		}
	case types.OpedRecordGateUnavailable:
		if body.GateUnavailable != nil {
			recordID, sender = types.OpedGateUnavailableRecordID(body.GateUnavailable.VersionID), types.OpedSenderInbox(body.GateUnavailable.VersionID)
		}
	}
	management := deskmanifest.AdmitterInbox(ownerID)
	if body.Kind == types.OpedRecordWake && body.Wake != nil {
		recordID, sender = types.OpedWakeRecordID(body.Wake.VersionID), types.OpedSenderInbox(body.Wake.VersionID)
		management = deskmanifest.DocumentInbox(body.Wake.StoryDocID)
	}
	rec := types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: recordID,
		Kind: types.CommitmentKindReport, Discrepancy: types.DiscrepancyUnresolved,
		Provenance:  types.CommitmentProvenance{AgentID: types.OpedSystemAgentID},
		EmitterKind: types.CommitmentEmitterSystem, ComputerID: computerID,
		Inbox: management, SenderInbox: sender, Addressee: management,
		SubjectRefs: subject, Oped: &body,
	}
	if err := types.ValidateOpedRecord(rec); err != nil {
		return types.CommitInboxRecordsRequest{}, err
	}
	return finishSilentRecordRequest(ownerID, computerID, rec)
}

func finishSilentRecordRequest(ownerID, computerID string, rec types.CommitmentRecord) (types.CommitInboxRecordsRequest, error) {
	req := types.CommitInboxRecordsRequest{OwnerID: ownerID, ComputerID: computerID, CommandID: rec.RecordID, Records: []types.CommitmentRecord{rec}}
	digest, err := store.ComputeCommitInboxRecordsDigest(req)
	if err != nil {
		return types.CommitInboxRecordsRequest{}, err
	}
	req.CommandDigest = digest
	return req, nil
}

func (rt *Runtime) commitSilentRecord(ctx context.Context, req types.CommitInboxRecordsRequest) error {
	if rt == nil || rt.store == nil {
		return fmt.Errorf("record %s: runtime has no store", req.CommandID)
	}
	if _, err := rt.store.CommitInboxRecords(ctx, req); err != nil {
		return fmt.Errorf("record %s: %w", req.CommandID, err)
	}
	return nil
}

// EmitChangelog commits one changelog event to changelog:<owner>. The event
// id is the caller's (types.ChangelogEventID); the level is set from the kind.
func (rt *Runtime) EmitChangelog(ctx context.Context, ownerID, computerID string, body types.CommitmentChangelog) error {
	req, err := changelogRecordRequest(ownerID, computerID, body)
	if err != nil {
		return err
	}
	return rt.commitSilentRecord(ctx, req)
}

// EmitOpedArrival commits the oped_arrival record: id oped-arrival:<version id>.
func (rt *Runtime) EmitOpedArrival(ctx context.Context, ownerID, computerID string, a types.OpedArrival) error {
	req, err := opedRecordRequest(ownerID, computerID, types.CommitmentOped{Kind: types.OpedRecordArrival, Arrival: &a},
		map[string]string{"oped_version_id": a.VersionID, "responds_to": a.RespondsTo.PublicationVersionID})
	if err != nil {
		return err
	}
	return rt.commitSilentRecord(ctx, req)
}

// EmitOpedVerdict commits the oped_verdict record: id oped-verdict:<version id>:<run>.
func (rt *Runtime) EmitOpedVerdict(ctx context.Context, ownerID, computerID string, v types.OpedVerdict) error {
	req, err := opedRecordRequest(ownerID, computerID, types.CommitmentOped{Kind: types.OpedRecordVerdict, Verdict: &v},
		map[string]string{"oped_version_id": v.VersionID, "responds_to": v.RespondsTo.PublicationVersionID})
	if err != nil {
		return err
	}
	return rt.commitSilentRecord(ctx, req)
}

// EmitOpedWakeDecision commits the oped_wake_decision record: id oped-wake:<story doc>:<n>.
func (rt *Runtime) EmitOpedWakeDecision(ctx context.Context, ownerID, computerID string, w types.OpedWakeDecision) error {
	req, err := opedRecordRequest(ownerID, computerID, types.CommitmentOped{Kind: types.OpedRecordWakeDecision, WakeDecision: &w},
		map[string]string{"story_doc_id": w.StoryDocID})
	if err != nil {
		return err
	}
	return rt.commitSilentRecord(ctx, req)
}

// UpdateEventsWindow bounds the records one EmitUpdateEvents call examines in
// each inbox, and UpdateEventsMaxCasters the caster inboxes it follows.
const (
	UpdateEventsWindow     = 200
	UpdateEventsMaxCasters = 8
)

// UpdateEventsOption adjusts one EmitUpdateEvents call.
type UpdateEventsOption func(*updateEventsConfig)

type updateEventsConfig struct{ since int64 }

// UpdateEventsSince reads the document inbox from after the given inbox
// sequence (at most UpdateEventsWindow records) instead of its newest window.
// Sequences are per inbox, so the caster inboxes always read their newest
// window.
func UpdateEventsSince(seq int64) UpdateEventsOption {
	return func(c *updateEventsConfig) { c.since = seq }
}

// updateEventsRead reads at most UpdateEventsWindow records of an inbox: the
// newest window, or the window after since.
func (rt *Runtime) updateEventsRead(ctx context.Context, ownerID, computerID, inbox string, since int64) ([]types.CommitmentRecord, error) {
	if since > 0 {
		return rt.store.ListInboxRecords(ctx, ownerID, computerID, inbox, since, since+UpdateEventsWindow)
	}
	head, err := rt.store.InboxHead(ctx, ownerID, computerID, inbox)
	if err != nil {
		return nil, err
	}
	after := head - UpdateEventsWindow
	if after < 0 {
		after = 0
	}
	return rt.store.ListInboxRecords(ctx, ownerID, computerID, inbox, after, 0)
}

// EmitUpdateEvents derives the update_* events of a document's rounds from
// the records the computer already has (the document inbox's Casts and the
// closing Reports) and commits each as a changelog event. It is a read-side
// derivation: it closes nothing and is safe to call at any closure point or
// on a timer, because ids are deterministic and a repeat mints nothing. A
// round still open yields only update_started; a round closed by a restart
// yields update_interrupted and never update_finished.
//
// The read is bounded: the newest UpdateEventsWindow records of the document
// inbox (or the window after UpdateEventsSince) and of each of at most
// UpdateEventsMaxCasters caster inboxes. A round whose cast is older than the window
// is not examined. The signature is the original one plus options.
func (rt *Runtime) EmitUpdateEvents(ctx context.Context, ownerID, computerID, docID string, opts ...UpdateEventsOption) error {
	if rt == nil || rt.store == nil {
		return fmt.Errorf("update events: runtime has no store")
	}
	var cfg updateEventsConfig
	for _, o := range opts {
		o(&cfg)
	}
	doc := deskmanifest.DocumentInbox(docID)
	records, err := rt.updateEventsRead(ctx, ownerID, computerID, doc, cfg.since)
	if err != nil {
		return err
	}
	// A round's final Report lands in the caster's inbox: read those too.
	var casters []string
	seen := map[string]bool{}
	for i := len(records) - 1; i >= 0; i-- { // newest first
		rec := records[i]
		if rec.IsCast() && rec.SenderInbox != "" && rec.SenderInbox != doc && !seen[rec.SenderInbox] && len(casters) < UpdateEventsMaxCasters {
			seen[rec.SenderInbox] = true
			casters = append(casters, rec.SenderInbox)
		}
	}
	for _, caster := range casters {
		more, err := rt.updateEventsRead(ctx, ownerID, computerID, caster, 0)
		if err != nil {
			return err
		}
		records = append(records, more...)
	}
	for _, round := range types.UpdateRoundsFromRecords(docID, records) {
		for _, ev := range types.DeriveUpdateEvents(round) {
			if err := rt.EmitChangelog(ctx, ownerID, computerID, ev); err != nil {
				return err
			}
		}
	}
	return nil
}

// emitOpedWithSubject commits an op-ed record with the caller's subject
// refs (the references a reader of the tape needs without a second lookup).
func (rt *Runtime) emitOpedWithSubject(ctx context.Context, ownerID, computerID string, body types.CommitmentOped, subject map[string]string) error {
	req, err := opedRecordRequest(ownerID, computerID, body, subject)
	if err != nil {
		return err
	}
	return rt.commitSilentRecord(ctx, req)
}

// EmitOpedWake commits the oped_wake system record to the story document's
// inbox: id oped-wake:<version id>. It wakes nobody until lane C2 adds it to
// the waking list in RecordWakesReader (contract amendment A2).
func (rt *Runtime) EmitOpedWake(ctx context.Context, ownerID, computerID string, w types.OpedWake) error {
	req, err := opedRecordRequest(ownerID, computerID, types.CommitmentOped{Kind: types.OpedRecordWake, Wake: &w},
		map[string]string{"oped_version_id": w.VersionID, "story_doc_id": w.StoryDocID})
	if err != nil {
		return err
	}
	return rt.commitSilentRecord(ctx, req)
}

// EmitOpedGateUnavailable commits the gate_unavailable record: id
// oped-gate-unavailable:<version id>.
func (rt *Runtime) EmitOpedGateUnavailable(ctx context.Context, ownerID, computerID string, g types.OpedGateUnavailable) error {
	req, err := opedRecordRequest(ownerID, computerID, types.CommitmentOped{Kind: types.OpedRecordGateUnavailable, GateUnavailable: &g},
		map[string]string{"oped_version_id": g.VersionID, "responds_to": g.RespondsTo.PublicationVersionID})
	if err != nil {
		return err
	}
	return rt.commitSilentRecord(ctx, req)
}
