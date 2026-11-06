//go:build !linux

package yaegikernel

import "time"

// gVisor is a Linux mechanism (desk_gvisor.go); a non-Linux host runs desk
// workers natively for local unit tests only (R40).

type gvisorCell struct{}

func (c *gvisorCell) inspect() (sandboxInspectReport, error) {
	return sandboxInspectReport{Error: "gVisor needs Linux"}, nil
}

func gvisorAvailable() (bool, string) { return false, "gVisor needs Linux" }

// GvisorStatusNow reports gVisor unavailable on a non-Linux host.
func GvisorStatusNow() GvisorStatus {
	return GvisorStatus{Reason: "gVisor needs Linux", Sandboxes: []GvisorSandboxReceipt{}}
}

func (c *gvisorCell) sandboxKeyID() (string, string) { return "", "" }

func (c *gvisorCell) readCellFile(name string) ([]byte, error) { return nil, nil }

func (c *gvisorCell) writeCellCanary(name string) (string, error) { return "", nil }

func (c *gvisorCell) killWorker() bool { return true }

func (c *gvisorCell) cgroupPath() string { return "" }

func (c *gvisorCell) heapShare() uint64 { return 0 }

func (c *gvisorCell) queuedFor() time.Duration { return 0 }

// NotifyDeskSessionIdle: no admission queue off Linux.
func NotifyDeskSessionIdle() {}
