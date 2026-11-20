package textureowner

// The fifth review of the root desk's digest, written as a failure list first.
// Each test names the item it closes (the lead's numbering of the round).
//
//  1. A change committed between the scan and the run's creation is in neither
//     the ledger nor pending: the cut-off must precede every read.
//  2. A read that fails moves the watermark past a document it never read.
//  3. More than the bound changed, silently: the watermark passed the rest.
//  4. K planned restarts with continuous commits stack root runs: the hour
//     bound lived in memory.
//  5. A story takes a slot of the bound; an excerpt lists every document.
//  6. A root run that commits nothing costs a model call every hour; an
//     archived master is woken; a malformed watermark wedges every digest.
//  7. The watermark walk's bound is a number nobody tested.

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func entryAt(id string, at time.Time, mark string) store.ChangedTextureDocument {
	return store.ChangedTextureDocument{
		ChangedAt: at, ObjectID: id,
		Doc:        types.Document{DocID: id, CurrentRevisionID: mark},
		Trajectory: types.TrajectoryRecord{},
	}
}

// Item 1 (the invariant: a change is in the ledger a committed root revision
// read, or still pending; never neither). A document committed after the
// scan, before the run is created, is pending after the root commits.
func TestTextureRootACommitBetweenTheScanAndTheRunIsStillPending(t *testing.T) {
	rh := newRootHarness(t)
	b, runB := rh.createDoc("race-b", "Harbor")
	c, runC := rh.createDoc("race-c", "Bridge")
	rh.write(rh.h, &runB, "harbor", "The harbor is open.", "")
	injected := false
	rh.h.textureRootAfterScan = func() {
		if injected {
			return
		}
		injected = true
		rh.write(rh.h, &runC, "bridge", "The bridge is closed.", "") // after the scan, before the run
	}
	rh.deliver(rh.digests()[0])
	marker, _ := rh.marker()
	root, live := rh.activeRun(marker.DocumentID)
	if !live || !injected {
		t.Fatalf("the digest started no root run (live=%v injected=%v)", live, injected)
	}
	rh.h.textureRootAfterScan = nil
	rh.write(rh.h, &root, "master", "Harbor: open.", "") // the root commits what it read
	pending := rh.pendingDocs()
	if !contains(pending, c) {
		t.Fatalf("the commit on %s between the scan and the run is lost: pending %v", c, pending)
	}
	if contains(pending, b) {
		t.Fatalf("%s was read and committed, and is still pending: %v", b, pending)
	}
}

func contains(list []string, id string) bool {
	for _, item := range list {
		if item == id {
			return true
		}
	}
	return false
}

// Item 1: a change stamped just before the cut-off but visible after it (the
// store takes the time, builds the write, then writes) is in the overlap, and
// is pending unless the last root revision recorded it at the same mark.
func TestTextureRootOverlapCatchesAStampedBeforeVisibleChange(t *testing.T) {
	cut := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	state := textureRootState{Through: cut, Docs: map[string]textureRootMark{
		"read":    {At: cut.Add(-20 * time.Second).Format(time.RFC3339Nano), Mark: "rev-1|1.1"},
		"changed": {At: cut.Add(-20 * time.Second).Format(time.RFC3339Nano), Mark: "rev-1|1.1"},
	}}
	entries := []store.ChangedTextureDocument{
		entryAt("read", cut.Add(-20*time.Second), "rev-1"),    // read at this mark: not pending
		entryAt("changed", cut.Add(-10*time.Second), "rev-2"), // changed after it, stamped in the overlap
		entryAt("unseen", cut.Add(-30*time.Second), "rev-9"),  // visible late, never read
	}
	for i := range entries {
		entries[i].Trajectory.LifecycleVersion, entries[i].Trajectory.ReducerSeq = 1, 1
	}
	read, unread, left, _ := textureRootSelect(entries, state, 50)
	var ids []string
	for _, e := range read {
		ids = append(ids, e.Doc.DocID)
	}
	if len(unread) != 0 || left != 0 || len(ids) != 2 || !contains(ids, "changed") || !contains(ids, "unseen") || contains(ids, "read") {
		t.Fatalf("pending %v (unread %d left %d), want changed and unseen and not read", ids, len(unread), left)
	}
}

// Item 3: more than the bound changed. The oldest are read first, the rest
// are counted, and the watermark stops at the first one left, so nothing is
// marked digested unread.
func TestTextureRootMoreThanTheBoundReadsOldestFirstAndHoldsTheWatermark(t *testing.T) {
	start := time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)
	var entries []store.ChangedTextureDocument
	for i := 0; i < 120; i++ {
		entries = append(entries, entryAt(fmt.Sprintf("doc-%03d", i), start.Add(time.Duration(i)*time.Second), "r"))
	}
	read, _, left, firstLeftAt := textureRootSelect(entries, textureRootState{}, textureRootMaxDocuments)
	if len(read) != 50 || left != 70 || read[0].Doc.DocID != "doc-000" || read[49].Doc.DocID != "doc-049" {
		t.Fatalf("read %d (first %s, last %s), left %d; want the oldest 50 and 70 left", len(read), read[0].Doc.DocID, read[len(read)-1].Doc.DocID, left)
	}
	scan := textureRootScan{Cutoff: start.Add(time.Hour), Left: left, firstLeftAt: firstLeftAt}
	through, docs := scan.Finish(read, nil)
	if !through.Equal(start.Add(50*time.Second)) || len(docs) != 50 {
		t.Fatalf("watermark %s with %d documents; want it at the first document left (%s)", through, len(docs), start.Add(50*time.Second))
	}
}

// Item 3, end to end: more than the bound changed on a real computer; the run
// says how many are left, a digest is armed, and what was left is pending
// after the root commits. Stories take no slot.
func TestTextureRootEndToEndTheRemainderStaysPendingAndStoriesTakeNoSlot(t *testing.T) {
	rh := newRootHarness(t)
	story := rh.openStory("bound")
	var ordinary []string
	for i := 0; i < textureRootMaxDocuments+3; i++ {
		id, _ := rh.createDoc(fmt.Sprintf("bound-%02d", i), fmt.Sprintf("Doc %02d", i))
		ordinary = append(ordinary, id)
	}
	rh.h.NoteTextureRootChange(t.Context(), rootOwner, rootComputer, story)
	rh.advance(time.Hour)
	digests := rh.digests()
	rh.deliver(digests[len(digests)-1])
	marker, ok := rh.marker()
	if !ok {
		t.Fatal("no master")
	}
	root, live := rh.activeRun(marker.DocumentID)
	if !live {
		t.Fatal("no root run")
	}
	if !strings.Contains(root.Prompt, "3 more changed documents are left for the next digest") {
		t.Fatalf("the run does not say what was left:\n%s", root.Prompt[len(root.Prompt)-600:])
	}
	if strings.Contains(root.Prompt, "refs: doc="+story) {
		t.Fatal("the root read a story")
	}
	if !strings.Contains(root.Prompt, "refs: doc="+ordinary[0]) || strings.Contains(root.Prompt, "refs: doc="+ordinary[len(ordinary)-1]) {
		t.Fatal("the run did not read the oldest first and leave the newest")
	}
	if len(rh.digests()) == len(digests) {
		t.Fatal("no digest was armed for the remainder")
	}
	rh.write(rh.h, &root, "master", "Digested the first fifty.", "")
	pending := rh.pendingDocs()
	if len(pending) != 3 || !contains(pending, ordinary[len(ordinary)-1]) {
		t.Fatalf("pending after the commit = %v, want the 3 left", pending)
	}
}

// Item 2: a document whose ledger read fails is named, not skipped, and the
// watermark does not pass it. Here its trajectory does not exist.
func TestTextureRootAnUnreadableDocumentIsNamedAndHoldsTheWatermark(t *testing.T) {
	rh := newRootHarness(t)
	good, runGood := rh.createDoc("unread-good", "Harbor")
	rh.write(rh.h, &runGood, "harbor", "The harbor is open.", "")
	bad, runBad := rh.createDoc("unread-bad", "Broken")
	rh.write(rh.h, &runBad, "broken", "A broken one.", "")
	badTrajectory := rh.snapshot(bad).Trajectory.TrajectoryID
	changed, _, err := rh.s.ListTextureDocumentsChangedSince(t.Context(), rootOwner, rootComputer, time.Time{})
	if err != nil || len(changed) < 2 {
		t.Fatalf("changed = %d, %v", len(changed), err)
	}
	var badChangedAt time.Time
	for _, entry := range changed {
		if entry.Doc.DocID == bad {
			badChangedAt = entry.ChangedAt
		}
	}
	rh.h.textureRootReadSnapshot = func(ctx context.Context, ownerID, computerID, trajectoryID string) (types.LifecycleSnapshot, error) {
		if trajectoryID == badTrajectory {
			return types.LifecycleSnapshot{}, fmt.Errorf("the snapshot cannot be decoded")
		}
		return rh.s.GetLifecycleSnapshot(ctx, ownerID, computerID, trajectoryID)
	}
	rh.deliver(rh.digests()[0])
	marker, _ := rh.marker()
	root, live := rh.activeRun(marker.DocumentID)
	if !live {
		t.Fatal("no root run")
	}
	if !strings.Contains(root.Prompt, "unread: ") || !strings.Contains(root.Prompt, bad) {
		t.Fatalf("the ledger does not name the unread document:\n%s", root.Prompt[len(root.Prompt)-500:])
	}
	through, _ := root.Metadata[textureRootDigestedKey].(string)
	at, err := time.Parse(time.RFC3339Nano, through)
	if err != nil || at.After(badChangedAt) {
		t.Fatalf("the run's watermark %q passes the unread document (changed %s): %v", through, badChangedAt, err)
	}
	rh.write(rh.h, &root, "master", "Harbor: open.", "")
	pending := rh.pendingDocs()
	if !contains(pending, bad) || contains(pending, good) {
		t.Fatalf("pending = %v; want the broken document held and the good one digested", pending)
	}
}

// Item 2: a listing error is an error, never an empty ledger.
func TestTextureRootAFailedListingFailsTheDigestNotTheLedger(t *testing.T) {
	rh := newRootHarness(t)
	rh.createDoc("list-b", "Harbor")
	marker, _, err := rh.h.ensureTextureRoot(t.Context(), rootOwner, rootComputer)
	if err != nil {
		t.Fatal(err)
	}
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := rh.h.textureRootScan(cancelled, rootOwner, rootComputer, marker); err == nil {
		t.Fatal("a scan on a failing store returned an empty answer, not an error")
	}
}

// Item 4: K planned restarts with continuous commits. The hour bound is read
// from the store, so staggered digests (each restart arms its own) start no
// second root run inside the hour; they re-arm for when it may start.
func TestTextureRootRestartsAndContinuousCommitsStartOneRootRunAnHour(t *testing.T) {
	rh, master, root, _, _ := rootStarted(t)
	rh.write(rh.h, &root, "master", "Harbor: open. Bridge: closed.", "") // the first run commits
	passivateRun(t, rh.s, root)
	if n := rh.rootRuns(master); n != 1 {
		t.Fatalf("root runs = %d, want 1", n)
	}
	handler := rh.h
	for k := 0; k < 4; k++ {
		rh.advance(10 * time.Minute)
		handler = NewHandler(rh.core) // a planned restart: nothing armed in memory
		rh.wire(handler)
		id, runID := rh.createDoc(fmt.Sprintf("restart-%d", k), fmt.Sprintf("Ferry %d", k))
		rh.write(handler, &runID, fmt.Sprintf("ferry%d", k), "The ferry runs.", "")
		_ = id
		digests := rh.digests()
		if err := handler.HandleTextureRootDigest(t.Context(), rootOwner, rootComputer, digests[len(digests)-1].content); err != nil {
			t.Fatal(err)
		}
		if n := rh.rootRuns(master); n != 1 {
			t.Fatalf("after restart %d with a commit, %d root runs in the hour, want 1", k+1, n)
		}
		held := rh.digests()[len(rh.digests())-1]
		if !held.notBefore.After(rh.clock()) {
			t.Fatalf("restart %d: the held digest is due %s, not after now (%s)", k+1, held.notBefore, rh.clock())
		}
	}
	rh.advance(2 * time.Hour)
	digests := rh.digests()
	if err := handler.HandleTextureRootDigest(t.Context(), rootOwner, rootComputer, digests[len(digests)-1].content); err != nil {
		t.Fatal(err)
	}
	if n := rh.rootRuns(master); n != 2 {
		t.Fatalf("after the hour, %d root runs, want 2", n)
	}
}

// Item 6: after runs that moved no watermark the wait doubles, and a run that
// moves it resets the wait.
func TestTextureRootBackoffDoublesAfterRunsThatMoveNothingAndResets(t *testing.T) {
	rh, master, root, _, _ := rootStarted(t)
	deliverLast := func() {
		digests := rh.digests()
		rh.deliver(digests[len(digests)-1])
	}
	passivateRun(t, rh.s, root) // run 1 committed nothing: failures = 1, wait 2 h
	rh.h.NoteTextureRootChange(t.Context(), rootOwner, rootComputer, "doc-x")
	rh.advance(90 * time.Minute)
	deliverLast()
	if n := rh.rootRuns(master); n != 1 {
		t.Fatalf("90 min after a run that moved nothing: %d runs, want 1 (wait 2 h)", n)
	}
	rh.advance(45 * time.Minute)
	deliverLast()
	if n := rh.rootRuns(master); n != 2 {
		t.Fatalf("past 2 h: %d runs, want 2", n)
	}
	second, _ := rh.activeRun(master)
	passivateRun(t, rh.s, second) // run 2 committed nothing either: failures = 2, wait 4 h
	rh.h.NoteTextureRootChange(t.Context(), rootOwner, rootComputer, "doc-y")
	rh.advance(3 * time.Hour)
	deliverLast()
	if n := rh.rootRuns(master); n != 2 {
		t.Fatalf("3 h after the second: %d runs, want 2 (wait 4 h)", n)
	}
	rh.advance(90 * time.Minute)
	deliverLast()
	third, ok := rh.activeRun(master)
	if !ok || rh.rootRuns(master) != 3 {
		t.Fatalf("past 4 h: %d runs, want 3", rh.rootRuns(master))
	}
	rh.write(rh.h, &third, "master", "Rendered.", "") // run 3 moves the watermark: reset
	passivateRun(t, rh.s, third)
	if _, failures, err := rh.h.textureRootGate(t.Context(), rootOwner, rootComputer, store.TextureRoot{OwnerID: rootOwner, ComputerID: rootComputer, DocumentID: master}); err != nil || failures != 0 {
		t.Fatalf("after a run that moved the watermark: failures %d, %v; want 0", failures, err)
	}
}

// Item 6: an archived master is not live: no wake on it; the next digest
// replaces it. The store archives only a settled trajectory, so the master is
// settled first (the settled master is also replaced; the ArchivedAt guard in
// textureRootLive is the second reason).
func TestTextureRootAnArchivedMasterIsReplacedNotWoken(t *testing.T) {
	rh, master, root, b, _ := rootStarted(t)
	snapshot := rh.snapshot(master)
	standing := openWorkFor(snapshot)[0]
	settle := types.SettleLifecycleWorkRequest{
		OwnerID: rootOwner, ComputerID: rootComputer, CommandID: "settle-standing-arch",
		TrajectoryID: snapshot.Trajectory.TrajectoryID, WorkItemID: standing.WorkItemID,
		ActingAgentID: "texture:" + master, ResultRef: snapshot.HeadRevision.RevisionID,
	}
	settle.CommandDigest, _ = store.ComputeSettleLifecycleWorkDigest(settle)
	if _, err := rh.s.SettleLifecycleWork(t.Context(), settle); err != nil {
		t.Fatal(err)
	}
	passivateRun(t, rh.s, root)
	snapshot = rh.snapshot(master)
	settleTrajectory := types.SettleLifecycleTrajectoryRequest{
		OwnerID: rootOwner, ComputerID: rootComputer, CommandID: "settle-master-arch",
		TrajectoryID: snapshot.Trajectory.TrajectoryID, ExpectedLifecycleVersion: snapshot.Trajectory.LifecycleVersion,
		ExpectedHeadRevisionID: snapshot.HeadRevision.RevisionID,
	}
	settleTrajectory.CommandDigest, _ = store.ComputeSettleLifecycleTrajectoryDigest(settleTrajectory)
	if _, err := rh.s.SettleLifecycleTrajectory(t.Context(), settleTrajectory); err != nil {
		t.Fatal(err)
	}
	if _, err := rh.s.ArchiveTextureDocumentAuthority(asTextureCustodian(t.Context()), master, rootOwner, rootComputer); err != nil {
		t.Fatalf("archive the master: %v", err)
	}
	rh.advance(3 * time.Hour)
	rh.h.NoteTextureRootChange(t.Context(), rootOwner, rootComputer, b)
	rh.deliver(rh.digests()[len(rh.digests())-1])
	marker, ok := rh.marker()
	if !ok || marker.Generation != 2 || marker.DocumentID == master {
		t.Fatalf("marker after an archived master = %+v, want generation 2", marker)
	}
	if _, ok := rh.activeRun(master); ok {
		t.Fatal("the archived master was woken")
	}
}

// Item 6: a malformed watermark is none (one digest of everything), logged,
// never an error that wedges every digest.
func TestTextureRootAMalformedWatermarkIsNone(t *testing.T) {
	bad := types.Revision{RevisionID: "r2", Metadata: json.RawMessage(`{"texture_root_digested_through":"yesterday"}`), ParentRevisionID: "r1"}
	good := types.Revision{RevisionID: "r1", Metadata: json.RawMessage(`{"texture_root_digested_through":"2026-11-20T10:00:00Z"}`)}
	state, err := walkTextureRootWatermark("r2", 32, func(id string) (types.Revision, error) {
		if id == "r2" {
			return bad, nil
		}
		return good, nil
	})
	if err != nil || !state.Through.IsZero() {
		t.Fatalf("a malformed watermark = %+v, %v; want none and no error", state, err)
	}
}

// Item 7: the walk is bounded at 32 revisions (the head and 31 behind it).
// 31 owner edits on top of a root revision still find it; 32 do not, and the
// answer is none, one digest of everything changed; the next root revision's
// watermark ends it (there is no loop: it is found at the head).
func TestTextureRootTheWatermarkWalkBoundIsTheStatedOne(t *testing.T) {
	chain := func(ownerEdits int) func(string) (types.Revision, error) {
		return func(id string) (types.Revision, error) {
			var n int
			if _, err := fmt.Sscanf(id, "r%d", &n); err != nil {
				return types.Revision{}, err
			}
			if n == ownerEdits { // the root's revision is the oldest of the chain
				return types.Revision{RevisionID: id, Metadata: json.RawMessage(`{"texture_root_digested_through":"2026-11-20T10:00:00Z"}`)}, nil
			}
			return types.Revision{RevisionID: id, ParentRevisionID: fmt.Sprintf("r%d", n+1)}, nil
		}
	}
	for edits, want := range map[int]bool{31: true, 32: false} {
		state, err := walkTextureRootWatermark("r0", textureRootWatermarkWalk, chain(edits))
		if err != nil || (!state.Through.IsZero()) != want {
			t.Fatalf("%d owner edits on top: found=%v err=%v, want found=%v", edits, !state.Through.IsZero(), err, want)
		}
	}
	state, _ := walkTextureRootWatermark("head", textureRootWatermarkWalk, func(id string) (types.Revision, error) {
		return types.Revision{RevisionID: id, Metadata: json.RawMessage(`{"texture_root_digested_through":"2026-11-20T11:00:00Z"}`)}, nil
	})
	if state.Through.IsZero() {
		t.Fatal("a fresh root revision's watermark is not found at the head")
	}
}

// Item 5: the order of the excerpt comes from what the master's head recorded.
func TestTextureRootExcerptOrderComesFromTheMastersRecordedDocuments(t *testing.T) {
	base := time.Date(2026, 11, 20, 9, 0, 0, 0, time.UTC)
	head := types.Revision{Metadata: json.RawMessage(fmt.Sprintf(`{"texture_root_digested_through":%q,"texture_root_digested_docs":{"old":{"at":%q,"m":"x"},"new":{"at":%q,"m":"y"}}}`,
		base.Add(2*time.Hour).Format(time.RFC3339Nano), base.Format(time.RFC3339Nano), base.Add(time.Hour).Format(time.RFC3339Nano)))}
	got := textureRootRevisedAtOf(head)
	if !got["new"].After(got["old"]) || len(got) != 2 {
		t.Fatalf("recorded order = %v, want new after old", got)
	}
}
