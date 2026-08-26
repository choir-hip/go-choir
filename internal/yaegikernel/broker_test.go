package yaegikernel

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestHandleIssuerAndVerifier(t *testing.T) {
	secret := make([]byte, 32)
	for i := range secret {
		secret[i] = byte(i + 1)
	}
	issuer, err := NewHandleIssuer(secret)
	if err != nil {
		t.Fatalf("NewHandleIssuer failed: %v", err)
	}

	scopes := []BrokerAction{ActionExec, ActionReadFile}
	handleRef, err := issuer.Issue("computer-test-1", "cosuper", 1, scopes, 10*time.Minute)
	if err != nil {
		t.Fatalf("Issue failed: %v", err)
	}

	// 1. Valid verification
	data, err := issuer.Verify(handleRef, "computer-test-1", 1, ActionExec)
	if err != nil {
		t.Fatalf("valid handle verification failed: %v", err)
	}
	if data.ComputerID != "computer-test-1" || data.Epoch != 1 {
		t.Fatalf("unexpected handle data: %+v", data)
	}

	// 2. Computer mismatch
	if _, err := issuer.Verify(handleRef, "other-computer", 1, ActionExec); err == nil {
		t.Fatal("expected error on computer mismatch")
	}

	// 3. Stale epoch
	if _, err := issuer.Verify(handleRef, "computer-test-1", 2, ActionExec); err == nil {
		t.Fatal("expected error on stale epoch")
	}

	// 4. Unauthorized action scope
	if _, err := issuer.Verify(handleRef, "computer-test-1", 1, ActionWriteFile); err == nil {
		t.Fatal("expected error on unauthorized action scope")
	}

	// 5. Tampered handle signature
	tampered := handleRef[:len(handleRef)-4] + "xxxx"
	if _, err := issuer.Verify(tampered, "computer-test-1", 1, ActionExec); err == nil {
		t.Fatal("expected error on tampered handle")
	}
}

func TestBrokerExecAndPathSecurity(t *testing.T) {
	secret := make([]byte, 32)
	for i := range secret {
		secret[i] = byte(i + 1)
	}
	issuer, err := NewHandleIssuer(secret)
	if err != nil {
		t.Fatal(err)
	}

	root := t.TempDir()
	cfg := BrokerConfig{
		ComputerID:   "computer-test-1",
		CurrentEpoch: 1,
		AllowedRoot:  root,
	}
	broker, err := NewBroker(cfg, issuer)
	if err != nil {
		t.Fatal(err)
	}

	scopes := []BrokerAction{ActionExec, ActionReadFile, ActionWriteFile, ActionAssign, ActionMessage}
	handleRef, err := issuer.Issue("computer-test-1", "cosuper", 1, scopes, 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	// 1. Exec command
	execPayload, _ := json.Marshal(ExecPayload{
		Command: "echo",
		Args:    []string{"broker-exec-ok"},
	})
	req := &BrokerRequest{
		ProtocolVersion: ProtocolVersion,
		RequestID:       "req-001",
		HandleRef:       handleRef,
		Epoch:           1,
		Action:          ActionExec,
		Payload:         execPayload,
	}
	resp := broker.HandleRequest(ctx, req)
	if !resp.Success {
		t.Fatalf("HandleRequest exec failed: %s", resp.Error)
	}
	var execRes ExecResult
	if err := json.Unmarshal(resp.Result, &execRes); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(execRes.Stdout) != "broker-exec-ok" || execRes.ExitCode != 0 {
		t.Fatalf("unexpected exec result: %+v", execRes)
	}
	if !strings.HasPrefix(resp.ReceiptID, "rcpt_") {
		t.Fatalf("expected valid receipt ID: %s", resp.ReceiptID)
	}

	// 2. Write file
	writePayload, _ := json.Marshal(WriteFilePayload{
		Path:    "notes/task.txt",
		Content: "task content",
	})
	reqWrite := &BrokerRequest{
		ProtocolVersion: ProtocolVersion,
		RequestID:       "req-002",
		HandleRef:       handleRef,
		Epoch:           1,
		Action:          ActionWriteFile,
		Payload:         writePayload,
	}
	respWrite := broker.HandleRequest(ctx, reqWrite)
	if !respWrite.Success {
		t.Fatalf("HandleRequest write_file failed: %s", respWrite.Error)
	}

	// 3. Read file
	readPayload, _ := json.Marshal(ReadFilePayload{
		Path: "notes/task.txt",
	})
	reqRead := &BrokerRequest{
		ProtocolVersion: ProtocolVersion,
		RequestID:       "req-003",
		HandleRef:       handleRef,
		Epoch:           1,
		Action:          ActionReadFile,
		Payload:         readPayload,
	}
	respRead := broker.HandleRequest(ctx, reqRead)
	if !respRead.Success {
		t.Fatalf("HandleRequest read_file failed: %s", respRead.Error)
	}
	var readRes ReadFileResult
	if err := json.Unmarshal(respRead.Result, &readRes); err != nil {
		t.Fatal(err)
	}
	if readRes.Content != "task content" {
		t.Fatalf("unexpected read content: %q", readRes.Content)
	}

	// 4. Path traversal escape should be refused
	escapePayload, _ := json.Marshal(ReadFilePayload{
		Path: "../../../../etc/passwd",
	})
	reqEscape := &BrokerRequest{
		ProtocolVersion: ProtocolVersion,
		RequestID:       "req-004",
		HandleRef:       handleRef,
		Epoch:           1,
		Action:          ActionReadFile,
		Payload:         escapePayload,
	}
	respEscape := broker.HandleRequest(ctx, reqEscape)
	if respEscape.Success {
		t.Fatal("expected path traversal request to fail")
	}
	if !strings.Contains(respEscape.Error, "escapes allowed root") {
		t.Fatalf("unexpected escape error message: %s", respEscape.Error)
	}
}

func TestBrokerEpochFencing(t *testing.T) {
	secret := make([]byte, 32)
	issuer, err := NewHandleIssuer(secret)
	if err != nil {
		t.Fatal(err)
	}

	broker, err := NewBroker(BrokerConfig{
		ComputerID:   "computer-test-1",
		CurrentEpoch: 1,
		AllowedRoot:  t.TempDir(),
	}, issuer)
	if err != nil {
		t.Fatal(err)
	}

	handleRef, err := issuer.Issue("computer-test-1", "cosuper", 1, []BrokerAction{ActionExec}, 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	payload, _ := json.Marshal(ExecPayload{Command: "echo", Args: []string{"test"}})
	req := &BrokerRequest{
		ProtocolVersion: ProtocolVersion,
		RequestID:       "req-epoch-1",
		HandleRef:       handleRef,
		Epoch:           1,
		Action:          ActionExec,
		Payload:         payload,
	}

	// Succeeded on epoch 1
	resp1 := broker.HandleRequest(context.Background(), req)
	if !resp1.Success {
		t.Fatalf("epoch 1 request failed: %s", resp1.Error)
	}

	// Advance epoch to 2 (e.g. after forced activation kill and rewarm)
	broker.SetEpoch(2)

	// Old handle should now be rejected as stale epoch
	resp2 := broker.HandleRequest(context.Background(), req)
	if resp2.Success {
		t.Fatal("expected request with stale handle epoch to be rejected")
	}
	if !strings.Contains(resp2.Error, "stale activation epoch") {
		t.Fatalf("unexpected stale error message: %s", resp2.Error)
	}
}
