package textureowner

// T2: the source-entities list is no longer stored in the Texture run record;
// the self-development join ids derived from it are
// (docs/design/run-record-written-less-2026-11-15.md). Ways this could fail,
// written before the tests and the code:
//
//  1. The revision metadata a run ends with changes: a join id lost, changed or
//     reordered, for the same evidence sequence. The goldens below were
//     recorded from the code before this change.
//  2. The merged list is not the one stored today: req.SourceEntities is
//     merged at texture_agent_revision.go:680-688 into a shadowed variable, but
//     the merged value is what the run record holds, so the join must be
//     derived after both merges (non-empty req.SourceEntities is tested).
//  3. Two entities of one join kind: the last one wins today; it must still.
//  4. Precedence: the list-derived value beats a different value on the parent
//     revision at start, on the run's inherited copy, and on the parent
//     revision at completion; a join kind the list lacks keeps the inherited
//     value.
//  5. An empty join id becomes an empty string key instead of staying absent.
//  6. A run started before this change (the list inline, join keys possibly
//     inherited) is finished by the new binary and yields different revision
//     metadata.
//  7. A legacy parent revision whose persisted metadata holds the list still
//     passes it to a child through carryForwardDurableTextureMetadata.
//  8. The list is still stored in the run record, or the run record is not
//     smaller, or the key is still in durableMetadataKeys.
//  9. The run's model input changes: the prompt must be byte-equal.
// 10. A provenance value for a join key beats the list's value in the run record
//     (req.Provenance is copied into the run after the durable keys).
//
// Goldens: each scenario's revision metadata (loop_id removed) was recorded by
// running these scenarios on the code before the change.

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

var joinKeys = []string{textureJoinOperationID, textureJoinBundleDigest, textureJoinReceiptID, textureJoinEventHead}

// commitHeadWithMetadata is commitAgentHead with the head's metadata chosen by
// the test: extra is merged over the carried-forward metadata.
func commitHeadWithMetadata(t *testing.T, s *store.Store, start types.StartLifecycleRequest, revisionID, text string, extra map[string]any) types.Revision {
	t.Helper()
	ctx := context.Background()
	snapshot, err := s.GetLifecycleSnapshot(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	head := snapshot.HeadRevision
	meta := map[string]any{"source": "texture_cell"}
	for key, value := range extra {
		meta[key] = value
	}
	raw, _ := json.Marshal(meta)
	rev := types.Revision{
		RevisionID: revisionID, DocID: start.InitialDocument.DocID, OwnerID: start.OwnerID, ComputerID: start.ComputerID,
		TrajectoryID: start.TrajectoryID, AuthorKind: types.AuthorAppAgent, AuthorLabel: "appagent",
		BodyDoc: observationBodyDoc(text), Metadata: raw, ParentRevisionID: head.RevisionID,
	}
	command := types.CommitLifecycleArtifactHeadRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID, CommandID: "agent-head:" + revisionID,
		TrajectoryID: start.TrajectoryID, ExpectedLifecycleVersion: snapshot.Trajectory.LifecycleVersion,
		ExpectedHeadRevisionID: head.RevisionID, Revision: rev,
	}
	command.CommandDigest, err = store.ComputeCommitLifecycleArtifactHeadWithSourceGraphDigest(command, store.TextureSourceGraphWriteSet{})
	if err != nil {
		t.Fatalf("digest: %v", err)
	}
	result, err := s.CommitLifecycleArtifactHeadWithSourceGraph(ctx, command, store.TextureSourceGraphWriteSet{})
	if err != nil || result.Revision == nil {
		t.Fatalf("commit head: %+v %v", result, err)
	}
	return *result.Revision
}

type startedJoinRun struct {
	handler *Handler
	s       *store.Store
	start   types.StartLifecycleRequest
	doc     types.Document
	stored  types.RunRecord
	prompt  string
}

// startJoinRun starts a Texture revision run for a document whose head
// revision carries headMeta, with req.SourceEntities = reqEntities.
func startJoinRun(t *testing.T, suffix string, headMeta map[string]any, reqEntities []textureSourceEntity) startedJoinRun {
	t.Helper()
	return startJoinRunWithProvenance(t, suffix, headMeta, reqEntities, nil)
}

func startJoinRunWithProvenance(t *testing.T, suffix string, headMeta map[string]any, reqEntities []textureSourceEntity, provenance map[string]any) startedJoinRun {
	t.Helper()
	core, handler := testAPISetup(t)
	core.SetDispatchActor(noDispatch)
	s := core.Store()
	start := startBodyRequestLifecycle(t, s, suffix)
	commitHeadWithMetadata(t, s, start, "revision-join-head-"+suffix, "The head.", headMeta)
	doc, err := s.GetLifecycleDocument(t.Context(), start.OwnerID, start.ComputerID, start.InitialDocument.DocID)
	if err != nil {
		t.Fatal(err)
	}
	rec, err := handler.submitTextureAgentRevisionRun(t.Context(), doc, start.OwnerID, textureAgentRevisionRequest{
		Intent: "integrate_execution_findings", Provenance: withInboxRecord(provenance),
		SourceEntities: reqEntities,
	}, 0)
	if err != nil || rec == nil {
		t.Fatalf("submit Texture run: %+v %v", rec, err)
	}
	stored, err := s.GetLifecycleRun(t.Context(), start.OwnerID, start.ComputerID, rec.RunID)
	if err != nil {
		t.Fatal(err)
	}
	return startedJoinRun{handler: handler, s: s, start: start, doc: doc, stored: stored, prompt: rec.Prompt}
}

func withInboxRecord(provenance map[string]any) map[string]any {
	out := map[string]any{"request_source": "inbox_record"}
	for key, value := range provenance {
		out[key] = value
	}
	return out
}

func noDispatch(context.Context, string, string, string, string, string, string, string) error {
	return nil
}

// revisionMetadataOf is the revision metadata the run would end with, loop_id
// removed (it is the run's own id), as canonical JSON.
func (r startedJoinRun) revisionMetadataOf(t *testing.T, rec types.RunRecord, doc types.Document) string {
	t.Helper()
	meta := decodeRevisionMetadata(r.handler.buildAppagentRevisionMetadata(t.Context(), &rec, doc, r.start.OwnerID, nil, 0))
	delete(meta, "loop_id")
	out, err := json.Marshal(meta)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func joinEntities(pairs ...string) []textureSourceEntity {
	var out []textureSourceEntity
	for i := 0; i+1 < len(pairs); i += 2 {
		out = append(out, executionEvidenceSourceEntity(pairs[i], pairs[i+1], pairs[i+1], "management"))
	}
	return out
}

const (
	hexBundle = "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
	hexHead   = "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
)

// Goldens recorded from the code before the change (failure 1).
const (
	goldenAllFour       = `{"self_development_bundle_digest":"dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd","self_development_event_head":"eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee","self_development_operation_id":"op-1","self_development_receipt_id":"receipt-1","source":"texture_cell","trajectory_id":"trajectory-hold-allfour"}`
	goldenSameKindTwice = `{"self_development_operation_id":"op-second","source":"texture_cell","trajectory_id":"trajectory-hold-samekind"}`
	goldenListBeatsP    = `{"self_development_bundle_digest":"bundle-from-parent","self_development_operation_id":"op-from-list","source":"texture_cell","trajectory_id":"trajectory-hold-beatsp"}`
	// goldenListBeatsQ equals goldenListBeatsP on purpose: a later parent
	// revision with different join ids changes nothing, because the run holds
	// the list's value (and the inherited one for a kind the list lacks).
	goldenListBeatsQ    = `{"self_development_bundle_digest":"bundle-from-parent","self_development_operation_id":"op-from-list","source":"texture_cell","trajectory_id":"trajectory-hold-beatsp"}`
	goldenAllFourPrompt = "Your document woke you. Intent: integrate_execution_findings. Your frame follows: the document as blocks b1..bN and the records addressed to you.\n\nThe owner's request:\nReplace the PDF and EPUB readers with one Reader app package. Quality bar: the Reader renders the sample PDF's first page in the capture. Do not change package.json. HOLD-REQUEST-END\n(revision revision-hold-root-allfour)"
	goldenMergedList    = `{"self_development_operation_id":"op-from-request","self_development_receipt_id":"legacy-receipt","source":"texture_cell","trajectory_id":"trajectory-hold-merged"}`
	goldenProvenance    = `{"self_development_operation_id":"op-from-list","self_development_receipt_id":"receipt-from-provenance","source":"texture_cell","trajectory_id":"trajectory-hold-provenance"}`
	goldenEmptyAbsent   = `{"self_development_receipt_id":"receipt-only","source":"texture_cell","trajectory_id":"trajectory-hold-empty"}`
)

// Failures 1, 2, 9: a non-empty req.SourceEntities with one entity of each join
// kind gives the recorded revision metadata and the same prompt.
func TestListNotStoredRevisionMetadataIsByteEqualWithRequestEntities(t *testing.T) {
	r := startJoinRun(t, "allfour", nil, joinEntities("operation", "op-1", "capsule_bundle", hexBundle, "receipt", "receipt-1", "event_head", hexHead))
	got := r.revisionMetadataOf(t, r.stored, r.doc)
	t.Logf("GOLDEN allfour %s", got)
	if got != goldenAllFour {
		t.Errorf("revision metadata =\n%s\nwant\n%s", got, goldenAllFour)
	}
	if r.prompt != goldenAllFourPrompt {
		t.Errorf("prompt =\n%q\nwant\n%q", r.prompt, goldenAllFourPrompt)
	}
}

// Failure 3: two entities of one kind; the last wins.
func TestListNotStoredLastEntityOfAJoinKindWins(t *testing.T) {
	r := startJoinRun(t, "samekind", nil, joinEntities("operation", "op-first", "operation", "op-second"))
	got := r.revisionMetadataOf(t, r.stored, r.doc)
	t.Logf("GOLDEN samekind %s", got)
	if got != goldenSameKindTwice {
		t.Errorf("revision metadata =\n%s\nwant\n%s", got, goldenSameKindTwice)
	}
	if !strings.Contains(got, `"op-second"`) || strings.Contains(got, `"op-first"`) {
		t.Errorf("the last entity of a join kind must win: %s", got)
	}
}

// Failure 4: the list-derived value beats a different value on the parent
// revision at start (and so on the run's inherited copy), and a kind the list
// lacks keeps the inherited value.
func TestListNotStoredListValueBeatsParentAndInheritedValues(t *testing.T) {
	head := map[string]any{textureJoinOperationID: "op-from-parent", textureJoinBundleDigest: "bundle-from-parent"}
	r := startJoinRun(t, "beatsp", head, joinEntities("operation", "op-from-list"))
	got := r.revisionMetadataOf(t, r.stored, r.doc)
	t.Logf("GOLDEN beatsp %s", got)
	if got != goldenListBeatsP {
		t.Errorf("revision metadata =\n%s\nwant\n%s", got, goldenListBeatsP)
	}
	meta := decodeRevisionMetadata(json.RawMessage(got))
	if meta[textureJoinOperationID] != "op-from-list" || meta[textureJoinBundleDigest] != "bundle-from-parent" {
		t.Errorf("precedence wrong: %#v", meta)
	}
	// And the parent revision at completion differs again: the run's value
	// (the list's for the operation, the inherited one for the bundle) wins.
	commitHeadWithMetadata(t, r.s, r.start, "revision-join-later-beatsp", "Later head.",
		map[string]any{textureJoinOperationID: "op-from-later-parent", textureJoinBundleDigest: "bundle-from-later-parent"})
	later, err := r.s.GetLifecycleDocument(t.Context(), r.start.OwnerID, r.start.ComputerID, r.start.InitialDocument.DocID)
	if err != nil {
		t.Fatal(err)
	}
	gotQ := r.revisionMetadataOf(t, r.stored, later)
	t.Logf("GOLDEN beatsq %s", gotQ)
	if gotQ != goldenListBeatsQ {
		t.Errorf("revision metadata with a later parent =\n%s\nwant\n%s", gotQ, goldenListBeatsQ)
	}
}

// Failure 5: an entity of a join kind with an empty identity leaves the key
// absent, never an empty string.
func TestListNotStoredEmptyJoinIdsStayAbsent(t *testing.T) {
	empty := textureSourceEntity{EntityID: "entity-empty-op", Kind: "operation", Target: textureSourceEntityTarget{TargetKind: "operation"}}
	r := startJoinRun(t, "empty", nil, append([]textureSourceEntity{empty}, joinEntities("receipt", "receipt-only")...))
	got := r.revisionMetadataOf(t, r.stored, r.doc)
	t.Logf("GOLDEN empty %s", got)
	if got != goldenEmptyAbsent {
		t.Errorf("revision metadata =\n%s\nwant\n%s", got, goldenEmptyAbsent)
	}
	meta := decodeRevisionMetadata(json.RawMessage(got))
	for _, key := range []string{textureJoinOperationID, textureJoinBundleDigest, textureJoinEventHead} {
		if value, present := meta[key]; present {
			t.Errorf("%s present as %#v, want absent", key, value)
		}
	}
	for key, value := range r.stored.Metadata {
		if value == "" && strings.HasPrefix(key, "self_development_") {
			t.Errorf("the run record holds an empty %s", key)
		}
	}
}

// Failure 6: a run started by the old code (the list inline, join keys
// possibly inherited, no list-derived join keys of its own) yields, through the
// unchanged R2, the same revision metadata as a run started by the new code.
func TestListNotStoredALegacyRunRecordWithTheInlineListYieldsTheSameRevisionMetadata(t *testing.T) {
	legacyGolden := strings.ReplaceAll(goldenAllFour, "trajectory-hold-allfour", "trajectory-hold-legacy")
	entities := joinEntities("operation", "op-1", "capsule_bundle", hexBundle, "receipt", "receipt-1", "event_head", hexHead)
	r := startJoinRun(t, "legacy", nil, entities)
	legacy := r.stored
	legacy.Metadata = map[string]any{}
	for key, value := range r.stored.Metadata {
		legacy.Metadata[key] = value
	}
	for _, key := range joinKeys { // the old start copied only inherited join keys: none here
		delete(legacy.Metadata, key)
	}
	legacy.Metadata[textureAvailableSourceEntitiesKey] = entities
	// Through the store's re-encode: a typed list becomes []any of sorted maps.
	raw, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	var reread types.RunRecord
	if err := json.Unmarshal(raw, &reread); err != nil {
		t.Fatal(err)
	}
	if got, want := r.revisionMetadataOf(t, reread, r.doc), r.revisionMetadataOf(t, r.stored, r.doc); got != want {
		t.Errorf("a legacy run record gives\n%s\nthe new one gives\n%s", got, want)
	}
	if got := r.revisionMetadataOf(t, reread, r.doc); got != legacyGolden {
		t.Errorf("a legacy run record gives\n%s\nwant\n%s", got, legacyGolden)
	}
	// With inherited join keys of a different value on the legacy record, the
	// list still wins.
	legacy.Metadata[textureJoinOperationID] = "op-inherited-stale"
	raw, _ = json.Marshal(legacy)
	reread = types.RunRecord{}
	_ = json.Unmarshal(raw, &reread)
	if got := r.revisionMetadataOf(t, reread, r.doc); got != legacyGolden {
		t.Errorf("a legacy run record with an inherited stale join id gives\n%s\nwant\n%s", got, legacyGolden)
	}
}

// Failure 7: a legacy parent revision whose metadata holds the list no longer
// passes it to a child (a deliberate change: the list is run-time context).
func TestListNotStoredLegacyParentRevisionDoesNotCarryTheList(t *testing.T) {
	parent, _ := json.Marshal(map[string]any{
		"seed_prompt": "keep me", textureAvailableSourceEntitiesKey: joinEntities("operation", "op-legacy"),
		textureJoinOperationID: "op-legacy",
	})
	child := carryForwardDurableTextureMetadata(json.RawMessage(`{}`), parent)
	meta := decodeRevisionMetadata(child)
	if _, ok := meta[textureAvailableSourceEntitiesKey]; ok {
		t.Errorf("the list was carried into the child revision: %#v", meta)
	}
	if meta["seed_prompt"] != "keep me" || meta[textureJoinOperationID] != "op-legacy" {
		t.Errorf("the durable keys were not carried: %#v", meta)
	}
}

// Failure 8: the key is out of durableMetadataKeys, the started run's stored
// record lacks it and holds the join keys, and the stored body holds nothing of
// the list.
func TestListNotStoredTheRunRecordLacksTheList(t *testing.T) {
	for _, key := range durableMetadataKeys {
		if key == textureAvailableSourceEntitiesKey {
			t.Fatalf("%s is still a durable metadata key", key)
		}
	}
	entities := joinEntities("operation", "op-1", "capsule_bundle", hexBundle, "receipt", "receipt-1", "event_head", hexHead)
	r := startJoinRun(t, "smaller", nil, entities)
	if _, ok := r.stored.Metadata[textureAvailableSourceEntitiesKey]; ok {
		t.Fatalf("the stored run record still holds the list: %#v", r.stored.Metadata[textureAvailableSourceEntitiesKey])
	}
	if r.stored.Metadata[textureJoinOperationID] != "op-1" || r.stored.Metadata[textureJoinBundleDigest] != hexBundle ||
		r.stored.Metadata[textureJoinReceiptID] != "receipt-1" || r.stored.Metadata[textureJoinEventHead] != hexHead {
		t.Errorf("the stored run record lacks the join keys: %#v", r.stored.Metadata)
	}
	body, _ := json.Marshal(r.stored)
	// The stored body holds neither the key nor anything only an entity has.
	for _, needle := range []string{textureAvailableSourceEntitiesKey, "whole_resource", "collapsed_citation"} {
		if strings.Contains(string(body), needle) {
			t.Errorf("the stored run body still holds %q (%d bytes)", needle, len(body))
		}
	}
	if strings.Contains(r.prompt, textureAvailableSourceEntitiesKey) {
		t.Errorf("the key reached the prompt")
	}
}

// Failure 2, with a merged list that differs from req.SourceEntities: the head
// revision's metadata still carries the legacy key (a receipt entity) and the
// request carries an operation entity. Both join ids reach the stored run and
// the revision metadata. Deriving from req.SourceEntities only, or before the
// request merge, loses the receipt.
func TestListNotStoredTheMergedListNotTheRequestAloneDerivesTheJoin(t *testing.T) {
	head := map[string]any{textureAvailableSourceEntitiesKey: joinEntities("receipt", "legacy-receipt")}
	r := startJoinRun(t, "merged", head, joinEntities("operation", "op-from-request"))
	got := r.revisionMetadataOf(t, r.stored, r.doc)
	t.Logf("GOLDEN merged %s", got)
	if got != goldenMergedList {
		t.Errorf("revision metadata =\n%s\nwant\n%s", got, goldenMergedList)
	}
	if r.stored.Metadata[textureJoinReceiptID] != "legacy-receipt" || r.stored.Metadata[textureJoinOperationID] != "op-from-request" {
		t.Errorf("the stored run lacks a join id of the merged list: %#v", r.stored.Metadata)
	}
	meta := decodeRevisionMetadata(json.RawMessage(got))
	if meta[textureJoinReceiptID] != "legacy-receipt" || meta[textureJoinOperationID] != "op-from-request" {
		t.Errorf("the revision metadata lacks a join id of the merged list: %#v", meta)
	}
}

// Failure 4: req.Provenance is copied into the run after the durable keys. A
// provenance value for a join key the list has loses to the list, in the stored
// run and in the revision metadata, as it did when the builder derived the join
// at completion.
func TestListNotStoredProvenanceCannotBeatTheListsJoinValue(t *testing.T) {
	r := startJoinRunWithProvenance(t, "provenance", nil, joinEntities("operation", "op-from-list"),
		map[string]any{textureJoinOperationID: "op-from-provenance", textureJoinReceiptID: "receipt-from-provenance"})
	got := r.revisionMetadataOf(t, r.stored, r.doc)
	t.Logf("GOLDEN provenance %s", got)
	if got != goldenProvenance {
		t.Errorf("revision metadata =\n%s\nwant\n%s", got, goldenProvenance)
	}
	if r.stored.Metadata[textureJoinOperationID] != "op-from-list" {
		t.Errorf("the stored run holds %v for the operation id, want the list's", r.stored.Metadata[textureJoinOperationID])
	}
	if r.stored.Metadata[textureJoinReceiptID] != "receipt-from-provenance" {
		t.Errorf("a join kind the list lacks keeps the provenance value, got %v", r.stored.Metadata[textureJoinReceiptID])
	}
}
