package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/computerversion"
	"github.com/yusefmosiah/go-choir/internal/routeledger"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
	choirstore "github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/updater"
)

var ErrRematerializeUnavailable = errors.New("tape rematerialize unavailable")

type RematerializeReport struct {
	ComputerID           string `json:"computer_id"`
	CheckpointDigest     string `json:"checkpoint_digest"`
	Method               string `json:"method"`
	QuarantineDir        string `json:"quarantine_dir"`
	QuarantinedMarker    string `json:"quarantined_marker"`
	QuarantinedWorkspace string `json:"quarantined_workspace"`
	WitnessMatched       bool   `json:"witness_matched"`
	OriginalDenied       bool   `json:"original_denied"`
	FrontendRestaged     bool   `json:"frontend_restaged"`
	StoreClosed          bool   `json:"store_closed"`
	PinCheckoutUsed      bool   `json:"pin_checkout_used"`
}

type rematerializeAPIRequest struct {
	Checkpoint selfdevprotocol.Checkpoint `json:"checkpoint"`
}

type restoreAPIRequest struct {
	Checkpoint    selfdevprotocol.Checkpoint `json:"checkpoint"`
	OperandScopes []string                   `json:"operand_scopes"`
}

func (rt *Runtime) RematerializeFromTape(ctx context.Context, computerID string, checkpoint selfdevprotocol.Checkpoint) (RematerializeReport, error) {
	var report RematerializeReport
	report.Method = selfdevprotocol.RematerializeMethodTape
	if rt == nil || rt.store == nil || rt.eventAppender == nil {
		return report, fmt.Errorf("%w: event projection authority is not configured", ErrRematerializeUnavailable)
	}
	computerID = strings.TrimSpace(computerID)
	if computerID == "" || computerID != strings.TrimSpace(rt.cfg.ComputerID) {
		return report, fmt.Errorf("%w: computer binding does not match runtime", ErrRematerializeUnavailable)
	}
	if err := ctx.Err(); err != nil {
		return report, err
	}

	rt.workspaceReplaceMu.Lock()
	defer rt.workspaceReplaceMu.Unlock()
	if rt.store == nil {
		return report, fmt.Errorf("%w: runtime store is not configured", ErrRematerializeUnavailable)
	}

	originalMarker := strings.TrimSpace(rt.store.Path())
	originalWorkspace := strings.TrimSpace(rt.store.TexturePath())
	if originalMarker == "" || originalWorkspace == "" {
		return report, fmt.Errorf("%w: original realization paths are required", ErrRematerializeUnavailable)
	}
	stagingRoot := filepath.Join(filepath.Dir(originalMarker), "rematerialize-staging-"+time.Now().UTC().Format("20060102T150405.000000000Z"))
	stagedMarker := filepath.Join(stagingRoot, filepath.Base(originalMarker))
	request := selfdevprotocol.RematerializeRequest{
		ComputerID:            computerID,
		Checkpoint:            checkpoint,
		Method:                selfdevprotocol.RematerializeMethodTape,
		OriginalMarkerPath:    originalMarker,
		OriginalWorkspacePath: originalWorkspace,
		StagedMarkerPath:      stagedMarker,
		StagedWorkspacePath:   filepath.Join(stagingRoot, filepath.Base(originalWorkspace)+"-pending"),
	}
	if err := selfdevprotocol.RematerializeFromRequest(request); err != nil {
		return report, err
	}
	priorReleaseDigest, err := rt.verifyPinnedFrontend(checkpoint)
	if err != nil {
		return report, err
	}

	staged, err := choirstore.OpenFresh(stagedMarker)
	if err != nil {
		return report, fmt.Errorf("rematerialize: open staged realization: %w", err)
	}
	request.StagedWorkspacePath = staged.TexturePath()
	if err := selfdevprotocol.RematerializeFromRequest(request); err != nil {
		_ = staged.Close()
		_ = os.RemoveAll(stagingRoot)
		return report, err
	}
	targetHead := strings.TrimSpace(checkpoint.Request.AcceptedEventHead)
	if err := rt.eventAppender.ReconstructThroughTarget(ctx, staged, targetHead); err != nil {
		_ = staged.Close()
		_ = os.RemoveAll(stagingRoot)
		return report, fmt.Errorf("rematerialize: reconstruct through target: %w", err)
	}
	version := replayCompletenessVersion(rt, nil)
	if head, err := staged.Head(ctx, computerID); err == nil {
		version = replayCompletenessVersion(rt, head)
	}
	extractor := replayDoltStateExtractor(staged.TexturePath())
	replay, err := extractor.Extract(ctx, computerversion.ExtractRequest{Name: "tape-rematerialize", Version: version})
	if err != nil {
		_ = staged.Close()
		_ = os.RemoveAll(stagingRoot)
		return report, fmt.Errorf("rematerialize: extract reconstructed witness: %w", err)
	}
	got, err := selfdevprotocol.WitnessFromObservationSet(replay)
	if err != nil {
		_ = staged.Close()
		_ = os.RemoveAll(stagingRoot)
		return report, err
	}
	if err := selfdevprotocol.WitnessContentMatches(got, checkpoint.Request.VMLocalContentWitness); err != nil {
		_ = staged.Close()
		_ = os.RemoveAll(stagingRoot)
		return report, err
	}
	stagedWorkspace := staged.TexturePath()
	if err := staged.Close(); err != nil {
		_ = os.RemoveAll(stagingRoot)
		return report, fmt.Errorf("rematerialize: close staged realization: %w", err)
	}

	if err := updater.RestagePinnedRelease(rt.selfdevUpdaterRoot, checkpoint.Request.ReleaseDigest); err != nil {
		_ = os.RemoveAll(stagingRoot)
		return report, fmt.Errorf("rematerialize: restage served SPA: %w", err)
	}
	keepRestaged := false
	defer func() {
		if keepRestaged || priorReleaseDigest == "" || priorReleaseDigest == checkpoint.Request.ReleaseDigest {
			return
		}
		_ = updater.RestagePinnedRelease(rt.selfdevUpdaterRoot, priorReleaseDigest)
	}()

	quarantineDir := filepath.Join(filepath.Dir(originalMarker), "rematerialize-quarantine-"+time.Now().UTC().Format("20060102T150405.000000000Z"))
	if err := rt.store.Close(); err != nil {
		_ = os.RemoveAll(stagingRoot)
		return report, fmt.Errorf("rematerialize: close original realization: %w", err)
	}
	if err := os.Mkdir(quarantineDir, 0o755); err != nil {
		_ = os.RemoveAll(stagingRoot)
		return report, fmt.Errorf("rematerialize: create quarantine: %w", err)
	}
	quarantinedMarker := filepath.Join(quarantineDir, filepath.Base(originalMarker))
	quarantinedWorkspace := filepath.Join(quarantineDir, filepath.Base(originalWorkspace))
	if err := os.Rename(originalMarker, quarantinedMarker); err != nil {
		_ = os.RemoveAll(stagingRoot)
		return report, fmt.Errorf("rematerialize: quarantine original marker: %w", err)
	}
	if err := os.Rename(originalWorkspace, quarantinedWorkspace); err != nil {
		_ = os.Rename(quarantinedMarker, originalMarker)
		_ = os.RemoveAll(stagingRoot)
		return report, fmt.Errorf("rematerialize: quarantine original workspace: %w", err)
	}
	if err := os.Rename(stagedMarker, originalMarker); err != nil {
		_ = os.Rename(quarantinedMarker, originalMarker)
		_ = os.Rename(quarantinedWorkspace, originalWorkspace)
		_ = os.RemoveAll(stagingRoot)
		return report, fmt.Errorf("rematerialize: flip staged marker: %w", err)
	}
	if err := os.Rename(stagedWorkspace, originalWorkspace); err != nil {
		_ = os.Remove(originalMarker)
		_ = os.Rename(quarantinedMarker, originalMarker)
		_ = os.Rename(quarantinedWorkspace, originalWorkspace)
		_ = os.RemoveAll(stagingRoot)
		return report, fmt.Errorf("rematerialize: flip staged workspace: %w", err)
	}
	_ = os.RemoveAll(stagingRoot)

	if err := rt.store.Reopen(originalMarker); err != nil {
		return report, fmt.Errorf("rematerialize: reopen flipped realization: %w", err)
	}
	if err := rt.eventAppender.RebindProjection(rt.store); err != nil {
		return report, fmt.Errorf("rematerialize: rebind event projection: %w", err)
	}
	if operations, err := selfdev.NewStore(rt.store, rt.store); err == nil {
		rt.selfdevOperations = operations
	}

	report.ComputerID = computerID
	report.CheckpointDigest = checkpoint.Digest
	report.QuarantineDir = quarantineDir
	report.QuarantinedMarker = quarantinedMarker
	report.QuarantinedWorkspace = quarantinedWorkspace
	report.WitnessMatched = true
	report.OriginalDenied = true
	report.FrontendRestaged = true
	report.StoreClosed = false
	report.PinCheckoutUsed = false
	keepRestaged = true
	return report, nil
}

func (h *APIHandler) rematerializeComputerFromTape(w http.ResponseWriter, r *http.Request, computerID string) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if h == nil || h.rt == nil {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "tape rematerialize authority unavailable"})
		return
	}
	var request rematerializeAPIRequest
	if r.Body != nil {
		defer r.Body.Close()
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&request); err != nil {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid rematerialize request"})
			return
		}
	}
	report, err := h.rt.RematerializeFromTape(r.Context(), computerID, request.Checkpoint)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, ErrRematerializeUnavailable) {
			status = http.StatusServiceUnavailable
		}
		writeAPIJSON(w, status, apiError{Error: err.Error()})
		return
	}
	writeAPIJSON(w, http.StatusOK, report)
}

func (h *APIHandler) restoreComputer(w http.ResponseWriter, r *http.Request, computerID string) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if h == nil || h.rt == nil {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "tape restore authority unavailable"})
		return
	}
	var request restoreAPIRequest
	if r.Body != nil {
		defer r.Body.Close()
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&request); err != nil {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid restore request"})
			return
		}
	}
	scopes := request.OperandScopes
	if len(scopes) == 0 {
		scopes = []string{selfdevprotocol.RestoreScopeVMLocal, selfdevprotocol.RestoreScopeFrontend}
	}
	if err := selfdevprotocol.RestoreFromRequest(selfdevprotocol.RestoreRequest{
		ComputerID:       computerID,
		CheckpointDigest: request.Checkpoint.Digest,
		OperandScopes:    scopes,
	}, request.Checkpoint); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}
	if err := h.rt.appendRestoreIntent(r.Context(), computerID, request.Checkpoint); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, ErrRematerializeUnavailable) {
			status = http.StatusServiceUnavailable
		}
		writeAPIJSON(w, status, apiError{Error: err.Error()})
		return
	}
	report, err := h.rt.RematerializeFromTape(r.Context(), computerID, request.Checkpoint)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, ErrRematerializeUnavailable) {
			status = http.StatusServiceUnavailable
		}
		writeAPIJSON(w, status, apiError{Error: err.Error()})
		return
	}
	writeAPIJSON(w, http.StatusOK, report)
}

func (rt *Runtime) appendRestoreIntent(ctx context.Context, computerID string, checkpoint selfdevprotocol.Checkpoint) error {
	if rt == nil || rt.eventAppender == nil {
		return fmt.Errorf("%w: event projection authority is not configured", ErrRematerializeUnavailable)
	}
	computerID = strings.TrimSpace(computerID)
	if computerID == "" || computerID != strings.TrimSpace(rt.cfg.ComputerID) {
		return fmt.Errorf("%w: computer binding does not match runtime", ErrRematerializeUnavailable)
	}
	idempotency := "restore-intent-" + strings.TrimSpace(checkpoint.Digest)
	if rt.store != nil {
		if _, found, err := rt.store.EventByIdempotency(ctx, computerID, idempotency); err != nil {
			return fmt.Errorf("restore: lookup restore intent: %w", err)
		} else if found {
			return nil
		}
	}
	eventID, err := computerevent.NewEventID()
	if err != nil {
		return err
	}
	event := computerevent.Event{
		SchemaVersion: computerevent.SchemaVersionV1, EventID: eventID, ComputerID: computerID,
		EventKind: computerevent.EventRestoreRequested, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
		IdempotencyKey: idempotency, RequestCommitment: computerevent.ZeroHead,
		ActorProfile: "owner", AuthorityRef: "platform-control:restore",
		PayloadCommitment: computerevent.ZeroHead, PrivacyClass: "owner",
		ProposedEffectRef: checkpoint.Digest, DecisionRef: checkpoint.Request.AcceptedEventHead,
		ReducerVersion: computerevent.ReducerVersionV1,
	}
	if _, err := rt.eventAppender.AppendNew(ctx, event, computerevent.TransitionInput{}, nil); err != nil {
		return fmt.Errorf("restore: append restore intent: %w", err)
	}
	return nil
}

func (h *APIHandler) bindComputerCheckpoint(w http.ResponseWriter, r *http.Request, computerID string) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if h == nil || h.rt == nil {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "tape checkpoint authority unavailable"})
		return
	}
	if r.Body != nil {
		defer r.Body.Close()
	}
	report, err := h.rt.BindCheckpointRestoreSet(r.Context(), computerID)
	if err != nil {
		status := http.StatusConflict
		if errors.Is(err, ErrRematerializeUnavailable) || errors.Is(err, ErrReplayCompletenessUnavailable) {
			status = http.StatusServiceUnavailable
		}
		writeAPIJSON(w, status, apiError{Error: err.Error()})
		return
	}
	writeAPIJSON(w, http.StatusOK, report)
}

type CheckpointBindReport struct {
	ComputerID            string                                `json:"computer_id"`
	CheckpointEligible    bool                                  `json:"checkpoint_eligible"`
	ReleaseDigest         string                                `json:"release_digest"`
	VMLocalContentWitness selfdevprotocol.VMLocalContentWitness `json:"vm_local_content_witness"`
	FrontendIdentity      selfdevprotocol.FrontendIdentity      `json:"frontend_identity"`
	PublishedCheckpoint   *selfdevprotocol.CheckpointResponse   `json:"published_checkpoint,omitempty"`
}

func (rt *Runtime) BindCheckpointRestoreSet(ctx context.Context, computerID string) (CheckpointBindReport, error) {
	var report CheckpointBindReport
	if rt == nil || rt.store == nil || rt.eventAppender == nil {
		return report, fmt.Errorf("%w: event projection authority is not configured", ErrRematerializeUnavailable)
	}
	computerID = strings.TrimSpace(computerID)
	if computerID == "" || computerID != strings.TrimSpace(rt.cfg.ComputerID) {
		return report, fmt.Errorf("%w: computer binding does not match runtime", ErrRematerializeUnavailable)
	}
	manifest, err := rt.ensureCheckpointReleaseManifest(ctx, computerID)
	if err != nil {
		return report, err
	}
	witness, frontend, err := rt.checkpointRestoreBindings(ctx, computerID, manifest.ContentDigest, manifest.Files)
	if err != nil {
		return report, err
	}
	report.ComputerID = computerID
	report.CheckpointEligible = true
	report.ReleaseDigest = manifest.ContentDigest
	report.VMLocalContentWitness = witness
	report.FrontendIdentity = frontend
	if rt.ownerRecoveryControl == nil {
		// No platform control credential is wired: this remains a bind report.
		return report, nil
	}
	published, err := rt.publishOwnerRecoveryCheckpoint(ctx, computerID, manifest, witness, frontend)
	if err != nil {
		return report, err
	}
	report.PublishedCheckpoint = &published
	return report, nil
}

// publishOwnerRecoveryCheckpoint publishes the owner-recovery evidence class
// checkpoint for an accumulated computer: verifier fields absent, canonical
// head bound as the restore target, effective head as the projection head.
// The VM-local witness is attested here; restore-time tape reconstruction
// enforces that it is true. Idempotency is head-scoped so the same head can be
// re-published (lookup replays the stored response) but a moved head cannot
// overwrite the record.
func (rt *Runtime) publishOwnerRecoveryCheckpoint(ctx context.Context, computerID string, manifest updater.ReleaseManifest, witness selfdevprotocol.VMLocalContentWitness, frontend selfdevprotocol.FrontendIdentity) (selfdevprotocol.CheckpointResponse, error) {
	head, err := rt.store.Head(ctx, computerID)
	if err != nil || head == nil {
		return selfdevprotocol.CheckpointResponse{}, fmt.Errorf("self-development checkpoint: live projection head unavailable")
	}
	headEvent, found, err := rt.store.EventByDigest(ctx, computerID, head.CanonicalEventHead)
	if err != nil || !found {
		return selfdevprotocol.CheckpointResponse{}, fmt.Errorf("self-development checkpoint: head event unavailable")
	}
	headReceipt, found, err := rt.store.EventReceiptByIdempotency(ctx, computerID, headEvent.IdempotencyKey)
	if err != nil || !found {
		return selfdevprotocol.CheckpointResponse{}, fmt.Errorf("self-development checkpoint: head event receipt unavailable")
	}
	reconstruction, err := selfdevprotocol.Digest(struct {
		ReleaseDigest string `json:"release_digest"`
		EffectiveHead string `json:"effective_event_head"`
	}{
		manifest.ContentDigest, head.EffectiveEventHead,
	})
	if err != nil {
		return selfdevprotocol.CheckpointResponse{}, err
	}
	return rt.ownerRecoveryControl.PublishCheckpoint(ctx, selfdevprotocol.CheckpointRequest{
		ComputerID: computerID, IdempotencyKey: "owner-recovery-" + head.CanonicalEventHead,
		ComputerVersion:   checkpointComputerVersion(manifest),
		AcceptedEventHead: head.CanonicalEventHead, EffectiveEventHead: head.EffectiveEventHead,
		EffectiveStateCommitment: head.EffectiveStateCommitment,
		EventHeadReceiptID:       headReceipt.ReceiptID,
		ReleaseDigest:            manifest.ContentDigest,
		ReconstructionDigest:     reconstruction,
		ReducerVersion:           head.ReducerVersion,
		OwnerRecovery:            true,
		VMLocalContentWitness:    witness,
		FrontendIdentity:         frontend,
	})
}

func trustedBaselineReleaseRoot() string {
	root := filepath.Clean(strings.TrimSpace(os.Getenv("CHOIR_BASELINE_RELEASE_ROOT")))
	if strings.HasPrefix(root, "/nix/store/") {
		return root
	}
	return ""
}

func checkpointComputerVersion(manifest updater.ReleaseManifest) computerversion.ComputerVersion {
	codeRef := strings.TrimSpace(manifest.CodeRef)
	artifact := strings.TrimSpace(manifest.ArtifactProgramRef)
	if codeRef != "" && !strings.HasPrefix(codeRef, "code:") {
		codeRef = "code:sha256:" + codeRef
	}
	if artifact != "" && !strings.HasPrefix(artifact, "artifact-program:") {
		artifact = "artifact-program:sha256:" + artifact
	}
	return computerversion.ComputerVersion{
		CodeRef:            computerversion.CodeRef(codeRef),
		ArtifactProgramRef: computerversion.ArtifactProgramRef(artifact),
	}
}

func (rt *Runtime) ensureCheckpointReleaseManifest(ctx context.Context, computerID string) (updater.ReleaseManifest, error) {
	var empty updater.ReleaseManifest
	if rt == nil || strings.TrimSpace(rt.selfdevUpdaterRoot) == "" {
		return empty, fmt.Errorf("self-development checkpoint: served SPA is underivable")
	}
	manifest, err := updater.ReadCurrentManifest(rt.selfdevUpdaterRoot)
	if err == nil {
		return manifest, nil
	}
	baselineRoot := trustedBaselineReleaseRoot()
	if baselineRoot == "" || rt.selfdevUpdater == nil || strings.TrimSpace(rt.selfdevRealizationID) == "" || rt.selfdevRoute == nil {
		return empty, fmt.Errorf("self-development checkpoint: served SPA is underivable")
	}
	slotID, err := routeledger.RouteSlotID(rt.selfdevRouteOwnerID, rt.selfdevRouteDesktopID)
	if err != nil {
		return empty, fmt.Errorf("self-development checkpoint: served SPA is underivable")
	}
	resolution, err := rt.selfdevRoute.ResolveComputerVersionRoute(ctx, slotID)
	if err != nil {
		return empty, fmt.Errorf("self-development checkpoint: served SPA is underivable")
	}
	manifest, err = updater.BuildBaselineManifest(baselineRoot, computerID, string(resolution.Slot.Current.CodeRef), string(resolution.Slot.Current.ArtifactProgramRef))
	if err != nil {
		return empty, fmt.Errorf("self-development checkpoint: served SPA is underivable")
	}
	importRequest := updater.BaselineImportRequest{
		ComputerID: computerID, RealizationID: rt.selfdevRealizationID,
		IdempotencyKey: "checkpoint-baseline-" + computerID, SourceDir: baselineRoot, Manifest: manifest,
	}
	importRequest.RequestCommitment, err = updater.ComputeBaselineImportCommitment(importRequest)
	if err != nil {
		return empty, fmt.Errorf("self-development checkpoint: served SPA is underivable")
	}
	manifest, err = rt.selfdevUpdater.ImportBaseline(ctx, importRequest)
	if err != nil {
		return empty, fmt.Errorf("self-development checkpoint: served SPA is underivable")
	}
	return manifest, nil
}
func (rt *Runtime) verifyPinnedFrontend(checkpoint selfdevprotocol.Checkpoint) (string, error) {
	if rt == nil || strings.TrimSpace(rt.selfdevUpdaterRoot) == "" {
		return "", fmt.Errorf("rematerialize: served SPA is underivable")
	}
	identity, err := pinnedFrontendIdentity(rt.selfdevUpdaterRoot, checkpoint.Request.ReleaseDigest)
	if err != nil {
		return "", err
	}
	if identity != checkpoint.Request.FrontendIdentity {
		return "", fmt.Errorf("rematerialize: restaged SPA does not match checkpoint")
	}
	current, err := updater.ReadCurrentManifest(rt.selfdevUpdaterRoot)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("rematerialize: served SPA is underivable")
	}
	return current.ContentDigest, nil
}

func pinnedFrontendIdentity(root, releaseDigest string) (selfdevprotocol.FrontendIdentity, error) {
	manifest, _, err := updater.ReadPinnedManifest(root, releaseDigest)
	if err != nil {
		return selfdevprotocol.FrontendIdentity{}, fmt.Errorf("rematerialize: served SPA is underivable")
	}
	files := make([]selfdevprotocol.ReleaseFile, 0, len(manifest.Files))
	for _, file := range manifest.Files {
		files = append(files, selfdevprotocol.ReleaseFile{Path: file.Path, SHA256: file.SHA256})
	}
	identity, err := selfdevprotocol.FrontendIdentityFromReleaseFiles(releaseDigest, files)
	if err != nil {
		return selfdevprotocol.FrontendIdentity{}, err
	}
	return identity, nil
}
