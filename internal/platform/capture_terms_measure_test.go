package platform

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// TestMeasureTermSearchBroadWord is the synthetic broad match of the 07:55Z
// update: one word in the title of 4,000 of 6,000 captures, a caller's shape
// before (500 candidates and 2,000 label hints, about the 1,500 to 1,946 rows
// seen on the paper) and after (100 and 100). It prints one line per shape and
// asserts nothing about time. Same machine, same data; run it in both trees:
//
//	CHOIR_MEASURE=1 go test ./internal/platform -run MeasureTermSearchBroadWord -v
func TestMeasureTermSearchBroadWord(t *testing.T) {
	if os.Getenv("CHOIR_MEASURE") != "1" {
		t.Skip("a measurement, not a check: set CHOIR_MEASURE=1")
	}
	f := newFTFixture(t, false)
	base := time.Now().UTC().Truncate(time.Minute).Add(-7000 * time.Minute)
	var objs []objectgraph.Object
	for i := 0; i < 6000; i++ {
		title := fmt.Sprintf("Filler page %d", i)
		if i%3 != 0 {
			title = fmt.Sprintf("Harbor report %d", i)
		}
		objs = append(objs, f.capture(t, "owner-a", "computer-a", fmt.Sprintf("k%d", i), title, fmt.Sprintf("https://example.com/%d", i), "text", base.Add(time.Duration(i)*time.Minute)))
	}
	for start := 0; start < len(objs); start += 250 {
		f.put(t, objs[start:min(start+250, len(objs))]...)
	}
	if err := f.store.ReconcileCaptureIndex(context.Background()); err != nil {
		t.Fatal(err)
	}
	var hints []string
	for _, o := range objs[:2000] {
		hints = append(hints, o.CanonicalID)
	}
	for _, shape := range []struct {
		name              string
		candidates, hints int
	}{{"before 500 candidates 2000 hints", 500, 2000}, {"after 100 candidates 100 hints", 100, 100}, {"after again", 100, 100}, {"before again", 500, 2000}} {
		req := CaptureSearchRequest{OwnerID: "owner-a", ComputerID: "computer-a", Words: []string{"harbor"}, Need: 1, Candidates: shape.candidates, IDs: hints[:shape.hints]}
		var best time.Duration
		rows := 0
		for i := 0; i < 10; i++ {
			start := time.Now()
			resp, err := f.store.SearchCaptures(context.Background(), req)
			if err != nil {
				t.Fatal(err)
			}
			if d := time.Since(start); i == 0 || d < best {
				best = d
			}
			rows = len(resp.Objects)
		}
		t.Logf("MEASURE term search of a word in 4000 of 6000 captures, %s: %d rows, best of 10 %.1f ms", shape.name, rows, float64(best.Microseconds())/1000)
	}
}
