package agentcore

import (
	"context"
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/sourcegraph"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Capture arrival (Cut 2 2b, "Living stories"; defaults taken 5): after a
// source poll writes new captures, the runtime appends one system record to
// management:<owner> naming them. The record wakes management; its cursor
// batches whatever accumulated, so there is no debounce. A poll that wrote
// nothing new appends nothing. Only a live poll appends one: a crash boot
// closes every schedule, so no arrival is minted at boot.

// appendCaptureArrival commits the arrival record for one poll's new
// captures. Its id is deterministic in the source and the capture set, so
// a retried poll that names the same captures replays instead of minting a
// second record.
func (rt *Runtime) appendCaptureArrival(ctx context.Context, ownerID, computerID, sourceID string, captured []sourcegraph.CapturedItem) error {
	if rt == nil || rt.store == nil || len(captured) == 0 {
		return nil
	}
	ownerID, computerID, sourceID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID), strings.TrimSpace(sourceID)
	items := make([]types.CaptureArrival, 0, len(captured))
	ids := make([]string, 0, len(captured))
	for _, c := range captured {
		id := strings.TrimSpace(c.CaptureID)
		if id == "" {
			continue
		}
		ids = append(ids, id)
		items = append(items, types.CaptureArrival{CaptureID: id, Title: strings.TrimSpace(c.Title), URL: strings.TrimSpace(c.URL), PublishedAt: strings.TrimSpace(c.PublishedAt)})
	}
	if len(items) == 0 {
		return nil
	}
	management := agentprofile.Management + ":" + ownerID
	rec := types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: types.CaptureArrivalRecordID(sourceID, ids),
		Kind: types.CommitmentKindReport, Discrepancy: types.DiscrepancyUnresolved,
		Provenance:  types.CommitmentProvenance{AgentID: types.CaptureArrivalSystemAgentID},
		EmitterKind: types.CommitmentEmitterSystem, ComputerID: computerID,
		Inbox: management, SenderInbox: types.CaptureArrivalSenderInbox(sourceID), Addressee: management,
		SubjectRefs: map[string]string{"source_id": sourceID},
		Captures:    &types.CommitmentCaptures{SourceID: sourceID, Items: items},
	}
	req := types.CommitInboxRecordsRequest{OwnerID: ownerID, ComputerID: computerID, CommandID: rec.RecordID, Records: []types.CommitmentRecord{rec}}
	digest, err := store.ComputeCommitInboxRecordsDigest(req)
	if err != nil {
		return err
	}
	req.CommandDigest = digest
	if _, err := rt.store.CommitInboxRecords(ctx, req); err != nil {
		return fmt.Errorf("capture arrival %s: %w", rec.RecordID, err)
	}
	return nil
}
