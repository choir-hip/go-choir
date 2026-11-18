package textureowner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/yusefmosiah/go-choir/internal/agentcore"
	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/editiondoc"
	"github.com/yusefmosiah/go-choir/internal/lease"
	"github.com/yusefmosiah/go-choir/internal/notebook"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/texturedoc"
	"github.com/yusefmosiah/go-choir/internal/texturewrite"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The listener's notebook, Texture's side (docs/design/autoradio-learned-
// document-2026-11-17.md 3.2; docs/reference/AUTORADIO-NOTEBOOK.md). Texture is
// the custodian of the document and its only writer. The Autoradio desk asks
// for a patch (grant notebook.patch) or reads (notebook.read); Texture checks
// the grant, checks the base revision is still the head, applies the ops with
// internal/notebook (no model, no lifecycle wake) and commits one revision with
// AuthorAppAgent and the label app:autoradio. Ways it can fail, written before
// the code:
//
//	N1 A caller the grant table does not name reads or writes it.
//	N2 Two documents titled alike: the most recently updated, never archived,
//	   wins; the legacy "My edition" is adopted and retitled, not duplicated.
//	N3 A patch built on an old head is applied anyway and drops the listener's edit.
//	N4 A replayed patch writes a second revision.
//	N5 A patch that passes the page budget is written.
//	N6 The committed revision wakes Texture's model to rewrite the desk's words.
//	N7 The first write finds no document and fails instead of creating it.
const (
	notebookSource    = "autoradio_notebook_patch"
	notebookPatchMeta = "notebook_patch_id"
	notebookChainMax  = 60
)

func notebookCaller(caller, grant string) error {
	if !lease.GrantHeldBy(grant, objectgraph.Writer(caller)) {
		return fmt.Errorf("%s does not hold the grant %s", caller, grant)
	}
	return nil
}

// notebookDocID is the deterministic id of the notebook this handler creates
// for one owner on one computer.
func notebookDocID(ownerID, computerID string) string {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte("choir:notebook:"+ownerID+":"+computerID+":document")).String()
}

// notebookDoc finds the owner's notebook. The deterministic id is looked up
// first; then the owner's own non-archived documents titled My notebook (or
// the legacy My edition) that no model agent wrote. A listing that fails is an
// error, never "not found". archived is true when the notebook exists and the
// owner archived it: the desk reads and writes nothing then (un-archiving is
// the owner's act).
func (h *Handler) notebookDoc(ctx context.Context, ownerID, computerID string) (doc types.Document, found, archived bool, err error) {
	// The identity is persisted: the document the latest applied patch wrote
	// (its result record names it), then the deterministic id, then the title.
	// HandleEdition and the desk's reads and writes all come through here, so
	// the phone can never edit one document while the DJ reads another.
	ids := []string{notebookDocID(ownerID, computerID)}
	if hint := agentcore.NotebookDocHint(ctx, h.Store, ownerID, computerID); hint != "" && hint != ids[0] {
		ids = []string{hint, ids[0]}
	}
	for _, id := range ids {
		d, err := h.Store.GetLifecycleDocument(ctx, ownerID, computerID, id)
		if err == nil {
			if d.ArchivedAt != nil {
				return types.Document{}, false, true, nil
			}
			return d, true, false, nil
		} else if !errors.Is(err, store.ErrNotFound) {
			return types.Document{}, false, false, fmt.Errorf("look up the notebook by id: %w", err)
		}
	}
	docs, err := h.Store.ListDocumentsByScope(asTextureReader(ctx), ownerID, computerID, algorithm2DocLimit)
	if err != nil {
		return types.Document{}, false, false, fmt.Errorf("list the owner's documents: %w", err)
	}
	norm := func(t string) string { return strings.Join(strings.Fields(t), " ") }
	var cands []types.Document
	for _, d := range docs {
		if !editiondoc.IsEditionTitle(d.Title) {
			continue
		}
		if d.ArchivedAt != nil {
			archived = true
			continue
		}
		cands = append(cands, d)
	}
	// A document already titled as the notebook beats a legacy one; then the
	// most recently updated wins.
	sort.SliceStable(cands, func(i, j int) bool {
		ci, cj := strings.EqualFold(norm(cands[i].Title), editiondoc.Title), strings.EqualFold(norm(cands[j].Title), editiondoc.Title)
		if ci != cj {
			return ci
		}
		return cands[i].UpdatedAt.After(cands[j].UpdatedAt)
	})
	for _, d := range cands {
		ok, err := h.notebookAdoptable(ctx, ownerID, d)
		if err != nil {
			return types.Document{}, false, false, err
		}
		if ok {
			return d, true, false, nil
		}
	}
	return types.Document{}, false, archived, nil
}

// notebookAdoptable tells a document the listener (or this desk) wrote from one
// a model agent made: any revision in its chain by an agent other than the
// desk's own label means a model wrote it, and it is not adopted.
func (h *Handler) notebookAdoptable(ctx context.Context, ownerID string, d types.Document) (bool, error) {
	id := d.CurrentRevisionID
	for i := 0; id != "" && i < notebookChainMax; i++ {
		rev, err := h.Store.GetLifecycleRevision(ctx, ownerID, d.ComputerID, id)
		if err != nil {
			return false, fmt.Errorf("read the revisions of %s: %w", d.DocID, err)
		}
		if rev.AuthorKind == types.AuthorAppAgent && rev.AuthorLabel != notebook.DeskLabel {
			return false, nil
		}
		id = rev.ParentRevisionID
	}
	return true, nil
}

func notebookRevision(r types.Revision) notebook.Revision {
	return notebook.Revision{RevisionID: r.RevisionID, ParentID: r.ParentRevisionID, AuthorKind: string(r.AuthorKind),
		AuthorLabel: r.AuthorLabel, At: r.CreatedAt, Text: r.Content}
}

// NotebookRead is the notebook.read grant: the head and the chain back to the
// newest desk revision.
func (h *Handler) NotebookRead(ctx context.Context, ownerID, caller string) (notebook.State, error) {
	if err := notebookCaller(caller, lease.GrantNotebookRead); err != nil {
		return notebook.State{}, err
	}
	if h == nil || h.Store == nil || h.Core == nil {
		return notebook.State{}, errors.New("texture store unavailable")
	}
	computerID := strings.TrimSpace(h.Core.TextureComputerID())
	ctx = objectgraph.WithCustodianReader(ctx, "choir.texture_document")
	doc, found, archived, err := h.notebookDoc(ctx, ownerID, computerID)
	if err != nil {
		return notebook.State{}, err
	}
	if archived {
		return notebook.State{Archived: true}, nil
	}
	if !found {
		return notebook.State{}, nil
	}
	st := notebook.State{Found: true, DocID: doc.DocID}
	id := doc.CurrentRevisionID
	reached := false
	for i := 0; id != "" && i < notebookChainMax; i++ {
		rev, err := h.Store.GetLifecycleRevision(ctx, ownerID, doc.ComputerID, id)
		if err != nil {
			return notebook.State{}, err
		}
		nr := notebookRevision(rev)
		if i == 0 {
			st.Head = nr
		}
		st.Chain = append(st.Chain, nr)
		if nr.IsDesk() {
			reached = true
			break
		}
		id = rev.ParentRevisionID
	}
	// The cap reached before a desk revision (and not the document's first
	// revision) means the edits cannot be compared: the caller must not skip
	// strikes silently.
	st.ChainCut = !reached && id != ""
	return st, nil
}

// NotebookApply is the notebook.patch grant.
func (h *Handler) NotebookApply(ctx context.Context, ownerID, caller string, p notebook.Patch) (notebook.Result, error) {
	if err := notebookCaller(caller, lease.GrantNotebookPatch); err != nil {
		return notebook.Result{}, err
	}
	if h == nil || h.Store == nil || h.Core == nil {
		return notebook.Result{}, errors.New("texture store unavailable")
	}
	if strings.TrimSpace(p.PatchID) == "" {
		return notebook.Result{Status: notebook.StatusRefused, Reason: "a patch has an id"}, nil
	}
	h.textureEditMu.Lock()
	defer h.textureEditMu.Unlock()
	computerID := strings.TrimSpace(h.Core.TextureComputerID())
	ctx = objectgraph.WithCustodianReader(ctx, "choir.texture_document")
	doc, found, archived, err := h.notebookDoc(ctx, ownerID, computerID)
	if err != nil {
		return notebook.Result{}, err
	}
	if archived {
		return notebook.Result{Status: notebook.StatusRefused, Reason: "your notebook is archived"}, nil
	}
	if !found {
		if p.BaseRevisionID != "" {
			return notebook.Result{Status: notebook.StatusStale, Reason: "the notebook is gone"}, nil
		}
		next, err := notebook.Fresh().Apply(p)
		if err != nil {
			return notebook.Result{Status: notebook.StatusRefused, Reason: err.Error()}, nil
		}
		return h.startNotebook(ctx, ownerID, computerID, p, next)
	}
	head, err := h.Store.GetLifecycleRevision(ctx, ownerID, doc.ComputerID, doc.CurrentRevisionID)
	if err != nil {
		return notebook.Result{}, err
	}
	if metaString(head.Metadata, notebookPatchMeta) == p.PatchID {
		return notebook.Result{Status: notebook.StatusApplied, Reason: "replay", DocID: doc.DocID, RevisionID: head.RevisionID}, nil
	}
	if head.RevisionID != p.BaseRevisionID {
		return notebook.Result{Status: notebook.StatusStale, Reason: "the head is " + head.RevisionID, DocID: doc.DocID}, nil
	}
	cur := notebook.Parse(head.Content)
	next, err := cur.Apply(p)
	if err != nil {
		return notebook.Result{Status: notebook.StatusRefused, Reason: err.Error(), DocID: doc.DocID}, nil
	}
	if next.Render() == cur.Render() && strings.EqualFold(strings.Join(strings.Fields(doc.Title), " "), editiondoc.Title) {
		return notebook.Result{Status: notebook.StatusNoop, DocID: doc.DocID, RevisionID: head.RevisionID}, nil
	}
	if !strings.EqualFold(strings.Join(strings.Fields(doc.Title), " "), editiondoc.Title) {
		if _, err := h.Store.UpdateLifecycleDocumentTitleAuthority(asTextureCustodian(ctx), ownerID, doc.ComputerID, doc.DocID, editiondoc.Title); err != nil {
			return notebook.Result{}, fmt.Errorf("adopt the edition document: %w", err)
		}
	}
	return h.commitNotebook(ctx, ownerID, doc, head, p, next)
}

func metaString(raw json.RawMessage, key string) string {
	var m map[string]any
	if len(raw) == 0 || json.Unmarshal(raw, &m) != nil {
		return ""
	}
	s, _ := m[key].(string)
	return s
}

func notebookBody(docID, revisionID, content string) (json.RawMessage, string, error) {
	body := texturewrite.PlainDoc(docID, revisionID, content)
	proj, err := texturedoc.Project(body, nil)
	if err != nil {
		return nil, "", err
	}
	raw, err := json.Marshal(body)
	return raw, proj.Text, err
}

func (h *Handler) commitNotebook(ctx context.Context, ownerID string, doc types.Document, head types.Revision, p notebook.Patch, next notebook.Doc) (notebook.Result, error) {
	text := next.Render()
	revisionID := uuid.NewSHA1(uuid.NameSpaceOID, []byte("choir:notebook:"+doc.DocID+":"+p.PatchID)).String()
	bodyJSON, projected, err := notebookBody(doc.DocID, revisionID, text)
	if err != nil {
		return notebook.Result{}, err
	}
	snapshot, err := h.Store.GetLifecycleSnapshot(ctx, ownerID, doc.ComputerID, doc.TrajectoryID)
	if err != nil {
		return notebook.Result{}, err
	}
	meta, _ := json.Marshal(map[string]any{"source": notebookSource, notebookPatchMeta: p.PatchID, "texture_write_base_revision_id": head.RevisionID})
	rev := types.Revision{
		RevisionID: revisionID, DocID: doc.DocID, OwnerID: ownerID, ComputerID: doc.ComputerID, TrajectoryID: doc.TrajectoryID,
		AuthorKind: types.AuthorAppAgent, AuthorLabel: notebook.DeskLabel, Content: projected, BodyDoc: bodyJSON,
		Citations: json.RawMessage("[]"), Metadata: meta, ParentRevisionID: head.RevisionID, CreatedAt: time.Now().UTC(),
	}
	commandID := "notebook-patch:" + p.PatchID
	command := types.CommitLifecycleArtifactHeadRequest{
		OwnerID: ownerID, ComputerID: doc.ComputerID, CommandID: commandID, TrajectoryID: doc.TrajectoryID,
		ExpectedLifecycleVersion: snapshot.Trajectory.LifecycleVersion, ExpectedHeadRevisionID: head.RevisionID,
		Unbound: snapshot.Trajectory.Status != types.TrajectoryLive, Revision: rev,
	}
	graph, err := textureToolSourceGraphWriteSet(rev, materializedTextureEdit{BodyDoc: bodyJSON},
		&types.RunRecord{RunID: commandID, OwnerID: ownerID, ComputerID: doc.ComputerID})
	if err != nil {
		return notebook.Result{}, fmt.Errorf("notebook source graph: %w", err)
	}
	if command.CommandDigest, err = store.ComputeCommitLifecycleArtifactHeadWithSourceGraphDigest(command, graph); err != nil {
		return notebook.Result{}, err
	}
	result, err := h.Store.CommitLifecycleArtifactHeadWithSourceGraph(asTextureCustodian(ctx), command, graph)
	if err != nil {
		if errors.Is(err, store.ErrConcurrentStateChange) || errors.Is(err, store.ErrStaleDocumentHead) {
			return notebook.Result{Status: notebook.StatusStale, Reason: err.Error(), DocID: doc.DocID}, nil
		}
		return notebook.Result{}, fmt.Errorf("commit the notebook revision: %w", err)
	}
	if result.Revision == nil {
		return notebook.Result{}, errors.New("the commit returned no revision")
	}
	if !result.Replay {
		h.recordTextureAudit(ctx, "notebook_revision_committed", ownerID, doc.ComputerID, doc.TrajectoryID, doc.DocID, result.Revision.RevisionID, command.CommandID, command.CommandDigest, result.Trajectory.LifecycleVersion)
		h.emitTextureDocumentRevisionEvent(ctx, ownerID, *result.Revision)
	}
	return notebook.Result{Status: notebook.StatusApplied, DocID: doc.DocID, RevisionID: result.Revision.RevisionID, Words: next.WrittenWords()}, nil
}

// startNotebook creates the listener's notebook with the desk's first patch as
// its first revision. The document is an ordinary lifecycle Texture whose v0
// is written by the desk, not the owner, so the first revision is not read as a
// listener's edit.
func (h *Handler) startNotebook(ctx context.Context, ownerID, computerID string, p notebook.Patch, next notebook.Doc) (notebook.Result, error) {
	key := "choir:notebook:" + ownerID + ":" + computerID
	docID := notebookDocID(ownerID, computerID)
	trajectoryID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(key+":trajectory")).String()
	workID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(key+":work")).String()
	revisionID := uuid.NewSHA1(uuid.NameSpaceOID, []byte("choir:notebook:"+docID+":"+p.PatchID)).String()
	text := next.Render()
	bodyJSON, projected, err := notebookBody(docID, revisionID, text)
	if err != nil {
		return notebook.Result{}, err
	}
	now := time.Now().UTC()
	agentID := deskmanifest.DocumentCustodianName() + ":" + docID
	profile := deskmanifest.DocumentCustodianName()
	meta, _ := json.Marshal(map[string]any{"source": notebookSource, notebookPatchMeta: p.PatchID})
	start := types.StartLifecycleRequest{
		OwnerID: ownerID, ComputerID: computerID, CommandID: "start:notebook:" + p.PatchID, TrajectoryID: trajectoryID, Kind: types.TrajectoryKindDocument,
		SubjectRefs:     map[string]string{"artifact": deskmanifest.DocumentArtifactPrefix() + docID, "doc_id": docID},
		SettlementRule:  types.SettlementRule{Version: types.LifecycleReducerVersion, RequireNoOpenWorkItems: true, RequiredSubjectRefs: []string{"artifact"}},
		InitialWork:     types.WorkItemRecord{WorkItemID: workID, Objective: "The listener's notebook: Autoradio applies patches; no model writes it.", AssignedAgentID: agentID, AuthorityProfile: profile},
		InitialDocument: types.Document{DocID: docID, OwnerID: ownerID, ComputerID: computerID, TrajectoryID: trajectoryID, Title: editiondoc.Title, CreatedAt: now, UpdatedAt: now},
		InitialRevision: types.Revision{RevisionID: revisionID, DocID: docID, OwnerID: ownerID, ComputerID: computerID, TrajectoryID: trajectoryID,
			AuthorKind: types.AuthorAppAgent, AuthorLabel: notebook.DeskLabel, Content: projected, BodyDoc: bodyJSON, Citations: json.RawMessage("[]"), Metadata: meta, CreatedAt: now},
		Agent: types.AgentRecord{AgentID: agentID, OwnerID: ownerID, ComputerID: computerID, Profile: profile, Role: profile, ChannelID: docID, CreatedAt: now, UpdatedAt: now},
	}
	start.StartRequestDigest, _ = store.ComputeStartLifecycleRequestDigest(start)
	res, err := h.Store.StartLifecycle(asOwner(ctx), start)
	if err != nil {
		return notebook.Result{}, fmt.Errorf("start the notebook: %w", err)
	}
	if res.Revision == nil {
		return notebook.Result{}, errors.New("the notebook start returned no revision")
	}
	return notebook.Result{Status: notebook.StatusApplied, DocID: docID, RevisionID: res.Revision.RevisionID, Words: next.WrittenWords()}, nil
}
