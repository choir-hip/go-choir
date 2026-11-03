//go:build linux

package autoputer

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/store"
)

func configuredCapsuleExecutor(db *store.Store) (*capsule.Executor, bool, error) {
	broker := strings.TrimSpace(os.Getenv("CHOIR_CAPSULE_BROKER_PATH"))
	state := strings.TrimSpace(os.Getenv("CHOIR_CAPSULE_STATE_DIR"))
	source := strings.TrimSpace(os.Getenv("CHOIR_CAPSULE_SOURCE_ROOT"))
	lower := strings.TrimSpace(os.Getenv("CHOIR_CAPSULE_LOWER_ROOT"))
	artifact := strings.TrimSpace(os.Getenv("CHOIR_CAPSULE_ARTIFACT_DIR"))
	configured := broker != "" || state != "" || source != "" || lower != "" || artifact != ""
	if !configured {
		return nil, false, nil
	}
	if broker == "" || state == "" || source == "" || lower == "" || artifact == "" {
		return nil, false, fmt.Errorf("capsule executor requires broker, state, source, lower, and artifact roots together")
	}
	if err := os.MkdirAll(state, 0o700); err != nil {
		return nil, false, fmt.Errorf("create capsule state directory: %w", err)
	}
	if err := os.Chmod(state, 0o700); err != nil {
		return nil, false, fmt.Errorf("secure capsule state directory: %w", err)
	}
	state, err := capsule.BootScopedStateDir(state)
	if err != nil {
		return nil, false, err
	}
	if err := os.MkdirAll(artifact, 0o700); err != nil {
		return nil, false, fmt.Errorf("create capsule artifact directory: %w", err)
	}
	if err := os.Chmod(artifact, 0o700); err != nil {
		return nil, false, fmt.Errorf("secure capsule artifact directory: %w", err)
	}
	// The capsule budget is 3/4 of MemTotal, read again at every admission
	// so the budget follows the guest's memory (job_memory.go). This boot
	// reading is the floor check and the fallback.
	memoryTotal := int64(0)
	staticBudget := false
	if raw := strings.TrimSpace(os.Getenv("CHOIR_CAPSULE_MEMORY_TOTAL_BYTES")); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed <= 0 {
			return nil, false, fmt.Errorf("CHOIR_CAPSULE_MEMORY_TOTAL_BYTES must be positive")
		}
		memoryTotal, staticBudget = parsed, true
	} else {
		memoryTotal = capsule.LiveMemoryBudget(nil)()
	}
	if memoryTotal < int64(1<<30) {
		return nil, false, fmt.Errorf("capsule executor memory admission budget is unavailable")
	}
	executor := capsule.NewExecutorWithArtifacts(state, artifact, lower, source, broker, memoryTotal)
	if err := executor.InitializationError(); err != nil {
		return nil, false, err
	}
	configureJobMemory(executor, staticBudget)
	// An implementation assignment's capsule snapshots the files root at
	// /computer/files (docs/design/capsule-overlay-computer-state-2026-11-03.md).
	executor.SetComputerFilesRoot(provideriface.ResolveFilesRoot(os.Getenv("AUTOPUTER_FILES_ROOT")), source)
	// and gets a private copy of the store at /computer/state.db.
	if db != nil {
		executor.SetComputerStore(capsuleStoreCopySource{db: db})
	}
	// U1 (docs/design/computer-update-architecture-rnd-2026-10-24.md, section
	// 4.6): the source projection runs on every computer (capsule overlay S4),
	// so Freeze reuses the running binary when no binary-input path differs
	// between the running binary's commit and the capsule source.
	executor.SetBinaryReuseByInputDiff(true)
	// Capsule memory per doctrine (RES-04) is a policy: observe keeps the
	// MemoryMax shape and records what doctrine would have done.
	memoryPolicy := capsule.CapsuleMemoryPolicyFromEnv(nil)
	executor.SetMemoryPolicy(memoryPolicy)
	log.Printf("capsule executor: memory policy %s", memoryPolicy)
	// Egress tunnel identity (SDC SEC-06, internal/capsule/egress_tls.go):
	// observe records what binding a CONNECT tunnel to its name would refuse
	// and relays it; enforce refuses. A computer the host selects
	// (VM_CAPSULE_TUNNEL_ENFORCE_ONLY) has enforce in its environment file.
	if warning := capsule.TunnelIdentityEnvWarning(nil); warning != "" {
		log.Printf("capsule executor: WARNING: %s", warning)
	}
	tunnelIdentity := capsule.TunnelIdentityModeFromEnv(nil)
	executor.SetEgressTunnelIdentity(tunnelIdentity)
	log.Printf("capsule executor: egress tunnel identity %s", tunnelIdentity)
	// L2 capsules reach the host Go module proxy through a fixed egress
	// route; the address is the guest's tap peer unless set explicitly
	// (docs/reports/capsule-go-module-proxy-design-2026-10-14.md).
	goProxy, ok, err := capsule.GoModuleProxyAddr(os.Getenv("CHOIR_CAPSULE_GOPROXY_ADDR"), os.Getenv("RUNTIME_GATEWAY_URL"))
	if err != nil {
		// Not fatal: only capsule Go module fetches are lost, and they fail
		// with a reason that names the missing proxy.
		log.Printf("capsule executor: no Go module proxy route: %v", err)
	} else if ok {
		executor.SetGoModuleProxy(goProxy)
	}
	// Claude delegate (docs/problems/claude-delegate-token-on-three-computers-2026-11-03.md):
	// the gateway holds the credential and answers this computer's grant;
	// a claude-mode capsule reaches the gateway's relay through its egress
	// proxy, which adds this computer's gateway bearer. No credential is on
	// this computer's disk for it.
	executor.SetClaudeDelegateGateway(os.Getenv("RUNTIME_GATEWAY_URL"), provideriface.GatewayToken, os.Getenv(capsule.ClaudeDelegateBrokerBinEnv))
	log.Printf("capsule executor: claude delegate grant is read from the gateway")
	// The store Chromium every computer's jobs may launch (Playwright), and
	// the fontconfig file it renders text with. No grant.
	executor.SetBrowser(os.Getenv("CHOIR_BROWSER_BIN"), os.Getenv("CHOIR_FONTCONFIG_FILE"))
	// Package mirrors (station FP10): L2 capsules use the host package
	// mirror only when the host put a grant on the credential disk; the
	// route address is the guest's tap peer unless set explicitly.
	// A zero address (none configured, or an error) leaves the grant out of force.
	mirrorAddr, _, err := capsule.MirrorAddr(os.Getenv("CHOIR_CAPSULE_MIRROR_ADDR"), os.Getenv("RUNTIME_GATEWAY_URL"))
	if err != nil {
		log.Printf("capsule executor: no package mirror route: %v", err)
	}
	mirrorDir := strings.TrimSpace(os.Getenv("CHOIR_CAPSULE_MIRROR_CREDENTIAL_DIR"))
	if mirrorDir == "" {
		mirrorDir = capsule.DefaultCredentialDir
	}
	executor.SetCapsuleMirrors(mirrorAddr, mirrorDir)
	log.Printf("capsule executor: package mirrors: %s", executor.CapsuleMirrorStatus().Reason)
	// Heavy capsule work (S4): build capsules reserve their peak on the
	// host through vmctl, and build jobs read the host facts it returns.
	if hostMemory := configuredCapsuleHostMemory(); hostMemory != nil {
		executor.SetHostMemory(hostMemory)
		go hostMemory.run(context.Background())
		log.Printf("capsule executor: build capsules reserve %d MiB on the host through vmctl", capsule.BuildPeakBytes>>20)
	} else {
		log.Printf("capsule executor: no vmctl route; build capsules reserve nothing on the host")
	}
	return executor, true, nil
}

// capsuleStoreCopySource gives the capsule executor the store copy and
// nothing else of the store: a file in the capsule's upper, never a handle.
type capsuleStoreCopySource struct{ db *store.Store }

func (s capsuleStoreCopySource) SnapshotEstimate(ctx context.Context) (int64, error) {
	return s.db.SnapshotEstimate(ctx)
}

func (s capsuleStoreCopySource) SnapshotForCapsule(ctx context.Context, dir string) (capsule.ComputerStoreCopy, error) {
	r, err := s.db.SnapshotTo(ctx, dir)
	return capsule.ComputerStoreCopy{ComputerID: r.ComputerID, Sequence: r.Sequence, Head: r.Head, Bytes: r.Bytes,
		CopyMS: r.CopyMS, ScanMS: r.ScanMS, ScannedBytes: r.ScannedBytes, Blanked: r.Blanked, LeftOut: r.LeftOut}, err
}
