package proxy

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

// A new account that signs in only through `choir login` never loads the
// desktop, and a computer is created when the desktop bootstraps
// (docs/problems/cli-login-leaves-new-account-without-a-computer-2026-11-03.md).
// The first owner-wide API-key read of /api/compute/status that finds the
// primary desktop without a computer makes the same vmctl call bootstrap makes
// (resolveComputerURL -> ResolveDesktopContext), so the terminal path is
// complete on its own.
//
// Failure modes this is built against:
//   - double create: concurrent polls, or a poll racing the desktop's own
//     bootstrap. One flight per user at most here; vmctl's resolve is the one
//     idempotent create for both callers.
//   - retry storm: a resolve that fails (admission refused, vmctl down) must
//     not be re-issued by every 5-second poll; a failure holds off the next
//     attempt for firstComputerRetryAfter.
//   - revoked key: authenticate checks revoked_at on every request, so a
//     revoked key never reaches this. A key revoked while the create is in
//     flight still ends with the account's own primary computer, which the
//     desktop would have created anyway; no key-scoped state is written.
//   - request-bound work: the create outlives the request that triggered it
//     (a cold first boot is longer than a CLI timeout), so it runs on a
//     context detached from the request and bounded by vmctlOpContext.
//   - proxy crash mid-create: the flight is memory only; the next poll starts
//     again, and vmctl's resolve is idempotent on an admitted ownership.
//   - cookie sessions and bound keys: unchanged. A cookie session's desktop
//     bootstrap creates the computer; a bound key names an existing one.
const firstComputerRetryAfter = 30 * time.Second

type firstComputerFlight struct {
	running  bool
	failedAt time.Time
}

type firstComputerGate struct {
	mu      sync.Mutex
	flights map[string]*firstComputerFlight
}

// begin reports whether the caller should start the create for userID now.
func (g *firstComputerGate) begin(userID string, now time.Time) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.flights == nil {
		g.flights = make(map[string]*firstComputerFlight)
	}
	f := g.flights[userID]
	if f == nil {
		f = &firstComputerFlight{}
		g.flights[userID] = f
	}
	if f.running || (!f.failedAt.IsZero() && now.Sub(f.failedAt) < firstComputerRetryAfter) {
		return false
	}
	f.running = true
	return true
}

func (g *firstComputerGate) end(userID string, failed bool, now time.Time) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if failed {
		if f := g.flights[userID]; f != nil {
			f.running = false
			f.failedAt = now
		}
		return
	}
	delete(g.flights, userID)
}

// startFirstComputerForOwnerKey starts, in the background, the creation of the
// account's primary computer. It returns at once; the caller reports the
// computer as not yet started and the client polls.
func (h *Handler) startFirstComputerForOwnerKey(ctx context.Context, userID string) {
	if h.vmctlClient == nil || userID == "" || !h.firstComputers.begin(userID, time.Now()) {
		return
	}
	go func() {
		bg := vmctl.WithLifecycleCaller(context.WithoutCancel(ctx), "proxy.api-key-first-computer")
		_, err := h.resolveComputerURL(bg, userID, vmctl.PrimaryDesktopID)
		if err != nil {
			log.Printf("proxy: first computer for owner-wide api key user=%s: %v", userID, err)
		}
		h.firstComputers.end(userID, err != nil, time.Now())
	}()
}
