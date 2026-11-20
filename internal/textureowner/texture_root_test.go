package textureowner

// PD3 S1: one Texture root desk per computer, the master document's own
// Texture desk, woken by a debounced texture_root_digest after other
// documents' commits
// (docs/archive/definitions/choir-pd3-texture-root-desk-2026-10-11.md). Ways it can
// fail, written before the code (the design's numbering):
//
//	3   the root wakes on every commit (digests are coalesced and rate bound);
//	3a  a commit during a live root run is lost to coalescing (one re-armed
//	    digest after the run);
//	3b  the ensure step races and creates two masters (the marker's CAS);
//	3c  the master settles and the root can never write again (replacement
//	    at the next digest, before activation; the generation advances);
//	5   the master drops open work (each document's refs line names its open
//	    work items' short ids);
//	7   a crash leaves a digest to resume (actorruntime test; here: the next
//	    commit after a boot mints a new digest and boot runs nothing);
//	    the root's own commits wake the root; the root completes its
//	    standing item; the read is unbounded.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentcore"
	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/textureprompts"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

const rootOwner, rootComputer = "user-1", "autoputer-test"

type rootWake struct {
	to, kind, content string
	notBefore         time.Time
}

type rootHarness struct {
	t         *testing.T
	core      *agentcore.Runtime
	h         *Handler
	s         *store.Store
	mu        sync.Mutex
	scheduled []rootWake
	now       time.Time
}

func newRootHarness(t *testing.T) *rootHarness {
	t.Helper()
	core, h := testAPISetup(t)
	rh := &rootHarness{t: t, core: core, h: h, s: core.Store(), now: time.Now().UTC()}
	core.SetDispatchActor(func(context.Context, string, string, string, string, string, string, string) error { return nil })
	core.SetScheduleActor(func(_ context.Context, _, _, to, kind, content, _, _ string, notBefore time.Time) error {
		rh.mu.Lock()
		defer rh.mu.Unlock()
		rh.scheduled = append(rh.scheduled, rootWake{to: to, kind: kind, content: content, notBefore: notBefore})
		return nil
	})
	rh.wire(h)
	return rh
}

// wire points a handler's digest clock at the harness clock.
func (rh *rootHarness) wire(h *Handler) {
	h.textureRootNow = func() time.Time {
		rh.mu.Lock()
		defer rh.mu.Unlock()
		return rh.now
	}
}

func (rh *rootHarness) advance(d time.Duration) {
	rh.mu.Lock()
	defer rh.mu.Unlock()
	rh.now = rh.now.Add(d)
}

func (rh *rootHarness) clock() time.Time {
	rh.mu.Lock()
	defer rh.mu.Unlock()
	return rh.now
}

func (rh *rootHarness) digests() []rootWake {
	rh.mu.Lock()
	defer rh.mu.Unlock()
	var out []rootWake
	for _, w := range rh.scheduled {
		if w.kind == agentcore.TextureRootDigestKind {
			out = append(out, w)
		}
	}
	return out
}

// createDoc opens a document through the ordinary create path, which also
// starts its Texture run.
func (rh *rootHarness) createDoc(clientID, title string) (string, types.RunRecord) {
	rh.t.Helper()
	body := fmt.Sprintf(`{"client_request_id":%q,"title":%q,"initial_content":"Write about %s."}`, clientID, title, title)
	w := runtimeHandlerRequest(rh.t, rh.h.HandleTextureLifecycleCreate, http.MethodPost, "/api/texture/lifecycle/create", body, rootOwner)
	if w.Code != http.StatusCreated {
		rh.t.Fatalf("create %s: %d %s", title, w.Code, w.Body.String())
	}
	var resp textureLifecycleCreateResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		rh.t.Fatal(err)
	}
	run, ok := rh.activeRun(resp.DocID)
	if !ok {
		rh.t.Fatalf("create %s started no Texture run", title)
	}
	return resp.DocID, run
}

func (rh *rootHarness) activeRun(docID string) (types.RunRecord, bool) {
	rh.t.Helper()
	run, found, err := rh.core.TextureActiveRunByAgent(rh.t.Context(), rootOwner, rootComputer, "texture:"+docID)
	if err != nil {
		rh.t.Fatal(err)
	}
	return run, found
}

func (rh *rootHarness) write(h *Handler, run *types.RunRecord, identity, text, done string) {
	rh.t.Helper()
	base := headOf(rh.t, rh.s, *run)
	// A Write that leaves its request open says what it waits on
	// (texture_next_actor.go); these documents wait on the next report.
	wait := ""
	var cast []types.CommitmentRecord
	if done == "" {
		// A Wait nothing serves closes the request, so the report it waits
		// on is asked of research.
		wait = "the next report"
		cast = []types.CommitmentRecord{researchCast(*run, "texture-cell:cast:"+identity)}
	}
	turn := turnBody(rh.t, yaegikernel.TextureTurn{BaseRevisionID: base.RevisionID, Done: done, Wait: wait, InboxRecords: cast, Write: &yaegikernel.TextureWrite{Edits: "@all\n# " + identity + "\n\n" + text, Why: "report " + identity}})
	if _, err := h.CommitCellTextureAuthor(rh.t.Context(), run, turn, identity); err != nil {
		rh.t.Fatalf("Write %s: %v", identity, err)
	}
}

func (rh *rootHarness) deliver(w rootWake) {
	rh.t.Helper()
	if err := rh.h.HandleTextureRootDigest(rh.t.Context(), rootOwner, rootComputer, w.content); err != nil {
		rh.t.Fatalf("deliver digest: %v", err)
	}
}

// pendingDocs is the documents the next digest would find changed after the
// master's watermark, read from the store as the digest reads them.
func (rh *rootHarness) pendingDocs() []string {
	rh.t.Helper()
	var ids []string
	if marker, ok := rh.marker(); ok {
		scan, err := rh.h.textureRootScan(rh.t.Context(), rootOwner, rootComputer, marker)
		if err != nil {
			rh.t.Fatal(err)
		}
		for _, entry := range scan.Read {
			ids = append(ids, entry.Doc.DocID)
		}
		return ids
	}
	entries, _, err := rh.s.ListTextureDocumentsChangedSince(rh.t.Context(), rootOwner, rootComputer, time.Time{})
	if err != nil {
		rh.t.Fatal(err)
	}
	for _, entry := range entries {
		ids = append(ids, entry.Doc.DocID)
	}
	return ids
}

func (rh *rootHarness) pendingHas(docID string) bool {
	for _, d := range rh.pendingDocs() {
		if d == docID {
			return true
		}
	}
	return false
}

// rootRuns is every run the master's desk has had.
func (rh *rootHarness) rootRuns(master string) int {
	rh.t.Helper()
	runs, err := rh.s.ListRunsByAgent(rh.t.Context(), rootOwner, "texture:"+master, 100)
	if err != nil {
		rh.t.Fatal(err)
	}
	return len(runs)
}

func (rh *rootHarness) marker() (store.TextureRoot, bool) {
	rh.t.Helper()
	root, err := rh.s.GetTextureRoot(rh.t.Context(), rootOwner, rootComputer)
	if err != nil {
		return store.TextureRoot{}, false
	}
	return root, true
}

func (rh *rootHarness) snapshot(docID string) types.LifecycleSnapshot {
	rh.t.Helper()
	doc, err := rh.s.GetLifecycleDocument(rh.t.Context(), rootOwner, rootComputer, docID)
	if err != nil {
		rh.t.Fatal(err)
	}
	snapshot, err := rh.s.GetLifecycleSnapshot(rh.t.Context(), rootOwner, rootComputer, doc.TrajectoryID)
	if err != nil {
		rh.t.Fatal(err)
	}
	return snapshot
}

func openWorkFor(snapshot types.LifecycleSnapshot) []types.WorkItemRecord {
	var out []types.WorkItemRecord
	for _, work := range snapshot.WorkItems {
		if work.Status == types.WorkItemOpen {
			out = append(out, work)
		}
	}
	return out
}

func (rh *rootHarness) refsLine(docID string) string {
	rh.t.Helper()
	open := openWorkFor(rh.snapshot(docID))
	if len(open) == 0 {
		return "refs: doc=" + docID + " work=none"
	}
	ids := make([]string, 0, len(open))
	for _, work := range open {
		ids = append(ids, textureRootShortID(work.WorkItemID))
	}
	return "refs: doc=" + docID + " work=" + strings.Join(ids, ",")
}

func (rh *rootHarness) countDocsTitled(title string) int {
	rh.t.Helper()
	docs, err := rh.s.ListDocumentsByScope(rh.t.Context(), rootOwner, rootComputer, 500)
	if err != nil {
		rh.t.Fatal(err)
	}
	n := 0
	for _, doc := range docs {
		if doc.Title == title {
			n++
		}
	}
	return n
}

// rootStarted: two documents commit, the one digest is delivered, the
// master exists and its root run is live.
func rootStarted(t *testing.T) (rh *rootHarness, master string, root types.RunRecord, b string, c string) {
	t.Helper()
	rh = newRootHarness(t)
	b, runB := rh.createDoc("root-b", "Harbor")
	c, runC := rh.createDoc("root-c", "Bridge")
	rh.write(rh.h, &runB, "harbor", "The harbor is open.", "")
	rh.write(rh.h, &runC, "bridge", "The bridge is closed.", "")
	digests := rh.digests()
	if len(digests) != 1 {
		t.Fatalf("two commits minted %d digests, want one coalesced digest", len(digests))
	}
	rh.advance(3 * time.Minute)
	rh.deliver(digests[0])
	marker, ok := rh.marker()
	if !ok {
		t.Fatal("the first digest did not ensure a master document")
	}
	root, live := rh.activeRun(marker.DocumentID)
	if !live {
		t.Fatal("the first digest started no root run")
	}
	return rh, marker.DocumentID, root, b, c
}

// The root desk has no setting (owner, 2026-11-19: "I want everything on"):
// with nothing stored, a commit on another document mints a digest and the
// digest makes the master.
func TestTextureRootRunsWithNoSetting(t *testing.T) {
	rh := newRootHarness(t)
	b, runB := rh.createDoc("nos-b", "Harbor")
	rh.write(rh.h, &runB, "harbor", "The harbor is open.", "")
	if n := len(rh.digests()); n != 1 {
		t.Fatalf("a commit with no setting minted %d digests, want 1", n)
	}
	if !rh.pendingHas(b) {
		t.Fatalf("the commit left %s out of the scope's pending set: %v", b, rh.pendingDocs())
	}
	// The digest the commit armed, not a hand-made one.
	live := rh.digests()[0]
	if strings.Contains(live.content, b) {
		t.Fatalf("the digest names a document: %s", live.content)
	}
	rh.deliver(live)
	marker, ok := rh.marker()
	if !ok {
		t.Fatal("a digest with no setting made no master")
	}
	if _, live := rh.activeRun(marker.DocumentID); !live {
		t.Fatal("the armed digest started no root run")
	}
	if _, known := store.KnownComputerSettings["texture.root_desk"]; known {
		t.Fatal("texture.root_desk is still a known setting")
	}
}

func mustListDocs(t *testing.T, s *store.Store) []types.Document {
	t.Helper()
	docs, err := s.ListDocumentsByScope(t.Context(), rootOwner, rootComputer, 500)
	if err != nil {
		t.Fatal(err)
	}
	return docs
}

// Failure mode 3: commits on other documents mint one coalesced digest,
// after the commit and an hour out.
func TestTextureRootCommitsMintOneDebouncedDigest(t *testing.T) {
	rh := newRootHarness(t)
	b, runB := rh.createDoc("mint-b", "Harbor")
	c, runC := rh.createDoc("mint-c", "Bridge")
	if n := len(rh.digests()); n != 0 {
		t.Fatalf("creating documents minted %d digests", n)
	}
	rh.write(rh.h, &runB, "harbor", "The harbor is open.", "")
	rh.write(rh.h, &runC, "bridge", "The bridge is closed.", "")
	digests := rh.digests()
	if len(digests) != 1 {
		t.Fatalf("two commits minted %d digests, want one", len(digests))
	}
	d := digests[0]
	if d.to != agentcore.TextureRootDigestMailbox || strings.Contains(d.content, b) {
		t.Fatalf("digest = %+v, want the root digest mailbox and no document named in it", d)
	}
	if !rh.pendingHas(b) || !rh.pendingHas(c) {
		t.Fatalf("the scope holds %v, want both %s and %s pending", rh.pendingDocs(), b, c)
	}
	if min := rh.clock().Add(textureRootBatchInterval - time.Second); d.notBefore.Before(min) {
		t.Fatalf("digest due %s, before the batch interval (%s from %s)", d.notBefore, textureRootBatchInterval, rh.clock())
	}
}

// An owner revision is a commit on a document too: it mints a digest.
func TestTextureRootOwnerRevisionMintsADigest(t *testing.T) {
	rh := newRootHarness(t)
	b, _ := rh.createDoc("owner-b", "Harbor")
	head := rh.snapshot(b).HeadRevision.RevisionID
	if resp := postOwnerInstruction(t, rh.h, "/api/texture/documents/"+b+"/revise", rootOwner, "owner-b-1", "Add the ferry times.", head); resp.Code != http.StatusAccepted {
		t.Fatalf("owner revise: %d %s", resp.Code, resp.Body.String())
	}
	if digests := rh.digests(); len(digests) != 1 || !rh.pendingHas(b) {
		t.Fatalf("an owner revision minted %+v with %v pending, want one digest and %s pending", digests, rh.pendingDocs(), b)
	}
}

// The first digest runs the ensure step: a master document made by the
// ordinary create path, marked generation 1, with one open standing work
// item; its root run carries the root section and a refs line per named
// document, and nothing about the master's own standing item.
func TestTextureRootFirstDigestEnsuresTheMasterAndStartsTheRoot(t *testing.T) {
	rh, master, root, b, c := rootStarted(t)
	marker, _ := rh.marker()
	if marker.Generation != 1 || master == b || master == c {
		t.Fatalf("marker = %+v", marker)
	}
	snapshot := rh.snapshot(master)
	if snapshot.Trajectory.Status != types.TrajectoryLive {
		t.Fatalf("master trajectory is %s", snapshot.Trajectory.Status)
	}
	if open := openWorkFor(snapshot); len(open) != 1 || open[0].AssignedAgentID != "texture:"+master {
		t.Fatalf("master open work = %+v, want one standing item for texture:%s", open, master)
	}
	if snapshot.HeadRevision.AuthorKind != types.AuthorUser {
		t.Fatalf("master v0 author = %s, want the create path's owner-authored seed", snapshot.HeadRevision.AuthorKind)
	}
	if root.AgentID != "texture:"+master || root.ChannelID != master {
		t.Fatalf("root run agent=%s channel=%s, want texture:%s on its own document", root.AgentID, root.ChannelID, master)
	}
	if !strings.Contains(root.Prompt, textureprompts.RootSection()) {
		t.Fatalf("root run lacks the root section:\n%s", root.Prompt)
	}
	for _, doc := range []string{b, c} {
		if want := rh.refsLine(doc); !strings.Contains(root.Prompt, want) {
			t.Fatalf("root run lacks %q:\n%s", want, root.Prompt)
		}
	}
	if strings.Contains(root.Prompt, "refs: doc="+master) {
		t.Fatal("the root's ledger lists the master's own standing item")
	}
	if root.Metadata["texture_root"] != true {
		t.Fatalf("root run metadata = %+v", root.Metadata)
	}
	// The other documents' runs are untouched: no root section.
	if run, ok := rh.activeRun(b); ok && strings.Contains(run.Prompt, textureprompts.RootSection()) {
		t.Fatal("a document's own Texture got the root section")
	}
}

// The root's own commits mint no digest, and its turns leave the standing
// item open even when it calls Done.
func TestTextureRootOwnCommitMintsNoDigestAndKeepsTheStandingItem(t *testing.T) {
	rh, master, root, _, _ := rootStarted(t)
	before := len(rh.digests())
	rh.write(rh.h, &root, "master", "Harbor: open. Bridge: closed.", "rendered")
	if after := len(rh.digests()); after != before {
		t.Fatalf("the root's own commit minted %d digests", after-before)
	}
	snapshot := rh.snapshot(master)
	if open := openWorkFor(snapshot); len(open) != 1 || snapshot.Trajectory.Status != types.TrajectoryLive {
		t.Fatalf("after a root turn with Done: open=%+v status=%s, want the standing item open", open, snapshot.Trajectory.Status)
	}
	// Owner input on the master is ordinary owner input: it wakes the root,
	// not a digest.
	head := snapshot.HeadRevision.RevisionID
	if resp := postOwnerInstruction(t, rh.h, "/api/texture/documents/"+master+"/revise", rootOwner, "owner-master-1", "Group by desk.", head); resp.Code != http.StatusAccepted {
		t.Fatalf("owner revise on the master: %d %s", resp.Code, resp.Body.String())
	}
	if after := len(rh.digests()); after != before {
		t.Fatalf("owner input on the master minted %d digests", after-before)
	}
}

// Failure mode 3a: a commit while the root run is live is not lost. The
// digest delivered during the run starts no second run and re-arms exactly
// one digest; later commits coalesce into it; after the run, it names both.
func TestTextureRootCommitDuringALiveRunReArmsExactlyOneDigest(t *testing.T) {
	rh, master, root, b, c := rootStarted(t)
	rh.advance(3 * time.Minute)
	before := len(rh.digests())
	rh.h.NoteTextureRootChange(t.Context(), rootOwner, rootComputer, b)
	rh.h.NoteTextureRootChange(t.Context(), rootOwner, rootComputer, b)
	if n := len(rh.digests()) - before; n != 1 {
		t.Fatalf("commits during a live root run armed %d digests, want 1", n)
	}
	rh.advance(3 * time.Minute)
	rh.deliver(rh.digests()[len(rh.digests())-1])
	if still, ok := rh.activeRun(master); !ok || still.RunID != root.RunID {
		t.Fatalf("a digest during a live root run replaced it: %+v %v", still, ok)
	}
	if n := len(rh.digests()) - before; n != 2 {
		t.Fatalf("the digest during a live run re-armed %d digests, want exactly 1", n-1)
	}
	rh.h.NoteTextureRootChange(t.Context(), rootOwner, rootComputer, c)
	if n := len(rh.digests()) - before; n != 2 {
		t.Fatalf("a commit while re-armed armed another digest (%d)", n-1)
	}

	// The root run ends; the re-armed digest names both documents.
	passivateRun(t, rh.s, root)
	rh.advance(3 * time.Hour) // the gate: a run that moved no watermark waits twice the hour
	rh.deliver(rh.digests()[len(rh.digests())-1])
	next, ok := rh.activeRun(master)
	if !ok || next.RunID == root.RunID {
		t.Fatalf("the re-armed digest started no new root run: %+v %v", next, ok)
	}
	for _, doc := range []string{b, c} {
		if !strings.Contains(next.Prompt, "refs: doc="+doc) {
			t.Fatalf("the re-armed digest's run lacks %s:\n%s", doc, next.Prompt)
		}
	}
}

func passivateRun(t *testing.T, s *store.Store, run types.RunRecord) {
	t.Helper()
	stored, err := s.GetLifecycleRun(t.Context(), run.OwnerID, run.ComputerID, run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	stored.State, stored.UpdatedAt, stored.FinishedAt = types.RunPassivated, time.Now().UTC(), nil
	req := types.ReplaceLifecycleActivationRequest{OwnerID: run.OwnerID, ComputerID: run.ComputerID, CommandID: "passivate-" + run.RunID, TrajectoryID: run.TrajectoryID, AgentID: run.AgentID, Run: stored}
	req.CommandDigest, _ = store.ComputeReplaceLifecycleActivationDigest(req)
	if _, err := s.ReplaceLifecycleActivation(t.Context(), req); err != nil {
		t.Fatalf("passivate %s: %v", run.RunID, err)
	}
}

// Failure mode 3c: a settled master is replaced at the next digest, before
// any run activation; the marker's generation advances and names its
// predecessor, and the new master's first revision links it.
func TestTextureRootSettledMasterIsReplaced(t *testing.T) {
	rh, master, root, b, _ := rootStarted(t)
	snapshot := rh.snapshot(master)
	standing := openWorkFor(snapshot)[0]
	settle := types.SettleLifecycleWorkRequest{
		OwnerID: rootOwner, ComputerID: rootComputer, CommandID: "settle-standing",
		TrajectoryID: snapshot.Trajectory.TrajectoryID, WorkItemID: standing.WorkItemID,
		ActingAgentID: "texture:" + master, ResultRef: snapshot.HeadRevision.RevisionID,
	}
	settle.CommandDigest, _ = store.ComputeSettleLifecycleWorkDigest(settle)
	if _, err := rh.s.SettleLifecycleWork(t.Context(), settle); err != nil {
		t.Fatalf("complete the standing item: %v", err)
	}
	passivateRun(t, rh.s, root)
	snapshot = rh.snapshot(master)
	settleTrajectory := types.SettleLifecycleTrajectoryRequest{
		OwnerID: rootOwner, ComputerID: rootComputer, CommandID: "settle-master",
		TrajectoryID: snapshot.Trajectory.TrajectoryID, ExpectedLifecycleVersion: snapshot.Trajectory.LifecycleVersion,
		ExpectedHeadRevisionID: snapshot.HeadRevision.RevisionID,
	}
	settleTrajectory.CommandDigest, _ = store.ComputeSettleLifecycleTrajectoryDigest(settleTrajectory)
	if _, err := rh.s.SettleLifecycleTrajectory(t.Context(), settleTrajectory); err != nil {
		t.Fatalf("settle the master: %v", err)
	}
	if rh.snapshot(master).Trajectory.Status != types.TrajectorySettled {
		t.Fatal("the master did not settle")
	}

	rh.advance(3 * time.Minute)
	rh.h.NoteTextureRootChange(t.Context(), rootOwner, rootComputer, b)
	rh.advance(3 * time.Minute)
	rh.deliver(rh.digests()[len(rh.digests())-1])
	marker, ok := rh.marker()
	if !ok || marker.Generation != 2 || marker.DocumentID == master || marker.PredecessorDocumentID != master {
		t.Fatalf("marker after a settled master = %+v, want generation 2 replacing %s", marker, master)
	}
	replacement := rh.snapshot(marker.DocumentID)
	if replacement.Trajectory.Status != types.TrajectoryLive || len(openWorkFor(replacement)) != 1 {
		t.Fatalf("replacement master: status=%s open=%d", replacement.Trajectory.Status, len(openWorkFor(replacement)))
	}
	if !strings.Contains(replacement.HeadRevision.Content+string(replacement.HeadRevision.BodyDoc), master) {
		t.Fatal("the replacement's first revision does not link its predecessor")
	}
	if run, ok := rh.activeRun(marker.DocumentID); !ok || !strings.Contains(run.Prompt, textureprompts.RootSection()) {
		t.Fatalf("the replacement started no root run: %+v %v", run, ok)
	}
	if _, ok := rh.activeRun(master); ok {
		t.Fatal("the settled master was activated")
	}
	// Review 247 item 10: the replacement's full refresh does not list the
	// dead masters it replaces as documents.
	if run, _ := rh.activeRun(marker.DocumentID); strings.Contains(run.Prompt, "refs: doc="+master) {
		t.Fatalf("the replacement's ledger lists its predecessor master:\n%s", run.Prompt)
	}
}

// Failure mode 3b: concurrent ensure steps yield one marker and one master.
func TestTextureRootEnsureRaceYieldsOneMaster(t *testing.T) {
	rh := newRootHarness(t)
	var wg sync.WaitGroup
	got := make([]string, 8)
	errs := make([]error, 8)
	for i := range got {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			root, _, err := rh.h.ensureTextureRoot(t.Context(), rootOwner, rootComputer)
			got[i], errs[i] = root.DocumentID, err
		}(i)
	}
	wg.Wait()
	marker, ok := rh.marker()
	if !ok || marker.Generation != 1 {
		t.Fatalf("marker after the race = %+v %v", marker, ok)
	}
	for i := range got {
		if errs[i] == nil && got[i] != marker.DocumentID {
			t.Fatalf("ensure %d returned %s, the marker names %s", i, got[i], marker.DocumentID)
		}
	}
	if n := rh.countDocsTitled(textureRootTitle); n != 1 {
		t.Fatalf("the race created %d master documents", n)
	}
	// Idempotent on liveness: a live master is kept.
	again, _, err := rh.h.ensureTextureRoot(t.Context(), rootOwner, rootComputer)
	if err != nil || again.DocumentID != marker.DocumentID || again.Generation != 1 {
		t.Fatalf("ensure on a live master = %+v, %v", again, err)
	}
}

// Failure mode 7, the Texture half: boot runs nothing, and after a crash
// boot (a new process: no armed digest in memory, the pre-boot digest
// interrupted by the actor runtime) the next commit mints a new digest.
func TestTextureRootNextCommitAfterBootMintsANewDigest(t *testing.T) {
	rh := newRootHarness(t)
	b, runB := rh.createDoc("boot-b", "Harbor")
	_, runC := rh.createDoc("boot-c", "Bridge")
	rh.write(rh.h, &runB, "harbor", "The harbor is open.", "")
	if n := len(rh.digests()); n != 1 {
		t.Fatalf("pre-boot commit minted %d digests", n)
	}
	booted := NewHandler(rh.core)
	rh.wire(booted)
	if err := booted.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	if n := len(rh.digests()); n != 1 {
		t.Fatalf("boot minted %d digests", n-1)
	}
	if _, ok := rh.marker(); ok {
		t.Fatal("boot ran the ensure step")
	}
	rh.advance(time.Second) // the digest's due time is the clock's; a real boot is later
	rh.write(booted, &runC, "bridge", "The bridge is closed.", "")
	digests := rh.digests()
	if len(digests) != 2 || digests[1].content == digests[0].content {
		t.Fatalf("the first commit after boot minted %d digests (%+v), want one new digest", len(digests)-1, digests)
	}
	_ = b
}

// Failure mode 3 and the read bound: the ledger names at most 50 documents
// and 32 KiB, each document with its refs line.
func TestTextureRootLedgerIsBounded(t *testing.T) {
	entries := make([]textureRootLedgerEntry, 0, 60)
	for i := 0; i < 60; i++ {
		docID := fmt.Sprintf("doc-%02d", i)
		snapshot := types.LifecycleSnapshot{
			Trajectory: types.TrajectoryRecord{Status: types.TrajectoryLive},
			WorkItems:  []types.WorkItemRecord{{WorkItemID: fmt.Sprintf("%08x-0000-0000-0000-000000000000", i), Status: types.WorkItemOpen, AuthorityProfile: "research", AssignedAgentID: "research:" + docID, Objective: "find the ferry times"}},
		}
		entries = append(entries, textureRootLedgerEntry{Doc: types.Document{DocID: docID, Title: "Doc " + docID}, Snapshot: snapshot, HasSnapshot: true})
	}
	small := textureRootLedger(entries)
	if n := strings.Count(small, "refs: doc="); n != textureRootMaxDocuments || textureRootMaxDocuments != 50 {
		t.Fatalf("ledger named %d documents, want the bound of 50", n)
	}
	if !strings.Contains(small, "refs: doc=doc-00 work=00000000") {
		t.Fatalf("ledger lacks a refs line:\n%s", small)
	}
	for i := range entries {
		entries[i].Snapshot.WorkItems[0].Objective = strings.Repeat("long objective ", 200)
	}
	large := textureRootLedger(entries)
	if len(large) > textureRootMaxSummaryBytes || textureRootMaxSummaryBytes != 32<<10 {
		t.Fatalf("ledger is %d bytes, over the 32 KiB bound", len(large))
	}
	if !strings.Contains(large, "refs: doc=doc-00") {
		t.Fatal("a bounded ledger dropped every refs line")
	}
}

func TestTextureRootShortIDIsEightHex(t *testing.T) {
	if got := textureRootShortID("6711a266-8f9d-5ab2-b752-cf8145c6990d"); got != "6711a266" {
		t.Fatalf("short id of a uuid = %q", got)
	}
	got := textureRootShortID("work-hold-x")
	if len(got) != 8 || strings.Trim(got, "0123456789abcdef") != "" {
		t.Fatalf("short id of a non-hex id = %q, want 8 hex", got)
	}
}

// Review 247 (tier 2) on f233a0dba: each test below was written before its
// fix.

// B1: a digest whose reconcile starts no run (here the master's activation
// breaker is tripped) ends: it keeps its documents pending and arms nothing,
// so a tripped breaker is no poll. It started no run, so it made no model
// call. The next commit arms the next digest, which carries the earlier
// documents too.
func TestTextureRootNoRunDigestEndsAndKeepsItsDocuments(t *testing.T) {
	rh, master, root, _, _ := rootStarted(t)
	rh.write(rh.h, &root, "master", "Harbor: open. Bridge: closed.", "")
	passivateRun(t, rh.s, root)
	head := rh.snapshot(master).HeadRevision.RevisionID
	future := time.Now().UTC().Add(time.Hour)
	for i := 0; i < 3; i++ {
		if err := rh.s.CreateAgentMutation(t.Context(), store.AgentMutation{
			DocID: master, RunID: fmt.Sprintf("dead-root-%d", i), OwnerID: rootOwner, ComputerID: rootComputer,
			State: "stale_activation", RevisionID: head, CreatedAt: future.Add(time.Duration(i) * time.Second),
		}); err != nil {
			t.Fatal(err)
		}
	}
	// A document that changed after the root's commit (its watermark).
	d, _ := rh.createDoc("noop-d", "Ferry")
	rh.advance(3 * time.Minute)
	rh.h.NoteTextureRootChange(t.Context(), rootOwner, rootComputer, d)
	digests := rh.digests()
	runs := rh.rootRuns(master)
	rh.deliver(digests[len(digests)-1])
	if run, ok := rh.activeRun(master); ok {
		t.Fatalf("a tripped breaker started root run %s", run.RunID)
	}
	// It re-arms once, for when a run may start (the breaker is tripped, so
	// no model call was made).
	if n := len(rh.digests()) - len(digests); n != 1 {
		t.Fatalf("a digest that started no run armed %d more digests, want exactly 1", n)
	}
	if !rh.pendingHas(d) {
		t.Fatalf("the digest that started no run moved the watermark past %s: pending %v", d, rh.pendingDocs())
	}
	if got := rh.rootRuns(master); got != runs {
		t.Fatalf("root runs went %d to %d", runs, got)
	}
	rh.h.NoteTextureRootChange(t.Context(), rootOwner, rootComputer, d)
	if n := len(rh.digests()) - len(digests); n != 1 {
		t.Fatalf("the next commit armed %d digests in all, want the one re-armed (it coalesces)", n)
	}
	if !rh.pendingHas(d) {
		t.Fatalf("the next digest's pending set is %v, want %s", rh.pendingDocs(), d)
	}
}

// Item 2: a wake on the master never reactivates a parked root run (a
// parked run would replay its memory without the ledger); it starts a fresh
// run with the root section and a full refresh. Owner input moves the head
// past the parked run's revision, so it cannot reactivate; the reachable
// case is a wake with the head unchanged (open-work pressure, a record).
func TestTextureRootWakeOnTheMasterStartsAFreshRun(t *testing.T) {
	rh, master, root, b, c := rootStarted(t)
	doc, err := rh.s.GetLifecycleDocument(t.Context(), rootOwner, rootComputer, master)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rh.h.applyTextureLifecycleTurn(t.Context(), &root, doc, textureTurnArgs{ToolCallID: "root-wait", WorkDisposition: types.WorkItemOpen}, types.TextureTurnWait, types.Revision{}, store.TextureSourceGraphWriteSet{}, "the root waits"); err != nil {
		t.Fatalf("root wait turn: %v", err)
	}
	if err := rh.s.SleepAgentMutationAfterTextureTurn(t.Context(), rootOwner, rootComputer, doc.TrajectoryID, root.RunID); err != nil {
		t.Fatalf("sleep the root: %v", err)
	}
	passivateRun(t, rh.s, root)
	if _, err := rh.h.ReconcileAgentWake(t.Context(), rootOwner, master); err != nil {
		t.Fatalf("wake the master: %v", err)
	}
	next, ok := rh.activeRun(master)
	if !ok || next.RunID == root.RunID {
		t.Fatalf("a wake on the master reactivated the parked root run %s (active %s %v)", root.RunID, next.RunID, ok)
	}
	if !strings.Contains(next.Prompt, textureprompts.RootSection()) || !strings.Contains(next.Prompt, "a full refresh") {
		t.Fatalf("the fresh root run lacks the root section or the full refresh:\n%s", next.Prompt)
	}
	for _, docID := range []string{b, c} {
		if !strings.Contains(next.Prompt, "refs: doc="+docID) {
			t.Fatalf("the fresh root run's full refresh lacks %s", docID)
		}
	}
}

// Item 3: ledger changes the actor runtime sees by trajectory (work
// assigned, a cancellation, a delegated engineering spawn) note the document
// for the root; the master's own trajectory notes nothing.
func TestTextureRootTrajectoryChangeNotesItsDocument(t *testing.T) {
	rh, master, _, b, _ := rootStarted(t)
	rh.advance(3 * time.Minute)
	before := len(rh.digests())
	rh.h.NoteTextureRootTrajectoryChange(t.Context(), rootOwner, rootComputer, rh.snapshot(master).Trajectory.TrajectoryID)
	if n := len(rh.digests()) - before; n != 0 {
		t.Fatalf("a change on the master's own trajectory armed %d digests", n)
	}
	rh.h.NoteTextureRootTrajectoryChange(t.Context(), rootOwner, rootComputer, rh.snapshot(b).Trajectory.TrajectoryID)
	if digests := rh.digests(); len(digests)-before != 1 || !rh.pendingHas(b) {
		t.Fatalf("a change on %s's trajectory armed %d digests, pending %v, want one digest and the document pending", b, len(digests)-before, rh.pendingDocs())
	}
}

// Item 4: the digest is scheduled outside the scope mutex (the actor-log
// write does not serialize every commit's note), arming stays exactly-once,
// and a failed schedule rolls the arm back so the next note schedules.
func TestTextureRootScheduleRunsOutsideTheLockAndRollsBackOnFailure(t *testing.T) {
	rh := newRootHarness(t)
	var calls int
	heldLock, fail := false, true
	rh.core.SetScheduleActor(func(_ context.Context, _, _, to, kind, content, _, _ string, notBefore time.Time) error {
		if rh.h.textureRootMu.TryLock() {
			rh.h.textureRootMu.Unlock()
		} else {
			heldLock = true
		}
		calls++
		if fail {
			fail = false
			return fmt.Errorf("actor log unavailable")
		}
		return nil
	})
	rh.h.NoteTextureRootChange(t.Context(), rootOwner, rootComputer, "doc-b")
	rh.h.NoteTextureRootChange(t.Context(), rootOwner, rootComputer, "doc-c")
	rh.h.NoteTextureRootChange(t.Context(), rootOwner, rootComputer, "doc-d")
	if heldLock {
		t.Fatal("the digest was scheduled while holding the root scope mutex")
	}
	if calls != 2 {
		t.Fatalf("schedule calls = %d, want 2 (a failed schedule, then one armed digest the third note coalesces into)", calls)
	}
}

// Item 6: once a document's head is Texture's, the ledger still says what
// the owner last asked, from the revision chain.
func TestTextureRootLedgerKeepsTheOwnerAskPastAnAgentHead(t *testing.T) {
	_, _, root, b, _ := rootStarted(t)
	_ = b
	if !strings.Contains(root.Prompt, "the owner last asked: Write about Harbor.") {
		t.Fatalf("the ledger lost the owner's ask once Texture wrote the head:\n%s", root.Prompt)
	}
}

// Item 7: the master is bounded: a root revision over 48 KiB is refused with
// a reason the root sees; another document's is not.
func TestTextureRootRefusesAMasterRevisionOver48KiB(t *testing.T) {
	rh, _, root, b, _ := rootStarted(t)
	big := strings.Repeat("The harbor is open and the ferry runs. ", 50<<10/39+1)
	base := headOf(rh.t, rh.s, root)
	turn := turnBody(t, yaegikernel.TextureTurn{BaseRevisionID: base.RevisionID, Write: &yaegikernel.TextureWrite{Edits: "@all\n# master\n\n" + big, Why: "too long"}})
	_, err := rh.h.CommitCellTextureAuthor(t.Context(), &root, turn, "master-big")
	if err == nil || !strings.Contains(err.Error(), "48 KiB") {
		t.Fatalf("a 50 KiB master revision: err=%v, want a refusal naming 48 KiB", err)
	}
	if !strings.Contains(textureprompts.RootSection(), "48 KiB") {
		t.Fatal("the root section does not tell the root its bound")
	}
	runB, ok := rh.activeRun(b)
	if !ok {
		t.Fatal("no run on b")
	}
	rh.write(rh.h, &runB, "harbor-big", big, "")
}

// Item 8: the master's client request ids are reserved.
func TestTextureRootReservesItsClientRequestIDs(t *testing.T) {
	rh := newRootHarness(t)
	body := `{"client_request_id":"texture-root:1","title":"Mine","initial_content":"Hello."}`
	w := runtimeHandlerRequest(t, rh.h.HandleTextureLifecycleCreate, http.MethodPost, "/api/texture/lifecycle/create", body, rootOwner)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("create with a texture-root: request id = %d %s, want 400", w.Code, w.Body.String())
	}
}

// Item 9: a note after a committed change survives its caller's cancelled
// context.
func TestTextureRootNoteSurvivesACancelledContext(t *testing.T) {
	rh := newRootHarness(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	rh.h.NoteTextureRootChange(ctx, rootOwner, rootComputer, "doc-b")
	if n := len(rh.digests()); n != 1 {
		t.Fatalf("a note with a cancelled context armed %d digests, want 1", n)
	}
}

// Item 11 (DeepSeek #9): a document started after the marker is ordinary:
// no root section, and its Done completes its own work item (no clamp).
func TestTextureRootDocumentStartedAfterTheMarkerIsOrdinary(t *testing.T) {
	rh, _, _, _, _ := rootStarted(t)
	d, runD := rh.createDoc("after-d", "Ferry")
	if strings.Contains(runD.Prompt, textureprompts.RootSection()) || runD.Metadata["texture_root"] == true {
		t.Fatalf("a document started after the marker got the root section:\n%s", runD.Prompt)
	}
	rh.write(rh.h, &runD, "ferry", "The ferry runs hourly.", "answered")
	if open := openWorkFor(rh.snapshot(d)); len(open) != 0 {
		t.Fatalf("Done on an ordinary document left its work open (the root clamp applied): %+v", open)
	}
}

// MA3.7b counter-zero: a root digest round (documents written, a digest
// minted, the root ensured and started) reads no kind through a reader that is
// not its custodian or admitted, and reads nothing undeclared. The settings
// the root reads to configure itself are read as the shell, and the rows it
// reads are Texture's own.
func TestTextureRootRoundCrossesNoReader(t *testing.T) {
	objectgraph.ResetReaderGateCounts()
	objectgraph.ResetUndeclaredReads()
	rootStarted(t)
	for key, n := range objectgraph.ReaderGateCounts() {
		// An undeclared read counted here may be the fixture's own (it seeds
		// through the store); the oracle below sees the product's call sites.
		if strings.Contains(key, "|undeclared|") {
			continue
		}
		t.Errorf("reader gate counted during a root digest round: %s x%d", key, n)
	}
	for key, n := range objectgraph.UndeclaredReads() {
		t.Errorf("undeclared read during a root digest round: %s x%d", key, n)
	}
}

// MA3.7b: the settings the host reads to configure Texture's run are read as
// the settings' custodian, so a Texture-declared context crosses nothing. A
// content-item ref Texture resolves into a source card stays Texture's read
// and is counted: the ref comes from a desk's record and the card's text
// reaches Texture's prompt, so declaring the custodian would launder it.
func TestTextureHostReadsOfSettingsCrossNothingAndContentItemsAreCounted(t *testing.T) {
	rh := newRootHarness(t)
	ctx := objectgraph.WithReader(t.Context(), objectgraph.WriterTexture)
	objectgraph.ResetReaderGateCounts()
	if !rh.h.textureSettingOn(ctx, rootOwner, rootComputer, store.SettingTextureOwnerDirections) {
		t.Fatal("the owner directions setting reads off")
	}
	for key, n := range objectgraph.ReaderGateCounts() {
		t.Errorf("reader gate counted a settings read: %s x%d", key, n)
	}
	_ = sourceEntityFromEvidenceRef(ctx, rh.h, rootOwner, "content_item:no-such-item")
	// Enforce flip D3: Texture is a supplemental reader of content_item, so the
	// ref read is declared, not a crossing.
	for key, n := range objectgraph.ReaderGateCounts() {
		t.Errorf("reader gate counted: %s x%d", key, n)
	}
}

// MA3.7c (plan 8.1 rule (b)): an evidence ref carried in a record the
// evidence's own custodian (research) stamped is carried data, read as that
// custodian and not counted; the same ref carried by any other desk's record,
// or by none, stays Texture's read and is counted.
func TestEvidenceRefCarriedByItsCustodianIsUncountedAndOtherwiseCounted(t *testing.T) {
	rh := newRootHarness(t)
	base := objectgraph.WithReader(t.Context(), objectgraph.WriterTexture)
	if err := rh.s.CreateEvidence(objectgraph.WithWriter(t.Context(), objectgraph.WriterResearch), types.EvidenceRecord{
		EvidenceID: "ev-ma37c", OwnerID: rootOwner, AgentID: "research:doc-1", Kind: "web_page",
		SourceURI: "https://example.com/a", Title: "Harbor", Content: "the harbor reopened", CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
	const crossing = "choir.agent_evidence|desk:texture|wrong_reader"

	for _, carrier := range []string{"research:doc-1", "research"} {
		objectgraph.ResetReaderGateCounts()
		entity := sourceEntityFromEvidenceRef(withCarrierDesk(base, carrier), rh.h, rootOwner, "evidence:ev-ma37c")
		if entity.EntityID == "" {
			t.Fatalf("carrier %q: the carried evidence did not resolve", carrier)
		}
		if counts := objectgraph.ReaderGateCounts(); counts[crossing] != 0 {
			t.Errorf("carrier %q: carried evidence was counted: %v", carrier, counts)
		}
	}
	for _, carrier := range []string{"management:x", "engineering:y", "texture:doc", "not-a-desk:z", ""} {
		objectgraph.ResetReaderGateCounts()
		ctx := base
		if carrier != "" {
			ctx = withCarrierDesk(base, carrier)
		}
		_ = sourceEntityFromEvidenceRef(ctx, rh.h, rootOwner, "evidence_id:ev-ma37c")
		// Enforce flip D3: Texture is a supplemental reader of agent_evidence,
		// whichever record carried the ref.
		if counts := objectgraph.ReaderGateCounts(); counts[crossing] != 0 {
			t.Errorf("carrier %q: an evidence ref Texture may read was counted as %s: %v", carrier, crossing, counts)
		}
	}
}

// The carrier is the record's reducer-stamped provenance desk, end to end: a
// pending record stamped by research that carries an evidence ref resolves
// uncounted; the same ref in a record stamped by management is counted.
func TestEvidenceRefsOfPendingRecordsUseTheStampedProvenanceDesk(t *testing.T) {
	rh := newRootHarness(t)
	ctx := objectgraph.WithReader(t.Context(), objectgraph.WriterTexture)
	if err := rh.s.CreateEvidence(objectgraph.WithWriter(t.Context(), objectgraph.WriterResearch), types.EvidenceRecord{
		EvidenceID: "ev-ma37c-2", OwnerID: rootOwner, AgentID: "research:doc-2", Kind: "web_page",
		SourceURI: "https://example.com/b", Title: "Port", Content: "the port opened", CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
	commit := func(id, desk string) {
		rec := types.CommitmentRecord{
			SchemaID: types.CommitmentRecordSchemaV1, RecordID: id, Kind: types.CommitmentKindDirective,
			Directive: &types.CommitmentDirective{Subtype: types.CommitmentDirectiveCast, Objective: "o"},
			Cast:      &types.CommitmentCast{}, Inbox: "texture:doc-2", SenderInbox: desk + ":doc-2", Addressee: "texture:doc-2",
			EmitterKind: types.CommitmentEmitterAgent, Discrepancy: types.DiscrepancyUnresolved,
			EvidenceRefs: []string{"evidence:ev-ma37c-2"},
			Provenance:   types.CommitmentProvenance{AgentID: desk + ":doc-2", Desk: desk + ":doc-2"},
		}
		req := types.CommitInboxRecordsRequest{OwnerID: rootOwner, ComputerID: rootComputer, CommandID: "seed-" + id, Records: []types.CommitmentRecord{rec}}
		req.CommandDigest, _ = store.ComputeCommitInboxRecordsDigest(req)
		if _, err := rh.s.CommitInboxRecords(t.Context(), req); err != nil {
			t.Fatal(err)
		}
	}
	const crossing = "choir.agent_evidence|desk:texture|wrong_reader"
	commit("rec-by-research", "research")
	objectgraph.ResetReaderGateCounts()
	entities, _ := rh.h.evidenceSourceEntitiesFromCommitmentRecords(ctx, rootOwner, rootComputer, "texture:doc-2", 20)
	if len(entities) == 0 {
		t.Fatal("the research-carried evidence did not resolve")
	}
	if n := objectgraph.ReaderGateCounts()[crossing]; n != 0 {
		t.Errorf("research-stamped carrier was counted x%d", n)
	}
	commit("rec-by-management", "management")
	objectgraph.ResetReaderGateCounts()
	_, _ = rh.h.evidenceSourceEntitiesFromCommitmentRecords(ctx, rootOwner, rootComputer, "texture:doc-2", 20)
	// Both records carry the ref: Texture is admitted to agent_evidence, so
	// the management-stamped one is not a crossing either (enforce flip D3).
	if n := objectgraph.ReaderGateCounts()[crossing]; n != 0 {
		t.Errorf("a management-stamped carrier was counted: %v", objectgraph.ReaderGateCounts())
	}
}

// openStory opens a living story on the harness's computer the way management
// does (a Cast to texture:new): its trajectory is of kind story.
func (rh *rootHarness) openStory(suffix string) string {
	rh.t.Helper()
	management := agentprofile.Management + ":" + rootOwner
	cast := types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: "cell-open-" + suffix + ":cast:c1", Kind: types.CommitmentKindDirective,
		Directive: &types.CommitmentDirective{Subtype: types.CommitmentDirectiveCast, Objective: "Story " + suffix},
		Cast:      &types.CommitmentCast{Captures: []string{"cap-" + suffix}, Publish: types.CastPublishAuto},
		Inbox:     types.StoryOpenInbox(), SenderInbox: management, Addressee: types.StoryOpenInbox(),
		EmitterKind: types.CommitmentEmitterAgent, Discrepancy: types.DiscrepancyUnresolved,
		Provenance: types.CommitmentProvenance{AgentID: management},
	}
	req := types.CommitInboxRecordsRequest{OwnerID: rootOwner, ComputerID: rootComputer, CommandID: "cell-records:open-" + suffix, Records: []types.CommitmentRecord{cast}}
	req.CommandDigest, _ = store.ComputeCommitInboxRecordsDigest(req)
	receipt, err := rh.s.CommitInboxRecords(rh.t.Context(), req)
	if err != nil || len(receipt.Records) != 1 || !receipt.Records[0].OpenedStory {
		rh.t.Fatalf("open story %s: %+v, %v", suffix, receipt, err)
	}
	return strings.TrimPrefix(receipt.Records[0].Inbox, agentprofile.Texture+":")
}

// One batched digest for every document (story or ordinary): a hundred
// commits over fifty minutes make one digest, due an hour after the first,
// and one root run.
func TestTextureRootAHundredCommitsInAnHourMakeOneRootRun(t *testing.T) {
	rh := newRootHarness(t)
	docs := []string{rh.openStory("s1"), rh.openStory("s2")}
	o, _ := rh.createDoc("batch-o", "Harbor")
	docs = append(docs, o)
	start := rh.clock()
	for i := 0; i < 100; i++ {
		rh.h.NoteTextureRootChange(t.Context(), rootOwner, rootComputer, docs[i%len(docs)])
		rh.advance(30 * time.Second)
	}
	digests := rh.digests()
	if len(digests) != 1 {
		t.Fatalf("100 commits in an hour armed %d digests, want 1", len(digests))
	}
	if want := start.Add(time.Hour); digests[0].notBefore.Before(want) {
		t.Fatalf("the digest is due %s, before an hour from the first commit (%s)", digests[0].notBefore, want)
	}
	if _, ok := rh.marker(); ok {
		t.Fatal("a master was made before the batch was due")
	}
	rh.mu.Lock()
	rh.now = digests[0].notBefore.Add(time.Second)
	rh.mu.Unlock()
	rh.deliver(digests[0])
	marker, ok := rh.marker()
	if !ok {
		t.Fatal("the hourly digest made no master")
	}
	if n := rh.rootRuns(marker.DocumentID); n != 1 {
		t.Fatalf("the batch made %d root runs, want 1", n)
	}
	if n := len(rh.digests()); n != 1 {
		t.Fatalf("delivering the batch armed %d more digests", n-1)
	}
}

// A story commit, then an ordinary commit ten seconds later, then the digest
// an hour on, make exactly ONE root run, and the same digest delivered again,
// or another digest of any content, makes none once the root has committed:
// every digest is idempotent against the store.
func TestTextureRootAStoryThenAnOrdinaryCommitMakeExactlyOneRootRun(t *testing.T) {
	rh := newRootHarness(t)
	story := rh.openStory("s-once")
	rh.h.NoteTextureRootChange(t.Context(), rootOwner, rootComputer, story)
	rh.advance(10 * time.Second)
	b, runB := rh.createDoc("once-b", "Harbor")
	rh.write(rh.h, &runB, "harbor", "The harbor is open.", "")
	digests := rh.digests()
	if len(digests) != 1 {
		t.Fatalf("a story commit and an ordinary commit armed %d digests, want 1", len(digests))
	}
	if rh.pendingHas(story) || !rh.pendingHas(b) {
		t.Fatalf("pending %v, want %s and not the story (the root desk does not digest stories)", rh.pendingDocs(), b)
	}
	rh.mu.Lock()
	rh.now = digests[0].notBefore.Add(time.Second)
	rh.mu.Unlock()
	rh.deliver(digests[0])
	marker, ok := rh.marker()
	if !ok {
		t.Fatal("the digest made no master")
	}
	run, live := rh.activeRun(marker.DocumentID)
	if !live {
		t.Fatal("the digest started no root run")
	}
	// The root commits its revision: the watermark moves with it.
	rh.write(rh.h, &run, "master", "Harbor: open. Story: told.", "")
	passivateRun(t, rh.s, run)
	rh.deliver(digests[0])
	rh.deliver(rootWake{content: `{"v":1,"due":"another"}`})
	if n := rh.rootRuns(marker.DocumentID); n != 1 {
		t.Fatalf("the sequence made %d root runs, want exactly 1", n)
	}
}

// A planned restart resumes a digest of an earlier boot. It carries nothing
// the new process lacks: it reads the store, finds the documents changed
// after the master's watermark (all of them, for a computer with no master
// digest yet), and starts the one run.
func TestTextureRootAResumedDigestRunsOnTheStoreNotTheMessage(t *testing.T) {
	rh := newRootHarness(t)
	b, runB := rh.createDoc("resume-b", "Harbor")
	rh.write(rh.h, &runB, "harbor", "The harbor is open.", "")
	// A new process: nothing armed in memory, the digest row from another boot.
	rh.h.textureRootMu.Lock()
	rh.h.textureRootArmed = nil
	rh.h.textureRootMu.Unlock()
	rh.deliver(rootWake{content: `{"v":1,"due":"2026-01-01T00:00:00Z"}`})
	marker, ok := rh.marker()
	if !ok {
		t.Fatal("the resumed digest made no master")
	}
	run, live := rh.activeRun(marker.DocumentID)
	if !live {
		t.Fatal("the resumed digest started no root run")
	}
	if !strings.Contains(run.Prompt, "refs: doc="+b) {
		t.Fatalf("the resumed digest's run lacks %s:\n%s", b, run.Prompt)
	}
}

// Documents and bytes a story run and an ordinary run carry from the master.
// Measured on a master of 3 sections with a 4 KiB excerpt bound: the ordinary
// run reads the document listing once and carries the excerpt; the story run
// reads no listing and carries nothing.
func TestTextureRootStoryRunsCarryNoExcerptAndOrdinaryRunsDo(t *testing.T) {
	rh, _, root, b, c := rootStarted(t)
	story := rh.openStory("s-ctx")
	filler := strings.Repeat("A status line that takes room in the master. ", 50)
	text := "## Harbor\n\n" + filler + "\n\nrefs: doc=" + b + " work=none\n\n" +
		"## Bridge\n\n" + filler + "\n\nrefs: doc=" + c + " work=none\n\n" +
		"## Story\n\n" + filler + "\n\nrefs: doc=" + story + " work=none"
	rh.write(rh.h, &root, "master", text, "")
	storyRun := rh.freshRun(story, "ctx-story-1")
	if hasRootContext(storyRun) {
		t.Fatalf("a story run carried the master excerpt:\n%s", storyRun.Prompt)
	}
	_, ordinaryRun := rh.createDoc("ctx-ord-d", "Ferry")
	if !hasRootContext(ordinaryRun) {
		t.Fatal("an ordinary run lacks the excerpt")
	}
	excerpt := ordinaryRun.Prompt[strings.Index(ordinaryRun.Prompt, textureprompts.RootContextLabel()):]
	if len(excerpt) > textureRootContextMaxBytes+len(textureprompts.RootContextLabel())+4 {
		t.Fatalf("the excerpt is %d bytes, over the bound", len(excerpt))
	}
	if got := fmt.Sprint(storyRun.Metadata["texture_root_excerpt_bytes"]); got != "0" {
		t.Fatalf("the story run's start record says excerpt bytes %q, want 0", got)
	}
	if got := fmt.Sprint(ordinaryRun.Metadata["texture_root_excerpt_bytes"]); got == "0" || got == "<nil>" {
		t.Fatalf("the ordinary run's start record says excerpt bytes %q, want the excerpt's size", got)
	}
	t.Logf("measured: story run excerpt_bytes=0; ordinary run excerpt_bytes=%d", len(excerpt))
}
