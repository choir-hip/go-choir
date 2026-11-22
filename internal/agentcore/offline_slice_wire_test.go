package agentcore

// Rung 0c: the event layer's wireOrigin (event_layer.go), the deployed
// function, run over the exported captures to say which are wire copies.
// Skipped unless OFFLINE_SLICE_DATA names the export directory; writes
// wire.tsv (capture id, wire origin) beside it. No database, no model.

import (
	"bufio"
	"os"
	"path/filepath"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/offlineslice"
)

func TestOfflineSliceWireOrigin(t *testing.T) {
	dir := os.Getenv("OFFLINE_SLICE_DATA")
	if dir == "" {
		t.Skip("OFFLINE_SLICE_DATA not set")
	}
	caps, err := offlineslice.ReadCaptures(filepath.Join(dir, "captures.csv"))
	if err != nil {
		t.Fatal(err)
	}
	leads, _, err := offlineslice.ReadLeads(filepath.Join(dir, "bodies.bin"))
	if err != nil {
		t.Fatal(err)
	}
	out := os.Getenv("OFFLINE_SLICE_OUT")
	if out == "" {
		out = dir
	}
	f, err := os.Create(filepath.Join(out, "wire.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	w := bufio.NewWriter(f)
	n := 0
	for _, c := range caps {
		// as the event layer calls it: the item's summary, then its title
		if o := wireOrigin(leads[c.TextHash], c.Title); o != "" {
			w.WriteString(c.ID + "\t" + o + "\n")
			n++
		}
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	t.Logf("%d of %d captures carry a wire tag", n, len(caps))
}
