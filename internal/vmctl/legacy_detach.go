package vmctl

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/routeledger"
)

const legacyInventoryDigestBytes = sha256.Size

// LegacyOwnershipDetachRequest binds a one-time legacy ownership detach to the
// exact inventory row reviewed before fleet cutover. Detach preserves the VM
// state directory and removes only its routable ownership registration.
type LegacyOwnershipDetachRequest struct {
	RouteSlotID     string                    `json:"route_slot_id"`
	VMID            string                    `json:"vm_id"`
	ExpectedState   VMState                   `json:"expected_state"`
	ExpectedEpoch   int64                     `json:"expected_epoch"`
	InventorySHA256 string                    `json:"inventory_sha256"`
	Authorization   LegacyDetachAuthorization `json:"authorization"`
}

// LegacyDetachAuthorization is the signed owner/gate authority for one exact
// detach. The configured promotion authority key verifies the same protected
// operator boundary used for route transitions.
type LegacyDetachAuthorization struct {
	RouteSlotID     string    `json:"route_slot_id"`
	VMID            string    `json:"vm_id"`
	ExpectedState   VMState   `json:"expected_state"`
	ExpectedEpoch   int64     `json:"expected_epoch"`
	InventorySHA256 string    `json:"inventory_sha256"`
	Decision        string    `json:"decision"`
	KeyID           string    `json:"key_id"`
	AuthorizedAt    time.Time `json:"authorized_at"`
	Signature       string    `json:"signature"`
}

func (a LegacyDetachAuthorization) SigningPayload() ([]byte, error) {
	a.Signature = ""
	return json.Marshal(a)
}

func (a LegacyDetachAuthorization) verify(publicKey ed25519.PublicKey, request LegacyOwnershipDetachRequest) error {
	if len(publicKey) != ed25519.PublicKeySize || a.Decision != "detach" || a.RouteSlotID != request.RouteSlotID || a.VMID != request.VMID || a.ExpectedState != request.ExpectedState || a.ExpectedEpoch != request.ExpectedEpoch || a.InventorySHA256 != strings.ToLower(strings.TrimSpace(request.InventorySHA256)) || strings.TrimSpace(a.KeyID) == "" || a.AuthorizedAt.IsZero() {
		return fmt.Errorf("vmctl legacy detach: signed authorization bindings are invalid")
	}
	signature, err := base64.StdEncoding.DecodeString(a.Signature)
	if err != nil || len(signature) != ed25519.SignatureSize {
		return fmt.Errorf("vmctl legacy detach: authorization signature is invalid")
	}
	payload, err := a.SigningPayload()
	if err != nil || !ed25519.Verify(publicKey, payload, signature) {
		return fmt.Errorf("vmctl legacy detach: authorization signature verification failed")
	}
	return nil
}

// LegacyOwnershipDetachReceipt is the restart-durable authority for restoring
// a detached legacy ownership before its first ComputerVersion route exists.
type LegacyOwnershipDetachReceipt struct {
	ID              string                    `json:"receipt_id"`
	RouteSlotID     string                    `json:"route_slot_id"`
	InventorySHA256 string                    `json:"inventory_sha256"`
	Ownership       VMOwnership               `json:"ownership"`
	PriorState      VMState                   `json:"prior_state"`
	DetachedAt      time.Time                 `json:"detached_at"`
	StatePreserved  bool                      `json:"state_preserved"`
	Authorization   LegacyDetachAuthorization `json:"authorization"`
}

type restoreLegacyOwnershipRequest struct {
	Receipt LegacyOwnershipDetachReceipt `json:"receipt"`
}

func validLegacyInventoryDigest(value string) bool {
	decoded, err := hex.DecodeString(strings.TrimSpace(value))
	return err == nil && len(decoded) == legacyInventoryDigestBytes
}

func (r LegacyOwnershipDetachReceipt) payload() ([]byte, error) {
	r.ID = ""
	return json.Marshal(r)
}

func (r LegacyOwnershipDetachReceipt) Validate() error {
	ownerID, desktopID, err := routeledger.ParseRouteSlotID(r.RouteSlotID)
	if err != nil || strings.TrimSpace(r.ID) == "" || !validLegacyInventoryDigest(r.InventorySHA256) || r.DetachedAt.IsZero() || !r.StatePreserved {
		return fmt.Errorf("vmctl legacy detach: receipt bindings are incomplete")
	}
	if r.Ownership.UserID != ownerID || normalizeDesktopID(r.Ownership.DesktopID) != desktopID || strings.TrimSpace(r.Ownership.VMID) == "" || r.Ownership.Epoch <= 0 {
		return fmt.Errorf("vmctl legacy detach: receipt ownership does not match route")
	}
	if r.Authorization.RouteSlotID != r.RouteSlotID || r.Authorization.VMID != r.Ownership.VMID || r.Authorization.ExpectedState != r.PriorState || r.Authorization.ExpectedEpoch != r.Ownership.Epoch || r.Authorization.InventorySHA256 != r.InventorySHA256 || r.Authorization.Decision != "detach" || strings.TrimSpace(r.Authorization.KeyID) == "" || r.Authorization.AuthorizedAt.IsZero() || strings.TrimSpace(r.Authorization.Signature) == "" {
		return fmt.Errorf("vmctl legacy detach: receipt authorization does not match ownership")
	}
	if r.Ownership.ConstructionVersion != nil || r.Ownership.ConstructionDisk != nil || r.Ownership.ConstructionCommitted || r.Ownership.SnapshotKind == "constructed-computer-version" {
		return fmt.Errorf("vmctl legacy detach: constructed ownership cannot be represented as legacy")
	}
	payload, err := r.payload()
	if err != nil {
		return fmt.Errorf("vmctl legacy detach: encode receipt: %w", err)
	}
	digest := sha256.Sum256(payload)
	if r.ID != "legacy-detach:sha256:"+hex.EncodeToString(digest[:]) {
		return fmt.Errorf("vmctl legacy detach: receipt digest is invalid")
	}
	return nil
}

func newLegacyOwnershipDetachReceipt(routeSlotID, inventorySHA256 string, authorization LegacyDetachAuthorization, ownership VMOwnership, priorState VMState, detachedAt time.Time) (LegacyOwnershipDetachReceipt, error) {
	ownership.SandboxCredential = ""
	receipt := LegacyOwnershipDetachReceipt{
		RouteSlotID: routeSlotID, InventorySHA256: strings.ToLower(strings.TrimSpace(inventorySHA256)),
		Ownership: ownership, PriorState: priorState, DetachedAt: detachedAt.UTC(), StatePreserved: true, Authorization: authorization,
	}
	payload, err := receipt.payload()
	if err != nil {
		return LegacyOwnershipDetachReceipt{}, err
	}
	digest := sha256.Sum256(payload)
	receipt.ID = "legacy-detach:sha256:" + hex.EncodeToString(digest[:])
	return receipt, receipt.Validate()
}

func legacyDetachMatchesRequest(receipt LegacyOwnershipDetachReceipt, request LegacyOwnershipDetachRequest) bool {
	return receipt.RouteSlotID == request.RouteSlotID && receipt.Ownership.VMID == request.VMID &&
		receipt.PriorState == request.ExpectedState && receipt.Ownership.Epoch == request.ExpectedEpoch &&
		receipt.InventorySHA256 == strings.ToLower(strings.TrimSpace(request.InventorySHA256)) && reflect.DeepEqual(receipt.Authorization, request.Authorization)
}

func (r *OwnershipRegistry) detachLegacyOwnershipExact(request LegacyOwnershipDetachRequest, now time.Time) (LegacyOwnershipDetachReceipt, error) {
	ownerID, desktopID, err := routeledger.ParseRouteSlotID(request.RouteSlotID)
	if err != nil || strings.TrimSpace(request.VMID) == "" || request.ExpectedState == "" || request.ExpectedEpoch <= 0 || !validLegacyInventoryDigest(request.InventorySHA256) || now.IsZero() {
		return LegacyOwnershipDetachReceipt{}, fmt.Errorf("vmctl legacy detach: exact inventory preconditions are required")
	}
	normalizedSlotID, _ := routeledger.RouteSlotID(ownerID, desktopID)
	request.RouteSlotID = normalizedSlotID
	request.VMID = strings.TrimSpace(request.VMID)
	request.InventorySHA256 = strings.ToLower(strings.TrimSpace(request.InventorySHA256))

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.persistencePath == "" {
		return LegacyOwnershipDetachReceipt{}, fmt.Errorf("vmctl legacy detach: durable ownership persistence is required")
	}
	if r.detachedLegacy == nil {
		r.detachedLegacy = make(map[string]LegacyOwnershipDetachReceipt)
	}
	for _, detached := range r.detachedLegacy {
		if legacyDetachMatchesRequest(detached, request) {
			return detached, nil
		}
	}
	own := r.vmByID[request.VMID]
	if own == nil || own.UserID != ownerID || normalizeDesktopID(own.DesktopID) != desktopID || own.State != request.ExpectedState || own.Epoch != request.ExpectedEpoch {
		return LegacyOwnershipDetachReceipt{}, fmt.Errorf("vmctl legacy detach: ownership changed from frozen inventory")
	}
	if own.ConstructionVersion != nil || own.ConstructionDisk != nil || own.ConstructionCommitted || own.SnapshotKind == "constructed-computer-version" {
		return LegacyOwnershipDetachReceipt{}, fmt.Errorf("vmctl legacy detach: constructed ownership requires ComputerVersion disposal")
	}
	if own.State == VMStateBooting || own.State == VMStateStopping {
		return LegacyOwnershipDetachReceipt{}, fmt.Errorf("vmctl legacy detach: transitional ownership state is not detachable")
	}
	priorState := own.State
	if r.vmManager != nil && (own.State == VMStateActive || own.State == VMStateDegraded) {
		if err := r.vmManager.StopVM(own.VMID); err != nil {
			return LegacyOwnershipDetachReceipt{}, fmt.Errorf("vmctl legacy detach: stop VM: %w", err)
		}
		own.State = VMStateStopped
		own.StoppedBy = "legacy-detach"
	}
	receipt, err := newLegacyOwnershipDetachReceipt(request.RouteSlotID, request.InventorySHA256, request.Authorization, *own, priorState, now)
	if err != nil {
		return LegacyOwnershipDetachReceipt{}, err
	}
	key := ownershipKey(ownerID, desktopID)
	isWorker := own.Kind == VMKindWorker && strings.TrimSpace(own.WorkerID) != ""
	if isWorker {
		delete(r.workerVMs, own.WorkerID)
	} else {
		if r.ownerships[key] != own {
			return LegacyOwnershipDetachReceipt{}, fmt.Errorf("vmctl legacy detach: ownership index is inconsistent")
		}
		delete(r.ownerships, key)
	}
	delete(r.vmByID, own.VMID)
	r.detachedLegacy[receipt.ID] = receipt
	if err := r.writePersistenceLocked(); err != nil {
		delete(r.detachedLegacy, receipt.ID)
		r.vmByID[own.VMID] = own
		if isWorker {
			r.workerVMs[own.WorkerID] = own
		} else {
			r.ownerships[key] = own
		}
		return LegacyOwnershipDetachReceipt{}, fmt.Errorf("vmctl legacy detach: persist detach: %w", err)
	}
	return receipt, nil
}

func (r *OwnershipRegistry) restoreLegacyOwnershipExact(receipt LegacyOwnershipDetachReceipt) (*VMOwnership, error) {
	if err := receipt.Validate(); err != nil {
		return nil, err
	}
	ownerID, desktopID, _ := routeledger.ParseRouteSlotID(receipt.RouteSlotID)
	key := ownershipKey(ownerID, desktopID)

	r.mu.Lock()
	defer r.mu.Unlock()
	if existing := r.vmByID[receipt.Ownership.VMID]; existing != nil {
		if existing.UserID == ownerID && normalizeDesktopID(existing.DesktopID) == desktopID && reflect.DeepEqual(*existing, receipt.Ownership) {
			return cloneOwnership(existing), nil
		}
		return nil, fmt.Errorf("vmctl legacy restore: VM identity is already registered")
	}
	stored, ok := r.detachedLegacy[receipt.ID]
	if !ok || !reflect.DeepEqual(stored, receipt) {
		return nil, fmt.Errorf("vmctl legacy restore: detach receipt is not durably registered")
	}
	if _, exists := r.ownerships[key]; exists {
		return nil, fmt.Errorf("vmctl legacy restore: owner desktop already has a replacement")
	}
	restored := receipt.Ownership
	restored.SandboxCredential = ""
	ptr := &restored
	isWorker := restored.Kind == VMKindWorker && strings.TrimSpace(restored.WorkerID) != ""
	if isWorker {
		if _, exists := r.workerVMs[restored.WorkerID]; exists {
			return nil, fmt.Errorf("vmctl legacy restore: worker identity already exists")
		}
		r.workerVMs[restored.WorkerID] = ptr
	} else {
		r.ownerships[key] = ptr
	}
	r.vmByID[restored.VMID] = ptr
	delete(r.detachedLegacy, receipt.ID)
	if err := r.writePersistenceLocked(); err != nil {
		r.detachedLegacy[receipt.ID] = receipt
		delete(r.vmByID, restored.VMID)
		if isWorker {
			delete(r.workerVMs, restored.WorkerID)
		} else {
			delete(r.ownerships, key)
		}
		return nil, fmt.Errorf("vmctl legacy restore: persist restore: %w", err)
	}
	return cloneOwnership(ptr), nil
}

func (a *RouteAuthority) detachLegacyOwnership(ctx context.Context, registry *OwnershipRegistry, request LegacyOwnershipDetachRequest, now time.Time) (LegacyOwnershipDetachReceipt, error) {
	if a == nil || a.ledger == nil || registry == nil {
		return LegacyOwnershipDetachReceipt{}, fmt.Errorf("vmctl legacy detach: route authority and registry are required")
	}
	if err := request.Authorization.verify(a.promotionKey, request); err != nil {
		return LegacyOwnershipDetachReceipt{}, err
	}
	a.mutationMu.Lock()
	defer a.mutationMu.Unlock()
	if _, _, err := a.ledger.Resolve(ctx, request.RouteSlotID); !errors.Is(err, routeledger.ErrSlotNotFound) {
		if err == nil {
			return LegacyOwnershipDetachReceipt{}, fmt.Errorf("vmctl legacy detach: route slot is already present")
		}
		return LegacyOwnershipDetachReceipt{}, fmt.Errorf("vmctl legacy detach: resolve route absence: %w", err)
	}
	receipt, err := registry.detachLegacyOwnershipExact(request, now)
	if err != nil {
		return LegacyOwnershipDetachReceipt{}, err
	}
	if _, _, err := a.ledger.Resolve(ctx, request.RouteSlotID); !errors.Is(err, routeledger.ErrSlotNotFound) {
		return LegacyOwnershipDetachReceipt{}, fmt.Errorf("vmctl legacy detach: route absence changed during detach")
	}
	return receipt, nil
}

func (a *RouteAuthority) restoreLegacyOwnership(ctx context.Context, registry *OwnershipRegistry, receipt LegacyOwnershipDetachReceipt) (*VMOwnership, error) {
	if a == nil || a.ledger == nil || registry == nil {
		return nil, fmt.Errorf("vmctl legacy restore: route authority and registry are required")
	}
	a.mutationMu.Lock()
	defer a.mutationMu.Unlock()
	if _, _, err := a.ledger.Resolve(ctx, receipt.RouteSlotID); !errors.Is(err, routeledger.ErrSlotNotFound) {
		if err == nil {
			return nil, fmt.Errorf("vmctl legacy restore: route slot is already present")
		}
		return nil, fmt.Errorf("vmctl legacy restore: resolve route absence: %w", err)
	}
	return registry.restoreLegacyOwnershipExact(receipt)
}

func (h *Handler) HandleDetachLegacyComputerVersionOwnership(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeVMCTLJSON(w, http.StatusMethodNotAllowed, vmctlErrorResponse{Error: "method not allowed"})
		return
	}
	if !isInternalCaller(r) || h.routeAuthority == nil {
		writeVMCTLJSON(w, http.StatusForbidden, vmctlErrorResponse{Error: "legacy detach unavailable"})
		return
	}
	var request LegacyOwnershipDetachRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: "invalid legacy detach request"})
		return
	}
	receipt, err := h.routeAuthority.detachLegacyOwnership(r.Context(), h.registry, request, time.Now().UTC())
	if err != nil {
		writeVMCTLJSON(w, http.StatusConflict, vmctlErrorResponse{Error: err.Error()})
		return
	}
	writeVMCTLJSON(w, http.StatusOK, receipt)
}

func (h *Handler) HandleRestoreLegacyComputerVersionOwnership(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeVMCTLJSON(w, http.StatusMethodNotAllowed, vmctlErrorResponse{Error: "method not allowed"})
		return
	}
	if !isInternalCaller(r) || h.routeAuthority == nil {
		writeVMCTLJSON(w, http.StatusForbidden, vmctlErrorResponse{Error: "legacy restore unavailable"})
		return
	}
	var request restoreLegacyOwnershipRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: "invalid legacy restore request"})
		return
	}
	ownership, err := h.routeAuthority.restoreLegacyOwnership(r.Context(), h.registry, request.Receipt)
	if err != nil {
		writeVMCTLJSON(w, http.StatusConflict, vmctlErrorResponse{Error: err.Error()})
		return
	}
	writeVMCTLJSON(w, http.StatusOK, ownership)
}
