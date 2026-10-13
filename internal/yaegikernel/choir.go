package yaegikernel

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/traefik/yaegi/interp"
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
}

type verbRefusal struct {
	verb string
	err  string
}

// SessionRoleResearch is the read-only role: sessions bound to it observe
// files and directories but cannot write, execute, assign, or message. The
// role arrives on a trusted worker flag from the verified outer capability,
// never from model input. Any other role string means full engineering scope.
const SessionRoleResearch = "research"

// SessionRoleResearchChild is a child research activation run for a
// research cell's choir.Research: read-only world verbs and further
// choir.Research, no messaging or reporting. Its result returns to the
// parent cell as a value.
const SessionRoleResearchChild = "research_child"

// NewChoirScope mints a session-scoped handle for exactly the file, assign,
// and message actions and returns the scope the choir symbols close over.
// The caller (worker session setup) owns the broker lifetime.
func NewChoirScope(broker *Broker, issuer *HandleIssuer, computerID, activationID string, epoch uint64, role, slot string) (*ChoirScope, error) {
	if broker == nil {
		return nil, fmt.Errorf("choir: broker is required")
	}
	if issuer == nil {
		return nil, fmt.Errorf("choir: handle issuer is required")
	}
	readOnly := role == SessionRoleResearch || role == SessionRoleResearchChild
	actions := []BrokerAction{ActionExec, ActionReadFile, ActionWriteFile, ActionListDir, ActionAssign, ActionMessage, ActionEmit}
	if role == SessionRoleResearchChild {
		actions = []BrokerAction{ActionReadFile, ActionListDir,
			ActionSearchStart, ActionRead, ActionReadCapture, ActionSearchWireCorpus,
			ActionReadEvidence, ActionListEvidence}
	} else if readOnly {
		// Research is read-only on the filesystem but owns the host-mediated
		// egress verbs (search_start/read/read_capture/search_wire_corpus/
		// poll_source + evidence/run-memory) — the handle
		// must authorize the frame actions the cell verbs dispatch.
		actions = []BrokerAction{ActionReadFile, ActionListDir, ActionEmit,
			ActionSearchStart, ActionRead, ActionSearchWireCorpus,
			ActionSaveEvidence, ActionReadEvidence, ActionListEvidence, ActionGetRunMemoryEntry, ActionResearch,
			ActionPollSource, ActionReadCapture}
	}
	// SMG: the management desk's product API call rides the broker like the
	// research egress verbs — the typed product_api_request tool is deleted.
	if normalizeDeskRole(role) == "management" {
		actions = append(actions, ActionProductAPI, ActionJudge)
	}
	handleRef, err := issuer.Issue(computerID, "choir-session", epoch, actions, sessionHandleTTL)
	if err != nil {
		return nil, fmt.Errorf("choir: issue session handle: %w", err)
	}
	scope := &ChoirScope{broker: broker, issuer: issuer, actions: actions, computerID: computerID, epoch: epoch, activationID: activationID, readOnly: readOnly, desk: normalizeDeskRole(role), slot: slot, searches: newSearchStore()}
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
// applies. The four desks are management, engineering, research, texture;
// co-super and unknown roles resolve to the engineering surface so the
// generalized carrier never under-provisions a working desk.
func normalizeDeskRole(role string) string {
	switch role {
	case "management", "engineering", "research", "texture", SessionRoleResearchChild:
		return role
	}
	return "engineering"
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
			return out
		},
		Refusals:     s.takeRefusals,
		SearchCounts: s.searchCountLines,
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
// "file" = ReadFile+ListDir+WriteFile+Exec; "delegate" = Assign+Cast;
// "commit" = Complete+Freeze; "epistemic" = the full semantic-act surface.
var deskModuleSets = map[string][]string{
	// Management delegates engineering work and reports; it does not touch
	// the filesystem (mutation is capsule-bound under engineering). It alone
	// judges citation support (Judge, decisions §12.7).
	"management": {"Cast", "Ask", "Note", "Reply",
		"CancelAct", "CancelAssignment", "Escalate", "Precommit", "Report", "ReportPacket", "Resolve", "Disagreement",
		"ProductAPI", "Judge"},
	// Engineering mutates inside its capsule and reports fate.
	"engineering": {"WriteFile", "Exec", "Assign", "Message", "Emit",
		"Complete", "Freeze", "Cast", "Ask", "Note", "Reply", "CancelAct",
		"Escalate", "Precommit", "Report", "ReportPacket", "Resolve", "Disagreement"},
	// Research observes the world read-only but has full message authority —
	// read-only world access is not read-only messaging.
	"research": {"Cast", "Ask", "Note", "Reply", "CancelAct",
		"Escalate", "Precommit", "Report", "ReportPacket", "Resolve", "Disagreement",
		"SearchStart", "Searches", "SearchWait", "Read", "ReadCapture", "SearchWireCorpus",
		"SaveEvidence", "ReadEvidence", "ListEvidence", "RunMemoryEntry", "Research", "PollSource"},
	// A child research activation (choir.Research) reads the world and may
	// fan out further; it cannot message, report or write. Its findings
	// return to the parent cell as a value. It runs on the same desk-session
	// machinery as its parent, so it searches the same async way.
	SessionRoleResearchChild: {"SearchStart", "Searches", "SearchWait", "Read", "ReadCapture", "SearchWireCorpus",
		"ReadEvidence", "ListEvidence"},
	// Texture writes its document with one verb, Write, and closes a turn
	// with Done, Wait or Reject (Cut 2 2a); the cell's turn commits through
	// ApplyTextureTurn. It dispatches to research and management with Cast,
	// Ask and Note, which the reducer folds into the same cell's turn
	// (protocol P2a), so a dispatch commits with the turn.
	"texture": {"Cast", "Ask", "Note", "ReadDoc", "Write", "Done", "Wait", "Reject"},
}

// legacyObservation gates the observation verbs of the packet transports
// (Cut 1 L6). Texture, research and management converse on records and read
// choir.Records. Inbox and Emits read the retired Message and Emit mail, and
// Updates reads update packets: engineering keeps all three until its
// conversation moves onto records (C1e), and Texture and management keep
// Updates for the terminal outcomes still delivered as packets.
func (s *ChoirScope) legacyObservation(name string) bool {
	if s == nil {
		return true
	}
	switch s.desk {
	case "texture", "management":
		return name == "Updates"
	case "research", SessionRoleResearchChild:
		return false
	default:
		return true
	}
}

func (s *ChoirScope) deskModule(name string) bool {
	if s == nil {
		return false
	}
	for _, m := range deskModuleSets[s.desk] {
		if m == name {
			return true
		}
	}
	return false
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
	}
	if s.legacyObservation("Inbox") {
		exports["Inbox"] = reflect.ValueOf(s.Inbox)
	}
	if s.legacyObservation("Updates") {
		exports["Updates"] = reflect.ValueOf(s.Updates)
	}
	if s.legacyObservation("Emits") {
		exports["Emits"] = reflect.ValueOf(s.Emits)
	}
	verbs := map[string]func() reflect.Value{
		"WriteFile":        func() reflect.Value { return reflect.ValueOf(s.WriteFile) },
		"Exec":             func() reflect.Value { return reflect.ValueOf(s.Exec) },
		"Assign":           func() reflect.Value { return reflect.ValueOf(s.Assign) },
		"Message":          func() reflect.Value { return reflect.ValueOf(s.Message) },
		"Emit":             func() reflect.Value { return reflect.ValueOf(s.Emit) },
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
		"ReadCapture":      func() reflect.Value { return reflect.ValueOf(s.ReadCapture) },
		"SearchWireCorpus": func() reflect.Value { return reflect.ValueOf(s.SearchWireCorpus) },
		"SaveEvidence":     func() reflect.Value { return reflect.ValueOf(s.SaveEvidence) },
		"ReadEvidence":     func() reflect.Value { return reflect.ValueOf(s.ReadEvidence) },
		"ListEvidence":     func() reflect.Value { return reflect.ValueOf(s.ListEvidence) },
		"RunMemoryEntry":   func() reflect.Value { return reflect.ValueOf(s.RunMemoryEntry) },
		"ProductAPI":       func() reflect.Value { return reflect.ValueOf(s.ProductAPI) },
		"Judge":            func() reflect.Value { return reflect.ValueOf(s.Judge) },
	}
	for name, mint := range verbs {
		// A verb is exported only when the desk's module set admits it AND the
		// scope permits it: mutation verbs additionally require !readOnly.
		if !s.deskModule(name) {
			continue
		}
		if s.readOnly && (name == "WriteFile" || name == "Exec") {
			continue
		}
		exports[name] = mint()
	}
	if s != nil && s.slot == "verifier" {
		exports["Verify"] = reflect.ValueOf(s.Verify)
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

// Assign records a subagent/worker assignment and returns its receipt.
func (s *ChoirScope) Assign(taskID, actorProfile, instruction string) (AssignResult, error) {
	if err := s.mutateDenied("Assign"); err != nil {
		return AssignResult{}, err
	}
	var result AssignResult
	if err := s.call(ActionAssign, AssignPayload{TaskID: taskID, ActorProfile: actorProfile, Instruction: instruction}, &result); err != nil {
		return AssignResult{}, err
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

// ReadCapture reads one shared capture (from any computer's poll or Read):
// its blocks with their ids from block fromBlock, as many as fit in
// maxChars characters (0 means 8000), and next_block when more remain.
func (s *ChoirScope) ReadCapture(captureID string, fromBlock, maxChars int) (JSON, error) {
	var out JSON
	err := s.call(ActionReadCapture, ReadCapturePayload{CaptureID: captureID, FromBlock: fromBlock, MaxChars: maxChars}, &out)
	return out, err
}

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

// Judge judges whether each listed citation pair's cited block supports its
// citing sentence (Cut 2 2a (v)), and stages the verdicts to commit with the
// cell. pairs is a support list's pairs (r.Support.Pairs), one pair, or
// {"key", "verdict", "why"} maps: a pair with a verdict ("supported" or
// "unsupported", a failure with why) is management's own judgement, and the
// others go to Jev through the host, which reads the committed sentence and
// the captured block itself. A cell cannot set a field of a pair it was
// given (yaegi), so its own verdicts are maps. A pair already judged
// returns its memoized verdict (Memo) and is not judged again. The cell's
// commit records each verdict by its pair key: a pass mints nothing, a
// failure reaches the pair's Texture as a record.
func (s *ChoirScope) Judge(pairs any) ([]types.SupportVerdict, error) {
	t, err := s.boundTray("judge")
	if err != nil {
		return nil, err
	}
	list, err := judgePairsArg(pairs)
	if err != nil {
		return nil, err
	}
	var out []types.SupportVerdict
	if err := s.call(ActionJudge, JudgePayload{Pairs: list}, &out); err != nil {
		return nil, err
	}
	var fresh []types.SupportVerdict
	for _, v := range out {
		if v.Memo {
			continue
		}
		v.Pair = nil
		fresh = append(fresh, v)
	}
	if len(fresh) > 0 {
		raw, err := json.Marshal(fresh)
		if err != nil {
			return nil, fmt.Errorf("choir: judge: %w", err)
		}
		if _, err := t.Judge(string(raw)); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// judgePairsArg reads Judge's argument: a list of pairs or maps, or one.
func judgePairsArg(pairs any) ([]types.CitationPair, error) {
	raw, err := jsonCellArg(pairs)
	if err != nil {
		return nil, err
	}
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "{") {
		raw = "[" + raw + "]"
	}
	var list []types.CitationPair
	if raw != "" && raw != "null" {
		if err := json.Unmarshal([]byte(raw), &list); err != nil {
			return nil, fmt.Errorf("choir: judge: pairs are a support list's pairs or {\"key\", \"verdict\", \"why\"} maps: %w", err)
		}
	}
	if len(list) == 0 {
		return nil, fmt.Errorf("choir: judge: no pairs; pass the pairs a support_pairs record lists (r.Support.Pairs)")
	}
	return list, nil
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

// Verify stages the independent verifier decision for the mounted frozen
// bundle. Verifier-slot activations only; the reducer records it against the
// exact mounted bundle on cell return. At most one verify per cell;
// requires a bound cell. bundleDigest is the content_digest InspectBundle
// returned; the reducer asserts it equals the operation's durable digest.
func (s *ChoirScope) Verify(decision string, verifierRefs []string, bundleDigest string) error {
	if err := s.mutateDenied("Verify"); err != nil {
		return err
	}
	if s.slot != "verifier" {
		return fmt.Errorf("choir: verify is restricted to the co-super verifier slot")
	}
	if s.tray == nil {
		return fmt.Errorf("choir: verify requires a bound cell")
	}
	return s.tray.Verify(decision, verifierRefs, bundleDigest)
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
	return append([]PendingUpdate(nil), s.updates...)
}

// Records returns the cell-start snapshot of records addressed to this desk
// in (cursor, head]: Casts, Asks, Notes and Reports, each with its sequence
// number, sender inbox, conversation and round. Side-effect-free inside the
// cell; the cursor moves only when the cell's acts commit.
func (s *ChoirScope) Records() []InboxRecord {
	if s == nil {
		return []InboxRecord{}
	}
	return append([]InboxRecord(nil), s.records...)
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
// One Write per cell.
func (s *ChoirScope) Write(edits, why string) (string, error) {
	t, err := s.boundTray("write")
	if err != nil {
		return "", err
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
		turn.Write = &TextureWrite{Edits: edits, Why: strings.TrimSpace(why)}
		return nil
	})
}

// Done says the requested work is complete. With a Write in the same cell
// the revision completes it; alone it completes it at the current revision.
func (s *ChoirScope) Done(why string) (string, error) {
	return s.closeTurn("done", why, func(turn *TextureTurn, why string) error {
		switch {
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
// (mission R2). packet is a JSON-encoded types.CoagentSourcePacketPayload or
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
