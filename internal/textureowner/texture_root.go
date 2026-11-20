package textureowner

// The Texture root desk (PD3 S1,
// docs/archive/definitions/choir-pd3-texture-root-desk-2026-10-11.md). The root is
// not a new agent: it is the Texture desk of the computer's master document,
// texture:<master>, created by the ordinary create path and named by the
// texture_root marker. Every write check is the one each Texture already
// has, so the root writes only the master and no other Texture writes it.
//
// One authority: the store (texture_root_plan.go). What the master has
// digested is a watermark the root writes into its own revision when a root
// run commits it: a cut-off taken before the run's first read, and the
// documents it read at a version mark. What is pending is every ordinary
// document (never a story) that changed after it. A texture_root_digest
// occurrence carries nothing but its due time, so it is idempotent: a
// duplicate, a retry after a deferral, a digest resumed after a planned
// restart or one orphaned by an earlier boot recomputes the same answer from
// the store, and an answer of nothing ends it with no run.
//
// Commits on other documents (and desk reports to them) note themselves here,
// after their transaction, only to arm a digest: at most one is armed per
// computer, due an hour later. The digest's dispatch ensures the marked
// master (a fresh master when it is not live) before any run activation.
// Nothing here runs at boot: a crash interrupts a pre-boot digest
// (agentcore.RestartWorkKinds), and loses nothing, because the watermark is
// durable and the next commit arms a new digest. The root desk has no
// setting: it runs on every computer.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/actor"
	"github.com/yusefmosiah/go-choir/internal/agentcore"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/textureprompts"
	"github.com/yusefmosiah/go-choir/internal/types"
)

const (
	// textureRootTitle is the master document's title.
	textureRootTitle = "Master document"
	// textureRootMaxDocuments and textureRootMaxSummaryBytes bound one root
	// run's read: at most 50 documents and 32 KiB of ledger summary.
	textureRootMaxDocuments    = 50
	textureRootMaxSummaryBytes = 32 << 10
	// textureRootStaleArm is how long after its due time an armed digest is
	// believed to be on its way: a digest never delivered (dead-lettered)
	// stops holding back the next one after this.
	textureRootStaleArm = 10 * time.Minute
	// textureRootObjectiveChars and textureRootAskChars cut long text in
	// the ledger.
	textureRootObjectiveChars = 240
	textureRootAskChars       = 400
	// textureRootAskWalk bounds the revisions read per document to find what
	// the owner last asked once the head is Texture's.
	textureRootAskWalk = 16
	// textureRootMaxRevisionBytes bounds the master itself: a root revision
	// over 48 KiB is refused, so a root run's input stays bounded too.
	textureRootMaxRevisionBytes = 48 << 10
	// textureRootBatchInterval is how long the root waits after the first
	// commit on any other document: one batched digest an hour, whatever
	// the document. Nothing reads the master within minutes (its readers
	// are the owner's view, the excerpt on an ordinary run, which is
	// context, and the rebase notice, which needs a live root and not fresh
	// text), and hundreds of story documents commit all day while the master
	// is one document. A digest that meets a live root run waits this long
	// again.
	textureRootBatchInterval = time.Hour
	// textureRootDigestedKey and textureRootDigestedDocsKey are the revision
	// (and run) metadata keys of the watermark: the cut-off captured before
	// the run's first ledger read, and the documents it read (id to the
	// change time and version mark it read).
	textureRootDigestedKey     = "texture_root_digested_through"
	textureRootDigestedDocsKey = "texture_root_digested_docs"
	// textureRootStartedKey is the run metadata key of the time the run was
	// built, read from the schedule's clock (the wall clock, outside tests),
	// which the gate measures the hour from.
	textureRootStartedKey = "texture_root_started_at"
	// textureRootOverlap is how far behind the cut-off a digest looks again:
	// a commit stamps its time before the commit is visible (lifecycle.go
	// takes the time, builds the write set, then writes), so a change
	// stamped just before a cut-off can become visible just after it.
	textureRootOverlap = time.Minute
	// textureRootLookback bounds how far back a digest with no watermark, or
	// a watermark held behind a document it could not read, looks.
	textureRootLookback = 14 * 24 * time.Hour
	// textureRootMaxMarks bounds the document marks a revision carries.
	textureRootMaxMarks = 200
	// textureRootWatermarkWalk bounds the revisions read to find the
	// watermark behind the master's head (the owner may have revised the
	// master since the root did): the head and the 31 behind it. Past that,
	// the master has been edited by the owner 32 times without a root
	// revision, and one digest reads every changed document again.
	textureRootWatermarkWalk = 32
	// textureRootBackoffCap caps the wait after root runs that moved no
	// watermark: 1 h, 2 h, 4 h, up to a day.
	textureRootBackoffCap = 24 * time.Hour
	// textureRootClientRequestPrefix is reserved for the master documents'
	// create commands (the create handler refuses it).
	textureRootClientRequestPrefix = "texture-root:"
)

// textureRootDigest is the digest occurrence's content: a version and its due
// time, which only makes each armed digest a distinct occurrence (the actor
// log names an occurrence by its content). Nothing in it is read as state.
type textureRootDigest struct {
	V   int    `json:"v"`
	Due string `json:"due"`
}

// textureRootWake marks a digest's wake of the master's reconcile: the run
// finds what changed itself, in its own request section.
type textureRootWake struct{}

func (h *Handler) textureRootClock() time.Time {
	if h != nil && h.textureRootNow != nil {
		return h.textureRootNow().UTC()
	}
	return time.Now().UTC()
}

// textureRootFor returns the marker when docID is
// the master document: the one condition that makes a Texture run the root.
func (h *Handler) textureRootFor(ctx context.Context, ownerID, computerID, docID string) (store.TextureRoot, bool) {
	ctx = asTextureReader(ctx)
	ownerID, computerID, docID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID), strings.TrimSpace(docID)
	if h == nil || h.Store == nil || docID == "" {
		return store.TextureRoot{}, false
	}
	root, err := h.Store.GetTextureRoot(ctx, ownerID, computerID)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			log.Printf("texture root: read marker on %s: %v", computerID, err)
		}
		return store.TextureRoot{}, false
	}
	return root, root.DocumentID == docID
}

// textureRootShortID is a work item's short id in the master's refs lines:
// the first 8 hex characters of the id with dashes removed, or, for an id
// that does not start with 8 hex characters, the first 8 hex characters of
// its sha256.
func textureRootShortID(id string) string {
	id = strings.TrimSpace(id)
	compact := strings.ToLower(strings.ReplaceAll(id, "-", ""))
	if len(compact) >= 8 && strings.Trim(compact[:8], "0123456789abcdef") == "" {
		return compact[:8]
	}
	sum := sha256.Sum256([]byte(id))
	return hex.EncodeToString(sum[:4])
}

// NoteTextureRootChange records, after its transaction, that a document
// other than the master changed (a Texture commit, an owner revision, a desk
// report), and arms one digest unless one is already armed. On the
// master itself it does nothing. The change is committed, so a cancelled
// caller context does not drop the note.
func (h *Handler) NoteTextureRootChange(ctx context.Context, ownerID, computerID, docID string) {
	ctx = context.WithoutCancel(ctx)
	ownerID, computerID, docID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID), strings.TrimSpace(docID)
	if h == nil || h.Store == nil || h.Core == nil || ownerID == "" || computerID == "" || docID == "" {
		return
	}
	h.noteTextureRootChangeOn(ctx, ownerID, computerID, docID)
}

// NoteTextureRootTrajectoryChange notes the document a trajectory names
// (its doc_id subject ref): for ledger changes the actor runtime reconciles
// by trajectory, after their commit (work assigned, a cancellation, a
// delegated engineering spawn).
func (h *Handler) NoteTextureRootTrajectoryChange(ctx context.Context, ownerID, computerID, trajectoryID string) {
	ctx = context.WithoutCancel(ctx)
	ownerID, computerID, trajectoryID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID), strings.TrimSpace(trajectoryID)
	if h == nil || h.Store == nil || h.Core == nil || ownerID == "" || computerID == "" || trajectoryID == "" {
		return
	}
	trajectory, err := h.Store.GetLifecycleTrajectory(ctx, ownerID, computerID, trajectoryID)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			log.Printf("texture root: read trajectory %s on %s: %v", trajectoryID, computerID, err)
		}
		return
	}
	if docID := strings.TrimSpace(trajectory.SubjectRefs["doc_id"]); docID != "" {
		h.noteTextureRootChangeOn(ctx, ownerID, computerID, docID)
	}
}

// noteTextureRootChangeOn arms the digest for a change to docID.
func (h *Handler) noteTextureRootChangeOn(ctx context.Context, ownerID, computerID, docID string) {
	ctx = asTextureReader(ctx)
	if root, err := h.Store.GetTextureRoot(ctx, ownerID, computerID); err == nil && root.DocumentID == docID {
		return // the root's own document never wakes the root
	} else if err != nil && !errors.Is(err, store.ErrNotFound) {
		log.Printf("texture root: read marker on %s: %v", computerID, err)
	}
	now := h.textureRootClock()
	h.armTextureRootDigest(ctx, ownerID, computerID, now, now.Add(textureRootBatchInterval))
}

// armTextureRootDigest schedules one digest due at due, unless one is armed.
//
// The armed time is the one process-local value, and it cannot hold anything
// the store does not: a lost or stale value (a restart, a digest that was
// never delivered) costs one late digest, fixed by the next note after
// textureRootStaleArm, and a spare digest costs nothing, because the handler
// recomputes what is pending from the store and ends when nothing is. So a
// note that finds one armed may rely on it, and a note that does not may
// arm another. The write is made outside the dispatcher activation that
// noted the change (actor.WithoutEmissions): a buffered digest dies with a
// handler that defers, fails or loses its epoch fence, which would leave the
// armed time set for a digest that never existed; sent at once, a digest for
// an activation that later rolls back is only a spare one.
func (h *Handler) armTextureRootDigest(ctx context.Context, ownerID, computerID string, now, due time.Time) {
	key := ownerID + "\x00" + computerID
	holdUntil := due.Add(textureRootStaleArm)
	h.textureRootMu.Lock()
	if h.textureRootArmed == nil {
		h.textureRootArmed = make(map[string]time.Time)
	}
	if now.Before(h.textureRootArmed[key]) {
		h.textureRootMu.Unlock()
		return
	}
	h.textureRootArmed[key] = holdUntil
	h.textureRootMu.Unlock()
	content, err := json.Marshal(textureRootDigest{V: 1, Due: due.UTC().Format(time.RFC3339Nano)})
	if err == nil {
		err = h.Core.ScheduleActor(actor.WithoutEmissions(ctx), ownerID, computerID, agentcore.TextureRootDigestMailbox, agentcore.TextureRootDigestKind, string(content), "", "", due)
	}
	if err == nil {
		return
	}
	log.Printf("texture root: schedule digest on %s: %v", computerID, err)
	h.textureRootMu.Lock()
	if h.textureRootArmed[key].Equal(holdUntil) {
		delete(h.textureRootArmed, key)
	}
	h.textureRootMu.Unlock()
}

// disarmTextureRootDigest records that the armed digest has fired, so the
// next note arms the next one.
func (h *Handler) disarmTextureRootDigest(ownerID, computerID string) {
	h.textureRootMu.Lock()
	delete(h.textureRootArmed, ownerID+"\x00"+computerID)
	h.textureRootMu.Unlock()
}

// HandleTextureRootDigest is the digest's dispatch, and it is idempotent: it
// reads nothing from the digest. It ensures the marked master (its liveness,
// checked before any activation), scans the store for the documents changed
// since the master last digested (texture_root_plan.go), and ends when none
// did. When a root run is live it re-arms one digest an hour out and ends
// (the run's commit moves the watermark; what changed after the run's cut-off
// is still pending then). The bound lives in the store as well: no digest
// starts a root run less than an hour after the last digest-started one, and
// after runs that moved no watermark the wait doubles (textureRootGate), so
// duplicate, staggered or resumed digests cannot stack and a failing root is
// not a model call an hour forever. A digest held back, or whose wake started
// no run (the activation breaker is tripped, the standing item is closed),
// re-arms once for when it may run. An error is returned for the dispatcher
// to retry the same digest, which finds the same state.
func (h *Handler) HandleTextureRootDigest(ctx context.Context, ownerID, computerID, content string) error {
	ownerID, computerID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID)
	if h == nil || h.Store == nil || h.Core == nil || ownerID == "" || computerID == "" {
		return fmt.Errorf("texture root digest: incomplete scope")
	}
	var digest textureRootDigest
	if err := json.Unmarshal([]byte(content), &digest); err != nil {
		log.Printf("texture root: malformed digest on %s consumed: %v", computerID, err)
		return nil
	}
	h.disarmTextureRootDigest(ownerID, computerID)
	now := h.textureRootClock()
	root, created, err := h.ensureTextureRoot(ctx, ownerID, computerID)
	if err != nil {
		return fmt.Errorf("texture root: ensure the master document: %w", err)
	}
	scan, err := h.textureRootScan(ctx, ownerID, computerID, root)
	if err != nil {
		return fmt.Errorf("texture root: scan for changed documents: %w", err)
	}
	for _, unread := range scan.Unread {
		log.Printf("texture root: document %s on %s cannot be read and stays pending (the watermark does not pass it): %v", unread.ObjectID, computerID, unread.Err)
	}
	if len(scan.Read) == 0 && len(scan.Unread) == 0 {
		return nil // nothing changed since the master last digested
	}
	if len(scan.Read) == 0 {
		// Only documents that cannot be read: a model call cannot help them.
		h.armTextureRootDigest(ctx, ownerID, computerID, now, now.Add(textureRootBackoffCap))
		return nil
	}
	agentID := currentTextureAgentID(root.DocumentID)
	active, found, err := h.Core.TextureActiveRunByAgent(ctx, ownerID, computerID, agentID)
	if err != nil {
		return fmt.Errorf("texture root: read the root's active run: %w", err)
	}
	if found && h.ValidateActivationAuthority(ctx, ownerID, computerID, agentID, active.RunID) == nil {
		h.armTextureRootDigest(ctx, ownerID, computerID, now, now.Add(textureRootBatchInterval))
		log.Printf("texture root: digest on %s re-armed: root run %s is live", computerID, active.RunID)
		return nil
	}
	until, failures, err := h.textureRootGate(ctx, ownerID, computerID, root)
	if err != nil {
		return fmt.Errorf("texture root: read the last root runs: %w", err)
	}
	if now.Before(until) {
		h.armTextureRootDigest(ctx, ownerID, computerID, now, until)
		log.Printf("texture root: digest on %s held until %s: the last digest-started run began less than %s ago (%d without a watermark move)", computerID, until.Format(time.RFC3339), textureRootBatchInterval<<min(failures, 5), failures)
		return nil
	}
	master, err := h.getTextureDocument(ctx, ownerID, root.DocumentID)
	if err != nil {
		return fmt.Errorf("texture root: load the master document: %w", err)
	}
	rec, err := h.reconcileTextureRootDigest(ctx, master, textureRootWake{})
	if err != nil {
		return fmt.Errorf("texture root: wake the root: %w", err)
	}
	if rec == nil {
		h.armTextureRootDigest(ctx, ownerID, computerID, now, now.Add(textureRootBatchInterval<<min(failures, 5)))
		log.Printf("texture root: digest on %s ended: the master's reconcile started no run", computerID)
		return nil
	}
	if scan.Left > 0 {
		h.armTextureRootDigest(ctx, ownerID, computerID, now, now.Add(textureRootBatchInterval))
	}
	log.Printf("texture root: digest on %s started root run %s: documents=%d left=%d unread=%d created=%t", computerID, rec.RunID, len(scan.Read), scan.Left, len(scan.Unread), created)
	return nil
}

// textureRootGate is the earliest time a digest may start a root run, from
// the store: an hour after the newest digest-started root run began, and
// after consecutive digest-started runs that moved no watermark (those begun
// after the revision that carries the watermark), 1 h doubled for each, up to
// textureRootBackoffCap. It is backoff after real failures, not a cap on
// calls: a run that moves the watermark resets it.
func (h *Handler) textureRootGate(ctx context.Context, ownerID, computerID string, root store.TextureRoot) (time.Time, int, error) {
	agentID := currentTextureAgentID(root.DocumentID)
	runs, err := h.Store.ListRunsByAgent(asTextureReader(ctx), ownerID, agentID, 64)
	if err != nil {
		return time.Time{}, 0, err
	}
	state, err := h.textureRootWatermark(ctx, ownerID, computerID, root)
	if err != nil {
		return time.Time{}, 0, err
	}
	var last time.Time
	failures := 0
	for _, run := range runs {
		if metadataStringValue(run.Metadata, "request_source") != agentcore.TextureRootDigestKind {
			continue
		}
		started := run.CreatedAt
		if at, err := time.Parse(time.RFC3339Nano, metadataStringValue(run.Metadata, textureRootStartedKey)); err == nil {
			started = at // the schedule clock's time: the wall clock's, outside tests
		}
		if started.After(last) {
			last = started
		}
		if run.CreatedAt.After(state.RevisionAt) {
			failures++
		}
	}
	if last.IsZero() {
		return time.Time{}, 0, nil
	}
	wait := textureRootBatchInterval << min(failures, 5)
	if wait > textureRootBackoffCap || wait <= 0 {
		wait = textureRootBackoffCap
	}
	return last.Add(wait), failures, nil
}

func (h *Handler) reconcileTextureRootDigest(ctx context.Context, master types.Document, wake textureRootWake) (*types.RunRecord, error) {
	ownerID, computerID, docID := strings.TrimSpace(master.OwnerID), strings.TrimSpace(master.ComputerID), strings.TrimSpace(master.DocID)
	unlock := h.lockTextureWakeScope(ownerID, computerID, docID)
	defer unlock()
	doc, err := h.getTextureDocument(ctx, ownerID, docID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(doc.ComputerID) != computerID || strings.TrimSpace(doc.TrajectoryID) == "" {
		return nil, fmt.Errorf("texture root: master document lifecycle binding conflict")
	}
	return h.reconcileAgentWakeLockedFor(ctx, doc, currentTextureAgentID(docID), &wake)
}

// ensureTextureRoot claims the marker by compare-and-set before it creates
// the document, so the claim is the atomic step: a lost race creates
// nothing, and the master's first run already sees the marker. The document
// id follows from the generation, so a claim whose document is missing (a
// crash between claim and create) is completed, not replaced. created
// reports that this call made the document. It returns the live master,
// creating it, or replacing a master that is no longer live, first; it is
// idempotent on liveness.
func (h *Handler) ensureTextureRoot(ctx context.Context, ownerID, computerID string) (store.TextureRoot, bool, error) {
	ctx = asTextureReader(ctx)
	ownerID, computerID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID)
	if h == nil || h.Store == nil {
		return store.TextureRoot{}, false, fmt.Errorf("texture root: store unavailable")
	}
	for attempt := 0; attempt < 4; attempt++ {
		current, err := h.Store.GetTextureRoot(ctx, ownerID, computerID)
		var expected *store.TextureRoot
		generation, predecessor := int64(1), ""
		switch {
		case err == nil:
			exists, live, liveErr := h.textureRootLive(ctx, ownerID, computerID, current)
			if liveErr != nil {
				return store.TextureRoot{}, false, liveErr
			}
			if live {
				return current, false, nil
			}
			if !exists {
				created, createErr := h.createTextureRootDocument(ctx, current)
				if createErr != nil {
					return current, created, createErr
				}
				if present, _, liveErr := h.textureRootLive(ctx, ownerID, computerID, current); liveErr != nil || present {
					return current, created, liveErr
				}
				// The create command replayed as done, and the document is
				// not there: replace the master, once, through the claim.
				log.Printf("texture root: master %s of generation %d is missing after its create: replacing it", current.DocumentID, current.Generation)
			}
			expected, generation, predecessor = &current, current.Generation+1, current.DocumentID
		case errors.Is(err, store.ErrNotFound):
		default:
			return store.TextureRoot{}, false, err
		}
		docID := textureRootDocumentID(ownerID, computerID, generation)
		claimed, err := h.Store.ClaimTextureRoot(asTextureCustodian(ctx), expected, store.TextureRoot{
			OwnerID: ownerID, ComputerID: computerID, DocumentID: docID,
			Generation: generation, PredecessorDocumentID: predecessor,
		})
		if errors.Is(err, store.ErrConcurrentStateChange) {
			continue // another ensure step won: re-read and use its master
		}
		if err != nil {
			return store.TextureRoot{}, false, err
		}
		log.Printf("texture root: claimed master %s generation %d on %s (predecessor %q)", docID, generation, computerID, predecessor)
		created, createErr := h.createTextureRootDocument(ctx, claimed)
		return claimed, created, createErr
	}
	return store.TextureRoot{}, false, store.ErrConcurrentStateChange
}

func textureRootClientRequestID(generation int64) string {
	return fmt.Sprintf("%s%d", textureRootClientRequestPrefix, generation)
}

// textureRootDocumentID is generation's master document id: it follows from
// the reserved client request id.
func textureRootDocumentID(ownerID, computerID string, generation int64) string {
	_, _, docID, _, _, _ := textureCreateIdentity(ownerID, computerID, textureRootClientRequestID(generation))
	return docID
}

// textureRootMasterIDs is the current master and every master it replaced:
// none of them is a document the ledger lists.
func textureRootMasterIDs(root store.TextureRoot) map[string]bool {
	ids := map[string]bool{strings.TrimSpace(root.DocumentID): true}
	if p := strings.TrimSpace(root.PredecessorDocumentID); p != "" {
		ids[p] = true
	}
	for generation := int64(1); generation <= root.Generation; generation++ {
		ids[textureRootDocumentID(root.OwnerID, root.ComputerID, generation)] = true
	}
	return ids
}

// createTextureRootDocument creates the marked master through the ordinary
// create path. Its v0 is the owner-authored seed, as for every document, and
// its one initial work item is the standing item that keeps the master's
// trajectory live; a replacement's seed links its predecessor.
func (h *Handler) createTextureRootDocument(ctx context.Context, root store.TextureRoot) (bool, error) {
	seed := "The computer's master document: for every document on this computer, what the owner asked, what is open, which desk holds it and what waits on the owner. Kept current by this document's Texture, the computer's Texture root."
	if root.PredecessorDocumentID != "" {
		seed += " It replaces the master document " + root.PredecessorDocumentID + "."
	}
	created, err := h.startLifecycleTextureDocument(asTextureCustodian(ctx), root.OwnerID, root.ComputerID, textureRootClientRequestID(root.Generation), textureRootTitle, seed)
	if err == nil {
		return !created.Replay, nil
	}
	// A concurrent ensure step creating the same generation's document wins
	// the start command; the document exists either way.
	if exists, _, liveErr := h.textureRootLive(ctx, root.OwnerID, root.ComputerID, root); liveErr == nil && exists {
		return false, nil
	}
	return false, fmt.Errorf("texture root: create the master document: %w", err)
}

// textureRootLive reports whether the marked master exists and can still
// take a root run: its trajectory is live with no cancellation intent, and
// its standing item is open.
func (h *Handler) textureRootLive(ctx context.Context, ownerID, computerID string, root store.TextureRoot) (exists, live bool, err error) {
	ctx = asTextureReader(ctx)
	doc, err := h.Store.GetLifecycleDocument(ctx, ownerID, computerID, root.DocumentID)
	if errors.Is(err, store.ErrNotFound) {
		return false, false, nil
	}
	if err != nil {
		return false, false, err
	}
	if doc.ArchivedAt != nil || strings.TrimSpace(doc.TrajectoryID) == "" {
		return true, false, nil // an archived master is not live: it is replaced
	}
	eligible, err := h.textureLifecycleActivationEligible(ctx, doc)
	if err != nil || !eligible {
		return true, false, err
	}
	snapshot, err := h.Store.GetLifecycleSnapshot(ctx, ownerID, computerID, doc.TrajectoryID)
	if err != nil {
		return true, false, err
	}
	agentID := currentTextureAgentID(doc.DocID)
	for _, work := range snapshot.WorkItems {
		if work.Status == types.WorkItemOpen && work.AssignedAgentID == agentID {
			return true, true, nil
		}
	}
	return true, false, nil
}

// textureRootLedgerEntry is one document the root reads: the document and
// its trajectory's snapshot, read per trajectory.
type textureRootLedgerEntry struct {
	Doc         types.Document
	Snapshot    types.LifecycleSnapshot
	HasSnapshot bool
	// Ask is what the owner last asked, from the revision chain.
	Ask string
}

// textureRootRequestSection is what a root run's request turn adds: the root
// section and the ledger of the documents that changed since the master last
// digested, and the run metadata that records what this run read (the
// watermark keys, written into the revision the run commits). It scans the
// store first, so the cut-off precedes every read. A listing error is an
// error (the run is not made); a document whose ledger read fails is named in
// the ledger as unread and holds the watermark behind it.
func (h *Handler) textureRootRequestSection(ctx context.Context, ownerID string, doc types.Document, root store.TextureRoot) (string, map[string]any, error) {
	ctx = asTextureReader(ctx)
	computerID := strings.TrimSpace(doc.ComputerID)
	scan, err := h.textureRootScan(ctx, ownerID, computerID, root)
	if err != nil {
		return "", nil, err
	}
	if h.textureRootAfterScan != nil {
		h.textureRootAfterScan()
	}
	unread := append([]store.ChangedTextureDocument(nil), scan.Unread...)
	var read []store.ChangedTextureDocument
	entries := make([]textureRootLedgerEntry, 0, len(scan.Read))
	for _, candidate := range scan.Read {
		entry := textureRootLedgerEntry{Doc: candidate.Doc}
		if strings.TrimSpace(candidate.Doc.TrajectoryID) != "" {
			readSnapshot := h.Store.GetLifecycleSnapshot
			if h.textureRootReadSnapshot != nil {
				readSnapshot = h.textureRootReadSnapshot
			}
			snapshot, err := readSnapshot(ctx, ownerID, computerID, candidate.Doc.TrajectoryID)
			if err != nil {
				candidate.Err = fmt.Errorf("%s: snapshot: %w", candidate.Doc.DocID, err)
			} else {
				entry.Snapshot, entry.HasSnapshot = snapshot, true
				if entry.Ask, err = h.textureRootOwnerAsk(ctx, ownerID, computerID, snapshot.HeadRevision); err != nil {
					candidate.Err = fmt.Errorf("%s: owner ask: %w", candidate.Doc.DocID, err)
				}
			}
		}
		if candidate.Err != nil {
			log.Printf("texture root: document %s on %s unread: %v", candidate.Doc.DocID, computerID, candidate.Err)
			unread = append(unread, candidate)
			continue
		}
		read = append(read, candidate)
		entries = append(entries, entry)
	}
	through, docs := scan.Finish(read, unread)
	var b strings.Builder
	b.WriteString("\n\n")
	b.WriteString(textureprompts.RootSection())
	if root.PredecessorDocumentID != "" {
		b.WriteString("\n\nThis master document replaces " + root.PredecessorDocumentID + ", which can no longer be revised; say so once.")
	}
	if scan.State.Through.IsZero() {
		b.WriteString("\n\nThe ledger, the documents that changed (the master has digested none before):\n\n")
	} else {
		b.WriteString("\n\nThe ledger, the documents that changed since the master last digested:\n\n")
	}
	b.WriteString(textureRootLedger(entries))
	if len(entries) == 0 && len(unread) == 0 {
		b.WriteString("(no document changed since the master last digested.)\n")
	}
	if scan.Left > 0 {
		fmt.Fprintf(&b, "(%d more changed documents are left for the next digest; keep their sections as they are.)\n", scan.Left)
		log.Printf("texture root: %d changed documents on %s are left for the next digest", scan.Left, computerID)
	}
	for _, entry := range unread {
		fmt.Fprintf(&b, "(unread: %s could not be read: %v; keep its section as it is.)\n", entry.ObjectID, entry.Err)
	}
	ids := make([]string, 0, len(read))
	for _, entry := range read {
		ids = append(ids, entry.Doc.DocID)
	}
	meta := map[string]any{
		textureRootStartedKey:      h.textureRootClock().Format(time.RFC3339Nano),
		textureRootDigestedKey:     through.UTC().Format(time.RFC3339Nano),
		textureRootDigestedDocsKey: docs,
	}
	if len(ids) > 0 {
		meta["texture_root_documents"] = strings.Join(ids, ",")
	}
	return b.String(), meta, nil
}

// textureRootLedger renders the entries within the read bound: at most 50
// documents and 32 KiB. A section that would cross the byte bound is cut to
// its title and refs line; when even that does not fit, the rest are
// counted, not shown.
func textureRootLedger(entries []textureRootLedgerEntry) string {
	const trailerReserve = 160
	var b strings.Builder
	shown := 0
	for _, entry := range entries {
		if shown >= textureRootMaxDocuments {
			break
		}
		section := textureRootLedgerSection(entry, false)
		if b.Len()+len(section) > textureRootMaxSummaryBytes-trailerReserve {
			section = textureRootLedgerSection(entry, true)
			if b.Len()+len(section) > textureRootMaxSummaryBytes-trailerReserve {
				break
			}
		}
		b.WriteString(section)
		shown++
	}
	if rest := len(entries) - shown; rest > 0 {
		fmt.Fprintf(&b, "(%d more documents are past this run's read bound; keep their sections as they are.)\n", rest)
	}
	return b.String()
}

func textureRootLedgerSection(entry textureRootLedgerEntry, compact bool) string {
	var b strings.Builder
	title := strings.TrimSpace(entry.Doc.Title)
	if title == "" {
		title = "(untitled)"
	}
	b.WriteString("## " + truncatePromptSnippet(title, 160) + "\n")
	var open []types.WorkItemRecord
	if entry.HasSnapshot {
		for _, work := range entry.Snapshot.WorkItems {
			if work.Status == types.WorkItemOpen {
				open = append(open, work)
			}
		}
	}
	if !compact && entry.HasSnapshot {
		snapshot := entry.Snapshot
		b.WriteString("trajectory: " + string(snapshot.Trajectory.Status))
		if head := snapshot.HeadRevision; head.RevisionID != "" {
			b.WriteString("; head revision by " + string(head.AuthorKind))
			if !head.CreatedAt.IsZero() {
				b.WriteString(" at " + head.CreatedAt.UTC().Format(time.RFC3339))
			}
		}
		b.WriteString("\n")
		if ask := entry.Ask; ask != "" {
			b.WriteString("the owner last asked: " + ask + "\n")
		}
		if len(open) > 0 {
			b.WriteString("open work:\n")
			for _, work := range open {
				fmt.Fprintf(&b, "- [%s] %s, held by %s: %s\n", textureRootShortID(work.WorkItemID), strings.TrimSpace(work.AuthorityProfile), strings.TrimSpace(work.AssignedAgentID), truncatePromptSnippet(strings.Join(strings.Fields(work.Objective), " "), textureRootObjectiveChars))
			}
		}
		for _, assignment := range snapshot.EngineeringAssignments {
			if assignment.TerminalAt != nil {
				continue
			}
			fmt.Fprintf(&b, "- engineering assignment [%s]: %s, capsule %s", textureRootShortID(assignment.AssignmentID), assignment.Disposition, assignment.CapsuleDisposition)
			if assignment.PendingProposal != nil {
				b.WriteString(", a proposal is pending")
			}
			b.WriteString("\n")
		}
		pending := 0
		for _, update := range snapshot.Updates {
			if update.Disposition == types.UpdatePending {
				pending++
			}
		}
		if pending > 0 {
			fmt.Fprintf(&b, "reports not yet read by this document's Texture: %d\n", pending)
		}
	}
	b.WriteString(textureRootRefsLine(entry.Doc.DocID, open) + "\n\n")
	return b.String()
}

// textureRootRefsLine names the document and each open work item's short
// id: the line the acceptance proof checks against the ledger.
func textureRootRefsLine(docID string, open []types.WorkItemRecord) string {
	if len(open) == 0 {
		return "refs: doc=" + docID + " work=none"
	}
	ids := make([]string, 0, len(open))
	for _, work := range open {
		ids = append(ids, textureRootShortID(work.WorkItemID))
	}
	return "refs: doc=" + docID + " work=" + strings.Join(ids, ",")
}

// textureRootOwnerAsk is the owner's last direction on a document: the head
// revision's own ask, else the newest owner direction on its revision chain,
// else the first revision the owner wrote. The walk reads at most
// textureRootAskWalk revisions; a read that fails is an error (the document
// is unread), the walk bound is not.
func (h *Handler) textureRootOwnerAsk(ctx context.Context, ownerID, computerID string, head types.Revision) (string, error) {
	ctx = asTextureReader(ctx)
	if ask := textureRootHeadAsk(head); ask != "" {
		return ask, nil
	}
	reads := 0
	var readErr error
	load := func(revisionID string) (types.Revision, error) {
		if reads >= textureRootAskWalk {
			return types.Revision{}, errTextureRootAskWalk
		}
		reads++
		revision, err := h.Store.GetLifecycleRevision(ctx, ownerID, computerID, revisionID)
		if err != nil {
			readErr = err
		}
		return revision, err
	}
	directions, first := collectTextureOwnerStory(head, nil, load, true)
	if readErr != nil {
		return "", readErr
	}
	if n := len(directions); n > 0 {
		return truncatePromptSnippet(strings.Join(strings.Fields(directions[n-1].Text), " "), textureRootAskChars), nil
	}
	if first != nil {
		return truncatePromptSnippet(strings.Join(strings.Fields(first.Content), " "), textureRootAskChars), nil
	}
	return "", nil
}

var errTextureRootAskWalk = errors.New("texture root: owner ask walk bound")

// textureRootHeadAsk is the head revision's own ask: its owner prompt, else
// its seed, else the head itself when it is a first revision the owner wrote.
func textureRootHeadAsk(head types.Revision) string {
	metadata := decodeRevisionMetadata(head.Metadata)
	for _, key := range []string{"owner_prompt", "seed_prompt"} {
		if ask := metadataString(metadata, key); ask != "" {
			return truncatePromptSnippet(strings.Join(strings.Fields(ask), " "), textureRootAskChars)
		}
	}
	if head.AuthorKind == types.AuthorUser && strings.TrimSpace(head.ParentRevisionID) == "" {
		return truncatePromptSnippet(strings.Join(strings.Fields(head.Content), " "), textureRootAskChars)
	}
	return ""
}
