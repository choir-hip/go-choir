package pii

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Synthetic PII only. The SLM redactor is exercised against a stub Ollama
// server so no real model is required for the interface contract test.

func TestSLMRedactor_Name(t *testing.T) {
	s := NewSLMRedactor("llama3.2:3b")
	if s.Name() != "slm-ollama:llama3.2:3b" {
		t.Fatalf("name = %q", s.Name())
	}
}

func TestSLMRedactor_RedactsViaStubServer(t *testing.T) {
	// Stub Ollama /api/chat: returns a JSON array of findings.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/api/chat") {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		var req slmChatRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Model != "llama3.2:3b" {
			t.Errorf("model = %q", req.Model)
		}
		// Echo back a finding for the email embedded in the user text.
		resp := slmChatResponse{Message: slmChatMessage{
			Role: "assistant",
			Content: `[{"class":"email","text":"zoe@example.com"}]`,
		}}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	s := NewSLMRedactor("llama3.2:3b", WithSLMBaseURL(srv.URL), WithSLMFallback(nil))
	in := "please email zoe@example.com for the report"
	out, findings, err := s.RedactText(in)
	if err != nil {
		t.Fatalf("redact: %v", err)
	}
	if strings.Contains(out, "zoe@example.com") {
		t.Fatalf("raw PII leaked: %q", out)
	}
	if !strings.Contains(out, RedactionToken(ClassEmail)) {
		t.Fatalf("expected email token, got %q", out)
	}
	if len(findings) != 1 || findings[0].Class != ClassEmail {
		t.Fatalf("expected 1 email finding, got %+v", findings)
	}
}

func TestSLMRedactor_FallsBackOnServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "model not loaded", http.StatusInternalServerError)
	}))
	defer srv.Close()

	// Fallback is the regex redactor; it must catch the email even though
	// the SLM path failed.
	s := NewSLMRedactor("llama3.2:3b", WithSLMBaseURL(srv.URL))
	in := "reach amy@example.com anytime"
	out, findings, err := s.RedactText(in)
	if err != nil {
		t.Fatalf("redact: %v", err)
	}
	if strings.Contains(out, "amy@example.com") {
		t.Fatalf("raw PII leaked via fallback: %q", out)
	}
	if len(findings) == 0 {
		t.Fatalf("fallback produced no findings")
	}
	if findings[0].Class != ClassEmail {
		t.Fatalf("fallback finding class = %q", findings[0].Class)
	}
}

func TestSLMRedactor_FailClosedWhenNoFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down", http.StatusBadGateway)
	}))
	defer srv.Close()

	s := NewSLMRedactor("llama3.2:3b", WithSLMBaseURL(srv.URL), WithSLMFallback(nil))
	out, _, err := s.RedactText("anything")
	if err == nil {
		t.Fatalf("expected error when no fallback configured, got %q", out)
	}
}

func TestSLMRedactor_StripsCodeFences(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := slmChatResponse{Message: slmChatMessage{
			Content: "```json\n[{\"class\":\"email\",\"text\":\"kim@example.com\"}]\n```",
		}}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	s := NewSLMRedactor("llama3.2:3b", WithSLMBaseURL(srv.URL), WithSLMFallback(nil))
	out, _, err := s.RedactText("mail kim@example.com")
	if err != nil {
		t.Fatalf("redact: %v", err)
	}
	if strings.Contains(out, "kim@example.com") {
		t.Fatalf("raw PII leaked: %q", out)
	}
}

func TestSLMRedactor_EmptyResponseNoFindings(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := slmChatResponse{Message: slmChatMessage{Content: "[]"}}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	s := NewSLMRedactor("llama3.2:3b", WithSLMBaseURL(srv.URL), WithSLMFallback(nil))
	out, findings, err := s.RedactText("no pii here at all")
	if err != nil {
		t.Fatalf("redact: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %+v", findings)
	}
	if out != "no pii here at all" {
		t.Fatalf("text changed without findings: %q", out)
	}
}

func TestNormalizeClass(t *testing.T) {
	cases := map[string]PIIClass{
		"email":       ClassEmail,
		"EMAIL":       ClassEmail,
		"credit_card": ClassCreditCard,
		"creditcard":  ClassCreditCard,
		"card":        ClassCreditCard,
		"apikey":      ClassAPIKey,
		"ip_address":  ClassIP,
		"weird":       ClassUnknown,
		"":            ClassUnknown,
	}
	for in, want := range cases {
		if got := normalizeClass(in); got != want {
			t.Errorf("normalizeClass(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSLMRedactor_MultipleOccurrences(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := slmChatResponse{Message: slmChatMessage{
			Content: `[{"class":"email","text":"dup@example.com"}]`,
		}}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	s := NewSLMRedactor("llama3.2:3b", WithSLMBaseURL(srv.URL), WithSLMFallback(nil))
	in := "from dup@example.com to dup@example.com"
	out, findings, err := s.RedactText(in)
	if err != nil {
		t.Fatalf("redact: %v", err)
	}
	if len(findings) != 2 {
		t.Fatalf("expected 2 occurrences, got %+v", findings)
	}
	if strings.Contains(out, "dup@example.com") {
		t.Fatalf("raw PII leaked: %q", out)
	}
}
