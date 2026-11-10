package auth

import (
	"context"
	"testing"
	"time"
)

func TestSweeperDeletesExpiredChallengesOnly(t *testing.T) {
	h, _ := emailEnv(t)
	if _, err := h.store.CreateUser("sweep-user", "sweep@example.com"); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	for id, exp := range map[string]time.Time{"sweep-old": now.Add(-5 * time.Minute), "sweep-live": now.Add(5 * time.Minute)} {
		if err := h.store.SaveChallengeState(&ChallengeState{ID: id, UserID: "sweep-user", Challenge: "c", Type: "registration", CreatedAt: now.Add(-10 * time.Minute), ExpiresAt: exp}); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	h.StartDeviceSweeper(ctx, 10*time.Millisecond)
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := h.store.GetChallengeStateByID("sweep-old"); err != nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("expired challenge was never swept")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if _, err := h.store.GetChallengeStateByID("sweep-live"); err != nil {
		t.Fatalf("live challenge was swept: %v", err)
	}
}
