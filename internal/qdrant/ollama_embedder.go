package qdrant

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	defaultOllamaBaseURL = "http://localhost:11434"
	defaultOllamaTimeout = 60 * time.Second
)

// OllamaEmbedder is an Embedder backed by the Ollama HTTP /api/embed endpoint.
type OllamaEmbedder struct {
	baseURL string
	model   string
	client  *http.Client
}

// NewOllamaEmbedder returns an OllamaEmbedder for the given base URL and model
// name. If baseURL is empty, http://localhost:11434 is used.
func NewOllamaEmbedder(baseURL string, model string) *OllamaEmbedder {
	if baseURL == "" {
		baseURL = defaultOllamaBaseURL
	}
	baseURL = strings.TrimRight(baseURL, "/")
	return &OllamaEmbedder{
		baseURL: baseURL,
		model:   model,
		client:  &http.Client{Timeout: defaultOllamaTimeout},
	}
}

// Model returns the EmbeddingModel metadata for this embedder.
func (e *OllamaEmbedder) Model() EmbeddingModel {
	return EmbeddingModel{
		Name:       e.model,
		Version:    "1",
		Dimensions: 1024,
	}
}

type ollamaEmbedRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type ollamaEmbedResponse struct {
	Model      string      `json:"model"`
	Embeddings [][]float32 `json:"embeddings"`
}

// EmbedTexts calls the Ollama /api/embed endpoint and returns one vector per
// input text. The context is respected for cancellation and deadlines.
func (e *OllamaEmbedder) EmbedTexts(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, fmt.Errorf("ollama embedder: no texts provided")
	}

	body, err := json.Marshal(ollamaEmbedRequest{Model: e.model, Input: texts})
	if err != nil {
		return nil, fmt.Errorf("ollama embedder: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL+"/api/embed", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("ollama embedder: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama embedder: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama embedder: HTTP %d: %s", resp.StatusCode, string(raw))
	}

	var out ollamaEmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("ollama embedder: decode response: %w", err)
	}

	if len(out.Embeddings) != len(texts) {
		return nil, fmt.Errorf("ollama embedder: expected %d embeddings, got %d", len(texts), len(out.Embeddings))
	}

	for i, vec := range out.Embeddings {
		if len(vec) == 0 {
			return nil, fmt.Errorf("ollama embedder: empty embedding at index %d", i)
		}
	}

	return out.Embeddings, nil
}
