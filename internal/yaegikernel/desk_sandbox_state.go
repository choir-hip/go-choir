package yaegikernel

import (
	"context"
	"strings"
	"sync"
	"time"
)

// The desk sandbox state (MA6 follow-up, R24): whether THIS computer has the
// worker isolation a non-first-party desk needs. It is a capability reading,
// not a setting. Ready is true only when the latest isolation selftest of this
// process (the boot run, or a later owner run of
// POST /api/runtime/isolation-selftest) judged the desk workers gVisor-confined
// in required mode. Nothing sets it by hand; a later failing run closes it
// again. internal/agentcore feeds apppackage.CurrentPlatform from it.

// DeskSandboxStatus is the reading, shown read-only by the product API.
type DeskSandboxStatus struct {
	Ready bool `json:"ready"`
	// Reason says why it is not ready (empty when ready).
	Reason string `json:"reason,omitempty"`
	// Source is "boot" or "api": which selftest run set it.
	Source    string `json:"source,omitempty"`
	CheckedAt string `json:"checked_at,omitempty"`
}

const deskSandboxUnchecked = "the boot selftest has not passed"

var (
	deskSandboxMu sync.Mutex
	deskSandboxSt = DeskSandboxStatus{Reason: deskSandboxUnchecked}
)

// JudgeDeskSandboxReady returns every way the receipt falls short of the
// isolation a non-first-party desk needs; empty means it holds: required mode,
// a passing selftest, gVisor available, and every worker under gVisor.
func JudgeDeskSandboxReady(rec DeskCellSelfTestReceipt) []string {
	var fails []string
	if rec.Mode != string(CellConfinementRequired) {
		fails = append(fails, "desk confinement is "+rec.Mode+", not required")
	}
	if !rec.OK || len(rec.Failures) > 0 {
		fails = append(fails, "the desk selftest failed: "+strings.Join(rec.Failures, "; "))
	}
	if rec.Gvisor == nil || !rec.Gvisor.Available {
		reason := "no gVisor state"
		if rec.Gvisor != nil {
			reason = rec.Gvisor.Reason
		}
		fails = append(fails, "gVisor is unavailable ("+reason+")")
	}
	if len(rec.Workers) == 0 {
		fails = append(fails, "the selftest ran no workers")
	}
	for _, w := range rec.Workers {
		if w.Profile != "gvisor" {
			fails = append(fails, w.Role+" ran under "+w.Profile+" confinement, not gVisor")
		}
	}
	return fails
}

// RecordDeskSandboxSelfTest sets the state from a selftest receipt.
func RecordDeskSandboxSelfTest(rec DeskCellSelfTestReceipt, source string) DeskSandboxStatus {
	st := DeskSandboxStatus{Source: source, CheckedAt: time.Now().UTC().Format(time.RFC3339)}
	if fails := JudgeDeskSandboxReady(rec); len(fails) == 0 {
		st.Ready = true
	} else {
		st.Reason = strings.Join(fails, "; ")
	}
	deskSandboxMu.Lock()
	deskSandboxSt = st
	deskSandboxMu.Unlock()
	return st
}

// DeskSandboxStatusNow is the current reading.
func DeskSandboxStatusNow() DeskSandboxStatus {
	deskSandboxMu.Lock()
	defer deskSandboxMu.Unlock()
	return deskSandboxSt
}

// DeskSandboxReady is DeskSandboxStatusNow().Ready.
func DeskSandboxReady() bool { return DeskSandboxStatusNow().Ready }

// ResetDeskSandboxState returns the state to "not checked". Tests.
func ResetDeskSandboxState() {
	deskSandboxMu.Lock()
	deskSandboxSt = DeskSandboxStatus{Reason: deskSandboxUnchecked}
	deskSandboxMu.Unlock()
}

// RunDeskSandboxBootSelfTest runs the desk selftest once and records it. A
// computer not in required mode records "not ready" without spawning anything.
func RunDeskSandboxBootSelfTest(ctx context.Context, cfg DeskCellSelfTestConfig) DeskSandboxStatus {
	if cfg.Base.Confinement.Mode != CellConfinementRequired {
		return RecordDeskSandboxSelfTest(DeskCellSelfTestReceipt{Mode: string(cfg.Base.Confinement.Mode)}, "boot")
	}
	return RecordDeskSandboxSelfTest(DeskCellSelfTest(ctx, cfg), "boot")
}
