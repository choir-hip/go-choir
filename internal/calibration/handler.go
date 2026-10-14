package calibration

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// Ledger is the slice of the guest store the routes use.
type Ledger interface {
	AppendCommitmentRecord(ctx context.Context, ownerID, computerID string, rec types.CommitmentRecord) (string, error)
	GetCommitmentRecord(ctx context.Context, ownerID, computerID, recordID string) (*types.CommitmentRecord, error)
	ListCalibrationRecords(ctx context.Context, ownerID, computerID, agent string, limit int) ([]types.CommitmentRecord, error)
}

// DefaultMaxRecords bounds how many calibration records one read joins.
const DefaultMaxRecords = 10000

const (
	routePrefix     = "/api/precommit/"
	maxBodyBytes    = 64 << 10
	defaultListSize = 50
	maxListSize     = 1000
)

// Handler serves /api/precommit/. Authenticate returns the owner (the
// proxy-authenticated user); ComputerID names this computer.
type Handler struct {
	Ledger       Ledger
	ComputerID   func() string
	Authenticate func(*http.Request) (string, error)
	Now          func() time.Time
	MaxRecords   int
}

// ListResponse is GET /api/precommit/claims.
type ListResponse struct {
	Claims    []ClaimView `json:"claims"`
	Filters   Filters     `json:"filters"`
	Status    string      `json:"status"`
	Truncated bool        `json:"truncated"`
}

type claimResponse struct {
	Claim ClaimView `json:"claim"`
}

type errorBody struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("precommit api: json encode: %v", err)
	}
}

func (h *Handler) now() time.Time {
	if h.Now != nil {
		return h.Now().UTC()
	}
	return time.Now().UTC()
}

func (h *Handler) maxRecords() int {
	if h.MaxRecords > 0 {
		return h.MaxRecords
	}
	return DefaultMaxRecords
}

func randomHex() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	return hex.EncodeToString(b[:])
}

type scope struct {
	owner, computer string
}

// ServeHTTP routes /api/precommit/claims, /claims/{id},
// /claims/{id}/settlements and /calibration. There is no edit route.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Ledger == nil || h.ComputerID == nil || h.Authenticate == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorBody{Error: "precommit store unavailable"})
		return
	}
	owner, err := h.Authenticate(r)
	if err != nil || strings.TrimSpace(owner) == "" {
		writeJSON(w, http.StatusUnauthorized, errorBody{Error: "authentication required"})
		return
	}
	sc := scope{owner: strings.TrimSpace(owner), computer: strings.TrimSpace(h.ComputerID())}
	if sc.computer == "" {
		writeJSON(w, http.StatusServiceUnavailable, errorBody{Error: "computer identity unavailable"})
		return
	}
	rest := strings.TrimPrefix(r.URL.Path, routePrefix)
	switch {
	case rest == "claims":
		switch r.Method {
		case http.MethodGet:
			h.list(w, r, sc)
		case http.MethodPost:
			h.create(w, r, sc)
		default:
			writeJSON(w, http.StatusMethodNotAllowed, errorBody{Error: "method not allowed"})
		}
	case rest == "calibration":
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, errorBody{Error: "method not allowed"})
			return
		}
		h.calibration(w, r, sc)
	case strings.HasPrefix(rest, "claims/"):
		id := strings.TrimPrefix(rest, "claims/")
		if claimID, ok := strings.CutSuffix(id, "/settlements"); ok {
			if r.Method != http.MethodPost {
				writeJSON(w, http.StatusMethodNotAllowed, errorBody{Error: "method not allowed: settlements are appended, never edited"})
				return
			}
			h.settle(w, r, sc, claimID)
			return
		}
		if strings.Contains(id, "/") || strings.TrimSpace(id) == "" {
			writeJSON(w, http.StatusNotFound, errorBody{Error: "not found"})
			return
		}
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, errorBody{Error: "method not allowed: claims are never edited; append a settlement"})
			return
		}
		h.get(w, r, sc, id)
	default:
		writeJSON(w, http.StatusNotFound, errorBody{Error: "not found"})
	}
}

func decodeBody(r *http.Request, out any) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, maxBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		return invalid("invalid JSON body: %v", err)
	}
	return nil
}

func (h *Handler) fail(w http.ResponseWriter, err error) {
	var verr *ValidationError
	if errors.As(err, &verr) {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: verr.Error()})
		return
	}
	log.Printf("precommit api: %v", err)
	writeJSON(w, http.StatusInternalServerError, errorBody{Error: "precommit store error"})
}

// view joins one claim with its settlements.
func (h *Handler) view(ctx context.Context, sc scope, claim types.CommitmentRecord) (ClaimView, error) {
	records, err := h.Ledger.ListCalibrationRecords(ctx, sc.owner, sc.computer, claim.Calibration.Agent, h.maxRecords())
	if err != nil {
		return ClaimView{}, err
	}
	for _, v := range JoinClaims(append(records, claim), h.now()) {
		if v.ID == claim.RecordID {
			return v, nil
		}
	}
	return claimView(claim, nil, h.now()), nil
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request, sc scope) {
	ctx := r.Context()
	var req ClaimRequest
	if err := decodeBody(r, &req); err != nil {
		h.fail(w, err)
		return
	}
	rec, err := NewClaimRecord(req, h.now(), randomHex())
	if err != nil {
		h.fail(w, err)
		return
	}
	rec.ComputerID = sc.computer
	status := http.StatusCreated
	existing, err := h.Ledger.GetCommitmentRecord(ctx, sc.owner, sc.computer, rec.RecordID)
	if err != nil {
		h.fail(w, err)
		return
	}
	if existing == nil {
		if _, err := h.Ledger.AppendCommitmentRecord(ctx, sc.owner, sc.computer, rec); err != nil {
			h.fail(w, err)
			return
		}
		// The append is conditional on not-exists; re-read to learn whether
		// this write or a concurrent one landed.
		if existing, err = h.Ledger.GetCommitmentRecord(ctx, sc.owner, sc.computer, rec.RecordID); err != nil || existing == nil {
			h.fail(w, errors.Join(errors.New("claim not readable after append"), err))
			return
		}
		if existing.Provenance.CommittedAt != rec.Provenance.CommittedAt {
			status = http.StatusOK
		}
	} else {
		status = http.StatusOK
	}
	if !SameClaim(*existing, rec) {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "idempotency key already used for a different claim; claims are never edited", "claim_id": existing.RecordID})
		return
	}
	v, err := h.view(ctx, sc, *existing)
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, status, claimResponse{Claim: v})
}

func (h *Handler) loadClaim(ctx context.Context, sc scope, id string) (*types.CommitmentRecord, error) {
	rec, err := h.Ledger.GetCommitmentRecord(ctx, sc.owner, sc.computer, strings.TrimSpace(id))
	if err != nil || !IsClaim(rec) {
		return nil, err
	}
	return rec, nil
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request, sc scope, id string) {
	claim, err := h.loadClaim(r.Context(), sc, id)
	if err != nil {
		h.fail(w, err)
		return
	}
	if claim == nil {
		writeJSON(w, http.StatusNotFound, errorBody{Error: "no calibration claim " + id})
		return
	}
	v, err := h.view(r.Context(), sc, *claim)
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, claimResponse{Claim: v})
}

func (h *Handler) settle(w http.ResponseWriter, r *http.Request, sc scope, id string) {
	ctx := r.Context()
	claim, err := h.loadClaim(ctx, sc, id)
	if err != nil {
		h.fail(w, err)
		return
	}
	if claim == nil {
		writeJSON(w, http.StatusNotFound, errorBody{Error: "no calibration claim " + id})
		return
	}
	var req SettlementRequest
	if err := decodeBody(r, &req); err != nil {
		h.fail(w, err)
		return
	}
	rec, err := NewSettlementRecord(*claim, req, h.now())
	if err != nil {
		h.fail(w, err)
		return
	}
	rec.ComputerID = sc.computer
	status := http.StatusCreated
	existing, err := h.Ledger.GetCommitmentRecord(ctx, sc.owner, sc.computer, rec.RecordID)
	if err != nil {
		h.fail(w, err)
		return
	}
	if existing == nil {
		if _, err := h.Ledger.AppendCommitmentRecord(ctx, sc.owner, sc.computer, rec); err != nil {
			h.fail(w, err)
			return
		}
		if existing, err = h.Ledger.GetCommitmentRecord(ctx, sc.owner, sc.computer, rec.RecordID); err != nil || existing == nil {
			h.fail(w, errors.Join(errors.New("settlement not readable after append"), err))
			return
		}
		if existing.Provenance.ResolvedAt != rec.Provenance.ResolvedAt {
			status = http.StatusOK
		}
	} else {
		status = http.StatusOK
	}
	if !SameSettlement(*existing, rec) {
		writeJSON(w, http.StatusConflict, map[string]any{
			"error":         "this scorer already settled the claim differently; settlements are never edited (another scorer may append)",
			"settlement_id": existing.RecordID,
		})
		return
	}
	v, err := h.view(ctx, sc, *claim)
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, status, claimResponse{Claim: v})
}

func filtersFrom(r *http.Request) Filters {
	q := r.URL.Query()
	return Filters{
		Agent: strings.TrimSpace(q.Get("agent")), Model: strings.TrimSpace(q.Get("model")),
		Type: strings.TrimSpace(q.Get("type")), Project: strings.TrimSpace(q.Get("project")),
	}
}

func (h *Handler) claims(ctx context.Context, sc scope, f Filters) ([]ClaimView, bool, error) {
	max := h.maxRecords()
	records, err := h.Ledger.ListCalibrationRecords(ctx, sc.owner, sc.computer, f.Agent, max)
	if err != nil {
		return nil, false, err
	}
	var out []ClaimView
	for _, c := range JoinClaims(records, h.now()) {
		if f.Match(c) {
			out = append(out, c)
		}
	}
	return out, len(records) >= max, nil
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request, sc scope) {
	f := filtersFrom(r)
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	if status == "" {
		status = "all"
	}
	switch status {
	case "all", StatusOpen, StatusOverdue, StatusSettled:
	default:
		h.fail(w, invalid("status must be open, overdue, settled or all"))
		return
	}
	limit := defaultListSize
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 {
			h.fail(w, invalid("limit must be a positive integer"))
			return
		}
		limit = min(n, maxListSize)
	}
	claims, truncated, err := h.claims(r.Context(), sc, f)
	if err != nil {
		h.fail(w, err)
		return
	}
	resp := ListResponse{Claims: []ClaimView{}, Filters: f, Status: status, Truncated: truncated}
	for _, c := range claims {
		keep := status == "all" || c.Status == status || (status == StatusOpen && c.Status == StatusOverdue)
		if keep && len(resp.Claims) < limit {
			resp.Claims = append(resp.Claims, c)
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

// ParseGroupBy validates a comma-separated group_by value.
func ParseGroupBy(raw string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var by []string
	for _, k := range strings.Split(raw, ",") {
		k = strings.TrimSpace(k)
		if !GroupKeys[k] {
			return nil, invalid("group_by keys are agent, model, type and project")
		}
		by = append(by, k)
	}
	return by, nil
}

func (h *Handler) calibration(w http.ResponseWriter, r *http.Request, sc scope) {
	q := r.URL.Query()
	f := filtersFrom(r)
	scorer := strings.TrimSpace(q.Get("scorer"))
	switch scorer {
	case "", "governing", types.CalibrationScorerSelf, types.CalibrationScorerIndependent, types.CalibrationScorerRule, types.CalibrationScorerOwner:
	default:
		h.fail(w, invalid("scorer must be governing, self, independent, rule or owner"))
		return
	}
	by, err := ParseGroupBy(q.Get("group_by"))
	if err != nil {
		h.fail(w, err)
		return
	}
	include := false
	if raw := strings.TrimSpace(q.Get("include_reconstructed")); raw != "" {
		if include, err = strconv.ParseBool(raw); err != nil {
			h.fail(w, invalid("include_reconstructed must be true or false"))
			return
		}
	}
	claims, truncated, err := h.claims(r.Context(), sc, f)
	if err != nil {
		h.fail(w, err)
		return
	}
	sum := Summarize(claims, SummaryOptions{Scorer: scorer, GroupBy: by, IncludeReconstructed: include, Filters: f}, h.now())
	sum.Truncated = truncated
	writeJSON(w, http.StatusOK, sum)
}
