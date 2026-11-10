package store

// Every object kind and record body lane C2 writes on a computer's store,
// committed with the vocabulary cutover on (the object-graph write guard reads
// every string leaf named role, profile, agent_profile, agent_role,
// authority_profile, requested_by_profile, actor_profile or target_profile as
// a desk name and refuses values outside the v2 vocabulary; package tests that
// leave the cutover off never see it). A key of that name in our bodies would
// be refused on a real computer.

import (
	"context"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func commitRecordAfterCutover(t *testing.T, s *Store, rec types.CommitmentRecord) {
	t.Helper()
	req := types.CommitInboxRecordsRequest{OwnerID: inboxTestOwner, ComputerID: inboxTestComputer, CommandID: rec.RecordID, Records: []types.CommitmentRecord{rec}}
	digest, err := ComputeCommitInboxRecordsDigest(req)
	if err != nil {
		t.Fatal(err)
	}
	req.CommandDigest = digest
	if _, err := s.CommitInboxRecords(context.Background(), req); err != nil {
		t.Fatalf("%s was refused with the cutover on: %v", rec.RecordID, err)
	}
}

func opedSystemRecord(inbox string, body types.CommitmentOped, id, sender string, subject map[string]string) types.CommitmentRecord {
	return types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: id, Kind: types.CommitmentKindReport,
		Discrepancy: types.DiscrepancyUnresolved, Provenance: types.CommitmentProvenance{AgentID: types.OpedSystemAgentID},
		EmitterKind: types.CommitmentEmitterSystem, ComputerID: inboxTestComputer,
		Inbox: inbox, SenderInbox: sender, Addressee: inbox, SubjectRefs: subject, Oped: &body,
	}
}

func TestLaneC2BodiesPassTheVocabularyWriteGuardAfterCutover(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	s.vocabCutover.Store(true)
	mgmt := deskmanifest.AdmitterInbox(inboxTestOwner)
	respondsTo := types.OpedRespondsTo{PublicationVersionID: "pub-story-v1", RoutePath: "/pub/texture/story/v1", PublicationID: "pub-story"}
	subject := map[string]string{"oped_version_id": "pub-op-v1", "responds_to": "pub-story-v1", "story_doc_id": "doc-1",
		"manifest_hash": "mh", "author_label": "writer-aa11bb", "verdict": "accepted"}

	arrival := types.OpedArrival{VersionID: "pub-op-v1", RoutePath: "/pub/texture/op/v1", AuthorLabel: "writer-aa11bb", RespondsTo: respondsTo, TextHash: "mh"}
	commitRecordAfterCutover(t, s, opedSystemRecord(mgmt, types.CommitmentOped{Kind: types.OpedRecordArrival, Arrival: &arrival},
		types.OpedArrivalRecordID("pub-op-v1"), types.OpedSenderInbox("pub-op-v1"), subject))

	verdict := types.OpedVerdict{VersionID: "pub-op-v1", Run: 1, Verdict: types.OpedRejectedRedundant,
		Questions:          []types.OpedQuestionDistribution{{Question: "Is it redundant?", Distribution: map[string]float64{"yes": 0.9, "no": 0.1}}},
		CriteriaRevisionID: "builtin:oped-gate-criteria:1", CriteriaHash: "ch", Model: "~typesafe/jev-latest", RespondsTo: respondsTo,
		Reason: "The gate judged this op-ed redundant (p=0.90).", ReasonUntrusted: true}
	commitRecordAfterCutover(t, s, opedSystemRecord(mgmt, types.CommitmentOped{Kind: types.OpedRecordVerdict, Verdict: &verdict},
		types.OpedVerdictRecordID("pub-op-v1", 1), types.OpedSenderInbox("pub-op-v1"), subject))

	unavailable := types.OpedGateUnavailable{VersionID: "pub-op-v1", Attempts: 3, Cause: "jev is held", RespondsTo: respondsTo}
	commitRecordAfterCutover(t, s, opedSystemRecord(mgmt, types.CommitmentOped{Kind: types.OpedRecordGateUnavailable, GateUnavailable: &unavailable},
		types.OpedGateUnavailableRecordID("pub-op-v1"), types.OpedSenderInbox("pub-op-v1"), subject))

	decision := types.OpedWakeDecision{StoryDocID: "doc-1", N: 1, Woken: true, Why: types.OpedWakeAccepted, VersionIDs: []string{"pub-op-v1"},
		Counts: types.OpedWakeCounts{Arrivals: 1, Accepted: 1, Threshold: 3}}
	commitRecordAfterCutover(t, s, opedSystemRecord(mgmt, types.CommitmentOped{Kind: types.OpedRecordWakeDecision, WakeDecision: &decision},
		types.OpedWakeDecisionRecordID("doc-1", 1), types.OpedWakeSenderInbox("doc-1"), map[string]string{"story_doc_id": "doc-1"}))

	wake := opedWakeRecord(inboxTestOwner, inboxTestComputer, "doc-1", "pub-op-v1")
	wake.SubjectRefs = subject
	commitRecordAfterCutover(t, s, wake)

	// The changelog record the notice poll adopts.
	notice := changelogTestRecord(inboxTestOwner, func(r *types.CommitmentRecord) {
		r.Changelog.Kind, r.Changelog.Level = types.ChangelogParentUsed, types.ChangelogMajor
		r.Changelog.Relation, r.Changelog.ActorLabel = types.ChangelogRelationForkPublished, "writer-cc22dd"
		r.Changelog.RemoteRef = "obj:choir.changelog_notice:x"
		r.Changelog.Subject = types.ChangelogSubject{PublicationVersionID: "pub-child-v1", ParentVersionID: "pub-op-v1"}
		r.Changelog.EventID = types.ChangelogEventID(types.ChangelogParentUsed, "a", "b")
		r.RecordID = types.ChangelogRecordID(r.Changelog.EventID)
	})
	commitRecordAfterCutover(t, s, notice)

	// choir.platform_poll, with every list the poll keeps filled in.
	now := time.Now().UTC()
	poll := PlatformPoll{
		OwnerID: inboxTestOwner, ComputerID: inboxTestComputer, Name: PlatformPollOpedIntake, IntervalSeconds: 60, State: PlatformPollActive,
		Cursor: "123", LastPollAt: now, LastOKAt: now, LastError: "boom", ConsecutiveFailures: 1,
		GateQueue: map[string]PollGateItem{"pub-op-v1": {VersionID: "pub-op-v1", AuthorLabel: "writer-aa11bb", ManifestHash: "mh", StoryDocID: "doc-1",
			RespondsToID: "pub-story-v1", Failures: 1, LastAt: now, Cause: "held"}},
		OpenStories:    map[string]time.Time{"doc-1": now},
		CourierPending: map[string]int{"pub-op-v1": 1},
		Skipped:        []PollSkipped{{ID: "pub-bad-v1", Reason: "the item names no story document", At: now}},
		GateLog:        []PollGateCall{{Publisher: "writer-aa11bb", Story: "doc-1", At: now}},
		UpdatedAt:      now,
	}
	w, ok := objectgraph.CustodianOf("choir.platform_poll")
	if !ok {
		t.Fatal("choir.platform_poll has no custodian")
	}
	if err := s.PutPlatformPoll(objectgraph.WithWriter(ctx, w), poll); err != nil {
		t.Fatalf("choir.platform_poll was refused with the cutover on: %v", err)
	}
	if got, err := s.GetPlatformPoll(objectgraph.WithCustodianReader(ctx, "choir.platform_poll"), inboxTestOwner, PlatformPollOpedIntake); err != nil || len(got.Skipped) != 1 {
		t.Fatalf("read back: %+v %v", got, err)
	}
}

// The op-ed links the desk's revision carries (Kind, not role) commit after the cutover.
func TestOpedSourceLinksOnARevisionPassTheGuardAfterCutover(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	start := lifecycleStartFixture()
	if _, err := s.StartLifecycle(ctx, start); err != nil {
		t.Fatal(err)
	}
	s.vocabCutover.Store(true)
	prov, err := types.Provenance{SchemaVersion: types.ProvenanceSchemaVersion, ParentLinks: []types.ParentLink{
		{Kind: types.ParentLinkSource, PublicationVersionID: "pub-op-v1", ManifestHash: "mh", RoutePath: "/pub/texture/op/v1", AuthorLabel: "writer-aa11bb"},
	}}.CanonicalJSON()
	if err != nil {
		t.Fatal(err)
	}
	commit := types.CommitLifecycleArtifactHeadRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID, CommandID: "commit-oped-links", TrajectoryID: start.TrajectoryID,
		ExpectedLifecycleVersion: 1, ExpectedHeadRevisionID: start.InitialRevision.RevisionID,
		Revision: types.Revision{RevisionID: "revision-oped-links", AuthorKind: types.AuthorUser, AuthorLabel: "owner", Content: "Response", Provenance: prov},
	}
	commit.CommandDigest, _ = ComputeCommitLifecycleArtifactHeadDigest(commit)
	if _, err := s.CommitLifecycleArtifactHead(ctx, commit); err != nil {
		t.Fatalf("a revision citing an op-ed was refused with the cutover on: %v", err)
	}
}
