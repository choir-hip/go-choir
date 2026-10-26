package agentcore

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/capsule/transaction"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
)

// sourcePatchExportMaxBytes caps the exported patch so its base64 JSON
// envelope stays under the proxy's 4 MiB self-development response relay
// (internal/proxy/self_development.go); a larger patch would arrive
// truncated.
const sourcePatchExportMaxBytes = 2 << 20

// sourcePatchExport is the read-only answer of
// GET /api/computers/<id>/self-development/operations/<op>/source-patch: a
// frozen candidate's source.patch with the bindings an outside agent needs
// to open a draft PR (docs/reports/selfdev-readiness-2026-10-15.md B3).
type sourcePatchExport struct {
	OperationID           string `json:"operation_id"`
	ComputerID            string `json:"computer_id"`
	State                 string `json:"state"`
	BundleDigest          string `json:"bundle_digest"`
	SourcePatchSHA256     string `json:"source_patch_sha256"`
	SourcePatchBaseCommit string `json:"source_patch_base_commit"`
	PatchBytes            int    `json:"patch_bytes"`
	PatchBase64           string `json:"patch_base64"`
}

// exportSelfDevelopmentSourcePatch serves the frozen bundle's source.patch.
// It never mutates: no state moves, no file is written. It refuses before
// Freeze, and whenever the bytes are not exactly what the bundle bound.
func (h *APIHandler) exportSelfDevelopmentSourcePatch(w http.ResponseWriter, r *http.Request, computerID, operationID string) {
	operation, err := h.rt.selfdevOperations.Get(r.Context(), computerID, operationID)
	if err != nil {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "self-development operation not found"})
		return
	}
	if operation.State == selfdev.StateRequested || operation.State == selfdev.StateExecuting || operation.BundleDigest == "" {
		writeAPIJSON(w, http.StatusConflict, apiError{Error: "operation is " + operation.State + "; a source patch exists only once the candidate is frozen"})
		return
	}
	if strings.TrimSpace(h.rt.selfdevUpdaterRoot) == "" {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "self-development bundle store unavailable"})
		return
	}
	if !computerevent.IsSHA256(operation.BundleDigest) {
		writeAPIJSON(w, http.StatusConflict, apiError{Error: "operation bundle digest is malformed"})
		return
	}
	bundleRoot := filepath.Join(h.rt.selfdevUpdaterRoot, "incoming", operation.BundleDigest)
	bundle, status, err := readBoundSelfDevelopmentBundle(bundleRoot, operation)
	if err != nil {
		writeAPIJSON(w, status, apiError{Error: err.Error()})
		return
	}
	if bundle.SourcePatchSHA256 == "" {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "the candidate changed no workspace/platform source; there is no source patch"})
		return
	}
	if strings.TrimSpace(bundle.SourcePatchBaseCommit) == "" {
		writeAPIJSON(w, http.StatusConflict, apiError{Error: "the bundle records no source patch base commit"})
		return
	}
	patch, status, err := readBoundSourcePatch(filepath.Join(bundleRoot, "source.patch"), bundle.SourcePatchSHA256)
	if err != nil {
		writeAPIJSON(w, status, apiError{Error: err.Error()})
		return
	}
	if kinds := sourcePatchAddedSecrets(patch); len(kinds) > 0 {
		writeAPIJSON(w, http.StatusUnprocessableEntity, apiError{Error: "source patch refused: added lines carry secret content (" + strings.Join(kinds, ", ") + ")"})
		return
	}
	writeAPIJSON(w, http.StatusOK, sourcePatchExport{
		OperationID: operation.OperationID, ComputerID: operation.ComputerID, State: operation.State,
		BundleDigest: operation.BundleDigest, SourcePatchSHA256: bundle.SourcePatchSHA256,
		SourcePatchBaseCommit: bundle.SourcePatchBaseCommit, PatchBytes: len(patch),
		PatchBase64: base64.StdEncoding.EncodeToString(patch),
	})
}

// readBoundSelfDevelopmentBundle reads the bundle the operation names: the
// final bundle.json (verified onward; its bytes hash to the digest) or the
// Freeze draft (frozen; its recomputed content digest is the digest), and
// requires it to bind this operation.
func readBoundSelfDevelopmentBundle(bundleRoot string, operation selfdev.Operation) (transaction.CapsuleEffectBundle, int, error) {
	var bundle transaction.CapsuleEffectBundle
	if _, err := os.Lstat(bundleRoot); errors.Is(err, fs.ErrNotExist) {
		return bundle, http.StatusGone, errors.New("the frozen bundle is no longer retained on this computer")
	}
	raw, err := os.ReadFile(filepath.Join(bundleRoot, "bundle.json"))
	final := err == nil
	if errors.Is(err, fs.ErrNotExist) {
		raw, err = os.ReadFile(filepath.Join(bundleRoot, "bundle.draft.json"))
	}
	if err != nil {
		return bundle, http.StatusGone, errors.New("the frozen bundle is no longer retained on this computer")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	// One status, three named checks: a bundle torn by a hard kill does not
	// decode, which is not the same finding as a digest or identity mismatch.
	// Clients match the "frozen bundle binding mismatch: " prefix.
	if decoder.Decode(&bundle) != nil {
		return bundle, http.StatusConflict, errors.New("frozen bundle binding mismatch: frozen bundle does not decode")
	}
	bound := false
	if final {
		bound = computerevent.DigestBytes(raw) == operation.BundleDigest
	} else if digest, digestErr := bundle.ComputeContentDigest(); digestErr == nil {
		bound = digest == operation.BundleDigest && bundle.ContentDigest == operation.BundleDigest
	}
	if !bound {
		return bundle, http.StatusConflict, errors.New("frozen bundle binding mismatch: frozen bundle digest does not match the operation")
	}
	if !selfDevelopmentBundleMatchesOperation(bundle, operation) {
		return bundle, http.StatusConflict, errors.New("frozen bundle binding mismatch: frozen bundle names another computer, trajectory, capsule or base")
	}
	return bundle, http.StatusOK, nil
}

// readBoundSourcePatch reads a regular (never symlinked) source.patch under
// the size cap and requires its bytes to hash to the bundle's sha256.
func readBoundSourcePatch(path, wantSHA string) ([]byte, int, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, http.StatusGone, errors.New("the frozen bundle's source.patch is no longer retained")
	}
	if err != nil || !info.Mode().IsRegular() {
		return nil, http.StatusConflict, errors.New("the frozen bundle's source.patch is not a regular file")
	}
	if info.Size() > sourcePatchExportMaxBytes {
		return nil, http.StatusRequestEntityTooLarge, errors.New("source patch exceeds the export cap of 2 MiB")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, http.StatusConflict, errors.New("the frozen bundle's source.patch is unreadable")
	}
	defer file.Close()
	patch, err := io.ReadAll(io.LimitReader(file, sourcePatchExportMaxBytes+1))
	if err != nil || len(patch) > sourcePatchExportMaxBytes {
		return nil, http.StatusRequestEntityTooLarge, errors.New("source patch exceeds the export cap of 2 MiB")
	}
	sum := sha256.Sum256(patch)
	if hex.EncodeToString(sum[:]) != strings.ToLower(strings.TrimSpace(wantSHA)) {
		return nil, http.StatusConflict, errors.New("source patch bytes do not hash to the bundle's recorded source_patch_sha256")
	}
	return patch, http.StatusOK, nil
}

// sourcePatchAddedSecrets runs the release's refusal-grade secret scan over
// the lines the patch adds: those are what a pushed branch would publish.
// Context and removed lines are already in the base commit.
func sourcePatchAddedSecrets(patch []byte) []string {
	seen := map[string]bool{}
	var kinds []string
	scanner := bufio.NewScanner(bytes.NewReader(patch))
	scanner.Buffer(make([]byte, 64<<10), sourcePatchExportMaxBytes)
	for scanner.Scan() {
		line := scanner.Bytes()
		if !bytes.HasPrefix(line, []byte("+")) || bytes.HasPrefix(line, []byte("+++ ")) {
			continue
		}
		for _, kind := range computerevent.DetectRefusalSecrets(line[1:]) {
			if !seen[kind] {
				seen[kind] = true
				kinds = append(kinds, kind)
			}
		}
	}
	if scanner.Err() != nil && len(kinds) == 0 {
		kinds = append(kinds, "unscannable_line")
	}
	return kinds
}
