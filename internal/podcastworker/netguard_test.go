package podcastworker

// The worker fetches URLs strangers can supply (a reader's feed, an episode's
// enclosure). Ways it can fail, written first (review 1 of 2026-11-16, B1):
//   N1. a feed or enclosure on a loopback, private, link-local, CGNAT,
//       unspecified or multicast address is fetched, in any spelling;
//   N2. a public NAME that resolves to a private address is fetched (the
//       check must be on the address the connection goes to, after
//       resolution, so a rebinding answer cannot get past it);
//   N3. a redirect from a permitted host to a private one is followed;
//   N4. a port other than 80 or 443 is dialled;
//   N5. a redirect chain has no end;
//   N6. the key goes over cleartext to a non-loopback server;
//   N7. a downloaded playlist is handed to ffmpeg, which would open the
//       URLs it names.

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestN1_PrivateAddressesAreNeverPermitted(t *testing.T) {
	for _, bad := range []string{
		"127.0.0.1", "127.8.9.10", "::1", "10.0.0.5", "172.16.3.4", "172.31.255.255", "192.168.1.1", "169.254.169.254", "fe80::1",
		"fc00::1", "fd12:3456::1", "100.64.0.1", "100.127.255.255", "0.0.0.0", "::", "224.0.0.1", "ff02::1", "240.0.0.1", "198.18.0.1",
		"::ffff:127.0.0.1", "::ffff:10.1.2.3", "255.255.255.255",
		"::127.0.0.1", "::8.8.8.8", "2002:7f00:1::1", "2002:a00:2::1", "2001:0:4136:e378:8000:63bf:3fff:fdd2", "192.88.99.1",
	} {
		if permittedIP(net.ParseIP(bad)) {
			t.Errorf("N1: %s is permitted", bad)
		}
	}
	for _, good := range []string{"8.8.8.8", "1.1.1.1", "93.184.216.34", "2606:4700:4700::1111", "172.32.0.1", "100.128.0.1"} {
		if !permittedIP(net.ParseIP(good)) {
			t.Errorf("N1: %s is refused", good)
		}
	}
}

func TestN2_N4_AResolvedPrivateAddressAndOddPortsAreRefused(t *testing.T) {
	lookup := func(_ context.Context, host string) ([]net.IP, error) {
		switch host {
		case "rebind.example.test":
			return []net.IP{net.ParseIP("10.1.2.3")}, nil
		case "mixed.example.test":
			return []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("8.8.8.8")}, nil
		case "public.example.test":
			return []net.IP{net.ParseIP("8.8.4.4")}, nil
		}
		return nil, errors.New("no such host")
	}
	var dialed []string
	dial := func(_ context.Context, _, addr string) (net.Conn, error) {
		dialed = append(dialed, addr)
		return nil, errors.New("test: no network")
	}
	d := guardedDialer(lookup, dial)
	for _, addr := range []string{"rebind.example.test:443", "127.0.0.1:80", "169.254.169.254:80", "[::1]:443", "mixed.example.test:443", "10.0.0.1:443"} {
		if _, err := d(context.Background(), "tcp", addr); err == nil || !strings.Contains(err.Error(), "not permitted") {
			t.Errorf("N2: %s: err = %v", addr, err)
		}
	}
	for _, addr := range []string{"public.example.test:8080", "public.example.test:22", "8.8.8.8:25", "public.example.test:0"} {
		if _, err := d(context.Background(), "tcp", addr); err == nil || !strings.Contains(err.Error(), "port") {
			t.Errorf("N4: %s: err = %v", addr, err)
		}
	}
	if len(dialed) != 0 {
		t.Fatalf("N2: a refused address was dialled: %v", dialed)
	}
	// A permitted name is dialled by the address the check saw, not the name.
	_, _ = d(context.Background(), "tcp", "public.example.test:443")
	if len(dialed) != 1 || dialed[0] != "8.8.4.4:443" {
		t.Fatalf("N2: dialled %v, want the checked address 8.8.4.4:443", dialed)
	}
}

func TestN3_N5_ARedirectToAPrivateHostAndAnEndlessChainStop(t *testing.T) {
	var origin *httptest.Server
	mux := http.NewServeMux()
	mux.HandleFunc("/to-private", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://10.9.9.9/secret", http.StatusFound)
	})
	mux.HandleFunc("/loop", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, origin.URL+"/loop", http.StatusFound)
	})
	mux.HandleFunc("/ftp", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "ftp://example.test/x", http.StatusFound)
	})
	mux.HandleFunc("/ok", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("fine")) })
	origin = httptest.NewServer(mux)
	defer origin.Close()
	host := mustHostPort(t, origin.URL)
	// A policy that permits only the test origin, with the redirect rules of the real client.
	c := newGuardedClient(guardConfig{
		permit: func(ip net.IP, port string) error {
			if ip.String() == "127.0.0.1" && port == host.port {
				return nil
			}
			return errors.New("not permitted by the test policy")
		},
	})
	get := func(path string) error {
		resp, err := c.Get(origin.URL + path)
		if err == nil {
			_ = resp.Body.Close()
		}
		return err
	}
	if err := get("/ok"); err != nil {
		t.Fatalf("N3: the permitted origin: %v", err)
	}
	if err := get("/to-private"); err == nil || !strings.Contains(err.Error(), "not permitted") {
		t.Fatalf("N3: a redirect to a private host: %v", err)
	}
	if err := get("/loop"); err == nil || !strings.Contains(err.Error(), "redirect") {
		t.Fatalf("N5: an endless chain: %v", err)
	}
	if err := get("/ftp"); err == nil {
		t.Fatal("N3: a redirect to ftp was followed")
	}
}

type hostPort struct{ host, port string }

func mustHostPort(t *testing.T, raw string) hostPort {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return hostPort{host: u.Hostname(), port: u.Port()}
}

func TestN6_TheKeyDoesNotGoOverCleartextToAForeignServer(t *testing.T) {
	for _, bad := range []string{"http://choir.news", "http://example.test", "http://10.0.0.5:8080", "ftp://x"} {
		if _, err := NewClient(bad, "key"); err == nil {
			t.Errorf("N6: %s accepted", bad)
		}
	}
	for _, good := range []string{"https://choir.news", "http://127.0.0.1:8080", "http://localhost:3000", "http://[::1]:8080"} {
		if _, err := NewClient(good, "key"); err != nil {
			t.Errorf("N6: %s refused: %v", good, err)
		}
	}
}

func TestN7_APlaylistIsNeverHandedToFfmpeg(t *testing.T) {
	for _, head := range []string{"#EXTM3U\n#EXTINF:1,x\nhttp://10.0.0.1/a.mp3\n", "  \n#extm3u\n", "ffconcat version 1.0\nfile '/etc/passwd'\n", "\xef\xbb\xbf#EXTM3U\n"} {
		if !looksLikePlaylist([]byte(head)) {
			t.Errorf("N7: %q is not recognised as a playlist", head)
		}
	}
	for _, ok := range []string{"ID3\x04\x00\x00\x00", "\xff\xfb\x90\x00", "OggS\x00\x02", "RIFF....WAVE", "\x00\x00\x00\x20ftypM4A "} {
		if looksLikePlaylist([]byte(ok)) {
			t.Errorf("N7: audio %q taken for a playlist", ok)
		}
	}
}

func TestN7_ADownloadedPlaylistStopsBeforeFfmpegRuns(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Range") != "" {
			w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
			return
		}
		_, _ = w.Write([]byte("#EXTM3U\n#EXTINF:1,x\nhttp://10.0.0.1/a.mp3\n"))
	}))
	defer srv.Close()
	ran := filepath.Join(t.TempDir(), "ffmpeg-ran")
	stub := filepath.Join(t.TempDir(), "ffmpeg")
	if err := os.WriteFile(stub, []byte("#!/bin/sh\ntouch "+ran+"\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	tr := &Transcriber{FFmpeg: stub, Parakeet: "false", WorkDir: t.TempDir(), MinFreeBytes: -1, HTTP: srv.Client()}
	_, err := tr.Transcribe(context.Background(), srv.URL+"/a.mp3")
	if err == nil || !strings.Contains(err.Error(), "playlist") {
		t.Fatalf("N7: err = %v", err)
	}
	if _, statErr := os.Stat(ran); statErr == nil {
		t.Fatal("N7: ffmpeg ran on a playlist")
	}
}

func TestN8_ProbeAndFetchRefuseAPrivateHostThroughTheDefaultClient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("<rss/>")) }))
	defer srv.Close() // a loopback server: the real guard must refuse it
	if n := ProbeLength(context.Background(), SafeClient(5*time.Second), srv.URL); n != 0 {
		t.Fatalf("N8: probe reached a loopback host: %d", n)
	}
	if _, err := FetchFeed(context.Background(), SafeClient(5*time.Second), srv.URL, "x"); err == nil || !strings.Contains(err.Error(), "not permitted") {
		t.Fatalf("N8: a feed on a loopback host was fetched: %v", err)
	}
	tr := &Transcriber{FFmpeg: "false", Parakeet: "false", WorkDir: t.TempDir(), MinFreeBytes: -1}
	if _, err := tr.Transcribe(context.Background(), srv.URL+"/a.mp3"); err == nil || !strings.Contains(err.Error(), "not permitted") {
		t.Fatalf("N8: an enclosure on a loopback host was downloaded: %v", err)
	}
}
