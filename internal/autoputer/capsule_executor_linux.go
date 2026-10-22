//go:build linux

package autoputer

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/capsule"
)

func configuredCapsuleExecutor() (*capsule.Executor, bool, error) {
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
	// so a memory lease that plugged memory raises it (job_memory.go; F18
	// in docs/reports/memory-hotplug-controller-design-2026-10-14.md). This
	// boot reading is the floor check and the fallback.
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
	// Capsule memory per doctrine (RES-04) is a policy: observe keeps the
	// MemoryMax shape and records what doctrine would have done.
	memoryPolicy := capsule.CapsuleMemoryPolicyFromEnv(nil)
	executor.SetMemoryPolicy(memoryPolicy)
	log.Printf("capsule executor: memory policy %s", memoryPolicy)
	// Egress tunnel identity (SDC SEC-06, internal/capsule/egress_tls.go):
	// observe records what binding a CONNECT tunnel to its name would refuse
	// and relays it; enforce refuses.
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
	// Claude delegate (docs/design/claude-delegate-slice1-2026-10-14.md):
	// the grant and token are on the credential disk only for the
	// allowlisted computer; elsewhere the status reads "not allowlisted".
	delegateDir := strings.TrimSpace(os.Getenv("CHOIR_CLAUDE_DELEGATE_CREDENTIAL_DIR"))
	if delegateDir == "" {
		delegateDir = capsule.DefaultClaudeDelegateCredentialDir
	}
	executor.SetClaudeDelegateSource(delegateDir, os.Getenv(capsule.ClaudeDelegateBrokerBinEnv))
	if status := executor.ClaudeDelegateStatus(); status.Available {
		log.Printf("capsule executor: claude delegate %s", status.Reason)
	}
	// Visual capture (docs/design/visual-evidence-slice-2026-10-15.md): the
	// grant is on the credential disk only for computers the host granted.
	captureDir := strings.TrimSpace(os.Getenv("CHOIR_VISUAL_CAPTURE_CREDENTIAL_DIR"))
	if captureDir == "" {
		captureDir = capsule.DefaultClaudeDelegateCredentialDir
	}
	executor.SetVisualCaptureSource(captureDir, os.Getenv("CHOIR_CAPTURE_BROWSER_BIN"), os.Getenv("CHOIR_CAPTURE_FONTCONFIG_FILE"))
	log.Printf("capsule executor: visual capture: %s", executor.VisualCaptureStatus().Reason)
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
		mirrorDir = capsule.DefaultClaudeDelegateCredentialDir
	}
	executor.SetCapsuleMirrors(mirrorAddr, mirrorDir)
	log.Printf("capsule executor: package mirrors: %s", executor.CapsuleMirrorStatus().Reason)
	return executor, true, nil
}
