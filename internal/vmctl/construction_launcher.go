package vmctl

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerversion"
	"github.com/yusefmosiah/go-choir/internal/diskinstantiation"
)

type VMConstructionLauncher struct {
	registry *OwnershipRegistry
	client   *http.Client
}

func NewVMConstructionLauncher(registry *OwnershipRegistry, client *http.Client) *VMConstructionLauncher {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &VMConstructionLauncher{registry: registry, client: client}
}

var _ computerversion.ConstructedLauncher = (*VMConstructionLauncher)(nil)

func (l *VMConstructionLauncher) Launch(ctx context.Context, request computerversion.ConstructedLaunchRequest) (computerversion.BootReceipt, error) {
	if err := ctx.Err(); err != nil {
		return computerversion.BootReceipt{}, err
	}
	if l == nil || l.registry == nil {
		return computerversion.BootReceipt{}, fmt.Errorf("construction launcher: ownership registry is required")
	}
	l.registry.mu.Lock()
	manager := l.registry.vmManager
	l.registry.mu.Unlock()
	if manager == nil {
		return computerversion.BootReceipt{}, fmt.Errorf("construction launcher: Firecracker VM manager is unavailable")
	}
	identity := request.Identity
	if identity.RealizationID == "" || identity.ComputerKind != "candidate" || identity.OwnerID == "" || identity.DesktopID == "" || identity.CandidateID == "" || identity.DesktopID != identity.CandidateID || request.Disk.DevicePath == "" || request.CodeClosure.Ref != request.Version.CodeRef {
		return computerversion.BootReceipt{}, fmt.Errorf("construction launcher: immutable candidate launch bindings are incomplete")
	}
	credential := l.registry.issueGatewayToken(identity.RealizationID)
	if err := l.registry.beginConstructedCandidate(identity.RealizationID, identity.OwnerID, identity.DesktopID, credential, request.Version, request.Disk); err != nil {
		return computerversion.BootReceipt{}, err
	}
	boot := computerversion.BootReceipt{VMID: request.Identity.RealizationID}
	info, bootErr := manager.BootVM(VMManagerConfig{
		VMID:              request.Identity.RealizationID,
		DataDevicePath:    request.Disk.DevicePath,
		GuestPort:         8085,
		MachineCPUCount:   interactiveVMCPUCount,
		MachineMemSizeMib: interactiveVMMemSizeMib,
		GatewayToken:      credential,
		ComputerKind:      request.Identity.ComputerKind,
		OwnerID:           request.Identity.OwnerID,
		DesktopID:         request.Identity.DesktopID,
		WorkerID:          request.Identity.WorkerID,
		CandidateID:       request.Identity.CandidateID,
		CodeRef:           string(request.Version.CodeRef),
	})
	if bootErr != nil {
		if cleanupErr := l.Destroy(context.Background(), boot); cleanupErr != nil {
			return boot, errors.Join(bootErr, fmt.Errorf("construction launcher: failed boot cleanup: %w", cleanupErr))
		}
		return computerversion.BootReceipt{}, bootErr
	}
	if info == nil {
		missingErr := fmt.Errorf("construction launcher: VM manager returned no instance after boot")
		if cleanupErr := l.Destroy(context.Background(), boot); cleanupErr != nil {
			return boot, errors.Join(missingErr, cleanupErr)
		}
		return computerversion.BootReceipt{}, missingErr
	}
	boot.HostURL = info.HostURL
	boot.Epoch = info.Epoch
	boot.Healthy = info.Healthy
	boot.BootedAt = info.StartedAt.UTC()
	if activateErr := l.registry.activateConstructedCandidate(identity.RealizationID, info.HostURL, info.Epoch); activateErr != nil {
		if cleanupErr := l.Destroy(context.Background(), boot); cleanupErr != nil {
			return boot, errors.Join(activateErr, fmt.Errorf("construction launcher: unactivated VM cleanup: %w", cleanupErr))
		}
		return computerversion.BootReceipt{}, activateErr
	}
	return boot, nil
}

func (l *VMConstructionLauncher) Observe(ctx context.Context, request computerversion.ConstructedLaunchRequest, boot computerversion.BootReceipt) (computerversion.LiveConstructionObservation, error) {
	if l == nil || l.registry == nil || request.Identity.RealizationID != boot.VMID || request.Disk.RealizationID != boot.VMID || request.Version != (computerversion.ComputerVersion{CodeRef: request.CodeClosure.Ref, ArtifactProgramRef: request.Version.ArtifactProgramRef}) {
		return computerversion.LiveConstructionObservation{}, fmt.Errorf("construction launcher: observation bindings are incomplete")
	}
	l.registry.mu.Lock()
	manager := l.registry.vmManager
	own := cloneOwnership(l.registry.vmByID[boot.VMID])
	l.registry.mu.Unlock()
	if manager == nil || own == nil || validateConstructedOwnership(own) != nil || own.State != VMStateActive || own.UserID != request.Identity.OwnerID || own.DesktopID != request.Identity.DesktopID || own.DesktopID != request.Identity.CandidateID || *own.ConstructionVersion != request.Version || !reflect.DeepEqual(*own.ConstructionDisk, request.Disk) {
		return computerversion.LiveConstructionObservation{}, fmt.Errorf("construction launcher: persisted realization binding mismatch")
	}
	info := manager.GetVM(boot.VMID)
	if info == nil || !info.Healthy || info.HostURL == "" || info.HostURL != boot.HostURL || info.Epoch != boot.Epoch {
		return computerversion.LiveConstructionObservation{}, fmt.Errorf("construction launcher: live VM identity does not match boot receipt")
	}
	endpoint, err := url.Parse(strings.TrimRight(info.HostURL, "/") + "/internal/computer-version/observations")
	if err != nil {
		return computerversion.LiveConstructionObservation{}, fmt.Errorf("construction launcher: observation URL: %w", err)
	}
	query := endpoint.Query()
	query.Set("code_ref", string(request.Version.CodeRef))
	query.Set("artifact_program_ref", string(request.Version.ArtifactProgramRef))
	endpoint.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return computerversion.LiveConstructionObservation{}, err
	}
	req.Header.Set("X-Internal-Caller", "true")
	response, err := l.client.Do(req)
	if err != nil {
		return computerversion.LiveConstructionObservation{}, fmt.Errorf("construction launcher: product readback: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return computerversion.LiveConstructionObservation{}, fmt.Errorf("construction launcher: product readback status %s", response.Status)
	}
	var observation computerversion.LiveConstructionObservation
	decoder := json.NewDecoder(response.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&observation); err != nil {
		return computerversion.LiveConstructionObservation{}, fmt.Errorf("construction launcher: decode product readback: %w", err)
	}
	if observation.State.Version != request.Version {
		return computerversion.LiveConstructionObservation{}, fmt.Errorf("construction launcher: product readback ComputerVersion mismatch")
	}
	return observation, nil
}

func (l *VMConstructionLauncher) Commit(_ context.Context, boot computerversion.BootReceipt, version computerversion.ComputerVersion, disk diskinstantiation.Receipt) error {
	if l == nil || l.registry == nil || boot.VMID == "" {
		return fmt.Errorf("construction launcher: final lifecycle identity is required")
	}
	return l.registry.commitConstructedCandidate(boot.VMID, version, disk)
}

func (l *VMConstructionLauncher) Destroy(_ context.Context, boot computerversion.BootReceipt) error {
	if l == nil || l.registry == nil || strings.TrimSpace(boot.VMID) == "" {
		return fmt.Errorf("construction launcher: VM identity is required")
	}
	l.registry.mu.Lock()
	manager := l.registry.vmManager
	l.registry.mu.Unlock()
	if manager == nil {
		return fmt.Errorf("construction launcher: VM manager is unavailable")
	}
	if err := l.registry.markConstructedCandidateFailed(boot.VMID); err != nil {
		return err
	}
	if manager.GetVM(boot.VMID) != nil {
		if err := manager.StopVM(boot.VMID); err != nil {
			return err
		}
	}
	if err := manager.DestroyVMState(boot.VMID); err != nil {
		return err
	}
	return l.registry.removeConstructedCandidate(boot.VMID)
}
