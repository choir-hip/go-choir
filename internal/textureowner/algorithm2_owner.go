package textureowner

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/algorithm2"
	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Algorithm2 on the owner's computer (docs/design/autopaper-next-mission-handoff-2026-11-09.md,
// 4.H). The behavior ledger is derived on read from the changelog events and
// the revisions they point at, and is served as text on owner routes only. A
// round makes at most one model call through the Texture desk's model policy
// and stores a proposal: the whole proposed algorithm text, a local object
// shaped like a published version. Accepting a proposal is a revise call with
// the proposal as its source (a take), so the revision carries the proposal as
// a source link. A proposal is a source only for the acceptance of that
// proposal on its own document (proposalGrantKey), never for an ordinary
// revise; that is the one place decision D9 is relaxed.

const (
	algorithm2PathPrefix      = "/api/texture/algorithm2/"
	algorithm2ProposalsPrefix = algorithm2PathPrefix + "proposals/"
	algorithm2DocLimit        = 500
	algorithm2RevisionReads   = 400
	algorithm2RoundTimeout    = 3 * time.Minute
)

type proposalGrantKey struct{}

// proposalVersionReader serves a pending proposal as a published-version-shaped
// source to the one call that is accepting it, and everything else from the
// platform.
type proposalVersionReader struct {
	h        *Handler
	platform PublicationVersionReader
}

func (p proposalVersionReader) ReadPublicationVersion(ctx context.Context, id string) (PublicationVersion, error) {
	if !algorithm2.IsProposalID(id) {
		if p.platform == nil {
			return PublicationVersion{}, errors.New("the platform is not reachable")
		}
		return p.platform.ReadPublicationVersion(ctx, id)
	}
	docID, _ := ctx.Value(proposalGrantKey{}).(string)
	if docID == "" {
		return PublicationVersion{}, ErrSourceNotReadable
	}
	st := algorithm2.NewFileStore(p.h.Algorithm2Dir)
	prop, ok, err := st.Get(id)
	if err != nil || !ok || prop.DocID != docID || prop.State != algorithm2.StatePending {
		return PublicationVersion{}, ErrSourceNotReadable
	}
	out := PublicationVersion{
		PublicationVersionID: prop.ID, OwnerID: "local", AuthorLabel: "algorithm2", Title: "Algorithm proposal",
		ManifestHash: prop.ManifestHash, Content: prop.Content,
	}
	// A revision's content must equal its body's projection, so the version
	// carries both from the same conversion. The pin stays on the proposal's
	// own text.
	if body, _, projected, err := markdownLineageStructuredRevision(prop.DocID, "proposal-"+strings.TrimPrefix(prop.ID, algorithm2.ProposalIDPrefix), prop.Content, nil, nil); err == nil {
		out.BodyDoc, out.Content = body, projected
	}
	return out, nil
}

func (h *Handler) algorithm2Clock() time.Time {
	if h.algorithm2Now != nil {
		return h.algorithm2Now()
	}
	return time.Now()
}

// algorithm2DocIDs optionally names the two documents; empty finds them by title.
type algorithm2DocIDs struct {
	Algorithm  string `json:"algorithm_doc_id,omitempty"`
	Algorithm2 string `json:"algorithm2_doc_id,omitempty"`
}

func normalizeDocTitle(t string) string {
	t = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(t), "²", "2"))
	return strings.Join(strings.Fields(t), "")
}

// findAlgorithmDocs resolves the algorithm document and the algorithm2
// document: the ids the owner named, else the newest unarchived document
// titled "Algorithm" and "Algorithm 2".
func (h *Handler) findAlgorithmDocs(ctx context.Context, ownerID string, ids algorithm2DocIDs) (algo, algo2 types.Document, err error) {
	computerID := strings.TrimSpace(h.Core.TextureComputerID())
	byID := func(id string) (types.Document, bool) {
		if id = strings.TrimSpace(id); id == "" {
			return types.Document{}, false
		}
		d, e := h.getTextureDocument(asOwner(ctx), ownerID, id)
		return d, e == nil
	}
	var found1, found2 bool
	algo, found1 = byID(ids.Algorithm)
	algo2, found2 = byID(ids.Algorithm2)
	if found1 && found2 {
		return algo, algo2, nil
	}
	docs, lerr := h.Store.ListDocumentsByScope(asTextureReader(ctx), ownerID, computerID, algorithm2DocLimit)
	if lerr != nil {
		return algo, algo2, lerr
	}
	sort.SliceStable(docs, func(i, j int) bool { return docs[i].UpdatedAt.After(docs[j].UpdatedAt) })
	for _, d := range docs {
		if d.ArchivedAt != nil {
			continue
		}
		switch normalizeDocTitle(d.Title) {
		case "algorithm":
			if !found1 {
				algo, found1 = d, true
			}
		case "algorithm2":
			if !found2 {
				algo2, found2 = d, true
			}
		}
	}
	if !found1 || !found2 {
		return algo, algo2, errors.New("the owner has no document titled Algorithm and one titled Algorithm 2")
	}
	return algo, algo2, nil
}

func (h *Handler) headText(ctx context.Context, ownerID string, doc types.Document) (revisionID, text string, err error) {
	// Re-read the document: the head is what the decision must see.
	cur, err := h.getTextureDocument(asOwner(ctx), ownerID, doc.DocID)
	if err != nil {
		return "", "", err
	}
	rev, err := h.getTextureRevision(ctx, ownerID, cur.CurrentRevisionID)
	if err != nil {
		return "", "", err
	}
	return rev.RevisionID, rev.Content, nil
}

// buildAlgorithm2Ledger derives the owner's behavior ledger from the
// changelog events and the revisions they point at. It reads, and never writes.
func (h *Handler) buildAlgorithm2Ledger(ctx context.Context, ownerID string) ([]algorithm2.Entry, error) {
	computerID := strings.TrimSpace(h.Core.TextureComputerID())
	events, err := h.Store.ListChangelogEvents(ctx, ownerID, computerID, types.ChangelogQuery{IncludeMinor: true})
	if err != nil {
		return nil, err
	}
	if len(events) > algorithm2RevisionReads {
		events = events[len(events)-algorithm2RevisionReads:]
	}
	var out []algorithm2.Entry
	seen := map[string]bool{}
	titles := map[string]string{}
	title := func(docID string) string {
		if t, ok := titles[docID]; ok {
			return t
		}
		t := ""
		if d, err := h.getTextureDocument(asOwner(ctx), ownerID, docID); err == nil {
			t = d.Title
		}
		titles[docID] = t
		return t
	}
	for _, ev := range events {
		c := ev.Changelog
		at, _ := time.Parse(time.RFC3339Nano, c.OccurredAt)
		switch c.Kind {
		case types.ChangelogParentUsed:
			if c.Relation != types.ChangelogRelationForkPublished {
				continue
			}
			out = append(out, algorithm2.Entry{
				Kind: algorithm2.KindPublication, At: at, Ref: c.Subject.PublicationVersionID,
				Title: firstNonEmpty(c.Subject.RoutePath, c.Summary), Authors: nonEmptyStrings(c.ActorLabel),
			})
		case types.ChangelogRevisionCommitted, types.ChangelogUpdateStarted:
			if c.TapeRef == nil || c.TapeRef.RevisionID == "" || seen[c.TapeRef.RevisionID] {
				continue
			}
			rev, err := h.getTextureRevision(ctx, ownerID, c.TapeRef.RevisionID)
			if err != nil {
				continue
			}
			meta := decodeRevisionMetadata(rev.Metadata)
			var prov types.Provenance
			_ = json.Unmarshal(rev.Provenance, &prov)
			var authors []string
			sourced := false
			for _, l := range prov.ParentLinks {
				if l.Role == types.ParentLinkSource {
					sourced = true
					authors = append(authors, l.AuthorLabel)
				}
			}
			prompted := metadataString(meta, "owner_prompt") != ""
			var kind algorithm2.Kind
			switch {
			case meta["forked_from"] != nil:
				kind = algorithm2.KindFork
			case sourced && prompted:
				kind = algorithm2.KindResponse
			case sourced:
				kind = algorithm2.KindCitation
			case prompted && c.Kind == types.ChangelogUpdateStarted:
				kind = algorithm2.KindResponse
			default:
				continue
			}
			seen[c.TapeRef.RevisionID] = true
			out = append(out, algorithm2.Entry{
				Kind: kind, At: at, Ref: rev.RevisionID, Title: title(rev.DocID),
				Categories: metadataStringSlice(meta["categories"]), Entities: metadataStringSlice(meta["entities"]),
				Authors: authors,
			})
		}
	}
	return out, nil
}

func nonEmptyStrings(in ...string) []string {
	var out []string
	for _, s := range in {
		if strings.TrimSpace(s) != "" {
			out = append(out, s)
		}
	}
	return out
}

// algorithm2Complete is the round's one model call: the Texture desk's role
// through the owner's model policy, one provider call with no tools and no
// failover. There is no route here the policy does not already have.
func (h *Handler) algorithm2Complete(ctx context.Context, ownerID, system, user string) (string, error) {
	if h.algorithm2Model != nil {
		return h.algorithm2Model(ctx, ownerID, system, user)
	}
	if h.ModelPolicy == nil || h.Provider == nil {
		return "", errors.New("no model is configured on this computer")
	}
	role := deskmanifest.DocumentCustodianName()
	policy, err := h.ModelPolicy.Load(ctx, ownerID)
	if err != nil {
		return "", err
	}
	sel, _, _ := policy.Assign(policy.Resolve(role), "algorithm2:"+ownerID)
	msg, _ := json.Marshal(map[string]any{"role": "user", "content": user})
	resp, err := toolregistry.AsToolLoopProvider(h.Provider).CallWithTools(ctx, provideriface.ToolLoopRequest{
		Provider: sel.Provider, Model: sel.Model, ReasoningEffort: sel.ReasoningEffort,
		ConversationID: "algorithm2:" + ownerID, System: system, Messages: []json.RawMessage{msg},
		MaxTokens: provideriface.MaxInteractiveOutputTokensForSelection(sel, role),
	})
	if err != nil {
		return "", err
	}
	return resp.Text, nil
}

type algorithm2RoundResponse struct {
	Outcome    string `json:"outcome"`
	ProposalID string `json:"proposal_id,omitempty"`
	Decision   string `json:"decision,omitempty"`
	Applied    bool   `json:"applied"`
	Error      string `json:"error,omitempty"`
}

// runAlgorithm2Round makes at most one model call and, when the owner's rules
// in the algorithm2 document allow it, applies the proposal it produced. The
// rules are read from the algorithm2 document's head after the model answers.
func (h *Handler) runAlgorithm2Round(ctx context.Context, ownerID string, ids algorithm2DocIDs) (algorithm2RoundResponse, error) {
	h.algorithm2Mu.Lock()
	defer h.algorithm2Mu.Unlock()
	algo, algo2, err := h.findAlgorithmDocs(ctx, ownerID, ids)
	if err != nil {
		return algorithm2RoundResponse{}, refuseOwnerRevision(http.StatusNotFound, err.Error())
	}
	revID, algoText, err := h.headText(ctx, ownerID, algo)
	if err != nil {
		return algorithm2RoundResponse{}, refuseOwnerRevision(http.StatusNotFound, "algorithm document not readable")
	}
	_, algo2Text, err := h.headText(ctx, ownerID, algo2)
	if err != nil {
		return algorithm2RoundResponse{}, refuseOwnerRevision(http.StatusNotFound, "algorithm2 document not readable")
	}
	ledger, err := h.buildAlgorithm2Ledger(ctx, ownerID)
	if err != nil {
		return algorithm2RoundResponse{}, refuseOwnerRevision(http.StatusInternalServerError, "the ledger could not be read")
	}
	_, planned := h.Core.BootWasPlannedRestart()
	st := algorithm2.NewFileStore(h.Algorithm2Dir)
	callCtx, cancel := context.WithTimeout(ctx, algorithm2RoundTimeout)
	defer cancel()
	res, err := algorithm2.RunRound(callCtx, algorithm2.RoundInput{
		Now: h.algorithm2Clock(), ProcessStart: h.algorithm2ProcessStart, CrashBoot: !planned,
		Algorithm: algorithm2.Doc{ID: algo.DocID, RevisionID: revID, Text: algoText}, Algorithm2Text: algo2Text,
		Ledger: ledger, Store: st,
		Model: func(c context.Context, system, user string) (string, error) {
			return h.algorithm2Complete(c, ownerID, system, user)
		},
	})
	if err != nil {
		log.Printf("texture api: algorithm2 round: %v", err)
		return algorithm2RoundResponse{}, refuseOwnerRevision(http.StatusInternalServerError, "the round failed")
	}
	out := algorithm2RoundResponse{Outcome: string(res.Outcome)}
	if res.Err != nil {
		out.Error = res.Err.Error()
	}
	if res.Proposal == nil {
		return out, nil
	}
	out.ProposalID = res.Proposal.ID
	// The rules are the owner's words in the algorithm2 document as it is now.
	_, currentAlgo2, err := h.headText(ctx, ownerID, algo2)
	if err != nil {
		currentAlgo2 = ""
	}
	decision := algorithm2.Decide(algorithm2.ParseRules(currentAlgo2), res.Proposal.Changes)
	out.Decision = string(decision)
	if decision != algorithm2.DecisionAuto {
		return out, nil
	}
	if _, err := h.acceptAlgorithm2Proposal(ctx, ownerID, res.Proposal.ID, "rule"); err != nil {
		// A refused auto-accept leaves the proposal waiting for the owner.
		out.Decision = string(algorithm2.DecisionAsk)
		out.Error = err.Error()
		return out, nil
	}
	out.Applied = true
	return out, nil
}

// acceptAlgorithm2Proposal is the revise call with the proposal as its source.
// The client request id derives from the proposal id, so a retry replays.
func (h *Handler) acceptAlgorithm2Proposal(ctx context.Context, ownerID, id, by string) (algorithm2.Proposal, error) {
	st := algorithm2.NewFileStore(h.Algorithm2Dir)
	p, ok, err := st.Get(id)
	switch {
	case err != nil:
		return p, refuseOwnerRevision(http.StatusBadRequest, "not a proposal id")
	case !ok:
		return p, refuseOwnerRevision(http.StatusNotFound, "proposal not found")
	case p.State == algorithm2.StateAccepted:
		return p, nil
	case p.State != algorithm2.StatePending:
		return p, refuseOwnerRevision(http.StatusConflict, "the proposal is "+string(p.State))
	}
	grant := context.WithValue(ctx, proposalGrantKey{}, p.DocID)
	receipt, err := h.reviseDocument(grant, ReviseRequest{
		OwnerID: ownerID, DocID: p.DocID, BaseRevisionID: p.BaseRevisionID,
		Sources:         []ReviseSource{{PublicationVersionID: p.ID, ManifestHash: p.ManifestHash}},
		ClientRequestID: "algorithm2-accept:" + strings.TrimPrefix(p.ID, algorithm2.ProposalIDPrefix),
	})
	if err != nil {
		var refusal *ownerRevisionRefusal
		if errors.As(err, &refusal) && refusal.status == http.StatusConflict {
			_ = st.SetState(p.ID, algorithm2.StateStale, by, "", h.algorithm2Clock())
			return p, refuseOwnerRevision(http.StatusConflict, "the algorithm document changed since this proposal; it is stale and the next round will propose afresh")
		}
		return p, err
	}
	if err := st.SetState(p.ID, algorithm2.StateAccepted, by, receipt.RevisionID, h.algorithm2Clock()); err != nil {
		log.Printf("texture api: algorithm2 mark accepted %s: %v", p.ID, err)
	}
	p, _, _ = st.Get(p.ID)
	return p, nil
}

// HandleAlgorithm2 serves the owner's behavior ledger and algorithm2
// proposals. Every route is the owner's: an in-process desk is refused.
func (h *Handler) HandleAlgorithm2(w http.ResponseWriter, r *http.Request) {
	r = asOwnerRoute(r)
	if strings.TrimSpace(r.RemoteAddr) == "" {
		writeAPIJSON(w, http.StatusForbidden, apiError{Error: "the behavior ledger and algorithm2 are the owner's; only the owner reads or acts on them"})
		return
	}
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	if h == nil || h.Core == nil || h.Store == nil || strings.TrimSpace(h.Algorithm2Dir) == "" {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "algorithm2 has no home on this computer"})
		return
	}
	path := r.URL.Path
	switch {
	case path == algorithm2PathPrefix+"ledger" && r.Method == http.MethodGet:
		ledger, err := h.buildAlgorithm2Ledger(r.Context(), ownerID)
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "the ledger could not be read"})
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = io.WriteString(w, algorithm2.RenderOutline(ledger))
	case path == algorithm2PathPrefix+"proposals" && r.Method == http.MethodGet:
		list, err := algorithm2.NewFileStore(h.Algorithm2Dir).List()
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "proposals could not be read"})
			return
		}
		if list == nil {
			list = []algorithm2.Proposal{}
		}
		w.Header().Set("Cache-Control", "no-store")
		writeAPIJSON(w, http.StatusOK, map[string]any{"proposals": list})
	case path == algorithm2PathPrefix+"round" && r.Method == http.MethodPost:
		var ids algorithm2DocIDs
		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&ids); err != nil && !errors.Is(err, io.EOF) {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid request body"})
			return
		}
		out, err := h.runAlgorithm2Round(r.Context(), ownerID, ids)
		if err != nil {
			writeOwnerRevisionRefusal(w, err)
			return
		}
		writeAPIJSON(w, http.StatusOK, out)
	case strings.HasPrefix(path, algorithm2ProposalsPrefix) && r.Method == http.MethodPost:
		rest := strings.TrimPrefix(path, algorithm2ProposalsPrefix)
		id, action, ok := strings.Cut(rest, "/")
		if !ok || (action != "accept" && action != "decline") {
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "texture endpoint not found"})
			return
		}
		h.algorithm2Mu.Lock()
		defer h.algorithm2Mu.Unlock()
		if action == "decline" {
			st := algorithm2.NewFileStore(h.Algorithm2Dir)
			if err := st.SetState(id, algorithm2.StateDeclined, "owner", "", h.algorithm2Clock()); err != nil {
				writeAPIJSON(w, http.StatusConflict, apiError{Error: err.Error()})
				return
			}
			p, _, _ := st.Get(id)
			writeAPIJSON(w, http.StatusOK, p)
			return
		}
		p, err := h.acceptAlgorithm2Proposal(r.Context(), ownerID, id, "owner")
		if err != nil {
			writeOwnerRevisionRefusal(w, err)
			return
		}
		writeAPIJSON(w, http.StatusOK, p)
	default:
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "texture endpoint not found"})
	}
}
