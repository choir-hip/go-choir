package gateway

import (
	"context"
	"testing"
	"time"
)

// Review B3: the op-ed gate's judgments have their own bounded per-computer
// bucket (contract 3.4), so a gate burst cannot spend the computer's share of
// intake labels and the gate is itself bounded. An unknown scope is still refused.
func TestJevOpedGateScopeHasItsOwnBoundedBucket(t *testing.T) {
	if class, err := judgmentRateScope(JudgmentScopeOpedGate); err != nil || class != jevOpedGate {
		t.Fatalf("oped_gate scope = %v %v", class, err)
	}
	if _, err := judgmentRateScope("judgments:nothing"); err == nil {
		t.Fatal("an unknown scope was accepted")
	}
	l := testLimiter(32, 8, 100)
	l.rate = 1000 // the global bucket is not the constraint here
	var rels []func()
	for i := 0; i < int(jevOpedGateBurst); i++ {
		rel, _, err := l.acquire(context.Background(), "paper", jevOpedGate)
		if err != nil {
			t.Fatalf("gate call %d inside the burst: %v", i, err)
		}
		rel()
		rels = append(rels, rel)
	}
	// The next gate call on this computer waits out the bucket and is refused at the queue bound.
	if _, _, err := l.acquire(context.Background(), "paper", jevOpedGate); err == nil {
		t.Fatal("the gate bucket did not bound the computer's gate calls")
	}
	// The same computer's other bulk calls, and another computer's gate calls, are not touched.
	if rel, _, err := l.acquire(context.Background(), "paper", jevBulk); err != nil {
		t.Fatalf("a gate burst starved the computer's bulk calls: %v", err)
	} else {
		rel()
	}
	if rel, _, err := l.acquire(context.Background(), "other", jevOpedGate); err != nil {
		t.Fatalf("another computer's gate call: %v", err)
	} else {
		rel()
	}
	// In flight is bounded too.
	l2 := testLimiter(32, 8, 100)
	l2.rate = 1000
	var held []func()
	for i := 0; i < jevOpedGateInFlight; i++ {
		rel, _, err := l2.acquire(context.Background(), "paper", jevOpedGate)
		if err != nil {
			t.Fatal(err)
		}
		held = append(held, rel)
	}
	start := time.Now()
	if _, _, err := l2.acquire(context.Background(), "paper", jevOpedGate); err == nil || time.Since(start) < 30*time.Millisecond {
		t.Fatalf("a gate call beyond the in-flight bound was admitted: %v", err)
	}
	for _, r := range held {
		r()
	}
}
