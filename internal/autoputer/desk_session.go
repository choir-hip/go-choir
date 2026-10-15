package autoputer

import (
	"fmt"
	"os"

	"github.com/yusefmosiah/go-choir/internal/gomemlimit"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// RunDeskSessionWorker is the host desk-cell session-worker entrypoint the
// daemon re-executes into with `autoputer desk-session`. The host spawner
// (agentcore desk_go_eval) re-execs this binary with a socketpair fd 3 and
// CHOIR_DESK_SESSION_* env; this entrypoint reconstructs the SessionWorkerConfig
// and serves framed eval cells via yaegikernel.ExecuteWorkerSessionConn. It is
// the killable-subprocess boundary for non-capsule desk model-authored Go —
// parallel to cmd/capsule-broker's exec-go-session inside the guest.
func RunDeskSessionWorker() int {
	cfg := yaegikernel.SessionWorkerConfigFromEnv(nil, "")
	fd, conn, err := yaegikernel.SessionWorkerSockFD(nil, "")
	if err != nil || fd < 0 || conn == nil {
		fmt.Fprintf(os.Stderr, "desk-session: session socket unavailable (fd=%d err=%v)\n", fd, err)
		return 2
	}
	if len(cfg.AllowedPackages) == 0 {
		cfg.AllowedPackages = yaegikernel.DefaultSafeStdlibPackagesList()
	}
	if err := yaegikernel.ApplyWorkerMemoryLimit(cfg.MemoryLimitBytes); err != nil {
		fmt.Fprintf(os.Stderr, "desk-session: memory cap unavailable: %v\n", err)
		return 2
	}
	// Soft Go limit beside the hard RLIMIT_AS cap: the GC collects before
	// the heap doubles, so a finished cell's garbage does not sit in guest
	// memory. The daemon's boot log names the value (desk_worker_limit_mib).
	gomemlimit.ApplyDeskWorker()
	// Last step before the first cell: no_new_privs, Landlock and the cell
	// seccomp filter (desk_cell_confinement.go). Everything above reads
	// files (/proc/meminfo) that a confined worker can no longer open.
	if err := yaegikernel.ConfineDeskSessionWorker(nil, ""); err != nil {
		fmt.Fprintf(os.Stderr, "desk-session: confinement: %v\n", err)
		return 3
	}
	// Only the isolation self-test's spawner asks for this (staging proof of
	// the confinement above); a production desk worker skips it.
	yaegikernel.RunDeskCellSelfTestIfRequested(nil, "")
	yaegikernel.ExecuteWorkerSessionConn(conn, cfg)
	return 0
}
