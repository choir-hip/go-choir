package doctool

import (
	"context"
	"testing"
)

// The isolation self-test (internal/agentcore, no build tag) uses these on
// every platform; this file has no build tag either, so a darwin build of
// the tests fails if one becomes Linux-only again (review B1).
var _ func(context.Context, string, []string) SelfTestReceipt = SelfTest

func TestJudgeProbeIsPortable(t *testing.T) {
	if f := JudgeProbe(ProbeReport{}, nil); len(f) == 0 {
		t.Fatal("an empty report passed the judge")
	}
}
