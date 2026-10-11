package proxy

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/platform"
	"github.com/yusefmosiah/go-choir/internal/server"
	"github.com/yusefmosiah/go-choir/internal/sources"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

// Guests reach corpusd only through the proxy
// (problems/capture-route-unreachable-through-proxy-2026-10-11.md). Failure
// modes pinned: the capture routes are not forwarded; a guest writes
// captures for another computer (corpusd trusts the proxy as an internal
// caller, so the proxy must check); a bound guest cannot read a body; an
// unbound caller reads one; a second computer cannot read a capture's
// metadata and text.
func TestGuestCapturesCrossTheProxyToCorpusd(t *testing.T) {
	mem := objectgraph.NewMemoryStore()
	svc := objectgraph.NewService(objectgraph.Config{Memory: mem, Durable: mem})
	defer svc.Close()
	og := platform.NewObjectGraphHandler(svc, mem)
	og.ConfigureCaptureBodies(t.TempDir())
	corpusMux := http.NewServeMux()
	corpusMux.HandleFunc("/internal/platform/captures", og.HandleCaptures)
	corpusMux.HandleFunc("/internal/platform/captures/bodies/", og.HandleCaptureBody)
	corpusMux.HandleFunc("/internal/platform/captures/", og.HandleCaptureRead)
	corpusd := httptest.NewServer(corpusMux)
	defer corpusd.Close()

	guests := map[string]vmctl.GuestOwnership{
		"10.200.1.2": {Found: true, UserID: "paper-owner", ComputerID: "computer-paper"},
		"10.200.9.2": {Found: true, UserID: "reader-owner", ComputerID: "computer-reader"},
	}
	vmctlServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			RemoteAddr string `json:"remote_addr"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		owner := guests[strings.Split(req.RemoteAddr, ":")[0]]
		_ = json.NewEncoder(w).Encode(owner)
	}))
	defer vmctlServer.Close()

	handler, _, _ := testProxyEnv(t)
	handler.cfg.CorpusdURL = corpusd.URL
	handler.vmctlClient = vmctl.NewClient(vmctlServer.URL)
	srv := server.NewServer("proxy-test", "0")
	RegisterRoutes(srv, handler)

	send := func(method, path, from string, body any) *httptest.ResponseRecorder {
		t.Helper()
		var raw []byte
		if body != nil {
			raw, _ = json.Marshal(body)
		}
		req := httptest.NewRequest(method, path, bytes.NewReader(raw))
		req.RemoteAddr = from + ":40000"
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		return rec
	}
	items := []sources.Item{{ID: "item-1", SourceID: "rss:example", SourceType: sources.SourceTypeRSS,
		Title: "One", Body: "Body one", URL: "https://example.com/1", FetchedAt: time.Date(2026, 10, 11, 10, 0, 0, 0, time.UTC)}}

	if rec := send(http.MethodPost, "/internal/platform/captures", "10.200.1.2",
		platform.CaptureWriteRequest{OwnerID: "reader-owner", ComputerID: "computer-reader", Items: items}); rec.Code != http.StatusForbidden {
		t.Fatalf("guest wrote captures for another computer through the proxy: %d %s", rec.Code, rec.Body)
	}
	rec := send(http.MethodPost, "/internal/platform/captures", "10.200.1.2",
		platform.CaptureWriteRequest{OwnerID: "paper-owner", ComputerID: "computer-paper", Items: items})
	if rec.Code != http.StatusOK {
		t.Fatalf("capture write through the proxy = %d %s", rec.Code, rec.Body)
	}
	var written platform.CaptureWriteResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &written); err != nil || written.New != 1 {
		t.Fatalf("capture write = %+v (%v)", written, err)
	}
	obj, err := svc.GetObject(t.Context(), written.Captured[0].CaptureID)
	if err != nil {
		t.Fatal(err)
	}
	meta, err := objectgraph.WebCaptureMetadataFromObject(obj)
	if err != nil {
		t.Fatal(err)
	}
	bodyPath := "/internal/platform/captures/bodies/" + strings.TrimPrefix(meta.ContentBlobID, "sha256:")
	read := send(http.MethodGet, bodyPath, "10.200.9.2", nil)
	if read.Code != http.StatusOK || read.Body.String() != "Body one" {
		t.Fatalf("second computer body read through the proxy = %d %q", read.Code, read.Body)
	}
	capture := send(http.MethodGet, "/internal/platform/captures/"+written.Captured[0].CaptureID, "10.200.9.2", nil)
	var got platform.CaptureReadResponse
	if capture.Code != http.StatusOK || json.Unmarshal(capture.Body.Bytes(), &got) != nil || got.Body != "Body one" || got.Title != "One" || got.SourceID != "rss:example" {
		t.Fatalf("second computer capture read through the proxy = %d %s", capture.Code, capture.Body)
	}
	if rec := send(http.MethodGet, bodyPath, "10.200.5.5", nil); rec.Code != http.StatusForbidden {
		t.Fatalf("unbound caller body read = %d", rec.Code)
	}
}
