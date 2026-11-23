package vmctl

// Account deletion in vmctl (docs/design/account-create-delete-2026-11-23.md,
// the owner's rulings of 2026-11-23). Three host-internal routes, called by
// the auth service's purge runner:
//
//	POST /internal/vmctl/account/computers  what the account owns, and which
//	                                        of it may never be deleted
//	POST /internal/vmctl/account/stop       stop every computer of the account
//	                                        (at once, when access is revoked)
//	POST /internal/vmctl/account/destroy    after the grace period: drop the
//	                                        ownership and delete the VM state
//
// Destroy is the irreversible step and the one this file guards:
//
//   - It reads the deletion record itself (auth.db, read-only, through
//     purgeguard) and refuses an account with no record or a record younger
//     than 72 hours, whatever the caller says.
//   - It refuses, for the whole account and before touching anything, any
//     computer in a critical_protected or public_platform class (the paper
//     is both; no request field overrides it), any computer under a
//     maintenance hold, and any computer with a live capsule.
//   - It never deletes a path outside the state directory (DestroyVMState's
//     own rule).
//
// Ways this could go wrong, written before the code (tests in
// account_deletion_test.go):
//  1. destroy runs for an account with no record, or one younger than 72h;
//  2. destroy takes the paper's or the owner's protected computer, alone or
//     as one of several computers of the account;
//  3. destroy takes a held computer, or one with a live capsule;
//  4. stop stops a protected computer;
//  5. a destroy that fails halfway leaves ownership gone and files kept (the
//     VM state goes first, the ownership after);
//  6. the routes answer a caller that is not the host.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/hostauth"
	"github.com/yusefmosiah/go-choir/internal/purgeguard"
)

// AccountComputer is one computer of an account, as the purge reads it.
type AccountComputer struct {
	ComputerID string `json:"computer_id"`
	VMID       string `json:"vm_id"`
	DesktopID  string `json:"desktop_id"`
	State      string `json:"state"`
	Class      string `json:"class"`
	Held       bool   `json:"held,omitempty"`
	// Refused names why this computer may never be deleted by an account
	// deletion: its class, a hold, or a live capsule. Empty means deletable.
	Refused string `json:"refused,omitempty"`
}

// ErrAccountRefused is the refusal of a protected account.
var ErrAccountRefused = errors.New("vmctl: this account owns a computer that an account deletion never touches")

// SetAccountPurgeAuthDB names the auth database the destroy route reads the
// deletion record from. Without it destroy refuses every account.
func (r *OwnershipRegistry) SetAccountPurgeAuthDB(path string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.accountPurgeAuthDB = strings.TrimSpace(path)
}

func (r *OwnershipRegistry) accountPurgeEligibleFor(userID string, now time.Time) error {
	r.mu.RLock()
	check, path := r.accountPurgeEligible, r.accountPurgeAuthDB
	r.mu.RUnlock()
	if check != nil {
		return check(userID, now)
	}
	db, err := purgeguard.OpenReadOnly(path)
	if err != nil {
		return err
	}
	defer db.Close()
	return purgeguard.Eligible(db, userID, now)
}

// AccountComputers lists the computers the user owns, in every state.
func (r *OwnershipRegistry) AccountComputers(userID string) []AccountComputer {
	userID = strings.TrimSpace(userID)
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.accountComputersLocked(userID)
}

func (r *OwnershipRegistry) accountComputersLocked(userID string) []AccountComputer {
	var out []AccountComputer
	if userID == "" {
		return out
	}
	policy := normalizeWarmnessPolicyConfig(r.warmnessPolicy)
	for _, own := range r.ownerships {
		if strings.TrimSpace(own.UserID) != userID {
			continue
		}
		class := warmnessClassForOwnership(own, policy)
		c := AccountComputer{ComputerID: own.ComputerID, VMID: own.VMID, DesktopID: own.DesktopID,
			State: string(own.State), Class: string(class), Held: own.IsHeld()}
		switch {
		case purgeguard.ClassRefused(string(class)) || policy.PlatformComputerIDs[strings.TrimSpace(own.ComputerID)]:
			c.Refused = "protected class " + string(class)
		case own.IsHeld():
			c.Refused = "under a maintenance hold"
		case normalizeDesktopID(own.DesktopID) != PrimaryDesktopID && liveState(own.State):
			c.Refused = "a capsule is live"
		}
		out = append(out, c)
	}
	return out
}

func liveState(s VMState) bool {
	switch s {
	case VMStateBooting, VMStateActive, VMStateDegraded, VMStateStopping:
		return true
	}
	return false
}

// AccountPrecheck refuses an account that owns a computer an account deletion
// never touches. It is the whole account or nothing: a partial deletion of a
// protected account is never made.
func (r *OwnershipRegistry) AccountPrecheck(userID string) error {
	for _, c := range r.AccountComputers(userID) {
		if c.Refused != "" {
			return fmt.Errorf("%w (%s: %s)", ErrAccountRefused, c.ComputerID, c.Refused)
		}
	}
	return nil
}

// StopAccountComputers stops every computer of the account. A protected
// account stops nothing. It is idempotent: a stopped computer stays stopped.
func (r *OwnershipRegistry) StopAccountComputers(userID string) (int, error) {
	if err := r.AccountPrecheck(userID); err != nil {
		return 0, err
	}
	stopped := 0
	var errs []error
	for _, c := range r.AccountComputers(userID) {
		switch VMState(c.State) {
		case VMStateStopped, VMStateHibernated, VMStateFailed:
			continue
		}
		if err := r.StopVMForDesktop(userID, c.DesktopID); err != nil {
			errs = append(errs, fmt.Errorf("stop %s: %w", c.ComputerID, err))
			continue
		}
		stopped++
	}
	return stopped, errors.Join(errs...)
}

// AccountDestroyResult counts what a destroy removed.
type AccountDestroyResult struct {
	Computers []string `json:"computers"`
	VMStates  int      `json:"vm_states"`
}

// DestroyAccountComputers deletes the VM state of every computer of the
// account and drops its ownerships, after the three checks in the file
// header. The state goes first and the ownership after, so a failure leaves
// an ownership that still names state to finish, never state with no owner.
func (r *OwnershipRegistry) DestroyAccountComputers(userID string, now time.Time) (AccountDestroyResult, error) {
	var res AccountDestroyResult
	userID = strings.TrimSpace(userID)
	if err := r.accountPurgeEligibleFor(userID, now); err != nil {
		return res, err
	}
	if err := r.AccountPrecheck(userID); err != nil {
		return res, err
	}
	r.mu.RLock()
	mgr := r.vmManager
	r.mu.RUnlock()
	if mgr == nil {
		return res, errors.New("vmctl: no VM manager; nothing destroyed")
	}
	var errs []error
	for _, c := range r.AccountComputers(userID) {
		key := ownershipKey(userID, c.DesktopID)
		r.mu.Lock()
		cur := r.ownerships[key]
		if cur == nil || cur.VMID != c.VMID {
			r.mu.Unlock()
			continue
		}
		// Re-read under the lock: class and hold are checked again at the
		// moment of destruction, and the lifecycle is claimed so a wake or
		// refresh cannot start while its state goes.
		if again := r.accountComputersLocked(userID); !accountRowDeletable(again, c.VMID) {
			r.mu.Unlock()
			errs = append(errs, fmt.Errorf("%w (%s changed)", ErrAccountRefused, c.ComputerID))
			continue
		}
		if err := r.lifecycleConflictLocked(key); err != nil {
			r.mu.Unlock()
			errs = append(errs, err)
			continue
		}
		r.lifecycle[key] = lifecycleAccountDeletion
		r.mu.Unlock()

		err := func() error {
			defer r.releaseLifecycle(key, lifecycleAccountDeletion)
			// Stop it if it runs: DestroyVMState refuses a live VM.
			if liveState(cur.State) {
				if err := mgr.StopVM(c.VMID); err != nil {
					return fmt.Errorf("stop %s: %w", c.ComputerID, err)
				}
			}
			if err := mgr.DestroyVMState(c.VMID); err != nil {
				return fmt.Errorf("destroy %s: %w", c.ComputerID, err)
			}
			r.mu.Lock()
			defer r.mu.Unlock()
			if cur := r.ownerships[key]; cur != nil {
				delete(r.ownerships, key)
				delete(r.vmByID, cur.VMID)
				r.saveLocked()
			}
			return nil
		}()
		if err != nil {
			errs = append(errs, err)
			continue
		}
		res.Computers = append(res.Computers, c.ComputerID)
		res.VMStates++
		log.Printf("vmctl: account deletion destroyed computer %s (%s)", c.ComputerID, c.VMID)
	}
	return res, errors.Join(errs...)
}

func accountRowDeletable(rows []AccountComputer, vmID string) bool {
	for _, c := range rows {
		if c.VMID == vmID {
			return c.Refused == ""
		}
	}
	return false
}

type accountRequest struct {
	UserID string `json:"user_id"`
}

func (h *Handler) accountRoute(w http.ResponseWriter, r *http.Request) (string, bool) {
	if r.Method != http.MethodPost {
		writeVMCTLJSON(w, http.StatusMethodNotAllowed, vmctlErrorResponse{Error: "method not allowed"})
		return "", false
	}
	if !isInternalCaller(r) {
		writeVMCTLJSON(w, http.StatusForbidden, vmctlErrorResponse{Error: "vmctl control endpoints are not publicly accessible"})
		return "", false
	}
	var req accountRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil || strings.TrimSpace(req.UserID) == "" {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: "user_id is required"})
		return "", false
	}
	return strings.TrimSpace(req.UserID), true
}

// HandleAccountComputers handles POST /internal/vmctl/account/computers.
func (h *Handler) HandleAccountComputers(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.accountRoute(w, r)
	if !ok {
		return
	}
	list := h.registry.AccountComputers(userID)
	if list == nil {
		list = []AccountComputer{}
	}
	writeVMCTLJSON(w, http.StatusOK, map[string]any{"computers": list})
}

// HandleAccountStop handles POST /internal/vmctl/account/stop.
func (h *Handler) HandleAccountStop(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.accountRoute(w, r)
	if !ok {
		return
	}
	n, err := h.registry.StopAccountComputers(userID)
	if errors.Is(err, ErrAccountRefused) {
		writeVMCTLJSON(w, http.StatusForbidden, vmctlErrorResponse{Error: err.Error()})
		return
	}
	if err != nil {
		writeVMCTLJSON(w, http.StatusConflict, vmctlErrorResponse{Error: err.Error()})
		return
	}
	writeVMCTLJSON(w, http.StatusOK, map[string]any{"stopped": n})
}

// HandleAccountDestroy handles POST /internal/vmctl/account/destroy.
func (h *Handler) HandleAccountDestroy(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.accountRoute(w, r)
	if !ok {
		return
	}
	res, err := h.registry.DestroyAccountComputers(userID, time.Now().UTC())
	switch {
	case errors.Is(err, ErrAccountRefused), errors.Is(err, purgeguard.ErrNoRecord), errors.Is(err, purgeguard.ErrTooEarly):
		writeVMCTLJSON(w, http.StatusForbidden, vmctlErrorResponse{Error: err.Error()})
	case err != nil:
		writeVMCTLJSON(w, http.StatusConflict, vmctlErrorResponse{Error: err.Error()})
	default:
		writeVMCTLJSON(w, http.StatusOK, res)
	}
}

// postAccount calls one of the account routes and decodes its answer into out
// (nil to ignore it). A non-200 answer is an error carrying vmctl's words; a
// 403 wraps ErrAccountRefused so a caller can tell a refusal from an outage.
func (c *Client) postAccount(ctx context.Context, route, userID string, out any) error {
	data, _ := json.Marshal(accountRequest{UserID: userID})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+route, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("vmctl client: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	hostauth.MarkInternal(req)
	setLifecycleCaller(req)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("vmctl client: call failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("%w: %s", ErrAccountRefused, strings.TrimSpace(string(raw)))
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("vmctl client: %s answered %s: %s", route, resp.Status, strings.TrimSpace(string(raw)))
	}
	if out != nil {
		return json.Unmarshal(raw, out)
	}
	return nil
}

// AccountComputers lists the account's computers and which are refused.
func (c *Client) AccountComputers(ctx context.Context, userID string) ([]AccountComputer, error) {
	var out struct {
		Computers []AccountComputer `json:"computers"`
	}
	err := c.postAccount(ctx, "/internal/vmctl/account/computers", userID, &out)
	return out.Computers, err
}

// AccountStop stops every computer of the account.
func (c *Client) AccountStop(ctx context.Context, userID string) error {
	return c.postAccount(ctx, "/internal/vmctl/account/stop", userID, nil)
}

// AccountDestroy deletes the account's computers; vmctl checks the deletion
// record itself.
func (c *Client) AccountDestroy(ctx context.Context, userID string) (AccountDestroyResult, error) {
	var out AccountDestroyResult
	err := c.postAccount(ctx, "/internal/vmctl/account/destroy", userID, &out)
	return out, err
}
