package platform

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/computerversion"
	"github.com/yusefmosiah/go-choir/internal/platformrelease"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
	"github.com/yusefmosiah/go-choir/internal/updater"
)

// platformUpdateOfferMintRequest is the corpusd mint input for a
// platform-follow push (M9a). The caller supplies identity, payload bytes,
// lineage attestation, and the base head; corpusd derives the closure,
// artifact program, and manifest canonically — requesters never hand-compute
// content-addressed refs.
type platformUpdateOfferMintRequest struct {
	ComputerID    string `json:"computer_id"`
	UpdateID      string `json:"update_id"`
	Realization   string `json:"realization_id"`
	BaseEventHead string `json:"base_event_head"`
	ExpiresAt     string `json:"expires_at"`

	// Layering join: when the release carries an app-layer closure, the mint
	// names the booted base manifest digest it resolves against and the
	// sha256 of the exported narchive payload file (closure.nar).
	BaseImageManifestDigest string `json:"base_image_manifest_digest,omitempty"`
	ClosureDigest           string `json:"closure_digest,omitempty"`

	Marker     string `json:"marker"`
	CodeCommit string `json:"code_commit"`
	Files      []struct {
		Path  string `json:"path"`
		Mode  uint32 `json:"mode"`
		Bytes string `json:"bytes"` // base64; empty when ref is used
		Ref   string `json:"ref"`   // artifact+sha256://<sha>/sha256/platform-update/<sha>
	} `json:"files"`
	VerifierRefs []string `json:"verifier_refs"`

	DivergenceStatus     string   `json:"divergence_status"`
	PlatformFollowPolicy string   `json:"platform_follow_policy"`
	DivergedComponents   []string `json:"diverged_components,omitempty"`
}

// HandlePlatformUpdateOfferMint serves POST
// /internal/computers/platform-updates/offer — the corpusd mint for
// platform-follow update pushes (M9a). Corpusd builds the canonical offer
// (content-addressed closure/program, manifest, payload digests) and signs it
// under platform-control. The caller transports the signed offer to the guest
// over the vmctl autoputer proxy — corpusd signs, the guest verifies and
// applies.
func (h *Handler) HandlePlatformUpdateOfferMint(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if !trustedInternalCaller(r) || h == nil || h.checkpointAuthority == nil {
		writeJSON(w, http.StatusForbidden, apiError{Error: "platform update mint is not publicly accessible"})
		return
	}
	var request platformUpdateOfferMintRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid platform update mint request"})
		return
	}
	offer, err := buildPlatformUpdateOffer(request, time.Now().UTC(), func(digest string) (bool, error) {
		path, pathErr := h.service.artifactPath(filepath.Join("sha256", "platform-update", digest))
		if pathErr != nil {
			return false, pathErr
		}
		info, statErr := os.Stat(path)
		if errors.Is(statErr, os.ErrNotExist) {
			return false, nil
		}
		return statErr == nil && !info.IsDir(), statErr
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}
	// Persist payload bytes into platform-artifacts: vmctl's route-apply
	// verifier resolves artifact+sha256 URIs against the same shared root, so
	// the mint must stage each file under its content digest.
	for _, file := range offer.Files {
		if file.Ref != "" {
			continue // ref payloads were pre-staged by the PUT blob endpoint
		}
		raw, decErr := base64.StdEncoding.DecodeString(file.Bytes)
		if decErr != nil {
			writeJSON(w, http.StatusBadRequest, apiError{Error: fmt.Sprintf("platform update mint: payload %q does not decode", file.Path)})
			return
		}
		sum := sha256.Sum256(raw)
		if hex.EncodeToString(sum[:]) != file.SHA256 {
			writeJSON(w, http.StatusBadRequest, apiError{Error: fmt.Sprintf("platform update mint: payload %q digest mismatch", file.Path)})
			return
		}
		if writeErr := h.checkpointAuthority.service.writeBlob(filepath.Join("sha256", "platform-update", file.SHA256), raw); writeErr != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Error: fmt.Sprintf("platform update mint: stage artifact: %v", writeErr)})
			return
		}
	}
	offerDigest, err := offer.Digest()
	if err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "platform update offer is not digestible"})
		return
	}
	receipt, err := selfdevprotocol.NewAuthorityReceipt(
		selfdevprotocol.ReceiptKindPlatformUpdate, offer.ComputerID,
		offerDigest, offerDigest, h.checkpointAuthority.cas.issuer,
		h.checkpointAuthority.cas.signingKey, time.Now().UTC())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
		return
	}
	offer.Authorization = receipt
	writeJSON(w, http.StatusCreated, offer)
}

// buildPlatformUpdateOffer derives the content-addressed closure, artifact
// program, manifest, and per-file digests for the mint request, then validates
// the assembled offer exactly as the guest would.
func buildPlatformUpdateOffer(request platformUpdateOfferMintRequest, now time.Time, refExists func(digest string) (bool, error)) (selfdevprotocol.PlatformUpdateOffer, error) {
	if strings.TrimSpace(request.ComputerID) == "" || strings.TrimSpace(request.UpdateID) == "" ||
		strings.TrimSpace(request.Realization) == "" || len(request.Files) == 0 {
		return selfdevprotocol.PlatformUpdateOffer{}, fmt.Errorf("platform update mint: computer, update id, realization, and files are required")
	}
	manifestFiles := make([]updater.ManifestFile, 0, len(request.Files))
	payloadFiles := make([]selfdevprotocol.PlatformUpdateFile, 0, len(request.Files))
	seen := map[string]bool{}
	for _, file := range request.Files {
		clean := strings.TrimSpace(strings.TrimPrefix(file.Path, "/"))
		if clean == "" || clean == ".." || strings.HasPrefix(clean, "../") || seen[clean] {
			return selfdevprotocol.PlatformUpdateOffer{}, fmt.Errorf("platform update mint: payload path %q is invalid", file.Path)
		}
		seen[clean] = true
		var digest string
		if strings.TrimSpace(file.Ref) != "" {
			// Ref-carried payload: the digest is the canonical artifact ref, not
			// the (absent) inline bytes. The blob must already be staged under
			// sha256/platform-update/<digest> (PUT blob endpoint).
			if file.Bytes != "" {
				return selfdevprotocol.PlatformUpdateOffer{}, fmt.Errorf("platform update mint: payload file %q carries both bytes and ref", file.Path)
			}
			var parseErr error
			digest, parseErr = platformUpdateRefDigest(file.Ref)
			if parseErr != nil {
				return selfdevprotocol.PlatformUpdateOffer{}, fmt.Errorf("platform update mint: payload file %q ref is not the canonical artifact digest", file.Path)
			}
			if refExists == nil {
				return selfdevprotocol.PlatformUpdateOffer{}, fmt.Errorf("platform update mint: ref payloads require a staged blob resolver")
			}
			ok, existsErr := refExists(digest)
			if existsErr != nil || !ok {
				return selfdevprotocol.PlatformUpdateOffer{}, fmt.Errorf("platform update mint: payload file %q ref blob is not staged", file.Path)
			}
		} else {
			raw, err := base64.StdEncoding.DecodeString(file.Bytes)
			if err != nil {
				return selfdevprotocol.PlatformUpdateOffer{}, fmt.Errorf("platform update mint: payload file %q does not decode", file.Path)
			}
			sum := sha256.Sum256(raw)
			digest = hex.EncodeToString(sum[:])
		}
		manifestFiles = append(manifestFiles, updater.ManifestFile{Path: clean, SHA256: digest, Mode: file.Mode})
		payloadFiles = append(payloadFiles, selfdevprotocol.PlatformUpdateFile{
			Path: clean, SHA256: digest, Mode: file.Mode, Bytes: file.Bytes, Ref: file.Ref,
		})
	}
	sort.Slice(manifestFiles, func(i, j int) bool { return manifestFiles[i].Path < manifestFiles[j].Path })
	sort.Slice(payloadFiles, func(i, j int) bool { return payloadFiles[i].Path < payloadFiles[j].Path })

	// Closure binds the source commit plus one artifact per payload file; the
	// artifact program names the release as one program entry chain.
	codeArtifacts := make([]computerversion.CodeArtifact, 0, len(payloadFiles))
	for _, file := range payloadFiles {
		codeArtifacts = append(codeArtifacts, computerversion.CodeArtifact{
			Name: file.Path, SHA256: file.SHA256,
			URI: "artifact+sha256://" + file.SHA256 + "/sha256/platform-update/" + file.SHA256,
		})
	}
	closure, err := computerversion.NewCodeClosure(request.CodeCommit, codeArtifacts, now)
	if err != nil {
		return selfdevprotocol.PlatformUpdateOffer{}, err
	}
	programURI := "artifact+sha256://" + payloadFiles[0].SHA256 + "/sha256/platform-update/" + payloadFiles[0].SHA256
	program, err := computerversion.NewArtifactProgram([]computerversion.ArtifactProgramEntry{{
		Kind: "platform_update_release", ContentSHA256: payloadFiles[0].SHA256, ArtifactURI: programURI,
	}}, now)
	if err != nil {
		return selfdevprotocol.PlatformUpdateOffer{}, err
	}
	manifest, err := updater.FinalizeManifest(updater.ReleaseManifest{
		Version: updater.ManifestVersion, ComputerID: request.ComputerID,
		CodeRef: string(closure.Ref), ArtifactProgramRef: string(program.Ref),
		EventSchemaVersion: computerevent.SchemaVersionV1, ReducerVersion: computerevent.ReducerVersionV1,
		Marker: strings.TrimSpace(request.Marker), Files: manifestFiles,
		BaseImageManifestDigest: request.BaseImageManifestDigest,
		ClosureDigest:           request.ClosureDigest,
	})
	if err != nil {
		return selfdevprotocol.PlatformUpdateOffer{}, err
	}
	offer := selfdevprotocol.PlatformUpdateOffer{
		Version: 1, ComputerID: request.ComputerID, UpdateID: request.UpdateID,
		Realization: request.Realization, Manifest: manifest, Files: payloadFiles,
		CodeClosure: closure, ArtifactProgram: program,
		VerifierRefs:         request.VerifierRefs,
		DivergenceStatus:     request.DivergenceStatus,
		PlatformFollowPolicy: request.PlatformFollowPolicy,
		DivergedComponents:   request.DivergedComponents,
		BaseEventHead:        request.BaseEventHead,
		ExpiresAt:            request.ExpiresAt,
	}
	if offer.PlatformFollowPolicy == "" {
		offer.PlatformFollowPolicy = platformrelease.FollowPolicyAuto
	}
	if offer.DivergenceStatus == "" {
		offer.DivergenceStatus = platformrelease.DivergenceTracking
	}
	if err := selfdevprotocol.PlatformUpdateOfferFromRequest(offer, now); err != nil {
		return selfdevprotocol.PlatformUpdateOffer{}, err
	}
	return offer, nil
}

// platformUpdateRefDigest extracts the sha256 digest from a canonical
// platform-update artifact ref
// (artifact+sha256://<sha>/sha256/platform-update/<sha>). Any other form is
// rejected so callers cannot name an arbitrary fetch URI.
func platformUpdateRefDigest(ref string) (string, error) {
	const prefix = "artifact+sha256://"
	const infix = "/sha256/platform-update/"
	rest, ok := strings.CutPrefix(ref, prefix)
	if !ok {
		return "", fmt.Errorf("platform update ref: bad scheme")
	}
	head, tail, ok := strings.Cut(rest, infix)
	if !ok || head != tail || !computerevent.IsSHA256(head) {
		return "", fmt.Errorf("platform update ref: not canonical")
	}
	return head, nil
}

// platformUpdateBlobPath returns the artifact-store path for a platform-update
// payload blob under sha256/platform-update/<digest>.
func (h *Handler) platformUpdateBlobPath(digest string) (string, error) {
	if !computerevent.IsSHA256(digest) {
		return "", fmt.Errorf("platform update blob: digest required")
	}
	return h.service.artifactPath(filepath.Join("sha256", "platform-update", digest))
}

// HandlePlatformUpdateBlob serves
//
//	PUT  /internal/computers/platform-updates/blob/<sha256> — internal upload:
//	     streams the payload to a temp file while hashing, renames into
//	     sha256/platform-update/<sha256> only when the digest matches.
//	GET  /internal/computers/platform-updates/blob/<sha256> — guest fetch:
//	     streams the staged blob; the guest verifies the digest after
//	     download, never trusting the wire (same posture as
//	     HandleProjectionBaseBlob).
//
// The signed offer's Ref field names the blob; the mint refuses a ref whose
// blob was not uploaded first.
func (h *Handler) HandlePlatformUpdateBlob(w http.ResponseWriter, r *http.Request) {
	digest := strings.Trim(strings.TrimPrefix(r.URL.Path, "/internal/computers/platform-updates/blob/"), "/")
	if !computerevent.IsSHA256(digest) {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "platform update blob: sha256 digest required"})
		return
	}
	switch r.Method {
	case http.MethodPut:
		if !trustedInternalCaller(r) {
			writeJSON(w, http.StatusForbidden, apiError{Error: "platform update blob upload is not publicly accessible"})
			return
		}
		path, err := h.platformUpdateBlobPath(digest)
		if err != nil || h == nil || h.service == nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Error: "platform update blob store unavailable"})
			return
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Error: "platform update blob: create dir"})
			return
		}
		tmp := path + ".tmp-" + fmt.Sprintf("%d", time.Now().UnixNano())
		out, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o640)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Error: "platform update blob: stage temp"})
			return
		}
		hasher := sha256.New()
		// Stream the body to disk and hash it in one pass (TeeReader), bounded.
		if _, err := io.Copy(out, io.TeeReader(io.LimitReader(r.Body, 2<<30), hasher)); err != nil {
			out.Close()
			os.Remove(tmp)
			writeJSON(w, http.StatusInternalServerError, apiError{Error: "platform update blob: read upload"})
			return
		}
		if err := out.Close(); err != nil {
			os.Remove(tmp)
			writeJSON(w, http.StatusInternalServerError, apiError{Error: "platform update blob: write"})
			return
		}
		if hex.EncodeToString(hasher.Sum(nil)) != digest {
			os.Remove(tmp)
			writeJSON(w, http.StatusBadRequest, apiError{Error: "platform update blob: digest mismatch"})
			return
		}
		if err := os.Rename(tmp, path); err != nil {
			os.Remove(tmp)
			writeJSON(w, http.StatusInternalServerError, apiError{Error: "platform update blob: install"})
			return
		}
		writeJSON(w, http.StatusCreated, map[string]string{"digest": digest})
	case http.MethodGet:
		computerID := strings.TrimSpace(r.URL.Query().Get("computer_id"))
		if !h.authorizeFileCAS(r, computerID, "event:read") {
			writeJSON(w, http.StatusForbidden, apiError{Error: "computer capability required"})
			return
		}
		path, err := h.platformUpdateBlobPath(digest)
		if err != nil || h == nil || h.service == nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Error: "platform update blob store unavailable"})
			return
		}
		file, err := os.Open(path)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				writeJSON(w, http.StatusNotFound, apiError{Error: "platform update blob unavailable"})
				return
			}
			writeJSON(w, http.StatusInternalServerError, apiError{Error: "platform update blob unreadable"})
			return
		}
		defer file.Close()
		info, err := file.Stat()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Error: "platform update blob unreadable"})
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		http.ServeContent(w, r, digest, info.ModTime(), file)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
	}
}
