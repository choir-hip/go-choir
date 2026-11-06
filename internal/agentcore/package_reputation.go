package agentcore

import (
	"context"
	"crypto/ed25519"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/installreview"
	"github.com/yusefmosiah/go-choir/internal/trust"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Publisher reputation in the Library (MA7.6, trust draft section 8):
//
//	GET /api/packages/reputation?publisher=<track>
//
// answers the publisher's record on this computer as track-record sentences,
// each with its n, never a score: the settled acts of its installed packages'
// desks (the trust extraction, placed under their publisher), the sealed
// install review verdicts on its releases, and the settled predictions of
// their seats. When the computer has its guest-core signer it also returns
// the same outcomes as a signed bundle, the form another computer or the
// Library aggregates (trust.Aggregate). Read-only; nothing is stored.

const (
	packagesReputationPath = "/api/packages/reputation"
	// reputationReviewWindow bounds the review records read per publisher.
	reputationReviewWindow = 2000
)

type reputationResponse struct {
	Publisher string           `json:"publisher"`
	Sentences []trust.Sentence `json:"sentences"`
	// N is the number of settled outcomes the sentences rest on.
	N      int                 `json:"n"`
	Signed *trust.SignedBundle `json:"signed,omitempty"`
}

func (h *APIHandler) servePackageReputation(w http.ResponseWriter, r *http.Request, ownerID, computerID string) {
	publisher := strings.TrimSpace(r.URL.Query().Get("publisher"))
	if publisher == "" || len(publisher) > 256 {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "publisher is required"})
		return
	}
	rt := h.rt
	outcomes, err := rt.publisherOutcomes(r.Context(), ownerID, computerID, publisher)
	if err != nil {
		log.Printf("runtime api: reputation: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "the ledger is unreadable"})
		return
	}
	resp := reputationResponse{Publisher: publisher, Sentences: trust.Sentences(outcomes), N: len(outcomes)}
	if rt.installReviewSigner != nil && len(outcomes) > 0 {
		signed, err := trust.SignBundle(r.Context(), rt.installReviewSigner, trust.OutcomeBundle{ComputerID: computerID, Publisher: publisher, Outcomes: outcomes}, time.Now())
		if err != nil {
			log.Printf("runtime api: reputation: sign: %v", err)
		} else {
			resp.Signed = &signed
		}
	}
	writeAPIJSON(w, http.StatusOK, resp)
}

// publisherOutcomes reads the publisher's settled outcomes on this computer.
func (rt *Runtime) publisherOutcomes(ctx context.Context, ownerID, computerID, publisher string) ([]trust.SettledOutcome, error) {
	in, err := rt.trustInputs(ctx, ownerID, computerID)
	if err != nil {
		return nil, err
	}
	in.Resolver = rt.installedPackageResolver(ctx, ownerID, computerID)
	outcomes := trust.PublisherOutcomes(in, trust.DefaultParams(), publisher)
	reviews, err := rt.reviewOutcomes(ctx, ownerID, computerID, publisher)
	if err != nil {
		return nil, err
	}
	return append(outcomes, reviews...), nil
}

// reviewOutcomes are the install reviews of the publisher's releases on this
// computer: each release's latest verdict that carries a valid seal, and each
// seat prediction that has a resolve. A verdict that fails its seal is not
// the publisher's record.
func (rt *Runtime) reviewOutcomes(ctx context.Context, ownerID, computerID, publisher string) ([]trust.SettledOutcome, error) {
	if rt.store == nil {
		return nil, nil
	}
	records, err := rt.store.ListCommitmentRecordsBySubjectRef(ctx, ownerID, computerID, installreview.RefPublisher, publisher, reputationReviewWindow)
	if err != nil {
		return nil, err
	}
	var pub ed25519.PublicKey
	if rt.installReviewSigner != nil {
		if _, key, err := rt.installReviewSigner.PublicKey(ctx); err == nil {
			pub = key
		}
	}
	latest := map[string]types.CommitmentRecord{}
	releases := map[string]bool{}
	seats := map[string]types.CommitmentRecord{}
	for _, rec := range records {
		release := rec.SubjectRefs[installreview.RefRelease]
		switch rec.SubjectRefs[installreview.RefRole] {
		case installreview.RoleVerdict:
			if pub == nil || installreview.VerifySeal(rec, computerID, pub) != nil {
				continue
			}
			key := release + "\x00" + rec.SubjectRefs[installreview.RefPatch]
			if cur, ok := latest[key]; !ok || rec.Provenance.CommittedAt > cur.Provenance.CommittedAt {
				latest[key] = rec
			}
		case installreview.RoleSeat:
			if strings.HasPrefix(rec.Provenance.AgentID, installreview.AgentID+":") {
				seats[rec.RecordID] = rec
				releases[release] = true
			}
		}
	}
	var out []trust.SettledOutcome
	for _, rec := range latest {
		out = append(out, trust.SettledOutcome{Kind: trust.OutcomeReview, RecordID: rec.RecordID, Publisher: publisher,
			Package: rec.SubjectRefs[installreview.RefPackage], Release: rec.SubjectRefs[installreview.RefRelease],
			Class:  trust.ActionClass{Grant: "package.install", Impact: types.ImpactClass(rec.SubjectRefs[installreview.RefRisk])},
			Within: rec.SubjectRefs[installreview.RefVerdict] == installreview.VerdictAccept, ReviewVerdict: rec.SubjectRefs[installreview.RefVerdict],
			SettledAt: rec.Provenance.CommittedAt})
	}
	// A seat prediction's settlement is a resolve addressed to its release's
	// review (the settler is MA7.8's frame-log rule; none is written yet).
	ids := make([]string, 0, len(releases))
	for id := range releases {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, release := range ids {
		addressed, err := rt.store.ListCommitmentRecords(ctx, ownerID, computerID, installreview.AddresseePrefix+release, reputationReviewWindow)
		if err != nil {
			return nil, err
		}
		for _, res := range addressed {
			seat, ok := seats[res.ParentID]
			if res.Resolve == nil || !ok || strings.HasPrefix(res.Provenance.AgentID, installreview.AgentID) {
				continue
			}
			out = append(out, trust.SettledOutcome{Kind: trust.OutcomeReviewPrediction, RecordID: res.RecordID, Publisher: publisher,
				Package: seat.SubjectRefs[installreview.RefPackage], Release: release,
				Class:  trust.ActionClass{Grant: "package.install", Impact: types.ImpactClass(seat.SubjectRefs[installreview.RefRisk])},
				Within: res.Resolve.Verdict == "yes", SettledAt: res.Provenance.CommittedAt})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].RecordID < out[j].RecordID })
	return out, nil
}
