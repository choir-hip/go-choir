package vmctl

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/routeledger"
)

// ColdRecoverRequest is intentionally the entire recover_current protocol.
// Strict JSON decoding rejects every additional field before host state changes.
type ColdRecoverRequest struct {
	ComputerID              string `json:"computer_id"`
	ExpectedCanonicalHead   string `json:"expected_canonical_head"`
	ExpectedRouteGeneration uint64 `json:"expected_route_generation"`
	IdempotencyKey          string `json:"idempotency_key"`
}

// RecoveryFencingToken is the single-use, realization-bound recovery lease.
type RecoveryFencingToken struct {
	Audience           string `json:"-"`
	Operation          string `json:"-"`
	ComputerID         string `json:"computer_id"`
	OwnerID            string `json:"owner_id"`
	VMID               string `json:"vm_id"`
	RouteGeneration    uint64 `json:"route_generation"`
	CanonicalHead      string `json:"canonical_head"`
	RecoveryGeneration uint64 `json:"recovery_generation"`
	Nonce              string `json:"nonce"`
	Expiry             string `json:"expiry"`
}

// ColdRecoverResponse is the idempotent recovery receipt.
type ColdRecoverResponse struct {
	RecoveryID         string               `json:"recovery_id"`
	RecoveryGeneration uint64               `json:"recovery_generation"`
	CanonicalHead      string               `json:"canonical_head"`
	FinalHead          string               `json:"final_head"`
	FencingToken       RecoveryFencingToken `json:"fencing_token"`
	QuarantinePath     string               `json:"quarantine_path"`
	StagingPath        string               `json:"staging_path"`
	Status             string               `json:"status"`
	RouteGeneration    uint64               `json:"route_generation"`
}

// RecoveryLease allows exactly the boot lifecycle append(s) made by the fresh
// realization. It never authorizes an arbitrary host semantic append.
type RecoveryLease struct {
	mu    sync.Mutex
	token RecoveryFencingToken
	used  bool
}

// AllowAppend is a HeadCAS guard for the boot realization. Callers must also
// prove that the append is a boot lifecycle append before invoking it.
func (l *RecoveryLease) AllowAppend(computerID string, recoveryGeneration uint64, head string) bool {
	if l == nil {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.used || time.Now().After(parseRecoveryExpiry(l.token.Expiry)) {
		return false
	}
	if computerID != l.token.ComputerID || recoveryGeneration != l.token.RecoveryGeneration || head != l.token.CanonicalHead {
		return false
	}
	l.used = true
	return true
}

func parseRecoveryExpiry(value string) time.Time {
	expiry, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}
	}
	return expiry
}

// ColdRecoveryIdentity is derived only from the registry and route authority.
type ColdRecoveryIdentity struct {
	RecoveryID string
	Token      RecoveryFencingToken
	RouteSlot  string
}

// ColdRecoveryHeadReader re-reads corpusd before any disk transition and after
// boot. The caller-provided head is only an optimistic concurrency value.
type ColdRecoveryHeadReader interface {
	CanonicalHead(context.Context, string, RecoveryFencingToken) (string, error)
}

// TrustedGuestKeyCopier is the sole path by which recovery reads guest data.
// Its implementation attaches quarantine read-only to a trusted guest unit and
// copies only the privacy key into staging; vmctl never mounts or parses ext4.
type TrustedGuestKeyCopier interface {
	CopyPrivacyKey(context.Context, RecoveryFencingToken, string, string) error
}

// ColdRecoveryVerifier confirms replay equivalence, effective ComputerVersion,
// and frontend serving_join before route publication.
type ColdRecoveryVerifier interface {
	VerifyRecovery(context.Context, ColdRecoveryIdentity) (finalHead string, err error)
}

type coldRecoveryJournal struct {
	RecoveryID      string               `json:"recovery_id"`
	IdempotencyKey  string               `json:"idempotency_key"`
	Phase           string               `json:"phase"`
	ComputerID      string               `json:"computer_id"`
	VMID            string               `json:"vm_id"`
	CanonicalHead   string               `json:"canonical_head"`
	FinalHead       string               `json:"final_head,omitempty"`
	Status          string               `json:"status,omitempty"`
	RouteGeneration uint64               `json:"route_generation"`
	QuarantinePath  string               `json:"quarantine_path,omitempty"`
	StagingPath     string               `json:"staging_path,omitempty"`
	Token           RecoveryFencingToken `json:"fencing_token"`
}

type coldRecoveryRecord struct {
	request  ColdRecoverRequest
	response ColdRecoverResponse
}

type coldRecoveryStorage interface {
	QuarantineDataImage(stateRoot, vmID string, recoveryGeneration uint64, operationID string, maxRetained int) (string, error)
	StageSparseImage(stateRoot, vmID string, recoveryGeneration uint64, operationID string, sizeMB int) (string, error)
}

type coldRecoveryState struct {
	mu          sync.Mutex
	generations map[string]uint64
	records     map[string]coldRecoveryRecord
	locks       map[string]*sync.Mutex
	leases      map[string]*RecoveryLease
	headReader  ColdRecoveryHeadReader
	keyCopier   TrustedGuestKeyCopier
	verifier    ColdRecoveryVerifier
	storage     coldRecoveryStorage
	stateRoot   string
}

func newColdRecoveryState() *coldRecoveryState {
	stateRoot := strings.TrimSpace(os.Getenv("VMCTL_VM_STATE_DIR"))
	if stateRoot == "" {
		stateRoot = "/var/lib/go-choir/vm-state"
	}
	return &coldRecoveryState{
		generations: make(map[string]uint64),
		records:     make(map[string]coldRecoveryRecord),
		locks:       make(map[string]*sync.Mutex),
		leases:      make(map[string]*RecoveryLease),
		stateRoot:   stateRoot,
	}
}

func (s *coldRecoveryState) lockFor(computerID string) *sync.Mutex {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.locks[computerID] == nil {
		s.locks[computerID] = &sync.Mutex{}
	}
	return s.locks[computerID]
}

func (h *Handler) coldRecovery() *coldRecoveryState {
	h.coldRecoveryMu.Lock()
	defer h.coldRecoveryMu.Unlock()
	if h.coldRecoveryState == nil {
		h.coldRecoveryState = newColdRecoveryState()
	}
	return h.coldRecoveryState
}

func (h *Handler) SetColdRecoveryHeadReader(reader ColdRecoveryHeadReader) {
	state := h.coldRecovery()
	state.mu.Lock()
	state.headReader = reader
	state.mu.Unlock()
}

func (h *Handler) SetTrustedGuestKeyCopier(copier TrustedGuestKeyCopier) {
	state := h.coldRecovery()
	state.mu.Lock()
	state.keyCopier = copier
	state.mu.Unlock()
}

func (h *Handler) SetColdRecoveryVerifier(verifier ColdRecoveryVerifier) {
	state := h.coldRecovery()
	state.mu.Lock()
	state.verifier = verifier
	state.mu.Unlock()
}

// SetColdRecoveryStateRoot configures the manager-owned VM state root. It is
// useful when VMCTL_VM_STATE_DIR differs from the production default.
// SetColdRecoveryStorage supplies the VM manager's filesystem-safe quarantine
// and sparse-image operations. The handler never mounts or parses an image.
func (h *Handler) SetColdRecoveryStorage(storage coldRecoveryStorage) {
	state := h.coldRecovery()
	state.mu.Lock()
	state.storage = storage
	state.mu.Unlock()
}
func (h *Handler) SetColdRecoveryStateRoot(root string) {
	state := h.coldRecovery()
	state.mu.Lock()
	state.stateRoot = strings.TrimSpace(root)
	state.mu.Unlock()
}
func readExistingColdRecoveryJournal(vmDir string, request ColdRecoverRequest) (*coldRecoveryJournal, error) {
	paths, err := filepath.Glob(filepath.Join(vmDir, "*.journal"))
	if err != nil {
		return nil, err
	}
	for _, path := range paths {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			continue
		}
		var journal coldRecoveryJournal
		if json.Unmarshal(data, &journal) != nil ||
			journal.ComputerID != request.ComputerID ||
			journal.VMID == "" ||
			journal.IdempotencyKey != request.IdempotencyKey ||
			journal.CanonicalHead != request.ExpectedCanonicalHead ||
			journal.RouteGeneration != request.ExpectedRouteGeneration {
			continue
		}
		return &journal, nil
	}
	return nil, nil
}

// RecoveryLease returns the currently fenced lease for a ComputerID. The
// returned capability expires promptly and is single-use through AllowAppend.
func (h *Handler) RecoveryLease(computerID string) *RecoveryLease {
	state := h.coldRecovery()
	state.mu.Lock()
	defer state.mu.Unlock()
	return state.leases[strings.TrimSpace(computerID)]
}

func strictColdRecoverRequest(body io.Reader) (ColdRecoverRequest, error) {
	decoder := json.NewDecoder(body)
	decoder.DisallowUnknownFields()
	var request ColdRecoverRequest
	if err := decoder.Decode(&request); err != nil {
		return ColdRecoverRequest{}, err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return ColdRecoverRequest{}, fmt.Errorf("invalid JSON stream")
	}
	request.ComputerID = strings.TrimSpace(request.ComputerID)
	request.ExpectedCanonicalHead = strings.TrimSpace(request.ExpectedCanonicalHead)
	request.IdempotencyKey = strings.TrimSpace(request.IdempotencyKey)
	if request.ComputerID == "" || request.IdempotencyKey == "" || request.ExpectedRouteGeneration == 0 || !isSHA256Hex(request.ExpectedCanonicalHead) {
		return ColdRecoverRequest{}, fmt.Errorf("invalid cold recovery request")
	}
	return request, nil
}

func isSHA256Hex(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func coldRecoverPathComputerID(r *http.Request) string {
	if value := strings.TrimSpace(r.PathValue("computerID")); value != "" {
		return value
	}
	const prefix = "/internal/vmctl/computers/"
	const suffix = "/cold-recover"
	value := strings.TrimPrefix(r.URL.Path, prefix)
	if !strings.HasSuffix(value, suffix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimSuffix(value, suffix))
}

func mintRecoveryToken(computerID, ownerID, vmID string, routeGeneration, recoveryGeneration uint64, head string) (RecoveryFencingToken, error) {
	var nonce [32]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return RecoveryFencingToken{}, err
	}
	return RecoveryFencingToken{
		Audience:           "vmctl",
		Operation:          "recover_current",
		ComputerID:         computerID,
		OwnerID:            ownerID,
		VMID:               vmID,
		RouteGeneration:    routeGeneration,
		CanonicalHead:      head,
		RecoveryGeneration: recoveryGeneration,
		Nonce:              hex.EncodeToString(nonce[:]),
		Expiry:             time.Now().Add(5 * time.Minute).UTC().Format(time.RFC3339Nano),
	}, nil
}

func coldRecoveryRecordKey(computerID, idempotencyKey string) string {
	return computerID + "\x00" + idempotencyKey
}

func sameColdRecoveryTarget(left, right ColdRecoverRequest) bool {
	return left.ComputerID == right.ComputerID && left.ExpectedCanonicalHead == right.ExpectedCanonicalHead && left.ExpectedRouteGeneration == right.ExpectedRouteGeneration
}

func coldRecoveryOperationID(key string) string {
	digest := sha256.Sum256([]byte(key))
	return hex.EncodeToString(digest[:8])
}

func recoveryVMDir(root, vmID string) (string, error) {
	if root == "" || vmID == "" || filepath.Base(vmID) != vmID {
		return "", fmt.Errorf("invalid VM state path")
	}
	root = filepath.Clean(root)
	rootInfo, err := os.Lstat(root)
	if err != nil {
		return "", err
	}
	if rootInfo.Mode()&os.ModeSymlink != 0 || !rootInfo.IsDir() {
		return "", fmt.Errorf("VM state root is not a directory")
	}
	vmDir := filepath.Join(root, vmID)
	rel, err := filepath.Rel(root, vmDir)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("VM state path escapes root")
	}
	info, err := os.Lstat(vmDir)
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return "", fmt.Errorf("VM state directory is not a directory")
	}
	return vmDir, nil
}

func regularFile(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("path is not a regular file")
	}
	return nil
}

func fsyncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}

func writeColdRecoveryJournal(path string, journal coldRecoveryJournal) error {
	data, err := json.Marshal(journal)
	if err != nil {
		return err
	}
	temporary := path + ".tmp"
	file, err := os.OpenFile(temporary, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := file.Write(data); err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err := os.Rename(temporary, path); err != nil {
		return err
	}
	return fsyncDirectory(filepath.Dir(path))
}

func (h *Handler) writeColdRecoveryPhase(vmDir string, journal *coldRecoveryJournal, phase string) error {
	journal.Phase = phase
	return writeColdRecoveryJournal(filepath.Join(vmDir, journal.RecoveryID+".journal"), *journal)
}

// HandleColdRecover handles recover_current only. It rejects extra JSON fields
// before lookup or fencing and derives owner, VMID, route, and data-image path
// from the durable ownership/route authorities rather than request metadata.
func (h *Handler) HandleColdRecover(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeVMCTLJSON(w, http.StatusMethodNotAllowed, vmctlErrorResponse{Error: "method not allowed"})
		return
	}
	if !isInternalCaller(r) {
		writeVMCTLJSON(w, http.StatusForbidden, vmctlErrorResponse{Error: "vmctl control endpoints are not publicly accessible"})
		return
	}
	request, err := strictColdRecoverRequest(r.Body)
	if err != nil {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: "invalid cold recovery request"})
		return
	}
	if computerID := coldRecoverPathComputerID(r); computerID == "" || computerID != request.ComputerID {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: "computer_id does not match request path"})
		return
	}

	ownership := h.registry.GetOwnershipByComputerID(request.ComputerID)
	if ownership == nil || stableComputerID(ownership.UserID, ownership.DesktopID, ownership.ComputerID) != request.ComputerID {
		writeVMCTLJSON(w, http.StatusNotFound, vmctlErrorResponse{Error: "computer ownership not found"})
		return
	}
	slotID, err := routeledger.RouteSlotID(ownership.UserID, ownership.DesktopID)
	if err != nil {
		writeVMCTLJSON(w, http.StatusConflict, vmctlErrorResponse{Error: "computer route is unavailable"})
		return
	}
	route, known, err := h.resolveComputerVersionRoute(r.Context(), ownership.UserID, ownership.DesktopID)
	if err != nil || !known || route.RouteAbsent || route.Slot.ID != slotID || route.Slot.Generation != request.ExpectedRouteGeneration {
		writeVMCTLJSON(w, http.StatusConflict, vmctlErrorResponse{Error: "computer route generation changed"})
		return
	}

	state := h.coldRecovery()
	lock := state.lockFor(request.ComputerID)
	lock.Lock()
	defer lock.Unlock()
	key := coldRecoveryRecordKey(request.ComputerID, request.IdempotencyKey)
	state.mu.Lock()
	if existing, ok := state.records[key]; ok {
		state.mu.Unlock()
		if !sameColdRecoveryTarget(existing.request, request) {
			writeVMCTLJSON(w, http.StatusConflict, vmctlErrorResponse{Error: "idempotency key has a different recovery target"})
			return
		}
		writeVMCTLJSON(w, http.StatusAccepted, existing.response)
		return
	}
	reader, copier, verifier, storage, root := state.headReader, state.keyCopier, state.verifier, state.storage, state.stateRoot
	if reader == nil || copier == nil || verifier == nil || storage == nil {
		state.mu.Unlock()
		writeVMCTLJSON(w, http.StatusServiceUnavailable, vmctlErrorResponse{Error: "cold recovery dependencies are unavailable"})
		return
	}
	vmDir, vmDirErr := recoveryVMDir(root, ownership.VMID)
	if vmDirErr == nil {
		if prior, journalErr := readExistingColdRecoveryJournal(vmDir, request); journalErr == nil && prior != nil {
			status := prior.Status
			if status == "" {
				status = "rematerializing"
			}
			response := ColdRecoverResponse{
				RecoveryID: prior.RecoveryID, RecoveryGeneration: prior.Token.RecoveryGeneration,
				CanonicalHead: prior.CanonicalHead, FinalHead: prior.FinalHead,
				FencingToken: prior.Token, QuarantinePath: prior.QuarantinePath,
				StagingPath: prior.StagingPath, Status: status,
				RouteGeneration: prior.RouteGeneration,
			}
			state.records[key] = coldRecoveryRecord{request: request, response: response}
			state.mu.Unlock()
			writeVMCTLJSON(w, http.StatusAccepted, response)
			return
		}
	}
	generation := state.generations[request.ComputerID] + 1
	state.generations[request.ComputerID] = generation
	state.mu.Unlock()

	token, err := mintRecoveryToken(request.ComputerID, ownership.UserID, ownership.VMID, request.ExpectedRouteGeneration, generation, request.ExpectedCanonicalHead)
	if err != nil {
		writeVMCTLJSON(w, http.StatusInternalServerError, vmctlErrorResponse{Error: "could not mint recovery token"})
		return
	}
	currentHead, err := reader.CanonicalHead(r.Context(), request.ComputerID, token)
	if err != nil || currentHead != request.ExpectedCanonicalHead {
		writeVMCTLJSON(w, http.StatusConflict, vmctlErrorResponse{Error: "canonical head changed"})
		return
	}

	vmDir, err = recoveryVMDir(root, ownership.VMID)
	if err != nil {
		writeVMCTLJSON(w, http.StatusConflict, vmctlErrorResponse{Error: "VM state path is unavailable"})
		return
	}
	operationID := coldRecoveryOperationID(key)
	recoveryID := fmt.Sprintf("rec-%d-%s", generation, operationID)
	quarantine := filepath.Join(vmDir, fmt.Sprintf("data.img.quarantine-%d-%s", generation, operationID))
	staging := filepath.Join(vmDir, fmt.Sprintf("data.img.staging-%d-%s", generation, operationID))
	journal := coldRecoveryJournal{
		RecoveryID: recoveryID, IdempotencyKey: request.IdempotencyKey,
		ComputerID: request.ComputerID, VMID: ownership.VMID,
		CanonicalHead:   request.ExpectedCanonicalHead,
		RouteGeneration: request.ExpectedRouteGeneration, QuarantinePath: quarantine,
		StagingPath: staging, Token: token,
	}
	if err := h.writeColdRecoveryPhase(vmDir, &journal, "fenced"); err != nil {
		writeVMCTLJSON(w, http.StatusInternalServerError, vmctlErrorResponse{Error: "could not persist recovery journal"})
		return
	}
	state.mu.Lock()
	state.leases[request.ComputerID] = &RecoveryLease{token: token}
	state.mu.Unlock()

	if err := h.registry.StopVMForDesktop(ownership.UserID, ownership.DesktopID); err != nil {
		writeVMCTLJSON(w, http.StatusConflict, vmctlErrorResponse{Error: err.Error()})
		return
	}
	if err := h.registry.MarkRecoveryInProgress(ownership.UserID, ownership.DesktopID); err != nil {
		writeVMCTLJSON(w, http.StatusConflict, vmctlErrorResponse{Error: err.Error()})
		return
	}
	if err := h.writeColdRecoveryPhase(vmDir, &journal, "stopped"); err != nil {
		writeVMCTLJSON(w, http.StatusInternalServerError, vmctlErrorResponse{Error: "could not persist recovery journal"})
		return
	}
	if currentHead, err = reader.CanonicalHead(r.Context(), request.ComputerID, token); err != nil || currentHead != request.ExpectedCanonicalHead {
		writeVMCTLJSON(w, http.StatusConflict, vmctlErrorResponse{Error: "canonical head changed"})
		return
	}

	quarantine, err = storage.QuarantineDataImage(root, ownership.VMID, generation, operationID, 0)
	if err != nil {
		writeVMCTLJSON(w, http.StatusConflict, vmctlErrorResponse{Error: "could not quarantine data image"})
		return
	}
	staging, err = storage.StageSparseImage(root, ownership.VMID, generation, operationID, 32768)
	if err != nil {
		writeVMCTLJSON(w, http.StatusInternalServerError, vmctlErrorResponse{Error: "could not stage data image"})
		return
	}
	journal.QuarantinePath = quarantine
	journal.StagingPath = staging
	if err := regularFile(staging); err != nil {
		writeVMCTLJSON(w, http.StatusInternalServerError, vmctlErrorResponse{Error: "staging image is invalid"})
		return
	}
	if err := copier.CopyPrivacyKey(r.Context(), token, quarantine, staging); err != nil {
		writeVMCTLJSON(w, http.StatusConflict, vmctlErrorResponse{Error: "trusted guest key copy failed"})
		return
	}
	if err := h.writeColdRecoveryPhase(vmDir, &journal, "key_copied"); err != nil {
		writeVMCTLJSON(w, http.StatusInternalServerError, vmctlErrorResponse{Error: "could not persist recovery journal"})
		return
	}
	if err := h.writeColdRecoveryPhase(vmDir, &journal, "staging"); err != nil {
		writeVMCTLJSON(w, http.StatusInternalServerError, vmctlErrorResponse{Error: "could not persist recovery journal"})
		return
	}
	dataImage := filepath.Join(vmDir, "data.img")
	if err := os.Rename(staging, dataImage); err != nil || fsyncDirectory(vmDir) != nil {
		writeVMCTLJSON(w, http.StatusInternalServerError, vmctlErrorResponse{Error: "could not activate staging image"})
		return
	}
	if err := h.writeColdRecoveryPhase(vmDir, &journal, "swapped"); err != nil {
		writeVMCTLJSON(w, http.StatusInternalServerError, vmctlErrorResponse{Error: "could not persist recovery journal"})
		return
	}
	// Authorized maintenance recovery: boot the held computer into the B14
	// replay-only drive under the guest-visible hold (never a plain recover).
	if _, err := h.registry.RecoverVMForDesktopMaintenance(ownership.UserID, ownership.DesktopID, true); err != nil {
		writeVMCTLJSON(w, http.StatusConflict, vmctlErrorResponse{Error: err.Error()})
		return
	}
	if err := h.writeColdRecoveryPhase(vmDir, &journal, "booted"); err != nil {
		writeVMCTLJSON(w, http.StatusInternalServerError, vmctlErrorResponse{Error: "could not persist recovery journal"})
		return
	}
	identity := ColdRecoveryIdentity{RecoveryID: recoveryID, Token: token, RouteSlot: slotID}
	finalHead, err := verifier.VerifyRecovery(r.Context(), identity)
	if err != nil || !isSHA256Hex(finalHead) {
		journal.Status = "failed"
		_ = h.writeColdRecoveryPhase(vmDir, &journal, "verified")
		response := ColdRecoverResponse{RecoveryID: recoveryID, RecoveryGeneration: generation, CanonicalHead: request.ExpectedCanonicalHead, FencingToken: token, QuarantinePath: quarantine, StagingPath: staging, Status: "failed", RouteGeneration: request.ExpectedRouteGeneration}
		state.mu.Lock()
		state.records[key] = coldRecoveryRecord{request: request, response: response}
		state.mu.Unlock()
		writeVMCTLJSON(w, http.StatusConflict, response)
		return
	}
	latest, known, err := h.resolveComputerVersionRoute(r.Context(), ownership.UserID, ownership.DesktopID)
	if err != nil || !known || latest.RouteAbsent || latest.Slot.ID != slotID || latest.Slot.Generation != request.ExpectedRouteGeneration {
		writeVMCTLJSON(w, http.StatusConflict, vmctlErrorResponse{Error: "computer route generation changed"})
		return
	}
	journal.FinalHead = finalHead
	journal.Status = "active"
	if err := h.writeColdRecoveryPhase(vmDir, &journal, "verified"); err != nil {
		writeVMCTLJSON(w, http.StatusInternalServerError, vmctlErrorResponse{Error: "could not persist recovery journal"})
		return
	}
	if err := h.writeColdRecoveryPhase(vmDir, &journal, "route_published"); err != nil {
		writeVMCTLJSON(w, http.StatusInternalServerError, vmctlErrorResponse{Error: "could not persist recovery journal"})
		return
	}
	if err := h.writeColdRecoveryPhase(vmDir, &journal, "done"); err != nil {
		writeVMCTLJSON(w, http.StatusInternalServerError, vmctlErrorResponse{Error: "could not persist recovery journal"})
		return
	}
	response := ColdRecoverResponse{RecoveryID: recoveryID, RecoveryGeneration: generation, CanonicalHead: request.ExpectedCanonicalHead, FinalHead: finalHead, FencingToken: token, QuarantinePath: quarantine, StagingPath: staging, Status: "active", RouteGeneration: request.ExpectedRouteGeneration}
	state.mu.Lock()
	state.records[key] = coldRecoveryRecord{request: request, response: response}
	state.mu.Unlock()
	writeVMCTLJSON(w, http.StatusAccepted, response)
}
