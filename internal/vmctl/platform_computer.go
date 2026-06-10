package vmctl

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	UniversalWirePlatformOwnerID   = "universal-wire-platform"
	UniversalWirePlatformDesktopID = "platform"
	UniversalWirePlatformVMID      = "vm-universal-wire-platform"
)

// UniversalWirePlatformRuntimeEnv holds host-side dispatch binding for the
// always-on Universal Wire platform computer sandbox.
type UniversalWirePlatformRuntimeEnv struct {
	RuntimeBaseURL string
	OwnerID        string
}

// WriteUniversalWirePlatformRuntimeEnv atomically writes sourcecycled dispatch
// binding for the platform computer sandbox URL.
func WriteUniversalWirePlatformRuntimeEnv(path string, env UniversalWirePlatformRuntimeEnv) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("platform runtime env path is required")
	}
	baseURL := strings.TrimRight(strings.TrimSpace(env.RuntimeBaseURL), "/")
	if baseURL == "" {
		return fmt.Errorf("platform runtime base URL is required")
	}
	ownerID := strings.TrimSpace(env.OwnerID)
	if ownerID == "" {
		ownerID = UniversalWirePlatformOwnerID
	}
	content := fmt.Sprintf(
		"SOURCE_SERVICE_RUNTIME_BASE_URL=%s\nSOURCE_SERVICE_RUNTIME_OWNER_ID=%s\n",
		baseURL,
		ownerID,
	)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create platform runtime env dir: %w", err)
	}
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write platform runtime env: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("publish platform runtime env: %w", err)
	}
	return nil
}

// EnsureUniversalWirePlatformComputer boots or resumes the always-on platform
// computer and returns the host-reachable sandbox URL for sourcecycled dispatch.
func (r *OwnershipRegistry) EnsureUniversalWirePlatformComputer(ctx context.Context) (UniversalWirePlatformRuntimeEnv, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	own, err := r.ensureUniversalWirePlatformOwnership(ctx)
	if err != nil {
		return UniversalWirePlatformRuntimeEnv{}, err
	}
	baseURL := strings.TrimRight(strings.TrimSpace(own.SandboxURL), "/")
	if baseURL == "" {
		return UniversalWirePlatformRuntimeEnv{}, fmt.Errorf("platform computer %s has empty sandbox URL", own.VMID)
	}
	return UniversalWirePlatformRuntimeEnv{
		RuntimeBaseURL: baseURL,
		OwnerID:        UniversalWirePlatformOwnerID,
	}, nil
}

// WarmUniversalWirePlatformComputer resumes a stopped platform computer during
// the idle sweeper loop. It does not create a new ownership on its own.
func (r *OwnershipRegistry) WarmUniversalWirePlatformComputer() int {
	key := ownershipKey(UniversalWirePlatformOwnerID, UniversalWirePlatformDesktopID)
	r.mu.RLock()
	own := r.ownerships[key]
	r.mu.RUnlock()
	if own == nil {
		return 0
	}
	if own.State != VMStateStopped && own.State != VMStateHibernated {
		return 0
	}
	resumed, err := r.ResumeVMForDesktop(UniversalWirePlatformOwnerID, UniversalWirePlatformDesktopID)
	if err != nil {
		log.Printf("vmctl: warmness policy failed to resume platform computer vm=%s: %v", own.VMID, err)
		return 0
	}
	if resumed != nil {
		return 1
	}
	return 0
}

func (r *OwnershipRegistry) ensureUniversalWirePlatformOwnership(ctx context.Context) (*VMOwnership, error) {
	key := ownershipKey(UniversalWirePlatformOwnerID, UniversalWirePlatformDesktopID)

	r.mu.Lock()
	if own, ok := r.ownerships[key]; ok {
		switch {
		case own.IsReady():
			snapshot := *own
			mgr := r.vmManager
			if activeOwnershipNeedsReadinessCheck(&snapshot, mgr) {
				r.mu.Unlock()
				info, err := r.ensureActiveVMReady(&snapshot, mgr)
				if err != nil {
					return nil, fmt.Errorf("verify platform computer %s: %w", snapshot.VMID, err)
				}
				r.mu.Lock()
				current := r.ownerships[key]
				if current == nil {
					r.mu.Unlock()
					return r.ensureUniversalWirePlatformOwnership(ctx)
				}
				if info != nil {
					current.SandboxURL = info.HostURL
					current.Epoch = info.Epoch
				}
				current.State = VMStateActive
				current.LastActiveAt = time.Now()
				r.saveLocked()
				vmID := current.VMID
				r.mu.Unlock()
				r.ensureExistingGatewayCredential(vmID)
				return current, nil
			}
			own.LastActiveAt = time.Now()
			r.saveLocked()
			vmID := own.VMID
			r.mu.Unlock()
			r.ensureExistingGatewayCredential(vmID)
			return own, nil
		case own.State == VMStateStopped || own.State == VMStateHibernated:
			mgr := r.vmManager
			snapshot := *own
			r.mu.Unlock()
			if mgr != nil {
				info, err := r.startExistingVM(&snapshot, mgr)
				if err != nil {
					return nil, fmt.Errorf("start platform computer %s: %w", snapshot.VMID, err)
				}
				r.mu.Lock()
				current := r.ownerships[key]
				if current == nil {
					r.mu.Unlock()
					return nil, fmt.Errorf("platform computer ownership disappeared during resume")
				}
				current.SandboxURL = info.HostURL
				current.Epoch = info.Epoch
				current.State = VMStateActive
				current.LastActiveAt = time.Now()
				current.StoppedBy = ""
				r.saveLocked()
				vmID := current.VMID
				sandboxURL := current.SandboxURL
				r.mu.Unlock()
				r.ensureExistingGatewayCredential(vmID)
				log.Printf("vmctl: resumed platform computer %s at %s", vmID, sandboxURL)
				return current, nil
			}
			r.mu.Lock()
			own.State = VMStateActive
			own.LastActiveAt = time.Now()
			own.StoppedBy = ""
			r.saveLocked()
			r.mu.Unlock()
			return own, nil
		case own.State == VMStateBooting:
			if waiters, ok := r.pendingWaiters[key]; ok {
				return r.waitForPendingAssignmentLocked(ctx, key, UniversalWirePlatformOwnerID, UniversalWirePlatformDesktopID, waiters)
			}
		}
		delete(r.vmByID, own.VMID)
		delete(r.ownerships, key)
	}
	if waiters, ok := r.pendingWaiters[key]; ok {
		return r.waitForPendingAssignmentLocked(ctx, key, UniversalWirePlatformOwnerID, UniversalWirePlatformDesktopID, waiters)
	}

	vmID := UniversalWirePlatformVMID
	epoch := r.nextEpoch()
	own := &VMOwnership{
		VMID:          vmID,
		UserID:        UniversalWirePlatformOwnerID,
		DesktopID:     UniversalWirePlatformDesktopID,
		Kind:          VMKindInteractive,
		WarmnessClass: WarmnessClassPublicPlatform,
		SandboxURL:    r.sandboxURLForVM(vmID),
		State:         VMStateBooting,
		CreatedAt:     time.Now(),
		LastActiveAt:  time.Now(),
		Epoch:         epoch,
		Published:     true,
	}
	r.pendingWaiters[key] = nil
	r.ownerships[key] = own
	r.vmByID[vmID] = own
	r.saveLocked()
	mgr := r.vmManager
	r.mu.Unlock()

	if mgr == nil {
		return nil, fmt.Errorf("platform computer requires Firecracker VM manager")
	}
	gwToken := r.issueGatewayToken(vmID)
	info, err := mgr.BootVM(VMManagerConfig{
		VMID:              vmID,
		GuestPort:         8085,
		MachineCPUCount:   interactiveVMCPUCount,
		MachineMemSizeMib: interactiveVMMemSizeMib,
		GatewayToken:      gwToken,
		ComputerKind:      "platform",
		OwnerID:           UniversalWirePlatformOwnerID,
		DesktopID:         UniversalWirePlatformDesktopID,
	})
	if err != nil {
		r.mu.Lock()
		own.State = VMStateFailed
		waiters := r.pendingWaiters[key]
		delete(r.pendingWaiters, key)
		r.saveLocked()
		r.mu.Unlock()
		for _, ch := range waiters {
			ch <- nil
		}
		return nil, fmt.Errorf("boot platform computer %s: %w", vmID, err)
	}
	r.mu.Lock()
	own.SandboxURL = info.HostURL
	own.Epoch = info.Epoch
	waiters := r.pendingWaiters[key]
	delete(r.pendingWaiters, key)
	r.saveLocked()
	r.mu.Unlock()
	r.transitionVM(vmID, VMStateActive)
	for _, ch := range waiters {
		ch <- own
	}
	r.ensureExistingGatewayCredential(vmID)
	log.Printf("vmctl: booted platform computer %s at %s (epoch=%d)", vmID, info.HostURL, info.Epoch)
	return own, nil
}
