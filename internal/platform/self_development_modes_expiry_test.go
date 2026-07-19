package platform

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

func TestSelfDevelopmentModeGetDurablyExpiresAcceptOnce(t *testing.T) {
	store, _ := openTestPlatformStore(t)
	defer store.Close()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	authority, err := NewSelfDevelopmentModeCAS(store, computerevent.SigningKey{
		SignerRef: computerevent.SignerRef{SignerDomain: "platform-control", KeyID: "mode-expiry-test"}, PrivateKey: privateKey,
	})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 19, 1, 0, 0, 0, time.UTC)
	authority.now = func() time.Time { return now }
	digest := strings.Repeat("a", 64)
	pending := ""
	accepted, err := authority.Set(context.Background(), "computer-expiry-test", SetSelfDevelopmentModeRequest{
		Mode: SelfDevelopmentModeAcceptOnce, ExpectedGeneration: 0, IdempotencyKey: "enable-once",
		OperationID: "operation-1", BundleDigest: digest,
		ExpectedDesiredEventHead: digest, ExpectedEffectiveEventHead: digest,
		ExpectedPendingTransitionRef:   &pending,
		ExpectedDesiredStateCommitment: digest, ExpectedEffectiveStateCommitment: digest,
		ExpiresAt: now.Add(time.Minute).Format(time.RFC3339Nano),
	})
	if err != nil || accepted.Mode != SelfDevelopmentModeAcceptOnce {
		t.Fatalf("enable accept_once = %+v, %v", accepted, err)
	}
	authority.now = func() time.Time { return now.Add(2 * time.Minute) }
	expired, err := authority.Get(context.Background(), "computer-expiry-test")
	if err != nil {
		t.Fatal(err)
	}
	if expired.Mode != SelfDevelopmentModeOff || expired.Generation != 2 || expired.Receipt == nil {
		t.Fatalf("expired mode = %+v", expired)
	}
	if expired.Receipt.KindFields["consumed_expires_at"] != accepted.ExpiresAt {
		t.Fatalf("expiry receipt lost original accept_once deadline: %+v", expired.Receipt.KindFields)
	}
	reloaded, err := authority.Get(context.Background(), "computer-expiry-test")
	if err != nil || reloaded.Mode != SelfDevelopmentModeOff || reloaded.Generation != expired.Generation {
		t.Fatalf("reloaded expired mode = %+v, %v", reloaded, err)
	}
}
