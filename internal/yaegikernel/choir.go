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
	// Cell binding (RLM Step 4): while a cell is bound, messaging and
	// delegation stage into the tray and return in microseconds without
	// blocking; Inbox() reads the bound snapshot. Unbound scopes keep the
	// legacy synchronous broker behavior (one-shot path). The session loop
	// owns binding lifetime: exactly one cell binds at a time.
	tray  *Tray
	inbox []IncomingMessage
}

// SessionRoleResearcher is the read-only role: sessions bound to it observe
// files and directories but cannot write, execute, assign, or message. The
// role arrives on a trusted worker flag from the verified outer capability,
// never from model input. Any other role string means full CoSuper scope.
const SessionRoleResearcher = "researcher"

// NewChoirScope mints a session-scoped handle for exactly the file, assign,
// and message actions and returns the scope the choir symbols close over.
// The caller (worker session setup) owns the broker lifetime.
func NewChoirScope(broker *Broker, issuer *HandleIssuer, computerID, activationID string, epoch uint64, role string) (*ChoirScope, error) {
	if broker == nil {
		return nil, fmt.Errorf("choir: broker is required")
	}
	if issuer == nil {
		return nil, fmt.Errorf("choir: handle issuer is required")
	}
	actions := []BrokerAction{ActionExec, ActionReadFile, ActionWriteFile, ActionListDir, ActionAssign, ActionMessage}
	readOnly := role == SessionRoleResearcher
	if readOnly {
		actions = []BrokerAction{ActionReadFile, ActionListDir}
	}
	handleRef, err := issuer.Issue(computerID, "choir-session", epoch, actions, time.Hour)
	if err != nil {
		return nil, fmt.Errorf("choir: issue session handle: %w", err)
	}
	return &ChoirScope{broker: broker, handleRef: handleRef, computerID: computerID, epoch: epoch, activationID: activationID, readOnly: readOnly}, nil
}

// BindCell binds one cell: installs the inbox snapshot and a fresh tray,
// returning hooks the session loop drives around evaluation.
func (s *ChoirScope) BindCell() CellHooks {
	return CellHooks{
		Begin: func(frame SessionFrame) {
			s.tray = &Tray{}
			s.inbox = append([]IncomingMessage(nil), frame.Inbox...)
		},
		End: func() []StagedIntent {
			var out []StagedIntent
			if s.tray != nil {
				out = s.tray.Drain()
				s.tray = nil
			}
			s.inbox = nil
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

// ChoirExports returns the prebound choir package for model-authored Go:
// file operations, assignment, activation context, outcome reporting, plus
// the RLM orchestration surface (tray-staged Message/Spawn/Complete and the
// side-effect-free Inbox snapshot). Model code imports it as `import
// "choir"`. Read-only scopes export observation plus context and inbox only;
// mutation entry points are omitted AND method-guarded, so a future export
// mistake cannot re-open them.
func (s *ChoirScope) ChoirExports() interp.Exports {
	exports := map[string]reflect.Value{
		"ReadFile": reflect.ValueOf(s.ReadFile),
		"ListDir":  reflect.ValueOf(s.ListDir),
		"Context":  reflect.ValueOf(s.Context),
		"Inbox":    reflect.ValueOf(s.Inbox),
	}
	if s == nil || !s.readOnly {
		exports["WriteFile"] = reflect.ValueOf(s.WriteFile)
		exports["Exec"] = reflect.ValueOf(s.Exec)
		exports["Assign"] = reflect.ValueOf(s.Assign)
		exports["Message"] = reflect.ValueOf(s.Message)
		exports["Outcome"] = reflect.ValueOf(s.Outcome)
		exports["Spawn"] = reflect.ValueOf(s.Spawn)
		exports["Complete"] = reflect.ValueOf(s.Complete)
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
// into the cell tray; the reducer binds execution receipts to it. At most
// one complete per cell. It requires a bound cell.
func (s *ChoirScope) Complete(result, verdict, summary string, evidenceRefs []string) error {
	if err := s.mutateDenied("Complete"); err != nil {
		return err
	}
	if s.tray == nil {
		return fmt.Errorf("choir: complete requires a bound cell")
	}
	return s.tray.Complete(result, verdict, summary, evidenceRefs)
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
	}
}

// Outcome records the cell's outcome as a durable self-report message to the
// owning activation, returning its receipt. It is the model-visible end of a
// read->compute->write->assign arc: the value is retained broker-side where
// the host reconciles it.
func (s *ChoirScope) Outcome(value string) (MessageResult, error) {
	if s == nil {
		return MessageResult{}, fmt.Errorf("choir: scope unavailable")
	}
	return s.Message(s.activationID, "outcome", value)
}
