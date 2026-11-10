package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computertiers"
	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/platform"
	"github.com/yusefmosiah/go-choir/internal/server"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

// The platform operator's acts on platform computers, from the operator's own
// account (docs/design/platform-operator-account-2026-11-06.md section 5,
// slices O2 and O3; docs/design/paper-control-cli-2026-11-09.md).
//
//	GET  /api/platform/operator/computers                      status of every platform computer (O2)
//	ANY  /api/platform/operator/computers/<id>/guest/<route>   one allowlisted act on that computer (O3)
//
// Every request passes authorizePlatformComputerAct, the one gate:
//
//  1. the caller is a passkey session, or an API key that is not bound to a
//     computer and carries the scope the route's method needs (the key keeps
//     its scopes: a read key cannot write);
//  2. the caller's account id is, exactly, in the operator roster. The roster
//     (CHOIR_PLATFORM_OPERATOR_USERS) is read only by corpusd; the proxy asks
//     it, and any failure to get a yes is a refusal;
//  3. the computer named in the PATH (never a header or query) is, exactly,
//     in the platform tier of the embedded roster, and vmctl knows it.
//
// The act is then forwarded to the guest as that computer's owner, on a
// request built from nothing: no Authorization, no Cookie and no client
// header reaches the guest, and the operator's account id rides in
// server.PlatformOperatorHeader so the guest records a receipt on its tape
// before the write lands (internal/autoputer/policy_write.go). The proxy sends
// the guest only the routes in operatorGuestRoute.

const (
	platformOperatorComputersPath = "/api/platform/operator/computers"
	operatorGuestSegment          = "guest"
	// operatorMaxBody bounds a forwarded request body.
	operatorMaxBody = 1 << 20
)

// platformRoster is the embedded tier roster; a test seam. A load error
// admits no computer.
var platformRoster = computertiers.Load

// operatorRevisionName is the shape the guest gives a kept policy revision.
var operatorRevisionName = regexp.MustCompile(`^[0-9]{8}T[0-9]{6}\.[0-9]{6}Z(-[0-9]+)?\.toml$`)

// operatorRoleName bounds the role query of a resolve.
var operatorRoleName = regexp.MustCompile(`^[a-z][a-z0-9_]{0,40}$`)

// operatorAct names what a request does, for the gate and the log line.
type operatorAct string

const (
	operatorActStatus operatorAct = "platform.computer.status"
	operatorActRead   operatorAct = "platform.computer.read"
	operatorActPolicy operatorAct = "platform.computer.model_policy"
	operatorActVerify operatorAct = "platform.computer.model_policy.verify"
	// O4 and O5 (docs/design/platform-operator-account-2026-11-06.md section 9).
	operatorActSchedule operatorAct = "platform.computer.schedule"
	operatorActRestart  operatorAct = "platform.computer.restart"
)

// platformActTarget is a platform computer the gate admitted an act on.
type platformActTarget struct {
	Target     *resolvedComputerTarget
	OperatorID string
	// GuestRecordsReceipts is set by the last guest call: the guest said, in
	// operatorReceiptsHeader, that it records operator receipts.
	GuestRecordsReceipts bool
}

// operatorReceiptsHeader is the guest's statement that a marked request is
// recorded on its tape (internal/autoputer/policy_write.go). The proxy reads
// it and relays nothing else of the guest's headers.
const (
	operatorReceiptsHeader = "X-Choir-Operator-Receipts"
	policyGuest            = "/api/files/System/model-policy.toml"
	schedulesGuest         = "/api/autopaper/sources/schedules"
	restartReceiptGuest    = "/api/platform/operator/restart-receipt"
	operatorRestartSegment = "restart"
	operatorReasonMax      = 280
)

// platformOperatorHolds asks corpusd, the only reader of the roster, whether
// the account holds the role. Any answer but an explicit yes is a no.
func (h *Handler) platformOperatorHolds(ctx context.Context, userID string) (holds bool, err error) {
	if strings.TrimSpace(userID) == "" || userID != strings.TrimSpace(userID) {
		return false, nil
	}
	status, raw, err := h.corpusdCall(ctx, http.MethodGet, platform.PlatformOperatorCheckRoute+"?user_id="+url.QueryEscape(userID), "", nil)
	if err != nil {
		return false, err
	}
	if status != http.StatusOK {
		return false, fmt.Errorf("operator check answered %d", status)
	}
	var out platform.PlatformOperatorCheck
	if err := json.Unmarshal(raw, &out); err != nil {
		return false, fmt.Errorf("operator check: %w", err)
	}
	return out.Operator, nil
}

// authorizePlatformOperator is steps 1 and 2 of the gate: who the caller is.
// It writes the refusal and returns "" when the caller is not an operator.
func (h *Handler) authorizePlatformOperator(w http.ResponseWriter, r *http.Request) (*AuthResult, bool) {
	authResult, err := h.authenticate(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "authentication required"})
		return nil, false
	}
	switch authResult.AuthMethod {
	case "cookie":
		// A passkey session only: an email session is as strong as the inbox.
		if authResult.SignInMethod != "passkey" {
			writeJSON(w, http.StatusForbidden, errorResponse{Error: "Sign in with a passkey to do this."})
			return nil, false
		}
	case "api_key":
		// A key bound to one computer is attenuated to that computer, which
		// is never a platform computer; the key keeps its scopes.
		if strings.TrimSpace(authResult.ComputerID) != "" {
			writeJSON(w, http.StatusForbidden, errorResponse{Error: "platform operator act: a computer-bound api key does not carry the role"})
			return nil, false
		}
		if !h.authorizeAPIKeyScope(w, r, authResult) {
			return nil, false
		}
	default:
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "platform operator act: not permitted"})
		return nil, false
	}
	holds, err := h.platformOperatorHolds(r.Context(), authResult.UserID)
	if err != nil {
		log.Printf("proxy: platform operator check failed (refusing): %v", err)
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "platform operator authority unavailable"})
		return nil, false
	}
	if !holds {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "not a platform operator"})
		return nil, false
	}
	return authResult, true
}

// platformComputerOwnership is step 3: the computer is on the platform tier (or is the named proof computer)
// and vmctl knows it. computerID must be exactly the roster's spelling.
func (h *Handler) platformComputerOwnership(ctx context.Context, computerID string) (*resolvedComputerTarget, int, string) {
	if computerID == "" || computerID != strings.TrimSpace(computerID) {
		return nil, http.StatusForbidden, "not a platform computer"
	}
	tiers, err := platformRoster()
	if err != nil || !tiers.OperatorActComputer(computerID) {
		return nil, http.StatusForbidden, "not a platform computer"
	}
	if h.vmctlClient == nil {
		return nil, http.StatusServiceUnavailable, "computer ownership authority unavailable"
	}
	ownership, err := h.vmctlClient.LookupComputerByIDContext(ctx, computerID)
	if err != nil {
		return nil, http.StatusServiceUnavailable, "computer ownership authority unavailable"
	}
	if ownership == nil {
		return nil, http.StatusNotFound, "platform computer not found"
	}
	if ownership.ComputerID != computerID || strings.TrimSpace(ownership.UserID) == "" || ownership.UserID != strings.TrimSpace(ownership.UserID) || strings.TrimSpace(ownership.DesktopID) == "" {
		return nil, http.StatusServiceUnavailable, "computer ownership authority unavailable"
	}
	return &resolvedComputerTarget{
		ComputerID: ownership.ComputerID, UserID: ownership.UserID, DesktopID: ownership.DesktopID,
		VMID: strings.TrimSpace(ownership.VMID), ComputerURL: strings.TrimSpace(ownership.ComputerURL),
		State: strings.TrimSpace(ownership.State), Epoch: ownership.Epoch,
		Held: ownership.Held, HoldReason: ownership.HoldReason, HeldBy: ownership.HeldBy,
	}, 0, ""
}

// authorizePlatformComputerAct is the one gate for an operator's act on a
// platform computer. It writes the refusal and returns false unless the
// caller holds the role and the computer is on the platform tier.
func (h *Handler) authorizePlatformComputerAct(w http.ResponseWriter, r *http.Request, computerID string, act operatorAct) (*platformActTarget, bool) {
	authResult, ok := h.authorizePlatformOperator(w, r)
	if !ok {
		return nil, false
	}
	target, status, msg := h.platformComputerOwnership(r.Context(), computerID)
	if target == nil {
		writeJSON(w, status, errorResponse{Error: msg})
		return nil, false
	}
	log.Printf("proxy: platform operator act %s on %s by account %s via %s", act, computerID, authResult.UserID, authResult.AuthMethod)
	return &platformActTarget{Target: target, OperatorID: authResult.UserID}, true
}

// ownerAuthFor is the authentication the guest is shown: the computer's own
// owner, bound to that one computer. It is built here and never from the
// operator's credential.
func ownerAuthFor(t *resolvedComputerTarget, scopes []string) *AuthResult {
	return &AuthResult{UserID: t.UserID, Valid: true, AuthMethod: "api_key", ComputerID: t.ComputerID, Scopes: scopes}
}

// operatorGuestRoute maps (method, path after /guest, query) to the one guest
// route the operator may reach. Everything else is refused; the DELETE and
// PATCH verbs of the files route are not among them.
func operatorGuestRoute(method, rest string, query url.Values) (guestPath string, act operatorAct, ok bool) {
	const files = "/api/files/"
	switch {
	case rest == files+"System/model-policy.toml" && method == http.MethodGet && len(query) == 0:
		return rest, operatorActRead, true
	case rest == files+"System/model-policy.toml" && method == http.MethodPut && len(query) == 0:
		return rest, operatorActPolicy, true
	case rest == files+"System/model-policy.revisions" && method == http.MethodGet && len(query) == 0:
		return rest, operatorActRead, true
	case strings.HasPrefix(rest, files+"System/model-policy.revisions/") && method == http.MethodGet && len(query) == 0:
		if operatorRevisionName.MatchString(strings.TrimPrefix(rest, files+"System/model-policy.revisions/")) {
			return rest, operatorActRead, true
		}
	case rest == "/api/model-policy/resolve" && method == http.MethodGet:
		if role := query.Get("role"); len(query) == 1 && operatorRoleName.MatchString(role) {
			return rest, operatorActRead, true
		}
	case rest == "/api/model-policy/verify" && method == http.MethodPost && len(query) == 0:
		return rest, operatorActVerify, true
	case rest == schedulesGuest && method == http.MethodGet && len(query) == 0:
		return rest, operatorActRead, true
	case rest == schedulesGuest && method == http.MethodPost && len(query) == 0:
		return rest, operatorActSchedule, true
	case rest == schedulesGuest && method == http.MethodDelete && operatorStopQuery(query):
		return rest, operatorActSchedule, true
	}
	return "", "", false
}

// operatorStopQuery admits the query of a schedule stop: source_id (once,
// required) and reason (at most once), nothing else.
func operatorStopQuery(q url.Values) bool {
	for k, v := range q {
		if (k != "source_id" && k != "reason") || len(v) != 1 {
			return false
		}
	}
	return q.Get("source_id") != "" && len(q.Get("reason")) <= operatorReasonMax
}

// operatorScheduleStartBody parses an operator's schedule start and returns
// the body the proxy forwards, rebuilt from the parsed fields: the four the
// operator may set. Anything else (a feed URL, a fixture run, a second
// document) is refused here and again at the guest.
func operatorScheduleStartBody(raw []byte) (string, bool) {
	var in struct {
		SourceID        string `json:"source_id"`
		IntervalMinutes int    `json:"interval_minutes"`
		MaxItems        int    `json:"max_items"`
		Reason          string `json:"reason"`
	}
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil || dec.More() || strings.TrimSpace(in.SourceID) == "" || len(in.Reason) > operatorReasonMax ||
		in.IntervalMinutes < 0 || in.MaxItems < 0 {
		return "", false
	}
	out, err := json.Marshal(in)
	if err != nil {
		return "", false
	}
	return string(out), true
}

// splitOperatorRestartPath splits /api/platform/operator/computers/<id>/restart.
func splitOperatorRestartPath(path string) (computerID string, ok bool) {
	tail := strings.TrimPrefix(path, platformOperatorComputersPath+"/")
	id, seg, found := strings.Cut(tail, "/")
	if !found || id == "" || seg != operatorRestartSegment {
		return "", false
	}
	return id, true
}

// splitOperatorComputerPath splits /api/platform/operator/computers/<id>/guest/<rest>.
func splitOperatorComputerPath(path string) (computerID, rest string, ok bool) {
	tail := strings.TrimPrefix(path, platformOperatorComputersPath+"/")
	id, after, found := strings.Cut(tail, "/")
	if !found || id == "" {
		return "", "", false
	}
	seg, rest, found := strings.Cut(after, "/")
	if !found || seg != operatorGuestSegment {
		return "", "", false
	}
	return id, "/" + rest, true
}

func isPlatformOperatorComputersPath(path string) bool {
	return path == platformOperatorComputersPath || strings.HasPrefix(path, platformOperatorComputersPath+"/")
}

// HandlePlatformOperatorComputers serves the operator's computer routes.
func (h *Handler) HandlePlatformOperatorComputers(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == platformOperatorComputersPath {
		h.handlePlatformOperatorStatus(w, r)
		return
	}
	if id, ok := splitOperatorRestartPath(r.URL.Path); ok && r.URL.EscapedPath() == r.URL.Path {
		h.handlePlatformOperatorRestart(w, r, id)
		return
	}
	id, rest, ok := splitOperatorComputerPath(r.URL.Path)
	if !ok || r.URL.EscapedPath() != r.URL.Path {
		// Unauthenticated callers learn nothing about the route shape.
		if _, err := h.authenticate(r); err != nil {
			writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "authentication required"})
			return
		}
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "not found"})
		return
	}
	h.handlePlatformOperatorGuest(w, r, id, rest)
}

func (h *Handler) handlePlatformOperatorGuest(w http.ResponseWriter, r *http.Request, computerID, rest string) {
	guestPath, act, ok := operatorGuestRoute(r.Method, rest, r.URL.Query())
	if !ok {
		// Authenticate first so the allowlist is not an oracle for strangers.
		if _, authed := h.authorizePlatformOperator(w, r); authed {
			writeJSON(w, http.StatusForbidden, errorResponse{Error: "platform operator act: route not permitted"})
		}
		return
	}
	pa, ok := h.authorizePlatformComputerAct(w, r, computerID, act)
	if !ok {
		return
	}
	var body io.Reader
	rawQuery := r.URL.RawQuery
	if r.Body != nil && r.Method != http.MethodGet {
		raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, operatorMaxBody))
		if err != nil {
			writeJSON(w, http.StatusRequestEntityTooLarge, errorResponse{Error: "request body too large"})
			return
		}
		if act == operatorActSchedule && r.Method == http.MethodPost {
			rebuilt, ok := operatorScheduleStartBody(raw)
			if !ok {
				writeJSON(w, http.StatusBadRequest, errorResponse{Error: "an operator schedule start takes source_id, interval_minutes, max_items and reason only"})
				return
			}
			raw = []byte(rebuilt)
		}
		body = strings.NewReader(string(raw))
	}
	if act == operatorActSchedule && r.Method == http.MethodDelete {
		// Rebuilt from the two admitted keys, so the guest sees nothing else.
		q := r.URL.Query()
		rawQuery = url.Values{"source_id": {q.Get("source_id")}, "reason": {q.Get("reason")}}.Encode()
	}
	if act == operatorActPolicy || act == operatorActSchedule {
		// A guest from before the operator receipt ignores the marker and
		// would write with no receipt. Ask first (a read of the same
		// surface); a guest that does not say it records receipts is not
		// written to.
		probe := policyGuest
		if act == operatorActSchedule {
			probe = schedulesGuest
		}
		if _, _, _, err := h.operatorGuestCall(r.Context(), pa, http.MethodGet, probe, "", "", nil, true); err != nil {
			writeAutoputerResolveFailure(w, http.StatusBadGateway, "platform computer unavailable", err)
			return
		}
		if !pa.GuestRecordsReceipts {
			writeJSON(w, http.StatusConflict, errorResponse{Error: "the computer's build does not record operator receipts yet; it will after its next update, and nothing was written"})
			return
		}
	}
	status, header, raw, err := h.operatorGuestCall(r.Context(), pa, r.Method, guestPath, rawQuery, r.Header.Get("Content-Type"), body, true)
	if err != nil {
		writeAutoputerResolveFailure(w, http.StatusBadGateway, "platform computer unavailable", err)
		return
	}
	if header != "" {
		w.Header().Set("Content-Type", header)
	}
	w.WriteHeader(status)
	_, _ = w.Write(raw)
}

// operatorGuestCall sends one request to the target's guest as its owner.
// The request is built here from nothing; the response is returned as status,
// content type and body only, so no header of the guest's reaches the caller.
func (h *Handler) operatorGuestCall(ctx context.Context, pa *platformActTarget, method, guestPath, rawQuery, contentType string, body io.Reader, wake bool) (int, string, []byte, error) {
	target := *pa.Target
	owner := ownerAuthFor(&target, nil)
	var base string
	var err error
	if wake {
		base, err = h.resolveComputerURLForComputerTarget(ctx, owner, &target, target.DesktopID)
	} else {
		base, err = h.resolveActiveComputerURLForComputerTarget(ctx, owner, &target, target.DesktopID)
	}
	if err != nil {
		return 0, "", nil, err
	}
	full, err := joinBasePath(base, guestPath)
	if err != nil {
		return 0, "", nil, err
	}
	if rawQuery != "" {
		full += "?" + rawQuery
	}
	req, err := http.NewRequestWithContext(ctx, method, full, body)
	if err != nil {
		return 0, "", nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("X-Authenticated-User", target.UserID)
	req.Header.Set("X-Authenticated-Computer", target.ComputerID)
	req.Header.Set(server.PlatformOperatorHeader, pa.OperatorID)
	client := h.autoputerHTTP
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, "", nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	pa.GuestRecordsReceipts = resp.Header.Get(operatorReceiptsHeader) == "1"
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return 0, "", nil, err
	}
	return resp.StatusCode, resp.Header.Get("Content-Type"), raw, nil
}

// operatorComputerStatus is one platform computer in the status answer.
type operatorComputerStatus struct {
	ComputerID        string `json:"computer_id"`
	Tier              string `json:"tier"`
	State             string `json:"state,omitempty"`
	WarmnessClass     string `json:"warmness_class,omitempty"`
	Held              bool   `json:"held,omitempty"`
	HoldReason        string `json:"hold_reason,omitempty"`
	LastActiveAt      string `json:"last_active_at,omitempty"`
	PlatformBaseRef   string `json:"platform_base_ref,omitempty"`
	ConstructionBuild any    `json:"construction_version,omitempty"`
	ReportedRelease   any    `json:"reported_release,omitempty"`
	// Schedules is the guest's source schedule list, as the guest reports it.
	Schedules json.RawMessage `json:"schedules,omitempty"`
	// Policy is the resolved model policy per role, as the guest reports it.
	Policy map[string]json.RawMessage `json:"policy,omitempty"`
	// Errors names what could not be read (a stopped guest, a missing route);
	// a field absent without an error here is a defect.
	Errors []string `json:"errors,omitempty"`
}

// operatorPolicyRoles are the roles whose resolve the status shows.
func operatorPolicyRoles() []string {
	return []string{deskmanifest.DocumentCustodianName(), deskmanifest.AdmitterName(), deskmanifest.TaskRoleName(), deskmanifest.ResearcherName(), deskmanifest.PerformerName()}
}

func (h *Handler) handlePlatformOperatorStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	operatorID, ok := h.authorizePlatformOperator(w, r)
	if !ok {
		return
	}
	tiers, err := platformRoster()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "computer roster unavailable"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	out := []operatorComputerStatus{}
	for _, id := range tiers.PlatformComputerIDs() {
		st := operatorComputerStatus{ComputerID: id, Tier: "platform"}
		target, _, msg := h.platformComputerOwnership(ctx, id)
		if target == nil {
			st.Errors = append(st.Errors, msg)
			out = append(out, st)
			continue
		}
		pa := &platformActTarget{Target: target, OperatorID: operatorID.UserID}
		st.State, st.Held, st.HoldReason = target.State, target.Held, target.HoldReason
		if ownership, err := h.vmctlClient.LookupComputerByIDContext(ctx, id); err == nil && ownership != nil {
			st.WarmnessClass, st.LastActiveAt, st.PlatformBaseRef = ownership.WarmnessClass, ownership.LastActiveAt, ownership.PlatformBaseRef
			if ownership.ConstructionVersion != nil {
				st.ConstructionBuild = ownership.ConstructionVersion
			}
			if ownership.ReportedRelease != nil {
				st.ReportedRelease = ownership.ReportedRelease
			}
		}
		// Status reads never wake a stopped guest.
		if status, _, raw, err := h.operatorGuestCall(ctx, pa, http.MethodGet, "/api/autopaper/sources/schedules", "", "", nil, false); err != nil {
			st.Errors = append(st.Errors, "schedules: guest not reachable ("+shortOperatorErr(err)+")")
		} else if status != http.StatusOK {
			st.Errors = append(st.Errors, fmt.Sprintf("schedules: guest answered %d", status))
		} else {
			st.Schedules = json.RawMessage(raw)
		}
		st.Policy = map[string]json.RawMessage{}
		for _, role := range operatorPolicyRoles() {
			status, _, raw, err := h.operatorGuestCall(ctx, pa, http.MethodGet, "/api/model-policy/resolve", "role="+url.QueryEscape(role), "", nil, false)
			if err != nil {
				st.Errors = append(st.Errors, "policy "+role+": guest not reachable ("+shortOperatorErr(err)+")")
				break
			}
			if status != http.StatusOK {
				st.Errors = append(st.Errors, fmt.Sprintf("policy %s: guest answered %d", role, status))
				continue
			}
			st.Policy[role] = json.RawMessage(raw)
		}
		out = append(out, st)
	}
	writeJSON(w, http.StatusOK, map[string]any{"computers": out})
}

// shortOperatorErr says why a guest read failed without repeating the
// error text, which can carry a host or guest address.
func shortOperatorErr(err error) string {
	var wake *computerWakeError
	if errors.As(err, &wake) {
		return "computer " + wake.State
	}
	return "unreachable"
}

// handlePlatformOperatorRestart is O5: POST .../computers/<id>/restart with a
// reason. The restart is vmctl's owner update (a planned, marker-backed
// refresh, owner_update.go) and nothing else: the guest refuses it mid
// self-development apply or while an engineering capsule is live, and vmctl
// defers a held or in-flight computer. This handler never calls vmctl's plain
// stop, start, restart or refresh, so it cannot end work the way a crash
// restart does (CLAUDE.md "Restarts End Work (Crash) Or Resume It (Planned
// Update)"). The receipt is recorded on the computer's tape first; with no
// receipt there is no restart. It does not wake a computer that is not active.
func (h *Handler) handlePlatformOperatorRestart(w http.ResponseWriter, r *http.Request, computerID string) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	pa, ok := h.authorizePlatformComputerAct(w, r, computerID, operatorActRestart)
	if !ok {
		return
	}
	var in struct {
		Reason string `json:"reason"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil || dec.More() || strings.TrimSpace(in.Reason) == "" || len(in.Reason) > operatorReasonMax {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "a restart needs a reason of at most 280 bytes, and no other field"})
		return
	}
	if pa.Target.State != "active" {
		writeJSON(w, http.StatusConflict, map[string]any{"outcome": "skipped", "reason_code": "not_active",
			"reason": "the computer is not active (state=" + pa.Target.State + "); nothing was restarted"})
		return
	}
	reqBody, _ := json.Marshal(map[string]string{"reason": in.Reason})
	status, _, raw, err := h.operatorGuestCall(r.Context(), pa, http.MethodPost, restartReceiptGuest, "", "application/json", strings.NewReader(string(reqBody)), false)
	if err != nil {
		writeAutoputerResolveFailure(w, http.StatusBadGateway, "platform computer unavailable", err)
		return
	}
	if !pa.GuestRecordsReceipts {
		writeJSON(w, http.StatusConflict, errorResponse{Error: "the computer's build does not record operator receipts yet; it will after its next update, and nothing was restarted"})
		return
	}
	var receipt struct {
		Receipt string `json:"receipt"`
	}
	if status != http.StatusOK || json.Unmarshal(raw, &receipt) != nil || receipt.Receipt == "" {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "the restart receipt could not be recorded; nothing was restarted"})
		return
	}
	if h.vmctlClient == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "computer ownership authority unavailable"})
		return
	}
	opCtx, cancel := h.vmctlOpContext(vmctl.WithLifecycleCaller(r.Context(), "proxy.operator.restart"))
	defer cancel()
	updated, err := h.vmctlClient.OwnerUpdateContext(opCtx, pa.Target.UserID, pa.Target.DesktopID)
	if err != nil {
		log.Printf("proxy: platform operator restart of %s: %v", computerID, err)
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": "the planned restart could not be asked of vmctl", "receipt": receipt.Receipt})
		return
	}
	out := map[string]any{"outcome": updated.Outcome, "receipt": receipt.Receipt}
	if updated.Target != "" {
		out["target"] = updated.Target
	}
	switch updated.Outcome {
	case "refreshed", "current":
		writeJSON(w, http.StatusOK, out)
		return
	}
	code := updated.ReasonCode
	if code == "" {
		code = updated.Reason
	}
	if code != "" {
		out["reason_code"] = code
	}
	if updated.Reason != "" {
		out["reason"] = updated.Reason
	}
	if len(updated.Refusal) > 0 {
		out["detail"] = updated.Refusal
	}
	if updated.Outcome == "failed" {
		writeJSON(w, http.StatusInternalServerError, out)
		return
	}
	writeJSON(w, http.StatusConflict, out)
}
