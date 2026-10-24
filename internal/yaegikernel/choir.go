package yaegikernel

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/traefik/yaegi/interp"
	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/texturewrite"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// ChoirScope binds the prebound choir modules to one activation: the broker
// that executes, a session-scoped handle, and the activation identity
// reported by Context. The issuer is worker-scoped (per-session secret in
// production): the socket capability check at the broker boundary already
// authorized this activation, and in-worker handles only preserve the
// single-dispatch Broker API shape (no second trust root, no host key
// material in model reach).
type ChoirScope struct {
	broker *Broker
	// handleRef is reissued when a cell binds (the handle expires, the desk
	// session does not); a goroutine left from an earlier cell may read it
	// meanwhile, hence atomic.
	handleRef    atomic.Pointer[string]
	issuer       *HandleIssuer
	actions      []BrokerAction
	computerID   string
	epoch        uint64
	activationID string
	readOnly     bool
	// desk is the agentprofile desk identity carried from the verified outer
	// capability (management|engineering|research|texture). It selects the
	// per-desk choir module set in ChoirExports; the model can never set it.
	desk string
	// slot is the Engineering slot carried from the verified capability
	// (implementation|verifier). Verifier-only affordances gate on it; the
	// model can never set it.
	slot string
	// Cell binding (RLM Step 4): while a cell is bound, messaging and
	// delegation stage into the tray and return in microseconds without
	// blocking; Inbox() reads the bound snapshot. Unbound scopes keep the
	// legacy synchronous broker behavior (one-shot path). The session loop
	// owns binding lifetime: exactly one cell binds at a time.
	tray  *Tray
	inbox []IncomingMessage
	// jobs is the cell-start snapshot of this run's capsule jobs, read by
	// Poll (jobs.go).
	jobs []JobView
	// awaited are the jobs a Poll in the bound cell returned unfinished, in
	// first-poll order: the host parks the turn on them (jobs.go).
	awaitMu sync.Mutex
	awaited []string
	// doc is the cell-start texture document snapshot (R3d) a texture cell
	// reads via ReadDoc(): the bound doc's current revision id + content.
	doc *DocSnapshot
	// pack is the acting desk's cell-start commitment context (R4): its
	// committed/addressed acts joined with resolution observations —
	// score-free by construction (types.ActingPack has no score fields).
	pack *types.ActingPack
	// updates is the cell-start snapshot of pending update_coagent records
	// (RLM prompt-as-variable): read via Updates(). The wake turn in chat
	// carries only update ids; the payload lives here.
	updates []PendingUpdate
	// emits is the cell-start snapshot of emitted signals addressed to this
	// desk (boundary-drain): read via Emits(). The notice turn carries only
	// a fixed-format line; the untrusted body lives here.
	emits []PendingEmit
	// records is the cell-start snapshot of records addressed to this
	// desk's inbox in (cursor, head], read via Records().
	records []InboxRecord
	// recordsShown is the highest record seq a Records() call in the bound
	// cell returned, 0 when the cell never called it: the cell's commit
	// marks read only what the desk was shown.
	recordsShown atomic.Int64
	// refusals are the errors choir verbs returned during the bound cell,
	// shown in its result so a refusal is never lost when the cell ends in
	// a bare call (problems/desk-verb-error-vanishes-as-final-expression-2026-10-11.md).
	refusalMu sync.Mutex
	refusals  []verbRefusal
	// searches is this session's async search store (search_store.go): the
	// worker's reader loop writes it, cells read it through choir.Searches,
	// and a cell's end does not clear it. searchTold is how many of each
	// search's arrivals the desk has been told about in a cell result.
	searches   *SearchStore
	searchTold map[string]int
	// cellOuts keeps this session's cut cell outputs for choir.CellOut (LR
	// lever 6, cell_output.go). Only a desk worker started with
	// KeepCellOutput fills it.
	cellOuts cellOutputStore
}

type verbRefusal struct {
	verb string
	err  string
}

// SessionRoleResearch is the read-only role: sessions bound to it observe
// files and directories but cannot write, execute, or message. The
// role arrives on a trusted worker flag from the verified outer capability,
// never from model input. Any other role string means full engineering scope.
const SessionRoleResearch = "research"

// SessionRoleResearchChild is a child research activation run for a
// research cell's choir.Research: read-only world verbs and further
// choir.Research, no messaging or reporting. Its result returns to the
// parent cell as a value.
const SessionRoleResearchChild = "research_child"

// SessionRoleManagementTask is a management task (management delegation
// S2a, design round 2 R2-1/R2-2): a bounded sub-RLM of the management desk
// that disposes of the captures its lease names. It reads records and
// captures and stages Cast, Note, Feed, Decline and Done; it has no
// filesystem, assignment, message, product API or ledger authority. The
// spelling is agentprofile.ManagementTask's.
const SessionRoleManagementTask = "management_task"

// NewChoirScope mints a session-scoped handle for exactly the actions the
// desk's manifest row gives it and returns the scope the choir symbols close
// over.
// The caller (worker session setup) owns the broker lifetime.
func NewChoirScope(broker *Broker, issuer *HandleIssuer, computerID, activationID string, epoch uint64, role, slot string) (*ChoirScope, error) {
	if broker == nil {
		return nil, fmt.Errorf("choir: broker is required")
	}
	if issuer == nil {
		return nil, fmt.Errorf("choir: handle issuer is required")
	}
	desk := normalizeDeskRole(role)
	readOnly := role == SessionRoleResearch || role == SessionRoleResearchChild
	if desk == "" || desk == SessionRoleManagementTask {
		// An unknown role fails closed (R2-2): observation only. A task
		// stages acts the host reduces and touches no filesystem.
		readOnly = true
	}
	// The handle's scopes are the desk's manifest row (internal/deskmanifest):
	// the observation actions its worker serves itself, plus the host-served
	// actions its verbs dispatch. exec and write_file are engineering's only,
	// and the retired message and emit are no desk's. A role the manifest does
	// not name gets observation only.
	actions := []BrokerAction{ActionReadFile, ActionListDir}
	if d, ok := deskmanifest.Lookup(desk); ok && desk != "" {
		actions = actions[:0]
		for _, a := range d.HandleActions() {
			actions = append(actions, BrokerAction(a))
		}
	}
	handleRef, err := issuer.Issue(computerID, "choir-session", epoch, actions, sessionHandleTTL)
	if err != nil {
		return nil, fmt.Errorf("choir: issue session handle: %w", err)
	}
	scope := &ChoirScope{broker: broker, issuer: issuer, actions: actions, computerID: computerID, epoch: epoch, activationID: activationID, readOnly: readOnly, desk: desk, slot: slot, searches: newSearchStore()}
	scope.handleRef.Store(&handleRef)
	return scope, nil
}

// reissueHandle gives the scope a fresh session handle with the same
// computer, epoch and actions. On failure the old handle stays and its
// verbs report expiry.
func (s *ChoirScope) reissueHandle() {
	if s.issuer == nil {
		return
	}
	if ref, err := s.issuer.Issue(s.computerID, "choir-session", s.epoch, s.actions, sessionHandleTTL); err == nil {
		s.handleRef.Store(&ref)
	}
}

// sessionHandleTTL is how long one issued session handle verifies. A var so
// a test can shorten it.
var sessionHandleTTL = time.Hour

// HandleRef returns the bound session handle for host-bound frame requests.
func (s *ChoirScope) HandleRef() string { return *s.handleRef.Load() }

// normalizeDeskRole maps a session role to the desk profile whose module set
// applies: the four desks (management, engineering, research, texture), a
// research child and a management task. Any other role, the empty one
// included, is no desk and gets no module verbs (management delegation
// R2-2): it used to resolve to engineering, so a role the kernel did not
// know silently received WriteFile, Exec, Complete and Freeze.
func normalizeDeskRole(role string) string {
	if d, ok := deskmanifest.Lookup(role); ok && d.Carrier != deskmanifest.CarrierNone {
		return role
	}
	return ""
}

// BindCell binds one cell: installs the inbox snapshot and a fresh tray,
// returning hooks the session loop drives around evaluation.
func (s *ChoirScope) BindCell() CellHooks {
	return CellHooks{
		Begin: func(frame SessionFrame) {
			s.reissueHandle()
			s.tray = &Tray{}
			s.inbox = append([]IncomingMessage(nil), frame.Inbox...)
			s.doc = frame.Doc
			s.pack = frame.Pack
			s.updates = append([]PendingUpdate(nil), frame.Updates...)
			s.emits = append([]PendingEmit(nil), frame.Emits...)
			s.records = append([]InboxRecord(nil), frame.Records...)
			s.jobs = append([]JobView(nil), frame.Jobs...)
			s.recordsShown.Store(0)
			s.takeAwaited()
			s.takeRefusals()
		},
		End: func() []StagedIntent {
			var out []StagedIntent
			if s.tray != nil {
				out = s.tray.Drain()
				s.tray = nil
			}
			s.inbox = nil
			s.doc = nil
			s.pack = nil
			s.updates = nil
			s.emits = nil
			s.records = nil
			s.jobs = nil
			return out
		},
		Refusals:     s.takeRefusals,
		SearchCounts: s.searchCountLines,
		RecordsShown: s.recordsShown.Load,
		JobsAwaited:  s.takeAwaited,
	}
}

// takeRefusals returns and clears the refusals recorded since the last call.
func (s *ChoirScope) takeRefusals() []string {
	s.refusalMu.Lock()
	defer s.refusalMu.Unlock()
	out := make([]string, 0, len(s.refusals))
	for _, r := range s.refusals {
		out = append(out, "choir."+r.verb+" refused: "+r.err)
	}
	s.refusals = nil
	return out
}

// recordRefusals wraps a verb whose last result is an error so a non-nil
// error is recorded for the bound cell. The wrapped value keeps the verb's
// exact type; yaegi sees no difference.
func (s *ChoirScope) recordRefusals(name string, verb reflect.Value) reflect.Value {
	typ := verb.Type()
	errType := reflect.TypeOf((*error)(nil)).Elem()
	if typ.Kind() != reflect.Func || typ.NumOut() == 0 || typ.Out(typ.NumOut()-1) != errType {
		return verb
	}
	return reflect.MakeFunc(typ, func(args []reflect.Value) []reflect.Value {
		var results []reflect.Value
		if typ.IsVariadic() {
			results = verb.CallSlice(args)
		} else {
			results = verb.Call(args)
		}
		if last := results[len(results)-1]; !last.IsNil() {
			s.refusalMu.Lock()
			s.refusals = append(s.refusals, verbRefusal{verb: name, err: last.Interface().(error).Error()})
			s.refusalMu.Unlock()
		}
		return results
	})
}

func (s *ChoirScope) call(action BrokerAction, payload any, result any) error {
	if s == nil || s.broker == nil {
		return fmt.Errorf("choir: scope unavailable")
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("choir: marshal %s payload: %w", action, err)
	}
	resp := s.broker.HandleRequest(context.Background(), &BrokerRequest{
		ProtocolVersion: ProtocolVersion,
		RequestID:       newReceiptID(),
		HandleRef:       s.HandleRef(),
		Epoch:           s.epoch,
		Action:          action,
		Payload:         raw,
	})
	if resp == nil {
		return fmt.Errorf("choir: %s: empty broker response", action)
	}
	if !resp.Success {
		return fmt.Errorf("choir: %s: %s", action, resp.Error)
	}
	if result != nil {
		if err := json.Unmarshal(resp.Result, result); err != nil {
		}
	}
	return nil
}

// deskModuleSet names the choir verbs a desk's cells may stage (mission R2:
// the yaegi carrier generalized to per-desk module sets). Every desk shares
// the observation tier (ReadFile/ListDir/Context/Inbox); the semantic-act
// verbs are grouped so messaging authority is separable from world mutation.
// "file" = ReadFile+ListDir+WriteFile+Exec; "delegate" = Cast;
// "commit" = Complete+Freeze; "epistemic" = the full semantic-act surface.
var deskModuleSets = func() map[string][]string {
	sets := map[string][]string{}
	for _, name := range deskmanifest.Names() {
		if d, _ := deskmanifest.Lookup(name); d.Carrier != deskmanifest.CarrierNone && d.Kind != deskmanifest.KindApp {
			sets[name] = d.Verbs
		}
	}
	return sets
}()

// legacyObservation gates the observation verbs of the packet transports
// (Cut 1 L6). Every desk converses on records and reads choir.Records
// (engineering since C1e B1), so no desk reads the retired Message and Emit
// mail (Inbox, Emits). Updates reads update packets: Texture and management
// keep it for the terminal outcomes still delivered as packets.
func (s *ChoirScope) legacyObservation(name string) bool {
	if s == nil {
		return false
	}
	switch s.desk {
	case "texture", "management":
		return name == "Updates"
	default:
		return false
	}
}

func (s *ChoirScope) deskModule(name string) bool {
	if s == nil {
		return false
	}
	d, ok := deskmanifest.Lookup(s.desk)
	return ok && d.HasVerb(name)
}

// ChoirExports returns the prebound choir package for model-authored Go,
// scoped to the desk's module set. Observation (ReadFile/ListDir/Context/
// Inbox) is universal; mutation and semantic verbs gate on the desk. Read-only
// scopes still omit file/exec mutation, and verifier affordances gate on the
// slot — a module entry only admits what the scope also authorizes.
func (s *ChoirScope) ChoirExports() interp.Exports {
	exports := map[string]reflect.Value{
		"ReadFile": reflect.ValueOf(s.ReadFile),
		"ListDir":  reflect.ValueOf(s.ListDir),
		"Context":  reflect.ValueOf(s.Context),
		"Records":  reflect.ValueOf(s.Records),
		"Pack":     reflect.ValueOf(s.Pack),
		"Help":     reflect.ValueOf(s.Help),
		"CellOut":  reflect.ValueOf(s.CellOut),
	}
	if s.legacyObservation("Updates") {
		exports["Updates"] = reflect.ValueOf(s.Updates)
	}
	verbs := map[string]func() reflect.Value{
		"WriteFile":        func() reflect.Value { return reflect.ValueOf(s.WriteFile) },
		"Exec":             func() reflect.Value { return reflect.ValueOf(s.Exec) },
		"Start":            func() reflect.Value { return reflect.ValueOf(s.Start) },
		"Poll":             func() reflect.Value { return reflect.ValueOf(s.Poll) },
		"Stop":             func() reflect.Value { return reflect.ValueOf(s.Stop) },
		"Complete":         func() reflect.Value { return reflect.ValueOf(s.Complete) },
		"Freeze":           func() reflect.Value { return reflect.ValueOf(s.Freeze) },
		"Cast":             func() reflect.Value { return reflect.ValueOf(s.Cast) },
		"Ask":              func() reflect.Value { return reflect.ValueOf(s.Ask) },
		"Note":             func() reflect.Value { return reflect.ValueOf(s.Note) },
		"Reply":            func() reflect.Value { return reflect.ValueOf(s.Reply) },
		"CancelAct":        func() reflect.Value { return reflect.ValueOf(s.CancelAct) },
		"CancelAssignment": func() reflect.Value { return reflect.ValueOf(s.CancelAssignment) },
		"Escalate":         func() reflect.Value { return reflect.ValueOf(s.Escalate) },
		"Precommit":        func() reflect.Value { return reflect.ValueOf(s.Precommit) },
		"Report":           func() reflect.Value { return reflect.ValueOf(s.Report) },
		"ReportPacket":     func() reflect.Value { return reflect.ValueOf(s.ReportPacket) },
		"Resolve":          func() reflect.Value { return reflect.ValueOf(s.Resolve) },
		"Disagreement":     func() reflect.Value { return reflect.ValueOf(s.Disagreement) },
		"ReadDoc":          func() reflect.Value { return reflect.ValueOf(s.ReadDoc) },
		"Write":            func() reflect.Value { return reflect.ValueOf(s.Write) },
		"Done":             func() reflect.Value { return reflect.ValueOf(s.Done) },
		"Wait":             func() reflect.Value { return reflect.ValueOf(s.Wait) },
		"Reject":           func() reflect.Value { return reflect.ValueOf(s.Reject) },
		"SearchStart":      func() reflect.Value { return reflect.ValueOf(s.SearchStart) },
		"SearchWait":       func() reflect.Value { return reflect.ValueOf(s.SearchWait) },
		"Searches":         func() reflect.Value { return reflect.ValueOf(s.searches) }, // a value: choir.Searches.Get(id), .List()
		"Research":         func() reflect.Value { return reflect.ValueOf(s.Research) },
		"Read":             func() reflect.Value { return reflect.ValueOf(s.Read) },
		"PollSource":       func() reflect.Value { return reflect.ValueOf(s.PollSource) },
		"SourceAPI":        func() reflect.Value { return reflect.ValueOf(s.SourceAPI) },
		"ReadCapture":      func() reflect.Value { return reflect.ValueOf(s.ReadCapture) },
		"SearchWireCorpus": func() reflect.Value { return reflect.ValueOf(s.SearchWireCorpus) },
		"System1":          func() reflect.Value { return reflect.ValueOf(s.System1) },
		"SaveEvidence":     func() reflect.Value { return reflect.ValueOf(s.SaveEvidence) },
		"ReadEvidence":     func() reflect.Value { return reflect.ValueOf(s.ReadEvidence) },
		"ListEvidence":     func() reflect.Value { return reflect.ValueOf(s.ListEvidence) },
		"RunMemoryEntry":   func() reflect.Value { return reflect.ValueOf(s.RunMemoryEntry) },
		"ProductAPI":       func() reflect.Value { return reflect.ValueOf(s.ProductAPI) },
		"Stories":          func() reflect.Value { return reflect.ValueOf(s.Stories) },
		"Feed":             func() reflect.Value { return reflect.ValueOf(s.Feed) },
		"Decline":          func() reflect.Value { return reflect.ValueOf(s.Decline) },
		"Verify":           func() reflect.Value { return reflect.ValueOf(s.Verify) },
		"Delegate":         func() reflect.Value { return reflect.ValueOf(s.Delegate) },
		"CancelTask":       func() reflect.Value { return reflect.ValueOf(s.CancelTask) },
	}
	for name, mint := range verbs {
		// A verb is exported only when the desk's module set admits it AND the
		// scope permits it: mutation verbs additionally require !readOnly.
		if !s.deskModule(name) {
			continue
		}
		if s.readOnly && (name == "WriteFile" || name == "Exec" || name == "Start" || name == "Stop") {
			continue
		}
		exports[name] = mint()
	}
	if s != nil && s.slot == "verifier" {
		exports["InspectBundle"] = reflect.ValueOf(s.InspectBundle)
	}
	if s != nil {
		for name, verb := range exports {
			exports[name] = s.recordRefusals(name, verb)
		}
	}
	return interp.Exports{"choir/choir": exports}
}

// mutateDenied rejects world mutation on a read-only scope. Read-only is the
// research desk: it loses filesystem/capsule writes (WriteFile, Exec) but
// KEEPS full semantic-act authority — Emit, Message, Cast, Report and the
// rest are messaging, not world mutation, and are explicitly in research's
// module set ("read-only world access is not read-only messaging"). The
// export layer already blocks only WriteFile/Exec on readOnly; this gate
// must match it or a research cell's emitted findings are denied at call
// time while being advertised on the module surface.
func (s *ChoirScope) mutateDenied(op string) error {
	if s != nil && s.readOnly && (op == "WriteFile" || op == "Exec") {
		return fmt.Errorf("choir: %s denied for read-only role", op)
	}
	return nil
}

// ReadFile returns the full content of a jailed path.
func (s *ChoirScope) ReadFile(path string) (string, error) {
	var result ReadFileResult
	if err := s.call(ActionReadFile, ReadFilePayload{Path: path}, &result); err != nil {
		return "", err
	}
	return result.Content, nil
}

// WriteFile writes content to a jailed path, creating parents. It returns
// bytes written.
func (s *ChoirScope) WriteFile(path, content string) (int, error) {
	if err := s.mutateDenied("WriteFile"); err != nil {
		return 0, err
	}
	var result WriteFileResult
	if err := s.call(ActionWriteFile, WriteFilePayload{Path: path, Content: content}, &result); err != nil {
		return 0, err
	}
	return result.BytesWritten, nil
}

// ListDir returns entry names of a jailed directory.
func (s *ChoirScope) ListDir(path string) ([]string, error) {
	var result ListDirResult
	if err := s.call(ActionListDir, ListDirPayload{Path: path}, &result); err != nil {
		return nil, err
	}
	return result.Entries, nil
}

// Exec runs a command through the broker and returns its result record.
func (s *ChoirScope) Exec(command string, args []string) (ExecResult, error) {
	if err := s.mutateDenied("Exec"); err != nil {
		return ExecResult{}, err
	}
	var result ExecResult
	if err := s.call(ActionExec, ExecPayload{Command: command, Args: args}, &result); err != nil {
		return ExecResult{}, err
	}
	return result, nil
}

// Message and Emit are retired (protocol P2c,
// docs/definitions/choir-protocol-p2-one-surface-2026-10-11.md): one agent
// reaches another only through a record addressed to it. They stay exported
// so a model that still calls them is told what to use instead of hitting an
// undefined symbol; the reducer keeps its IntentMessage branch for one release
// so a tray staged before the change still replays.
const retiredMessageVerbs = "use choir.ReportPacket(to, packet, \"\") to send a packet to your requester or the desk that cast to you, choir.Note(desk, body) for information, or choir.Cast / choir.Ask to request work"

func (s *ChoirScope) Message(recipientID, kind, body string) (MessageResult, error) {
	return MessageResult{}, fmt.Errorf("choir.Message is retired: %s", retiredMessageVerbs)
}

func (s *ChoirScope) Emit(toDesk, kind, body string) (EmitResult, error) {
	return EmitResult{}, fmt.Errorf("choir.Emit is retired: %s", retiredMessageVerbs)
}

// JSON is the raw JSON a world verb returns. It is a byte slice, so
// string(r), json.Unmarshal(r, &v) and []byte parameters take it, and it
// prints as text: println(r), fmt.Println(r) and %v show the JSON, not a
// list of byte values, and json.Marshal embeds it as JSON
// (problems/research-parent-cannot-read-its-children-2026-10-11.md).
type JSON []byte

func (j JSON) String() string { return string(j) }

func (j JSON) MarshalJSON() ([]byte, error) {
	if len(j) == 0 {
		return []byte("null"), nil
	}
	return j, nil
}

func (j *JSON) UnmarshalJSON(data []byte) error {
	*j = append((*j)[:0], data...)
	return nil
}

// Research runs child research activations on the host, in parallel, and
// returns each child's findings to this cell. children is one objective (a
// string), a list of objectives, or a list of {"objective", "model"} maps;
// an empty model means this desk's own. The children share this cell's
// deadline.
func (s *ChoirScope) Research(children any) ([]ResearchChildResult, error) {
	specs, err := researchChildSpecs(children)
	if err != nil {
		return nil, err
	}
	var out []ResearchChildResult
	err = s.call(ActionResearch, ResearchPayload{Children: specs}, &out)
	return out, err
}

func researchChildSpecs(children any) ([]ResearchChild, error) {
	raw, err := jsonCellArg(children)
	if err != nil {
		return nil, err
	}
	raw = strings.TrimSpace(raw)
	var specs []ResearchChild
	switch {
	case raw == "":
	case strings.HasPrefix(raw, "["):
		var objectives []string
		if json.Unmarshal([]byte(raw), &objectives) == nil {
			for _, o := range objectives {
				specs = append(specs, ResearchChild{Objective: o})
			}
		} else if err := json.Unmarshal([]byte(raw), &specs); err != nil {
			return nil, fmt.Errorf("choir: Research takes an objective, a list of objectives, or a list of {objective, model} maps: %w", err)
		}
	case strings.HasPrefix(raw, "{"):
		var one ResearchChild
		if err := json.Unmarshal([]byte(raw), &one); err != nil {
			return nil, fmt.Errorf("choir: Research takes an objective, a list of objectives, or a list of {objective, model} maps: %w", err)
		}
		specs = append(specs, one)
	default:
		specs = append(specs, ResearchChild{Objective: raw})
	}
	kept := specs[:0]
	for _, spec := range specs {
		spec.Objective = strings.TrimSpace(spec.Objective)
		spec.Model = strings.TrimSpace(spec.Model)
		if spec.Objective != "" {
			kept = append(kept, spec)
		}
	}
	if len(kept) == 0 {
		return nil, fmt.Errorf("choir: Research needs at least one objective")
	}
	return kept, nil
}

// Read fetches a web page or PDF through the host's address-guarded client
// into a shared capture (a block document with its raw bytes) and returns
// the capture id, URL, title, block count and the first blocks with their
// ids; a long page also returns next_block and a digest of its headings.
// An unchanged page re-read returns its existing capture.
func (s *ChoirScope) Read(url string) (JSON, error) {
	var out JSON
	err := s.call(ActionRead, ReadPayload{URL: url}, &out)
	return out, err
}

// PollSource polls one registry source through the host and writes its
// items as shared choir.web_capture objects in the global store behind
// corpusd; returns counts and the new captures as JSON.
func (s *ChoirScope) PollSource(sourceID string, maxItems int) (JSON, error) {
	var out JSON
	err := s.call(ActionPollSource, PollSourcePayload{SourceID: sourceID, MaxItems: maxItems}, &out)
	return out, err
}

// SourceAPI calls one named endpoint of one keyed source API through the
// host gateway, which adds the key; returns the provider's answer as JSON.
func (s *ChoirScope) SourceAPI(provider, endpoint string, params map[string]any) (JSON, error) {
	var out JSON
	err := s.call(ActionSourceAPI, SourceAPIPayload{Provider: provider, Endpoint: endpoint, Params: params}, &out)
	return out, err
}

// ReadCapture reads one shared capture (from any computer's poll or Read):
// its blocks with their ids from block fromBlock, as many as fit in
// maxChars characters (0 means 8000), and next_block when more remain.
//
// On Texture, captureID may also be a frame source handle (r1.c1): it reads
// the capture that source names.
func (s *ChoirScope) ReadCapture(captureID string, fromBlock, maxChars int) (JSON, error) {
	if s.desk == "texture" {
		id, err := s.frameCaptureID(captureID)
		if err != nil {
			return nil, err
		}
		captureID = id
	}
	var out JSON
	err := s.call(ActionReadCapture, ReadCapturePayload{CaptureID: captureID, FromBlock: fromBlock, MaxChars: maxChars}, &out)
	return out, err
}

// frameCaptureID resolves a Texture frame source handle to the capture it
// names. Anything that is not a handle passes through as a capture id.
func (s *ChoirScope) frameCaptureID(ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if !frameHandleRE.MatchString(ref) {
		return ref, nil
	}
	if s.doc != nil {
		for _, src := range s.doc.Sources {
			if src.Handle != ref {
				continue
			}
			if id := strings.TrimSpace(src.CaptureID); id != "" {
				return id, nil
			}
			return "", fmt.Errorf("choir: %s has no capture (%s); cite its URL, or Cast research to capture it", ref, src.URL)
		}
	}
	return "", fmt.Errorf("choir: %s is not a source in your frame; choir.ReadDoc().Sources lists them", ref)
}

var frameHandleRE = regexp.MustCompile(`^r[0-9]+\.c[0-9]+$`)

// SearchWireCorpus searches the owner's published wire corpus.
func (s *ChoirScope) SearchWireCorpus(query string, limit int) (JSON, error) {
	var out JSON
	err := s.call(ActionSearchWireCorpus, SearchWireCorpusPayload{Query: query, Limit: limit}, &out)
	return out, err
}

// SaveEvidence persists evidentiary material into the owner's Dolt workspace.
func (s *ChoirScope) SaveEvidence(kind, sourceURI, title, content string, metadata []byte) (JSON, error) {
	var out JSON
	err := s.call(ActionSaveEvidence, SaveEvidencePayload{Kind: kind, SourceURI: sourceURI, Title: title, Content: content, Metadata: json.RawMessage(metadata)}, &out)
	return out, err
}

// ReadEvidence reads one saved evidence record by id.
func (s *ChoirScope) ReadEvidence(evidenceID string) (JSON, error) {
	var out JSON
	err := s.call(ActionReadEvidence, ReadEvidencePayload{EvidenceID: evidenceID}, &out)
	return out, err
}

// ListEvidence lists recent saved evidence for an agent or the owner scope.
func (s *ChoirScope) ListEvidence(agentID string, limit int) (JSON, error) {
	var out JSON
	err := s.call(ActionListEvidence, ListEvidencePayload{AgentID: agentID, Limit: limit}, &out)
	return out, err
}

// RunMemoryEntry retrieves one durable run-memory entry by id.
func (s *ChoirScope) RunMemoryEntry(entryID string) (JSON, error) {
	var out JSON
	err := s.call(ActionGetRunMemoryEntry, GetRunMemoryEntryPayload{EntryID: entryID}, &out)
	return out, err
}

// ProductAPI calls an allowlisted authenticated product API route as the run
// owner — the management desk's in-cell replacement for the deleted
// product_api_request tool (SMG). Method is GET/POST/PUT/DELETE; path is an
// absolute /api/... route on the product-path allowlist; body is an optional
// JSON object. The host validates and serves the route, returning a bounded
// {status_code, content_type, body} JSON result.
func (s *ChoirScope) ProductAPI(method, path string, body []byte) (JSON, error) {
	var out JSON
	err := s.call(ActionProductAPI, ProductAPIPayload{Method: method, Path: path, Body: json.RawMessage(body)}, &out)
	return out, err
}

// Stories pages the computer's story library, newest first: limit stories
// (0 means 50, at most 200) updated before before (RFC 3339; "" for the
// newest). Pass the page's Next as before for the next page (L2a A2).
func (s *ChoirScope) Stories(limit int, before string) (StoriesPage, error) {
	var out StoriesPage
	if err := s.call(ActionStories, StoriesPayload{Limit: limit, Before: strings.TrimSpace(before)}, &out); err != nil {
		return StoriesPage{}, err
	}
	return out, nil
}

// Feed feeds captures into an existing story (L2a R2-5): the same Note to
// texture:<doc> a hand-written feed is, plus the structured list of
// captures. doc is the story's doc id or its inbox; reason says what the
// captures add, and why, when you feed elsewhere than Jev's label said.
func (s *ChoirScope) Feed(doc string, captureIDs any, reason string) (string, error) {
	t, err := s.boundTray("feed")
	if err != nil {
		return "", err
	}
	doc = strings.TrimPrefix(strings.TrimSpace(doc), "texture:")
	if doc == "" || doc == "new" {
		return "", fmt.Errorf("choir: Feed needs an existing story's doc id or inbox (texture:<doc>); open a new story with choir.Cast(\"texture:new\", headline, spec)")
	}
	ids, err := captureIDsArg(captureIDs)
	if err != nil {
		return "", fmt.Errorf("choir: Feed: %w", err)
	}
	return t.Dispose("texture:"+doc, DispositionFeed, ids, reason)
}

// Decline declines captures that open no story and feed none (L2a R2-5):
// the same Note to their source a hand-written decline is, one per source,
// plus the structured list of captures. Each id must be named by a
// captures record of this turn.
func (s *ChoirScope) Decline(captureIDs any, reason string) (string, error) {
	t, err := s.boundTray("decline")
	if err != nil {
		return "", err
	}
	ids, err := captureIDsArg(captureIDs)
	if err != nil {
		return "", fmt.Errorf("choir: Decline: %w", err)
	}
	sourceOf := map[string]string{}
	for _, r := range s.records {
		if r.Captures == nil {
			continue
		}
		for _, item := range r.Captures.Items {
			// An event's record names captures of many sources, each
			// with its own (event layer §15).
			src := strings.TrimSpace(item.SourceID)
			if src == "" {
				src = r.Captures.SourceID
			}
			sourceOf[item.CaptureID] = src
		}
	}
	bySource := map[string][]string{}
	var sources []string
	for _, id := range ids {
		src, ok := sourceOf[id]
		if !ok {
			return "", fmt.Errorf("choir: Decline: capture %s is not named by a captures record of this turn (r.Captures.Items)", id)
		}
		if _, seen := bySource[src]; !seen {
			sources = append(sources, src)
		}
		bySource[src] = append(bySource[src], id)
	}
	var refs []string
	for _, src := range sources {
		ref, err := t.Dispose(types.CaptureArrivalSenderInbox(src), DispositionDecline, bySource[src], reason)
		if err != nil {
			return strings.Join(refs, ","), err
		}
		refs = append(refs, ref)
	}
	return strings.Join(refs, ","), nil
}

// Delegate hands capture intake to management tasks (management delegation
// S2a, R2-14) and returns at once: nothing waits on a task. refs are the
// record ids of captures records from choir.Records(); grouping is
// "per_record" (or "" or nil: one task per record) or a list of groups,
// each a list of those record ids given to one task, at most
// MaxTaskCaptures captures per task; a ref no group names gets its own
// task. Each task's outcome comes back later as a record from
// management-task:<id>.
func (s *ChoirScope) Delegate(kind, objective string, refs any, grouping any) (string, error) {
	t, err := s.boundTray("delegate")
	if err != nil {
		return "", err
	}
	switch kind = strings.TrimSpace(kind); kind {
	case TaskKindIntake:
	case TaskKindAnswer:
		return "", fmt.Errorf("choir: Delegate: answer tasks are not open yet; answer the Texture yourself")
	default:
		return "", fmt.Errorf("choir: Delegate: kind is %q (one captures record or more per task)", TaskKindIntake)
	}
	ids, err := captureIDsArg(refs)
	if err != nil {
		return "", fmt.Errorf("choir: Delegate: refs are record ids from choir.Records(): %w", err)
	}
	// A captures record delegates its captures; a task's final Report
	// delegates its residuals again.
	captures := map[string]int{}
	for _, r := range s.records {
		if r.Captures != nil && len(r.Captures.Items) > 0 {
			captures[r.RecordID] = len(r.Captures.Items)
		}
		if r.TaskOutcome != nil && len(r.TaskOutcome.Residuals) > 0 {
			captures[r.RecordID] = len(r.TaskOutcome.Residuals)
		}
	}
	for _, id := range ids {
		if _, ok := captures[id]; !ok {
			return "", fmt.Errorf("choir: Delegate: %s is not a captures record or a task Report with residuals of this turn (pass r.RecordID of a record whose r.Captures or r.TaskOutcome.Residuals is set)", id)
		}
	}
	var groups [][]string
	grouped := map[string]bool{}
	switch g := grouping.(type) {
	case nil:
	case string:
		if g = strings.TrimSpace(g); g != "" && g != "per_record" {
			return "", fmt.Errorf("choir: Delegate: grouping is \"per_record\" or a list of groups of record ids, not %q", g)
		}
	default:
		raw, err := json.Marshal(g)
		if err != nil {
			return "", fmt.Errorf("choir: Delegate: grouping: %w", err)
		}
		if err := json.Unmarshal(raw, &groups); err != nil {
			return "", fmt.Errorf("choir: Delegate: grouping is \"per_record\" or a list of groups of record ids ([][]string)")
		}
	}
	inRefs := map[string]bool{}
	for _, id := range ids {
		inRefs[id] = true
	}
	var out [][]string
	for i, group := range groups {
		var kept []string
		n := 0
		for _, id := range group {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			if !inRefs[id] {
				return "", fmt.Errorf("choir: Delegate: group %d names %s, which is not in refs", i+1, id)
			}
			if grouped[id] {
				return "", fmt.Errorf("choir: Delegate: %s is in two groups; each record goes to one task", id)
			}
			grouped[id] = true
			kept = append(kept, id)
			n += captures[id]
		}
		if n > MaxTaskCaptures {
			return "", fmt.Errorf("choir: Delegate: group %d holds %d captures; a task takes at most %d", i+1, n, MaxTaskCaptures)
		}
		if len(kept) > 0 {
			out = append(out, kept)
		}
	}
	// Records of one world event go to one task, so two tasks never open
	// two stories of one event (docs/problems/event-records-of-one-event-
	// delegated-apart-2026-10-16.md): a record no group names joins the
	// task holding its event's other records while they fit in one task.
	eventOf := map[string]string{}
	for _, r := range s.records {
		if r.Captures != nil && r.Captures.Event != nil && strings.TrimSpace(r.Captures.Event.EventID) != "" {
			eventOf[r.RecordID] = r.Captures.Event.EventID
		}
	}
	taskOf, size := map[string]int{}, make([]int, len(out))
	for i, group := range out {
		for _, id := range group {
			size[i] += captures[id]
			if ev := eventOf[id]; ev != "" {
				taskOf[ev] = i
			}
		}
	}
	for _, id := range ids {
		if grouped[id] {
			continue
		}
		grouped[id] = true
		if ev := eventOf[id]; ev != "" {
			if i, ok := taskOf[ev]; ok && size[i]+captures[id] <= MaxTaskCaptures {
				out[i] = append(out[i], id)
				size[i] += captures[id]
				continue
			}
			taskOf[ev] = len(out)
		}
		out = append(out, []string{id})
		size = append(size, captures[id])
	}
	if strings.TrimSpace(objective) == "" {
		objective = "Open, feed or decline every capture these records name."
	}
	return t.Delegate(DelegateSpec{Kind: kind, Objective: strings.TrimSpace(objective), Groups: out})
}

// CancelTask cancels a management task (R2-20): a running task's run is
// cancelled, a queued task's round is closed. Either way its final Report
// lists the captures it left, which you may delegate again. taskID is the
// id or its inbox (management-task:<id>).
func (s *ChoirScope) CancelTask(taskID, reason string) (string, error) {
	t, err := s.boundTray("cancel_task")
	if err != nil {
		return "", err
	}
	return t.CancelTask(strings.TrimPrefix(strings.TrimSpace(taskID), "management-task:"), reason)
}

// captureIDsArg reads a list of capture ids: a []string, a list of
// strings, or one id.
func captureIDsArg(v any) ([]string, error) {
	var ids []string
	switch t := v.(type) {
	case string:
		ids = []string{t}
	case []string:
		ids = t
	default:
		raw, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("capture ids: %w", err)
		}
		if err := json.Unmarshal(raw, &ids); err != nil {
			return nil, fmt.Errorf("capture ids must be a list of strings (for example []string{c.CaptureID})")
		}
	}
	var out []string
	seen := map[string]bool{}
	for _, id := range ids {
		if id = strings.TrimSpace(id); id != "" && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no capture ids")
	}
	return out, nil
}

// Complete marks the assignment finished with a typed verdict. It stages
// into the cell tray; the reducer authors the assignment fate from it. At
// most one complete per cell. It requires a bound cell. executionRefs are
// the exact capsule-go-eval receipt_ref strings the evidence binds to; a
// terminal completed pass requires at least one.
func (s *ChoirScope) Complete(result, verdict, summary string, evidenceRefs, executionRefs []string) error {
	if err := s.mutateDenied("Complete"); err != nil {
		return err
	}
	if s.tray == nil {
		return fmt.Errorf("choir: complete requires a bound cell")
	}
	return s.tray.Complete(result, verdict, summary, evidenceRefs, executionRefs)
}

// Freeze stages the self-development freeze: the reducer freezes the cell's
// bound capsule diff as a verifier-ready effect bundle on cell return. The
// capsule handle is the bound handle, never model input. At most one freeze
// per cell; requires a bound cell.
func (s *ChoirScope) Freeze(buildRecipeRef string, testReceipts, dependencyToolchainRefs []string) error {
	if err := s.mutateDenied("Freeze"); err != nil {
		return err
	}
	if s.tray == nil {
		return fmt.Errorf("choir: freeze requires a bound cell")
	}
	return s.tray.Freeze(buildRecipeRef, testReceipts, dependencyToolchainRefs)
}

// Verify stages management's verdict on the candidate an engineering
// assignment froze (C1e step 4). assignmentID names the assignment (an
// engineering Report's evidence_refs carry "assignment:<id>" and
// "bundle:<digest>"); the reducer resolves the assignment's own operation
// and asserts bundleDigest equals its durable digest. At most one verify per
// cell; requires a bound cell.
func (s *ChoirScope) Verify(assignmentID, decision string, verifierRefs []string, bundleDigest string) error {
	if err := s.mutateDenied("Verify"); err != nil {
		return err
	}
	if !s.deskModule("Verify") {
		return fmt.Errorf("choir: verify is management's")
	}
	if s.tray == nil {
		return fmt.Errorf("choir: verify requires a bound cell")
	}
	return s.tray.Verify(assignmentID, decision, verifierRefs, bundleDigest)
}

// InspectBundle synchronously verifies the mounted frozen self-development
// bundle: parses the draft, re-checks every runtime file digest, and returns
// the canonical receipt fields. Read-only under the observation exemption;
// verifier-slot activations only. The mount itself is the binding: the host
// installs exactly the operation's frozen bundle read-only at spawn.
func (s *ChoirScope) InspectBundle() (map[string]any, error) {
	if s == nil {
		return nil, fmt.Errorf("choir: scope unavailable")
	}
	if s.slot != "verifier" {
		return nil, fmt.Errorf("choir: inspect_bundle is restricted to the co-super verifier slot")
	}
	return inspectMountedBundle()
}

// Inbox returns the cell-start snapshot of unread messages. It is
// side-effect-free inside the cell: no network call, no cursor movement.
// The durable cursor advances only when the cell's intents reduce
// successfully. Unbound scopes observe an empty inbox.
func (s *ChoirScope) Inbox() []IncomingMessage {
	if s == nil {
		return []IncomingMessage{}
	}
	return append([]IncomingMessage(nil), s.inbox...)
}

// Updates returns the cell-start snapshot of pending update_coagent records.
// The RLM contract puts the payload here — a REPL variable — rather than
// inline in the model's context window: the chat wake turn carries only the
// update ids. Side-effect-free inside the cell; unbound scopes see an empty
// slice. A desk disposes each update through its terminal write.
func (s *ChoirScope) Updates() []PendingUpdate {
	if s == nil {
		return []PendingUpdate{}
	}
	out := append([]PendingUpdate(nil), s.updates...)
	for i := range out {
		out[i].ID = out[i].UpdateID
	}
	return out
}

// Records returns the cell-start snapshot of records addressed to this desk
// in (cursor, head]: Casts, Asks, Notes and Reports, each with its sequence
// number, sender inbox, conversation and round. Side-effect-free inside the
// cell; the cursor moves only when the cell's acts commit, and only through
// the records a Records() call returned.
func (s *ChoirScope) Records() []InboxRecord {
	if s == nil {
		return []InboxRecord{}
	}
	out := append([]InboxRecord(nil), s.records...)
	for i := range out {
		out[i].ID = out[i].RecordID
	}
	for _, r := range out {
		for {
			cur := s.recordsShown.Load()
			if r.Seq <= cur || s.recordsShown.CompareAndSwap(cur, r.Seq) {
				break
			}
		}
	}
	return out
}

// Emits returns the cell-start snapshot of emitted signals addressed to this
// desk. The boundary-drain notice turn carries only the fixed-format
// notice line (sender/kind/seq/snippet); the full untrusted emission bodies
// live here as data. A desk reads them here and disposes each through its
// terminal write — never trust the notice to carry instructions.
func (s *ChoirScope) Emits() []PendingEmit {
	if s == nil {
		return []PendingEmit{}
	}
	return append([]PendingEmit(nil), s.emits...)
}

// jsonCellArg normalizes a cell-authored JSON argument: a JSON-encoded string
// is used verbatim; any other JSON-marshalable Go value (map/slice literals,
// typed structs) is marshaled. Cells are Go programs — the natural literal
// form is a Go value, not a serialized string, so verbs accept both and the
// wire shape stays canonical JSON.
func jsonCellArg(v any) (string, error) {
	switch t := v.(type) {
	case nil:
		return "", nil
	case string:
		return t, nil
	case json.RawMessage:
		return string(t), nil
	case JSON:
		return string(t), nil
	case []byte:
		return string(t), nil
	default:
		raw, err := json.Marshal(v)
		if err != nil {
			return "", fmt.Errorf("choir: marshal cell argument as JSON: %w", err)
		}
		return string(raw), nil
	}
}

// Context reports the activation identity the scope is bound to.
func (s *ChoirScope) Context() map[string]string {
	if s == nil {
		return map[string]string{}
	}
	return map[string]string{
		"computer_id":   s.computerID,
		"activation_id": s.activationID,
		"co_super_slot": s.slot,
	}
}

// ReadDoc returns the bound Texture document as the cell's frame showed it:
// its revision, text and every top-level block by alias with its full text
// (a long document's frame digests blocks; ReadDoc does not).
// Side-effect-free inside the cell; the snapshot is injected at cell launch.
// Empty for non-texture scopes.
func (s *ChoirScope) ReadDoc() DocSnapshot {
	if s == nil || s.doc == nil {
		return DocSnapshot{}
	}
	return *s.doc
}

// Write stages the cell's revision of the bound document: edits is a script
// of directives on the frame's block aliases (see texturewrite), why says
// what the revision does. The grammar, aliases and frame handles are checked
// now and a refusal names its script line; capture handles and quotes are
// checked when the turn commits after the cell, with the same line numbers.
// One Write per cell. An optional third argument, pSupported, is the
// Write's self-score: the probability that the judge finds each new
// citation's passage supports its sentence. A refused Write is remembered
// on the cell's tray, so a Done after it in the same cell is refused too
// unless an earlier Write in it staged, rather than completing the work at
// the unrevised document.
func (s *ChoirScope) Write(edits, why string, pSupported ...float64) (string, error) {
	t, err := s.boundTray("write")
	if err != nil {
		return "", err
	}
	id, err := s.stageWrite(t, edits, why, pSupported)
	if err != nil {
		t.writeRefused = err.Error()
	}
	return id, err
}

func (s *ChoirScope) stageWrite(t *Tray, edits, why string, pSupported []float64) (string, error) {
	var selfP *float64
	switch {
	case len(pSupported) > 1:
		return "", fmt.Errorf("choir: Write refused, nothing staged: pSupported is one number for the whole Write (the probability that every new citation's passage supports its sentence), not one per citation; got %d", len(pSupported))
	case len(pSupported) == 1:
		if p := pSupported[0]; !(p >= 0 && p <= 1) {
			return "", fmt.Errorf("choir: Write refused, nothing staged: pSupported is a probability from 0 to 1, not %v", p)
		}
		p := pSupported[0]
		selfP = &p
	}
	if s.doc == nil || strings.TrimSpace(s.doc.RevisionID) == "" {
		return "", fmt.Errorf("choir: Write has no bound document in this cell")
	}
	if strings.TrimSpace(why) == "" {
		return "", fmt.Errorf("choir: Write needs why: one line on what this revision changes and why")
	}
	handles := make(map[string]bool, len(s.doc.Sources))
	for _, src := range s.doc.Sources {
		handles[src.Handle] = true
	}
	if err := texturewrite.Check(edits, len(s.doc.Blocks), handles); err != nil {
		return "", fmt.Errorf("choir: Write refused, nothing staged: %w", err)
	}
	return t.TextureTurn(s.doc.RevisionID, func(turn *TextureTurn) error {
		switch {
		case turn.Write != nil:
			return fmt.Errorf("choir: one Write per cell; put every directive in one script")
		case turn.Wait != "":
			return fmt.Errorf("choir: this cell already called Wait, which records a turn without a revision; a cell either Writes or Waits")
		}
		turn.Write = &TextureWrite{Edits: edits, Why: strings.TrimSpace(why), PSupported: selfP}
		return nil
	})
}

// Done says the requested work is complete. With a Write in the same cell
// the revision completes it; alone it completes it at the current revision.
// On a management task it ends the task: the cell's acts commit, then the
// host sends management the task's final Report, listing what each leased
// capture became and, with why as their reason, the captures left
// undisposed.
func (s *ChoirScope) Done(why string) (string, error) {
	if s != nil && s.desk == SessionRoleManagementTask {
		t, err := s.boundTray("done")
		if err != nil {
			return "", err
		}
		return t.TaskDone(why)
	}
	return s.closeTurn("done", why, func(turn *TextureTurn, why string) error {
		switch {
		case turn.Write == nil && s.tray.writeRefused != "":
			return fmt.Errorf("choir: Done refused, nothing staged: this cell's Write was refused (%s); fix the script and call Write again, then Done", s.tray.writeRefused)
		case turn.Done != "":
			return fmt.Errorf("choir: Done once per cell")
		case turn.Wait != "":
			return fmt.Errorf("choir: this cell already called Wait; the work is either done or waiting")
		}
		turn.Done = why
		return nil
	})
}

// Wait records that the document is waiting for evidence: a turn without a
// revision. A Cast, Ask or Note in the same cell commits with it.
func (s *ChoirScope) Wait(why string) (string, error) {
	return s.closeTurn("wait", why, func(turn *TextureTurn, why string) error {
		switch {
		case turn.Wait != "":
			return fmt.Errorf("choir: Wait once per cell")
		case turn.Write != nil:
			return fmt.Errorf("choir: this cell already called Write; Wait is a turn without a revision")
		case turn.Done != "":
			return fmt.Errorf("choir: this cell already called Done; the work is either done or waiting")
		}
		turn.Wait = why
		return nil
	})
}

// Reject declines a record from the frame (its alias such as "r2", or its
// record id) or a leftover update packet from choir.Updates() (its update
// id), with the reason. The reason is committed with the cell's turn.
func (s *ChoirScope) Reject(record, why string) (string, error) {
	record = strings.TrimSpace(record)
	reject := TextureReject{}
	if strings.HasPrefix(record, "r") {
		if n, err := strconv.Atoi(record[1:]); err == nil && n >= 1 && n <= len(s.records) {
			reject.RecordID = s.records[n-1].RecordID
		}
	}
	for _, r := range s.records {
		if reject.RecordID == "" && r.RecordID == record {
			reject.RecordID = r.RecordID
		}
	}
	for _, u := range s.updates {
		if reject.RecordID == "" && reject.UpdateID == "" && u.UpdateID == record {
			reject.UpdateID = u.UpdateID
		}
	}
	if reject.RecordID == "" && reject.UpdateID == "" {
		return "", fmt.Errorf("choir: Reject: %q is not a record in your frame (r1..r%d or a record id) or an update id from choir.Updates()", record, len(s.records))
	}
	return s.closeTurn("reject", why, func(turn *TextureTurn, why string) error {
		for _, prior := range turn.Rejects {
			if prior.RecordID == reject.RecordID && prior.UpdateID == reject.UpdateID {
				return fmt.Errorf("choir: %s is already rejected in this cell", record)
			}
		}
		reject.Why = why
		turn.Rejects = append(turn.Rejects, reject)
		return nil
	})
}

func (s *ChoirScope) closeTurn(verb, why string, update func(*TextureTurn, string) error) (string, error) {
	t, err := s.boundTray(verb)
	if err != nil {
		return "", err
	}
	why = strings.TrimSpace(why)
	if why == "" {
		return "", fmt.Errorf("choir: %s needs why", verb)
	}
	base := ""
	if s.doc != nil {
		base = s.doc.RevisionID
	}
	if base == "" {
		return "", fmt.Errorf("choir: %s has no bound document in this cell", verb)
	}
	return t.TextureTurn(base, func(turn *TextureTurn) error { return update(turn, why) })
}

// Pack returns the acting desk's cell-start commitment pack (R4): its own
// committed/addressed acts joined with resolution observations and
// discrepancies — score-free by construction (types.ActingPack carries no
// score fields, so the epistemic boundary holds on the wire, not by
// instruction). Side-effect-free inside the cell; the snapshot is
// injected by autoputer at cell launch. Unbound scopes observe an empty
// pack.
func (s *ChoirScope) Pack() types.ActingPack {
	if s == nil || s.pack == nil {
		return types.ActingPack{Items: []types.ActingPackItem{}}
	}
	return *s.pack
}

// --- Semantic-act verbs (mission R2). Each delegates to the bound tray;
// all require a bound cell and a non-read-only scope. The reducer authors
// the act and its commitment-ledger record on cell return.

func (s *ChoirScope) boundTray(op string) (*Tray, error) {
	if err := s.mutateDenied(op); err != nil {
		return nil, err
	}
	if s.tray == nil {
		return nil, fmt.Errorf("choir: %s requires a bound cell", op)
	}
	return s.tray, nil
}

// Cast stages delegated admission of a downstream assignment. spec is an
// optional structured payload — a JSON-encoded string or any
// JSON-marshalable Go value (""/nil stage no spec).
func (s *ChoirScope) Cast(desk, objective string, spec any) (string, error) {
	t, err := s.boundTray("cast")
	if err != nil {
		return "", err
	}
	specJSON, err := jsonCellArg(spec)
	if err != nil {
		return "", err
	}
	return t.Cast(desk, objective, specJSON)
}

// Ask stages a directed query that resolves on the target's Reply.
func (s *ChoirScope) Ask(toDesk, question string) (string, error) {
	t, err := s.boundTray("ask")
	if err != nil {
		return "", err
	}
	return t.Ask(toDesk, question)
}

// Note stages raw unscored transport.
func (s *ChoirScope) Note(toDesk, body string) (string, error) {
	t, err := s.boundTray("note")
	if err != nil {
		return "", err
	}
	return t.Note(toDesk, body)
}

// Reply answers a staged Ask.
func (s *ChoirScope) Reply(toDesk, targetRef, answer string) (string, error) {
	t, err := s.boundTray("reply")
	if err != nil {
		return "", err
	}
	return t.Reply(toDesk, targetRef, answer)
}

// CancelAct retracts a commitment by ref.
func (s *ChoirScope) CancelAct(targetRef string) (string, error) {
	t, err := s.boundTray("cancel")
	if err != nil {
		return "", err
	}
	return t.Cancel(targetRef)
}

// CancelAssignment revokes an engineering assignment capsule with executor
// acknowledgement (SMG — the verb carrier for the deleted
// cancel_co_super_assignment typed tool). Management desk only; the staged
// intent reduces through rt.cancelAssignedEngineering under the exact
// persistent-management gate.
func (s *ChoirScope) CancelAssignment(assignmentID, reason string) (string, error) {
	t, err := s.boundTray("cancel_assignment")
	if err != nil {
		return "", err
	}
	return t.CancelAssignment(assignmentID, reason)
}

// Escalate surfaces an issue to management or the owner.
func (s *ChoirScope) Escalate(toDesk, issue string) (string, error) {
	t, err := s.boundTray("escalate")
	if err != nil {
		return "", err
	}
	return t.Escalate(toDesk, issue)
}

// Precommit freezes a typed, machine-scoreable prediction on the ledger.
func (s *ChoirScope) Precommit(precommit any) (string, error) {
	t, err := s.boundTray("precommit")
	if err != nil {
		return "", err
	}
	precommitJSON, err := jsonCellArg(precommit)
	if err != nil {
		return "", err
	}
	return t.Precommit(precommitJSON)
}

// Report asserts a typed claim with evidence refs and a named resolver.
func (s *ChoirScope) Report(toDesk, claim string, evidenceRefs []string, resolverID string) (string, error) {
	t, err := s.boundTray("report")
	if err != nil {
		return "", err
	}
	return t.Report(toDesk, claim, evidenceRefs, resolverID)
}

// ReportPacket asserts a report whose body is the full coagent source-packet
// schema — the update_coagent packet contract surviving as Report's body
// (mission R2). packet is a JSON-encoded types.SourcePacket or
// its equivalent Go value (the map literal a cell writes directly); the
// reducer validates kind/claims/sources/actions/questions before commit.
func (s *ChoirScope) ReportPacket(toDesk string, packet any, resolverID string) (string, error) {
	t, err := s.boundTray("report")
	if err != nil {
		return "", err
	}
	packetJSON, err := jsonCellArg(packet)
	if err != nil {
		return "", err
	}
	if packetJSON, err = checkReportPacket(packetJSON); err != nil {
		return "", err
	}
	return t.ReportPacket(toDesk, packetJSON, resolverID)
}

// Resolve closes a commitment with a typed evidence-bearing resolver verdict.
func (s *ChoirScope) Resolve(targetRef string, resolve any) (string, error) {
	t, err := s.boundTray("resolve")
	if err != nil {
		return "", err
	}
	resolveJSON, err := jsonCellArg(resolve)
	if err != nil {
		return "", err
	}
	return t.Resolve(targetRef, resolveJSON)
}

// Disagreement preserves scorer versus resolver verdicts as a separate act.
func (s *ChoirScope) Disagreement(disagreement any) (string, error) {
	t, err := s.boundTray("disagreement")
	if err != nil {
		return "", err
	}
	disagreementJSON, err := jsonCellArg(disagreement)
	if err != nil {
		return "", err
	}
	return t.Disagreement(disagreementJSON)
}
