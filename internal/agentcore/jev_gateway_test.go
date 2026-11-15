package agentcore

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/gatewayruntime"
)

// The failure tests go through the production client (gatewayruntime.
// JudgmentClient, the same adapter autoputer's candidateJudge is) against an
// httptest gateway that answers exactly as internal/gateway/jev.go does:
// every upstream failure, a vendor refusal for size included, is 502 with
// {"error":"Jev judgment provider unavailable"}, a rate refusal is 429 with
// {"error":"Jev judgment rate limit exceeded"} and Retry-After. The caller
// sees no vendor reason, so no fake in these tests may return an error shape
// production cannot (docs/problems/jev-misuse-structural-assessment-2026-11-15.md).

// fakeGateway serves /provider/v1/judgments. answer decides per request: nil to
// answer through inner, or a status and body to answer with.
type fakeGateway struct {
	srv      *httptest.Server
	inner    CandidateJudge
	mu       sync.Mutex
	requests int
	// refuse returns a gateway failure for request n (1-based) carrying
	// questions questions, or zero to answer.
	refuse func(n, questions int) int
}

func newFakeGateway(t *testing.T, inner CandidateJudge, refuse func(n, questions int) int) (*fakeGateway, CandidateJudge) {
	t.Helper()
	g := &fakeGateway{inner: inner, refuse: refuse}
	g.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req gatewayruntime.JudgmentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad", http.StatusBadRequest)
			return
		}
		g.mu.Lock()
		g.requests++
		n := g.requests
		g.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch g.refuse(n, len(req.Questions)) {
		case http.StatusBadGateway:
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{"error":"Jev judgment provider unavailable"}`))
			return
		case http.StatusBadRequest:
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"invalid judgment request"}`))
			return
		case http.StatusTooManyRequests:
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"Jev judgment rate limit exceeded"}`))
			return
		}
		raw, err := g.inner.Judge(r.Context(), req.State, req.Questions)
		if err != nil {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{"error":"Jev judgment provider unavailable"}`))
			return
		}
		_, _ = w.Write(raw)
	}))
	t.Cleanup(g.srv.Close)
	client := gatewayruntime.NewJudgmentClient(g.srv.URL, "test-token")
	return g, gatewayJudge{client: client}
}

func (g *fakeGateway) count() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.requests
}

// gatewayJudge is autoputer.candidateJudge: the production adapter from the
// gateway client to the runtime's CandidateJudge seam.
type gatewayJudge struct{ client *gatewayruntime.JudgmentClient }

func (j gatewayJudge) Judge(ctx context.Context, state json.RawMessage, questions map[string]json.RawMessage) (json.RawMessage, error) {
	return j.JudgeScoped(ctx, "", state, questions)
}

func (j gatewayJudge) JudgeScoped(ctx context.Context, scope string, state json.RawMessage, questions map[string]json.RawMessage) (json.RawMessage, error) {
	response, err := j.client.Decide(ctx, gatewayruntime.JudgmentRequest{State: state, Questions: questions, Scope: scope})
	if err != nil {
		return nil, err
	}
	return response.Distribution, nil
}

func fastRetries(t *testing.T) {
	t.Helper()
	prev := intakeRetryBackoff
	intakeRetryBackoff = time.Millisecond
	t.Cleanup(func() { intakeRetryBackoff = prev })
}
