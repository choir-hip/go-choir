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
	rt.store = nil
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

	report.ComputerID = computerID
	report.CheckpointDigest = checkpoint.Digest
	report.QuarantineDir = quarantineDir
	report.QuarantinedMarker = quarantinedMarker
	report.QuarantinedWorkspace = quarantinedWorkspace
	report.WitnessMatched = true
	report.OriginalDenied = true
	report.FrontendRestaged = true
	report.StoreClosed = true
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
