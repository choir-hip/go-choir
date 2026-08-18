package agentcore

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerversion"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/routeledger"
	"github.com/yusefmosiah/go-choir/internal/updater"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

func TestEnsureServingBaselineImportsImmutableBaselineWhenCurrentMissing(t *testing.T) {
	computerID := "computer-surface-bootstrap"
	realizationID := "realization-surface-bootstrap"
	ownerID := "owner-surface-bootstrap"
	desktopID := "primary"
	updaterRoot := filepath.Join(t.TempDir(), "updater")
	baselineRoot := filepath.Join(t.TempDir(), "baseline")
	if err := os.MkdirAll(filepath.Join(baselineRoot, "frontend", "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(baselineRoot, "frontend", "index.html"), []byte("immutable-baseline"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(baselineRoot, "frontend", "assets", "app.js"), []byte("baseline-asset"), 0o644); err != nil {
		t.Fatal(err)
	}

	routeSlot, resolution := surfaceBootstrapRoute(t, ownerID, desktopID)
	routeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/vmctl/computer-version-routes/resolve" || r.URL.Query().Get("route_slot_id") != routeSlot {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resolution)
	}))
	defer routeServer.Close()

	socketPath := filepath.Join("/tmp", "choir-surface-"+strings.ReplaceAll(t.Name(), "/", "-")+".sock")
	_ = os.Remove(socketPath)
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}
	var importCalls atomic.Int32
	updaterServer := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/import-baseline" || r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		var request updater.BaselineImportRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		importCalls.Add(1)
		if request.ComputerID != computerID || request.RealizationID != realizationID || request.SourceDir != baselineRoot ||
			request.IdempotencyKey != "checkpoint-baseline-"+computerID {
			http.Error(w, "wrong baseline binding", http.StatusBadRequest)
			return
		}
		commitment, commitmentErr := updater.ComputeBaselineImportCommitment(request)
		if commitmentErr != nil || commitment != request.RequestCommitment {
			http.Error(w, "wrong request commitment", http.StatusBadRequest)
			return
		}
		releaseDir := filepath.Join(updaterRoot, "releases", request.Manifest.ContentDigest)
		for _, file := range request.Manifest.Files {
			source := filepath.Join(baselineRoot, filepath.FromSlash(file.Path))
			target := filepath.Join(releaseDir, filepath.FromSlash(file.Path))
			if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
				http.Error(w, "stage failed", http.StatusInternalServerError)
				return
			}
			raw, readErr := os.ReadFile(source)
			if readErr != nil || os.WriteFile(target, raw, 0o644) != nil {
				http.Error(w, "stage failed", http.StatusInternalServerError)
				return
			}
		}
		manifestBytes, manifestErr := json.Marshal(request.Manifest)
		if manifestErr != nil || os.WriteFile(filepath.Join(releaseDir, "release-manifest.json"), manifestBytes, 0o644) != nil {
			http.Error(w, "manifest failed", http.StatusInternalServerError)
			return
		}
		if err := os.MkdirAll(updaterRoot, 0o700); err != nil || os.Symlink(releaseDir, filepath.Join(updaterRoot, "current")) != nil {
			http.Error(w, "pointer failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(request.Manifest)
	})}
	go func() { _ = updaterServer.Serve(listener) }()
	defer func() {
		_ = updaterServer.Shutdown(context.Background())
		_ = os.Remove(socketPath)
	}()

	client, err := updater.NewClient(socketPath)
	if err != nil {
		t.Fatal(err)
	}
	rt := &Runtime{
		cfg:                   provideriface.Config{ComputerID: computerID},
		selfdevUpdater:        client,
		selfdevUpdaterRoot:    updaterRoot,
		selfdevComputerID:     computerID,
		selfdevRealizationID:  realizationID,
		selfdevRoute:          vmctl.NewClient(routeServer.URL),
		selfdevRouteOwnerID:   ownerID,
		selfdevRouteDesktopID: desktopID,
	}
	manifest, err := rt.ensureServingBaseline(context.Background(), computerID, baselineRoot)
	if err != nil {
		t.Fatalf("baseline bootstrap failed: %v", err)
	}
	if importCalls.Load() != 1 {
		t.Fatalf("baseline import calls=%d, want 1", importCalls.Load())
	}
	if manifest.ContentDigest == "" || rt.selfdevStartupReleaseDigest != manifest.ContentDigest {
		t.Fatalf("startup manifest was not recorded: manifest=%q startup=%q", manifest.ContentDigest, rt.selfdevStartupReleaseDigest)
	}
	served, err := os.ReadFile(filepath.Join(updaterRoot, "current", "frontend", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if string(served) != "immutable-baseline" {
		t.Fatalf("served baseline=%q", served)
	}

	if _, err := rt.ensureServingBaseline(context.Background(), computerID, baselineRoot); err != nil {
		t.Fatalf("idempotent bootstrap failed: %v", err)
	}
	if importCalls.Load() != 1 {
		t.Fatalf("existing current was re-imported: calls=%d", importCalls.Load())
	}
}

func TestEnsureComputerSurfaceKeepsFailClosedWhenRouteOrBaselineUnavailable(t *testing.T) {
	computerID := "computer-surface-fail-closed"
	updaterRoot := filepath.Join(t.TempDir(), "updater")
	rt := &Runtime{
		cfg:                provideriface.Config{ComputerID: computerID},
		selfdevUpdaterRoot: updaterRoot,
	}
	if err := rt.EnsureComputerSurface(context.Background()); err == nil || !strings.Contains(err.Error(), "served SPA is underivable") {
		t.Fatalf("missing baseline returned err=%v", err)
	}
	if _, err := os.Lstat(filepath.Join(updaterRoot, "current")); !os.IsNotExist(err) {
		t.Fatalf("fail-closed bootstrap created current: %v", err)
	}
}

func TestEnsureComputerSurfaceLeavesExistingCurrentUntouched(t *testing.T) {
	computerID := "computer-surface-existing"
	updaterRoot := filepath.Join(t.TempDir(), "updater")
	digest, _ := pinFrontendRelease(t, updaterRoot, computerID, "existing-current")
	pointCurrent(t, updaterRoot, digest)
	rt := &Runtime{cfg: provideriface.Config{ComputerID: computerID}, selfdevUpdaterRoot: updaterRoot}
	if err := rt.EnsureComputerSurface(context.Background()); err != nil {
		t.Fatalf("existing current was refused: %v", err)
	}
	served, err := os.ReadFile(filepath.Join(updaterRoot, "current", "frontend", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if string(served) != "existing-current" {
		t.Fatalf("existing current changed to %q", served)
	}
}

func surfaceBootstrapRoute(t *testing.T, ownerID, desktopID string) (string, vmctl.RouteResolution) {
	t.Helper()
	createdAt := time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC)
	contentDigest := strings.Repeat("a", 64)
	closure, err := computerversion.NewCodeClosure(strings.Repeat("b", 40), []computerversion.CodeArtifact{{
		Name: "autoputer", SHA256: contentDigest, URI: "artifact+sha256://" + contentDigest + "/autoputer",
	}}, createdAt)
	if err != nil {
		t.Fatal(err)
	}
	program, err := computerversion.NewArtifactProgram([]computerversion.ArtifactProgramEntry{{
		Kind: "autoputer", ContentSHA256: contentDigest, ArtifactURI: "artifact+sha256://" + contentDigest + "/autoputer",
	}}, createdAt)
	if err != nil {
		t.Fatal(err)
	}
	slotID, err := routeledger.RouteSlotID(ownerID, desktopID)
	if err != nil {
		t.Fatal(err)
	}
	version := computerversion.ComputerVersion{CodeRef: closure.Ref, ArtifactProgramRef: program.Ref}
	receiptID := routeledger.ReceiptID("receipt-surface-bootstrap")
	return slotID, vmctl.RouteResolution{
		Slot:          routeledger.Slot{ID: slotID, Current: version, Generation: 1, LatestReceiptID: receiptID},
		LatestReceipt: routeledger.TransitionReceipt{ID: receiptID, RouteSlotID: slotID, New: version, CommittedGeneration: 1},
		CodeClosure:   closure, ArtifactProgram: program,
	}
}
