//go:build linux

package capsule

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type testCapsuleCgroup struct {
	frozen    bool
	freezeErr error
	thawErr   error
}

func (c *testCapsuleCgroup) Open() (*os.File, error) { return nil, nil }
func (c *testCapsuleCgroup) Delete() error           { return nil }
func (c *testCapsuleCgroup) Freeze(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.frozen = true
	return c.freezeErr
}
func (c *testCapsuleCgroup) Thaw(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.frozen = false
	return c.thawErr
}

func TestStageGrantedReleaseRefusesSecrets(t *testing.T) {
	for name, relative := range map[string]struct {
		path    string
		content string
	}{
		"secret path":                {path: ".env.production", content: "ordinary"},
		"secret directory component": {path: ".env.production/config", content: "ordinary"},
		"secret content":             {path: "config.txt", content: "api_key=abcdefghijklmnop"},
	} {
		t.Run(name, func(t *testing.T) {
			merged := t.TempDir()
			upper := t.TempDir()
			for _, root := range []string{merged, upper} {
				if err := os.MkdirAll(filepath.Join(root, "var/lib/artifact/release/bin"), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, "var/lib/artifact/release/bin/sandbox"), []byte("sandbox"), 0o755); err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(root, "var/lib/artifact/release", filepath.FromSlash(relative.path))
				if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(relative.content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
			if err != nil {
				t.Fatal(err)
			}
			capability := &Capability{CapabilityID: "cap-1", Handle: "grant-1", CapsuleID: "capsule-1", TargetCapsule: "capsule-1", AgentRunID: "cosuper-1", AgentRole: RoleCoSuper, Verbs: RoleVerbSets[RoleCoSuper], ExpiresAt: time.Now().Add(time.Hour)}
			if err := SignCapability(capability, privateKey, "test-key"); err != nil {
				t.Fatal(err)
			}
			executor := &Executor{
				capsules:     map[string]*Capsule{"capsule-1": {ID: "capsule-1", State: StateFrozen, UpperDir: upper, MergedDir: merged, MemoryMax: 16 << 20}},
				capabilities: map[capKey]*Capability{{AgentRunID: "cosuper-1", Handle: "grant-1"}: capability},
				revokedCaps:  map[string]bool{}, publicKey: publicKey,
			}
			incoming := t.TempDir()
			if err := os.Chmod(incoming, 0o700); err != nil {
				t.Fatal(err)
			}
			_, _, err = executor.StageGrantedRelease("cosuper-1", "grant-1", incoming)
			if err == nil || !strings.Contains(err.Error(), "refuses secret") {
				t.Fatalf("secret release error = %v", err)
			}
		})
	}
}

func TestStageGrantedReleaseStagesRelativeUpperdirPaths(t *testing.T) {
	merged := t.TempDir()
	upper := t.TempDir()
	for _, root := range []string{merged, upper} {
		path := filepath.Join(root, "var/lib/artifact/release/bin/sandbox")
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("sandbox"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	capability := &Capability{
		CapabilityID: "cap-success", Handle: "grant-success", CapsuleID: "capsule-success",
		TargetCapsule: "capsule-success", AgentRunID: "cosuper-success", AgentRole: RoleCoSuper,
		Verbs: RoleVerbSets[RoleCoSuper], ExpiresAt: time.Now().Add(time.Hour),
	}
	if err := SignCapability(capability, privateKey, "test-key"); err != nil {
		t.Fatal(err)
	}
	executor := &Executor{
		capsules: map[string]*Capsule{"capsule-success": {
			ID: "capsule-success", State: StateFrozen, UpperDir: upper, MergedDir: merged, MemoryMax: 16 << 20,
		}},
		capabilities: map[capKey]*Capability{{AgentRunID: "cosuper-success", Handle: "grant-success"}: capability},
		revokedCaps:  map[string]bool{}, publicKey: publicKey,
	}
	incoming := t.TempDir()
	if err := os.Chmod(incoming, 0o700); err != nil {
		t.Fatal(err)
	}
	files, staged, err := executor.StageGrantedRelease("cosuper-success", "grant-success", incoming)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Path != "bin/sandbox" || staged == "" {
		t.Fatalf("staged release files=%+v path=%q", files, staged)
	}
	if content, err := os.ReadFile(filepath.Join(staged, "bin/sandbox")); err != nil || string(content) != "sandbox" {
		t.Fatalf("staged sandbox = %q, %v", content, err)
	}
}

func TestExtractGrantedFreezesBeforeDiff(t *testing.T) {
	upper := t.TempDir()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	capability := &Capability{
		CapabilityID: "cap-freeze", Handle: "grant-freeze", CapsuleID: "capsule-freeze",
		TargetCapsule: "capsule-freeze", AgentRunID: "cosuper-freeze", AgentRole: RoleCoSuper,
		Verbs: RoleVerbSets[RoleCoSuper], ExpiresAt: time.Now().Add(time.Hour),
	}
	if err := SignCapability(capability, privateKey, "test-key"); err != nil {
		t.Fatal(err)
	}
	caps := &Capsule{ID: "capsule-freeze", State: StateActive, UpperDir: upper, Cgroup: &testCapsuleCgroup{}}
	executor := &Executor{
		capsules:     map[string]*Capsule{"capsule-freeze": caps},
		capabilities: map[capKey]*Capability{{AgentRunID: "cosuper-freeze", Handle: "grant-freeze"}: capability},
		revokedCaps:  map[string]bool{}, publicKey: publicKey,
	}
	if _, _, err := executor.StageGrantedRelease("cosuper-freeze", "grant-freeze", t.TempDir()); err == nil || !strings.Contains(err.Error(), "requires frozen capsule") {
		t.Fatalf("active capsule stage error = %v", err)
	}
	if _, err := executor.ExtractGranted(context.Background(), "cosuper-freeze", "grant-freeze"); err != nil {
		t.Fatal(err)
	}
	if caps.State != StateFrozen {
		t.Fatalf("capsule state = %s, want %s", caps.State, StateFrozen)
	}
}

func TestStageGrantedReleaseRefusesSymlinkComponents(t *testing.T) {
	merged := t.TempDir()
	upper := t.TempDir()
	external := t.TempDir()
	if err := os.MkdirAll(filepath.Join(upper, "var/lib/artifact/release/bin"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(upper, "var/lib/artifact/release/bin/sandbox"), []byte("sandbox"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(merged, "var/lib/artifact/release"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(external, "sandbox"), []byte("outside"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(merged, "var/lib/artifact/release/bin")); err != nil {
		t.Fatal(err)
	}
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	capability := &Capability{
		CapabilityID: "cap-symlink", Handle: "grant-symlink", CapsuleID: "capsule-symlink",
		TargetCapsule: "capsule-symlink", AgentRunID: "cosuper-symlink", AgentRole: RoleCoSuper,
		Verbs: RoleVerbSets[RoleCoSuper], ExpiresAt: time.Now().Add(time.Hour),
	}
	if err := SignCapability(capability, privateKey, "test-key"); err != nil {
		t.Fatal(err)
	}
	executor := &Executor{
		capsules: map[string]*Capsule{"capsule-symlink": {
			ID: "capsule-symlink", State: StateFrozen, UpperDir: upper, MergedDir: merged, MemoryMax: 16 << 20,
		}},
		capabilities: map[capKey]*Capability{{AgentRunID: "cosuper-symlink", Handle: "grant-symlink"}: capability},
		revokedCaps:  map[string]bool{}, publicKey: publicKey,
	}
	incoming := t.TempDir()
	if err := os.Chmod(incoming, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, _, err := executor.StageGrantedRelease("cosuper-symlink", "grant-symlink", incoming); err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("symlink component stage error = %v", err)
	}
}

func TestQuiesceWaitsForBrokerExecution(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	capability := &Capability{
		CapabilityID: "cap-exec", Handle: "grant-exec", CapsuleID: "capsule-exec",
		TargetCapsule: "capsule-exec", AgentRunID: "cosuper-exec", AgentRole: RoleCoSuper,
		Verbs: RoleVerbSets[RoleCoSuper], ExpiresAt: time.Now().Add(time.Hour),
	}
	if err := SignCapability(capability, privateKey, "test-key"); err != nil {
		t.Fatal(err)
	}
	client, server := net.Pipe()
	t.Cleanup(func() {
		_ = client.Close()
		_ = server.Close()
	})
	cgroup := &testCapsuleCgroup{}
	caps := &Capsule{
		ID: "capsule-exec", State: StateActive, Cgroup: cgroup,
		broker: &BrokerClient{conn: client, publicKey: publicKey}, revokedCaps: map[string]bool{},
	}
	requestSeen := make(chan struct{})
	releaseResponse := make(chan struct{})
	serverErr := make(chan error, 1)
	go func() {
		var request BrokerRPCRequest
		if err := json.NewDecoder(server).Decode(&request); err != nil {
			serverErr <- err
			return
		}
		close(requestSeen)
		<-releaseResponse
		serverErr <- json.NewEncoder(server).Encode(BrokerRPCResponse{Result: json.RawMessage(`{"exit_code":0}`)})
	}()
	execDone := make(chan error, 1)
	go func() {
		_, err := caps.Exec(context.Background(), capability, ExecRequest{Command: "true"})
		execDone <- err
	}()
	<-requestSeen
	quiesceDone := make(chan error, 1)
	go func() { quiesceDone <- caps.Quiesce(context.Background()) }()
	select {
	case err := <-quiesceDone:
		t.Fatalf("quiesce completed before broker execution: %v", err)
	case <-time.After(25 * time.Millisecond):
	}
	close(releaseResponse)
	if err := <-serverErr; err != nil {
		t.Fatal(err)
	}
	if err := <-execDone; err != nil {
		t.Fatal(err)
	}
	if err := <-quiesceDone; err != nil {
		t.Fatal(err)
	}
	if caps.State != StateFrozen {
		t.Fatalf("capsule state = %s, want %s", caps.State, StateFrozen)
	}
	if !cgroup.frozen {
		t.Fatal("capsule cgroup was not frozen")
	}
	if err := caps.Thaw(context.Background()); err != nil {
		t.Fatal(err)
	}
	if caps.State != StateActive || cgroup.frozen {
		t.Fatalf("thawed capsule state=%s cgroup_frozen=%t", caps.State, cgroup.frozen)
	}
}

func TestQuiesceCancellationRestoresActiveAndUnlocksInflight(t *testing.T) {
	caps := &Capsule{ID: "capsule-cancel", State: StateActive, Cgroup: &testCapsuleCgroup{}}
	if err := caps.acquireOp(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := caps.Quiesce(ctx); err == nil {
		t.Fatal("canceled quiesce succeeded")
	}
	if caps.State != StateActive {
		t.Fatalf("capsule state = %s, want %s", caps.State, StateActive)
	}
	released := make(chan struct{})
	go func() {
		caps.releaseOp()
		close(released)
	}()
	select {
	case <-released:
	case <-time.After(time.Second):
		t.Fatal("releaseOp remained deadlocked after canceled quiesce")
	}
	if err := caps.acquireOp(); err != nil {
		t.Fatalf("operation admission did not recover: %v", err)
	}
	caps.releaseOp()
}

func TestFrozenCapsuleRefusesEveryBrokerOperation(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	capability := &Capability{
		CapabilityID: "cap-frozen", Handle: "grant-frozen", CapsuleID: "capsule-frozen",
		TargetCapsule: "capsule-frozen", AgentRunID: "cosuper-frozen", AgentRole: RoleCoSuper,
		Verbs: RoleVerbSets[RoleCoSuper], ExpiresAt: time.Now().Add(time.Hour),
	}
	if err := SignCapability(capability, privateKey, "test-key"); err != nil {
		t.Fatal(err)
	}
	executor := &Executor{
		capsules: map[string]*Capsule{"capsule-frozen": {
			ID: "capsule-frozen", State: StateFrozen, Cgroup: &testCapsuleCgroup{frozen: true},
		}},
		capabilities: map[capKey]*Capability{{AgentRunID: "cosuper-frozen", Handle: "grant-frozen"}: capability},
		revokedCaps:  map[string]bool{}, publicKey: publicKey,
	}
	if _, err := executor.ReadFile(context.Background(), "cosuper-frozen", "grant-frozen", "file"); err == nil {
		t.Fatal("ReadFile admitted after freeze")
	}
	if err := executor.WriteFile(context.Background(), "cosuper-frozen", "grant-frozen", "file", []byte("changed"), 0o600); err == nil {
		t.Fatal("WriteFile admitted after freeze")
	}
	if _, err := executor.ListDir(context.Background(), "cosuper-frozen", "grant-frozen", "."); err == nil {
		t.Fatal("ListDir admitted after freeze")
	}
}

func TestAmbiguousFreezerTransitionsRemainFailClosed(t *testing.T) {
	freezeCgroup := &testCapsuleCgroup{freezeErr: errors.New("freeze event unavailable")}
	freezing := &Capsule{ID: "capsule-freeze-error", State: StateActive, Cgroup: freezeCgroup}
	if err := freezing.Quiesce(context.Background()); err == nil {
		t.Fatal("ambiguous freeze succeeded")
	}
	if freezing.State != StateQuiescing || !freezeCgroup.frozen {
		t.Fatalf("failed freeze state=%s cgroup_frozen=%t", freezing.State, freezeCgroup.frozen)
	}
	if err := freezing.acquireOp(); err == nil {
		t.Fatal("operation admitted after ambiguous freeze")
	}

	thawCgroup := &testCapsuleCgroup{frozen: true, thawErr: errors.New("thaw event unavailable")}
	thawing := &Capsule{ID: "capsule-thaw-error", State: StateFrozen, Cgroup: thawCgroup}
	if err := thawing.Thaw(context.Background()); err == nil {
		t.Fatal("ambiguous thaw succeeded")
	}
	if thawing.State != StateQuiescing || thawCgroup.frozen {
		t.Fatalf("failed thaw state=%s cgroup_frozen=%t", thawing.State, thawCgroup.frozen)
	}
	if err := thawing.acquireOp(); err == nil {
		t.Fatal("operation admitted after ambiguous thaw")
	}
}

func TestExecutionReceiptValidationRequiresFrozenCapsule(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	capability := &Capability{
		CapabilityID: "cap-receipt", Handle: "grant-receipt", CapsuleID: "capsule-receipt",
		TargetCapsule: "capsule-receipt", AgentRunID: "cosuper-receipt", AgentRole: RoleCoSuper,
		Verbs: RoleVerbSets[RoleCoSuper], ExpiresAt: time.Now().Add(time.Hour),
	}
	if err := SignCapability(capability, privateKey, "test-key"); err != nil {
		t.Fatal(err)
	}
	executor := &Executor{
		capsules: map[string]*Capsule{"capsule-receipt": {
			ID: "capsule-receipt", State: StateActive, Cgroup: &testCapsuleCgroup{},
		}},
		capabilities: map[capKey]*Capability{{AgentRunID: "cosuper-receipt", Handle: "grant-receipt"}: capability},
		revokedCaps:  map[string]bool{}, publicKey: publicKey,
	}
	if _, err := executor.ResolveGrantedExecutionReceipts(context.Background(), "cosuper-receipt", "grant-receipt", []string{"receipt"}); err == nil || !strings.Contains(err.Error(), "requires frozen capsule") {
		t.Fatalf("active receipt validation error = %v", err)
	}
}

func TestExtractGrantedPropagatesCancellation(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	capability := &Capability{
		CapabilityID: "cap-cancel", Handle: "grant-cancel", CapsuleID: "capsule-cancel",
		TargetCapsule: "capsule-cancel", AgentRunID: "cosuper-cancel", AgentRole: RoleCoSuper,
		Verbs: RoleVerbSets[RoleCoSuper], ExpiresAt: time.Now().Add(time.Hour),
	}
	if err := SignCapability(capability, privateKey, "test-key"); err != nil {
		t.Fatal(err)
	}
	caps := &Capsule{ID: "capsule-cancel", State: StateActive, Cgroup: &testCapsuleCgroup{}}
	if err := caps.acquireOp(); err != nil {
		t.Fatal(err)
	}
	defer caps.releaseOp()
	executor := &Executor{
		capsules:     map[string]*Capsule{"capsule-cancel": caps},
		capabilities: map[capKey]*Capability{{AgentRunID: "cosuper-cancel", Handle: "grant-cancel"}: capability},
		revokedCaps:  map[string]bool{}, publicKey: publicKey,
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := executor.ExtractGranted(ctx, "cosuper-cancel", "grant-cancel"); err == nil {
		t.Fatal("canceled extraction succeeded")
	}
	if caps.State != StateActive {
		t.Fatalf("canceled extraction state = %s, want %s", caps.State, StateActive)
	}
}
