package store

// F17, the journal probe that gates deploy 2
// (docs/design/choir-event-table-f-c-2026-10-22.md sections 4.1 and 5): Dolt
// journal bytes per batched event with store.event_table off against on, on
// the store's real tape harness (a real Store, the real appender, recording
// pin and CAS). The go bar is a ratio of at most 0.5. The number is
// directional only: it can stop deploy 2, but only staging and the paper can
// pass it.
//
// It is slow (thousands of events), so it runs only when asked:
//
//	CHOIR_EVENT_TABLE_JOURNAL_PROBE=1 \
//	CHOIR_EVENT_TABLE_JOURNAL_PROBE_OUT=/tmp/probe.json \
//	go test ./internal/store -run TestEventTableJournalProbe -count=1 -v -timeout 60m
//
// Ways the probe could mislead, written before it ran:
//   - the two tables differ in size, so one arm pays a deeper tree: both are
//     seeded with the same number of rows before any sample;
//   - a drift over time (the journal's own growth, a cache) is read as the
//     setting: the arms alternate, off, on, off, on, ..., and each arm's
//     samples are reported, not only their mean;
//   - one tape event per op hides the row cost under the per-event overhead
//     (prepare, finalize, heads): the batched arm (one sealed batch of k ops)
//     prices the op itself, b_og against b_ce, the design's gate quantity;
//   - the trajectory index's price is guessed: the 3-tree variant drops it on
//     the probe's own store and measures again (Q2);
//   - a toy body makes rows smaller than staging's (2.97 KB per op): each
//     event carries a payload of about 2 KB.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

type journalProbeSample struct {
	Arm           string  `json:"arm"`
	Events        int     `json:"events"`
	TapeEvents    int64   `json:"tape_events"`
	JournalBytes  uint64  `json:"journal_bytes"`
	BytesPerEvent float64 `json:"bytes_per_event"`
}

type journalProbeResult struct {
	Probe            string               `json:"probe"`
	Note             string               `json:"note"`
	SeedEventsPerArm int                  `json:"seed_events_per_table"`
	PayloadBytes     int                  `json:"payload_bytes"`
	BatchK           int                  `json:"batch_k"`
	Samples          []journalProbeSample `json:"samples"`
	// Means in KiB per event, by arm.
	MeanKiB map[string]float64 `json:"mean_kib_per_event"`
	// Ratios: on/off with one tape event per op (the per-event ratio), and
	// on/off in sealed batches of k ops (b_ce/b_og, the design's gate).
	RatioPerEvent      float64 `json:"ratio_per_event_unbatched"`
	RatioPerOp         float64 `json:"ratio_per_op_batched"`
	RatioPerOp3Tree    float64 `json:"ratio_per_op_batched_3_tree"`
	RatioPerEvent3Tree float64 `json:"ratio_per_event_unbatched_3_tree"`
	GoBar              float64 `json:"go_bar"`
}

func TestEventTableJournalProbe(t *testing.T) {
	if os.Getenv("CHOIR_EVENT_TABLE_JOURNAL_PROBE") != "1" {
		t.Skip("set CHOIR_EVENT_TABLE_JOURNAL_PROBE=1 to run the F17 journal probe")
	}
	ctx := context.Background()
	const (
		seedPerTable = 3000
		perSample    = 150
		rounds       = 3
		batchK       = 32
	)
	payload := strings.Repeat("the quick brown fox jumps over the lazy dog; ", 45) // about 2 KB
	h := newTapeHarness(t)
	n := 0
	appendOne := func(kind types.EventKind) {
		n++
		run := fmt.Sprintf("probe-run-%02d", n%10)
		rec := h.event(fmt.Sprintf("probe-%07d", n), run, []string{"owner-1", "owner-2"}[n%2], kind, telemetryBase.Add(time.Duration(n)*time.Millisecond))
		raw, _ := json.Marshal(map[string]string{"text": payload})
		rec.Payload = raw
		if err := h.store.AppendEvent(ctx, rec); err != nil {
			t.Fatal(err)
		}
	}
	// Seed both tables to the same size.
	h.store.SetEventTableWrites(false)
	for i := 0; i < seedPerTable; i++ {
		appendOne(types.EventRunDelta)
	}
	h.store.SetEventTableWrites(true)
	for i := 0; i < seedPerTable; i++ {
		appendOne(types.EventRunDelta)
	}
	if tableCount(t, h.store, "choir_events", "") != seedPerTable {
		t.Fatal("fixture: choir_events was not seeded")
	}

	res := journalProbeResult{
		Probe: "F17", GoBar: 0.5, SeedEventsPerArm: seedPerTable, PayloadBytes: len(payload), BatchK: batchK,
		Note: "local tape harness; directional only (design 4.1): a miss stops deploy 2, only staging and the paper pass it",
	}
	sample := func(arm string, on, batched bool) {
		t.Helper()
		h.store.SetEventTableWrites(on)
		var sl *telemetrySealer
		if batched {
			sl = h.enableBatching(time.Hour)
		}
		tapeBefore := int64(len(h.tape.snapshot()))
		before := doltJournalBytes(h.store.texturePath)
		for i := 0; i < perSample; i++ {
			appendOne(types.EventRunDelta)
			if batched && (i+1)%batchK == 0 {
				sl.drain()
			}
		}
		if batched {
			sl.drain()
			if err := h.store.ConfigureTelemetryBatching(TelemetryBatchConfig{Enabled: false}); err != nil {
				t.Fatal(err)
			}
		}
		after := doltJournalBytes(h.store.texturePath)
		s := journalProbeSample{
			Arm: arm, Events: perSample, TapeEvents: int64(len(h.tape.snapshot())) - tapeBefore,
			JournalBytes: after - before, BytesPerEvent: float64(after-before) / perSample,
		}
		res.Samples = append(res.Samples, s)
		t.Logf("%-22s %4d events %4d tape events %8.1f KiB/event", arm, s.Events, s.TapeEvents, s.BytesPerEvent/1024)
	}
	for r := 0; r < rounds; r++ {
		sample("unbatched_off", false, false)
		sample("unbatched_on", true, false)
		sample("batched_off", false, true)
		sample("batched_on", true, true)
	}
	// Q2's price: the same store without the trajectory index (3 trees).
	if _, err := h.store.db.Exec(`DROP INDEX idx_choir_events_trajectory_stream ON choir_events`); err != nil {
		t.Fatal(err)
	}
	for r := 0; r < rounds; r++ {
		sample("unbatched_off_3tree", false, false)
		sample("unbatched_on_3tree", true, false)
		sample("batched_off_3tree", false, true)
		sample("batched_on_3tree", true, true)
	}

	res.MeanKiB = map[string]float64{}
	counts := map[string]int{}
	for _, s := range res.Samples {
		res.MeanKiB[s.Arm] += s.BytesPerEvent / 1024
		counts[s.Arm]++
	}
	for arm, c := range counts {
		res.MeanKiB[arm] /= float64(c)
	}
	res.RatioPerEvent = res.MeanKiB["unbatched_on"] / res.MeanKiB["unbatched_off"]
	res.RatioPerOp = res.MeanKiB["batched_on"] / res.MeanKiB["batched_off"]
	res.RatioPerEvent3Tree = res.MeanKiB["unbatched_on_3tree"] / res.MeanKiB["unbatched_off_3tree"]
	res.RatioPerOp3Tree = res.MeanKiB["batched_on_3tree"] / res.MeanKiB["batched_off_3tree"]
	raw, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("F17 result:\n%s", raw)
	if out := os.Getenv("CHOIR_EVENT_TABLE_JOURNAL_PROBE_OUT"); out != "" {
		if err := os.WriteFile(out, append(raw, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if res.RatioPerOp > res.GoBar {
		t.Errorf("F17 go bar missed: b_ce/b_og = %.3f > %.1f; deploy 2 stops until the gap is explained", res.RatioPerOp, res.GoBar)
	}
}
