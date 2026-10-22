//go:build !linux

package capsule

import "context"

// SeccompSelfTest: seccomp is Linux's; a dev host reports that it probed
// nothing, which the judge refuses.
func SeccompSelfTest(ctx context.Context, bin string) SeccompSelfTestReceipt {
	rec := SeccompSelfTestReceipt{Kind: "capsule_seccomp_selftest", Failures: []string{"capsule seccomp needs Linux"}}
	return rec
}

// RunSeccompSelfTestChild: nothing to load off Linux.
func RunSeccompSelfTestChild(args []string) int { return 2 }
