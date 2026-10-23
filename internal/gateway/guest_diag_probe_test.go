package gateway

import (
	"bufio"
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/gatewaycapprobe"
)

// The guest's gateway body-cap diagnostic (internal/autoputer, record
// docs/problems/guest-proof-steps-have-no-route-2026-10-23.md) sends two
// requests with the computer's own token. This pins the claim that neither
// can ever reach a provider: the exact bytes the route sends, from the shared
// payload package, go to the real Handler with a counting model provider and
// a counting search provider, in observe and enforce mode, at the default
// limits and with every limit raised past the probe's sizes (the worst case:
// the caps do not stop it, only the shapes do).
func TestGuestDiagProbeBodiesNeverReachAProvider(t *testing.T) {
	cases := []struct {
		name, mode, limits string
		wantSearch         int // 0: any non-200 other than a provider path is fine
		wantModel          int
	}{
		{name: "enforce default", mode: "enforce", limits: "", wantSearch: http.StatusRequestEntityTooLarge, wantModel: http.StatusRequestEntityTooLarge},
		{name: "observe default", mode: "observe", limits: "", wantSearch: http.StatusRequestEntityTooLarge, wantModel: http.StatusRequestEntityTooLarge},
		// Caps raised past the probe: the search body is now under its cap
		// and the model body is expected, so the shapes alone must hold.
		{name: "enforce raised", mode: "enforce", limits: "search=64MiB,model=2GiB,model_ceiling=2GiB", wantSearch: http.StatusBadRequest},
		{name: "observe raised", mode: "observe", limits: "search=64MiB,model=2GiB,model_ceiling=2GiB", wantSearch: http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("GATEWAY_BODY_CAPS", tc.mode)
			t.Setenv("GATEWAY_BODY_LIMITS", tc.limits)
			h, reg, mp := setupHandler(t)
			search := answering("counting-search", 0, SearchResult{URL: "https://example.com", Title: "t"})
			h.searchClient = testSearchClient([]SearchProvider{search})
			cred, err := reg.IssueCredential("computer-guest-diag-probe")
			if err != nil {
				t.Fatal(err)
			}
			mux := http.NewServeMux()
			mux.HandleFunc(gatewaycapprobe.SearchPath, h.HandleSearch)
			mux.HandleFunc(gatewaycapprobe.ModelPath, h.HandleInference)
			srv := httptest.NewServer(mux)

			// Leg 1: the streamed (chunked) search body.
			req, err := http.NewRequest(http.MethodPost, srv.URL+gatewaycapprobe.SearchPath, gatewaycapprobe.SearchBody())
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Authorization", "Bearer "+cred.RawToken)
			req.Header.Set("Content-Type", "application/json")
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			resp, err := http.DefaultClient.Do(req.WithContext(ctx))
			if err != nil {
				t.Fatalf("search leg: %v", err)
			}
			_ = resp.Body.Close()
			if resp.StatusCode != tc.wantSearch {
				t.Fatalf("search leg = %d, want %d", resp.StatusCode, tc.wantSearch)
			}

			// Leg 2: the declared model body, no byte of it sent, then the
			// connection is dropped as the route does at its deadline.
			conn, err := net.Dial("tcp", strings.TrimPrefix(srv.URL, "http://"))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := conn.Write(gatewaycapprobe.ModelDeclaredHead(strings.TrimPrefix(srv.URL, "http://"), cred.RawToken)); err != nil {
				t.Fatal(err)
			}
			_ = conn.SetReadDeadline(time.Now().Add(700 * time.Millisecond))
			status := 0
			if r, err := http.ReadResponse(bufio.NewReader(conn), nil); err == nil {
				status = r.StatusCode
				_ = r.Body.Close()
			}
			_ = conn.Close()
			if tc.wantModel != 0 && status != tc.wantModel {
				t.Fatalf("model leg = %d, want %d", status, tc.wantModel)
			}
			if tc.wantModel == 0 && status == http.StatusOK {
				t.Fatalf("model leg answered 200 with no body")
			}

			// Let the server finish its handlers before reading the counters.
			time.Sleep(100 * time.Millisecond)
			srv.Close()
			mp.mu.Lock()
			reached := mp.lastReq
			mp.mu.Unlock()
			if reached != nil {
				t.Fatalf("the model provider was called")
			}
			if search.searchCount != 0 {
				t.Fatalf("the search provider was called %d time(s)", search.searchCount)
			}
		})
	}
}
