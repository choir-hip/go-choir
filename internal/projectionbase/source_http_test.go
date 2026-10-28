package projectionbase

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestHTTPSourceTransportContract pins the exact platform query surface the
// installer depends on: endpoint paths, parameter names, and refusal mapping.
// The boot installer once fetched blobs with a parameter the platform never
// read; this test fails that class of drift on either side.
func TestHTTPSourceTransportContract(t *testing.T) {
	ctx := context.Background()
	blob := []byte("base blob bytes")
	seen := map[string]string{}
	mux := http.NewServeMux()
	mux.HandleFunc("/internal/computers/files/watermark", func(w http.ResponseWriter, r *http.Request) {
		seen["watermark_params"] = r.URL.RawQuery
		_ = json.NewEncoder(w).Encode(map[string]any{"watermark_sequence": 7, "base_ref": strings.Repeat("b", 64)})
	})
	mux.HandleFunc("/internal/computers/files/projection-base/descriptor", func(w http.ResponseWriter, r *http.Request) {
		seen["descriptor_params"] = r.URL.RawQuery
		if r.URL.Query().Get("base_ref") == strings.Repeat("0", 64) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		d := validTestDescriptor()
		d.Sequence = 7
		d.BlobSHA256 = strings.Repeat("b", 64)
		raw, _ := MarshalDescriptor(d)
		_, _ = w.Write(raw)
	})
	mux.HandleFunc("/internal/computers/files/projection-base/blob", func(w http.ResponseWriter, r *http.Request) {
		seen["blob_params"] = r.URL.RawQuery
		_, _ = w.Write(blob)
	})
	mux.HandleFunc("/internal/computers/events/replay", func(w http.ResponseWriter, r *http.Request) {
		seen["replay_params"] = r.URL.RawQuery
		_, _ = w.Write([]byte("[]"))
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	src := NewHTTPSource(server.URL, func(ctx context.Context) (string, error) { return "cap", nil })

	seq, ref, err := src.Watermark(ctx, "computer-base-test")
	if err != nil || seq != 7 || ref != strings.Repeat("b", 64) {
		t.Fatalf("watermark = %d %q %v", seq, ref, err)
	}
	if q := seen["watermark_params"]; !strings.Contains(q, "computer_id=computer-base-test") {
		t.Fatalf("watermark params = %q", q)
	}
	d, err := src.Descriptor(ctx, "computer-base-test", strings.Repeat("b", 64))
	if err != nil || d.Sequence != 7 {
		t.Fatalf("descriptor = %+v %v", d, err)
	}
	var streamed strings.Builder
	if err := src.DownloadBlob(ctx, "computer-base-test", strings.Repeat("b", 64), &streamed); err != nil {
		t.Fatalf("download = %v", err)
	}
	if streamed.String() != string(blob) {
		t.Fatalf("streamed %q", streamed.String())
	}
	if q := seen["blob_params"]; !strings.Contains(q, "base_ref=") || !strings.Contains(q, "computer_id=") {
		t.Fatalf("blob params = %q", q)
	}
	if _, err := src.Descriptor(ctx, "computer-base-test", strings.Repeat("0", 64)); !errors.Is(err, ErrBaseRefused) {
		t.Fatalf("missing descriptor did not refuse: %v", err)
	}
}

// The guest reads the host's checkpoint verdict over the platform's verdict
// route: path, parameter names, a 404 as "no verdict row", other statuses as
// errors (so a platform outage is never read as "no verdict").
func TestHTTPSourceCheckpointVerdictContract(t *testing.T) {
	ctx := context.Background()
	var query string
	status := http.StatusOK
	mux := http.NewServeMux()
	mux.HandleFunc("/internal/computers/checkpoints/verdict", func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		if status != http.StatusOK {
			w.WriteHeader(status)
			return
		}
		_, _ = w.Write([]byte(`{"status":"not_derivable","reason":"witness_mismatch","differing_tables":["og_objects"]}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	src := NewHTTPSource(server.URL, func(ctx context.Context) (string, error) { return "cap", nil })

	digest := strings.Repeat("d", 64)
	verdict, found, err := src.CheckpointVerdict(ctx, "computer-verdict", digest)
	if err != nil || !found || verdict.Status != "not_derivable" || verdict.Reason != "witness_mismatch" || len(verdict.DifferingTables) != 1 {
		t.Fatalf("verdict = %+v %v %v", verdict, found, err)
	}
	if !strings.Contains(query, "computer_id=computer-verdict") || !strings.Contains(query, "checkpoint_digest="+digest) {
		t.Fatalf("verdict params = %q", query)
	}
	status = http.StatusNotFound
	if _, found, err := src.CheckpointVerdict(ctx, "computer-verdict", digest); err != nil || found {
		t.Fatalf("404 = found %v err %v, want no verdict and no error", found, err)
	}
	status = http.StatusBadGateway
	if _, found, err := src.CheckpointVerdict(ctx, "computer-verdict", digest); err == nil || found {
		t.Fatalf("502 = found %v err %v, want an error", found, err)
	}
}

// S4: the guest's installer fetches parts through HTTPSource. The query
// surface is pinned like the blob's; a 404/409 is a refusal, a 5xx stays a
// plain transport error.
func TestHTTPSourceDownloadPartContract(t *testing.T) {
	ctx := context.Background()
	var query string
	status := http.StatusOK
	mux := http.NewServeMux()
	mux.HandleFunc("/internal/computers/files/projection-base/part", func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		w.WriteHeader(status)
		_, _ = w.Write([]byte("part"))
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	var src BaseSource = NewHTTPSource(server.URL, func(ctx context.Context) (string, error) { return "cap", nil })
	parts, ok := src.(PartSource)
	if !ok {
		t.Fatal("HTTPSource cannot serve parts")
	}
	var buf strings.Builder
	base, part := strings.Repeat("b", 64), strings.Repeat("d", 64)
	if err := parts.DownloadPart(ctx, "computer-x", base, part, &buf); err != nil || buf.String() != "part" {
		t.Fatalf("DownloadPart = %q %v", buf.String(), err)
	}
	for _, want := range []string{"computer_id=computer-x", "base_ref=" + base, "part=" + part} {
		if !strings.Contains(query, want) {
			t.Fatalf("part query %q lacks %q", query, want)
		}
	}
	for _, refused := range []int{http.StatusNotFound, http.StatusConflict} {
		status = refused
		if err := parts.DownloadPart(ctx, "computer-x", base, part, &strings.Builder{}); !errors.Is(err, ErrBaseRefused) {
			t.Fatalf("status %d: err = %v, want ErrBaseRefused", refused, err)
		}
	}
	status = http.StatusBadGateway
	if err := parts.DownloadPart(ctx, "computer-x", base, part, &strings.Builder{}); err == nil || errors.Is(err, ErrBaseRefused) {
		t.Fatalf("502: err = %v, want a plain error", err)
	}
}
