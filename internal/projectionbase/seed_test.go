package projectionbase

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

func TestFindLatestCompatibleBaseOrdersAndFilters(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	computerID := "computer-seed-order"
	digests := buildChainN(t, root, computerID, 3)
	key := fixtureKey(31)

	w1 := runRebuild(t, ctx, Config{
		ComputerID: computerID, TargetHead: digests[0], ArtifactsRoot: root,
		ScratchDir: t.TempDir(), KeyMaterial: key, BatchSize: 100,
		MemoryLimitRSS: 512 * 1024 * 1024, SeedPolicy: SeedNone, TargetSequence: 1,
	}, fixtureReplay(NewDiskEventSource(root, computerID, digests[0])))
	w3 := runRebuild(t, ctx, Config{
		ComputerID: computerID, TargetHead: digests[2], ArtifactsRoot: root,
		ScratchDir: t.TempDir(), KeyMaterial: key, BatchSize: 100,
		MemoryLimitRSS: 512 * 1024 * 1024, SeedPolicy: SeedNone, TargetSequence: 3,
	}, fixtureReplay(NewDiskEventSource(root, computerID, digests[2])))

	got, blobPath, ok, err := FindLatestCompatibleBase(root, computerID, digests[2], 3)
	if err != nil || !ok {
		t.Fatalf("latest base not found: ok=%t err=%v", ok, err)
	}
	if got.Sequence != 3 || got.BlobSHA256 != w3.Descriptor.BlobSHA256 {
		t.Fatalf("latest base = seq %d blob %s, want seq 3 blob %s", got.Sequence, got.BlobSHA256, w3.Descriptor.BlobSHA256)
	}
	if want := filepath.Join(root, "sha256", Namespace, w3.Descriptor.BlobSHA256); blobPath != want {
		t.Fatalf("latest blob path = %s, want %s", blobPath, want)
	}

	below, _, ok, err := FindLatestCompatibleBase(root, computerID, digests[1], 2)
	if err != nil || !ok {
		t.Fatalf("base below target not found: ok=%t err=%v", ok, err)
	}
	if below.Sequence != 1 || below.BlobSHA256 != w1.Descriptor.BlobSHA256 {
		t.Fatalf("base below target = seq %d blob %s, want seq 1 blob %s", below.Sequence, below.BlobSHA256, w1.Descriptor.BlobSHA256)
	}

	unfrozen, _, ok, err := FindLatestCompatibleBase(root, computerID, digests[2], 0)
	if err != nil || !ok || unfrozen.Sequence != 3 {
		t.Fatalf("unfrozen selection = seq %d ok=%t err=%v, want seq 3", unfrozen.Sequence, ok, err)
	}

	if _, _, ok, err := FindLatestCompatibleBase(root, computerID, strings.Repeat("9", 64), 1); err != nil || ok {
		t.Fatalf("foreign head at the same sequence admitted: ok=%t err=%v", ok, err)
	}
	if _, _, ok, err := FindLatestCompatibleBase(root, "computer-other", digests[2], 3); err != nil || ok {
		t.Fatalf("foreign computer base admitted: ok=%t err=%v", ok, err)
	}
}

func TestLoadPublishedBaseRefusesMissingAndForeign(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	computerID := "computer-seed-pinned-load"
	digests := buildChainN(t, root, computerID, 2)
	published := runRebuild(t, ctx, Config{
		ComputerID: computerID, TargetHead: digests[0], ArtifactsRoot: root,
		ScratchDir: t.TempDir(), KeyMaterial: fixtureKey(33), BatchSize: 100,
		MemoryLimitRSS: 512 * 1024 * 1024, SeedPolicy: SeedNone, TargetSequence: 1,
	}, fixtureReplay(NewDiskEventSource(root, computerID, digests[0])))

	descriptor, blobPath, err := LoadPublishedBase(root, computerID, published.Descriptor.BlobSHA256)
	if err != nil {
		t.Fatalf("published base load refused: %v", err)
	}
	if descriptor.Sequence != 1 || blobPath != published.BlobPath {
		t.Fatalf("loaded base = (seq %d, %s), want (1, %s)", descriptor.Sequence, blobPath, published.BlobPath)
	}

	if _, _, err := LoadPublishedBase(root, computerID, strings.Repeat("0", 64)); !errors.Is(err, ErrBaseRefused) {
		t.Fatalf("missing sidecar did not refuse: %v", err)
	}
	if _, _, err := LoadPublishedBase(root, "computer-other", published.Descriptor.BlobSHA256); !errors.Is(err, ErrBaseRefused) {
		t.Fatalf("foreign computer did not refuse: %v", err)
	}
	if _, _, err := LoadPublishedBase(root, computerID, "not-a-digest"); !errors.Is(err, ErrBaseRefused) {
		t.Fatalf("malformed ref did not refuse: %v", err)
	}
	if err := os.Remove(blobPath); err != nil {
		t.Fatal(err)
	}
	if _, _, err := LoadPublishedBase(root, computerID, published.Descriptor.BlobSHA256); !errors.Is(err, ErrBaseRefused) {
		t.Fatalf("missing blob did not refuse: %v", err)
	}
}

func TestLoadPublishedBaseRefusesIncompatibleVocabulary(t *testing.T) {
	root := t.TempDir()
	computerID := "computer-seed-vocab"
	ref := strings.Repeat("b", 64)
	descriptor := validTestDescriptor()
	descriptor.ComputerID = computerID
	descriptor.Sequence = 4
	descriptor.BlobSHA256 = ref
	descriptor.VocabularyVersion = "v9"
	raw, err := MarshalDescriptor(descriptor)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "sha256", Namespace)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, DescriptorSidecarName(ref)), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := LoadPublishedBase(root, computerID, ref); !errors.Is(err, ErrBaseRefused) {
		t.Fatalf("unknown vocabulary admitted: %v", err)
	}
	if _, _, ok, err := FindLatestCompatibleBase(root, computerID, strings.Repeat("a", 64), 4); err != nil || ok {
		t.Fatalf("unknown vocabulary admitted by discovery: ok=%t err=%v", ok, err)
	}
}

func TestDiskEventSourceRefusesUnverifiableReceipts(t *testing.T) {
	disk := NewDiskEventSource(t.TempDir(), "computer-disk-verify")
	err := disk.VerifyEventHeadReceipt(context.Background(), computerevent.Receipt{
		ReceiptKind: "EventHeadReceipt",
		Issuer:      "corpusd",
		KindFields:  map[string]any{"event_digest": strings.Repeat("a", 64)},
	}, computerevent.CASRequest{})
	if !errors.Is(err, ErrReceiptUnverifiable) {
		t.Fatalf("disk source admitted an unverifiable receipt: %v", err)
	}
}

func TestPinnedReceiptKeyResolverVerifiesSignedReceipt(t *testing.T) {
	ctx := context.Background()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	otherPub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signingKey := computerevent.SigningKey{
		SignerRef:  computerevent.SignerRef{SignerDomain: "platform-control", KeyID: "corpusd-test-key"},
		PrivateKey: priv,
	}
	computerID := "computer-receipt-trust"
	request := computerevent.CASRequest{
		Event: computerevent.Event{
			ComputerID:        computerID,
			Sequence:          1,
			PreviousHead:      computerevent.ZeroHead,
			EventKind:         computerevent.EventGenesisImported,
			RequestCommitment: strings.Repeat("1", 64),
		},
		EventDigest:              strings.Repeat("2", 64),
		EventPinReceiptDigest:    strings.Repeat("3", 64),
		PayloadPinReceiptDigests: []string{strings.Repeat("4", 64)},
		Next: computerevent.Head{
			DesiredEventHead:         strings.Repeat("5", 64),
			EffectiveEventHead:       strings.Repeat("6", 64),
			DesiredStateCommitment:   strings.Repeat("7", 64),
			EffectiveStateCommitment: strings.Repeat("8", 64),
		},
	}
	fields := map[string]any{
		"computer_id":                request.Event.ComputerID,
		"previous_head":              request.Event.PreviousHead,
		"event_digest":               request.EventDigest,
		"sequence":                   request.Event.Sequence,
		"event_kind":                 request.Event.EventKind,
		"request_commitment":         request.Event.RequestCommitment,
		"pin_receipt_digests":        append([]string{request.EventPinReceiptDigest}, request.PayloadPinReceiptDigests...),
		"desired_event_head":         request.Next.DesiredEventHead,
		"effective_event_head":       request.Next.EffectiveEventHead,
		"pending_transition_ref":     request.Next.PendingTransitionRef,
		"desired_state_commitment":   request.Next.DesiredStateCommitment,
		"effective_state_commitment": request.Next.EffectiveStateCommitment,
	}
	receipt, err := computerevent.NewSignedReceipt("EventHeadReceipt", "corpusd", fields, []computerevent.SigningKey{signingKey}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}

	verifier := computerevent.EventHeadReceiptVerifier{Keys: PinnedReceiptKeyResolver{
		SignerDomain: "platform-control",
		KeyID:        signingKey.KeyID,
		PublicKey:    pub,
	}}
	if err := verifier.VerifyEventHeadReceipt(ctx, receipt, request); err != nil {
		t.Fatalf("pinned trust refused a valid signed receipt: %v", err)
	}

	anyKeyID := computerevent.EventHeadReceiptVerifier{Keys: PinnedReceiptKeyResolver{
		SignerDomain: "platform-control",
		PublicKey:    pub,
	}}
	if err := anyKeyID.VerifyEventHeadReceipt(ctx, receipt, request); err != nil {
		t.Fatalf("pinned key material refused a valid signed receipt: %v", err)
	}

	wrongDomain := computerevent.EventHeadReceiptVerifier{Keys: PinnedReceiptKeyResolver{
		SignerDomain: "other-domain",
		KeyID:        signingKey.KeyID,
		PublicKey:    pub,
	}}
	if err := wrongDomain.VerifyEventHeadReceipt(ctx, receipt, request); err == nil {
		t.Fatal("untrusted signer domain admitted")
	}

	wrongKey := computerevent.EventHeadReceiptVerifier{Keys: PinnedReceiptKeyResolver{
		SignerDomain: "platform-control",
		KeyID:        signingKey.KeyID,
		PublicKey:    otherPub,
	}}
	if err := wrongKey.VerifyEventHeadReceipt(ctx, receipt, request); err == nil {
		t.Fatal("wrong pinned key admitted")
	}

	missing := computerevent.EventHeadReceiptVerifier{Keys: PinnedReceiptKeyResolver{SignerDomain: "platform-control"}}
	if err := missing.VerifyEventHeadReceipt(ctx, receipt, request); !errors.Is(err, ErrReceiptUnverifiable) {
		t.Fatalf("missing pinned key did not fail closed: %v", err)
	}

	tampered := receipt
	tampered.SignatureSet = append([]computerevent.Signature(nil), receipt.SignatureSet...)
	tampered.SignatureSet[0].Signature = ""
	if err := verifier.VerifyEventHeadReceipt(ctx, tampered, request); err == nil {
		t.Fatal("tampered signature admitted")
	}
}
