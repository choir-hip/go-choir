//go:build !linux

package capsule

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"os"
	"time"
)

func stubErr(operation string) error {
	return fmt.Errorf("capsule: %s requires the Linux guest kernel", operation)
}

type Executor struct{}

func NewExecutor(stateDir, lowerDir, brokerPath string, vmMemoryTotal int64) *Executor {
	return &Executor{}
}
func NewExecutorWithSource(stateDir, lowerDir, sourceDir, brokerPath string, vmMemoryTotal int64) *Executor {
	return &Executor{}
}
func (e *Executor) Spawn(context.Context, SpawnSpec) (*Capsule, error) { return nil, stubErr("spawn") }
func (e *Executor) Destroy(context.Context, string) error              { return stubErr("destroy") }
func (e *Executor) ForceDestroy(context.Context, string) error         { return stubErr("destroy") }
func (e *Executor) ControlHandle(string, string) (string, error)       { return "", stubErr("control") }
func (e *Executor) GrantCoSuper(string, string, string, time.Duration) (string, error) {
	return "", stubErr("grant")
}
func (e *Executor) DestroyOwned(context.Context, string, string, bool) error {
	return stubErr("destroy")
}
func (e *Executor) InspectOwned(string, string) (CapsuleControlSummary, error) {
	return CapsuleControlSummary{}, stubErr("inspect")
}
func (e *Executor) ExtractOwned(string, string) ([]FileChange, error) { return nil, stubErr("diff") }
func (e *Executor) ResolveOwnedCapsuleID(string, string) (string, error) {
	return "", stubErr("resolve")
}
func (e *Executor) ExtractGranted(string, string) ([]FileChange, error) {
	return nil, stubErr("diff")
}
func (e *Executor) ResolveGrantedCapsuleID(string, string) (string, error) {
	return "", stubErr("resolve")
}
func (e *Executor) ResolveGrantedSourceSnapshotDigest(string, string) (string, error) {
	return "", stubErr("resolve")
}
func (e *Executor) ResolveGrantedFreezeBindings(string, string) (string, string, error) {
	return "", "", stubErr("resolve")
}
func (e *Executor) StageGrantedRelease(string, string, string) ([]FrozenReleaseFile, string, error) {
	return nil, "", stubErr("stage")
}
func (e *Executor) ListOwned(string) []CapsuleControlSummary { return nil }
func (e *Executor) MintCapability(string, AgentRole, string, time.Duration) (*Capability, error) {
	return nil, stubErr("mint")
}
func (e *Executor) ResolveCapability(string, string) (*Capability, error) {
	return nil, stubErr("resolve")
}
func (e *Executor) RevokeCapability(string, string) error       { return stubErr("revoke") }
func (e *Executor) ResolveTarget(*Capability) ([]string, error) { return nil, stubErr("resolve") }
func (e *Executor) Exec(context.Context, string, string, ExecRequest) (ExecResult, error) {
	return ExecResult{}, stubErr("exec")
}
func (e *Executor) ResolveGrantedExecutionReceipts(context.Context, string, string, []string) ([]ExecutionReceipt, error) {
	return nil, stubErr("execution receipts")
}
func (e *Executor) ResolveExecutionReceipts([]string) ([]ExecutionReceipt, error) {
	return nil, stubErr("execution receipts")
}
func (e *Executor) ReadFile(context.Context, string, string, string) ([]byte, error) {
	return nil, stubErr("read")
}
func (e *Executor) WriteFile(context.Context, string, string, string, []byte, uint32) error {
	return stubErr("write")
}
func (e *Executor) ListDir(context.Context, string, string, string) ([]string, error) {
	return nil, stubErr("list")
}
func (e *Executor) InspectCapsuleRaw(string) (*CapsuleDiagnostics, error) {
	return nil, stubErr("inspect")
}
func (e *Executor) ExtractDiff(string) ([]FileChange, error) { return nil, stubErr("diff") }
func (e *Executor) ListCapsules() []CapsuleSummary           { return nil }

type Capsule struct {
	ID                   string
	State                CapsuleState
	SourceSnapshotDigest string
}

func (c *Capsule) Exec(context.Context, *Capability, ExecRequest) (ExecResult, error) {
	return ExecResult{}, stubErr("exec")
}
func (c *Capsule) Quiesce(context.Context) error              { return stubErr("quiesce") }
func (c *Capsule) Thaw(context.Context) error                 { return stubErr("thaw") }
func (c *Capsule) Diff(context.Context) ([]FileChange, error) { return nil, stubErr("diff") }
func (c *Capsule) CommitManifest(context.Context) error       { return stubErr("commit") }
func (c *Capsule) Destroy(context.Context) error              { return stubErr("destroy") }
func (c *Capsule) UpdateRevokedCaps([]string)                 {}
func (c *Capsule) IsPinned() bool                             { return false }

type BrokerClient struct{}

func NewBrokerClient(string, ed25519.PublicKey) *BrokerClient { return &BrokerClient{} }
func (b *BrokerClient) Connect(context.Context) error         { return stubErr("broker") }
func (b *BrokerClient) Close() error                          { return nil }

func LoadWorkloadFilter() error { return stubErr("seccomp") }
func LoadBrokerFilter() error   { return stubErr("seccomp") }

type LandlockRestrictor struct{}

func NewBrokerLandlock(string, string) *LandlockRestrictor { return &LandlockRestrictor{} }
func NewWorkloadLandlock(string) *LandlockRestrictor       { return &LandlockRestrictor{} }
func (r *LandlockRestrictor) Apply() error                 { return stubErr("landlock") }
func DropBrokerCapabilities() error                        { return stubErr("capabilities") }
func DropWorkloadCapabilities() error                      { return stubErr("capabilities") }

type FileInfo struct {
	FiName    string `json:"name"`
	FiSize    int64  `json:"size"`
	FiMode    uint32 `json:"mode"`
	FiIsDir   bool   `json:"is_dir"`
	FiModTime int64  `json:"mod_time_unix"`
}

func (f *FileInfo) Name() string       { return f.FiName }
func (f *FileInfo) Size() int64        { return f.FiSize }
func (f *FileInfo) Mode() os.FileMode  { return os.FileMode(f.FiMode) }
func (f *FileInfo) ModTime() time.Time { return time.Unix(f.FiModTime, 0) }
func (f *FileInfo) IsDir() bool        { return f.FiIsDir }
func (f *FileInfo) Sys() any           { return nil }
