package agentcore

import (
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"sort"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/release"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// releaseAPIError is a package route's refusal: an error and its reason.
type releaseAPIError struct {
	Error  string `json:"error"`
	Reason string `json:"reason,omitempty"`
}

// releasePrecommitments reads engineering's rule-settled precommitments for
// the assignment whose capsule froze the operation's bundle.
func (rt *Runtime) releasePrecommitments(r *http.Request, ownerID, computerID string, operation selfdev.Operation) ([]release.Precommitment, error) {
	if rt.store == nil || strings.TrimSpace(operation.TrajectoryID) == "" {
		return nil, nil
	}
	assignments, err := rt.store.ListEngineeringAssignments(r.Context(), ownerID, computerID, operation.TrajectoryID)
	if err != nil {
		return nil, err
	}
	var out []release.Precommitment
	for _, assignment := range assignments {
		if operation.CapsuleID == "" || assignment.Binding.CapsuleID != operation.CapsuleID {
			continue
		}
		records, err := rt.store.ListRulePrecommitRecords(r.Context(), ownerID, computerID, assignment.AssignmentID)
		if err != nil {
			return nil, err
		}
		for _, precommit := range records {
			resolve, err := rt.store.GetCommitmentRecord(r.Context(), ownerID, computerID, types.RuleResolveRecordID(precommit.RecordID))
			if err != nil {
				return nil, err
			}
			out = append(out, releasePrecommitment(precommit, resolve))
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].RecordID < out[j].RecordID })
	return out, nil
}

func releasePrecommitment(precommit types.CommitmentRecord, resolve *types.CommitmentRecord) release.Precommitment {
	out := release.Precommitment{RecordID: precommit.RecordID}
	if precommit.Precommit != nil {
		out.Question = precommit.Precommit.Question
		out.PYesPermille = int(math.Round(precommit.Precommit.YesProbability() * 1000))
		if precommit.Precommit.Rule != nil {
			out.Kind = string(precommit.Precommit.Rule.Kind)
		}
	}
	if resolve != nil && resolve.Resolve != nil {
		out.Settled, out.ResolveRecordID = resolve.Resolve.Verdict, resolve.RecordID
	}
	return out
}

// handleReleaseAdoptions serves the Library's package installs, runtime
// installs since S12 (docs/design/s12-runtime-install-build-2026-11-04.md):
//
//	POST /api/computers/<id>/self-development/adoptions        install or uninstall (owner act; the proxy supplies the tree)
//	GET  /api/computers/<id>/self-development/adoptions        list
//	GET  /api/computers/<id>/self-development/adoptions/<aid>  one
func (h *APIHandler) handleReleaseAdoptions(w http.ResponseWriter, r *http.Request, ownerID, computerID, adoption string) {
	switch {
	case adoption == "" && r.Method == http.MethodPost:
		var request adoptReleaseRequest
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, adoptionRequestMaxBytes))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&request); err != nil {
			writeAPIJSON(w, http.StatusBadRequest, releaseAPIError{Error: "invalid adoption request", Reason: "request_invalid"})
			return
		}
		if !isPackageReleaseRecord(request.Record) {
			// Software-release adoption (P8) is deleted: only a Library
			// package release installs (docs/design/package-install-2026-11-04.md).
			writeAPIJSON(w, http.StatusUnprocessableEntity, releaseAPIError{Error: "only a package release from the Library installs", Reason: "release_record_invalid"})
			return
		}
		view, created, refusal := h.rt.installPackage(r.Context(), ownerID, computerID, request)
		if refusal != nil {
			writeAPIJSON(w, refusal.status, releaseAPIError{Error: refusal.detail, Reason: refusal.reason})
			return
		}
		status := http.StatusOK
		if created {
			status = http.StatusCreated
		}
		writeAPIJSON(w, status, view)
	case adoption == "" && r.Method == http.MethodGet:
		// Newest first; a P8 software-release adoption from before its
		// deletion is not an install.
		list, err := h.rt.packageInstalls(r.Context(), ownerID, computerID)
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, releaseAPIError{Error: "adoptions unreadable"})
			return
		}
		views := make([]ReleaseAdoptionView, 0, len(list))
		for _, item := range list {
			views = append(views, h.rt.adoptionView(item, list))
		}
		writeAPIJSON(w, http.StatusOK, map[string]any{"adoptions": views})
	case adoption != "" && r.Method == http.MethodGet:
		item, err := h.rt.store.GetReleaseAdoption(r.Context(), ownerID, computerID, adoption)
		if errors.Is(err, store.ErrNotFound) {
			writeAPIJSON(w, http.StatusNotFound, releaseAPIError{Error: "adoption not found"})
			return
		}
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, releaseAPIError{Error: "adoption unreadable"})
			return
		}
		list, err := h.rt.packageInstalls(r.Context(), ownerID, computerID)
		if err != nil || item.Kind != adoptionKindPackage {
			writeAPIJSON(w, http.StatusNotFound, releaseAPIError{Error: "adoption not found"})
			return
		}
		writeAPIJSON(w, http.StatusOK, h.rt.adoptionView(item, list))
	default:
		writeAPIJSON(w, http.StatusMethodNotAllowed, releaseAPIError{Error: "method not allowed"})
	}
}
