package platform

// The highest level the harness allows for item 2 of the own-read fix
// (docs/problems/own-published-pages-unreadable-by-host-fetch-2026-11-10.md,
// F12): a guest reads a story's read_url through the real guest transport and
// the real host fetch executor, whose public host resolves to the host's own
// address (so the old guard refuses it); the bytes it gets are captured under
// that URL; a later story citing the capture publishes with a relies_on edge.
// The public surface behind the loopback is a stub of the proxy's raw export
// over the platform service's own export.

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/hostfetch"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

func TestResearchReadOfAReadURLYieldsACaptureThatRecordsReliance(t *testing.T) {
	l := newRelatedLab(t)
	earlier := l.publish("doc-earlier", 1, "Harbor reopens")

	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/platform/publications/export" || r.URL.Query().Get("raw") != "1" {
			http.NotFound(w, r)
			return
		}
		out, err := l.svc.ExportPublicationByRoute(r.Context(), r.URL.Query().Get("route"), r.URL.Query().Get("format"))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = io.WriteString(w, out.Content)
	}))
	defer proxy.Close()
	site, err := hostfetch.NewOwnSite(l.svc.publicBase(), proxy.URL)
	if err != nil {
		t.Fatal(err)
	}
	const hostIP = "203.0.113.7"
	exec := &hostfetch.Executor{
		OwnSite: site,
		Lookup: func(context.Context, string) ([]net.IPAddr, error) {
			return []net.IPAddr{{IP: net.ParseIP(hostIP)}}, nil
		},
		Dial: func(context.Context, string, string) (net.Conn, error) {
			t.Error("the host dialled the public address")
			return nil, io.ErrClosedPipe
		},
		Self: func(ip net.IP) bool { return ip.Equal(net.ParseIP(hostIP)) },
	}
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { exec.Serve(w, r) }))
	defer gateway.Close()
	guest := &http.Client{Transport: &hostfetch.Transport{BaseURL: gateway.URL, Token: func() string { return "t" }, Base: http.DefaultTransport}}

	readURL := l.svc.publicationExportReadURL(earlier.RoutePath)
	resp, err := guest.Get(readURL)
	if err != nil {
		t.Fatalf("the guest could not read %s: %v", readURL, err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	text := strings.TrimSpace(strings.SplitN(string(body), "\n", 2)[0])
	if resp.StatusCode != http.StatusOK || text == "" {
		t.Fatalf("read %s = %d %q", readURL, resp.StatusCode, body)
	}

	captured := l.capture(readURL, "", text)
	l.publish("doc-later", 1, "Ferries resume", captured)
	edges := l.reliesOn(l.pubID("doc-later"))
	if len(edges) != 1 || edges[0].ToID != l.pubID("doc-earlier") || edges[0].Kind != objectgraph.EdgeReliesOn {
		t.Fatalf("relies_on edges = %+v, want one to the earlier story", edges)
	}

	// The public page form of the same story resolves to the same publication.
	page := l.capture(l.svc.publicBase()+earlier.RoutePath, "", "Harbor page text")
	l.publish("doc-third", 1, "Ferries resume again", page)
	if third := l.reliesOn(l.pubID("doc-third")); len(third) != 1 || third[0].ToID != l.pubID("doc-earlier") {
		t.Fatalf("page-form relies_on edges = %+v", third)
	}
}
