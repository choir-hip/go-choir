//go:build !linux

package shotsandbox

import (
	"context"
	"errors"
)

// MaybeRunStage is a no-op off the guest kernel.
func MaybeRunStage() {}

// Launch needs the Linux guest kernel's namespaces and seccomp.
func Launch(ctx context.Context, exe string, spec Spec) (Result, error) {
	return Result{}, errors.New("shot sandbox: requires the Linux guest kernel")
}
