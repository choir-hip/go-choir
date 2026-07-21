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
	markerPath := filepath.Join(t.TempDir(), "updater-unit-entered")
	if err := os.WriteFile(markerPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name       string
		err        error
		markerPath string
		want       string
	}{
		{name: "unit not started", err: fmt.Errorf("wrapped: %w", os.ErrNotExist), markerPath: filepath.Join(t.TempDir(), "missing"), want: KernelCapabilityFailureUpdaterUnitNotStarted},
		{name: "process unavailable", err: fmt.Errorf("wrapped: %w", os.ErrNotExist), markerPath: markerPath, want: KernelCapabilityFailureUpdaterProcessUnavailable},
		{name: "socket missing without projection", err: fmt.Errorf("wrapped: %w", os.ErrNotExist), want: KernelCapabilityFailureUpdaterSocketMissing},
		{name: "permission denied", err: fmt.Errorf("wrapped: %w", os.ErrPermission), want: KernelCapabilityFailureUpdaterPermissionDenied},
		{name: "connection refused", err: fmt.Errorf("wrapped: %w", syscall.ECONNREFUSED), want: KernelCapabilityFailureUpdaterConnectionRefused},
		{name: "deadline", err: context.DeadlineExceeded, want: KernelCapabilityFailureUpdaterTimeout},
		{name: "unknown", err: errors.New("unknown transport failure"), want: KernelCapabilityFailureUpdaterUnreachable},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := &Client{
				unitEntryMarkerPath: test.markerPath,
				http:                &http.Client{Transport: failingKernelCapabilityTransport{err: test.err}},
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
