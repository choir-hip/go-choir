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
