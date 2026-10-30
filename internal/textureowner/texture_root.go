package textureowner

// The Texture root desk (PD3 S1,
// docs/archive/definitions/choir-pd3-texture-root-desk-2026-10-11.md). The root is
// not a new agent: it is the Texture desk of the computer's master document,
// texture:<master>, created by the ordinary create path and named by the
// texture_root marker. Every write check is the one each Texture already
// has, so the root writes only the master and no other Texture writes it.
//
// It is woken by a texture_root_digest occurrence, never on another
// document's wake path: commits on other documents (and desk reports to
// them) note the document here, after their transaction; the notes are
// coalesced into one digest, due a debounce interval later and bounded per
// hour (runtime configuration). The digest's dispatch checks the marked
// master's liveness before any run activation and, when it is not live,
// runs the ensure step, which makes a fresh master and advances the
// marker's generation. Nothing here runs at boot: a crash interrupts a
// pre-boot digest (agentcore.RestartWorkKinds) and drops the notes held in
// memory, and the next commit mints a new digest. All of it is behind
// texture.root_desk, off by default.

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
	// textureRootStaleArm forgets an armed digest that was never delivered
	// (dead-lettered), so a lost digest cannot stop every later one.
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
	// textureRootClientRequestPrefix is reserved for the master documents'
	// create commands (the create handler refuses it).
	textureRootClientRequestPrefix = "texture-root:"
)

// textureRootDigest is the digest occurrence's content. Nonce makes each
// armed digest its own actor row (the mailbox dedupes on content).
type textureRootDigest struct {
	V     int      `json:"v"`
	Nonce string   `json:"nonce"`
	Docs  []string `json:"docs"`
	Full  bool     `json:"full,omitempty"`
}

// textureRootScope is one computer's digest state, in memory only: a
// restart forgets it, which is the crash rule (the next commit mints anew).
type textureRootScope struct {
	pending     []string
	pendingFull bool
	armedNonce  string
	armedDue    time.Time
	// attempts are the digests that reached the master's reconcile, whether
	// or not it started a run: the rate bound counts them, so a reconcile
	// that keeps starting nothing (a tripped breaker) cannot loop.
	attempts []time.Time
	seq      int64
}

// textureRootArm is one digest armed under the scope mutex, scheduled after
// it is released.
type textureRootArm struct {
	nonce, content string
	due            time.Time
}

// textureRootWake carries a digest into the master's reconcile.
type textureRootWake struct {
	Docs []string
	Full bool
}

func (h *Handler) textureRootClock() time.Time {
	if h != nil && h.textureRootNow != nil {
		return h.textureRootNow().UTC()
	}
	return time.Now().UTC()
}

func (h *Handler) textureRootScopeLocked(ownerID, computerID string) *textureRootScope {
	key := ownerID + "\x00" + computerID
	if h.textureRootScopes == nil {
		h.textureRootScopes = make(map[string]*textureRootScope)
	}
	scope := h.textureRootScopes[key]
	if scope == nil {
		scope = &textureRootScope{}
		h.textureRootScopes[key] = scope
	}
	return scope
}

// textureRootFor returns the marker when the root desk is on and docID is
// the master document: the one condition that makes a Texture run the root.
func (h *Handler) textureRootFor(ctx context.Context, ownerID, computerID, docID string) (store.TextureRoot, bool) {
	ownerID, computerID, docID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID), strings.TrimSpace(docID)
	if h == nil || h.Store == nil || docID == "" || !h.textureSettingOn(ctx, ownerID, computerID, store.SettingTextureRootDesk) {
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

// textureRootNextSlot is the earliest time a digest may start a root run:
// interval after the last digest-started run, and no more than perHour runs
// in any hour.
func textureRootNextSlot(now time.Time, starts []time.Time, interval time.Duration, perHour int) time.Time {
	next := now
	if n := len(starts); n > 0 {
		if at := starts[n-1].Add(interval); at.After(next) {
			next = at
		}
	}
	var hour []time.Time
	for _, at := range starts {
		if at.After(now.Add(-time.Hour)) {
			hour = append(hour, at)
		}
	}
	if perHour > 0 && len(hour) >= perHour {
		if at := hour[len(hour)-perHour].Add(time.Hour); at.After(next) {
			next = at
		}
	}
	return next
}

func addTextureRootPending(pending []string, docs ...string) []string {
	for _, docID := range docs {
		docID = strings.TrimSpace(docID)
		if docID == "" {
			continue
		}
		kept := pending[:0]
		for _, existing := range pending {
			if existing != docID {
				kept = append(kept, existing)
			}
		}
		pending = append(kept, docID)
	}
	if len(pending) > textureRootMaxDocuments {
		pending = append([]string(nil), pending[len(pending)-textureRootMaxDocuments:]...)
	}
	return pending
}

// NoteTextureRootChange records, after its transaction, that a document
// other than the master changed (a Texture commit, an owner revision, a desk
// report), and arms one digest unless one is already armed. Off, or on the
// master itself, it does nothing. The change is committed, so a cancelled
// caller context does not drop the note.
func (h *Handler) NoteTextureRootChange(ctx context.Context, ownerID, computerID, docID string) {
	ctx = context.WithoutCancel(ctx)
	ownerID, computerID, docID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID), strings.TrimSpace(docID)
	if h == nil || h.Store == nil || h.Core == nil || ownerID == "" || computerID == "" || docID == "" {
		return
	}
	if !h.textureSettingOn(ctx, ownerID, computerID, store.SettingTextureRootDesk) {
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
	if !h.textureSettingOn(ctx, ownerID, computerID, store.SettingTextureRootDesk) {
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

// noteTextureRootChangeOn is the note with the switch already read on.
func (h *Handler) noteTextureRootChangeOn(ctx context.Context, ownerID, computerID, docID string) {
	if root, err := h.Store.GetTextureRoot(ctx, ownerID, computerID); err == nil && root.DocumentID == docID {
		return // the root's own document never wakes the root
	} else if err != nil && !errors.Is(err, store.ErrNotFound) {
		log.Printf("texture root: read marker on %s: %v", computerID, err)
	}
	now := h.textureRootClock()
	interval := h.Core.TextureRootDigestInterval()
	h.textureRootMu.Lock()
	scope := h.textureRootScopeLocked(ownerID, computerID)
	due := now.Add(interval)
	if slot := textureRootNextSlot(now, scope.attempts, interval, h.Core.TextureRootDigestPerHour()); slot.After(due) {
		due = slot
	}
	arm := h.armTextureRootDigestLocked(computerID, scope, now, due, false, docID)
	h.textureRootMu.Unlock()
	h.scheduleTextureRootDigest(ctx, ownerID, computerID, arm)
}

// armTextureRootDigestLocked adds docs to the pending set and arms one
// digest unless one is armed and not stale. The arm is marked under the
// mutex, so arming is exactly-once; the caller schedules it after
// releasing the mutex (scheduleTextureRootDigest), so the actor-log write
// does not serialize every commit's note.
func (h *Handler) armTextureRootDigestLocked(computerID string, scope *textureRootScope, now, due time.Time, full bool, docs ...string) *textureRootArm {
	scope.pending = addTextureRootPending(scope.pending, docs...)
	scope.pendingFull = scope.pendingFull || full
	if scope.armedNonce != "" && now.Before(scope.armedDue.Add(textureRootStaleArm)) {
		return nil // coalesced into the armed digest
	}
	scope.seq++
	digest := textureRootDigest{V: 1, Nonce: fmt.Sprintf("%d-%d", now.UnixNano(), scope.seq), Docs: append([]string(nil), scope.pending...), Full: scope.pendingFull}
	content, err := json.Marshal(digest)
	if err != nil {
		log.Printf("texture root: encode digest on %s: %v", computerID, err)
		return nil
	}
	scope.armedNonce, scope.armedDue = digest.Nonce, due
	return &textureRootArm{nonce: digest.Nonce, content: string(content), due: due}
}

// scheduleTextureRootDigest writes an armed digest to the actor log, outside
// the scope mutex. A failed write rolls the arm back (when it is still the
// armed one), so the next note arms again; the documents stay pending.
func (h *Handler) scheduleTextureRootDigest(ctx context.Context, ownerID, computerID string, arm *textureRootArm) {
	if arm == nil {
		return
	}
	err := h.Core.ScheduleActor(ctx, ownerID, computerID, agentcore.TextureRootDigestMailbox, agentcore.TextureRootDigestKind, arm.content, "", "", arm.due)
	if err == nil {
		return
	}
	log.Printf("texture root: schedule digest on %s: %v", computerID, err)
	h.textureRootMu.Lock()
	defer h.textureRootMu.Unlock()
	if scope := h.textureRootScopeLocked(ownerID, computerID); scope.armedNonce == arm.nonce {
		scope.armedNonce, scope.armedDue = "", time.Time{}
	}
}

// HandleTextureRootDigest is the digest's dispatch. It runs the ensure step
// (the marked master's liveness, checked before any activation), then wakes
// the master's own Texture desk with the documents the digest names, unless
// a root run is live or the rate bound holds it: then it re-arms one digest.
// A wake that starts no run (the master's reconcile found a live run in the
// gap, or its activation breaker is tripped) re-arms one digest too, with the
// documents kept; it counts against the rate bound, so it cannot loop.
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
	now := h.textureRootClock()
	h.textureRootMu.Lock()
	scope := h.textureRootScopeLocked(ownerID, computerID)
	docs := addTextureRootPending(append([]string(nil), digest.Docs...), scope.pending...)
	full := digest.Full || scope.pendingFull
	scope.pending, scope.pendingFull = nil, false
	if scope.armedNonce == digest.Nonce {
		scope.armedNonce, scope.armedDue = "", time.Time{}
	}
	h.textureRootMu.Unlock()
	requeue := func(due time.Time) {
		h.textureRootMu.Lock()
		arm := h.armTextureRootDigestLocked(computerID, h.textureRootScopeLocked(ownerID, computerID), now, due, full, docs...)
		h.textureRootMu.Unlock()
		h.scheduleTextureRootDigest(ctx, ownerID, computerID, arm)
	}
	if !h.textureSettingOn(ctx, ownerID, computerID, store.SettingTextureRootDesk) {
		log.Printf("texture root: digest on %s consumed: texture.root_desk is off", computerID)
		return nil
	}
	root, created, err := h.ensureTextureRoot(ctx, ownerID, computerID)
	if err != nil {
		h.textureRootMu.Lock()
		scope := h.textureRootScopeLocked(ownerID, computerID)
		scope.pending = addTextureRootPending(scope.pending, docs...)
		scope.pendingFull = scope.pendingFull || full
		h.textureRootMu.Unlock()
		return fmt.Errorf("texture root: ensure the master document: %w", err)
	}
	masters := textureRootMasterIDs(root)
	named := make([]string, 0, len(docs))
	for _, docID := range docs {
		if !masters[docID] {
			named = append(named, docID)
		}
	}
	docs = named
	full = full || created || len(docs) == 0
	agentID := currentTextureAgentID(root.DocumentID)
	interval := h.Core.TextureRootDigestInterval()
	if active, found, err := h.Core.TextureActiveRunByAgent(ctx, ownerID, computerID, agentID); err != nil {
		requeue(now.Add(interval))
		log.Printf("texture root: digest on %s re-armed: read the root's active run: %v", computerID, err)
		return nil
	} else if found && h.ValidateActivationAuthority(ctx, ownerID, computerID, agentID, active.RunID) == nil {
		// A root run is live: this digest's documents wait for one digest
		// after it, so no change is lost to coalescing.
		requeue(now.Add(interval))
		log.Printf("texture root: digest on %s re-armed: root run %s is live", computerID, active.RunID)
		return nil
	}
	perHour := h.Core.TextureRootDigestPerHour()
	h.textureRootMu.Lock()
	slot := textureRootNextSlot(now, h.textureRootScopeLocked(ownerID, computerID).attempts, interval, perHour)
	h.textureRootMu.Unlock()
	if slot.After(now) {
		requeue(slot)
		log.Printf("texture root: digest on %s re-armed for %s: the rate bound holds it", computerID, slot.Format(time.RFC3339))
		return nil
	}
	master, err := h.getTextureDocument(ctx, ownerID, root.DocumentID)
	if err != nil {
		requeue(now.Add(interval))
		return fmt.Errorf("texture root: load the master document: %w", err)
	}
	rec, err := h.reconcileTextureRootDigest(ctx, master, textureRootWake{Docs: docs, Full: full})
	if err != nil {
		requeue(now.Add(interval))
		return fmt.Errorf("texture root: wake the root: %w", err)
	}
	h.textureRootMu.Lock()
	scope = h.textureRootScopeLocked(ownerID, computerID)
	scope.attempts = append(scope.attempts, now)
	if keep := max(perHour, 1); len(scope.attempts) > 4*keep {
		scope.attempts = append([]time.Time(nil), scope.attempts[len(scope.attempts)-keep:]...)
	}
	next := textureRootNextSlot(now, scope.attempts, interval, perHour)
	h.textureRootMu.Unlock()
	if rec == nil {
		// Same semantics as a live root run: the documents wait for one
		// digest, at the interval and within the rate bound.
		if at := now.Add(interval); at.After(next) {
			next = at
		}
		requeue(next)
		log.Printf("texture root: digest on %s re-armed for %s: the master's reconcile started no run", computerID, next.Format(time.RFC3339))
	}
	return nil
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
// idempotent on liveness, and refuses with texture.root_desk off.
func (h *Handler) ensureTextureRoot(ctx context.Context, ownerID, computerID string) (store.TextureRoot, bool, error) {
	ownerID, computerID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID)
	if h == nil || h.Store == nil {
		return store.TextureRoot{}, false, fmt.Errorf("texture root: store unavailable")
	}
	if !h.textureSettingOn(ctx, ownerID, computerID, store.SettingTextureRootDesk) {
		return store.TextureRoot{}, false, fmt.Errorf("texture root: %s is off", store.SettingTextureRootDesk)
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
				return current, created, createErr
			}
			expected, generation, predecessor = &current, current.Generation+1, current.DocumentID
		case errors.Is(err, store.ErrNotFound):
		default:
			return store.TextureRoot{}, false, err
		}
		docID := textureRootDocumentID(ownerID, computerID, generation)
		claimed, err := h.Store.ClaimTextureRoot(ctx, expected, store.TextureRoot{
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
	created, err := h.startLifecycleTextureDocument(ctx, root.OwnerID, root.ComputerID, textureRootClientRequestID(root.Generation), textureRootTitle, seed)
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
	doc, err := h.Store.GetLifecycleDocument(ctx, ownerID, computerID, root.DocumentID)
	if errors.Is(err, store.ErrNotFound) {
		return false, false, nil
	}
	if err != nil {
		return false, false, err
	}
	if strings.TrimSpace(doc.TrajectoryID) == "" {
		return true, false, nil
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
// section and the ledger. A digest names the documents to read; otherwise
// (owner input on the master, a fresh master) it is a full refresh of the
// most recently revised documents. Both are bounded.
func (h *Handler) textureRootRequestSection(ctx context.Context, ownerID string, doc types.Document, root store.TextureRoot, req textureAgentRevisionRequest) string {
	computerID := strings.TrimSpace(doc.ComputerID)
	full := req.RootFullRefresh || len(req.RootDocuments) == 0
	masters := textureRootMasterIDs(root)
	masters[doc.DocID] = true
	var docs []types.Document
	if full {
		listed, err := h.Store.ListDocumentsByScope(ctx, ownerID, computerID, textureRootMaxDocuments+len(masters))
		if err != nil {
			log.Printf("texture root: list documents on %s: %v", computerID, err)
		}
		for _, candidate := range listed {
			if !masters[candidate.DocID] && len(docs) < textureRootMaxDocuments {
				docs = append(docs, candidate)
			}
		}
	} else {
		for _, docID := range req.RootDocuments {
			if len(docs) >= textureRootMaxDocuments {
				break
			}
			if masters[docID] {
				continue
			}
			candidate, err := h.Store.GetLifecycleDocument(ctx, ownerID, computerID, docID)
			if err != nil {
				continue
			}
			docs = append(docs, candidate)
		}
	}
	entries := make([]textureRootLedgerEntry, 0, len(docs))
	for _, candidate := range docs {
		entry := textureRootLedgerEntry{Doc: candidate}
		if strings.TrimSpace(candidate.TrajectoryID) != "" {
			if snapshot, err := h.Store.GetLifecycleSnapshot(ctx, ownerID, computerID, candidate.TrajectoryID); err == nil {
				entry.Snapshot, entry.HasSnapshot = snapshot, true
				entry.Ask = h.textureRootOwnerAsk(ctx, ownerID, computerID, snapshot.HeadRevision)
			}
		}
		entries = append(entries, entry)
	}
	var b strings.Builder
	b.WriteString("\n\n")
	b.WriteString(textureprompts.RootSection())
	if root.PredecessorDocumentID != "" {
		b.WriteString("\n\nThis master document replaces " + root.PredecessorDocumentID + ", which can no longer be revised; say so once.")
	}
	if full {
		b.WriteString("\n\nThe ledger, a full refresh (the most recently revised documents):\n\n")
	} else {
		b.WriteString("\n\nThe ledger, the documents that changed:\n\n")
	}
	b.WriteString(textureRootLedger(entries))
	return b.String()
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
// textureRootAskWalk revisions.
func (h *Handler) textureRootOwnerAsk(ctx context.Context, ownerID, computerID string, head types.Revision) string {
	if ask := textureRootHeadAsk(head); ask != "" {
		return ask
	}
	reads := 0
	load := func(revisionID string) (types.Revision, error) {
		if reads >= textureRootAskWalk {
			return types.Revision{}, errTextureRootAskWalk
		}
		reads++
		return h.Store.GetLifecycleRevision(ctx, ownerID, computerID, revisionID)
	}
	directions, first := collectTextureOwnerStory(head, nil, load, true)
	if n := len(directions); n > 0 {
		return truncatePromptSnippet(strings.Join(strings.Fields(directions[n-1].Text), " "), textureRootAskChars)
	}
	if first != nil {
		return truncatePromptSnippet(strings.Join(strings.Fields(first.Content), " "), textureRootAskChars)
	}
	return ""
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
