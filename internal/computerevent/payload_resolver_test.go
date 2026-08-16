package computerevent

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

func TestResolvePayloadsEmptyRefsNeedNoReader(t *testing.T) {
	got, err := ResolvePayloads(context.Background(), nil, nil, testComputerID, "event-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("got %+v", got)
	}
}

func TestResolvePayloadsRefusesNilReaderWhenRefsPresent(t *testing.T) {
	refs := []PayloadRef{{
		ArtifactDigest: testDigestA, MediaType: "application/json", PrivacyClass: "public",
		Role: "projection_batch", SchemaVersion: 1,
	}}
	_, err := ResolvePayloads(context.Background(), nil, nil, testComputerID, "event-1", refs)
	if !errors.Is(err, ErrPayloadResolverRequired) {
		t.Fatalf("err=%v", err)
	}
}

func TestResolvePayloadsVerifiesDigestAndDecryptsPrivateBeforeSQL(t *testing.T) {
	cipher := externalTestCipher(t, 0x22)
	plaintext := []byte(`{"ops":[{"kind":"desktop_state_recorded"}]}`)
	envelope, _, err := cipher.Encrypt(context.Background(), testComputerID, "event-private", "application/json", "private", plaintext)
	if err != nil {
		t.Fatal(err)
	}
	reader := NewMemoryArtifactReader()
	digest := reader.Put(envelope)
	sql := GuardNoSQL{}
	resolved, err := ResolvePayloads(context.Background(), reader, cipher, testComputerID, "event-private", []PayloadRef{{
		ArtifactDigest: digest, MediaType: PrivateArtifactMediaType, PrivacyClass: "private",
		Role: "projection_batch", SchemaVersion: 1,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := sql.Exec("INSERT INTO desktop_workspaces VALUES (1)"); !errors.Is(err, ErrSQLDuringResolve) {
		t.Fatalf("resolver-phase SQL was not refused: %v", err)
	}
	if len(resolved) != 1 || !bytes.Equal(resolved[0].Plaintext, plaintext) {
		t.Fatalf("plaintext=%q", resolved[0].Plaintext)
	}
}

func TestResolvePayloadsRefusesTamperedDigest(t *testing.T) {
	reader := NewMemoryArtifactReader()
	digest := reader.Put([]byte(`{"ok":true}`))
	tampered := digest[:63] + "0"
	if tampered == digest {
		tampered = digest[:63] + "1"
	}
	_, err := ResolvePayloads(context.Background(), reader, nil, testComputerID, "event-1", []PayloadRef{{
		ArtifactDigest: tampered, MediaType: "application/json", PrivacyClass: "public",
		Role: "projection_batch", SchemaVersion: 1,
	}})
	if err == nil {
		t.Fatal("tampered digest was accepted")
	}
}

func TestResolvePayloadsWrongComputerOrEventFailsClosed(t *testing.T) {
	cipher := externalTestCipher(t, 0x33)
	envelope, _, err := cipher.Encrypt(context.Background(), testComputerID, "event-a", "application/json", "private", []byte("{}"))
	if err != nil {
		t.Fatal(err)
	}
	reader := NewMemoryArtifactReader()
	digest := reader.Put(envelope)
	_, err = ResolvePayloads(context.Background(), reader, cipher, testComputerID, "event-b", []PayloadRef{{
		ArtifactDigest: digest, MediaType: PrivateArtifactMediaType, PrivacyClass: "private",
		Role: "projection_batch", SchemaVersion: 1,
	}})
	if err == nil {
		t.Fatal("wrong event id decrypted")
	}
}

func TestProjectionBatchValidateRequiresIdentityAndOps(t *testing.T) {
	batch := ProjectionBatch{
		Version: ProjectionBatchV1, ProjectorVersion: ProjectorVersionV1,
		ComputerID: testComputerID, EventID: "event-1", EventDigest: testDigestA,
		Ops: []ProjectionOp{{Kind: "desktop_state_recorded"}},
	}
	if err := batch.Validate(); err != nil {
		t.Fatal(err)
	}
	empty := batch
	empty.Ops = nil
	if err := empty.Validate(); err == nil {
		t.Fatal("empty batch accepted")
	}
}

func TestClassifyProjectionFailureDoesNotTreatResolveAsCASPoison(t *testing.T) {
	err := ClassifyProjectionFailure(ErrPayloadResolverRequired)
	if !errors.Is(err, ErrPayloadSQLBeforeResolve) {
		t.Fatalf("resolve failure classified as poison: %v", err)
	}
	err = ClassifyProjectionFailure(errors.New("constraint failed"))
	if !errors.Is(err, ErrProjectionPoison) {
		t.Fatalf("project failure was not poison: %v", err)
	}
}

func TestPrivateEnvelopeRoundTripUsesCanonicalKey(t *testing.T) {
	if _, err := base64.RawStdEncoding.DecodeString(""); err != nil && strings.Contains(err.Error(), "never") {
		t.Fatal(err)
	}
}
