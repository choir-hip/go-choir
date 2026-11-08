package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/sourceproj"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// A Cast refused at stack_base_moved leaves its Texture waiting: the refusal
// reaches the management cell as an error and nothing is durable, so when the
// platform's rebase applies (or the stack becomes base, upstreamed or
// rematerialized) nobody tells the waiting Textures, and each needs an owner
// note to re-Cast (docs/problems/stack-base-moved-waiters-not-woken-2026-11-04.md).
//
// The refusal registers its document in a small file under the selfdev
// updater root, because the rebase apply goes through a planned restart and an
// in-memory map would be empty exactly when the block clears. The wake comes
// from a reconcile act: the live pass that moves the stack off stack_base_moved,
// or the boot pass of a PLANNED restart (marker present) that finds the stack
// off it. A crash boot (no marker) wakes no one and deletes the entries it
// finds (CLAUDE.md: a crash restart never resumes work).

const stackBaseWaitersFile = "source-projection/base-waiters.json"

type stackBaseWaiter struct {
	OwnerID      string `json:"owner_id"`
	ComputerID   string `json:"computer_id"`
	DocID        string `json:"doc_id"`
	TrajectoryID string `json:"trajectory_id,omitempty"`
	CastRecordID string `json:"cast_record_id"`
	At           string `json:"at"`
}

func (rt *Runtime) stackBaseWaitersPath() string {
	root := strings.TrimSpace(rt.selfdevUpdaterRoot)
	if root == "" {
		return ""
	}
	return filepath.Join(root, filepath.FromSlash(stackBaseWaitersFile))
}

// readStackBaseWaiters and writeStackBaseWaiters run under stackBaseWaitMu.
func (rt *Runtime) readStackBaseWaiters() map[string]stackBaseWaiter {
	out := map[string]stackBaseWaiter{}
	path := rt.stackBaseWaitersPath()
	if path == "" {
		return out
	}
	if raw, err := os.ReadFile(path); err == nil {
		if json.Unmarshal(raw, &out) != nil {
			return map[string]stackBaseWaiter{} // a torn file reads as no waiters
		}
	}
	return out
}

func (rt *Runtime) writeStackBaseWaiters(waiters map[string]stackBaseWaiter) {
	path := rt.stackBaseWaitersPath()
	if path == "" {
		return
	}
	if len(waiters) == 0 {
		_ = os.Remove(path)
		return
	}
	raw, err := json.Marshal(waiters)
	if err == nil {
		err = os.MkdirAll(filepath.Dir(path), 0o700)
	}
	if err == nil {
		tmp := path + ".tmp"
		if err = os.WriteFile(tmp, raw, 0o600); err == nil {
			err = os.Rename(tmp, path)
		}
	}
	if err != nil {
		log.Printf("source projection: stack_base_moved waiters: %v", err)
	}
}

// noteStackBaseWaiter records the document whose Cast was just refused at
// stack_base_moved. One entry per document: a second refusal replaces it. If
// the last reconcile already admits a cast (the block cleared between the
// refusal and this registration), the waiter is woken at once.
func (rt *Runtime) noteStackBaseWaiter(ctx context.Context, w stackBaseWaiter, refusal string) {
	if rt == nil || strings.TrimSpace(w.DocID) == "" || !strings.HasPrefix(refusal, string(sourceproj.StateBaseMoved)) {
		return
	}
	w.At = time.Now().UTC().Format(time.RFC3339Nano)
	rt.stackBaseWaitMu.Lock()
	waiters := rt.readStackBaseWaiters()
	waiters[w.DocID] = w
	rt.writeStackBaseWaiters(waiters)
	rt.stackBaseWaitMu.Unlock()
	if last := rt.sourceProjectionStatus(); last != nil {
		rt.wakeStackBaseWaiters(ctx, *last)
	}
}

// startStackBaseBootPass runs once per process, at the first reconcile of any
// reason and before the reconcile reads anything: a crash boot (no planned
// restart marker) deletes the waiters registered before this boot and wakes no
// one (CLAUDE.md: a crash restart never resumes work). A planned boot keeps
// them; the first reconcile that admits a cast wakes them.
func (rt *Runtime) startStackBaseBootPass() {
	rt.stackBaseBootOnce.Do(func() {
		if _, planned := rt.BootWasPlannedRestart(); planned || rt.stackBaseWaitersPath() == "" {
			return
		}
		rt.stackBaseWaitMu.Lock()
		defer rt.stackBaseWaitMu.Unlock()
		all := rt.readStackBaseWaiters()
		for id, w := range all {
			if at, _ := time.Parse(time.RFC3339Nano, w.At); at.Before(rt.bootedAt) {
				delete(all, id)
			}
		}
		rt.writeStackBaseWaiters(all)
	})
}

// wakeStackBaseWaiters runs after a reconcile stored result. When the result
// admits a cast (Refusal is empty: not pending, unavailable or unrebuildable,
// which would refuse the re-Cast and lose the waiter), every registered
// document is woken once and its entry removed after its note commits.
func (rt *Runtime) wakeStackBaseWaiters(ctx context.Context, result sourceproj.Result) {
	if rt == nil || rt.stackBaseWaitersPath() == "" || result.Refusal() != "" {
		return
	}
	// One wake pass at a time: two overlapping passes over different results
	// would each commit a note for one refusal.
	rt.stackBaseWakeMu.Lock()
	defer rt.stackBaseWakeMu.Unlock()
	rt.stackBaseWaitMu.Lock()
	pending := rt.readStackBaseWaiters()
	rt.stackBaseWaitMu.Unlock()
	for id, w := range pending {
		done, err := rt.wakeStackBaseWaiter(ctx, w, result)
		if err != nil {
			log.Printf("source projection: stack_base_moved cleared: Texture %s not woken: %v", w.DocID, err)
		}
		if !done {
			continue
		}
		rt.stackBaseWaitMu.Lock()
		current := rt.readStackBaseWaiters()
		if cur, ok := current[id]; ok && cur.CastRecordID == w.CastRecordID {
			delete(current, id)
			rt.writeStackBaseWaiters(current)
		}
		rt.stackBaseWaitMu.Unlock()
	}
}

// wakeStackBaseWaiter delivers the note and reports whether the entry is done:
// the note committed, or nothing waits (document gone, archived, cancelled).
// A read or commit failure leaves it for the next admitting reconcile.
func (rt *Runtime) wakeStackBaseWaiter(ctx context.Context, w stackBaseWaiter, result sourceproj.Result) (bool, error) {
	if rt.store == nil {
		return true, nil
	}
	// The host checks the waiting document still lives, on whatever context
	// the admitting reconcile carries (a background one from a rebase notice):
	// a host read of Texture's kind, made as its custodian (enforce flip S3,
	// Devin r2 note).
	doc, err := rt.store.GetLifecycleDocument(objectgraph.WithCustodianReader(ctx, "choir.texture_document"), w.OwnerID, w.ComputerID, w.DocID)
	if errors.Is(err, store.ErrNotFound) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("document %s: %w", w.DocID, err)
	}
	if doc.ArchivedAt != nil {
		return true, nil
	}
	inbox := currentTextureAgentID(w.DocID)
	// Narrower than the rebase notice's addressee rule (rebaseOperationTexture):
	// a live trajectory without a cancellation intent, whether or not it has
	// open work (a Texture settles its items at the end of each round; the note
	// is the wake), and a trajectory whose episode has ended, where the note
	// only waits in the document-keyed inbox and the document is NOT continued
	// (this note is advisory; it must not resurrect an ended episode). A
	// cancelled or not-live trajectory gets nothing.
	state, err := rt.rebaseTextureRunState(ctx, w.OwnerID, w.ComputerID, strings.TrimSpace(doc.TrajectoryID), inbox)
	if err != nil {
		return false, err
	}
	if state != rebaseTextureOpen && state != rebaseTextureSettled && state != rebaseTextureTrajectorySettled {
		return true, nil
	}
	operationID := result.OperationID
	sender := "selfdev:stack-base"
	if operationID = strings.TrimSpace(operationID); operationID != "" {
		sender = types.OwnerActSenderInbox(operationID)
	}
	body := fmt.Sprintf("The block that refused your Cast has cleared: the applied stack is no longer on a moved base (it is now %s). Cast to management again.", result.State)
	rec := types.CommitmentRecord{
		SchemaID:    types.CommitmentRecordSchemaV1,
		RecordID:    "stack-base-cleared:" + w.DocID + ":" + w.CastRecordID + ":" + shortDigest(result.Head) + ":" + shortDigest(result.BootedBase),
		Kind:        types.CommitmentKindDirective,
		Discrepancy: types.DiscrepancyUnresolved,
		Provenance:  types.CommitmentProvenance{AgentID: types.OwnerActAgentID},
		EmitterKind: types.CommitmentEmitterSystem,
		ComputerID:  w.ComputerID,
		Inbox:       inbox, SenderInbox: sender, Addressee: inbox,
		Directive:    &types.CommitmentDirective{Subtype: types.CommitmentDirectiveNotice, Body: body},
		EvidenceRefs: []string{w.CastRecordID},
		SubjectRefs:  map[string]string{"stack_base_cleared": string(result.State), "cast_record_id": w.CastRecordID},
	}
	req := types.CommitInboxRecordsRequest{OwnerID: w.OwnerID, ComputerID: w.ComputerID, CommandID: rec.RecordID, Records: []types.CommitmentRecord{rec}}
	digest, err := store.ComputeCommitInboxRecordsDigest(req)
	if err != nil {
		return false, err
	}
	req.CommandDigest = digest
	if _, err := rt.store.CommitInboxRecords(ctx, req); err != nil {
		return false, fmt.Errorf("commit %s: %w", rec.RecordID, err)
	}
	return true, nil
}
