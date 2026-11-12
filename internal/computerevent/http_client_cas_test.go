package computerevent

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The append path decides whether to discard its prepared row from this
// classification: only a refusal the host made before committing may delete
// it. Everything else is an unknown outcome.
func TestHTTPClientCASClassifiesRefusalsAndUnknownOutcomes(t *testing.T) {
	cases := []struct {
		name     string
		status   int
		hangup   bool
		badJSON  bool
		definite bool
	}{
		{name: "400 refusal", status: http.StatusBadRequest, definite: true},
		{name: "403 refusal", status: http.StatusForbidden, definite: true},
		{name: "409 conflict", status: http.StatusConflict, definite: true},
		{name: "422 refusal", status: http.StatusUnprocessableEntity, definite: true},
		{name: "500 server error", status: http.StatusInternalServerError},
		{name: "503 retryable", status: http.StatusServiceUnavailable},
		{name: "502 gateway", status: http.StatusBadGateway},
		{name: "connection reset", hangup: true},
		{name: "201 with an undecodable body", status: http.StatusCreated, badJSON: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tc.hangup {
					conn, _, err := w.(http.Hijacker).Hijack()
					if err != nil {
						t.Error(err)
						return
					}
					conn.Close()
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				if tc.badJSON {
					_, _ = w.Write([]byte("{not json"))
					return
				}
				_, _ = w.Write([]byte(`{"error":"x"}`))
			}))
			defer server.Close()
			client, err := NewHTTPClient(server.URL, server.Client(), func(context.Context) (string, error) { return "t", nil }, true)
			if err != nil {
				t.Fatal(err)
			}
			_, err = client.CompareAndSwap(context.Background(), CASRequest{})
			if err == nil {
				t.Fatal("a failed CAS reported success")
			}
			if got := CASRefusedDefinitely(err); got != tc.definite {
				t.Fatalf("CASRefusedDefinitely(%v) = %v, want %v", err, got, tc.definite)
			}
		})
	}
	t.Run("409 stays ErrCASConflict", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, `{"error":"head CAS conflict"}`, http.StatusConflict)
		}))
		defer server.Close()
		client, _ := NewHTTPClient(server.URL, server.Client(), func(context.Context) (string, error) { return "t", nil }, true)
		if _, err := client.CompareAndSwap(context.Background(), CASRequest{}); !CASRefusedDefinitely(err) || !errors.Is(err, ErrCASConflict) {
			t.Fatalf("409 error = %v", err)
		}
	})
	t.Run("a cancelled caller is unknown", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		defer server.Close()
		client, _ := NewHTTPClient(server.URL, server.Client(), func(context.Context) (string, error) { return "t", nil }, true)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := client.CompareAndSwap(ctx, CASRequest{})
		if err == nil || CASRefusedDefinitely(err) {
			t.Fatalf("cancelled CAS = %v, want an unknown outcome", err)
		}
	})
}
