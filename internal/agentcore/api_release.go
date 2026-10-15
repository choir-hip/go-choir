package agentcore

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"path/filepath"
	"sort"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/release"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/updater"
)

// releaseDraftResponse is the read-only answer of
// GET /api/computers/<id>/self-development/operations/<op>/release-draft:
// the release record this computer would publish for an applied operation,
// with its patch (P7). The proxy publishes it on the owner's act; nothing
// here writes.
type releaseDraft struct {
	ReleaseID   string          `json:"release_id"`
	Record      json.RawMessage `json:"record"`
	PatchSHA256 string          `json:"patch_sha256"`
	PatchBytes  int             `json:"patch_bytes"`
	PatchBase64 string          `json:"patch_base64"`
}

type releaseAPIError struct {
	Error  string `json:"error"`
	Reason string `json:"reason,omitempty"`
}

// releaseDraftFor builds the record for an applied operation: its bound
// bundle and patch, the patched tree, its receipts, engineering's settled
// precommitments and, when the applied release is layered, the host build's
// digests. It refuses an unapplied operation and a secret-bearing patch (R5).
func (h *APIHandler) releaseDraft(w http.ResponseWriter, r *http.Request, ownerID, computerID, operationID string) {
	ctx := r.Context()
	operation, err := h.rt.selfdevOperations.Get(ctx, computerID, operationID)
	if err != nil {
		writeAPIJSON(w, http.StatusNotFound, releaseAPIError{Error: "self-development operation not found"})
		return
	}
	if operation.State != selfdev.StateApplied {
		writeAPIJSON(w, http.StatusConflict, releaseAPIError{Error: "operation is " + operation.State + "; only an applied operation is published", Reason: "operation_not_applied"})
		return
	}
	if strings.TrimSpace(h.rt.selfdevUpdaterRoot) == "" || !computerevent.IsSHA256(operation.BundleDigest) {
		writeAPIJSON(w, http.StatusServiceUnavailable, releaseAPIError{Error: "self-development bundle store unavailable"})
		return
	}
	bundleRoot := filepath.Join(h.rt.selfdevUpdaterRoot, "incoming", operation.BundleDigest)
	bundle, status, err := readBoundSelfDevelopmentBundle(bundleRoot, operation)
	if err != nil {
		writeAPIJSON(w, status, releaseAPIError{Error: err.Error()})
		return
	}
	if bundle.SourcePatchSHA256 == "" || strings.TrimSpace(bundle.SourcePatchBaseCommit) == "" {
		writeAPIJSON(w, http.StatusConflict, releaseAPIError{Error: "the operation changed no workspace/platform source, or its bundle names no base commit; a release is source", Reason: "no_source_patch"})
		return
	}
	patch, status, err := readBoundSourcePatch(filepath.Join(bundleRoot, "source.patch"), bundle.SourcePatchSHA256)
	if err != nil {
		writeAPIJSON(w, status, releaseAPIError{Error: err.Error()})
		return
	}
	if kinds := sourcePatchAddedSecrets(patch); len(kinds) > 0 {
		writeAPIJSON(w, http.StatusUnprocessableEntity, releaseAPIError{Error: "source patch refused: added lines carry secret content (" + strings.Join(kinds, ", ") + ")", Reason: "patch_secret_refused"})
		return
	}
	tree, err := release.PatchedTree(ctx, h.rt.releaseRepo(), bundle.SourcePatchBaseCommit, patch)
	if err != nil {
		writeAPIJSON(w, http.StatusConflict, releaseAPIError{Error: "the patched tree cannot be computed on this computer: " + err.Error(), Reason: "tree_unavailable"})
		return
	}
	precommitments, err := h.rt.releasePrecommitments(r, ownerID, computerID, operation)
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, releaseAPIError{Error: "precommitments unreadable: " + err.Error()})
		return
	}
	record := release.Record{
		Schema:    release.Schema,
		Publisher: release.Publisher{ComputerID: computerID, OwnerID: ownerID},
		Operation: release.Operation{
			OperationID: operation.OperationID, BundleDigest: operation.BundleDigest, ReleaseDigest: operation.ReleaseDigest,
			CodeRef: operation.CodeRef, DecisionEvent: operation.DecisionEvent, MaterializationReceipt: operation.MaterializationReceipt,
			CheckpointRef: operation.CheckpointRef, RouteCertificate: operation.RouteCertificate, VerifierRefs: operation.VerifierRefs,
		},
		Source: release.Source{
			BaseCommit: bundle.SourcePatchBaseCommit, PatchSHA256: bundle.SourcePatchSHA256, PatchBytes: len(patch),
			TreeHash: tree, ChangedPaths: patchChangedPaths(patch),
		},
		Precommitments: precommitments,
		HostBuild:      hostBuildOfRelease(h.rt.selfdevUpdaterRoot, operation.ReleaseDigest),
		Declarations:   release.Declarations{TestCommand: strings.TrimSpace(r.URL.Query().Get("test_command"))},
	}
	releaseID, err := record.ReleaseID()
	if err == nil {
		err = record.Validate()
	}
	if err != nil {
		writeAPIJSON(w, http.StatusUnprocessableEntity, releaseAPIError{Error: err.Error(), Reason: "release_record_invalid"})
		return
	}
	canonical, _ := record.Canonical()
	writeAPIJSON(w, http.StatusOK, releaseDraft{
		ReleaseID: releaseID, Record: canonical, PatchSHA256: bundle.SourcePatchSHA256, PatchBytes: len(patch),
		PatchBase64: base64.StdEncoding.EncodeToString(patch),
	})
}

// hostBuildOfRelease names the host build of a layered applied release (the
// P5 seam). A plain release has none.
func hostBuildOfRelease(root, releaseDigest string) *release.HostBuild {
	if !computerevent.IsSHA256(releaseDigest) {
		return nil
	}
	manifest, _, err := updater.ReadPinnedManifest(root, releaseDigest)
	if err != nil || manifest.BuilderReceiptDigest == "" || manifest.ClosureDigest == "" || manifest.CodeCommit == "" {
		return nil
	}
	return &release.HostBuild{
		BuilderReceiptDigest: manifest.BuilderReceiptDigest, ClosureDigest: manifest.ClosureDigest,
		CodeCommit: manifest.CodeCommit, BaseImageManifestDigest: manifest.BaseImageManifestDigest,
	}
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

// patchChangedPaths lists the repository paths a git patch touches (the new
// path; the old one for a deletion), sorted and unique.
func patchChangedPaths(patch []byte) []string {
	seen := map[string]bool{}
	var out []string
	var oldPath string
	scanner := bufio.NewScanner(bytes.NewReader(patch))
	scanner.Buffer(make([]byte, 64<<10), release.MaxPatchBytes)
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "--- "):
			oldPath = strings.TrimPrefix(strings.TrimPrefix(line, "--- "), "a/")
		case strings.HasPrefix(line, "+++ "):
			path := strings.TrimPrefix(line, "+++ ")
			if path == "/dev/null" {
				path = oldPath
			} else {
				path = strings.TrimPrefix(path, "b/")
			}
			if path != "" && path != "/dev/null" && !seen[path] {
				seen[path] = true
				out = append(out, path)
			}
		}
	}
	sort.Strings(out)
	return out
}

// handleReleaseAdoptions serves
//
//	POST /api/computers/<id>/self-development/adoptions        adopt (owner act at the proxy)
//	GET  /api/computers/<id>/self-development/adoptions        list
//	GET  /api/computers/<id>/self-development/adoptions/<aid>  one
func (h *APIHandler) handleReleaseAdoptions(w http.ResponseWriter, r *http.Request, ownerID, computerID, adoption string) {
	switch {
	case adoption == "" && r.Method == http.MethodPost:
		var request adoptReleaseRequest
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, release.MaxRecordBytes+(64<<10)))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&request); err != nil {
			writeAPIJSON(w, http.StatusBadRequest, releaseAPIError{Error: "invalid adoption request", Reason: "request_invalid"})
			return
		}
		view, created, refusal := h.rt.adoptRelease(r.Context(), ownerID, computerID, request)
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
		list, err := h.rt.store.ListReleaseAdoptions(r.Context(), ownerID, computerID)
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, releaseAPIError{Error: "adoptions unreadable"})
			return
		}
		views := make([]ReleaseAdoptionView, 0, len(list))
		for _, item := range list {
			views = append(views, h.rt.adoptionView(r.Context(), item))
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
		writeAPIJSON(w, http.StatusOK, h.rt.adoptionView(r.Context(), item))
	default:
		writeAPIJSON(w, http.StatusMethodNotAllowed, releaseAPIError{Error: "method not allowed"})
	}
}
