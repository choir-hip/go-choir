package agentcore

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capfloor"
	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/doctool"
	"github.com/yusefmosiah/go-choir/internal/gomemlimit"
	"github.com/yusefmosiah/go-choir/internal/hostauth"
	"github.com/yusefmosiah/go-choir/internal/server"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// POST /api/runtime/isolation-selftest: the staging proof of F4 and F6
// (docs/problems/desk-boundary-residuals-2026-10-21.md),
// driven by scripts/desk_cell_confinement_probe.py. A guest is never reached
// by SSH, and neither Landlock nor the host's egress policy can be proven off
// staging, so the owner asks the guest to show it:
//
//   - desk_cells: one worker per desk through the production spawner (the
//     desk_go_eval config, CHOIR_DESK_CELL_CONFINEMENT from the guest image),
//     each acting as an escaped cell before its first cell
//     (yaegikernel.DeskCellSelfTest);
//   - document_tool: the daemon's own binary run as a document tool through
//     the document-tool sandbox (SEC-15, internal/doctool) in required mode,
//     acting as a hostile tool would;
//   - egress: TCP dials from the guest runtime itself (guest root, which a
//     guest-side firewall could not bind) to host ports that must be dropped
//     and to ones that must stay open.
//
// Owner route: server.AuthenticatedOwner admits the host peer carrying
// X-Authenticated-User, which the proxy injects for any owner session or API
// key with write:runtime. Not on the ProductAPI allowlist, so no desk can call
// it. It changes no state: the workers and their tmp dirs are gone when it
// answers. A leg query here is refused (and the proxy refuses it first).
//
// POST /internal/runtime/isolation-selftest?leg=oom|seccomp: the two legs
// that load the guest on purpose, for the host's internal caller only
// (review bunch 22, docs/problems/fp2-step4-deployed-oom-not-drivable-desk-refuses-runaway-2026-10-22.md).
// The public edge refuses /internal/* and the proxy never forwards it. The
// handler checks server.GuestHostCaller (the guest's default gateway on the
// tap, never loopback or another in-guest address) and then
// hostauth.Authorize, the pairing /internal/debug/pprof/ uses (with the
// looser HostSourcedCaller, loopback included). On the deployed guest
// hostauth runs in off mode (nix/autoputer-vm.nix sets neither
// CHOIR_INTERNAL_AUTH_MODE nor a credential, and Authorize returns true in
// off mode), so the effective gate is GuestHostCaller alone, as for the
// planned-restart routes (requireInternalRuntimeCaller). The credential
// counts only if a guest is later run in enforce mode. Residual: a
// guest-root process with CAP_NET_ADMIN could assign the gateway address
// and pass the check; a confined desk cell (non-root, private netns,
// seccomp) cannot, and pprof and planned-restart share the residual. Named
// follow-up: guest internal routes rely on the transport gate; enforce mode
// on guests is a follow-up.
//
//   - oom (yaegikernel.DeskOOMSelfTest): a throwaway research worker under its
//     real budget cgroup runs a fixed allocating cell until the kernel kills
//     it;
//   - seccomp (capsule.SeccompSelfTest): the daemon re-execs its own binary
//     once per capsule filter; each child loads the filter and tries clone3
//     and a new user namespace (SEC-13 on the deployed kernel; not inside a
//     live capsule).
//
// Legs are an enum; the route takes no cell and no filter. Both routes share
// one lock: one self-test at a time.

var isolationSelfTestMu sync.Mutex

type isolationEgressProbe struct {
	Name string `json:"name"`
	Addr string `json:"addr"`
	// Want is "open" or "blocked" (dropped or unreachable; a refused
	// connection reached the host and is not blocked).
	Want   string `json:"want"`
	Result string `json:"result"`
	Detail string `json:"detail,omitempty"`
	OK     bool   `json:"ok"`
}

type isolationSelfTestResponse struct {
	ComputerID string `json:"computer_id"`
	// Leg is "" for the confinement legs, "oom" for the out-of-memory leg,
	// "seccomp" for the capsule seccomp leg.
	Leg       string                              `json:"leg,omitempty"`
	OOM       *yaegikernel.DeskOOMSelfTestReceipt `json:"oom,omitempty"`
	Seccomp   *capsule.SeccompSelfTestReceipt     `json:"seccomp,omitempty"`
	DeskCells yaegikernel.DeskCellSelfTestReceipt `json:"desk_cells"`
	// FrameBoundary is the forged-frame probe: raw observations of the
	// daemon's frame admission per desk, judged by the staging script against
	// a table it holds itself.
	FrameBoundary yaegikernel.FrameBoundaryReceipt `json:"frame_boundary"`
	// DocumentTool is the document-tool sandbox's probe (SEC-15).
	DocumentTool doctool.SelfTestReceipt `json:"document_tool"`
	// RuntimeCapabilities is the daemon's own capability state (SEC-15's
	// floor): under enforce its bounding set holds nothing outside the floor.
	RuntimeCapabilities capfloor.Status        `json:"runtime_capabilities"`
	Egress              []isolationEgressProbe `json:"egress"`
	EgressHost          string                 `json:"egress_host,omitempty"`
	OK                  bool                   `json:"ok"`
	Failures            []string               `json:"failures,omitempty"`
}

// HandleIsolationSelfTest runs the desk cell and guest egress self-tests.
func (h *APIHandler) HandleIsolationSelfTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if _, err := authenticateUser(r); err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	if r.URL.Query().Has("leg") {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "the self-test's oom and seccomp legs are host-internal (" + internalIsolationSelfTestPath + ")"})
		return
	}
	if !isolationSelfTestMu.TryLock() {
		writeAPIJSON(w, http.StatusConflict, apiError{Error: "an isolation self-test is already running"})
		return
	}
	defer isolationSelfTestMu.Unlock()
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()

	resp := isolationSelfTestResponse{ComputerID: h.rt.cfg.ComputerID}
	resp.DeskCells = yaegikernel.DeskCellSelfTest(ctx, deskCellSelfTestConfig(h.rt.cfg.ComputerID))
	for _, f := range resp.DeskCells.Failures {
		resp.Failures = append(resp.Failures, "desk_cells: "+f)
	}
	resp.FrameBoundary = yaegikernel.FrameBoundarySelfTest()
	for _, r := range resp.FrameBoundary.Roles {
		if r.Error != "" {
			resp.Failures = append(resp.Failures, "frame_boundary: "+r.Role+": "+r.Error)
		}
	}
	cellCfg := deskCellSelfTestConfig(h.rt.cfg.ComputerID)
	resp.DocumentTool = doctool.SelfTest(ctx, daemonSelfTestBinary(), append([]string{"/etc/passwd"}, cellCfg.ExtraReads...))
	for _, f := range resp.DocumentTool.Failures {
		resp.Failures = append(resp.Failures, "document_tool: "+f)
	}
	resp.RuntimeCapabilities = capfloor.Snapshot()
	if resp.RuntimeCapabilities.Mode == capfloor.ModeEnforce && len(resp.RuntimeCapabilities.Extra) != 0 {
		resp.Failures = append(resp.Failures, "runtime_capabilities: enforce, but the daemon holds "+strings.Join(resp.RuntimeCapabilities.Extra, " "))
	}
	host, targets := guestEgressTargets(os.Getenv("RUNTIME_GATEWAY_URL"))
	resp.EgressHost = host
	if host == "" {
		resp.Failures = append(resp.Failures, "egress: RUNTIME_GATEWAY_URL names no host address")
	}
	resp.Egress = probeGuestEgress(ctx, targets, 4*time.Second)
	for _, p := range resp.Egress {
		if !p.OK {
			resp.Failures = append(resp.Failures, "egress: "+p.Name+" "+p.Addr+" = "+p.Result+", want "+p.Want)
		}
	}
	resp.OK = len(resp.Failures) == 0
	writeAPIJSON(w, http.StatusOK, resp)
}

const (
	isolationLegOOM     = "oom"
	isolationLegSeccomp = "seccomp"
)

// internalIsolationSelfTestPath serves the oom and seccomp legs.
const internalIsolationSelfTestPath = "/internal/runtime/isolation-selftest"

// HandleInternalIsolationSelfTest runs one host-internal leg.
func (h *APIHandler) HandleInternalIsolationSelfTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if !server.GuestHostCaller(r) || !hostauth.Authorize(r) {
		writeAPIJSON(w, http.StatusForbidden, apiError{Error: "internal runtime endpoints are not publicly accessible"})
		return
	}
	leg, err := internalIsolationSelfTestLeg(r)
	if err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}
	if h == nil || h.rt == nil {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "isolation self-test unavailable"})
		return
	}
	if !isolationSelfTestMu.TryLock() {
		writeAPIJSON(w, http.StatusConflict, apiError{Error: "an isolation self-test is already running"})
		return
	}
	defer isolationSelfTestMu.Unlock()
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()

	resp := isolationSelfTestResponse{ComputerID: h.rt.cfg.ComputerID, Leg: leg}
	var failures []string
	switch leg {
	case isolationLegOOM:
		rec := yaegikernel.DeskOOMSelfTest(ctx, deskOOMSelfTestConfig(h.rt.cfg.ComputerID))
		resp.OOM, failures = &rec, rec.Failures
	case isolationLegSeccomp:
		rec := capsule.SeccompSelfTest(ctx, daemonSelfTestBinary())
		resp.Seccomp, failures = &rec, rec.Failures
	}
	for _, f := range failures {
		resp.Failures = append(resp.Failures, leg+": "+f)
	}
	resp.OK = len(resp.Failures) == 0
	writeAPIJSON(w, http.StatusOK, resp)
}

// internalIsolationSelfTestLeg reads the one input the internal route takes:
// exactly one of leg=oom, leg=seccomp, and nothing else.
func internalIsolationSelfTestLeg(r *http.Request) (string, error) {
	q := r.URL.Query()
	if len(q) == 1 && len(q["leg"]) == 1 {
		switch leg := q.Get("leg"); leg {
		case isolationLegOOM, isolationLegSeccomp:
			return leg, nil
		}
	}
	return "", errors.New("unknown self-test leg: the selectors are leg=oom and leg=seccomp")
}

// deskOOMSelfTestConfig is the OOM leg's: the research desk (untrusted input)
// on desk_go_eval's production worker config.
func deskOOMSelfTestConfig(computerID string) yaegikernel.DeskOOMSelfTestConfig {
	return yaegikernel.DeskOOMSelfTestConfig{
		Base: deskCellSelfTestConfig(computerID).Base,
		Role: "research",
	}
}

// deskCellSelfTestConfig is desk_go_eval's worker config (tools_desk.go)
// without a run: the same binary, entrypoint, allowed root, memory cap,
// environment and confinement mode.
func deskCellSelfTestConfig(computerID string) yaegikernel.DeskCellSelfTestConfig {
	return yaegikernel.DeskCellSelfTestConfig{
		Base: yaegikernel.DeskSessionWorkerConfig{
			Bin:       deskWorkerBinary(),
			WorkerArg: deskWorkerArg,
			Session: yaegikernel.SessionWorkerConfig{
				AllowedPackages:  yaegikernel.DefaultSafeStdlibPackagesList(),
				ComputerID:       computerID,
				ActivationID:     "isolation-selftest-" + strconv.FormatInt(time.Now().UnixNano(), 36),
				Epoch:            1,
				AllowedRoot:      os.TempDir(),
				MemoryLimitBytes: deskWorkerMemoryLimitBytes,
			},
			ProcessGroup: true,
			Confinement:  yaegikernel.CellConfinement{Mode: yaegikernel.DeskCellConfinementModeFromEnv(os.Getenv)},
			ExtraEnv:     gomemlimit.WorkerEnv(os.Getenv),
		},
		// Things a desk cell must never read: the owner's files and the
		// runtime's store (world-readable directories: only Landlock
		// refuses them), the credential disk's mount (it carries the
		// Claude delegate's token on an allowlisted computer) and its
		// gateway token (root 0600), and the guest signer sockets'
		// directory.
		ExtraReads: []string{
			os.Getenv("AUTOPUTER_FILES_ROOT"),
			os.Getenv("RUNTIME_STORE_PATH"),
			strings.TrimSpace(os.Getenv("CHOIR_CLAUDE_DELEGATE_CREDENTIAL_DIR")),
			strings.TrimSpace(os.Getenv("RUNTIME_GATEWAY_TOKEN_FILE")),
			"/run/choir-signers",
		},
	}
}

// guestEgressTargets derives the probe set from the gateway URL's host (the
// guest's tap peer, which is the host). The host policy
// (internal/vmmanager/guest_egress.go) admits the tap service ports and
// 80/443 on host addresses and drops everything else there, every private
// and link-local range, and SMTP.
func guestEgressTargets(gatewayURL string) (string, []isolationEgressProbe) {
	u, err := url.Parse(strings.TrimSpace(gatewayURL))
	if err != nil {
		return "", nil
	}
	ip := net.ParseIP(u.Hostname())
	if ip == nil || ip.To4() == nil {
		return "", nil
	}
	host := ip.To4().String()
	// The next /30's guest address: another computer's guest (or an unused
	// address in the pool); either way private and dropped.
	neighbor := append(net.IP(nil), ip.To4()...)
	neighbor[3] += 5
	return host, []isolationEgressProbe{
		{Name: "host_gateway", Addr: net.JoinHostPort(host, "8084"), Want: "open"},
		{Name: "host_corpusd", Addr: net.JoinHostPort(host, "8086"), Want: "open"},
		{Name: "host_sshd", Addr: net.JoinHostPort(host, "22"), Want: "blocked"},
		{Name: "host_dolt", Addr: net.JoinHostPort(host, "13306"), Want: "blocked"},
		{Name: "host_caddy_admin", Addr: net.JoinHostPort(host, "2019"), Want: "blocked"},
		{Name: "metadata", Addr: "169.254.169.254:80", Want: "blocked"},
		{Name: "other_guest", Addr: net.JoinHostPort(neighbor.String(), "8085"), Want: "blocked"},
		{Name: "public_https", Addr: "1.1.1.1:443", Want: "open"},
	}
}

func probeGuestEgress(ctx context.Context, targets []isolationEgressProbe, timeout time.Duration) []isolationEgressProbe {
	out := make([]isolationEgressProbe, len(targets))
	var wg sync.WaitGroup
	for i, t := range targets {
		wg.Add(1)
		go func(i int, t isolationEgressProbe) {
			defer wg.Done()
			d := net.Dialer{Timeout: timeout}
			c, err := d.DialContext(ctx, "tcp", t.Addr)
			switch {
			case err == nil:
				_ = c.Close()
				t.Result = "open"
			case errors.Is(err, syscall.ECONNREFUSED):
				t.Result = "refused"
			case errors.Is(err, syscall.EHOSTUNREACH), errors.Is(err, syscall.ENETUNREACH):
				t.Result = "unreachable"
			default:
				var ne net.Error
				if errors.As(err, &ne) && ne.Timeout() {
					t.Result = "dropped"
				} else {
					t.Result = "error"
				}
			}
			if err != nil {
				t.Detail = err.Error()
			}
			switch t.Want {
			case "open":
				t.OK = t.Result == "open"
			default:
				t.OK = t.Result == "dropped" || t.Result == "unreachable"
			}
			out[i] = t
		}(i, t)
	}
	wg.Wait()
	return out
}
