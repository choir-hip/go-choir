package platform

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/builder"
	"github.com/yusefmosiah/go-choir/internal/hostbuild"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
)

// hostBuildAuth binds a bearer token to one computer and one scope set, the
// way the computer capability does.
type hostBuildAuth struct{ computer string }

func (a hostBuildAuth) Authorize(r *http.Request, computerID, scope string) error {
	if r.Header.Get("Authorization") != "Bearer "+a.computer || computerID != a.computer {
		return fmt.Errorf("refused")
	}
	return nil
}
func (a hostBuildAuth) AuthorizeRenewal(*http.Request, string) error { return fmt.Errorf("no") }

func TestHostBuildRoutes(t *testing.T) {
	dir := t.TempDir()
	manifest := "contract=choir-guest-image-v1\nbuild_commit=0123456789abcdef0123456789abcdef01234567\n"
	manifestPath := filepath.Join(dir, "m")
	_ = os.WriteFile(manifestPath, []byte(manifest), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "d"), []byte("d"), 0o644)
	h := &Handler{service: &Service{artifactsRoot: filepath.Join(dir, "artifacts")}, eventAuth: hostBuildAuth{computer: "computer-a"}}

	patch := "--- a/internal/x/x.go\n+++ b/internal/x/x.go\n@@ -1 +1 @@\n-a\n+b\n"
	req := hostbuild.Request{
		ComputerID: "computer-a", OperationID: "op", BundleDigest: strings.Repeat("b", 64),
		BaseCommit: "0123456789abcdef0123456789abcdef01234567", SourcePatchSHA256: sha256HexForTest(patch),
		BaseImageManifestDigest: sha256HexForTest(manifest), PatchBase64: base64.StdEncoding.EncodeToString([]byte(patch)),
	}
	do := func(method, path, token string, body any) (int, map[string]any) {
		var buf bytes.Buffer
		if body != nil {
			_ = json.NewEncoder(&buf).Encode(body)
		}
		r := httptest.NewRequest(method, path, &buf)
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		h.HandleHostBuilds(w, r)
		var out map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &out)
		return w.Code, out
	}

	// Unconfigured: a typed 503 the guest reads as a terminal build failure.
	if code, out := do(http.MethodPost, hostbuild.Route, "computer-a", req); code != http.StatusServiceUnavailable ||
		!strings.HasPrefix(fmt.Sprint(out["error"]), hostbuild.ReasonHostBuilderUnconfigured) {
		t.Fatalf("unconfigured = %d %v", code, out)
	}

	svc, err := hostbuild.New(hostbuild.Config{
		StateDir: filepath.Join(dir, "state"), SourceDir: dir, BaseManifestPath: manifestPath, BaseStoreDiskPath: filepath.Join(dir, "d"),
		Build: func(ctx context.Context, r builder.Request) (*builder.ClosureResult, error) {
			return nil, fmt.Errorf("no nix here")
		},
		Sample: func() (hostbuild.Pressure, error) { return hostbuild.Pressure{MemAvailableBytes: 64 << 30}, nil },
		Sign: func(string, string, string, time.Time) (selfdevprotocol.AuthorityReceipt, error) {
			return selfdevprotocol.AuthorityReceipt{}, nil
		},
		Store: h.storePlatformUpdateBlobFile,
	})
	if err != nil {
		t.Fatal(err)
	}
	h.hostBuild = svc

	// H13: another computer's capability cannot submit for or read this one.
	if code, _ := do(http.MethodPost, hostbuild.Route, "computer-b", req); code != http.StatusForbidden {
		t.Fatalf("foreign submit = %d", code)
	}
	code, out := do(http.MethodPost, hostbuild.Route, "computer-a", req)
	if code != http.StatusOK || out["state"] != hostbuild.StateQueued {
		t.Fatalf("submit = %d %v", code, out)
	}
	jobID := fmt.Sprint(out["job_id"])
	if code, _ := do(http.MethodGet, hostbuild.Route+"/"+jobID+"?computer_id=computer-b", "computer-b", nil); code != http.StatusNotFound && code != http.StatusForbidden {
		t.Fatalf("foreign read = %d", code)
	}
	if code, out := do(http.MethodGet, hostbuild.Route+"/"+jobID+"?computer_id=computer-a", "computer-a", nil); code != http.StatusOK || out["job_id"] != jobID {
		t.Fatalf("read = %d %v", code, out)
	}
	// H1 at the route: a tampered patch is a 400, no job.
	bad := req
	bad.OperationID = "op-2"
	bad.PatchBase64 = base64.StdEncoding.EncodeToString([]byte(patch + "x"))
	if code, _ := do(http.MethodPost, hostbuild.Route, "computer-a", bad); code != http.StatusBadRequest {
		t.Fatalf("tampered = %d", code)
	}
	// H20 at the route: a busy computer is a 429 led by its reason code,
	// with no job recorded, so the guest stays accepted and retries.
	busy := req
	busy.OperationID = "op-3"
	if code, out := do(http.MethodPost, hostbuild.Route, "computer-a", busy); code != http.StatusTooManyRequests ||
		!strings.HasPrefix(fmt.Sprint(out["error"]), hostbuild.ReasonComputerBusy) {
		t.Fatalf("busy = %d %v", code, out)
	}
}

func TestStorePlatformUpdateBlobFileVerifiesDigest(t *testing.T) {
	dir := t.TempDir()
	h := &Handler{service: &Service{artifactsRoot: dir}}
	src := filepath.Join(dir, "src")
	_ = os.WriteFile(src, []byte("closure"), 0o644)
	if err := h.storePlatformUpdateBlobFile(strings.Repeat("0", 64), src); err == nil {
		t.Fatal("stored a blob under the wrong digest")
	}
	digest := sha256HexForTest("closure")
	if err := h.storePlatformUpdateBlobFile(digest, src); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "sha256", "platform-update", digest))
	if err != nil || string(got) != "closure" {
		t.Fatalf("stored = %q %v", got, err)
	}
}

func sha256HexForTest(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
