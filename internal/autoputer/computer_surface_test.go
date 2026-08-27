package autoputer

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestComputerSurfaceServesStagedSPAAndAssets(t *testing.T) {
	root := filepath.Join(t.TempDir(), "current", "frontend")
	if err := os.MkdirAll(filepath.Join(root, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "index.html"), []byte("computer-a-shell"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "assets", "app.js"), []byte("computer-a-asset"), 0o644); err != nil {
		t.Fatal(err)
	}
	surface := &ComputerSurface{Root: root}

	index := httptest.NewRecorder()
	surface.ServeHTTP(index, httptest.NewRequest(http.MethodGet, "/", nil))
	if index.Code != http.StatusOK || !strings.Contains(index.Body.String(), "computer-a-shell") {
		t.Fatalf("index status=%d body=%s", index.Code, index.Body.String())
	}
	if index.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("index cache = %q", index.Header().Get("Cache-Control"))
	}

	asset := httptest.NewRecorder()
	surface.ServeHTTP(asset, httptest.NewRequest(http.MethodGet, "/assets/app.js", nil))
	if asset.Code != http.StatusOK || asset.Body.String() != "computer-a-asset" {
		t.Fatalf("asset status=%d body=%s", asset.Code, asset.Body.String())
	}

	fallback := httptest.NewRecorder()
	surface.ServeHTTP(fallback, httptest.NewRequest(http.MethodGet, "/desktop/texture", nil))
	if fallback.Code != http.StatusOK || !strings.Contains(fallback.Body.String(), "computer-a-shell") {
		t.Fatalf("spa fallback status=%d body=%s", fallback.Code, fallback.Body.String())
	}
}

func TestComputerSurfaceRefusesMissingSPA(t *testing.T) {
	surface := &ComputerSurface{Root: filepath.Join(t.TempDir(), "current", "frontend")}
	response := httptest.NewRecorder()
	surface.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("missing SPA status=%d", response.Code)
	}
}

func TestComputerSurfaceRejectsPathEscape(t *testing.T) {
	root := filepath.Join(t.TempDir(), "current", "frontend")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "index.html"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	secret := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(secret, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	surface := &ComputerSurface{Root: root}
	response := httptest.NewRecorder()
	surface.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/../"+filepath.Base(secret), nil))
	if strings.Contains(response.Body.String(), "secret") {
		t.Fatalf("path escape leaked: %s", response.Body.String())
	}
}

func TestTwoComputerSurfacesServeDivergentBytes(t *testing.T) {
	serve := func(label string) *ComputerSurface {
		root := filepath.Join(t.TempDir(), "current", "frontend")
		if err := os.MkdirAll(root, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "index.html"), []byte(label), 0o644); err != nil {
			t.Fatal(err)
		}
		return &ComputerSurface{Root: root}
	}
	a := httptest.NewRecorder()
	b := httptest.NewRecorder()
	serve("ui-a").ServeHTTP(a, httptest.NewRequest(http.MethodGet, "/", nil))
	serve("ui-b").ServeHTTP(b, httptest.NewRequest(http.MethodGet, "/", nil))
	if a.Body.String() == b.Body.String() {
		t.Fatal("two computers served the same SPA")
	}
}

func TestComputerSurfaceRefusesMissingHashedAssetWithoutHTMLFallback(t *testing.T) {
	root := filepath.Join(t.TempDir(), "current", "frontend")
	if err := os.MkdirAll(filepath.Join(root, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "index.html"), []byte("<html>shell</html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	surface := &ComputerSurface{Root: root}

	// Missing asset must return 404 with no-store, NEVER 200 index.html
	missingAsset := httptest.NewRecorder()
	surface.ServeHTTP(missingAsset, httptest.NewRequest(http.MethodGet, "/assets/SettingsApp-DtEB7MbW.css", nil))
	if missingAsset.Code != http.StatusNotFound {
		t.Fatalf("missing asset status=%d want 404", missingAsset.Code)
	}
	if strings.Contains(missingAsset.Body.String(), "<html>shell</html>") {
		t.Fatalf("missing asset fell back to HTML: %s", missingAsset.Body.String())
	}
	if missingAsset.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("missing asset cache header = %q want no-store", missingAsset.Header().Get("Cache-Control"))
	}

	// Non-asset SPA route must still fall back to index.html with 200
	spaRoute := httptest.NewRecorder()
	surface.ServeHTTP(spaRoute, httptest.NewRequest(http.MethodGet, "/desktop/settings", nil))
	if spaRoute.Code != http.StatusOK || !strings.Contains(spaRoute.Body.String(), "<html>shell</html>") {
		t.Fatalf("spa route status=%d body=%s", spaRoute.Code, spaRoute.Body.String())
	}
}
