package vmctl

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/hostauth"
	"github.com/yusefmosiah/go-choir/internal/recoveryplan"
)

// Durable recovery admission constants.
const (
	recoveryConditionsSchemaVersion = 1
	// recoveryRetryAfterSeconds is the product-visible Retry-After for a known
	// blocked computer. Re-evaluation is input-driven, not cooldown-driven.
	recoveryRetryAfterSeconds = 60
	// recoveryAdmissionTimeout bounds the corpusd evidence reads that precede
	// an admission decision; a known blocked computer must answer well inside
	// a second on loopback.
	recoveryAdmissionTimeout = 5 * time.Second
)

// RecoveryRefusalKind classifies a durable recovery refusal. Tail excess and a
// missing base are deterministic planner refusals; metadata unavailability is
// an observation failure and must never be reported as tail excess.
type RecoveryRefusalKind string

const (
	RecoveryRefusalTailExcess          RecoveryRefusalKind = "recovery_tail_excess"
	RecoveryRefusalBaseMissing         RecoveryRefusalKind = "projection_base_missing"
	RecoveryRefusalGuestRefused        RecoveryRefusalKind = "guest_recovery_refused"
	RecoveryRefusalMetadataUnavailable RecoveryRefusalKind = "recovery_metadata_unavailable"
	// RecoveryRefusalPrivacyKeyUnavailable: the realization cannot obtain the
	// computer's privacy key. A fresh realization of an existing chain has no
	// key source until escrow delivery lands (station SH, O21).
	RecoveryRefusalPrivacyKeyUnavailable RecoveryRefusalKind = "privacy_key_unavailable"
	// RecoveryRefusalGuestStartupFailed: the guest hit a fatal startup error
	// outside the recovery planner and reported it before readiness.
	RecoveryRefusalGuestStartupFailed RecoveryRefusalKind = "guest_startup_failed"
)

// privacyKeyRetryAfterSeconds: waiting does not supply a key; the hint only
// paces clients while an operator acts.
const privacyKeyRetryAfterSeconds = 300

// RecoveryInputWitness freezes the recovery inputs an admission decision was
// made on: the guest-measured local sequence (when known), the advertised
// watermark/base, and the frozen canonical target.
type RecoveryInputWitness struct {
	LocalSequence     uint64 `json:"local_sequence"`
	WatermarkSequence uint64 `json:"watermark_sequence"`
	TargetSequence    uint64 `json:"target_sequence"`
	BaseRef           string `json:"base_ref,omitempty"`
	EmptyStore        bool   `json:"empty_store,omitempty"`
	ChainExists       bool   `json:"chain_exists"`
	HasLocalWitness   bool   `json:"has_local_witness"`
	// FreshRealization marks a refusal vmctl made before any boot of a new
	// realization. The refused ownership is retained, so a later start of it
	// is still a fresh realization even though it takes the retained path.
	FreshRealization bool `json:"fresh_realization,omitempty"`
}

// RecoveryCondition is the durable typed recovery state for one computer. It
// is persisted separately from process lifecycle: a vmctl restart reloads it
// and resumes refusing deterministic over-cap starts without a boot attempt.
type RecoveryCondition struct {
	ComputerID      string               `json:"computer_id"`
	OwnerID         string               `json:"owner_id,omitempty"`
	Kind            RecoveryRefusalKind  `json:"kind"`
	Reason          string               `json:"reason"`
	Witness         RecoveryInputWitness `json:"witness"`
	ObservedAt      time.Time            `json:"observed_at"`
	LastEvaluatedAt time.Time            `json:"last_evaluated_at,omitempty"`
}

// recoveryConditionsFile is the persisted document.
type recoveryConditionsFile struct {
	SchemaVersion int                  `json:"schema_version"`
	Conditions    []*RecoveryCondition `json:"conditions"`
}

// RecoveryRefusal is the typed refusal returned by the admission gate. It is
// an error so every start path fails fast; handlers render it as a structured
// HTTP 503 with Retry-After.
type RecoveryRefusal struct {
	ComputerID        string
	Kind              RecoveryRefusalKind
	Reason            string
	Witness           RecoveryInputWitness
	RetryAfterSeconds int
}

func (e *RecoveryRefusal) Error() string {
	if e == nil {
		return "computer recovery blocked"
	}
	return fmt.Sprintf("computer recovery blocked (%s): %s", e.Kind, e.Reason)
}

// RecoveryRefusalFrom extracts a typed refusal from a wrapped error chain.
func RecoveryRefusalFrom(err error) *RecoveryRefusal {
	if err == nil {
		return nil
	}
	var refusal *RecoveryRefusal
	if errors.As(err, &refusal) {
		return refusal
	}
	return nil
}

// GuestBootRefusalInput is the guest planner refusal witness recorded through
// the structural guestBootRefusal surface of a VM-manager boot error.
type GuestBootRefusalInput struct {
	ComputerID        string
	OwnerID           string
	Kind              string
	Reason            string
	LocalSequence     uint64
	WatermarkSequence uint64
	TargetSequence    uint64
	EmptyStore        bool
	ChainExists       bool
}

// RecoveryAdmissionConfig configures the durable recovery breaker.
type RecoveryAdmissionConfig struct {
	// StatePath is the durable conditions file. Required for enabling the gate.
	StatePath string
	// CorpusdURL serves the canonical head and advertised watermark reads.
	CorpusdURL string
}

// recoveryEvidence is one canonical recovery-input observation.
type recoveryEvidence struct {
	ChainExists       bool
	TargetSequence    uint64
	WatermarkSequence uint64
	BaseRef           string
	HasWatermark      bool
	// EscrowPresent: the platform holds a custodian escrow of the computer's
	// privacy key, so a fresh realization can receive it (SH slice 3, O21).
	EscrowPresent bool
}

// recoveryEvidenceReader reads canonical recovery inputs. It never writes.
type recoveryEvidenceReader interface {
	Evidence(ctx context.Context, computerID, ownerID string) (recoveryEvidence, error)
}

// RecoveryAdmission is the durable retained-aware recovery breaker: it decides
// whether a realization may start and persists typed conditions. It never
// performs replay.
type RecoveryAdmission struct {
	mu         sync.Mutex
	path       string
	reader     recoveryEvidenceReader
	conditions map[string]*RecoveryCondition

	now func() time.Time
}

// NewRecoveryAdmission loads the durable conditions file and returns the gate.
func NewRecoveryAdmission(cfg RecoveryAdmissionConfig) (*RecoveryAdmission, error) {
	path := strings.TrimSpace(cfg.StatePath)
	if path == "" {
		return nil, fmt.Errorf("recovery admission: state path is required")
	}
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.CorpusdURL), "/")
	if baseURL == "" {
		return nil, fmt.Errorf("recovery admission: corpusd URL is required")
	}
	a := &RecoveryAdmission{
		path:       path,
		reader:     httpRecoveryEvidence{baseURL: baseURL, client: &http.Client{Timeout: recoveryAdmissionTimeout}},
		conditions: map[string]*RecoveryCondition{},
		now:        func() time.Time { return time.Now().UTC() },
	}
	if err := a.load(); err != nil {
		return nil, err
	}
	return a, nil
}

func (a *RecoveryAdmission) load() error {
	data, err := os.ReadFile(a.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("recovery admission: read conditions: %w", err)
	}
	var file recoveryConditionsFile
	if err := json.Unmarshal(data, &file); err != nil {
		return fmt.Errorf("recovery admission: decode conditions: %w", err)
	}
	if file.SchemaVersion != recoveryConditionsSchemaVersion {
		return fmt.Errorf("recovery admission: unsupported conditions schema %d", file.SchemaVersion)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, condition := range file.Conditions {
		if condition == nil || strings.TrimSpace(condition.ComputerID) == "" {
			continue
		}
		a.conditions[condition.ComputerID] = condition
	}
	return nil
}

// persistLocked writes the durable conditions file atomically. Callers hold a.mu.
func (a *RecoveryAdmission) persistLocked() error {
	file := recoveryConditionsFile{SchemaVersion: recoveryConditionsSchemaVersion}
	for _, condition := range a.conditions {
		file.Conditions = append(file.Conditions, condition)
	}
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(a.path), 0o755); err != nil {
		return err
	}
	tmp := a.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, a.path)
}

// saveLocked persists the durable conditions and logs failures, mirroring the
// ownership registry: a lost condition file means a boot-loop can return.
func (a *RecoveryAdmission) saveLocked() {
	if err := a.persistLocked(); err != nil {
		log.Printf("vmctl: persist recovery conditions: %v", err)
	}
}

// Condition returns a snapshot of the durable condition for one computer.
func (a *RecoveryAdmission) Condition(computerID string) (*RecoveryCondition, bool) {
	if a == nil {
		return nil, false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	condition, ok := a.conditions[strings.TrimSpace(computerID)]
	if !ok || condition == nil {
		return nil, false
	}
	return cloneRecoveryCondition(condition), true
}

// Clear drops the durable condition (recovery inputs changed, or a realization
// started successfully).
func (a *RecoveryAdmission) Clear(computerID string) {
	if a == nil {
		return
	}
	computerID = strings.TrimSpace(computerID)
	if computerID == "" {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, ok := a.conditions[computerID]; !ok {
		return
	}
	delete(a.conditions, computerID)
	a.saveLocked()
}

func (a *RecoveryAdmission) storeCondition(condition *RecoveryCondition) {
	if a == nil || condition == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.conditions[condition.ComputerID] = cloneRecoveryCondition(condition)
	a.saveLocked()
}

// mergeReevaluation applies a re-evaluation to the live stored condition and
// returns the merged clone. Only Kind, Reason, Witness and LastEvaluatedAt come
// from the re-evaluation; the stored fields it does not name stay as they are.
// It returns nil, storing nothing, when
// the condition was cleared while the evidence was being read.
func (a *RecoveryAdmission) mergeReevaluation(updated *RecoveryCondition) *RecoveryCondition {
	if a == nil || updated == nil {
		return nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	current := a.conditions[updated.ComputerID]
	if current == nil {
		return nil
	}
	current.Kind = updated.Kind
	current.Reason = updated.Reason
	current.Witness = updated.Witness
	current.LastEvaluatedAt = updated.LastEvaluatedAt
	a.saveLocked()
	return cloneRecoveryCondition(current)
}

// Admit decides whether a realization start may proceed for one computer.
//
//   - A durable condition is re-evaluated against fresh canonical evidence.
//     A verified local witness re-runs the shared PlanRecovery; without one,
//     only a changed seed dependency (watermark/base_ref) reopens the start.
//     Head growth alone never makes a blocked computer retriable.
//   - Without a condition, only a provably empty store (fresh assignment) may
//     refuse before boot; retained stores with unknown local state are left to
//     the guest planner, so stale W never strands a valid retained store.
//   - Unverifiable metadata is reported distinctly and never misclassified as
//     tail excess.
func (a *RecoveryAdmission) Admit(ctx context.Context, computerID, ownerID string, emptyStore bool) *RecoveryRefusal {
	if a == nil {
		return nil
	}
	computerID = strings.TrimSpace(computerID)
	if computerID == "" {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ownerID = strings.TrimSpace(ownerID)

	a.mu.Lock()
	condition := cloneRecoveryCondition(a.conditions[computerID])
	a.mu.Unlock()
	if condition != nil && strings.TrimSpace(condition.OwnerID) != "" {
		ownerID = condition.OwnerID
	}

	var evidence recoveryEvidence
	var evidenceErr error
	evidenceRead := false
	readEvidence := func() (recoveryEvidence, error) {
		if !evidenceRead {
			evidence, evidenceErr = a.reader.Evidence(ctx, computerID, ownerID)
			evidenceRead = true
		}
		return evidence, evidenceErr
	}

	if condition != nil {
		observed, err := readEvidence()
		if err != nil {
			// Metadata unavailability is distinct from a planner refusal; the
			// durable condition stays untouched and keeps blocking.
			return &RecoveryRefusal{
				ComputerID:        computerID,
				Kind:              RecoveryRefusalMetadataUnavailable,
				Reason:            fmt.Sprintf("recovery evidence unavailable (%v); last known condition: %s", err, condition.Reason),
				Witness:           condition.Witness,
				RetryAfterSeconds: recoveryRetryAfterSeconds,
			}
		}
		allowed, updated := reevaluateRecoveryCondition(a.now(), condition, observed)
		if !allowed {
			// The snapshot predates the evidence read. Merge the re-evaluated
			// fields onto the live condition rather than storing the snapshot
			// over it.
			if merged := a.mergeReevaluation(updated); merged != nil {
				updated = merged
			}
			return refusalFromCondition(updated)
		}
		emptyStore = emptyStore || condition.Witness.FreshRealization
		if !emptyStore {
			a.Clear(computerID)
			return nil
		}
	}
	if !emptyStore {
		return nil
	}

	// For a fresh realization the old condition is replaced, never cleared
	// first: a crash between a clear and the next store would lose the
	// fresh-realization witness and let the retry boot as retained.
	priorCondition := condition != nil
	observed, err := readEvidence()
	if err != nil {
		// No verifiable inputs: preserve the guest planner for races and
		// unknown local state.
		if priorCondition {
			a.Clear(computerID)
		}
		return nil
	}
	refusal := deterministicEmptyStoreRefusal(computerID, ownerID, observed)
	if refusal == nil {
		// Recovery inputs admit the install, but a provably empty store over
		// an existing chain is a fresh realization of an existing computer:
		// its privacy key lived on a previous realization's data image, so
		// the start cannot succeed. Refuse durably before boot rather than
		// crash-loop to the readiness deadline.
		refusal = privacyKeyUnavailableRefusal(computerID, observed)
	}
	if refusal == nil {
		if priorCondition {
			a.Clear(computerID)
		}
		return nil
	}
	condition = &RecoveryCondition{
		ComputerID:      refusal.ComputerID,
		OwnerID:         ownerID,
		Kind:            refusal.Kind,
		Reason:          refusal.Reason,
		Witness:         refusal.Witness,
		ObservedAt:      a.now(),
		LastEvaluatedAt: a.now(),
	}
	a.storeCondition(condition)
	return refusalFromCondition(condition)
}

// RecordGuestRefusal persists a typed guest planner refusal as a durable
// condition when it is deterministic (tail excess / missing base / missing
// privacy key) and returns
// the refusal for prompt propagation. Other guest failures stay prompt but not
// durable: they may clear on the next start attempt.
func (a *RecoveryAdmission) RecordGuestRefusal(input GuestBootRefusalInput) *RecoveryRefusal {
	computerID := strings.TrimSpace(input.ComputerID)
	if a == nil || computerID == "" {
		return nil
	}
	kind := RecoveryRefusalGuestRefused
	switch strings.ToLower(strings.TrimSpace(input.Kind)) {
	case "tail_excess":
		kind = RecoveryRefusalTailExcess
	case "base_missing":
		kind = RecoveryRefusalBaseMissing
	case "privacy_key_unavailable":
		kind = RecoveryRefusalPrivacyKeyUnavailable
	case "startup_failed":
		kind = RecoveryRefusalGuestStartupFailed
	}
	refusal := &RecoveryRefusal{
		ComputerID: computerID,
		Kind:       kind,
		Reason:     input.Reason,
		Witness: RecoveryInputWitness{
			LocalSequence:     input.LocalSequence,
			WatermarkSequence: input.WatermarkSequence,
			TargetSequence:    input.TargetSequence,
			EmptyStore:        input.EmptyStore,
			ChainExists:       input.ChainExists,
			HasLocalWitness:   kind == RecoveryRefusalTailExcess || kind == RecoveryRefusalBaseMissing,
		},
		RetryAfterSeconds: recoveryRetryAfterSeconds,
	}
	if kind == RecoveryRefusalPrivacyKeyUnavailable {
		refusal.RetryAfterSeconds = privacyKeyRetryAfterSeconds
	}
	if kind != RecoveryRefusalTailExcess && kind != RecoveryRefusalBaseMissing && kind != RecoveryRefusalPrivacyKeyUnavailable {
		return refusal
	}
	condition := &RecoveryCondition{
		ComputerID:      computerID,
		OwnerID:         strings.TrimSpace(input.OwnerID),
		Kind:            kind,
		Reason:          input.Reason,
		Witness:         refusal.Witness,
		ObservedAt:      a.now(),
		LastEvaluatedAt: a.now(),
	}
	a.storeCondition(condition)
	if stored, ok := a.Condition(computerID); ok {
		return refusalFromCondition(stored)
	}
	return refusal
}

// reevaluateRecoveryCondition re-runs the admission decision against fresh
// canonical evidence. Only changed recovery inputs may reopen a blocked start.
func reevaluateRecoveryCondition(now time.Time, condition *RecoveryCondition, observed recoveryEvidence) (bool, *RecoveryCondition) {
	updated := cloneRecoveryCondition(condition)
	updated.Witness.TargetSequence = observed.TargetSequence
	updated.Witness.WatermarkSequence = observed.WatermarkSequence
	updated.Witness.BaseRef = observed.BaseRef
	updated.Witness.ChainExists = observed.ChainExists
	updated.LastEvaluatedAt = now.UTC()

	if !observed.ChainExists {
		// The canonical chain is gone (re-provisioned computer): there is
		// nothing to recover and genesis is explicit, not a refusal.
		return true, updated
	}
	if condition.Kind == RecoveryRefusalPrivacyKeyUnavailable {
		// Only a key source reopens this — custodian escrow, from which the
		// realization receives the key. Recovery inputs (watermark, base,
		// head) cannot supply a key. Operator cold-recover uses the
		// maintenance path, which bypasses admission.
		return observed.EscrowPresent, updated
	}
	if condition.Witness.HasLocalWitness {
		plan, err := recoveryplan.PlanRecovery(condition.Witness.EmptyStore, condition.Witness.LocalSequence, true, observed.WatermarkSequence, observed.TargetSequence)
		if err == nil {
			return true, updated
		}
		updated.Kind = recoveryRefusalKindForPlan(observed, plan)
		updated.Reason = plan.Reason
		return false, updated
	}
	// No verified local witness: only a changed seed dependency (watermark or
	// published base) reopens the decision. Head growth alone must not.
	seedChanged := observed.WatermarkSequence != condition.Witness.WatermarkSequence ||
		strings.TrimSpace(observed.BaseRef) != strings.TrimSpace(condition.Witness.BaseRef)
	if seedChanged {
		return true, updated
	}
	return false, updated
}

// privacyKeyUnavailableRefusal refuses a fresh realization of an existing
// chain. With no chain the guest creates the key before minting genesis.
func privacyKeyUnavailableRefusal(computerID string, observed recoveryEvidence) *RecoveryRefusal {
	if !observed.ChainExists || observed.EscrowPresent {
		// No chain: the guest creates the key. Escrow: the guest receives it.
		return nil
	}
	return &RecoveryRefusal{
		ComputerID: computerID,
		Kind:       RecoveryRefusalPrivacyKeyUnavailable,
		Reason: fmt.Sprintf("fresh realization of existing computer (canonical head %d) has no privacy key source: "+
			"the platform holds no custodian escrow of its key", observed.TargetSequence),
		Witness: RecoveryInputWitness{
			WatermarkSequence: observed.WatermarkSequence,
			TargetSequence:    observed.TargetSequence,
			BaseRef:           observed.BaseRef,
			EmptyStore:        true,
			ChainExists:       true,
			FreshRealization:  true,
		},
		RetryAfterSeconds: privacyKeyRetryAfterSeconds,
	}
}

// deterministicEmptyStoreRefusal applies the shared planner to a provably
// empty store (fresh realization) with a live chain. This is the only
// pre-boot refusal: the retained-local case is decided by the guest planner.
func deterministicEmptyStoreRefusal(computerID, ownerID string, observed recoveryEvidence) *RecoveryRefusal {
	if !observed.ChainExists || observed.TargetSequence == 0 {
		return nil
	}
	plan, err := recoveryplan.PlanRecovery(true, 0, true, observed.WatermarkSequence, observed.TargetSequence)
	if err == nil {
		return nil
	}
	return &RecoveryRefusal{
		ComputerID: computerID,
		Kind:       recoveryRefusalKindForPlan(observed, plan),
		Reason:     plan.Reason,
		Witness: RecoveryInputWitness{
			LocalSequence:     0,
			WatermarkSequence: observed.WatermarkSequence,
			TargetSequence:    observed.TargetSequence,
			BaseRef:           observed.BaseRef,
			EmptyStore:        true,
			ChainExists:       true,
			HasLocalWitness:   true,
			FreshRealization:  true,
		},
		RetryAfterSeconds: recoveryRetryAfterSeconds,
	}
}

func recoveryRefusalKindForPlan(observed recoveryEvidence, plan recoveryplan.RecoveryPlan) RecoveryRefusalKind {
	if !observed.HasWatermark || plan.WatermarkSequence > plan.TargetSequence {
		return RecoveryRefusalBaseMissing
	}
	return RecoveryRefusalTailExcess
}

func retryAfterForKind(kind RecoveryRefusalKind) int {
	if kind == RecoveryRefusalPrivacyKeyUnavailable {
		return privacyKeyRetryAfterSeconds
	}
	return recoveryRetryAfterSeconds
}

func refusalFromCondition(condition *RecoveryCondition) *RecoveryRefusal {
	if condition == nil {
		return nil
	}
	return &RecoveryRefusal{
		ComputerID:        condition.ComputerID,
		Kind:              condition.Kind,
		Reason:            condition.Reason,
		Witness:           condition.Witness,
		RetryAfterSeconds: retryAfterForKind(condition.Kind),
	}
}

func cloneRecoveryCondition(condition *RecoveryCondition) *RecoveryCondition {
	if condition == nil {
		return nil
	}
	snapshot := *condition
	return &snapshot
}

// httpRecoveryEvidence reads the canonical head and advertised watermark from
// the platform's existing endpoints: the same authority the guest planner
// consumes, never a cached or derived copy.
type httpRecoveryEvidence struct {
	baseURL string
	client  *http.Client
}

func (h httpRecoveryEvidence) Evidence(ctx context.Context, computerID, ownerID string) (recoveryEvidence, error) {
	base := strings.TrimRight(strings.TrimSpace(h.baseURL), "/")
	computerID = strings.TrimSpace(computerID)
	if base == "" || computerID == "" {
		return recoveryEvidence{}, fmt.Errorf("recovery evidence: invalid binding")
	}
	client := h.client
	if client == nil {
		client = &http.Client{Timeout: recoveryAdmissionTimeout}
	}
	observed := recoveryEvidence{}

	headBody, headStatus, err := h.get(ctx, client, base+"/internal/computers/events/head?"+url.Values{"computer_id": {computerID}}.Encode(), ownerID)
	if err != nil {
		return observed, err
	}
	switch headStatus {
	case http.StatusNotFound:
		// No canonical chain: explicit bootstrap territory.
		return observed, nil
	case http.StatusOK:
		var head computerevent.Head
		if err := json.Unmarshal(headBody, &head); err != nil {
			return observed, fmt.Errorf("recovery evidence: decode canonical head: %w", err)
		}
		if head.ComputerID != computerID {
			return observed, fmt.Errorf("recovery evidence: canonical head computer mismatch")
		}
		if head.Sequence == 0 {
			return observed, nil
		}
		if !computerevent.IsSHA256(head.CanonicalEventHead) {
			return observed, fmt.Errorf("recovery evidence: invalid canonical head digest")
		}
		observed.ChainExists = true
		observed.TargetSequence = head.Sequence
		observed.EscrowPresent = h.custodianEscrowPresent(ctx, client, base, computerID, ownerID)
	default:
		return observed, fmt.Errorf("recovery evidence: canonical head status %d", headStatus)
	}

	watermarkBody, watermarkStatus, err := h.get(ctx, client, base+"/internal/computers/files/watermark?"+url.Values{"computer_id": {computerID}}.Encode(), ownerID)
	if err != nil {
		return observed, err
	}
	switch watermarkStatus {
	case http.StatusNotFound:
		return observed, nil
	case http.StatusOK:
		var watermark struct {
			WatermarkSequence uint64 `json:"watermark_sequence"`
			BaseRef           string `json:"base_ref"`
		}
		if err := json.Unmarshal(watermarkBody, &watermark); err != nil {
			return observed, fmt.Errorf("recovery evidence: decode watermark: %w", err)
		}
		if watermark.WatermarkSequence == 0 || strings.TrimSpace(watermark.BaseRef) == "" {
			return observed, fmt.Errorf("recovery evidence: platform advertised an empty base")
		}
		observed.HasWatermark = true
		observed.WatermarkSequence = watermark.WatermarkSequence
		observed.BaseRef = strings.TrimSpace(watermark.BaseRef)
		return observed, nil
	default:
		return observed, fmt.Errorf("recovery evidence: watermark status %d", watermarkStatus)
	}
}

// custodianEscrowPresent reads the platform's escrow status. Any failure
// reads as absent: the start is then refused as before, never admitted on a
// guess.
func (h httpRecoveryEvidence) custodianEscrowPresent(ctx context.Context, client *http.Client, base, computerID, ownerID string) bool {
	body, status, err := h.get(ctx, client, base+"/internal/computers/keys/escrow/status?"+url.Values{"computer_id": {computerID}}.Encode(), ownerID)
	if err != nil || status != http.StatusOK {
		return false
	}
	var parsed struct {
		Escrows []struct {
			Protector string `json:"protector"`
			KeyDigest string `json:"key_digest"`
		} `json:"escrows"`
	}
	if json.Unmarshal(body, &parsed) != nil {
		return false
	}
	for _, escrow := range parsed.Escrows {
		if escrow.Protector == "custodian" && strings.TrimSpace(escrow.KeyDigest) != "" {
			return true
		}
	}
	return false
}

func (h httpRecoveryEvidence) get(ctx context.Context, client *http.Client, endpoint, ownerID string) ([]byte, int, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, 0, err
	}
	hostauth.MarkInternal(request)
	if strings.TrimSpace(ownerID) != "" {
		request.Header.Set("X-Authenticated-User", strings.TrimSpace(ownerID))
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = response.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(response.Body, 64<<10))
	if err != nil {
		return nil, 0, err
	}
	return body, response.StatusCode, nil
}

// guestBootRefusal is implemented by VM-manager boot errors that carry the
// guest planner's typed refusal witness (vmmanager.GuestBootRefusedError).
// The structural coupling keeps vmctl free of a vmmanager import; the method
// set must match exactly on both sides.
type guestBootRefusal interface {
	GuestBootRefusal() (kind, reason string, localSequence, watermarkSequence, targetSequence uint64, emptyStore, chainExists bool)
}
