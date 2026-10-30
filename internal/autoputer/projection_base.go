package autoputer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentcore"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/recoveryplan"
	"github.com/yusefmosiah/go-choir/internal/snapshot"
	choirstore "github.com/yusefmosiah/go-choir/internal/store"
)

// Guest planner refusal kinds. The host maps these onto its durable recovery
// condition without parsing guest logs; tail_excess and base_missing are
// deterministic for an unchanged store and (W, H), the others are transient
// or operator-repair states.
const (
	BootRefusalKindTailExcess          = "tail_excess"
	BootRefusalKindBaseMissing         = "base_missing"
	BootRefusalKindBaseUnavailable     = "base_unavailable"
	BootRefusalKindRecoveryUnavailable = "recovery_unavailable"
	// BootRefusalKindPrivacyKeyUnavailable: the computer has a chain but this
	// realization has no privacy key (the key may be created only pre-genesis).
	BootRefusalKindPrivacyKeyUnavailable = "privacy_key_unavailable"
	// BootRefusalKindStartupFailed: any other fatal startup error.
	BootRefusalKindStartupFailed = "startup_failed"
)

// ProjectionBaseRefusal is the guest planner's typed refusal. It carries the
// exact (empty, local, W, H) inputs PlanRecovery decided on so the host can
// persist a durable recovery condition and serve a structured 503 instead of
// boot-looping. W is the head sequence of the snapshot the plan would start
// from (0 when the tape has none). Error wraps ErrBaseRefused so errors.Is
// keeps working. (The name predates the snapshots that replaced projection
// bases; the health schema the host parses keeps it.)
type ProjectionBaseRefusal struct {
	ComputerID        string
	Kind              string
	Reason            string
	LocalSequence     uint64
	WatermarkSequence uint64
	TargetSequence    uint64
	EmptyStore        bool
	ChainExists       bool
}

func (e *ProjectionBaseRefusal) Error() string {
	if e == nil {
		return "required snapshot refused"
	}
	return fmt.Sprintf("required snapshot refused (%s) for %s: %s (local=%d W=%d H=%d)",
		e.Kind, e.ComputerID, e.Reason, e.LocalSequence, e.WatermarkSequence, e.TargetSequence)
}

// Unwrap reports ErrBaseRefused so existing refusal checks stay valid.
func (e *ProjectionBaseRefusal) Unwrap() error { return recoveryplan.ErrBaseRefused }

// projectionBaseRefusalKind classifies a PlanRecovery refusal for the host.
// A missing or ahead-of-target snapshot is a dependency problem; anything
// else with a live chain is the tail bound.
func projectionBaseRefusalKind(watermarkSeq, targetSeq uint64) string {
	if watermarkSeq == 0 || watermarkSeq > targetSeq {
		return BootRefusalKindBaseMissing
	}
	return BootRefusalKindTailExcess
}

// materializeProjectionBaseIfNeeded restores the store from the newest
// snapshot when the computer's disk is missing, before reconstruct replays the
// tape after it. The contract:
//
//   - the store holds events (the usual boot): resume from its own head; the
//     remaining tail (local, H] must be at most MaxRecoveryTailEvents
//   - empty store + no canonical chain: explicit new-computer genesis
//   - empty store + chain: the newest snapshot_recorded on the tape is
//     fetched, unsealed and installed as the state file, then (W, H] replays;
//     with no snapshot, a chain of at most MaxRecoveryTailEvents replays from
//     genesis, a longer one refuses
//   - a legacy Dolt workspace beside an empty store belongs to the guest
//     migrator, not to this function
//
// Returns the recovery plan and whether the store was materialized; the
// caller mints the genesis event when the plan is RecoveryGenesis. The name is
// historical: there are no projection bases.
func materializeProjectionBaseIfNeeded(ctx context.Context, storePath, computerID, platformURL string, capability func(context.Context) (string, error), live *choirstore.Store) (recoveryplan.RecoveryPlan, bool, error) {
	storePath = filepath.Clean(storePath)
	storeDir := filepath.Dir(storePath)
	computerID = strings.TrimSpace(computerID)
	platformURL = strings.TrimRight(strings.TrimSpace(platformURL), "/")
	if storeDir == "" || storeDir == "." || filepath.Base(storePath) == "." || filepath.Base(storePath) == "/" || computerID == "" || platformURL == "" || capability == nil {
		return recoveryplan.RecoveryPlan{}, false, nil
	}
	sweepStagingArtifacts(storeDir)
	statePath := choirstore.StateDBPath(storePath)

	var localSeq uint64
	if live != nil {
		head, err := live.Head(ctx, computerID)
		if err != nil {
			return recoveryplan.RecoveryPlan{}, false, fmt.Errorf("%w: retained store head: %v", recoveryplan.ErrBaseRefused, err)
		}
		if head != nil {
			localSeq = head.Sequence
		}
	}
	empty := localSeq == 0

	head, err := queryCanonicalHead(ctx, platformURL, computerID, capability)
	if err != nil {
		return recoveryplan.RecoveryPlan{}, false, err
	}
	chainExists := head != nil && head.Sequence > 0
	var targetSeq uint64
	if chainExists {
		targetSeq = head.Sequence
	}
	refuse := func(kind, reason string, watermark uint64) error {
		return &ProjectionBaseRefusal{
			ComputerID:        computerID,
			Kind:              kind,
			Reason:            reason,
			LocalSequence:     localSeq,
			WatermarkSequence: watermark,
			TargetSequence:    targetSeq,
			EmptyStore:        empty,
			ChainExists:       chainExists,
		}
	}

	if !chainExists || !empty {
		// W is only a start point for an empty store; a store that holds events
		// resumes from its own head.
		plan, err := recoveryplan.PlanRecovery(empty, localSeq, chainExists, 0, targetSeq)
		if err != nil {
			return recoveryplan.RecoveryPlan{}, false, refuse(projectionBaseRefusalKind(0, targetSeq), plan.Reason, 0)
		}
		log.Printf("autoputer: recovery %s for %s (local=%d H=%d tail=%d)", plan.Action, computerID, localSeq, targetSeq, plan.TailEvents)
		return plan, false, nil
	}

	// An empty store with a chain: this computer's disk is missing. The legacy
	// branch is for a workspace that still holds a Dolt database. The bare
	// directory is not that test: store.Open always creates the .texture
	// sidecar beside the state file, so a stat of it is true on every
	// production computer and would hide snapshot install, the bounded tail and
	// the typed base_missing refusal.
	if legacyDoltWorkspaceHoldsDatabase(choirstore.LegacyDoltWorkspacePath(storePath)) {
		log.Printf("autoputer: legacy Dolt workspace present beside an empty store for %s; the guest migrator owns this boot", computerID)
		return recoveryplan.RecoveryPlan{Action: recoveryplan.RecoveryResume, TargetSequence: targetSeq, Reason: "legacy Dolt workspace awaits migration"}, false, nil
	}
	privacyKeyPath := strings.TrimSpace(os.Getenv("CHOIR_PRIVACY_KEY_FILE"))
	if privacyKeyPath == "" {
		return recoveryplan.RecoveryPlan{}, false, refuse(BootRefusalKindBaseUnavailable, "privacy key file is not configured", 0)
	}
	cipher, err := computerevent.LoadGuestPrivateArtifactCipher(privacyKeyPath, computerID, false)
	if err != nil {
		return recoveryplan.RecoveryPlan{}, false, refuse(BootRefusalKindBaseUnavailable, "privacy key unavailable: "+err.Error(), 0)
	}
	events, err := computerevent.NewGuestHTTPClient(platformURL, capability)
	if err != nil {
		return recoveryplan.RecoveryPlan{}, false, refuse(BootRefusalKindBaseUnavailable, err.Error(), 0)
	}
	chunks := snapshot.HTTPChunks{BaseURL: platformURL, ComputerID: computerID, Capability: capability}
	candidates, err := snapshot.Scan(ctx, events, computerID, nil)
	if err != nil {
		return recoveryplan.RecoveryPlan{}, false, refuse(BootRefusalKindBaseUnavailable, err.Error(), 0)
	}
	if len(candidates) == 0 {
		if targetSeq > recoveryplan.MaxRecoveryTailEvents {
			return recoveryplan.RecoveryPlan{}, false, refuse(BootRefusalKindBaseMissing, fmt.Sprintf("no snapshot on the tape and the chain of %d events exceeds the %d-event genesis replay bound", targetSeq, recoveryplan.MaxRecoveryTailEvents), 0)
		}
		// A short chain with no snapshot replays from genesis: exact, bounded.
		log.Printf("autoputer: no snapshot for %s; replaying %d events from genesis", computerID, targetSeq)
		return recoveryplan.RecoveryPlan{Action: recoveryplan.RecoveryResume, TargetSequence: targetSeq, TailEvents: targetSeq, Reason: "no snapshot; bounded genesis replay"}, false, nil
	}

	// Install the snapshot as the state file. The empty store opened by the
	// caller is closed and its (empty) files removed first.
	if live != nil {
		if err := live.Close(); err != nil {
			return recoveryplan.RecoveryPlan{}, false, fmt.Errorf("autoputer: close empty store before snapshot restore: %w", err)
		}
	}
	for _, name := range []string{statePath, statePath + "-wal", statePath + "-shm"} {
		if err := os.Remove(name); err != nil && !errors.Is(err, os.ErrNotExist) {
			return recoveryplan.RecoveryPlan{}, false, fmt.Errorf("autoputer: clear empty state before snapshot restore: %w", err)
		}
	}
	record, err := snapshot.Install(ctx, snapshot.RestoreConfig{
		ComputerID: computerID, DestPath: statePath, Source: events, Chunks: chunks, Cipher: cipher,
		MaxSequence: targetSeq,
		// The chunks are authenticated by their seal; this checks that the
		// pages agree with the witness the producer recorded. A snapshot that
		// fails falls back to the next older one.
		Verify: agentcore.VerifySnapshotFile,
	}, candidates)
	if err != nil {
		if live != nil {
			_ = live.Reopen(storePath)
		}
		return recoveryplan.RecoveryPlan{}, false, refuse(BootRefusalKindBaseUnavailable, err.Error(), 0)
	}
	if live != nil {
		if err := live.Reopen(storePath); err != nil {
			return recoveryplan.RecoveryPlan{}, false, fmt.Errorf("autoputer: reopen restored store: %w", err)
		}
	}
	plan, err := recoveryplan.PlanRecovery(true, 0, true, record.StoreHeadSequence, targetSeq)
	if err != nil {
		return recoveryplan.RecoveryPlan{}, false, refuse(projectionBaseRefusalKind(record.StoreHeadSequence, targetSeq), plan.Reason, record.StoreHeadSequence)
	}
	log.Printf("autoputer: snapshot installed at sequence %d (root %s) for target %d", record.StoreHeadSequence, record.Manifest.Root, targetSeq)
	return plan, true, nil
}

// queryCanonicalHead reads the platform canonical head. A missing head is the
// explicit bootstrap signal (nil, nil); any other failure is loud because
// boot cannot distinguish a new computer from a broken recovery without it.
func queryCanonicalHead(ctx context.Context, platformURL, computerID string, capability func(context.Context) (string, error)) (*computerevent.Head, error) {
	token, err := capability(ctx)
	if err != nil || strings.TrimSpace(token) == "" {
		return nil, fmt.Errorf("autoputer: platform capability unavailable: %w", err)
	}
	query := url.Values{"computer_id": {computerID}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, platformURL+"/internal/computers/events/head?"+query.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("autoputer: canonical head request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(token))
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("autoputer: canonical head fetch: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("autoputer: canonical head status %d", resp.StatusCode)
	}
	var head computerevent.Head
	if err := json.NewDecoder(resp.Body).Decode(&head); err != nil {
		return nil, fmt.Errorf("autoputer: decode canonical head: %w", err)
	}
	return &head, nil
}

func sweepStagingArtifacts(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, "restore-staging-") || strings.HasPrefix(name, ".snapshot-restore-") {
			_ = os.RemoveAll(filepath.Join(dir, name))
		}
	}
}
