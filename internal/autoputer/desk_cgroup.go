package autoputer

import (
	"log"
	"os"

	"github.com/yusefmosiah/go-choir/internal/doctool"
	"github.com/yusefmosiah/go-choir/internal/gomemlimit"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// Desks cgroup aggregate (SDC RES-01): all desk workers together may hold half
// of guest memory hard and three eighths before the kernel throttles them, so
// workers can never starve the daemon and the page cache the daemon needs.
// Each worker has its own, smaller budget from its desk's manifest row
// (RES-02). The guest image names the cgroup in CHOIR_DESK_CGROUP_ROOT; a
// host without it (dev, CI) runs workers on RLIMIT_AS and GOMEMLIMIT alone.
const (
	deskCgroupMaxDivisor  = 2
	deskCgroupHighDivisor = 8 // times 3
)

// ensureDeskCgroups makes the desks cgroup and removes the cgroups of workers
// a crashed daemon left behind. A failure is logged loudly and the daemon
// carries on: in required confinement mode every worker spawn then refuses
// for want of its cgroup, so the desks fail visibly and nothing runs
// unbudgeted.
func ensureDeskCgroups() {
	root := os.Getenv(yaegikernel.DeskCgroupRootEnv)
	if root == "" {
		log.Printf("autoputer: desk cgroup: %s unset; desk workers run on RLIMIT_AS and GOMEMLIMIT only", yaegikernel.DeskCgroupRootEnv)
		return
	}
	total, err := gomemlimit.MemTotal()
	if err != nil || total <= 0 {
		log.Printf("autoputer: desk cgroup %s: guest memory unknown (%v); desk workers will refuse to start in required mode", root, err)
		return
	}
	max := uint64(total) / deskCgroupMaxDivisor
	high := uint64(total) / deskCgroupHighDivisor * 3
	if err := yaegikernel.EnsureDeskCgroupRoot(root, max, high); err != nil {
		log.Printf("autoputer: desk cgroup %s: %v; desk workers will refuse to start in required mode", root, err)
		return
	}
	log.Printf("autoputer: desk cgroup %s memory.max %d MiB memory.high %d MiB", root, max>>20, high>>20)
}

// ensureDocToolCgroups makes the document tools' cgroup root (SEC-15) when
// the sandbox is on: their own aggregate, never the desks' (review B2).
// Required runs, observe's shadows (review E2) and the isolation self-test
// each run in a child of it. A failure is logged: required mode then refuses
// every tool (the extraction falls back to its pure-Go projection); observe
// returns the direct run as always and runs no shadow.
func ensureDocToolCgroups() {
	if doctool.ModeFromEnv(os.Getenv) == doctool.ModeOff {
		return
	}
	root := os.Getenv("CHOIR_DOCTOOL_CGROUP_ROOT")
	if root == "" {
		log.Printf("autoputer: document tools cgroup: CHOIR_DOCTOOL_CGROUP_ROOT unset; required mode will refuse document tools")
		return
	}
	if err := doctool.EnsureToolCgroupRoot(root); err != nil {
		log.Printf("autoputer: document tools cgroup %s: %v; required mode will refuse document tools", root, err)
		return
	}
	log.Printf("autoputer: document tools cgroup %s ready", root)
}
