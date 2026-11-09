package yaegikernel

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
	"testing"
)

// A frame over the transport bound must fail soft: the worker gets a cut,
// clearly marked frame and starts, instead of "frame: payload N bytes exceeds
// 4194304" killing every cell of the desk
// (docs/evidence/paper-token-budget-2026-11-09.md section 1.3).
//
// Ways this could fail, written first: the oversized write still happens and
// poisons the worker; the cut drops the oldest records instead of the newest,
// so the cursor skips unseen ones; the cut leaves a seq group half kept; a
// record bigger than the frame wedges the window; the doc alone being too big
// is reported as backlog instead of an error.

func bigRecords(n, size int) []InboxRecord {
	body := strings.Repeat("x", size)
	out := make([]InboxRecord, n)
	for i := range out {
		out[i] = InboxRecord{Seq: int64(i + 1), RecordID: fmt.Sprintf("r%03d", i), Kind: "notice", Body: body}
	}
	return out
}

func TestWindowRecordsKeepsOldestWholeSeqsAndStubsOversize(t *testing.T) {
	recs := bigRecords(50, 100<<10)
	// Seq 2 carries two records (a Cast and a delegated captures record).
	recs = append(recs[:2], append([]InboxRecord{{Seq: 2, RecordID: "twin", Kind: "captures", Body: strings.Repeat("y", 100<<10)}}, recs[2:]...)...)
	kept, cut := WindowRecords(recs, 450<<10)
	if !cut || len(kept) == 0 || kept[0].RecordID != "r000" {
		t.Fatalf("window must be cut and start at the oldest: cut=%v kept=%d", cut, len(kept))
	}
	for i := 1; i < len(kept); i++ {
		if kept[i].Seq < kept[i-1].Seq {
			t.Fatalf("window is not oldest-first at %d", i)
		}
	}
	last := kept[len(kept)-1].Seq
	for _, r := range recs {
		in := false
		for _, k := range kept {
			if k.RecordID == r.RecordID {
				in = true
			}
		}
		if r.Seq <= last && !in {
			t.Fatalf("record %s (seq %d) is below the window end %d but missing: a seq group was split", r.RecordID, r.Seq, last)
		}
	}
	huge := []InboxRecord{{Seq: 7, RecordID: "huge", Kind: "notice", From: "source:x", Body: strings.Repeat("z", 3<<20)}, {Seq: 8, RecordID: "after", Kind: "notice"}}
	kept, cut = WindowRecords(huge, FrameRecordsBudget)
	if len(kept) != 2 || cut || kept[0].Seq != 7 || kept[0].RecordID != "huge" || RecordSize(kept[0]) > FrameItemBudget || !strings.Contains(kept[0].Body, "record cut") {
		t.Fatalf("an oversize record must become a stub that keeps its seq and id: %+v", kept[0])
	}
}

func TestFitFrameCutsBacklogAndErrorsOnlyWhenTheRestCannotFit(t *testing.T) {
	frame := SessionFrame{ID: "f", Source: "1", Records: bigRecords(100, 100<<10)} // 10 MB
	for i := 0; i < 400; i++ {
		frame.Updates = append(frame.Updates, PendingUpdate{UpdateID: fmt.Sprintf("u%03d", i), HumanProjection: strings.Repeat("u", 20<<10)})
	}
	raw, err := fitFrame(frame)
	if err != nil {
		t.Fatalf("fitFrame: %v", err)
	}
	if len(raw) > MaxFramePayload {
		t.Fatalf("frame still %d bytes", len(raw))
	}
	var got SessionFrame
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.Backlog == nil || !got.Backlog.Truncated || got.Backlog.RecordsRemaining == 0 {
		t.Fatalf("a cut frame must say so: %+v", got.Backlog)
	}
	if len(got.Records) == 0 || got.Records[0].RecordID != "r000" {
		t.Fatalf("the cut must keep the oldest records, got %d", len(got.Records))
	}
	// Whatever the cut kept is a prefix: the cursor follows the last kept.
	for i, r := range got.Records {
		if r.RecordID != fmt.Sprintf("r%03d", i) {
			t.Fatalf("kept records are not a prefix at %d: %s", i, r.RecordID)
		}
	}

	// A doc alone over the bound is not backlog: it is an error, and nothing
	// was written.
	big := SessionFrame{ID: "g", Doc: &DocSnapshot{Content: strings.Repeat("d", MaxFramePayload+1)}, Records: bigRecords(3, 1000)}
	if _, err := fitFrame(big); err == nil {
		t.Fatal("a frame whose document alone exceeds the bound must be refused")
	}
}

// TestDeskSessionWorkerStartsCellOnOversizedBacklog drives the real worker:
// before the cut, the 10 MB frame was refused at the write and the worker
// marked dead.
func TestDeskSessionWorkerStartsCellOnOversizedBacklog(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("desk session worker spawn requires unix socketpair")
	}
	w, err := SpawnDeskSessionWorker(deskWorkerConfig(t))
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	defer w.Close()
	res, err := w.EvalFrame(context.Background(), SessionFrame{Source: `rs := choir.Records()
fmt.Println(len(rs), rs[0].ID, choir.Context()["backlog_truncated"], choir.Context()["records_remaining"] != "")`, Records: bigRecords(100, 100<<10)})
	if err != nil || res.Error != "" {
		t.Fatalf("the cell did not start on an oversized backlog: err=%v res=%+v", err, res)
	}
	if w.Dead() {
		t.Fatal("the worker was marked dead")
	}
	if !strings.Contains(res.Stdout, " r000 true true") {
		t.Fatalf("the cell must see the oldest record and the truncation marker: %q", res.Stdout)
	}
}
