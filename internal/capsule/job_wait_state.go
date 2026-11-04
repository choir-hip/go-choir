package capsule

import (
	"path"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// The typed record of a build job's wait for host memory
// (docs/problems/selfdev-waits-untyped-and-invisible-2026-11-04.md). The
// decision to wait is vmctl's lease refusal; this renders the refusal it
// sends (yaegikernel.MemoryWait) as a types.WaitState, so engineering sees
// what the job needs, what room there is, who holds it and when the next
// check is, and management and Texture see the same record.

// HostMemoryWaitState renders vmctl's refusal of a job's lease (mw, the
// job view's memory_wait) as the typed wait, without re-deriving it: need
// is the lease, have the room the host has for it, the holders vmctl's
// (this computer's by capsule and job id; another computer's by kind and
// class only, never an id). reason is the job's wait text (Detail). It is
// nil for a nil refusal.
func HostMemoryWaitState(subject string, mw *yaegikernel.MemoryWait, reason string, since time.Time, poll, maxWait time.Duration) *types.WaitState {
	if mw == nil {
		return nil
	}
	w := &types.WaitState{
		Kind: types.WaitKindHostMemory, Class: types.WaitClassContention, Subject: subject, Resource: "host memory",
		Since: since.UTC(), CheckEverySec: checkSeconds(poll), Detail: reason,
		NeedBytes: clampBytes(mw.NeedBytes),
	}
	if maxWait > 0 {
		w.Until = since.Add(maxWait).UTC()
	}
	if mw.PressureHigh {
		w.Have = "the host is under memory pressure and leases nothing until it clears"
	} else {
		w.HaveBytes, w.HasHave = clampBytes(mw.RoomBytes), true
	}
	for _, h := range mw.Holders {
		holder := types.WaitHolder{What: h.Kind, Bytes: clampBytes(h.Bytes)}
		switch {
		case h.Own && h.JobID != "":
			holder.Who = "job " + h.JobID
		case h.Own && h.CapsuleID != "":
			holder.Who = "capsule " + h.CapsuleID
		case h.Own:
			holder.Who = "this computer"
		default:
			holder.Who = "another computer"
			var parts []string
			for _, part := range []string{h.Kind, h.Class} {
				if strings.TrimSpace(part) != "" {
					parts = append(parts, strings.TrimSpace(part))
				}
			}
			holder.What = strings.Join(parts, ", ")
		}
		w.Holders = append(w.Holders, holder)
	}
	return w
}

func clampBytes(b uint64) int64 {
	if b > uint64(^uint64(0)>>1) {
		return int64(^uint64(0) >> 1)
	}
	return int64(b)
}

// PureWaitJob reports whether argv only waits: the Claude delegate's wait
// or status (its run's memory is the delegate's, started by
// `choir-delegate start` outside the job), or sleep. Such a job holds no
// build's memory, so it never waits for host memory; a full build-peak
// claim made it queue behind other computers' builds for nothing (the
// delegate wait-park of 2026-11-03). One simple command only, directly or
// as the one command of sh -c: anything chained may build.
func PureWaitJob(argv []string) bool {
	if len(argv) == 0 {
		return false
	}
	if base := path.Base(argv[0]); base == "sh" || base == "bash" || base == "dash" {
		if len(argv) != 3 || (argv[1] != "-c" && argv[1] != "-lc") {
			return false
		}
		cmd := argv[2]
		if strings.ContainsAny(cmd, ";&|\n`<>$") {
			return false
		}
		return pureWaitArgv(strings.Fields(cmd))
	}
	return pureWaitArgv(argv)
}

func pureWaitArgv(argv []string) bool {
	if len(argv) < 2 {
		return false
	}
	switch path.Base(argv[0]) {
	case "choir-delegate":
		return argv[1] == "wait" || argv[1] == "status"
	case "sleep":
		return true
	}
	return false
}

// JobWaitEvent tells the runtime a job's wait for host memory began, was
// re-checked, or ended (Ended, with Waited its length), so the waiting
// assignment's record and tally follow the job (Executor.SetJobWaitHandler).
type JobWaitEvent struct {
	AgentRunID string
	JobID      string
	Notify     map[string]string
	Wait       *types.WaitState
	Ended      bool
	Waited     time.Duration
	// Outcome says how an ended wait ended: "started", or the job's end
	// state when it never started.
	Outcome string
}

// checkSeconds is a check cadence in whole seconds, at least 1 when there
// is one (review r2 F5).
func checkSeconds(d time.Duration) int {
	if d <= 0 {
		return 0
	}
	return max(int(d/time.Second), 1)
}
