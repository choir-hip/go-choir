package autoputer

import (
	"fmt"
	"os"

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
	yaegikernel.ExecuteWorkerSessionConn(conn, cfg)
	return 0
}
