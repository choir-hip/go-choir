package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/calibration"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// synthHeard builds n segment records and their heard settlements in memory.
func synthHeard(n int) (map[string]types.CommitmentRecord, map[string]time.Time) {
	segs := make(map[string]types.CommitmentRecord, n)
	heard := make(map[string]time.Time, n)
	now := time.Now()
	text := strings.Repeat("The council met and talked about the long budget. ", 40)
	for i := 0; i < n; i++ {
		stretch, seg := fmt.Sprintf("st-%d", i), fmt.Sprintf("st-%d.s0", i)
		cites, _ := json.Marshal([]programmeCite{{Ref: fmt.Sprintf("pub:r-%d@v1", i), Role: "subject"}})
		segs[segmentRecordID(stretch, seg)] = types.CommitmentRecord{
			RecordID:    segmentRecordID(stretch, seg),
			Directive:   &types.CommitmentDirective{Objective: fmt.Sprintf("Story %d", i), Body: text},
			SubjectRefs: map[string]string{"stretch_id": stretch, "segment_id": seg, "kind": "story", "forecast": "true", "cites": string(cites)},
		}
		heard[calibration.ClaimRecordID(calibration.AutoradioAgent, stretch+"/"+seg, "")] = now.Add(-time.Duration(i) * time.Minute)
	}
	return segs, heard
}

// BenchmarkBuildHeardSegs5000 is the join of 5,000 segments with 5,000 heard
// settlements: the CPU cost of the index once the two lists are read.
func BenchmarkBuildHeardSegs5000(b *testing.B) {
	segs, heard := synthHeard(5000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if got := buildHeardSegs(segs, heard); len(got) != 5000 {
			b.Fatal(len(got))
		}
	}
}

// TestHeardIndexTimeOver5000Segments times the whole heard read on a real
// store holding 5,000 heard segments: the two lists, then the join. It prints
// the numbers and asserts nothing about them; it is slow to seed, so it runs
// only with HEARD_TIMED=1.
func TestHeardIndexTimeOver5000Segments(t *testing.T) {
	if os.Getenv("HEARD_TIMED") != "1" {
		t.Skip("set HEARD_TIMED=1 to seed 5,000 heard segments and time the read")
	}
	rt, _ := radioRuntime(t, oneStory())
	now := time.Now()
	for i := 0; i < 5000; i++ {
		seedHeard(t, rt, heardSeed{stretch: fmt.Sprintf("st-%d", i), route: fmt.Sprintf("r-%d", i), version: 1, title: fmt.Sprintf("Story %d", i),
			text: strings.Repeat("The council met and talked about the long budget. ", 40), at: now.Add(-time.Duration(i) * time.Minute)})
	}
	c := &programmeCall{rt: rt, ledger: rt.store, ownerID: progOwner, req: programmeRequest{RequestID: "timed", Cause: "open"}}
	t0 := time.Now()
	segs, heard, err := c.readHeard(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t1 := time.Now()
	ix := buildHeardSegs(segs, heard)
	t2 := time.Now()
	t.Logf("heard read over 5000 segments: lists %v, join %v, index %d segments, full reads %v", t1.Sub(t0), t2.Sub(t1), len(ix), c.heardFull)
}
