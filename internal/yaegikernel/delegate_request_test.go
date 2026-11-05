package yaegikernel

import (
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// A Delegate's request fields count against the tray's byte budget, and a
// refused request stages nothing.
func TestDelegateRequestCountsAgainstTheTrayBudget(t *testing.T) {
	spec := DelegateSpec{Kind: TaskKindIntake, Objective: "o", Groups: [][]string{{"r1"}}}
	bare, withReq := &Tray{}, &Tray{}
	if _, err := bare.DelegateWithRequest(spec, nil); err != nil {
		t.Fatal(err)
	}
	p, w := 0.8, 0.4
	req := &types.RequestFields{PSuccess: &p, Weight: &w, Settlement: types.SettlementRecipient, Questions: []types.TypedQuestion{
		{Question: "Will it?", Answers: []string{"yes", "no"}, Probabilities: map[string]float64{"yes": 0.8, "no": 0.2}}}}
	if _, err := withReq.DelegateWithRequest(spec, req); err != nil {
		t.Fatal(err)
	}
	if withReq.bytes <= bare.bytes {
		t.Fatalf("request fields not counted: %d vs %d", withReq.bytes, bare.bytes)
	}
	bad := *req
	bad.Questions = []types.TypedQuestion{{Question: "q", Answers: []string{"only"}}}
	if _, err := (&Tray{}).DelegateWithRequest(spec, &bad); err == nil {
		t.Fatal("a malformed question was staged")
	}
}
