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

// ChoirExports returns the prebound choir package for model-authored Go
// (Def 2 item 3): file operations, assignment, messaging, activation context,
// and outcome reporting, all executed through the broker with receipts. Model
// code imports it as `import "choir"`. Read-only scopes export observation
// plus context only; mutation entry points are omitted AND method-guarded,
// so a future export mistake cannot re-open them.
func (s *ChoirScope) ChoirExports() interp.Exports {
	exports := map[string]reflect.Value{
		"ReadFile": reflect.ValueOf(s.ReadFile),
		"ListDir":  reflect.ValueOf(s.ListDir),
		"Context":  reflect.ValueOf(s.Context),
	}
	if s == nil || !s.readOnly {
		exports["WriteFile"] = reflect.ValueOf(s.WriteFile)
		exports["Exec"] = reflect.ValueOf(s.Exec)
		exports["Assign"] = reflect.ValueOf(s.Assign)
		exports["Message"] = reflect.ValueOf(s.Message)
		exports["Outcome"] = reflect.ValueOf(s.Outcome)
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

// Message records a typed inter-agent message and returns its receipt.
func (s *ChoirScope) Message(recipientID, kind, body string) (MessageResult, error) {
	if err := s.mutateDenied("Message"); err != nil {
		return MessageResult{}, err
	}
	var result MessageResult
	if err := s.call(ActionMessage, MessagePayload{RecipientID: recipientID, Kind: kind, Body: body}, &result); err != nil {
		return MessageResult{}, err
	}
	return result, nil
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
