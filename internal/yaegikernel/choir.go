package yaegikernel

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/traefik/yaegi/interp"
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
	actions := []BrokerAction{ActionExec, ActionReadFile, ActionWriteFile, ActionListDir, ActionAssign, ActionMessage}
	if readOnly {
		actions = []BrokerAction{ActionReadFile, ActionListDir}
	}
	handleRef, err := issuer.Issue(computerID, "choir-session", epoch, actions, time.Hour)
	if err != nil {
		return nil, fmt.Errorf("choir: issue session handle: %w", err)
	}
	return &ChoirScope{broker: broker, handleRef: handleRef, computerID: computerID, epoch: epoch, activationID: activationID, readOnly: readOnly, desk: normalizeDeskRole(role), slot: slot}, nil
}

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
		},
		End: func() []StagedIntent {
			var out []StagedIntent
			if s.tray != nil {
				out = s.tray.Drain()
				s.tray = nil
			}
			s.inbox = nil
			s.doc = nil
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
// "file" = ReadFile+ListDir+WriteFile+Exec; "delegate" = Assign+Spawn;
// "commit" = Complete+Freeze; "epistemic" = the full semantic-act surface.
var deskModuleSets = map[string][]string{
	// Management delegates engineering work and reports; it does not touch
	// the filesystem (mutation is capsule-bound under engineering).
	"management": {"Message", "Outcome", "Spawn", "Cast", "Ask", "Note", "Reply",
		"CancelAct", "Escalate", "EscalateActions", "Precommit", "Report", "ReportPacket", "ResolveAct"},
	// Engineering mutates inside its capsule and reports fate.
	"engineering": {"WriteFile", "Exec", "Assign", "Message", "Outcome", "Spawn",
		"Complete", "Freeze", "Cast", "Ask", "Note", "Reply", "CancelAct",
		"Escalate", "EscalateActions", "Precommit", "Report", "ReportPacket", "ResolveAct"},
	// Research observes the world read-only but has full message authority —
	// read-only world access is not read-only messaging.
	"research": {"Message", "Outcome", "Cast", "Ask", "Note", "Reply", "CancelAct",
		"Escalate", "EscalateActions", "Precommit", "Report", "ReportPacket", "ResolveAct"},
	// Texture authors document revisions and escalates; artifact writes are
	// the staged ApplyTexture intent (committed through ApplyTextureTurn),
	// not capsule file ops. Children (research probes, persistent management)
	// open atomically inside the turn via the edit's controls arg — texture
	// never free-spawns or casts.
	"texture": {"Message", "Outcome", "Ask", "Note", "Reply", "CancelAct",
		"Escalate", "EscalateActions", "Precommit", "Report", "ReportPacket", "ResolveAct",
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
	}
	verbs := map[string]func() reflect.Value{
		"WriteFile":       func() reflect.Value { return reflect.ValueOf(s.WriteFile) },
		"Exec":            func() reflect.Value { return reflect.ValueOf(s.Exec) },
		"Assign":          func() reflect.Value { return reflect.ValueOf(s.Assign) },
		"Message":         func() reflect.Value { return reflect.ValueOf(s.Message) },
		"Outcome":         func() reflect.Value { return reflect.ValueOf(s.Outcome) },
		"Spawn":           func() reflect.Value { return reflect.ValueOf(s.Spawn) },
		"Complete":        func() reflect.Value { return reflect.ValueOf(s.Complete) },
		"Freeze":          func() reflect.Value { return reflect.ValueOf(s.Freeze) },
		"Cast":            func() reflect.Value { return reflect.ValueOf(s.Cast) },
		"Ask":             func() reflect.Value { return reflect.ValueOf(s.Ask) },
		"Note":            func() reflect.Value { return reflect.ValueOf(s.Note) },
		"Reply":           func() reflect.Value { return reflect.ValueOf(s.Reply) },
		"CancelAct":       func() reflect.Value { return reflect.ValueOf(s.CancelAct) },
		"Escalate":        func() reflect.Value { return reflect.ValueOf(s.Escalate) },
		"EscalateActions": func() reflect.Value { return reflect.ValueOf(s.EscalateActions) },
		"Precommit":       func() reflect.Value { return reflect.ValueOf(s.Precommit) },
		"Report":          func() reflect.Value { return reflect.ValueOf(s.Report) },
		"ReportPacket":    func() reflect.Value { return reflect.ValueOf(s.ReportPacket) },
		"ReadDoc":         func() reflect.Value { return reflect.ValueOf(s.ReadDoc) },
		"ApplyTexture":    func() reflect.Value { return reflect.ValueOf(s.ApplyTexture) },
		"ResolveAct": func() reflect.Value {
			return reflect.ValueOf(s.ResolveAct)
		},
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

// mutateDenied rejects model-reachable mutation on a read-only scope.
func (s *ChoirScope) mutateDenied(op string) error {
	if s != nil && s.readOnly {
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

// Spawn asynchronously delegates a subtask within role policy. It stages
// into the cell tray and returns a cell-local child handle; it requires a
// bound cell because delegation only reduces after a successful cell.
func (s *ChoirScope) Spawn(role, objective string) (string, error) {
	if err := s.mutateDenied("Spawn"); err != nil {
		return "", err
	}
	if s.tray == nil {
		return "", fmt.Errorf("choir: spawn requires a bound cell")
	}
	return s.tray.Spawn(role, objective)
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

// ApplyTexture stages a full-RLM texture authoring turn (R3d): editJSON is the
// JSON-encoded texture edit {doc_id?, base_revision_id, content? or edits?,
// update_dispositions?, controls?, work_disposition?, rationale?}. The reducer
// commits it as an AuthorAppAgent revision through the atomic ApplyTextureTurn
// transaction — the genuine authoring turn, not a projection.
func (s *ChoirScope) ApplyTexture(editJSON string) (string, error) {
	t, err := s.boundTray("apply_texture")
	if err != nil {
		return "", err
	}
	return t.ApplyTexture(editJSON)
}

// Outcome records the cell's outcome as a durable self-report message to the
// owning activation, returning its receipt. It is the model-visible end of a
// read->compute->write->assign arc: the value is retained broker-side where
// the host reconciles it. It stages as IntentOutcome, a distinct kind, so a
// staged Message cannot claim the outcome envelope path and skip the
// assigned desk's update authority.
func (s *ChoirScope) Outcome(value string) (MessageResult, error) {
	if s == nil {
		return MessageResult{}, fmt.Errorf("choir: scope unavailable")
	}
	if err := s.mutateDenied("Outcome"); err != nil {
		return MessageResult{}, err
	}
	if s.tray != nil {
		localID, err := s.tray.Outcome(s.activationID, value)
		if err != nil {
			return MessageResult{}, err
		}
		return MessageResult{MessageID: localID}, nil
	}
	return s.Message(s.activationID, "outcome", value)
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

// Cast stages delegated admission of a downstream assignment.
func (s *ChoirScope) Cast(desk, objective, spec string) (string, error) {
	t, err := s.boundTray("cast")
	if err != nil {
		return "", err
	}
	return t.Cast(desk, objective, spec)
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

// Escalate surfaces an issue to management or the owner.
func (s *ChoirScope) Escalate(toDesk, issue string) (string, error) {
	t, err := s.boundTray("escalate")
	if err != nil {
		return "", err
	}
	return t.Escalate(toDesk, issue)
}

// EscalateActions surfaces a privileged-execution request to management: the
// desk asks the target to run a set of guarded actions (the execution_request
// packet kind on the carrier). actionsJSON is a JSON-encoded
// []types.CoagentPacketAction with explicit per-action safety annotations; the
// reducer validates the schema before the envelope mails.
func (s *ChoirScope) EscalateActions(toDesk, issue, actionsJSON string) (string, error) {
	t, err := s.boundTray("escalate")
	if err != nil {
		return "", err
	}
	return t.EscalateActions(toDesk, issue, actionsJSON)
}

// Precommit freezes a typed prediction on the commitment ledger.
func (s *ChoirScope) Precommit(statement, resolverID, deadline string) (string, error) {
	t, err := s.boundTray("precommit")
	if err != nil {
		return "", err
	}
	return t.Precommit(statement, resolverID, deadline)
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
// (mission R2). packetJSON is a JSON-encoded types.CoagentSourcePacketPayload;
// the reducer validates kind/claims/sources/actions/questions before commit.
func (s *ChoirScope) ReportPacket(toDesk, packetJSON, resolverID string) (string, error) {
	t, err := s.boundTray("report")
	if err != nil {
		return "", err
	}
	return t.ReportPacket(toDesk, packetJSON, resolverID)
}

// ResolveAct is the named resolver's act closing a Report/Ask/Precommit.
func (s *ChoirScope) ResolveAct(targetRef, outcome string) (string, error) {
	t, err := s.boundTray("resolve")
	if err != nil {
		return "", err
	}
	return t.Resolve(targetRef, outcome)
}
