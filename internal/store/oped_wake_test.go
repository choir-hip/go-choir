package store

import (
	"context"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// X14: oped_wake is the one op-ed record that wakes, and it wakes only the
// story document's desk; the other op-ed records stay silent. Through the
// real commit path, the wake row is minted for the document's inbox.
func opedWakeRecord(owner, computer, doc, version string) types.CommitmentRecord {
	body := types.CommitmentOped{Kind: types.OpedRecordWake, Wake: &types.OpedWake{
		StoryDocID: doc, VersionID: version, Why: types.OpedWakeAccepted,
		RespondsTo: types.OpedRespondsTo{PublicationVersionID: "pub-story-v1"},
	}}
	return types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: types.OpedWakeRecordID(version), Kind: types.CommitmentKindReport,
		Discrepancy: types.DiscrepancyUnresolved, Provenance: types.CommitmentProvenance{AgentID: types.OpedSystemAgentID},
		EmitterKind: types.CommitmentEmitterSystem, ComputerID: computer,
		Inbox: deskmanifest.DocumentInbox(doc), SenderInbox: types.OpedSenderInbox(version), Addressee: deskmanifest.DocumentInbox(doc),
		Oped: &body,
	}
}

func TestOpedWakeWakesAndNoOtherOpedRecordDoes(t *testing.T) {
	wake := opedWakeRecord("o", "c", "doc-1", "v1")
	if !RecordWakesReader(wake) {
		t.Fatal("an oped_wake record does not wake the story's desk")
	}
	// A wake body that is not the wake kind, or an empty one, does not.
	silent := wake
	silent.Oped = &types.CommitmentOped{Kind: types.OpedRecordArrival, Arrival: &types.OpedArrival{VersionID: "v1"}}
	if RecordWakesReader(silent) {
		t.Fatal("an oped arrival wakes")
	}
	empty := wake
	empty.Oped = &types.CommitmentOped{Kind: types.OpedRecordWake}
	if RecordWakesReader(empty) {
		t.Fatal("an oped_wake without a body wakes")
	}
	// An agent-emitted record claiming the body is not a system record: the
	// validator refuses it before it can wake anyone (not tested here: ValidateOpedRecord).
}

func TestOpedWakeMintsOneWakeForTheDocumentInbox(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	rec := opedWakeRecord(inboxTestOwner, inboxTestComputer, "doc-1", "v1")
	req := types.CommitInboxRecordsRequest{OwnerID: inboxTestOwner, ComputerID: inboxTestComputer, CommandID: rec.RecordID, Records: []types.CommitmentRecord{rec}}
	digest, err := ComputeCommitInboxRecordsDigest(req)
	if err != nil {
		t.Fatal(err)
	}
	req.CommandDigest = digest
	receipt, err := s.CommitInboxRecords(ctx, req)
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	if len(receipt.Records) != 1 || receipt.Records[0].WakeID == "" {
		t.Fatalf("receipt = %+v, want a wake for the document inbox", receipt.Records)
	}
	// A replay mints nothing new.
	again, err := s.CommitInboxRecords(ctx, req)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if len(again.Records) != 1 || again.Records[0].InboxSeq != receipt.Records[0].InboxSeq {
		t.Fatalf("replay receipt = %+v", again.Records)
	}
}
