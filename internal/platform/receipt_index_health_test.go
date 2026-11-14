package platform

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// DeepSeek B2 of round 3: the only source of samples is the append itself, so
// the test goes through ComputerEventCAS.CompareAndSwap. A successful append
// records one sample; an idempotent replay and a refused append record none.
func TestAppendTimingRecordsOneSampleForASuccessfulAppendOnly(t *testing.T) {
	ctx := context.Background()
	d := newDrillComputer(t, "computer-59c902c5d46caecc0a180a8557164e72")
	samples := func() []appendSample { return d.store.appendTimes.since(time.Time{}) }
	if n := len(samples()); n != 0 {
		t.Fatalf("samples before any append = %d", n)
	}
	first := d.buildRequest(computerevent.EventGenesisImported, "", nil, "first")
	rival := d.buildRequest(computerevent.EventGenesisImported, "", nil, "rival") // built on the same head
	if _, err := d.cas.CompareAndSwap(ctx, first); err != nil {
		t.Fatal(err)
	}
	if got := samples(); len(got) != 1 || got[0].computer != d.id || got[0].took <= 0 {
		t.Fatalf("after one append: %+v", got)
	}
	if _, err := d.cas.CompareAndSwap(ctx, first); err != nil { // an idempotent resend
		t.Fatalf("resend: %v", err)
	}
	if n := len(samples()); n != 1 {
		t.Fatalf("an idempotent replay recorded a sample: %d", n)
	}
	if _, err := d.cas.CompareAndSwap(ctx, rival); err == nil { // the sequence is taken
		t.Fatal("the rival append was accepted")
	}
	if n := len(samples()); n != 1 {
		t.Fatalf("a refused append recorded a sample: %d", n)
	}
	d.appendEvent(computerevent.EventMessageRecorded, "text/plain", []byte("second"))
	if n := len(samples()); n != 2 {
		t.Fatalf("after the second append: %d", n)
	}
}

// The line is the busiest computer's count, p50 and p95 over a minute and over
// ten minutes.
func TestAppendTimingLineReadsTheBusiestComputer(t *testing.T) {
	s, _ := openTestPlatformStore(t)
	now := time.Now()
	if line := s.appendTimingLine(now); !strings.Contains(line, "1m n=0") || !strings.Contains(line, "10m n=0") {
		t.Fatalf("empty line = %q", line)
	}
	for i := 1; i <= 100; i++ {
		s.appendTimes.record("computer-busy", time.Duration(i)*time.Millisecond)
	}
	for i := 0; i < 5; i++ {
		s.appendTimes.record("computer-quiet", time.Hour)
	}
	// Ten minutes ago, outside the one-minute window.
	s.appendTimes.mu.Lock()
	for i := 0; i < 50; i++ {
		s.appendTimes.samples[s.appendTimes.next] = appendSample{at: now.Add(-5 * time.Minute), computer: "computer-old", took: time.Second}
		s.appendTimes.next++
		s.appendTimes.filled++
	}
	s.appendTimes.mu.Unlock()
	line := s.appendTimingLine(now.Add(time.Second))
	for _, want := range []string{"1m busiest=computer-busy n=100 p50=50ms p95=95ms", "10m busiest=computer-busy n=100 p50=50ms p95=95ms"} {
		if !strings.Contains(line, want) {
			t.Fatalf("line = %q, want it to contain %q", line, want)
		}
	}
	if line := s.appendTimingLine(now.Add(6 * time.Minute)); !strings.Contains(line, "1m n=0") || !strings.Contains(line, "10m busiest=computer-busy") {
		t.Fatalf("later line = %q", line)
	}
}
