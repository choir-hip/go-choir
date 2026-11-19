// Package embedder is the client of the host's EmbeddingGemma 2 server
// (nix/embedder.nix, docs/reference/EMBEDDINGGEMMA.md "The server") and the
// arithmetic around its vectors: the prompt strings, the cut to 256
// Matryoshka dimensions and the cosine.
//
// There is no rate bucket, in-flight cap or call counter here: the server
// refuses nothing and counts nothing (CLAUDE.md, no limits on our own model
// calls). A caller puts as many texts in one call as it has.
package embedder

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"
)

const (
	// ModelHash is the SHA-256 of the GGUF the server loads
	// (embeddinggemma-2-Q8_0.gguf). Every stored vector carries it, and a
	// vector file made elsewhere is refused unless it names it.
	ModelHash = "2188ac1deca4b77dffefd603c2776a9d76d9d74ec01841392982ebb840b09135"
	// Dim is the stored dimension: the model's first 256 Matryoshka values,
	// normalised again (quality 60.4 against 61.4 at 768).
	Dim = 256
	// DefaultURL is the host unit's address (nix/embedder.nix).
	DefaultURL = "http://127.0.0.1:8090"
)

// DocumentInput is the text the model reads for a document: the card's titled
// form. An empty title is "none", as the card says.
func DocumentInput(title, text string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		title = "none"
	}
	return "title: " + title + " | text: " + text
}

// QueryInput is the text the model reads for a search query.
func QueryInput(query string) string {
	return "task: search result | query: " + strings.TrimSpace(query)
}

// TextSHA names a passage's text as UTF-8 (not the prompt around it). A stored
// vector records it, so a vector is never taken for another text's, and a
// vector file made elsewhere is checked against the passage this host cuts.
func TextSHA(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

// Embedder makes vectors. Embed takes ready inputs (DocumentInput or
// QueryInput) and returns one Dim-length unit vector for each, in order.
type Embedder interface {
	Embed(ctx context.Context, inputs []string) ([][]float32, error)
	// Health is nil when the server can answer now.
	Health(ctx context.Context) error
}

// Reduce cuts a full vector to its first Dim values and normalises them. It
// refuses a vector shorter than Dim or with no length.
func Reduce(v []float32) ([]float32, error) {
	if len(v) < Dim {
		return nil, fmt.Errorf("embedder: vector of %d values, want at least %d", len(v), Dim)
	}
	out := make([]float32, Dim)
	var sum float64
	for i := 0; i < Dim; i++ {
		x := float64(v[i])
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return nil, errors.New("embedder: vector holds a non-finite value")
		}
		sum += x * x
	}
	if sum == 0 {
		return nil, errors.New("embedder: zero vector")
	}
	n := float32(math.Sqrt(sum))
	for i := range out {
		out[i] = v[i] / n
	}
	return out, nil
}

// Cosine is the cosine of two vectors of equal length (0 for different
// lengths or a zero vector).
func Cosine(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / math.Sqrt(na*nb)
}

// Client talks to the llama.cpp server.
type Client struct {
	// BaseURL is the server's address, without a trailing slash.
	BaseURL string
	HTTP    *http.Client
}

// NewClient returns a client for the server at baseURL (DefaultURL when empty).
// A call ends when the caller's context does, and in any case after ten
// minutes (a bound on one HTTP exchange, far above a batch of four passages;
// not a limit on how many calls are made).
func NewClient(baseURL string) *Client {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = DefaultURL
	}
	return &Client{BaseURL: strings.TrimRight(baseURL, "/"), HTTP: &http.Client{Timeout: 10 * time.Minute}}
}

// Health reads GET /health: 200 when the model is loaded, 503 while loading.
func (c *Client) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/health", nil)
	if err != nil {
		return err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("embedder health: status %d", resp.StatusCode)
	}
	return nil
}

type embeddingsRequest struct {
	Input []string `json:"input"`
	Model string   `json:"model,omitempty"`
}

type embeddingsResponse struct {
	Data []struct {
		Index     int       `json:"index"`
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

// Embed sends every input in one call to POST /v1/embeddings and reduces
// each returned vector to Dim values.
func (c *Client) Embed(ctx context.Context, inputs []string) ([][]float32, error) {
	if len(inputs) == 0 {
		return nil, nil
	}
	body, err := json.Marshal(embeddingsRequest{Input: inputs, Model: "embeddinggemma-2"})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/v1/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embedder: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 256<<20))
	if err != nil {
		return nil, fmt.Errorf("embedder: read: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, &StatusError{Code: resp.StatusCode, Body: strings.TrimSpace(string(raw[:min(len(raw), 300)]))}
	}
	var out embeddingsResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("embedder: decode: %w", err)
	}
	if len(out.Data) != len(inputs) {
		return nil, fmt.Errorf("embedder: %d vectors for %d inputs", len(out.Data), len(inputs))
	}
	vecs := make([][]float32, len(inputs))
	for _, d := range out.Data {
		if d.Index < 0 || d.Index >= len(inputs) || vecs[d.Index] != nil {
			return nil, fmt.Errorf("embedder: bad index %d", d.Index)
		}
		v, err := Reduce(d.Embedding)
		if err != nil {
			return nil, err
		}
		vecs[d.Index] = v
	}
	return vecs, nil
}

// StatusError is a non-200 answer. A 4xx names an input the server will not
// take; a 5xx or 503 is the server's state.
type StatusError struct {
	Code int
	Body string
}

func (e *StatusError) Error() string { return fmt.Sprintf("embedder: status %d: %s", e.Code, e.Body) }
