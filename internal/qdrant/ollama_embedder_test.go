package qdrant

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestOllamaEmbedder_Model(t *testing.T) {
	e := NewOllamaEmbedder("http://localhost:11434", "batiai/qwen3-embedding:0.6b")
	m := e.Model()
	if m.Name != "batiai/qwen3-embedding:0.6b" {
		t.Fatalf("model name = %q, want batiai/qwen3-embedding:0.6b", m.Name)
	}
	if m.Dimensions != 1024 {
		t.Fatalf("dimensions = %d, want 1024", m.Dimensions)
	}
	if m.Version == "" {
		t.Fatal("version is empty")
	}
	if err := m.Validate(); err != nil {
		t.Fatalf("model validate: %v", err)
	}
}

func TestOllamaEmbedder_EmbedTexts_MockServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/api/embed" {
			t.Errorf("path = %s, want /api/embed", r.URL.Path)
		}

		var req ollamaEmbedRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if req.Model != "batiai/qwen3-embedding:0.6b" {
			t.Errorf("model = %q, want batiai/qwen3-embedding:0.6b", req.Model)
		}
		if len(req.Input) != 2 {
			t.Errorf("input length = %d, want 2", len(req.Input))
		}

		embeddings := make([][]float32, len(req.Input))
		for i := range req.Input {
			vec := make([]float32, 1024)
			for j := range vec {
				vec[j] = float32(i + 1)
			}
			embeddings[i] = vec
		}
		_ = json.NewEncoder(w).Encode(ollamaEmbedResponse{
			Model:      req.Model,
			Embeddings: embeddings,
		})
	}))
	defer srv.Close()

	e := NewOllamaEmbedder(srv.URL, "batiai/qwen3-embedding:0.6b")
	vectors, err := e.EmbedTexts(context.Background(), []string{"hello", "world"})
	if err != nil {
		t.Fatalf("EmbedTexts: %v", err)
	}
	if len(vectors) != 2 {
		t.Fatalf("vectors length = %d, want 2", len(vectors))
	}
	for i, vec := range vectors {
		if len(vec) != 1024 {
			t.Fatalf("vector %d dims = %d, want 1024", i, len(vec))
		}
	}
}

func TestOllamaEmbedder_EmbedTexts_ConnectionRefused(t *testing.T) {
	e := NewOllamaEmbedder("http://127.0.0.1:1", "batiai/qwen3-embedding:0.6b")
	_, err := e.EmbedTexts(context.Background(), []string{"hello"})
	if err == nil {
		t.Fatal("expected error for connection refused, got nil")
	}
	if !strings.Contains(err.Error(), "request failed") {
		t.Fatalf("error should mention request failed, got: %v", err)
	}
}

func TestOllamaEmbedder_EmbedTexts_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("{not valid json"))
	}))
	defer srv.Close()

	e := NewOllamaEmbedder(srv.URL, "batiai/qwen3-embedding:0.6b")
	_, err := e.EmbedTexts(context.Background(), []string{"hello"})
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
	if !strings.Contains(err.Error(), "decode response") {
		t.Fatalf("error should mention decode response, got: %v", err)
	}
}

func TestOllamaEmbedder_EmbedTexts_EmptyInput(t *testing.T) {
	e := NewOllamaEmbedder("http://localhost:11434", "batiai/qwen3-embedding:0.6b")
	_, err := e.EmbedTexts(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for empty input, got nil")
	}
	if !strings.Contains(err.Error(), "no texts provided") {
		t.Fatalf("error should mention no texts provided, got: %v", err)
	}
}

func TestOllamaEmbedder_EmbedTexts_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "model not found", http.StatusNotFound)
	}))
	defer srv.Close()

	e := NewOllamaEmbedder(srv.URL, "batiai/qwen3-embedding:0.6b")
	_, err := e.EmbedTexts(context.Background(), []string{"hello"})
	if err == nil {
		t.Fatal("expected error for HTTP 404, got nil")
	}
	if !strings.Contains(err.Error(), "HTTP 404") {
		t.Fatalf("error should mention HTTP 404, got: %v", err)
	}
}

func TestOllamaEmbedder_EmbedTexts_CountMismatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(ollamaEmbedResponse{
			Model:      "batiai/qwen3-embedding:0.6b",
			Embeddings: [][]float32{{0.1, 0.2}},
		})
	}))
	defer srv.Close()

	e := NewOllamaEmbedder(srv.URL, "batiai/qwen3-embedding:0.6b")
	_, err := e.EmbedTexts(context.Background(), []string{"a", "b", "c"})
	if err == nil {
		t.Fatal("expected error for count mismatch, got nil")
	}
	if !strings.Contains(err.Error(), "expected 3 embeddings") {
		t.Fatalf("error should mention expected 3 embeddings, got: %v", err)
	}
}

func TestOllamaEmbedder_EmbedTexts_Integration(t *testing.T) {
	if os.Getenv("OLLAMA_TEST") == "" {
		t.Skip("set OLLAMA_TEST=1 to run Ollama integration test")
	}

	e := NewOllamaEmbedder("", "batiai/qwen3-embedding:0.6b")
	vectors, err := e.EmbedTexts(context.Background(), []string{"hello world", "choir news"})
	if err != nil {
		t.Fatalf("EmbedTexts integration: %v", err)
	}
	if len(vectors) != 2 {
		t.Fatalf("vectors length = %d, want 2", len(vectors))
	}
	for i, vec := range vectors {
		if len(vec) != 1024 {
			t.Fatalf("vector %d dims = %d, want 1024", i, len(vec))
		}
	}
	fmt.Printf("integration: got %d vectors of dim %d\n", len(vectors), len(vectors[0]))
}
