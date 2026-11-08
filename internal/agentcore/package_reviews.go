package agentcore

import (
	"crypto/ed25519"
	"log"
	"net/http"
	"regexp"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/installreview"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The owner's read of an install review:
//
//	GET /api/packages/reviews?release=<sha256>
//
// answers the commitment records on the release's review (the sealed verdict,
// management's seat record, any resolve), and the latest verdict with its
// seal checked and the seat that backs it. The verdict and the seat are
// commitment-ledger records, not inbox rows, so no inbox route shows them.
// Read-only; scoped to the owner and computer by the ledger read.

const packagesReviewsPath = "/api/packages/reviews"

var releaseIDPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

type reviewSeatView struct {
	RecordID string `json:"record_id"`
	Seat     string `json:"seat"`
	Verdict  string `json:"verdict"`
	// ServedModel is the model that served the seat's cell.
	ServedModel    string  `json:"served_model,omitempty"`
	PWithinCeiling float64 `json:"p_within_ceiling"`
	Findings       string  `json:"findings,omitempty"`
	Reason         string  `json:"reason,omitempty"`
	Hypothesis     string  `json:"hypothesis"`
	CommittedAt    string  `json:"committed_at"`
	ResolvesBy     string  `json:"resolves_by,omitempty"`
}

type reviewVerdictView struct {
	RecordID    string           `json:"record_id"`
	Verdict     string           `json:"verdict"`
	Reason      string           `json:"reason,omitempty"`
	Hypothesis  string           `json:"hypothesis"`
	CommittedAt string           `json:"committed_at"`
	SealValid   bool             `json:"seal_valid"`
	SealError   string           `json:"seal_error,omitempty"`
	Seats       []reviewSeatView `json:"seats"`
}

type reviewsResponse struct {
	Release string                   `json:"release"`
	Records []types.CommitmentRecord `json:"records"`
	Verdict *reviewVerdictView       `json:"verdict"`
}

func (h *APIHandler) servePackageReviews(w http.ResponseWriter, r *http.Request, ownerID, computerID string) {
	release := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("release")))
	if !releaseIDPattern.MatchString(release) {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "release is required: a sha256 release id"})
		return
	}
	rt := h.rt
	records, err := rt.store.ListCommitmentRecords(r.Context(), ownerID, computerID, installreview.AddresseePrefix+release, installReviewRecordWindow)
	if err != nil {
		log.Printf("runtime api: reviews: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "the ledger is unreadable"})
		return
	}
	if records == nil {
		records = []types.CommitmentRecord{}
	}
	resp := reviewsResponse{Release: release, Records: records}
	var pub ed25519.PublicKey
	if rt.installReviewSigner != nil {
		if _, key, err := rt.installReviewSigner.PublicKey(r.Context()); err == nil {
			pub = key
		}
	}
	byID := map[string]types.CommitmentRecord{}
	var latest *types.CommitmentRecord
	for i := range records {
		rec := records[i]
		byID[rec.RecordID] = rec
		if rec.SubjectRefs[installreview.RefRole] == installreview.RoleVerdict && rec.Provenance.AgentID == installreview.AgentID {
			if latest == nil || rec.Provenance.CommittedAt > latest.Provenance.CommittedAt {
				latest = &records[i]
			}
		}
	}
	if latest != nil {
		v := &reviewVerdictView{RecordID: latest.RecordID, Verdict: latest.SubjectRefs[installreview.RefVerdict],
			Hypothesis: latest.Prediction.Hypothesis, CommittedAt: latest.Provenance.CommittedAt, Seats: []reviewSeatView{}}
		if _, after, ok := strings.Cut(v.Hypothesis, "; management: "); ok {
			v.Reason = after
		}
		if pub == nil {
			v.SealError = "no verdict signer"
		} else if err := installreview.VerifySeal(*latest, computerID, pub); err != nil {
			v.SealError = err.Error()
		} else {
			v.SealValid = true
		}
		for _, id := range latest.ChildIDs {
			if seat, ok := byID[id]; ok && seat.SubjectRefs[installreview.RefRole] == installreview.RoleSeat {
				v.Seats = append(v.Seats, reviewSeatOf(seat))
			}
		}
		resp.Verdict = v
	}
	writeAPIJSON(w, http.StatusOK, resp)
}

func reviewSeatOf(rec types.CommitmentRecord) reviewSeatView {
	s := reviewSeatView{RecordID: rec.RecordID, Seat: rec.SubjectRefs[installreview.RefSeat], Verdict: rec.SubjectRefs[installreview.RefVerdict],
		ServedModel: rec.Provenance.ModelID, Hypothesis: rec.Prediction.Hypothesis, CommittedAt: rec.Provenance.CommittedAt,
		ResolvesBy: rec.SubjectRefs[installreview.RefResolvesBy]}
	if rec.Precommit != nil {
		s.PWithinCeiling = rec.Precommit.Distribution["yes"]
	}
	rest := s.Hypothesis
	if head, reason, ok := strings.Cut(rest, "; reason: "); ok {
		rest, s.Reason = head, reason
	}
	if _, findings, ok := strings.Cut(rest, "; findings: "); ok {
		s.Findings = findings
	}
	return s
}
