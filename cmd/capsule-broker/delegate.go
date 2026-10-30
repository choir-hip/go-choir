//go:build linux

package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/claudedelegate"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// Claude delegate, broker side (docs/design/claude-delegate-slice1-2026-10-14.md).
// The executor puts the token in a claude-mode broker's environment. The
// broker takes it out at once, keeps it in memory, and gives it only to the
// claude process it starts through the delegate-claude stage below. Cells
// reach the delegate only through the socket (cmd/choir-delegate), which
// can start and stop a run but never read the token back.

// delegateStage is the isolation stage that becomes the claude process.
const delegateStage = "delegate-claude"

// takeDelegateEnv reads the delegate token and binary from the broker's
// environment and removes both, so no later child built from os.Environ()
// could inherit them. Session workers and direct exec build their
// environments from allowlists in any case.
func takeDelegateEnv() (token, binary string) {
	token = strings.TrimSpace(os.Getenv(capsule.ClaudeDelegateBrokerTokenEnv))
	binary = strings.TrimSpace(os.Getenv(capsule.ClaudeDelegateBrokerBinEnv))
	_ = os.Unsetenv(capsule.ClaudeDelegateBrokerTokenEnv)
	_ = os.Unsetenv(capsule.ClaudeDelegateBrokerBinEnv)
	return token, binary
}

// delegateBaseEnv is the claude process's environment before the token:
// the workload PATH and C toolchain, and the egress proxy settings.
func delegateBaseEnv(lookup func(string) (string, bool)) []string {
	path, toolchain := yaegikernel.WorkloadToolchainEnv("/run/current-system/sw/bin:/bin:/usr/bin", lookup)
	env := append([]string{"PATH=" + path, "LANG=C.UTF-8"}, toolchain...)
	return append(env, capsule.EgressProxyEnv()...)
}

// delegateServed reports whether this broker will serve the delegate. A
// capsule without egress never does: the delegate needs the model API.
func delegateServed(token, binary string, egress bool) bool {
	return token != "" && binary != "" && egress
}

// startDelegate serves the delegate socket when this broker holds a token.
func startDelegate(token, binary string, egress bool) {
	if token == "" {
		return
	}
	if !delegateServed(token, binary, egress) {
		log.Printf("capsule-broker: claude delegate token present but egress=%v binary=%v; delegate off", egress, binary != "")
		return
	}
	brokerBin := "/run/capsule/broker"
	srv := claudedelegate.NewServer(claudedelegate.RunRoot, token, delegateBaseEnv(os.LookupEnv), func(runDir string) *exec.Cmd {
		// The supervisor stage (jobs.go) reaps claude's whole subtree, which
		// slice 1 left as zombies under the broker (PID 1).
		cmd := supervisedCommand(brokerBin, -1, brokerBin, "--isolation-stage", delegateStage, "--delegate-run-dir", runDir, "--delegate-claude-bin", binary)
		cmd.Dir = claudedelegate.Workdir
		return cmd
	})
	_ = os.Remove(claudedelegate.SocketPath)
	listener, err := net.Listen("unix", claudedelegate.SocketPath)
	if err != nil {
		log.Printf("capsule-broker: claude delegate socket: %v; delegate off", err)
		return
	}
	_ = os.Chmod(claudedelegate.SocketPath, 0o600)
	log.Printf("capsule-broker: claude delegate ready on %s", claudedelegate.SocketPath)
	go func() { _ = srv.Serve(listener) }()
}

// runDelegateStage hardens this process the way a session worker is
// hardened (a workload Landlock layer of its own, capability drop, the L2
// seccomp filter plus the Bun runtime calls of the delegate profile), then
// becomes claude. Its Landlock domain sits beside the cells' domains, not
// under them, so a cell cannot ptrace it or read its /proc environ
// (UNVERIFIED on the staging kernel; F13).
func runDelegateStage(runDir, binary string) error {
	runDir = filepath.Clean(runDir)
	if !strings.HasPrefix(runDir, claudedelegate.RunRoot+"/") {
		return fmt.Errorf("delegate run dir %q is outside %s", runDir, claudedelegate.RunRoot)
	}
	raw, err := os.ReadFile(filepath.Join(runDir, claudedelegate.SpecFile))
	if err != nil {
		return err
	}
	spec, err := claudedelegate.DecodeSpec(raw)
	if err != nil {
		return err
	}
	if strings.TrimSpace(os.Getenv(claudedelegate.TokenEnv)) == "" {
		return fmt.Errorf("delegate stage started without a token")
	}
	if err := capsule.NewWorkloadLandlock("/").Apply(); err != nil {
		return fmt.Errorf("delegate landlock: %w", err)
	}
	if err := capsule.DropWorkloadCapabilities(); err != nil {
		return fmt.Errorf("delegate capability drop: %w", err)
	}
	// Without timerfd Bun panics at start (us_create_timer returned null);
	// docs/archive/problems/claude-delegate-bun-timer-seccomp-2026-10-15.md.
	if err := capsule.LoadDelegateFilter(); err != nil {
		return fmt.Errorf("delegate seccomp: %w", err)
	}
	if err := os.Chdir(claudedelegate.Workdir); err != nil {
		return err
	}
	argv := append([]string{binary}, claudedelegate.Args(spec)...)
	return syscall.Exec(binary, argv, os.Environ())
}
