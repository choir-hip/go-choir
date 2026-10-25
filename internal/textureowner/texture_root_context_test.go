package textureowner

// PD3 S2, shared context (docs/definitions/choir-pd3-texture-root-desk-
// 2026-10-11.md, "Shared context (S2)"): at wake, the run of every document
// other than the master carries the master's latest revision, at most 4 KiB,
// as read-only context outside the prompt core, "waits on the owner" first,
// truncated from the end. Ways it can fail, written before the code:
//
//	off, or on with no live master, the run changes (it must not change at
//	all); the master's own run gets the excerpt (it reads the ledger);
//	the excerpt crosses 4 KiB, cuts the front instead of the end, or does
//	not lead with what waits on the owner; it repeats the run's own
//	document's section; it is unlabelled, or labelled as owner input or an
//	instruction; it shows a master the root never wrote (the seed or an
//	owner edit) as the root's.

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/textureprompts"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// masterText is what a root writes: the "Waits on the owner" section is
// written last here, so the excerpt's order is its own.
func masterText(b, c string) string {
	return "## Harbor\n\nResearch is reading the ferry timetable.\n\nrefs: doc=" + b + " work=none\n\n" +
		"## Bridge\n\nThe bridge story is waiting for a capture.\n\nrefs: doc=" + c + " work=none\n\n" +
		"## Waits on the owner\n\n- Bridge: approve the engineering candidate."
}

// freshRun starts a new Texture run on docID: its current run is passivated
// and an owner revision wakes it.
func (rh *rootHarness) freshRun(docID, requestID string) types.RunRecord {
	rh.t.Helper()
	if run, ok := rh.activeRun(docID); ok {
		passivateRun(rh.t, rh.s, run)
	}
	head := rh.snapshot(docID).HeadRevision.RevisionID
	if resp := postOwnerInstruction(rh.t, rh.h, "/api/texture/documents/"+docID+"/revise", rootOwner, requestID, "Add the times.", head); resp.Code != http.StatusAccepted {
		rh.t.Fatalf("owner revise %s: %d %s", docID, resp.Code, resp.Body.String())
	}
	if _, err := rh.h.ReconcileAgentWake(rh.t.Context(), rootOwner, docID); err != nil {
		rh.t.Fatalf("wake %s: %v", docID, err)
	}
	run, ok := rh.activeRun(docID)
	if !ok {
		rh.t.Fatalf("no fresh run on %s", docID)
	}
	return run
}

func hasRootContext(run types.RunRecord) bool {
	return strings.Contains(run.Prompt, textureprompts.RootContextLabel())
}

// Off, or on with no master yet, the run is exactly what it was.
func TestTextureRootContextOffOrNoMasterChangesNothing(t *testing.T) {
	off := newRootHarness(t, false)
	_, runOff := off.createDoc("ctx-b", "Harbor")
	on := newRootHarness(t, true)
	_, runOn := on.createDoc("ctx-b", "Harbor")
	if _, ok := on.marker(); ok {
		t.Fatal("creating a document claimed a master")
	}
	if hasRootContext(runOff) || hasRootContext(runOn) {
		t.Fatal("a run got the master's excerpt with the switch off or no master")
	}
	if runOff.Prompt != runOn.Prompt {
		t.Fatalf("on with no master changed the run's prompt:\noff:\n%s\non:\n%s", runOff.Prompt, runOn.Prompt)
	}
	if _, ok := runOn.Metadata["texture_root_context_revision_id"]; ok {
		t.Fatal("on with no master stamped the root context on the run")
	}
}

// On with a live master the root has written: another document's fresh run
// carries the excerpt, labelled, with "waits on the owner" first and without
// its own document's section; the master's own run does not.
func TestTextureRootContextReachesOtherDocumentsNotTheMaster(t *testing.T) {
	rh, master, root, b, c := rootStarted(t)
	// The seed is not the root's writing: no excerpt before the root writes.
	if run := rh.freshRun(b, "ctx-before"); hasRootContext(run) {
		t.Fatal("a master the root never wrote (its seed) was shown as the root's")
	}
	rh.write(rh.h, &root, "master", masterText(b, c), "")
	head := rh.snapshot(master).HeadRevision

	run := rh.freshRun(b, "ctx-after")
	if !hasRootContext(run) {
		t.Fatalf("another document's run lacks the master's excerpt:\n%s", run.Prompt)
	}
	if run.Metadata["texture_root_context_revision_id"] != head.RevisionID {
		t.Fatalf("run metadata names master revision %v, want %s", run.Metadata["texture_root_context_revision_id"], head.RevisionID)
	}
	excerpt := run.Prompt[strings.Index(run.Prompt, textureprompts.RootContextLabel()):]
	waits, bridge := strings.Index(excerpt, "approve the engineering candidate"), strings.Index(excerpt, "waiting for a capture")
	if waits < 0 || bridge < 0 || waits > bridge {
		t.Fatalf("the excerpt does not lead with what waits on the owner:\n%s", excerpt)
	}
	if strings.Contains(excerpt, "refs: doc="+b) || strings.Contains(excerpt, "ferry timetable") {
		t.Fatalf("the excerpt repeats the run's own document's section:\n%s", excerpt)
	}
	if !strings.Contains(excerpt, "refs: doc="+c) {
		t.Fatalf("the excerpt lacks the other document's lines:\n%s", excerpt)
	}
	if strings.Contains(textureprompts.DefaultSystemPrompt(), textureprompts.RootContextLabel()) {
		t.Fatal("the excerpt's label is in the prompt core")
	}

	// The master's own next run reads the ledger, not the excerpt.
	passivateRun(t, rh.s, root)
	rh.advance(3 * time.Minute)
	rh.h.NoteTextureRootChange(t.Context(), rootOwner, rootComputer, c)
	rh.advance(3 * time.Minute)
	rh.deliver(rh.digests()[len(rh.digests())-1])
	next, ok := rh.activeRun(master)
	if !ok || next.RunID == root.RunID {
		t.Fatal("the digest started no new root run")
	}
	if hasRootContext(next) {
		t.Fatal("the master's own run got the master's excerpt")
	}
}

// A master that is no longer live is not shown: a settled master's text is
// stale until the next digest replaces it.
func TestTextureRootContextNeedsALiveMaster(t *testing.T) {
	rh, master, root, b, c := rootStarted(t)
	rh.write(rh.h, &root, "master", masterText(b, c), "")
	snapshot := rh.snapshot(master)
	standing := openWorkFor(snapshot)[0]
	settle := types.SettleLifecycleWorkRequest{
		OwnerID: rootOwner, ComputerID: rootComputer, CommandID: "ctx-settle-standing",
		TrajectoryID: snapshot.Trajectory.TrajectoryID, WorkItemID: standing.WorkItemID,
		ActingAgentID: "texture:" + master, ResultRef: snapshot.HeadRevision.RevisionID,
	}
	settle.CommandDigest, _ = store.ComputeSettleLifecycleWorkDigest(settle)
	if _, err := rh.s.SettleLifecycleWork(t.Context(), settle); err != nil {
		t.Fatalf("complete the standing item: %v", err)
	}
	if run := rh.freshRun(b, "ctx-dead"); hasRootContext(run) {
		t.Fatal("a master that is no longer live was shown")
	}
}

// The bound and the order: at most 4 KiB of the master, what waits on the
// owner first, the rest cut from the end, the run's own section left out.
func TestTextureRootExcerptIsBoundedAndOrdered(t *testing.T) {
	var b strings.Builder
	b.WriteString("# Master document\n\n")
	for i := 0; i < 60; i++ {
		fmt.Fprintf(&b, "## Doc %02d\n\nstatus line for document %02d, %s\n\nrefs: doc=doc-%02d work=none\n\n", i, i, strings.Repeat("x", 60), i)
	}
	b.WriteString("## Waits on the owner\n\n- Doc 59: approve the candidate.\n")
	excerpt := textureRootExcerpt(b.String(), "doc-01", nil)
	if len(excerpt) > textureRootContextMaxBytes || textureRootContextMaxBytes != 4<<10 {
		t.Fatalf("excerpt is %d bytes, over the 4 KiB bound", len(excerpt))
	}
	if !strings.HasPrefix(strings.TrimSpace(excerpt), "## Waits on the owner") {
		t.Fatalf("the excerpt does not lead with what waits on the owner:\n%.300s", excerpt)
	}
	if !strings.Contains(excerpt, "refs: doc=doc-00") || strings.Contains(excerpt, "refs: doc=doc-59") {
		t.Fatal("the excerpt was not cut from the end")
	}
	if strings.Contains(excerpt, "refs: doc=doc-01 ") || strings.Contains(excerpt, "## Doc 01\n") {
		t.Fatal("the excerpt kept the run's own document's section")
	}
	// A "waits on the owner" section over the bound is itself cut from its end.
	long := "## Waits on the owner\n\n" + strings.Repeat("- a thing that waits on the owner\n", 300) + "\n## Doc 00\n\nrefs: doc=doc-00 work=none\n"
	if cut := textureRootExcerpt(long, "doc-99", nil); len(cut) > textureRootContextMaxBytes || !strings.HasPrefix(cut, "## Waits on the owner") {
		t.Fatalf("a long waits section: %d bytes, prefix %.40q", len(cut), cut)
	}
}

// S2 paired test F4 (docs/evidence/pd3-s1-acceptance-2026-10-25.md): the
// root appends new documents at the end, and the excerpt cut from the end,
// so a run saw the oldest sections and never the newest. After "Waits on the
// owner", the excerpt orders the other sections by their documents' last
// revision time, newest first, whatever order the root wrote; sections whose
// document the listing does not know go last in their original order; then
// the run's own section is dropped and the whole cut to 4 KiB.
func TestTextureRootExcerptPutsTheNewestDocumentsFirst(t *testing.T) {
	base := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	revisedAt := map[string]time.Time{}
	var b strings.Builder
	b.WriteString("# Master document\n\n")
	for i := 0; i < 60; i++ { // oldest first, as the root wrote it
		docID := fmt.Sprintf("doc-%02d", i)
		revisedAt[docID] = base.Add(time.Duration(i) * time.Minute)
		fmt.Fprintf(&b, "## Doc %02d\n\nstatus line for document %02d, %s\n\nrefs: doc=%s work=none\n\n", i, i, strings.Repeat("x", 60), docID)
		if i == 30 {
			b.WriteString("## Waits on the owner\n\n- Doc 59: approve the offsite plan.\n\n")
		}
	}
	excerpt := textureRootExcerpt(b.String(), "doc-58", revisedAt)
	if len(excerpt) > textureRootContextMaxBytes {
		t.Fatalf("excerpt is %d bytes, over the 4 KiB bound", len(excerpt))
	}
	if !strings.HasPrefix(excerpt, "## Waits on the owner") {
		t.Fatalf("the waits section is not first:\n%.200s", excerpt)
	}
	if !strings.Contains(excerpt, "refs: doc=doc-59 ") || strings.Contains(excerpt, "refs: doc=doc-00 ") {
		t.Fatalf("over 4 KiB the excerpt kept the oldest document and dropped the newest:\n%s", excerpt)
	}
	if strings.Contains(excerpt, "refs: doc=doc-58 ") {
		t.Fatal("the excerpt kept the run's own section")
	}
	if i59, i57 := strings.Index(excerpt, "doc=doc-59 "), strings.Index(excerpt, "doc=doc-57 "); i57 < 0 || i59 > i57 {
		t.Fatalf("the newest document is not first after the waits section:\n%s", excerpt)
	}

	// Unknown documents (and sections with no refs line) go last, in order.
	small := "## Notes A\n\nno refs here\n\n## Old\n\nrefs: doc=old work=none\n\n## Stranger\n\nrefs: doc=stranger work=none\n\n## New\n\nrefs: doc=new work=none\n\n## Waits on the owner\n\n- nothing"
	got := textureRootExcerpt(small, "", map[string]time.Time{"old": base, "new": base.Add(time.Hour)})
	order := []string{"## Waits on the owner", "## New", "## Old", "## Notes A", "## Stranger"}
	last := -1
	for _, want := range order {
		at := strings.Index(got, want)
		if at < 0 || at < last {
			t.Fatalf("order of %q wrong, want %v:\n%s", want, order, got)
		}
		last = at
	}
}

// One document listing per run, and on a real computer the newest document
// survives the cut: the root wrote Harbor (older) before Bridge (newer), each
// over half the bound, and a new document's run sees Bridge.
func TestTextureRootContextReadsOneListingAndKeepsTheNewest(t *testing.T) {
	rh, _, root, b, c := rootStarted(t)
	filler := strings.Repeat("A status line that takes room in the master. ", 50)
	master := "## Harbor\n\n" + filler + "\n\nrefs: doc=" + b + " work=none\n\n" +
		"## Bridge\n\n" + filler + "\n\nrefs: doc=" + c + " work=none\n\n" +
		"## Waits on the owner\n\n- nothing waits on the owner."
	rh.write(rh.h, &root, "master", master, "")
	listings := 0
	rh.h.textureRootListDocuments = func(ctx context.Context, ownerID, computerID string, limit int) ([]types.Document, error) {
		listings++
		return rh.s.ListDocumentsByScope(ctx, ownerID, computerID, limit)
	}
	_, run := rh.createDoc("ctx-newest-d", "Ferry")
	if listings != 1 {
		t.Fatalf("one run read the document listing %d times, want 1", listings)
	}
	if !hasRootContext(run) {
		t.Fatalf("the new document's run lacks the excerpt:\n%s", run.Prompt)
	}
	excerpt := run.Prompt[strings.Index(run.Prompt, textureprompts.RootContextLabel()):]
	if !strings.Contains(excerpt, "refs: doc="+c) || strings.Contains(excerpt, "refs: doc="+b) {
		t.Fatalf("the excerpt kept the older document and cut the newer one:\n%s", excerpt)
	}
}

// The root writes each document's refs at the end of its paragraph, not on a
// line of their own (b@c's master ef0e2ef8, 2026-10-25; the S2 paired test's
// F5). The excerpt must find those refs, or every section counts as unknown:
// it keeps the root's oldest-first order, cuts the newest documents, and
// repeats the run's own section.
func TestTextureRootExcerptReadsTheRootsInlineRefs(t *testing.T) {
	base := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	revisedAt := map[string]time.Time{}
	var b strings.Builder
	b.WriteString("## Waits on the owner\n\nNone listed across the documents in this ledger.\n\n" +
		"The computer's master document: for every document on this computer, what the owner asked, what is open, which desk holds it and what waits on the owner. Kept current by this document's Texture, the computer's Texture root.\n\n")
	for i := 0; i < 20; i++ { // oldest first, as the root wrote it
		docID := fmt.Sprintf("0b4f3866-5966-5a06-9582-e03d1ae4cb%02d", i)
		revisedAt[docID] = base.Add(time.Duration(i) * time.Minute)
		fmt.Fprintf(&b, "### Change this computer itself: redesign document %02d\n\nWaits on owner: none listed. The owner asked for a full, careful rewrite of the document in four paragraphs. Open work: none listed; no desk assignment or candidate is listed. refs: doc=%s work=none\n\n", i, docID)
	}
	offsite, own := "5a090ab3-0000-5000-8000-000000000001", "e89959c8-0000-5000-8000-000000000002"
	revisedAt[offsite] = base.Add(time.Hour)
	revisedAt[own] = base.Add(2 * time.Hour)
	b.WriteString("### Team offsite plan\n\nWaits on owner: none listed. The owner set a full-day team offsite at the lake house on Saturday, November 7th. Open work: none listed. refs: doc=" + offsite + " work=none\n\n")
	b.WriteString("### Reunion booking (on)\n\nWaits on owner: none listed. The owner asked to book the lake house for a family reunion. Open work: Texture is assigned to revise. refs: doc=" + own + " work=4c55862e\n")
	if b.Len() <= textureRootContextMaxBytes {
		t.Fatalf("fixture is %d bytes; it must exceed the bound to test the cut", b.Len())
	}
	excerpt := textureRootExcerpt(b.String(), own, revisedAt)
	if len(excerpt) > textureRootContextMaxBytes {
		t.Fatalf("excerpt is %d bytes, over the 4 KiB bound", len(excerpt))
	}
	if !strings.HasPrefix(excerpt, "## Waits on the owner") {
		t.Fatalf("the waits section is not first:\n%.200s", excerpt)
	}
	at := strings.Index(excerpt, "### Team offsite plan")
	if at < 0 {
		t.Fatalf("the newest other document was cut; the inline refs were not read:\n%s", excerpt)
	}
	if first := strings.Index(excerpt, "### Change this computer itself"); first >= 0 && first < at {
		t.Fatalf("an older document comes before the newest:\n%s", excerpt)
	}
	if strings.Contains(excerpt, own) {
		t.Fatalf("the excerpt kept the run's own section:\n%s", excerpt)
	}
}
