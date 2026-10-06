package yaegikernel

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/traefik/yaegi/interp"
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
	broker       *Broker
	handleRef    string
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
}

// SessionRoleResearch is the read-only role: sessions bound to it observe
// files and directories but cannot write, execute, assign, or message. The
// role arrives on a trusted worker flag from the verified outer capability,
// never from model input. Any other role string means full engineering scope.
const SessionRoleResearch = "research"

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
	readOnly := role == SessionRoleResearch
	actions := []BrokerAction{ActionExec, ActionReadFile, ActionWriteFile, ActionListDir, ActionAssign, ActionMessage, ActionEmit}
	if readOnly {
		// Research is read-only on the filesystem but owns the host-mediated
		// egress verbs (web_search/fetch_url/source_search/import_*/read_*/
		// list_*/search_wire_corpus + evidence/run-memory) — the handle must
		// authorize the frame actions the cell verbs dispatch.
		actions = []BrokerAction{ActionReadFile, ActionListDir, ActionEmit,
			ActionWebSearch, ActionFetchURL, ActionSourceSearch, ActionImportDocument,
			ActionImportURL, ActionReadContentItem, ActionListContentSelectors,
			ActionReadContentSelector, ActionSearchWireCorpus,
			ActionSaveEvidence, ActionReadEvidence, ActionListEvidence, ActionGetRunMemoryEntry}
	}
	// SMG: the management desk's product API call rides the broker like the
	// research egress verbs — the typed product_api_request tool is deleted.
	if normalizeDeskRole(role) == "management" {
		actions = append(actions, ActionProductAPI)
	}
	handleRef, err := issuer.Issue(computerID, "choir-session", epoch, actions, time.Hour)
	if err != nil {
		return nil, fmt.Errorf("choir: issue session handle: %w", err)
	}
	return &ChoirScope{broker: broker, handleRef: handleRef, computerID: computerID, epoch: epoch, activationID: activationID, readOnly: readOnly, desk: normalizeDeskRole(role), slot: slot}, nil
}

// HandleRef returns the bound session handle for host-bound frame requests.
func (s *ChoirScope) HandleRef() string { return s.handleRef }

// normalizeDeskRole maps a session role to the desk profile whose module set
// applies. The four desks are management, engineering, research, texture;
// co-super and unknown roles resolve to the engineering surface so the
// generalized carrier never under-provisions a working desk.
func normalizeDeskRole(role string) string {
	switch role {
	case "management", "engineering", "research", "texture":
		return role
	}
	return "engineering"
}

// BindCell binds one cell: installs the inbox snapshot and a fresh tray,
// returning hooks the session loop drives around evaluation.
func (s *ChoirScope) BindCell() CellHooks {
	return CellHooks{
		Begin: func(frame SessionFrame) {
			s.tray = &Tray{}
			s.inbox = append([]IncomingMessage(nil), frame.Inbox...)
			s.doc = frame.Doc
			s.pack = frame.Pack
			s.updates = append([]PendingUpdate(nil), frame.Updates...)
			s.emits = append([]PendingEmit(nil), frame.Emits...)
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
			return out
		},
	}
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
		HandleRef:       s.handleRef,
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
	// the filesystem (mutation is capsule-bound under engineering).
	"management": {"Message", "Emit", "Cast", "Ask", "Note", "Reply",
		"CancelAct", "CancelAssignment", "Escalate", "Precommit", "Report", "ReportPacket", "Resolve", "Disagreement",
		"ProductAPI"},
	// Engineering mutates inside its capsule and reports fate.
	"engineering": {"WriteFile", "Exec", "Assign", "Message", "Emit",
		"Complete", "Freeze", "Cast", "Ask", "Note", "Reply", "CancelAct",
		"Escalate", "Precommit", "Report", "ReportPacket", "Resolve", "Disagreement"},
	// Research observes the world read-only but has full message authority —
	// read-only world access is not read-only messaging.
	"research": {"Message", "Emit", "Cast", "Ask", "Note", "Reply", "CancelAct",
		"Escalate", "Precommit", "Report", "ReportPacket", "Resolve", "Disagreement",
		"WebSearch", "FetchURL", "SourceSearch", "ImportDocument", "ImportURL",
		"ReadContentItem", "ListContentSelectors", "ReadContentSelector", "SearchWireCorpus",
		"SaveEvidence", "ReadEvidence", "ListEvidence", "RunMemoryEntry"},
	// Texture authors document revisions and escalates; artifact writes are
	// the staged ApplyTexture intent (committed through ApplyTextureTurn),
	// not capsule file ops. Children (research probes, persistent management)
	// open atomically inside the turn via the edit's controls arg — texture
	// never free-spawns or casts.
	"texture": {"Message", "Emit", "Ask", "Note", "Reply", "CancelAct",
		"Escalate", "Precommit", "Report", "ReportPacket", "Resolve", "Disagreement",
		"ReadDoc", "ApplyTexture"},
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
		"Inbox":    reflect.ValueOf(s.Inbox),
		"Updates":  reflect.ValueOf(s.Updates),
		"Emits":    reflect.ValueOf(s.Emits),
		"Pack":     reflect.ValueOf(s.Pack),
	}
	verbs := map[string]func() reflect.Value{
		"WriteFile":            func() reflect.Value { return reflect.ValueOf(s.WriteFile) },
		"Exec":                 func() reflect.Value { return reflect.ValueOf(s.Exec) },
		"Assign":               func() reflect.Value { return reflect.ValueOf(s.Assign) },
		"Message":              func() reflect.Value { return reflect.ValueOf(s.Message) },
		"Emit":                 func() reflect.Value { return reflect.ValueOf(s.Emit) },
		"Complete":             func() reflect.Value { return reflect.ValueOf(s.Complete) },
		"Freeze":               func() reflect.Value { return reflect.ValueOf(s.Freeze) },
		"Cast":                 func() reflect.Value { return reflect.ValueOf(s.Cast) },
		"Ask":                  func() reflect.Value { return reflect.ValueOf(s.Ask) },
		"Note":                 func() reflect.Value { return reflect.ValueOf(s.Note) },
		"Reply":                func() reflect.Value { return reflect.ValueOf(s.Reply) },
		"CancelAct":            func() reflect.Value { return reflect.ValueOf(s.CancelAct) },
		"CancelAssignment":     func() reflect.Value { return reflect.ValueOf(s.CancelAssignment) },
		"Escalate":             func() reflect.Value { return reflect.ValueOf(s.Escalate) },
		"Precommit":            func() reflect.Value { return reflect.ValueOf(s.Precommit) },
		"Report":               func() reflect.Value { return reflect.ValueOf(s.Report) },
		"ReportPacket":         func() reflect.Value { return reflect.ValueOf(s.ReportPacket) },
		"Resolve":              func() reflect.Value { return reflect.ValueOf(s.Resolve) },
		"Disagreement":         func() reflect.Value { return reflect.ValueOf(s.Disagreement) },
		"ReadDoc":              func() reflect.Value { return reflect.ValueOf(s.ReadDoc) },
		"ApplyTexture":         func() reflect.Value { return reflect.ValueOf(s.ApplyTexture) },
		"WebSearch":            func() reflect.Value { return reflect.ValueOf(s.WebSearch) },
		"FetchURL":             func() reflect.Value { return reflect.ValueOf(s.FetchURL) },
		"SourceSearch":         func() reflect.Value { return reflect.ValueOf(s.SourceSearch) },
		"ImportDocument":       func() reflect.Value { return reflect.ValueOf(s.ImportDocument) },
		"ImportURL":            func() reflect.Value { return reflect.ValueOf(s.ImportURL) },
		"ReadContentItem":      func() reflect.Value { return reflect.ValueOf(s.ReadContentItem) },
		"ListContentSelectors": func() reflect.Value { return reflect.ValueOf(s.ListContentSelectors) },
		"ReadContentSelector":  func() reflect.Value { return reflect.ValueOf(s.ReadContentSelector) },
		"SearchWireCorpus":     func() reflect.Value { return reflect.ValueOf(s.SearchWireCorpus) },
		"SaveEvidence":         func() reflect.Value { return reflect.ValueOf(s.SaveEvidence) },
		"ReadEvidence":         func() reflect.Value { return reflect.ValueOf(s.ReadEvidence) },
		"ListEvidence":         func() reflect.Value { return reflect.ValueOf(s.ListEvidence) },
		"RunMemoryEntry":       func() reflect.Value { return reflect.ValueOf(s.RunMemoryEntry) },
		"ProductAPI":           func() reflect.Value { return reflect.ValueOf(s.ProductAPI) },
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

// Message records a typed inter-agent message and returns its receipt. While
// a cell is bound it stages into the tray and returns immediately with the
// cell-local correlation ID (tray bookkeeping, never a delivery receipt);
// unbound it keeps the legacy synchronous broker call.
func (s *ChoirScope) Message(recipientID, kind, body string) (MessageResult, error) {
	if err := s.mutateDenied("Message"); err != nil {
		return MessageResult{}, err
	}
	if s.tray != nil {
		if recipientID == "" {
			return MessageResult{}, fmt.Errorf("choir: message requires a destination desk")
		}
		localID, err := s.tray.stage(StagedIntent{Kind: IntentMessage, ToDesk: recipientID, MsgKind: kind, Body: body})
		if err != nil {
			return MessageResult{}, err
		}
		return MessageResult{MessageID: localID}, nil
	}
	var result MessageResult
	if err := s.call(ActionMessage, MessagePayload{RecipientID: recipientID, Kind: kind, Body: body}, &result); err != nil {
		return MessageResult{}, err
	}
	return result, nil
}

// Emit durably sends an immediate signal to another desk. Unlike Message, it
// never enters the bound cell's tray: the host writes the "emit" envelope and
// wakes the recipient before this call returns. The result's AdvisorySeq is
// the piggyback watermark — the newest emission seq addressed to *this* desk,
// so a caller can observe a mid-activation arrival without waiting for the
// next boundary notice.
func (s *ChoirScope) Emit(toDesk, kind, body string) (EmitResult, error) {
	if err := s.mutateDenied("Emit"); err != nil {
		return EmitResult{}, err
	}
	var result EmitResult
	if err := s.call(ActionEmit, EmitPayload{ToDesk: toDesk, Kind: kind, Body: body}, &result); err != nil {
		return EmitResult{}, err
	}
	return result, nil
}

// WebSearch resolves a web search through the host's search client under the
// per-activation egress budget — a synchronous broker call, never an open
// socket. Returns the tool's bounded projection as JSON.
func (s *ChoirScope) WebSearch(query string, maxResults int) (json.RawMessage, error) {
	var out json.RawMessage
	err := s.call(ActionWebSearch, WebSearchPayload{Query: query, MaxResults: maxResults}, &out)
	return out, err
}

// FetchURL resolves a URL fetch through the host's HTTP client under the
// per-activation egress budget — synchronous, bounded, never an open socket.
// Returns the tool's bounded projection as JSON.
func (s *ChoirScope) FetchURL(url string) (json.RawMessage, error) {
	var out json.RawMessage
	err := s.call(ActionFetchURL, FetchURLPayload{URL: url}, &out)
	return out, err
}

// SourceSearch queries the Source Service through the host; returns the
// bounded source-list projection as JSON.
func (s *ChoirScope) SourceSearch(query string, maxResults int) (json.RawMessage, error) {
	var out json.RawMessage
	err := s.call(ActionSourceSearch, SourceSearchPayload{Query: query, MaxResults: maxResults}, &out)
	return out, err
}

// ImportDocument imports a URL or file path into the ContentItem substrate.
func (s *ChoirScope) ImportDocument(url, filePath, query string) (json.RawMessage, error) {
	var out json.RawMessage
	err := s.call(ActionImportDocument, ImportDocumentPayload{URL: url, FilePath: filePath, Query: query}, &out)
	return out, err
}

// ImportURL imports a URL into the ContentItem substrate.
func (s *ChoirScope) ImportURL(url, query string) (json.RawMessage, error) {
	var out json.RawMessage
	err := s.call(ActionImportURL, ImportURLPayload{URL: url, Query: query}, &out)
	return out, err
}

// ReadContentItem reads an owner-scoped ContentItem's bounded text/metadata.
func (s *ChoirScope) ReadContentItem(contentID string, maxTextChars, maxSegments int) (json.RawMessage, error) {
	var out json.RawMessage
	err := s.call(ActionReadContentItem, ReadContentItemPayload{ContentID: contentID, MaxTextChars: maxTextChars, MaxSegments: maxSegments}, &out)
	return out, err
}

// ListContentSelectors lists addressable selectors (pages/slides/chunks).
func (s *ChoirScope) ListContentSelectors(contentID string) (json.RawMessage, error) {
	var out json.RawMessage
	err := s.call(ActionListContentSelectors, ListContentSelectorsPayload{ContentID: contentID}, &out)
	return out, err
}

// ReadContentSelector reads one exact selector's text from a ContentItem.
func (s *ChoirScope) ReadContentSelector(contentID, selectorID string, maxTextChars int) (json.RawMessage, error) {
	var out json.RawMessage
	err := s.call(ActionReadContentSelector, ReadContentSelectorPayload{ContentID: contentID, SelectorID: selectorID, MaxTextChars: maxTextChars}, &out)
	return out, err
}

// SearchWireCorpus searches the owner's published wire corpus.
func (s *ChoirScope) SearchWireCorpus(query string, limit int) (json.RawMessage, error) {
	var out json.RawMessage
	err := s.call(ActionSearchWireCorpus, SearchWireCorpusPayload{Query: query, Limit: limit}, &out)
	return out, err
}

// SaveEvidence persists evidentiary material into the owner's Dolt workspace.
func (s *ChoirScope) SaveEvidence(kind, sourceURI, title, content string, metadata json.RawMessage) (json.RawMessage, error) {
	var out json.RawMessage
	err := s.call(ActionSaveEvidence, SaveEvidencePayload{Kind: kind, SourceURI: sourceURI, Title: title, Content: content, Metadata: metadata}, &out)
	return out, err
}

// ReadEvidence reads one saved evidence record by id.
func (s *ChoirScope) ReadEvidence(evidenceID string) (json.RawMessage, error) {
	var out json.RawMessage
	err := s.call(ActionReadEvidence, ReadEvidencePayload{EvidenceID: evidenceID}, &out)
	return out, err
}

// ListEvidence lists recent saved evidence for an agent or the owner scope.
func (s *ChoirScope) ListEvidence(agentID string, limit int) (json.RawMessage, error) {
	var out json.RawMessage
	err := s.call(ActionListEvidence, ListEvidencePayload{AgentID: agentID, Limit: limit}, &out)
	return out, err
}

// RunMemoryEntry retrieves one durable run-memory entry by id.
func (s *ChoirScope) RunMemoryEntry(entryID string) (json.RawMessage, error) {
	var out json.RawMessage
	err := s.call(ActionGetRunMemoryEntry, GetRunMemoryEntryPayload{EntryID: entryID}, &out)
	return out, err
}

// ProductAPI calls an allowlisted authenticated product API route as the run
// owner — the management desk's in-cell replacement for the deleted
// product_api_request tool (SMG). Method is GET/POST/PUT/DELETE; path is an
// absolute /api/... route on the product-path allowlist; body is an optional
// JSON object. The host validates and serves the route, returning a bounded
// {status_code, content_type, body} JSON result.
func (s *ChoirScope) ProductAPI(method, path string, body json.RawMessage) (json.RawMessage, error) {
	var out json.RawMessage
	err := s.call(ActionProductAPI, ProductAPIPayload{Method: method, Path: path, Body: body}, &out)
	return out, err
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

// ReadDoc returns the cell-start texture document snapshot for a texture desk
// cell (R3d): the bound document's current revision id — the base_revision_id
// a staged ApplyTexture must cite — plus its full content. Side-effect-free
// inside the cell; the snapshot is injected by autoputer at cell launch.
// Empty for non-texture scopes.
func (s *ChoirScope) ReadDoc() DocSnapshot {
	if s == nil || s.doc == nil {
		return DocSnapshot{}
	}
	return *s.doc
}

// ApplyTexture stages a full-RLM texture authoring turn (R3d). edit is the
// cell-authored texture body — {op, doc_id?, base_revision_id, content? or
// edits?, update_dispositions?, controls?, work_disposition?, rationale?}
// — accepted as a JSON-encoded string or any JSON-marshalable Go value (the
// map literal form a cell naturally writes). The reducer commits it as an
// AuthorAppAgent revision through the atomic ApplyTextureTurn transaction —
// the genuine authoring turn, not a projection.
func (s *ChoirScope) ApplyTexture(edit any) (string, error) {
	t, err := s.boundTray("apply_texture")
	if err != nil {
		return "", err
	}
	editJSON, err := jsonCellArg(edit)
	if err != nil {
		return "", err
	}
	return t.ApplyTexture(editJSON)
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
