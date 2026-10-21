package gateway

import (
	"errors"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// SEC-05: a computer's gateway bearer is accepted only from the address the
// host attests for that computer
// (docs/problems/gateway-bearer-works-from-any-computer-2026-10-21.md).
//
// The pin is AutoputerIdentity.PeerIP. The source address is trustworthy
// because the host drops any packet entering a guest tap whose source is not
// that guest's address (internal/vmmanager/hostfirewall.go, tapInputDropSpec).
// Pinning every route turns a stale pin into a computer that cannot call its
// models, which is why the rollout is staged by mode, as host-internal auth
// was (nix/node-b.nix, internalAuthMode).

// BindingMode is the rollout mode of the bearer binding.
type BindingMode string

const (
	// BindingOff checks and writes nothing: the pre-SEC-05 behavior.
	BindingOff BindingMode = "off"
	// BindingObserve logs and counts what enforce would refuse, refuses
	// nothing, and writes no pin.
	BindingObserve BindingMode = "observe"
	// BindingEnforce refuses a call from any address but the pin's, and pins
	// an unpinned bearer to the address of its first authorized use.
	BindingEnforce BindingMode = "enforce"
	// BindingStrict is enforce without the first-use pin: an unpinned
	// bearer is refused until the host has attested its address.
	BindingStrict BindingMode = "strict"
)

// BearerBindingEnv selects the mode.
const BearerBindingEnv = "GATEWAY_BEARER_BINDING"

// ParseBindingMode reads a mode. Anything that is not exactly off, observe or
// strict (including an empty value and an unknown one) is enforce: a typo
// never loosens the gateway.
func ParseBindingMode(raw string) BindingMode {
	switch BindingMode(strings.ToLower(strings.TrimSpace(raw))) {
	case BindingOff:
		return BindingOff
	case BindingObserve:
		return BindingObserve
	case BindingStrict:
		return BindingStrict
	default:
		return BindingEnforce
	}
}

// LoadBindingMode reads GATEWAY_BEARER_BINDING.
func LoadBindingMode() BindingMode {
	return ParseBindingMode(os.Getenv(BearerBindingEnv))
}

// SetBearerBinding sets the binding mode. A handler that is never configured
// enforces.
func (h *Handler) SetBearerBinding(mode BindingMode) {
	h.bindingMode = ParseBindingMode(string(mode))
}

func (h *Handler) bearerBindingMode() BindingMode {
	if h.bindingMode == "" {
		return BindingEnforce
	}
	return h.bindingMode
}

// bindingCounters count what the binding did since the gateway started.
type bindingCounters struct {
	bound       atomic.Int64
	wouldRefuse atomic.Int64
	refused     atomic.Int64
}

// bindingStats is a snapshot of bindingCounters.
type bindingStats struct {
	Bound       int64
	WouldRefuse int64
	Refused     int64
}

func (h *Handler) bindingStatsSnapshot() bindingStats {
	return bindingStats{
		Bound:       h.binding.bound.Load(),
		WouldRefuse: h.binding.wouldRefuse.Load(),
		Refused:     h.binding.refused.Load(),
	}
}

// gatewayBindingHealth is the bearer binding as /health reports it: the
// mode, how many active computers are pinned, and what the binding has done.
// It names no address and no computer, so the public endpoint leaks nothing.
type gatewayBindingHealth struct {
	Mode        string `json:"mode"`
	Pinned      int    `json:"pinned"`
	Unpinned    int    `json:"unpinned"`
	Bound       int64  `json:"first_use_bound"`
	WouldRefuse int64  `json:"would_refuse"`
	Refused     int64  `json:"refused"`
}

func (h *Handler) bindingHealth() *gatewayBindingHealth {
	pinned, unpinned := h.registry.PinCounts()
	s := h.bindingStatsSnapshot()
	return &gatewayBindingHealth{
		Mode:        string(h.bearerBindingMode()),
		Pinned:      pinned,
		Unpinned:    unpinned,
		Bound:       s.Bound,
		WouldRefuse: s.WouldRefuse,
		Refused:     s.Refused,
	}
}

// The errors are logged by the caller on every refusal and answered to the
// client as a generic 401; they carry no address and no computer.
var (
	errPeerPinMismatch = errors.New("peer does not match the credential's pin")
	errPeerUnpinned    = errors.New("credential has no pin and the gateway requires one")
	errPeerUnbindable  = errors.New("credential could not be pinned")
)

// bindingRetryAfter is the Retry-After, in seconds, of a binding refusal.
const bindingRetryAfter = "2"

// authFailure picks the answer to a failed authenticateAutoputer. A binding
// refusal is still a refusal, but it tells the caller to come back: a computer
// rebooted onto a new address (a deploy refresh or a recovery keeps its token
// and takes a new address from the next host port) is refused until vmctl
// re-ensures its credential, right after the boot returns, while a resumed
// guest may already be calling. The guest waits out a 429 Retry-After
// (internal/gatewayruntime/provider.go) and the refusal heals when the host
// moves the pin; a genuine mismatch stays refused. A pin that could not be
// written is the gateway's fault (503). Everything else is the same generic
// 401 as before, and no answer names an address.
func authFailure(err error) (status int, retryAfter, message string) {
	switch {
	case errors.Is(err, errPeerPinMismatch), errors.Is(err, errPeerUnpinned):
		return http.StatusTooManyRequests, bindingRetryAfter, "credential is not attested for this address; retry shortly"
	case errors.Is(err, errPeerUnbindable):
		return http.StatusServiceUnavailable, bindingRetryAfter, "credential could not be bound; retry shortly"
	}
	return http.StatusUnauthorized, "", "authentication required"
}

// writeAuthFailure answers a failed authenticateAutoputer in the gateway's
// own error envelope.
func writeAuthFailure(w http.ResponseWriter, err error) {
	status, retry, message := authFailure(err)
	if retry != "" {
		w.Header().Set("Retry-After", retry)
	}
	writeGatewayJSON(w, status, ErrorResponse{Error: message})
}

// checkBearerBinding applies the binding to a call whose bearer has already
// validated and whose peer is a runtime peer. It returns nil to let the call
// continue.
func (h *Handler) checkBearerBinding(computerID string, r *http.Request) error {
	mode := h.bearerBindingMode()
	if mode == BindingOff {
		return nil
	}
	ip := remoteIP(r)
	if ip == nil {
		return errPeerUnbindable
	}
	peer := ip.String()
	state, pin, err := h.registry.CheckPeer(computerID, peer, mode == BindingEnforce)
	if err != nil {
		if mode == BindingObserve {
			h.binding.wouldRefuse.Add(1)
			h.bindingLogf("could-not-check/"+computerID, "gateway: bearer binding observe: would refuse %s from %s: %v", computerID, peer, err)
			return nil
		}
		h.binding.refused.Add(1)
		h.bindingLogf("could-not-check/"+computerID, "gateway: bearer binding %s: refused %s from %s: %v", mode, computerID, peer, err)
		return errPeerUnbindable
	}
	switch state {
	case PeerMatch:
		return nil
	case PeerBound:
		h.binding.bound.Add(1)
		log.Printf("gateway: bearer binding %s: %s pinned to %s on first use", mode, computerID, peer)
		return nil
	case PeerUnpinned:
		if mode == BindingStrict {
			h.binding.refused.Add(1)
			h.bindingLogf("unpinned/"+computerID, "gateway: bearer binding strict: refused %s from %s: no host-attested pin", computerID, peer)
			return errPeerUnpinned
		}
		return nil // observe: counted by /health as unpinned; nothing is written
	}
	// PeerMismatch.
	if mode == BindingObserve {
		h.binding.wouldRefuse.Add(1)
		h.bindingLogf("mismatch/"+computerID+"/"+peer, "gateway: bearer binding observe: would refuse %s from %s (pinned to %s)", computerID, peer, pin)
		return nil
	}
	h.binding.refused.Add(1)
	h.bindingLogf("mismatch/"+computerID+"/"+peer, "gateway: bearer binding %s: refused %s from %s (pinned to %s)", mode, computerID, peer, pin)
	return errPeerPinMismatch
}

// bindingLogf logs at most once a minute per key, so a retry storm from one
// address cannot flood the journal. The counters still count every call.
func (h *Handler) bindingLogf(key, format string, args ...any) {
	const every = time.Minute
	const maxKeys = 4096
	h.bindingLogMu.Lock()
	now := time.Now()
	if h.bindingLogLast == nil || len(h.bindingLogLast) >= maxKeys {
		h.bindingLogLast = make(map[string]time.Time)
	}
	if last, ok := h.bindingLogLast[key]; ok && now.Sub(last) < every {
		h.bindingLogMu.Unlock()
		return
	}
	h.bindingLogLast[key] = now
	h.bindingLogMu.Unlock()
	log.Printf(format, args...)
}

// bindingLogState is the handler's log-bounding state; it lives in its own
// type so the Handler literal in the constructors stays unchanged.
type bindingLogState struct {
	bindingLogMu   sync.Mutex
	bindingLogLast map[string]time.Time
}
