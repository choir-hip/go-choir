package store

import (
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

func objOp(t testing.TB, kind, suffix string, body int) computerevent.ProjectionOp {
	t.Helper()
	b := make([]byte, body)
	for i := range b {
		b[i] = 'x'
	}
	return computerevent.ProjectionOp{
		Kind:        computerevent.ProjectionOpObject,
		CanonicalID: mustCanonicalID(t, kind, "owner", suffix),
		Body:        b,
	}
}

func tallyOps(ops ...computerevent.ProjectionOp) tapeEventTally {
	return tallyTapeEvent(projectionAuthority, ops, 100)
}

func TestRewriteCountsAndBytesPerKind(t *testing.T) {
	var m tapeWriteMaps
	m.add(tallyOps(objOp(t, "choir.run", "a", 100), objOp(t, "choir.run", "a", 300), objOp(t, "choir.run", "b", 200)))
	m.add(tallyOps(objOp(t, "choir.commitment_record", "c", 50)))
	var out TelemetryStats
	m.copyInto(&out)
	run := out.ObjectRewrites["choir.run"]
	if run.Ops != 3 || run.Bytes != 600 || run.MeanBytesPerOp != 200 {
		t.Errorf("choir.run = %+v", run)
	}
	if run.DistinctObjects != 2 || run.WritesPerObject != 1.5 || run.Estimated {
		t.Errorf("choir.run distinct = %+v", run)
	}
	if c := out.ObjectRewrites["choir.commitment_record"]; c.Ops != 1 || c.DistinctObjects != 1 || c.WritesPerObject != 1 {
		t.Errorf("commitment = %+v", c)
	}
}

func TestRewriteDistinctEstimateIsExactAtTenAndCloseAtTenThousand(t *testing.T) {
	var m tapeWriteMaps
	for i := 0; i < 10; i++ {
		for rep := 0; rep < 3; rep++ {
			m.add(tallyOps(objOp(t, "choir.run", fmt.Sprintf("o%d", i), 10)))
		}
	}
	var out TelemetryStats
	m.copyInto(&out)
	if r := out.ObjectRewrites["choir.run"]; r.DistinctObjects != 10 || r.Estimated || r.Ops != 30 || r.WritesPerObject != 3 {
		t.Errorf("at 10: %+v", r)
	}
	var big tapeWriteMaps
	for i := 0; i < 10000; i++ {
		for rep := 0; rep < 2; rep++ {
			big.add(tallyOps(objOp(t, "choir.run", fmt.Sprintf("o%d", i), 1)))
		}
	}
	var out2 TelemetryStats
	big.copyInto(&out2)
	r := out2.ObjectRewrites["choir.run"]
	if !r.Estimated {
		t.Error("10,000 objects must say estimated")
	}
	if math.Abs(float64(r.DistinctObjects)-10000)/10000 > 0.10 {
		t.Errorf("distinct estimate %d not within 10%% of 10000", r.DistinctObjects)
	}
	if r.Ops != 20000 {
		t.Errorf("ops = %d", r.Ops)
	}
}

func TestRewriteKindsAreCapped(t *testing.T) {
	var m tapeWriteMaps
	for i := 0; i < 300; i++ {
		m.add(tapeEventTally{objKinds: []string{fmt.Sprintf("k%d", i)}, objBytes: []int64{5}, objHashes: []uint64{hashID(fmt.Sprint(i))}})
	}
	var out TelemetryStats
	m.copyInto(&out)
	if len(out.ObjectRewrites) > maxCounterKeys {
		t.Errorf("%d kinds tracked, cap %d", len(out.ObjectRewrites), maxCounterKeys)
	}
	if out.ObjectRewrites["other"].Ops == 0 {
		t.Error("no other bucket")
	}
	var total int64
	for _, r := range out.ObjectRewrites {
		total += r.Ops
	}
	if total != 300 {
		t.Errorf("ops lost to the cap: %d", total)
	}
}

func TestRewriteCountersRace(t *testing.T) {
	var c telemetryCounters
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 300; i++ {
				c.recordTapeEvent(projectionAuthority, [][]computerevent.ProjectionOp{{objOp(t, "choir.run", fmt.Sprintf("g%d-%d", g, i%50), 10)}}, false, 10)
			}
		}(g)
	}
	reader := make(chan struct{})
	go func() {
		defer close(reader)
		for i := 0; i < 100; i++ {
			c.mu.Lock()
			var out TelemetryStats
			c.tapeWrites.copyInto(&out)
			c.mu.Unlock()
			_, _ = json.Marshal(out.ObjectRewrites)
		}
	}()
	wg.Wait()
	<-reader
	var out TelemetryStats
	c.mu.Lock()
	c.tapeWrites.copyInto(&out)
	c.mu.Unlock()
	r := out.ObjectRewrites["choir.run"]
	if r.Ops != 2400 || math.Abs(float64(r.DistinctObjects)-400)/400 > 0.10 {
		t.Errorf("race result %+v", r)
	}
}
