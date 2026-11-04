package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

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
		_ = json.Unmarshal(raw, &out) // a torn file reads as no waiters
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
// stack_base_moved. One entry per document: a second refusal replaces it.
func (rt *Runtime) noteStackBaseWaiter(w stackBaseWaiter, refusal string) {
	if rt == nil || strings.TrimSpace(w.DocID) == "" || !strings.HasPrefix(refusal, string(sourceproj.StateBaseMoved)) {
		return
	}
	w.At = time.Now().UTC().Format(time.RFC3339Nano)
	rt.stackBaseWaitMu.Lock()
	defer rt.stackBaseWaitMu.Unlock()
	waiters := rt.readStackBaseWaiters()
	waiters[w.DocID] = w
	rt.writeStackBaseWaiters(waiters)
}

// wakeStackBaseWaiters runs after a reconcile stored result over previous.
//   - live transition (previous is stack_base_moved, result is not): wake all.
//   - boot pass (previous nil, reason "boot"): a planned restart wakes all
//     registered before this boot when the stack is off stack_base_moved and
//     keeps them otherwise; a crash boot deletes them and wakes no one.
//
// The file is drained first, so a document is woken once per refusal. Entries
// registered during this boot are never touched by the boot pass.
func (rt *Runtime) wakeStackBaseWaiters(ctx context.Context, reason string, previous *sourceproj.Result, result sourceproj.Result) {
	if rt == nil || rt.stackBaseWaitersPath() == "" {
		return
	}
	live := previous != nil && previous.State == sourceproj.StateBaseMoved && result.State != sourceproj.StateBaseMoved
	boot := previous == nil && reason == "boot"
	if !live && !boot {
		return
	}
	_, planned := rt.BootWasPlannedRestart()
	rt.stackBaseWaitMu.Lock()
	all := rt.readStackBaseWaiters()
	var wake []stackBaseWaiter
	for id, w := range all {
		if boot {
			at, _ := time.Parse(time.RFC3339Nano, w.At)
			if !at.Before(rt.bootedAt) {
				continue // registered during this boot
			}
			if planned && result.State == sourceproj.StateBaseMoved {
				continue // still blocked: keep it for the clearing pass
			}
			if planned {
				wake = append(wake, w)
			}
			delete(all, id) // a crash boot only clears
			continue
		}
		wake = append(wake, w)
		delete(all, id)
	}
	rt.writeStackBaseWaiters(all)
	rt.stackBaseWaitMu.Unlock()
	operationID := ""
	if previous != nil {
		operationID = previous.OperationID
	} else {
		operationID = result.OperationID
	}
	for _, w := range wake {
		if err := rt.wakeStackBaseWaiter(ctx, w, operationID, result); err != nil {
			log.Printf("source projection: stack_base_moved cleared: Texture %s not woken: %v", w.DocID, err)
		}
	}
}

func (rt *Runtime) wakeStackBaseWaiter(ctx context.Context, w stackBaseWaiter, operationID string, result sourceproj.Result) error {
	if rt.store == nil {
		return nil
	}
	doc, err := rt.store.GetLifecycleDocument(ctx, w.OwnerID, w.ComputerID, w.DocID)
	if err != nil || doc.ArchivedAt != nil {
		return nil // gone or archived: nothing waits
	}
	inbox := currentTextureAgentID(w.DocID)
	// Only a Texture that can take a run: a cancelled or settled document is
	// not resumed by this note (the owner's revise continues it).
	state, err := rt.rebaseTextureRunState(ctx, w.OwnerID, w.ComputerID, strings.TrimSpace(doc.TrajectoryID), inbox)
	if err != nil || state != rebaseTextureOpen {
		return err
	}
	sender := "selfdev:stack-base"
	if operationID = strings.TrimSpace(operationID); operationID != "" {
		sender = types.OwnerActSenderInbox(operationID)
	}
	body := fmt.Sprintf("The block that refused your Cast has cleared: the applied stack is no longer on a moved base (it is now %s). Cast to management again.", result.State)
	rec := types.CommitmentRecord{
		SchemaID:    types.CommitmentRecordSchemaV1,
		RecordID:    "stack-base-cleared:" + w.DocID + ":" + w.CastRecordID,
		Kind:        types.CommitmentKindDirective,
		Discrepancy: types.DiscrepancyUnresolved,
		Provenance:  types.CommitmentProvenance{AgentID: types.OwnerActAgentID},
		EmitterKind: types.CommitmentEmitterSystem,
		ComputerID:  w.ComputerID,
		Inbox:       inbox, SenderInbox: sender, Addressee: inbox,
		Directive:    &types.CommitmentDirective{Subtype: types.CommitmentDirectiveNote, Body: body},
		EvidenceRefs: []string{w.CastRecordID},
		SubjectRefs:  map[string]string{"stack_base_cleared": string(result.State), "cast_record_id": w.CastRecordID},
	}
	req := types.CommitInboxRecordsRequest{OwnerID: w.OwnerID, ComputerID: w.ComputerID, CommandID: rec.RecordID, Records: []types.CommitmentRecord{rec}}
	digest, err := store.ComputeCommitInboxRecordsDigest(req)
	if err != nil {
		return err
	}
	req.CommandDigest = digest
	if _, err := rt.store.CommitInboxRecords(ctx, req); err != nil {
		return fmt.Errorf("commit %s: %w", rec.RecordID, err)
	}
	return nil
}
