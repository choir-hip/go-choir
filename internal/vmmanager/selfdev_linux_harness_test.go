//go:build linux

package vmmanager

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestSelfDevelopmentEffectsOffGuestHarness boots the exact installed Nix guest
// on a disposable Firecracker identity. It is intentionally opt-in: it needs
// Linux, root, KVM, the deployed guest artifacts, and the Node A corpusd/vmctl
// credential path. The test never enables self-development effects.
func TestSelfDevelopmentEffectsOffGuestHarness(t *testing.T) {
	if os.Getenv("CHOIR_G1_LINUX_HARNESS") != "1" {
		t.Skip("set CHOIR_G1_LINUX_HARNESS=1 on the designated Linux harness")
	}
	if os.Geteuid() != 0 {
		t.Fatal("G1 Linux harness requires root for KVM, tap, mount, and cleanup")
	}

	const (
		vmID          = "vm-selfdev-g1-harness"
		computerID    = "computer-selfdev-g1-harness"
		ownerID       = "owner-selfdev-g1-harness"
		desktopID     = "g1-harness"
		realizationID = vmID + "-epoch-1"
	)

	stateDir := t.TempDir()
	cfg := DefaultManagerConfig()
	cfg.FirecrackerBinPath = harnessPath(t, "CHOIR_G1_FIRECRACKER", "firecracker")
	cfg.MkfsExt4Path = harnessPath(t, "CHOIR_G1_MKFS_EXT4", "mkfs.ext4")
	cfg.KernelImagePath = harnessFile(t, "CHOIR_G1_KERNEL", "/var/lib/go-choir/guest/vmlinux")
	cfg.InitrdPath = harnessFile(t, "CHOIR_G1_INITRD", "/var/lib/go-choir/guest/initrd")
	cfg.RootfsPath = harnessFile(t, "CHOIR_G1_ROOTFS", "/var/lib/go-choir/guest/rootfs.ext4")
	cfg.StoreDiskPath = harnessFile(t, "CHOIR_G1_STORE_DISK", "/var/lib/go-choir/guest/storedisk.erofs")
	cfg.KernelParams = strings.TrimSpace(string(harnessReadFile(t, "CHOIR_G1_KERNEL_PARAMS", "/var/lib/go-choir/guest/kernel-params")))
	cfg.StateDir = stateDir
	cfg.BootReadyTimeout = 2 * time.Minute
	cfg.HealthCheckTimeout = 5 * time.Second

	envelope := issueHarnessCredential(t, computerID, realizationID)
	manager := NewManager(cfg)
	manager.Start()
	t.Cleanup(func() {
		_ = manager.ForceKillVM(vmID)
		manager.Stop()
		_ = manager.DestroyVMState(vmID)
	})

	instance, err := manager.BootVM(VMConfig{
		VMID:                       vmID,
		ComputerID:                 computerID,
		RealizationID:              realizationID,
		ComputerKind:               "disposable_g1_harness",
		OwnerID:                    ownerID,
		DesktopID:                  desktopID,
		ComputerCredentialEnvelope: envelope,
		MachineCPUCount:            2,
		MachineMemSizeMib:          2048,
	})
	if err != nil {
		t.Fatalf("boot exact G1 guest: %v", err)
	}
	if !instance.Healthy || instance.State != StateRunning {
		t.Fatalf("guest instance = state=%s healthy=%t", instance.State, instance.Healthy)
	}

	health := harnessRequest(t, http.MethodGet, instance.HostURL+"/health", nil, nil)
	if health.StatusCode != http.StatusOK {
		t.Fatalf("guest health status = %d body=%s", health.StatusCode, health.Body)
	}
	var healthBody struct {
		Build struct {
			Commit string `json:"commit"`
		} `json:"build"`
	}
	if err := json.Unmarshal(health.Body, &healthBody); err != nil {
		t.Fatalf("decode guest health: %v body=%s", err, health.Body)
	}
	if expected := strings.TrimSpace(os.Getenv("CHOIR_G1_EXPECTED_COMMIT")); expected != "" && healthBody.Build.Commit != expected {
		t.Fatalf("guest build commit = %q, want %q", healthBody.Build.Commit, expected)
	}

	headers := map[string]string{
		"Content-Type":             "application/json",
		"X-Authenticated-User":     ownerID,
		"X-Authenticated-Computer": computerID,
	}
	startBody, err := json.Marshal(map[string]string{
		"idempotency_key": "g1-effects-off-start",
		"prompt":          "Prove that effects-off refuses a new self-development proposal.",
	})
	if err != nil {
		t.Fatal(err)
	}
	start := harnessRequest(t, http.MethodPost, instance.HostURL+"/api/computers/"+computerID+"/self-development/operations", startBody, headers)
	if start.StatusCode != http.StatusConflict || !bytes.Contains(start.Body, []byte("does not authorize proposal")) {
		t.Fatalf("effects-off proposal status = %d body=%s", start.StatusCode, start.Body)
	}

	capabilities := harnessRequest(t, http.MethodGet, instance.HostURL+"/api/computers/"+computerID+"/self-development/kernel-capabilities", nil, headers)
	t.Logf("exact_guest=%s build=%s effects_off_status=%d kernel_capability_status=%d", instance.HostURL, healthBody.Build.Commit, start.StatusCode, capabilities.StatusCode)
}

type harnessResponse struct {
	StatusCode int
	Body       []byte
}

func harnessRequest(t *testing.T, method, target string, body []byte, headers map[string]string) harnessResponse {
	t.Helper()
	request, err := http.NewRequestWithContext(t.Context(), method, target, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	for name, value := range headers {
		request.Header.Set(name, value)
	}
	client := &http.Client{Timeout: 20 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("%s %s: %v", method, target, err)
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		t.Fatal(err)
	}
	return harnessResponse{StatusCode: response.StatusCode, Body: responseBody}
}

func issueHarnessCredential(t *testing.T, computerID, realizationID string) string {
	t.Helper()
	corpusdURL := strings.TrimRight(strings.TrimSpace(os.Getenv("CHOIR_G1_CORPUSD_URL")), "/")
	if corpusdURL == "" {
		corpusdURL = "http://127.0.0.1:8086"
	}
	body, err := json.Marshal(map[string]string{
		"computer_id":     computerID,
		"realization_id":  realizationID,
		"idempotency_key": "g1-guest-credential:" + realizationID,
	})
	if err != nil {
		t.Fatal(err)
	}
	response := harnessRequest(t, http.MethodPost, corpusdURL+"/internal/computers/credentials/issue", body, map[string]string{
		"Content-Type":      "application/json",
		"X-Internal-Caller": "true",
	})
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("issue harness credential status = %d body=%s", response.StatusCode, response.Body)
	}
	var result struct {
		Envelope json.RawMessage `json:"envelope"`
	}
	if err := json.Unmarshal(response.Body, &result); err != nil || len(result.Envelope) == 0 {
		t.Fatalf("decode harness credential: %v body=%s", err, response.Body)
	}
	return base64.RawURLEncoding.EncodeToString(result.Envelope)
}

func harnessPath(t *testing.T, envName, fallback string) string {
	t.Helper()
	path := strings.TrimSpace(os.Getenv(envName))
	if path == "" {
		var err error
		path, err = findInPath(fallback)
		if err != nil {
			t.Fatalf("%s: %v", envName, err)
		}
	}
	return path
}

func harnessFile(t *testing.T, envName, fallback string) string {
	t.Helper()
	path := strings.TrimSpace(os.Getenv(envName))
	if path == "" {
		path = fallback
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		t.Fatalf("%s must name a regular file: %s: %v", envName, path, err)
	}
	return filepath.Clean(path)
}

func harnessReadFile(t *testing.T, envName, fallback string) []byte {
	t.Helper()
	path := harnessFile(t, envName, fallback)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}
