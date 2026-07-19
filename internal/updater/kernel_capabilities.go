package updater

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/computerversion"
)

const KernelCapabilityReceiptKind = "KernelCapabilityReceipt"

var mandatoryKernelCapabilities = []string{
	"user_namespace", "pid_namespace", "mount_namespace", "network_namespace",
	"uts_namespace", "ipc_namespace", "cgroup_v2", "overlayfs_loaded_and_mountable",
	"seccomp_filter_enforced", "landlock_enforcing",
}

type KernelCapabilityObservation struct {
	Supported      bool   `json:"supported"`
	Enforced       bool   `json:"enforced"`
	ObservationRef string `json:"observation_ref"`
}

type KernelCapabilityRequest struct {
	ComputerVersion computerversion.ComputerVersion `json:"computer_version"`
	ReleaseDigest   string                          `json:"release_digest"`
}

type KernelCapabilityProbe struct {
	KernelRelease       string                                 `json:"kernel_release"`
	BootID              string                                 `json:"boot_id"`
	BootParameters      string                                 `json:"boot_parameters"`
	CgroupFilesystem    string                                 `json:"cgroup_filesystem_type"`
	OverlayModuleDigest string                                 `json:"overlay_module_digest"`
	ObservedAt          string                                 `json:"observed_at"`
	Capabilities        map[string]KernelCapabilityObservation `json:"observed_capabilities"`
	ContractDigest      string                                 `json:"probe_contract_digest"`
}

type KernelCapabilityReport struct {
	Receipt   computerevent.Receipt `json:"receipt"`
	PublicKey string                `json:"public_key"`
}

type KernelCapabilityIdentity struct {
	ComputerID          string
	RealizationID       string
	GuestImageDigest    string
	KernelConfigDigest  string
	LifecycleGeneration uint64
}

func NewKernelCapabilityReport(identity KernelCapabilityIdentity, request KernelCapabilityRequest, probe KernelCapabilityProbe, key computerevent.SigningKey, now time.Time) (KernelCapabilityReport, error) {
	if strings.TrimSpace(identity.ComputerID) == "" || strings.TrimSpace(identity.RealizationID) == "" ||
		!computerevent.IsSHA256(identity.GuestImageDigest) || !computerevent.IsSHA256(identity.KernelConfigDigest) ||
		!request.ComputerVersion.Valid() || !computerevent.IsSHA256(request.ReleaseDigest) ||
		strings.TrimSpace(probe.KernelRelease) == "" || strings.TrimSpace(probe.BootID) == "" ||
		strings.TrimSpace(probe.BootParameters) == "" || probe.CgroupFilesystem != "cgroup2" ||
		!computerevent.IsSHA256(probe.OverlayModuleDigest) || !computerevent.IsSHA256(probe.ContractDigest) {
		return KernelCapabilityReport{}, fmt.Errorf("kernel capability receipt: incomplete immutable identity or probe")
	}
	for _, name := range mandatoryKernelCapabilities {
		observation, ok := probe.Capabilities[name]
		if !ok || !observation.Supported || !observation.Enforced || !strings.HasPrefix(observation.ObservationRef, "sha256:") || !computerevent.IsSHA256(strings.TrimPrefix(observation.ObservationRef, "sha256:")) {
			return KernelCapabilityReport{}, fmt.Errorf("kernel capability receipt: mandatory capability %s is not enforced", name)
		}
	}
	if len(probe.Capabilities) != len(mandatoryKernelCapabilities) {
		return KernelCapabilityReport{}, fmt.Errorf("kernel capability receipt: unexpected capability set")
	}
	now = now.UTC()
	observedAt, err := time.Parse(time.RFC3339Nano, probe.ObservedAt)
	if err != nil || observedAt.After(now) || now.Sub(observedAt) >= 10*time.Minute {
		return KernelCapabilityReport{}, fmt.Errorf("kernel capability receipt: probe is stale")
	}
	expires := observedAt.Add(10 * time.Minute)
	fields := map[string]any{
		"computer_id": identity.ComputerID, "realization_id": identity.RealizationID,
		"computer_version": request.ComputerVersion, "release_digest": request.ReleaseDigest,
		"guest_image_digest": identity.GuestImageDigest, "kernel_config_digest": identity.KernelConfigDigest,
		"kernel_release": probe.KernelRelease, "boot_id": probe.BootID, "boot_parameters": probe.BootParameters,
		"cgroup_filesystem_type": probe.CgroupFilesystem, "overlay_module_digest": probe.OverlayModuleDigest,
		"observed_capabilities": probe.Capabilities, "probe_contract_digest": probe.ContractDigest,
		"observed_at": observedAt.Format(time.RFC3339Nano), "expires_at": expires.Format(time.RFC3339Nano),
		"lifecycle_generation": identity.LifecycleGeneration,
	}
	receipt, err := computerevent.NewSignedReceipt(KernelCapabilityReceiptKind, "choir-updater", fields, []computerevent.SigningKey{key}, now)
	if err != nil {
		return KernelCapabilityReport{}, err
	}
	publicKey, ok := key.PrivateKey.Public().(ed25519.PublicKey)
	if !ok {
		return KernelCapabilityReport{}, fmt.Errorf("kernel capability receipt: public key unavailable")
	}
	return KernelCapabilityReport{Receipt: receipt, PublicKey: base64.RawStdEncoding.EncodeToString(publicKey)}, nil
}

func VerifyKernelCapabilityReport(report KernelCapabilityReport, computerID, realizationID string, version computerversion.ComputerVersion, now time.Time) error {
	publicKey, err := base64.RawStdEncoding.DecodeString(report.PublicKey)
	if err != nil || len(publicKey) != ed25519.PublicKeySize || len(report.Receipt.RequiredSigners) != 1 {
		return fmt.Errorf("kernel capability receipt: invalid public key")
	}
	resolver := fixedReceiptKey{ref: report.Receipt.RequiredSigners[0], key: ed25519.PublicKey(publicKey)}
	if report.Receipt.ReceiptKind != KernelCapabilityReceiptKind || report.Receipt.Issuer != "choir-updater" || report.Receipt.Verify(resolver) != nil {
		return fmt.Errorf("kernel capability receipt: signature verification failed")
	}
	names := []string{"computer_id", "realization_id", "computer_version", "release_digest", "guest_image_digest", "kernel_config_digest", "kernel_release", "boot_id", "boot_parameters", "cgroup_filesystem_type", "overlay_module_digest", "observed_capabilities", "probe_contract_digest", "observed_at", "expires_at", "lifecycle_generation"}
	sort.Strings(names)
	if err := report.Receipt.RequireKindFields(names...); err != nil {
		return err
	}
	if report.Receipt.KindFields["computer_id"] != computerID || report.Receipt.KindFields["realization_id"] != realizationID {
		return fmt.Errorf("kernel capability receipt: identity mismatch")
	}
	encodedVersion, _ := computerevent.CanonicalJSON(report.Receipt.KindFields["computer_version"])
	expectedVersion, _ := computerevent.CanonicalJSON(version)
	if string(encodedVersion) != string(expectedVersion) {
		return fmt.Errorf("kernel capability receipt: ComputerVersion mismatch")
	}
	for _, name := range []string{"release_digest", "guest_image_digest", "kernel_config_digest", "overlay_module_digest", "probe_contract_digest"} {
		if !computerevent.IsSHA256(fmt.Sprint(report.Receipt.KindFields[name])) {
			return fmt.Errorf("kernel capability receipt: invalid %s", name)
		}
	}
	if report.Receipt.KindFields["cgroup_filesystem_type"] != "cgroup2" {
		return fmt.Errorf("kernel capability receipt: cgroup v2 is not enforced")
	}
	rawCapabilities, err := json.Marshal(report.Receipt.KindFields["observed_capabilities"])
	if err != nil {
		return fmt.Errorf("kernel capability receipt: invalid capability evidence")
	}
	var capabilities map[string]KernelCapabilityObservation
	if err := json.Unmarshal(rawCapabilities, &capabilities); err != nil || len(capabilities) != len(mandatoryKernelCapabilities) {
		return fmt.Errorf("kernel capability receipt: invalid capability evidence")
	}
	for _, name := range mandatoryKernelCapabilities {
		observation, ok := capabilities[name]
		if !ok || !observation.Supported || !observation.Enforced ||
			!strings.HasPrefix(observation.ObservationRef, "sha256:") ||
			!computerevent.IsSHA256(strings.TrimPrefix(observation.ObservationRef, "sha256:")) {
			return fmt.Errorf("kernel capability receipt: mandatory capability %s is not enforced", name)
		}
	}
	observedAt, observedErr := time.Parse(time.RFC3339Nano, fmt.Sprint(report.Receipt.KindFields["observed_at"]))
	expiresAt, expiresErr := time.Parse(time.RFC3339Nano, fmt.Sprint(report.Receipt.KindFields["expires_at"]))
	issuedAt, issuedErr := time.Parse(time.RFC3339Nano, report.Receipt.IssuedAt)
	now = now.UTC()
	if observedErr != nil || expiresErr != nil || issuedErr != nil || observedAt.After(issuedAt) || issuedAt.After(expiresAt) ||
		expiresAt.Sub(observedAt) != 10*time.Minute || !now.Before(expiresAt) || now.Before(observedAt) {
		return fmt.Errorf("kernel capability receipt: stale")
	}
	return nil
}

type fixedReceiptKey struct {
	ref computerevent.SignerRef
	key ed25519.PublicKey
}

func (r fixedReceiptKey) ResolveReceiptKey(domain, _ string, keyID string, _ uint64, _ time.Time) (ed25519.PublicKey, error) {
	if domain != r.ref.SignerDomain || keyID != r.ref.KeyID {
		return nil, fmt.Errorf("kernel capability receipt: signer mismatch")
	}
	return r.key, nil
}

func MandatoryKernelCapabilities() []string {
	return append([]string(nil), mandatoryKernelCapabilities...)
}

func ReadKernelCapabilityProbe(path string) (KernelCapabilityProbe, error) {
	raw, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return KernelCapabilityProbe{}, err
	}
	var probe KernelCapabilityProbe
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&probe); err != nil {
		return KernelCapabilityProbe{}, err
	}
	canonical, err := computerevent.CanonicalJSON(probe)
	if err != nil || !bytes.Equal(canonical, raw) {
		return KernelCapabilityProbe{}, fmt.Errorf("kernel capability probe: non-canonical artifact")
	}
	return probe, nil
}

func DigestFile(path string) (string, error) {
	return fileSHA256(path)
}
