package platform

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
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

	Marker     string `json:"marker"`
	CodeCommit string `json:"code_commit"`
	Files      []struct {
		Path  string `json:"path"`
		Mode  uint32 `json:"mode"`
		Bytes string `json:"bytes"` // base64
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
	if r.Header.Get("X-Internal-Caller") != "true" || h == nil || h.checkpointAuthority == nil {
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
	offer, err := buildPlatformUpdateOffer(request, time.Now().UTC())
	if err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
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
func buildPlatformUpdateOffer(request platformUpdateOfferMintRequest, now time.Time) (selfdevprotocol.PlatformUpdateOffer, error) {
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
		raw, err := base64.StdEncoding.DecodeString(file.Bytes)
		if err != nil {
			return selfdevprotocol.PlatformUpdateOffer{}, fmt.Errorf("platform update mint: payload file %q does not decode", file.Path)
		}
		sum := sha256.Sum256(raw)
		digest := hex.EncodeToString(sum[:])
		manifestFiles = append(manifestFiles, updater.ManifestFile{Path: clean, SHA256: digest, Mode: file.Mode})
		payloadFiles = append(payloadFiles, selfdevprotocol.PlatformUpdateFile{
			Path: clean, SHA256: digest, Mode: file.Mode, Bytes: file.Bytes,
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
			URI: "artifact+sha256://" + file.SHA256 + "/" + file.Path,
		})
	}
	closure, err := computerversion.NewCodeClosure(request.CodeCommit, codeArtifacts, now)
	if err != nil {
		return selfdevprotocol.PlatformUpdateOffer{}, err
	}
	programURI := "artifact+sha256://" + payloadFiles[0].SHA256 + "/" + payloadFiles[0].Path
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
