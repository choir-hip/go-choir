package platform

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/quotematch"
	"github.com/yusefmosiah/go-choir/internal/texturewrite"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The check overlay in Store B (Cut 2 2b). Public reads never reach a paper
// computer, so the overlay a bundle serves is read from two Store B object
// kinds, both outside every manifest:
//
//   - choir.publication_check, one per (version, citation): the quote
//     check the revision committed with (texture_citation_checks: outcome,
//     tier, support pair key), written when the version publishes;
//   - choir.support_judgement, one per (owner, pair key): the support
//     verdict management recorded on the computer, pushed here as it
//     arrives, before or after any version that cites the pair.
//
// The overlay joins them by pair key on every read, so a verdict arriving
// after publish updates the overlay and never the manifest.

const (
	ogKindPublicationCheck = objectgraph.ObjectKind("choir.publication_check")
	ogKindSupportJudgement = objectgraph.ObjectKind("choir.support_judgement")
	storeOverlaySource     = "store"
	maxVersionChecks       = 2000
	maxVerdictsPerRequest  = 500
)

// publicationCheck is one version citation's quote check.
type publicationCheck struct {
	PublicationVersionID string `json:"publication_version_id"`
	AnchorID             string `json:"anchor_id"`
	PairKey              string `json:"pair_key,omitempty"`
	Outcome              string `json:"outcome"`
	Tier                 string `json:"tier,omitempty"`
}

// supportJudgement is one pair's support verdict as published.
type supportJudgement struct {
	PairKey  string `json:"pair_key"`
	Verdict  string `json:"verdict"`
	Why      string `json:"why,omitempty"`
	Judge    string `json:"judge"`
	JudgedAt string `json:"judged_at"`
}

func checkObject(kind objectgraph.ObjectKind, ownerID, key string, body any, metadata map[string]any, now time.Time) (objectgraph.Object, error) {
	canonicalID, err := objectgraph.BuildCanonicalID(kind, ownerID, objectgraph.StableSuffixFromKey(key))
	if err != nil {
		return objectgraph.Object{}, err
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return objectgraph.Object{}, err
	}
	meta, err := objectgraph.NormalizeMetadata(metadata)
	if err != nil {
		return objectgraph.Object{}, err
	}
	return objectgraph.Object{
		CanonicalID: canonicalID, ObjectKind: kind, OwnerID: ownerID,
		ContentHash: objectgraph.ContentHash(kind, raw, meta), Body: raw, Metadata: meta,
		CreatedAt: now, UpdatedAt: now,
	}, nil
}

// publicationCheckID is the ID of a version's check of one anchor, as
// writeVersionChecks stores it.
func publicationCheckID(ownerID, versionID, anchorID string) string {
	id, err := objectgraph.BuildCanonicalID(ogKindPublicationCheck, ownerID, objectgraph.StableSuffixFromKey(versionID+"\x00"+anchorID))
	if err != nil {
		return ""
	}
	return id
}

// supportJudgementID is the ID of an owner's judgement of one pair.
func supportJudgementID(ownerID, pairKey string) string {
	id, err := objectgraph.BuildCanonicalID(ogKindSupportJudgement, ownerID, objectgraph.StableSuffixFromKey(pairKey))
	if err != nil {
		return ""
	}
	return id
}

// overlayScans counts overlays that fell back to scanning every check row
// by JSON path; a resolve with a known owner reads by ID instead.
var overlayScans atomic.Int64

// writeVersionChecks stores the quote check of each anchor the version
// carries, read from the revision's texture_citation_checks.
func (s *Service) writeVersionChecks(ctx context.Context, ownerID, versionID string, anchors []PublicationAnchor, revisionMetadata json.RawMessage, now time.Time) error {
	if len(anchors) == 0 {
		return nil
	}
	checks := texturewrite.ChecksFrom(types.Revision{Metadata: revisionMetadata})
	byRef := make(map[string]texturewrite.QuoteCheck, len(checks))
	for _, c := range checks {
		byRef[c.RefID] = c
	}
	og := s.ogStore()
	var errs []error
	for _, a := range anchors {
		c, ok := byRef[a.AnchorID]
		if !ok {
			continue
		}
		row := publicationCheck{PublicationVersionID: versionID, AnchorID: a.AnchorID, PairKey: c.Pair, Outcome: string(c.Outcome), Tier: string(c.Tier)}
		obj, err := checkObject(ogKindPublicationCheck, ownerID, versionID+"\x00"+a.AnchorID, row, map[string]any{
			"publication_version_id": versionID, "anchor_id": a.AnchorID, "pair_key": c.Pair,
		}, now)
		if err == nil {
			err = og.PutObject(ctx, obj)
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("anchor %s: %w", a.AnchorID, err))
		}
	}
	return errors.Join(errs...)
}

// RecordSupportVerdicts stores a computer's support verdicts in Store B. A
// pair's later verdict replaces its earlier one.
func (s *Service) RecordSupportVerdicts(ctx context.Context, req types.StoryVerdictsRequest) error {
	ownerID := strings.TrimSpace(req.OwnerID)
	if ownerID == "" {
		return fmt.Errorf("owner_id is required")
	}
	if len(req.Verdicts) == 0 || len(req.Verdicts) > maxVerdictsPerRequest {
		return fmt.Errorf("a verdicts request carries 1 to %d verdicts", maxVerdictsPerRequest)
	}
	now := time.Now().UTC()
	objects := make([]objectgraph.Object, 0, len(req.Verdicts))
	for _, v := range req.Verdicts {
		key := strings.TrimSpace(v.Key)
		if key == "" || strings.TrimSpace(v.Judge) == "" || (v.Verdict != types.SupportSupported && v.Verdict != types.SupportUnsupported) {
			return fmt.Errorf("verdict %q needs a key, a judge and a verdict of %s or %s", key, types.SupportSupported, types.SupportUnsupported)
		}
		obj, err := checkObject(ogKindSupportJudgement, ownerID, key, supportJudgement{
			PairKey: key, Verdict: v.Verdict, Why: strings.TrimSpace(v.Why), Judge: strings.TrimSpace(v.Judge), JudgedAt: now.Format(time.RFC3339Nano),
		}, map[string]any{"pair_key": key, "verdict": v.Verdict, "computer_id": strings.TrimSpace(req.ComputerID)}, now)
		if err != nil {
			return err
		}
		objects = append(objects, obj)
	}
	og := s.ogStore()
	for _, obj := range objects {
		if err := og.PutObject(ctx, obj); err != nil {
			return err
		}
	}
	return nil
}

// HandleSupportVerdicts serves POST types.StoryVerdictsPath for a computer
// pushing its stories' verdicts (straight from the guest, whose platform URL
// is corpusd, or through the proxy) under the one bound-guest write rule.
func (h *Handler) HandleSupportVerdicts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	var req types.StoryVerdictsRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid request body"})
		return
	}
	if err := requireInternalOrBoundGuestWrite(r.Context(), h.guestLookup, r, req.OwnerID, req.ComputerID); err != nil {
		writeJSON(w, http.StatusForbidden, apiError{Error: err.Error()})
		return
	}
	if err := h.service.RecordSupportVerdicts(r.Context(), req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"recorded": len(req.Verdicts)})
}

// StoreCheckOverlay serves a version's overlay from Store B alone.
type StoreCheckOverlay struct {
	svc *Service
}

// NewStoreCheckOverlay is the overlay corpusd installs.
func NewStoreCheckOverlay(svc *Service) *StoreCheckOverlay { return &StoreCheckOverlay{svc: svc} }

func (o *StoreCheckOverlay) CheckOverlay(ctx context.Context, version CheckOverlayVersion, citations []CheckOverlayCitation) (PublicationCheckOverlay, error) {
	og := o.svc.ogStore()
	if og == nil {
		return PublicationCheckOverlay{}, fmt.Errorf("check overlay: store unavailable")
	}
	type checkRow struct {
		owner string
		check publicationCheck
	}
	byAnchor := map[string]checkRow{}
	addRow := func(obj objectgraph.Object) {
		var c publicationCheck
		if json.Unmarshal(obj.Body, &c) == nil && c.PublicationVersionID == version.PublicationVersionID {
			byAnchor[c.AnchorID] = checkRow{owner: obj.OwnerID, check: c}
		}
	}
	if owner := strings.TrimSpace(version.OwnerID); owner != "" {
		// The checks are keyed by owner, version and anchor: read exactly
		// those rows, never the whole kind
		// (docs/archive/problems/article-open-slow-store-b-scans-2026-10-16.md).
		ids := make([]string, 0, len(citations))
		for _, cite := range citations {
			ids = append(ids, publicationCheckID(owner, version.PublicationVersionID, cite.AnchorID))
		}
		objs, err := og.GetObjects(ctx, ids)
		if err != nil {
			return PublicationCheckOverlay{}, err
		}
		for _, obj := range objs {
			addRow(obj)
		}
	} else {
		overlayScans.Add(1)
		rows, err := og.ListObjectsByMetadata(ctx, string(ogKindPublicationCheck), "$.publication_version_id", version.PublicationVersionID, maxVersionChecks)
		if err != nil {
			return PublicationCheckOverlay{}, err
		}
		for _, obj := range rows {
			addRow(obj)
		}
	}
	judgementIDs := make([]string, 0, len(byAnchor))
	for _, row := range byAnchor {
		if row.check.PairKey != "" {
			judgementIDs = append(judgementIDs, supportJudgementID(row.owner, row.check.PairKey))
		}
	}
	judgements, err := og.GetObjects(ctx, judgementIDs)
	if err != nil {
		return PublicationCheckOverlay{}, err
	}
	out := PublicationCheckOverlay{Source: storeOverlaySource, Citations: make([]CitationCheck, 0, len(citations))}
	for _, cite := range citations {
		entry := CitationCheck{AnchorID: cite.AnchorID, Ref: cite.Ref, State: CheckPending, Quote: CheckPending, Judgement: CheckPending}
		row, ok := byAnchor[cite.AnchorID]
		if ok {
			entry.Quote = quoteState(row.check)
			if obj, found := judgements[supportJudgementID(row.owner, row.check.PairKey)]; found && row.check.PairKey != "" {
				var judgement supportJudgement
				if err := json.Unmarshal(obj.Body, &judgement); err != nil {
					return PublicationCheckOverlay{}, err
				}
				entry.Judgement, entry.Detail, entry.CheckedAt = judgementState(judgement.Verdict), judgement.Why, judgement.JudgedAt
			}
		}
		entry.State = badgeState(entry.Quote, entry.Judgement)
		out.Citations = append(out.Citations, entry)
	}
	return out, nil
}

// quoteState maps a committed quote check to the overlay: a matched quote
// is exact or tolerant by its tier, a quote not found failed, a source never
// captured unread, anything else pending.
func quoteState(c publicationCheck) string {
	switch quotematch.Outcome(c.Outcome) {
	case quotematch.Matched:
		if quotematch.Tier(c.Tier) == quotematch.TierExact || c.Tier == "" {
			return CheckExact
		}
		return CheckTolerant
	case quotematch.NotFound:
		return CheckFailed
	case quotematch.NotCaptured:
		return CheckUnread
	}
	return CheckPending
}

func judgementState(verdict string) string {
	switch verdict {
	case types.SupportSupported:
		return types.SupportSupported
	case types.SupportUnsupported:
		return CheckFailed
	}
	return CheckPending
}

// badgeState is the one badge a reader sees: failed if the quote or the
// support failed, unread if the source was never captured, pending until
// both are known, else the quote's tier.
func badgeState(quote, judgement string) string {
	switch {
	case quote == CheckFailed || judgement == CheckFailed:
		return CheckFailed
	case quote == CheckUnread:
		return CheckUnread
	case quote == CheckPending || judgement == CheckPending:
		return CheckPending
	}
	return quote
}
