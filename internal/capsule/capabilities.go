//go:build linux


package capsule

import (
	"fmt"

	"github.com/moby/sys/capability"
)

// DropCapabilities drops Linux capabilities that are not needed by the
// broker or workload. This is defense-in-depth on top of seccomp + Landlock.
//
// The broker retains: CAP_DAC_OVERRIDE, CAP_FOWNER (needed for overlayfs
// copy-up as root in user namespace), CAP_CHOWN, CAP_SETUID, CAP_SETGID
// (for privilege separation).
//
// The workload retains: CAP_DAC_OVERRIDE, CAP_FOWNER (overlayfs copy-up).
// All other caps are dropped.
//
// See design doc Q46: CLONE_NEWUSER is for broker privilege separation;
// workload retains root for overlayfs copy-up.

// DropBrokerCapabilities drops capabilities not needed by the broker.
func DropBrokerCapabilities() error {
	caps, err := capability.NewPid(0)
	if err != nil {
		return fmt.Errorf("failed to get current capabilities: %w", err)
	}

	// Clear all capabilities first.
	caps.Clear(capability.CAPS | capability.BOUNDS)

	// Set the capabilities the broker needs.
	needed := []capability.Cap{
		capability.CAP_DAC_OVERRIDE, // overlayfs copy-up
		capability.CAP_FOWNER,       // overlayfs copy-up
		capability.CAP_CHOWN,        // file ownership changes
		capability.CAP_SETUID,       // privilege separation
		capability.CAP_SETGID,       // privilege separation
	}

	for _, cap := range needed {
		caps.Set(capability.PERMITTED, cap)
		caps.Set(capability.EFFECTIVE, cap)
		caps.Set(capability.INHERITABLE, cap)
	}

	// Set bounding set (limits what can be re-acquired).
	if err := caps.Apply(capability.CAPS | capability.BOUNDS); err != nil {
		return fmt.Errorf("failed to apply capability restrictions: %w", err)
	}

	return nil
}

// DropWorkloadCapabilities drops capabilities not needed by the workload.
// The workload retains CAP_DAC_OVERRIDE and CAP_FOWNER for overlayfs copy-up.
func DropWorkloadCapabilities() error {
	caps, err := capability.NewPid(0)
	if err != nil {
		return fmt.Errorf("failed to get current capabilities: %w", err)
	}

	// Clear all capabilities first.
	caps.Clear(capability.CAPS | capability.BOUNDS)

	// Set the capabilities the workload needs.
	needed := []capability.Cap{
		capability.CAP_DAC_OVERRIDE, // overlayfs copy-up
		capability.CAP_FOWNER,       // overlayfs copy-up
	}

	for _, cap := range needed {
		caps.Set(capability.PERMITTED, cap)
		caps.Set(capability.EFFECTIVE, cap)
		caps.Set(capability.INHERITABLE, cap)
	}

	if err := caps.Apply(capability.CAPS | capability.BOUNDS); err != nil {
		return fmt.Errorf("failed to apply capability restrictions: %w", err)
	}

	return nil
}
