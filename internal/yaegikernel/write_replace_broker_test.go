package yaegikernel

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// The cell's choir.WriteFile reaches the file through Broker.HandleRequest;
// pin that the wiring edits a read-only existing file (the drill's routes.go).
func TestBrokerWriteFileEditsAReadOnlyExistingFile(t *testing.T) {
	skipIfRoot(t)
	secret := make([]byte, 32)
	for i := range secret {
		secret[i] = byte(i + 1)
	}
	issuer, err := NewHandleIssuer(secret)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.Chmod(root, 0o777); err != nil {
		t.Fatal(err)
	}
	routes := filepath.Join(root, "routes.go")
	if err := os.WriteFile(routes, []byte("package apihandler\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(routes, 0o444); err != nil {
		t.Fatal(err)
	}
	broker, err := NewBroker(BrokerConfig{ComputerID: "computer-test-1", CurrentEpoch: 1, AllowedRoot: root}, issuer)
	if err != nil {
		t.Fatal(err)
	}
	handleRef, err := issuer.Issue("computer-test-1", "engineering", 1, []BrokerAction{ActionWriteFile}, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(WriteFilePayload{Path: "routes.go", Content: "package apihandler\n// edited\n"})
	resp := broker.HandleRequest(context.Background(), &BrokerRequest{
		ProtocolVersion: ProtocolVersion, RequestID: "req-edit", HandleRef: handleRef,
		Epoch: 1, Action: ActionWriteFile, Payload: payload,
	})
	if !resp.Success {
		t.Fatalf("write_file over a 0444 file failed: %s", resp.Error)
	}
	if got, _ := os.ReadFile(routes); string(got) != "package apihandler\n// edited\n" {
		t.Fatalf("routes.go = %q", got)
	}
}
