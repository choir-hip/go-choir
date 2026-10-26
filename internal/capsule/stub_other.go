//go:build !linux

package capsule

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/steptrace"
	"github.com/yusefmosiah/go-choir/internal/visualcapture"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

func stubErr(operation string) error {
	return fmt.Errorf("capsule: %s requires the Linux guest kernel", operation)
}

type Executor struct{}

func NewExecutor(stateDir, lowerDir, brokerPath string, vmMemoryTotal int64) *Executor {
	return &Executor{}
}

// ErrSubjectArtifactUnavailable marks a durable capsule artifact ref whose
// bytes are permanently absent or corrupt. Mirrors the linux implementation.
var ErrSubjectArtifactUnavailable = errors.New("capsule subject artifact unavailable or corrupt")

func NewExecutorWithSource(stateDir, lowerDir, sourceDir, brokerPath string, vmMemoryTotal int64) *Executor {
	return &Executor{}
}
func NewExecutorWithArtifacts(stateDir, artifactDir, lowerDir, sourceDir, brokerPath string, vmMemoryTotal int64) *Executor {
	return &Executor{}
}
func (e *Executor) InitializationError() error { return stubErr("initialize") }

// SetBinaryReuseGate, SetBinaryReuseByInputDiff and SourceDir mirror the linux
// executor.
func (e *Executor) SetBinaryReuseGate(func() bool) {}
func (e *Executor) SetBinaryReuseByInputDiff(bool) {}
func (e *Executor) SourceDir() string              { return "" }
func (e *Executor) PreflightSourceSnapshot(context.Context, string) (SourcePreflight, error) {
	return SourcePreflight{}, stubErr("preflight source")
}
func (e *Executor) Spawn(context.Context, SpawnSpec) (*Capsule, error) { return nil, stubErr("spawn") }
func (e *Executor) Destroy(context.Context, string) error              { return stubErr("destroy") }
func (e *Executor) ForceDestroy(context.Context, string) error         { return stubErr("destroy") }
func (e *Executor) GrantEngineering(string, string, string, time.Duration) (string, error) {
	return "", stubErr("grant")
}
func (e *Executor) ThawGranted(context.Context, string, string) error {
	return stubErr("thaw")
}
func (e *Executor) ExtractOwned(string, string) ([]FileChange, error) { return nil, stubErr("diff") }
func (e *Executor) ResolveOwnedCapsuleID(string, string) (string, error) {
	return "", stubErr("resolve")
}
func (e *Executor) ExtractGranted(context.Context, string, string) ([]FileChange, error) {
	return nil, stubErr("diff")
}
func (e *Executor) InspectGrantedWorktree(context.Context, string, string) (GrantedWorktree, error) {
	return GrantedWorktree{}, stubErr("inspect worktree")
}
func (e *Executor) ResolveGrantedCapsuleID(string, string) (string, error) {
	return "", stubErr("resolve")
}
func (e *Executor) ResolveGrantedWorktreeDigest(context.Context, string, string) (string, error) {
	return "", stubErr("resolve")
}
func (e *Executor) PersistGrantedCandidate(context.Context, string, string) (SourcePreflight, error) {
	return SourcePreflight{}, stubErr("persist candidate")
}
func (e *Executor) DeriveCarriedPatch(context.Context, string, string) (CarriedPatch, error) {
	return CarriedPatch{}, stubErr("derive carried patch")
}

// ErrCapsuleNotSnapshottable mirrors the linux implementation.
var ErrCapsuleNotSnapshottable = errors.New("capsule cannot be snapshotted")

func (e *Executor) SnapshotUnfinishedSourcePatch(context.Context, string) (CarriedPatch, error) {
	return CarriedPatch{}, stubErr("snapshot unfinished source patch")
}
func (e *Executor) CheckCarriedPatchApplies(context.Context, CarriedPatch, string) error {
	return stubErr("check carried patch")
}
func (e *Executor) ResolveGrantedSourceSnapshotDigest(string, string) (string, error) {
	return "", stubErr("resolve")
}
func (e *Executor) ResolveGrantedFreezeBindings(string, string) (string, string, error) {
	return "", "", stubErr("resolve")
}
func (e *Executor) StageGrantedRelease(context.Context, string, string, string) (*StagedRelease, error) {
	return nil, stubErr("stage")
}
func (e *Executor) MintCapability(string, AgentRole, string, time.Duration) (*Capability, error) {
	return nil, stubErr("mint")
}
func (e *Executor) MintCapabilityHandle(string, AgentRole, string, string, time.Duration, string) (*Capability, error) {
	return nil, stubErr("mint")
}
func (e *Executor) AssignmentHandle(string, string) (string, error) {
	return "", stubErr("resolve")
}
func (e *Executor) ResolveCapability(string, string) (*Capability, error) {
	return nil, stubErr("resolve")
}
func (e *Executor) RevokeCapability(string, string) error       { return stubErr("revoke") }
func (e *Executor) ResolveTarget(*Capability) ([]string, error) { return nil, stubErr("resolve") }
func (e *Executor) Exec(context.Context, string, string, ExecRequest) (ExecResult, error) {
	return ExecResult{}, stubErr("exec")
}
func (e *Executor) GoEval(context.Context, string, string, GoEvalRequest) (GoEvalResult, error) {
	return GoEvalResult{}, stubErr("go_eval")
}
func (e *Executor) PersistGrantedFreezeReceipt(context.Context, string, string) (CapsuleFateReceipt, error) {
	return CapsuleFateReceipt{}, stubErr("freeze receipt")
}
func (e *Executor) OpenCapsuleFateReceipt(string) (CapsuleFateReceipt, error) {
	return CapsuleFateReceipt{}, stubErr("freeze receipt")
}
func (e *Executor) OpenExecutionReceipt(ref string) (ExecutionReceipt, error) {
	ref = strings.TrimSpace(ref)
	if strings.HasPrefix(ref, "rlm:") {
		return ExecutionReceipt{}, fmt.Errorf("receipt reference %q is an internal intent token, not an execution receipt (expected capsule-go-eval:sha256:* or capsule-exec:sha256:*)", ref)
	}
	if !strings.HasPrefix(ref, "capsule-exec:sha256:") && !strings.HasPrefix(ref, "capsule-go-eval:sha256:") {
		return ExecutionReceipt{}, fmt.Errorf("executor receipt %q is invalid: unsupported prefix (expected capsule-go-eval:sha256:* or capsule-exec:sha256:*)", ref)
	}
	return ExecutionReceipt{}, stubErr("execution receipt")
}
func (e *Executor) OpenGrantedExecutionReceipt(string) (GrantedExecutionReceipt, error) {
	return GrantedExecutionReceipt{}, stubErr("execution receipt")
}
func (e *Executor) ResolveGrantedExecutionReceipts(context.Context, string, string, []string) ([]ExecutionReceipt, error) {
	return nil, stubErr("execution receipts")
}
func (e *Executor) ResolveGrantedExecutionReceiptsFor(context.Context, string, string, []string, SubjectBinding) ([]ExecutionReceipt, error) {
	return nil, stubErr("execution receipts")
}
func (e *Executor) PreflightGrantedExecutionReceipts(context.Context, string, string, []string, SubjectBinding) error {
	return stubErr("execution receipts")
}
func (e *Executor) SetStepTrace(*steptrace.Tracker) {}
func (e *Executor) ResolveExecutionReceipts(refs []string) ([]ExecutionReceipt, error) {
	receipts := make([]ExecutionReceipt, 0, len(refs))
	for _, ref := range refs {
		receipt, err := e.OpenExecutionReceipt(ref)
		if err != nil {
			return nil, err
		}
		receipts = append(receipts, receipt)
	}
	return receipts, nil
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
func (e *Executor) CleanupOrphanedCapsule(context.Context, string) error {
	return stubErr("cleanup orphaned capsule")
}
func (e *Executor) PersistRevocationReceipt(string, string, string, string) (CapsuleRevocationReceipt, error) {
	return CapsuleRevocationReceipt{}, stubErr("persist revocation receipt")
}

func (e *Executor) HasCapsule(string) bool { return false }

// Capsule jobs (jobs.go) need the Linux guest.
func (e *Executor) SetJobCompletionHandler(func(JobCompletion)) {}
func (e *Executor) SetJobAdmission(JobAdmission)                {}
func (e *Executor) SetLiveMemoryBudget(func() int64)            {}
func (e *Executor) SetMemoryPolicy(CapsuleMemoryPolicy)         {}
func (e *Executor) SetEgressTunnelIdentity(TunnelIdentityMode)  {}
func (e *Executor) MemoryReceipts() []CapsuleMemoryReceipt      { return nil }
func (e *Executor) StartJob(context.Context, string, string, JobRequest) (yaegikernel.JobView, error) {
	return yaegikernel.JobView{}, stubErr("start job")
}
func (e *Executor) StopJob(context.Context, string, string, string) (yaegikernel.JobView, error) {
	return yaegikernel.JobView{}, stubErr("stop job")
}
func (e *Executor) RunningJob(string) (yaegikernel.JobView, time.Time, bool) {
	return yaegikernel.JobView{}, time.Time{}, false
}
func (e *Executor) JobViews(string) []yaegikernel.JobView { return nil }
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
func (c *Capsule) GoEval(context.Context, *Capability, GoEvalRequest) (GoEvalResult, error) {
	return GoEvalResult{}, stubErr("go_eval")
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
func (b *BrokerClient) GoEval(context.Context, *Capability, GoEvalRequest) (GoEvalResult, error) {
	return GoEvalResult{}, stubErr("go_eval")
}

func LoadWorkloadFilter() error       { return stubErr("seccomp") }
func LoadBrokerFilter() error         { return stubErr("seccomp") }
func LoadWorkloadInetFilter() error   { return stubErr("seccomp") }
func LoadBrokerInetFilter() error     { return stubErr("seccomp") }
func LoadDelegateFilter() error       { return stubErr("seccomp") }
func LoadBrokerDelegateFilter() error { return stubErr("seccomp") }

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

// Claude delegate (claude_delegate.go): never available off the guest kernel.
func (e *Executor) SetClaudeDelegateSource(dir, binary string) {}
func (e *Executor) ClaudeDelegateStatus() ClaudeDelegateStatus {
	return ClaudeDelegateStatus{Reason: "the Claude delegate requires the Linux guest kernel"}
}
func (e *Executor) MemoryBudget() int64 { return 0 }

// Visual capture (visual_capture_linux.go): never granted off the guest
// kernel.
func (e *Executor) SetVisualCaptureSource(dir, browser, fontconfig string) {}
func (e *Executor) VisualCaptureStatus() visualcapture.Status {
	return visualcapture.Status{Reason: "visual capture requires the Linux guest kernel"}
}
