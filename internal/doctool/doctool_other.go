//go:build !linux

package doctool

import (
	"context"
	"fmt"
)

// The sandbox is Linux only: elsewhere observe runs the tool directly with
// the fallback warning and required refuses.
func runSandboxed(context.Context, runOptions, bool, string, string, []string, string, []byte, []string) (string, []string, int64, error) {
	return "", nil, 0, setupErrorf("the document-tool sandbox needs Linux")
}

// EnsureToolCgroupRoot needs Linux.
func EnsureToolCgroupRoot(string) error {
	return fmt.Errorf("the document tools cgroup needs Linux")
}

// RunHelper is never started off Linux.
func RunHelper([]string) int { return helperSetupExit }

// RunProbe is never started off Linux.
func RunProbe([]string) int { return 2 }

// SelfTest fails closed off Linux: there is no sandbox to prove.
func SelfTest(context.Context, string, []string) SelfTestReceipt {
	return SelfTestReceipt{Mode: ModeFromEnv(nil), Failures: []string{"the document-tool sandbox needs Linux"}}
}
