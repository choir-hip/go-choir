package proxy

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

func TestIsComputerSurfacePathSplitsControlPlane(t *testing.T) {
	for _, path := range []string{"/api/texture", "/auth/login", "/health", "/provider/x", "/internal/vmctl/lookup"} {
		if isComputerSurfacePath(path) {
			t.Fatalf("%s must stay control-plane", path)
		}
	}
	for _, path := range []string{"/", "/index.html", "/assets/app.js", "/desktop"} {
		if !isComputerSurfacePath(path) {
			t.Fatalf("%s must be computer surface", path)
		}
	}
}

func TestComputerSurfaceSelectsGuestBytesAfterResolve(t *testing.T) {
	var guestHits atomic.Int32
	guestA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		guestHits.Add(1)
		if r.URL.Path == "/assets/app.js" {
			_, _ = w.Write([]byte("asset-a"))
			return
		}
		_, _ = w.Write([]byte("ui-a"))
	}))
	defer guestA.Close()
	guestB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		guestHits.Add(1)
		_, _ = w.Write([]byte("ui-b"))
	}))
	defer guestB.Close()

	ownership := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/vmctl/resolve" {
			t.Fatalf("unexpected vmctl path %s", r.URL.Path)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		var req struct {
			UserID    string `json:"user_id"`
			DesktopID string `json:"desktop_id"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatal(err)
		}
		url := guestA.URL
		computerID := "computer-a"
		switch req.UserID {
		case "owner-a":
		case "owner-b":
			url = guestB.URL
			computerID = "computer-b"
		default:
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"computer_id": computerID, "desktop_id": "primary", "user_id": req.UserID,
			"state": "active", "computer_url": url,
		})
	}))
	defer ownership.Close()

	shellDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(shellDir, "index.html"), []byte("platform-shell"), 0o644); err != nil {
		t.Fatal(err)
	}

	handler, privateKey, _, _ := testProxyEnvWithAuthStore(t)
	handler.cfg.PlatformShellRoot = shellDir
	handler.vmctlClient = vmctl.NewClient(ownership.URL)

	unsigned := httptest.NewRequest(http.MethodGet, "/", nil)
	unsignedRec := httptest.NewRecorder()
	handler.HandleComputerSurface(unsignedRec, unsigned)
	if unsignedRec.Code != http.StatusOK || !strings.Contains(unsignedRec.Body.String(), "platform-shell") {
		t.Fatalf("unsigned surface status=%d body=%s", unsignedRec.Code, unsignedRec.Body.String())
	}
	if guestHits.Load() != 0 {
		t.Fatalf("unsigned surface hit guest %d times", guestHits.Load())
	}

	reqA := httptest.NewRequest(http.MethodGet, "/", nil)
	reqA.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWT(privateKey, "owner-a")})
	recA := httptest.NewRecorder()
	handler.HandleComputerSurface(recA, reqA)
	if recA.Code != http.StatusOK || recA.Body.String() != "ui-a" {
		t.Fatalf("computer-a surface status=%d body=%s", recA.Code, recA.Body.String())
	}

	reqAsset := httptest.NewRequest(http.MethodGet, "/assets/app.js", nil)
	reqAsset.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWT(privateKey, "owner-a")})
	recAsset := httptest.NewRecorder()
	handler.HandleComputerSurface(recAsset, reqAsset)
	if recAsset.Code != http.StatusOK || recAsset.Body.String() != "asset-a" {
		t.Fatalf("computer-a asset status=%d body=%s", recAsset.Code, recAsset.Body.String())
	}

	reqB := httptest.NewRequest(http.MethodGet, "/", nil)
	reqB.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWT(privateKey, "owner-b")})
	recB := httptest.NewRecorder()
	handler.HandleComputerSurface(recB, reqB)
	if recB.Code != http.StatusOK || recB.Body.String() != "ui-b" {
		t.Fatalf("computer-b surface status=%d body=%s", recB.Code, recB.Body.String())
	}
	if recA.Body.String() == recB.Body.String() {
		t.Fatal("two computers served the same UI")
	}
}

func TestCaddyNoLongerServesHostFrontendCurrentAsComputerSurface(t *testing.T) {
	for _, rel := range []string{"../../nix/node-b.nix", "../../nix/node-a.nix"} {
		raw, err := os.ReadFile(rel)
		if err != nil {
			t.Fatal(err)
		}
		text := string(raw)
		if strings.Contains(text, "try_files /frontend-current{uri}") {
			t.Fatalf("%s still serves host frontend-current as computer-surface assets", rel)
		}
		if strings.Contains(text, "root * ${frontendCurrent}") {
			t.Fatalf("%s still serves host frontend-current as computer-surface HTML", rel)
		}
		if !strings.Contains(text, "reverse_proxy 127.0.0.1:8082") {
			t.Fatalf("%s must reverse-proxy computer surface through the proxy", rel)
		}
	}
}
