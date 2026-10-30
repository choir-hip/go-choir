package search

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/hostlink"
)

// The guest reads the gateway's NDJSON search stream (Cut 2 unit 2a,
// docs/archive/definitions/choir-reeng-cut2-evidence-to-publication-2026-10-12.md).
// Ways it can fail:
//  1. The client reads the whole body before returning a line, so a slow
//     provider delays every earlier provider's results.
//  2. A per-attempt bound (the old 30 s client) cuts a stream the gateway
//     is still writing; the refused-dial retry across a host restart is lost.
//  3. A stream that ends without its final line reads as a clean finish.
//  4. A gateway refusal (status and error body) is lost or misread as lines.

func TestNewGatewayClientFromEnvRequiresURLAndToken(t *testing.T) {
	t.Setenv("RUNTIME_GATEWAY_URL", "")
	t.Setenv("PROXY_VMCTL_URL", "")
	t.Setenv("RUNTIME_GATEWAY_TOKEN", "")
	if got := NewGatewayClientFromEnv(); got != nil {
		t.Fatalf("client without URL or token = %#v, want nil", got)
	}

	t.Setenv("RUNTIME_GATEWAY_URL", "https://gateway.example")
	if got := NewGatewayClientFromEnv(); got != nil {
		t.Fatalf("client without token = %#v, want nil", got)
	}

	t.Setenv("RUNTIME_GATEWAY_URL", "")
	t.Setenv("RUNTIME_GATEWAY_TOKEN", "token")
	if got := NewGatewayClientFromEnv(); got != nil {
		t.Fatalf("client without URL = %#v, want nil", got)
	}
}

func TestNewGatewayClientFromEnvHasNoAttemptBoundAndKeepsTheRetry(t *testing.T) {
	t.Setenv("RUNTIME_GATEWAY_URL", "  https://runtime-gateway.example  ")
	t.Setenv("PROXY_VMCTL_URL", "https://proxy-gateway.example")
	t.Setenv("RUNTIME_GATEWAY_TOKEN", "  secret-token  ")

	client, ok := NewGatewayClientFromEnv().(*gatewaySearchClient)
	if !ok {
		t.Fatalf("client type = %T, want *gatewaySearchClient", NewGatewayClientFromEnv())
	}
	if client.baseURL != "https://runtime-gateway.example" {
		t.Fatalf("baseURL = %q, want runtime gateway", client.baseURL)
	}
	if client.token != "secret-token" {
		t.Fatalf("token = %q, want trimmed token", client.token)
	}
	transport, ok := client.httpClient.Transport.(*hostlink.Transport)
	if !ok || transport.AttemptTimeout != 0 || client.httpClient.Timeout != 0 {
		t.Fatalf("http client = %#v, want a hostlink transport (refused-dial retry) with no attempt or client bound", client.httpClient)
	}

	t.Setenv("RUNTIME_GATEWAY_URL", "")
	client, ok = NewGatewayClientFromEnv().(*gatewaySearchClient)
	if !ok {
		t.Fatalf("fallback client type = %T, want *gatewaySearchClient", NewGatewayClientFromEnv())
	}
	if client.baseURL != "https://proxy-gateway.example" {
		t.Fatalf("fallback baseURL = %q, want proxy gateway", client.baseURL)
	}
}

func TestStreamSendsTheRequestAndReadsLinesAsTheyArrive(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/provider/v1/search" {
			t.Errorf("request = %s %s, want POST /provider/v1/search", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer gateway-token" {
			t.Errorf("Authorization = %q, want bearer token", got)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode request: %v", err)
		}
		if payload["query"] != "agentic systems" || payload["max_results"] != float64(40) {
			t.Errorf("payload = %#v", payload)
		}
		w.Header().Set("Content-Type", "application/x-ndjson")
		flush := func(line string) {
			_, _ = io.WriteString(w, line+"\n")
			w.(http.Flusher).Flush()
		}
		flush(`{"type":"start","query":"agentic systems","providers":["tavily","brave"],"cooling":[{"provider":"exa","state":"cooling_down","until":"2026-10-13T05:00:00Z"}]}`)
		flush(`{"type":"provider","provider":"tavily","status":"success","latency_ms":17,"results":[{"title":"Result","url":"https://example.test/result","snippet":"Evidence","provider":"tavily","score":0.75}]}`)
		<-release
		flush(`{"type":"provider","provider":"brave","status":"rate_limited","latency_ms":40,"results":[],"error":"status 429"}`)
		flush(`{"type":"done","answered":1,"total":2,"cooling":[{"provider":"exa","state":"cooling_down","until":"2026-10-13T05:00:00Z"}]}`)
	}))
	defer server.Close()
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()

	client := &gatewaySearchClient{baseURL: server.URL, token: "gateway-token", httpClient: server.Client()}
	stream, err := client.Stream(context.Background(), "agentic systems", 40)
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}
	defer stream.Close()
	if stream.Start.Query != "agentic systems" || strings.Join(stream.Start.Providers, ",") != "tavily,brave" {
		t.Fatalf("start = %+v", stream.Start)
	}
	if len(stream.Start.Cooling) != 1 || stream.Start.Cooling[0].Provider != "exa" {
		t.Fatalf("start cooling = %+v", stream.Start.Cooling)
	}

	first := make(chan Arrival, 1)
	go func() {
		a, err := stream.Next()
		if err != nil {
			t.Errorf("first Next() error = %v", err)
		}
		first <- a
	}()
	select {
	case a := <-first:
		if a.Provider != "tavily" || !a.Answered() || len(a.Results) != 1 || a.Results[0].URL != "https://example.test/result" || a.Results[0].Score != 0.75 {
			t.Fatalf("first arrival = %+v", a)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the first provider's line waited for the rest of the stream")
	}
	close(release)
	second, err := stream.Next()
	if err != nil || second.Provider != "brave" || second.Answered() || second.Error != "status 429" {
		t.Fatalf("second arrival = %+v, %v", second, err)
	}
	if _, err := stream.Next(); !errors.Is(err, io.EOF) {
		t.Fatalf("Next() after the final line = %v, want io.EOF", err)
	}
	done, ok := stream.Done()
	if !ok || done.Answered != 1 || done.Total != 2 || len(done.Cooling) != 1 {
		t.Fatalf("done = %+v (%v), want 1 of 2 with exa cooling", done, ok)
	}
}

func TestStreamOmitsNonPositiveMaxResults(t *testing.T) {
	for _, maxResults := range []int{0, -1} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Errorf("decode request: %v", err)
			}
			if _, ok := payload["max_results"]; ok {
				t.Errorf("payload = %#v, want max_results omitted", payload)
			}
			_, _ = io.WriteString(w, `{"type":"start","query":"q","providers":[],"cooling":[]}`+"\n")
		}))
		client := &gatewaySearchClient{baseURL: server.URL, token: "token", httpClient: server.Client()}
		stream, err := client.Stream(context.Background(), "q", maxResults)
		if err != nil {
			t.Fatalf("Stream() error = %v", err)
		}
		stream.Close()
		server.Close()
	}
}

func TestStreamEndingWithoutItsFinalLineIsAnError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"type":"start","query":"q","providers":["a"],"cooling":[]}`+"\n")
	}))
	defer server.Close()
	client := &gatewaySearchClient{baseURL: server.URL, token: "token", httpClient: server.Client()}
	stream, err := client.Stream(context.Background(), "q", 1)
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	if _, err := stream.Next(); err == nil || errors.Is(err, io.EOF) || !strings.Contains(err.Error(), "before its final line") {
		t.Fatalf("Next() on a cut stream = %v, want an error naming the missing final line", err)
	}
	if _, ok := stream.Done(); ok {
		t.Fatal("a cut stream reports a final line")
	}
}

func TestStreamRefusalsAndBadStarts(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		want       string
	}{
		{name: "gateway error", statusCode: http.StatusUnauthorized, body: `{"error":"credential rejected"}`, want: "gateway search: credential rejected"},
		{name: "generic status", statusCode: http.StatusTeapot, body: "not json", want: "gateway search: status 418 I'm a teapot"},
		{name: "malformed start", statusCode: http.StatusOK, body: "{\n", want: "gateway search: read start line:"},
		{name: "wrong first line", statusCode: http.StatusOK, body: `{"type":"done","answered":0,"total":0}` + "\n", want: `gateway search: first line is "done", want start`},
		{name: "empty body", statusCode: http.StatusOK, body: "", want: "gateway search: read start line:"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				_, _ = io.WriteString(w, tt.body)
			}))
			defer server.Close()
			client := &gatewaySearchClient{baseURL: server.URL, token: "token", httpClient: server.Client()}
			_, err := client.Stream(context.Background(), "q", 1)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Stream() error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestStreamRequestAndTransportErrors(t *testing.T) {
	client := &gatewaySearchClient{baseURL: ":", token: "token", httpClient: http.DefaultClient}
	if _, err := client.Stream(context.Background(), "q", 1); err == nil || !strings.Contains(err.Error(), "gateway search: create request:") {
		t.Fatalf("Stream() error = %v, want request creation error", err)
	}
	client = &gatewaySearchClient{
		baseURL: "https://gateway.example",
		token:   "token",
		httpClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("dial failed")
		})},
	}
	if _, err := client.Stream(context.Background(), "q", 1); err == nil || !strings.Contains(err.Error(), "gateway search: http call:") || !strings.Contains(err.Error(), "dial failed") {
		t.Fatalf("Stream() error = %v, want transport error", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
