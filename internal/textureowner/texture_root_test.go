package textureowner

// PD3 S1: one Texture root desk per computer, the master document's own
// Texture desk, woken by a debounced texture_root_digest after other
// documents' commits, behind texture.root_desk
// (docs/definitions/choir-pd3-texture-root-desk-2026-10-11.md). Ways it can
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
//	    standing item; off is not inert; the read is unbounded.

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

func newRootHarness(t *testing.T, on bool) *rootHarness {
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
	if on {
		if err := rh.s.PutComputerSetting(t.Context(), store.ComputerSetting{OwnerID: rootOwner, ComputerID: rootComputer, Key: store.SettingTextureRootDesk, Value: true}); err != nil {
			t.Fatal(err)
		}
	}
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
	turn := turnBody(rh.t, yaegikernel.TextureTurn{BaseRevisionID: base.RevisionID, Done: done, Write: &yaegikernel.TextureWrite{Edits: "@all\n# " + identity + "\n\n" + text, Why: "report " + identity}})
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
	rh = newRootHarness(t, true)
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

// Off is inert: no digest is minted, nothing is created, and no run carries
// the root section.
func TestTextureRootOffIsInert(t *testing.T) {
	rh := newRootHarness(t, false)
	b, runB := rh.createDoc("off-b", "Harbor")
	rh.write(rh.h, &runB, "harbor", "The harbor is open.", "")
	rh.h.NoteTextureRootChange(t.Context(), rootOwner, rootComputer, b)
	if n := len(rh.digests()); n != 0 {
		t.Fatalf("switch off minted %d digests", n)
	}
	before := len(mustListDocs(t, rh.s))
	// A digest minted before the switch went off is consumed and creates nothing.
	if err := rh.h.HandleTextureRootDigest(t.Context(), rootOwner, rootComputer, `{"v":1,"nonce":"before-off","docs":["`+b+`"]}`); err != nil {
		t.Fatalf("digest while off: %v", err)
	}
	if _, ok := rh.marker(); ok {
		t.Fatal("switch off claimed a root marker")
	}
	if after := len(mustListDocs(t, rh.s)); after != before {
		t.Fatalf("switch off created %d documents", after-before)
	}
	if strings.Contains(runB.Prompt, textureprompts.RootSection()) {
		t.Fatal("switch off gave a document's Texture the root section")
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
// after the commit and debounced by the runtime configuration's interval.
func TestTextureRootCommitsMintOneDebouncedDigest(t *testing.T) {
	rh := newRootHarness(t, true)
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
	if d.to != agentcore.TextureRootDigestMailbox || !strings.Contains(d.content, b) {
		t.Fatalf("digest = %+v, want the root digest mailbox naming %s", d, b)
	}
	if min := rh.clock().Add(rh.core.TextureRootDigestInterval() - time.Second); d.notBefore.Before(min) {
		t.Fatalf("digest due %s, before the debounce interval (%s from %s)", d.notBefore, rh.core.TextureRootDigestInterval(), rh.clock())
	}
	if rh.core.TextureRootDigestInterval() != 120*time.Second || rh.core.TextureRootDigestPerHour() != 20 {
		t.Fatalf("digest bounds = %s and %d per hour, want the defaults 120s and 20", rh.core.TextureRootDigestInterval(), rh.core.TextureRootDigestPerHour())
	}
	_ = c
}

// An owner revision is a commit on a document too: it mints a digest.
func TestTextureRootOwnerRevisionMintsADigest(t *testing.T) {
	rh := newRootHarness(t, true)
	b, _ := rh.createDoc("owner-b", "Harbor")
	head := rh.snapshot(b).HeadRevision.RevisionID
	if resp := postOwnerInstruction(t, rh.h, "/api/texture/documents/"+b+"/revise", rootOwner, "owner-b-1", "Add the ferry times.", head); resp.Code != http.StatusAccepted {
		t.Fatalf("owner revise: %d %s", resp.Code, resp.Body.String())
	}
	if digests := rh.digests(); len(digests) != 1 || !strings.Contains(digests[0].content, b) {
		t.Fatalf("an owner revision minted %+v, want one digest naming %s", digests, b)
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
	rh.advance(3 * time.Minute)
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
}

// Failure mode 3b: concurrent ensure steps yield one marker and one master.
func TestTextureRootEnsureRaceYieldsOneMaster(t *testing.T) {
	rh := newRootHarness(t, true)
	var wg sync.WaitGroup
	got := make([]string, 8)
	errs := make([]error, 8)
	for i := range got {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			root, err := rh.h.EnsureTextureRoot(t.Context(), rootOwner, rootComputer)
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
	again, err := rh.h.EnsureTextureRoot(t.Context(), rootOwner, rootComputer)
	if err != nil || again.DocumentID != marker.DocumentID || again.Generation != 1 {
		t.Fatalf("ensure on a live master = %+v, %v", again, err)
	}
}

// Failure mode 7, the Texture half: boot runs nothing, and after a crash
// boot (a new process: no armed digest in memory, the pre-boot digest
// interrupted by the actor runtime) the next commit mints a new digest.
func TestTextureRootNextCommitAfterBootMintsANewDigest(t *testing.T) {
	rh := newRootHarness(t, true)
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

// The rate bound: one root run per interval and at most perHour an hour.
func TestTextureRootNextSlotHonorsTheRateBound(t *testing.T) {
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	if got := textureRootNextSlot(now, nil, 2*time.Minute, 20); !got.Equal(now) {
		t.Fatalf("no runs: slot %s, want now", got)
	}
	if got := textureRootNextSlot(now, []time.Time{now.Add(-time.Minute)}, 2*time.Minute, 20); !got.Equal(now.Add(time.Minute)) {
		t.Fatalf("a run a minute ago: slot %s, want a minute from now", got)
	}
	var hour []time.Time
	for i := 0; i < 20; i++ {
		hour = append(hour, now.Add(-50*time.Minute+time.Duration(i)*2*time.Minute))
	}
	if got := textureRootNextSlot(now, hour, 2*time.Minute, 20); got.Before(now.Add(10 * time.Minute)) {
		t.Fatalf("twenty runs in the hour: slot %s, want no earlier than the oldest plus an hour", got)
	}
}
