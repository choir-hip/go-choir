package platform

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

func TestValidateSelfDevelopmentModeTransitionRequiresExactAcceptOnceBindings(t *testing.T) {
	now := time.Date(2026, 7, 18, 23, 0, 0, 0, time.UTC)
	current := SelfDevelopmentMode{ComputerID: "computer-test", Mode: SelfDevelopmentModeProposeOnly, Generation: 4}
	valid := SetSelfDevelopmentModeRequest{
		Mode: SelfDevelopmentModeAcceptOnce, OperationID: "operation-1",
		BundleDigest: strings.Repeat("a", 64), ExpectedDesiredEventHead: strings.Repeat("b", 64),
		ExpectedEffectiveEventHead: strings.Repeat("c", 64), ExpectedDesiredStateCommitment: strings.Repeat("d", 64),
		ExpectedEffectiveStateCommitment: strings.Repeat("e", 64), ExpiresAt: now.Add(time.Minute).Format(time.RFC3339Nano),
	}
	next, expiry, err := validateSelfDevelopmentModeTransition(current, valid, now)
	if err != nil {
		t.Fatal(err)
	}
	if next.Mode != SelfDevelopmentModeAcceptOnce || next.OperationID != valid.OperationID || expiry == nil {
		t.Fatalf("accept_once transition = %+v expiry=%v", next, expiry)
	}
	for name, mutate := range map[string]func(*SetSelfDevelopmentModeRequest){
		"operation":            func(r *SetSelfDevelopmentModeRequest) { r.OperationID = "" },
		"bundle":               func(r *SetSelfDevelopmentModeRequest) { r.BundleDigest = "bad" },
		"desired head":         func(r *SetSelfDevelopmentModeRequest) { r.ExpectedDesiredEventHead = "" },
		"effective commitment": func(r *SetSelfDevelopmentModeRequest) { r.ExpectedEffectiveStateCommitment = "" },
		"expiry":               func(r *SetSelfDevelopmentModeRequest) { r.ExpiresAt = now.Format(time.RFC3339Nano) },
	} {
		t.Run(name, func(t *testing.T) {
			invalid := valid
			mutate(&invalid)
			if _, _, err := validateSelfDevelopmentModeTransition(current, invalid, now); err == nil {
				t.Fatalf("invalid accept_once request accepted: %+v", invalid)
			}
		})
	}
}

func TestValidateSelfDevelopmentModeTransitionForbidsBindingsOutsideAcceptOnce(t *testing.T) {
	current := SelfDevelopmentMode{ComputerID: "computer-test", Mode: SelfDevelopmentModeOff}
	for _, mode := range []string{SelfDevelopmentModeOff, SelfDevelopmentModeAuditOnly, SelfDevelopmentModeProposeOnly} {
		request := SetSelfDevelopmentModeRequest{Mode: mode}
		if _, _, err := validateSelfDevelopmentModeTransition(current, request, time.Now()); err != nil {
			t.Fatalf("unbound %s refused: %v", mode, err)
		}
		request.OperationID = "forbidden"
		if _, _, err := validateSelfDevelopmentModeTransition(current, request, time.Now()); err == nil {
			t.Fatalf("bound %s accepted", mode)
		}
	}
}

func TestSelfDevelopmentModeRequestCommitmentBindsTargetAndIdempotency(t *testing.T) {
	request := SetSelfDevelopmentModeRequest{Mode: SelfDevelopmentModeOff, ExpectedGeneration: 2, IdempotencyKey: "idem-1"}
	first, err := selfDevelopmentModeRequestCommitment("computer-a", request)
	if err != nil {
		t.Fatal(err)
	}
	retry, err := selfDevelopmentModeRequestCommitment("computer-a", request)
	if err != nil || first != retry {
		t.Fatalf("retry commitment = %q, %v; want %q", retry, err, first)
	}
	other, err := selfDevelopmentModeRequestCommitment("computer-b", request)
	if err != nil || first == other {
		t.Fatal("request commitment did not bind ComputerID")
	}
}

func TestSelfDevelopmentModeCASPersistsGenerationAndIdempotentReceipt(t *testing.T) {
	store, _ := openTestPlatformStore(t)
	defer store.Close()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	authority, err := NewSelfDevelopmentModeCAS(store, computerevent.SigningKey{
		SignerRef:  computerevent.SignerRef{SignerDomain: "platform-control", KeyID: "mode-test"},
		PrivateKey: privateKey,
	})
	if err != nil {
		t.Fatal(err)
	}
	authority.now = func() time.Time { return time.Date(2026, 7, 18, 23, 0, 0, 0, time.UTC) }
	request := SetSelfDevelopmentModeRequest{Mode: SelfDevelopmentModeProposeOnly, ExpectedGeneration: 0, IdempotencyKey: "mode-idem-1"}
	first, err := authority.Set(context.Background(), "computer-mode-test", request)
	if err != nil {
		t.Fatal(err)
	}
	if first.Generation != 1 || first.Receipt == nil {
		t.Fatalf("first mode = %+v", first)
	}
	retry, err := authority.Set(context.Background(), "computer-mode-test", request)
	if err != nil {
		t.Fatal(err)
	}
	firstReceipt, firstErr := first.Receipt.CanonicalBytes()
	retryReceipt, retryErr := retry.Receipt.CanonicalBytes()
	if firstErr != nil || retryErr != nil || !bytes.Equal(firstReceipt, retryReceipt) {
		t.Fatalf("retry changed durable receipt: first=%s retry=%s errors=%v/%v", firstReceipt, retryReceipt, firstErr, retryErr)
	}
	conflict := request
	conflict.Mode = SelfDevelopmentModeOff
	if _, err := authority.Set(context.Background(), "computer-mode-test", conflict); !errors.Is(err, ErrSelfDevelopmentModeConflict) {
		t.Fatalf("changed idempotent request error = %v", err)
	}
}
