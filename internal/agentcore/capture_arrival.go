package agentcore

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/researchtools"
	"github.com/yusefmosiah/go-choir/internal/sourcegraph"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Capture arrival (Cut 2 2b, "Living stories"; defaults taken 5): after a
// source poll writes new captures, the runtime appends one system record to
// management:<owner> naming them. The record wakes management; its cursor
// batches whatever accumulated, so there is no debounce. A poll that wrote
// nothing new and has nothing pending appends nothing. A failed append
// leaves its captures on the schedule's pending list, which the next poll's
// arrival names first (L2a A5, defect 2). Only a live poll appends one: a
// crash boot closes every schedule and its pending list, so no arrival is
// minted at boot.

// sourceArrivalPendingLimit bounds a source's pending-arrival list while
// appends keep failing (L2a A5, defect 2). Each arrival names at most the
// poll's own limit; past this bound the oldest pending captures are closed
// unannounced, counted on the schedule and logged.
const sourceArrivalPendingLimit = 200

// captureArrivalItems turns a poll's new captures into arrival items. Every
// field is a pure function of the capture, so a retried poll naming the
// same captures builds the same items.
func captureArrivalItems(captured []sourcegraph.CapturedItem) []types.CaptureArrival {
	items := make([]types.CaptureArrival, 0, len(captured))
	for _, c := range captured {
		id := strings.TrimSpace(c.CaptureID)
		if id == "" {
			continue
		}
		items = append(items, types.CaptureArrival{CaptureID: id, Title: strings.TrimSpace(c.Title), URL: strings.TrimSpace(c.URL),
			PublishedAt: strings.TrimSpace(c.PublishedAt), Language: strings.TrimSpace(c.Language), Script: strings.TrimSpace(c.Script)})
	}
	return items
}

// captureArrivalRequest builds the arrival record command for items. Its
// id is deterministic in the source and the capture set, and its content
// in the items as given, so a retry naming the same items replays instead
// of minting a second record.
//
// A held arrival (event layer §15) is the same record with Held set: it
// names the captures and wakes no one.
func captureArrivalRequest(ownerID, computerID, sourceID string, items []types.CaptureArrival, held bool) (types.CommitInboxRecordsRequest, error) {
	ids := make([]string, len(items))
	for i, item := range items {
		ids[i] = item.CaptureID
	}
	return capturesRecordRequest(ownerID, computerID, sourceID, types.CaptureArrivalRecordID(strings.TrimSpace(sourceID), ids), held, nil, items)
}

// capturesRecordRequest builds a captures record command to management: a
// poll's arrival, or an event's record (event_wakes.go).
func capturesRecordRequest(ownerID, computerID, sourceID, recordID string, held bool, event *types.CaptureEvent, items []types.CaptureArrival) (types.CommitInboxRecordsRequest, error) {
	ownerID, computerID, sourceID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID), strings.TrimSpace(sourceID)
	management := agentprofile.Management + ":" + ownerID
	rec := types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: recordID,
		Kind: types.CommitmentKindReport, Discrepancy: types.DiscrepancyUnresolved,
		Provenance:  types.CommitmentProvenance{AgentID: types.CaptureArrivalSystemAgentID},
		EmitterKind: types.CommitmentEmitterSystem, ComputerID: computerID,
		Inbox: management, SenderInbox: types.CaptureArrivalSenderInbox(sourceID), Addressee: management,
		SubjectRefs: map[string]string{"source_id": sourceID},
		Captures:    &types.CommitmentCaptures{SourceID: sourceID, Held: held, Event: event, Items: append([]types.CaptureArrival(nil), items...)},
	}
	req := types.CommitInboxRecordsRequest{OwnerID: ownerID, ComputerID: computerID, CommandID: rec.RecordID, Records: []types.CommitmentRecord{rec}}
	digest, err := store.ComputeCommitInboxRecordsDigest(req)
	if err != nil {
		return types.CommitInboxRecordsRequest{}, err
	}
	req.CommandDigest = digest
	return req, nil
}

// appendCaptureArrival commits one arrival record naming items.
func (rt *Runtime) appendCaptureArrival(ctx context.Context, ownerID, computerID, sourceID string, items []types.CaptureArrival, held bool) error {
	if rt == nil || rt.store == nil || len(items) == 0 {
		return nil
	}
	req, err := captureArrivalRequest(ownerID, computerID, sourceID, items, held)
	if err != nil {
		return err
	}
	return rt.commitCapturesRecord(ctx, req)
}

// commitCapturesRecord commits a captures record command.
func (rt *Runtime) commitCapturesRecord(ctx context.Context, req types.CommitInboxRecordsRequest) error {
	commit := rt.commitCaptureArrival
	if commit == nil {
		commit = func(ctx context.Context, req types.CommitInboxRecordsRequest) error {
			_, err := rt.store.CommitInboxRecords(ctx, req)
			return err
		}
	}
	if err := commit(ctx, req); err != nil {
		return fmt.Errorf("capture arrival %s: %w", req.CommandID, err)
	}
	return nil
}

// announceCaptures appends one arrival naming the source's pending
// captures first (oldest first), then this poll's new ones, at most limit
// of them (L2a A5, defect 2). It returns what is still pending: the
// captures past the limit, or, when the append failed, all of them. An
// append refused as a conflicting command means a record naming exactly
// these captures was already committed, so they were announced and leave
// the list.
func (rt *Runtime) announceCaptures(ctx context.Context, ownerID, computerID, sourceID string, pending []types.CaptureArrival, captured []sourcegraph.CapturedItem, limit int, held bool) ([]types.CaptureArrival, error) {
	queue := make([]types.CaptureArrival, 0, len(pending)+len(captured))
	seen := map[string]bool{}
	for _, item := range append(append([]types.CaptureArrival(nil), pending...), captureArrivalItems(captured)...) {
		if item.CaptureID == "" || seen[item.CaptureID] {
			continue
		}
		seen[item.CaptureID] = true
		queue = append(queue, item)
	}
	if len(queue) == 0 {
		return nil, nil
	}
	if limit <= 0 {
		limit = researchtools.EffectivePollMaxItems(0)
	}
	announce := queue[:min(limit, len(queue))]
	rest := queue[len(announce):]
	if err := rt.appendCaptureArrival(ctx, ownerID, computerID, sourceID, announce, held); err != nil {
		if errors.Is(err, store.ErrLifecycleCommandConflict) {
			log.Printf("capture arrival: %s: %d captures already announced by a committed record: %v", sourceID, len(announce), err)
			return rest, nil
		}
		return queue, err
	}
	return rest, nil
}

// boundPendingArrivals keeps the newest sourceArrivalPendingLimit pending
// captures and reports how many older ones it dropped.
func boundPendingArrivals(pending []types.CaptureArrival) ([]types.CaptureArrival, int) {
	if len(pending) <= sourceArrivalPendingLimit {
		return pending, 0
	}
	dropped := len(pending) - sourceArrivalPendingLimit
	return append([]types.CaptureArrival(nil), pending[dropped:]...), dropped
}
