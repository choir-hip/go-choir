//go:build comprehensive

package agentcore

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/provider"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestPromptBarBareURLRoutesToDisplayApp(t *testing.T) {
	t.Parallel()
	_, handler := testAPISetup(t)
	body := `{"text":"https://example.com/report.pdf"}`
	req := authenticatedRequest(http.MethodPost, "/api/prompt-bar", body, "user-content")
	w := httptest.NewRecorder()
	handler.HandlePromptBar(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	var submitted promptBarSubmitResponse
	if err := json.Unmarshal(w.Body.Bytes(), &submitted); err != nil {
		t.Fatalf("decode submission: %v", err)
	}

	var status promptBarSubmissionStatusResponse
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		statusReq := authenticatedRequest(http.MethodGet, submitted.StatusURL, "", "user-content")
		statusW := httptest.NewRecorder()
		handler.HandlePromptBarSubmission(statusW, statusReq)
		if statusW.Code != http.StatusOK {
			t.Fatalf("status lookup = %d body=%s", statusW.Code, statusW.Body.String())
		}
		if err := json.Unmarshal(statusW.Body.Bytes(), &status); err != nil {
			t.Fatalf("decode status: %v", err)
		}
		if status.Decision != nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if status.Decision == nil {
		t.Fatalf("timed out waiting for conductor decision: %#v", status)
	}
	if status.Decision.App != "pdf" {
		t.Fatalf("decision app = %q, want pdf", status.Decision.App)
	}
	if status.Decision.SourceURL != "https://example.com/report.pdf" {
		t.Fatalf("source_url = %q", status.Decision.SourceURL)
	}
}

func TestPromptBarBareURLDoesNotRequireProvider(t *testing.T) {
	t.Parallel()
	rt, handler := testAPISetup(t)
	rt.provider = &provider.StubProvider{Delay: 10 * time.Millisecond, FailErr: errors.New("provider unavailable")}
	body := `{"text":"https://example.com/report.pdf"}`
	req := authenticatedRequest(http.MethodPost, "/api/prompt-bar", body, "user-content")
	w := httptest.NewRecorder()
	handler.HandlePromptBar(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	var submitted promptBarSubmitResponse
	if err := json.Unmarshal(w.Body.Bytes(), &submitted); err != nil {
		t.Fatalf("decode submission: %v", err)
	}

	statusReq := authenticatedRequest(http.MethodGet, submitted.StatusURL, "", "user-content")
	statusW := httptest.NewRecorder()
	handler.HandlePromptBarSubmission(statusW, statusReq)
	if statusW.Code != http.StatusOK {
		t.Fatalf("status lookup = %d body=%s", statusW.Code, statusW.Body.String())
	}
	var status promptBarSubmissionStatusResponse
	if err := json.Unmarshal(statusW.Body.Bytes(), &status); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if status.State != types.RunCompleted {
		t.Fatalf("state = %q, want completed", status.State)
	}
	if status.Decision == nil || status.Decision.App != "pdf" {
		t.Fatalf("decision = %#v, want pdf decision", status.Decision)
	}
	if strings.Contains(status.Error, "provider unavailable") {
		t.Fatalf("bare URL routing leaked provider error: %q", status.Error)
	}
}

func TestPromptBarContextualURLRoutesToTexture(t *testing.T) {
	t.Parallel()
	_, handler := testAPISetup(t)
	body := `{"text":"Summarize https://example.com/report.pdf for a research note"}`
	req := authenticatedRequest(http.MethodPost, "/api/prompt-bar", body, "user-content")
	w := httptest.NewRecorder()
	handler.HandlePromptBar(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	var submitted promptBarSubmitResponse
	if err := json.Unmarshal(w.Body.Bytes(), &submitted); err != nil {
		t.Fatalf("decode submission: %v", err)
	}

	var status promptBarSubmissionStatusResponse
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		statusReq := authenticatedRequest(http.MethodGet, submitted.StatusURL, "", "user-content")
		statusW := httptest.NewRecorder()
		handler.HandlePromptBarSubmission(statusW, statusReq)
		if statusW.Code != http.StatusOK {
			t.Fatalf("status lookup = %d body=%s", statusW.Code, statusW.Body.String())
		}
		if err := json.Unmarshal(statusW.Body.Bytes(), &status); err != nil {
			t.Fatalf("decode status: %v", err)
		}
		if status.Decision != nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if status.Decision == nil {
		t.Fatalf("timed out waiting for conductor decision: %#v", status)
	}
	if status.Decision.App != "texture" {
		t.Fatalf("decision app = %q, want texture", status.Decision.App)
	}
	if status.Decision.SourceURL != "" {
		t.Fatalf("contextual URL should not be routed as bare source_url, got %q", status.Decision.SourceURL)
	}
}
