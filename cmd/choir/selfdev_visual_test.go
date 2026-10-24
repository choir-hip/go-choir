package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// `choir self-dev visual` previews a frozen candidate's screenshots: it lists
// the visual-evidence captures and, with --out, fetches and verifies each PNG.
// Ways it could fail, written before the command:
//   - a corrupted or substituted PNG (proxy truncation, a wrong asset) is
//     written under a trusted-looking name and a preview shows the owner
//     something the candidate never rendered: every fetched body is hashed
//     against the listing row's digest and refused on mismatch, and a refused
//     body leaves no file (not even a partial or temp file) in --out;
//   - a resolvable:false row (the asset store lost the bytes) is skipped
//     silently and the exit code is 0: it must be named in the manifest and
//     the exit code must be non-zero, after the other captures are written;
//   - the route's 409 (not frozen yet) or 404 (no captures) is flattened into
//     a generic failure, or an empty manifest is printed as success;
//   - a hostile route or digest becomes a path (traversal, separators) or the
//     asset_url points somewhere other than the asset route: file names derive
//     from a validated 64-hex digest and a slugged route, and an asset_url that
//     is not /api/assets/sha256/<digest> is refused;
//   - files are world-readable or DIR is created 0755;
//   - without --out the command fetches bytes, or writes anything;
//   - --out leaks to MCP, which has no local filesystem, or MCP returns bytes.

var testPNG = append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0x42}, 64)...)

func hexOf(b []byte) string { sum := sha256.Sum256(b); return hex.EncodeToString(sum[:]) }

type visualRow struct {
	Artifact, Digest, Route string
	Width, Height           int
	Bytes                   int64
	Blank, Resolvable       bool
	AssetURL                string
}

func goodRow(route string, png []byte) visualRow {
	d := hexOf(png)
	return visualRow{Artifact: "artifact:sha256:" + d, Digest: d, Route: route, Width: 1280, Height: 720, Bytes: int64(len(png)), Resolvable: true, AssetURL: "/api/assets/sha256/" + d}
}

func visualListing(operationID string, rows ...visualRow) string {
	captures := []map[string]any{}
	for _, r := range rows {
		captures = append(captures, map[string]any{"artifact": r.Artifact, "digest": r.Digest, "route": r.Route,
			"width": r.Width, "height": r.Height, "bytes": r.Bytes, "blank": r.Blank, "resolvable": r.Resolvable, "asset_url": r.AssetURL})
	}
	raw, _ := json.Marshal(map[string]any{
		"operation_id": operationID, "computer_id": "c1", "state": "awaiting_approval",
		"bundle_digest": strings.Repeat("b", 64), "manifest_sha256": strings.Repeat("m", 8), "frontend_sha256": strings.Repeat("f", 8),
		"capture_receipt": "receipt:1", "captures": captures,
	})
	return string(raw)
}

// visualServer serves the listing (status listingStatus) and assets by path.
func visualServer(t *testing.T, listingStatus int, listing string, assets map[string][]byte) *[]string {
	t.Helper()
	var mu sync.Mutex
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.URL.EscapedPath())
		mu.Unlock()
		if strings.HasSuffix(r.URL.Path, "/visual-evidence") {
			w.WriteHeader(listingStatus)
			_, _ = w.Write([]byte(listing))
			return
		}
		if b, ok := assets[r.URL.Path]; ok {
			_, _ = w.Write(b)
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	t.Setenv(hostEnvVar, srv.URL)
	t.Setenv(apiKeyEnvVar, "choir_sk_visual")
	t.Setenv("CHOIR_COMPUTER", "")
	t.Setenv("CHOIR_API_KEY_FILE", "")
	t.Setenv(timeoutEnvVar, "")
	return &paths
}

func visualRun(args ...string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	code := run(append([]string{"self-dev", "visual", "--computer", "c1", "--operation", "op-1"}, args...), &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func dirEntries(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func TestSelfDevVisualListsWithoutFetchingBytes(t *testing.T) {
	row := goodRow("/choir-drill/home.html", testPNG)
	paths := visualServer(t, 200, visualListing("op-1", row), map[string][]byte{row.AssetURL: testPNG})
	code, stdout, stderr := visualRun()
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	var listing struct {
		OperationID string `json:"operation_id"`
		Captures    []struct {
			Route string `json:"route"`
		} `json:"captures"`
	}
	if err := json.Unmarshal([]byte(stdout), &listing); err != nil || listing.OperationID != "op-1" || len(listing.Captures) != 1 || listing.Captures[0].Route != "/choir-drill/home.html" {
		t.Fatalf("listing output %q (%v)", stdout, err)
	}
	if len(*paths) != 1 || !strings.HasSuffix((*paths)[0], "/self-development/operations/op-1/visual-evidence") {
		t.Fatalf("listing only must make exactly the listing request, got %v", *paths)
	}
}

func TestSelfDevVisualOutWritesVerifiedFiles(t *testing.T) {
	other := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0x07}, 32)...)
	a, b := goodRow("/choir-drill/home.html", testPNG), goodRow("/", other)
	b.Blank = true
	visualServer(t, 200, visualListing("op-1", a, b), map[string][]byte{a.AssetURL: testPNG, b.AssetURL: other})
	dir := filepath.Join(t.TempDir(), "shots")
	code, stdout, stderr := visualRun("--out", dir)
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if info, err := os.Stat(dir); err != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("out dir %v (%v), want mode 0700", info, err)
	}
	wantA, wantB := "choir-drill-home-html-"+a.Digest[:12]+".png", "root-"+b.Digest[:12]+".png"
	for name, want := range map[string][]byte{wantA: testPNG, wantB: other} {
		path := filepath.Join(dir, name)
		got, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("%s: %v / bytes equal %v", name, err, bytes.Equal(got, want))
		}
		if info, _ := os.Stat(path); info.Mode().Perm() != 0o600 {
			t.Fatalf("%s mode %v, want 0600", name, info.Mode().Perm())
		}
	}
	if names := dirEntries(t, dir); len(names) != 2 {
		t.Fatalf("out dir holds %v, want only the two verified PNGs", names)
	}
	var manifest struct {
		OperationID    string `json:"operation_id"`
		CaptureReceipt string `json:"capture_receipt"`
		Out            string `json:"out"`
		Captures       []struct {
			Route      string `json:"route"`
			File       string `json:"file"`
			Digest     string `json:"digest"`
			Bytes      int    `json:"bytes"`
			Blank      bool   `json:"blank"`
			Resolvable bool   `json:"resolvable"`
			Error      string `json:"error"`
		} `json:"captures"`
	}
	if err := json.Unmarshal([]byte(stdout), &manifest); err != nil || manifest.OperationID != "op-1" || manifest.CaptureReceipt != "receipt:1" || len(manifest.Captures) != 2 {
		t.Fatalf("manifest %q (%v)", stdout, err)
	}
	m := manifest.Captures[0]
	if m.Route != "/choir-drill/home.html" || m.File != filepath.Join(dir, wantA) || m.Digest != a.Digest || m.Bytes != len(testPNG) || !m.Resolvable || m.Error != "" {
		t.Fatalf("manifest row 0 %+v", m)
	}
	if !manifest.Captures[1].Blank {
		t.Fatalf("manifest row 1 lost blank: %+v", manifest.Captures[1])
	}
}

func TestSelfDevVisualRefusesDigestMismatchAndLeavesNothing(t *testing.T) {
	good, bad := goodRow("/good", testPNG), goodRow("/bad", []byte("claimed bytes"))
	// The asset route serves different bytes than the digest the row names.
	visualServer(t, 200, visualListing("op-1", bad, good), map[string][]byte{bad.AssetURL: []byte("substituted bytes"), good.AssetURL: testPNG})
	dir := filepath.Join(t.TempDir(), "shots")
	code, stdout, stderr := visualRun("--out", dir)
	if code != 1 || !strings.Contains(stderr, "sha256") {
		t.Fatalf("mismatch: exit %d stderr %q, want 1 naming the sha256", code, stderr)
	}
	names := dirEntries(t, dir)
	if len(names) != 1 || names[0] != "good-"+good.Digest[:12]+".png" {
		t.Fatalf("out dir holds %v, want only the verified capture and no file for the refused one", names)
	}
	if !strings.Contains(stdout, `"error"`) || !strings.Contains(stdout, "/bad") {
		t.Fatalf("manifest must name the refused capture, got %q", stdout)
	}
	var manifest struct {
		Captures []struct {
			Route, File, Error string
		} `json:"captures"`
	}
	_ = json.Unmarshal([]byte(stdout), &manifest)
	if len(manifest.Captures) != 2 || manifest.Captures[0].File != "" || manifest.Captures[0].Error == "" {
		t.Fatalf("refused row %+v must carry an error and no file", manifest.Captures)
	}
}

func TestSelfDevVisualUnresolvableRowIsAnErrorAfterWritingTheRest(t *testing.T) {
	lost, good := goodRow("/lost", []byte("gone")), goodRow("/good", testPNG)
	lost.Resolvable = false
	paths := visualServer(t, 200, visualListing("op-1", lost, good), map[string][]byte{good.AssetURL: testPNG})
	dir := filepath.Join(t.TempDir(), "shots")
	code, stdout, stderr := visualRun("--out", dir)
	if code == 0 {
		t.Fatalf("exit 0 with an unresolvable capture; stdout %q", stdout)
	}
	if !strings.Contains(stderr, "/lost") {
		t.Fatalf("stderr %q must name the unresolvable capture", stderr)
	}
	if names := dirEntries(t, dir); len(names) != 1 || !strings.HasPrefix(names[0], "good-") {
		t.Fatalf("out dir holds %v, want the resolvable capture written", names)
	}
	for _, p := range *paths {
		if strings.Contains(p, lost.Digest) {
			t.Fatalf("fetched the unresolvable asset: %v", *paths)
		}
	}
	if !strings.Contains(stdout, `"resolvable": false`) || !strings.Contains(stdout, `"error"`) {
		t.Fatalf("manifest must carry the unresolvable row with an error: %q", stdout)
	}
}

func TestSelfDevVisualSurfacesTheRoutesRefusals(t *testing.T) {
	visualServer(t, http.StatusConflict, `{"error":"operation is executing; visual evidence exists only once the candidate is frozen"}`, nil)
	dir := filepath.Join(t.TempDir(), "shots")
	code, stdout, stderr := visualRun("--out", dir)
	if code != 1 || !strings.Contains(stderr, "409") || !strings.Contains(stderr, "frozen") || stdout != "" {
		t.Fatalf("409: exit %d stdout %q stderr %q", code, stdout, stderr)
	}
	if names := dirEntries(t, dir); len(names) != 0 {
		t.Fatalf("409 left %v", names)
	}
	visualServer(t, http.StatusNotFound, `{"error":"the candidate carries no visual evidence"}`, nil)
	code, stdout, stderr = visualRun("--out", dir)
	if code != 1 || !strings.Contains(stderr, "404") || !strings.Contains(stderr, "no visual evidence") || stdout != "" {
		t.Fatalf("404: exit %d stdout %q stderr %q", code, stdout, stderr)
	}
}

func TestSelfDevVisualRefusesHostileListingRows(t *testing.T) {
	traversal := goodRow("/../../etc/passwd", testPNG)
	badDigest := goodRow("/x", testPNG)
	badDigest.Digest = "../../../evil"
	elsewhere := goodRow("/y", testPNG)
	elsewhere.AssetURL = "/api/other/" + elsewhere.Digest
	wrongOp := visualListing("op-2", goodRow("/z", testPNG))
	assets := map[string][]byte{traversal.AssetURL: testPNG, "/api/other/" + elsewhere.Digest: testPNG}

	visualServer(t, 200, visualListing("op-1", traversal), assets)
	parent := t.TempDir()
	dir := filepath.Join(parent, "shots")
	if code, _, stderr := visualRun("--out", dir); code != 0 {
		t.Fatalf("a hostile route is slugged, not refused: exit %d %q", code, stderr)
	}
	for _, name := range dirEntries(t, dir) {
		if strings.ContainsAny(name, `/\`) || strings.HasPrefix(name, ".") {
			t.Fatalf("route became path %q", name)
		}
	}
	if names := dirEntries(t, parent); len(names) != 1 {
		t.Fatalf("something was written outside --out: %v", names)
	}
	for name, listing := range map[string]string{"digest": visualListing("op-1", badDigest), "asset_url": visualListing("op-1", elsewhere), "operation": wrongOp} {
		visualServer(t, 200, listing, assets)
		d := filepath.Join(t.TempDir(), "shots")
		if code, _, _ := visualRun("--out", d); code == 0 {
			t.Fatalf("hostile %s accepted", name)
		}
		if names := dirEntries(t, d); len(names) != 0 {
			t.Fatalf("hostile %s wrote %v", name, names)
		}
	}
}

func TestSelfDevVisualMCPListsOnlyAndHasNoOut(t *testing.T) {
	row := goodRow("/choir-drill/home.html", testPNG)
	paths := visualServer(t, 200, visualListing("op-1", row), map[string][]byte{row.AssetURL: testPNG})
	result := toolCall(t, "self_dev_visual", `{"computer":"c1","operation":"op-1"}`)
	if result.IsError || !strings.Contains(result.Content[0].Text, row.Digest) || strings.Contains(result.Content[0].Text, "iVBOR") {
		t.Fatalf("mcp result %+v", result)
	}
	if len(*paths) != 1 {
		t.Fatalf("mcp fetched bytes: %v", *paths)
	}
	found := false
	for _, cmd := range mcpCommands() {
		if cmd.toolName() == "self_dev_visual" {
			found = true
			if _, ok := cmd.inputSchema()["properties"].(map[string]any)["out"]; ok {
				t.Fatal("--out leaked into the MCP schema")
			}
		}
	}
	if !found {
		t.Fatal("self_dev_visual is not an MCP tool")
	}
}
