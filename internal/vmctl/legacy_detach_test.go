package vmctl

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerversion"
	"github.com/yusefmosiah/go-choir/internal/diskinstantiation"
	"github.com/yusefmosiah/go-choir/internal/routeledger"
)

func registerLegacyOwnershipFixture(t *testing.T, registry *OwnershipRegistry, ownership VMOwnership) {
	t.Helper()
	registry.mu.Lock()
	defer registry.mu.Unlock()
	copy := ownership
	ptr := &copy
	if copy.Kind == VMKindWorker && copy.WorkerID != "" {
		registry.workerVMs[copy.WorkerID] = ptr
	} else {
		registry.ownerships[ownershipKey(copy.UserID, copy.DesktopID)] = ptr
	}
	registry.vmByID[copy.VMID] = ptr
	if err := registry.writePersistenceLocked(); err != nil {
		t.Fatal(err)
	}
}

func authorizeLegacyDetachRequest(t *testing.T, request *LegacyOwnershipDetachRequest, authorities ...*RouteAuthority) {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, authority := range authorities {
		if err := authority.SetPromotionAuthorityPublicKey(publicKey); err != nil {
			t.Fatal(err)
		}
	}
	request.Authorization = LegacyDetachAuthorization{
		RouteSlotID: request.RouteSlotID, VMID: request.VMID, ExpectedState: request.ExpectedState,
		ExpectedEpoch: request.ExpectedEpoch, InventorySHA256: request.InventorySHA256,
		Decision: "detach", KeyID: "g4-test", AuthorizedAt: time.Date(2026, 7, 17, 6, 0, 30, 0, time.UTC),
	}
	payload, err := request.Authorization.SigningPayload()
	if err != nil {
		t.Fatal(err)
	}
	request.Authorization.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(privateKey, payload))
}

func TestLegacyOwnershipDetachPersistsStateAndExactRestore(t *testing.T) {
	authority, _, _, _ := newRouteAuthorityFixture(t)
	root := t.TempDir()
	persistencePath := filepath.Join(root, "ownerships.json")
	statePath := filepath.Join(root, "vm-legacy", "data.img")
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		t.Fatal(err)
	}
	stateBytes := []byte("preserved legacy state")
	if err := os.WriteFile(statePath, stateBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	manager := &mockVMManager{}
	registry := NewOwnershipRegistry("http://sandbox")
	registry.SetVMManager(manager)
	if err := registry.SetPersistencePath(persistencePath); err != nil {
		t.Fatal(err)
	}
	createdAt := time.Date(2026, 7, 17, 6, 0, 0, 0, time.UTC)
	registerLegacyOwnershipFixture(t, registry, VMOwnership{
		VMID: "vm-legacy", UserID: "owner", DesktopID: "primary", Kind: VMKindInteractive,
		Published: true, SandboxURL: "http://guest", State: VMStateActive,
		CreatedAt: createdAt, LastActiveAt: createdAt, Epoch: 7,
	})
	slotID, _ := routeledger.RouteSlotID("owner", "primary")
	request := LegacyOwnershipDetachRequest{
		RouteSlotID: slotID, VMID: "vm-legacy", ExpectedState: VMStateActive,
		ExpectedEpoch: 7, InventorySHA256: repeatedHex('c'),
	}
	authorizeLegacyDetachRequest(t, &request, authority)
	detachedAt := createdAt.Add(time.Minute)
	receipt, err := authority.detachLegacyOwnership(context.Background(), registry, request, detachedAt)
	if err != nil {
		t.Fatal(err)
	}
	if err := receipt.Validate(); err != nil {
		t.Fatalf("receipt invalid: %v", err)
	}
	if receipt.PriorState != VMStateActive || receipt.Ownership.State != VMStateStopped || !receipt.StatePreserved {
		t.Fatalf("unexpected receipt lifecycle: %+v", receipt)
	}
	if len(manager.stops) != 1 || manager.stops[0] != "vm-legacy" || len(manager.destroys) != 0 {
		t.Fatalf("detach lifecycle calls stops=%v destroys=%v", manager.stops, manager.destroys)
	}
	if registry.GetOwnershipForDesktop("owner", "primary") != nil {
		t.Fatal("detached ownership remained routable")
	}
	if got, err := os.ReadFile(statePath); err != nil || string(got) != string(stateBytes) {
		t.Fatalf("legacy state changed: got=%q err=%v", got, err)
	}

	reloaded := NewOwnershipRegistry("http://sandbox")
	if err := reloaded.SetPersistencePath(persistencePath); err != nil {
		t.Fatal(err)
	}
	if reloaded.GetOwnershipForDesktop("owner", "primary") != nil || len(reloaded.detachedLegacy) != 1 {
		t.Fatalf("detach did not survive restart: ownership=%+v receipts=%d", reloaded.GetOwnershipForDesktop("owner", "primary"), len(reloaded.detachedLegacy))
	}
	idempotent, err := authority.detachLegacyOwnership(context.Background(), reloaded, request, detachedAt.Add(time.Minute))
	if err != nil || idempotent.ID != receipt.ID {
		t.Fatalf("identical detach replay = %+v, %v", idempotent, err)
	}
	restored, err := authority.restoreLegacyOwnership(context.Background(), reloaded, receipt)
	if err != nil {
		t.Fatal(err)
	}
	if restored.VMID != "vm-legacy" || restored.State != VMStateStopped || restored.Epoch != 7 {
		t.Fatalf("restored ownership mismatch: %+v", restored)
	}
	if len(reloaded.detachedLegacy) != 0 {
		t.Fatal("restore left detach receipt active")
	}

	restartedAgain := NewOwnershipRegistry("http://sandbox")
	if err := restartedAgain.SetPersistencePath(persistencePath); err != nil {
		t.Fatal(err)
	}
	persisted := restartedAgain.GetOwnershipForDesktop("owner", "primary")
	if persisted == nil || persisted.VMID != "vm-legacy" || persisted.State != VMStateStopped {
		t.Fatalf("restored ownership did not survive restart: %+v", persisted)
	}
	if _, err := authority.restoreLegacyOwnership(context.Background(), restartedAgain, receipt); err != nil {
		t.Fatalf("identical restore replay was not idempotent: %v", err)
	}
}

func TestLegacyDetachReloadAllowsCommittedConstructedRollbackPair(t *testing.T) {
	authority, version, _, _ := newRouteAuthorityFixture(t)
	root := t.TempDir()
	persistencePath := filepath.Join(root, "ownerships.json")
	registry := NewOwnershipRegistry("http://sandbox")
	registry.SetVMManager(&mockVMManager{})
	if err := registry.SetPersistencePath(persistencePath); err != nil {
		t.Fatal(err)
	}
	createdAt := time.Date(2026, 7, 17, 6, 0, 0, 0, time.UTC)
	registerLegacyOwnershipFixture(t, registry, VMOwnership{
		VMID: "vm-legacy-pair", UserID: "owner", DesktopID: "primary", Kind: VMKindInteractive,
		Published: true, SandboxURL: "http://legacy.test", State: VMStateHibernated,
		CreatedAt: createdAt, LastActiveAt: createdAt, Epoch: 4,
	})
	slotID, _ := routeledger.RouteSlotID("owner", "primary")
	request := LegacyOwnershipDetachRequest{RouteSlotID: slotID, VMID: "vm-legacy-pair", ExpectedState: VMStateHibernated, ExpectedEpoch: 4, InventorySHA256: repeatedHex('f')}
	authorizeLegacyDetachRequest(t, &request, authority)
	receipt, err := authority.detachLegacyOwnership(t.Context(), registry, request, createdAt.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	verification := validVerificationReceipt(t, version, createdAt.Add(2*time.Minute))
	if err := registry.beginConstructedCandidate(verification.VMID, verification.Identity.OwnerID, verification.Identity.DesktopID, "credential", version, verification.Disk); err != nil {
		t.Fatal(err)
	}
	if err := registry.activateConstructedCandidate(verification.VMID, "http://candidate.test", verification.Epoch); err != nil {
		t.Fatal(err)
	}
	if err := registry.commitConstructedCandidate(verification.VMID, version, verification.Disk); err != nil {
		t.Fatal(err)
	}
	if err := registry.setConstructedCandidatePublishedExact(slotID, verification.VMID, version, verification.DiskReceiptID, true); err != nil {
		t.Fatal(err)
	}
	reloaded := NewOwnershipRegistry("http://sandbox")
	if err := reloaded.SetPersistencePath(persistencePath); err != nil {
		t.Fatal(err)
	}
	ownership := reloaded.GetOwnershipForDesktop("owner", "primary")
	if ownership == nil || ownership.VMID != verification.VMID || !ownership.Published || ownership.State != VMStateStopped || !ownership.ConstructionCommitted {
		t.Fatalf("reloaded constructed rollback pair ownership = %+v", ownership)
	}
	stored, ok := reloaded.detachedLegacy[receipt.ID]
	if !ok || !reflect.DeepEqual(stored, receipt) {
		t.Fatalf("reloaded detach receipt = %+v, present=%v", stored, ok)
	}
}

func TestLegacyDetachReloadRejectsDuplicateLegacyOwner(t *testing.T) {
	authority, _, _, _ := newRouteAuthorityFixture(t)
	persistencePath := filepath.Join(t.TempDir(), "ownerships.json")
	registry := NewOwnershipRegistry("http://sandbox")
	registry.SetVMManager(&mockVMManager{})
	if err := registry.SetPersistencePath(persistencePath); err != nil {
		t.Fatal(err)
	}
	createdAt := time.Date(2026, 7, 17, 6, 0, 0, 0, time.UTC)
	registerLegacyOwnershipFixture(t, registry, VMOwnership{VMID: "vm-detached", UserID: "owner", DesktopID: "primary", Kind: VMKindInteractive, Published: true, State: VMStateStopped, CreatedAt: createdAt, LastActiveAt: createdAt, Epoch: 2})
	slotID, _ := routeledger.RouteSlotID("owner", "primary")
	request := LegacyOwnershipDetachRequest{RouteSlotID: slotID, VMID: "vm-detached", ExpectedState: VMStateStopped, ExpectedEpoch: 2, InventorySHA256: repeatedHex('a')}
	authorizeLegacyDetachRequest(t, &request, authority)
	if _, err := authority.detachLegacyOwnership(t.Context(), registry, request, createdAt.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(persistencePath)
	if err != nil {
		t.Fatal(err)
	}
	var state persistedOwnershipState
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatal(err)
	}
	state.Ownerships = append(state.Ownerships, &VMOwnership{VMID: "vm-live-legacy", UserID: "owner", DesktopID: "primary", Kind: VMKindInteractive, Published: true, State: VMStateStopped, CreatedAt: createdAt, LastActiveAt: createdAt, Epoch: 3})
	data, err = json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(persistencePath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	reloaded := NewOwnershipRegistry("http://sandbox")
	if err := reloaded.SetPersistencePath(persistencePath); err == nil || !strings.Contains(err.Error(), "owner desktop is also registered") {
		t.Fatalf("duplicate legacy owner reload error = %v", err)
	}
}

func TestLegacyOwnershipDetachRefusesStaleAndConstructedBindings(t *testing.T) {
	authority, version, _, _ := newRouteAuthorityFixture(t)
	registry := NewOwnershipRegistry("http://sandbox")
	if err := registry.SetPersistencePath(filepath.Join(t.TempDir(), "ownerships.json")); err != nil {
		t.Fatal(err)
	}
	createdAt := time.Date(2026, 7, 17, 6, 0, 0, 0, time.UTC)
	registerLegacyOwnershipFixture(t, registry, VMOwnership{
		VMID: "vm-legacy", UserID: "owner", DesktopID: "primary", Kind: VMKindInteractive,
		Published: true, State: VMStateHibernated, CreatedAt: createdAt, LastActiveAt: createdAt, Epoch: 4,
	})
	slotID, _ := routeledger.RouteSlotID("owner", "primary")
	base := LegacyOwnershipDetachRequest{RouteSlotID: slotID, VMID: "vm-legacy", ExpectedState: VMStateHibernated, ExpectedEpoch: 4, InventorySHA256: repeatedHex('d')}
	authorizeLegacyDetachRequest(t, &base, authority)
	stale := base
	stale.ExpectedEpoch = 3
	if _, err := authority.detachLegacyOwnership(context.Background(), registry, stale, createdAt.Add(time.Minute)); err == nil {
		t.Fatal("stale inventory epoch detached ownership")
	}
	if registry.GetOwnershipForDesktop("owner", "primary") == nil {
		t.Fatal("stale refusal mutated ownership")
	}

	registry.mu.Lock()
	own := registry.ownerships[ownershipKey("owner", "primary")]
	own.SnapshotKind = "constructed-computer-version"
	own.ConstructionVersion = &version
	own.ConstructionDisk = &diskinstantiation.Receipt{RealizationID: own.VMID}
	own.ConstructionCommitted = true
	registry.mu.Unlock()
	if _, err := authority.detachLegacyOwnership(context.Background(), registry, base, createdAt.Add(time.Minute)); err == nil {
		t.Fatal("constructed ownership was detached as legacy")
	}
	if registry.GetOwnershipForDesktop("owner", "primary") == nil {
		t.Fatal("constructed refusal mutated ownership")
	}
}

func TestLegacyOwnershipDetachAndRestoreRefuseExistingRoute(t *testing.T) {
	authority, version, approvalRef, certificateRef := newRouteAuthorityFixture(t)
	otherAuthority, _, _, _ := newRouteAuthorityFixture(t)
	registry := NewOwnershipRegistry("http://sandbox")
	if err := registry.SetPersistencePath(filepath.Join(t.TempDir(), "ownerships.json")); err != nil {
		t.Fatal(err)
	}
	createdAt := time.Date(2026, 7, 17, 6, 0, 0, 0, time.UTC)
	registerLegacyOwnershipFixture(t, registry, VMOwnership{
		VMID: "vm-legacy", UserID: "owner", DesktopID: "primary", Kind: VMKindInteractive,
		Published: true, State: VMStateHibernated, CreatedAt: createdAt, LastActiveAt: createdAt, Epoch: 2,
	})
	slotID, _ := routeledger.RouteSlotID("owner", "primary")
	request := LegacyOwnershipDetachRequest{RouteSlotID: slotID, VMID: "vm-legacy", ExpectedState: VMStateHibernated, ExpectedEpoch: 2, InventorySHA256: repeatedHex('e')}
	authorizeLegacyDetachRequest(t, &request, authority, otherAuthority)
	if _, err := authority.transitionAuthorized(context.Background(), routeledger.TransitionCommand{
		RouteSlotID: slotID, Kind: routeledger.TransitionBootstrap, New: version,
		ApprovalRef: approvalRef, PromotionCertificateRef: certificateRef, IdempotencyKey: "idempotency:legacy-route-present",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := authority.detachLegacyOwnership(context.Background(), registry, request, createdAt.Add(time.Minute)); err == nil {
		t.Fatal("legacy ownership detached after route appeared")
	}
	if registry.GetOwnershipForDesktop("owner", "primary") == nil {
		t.Fatal("route-present refusal mutated ownership")
	}

	receipt, err := otherAuthority.detachLegacyOwnership(context.Background(), registry, request, createdAt.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := authority.restoreLegacyOwnership(context.Background(), registry, receipt); err == nil {
		t.Fatal("legacy ownership restored after route appeared")
	}
	if registry.GetOwnershipForDesktop("owner", "primary") != nil {
		t.Fatal("route-present restore refusal registered ownership")
	}
}

func TestLegacyOwnershipDetachReceiptRejectsTampering(t *testing.T) {
	createdAt := time.Date(2026, 7, 17, 6, 0, 0, 0, time.UTC)
	authority, _, _, _ := newRouteAuthorityFixture(t)
	request := LegacyOwnershipDetachRequest{RouteSlotID: "computer:owner:primary", VMID: "vm-legacy", ExpectedState: VMStateActive, ExpectedEpoch: 1, InventorySHA256: repeatedHex('f')}
	authorizeLegacyDetachRequest(t, &request, authority)
	receipt, err := newLegacyOwnershipDetachReceipt("computer:owner:primary", repeatedHex('f'), request.Authorization, VMOwnership{
		VMID: "vm-legacy", UserID: "owner", DesktopID: "primary", Kind: VMKindInteractive,
		Published: true, State: VMStateStopped, CreatedAt: createdAt, LastActiveAt: createdAt, Epoch: 1,
	}, VMStateActive, createdAt.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	tampered := receipt
	tampered.Ownership.VMID = "vm-other"
	if err := tampered.Validate(); err == nil {
		t.Fatal("tampered receipt validated")
	}

	constructed := receipt
	constructed.Ownership.ConstructionVersion = &computerversion.ComputerVersion{CodeRef: computerversion.CodeRef("code:sha256:" + repeatedHex('a')), ArtifactProgramRef: computerversion.ArtifactProgramRef("artifact-program:sha256:" + repeatedHex('b'))}
	if err := constructed.Validate(); err == nil {
		t.Fatal("constructed ownership validated as legacy receipt")
	}
}

func TestLegacyOwnershipDetachHTTPRoundTrip(t *testing.T) {
	authority, _, _, _ := newRouteAuthorityFixture(t)
	registry := NewOwnershipRegistry("http://sandbox")
	if err := registry.SetPersistencePath(filepath.Join(t.TempDir(), "ownerships.json")); err != nil {
		t.Fatal(err)
	}
	createdAt := time.Date(2026, 7, 17, 6, 0, 0, 0, time.UTC)
	registerLegacyOwnershipFixture(t, registry, VMOwnership{
		VMID: "vm-http-legacy", UserID: "owner", DesktopID: "primary", Kind: VMKindInteractive,
		Published: true, State: VMStateHibernated, CreatedAt: createdAt, LastActiveAt: createdAt, Epoch: 9,
	})
	slotID, _ := routeledger.RouteSlotID("owner", "primary")
	detach := LegacyOwnershipDetachRequest{RouteSlotID: slotID, VMID: "vm-http-legacy", ExpectedState: VMStateHibernated, ExpectedEpoch: 9, InventorySHA256: repeatedHex('9')}
	authorizeLegacyDetachRequest(t, &detach, authority)
	payload, err := json.Marshal(detach)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(registry)
	handler.SetRouteAuthority(authority)
	request := httptest.NewRequest(http.MethodPost, "/internal/vmctl/computer-version-realizations/detach-legacy", bytes.NewReader(payload))
	request.Header.Set("X-Internal-Caller", "true")
	response := httptest.NewRecorder()
	handler.HandleDetachLegacyComputerVersionOwnership(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("detach HTTP %d: %s", response.Code, response.Body.String())
	}
	var receipt LegacyOwnershipDetachReceipt
	if err := json.NewDecoder(response.Body).Decode(&receipt); err != nil || receipt.Validate() != nil {
		t.Fatalf("detach response invalid: %+v decode=%v validate=%v", receipt, err, receipt.Validate())
	}
	restorePayload, err := json.Marshal(restoreLegacyOwnershipRequest{Receipt: receipt})
	if err != nil {
		t.Fatal(err)
	}
	restoreRequest := httptest.NewRequest(http.MethodPost, "/internal/vmctl/computer-version-realizations/restore-legacy", bytes.NewReader(restorePayload))
	restoreRequest.Header.Set("X-Internal-Caller", "true")
	restoreResponse := httptest.NewRecorder()
	handler.HandleRestoreLegacyComputerVersionOwnership(restoreResponse, restoreRequest)
	if restoreResponse.Code != http.StatusOK {
		t.Fatalf("restore HTTP %d: %s", restoreResponse.Code, restoreResponse.Body.String())
	}
	var restored VMOwnership
	if err := json.NewDecoder(restoreResponse.Body).Decode(&restored); err != nil || restored.VMID != "vm-http-legacy" || restored.Epoch != 9 {
		t.Fatalf("restore response mismatch: %+v err=%v", restored, err)
	}
}
