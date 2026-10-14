package vmmanager

// virtio-mem memory hotplug (docs/reports/memory-hotplug-controller-design-2026-10-14.md).
//
// A computer vmctl selects boots with a hotpluggable region beside its base
// memory. The region starts fully unplugged; vmctl's lease controller asks
// Firecracker to plug part of it while a browser runs and unplug it after.
//
// Firecracker v1.15.1 API, verified at the tag:
//   - config file key "memory-hotplug" (resources.rs VmmConfig is
//     rename_all = "kebab-case"), body MemoryHotplugConfig
//     {total_size_mib, block_size_mib, slot_size_mib} with
//     deny_unknown_fields (vmm_config/memory_hotplug.rs);
//   - validation: block >= 2 and a power of two; slot >= 128 and a multiple
//     of the block; total >= slot and a multiple of the slot;
//   - PATCH /hotplug/memory {"requested_size_mib": R}: R a multiple of the
//     block and <= total, and the guest driver must be bound
//     (devices/virtio/mem/device.rs update_requested_size);
//   - GET /hotplug/memory: {total_size_mib, slot_size_mib, block_size_mib,
//     plugged_size_mib, requested_size_mib} (swagger MemoryHotplugStatus).
//
// The guest needs CONFIG_VIRTIO_MEM (=m in the guest kernel) and
// memhp_default_state=online_movable so plugged memory is onlined into
// ZONE_MOVABLE and can be unplugged (docs/memory-hotplug.md).

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

const (
	defaultMemoryHotplugBlockMiB = 2
	defaultMemoryHotplugSlotMiB  = 128
	minMemoryHotplugSlotMiB      = 128

	// memoryHotplugKernelParam onlines plugged memory automatically into
	// ZONE_MOVABLE, so it can be unplugged again.
	memoryHotplugKernelParam = "memhp_default_state=online_movable"

	memoryHotplugAPITimeout = 3 * time.Second
	// snapshotUnplugTimeout bounds the shrink-to-base before a snapshot.
	snapshotUnplugTimeout = 10 * time.Second
	snapshotUnplugPoll    = 250 * time.Millisecond
)

// ErrMemoryHotplugUnavailable means the VM has no hotplug device the host
// can reach: not running, launched without the device, or no API socket.
var ErrMemoryHotplugUnavailable = errors.New("memory hotplug unavailable")

// ErrMemoryHotplugBusy means another manager operation (boot, hibernate,
// restore) holds the VM; the caller retries later.
var ErrMemoryHotplugBusy = errors.New("memory hotplug: VM operation in progress")

// ErrMemoryHotplugRejected means Firecracker refused the request (an invalid
// size, or the guest driver is not bound yet).
var ErrMemoryHotplugRejected = errors.New("memory hotplug request rejected")

// MemoryHotplugStatus is a VM's base memory and its hotplug device state.
type MemoryHotplugStatus struct {
	BaseMiB      int `json:"base_mib"`
	TotalMiB     int `json:"total_size_mib"`
	BlockMiB     int `json:"block_size_mib"`
	SlotMiB      int `json:"slot_size_mib"`
	PluggedMiB   int `json:"plugged_size_mib"`
	RequestedMiB int `json:"requested_size_mib"`
}

// validateMemoryHotplugSizes is Firecracker 1.15.1's MemoryHotplugConfig
// validation, so a bad size omits the device instead of failing the boot.
func validateMemoryHotplugSizes(total, block, slot int) error {
	switch {
	case block < defaultMemoryHotplugBlockMiB:
		return fmt.Errorf("block %d MiB is below %d MiB", block, defaultMemoryHotplugBlockMiB)
	case block&(block-1) != 0:
		return fmt.Errorf("block %d MiB is not a power of two", block)
	case slot < minMemoryHotplugSlotMiB:
		return fmt.Errorf("slot %d MiB is below %d MiB", slot, minMemoryHotplugSlotMiB)
	case slot%block != 0:
		return fmt.Errorf("slot %d MiB is not a multiple of the %d MiB block", slot, block)
	case total < slot:
		return fmt.Errorf("total %d MiB is below the %d MiB slot", total, slot)
	case total%slot != 0:
		return fmt.Errorf("total %d MiB is not a multiple of the %d MiB slot", total, slot)
	}
	return nil
}

func (m *Manager) memoryHotplugBlockSlot() (int, int) {
	block, slot := m.cfg.MemoryHotplugBlockMiB, m.cfg.MemoryHotplugSlotMiB
	if block <= 0 {
		block = defaultMemoryHotplugBlockMiB
	}
	if slot <= 0 {
		slot = defaultMemoryHotplugSlotMiB
	}
	return block, slot
}

// firecrackerMemoryHotplugConfig returns the "memory-hotplug" object for a
// launch, or nil when the manager flag is off, vmctl did not select this
// computer, or the sizes do not validate.
func (m *Manager) firecrackerMemoryHotplugConfig(cfg VMConfig) map[string]interface{} {
	if !m.cfg.MemoryHotplug || cfg.MemoryHotplugMiB <= 0 {
		return nil
	}
	block, slot := m.memoryHotplugBlockSlot()
	if err := validateMemoryHotplugSizes(cfg.MemoryHotplugMiB, block, slot); err != nil {
		log.Printf("vmmanager: memory hotplug for VM %s omitted: %v", cfg.VMID, err)
		return nil
	}
	return map[string]interface{}{
		"total_size_mib": cfg.MemoryHotplugMiB,
		"block_size_mib": block,
		"slot_size_mib":  slot,
	}
}

// wantsAPISocket reports whether a launch serves the Firecracker API:
// snapshot hibernate needs it, and so does a hotplug device (run-time PATCH).
func (m *Manager) wantsAPISocket(cfg VMConfig) bool {
	return m.hibernateMode() == HibernateModeSnapshot || m.firecrackerMemoryHotplugConfig(cfg) != nil
}

// runningAPISocket returns the API socket of a running VM and its base
// memory, or ErrMemoryHotplugUnavailable.
func (m *Manager) runningAPISocket(vmID string) (string, int, error) {
	m.mu.Lock()
	inst := m.vms[vmID]
	if inst == nil || inst.State != StateRunning {
		m.mu.Unlock()
		return "", 0, fmt.Errorf("%w: VM %s is not running", ErrMemoryHotplugUnavailable, vmID)
	}
	jailed := inst.confinement.jailed()
	base := inst.Config.MachineMemSizeMib
	m.mu.Unlock()
	sock := m.apiSocketPath(vmID)
	if jailed {
		sock = m.jailAPISocketHostPath(vmID)
	}
	if _, err := os.Stat(sock); err != nil {
		return "", 0, fmt.Errorf("%w: VM %s has no API socket", ErrMemoryHotplugUnavailable, vmID)
	}
	return sock, base, nil
}

func classifyHotplugAPIError(vmID string, err error) error {
	var apiErr *fcAPIError
	if errors.As(err, &apiErr) {
		if apiErr.Status >= 400 && apiErr.Status < 500 {
			return fmt.Errorf("%w: VM %s: %s", ErrMemoryHotplugRejected, vmID, apiErr.FaultMessage)
		}
		return fmt.Errorf("VM %s: %w", vmID, err)
	}
	// The socket did not answer: treat as no reachable device.
	return fmt.Errorf("%w: VM %s: %v", ErrMemoryHotplugUnavailable, vmID, err)
}

func memoryHotplugStatusVia(ctx context.Context, api *fcAPIClient, vmID string, base int) (MemoryHotplugStatus, error) {
	var st MemoryHotplugStatus
	if err := api.doJSON(ctx, http.MethodGet, "/hotplug/memory", nil, &st); err != nil {
		err = classifyHotplugAPIError(vmID, err)
		if errors.Is(err, ErrMemoryHotplugRejected) {
			// GET fails only when no device was configured.
			return MemoryHotplugStatus{}, fmt.Errorf("%w: %v", ErrMemoryHotplugUnavailable, err)
		}
		return MemoryHotplugStatus{}, err
	}
	if st.TotalMiB <= 0 {
		return MemoryHotplugStatus{}, fmt.Errorf("%w: VM %s reports no hotplug region", ErrMemoryHotplugUnavailable, vmID)
	}
	st.BaseMiB = base
	return st, nil
}

// MemoryHotplugStatus reads a running VM's hotplug device. It never blocks
// on another operation of the same VM (ErrMemoryHotplugBusy).
func (m *Manager) MemoryHotplugStatus(vmID string) (MemoryHotplugStatus, error) {
	unlock, ok := m.tryLockVMOperation(vmID)
	if !ok {
		return MemoryHotplugStatus{}, ErrMemoryHotplugBusy
	}
	defer unlock()
	sock, base, err := m.runningAPISocket(vmID)
	if err != nil {
		return MemoryHotplugStatus{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), memoryHotplugAPITimeout)
	defer cancel()
	return memoryHotplugStatusVia(ctx, newFCAPIClient(sock), vmID, base)
}

// SetMemoryHotplugRequestedMiB sets the hotplug target (memory above the
// base). Plugging and unplugging are asynchronous; poll MemoryHotplugStatus.
func (m *Manager) SetMemoryHotplugRequestedMiB(vmID string, requestedMiB int) error {
	if requestedMiB < 0 {
		return fmt.Errorf("%w: negative request %d MiB", ErrMemoryHotplugRejected, requestedMiB)
	}
	unlock, ok := m.tryLockVMOperation(vmID)
	if !ok {
		return ErrMemoryHotplugBusy
	}
	defer unlock()
	sock, _, err := m.runningAPISocket(vmID)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), memoryHotplugAPITimeout)
	defer cancel()
	if err := newFCAPIClient(sock).PatchMemoryHotplug(ctx, requestedMiB); err != nil {
		return classifyHotplugAPIError(vmID, err)
	}
	log.Printf("vmmanager: memory hotplug VM %s requested_size_mib=%d", vmID, requestedMiB)
	return nil
}

// unplugBeforeSnapshot shrinks a VM to its base before a snapshot (the
// guest is already fenced), waiting up to snapshotUnplugTimeout. The
// snapshot proceeds whatever the outcome: Firecracker snapshots plugged
// blocks as data and unplugged ones as holes. It returns a timeline detail,
// or "" when the VM has no device or nothing plugged.
func unplugBeforeSnapshot(api *fcAPIClient, vmID string, base int, timeout time.Duration) string {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	st, err := memoryHotplugStatusVia(ctx, api, vmID, base)
	if err != nil || (st.PluggedMiB == 0 && st.RequestedMiB == 0) {
		return ""
	}
	before := st.PluggedMiB
	if err := api.PatchMemoryHotplug(ctx, 0); err != nil {
		log.Printf("vmmanager: snapshot of VM %s: unplug request failed: %v; snapshotting %d MiB plugged", vmID, err, before)
		return fmt.Sprintf("plugged_before_mib=%d plugged_after_mib=%d error=%q", before, before, err.Error())
	}
	after := before
	for {
		if cur, err := memoryHotplugStatusVia(ctx, api, vmID, base); err == nil {
			after = cur.PluggedMiB
			if after == 0 {
				break
			}
		}
		select {
		case <-ctx.Done():
			log.Printf("vmmanager: snapshot of VM %s: guest unplugged %d of %d MiB in %s; snapshotting the rest", vmID, before-after, before, timeout)
			return fmt.Sprintf("plugged_before_mib=%d plugged_after_mib=%d timeout=true", before, after)
		case <-time.After(snapshotUnplugPoll):
		}
	}
	return fmt.Sprintf("plugged_before_mib=%d plugged_after_mib=0", before)
}

// launchConfigHasMemoryHotplug reports whether a written fc-config.json
// configured the device: the launch, not today's selection, says what the
// running Firecracker has.
func launchConfigHasMemoryHotplug(raw []byte) bool {
	var cfg map[string]json.RawMessage
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return false
	}
	_, ok := cfg["memory-hotplug"]
	return ok
}
