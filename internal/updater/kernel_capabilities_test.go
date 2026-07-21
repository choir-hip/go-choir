package updater

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/computerversion"
)

func TestKernelCapabilityReceiptBindsIdentityAndFailsClosed(t *testing.T) {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	key := computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: "guest-core", KeyID: "guest-test"}, PrivateKey: privateKey}
	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	version := computerversion.ComputerVersion{CodeRef: computerversion.CodeRef("code:sha256:" + strings.Repeat("a", 64)), ArtifactProgramRef: computerversion.ArtifactProgramRef("artifact-program:sha256:" + strings.Repeat("b", 64))}
	capabilities := make(map[string]KernelCapabilityObservation)
	for _, name := range MandatoryKernelCapabilities() {
		capabilities[name] = KernelCapabilityObservation{Supported: true, Enforced: true, ObservationRef: "sha256:" + strings.Repeat("c", 64)}
	}
	probe := KernelCapabilityProbe{
		KernelRelease: "6.18.21", BootID: "boot-1", BootParameters: "lsm=landlock,yama,bpf",
		CgroupFilesystem: "cgroup2", OverlayModuleDigest: strings.Repeat("d", 64),
		ObservedAt:   now.Format(time.RFC3339Nano),
		Capabilities: capabilities, ContractDigest: strings.Repeat("e", 64),
	}
	report, err := NewKernelCapabilityReport(context.Background(), KernelCapabilityIdentity{
		ComputerID: "computer-1", RealizationID: "realization-1",
		GuestImageDigest: strings.Repeat("f", 64), KernelConfigDigest: strings.Repeat("1", 64), LifecycleGeneration: 4,
	}, KernelCapabilityRequest{ComputerVersion: version, ReleaseDigest: strings.Repeat("2", 64)}, probe, testReceiptSigner{key: key}, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyKernelCapabilityReport(report, "computer-1", "realization-1", version, now.Add(time.Minute)); err != nil {
		t.Fatalf("verify report: %v", err)
	}
	if err := VerifyKernelCapabilityReport(report, "computer-1", "other-realization", version, now.Add(time.Minute)); err == nil {
		t.Fatal("realization mismatch was accepted")
	}
	if err := VerifyKernelCapabilityReport(report, "computer-1", "realization-1", version, now.Add(11*time.Minute)); err == nil {
		t.Fatal("stale report was accepted")
	}
	tampered := report
	tampered.Receipt.KindFields = make(map[string]any, len(report.Receipt.KindFields))
	for name, value := range report.Receipt.KindFields {
		tampered.Receipt.KindFields[name] = value
	}
	tampered.Receipt.KindFields["guest_image_digest"] = strings.Repeat("0", 64)
	if err := VerifyKernelCapabilityReport(tampered, "computer-1", "realization-1", version, now.Add(time.Minute)); err == nil {
		t.Fatal("tampered image digest was accepted")
	}

	missing := probe
	missing.Capabilities = make(map[string]KernelCapabilityObservation, len(capabilities))
	for name, observation := range capabilities {
		missing.Capabilities[name] = observation
	}
	delete(missing.Capabilities, "landlock_enforcing")
	if _, err := NewKernelCapabilityReport(context.Background(), KernelCapabilityIdentity{
		ComputerID: "computer-1", RealizationID: "realization-1",
		GuestImageDigest: strings.Repeat("f", 64), KernelConfigDigest: strings.Repeat("1", 64), LifecycleGeneration: 4,
	}, KernelCapabilityRequest{ComputerVersion: version, ReleaseDigest: strings.Repeat("2", 64)}, missing, testReceiptSigner{key: key}, now); err == nil {
		t.Fatal("missing mandatory capability was accepted")
	}
}

type failingKernelCapabilityTransport struct {
	err error
}

func (t failingKernelCapabilityTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, t.err
}

func TestKernelCapabilityTransportFailureCodesAreStable(t *testing.T) {
	updaterMarkerPath := filepath.Join(t.TempDir(), "updater-unit-entered")
	if err := os.WriteFile(updaterMarkerPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	migrationMarkerPath := filepath.Join(t.TempDir(), "guest-signer-state-migrated")
	if err := os.WriteFile(migrationMarkerPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	keyAbsentPath := filepath.Join(t.TempDir(), "key-absent")
	if err := os.WriteFile(keyAbsentPath, []byte("absent\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	keySizeInvalidPath := filepath.Join(t.TempDir(), "key-size-invalid")
	if err := os.WriteFile(keySizeInvalidPath, []byte("size-invalid\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	keyExactSizePath := filepath.Join(t.TempDir(), "key-exact-size")
	if err := os.WriteFile(keyExactSizePath, []byte("exact-size\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	keyUnknownPath := filepath.Join(t.TempDir(), "key-unknown")
	if err := os.WriteFile(keyUnknownPath, []byte("unexpected\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	missingUpdaterMarkerPath := filepath.Join(t.TempDir(), "missing-updater")
	missingMigrationMarkerPath := filepath.Join(t.TempDir(), "missing-migration")
	tests := []struct {
		name                string
		err                 error
		updaterMarkerPath   string
		migrationMarkerPath string
		keyShapePath        string
		want                string
	}{
		{name: "signer migration unavailable", err: fmt.Errorf("wrapped: %w", os.ErrNotExist), updaterMarkerPath: missingUpdaterMarkerPath, migrationMarkerPath: missingMigrationMarkerPath, want: KernelCapabilityFailureSignerMigrationUnavailable},
		{name: "signer unavailable after migration", err: fmt.Errorf("wrapped: %w", os.ErrNotExist), updaterMarkerPath: missingUpdaterMarkerPath, migrationMarkerPath: migrationMarkerPath, want: KernelCapabilityFailureSignerUnavailableAfterMigration},
		{name: "signer key absent", err: fmt.Errorf("wrapped: %w", os.ErrNotExist), updaterMarkerPath: missingUpdaterMarkerPath, migrationMarkerPath: migrationMarkerPath, keyShapePath: keyAbsentPath, want: KernelCapabilityFailureSignerUnavailableWithKeyAbsent},
		{name: "signer key size invalid", err: fmt.Errorf("wrapped: %w", os.ErrNotExist), updaterMarkerPath: missingUpdaterMarkerPath, migrationMarkerPath: migrationMarkerPath, keyShapePath: keySizeInvalidPath, want: KernelCapabilityFailureSignerKeySizeInvalid},
		{name: "signer key exact size", err: fmt.Errorf("wrapped: %w", os.ErrNotExist), updaterMarkerPath: missingUpdaterMarkerPath, migrationMarkerPath: migrationMarkerPath, keyShapePath: keyExactSizePath, want: KernelCapabilityFailureSignerUnavailableAfterKeyValidation},
		{name: "signer key shape unknown", err: fmt.Errorf("wrapped: %w", os.ErrNotExist), updaterMarkerPath: missingUpdaterMarkerPath, migrationMarkerPath: migrationMarkerPath, keyShapePath: keyUnknownPath, want: KernelCapabilityFailureSignerUnavailableAfterMigration},
		{name: "signer key shape missing", err: fmt.Errorf("wrapped: %w", os.ErrNotExist), updaterMarkerPath: missingUpdaterMarkerPath, migrationMarkerPath: migrationMarkerPath, keyShapePath: filepath.Join(t.TempDir(), "missing-shape"), want: KernelCapabilityFailureSignerUnavailableAfterMigration},
		{name: "unit not started without migration projection", err: fmt.Errorf("wrapped: %w", os.ErrNotExist), updaterMarkerPath: missingUpdaterMarkerPath, want: KernelCapabilityFailureUpdaterUnitNotStarted},
		{name: "updater process unavailable", err: fmt.Errorf("wrapped: %w", os.ErrNotExist), updaterMarkerPath: updaterMarkerPath, migrationMarkerPath: missingMigrationMarkerPath, want: KernelCapabilityFailureUpdaterProcessUnavailable},
		{name: "socket missing without projection", err: fmt.Errorf("wrapped: %w", os.ErrNotExist), want: KernelCapabilityFailureUpdaterSocketMissing},
		{name: "permission denied", err: fmt.Errorf("wrapped: %w", os.ErrPermission), want: KernelCapabilityFailureUpdaterPermissionDenied},
		{name: "connection refused", err: fmt.Errorf("wrapped: %w", syscall.ECONNREFUSED), want: KernelCapabilityFailureUpdaterConnectionRefused},
		{name: "deadline", err: context.DeadlineExceeded, want: KernelCapabilityFailureUpdaterTimeout},
		{name: "unknown", err: errors.New("unknown transport failure"), want: KernelCapabilityFailureUpdaterUnreachable},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := &Client{
				unitEntryMarkerPath:       test.updaterMarkerPath,
				signerMigrationMarkerPath: test.migrationMarkerPath,
				signerKeyShapePath:        test.keyShapePath,
				http:                      &http.Client{Transport: failingKernelCapabilityTransport{err: test.err}},
			}
			_, err := client.KernelCapabilities(context.Background(), KernelCapabilityRequest{})
			if got := KernelCapabilityFailureCode(err); got != test.want {
				t.Fatalf("failure code = %q, want %q", got, test.want)
			}
		})
	}
}

func TestKernelCapabilityHTTPFailureCodesAreStable(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "probe unavailable", body: `{"error":"mandatory kernel capability probe unavailable"}`, want: KernelCapabilityFailureProbeUnavailable},
		{name: "receipt refused", body: `{"error":"mandatory kernel capability receipt refused"}`, want: KernelCapabilityFailureReceiptRefused},
		{name: "unknown refusal", body: `{"error":"another refusal"}`, want: KernelCapabilityFailureUpdaterUnavailable},
		{name: "invalid response", body: `not-json`, want: KernelCapabilityFailureUpdaterUnavailable},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := kernelCapabilityHTTPFailure(strings.NewReader(test.body))
			if got := KernelCapabilityFailureCode(err); got != test.want {
				t.Fatalf("failure code = %q, want %q", got, test.want)
			}
		})
	}
}
