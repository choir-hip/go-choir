package embedder

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
)

func full(n int, f func(i int) float32) []float32 {
	v := make([]float32, n)
	for i := range v {
		v[i] = f(i)
	}
	return v
}

func TestReduceCutsTo256AndNormalises(t *testing.T) {
	v, err := Reduce(full(768, func(i int) float32 { return float32(i%7) + 1 }))
	if err != nil {
		t.Fatal(err)
	}
	if len(v) != Dim {
		t.Fatalf("len %d", len(v))
	}
	var s float64
	for _, x := range v {
		s += float64(x) * float64(x)
	}
	if math.Abs(s-1) > 1e-5 {
		t.Fatalf("norm^2 %v", s)
	}
	if _, err := Reduce(make([]float32, 255)); err == nil {
		t.Fatal("short vector accepted")
	}
	if _, err := Reduce(make([]float32, 768)); err == nil {
		t.Fatal("zero vector accepted")
	}
}

func TestPromptStrings(t *testing.T) {
	if got := DocumentInput("Harbor", "text body"); got != "title: Harbor | text: text body" {
		t.Fatal(got)
	}
	if got := DocumentInput("  ", "x"); got != "title: none | text: x" {
		t.Fatal(got)
	}
	if got := QueryInput(" harbor reopened "); got != "task: search result | query: harbor reopened" {
		t.Fatal(got)
	}
}

func TestClientOneCallForManyInputsAndOrder(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(200)
			return
		}
		calls++
		var req embeddingsRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		type d struct {
			Index     int       `json:"index"`
			Embedding []float32 `json:"embedding"`
		}
		var data []d
		// Answer in reverse order: the client must place by index.
		for i := len(req.Input) - 1; i >= 0; i-- {
			e := make([]float32, 768)
			e[i] = 3
			data = append(data, d{Index: i, Embedding: e})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
	}))
	defer srv.Close()
	c := NewClient(srv.URL)
	if err := c.Health(context.Background()); err != nil {
		t.Fatal(err)
	}
	vs, err := c.Embed(context.Background(), []string{"a", "b", "c"})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || len(vs) != 3 {
		t.Fatalf("calls %d vectors %d", calls, len(vs))
	}
	for i, v := range vs {
		if v[i] != 1 {
			t.Fatalf("vector %d misplaced: %v", i, v[:4])
		}
	}
}

func TestClientHealthAndErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(503) }))
	defer srv.Close()
	c := NewClient(srv.URL)
	if c.Health(context.Background()) == nil {
		t.Fatal("503 health accepted")
	}
	_, err := c.Embed(context.Background(), []string{"a"})
	if se, ok := err.(*StatusError); !ok || se.Code != 503 {
		t.Fatalf("err %v", err)
	}
}

func TestFakeIsDeterministicAndShared(t *testing.T) {
	a := FakeVector("title: x | text: harbor reopened at dawn")
	b := FakeVector("title: y | text: harbor reopened at dawn")
	c := FakeVector("task: search result | query: quantum ledger")
	if Cosine(a, b) < 0.9999 {
		t.Fatal("same text differs")
	}
	if Cosine(a, c) > 0.5 {
		t.Fatal("different text is near")
	}
	if Cosine(a, FakeVector("title: x | text: harbor reopened")) < 0.7 {
		t.Fatal("shared words not near")
	}
}

func TestOnlyTheTwoTooLargeMessagesAreAnInputRefusal(t *testing.T) {
	for _, c := range []struct {
		code    int
		body    string
		refused bool
	}{
		{500, `{"error":{"message":"input (3000 tokens) is too large to process. increase the physical batch size (current batch size: 2048)"}}`, true},
		{400, `{"error":{"message":"input (5000 tokens) is larger than the max context size (2048 tokens). skipping"}}`, true},
		{500, `{"error":{"message":"internal error"}}`, false},
		{503, `{"error":{"message":"Loading model"}}`, false},
		{429, `too many requests`, false},
		{404, `not found`, false},
		{400, `{"error":{"message":"bad request"}}`, false},
	} {
		err := classifyStatus(c.code, c.body)
		var ir *InputRefusedError
		if got := errors.As(err, &ir); got != c.refused {
			t.Fatalf("%d %q: refusal %v, want %v (%v)", c.code, c.body, got, c.refused, err)
		}
		var se *StatusError
		if !c.refused && !errors.As(err, &se) {
			t.Fatalf("%d: not a StatusError: %v", c.code, err)
		}
	}
}
