package gateway

// POST /provider/v1/fetch is the host fetch verb (SDC NET-01;
// docs/problems/guest-fetches-public-internet-itself-2026-10-21.md): a
// granted guest's public web fetches make their network hop here, where
// the address guard, a per-computer budget and concurrency bound, and one
// log line per hop run out of the guest's reach. The work is in
// internal/hostfetch; this file is authentication, budgets and the log.
//
// Unlike /provider/v1/source-api the guest names the URL: there is no
// allowlist for research reads (SDC, "Networking must enable, not starve").
// The guard refuses non-public answers and the host's own addresses.
//
// Off at the host: only a VM on the host's own list (CHOIR_HOST_FETCH_VMS,
// the list vmctl's grant uses) may call it; an empty list refuses everyone
// (docs/problems/host-fetch-verb-open-to-every-guest-2026-10-22.md).

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/hostfetch"
)

const (
	// EnvHostFetchVMs is the host's list of VM IDs (gateway identities)
	// admitted to the verb, shared with vmctl. Empty or unset: every call
	// is refused.
	EnvHostFetchVMs = hostfetch.EnvVMs
	// hostFetchNotGrantedCode is the typed refusal for an unlisted caller.
	hostFetchNotGrantedCode = "host_fetch_not_granted"
	// EnvHostFetchPerMinute overrides the per-computer hops a minute.
	EnvHostFetchPerMinute = "CHOIR_HOST_FETCH_PER_MINUTE"
	// hostFetchDefaultPerMinute is sized for the paper: about 180 feeds a
	// 15-minute cycle plus their article imports and redirects, with room
	// for research reads. The shared inference limit (60) would starve it.
	hostFetchDefaultPerMinute = 600
	// hostFetchInFlight bounds one computer's concurrent upstream hops.
	hostFetchInFlight = 16
)

type hostFetchService struct {
	exec *hostfetch.Executor
	// allowed is the host's list; a caller not on it is refused before
	// any budget or upstream work.
	allowed  map[string]bool
	limiter  *PerAutoputerRateLimiter
	maxSlots int

	mu       sync.Mutex
	inFlight map[string]int
}

func newHostFetchService(exec *hostfetch.Executor, allowed map[string]bool, perMinute, inFlight int) *hostFetchService {
	if allowed == nil {
		allowed = map[string]bool{}
	}
	return &hostFetchService{
		exec:     exec,
		allowed:  allowed,
		limiter:  NewPerAutoputerRateLimiter(perMinute, time.Minute),
		maxSlots: inFlight,
		inFlight: map[string]int{},
	}
}

func loadHostFetchAllowList(getenv func(string) string) map[string]bool {
	return hostfetch.ParseVMList(getenv(EnvHostFetchVMs))
}

func defaultHostFetchPerMinute(getenv func(string) string) int {
	if n, err := strconv.Atoi(strings.TrimSpace(getenv(EnvHostFetchPerMinute))); err == nil && n > 0 {
		return n
	}
	return hostFetchDefaultPerMinute
}

var (
	defaultHostFetchOnce sync.Once
	defaultHostFetchSvc  *hostFetchService
)

func (h *Handler) hostFetchService() *hostFetchService {
	if h.hostFetch != nil {
		return h.hostFetch
	}
	defaultHostFetchOnce.Do(func() {
		defaultHostFetchSvc = newHostFetchService(&hostfetch.Executor{}, loadHostFetchAllowList(os.Getenv), defaultHostFetchPerMinute(os.Getenv), hostFetchInFlight)
	})
	return defaultHostFetchSvc
}

func (s *hostFetchService) acquire(computerID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.inFlight[computerID] >= s.maxSlots {
		return false
	}
	s.inFlight[computerID]++
	return true
}

func (s *hostFetchService) release(computerID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.inFlight[computerID]--; s.inFlight[computerID] <= 0 {
		delete(s.inFlight, computerID)
	}
}

// HandleHostFetch handles POST /provider/v1/fetch.
func (h *Handler) HandleHostFetch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeGatewayJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "method not allowed"})
		return
	}
	computerID, err := h.authenticateAutoputer(r)
	if err != nil {
		log.Printf("gateway: fetch authentication denied: %v", err)
		writeGatewayJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "authentication required"})
		return
	}
	svc := h.hostFetchService()
	if !svc.allowed[computerID] {
		log.Printf("gateway: fetch refused computer=%s reason=not_granted", computerID)
		writeGatewayJSON(w, http.StatusForbidden, struct {
			Error string `json:"error"`
			Code  string `json:"code"`
		}{Error: "host fetch is not enabled for this computer", Code: hostFetchNotGrantedCode})
		return
	}
	if !svc.limiter.Record(computerID) {
		_, _, resetIn := svc.limiter.Status(computerID)
		w.Header().Set("Retry-After", strconv.Itoa(max(1, int(resetIn.Seconds()))))
		log.Printf("gateway: fetch refused computer=%s reason=budget", computerID)
		writeGatewayJSON(w, http.StatusTooManyRequests, ErrorResponse{Error: "host fetch budget exceeded for computer " + computerID})
		return
	}
	if !svc.acquire(computerID) {
		w.Header().Set("Retry-After", "1")
		log.Printf("gateway: fetch refused computer=%s reason=concurrency", computerID)
		writeGatewayJSON(w, http.StatusTooManyRequests, ErrorResponse{Error: "too many host fetches in flight for computer " + computerID})
		return
	}
	defer svc.release(computerID)
	start := time.Now()
	var res hostfetch.Result
	defer func() {
		ms := time.Since(start).Milliseconds()
		if p := recover(); p != nil {
			log.Printf("gateway: fetch aborted computer=%s method=%s host=%s status=%d bytes=%d ms=%d reason=body cap or upstream break", computerID, res.Method, res.Host, res.Status, res.Bytes, ms)
			panic(p)
		}
		switch {
		case res.Refused != "":
			log.Printf("gateway: fetch refused computer=%s method=%s host=%s reason=%q ms=%d", computerID, res.Method, res.Host, res.Refused, ms)
		case res.Err != nil:
			log.Printf("gateway: fetch failed computer=%s method=%s host=%s err=%q ms=%d", computerID, res.Method, res.Host, sanitizeError(res.Err), ms)
		default:
			log.Printf("gateway: fetch computer=%s method=%s host=%s status=%d bytes=%d ms=%d", computerID, res.Method, res.Host, res.Status, res.Bytes, ms)
		}
	}()
	svc.exec.ServeTo(w, r, &res)
}
