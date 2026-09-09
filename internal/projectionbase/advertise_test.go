package projectionbase

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAdvertiseWatermarkPostsSoleWriter(t *testing.T) {
	var gotComputer string
	var gotSeq uint64
	var gotRef string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/computers/files/watermark" || r.Method != http.MethodPost {
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-cap" {
			t.Fatalf("authorization %q", got)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		var payload struct {
			ComputerID        string `json:"computer_id"`
			WatermarkSequence uint64 `json:"watermark_sequence"`
			BaseRef           string `json:"base_ref"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatal(err)
		}
		gotComputer, gotSeq, gotRef = payload.ComputerID, payload.WatermarkSequence, payload.BaseRef
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"watermark_sequence": payload.WatermarkSequence, "base_ref": payload.BaseRef})
	}))
	defer server.Close()

	if err := AdvertiseWatermark(context.Background(), server.URL, "test-cap", "computer-owner", 148000, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"); err != nil {
		t.Fatal(err)
	}
	if gotComputer != "computer-owner" || gotSeq != 148000 || gotRef != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("advertised %#v %d %s", gotComputer, gotSeq, gotRef)
	}
}
