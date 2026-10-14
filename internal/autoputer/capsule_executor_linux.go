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
	memoryTotal := int64(0)
	if raw := strings.TrimSpace(os.Getenv("CHOIR_CAPSULE_MEMORY_TOTAL_BYTES")); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed <= 0 {
			return nil, false, fmt.Errorf("CHOIR_CAPSULE_MEMORY_TOTAL_BYTES must be positive")
		}
		memoryTotal = parsed
	} else if raw, err := os.ReadFile("/proc/meminfo"); err == nil {
		for _, line := range strings.Split(string(raw), "\n") {
			fields := strings.Fields(line)
			if len(fields) >= 2 && fields[0] == "MemTotal:" {
				kb, _ := strconv.ParseInt(fields[1], 10, 64)
				memoryTotal = kb * 1024 * 3 / 4
				break
			}
		}
	}
	if memoryTotal < int64(1<<30) {
		return nil, false, fmt.Errorf("capsule executor memory admission budget is unavailable")
	}
	executor := capsule.NewExecutorWithArtifacts(state, artifact, lower, source, broker, memoryTotal)
	if err := executor.InitializationError(); err != nil {
		return nil, false, err
	}
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
	return executor, true, nil
}
