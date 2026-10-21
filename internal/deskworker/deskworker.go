package deskworker

import (
	"fmt"
	"net"
	"os"

	"github.com/yusefmosiah/go-choir/internal/gomemlimit"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// steps is the worker's startup seam: Run wires the real ones, the test
// wires recorders to pin the order and that serving never precedes confinement.
type steps struct {
	config      func() yaegikernel.SessionWorkerConfig
	sockFD      func() (int, net.Conn, error)
	volunteer   func() error
	memoryLimit func(uint64) error
	goMemLimit  func()
	confine     func() error
	selfTest    func()
	serve       func(net.Conn, yaegikernel.SessionWorkerConfig)
}

func realSteps() steps {
	return steps{
		config:      func() yaegikernel.SessionWorkerConfig { return yaegikernel.SessionWorkerConfigFromEnv(nil, "") },
		sockFD:      func() (int, net.Conn, error) { return yaegikernel.SessionWorkerSockFD(nil, "") },
		volunteer:   yaegikernel.VolunteerForOOMKill,
		memoryLimit: yaegikernel.ApplyWorkerMemoryLimit,
		goMemLimit:  func() { gomemlimit.ApplyDeskWorker() },
		confine:     func() error { return yaegikernel.ConfineDeskSessionWorker(nil, "") },
		selfTest:    func() { yaegikernel.RunDeskCellSelfTestIfRequested(nil, "") },
		serve:       yaegikernel.ExecuteWorkerSessionConn,
	}
}

// Run is the host desk-cell session-worker entrypoint, shared by
// `autoputer desk-session` and the dedicated cmd/desk-worker binary. The host
// spawner (agentcore desk_go_eval) re-execs it with a socketpair fd 3 and
// CHOIR_DESK_SESSION_* env; it reconstructs the SessionWorkerConfig and serves
// framed eval cells via yaegikernel.ExecuteWorkerSessionConn. It is the
// killable-subprocess boundary for non-capsule desk model-authored Go.
//
// Startup order: gomemlimit.ApplyDeskWorker, ConfineDeskSessionWorker (exit 3
// on error), RunDeskCellSelfTestIfRequested, then serving.
func Run() int { return run(realSteps()) }

func run(s steps) int {
	cfg := s.config()
	fd, conn, err := s.sockFD()
	if err != nil || fd < 0 || conn == nil {
		fmt.Fprintf(os.Stderr, "desk-session: session socket unavailable (fd=%d err=%v)\n", fd, err)
		return 2
	}
	if len(cfg.AllowedPackages) == 0 {
		cfg.AllowedPackages = yaegikernel.DefaultSafeStdlibPackagesList()
	}
	// Best effort: a worker that cannot volunteer still serves cells.
	if err := s.volunteer(); err != nil {
		fmt.Fprintf(os.Stderr, "desk-session: oom_score_adj unavailable: %v\n", err)
	}
	if err := s.memoryLimit(cfg.MemoryLimitBytes); err != nil {
		fmt.Fprintf(os.Stderr, "desk-session: memory cap unavailable: %v\n", err)
		return 2
	}
	// Soft Go limit beside the hard RLIMIT_AS cap: the GC collects before
	// the heap doubles, so a finished cell's garbage does not sit in guest
	// memory. The daemon's boot log names the value (desk_worker_limit_mib).
	s.goMemLimit()
	// Last step before the first cell: no_new_privs, Landlock and the cell
	// seccomp filter (desk_cell_confinement.go). Everything above reads
	// files (/proc/meminfo) that a confined worker can no longer open.
	if err := s.confine(); err != nil {
		fmt.Fprintf(os.Stderr, "desk-session: confinement: %v\n", err)
		return 3
	}
	// Only the isolation self-test's spawner asks for this (staging proof of
	// the confinement above); a production desk worker skips it.
	s.selfTest()
	s.serve(conn, cfg)
	return 0
}
