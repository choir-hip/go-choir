package taperepack

import (
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// TestSynthTreeGenerate writes a synthetic legacy tree for the memory and
// throughput measurement (the run is a separate process, so its resident set
// is its own). It is skipped unless TAPEREPACK_SYNTH_DIR is set:
//
//	TAPEREPACK_SYNTH_DIR=/tmp/synth TAPEREPACK_SYNTH_PINS=70000 go test ./internal/taperepack -run SynthTreeGenerate
//
// Each pin is an event, a payload and their two receipts (four files), spread
// over three computers, about 1.2 KB a file.
func TestSynthTreeGenerate(t *testing.T) {
	dir := os.Getenv("TAPEREPACK_SYNTH_DIR")
	if dir == "" {
		t.Skip("TAPEREPACK_SYNTH_DIR not set")
	}
	pins, _ := strconv.Atoi(os.Getenv("TAPEREPACK_SYNTH_PINS"))
	if pins <= 0 {
		pins = 70000
	}
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	key := computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: "platform", KeyID: "synth"}, PrivateKey: priv}
	for _, d := range []string{"computer-event", "computer-event-payload", "pin-receipts"} {
		if err := os.MkdirAll(filepath.Join(dir, "sha256", d), 0o750); err != nil {
			t.Fatal(err)
		}
	}
	pad := strings.Repeat("x", 1000)
	put := func(sub string, body []byte) string {
		d := hexSum(body)
		name := d
		if sub == "pin-receipts" {
			name += ".json"
		}
		if err := os.WriteFile(filepath.Join(dir, "sha256", sub, name), body, 0o640); err != nil {
			t.Fatal(err)
		}
		return d
	}
	receipt := func(computer, ns, media, digest string, length int) {
		r, err := computerevent.NewSignedReceipt("PinReceipt", "corpusd", map[string]any{
			"computer_id": computer, "artifact_digest": digest, "media_type": media, "length": length,
			"privacy_class": "owner", "pin_namespace": ns, "request_commitment": strings.Repeat("a", 64),
		}, []computerevent.SigningKey{key}, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := r.CanonicalBytes()
		put("pin-receipts", raw)
	}
	for i := 0; i < pins; i++ {
		c := fmt.Sprintf("synth-%d", i%3)
		ev := []byte(`{"computer_id":"` + c + `","event_kind":"projection_batch_recorded","n":` + strconv.Itoa(i) + `,"pad":"` + pad + `"}`)
		pl := []byte(`{"batch":` + strconv.Itoa(i) + `,"computer_id":"` + c + `","pad":"` + pad + `"}`)
		receipt(c, "computer-event", "application/vnd.choir.computer-event+json", put("computer-event", ev), len(ev))
		receipt(c, "computer-event-payload", computerevent.ProjectionBatchMediaType, put("computer-event-payload", pl), len(pl))
	}
}
