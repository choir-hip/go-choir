package gatewayruntime

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
)

// The guest names the desk and run on every gateway model call (first design,
// F0). Failure list, written before the code:
//  1. A call made inside a run's execution context sends no desk or run.
//  2. An explicit caller (host code, a call with no run) does not win over
//     the execution context, or sends nothing.
//  3. A call with neither sends an empty header instead of none.

func captureHeaders(t *testing.T, ctx context.Context) http.Header {
	t.Helper()
	var got http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	p := New(srv.URL, "tok")
	_, _ = p.do(ctx, llmRequest{Model: "m"}, "")
	return got
}

func TestModelCallSendsTheDeskAndRunOfItsExecutionContext(t *testing.T) {
	ctx := toolregistry.WithExecutionContext(context.Background(), toolregistry.ExecutionContext{RunID: "run-5", Role: "management", Profile: "other"})
	h := captureHeaders(t, ctx)
	if h.Get("X-Choir-Desk") != "management" || h.Get("X-Choir-Run") != "run-5" {
		t.Fatalf("(1) headers = %v", h)
	}
	ctx = toolregistry.WithExecutionContext(context.Background(), toolregistry.ExecutionContext{RunID: "run-6", Profile: "texture"})
	if h := captureHeaders(t, ctx); h.Get("X-Choir-Desk") != "texture" {
		t.Fatalf("(1) profile fallback: %v", h)
	}
}

func TestExplicitCallerWinsAndHostIsNamed(t *testing.T) {
	ctx := toolregistry.WithExecutionContext(context.Background(), toolregistry.ExecutionContext{RunID: "run-5", Role: "management"})
	ctx = provideriface.WithCaller(ctx, provideriface.Caller{Desk: provideriface.DeskHost})
	h := captureHeaders(t, ctx)
	if h.Get("X-Choir-Desk") != "host" || h.Get("X-Choir-Run") != "run-5" {
		t.Fatalf("(2) headers = %v", h)
	}
	h = captureHeaders(t, provideriface.WithCaller(context.Background(), provideriface.Caller{Desk: "texture", RunID: "algorithm2:o"}))
	if h.Get("X-Choir-Desk") != "texture" || h.Get("X-Choir-Run") != "algorithm2:o" {
		t.Fatalf("(2) explicit only: %v", h)
	}
}

func TestNoCallerSendsNoHeader(t *testing.T) {
	h := captureHeaders(t, context.Background())
	if _, ok := h["X-Choir-Desk"]; ok {
		t.Fatalf("(3) desk header sent empty: %v", h)
	}
	if _, ok := h["X-Choir-Run"]; ok {
		t.Fatalf("(3) run header sent empty: %v", h)
	}
}

// The Jev client names the desk and run as the inference client does, and a
// research child (profile research, role research_web, the parent's run id)
// logs its role as the desk and the parent's run: the child has no run of its
// own, and the role is what the search lines already carry.
func TestJudgmentClientSendsTheDeskAndRun(t *testing.T) {
	var got http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	c := NewJudgmentClient(srv.URL, "tok")
	ctx := toolregistry.WithExecutionContext(context.Background(), toolregistry.ExecutionContext{RunID: "run-parent", Profile: "research", Role: "research_web"})
	_, _ = c.Decide(ctx, JudgmentRequest{})
	if got.Get("X-Choir-Desk") != "research_web" || got.Get("X-Choir-Run") != "run-parent" {
		t.Fatalf("headers = %v", got)
	}
	_, _ = c.Decide(provideriface.WithCaller(context.Background(), provideriface.Caller{Desk: provideriface.DeskHost}), JudgmentRequest{})
	if got.Get("X-Choir-Desk") != "host" || got.Get("X-Choir-Run") != "" {
		t.Fatalf("host headers = %v", got)
	}
}
