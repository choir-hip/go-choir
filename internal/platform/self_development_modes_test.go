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

// legacy stored values are plain strings now; rows that carry them are never
// rewritten (docs/design/selfdev-apply-and-revert-2026-11-06.md 3.1).
const (
	testLegacyProposeOnly        = "propose_only"
	testLegacyAcceptOnce         = "accept_once"
	testLegacyQualifiedConsensus = "qualified_consensus"
	testLegacyStandingConsensus  = "standing_consensus"
	testLegacyAuditOnly          = "audit_only"
)

func TestCanonicalSelfDevelopmentModeNormalisesEveryLegacyValueAndNoRow(t *testing.T) {
	for stored, want := range map[string]string{
		"off":                        SelfDevelopmentModeOff,
		testLegacyAuditOnly:          SelfDevelopmentModeOff,
		"apply":                      SelfDevelopmentModeApply,
		testLegacyProposeOnly:        SelfDevelopmentModeApply,
		testLegacyAcceptOnce:         SelfDevelopmentModeApply,
		testLegacyQualifiedConsensus: SelfDevelopmentModeApply,
		testLegacyStandingConsensus:  SelfDevelopmentModeApply,
		"":                           SelfDevelopmentModeApply, // no row
		" propose_only ":             SelfDevelopmentModeApply,
		"something_new":              SelfDevelopmentModeOff, // unknown fails closed
	} {
		if got := CanonicalSelfDevelopmentMode(stored); got != want {
			t.Errorf("CanonicalSelfDevelopmentMode(%q) = %q, want %q", stored, got, want)
		}
	}
}

func TestDefaultApplyModeDigestIsTheDocumentedConstant(t *testing.T) {
	want := computerevent.DigestBytes([]byte("selfdev-mode:default-apply:0"))
	if got := DefaultApplyModeDigest(); got != want || !computerevent.IsSHA256(got) {
		t.Fatalf("DefaultApplyModeDigest = %q, want %q", got, want)
	}
}

func TestValidateSelfDevelopmentModeTransitionAdmitsOnlyOffAndApplyWithoutBindings(t *testing.T) {
	current := SelfDevelopmentMode{ComputerID: "computer-test", Mode: SelfDevelopmentModeOff}
	for _, mode := range []string{SelfDevelopmentModeOff, SelfDevelopmentModeApply} {
		request := SetSelfDevelopmentModeRequest{Mode: mode}
		if _, _, err := validateSelfDevelopmentModeTransition(current, request, time.Now()); err != nil {
			t.Fatalf("unbound %s refused: %v", mode, err)
		}
		request.OperationID = "forbidden"
		if _, _, err := validateSelfDevelopmentModeTransition(current, request, time.Now()); err == nil {
			t.Fatalf("bound %s accepted", mode)
		}
		request = SetSelfDevelopmentModeRequest{Mode: mode, PolicyDigest: strings.Repeat("a", 64)}
		if _, _, err := validateSelfDevelopmentModeTransition(current, request, time.Now()); err == nil {
			t.Fatalf("%s with a policy digest accepted", mode)
		}
	}
	for _, retired := range []string{testLegacyAuditOnly, testLegacyProposeOnly, testLegacyAcceptOnce, testLegacyQualifiedConsensus, testLegacyStandingConsensus, "bogus"} {
		if _, _, err := validateSelfDevelopmentModeTransition(current, SetSelfDevelopmentModeRequest{Mode: retired}, time.Now()); err == nil {
			t.Fatalf("retired mode %q is writable", retired)
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

func newTestModeCAS(t *testing.T) (*SelfDevelopmentModeCAS, *Store) {
	t.Helper()
	store, _ := openTestPlatformStore(t)
	t.Cleanup(func() { store.Close() })
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
	return authority, store
}

func TestSelfDevelopmentModeCASPersistsGenerationAndIdempotentReceipt(t *testing.T) {
	authority, _ := newTestModeCAS(t)
	request := SetSelfDevelopmentModeRequest{Mode: SelfDevelopmentModeApply, ExpectedGeneration: 0, IdempotencyKey: "mode-idem-1"}
	first, err := authority.Set(context.Background(), "computer-mode-test", request)
	if err != nil {
		t.Fatal(err)
	}
	if first.Generation != 1 || first.Receipt == nil || first.Mode != SelfDevelopmentModeApply {
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

func TestSelfDevelopmentModeGetNoRowIsApplyGenerationZeroAndNeverExpires(t *testing.T) {
	authority, store := newTestModeCAS(t)
	ctx := context.Background()
	mode, err := authority.Get(ctx, "computer-no-row")
	if err != nil || mode.Mode != SelfDevelopmentModeApply || mode.Generation != 0 || mode.Receipt != nil {
		t.Fatalf("no-row mode = %+v, %v; want apply generation 0 with no receipt", mode, err)
	}
	// A legacy accept_once row whose window passed long ago reads apply: the
	// lazy expiry is deleted and the row is never rewritten.
	insertLegacyModeRow(t, authority, store, "computer-legacy-once", testLegacyAcceptOnce)
	authority.now = func() time.Time { return time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC) }
	legacy, err := authority.Get(ctx, "computer-legacy-once")
	if err != nil || legacy.Mode != SelfDevelopmentModeApply || legacy.Generation != 1 || legacy.Receipt == nil {
		t.Fatalf("legacy accept_once row = %+v, %v; want apply generation 1", legacy, err)
	}
	var raw string
	if err := store.db.QueryRowContext(ctx, `SELECT mode FROM computer_self_development_modes WHERE computer_id=?`, "computer-legacy-once").Scan(&raw); err != nil || raw != testLegacyAcceptOnce {
		t.Fatalf("stored row = %q, %v; want it left as accept_once", raw, err)
	}
	// audit_only reads off.
	insertLegacyModeRow(t, authority, store, "computer-legacy-audit", testLegacyAuditOnly)
	audit, err := authority.Get(ctx, "computer-legacy-audit")
	if err != nil || audit.Mode != SelfDevelopmentModeOff {
		t.Fatalf("legacy audit_only row = %+v, %v; want off", audit, err)
	}
}

// insertLegacyModeRow writes the row an old build would have: the raw stored
// mode under a receipt corpusd signed. It first writes a real receipt through
// the CAS (as apply) and then rewrites only the mode column.
func insertLegacyModeRow(t *testing.T, authority *SelfDevelopmentModeCAS, store *Store, computerID, rawMode string) {
	t.Helper()
	if _, err := authority.Set(context.Background(), computerID, SetSelfDevelopmentModeRequest{Mode: SelfDevelopmentModeApply, ExpectedGeneration: 0, IdempotencyKey: "legacy-" + computerID}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(context.Background(), `UPDATE computer_self_development_modes SET mode=? WHERE computer_id=?`, rawMode, computerID); err != nil {
		t.Fatal(err)
	}
}

func TestSelfDevelopmentModeWriterIsOwnerOnlyAndRetiredModesAreRefused(t *testing.T) {
	authority, _ := newTestModeCAS(t)
	ctx := context.Background()
	for _, retired := range []string{testLegacyAuditOnly, testLegacyProposeOnly, testLegacyAcceptOnce, testLegacyQualifiedConsensus, testLegacyStandingConsensus} {
		_, err := authority.Set(ctx, "computer-retired", SetSelfDevelopmentModeRequest{Mode: retired, ExpectedGeneration: 0, IdempotencyKey: "k-" + retired})
		if err == nil {
			t.Fatalf("retired mode %q was written", retired)
		}
	}
	// A writer other than the host-internal owner write is refused.
	if _, err := authority.set(ctx, "computer-retired", SetSelfDevelopmentModeRequest{Mode: SelfDevelopmentModeOff, ExpectedGeneration: 0, IdempotencyKey: "guest"}, "guest-consumption", nil); !errors.Is(err, ErrSelfDevelopmentModeOwnerRequired) {
		t.Fatalf("non-owner writer error = %v", err)
	}
}
