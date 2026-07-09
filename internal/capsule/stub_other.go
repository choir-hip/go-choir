//go:build !linux

package capsule

import (
	"context"
	"fmt"
	"time"
)

// Stub implementations for non-Linux platforms (development only).
// The capsule runtime requires Linux for namespaces, cgroups, seccomp,
// vsock, and overlayfs. These stubs allow the package to compile on
// macOS for development without providing any functionality.

func stubErr(msg string) error {
	return fmt.Errorf("capsule: %s (not supported on this platform)", msg)
}

// HostClient stub
type HostClient struct{}

func NewHostClient(hostCID, port uint32) *HostClient { return &HostClient{} }
func (c *HostClient) Connect(ctx context.Context) error { return stubErr("vsock") }
func (c *HostClient) Close() error { return nil }
func (c *HostClient) MintCapability(ctx context.Context, agentRunID string, role AgentRole, capsuleID string, ttl time.Duration) (*Capability, error) {
	return nil, stubErr("vsock")
}
func (c *HostClient) RevokeCapability(ctx context.Context, agentRunID, capsuleID, capabilityID string) error {
	return stubErr("vsock")
}
func (c *HostClient) GetRevokedCaps(ctx context.Context, capsuleID string) ([]string, error) {
	return nil, stubErr("vsock")
}
func (c *HostClient) RegisterCapsule(ctx context.Context, capsuleID string) error { return stubErr("vsock") }
func (c *HostClient) UnregisterCapsule(ctx context.Context, capsuleID string) error { return stubErr("vsock") }
func (c *HostClient) RegisterActiveRun(ctx context.Context, agentRunID string) error { return stubErr("vsock") }
func (c *HostClient) UnregisterActiveRun(ctx context.Context, agentRunID string) error { return stubErr("vsock") }

// Executor stub
type Executor struct{}

func NewExecutor(stateDir, erofsMount, brokerStore string, vmMemoryTotal int64, hostClient *HostClient) *Executor {
	return &Executor{}
}
func (e *Executor) Spawn(ctx context.Context, spec SpawnSpec) (*Capsule, error) {
	return nil, stubErr("spawn")
}
func (e *Executor) Destroy(ctx context.Context, id string) error { return stubErr("destroy") }
func (e *Executor) ForceDestroy(ctx context.Context, id string) error { return stubErr("destroy") }
func (e *Executor) MintCapability(agentRunID string, role AgentRole, capsuleID string, ttl time.Duration) (*Capability, error) {
	return nil, stubErr("mint")
}
func (e *Executor) ResolveCapability(agentRunID, handle string) (*Capability, error) {
	return nil, stubErr("resolve")
}
func (e *Executor) RevokeCapability(agentRunID, handle string) error { return stubErr("revoke") }
func (e *Executor) ResolveTarget(cap *Capability) ([]string, error) { return nil, stubErr("resolve") }
func (e *Executor) InspectCapsuleRaw(id string) (*CapsuleDiagnostics, error) {
	return nil, stubErr("inspect")
}
func (e *Executor) ExtractDiff(id string) ([]FileChange, error) { return nil, stubErr("diff") }
func (e *Executor) ListCapsules() []CapsuleSummary { return nil }
func (e *Executor) RestartBroker(id string) error { return stubErr("restart") }
func (e *Executor) SyncRevokedCaps(revokedIDs []string) {}

// Capsule stub
type Capsule struct {
	ID    string
	State CapsuleState
}

func (c *Capsule) Exec(ctx context.Context, cap *Capability, req ExecRequest) (ExecResult, error) {
	return ExecResult{}, stubErr("exec")
}
func (c *Capsule) Quiesce(ctx context.Context) error { return stubErr("quiesce") }
func (c *Capsule) Thaw(ctx context.Context) error { return stubErr("thaw") }
func (c *Capsule) Diff(ctx context.Context) ([]FileChange, error) { return nil, stubErr("diff") }
func (c *Capsule) CommitManifest(ctx context.Context) error { return stubErr("commit") }
func (c *Capsule) Destroy(ctx context.Context) error { return stubErr("destroy") }
func (c *Capsule) UpdateRevokedCaps(revokedIDs []string) {}
func (c *Capsule) IsPinned() bool { return false }

// BrokerClient stub
type BrokerClient struct{}

func NewBrokerClient(socketPath string, publicKey interface{}) *BrokerClient { return &BrokerClient{} }
func (b *BrokerClient) Connect(ctx context.Context) error { return stubErr("broker") }
func (b *BrokerClient) Close() error { return nil }

// Seccomp stubs
func LoadWorkloadFilter() error { return stubErr("seccomp") }
func LoadBrokerFilter() error { return stubErr("seccomp") }

// Landlock stub
type LandlockRestrictor struct{}

func NewBrokerLandlock(mergedDir, brokerStore string) *LandlockRestrictor { return &LandlockRestrictor{} }
func NewWorkloadLandlock(mergedDir string) *LandlockRestrictor { return &LandlockRestrictor{} }
func (r *LandlockRestrictor) Apply() error { return stubErr("landlock") }

// Capability dropping stubs
func DropBrokerCapabilities() error { return stubErr("capabilities") }
func DropWorkloadCapabilities() error { return stubErr("capabilities") }

// FileInfo stub (for os.FileInfo interface on non-Linux)
type FileInfo struct {
	FiName    string `json:"name"`
	FiSize    int64  `json:"size"`
	FiMode    uint32 `json:"mode"`
	FiIsDir   bool   `json:"is_dir"`
	FiModTime int64  `json:"mod_time_unix"`
}
