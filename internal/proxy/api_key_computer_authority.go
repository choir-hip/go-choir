package proxy

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

// requireAPIKeyComputerTarget joins a computer-selecting API-key request to the
// authenticated owner's stable ComputerID before any downstream route lookup,
// resolve, wake, dial, lifecycle intent, or product call. Registry binding is
// attenuation metadata; this vmctl ownership join is the live authority.
//
// A key with an empty ComputerID is owner-wide: it reaches every interactive
// computer the owner owns, but every request must still name its target — via
// pathComputerID, a computer_id query parameter, or an X-Choir-Computer
// header — and the exact same (UserID, ComputerID) ownership join runs against
// that named target. No implicit default targeting exists for owner-wide keys.
//
// pathComputerID is non-empty for routes whose path names a stable computer.
// requestedDesktopID is non-empty for routes that select a logical desktop
// through query, header, request body, or the default primary selector.
func (h *Handler) requireAPIKeyComputerTarget(
	w http.ResponseWriter,
	r *http.Request,
	authResult *AuthResult,
	pathComputerID string,
	requestedDesktopID string,
) (*resolvedComputerTarget, bool) {
	if authResult == nil || authResult.AuthMethod != "api_key" {
		return nil, true
	}
	boundComputerID := strings.TrimSpace(authResult.ComputerID)
	// Owner-wide keys must name their target — via pathComputerID, a
	// computer_id query parameter, or an X-Choir-Computer header — before any
	// infrastructure check, so an unnamed request fails closed as a client
	// error. The binding is attenuation metadata; this vmctl ownership join is
	// the live authority for both key kinds.
	var ownerWideNamed string
	if boundComputerID == "" {
		named := strings.TrimSpace(pathComputerID)
		if r != nil {
			if v := strings.TrimSpace(r.URL.Query().Get("computer_id")); v != "" {
				named = v
			} else if v := strings.TrimSpace(r.Header.Get("X-Choir-Computer")); v != "" {
				named = v
			}
		}
		if pathComputerID != "" && named != "" && named != strings.TrimSpace(pathComputerID) {
			writeJSON(w, http.StatusForbidden, errorResponse{Error: "api key computer target mismatch"})
			return nil, false
		}
		if named == "" {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "owner-wide api key requires a named computer (computer_id)"})
			return nil, false
		}
		ownerWideNamed = named
	}
	if h.vmctlClient == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "computer ownership authority unavailable"})
		return nil, false
	}
	if boundComputerID == "" {
		ownership, err := h.vmctlClient.LookupComputerContext(r.Context(), authResult.UserID, ownerWideNamed)
		if err != nil {
			if errors.Is(err, vmctl.ErrComputerLookupIdentityMismatch) {
				writeJSON(w, http.StatusForbidden, errorResponse{Error: "api key computer ownership required"})
			} else {
				writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "computer ownership authority unavailable"})
			}
			return nil, false
		}
		if ownership == nil || ownership.UserID != authResult.UserID || ownership.ComputerID != ownerWideNamed || ownership.Kind != "" && ownership.Kind != vmctl.VMKindInteractive {
			writeJSON(w, http.StatusForbidden, errorResponse{Error: "api key computer ownership required"})
			return nil, false
		}
		canonicalDesktopID := strings.TrimSpace(ownership.DesktopID)
		if canonicalDesktopID == "" {
			writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "computer ownership authority unavailable"})
			return nil, false
		}
		// Owner-wide keys derive their desktop from the joined ownership;
		// desktop selectors must still match it when explicitly provided.
		if r != nil {
			for _, v := range r.URL.Query()["desktop_id"] {
				if s := strings.TrimSpace(v); s != "" && s != canonicalDesktopID {
					writeJSON(w, http.StatusForbidden, errorResponse{Error: "api key is bound to another desktop"})
					return nil, false
				}
			}
			for _, v := range r.Header.Values("X-Choir-Desktop") {
				if s := strings.TrimSpace(v); s != "" && s != canonicalDesktopID {
					writeJSON(w, http.StatusForbidden, errorResponse{Error: "api key is bound to another desktop"})
					return nil, false
				}
			}
			if requested := strings.TrimSpace(requestedDesktopID); requested != "" && requested != canonicalDesktopID && requested != vmctl.PrimaryDesktopID {
				writeJSON(w, http.StatusForbidden, errorResponse{Error: "api key is bound to another desktop"})
				return nil, false
			}
		}
		return h.touchedAPIKeyTarget(&resolvedComputerTarget{
			ComputerID:  ownership.ComputerID,
			UserID:      ownership.UserID,
			DesktopID:   ownership.DesktopID,
			VMID:        strings.TrimSpace(ownership.VMID),
			ComputerURL: strings.TrimSpace(ownership.ComputerURL),
			State:       strings.TrimSpace(ownership.State),
			Epoch:       ownership.Epoch,
		}), true
	}
	if named := strings.TrimSpace(pathComputerID); named != "" && named != boundComputerID {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "api key is bound to another computer"})
		return nil, false
	}
	ownership, err := h.vmctlClient.LookupComputerContext(r.Context(), authResult.UserID, boundComputerID)
	if err != nil {
		if errors.Is(err, vmctl.ErrComputerLookupIdentityMismatch) {
			writeJSON(w, http.StatusForbidden, errorResponse{Error: "api key computer ownership required"})
		} else {
			writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "computer ownership authority unavailable"})
		}
		return nil, false
	}
	if ownership == nil || ownership.UserID != authResult.UserID || ownership.ComputerID != boundComputerID || ownership.Kind != "" && ownership.Kind != vmctl.VMKindInteractive {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "api key computer ownership required"})
		return nil, false
	}
	canonicalDesktopID := strings.TrimSpace(ownership.DesktopID)
	if canonicalDesktopID == "" || canonicalDesktopID != ownership.DesktopID {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "computer ownership authority unavailable"})
		return nil, false
	}
	desktopSelectors := []string{strings.TrimSpace(requestedDesktopID)}
	if r != nil {
		for _, value := range r.URL.Query()["desktop_id"] {
			desktopSelectors = append(desktopSelectors, strings.TrimSpace(value))
		}
		for _, value := range r.Header.Values("X-Choir-Desktop") {
			desktopSelectors = append(desktopSelectors, strings.TrimSpace(value))
		}
	}
	for _, requested := range desktopSelectors {
		if requested != "" && canonicalDesktopID != requested {
			writeJSON(w, http.StatusForbidden, errorResponse{Error: "api key is bound to another desktop"})
			return nil, false
		}
	}
	return h.touchedAPIKeyTarget(&resolvedComputerTarget{
		ComputerID:  ownership.ComputerID,
		UserID:      ownership.UserID,
		DesktopID:   ownership.DesktopID,
		VMID:        strings.TrimSpace(ownership.VMID),
		ComputerURL: strings.TrimSpace(ownership.ComputerURL),
		State:       strings.TrimSpace(ownership.State),
		Epoch:       ownership.Epoch,
	}), true
}

const (
	// apiKeyTouchThrottle bounds how often one computer's authenticated key
	// traffic reaches vmctl; vmctl applies its own 60 s floor on the write.
	apiKeyTouchThrottle = 30 * time.Second
	apiKeyTouchTimeout  = 2 * time.Second
)

// touchedAPIKeyTarget returns target after scheduling a best-effort refresh of
// the computer's idle clock. An authenticated API-key request is activity, but
// the touch never resolves a computer (a hibernated one is woken separately,
// by resolveComputerURLForComputerTarget), so
// LastActiveAt was never refreshed and the idle sweep hibernated a computer
// that a CLI was driving
// (docs/problems/idle-sweep-hibernates-a-computer-just-after-its-apply-restart-2026-10-24.md).
//
// The touch runs on its own goroutine with a short timeout, at most once per
// apiKeyTouchThrottle per computer, and its failure is logged and dropped: it
// never delays or fails the request. vmctl moves the clock of an active
// computer only; it cannot wake one.
func (h *Handler) touchedAPIKeyTarget(target *resolvedComputerTarget) *resolvedComputerTarget {
	if h == nil || h.vmctlClient == nil || target == nil || target.UserID == "" || target.ComputerID == "" {
		return target
	}
	now := time.Now()
	key := target.UserID + "\x00" + target.ComputerID
	if last, ok := h.apiKeyTouchSent.Load(key); ok && now.Sub(last.(time.Time)) < apiKeyTouchThrottle {
		return target
	}
	h.apiKeyTouchSent.Store(key, now)
	client, userID, computerID := h.vmctlClient, target.UserID, target.ComputerID
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), apiKeyTouchTimeout)
		defer cancel()
		if _, err := client.TouchComputerActivity(ctx, userID, computerID); err != nil {
			log.Printf("proxy: api key activity touch for %s failed: %v", computerID, err)
		}
	}()
	return target
}

// resolveComputerURLForComputerTarget serves a bearer request from the exact
// stable-computer join. A cookie request resolves by (user, desktop); a bearer
// request never re-resolves to a different realization than the (UserID,
// ComputerID) ownership join that authorized it (fbc7ff5a0). Waking the joined
// computer does not violate that: when the joined computer is hibernated or
// stopped, this wakes that exact computer through vmctl's resolve (the path
// `computer start` uses, so admission and make-room apply), waits boundedly,
// re-runs the exact join against the realized computer and serves only if it
// still matches. A refused, timed-out or mismatching wake is a
// *computerWakeError, answered as an honest 503 by the writers, never the
// opaque 502 (docs/problems/api-key-request-to-a-hibernated-computer-gets-502-2026-10-26.md).
// On a successful wake target is updated to the re-joined realization.
func (h *Handler) resolveComputerURLForComputerTarget(ctx context.Context, authResult *AuthResult, target *resolvedComputerTarget, desktopID string) (string, error) {
	return h.resolveAPIKeyComputerURL(ctx, authResult, target, desktopID, true)
}

// resolveActiveComputerURLForComputerTarget is the strict variant: it serves
// only the joined computer's current active realization and never wakes. Owner
// acts that rewrite guest state (restore, replace-workspace,
// rematerialize-from-tape, import-residue-snapshot, bootstrap-chain) use it;
// the cookie path does not wake for them either.
func (h *Handler) resolveActiveComputerURLForComputerTarget(ctx context.Context, authResult *AuthResult, target *resolvedComputerTarget, desktopID string) (string, error) {
	return h.resolveAPIKeyComputerURL(ctx, authResult, target, desktopID, false)
}

func (h *Handler) resolveAPIKeyComputerURL(ctx context.Context, authResult *AuthResult, target *resolvedComputerTarget, desktopID string, wake bool) (string, error) {
	if authResult == nil {
		return "", fmt.Errorf("authentication authority unavailable")
	}
	if authResult.AuthMethod != "api_key" {
		return h.resolveComputerURL(ctx, authResult.UserID, desktopID)
	}
	if target == nil || target.UserID != authResult.UserID || target.ComputerID == "" {
		return "", fmt.Errorf("api key computer authority unavailable")
	}
	if authResult.ComputerID != "" && target.ComputerID != authResult.ComputerID {
		return "", fmt.Errorf("api key computer authority unavailable")
	}
	if target.State != "active" || target.ComputerURL == "" {
		if !wake {
			return "", fmt.Errorf("api key computer realization unavailable")
		}
		if err := h.ensureComputerVersionRoute(ctx, target.UserID, target.DesktopID); err != nil {
			return "", err
		}
		woken, err := h.wakeAPIKeyComputer(ctx, target)
		if err != nil {
			return "", err
		}
		*target = woken
	}
	if err := h.ensureComputerVersionRoute(ctx, target.UserID, target.DesktopID); err != nil {
		return "", err
	}
	return target.ComputerURL, nil
}

// computerWakeError is a bearer request's refused, timed-out or mismatching
// wake of its joined computer. State is the realization's state before the
// wake; Cause is a short classified text safe to show a client (no vmctl
// address, no credential); Err is the underlying error.
type computerWakeError struct {
	ComputerID string
	State      string
	Cause      string
	Err        error
}

func (e *computerWakeError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("computer %s %s was not woken: %s: %v", e.ComputerID, e.State, e.Cause, e.Err)
	}
	return fmt.Sprintf("computer %s %s was not woken: %s", e.ComputerID, e.State, e.Cause)
}

func (e *computerWakeError) Unwrap() error { return e.Err }

// computerWakeFailureResponse is the honest 503 body of a failed wake.
type computerWakeFailureResponse struct {
	Error             string `json:"error"`
	State             string `json:"state"`
	Cause             string `json:"cause"`
	Reason            string `json:"reason,omitempty"`
	Kind              string `json:"kind,omitempty"`
	RetryAfterSeconds int    `json:"retry_after_seconds,omitempty"`
}

const apiKeyWakeRetryAfterSeconds = 5

// writeComputerWakeFailure renders a *computerWakeError as
// 503 {"error":"computer_waking_failed","state":...,"cause":...} with
// Retry-After. It reports whether err was one.
func writeComputerWakeFailure(w http.ResponseWriter, err error) bool {
	var wake *computerWakeError
	if !errors.As(err, &wake) {
		return false
	}
	retry := apiKeyWakeRetryAfterSeconds
	body := computerWakeFailureResponse{Error: "computer_waking_failed", State: wake.State, Cause: wake.Cause}
	if refusal := recoveryRefusalFromError(wake.Err); refusal != nil {
		body.Reason, body.Kind = refusal.Reason, refusal.Kind
		if refusal.RetryAfterSeconds > 0 {
			retry = refusal.RetryAfterSeconds
		}
	}
	body.RetryAfterSeconds = retry
	w.Header().Set("Retry-After", strconv.Itoa(retry))
	log.Printf("proxy: bearer wake failed: %v", wake)
	writeJSON(w, http.StatusServiceUnavailable, body)
	return true
}

// wakeableAPIKeyState reports whether a bearer request may start a computer
// in this state. booting, stopping, degraded, failed and unknown states are
// not woken by a request.
func wakeableAPIKeyState(state string) bool {
	return state == string(vmctl.VMStateHibernated) || state == string(vmctl.VMStateStopped)
}

// wakeAPIKeyComputer wakes the exact joined computer. Concurrent requests for
// one (UserID, ComputerID) share one wake (joinAPIKeyWake); a waiter whose request is canceled
// leaves without canceling it.
func (h *Handler) wakeAPIKeyComputer(ctx context.Context, target *resolvedComputerTarget) (resolvedComputerTarget, error) {
	prior := *target
	if !wakeableAPIKeyState(prior.State) {
		return resolvedComputerTarget{}, &computerWakeError{
			ComputerID: prior.ComputerID, State: prior.State,
			Cause: fmt.Sprintf("a computer in state %q is not woken by a request", prior.State),
		}
	}
	if h.vmctlClient == nil {
		return resolvedComputerTarget{}, &computerWakeError{ComputerID: prior.ComputerID, State: prior.State, Cause: "computer lifecycle authority unavailable"}
	}
	flight := h.joinAPIKeyWake(prior)
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-ctx.Done():
		return resolvedComputerTarget{}, &computerWakeError{ComputerID: prior.ComputerID, State: prior.State, Cause: "the request ended while waiting for the computer to wake", Err: ctx.Err()}
	case <-flight.done:
		return flight.result, flight.err
	}
}

// apiKeyWakeFlight is one in-flight wake; done closes after result and err
// are set.
type apiKeyWakeFlight struct {
	done   chan struct{}
	result resolvedComputerTarget
	err    error
}

// joinAPIKeyWake returns the in-flight wake for the target's (UserID,
// ComputerID), starting it if there is none.
func (h *Handler) joinAPIKeyWake(prior resolvedComputerTarget) *apiKeyWakeFlight {
	key := prior.UserID + "\x00" + prior.ComputerID
	h.apiKeyWakeMu.Lock()
	defer h.apiKeyWakeMu.Unlock()
	if flight, ok := h.apiKeyWakes[key]; ok {
		return flight
	}
	flight := &apiKeyWakeFlight{done: make(chan struct{})}
	if h.apiKeyWakes == nil {
		h.apiKeyWakes = make(map[string]*apiKeyWakeFlight)
	}
	h.apiKeyWakes[key] = flight
	go func() {
		flight.result, flight.err = h.runAPIKeyWake(prior)
		h.apiKeyWakeMu.Lock()
		delete(h.apiKeyWakes, key)
		h.apiKeyWakeMu.Unlock()
		close(flight.done)
	}()
	return flight
}

// runAPIKeyWake is the one wake: it re-reads the join (another request may
// have woken the computer already), starts the computer through the vmctl
// resolve the explicit start route uses, and re-joins the realized computer.
func (h *Handler) runAPIKeyWake(prior resolvedComputerTarget) (resolvedComputerTarget, error) {
	fail := func(cause string, err error) (resolvedComputerTarget, error) {
		return resolvedComputerTarget{}, &computerWakeError{ComputerID: prior.ComputerID, State: prior.State, Cause: cause, Err: err}
	}
	// Detached from any one request and bounded by the vmctl timeout.
	ctx, cancel := h.vmctlOpContext(vmctl.WithLifecycleCaller(context.Background(), "proxy.bearer.wake"))
	defer cancel()
	if joined, ok := h.rejoinAPIKeyComputer(ctx, prior); ok {
		return joined, nil
	}
	resp, err := h.vmctlClient.ResolveDesktopContext(ctx, prior.UserID, prior.DesktopID)
	if err != nil {
		return fail(wakeCause(err), err)
	}
	if resp != nil && resp.ComputerID != "" && resp.ComputerID != prior.ComputerID {
		return fail("vmctl woke a different computer than the one the key joined", nil)
	}
	joined, ok := h.rejoinAPIKeyComputer(ctx, prior)
	if !ok {
		return fail("the computer is not the joined, active realization after the wake", nil)
	}
	return joined, nil
}

// rejoinAPIKeyComputer re-runs the exact (UserID, ComputerID) ownership join
// and reports the realization only if it is the same interactive computer on
// the same desktop, active, with a URL.
func (h *Handler) rejoinAPIKeyComputer(ctx context.Context, prior resolvedComputerTarget) (resolvedComputerTarget, bool) {
	ownership, err := h.vmctlClient.LookupComputerContext(ctx, prior.UserID, prior.ComputerID)
	if err != nil || ownership == nil {
		return resolvedComputerTarget{}, false
	}
	if ownership.UserID != prior.UserID || ownership.ComputerID != prior.ComputerID || ownership.DesktopID != prior.DesktopID ||
		ownership.Kind != "" && ownership.Kind != vmctl.VMKindInteractive ||
		strings.TrimSpace(ownership.State) != "active" || strings.TrimSpace(ownership.ComputerURL) == "" {
		return resolvedComputerTarget{}, false
	}
	return resolvedComputerTarget{
		ComputerID: ownership.ComputerID, UserID: ownership.UserID, DesktopID: ownership.DesktopID,
		VMID: strings.TrimSpace(ownership.VMID), ComputerURL: strings.TrimSpace(ownership.ComputerURL),
		State: strings.TrimSpace(ownership.State), Epoch: ownership.Epoch,
	}, true
}

// wakeCause classifies a vmctl resolve failure into text safe to show a client.
func wakeCause(err error) string {
	if refusal := recoveryRefusalFromError(err); refusal != nil {
		if strings.TrimSpace(refusal.Reason) != "" {
			return "vmctl refused the wake: " + refusal.Reason
		}
		return "vmctl refused the wake"
	}
	if isResolveTimeoutError(err) {
		return "timed out waiting for the computer to become active"
	}
	text := strings.ToLower(err.Error())
	// vmctl's own refusal text is not echoed; known kinds map to fixed text.
	switch {
	case strings.Contains(text, "admission"), strings.Contains(text, "no room"), strings.Contains(text, "capacity"), strings.Contains(text, "memory"):
		return "vmctl admission refused the wake: no room on the host"
	case strings.Contains(text, "held"), strings.Contains(text, "hold"):
		return "vmctl refused the wake: the computer is held"
	case strings.Contains(text, "resolve failed"):
		return "vmctl refused the wake"
	}
	if strings.Contains(text, "resolve call failed") {
		return "vmctl was unreachable"
	}
	return "vmctl could not wake the computer"
}
