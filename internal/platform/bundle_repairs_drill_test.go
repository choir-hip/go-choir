package platform

// TestBundleImportRepairs drives the review repairs of the authenticity lane
// against PostgreSQL:
//
//	R1  receipts.jsonl swapped between verify and the overlap plan (the fork
//	    receipt of the target spliced in, the verified file restored after
//	    the plan) is refused, not committed as a non-chaining tape;
//	R2  a rewritten created_at imports as the signed issued_at;
//	R4  export refuses ranges verify would refuse.

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/bundle"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

func TestBundleImportRepairs(t *testing.T) {
	ctx := context.Background()
	const id = "computer-bundle-repairs"
	src := newDrillComputer(t, id)
	src.appendEvent(computerevent.EventGenesisImported, "", nil)
	src.appendMixed(3, "pre")
	snap := src.snapshot(2)
	src.appendSnapshotRecorded(snap.Root)
	src.appendMixed(2, "post")
	out := filepath.Join(t.TempDir(), "bundle")
	m := mustExport(t, src, out)

	// R4: ranges verify refuses are refused at export.
	if _, err := bundle.Export(ctx, src.store.db, src.root, bundle.ExportOptions{ComputerID: id, Out: filepath.Join(t.TempDir(), "x"), FromSeq: snap.HeadSequence + 1}); err == nil {
		t.Fatal("export from a sequence past its snapshot head was written")
	}
	bare := newDrillComputer(t, "computer-bundle-bare")
	bare.appendEvent(computerevent.EventGenesisImported, "", nil)
	bare.appendMixed(3, "x")
	if _, err := bundle.Export(ctx, bare.store.db, bare.root, bundle.ExportOptions{ComputerID: "computer-bundle-bare", Out: filepath.Join(t.TempDir(), "y"), FromSeq: 2}); err == nil {
		t.Fatal("export of a snapshot-less, non-delta range from 2 was written")
	}

	// R2: a rewritten created_at is not imported.
	tampered := filepath.Join(t.TempDir(), "tampered")
	copyTree(t, out, tampered)
	raw, err := os.ReadFile(filepath.Join(tampered, bundle.ReceiptsName))
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		var row map[string]any
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			t.Fatal(err)
		}
		row["created_at"] = "1999-01-01T00:00:00Z"
		b, _ := json.Marshal(row)
		lines = append(lines, string(b))
	}
	if err := os.WriteFile(filepath.Join(tampered, bundle.ReceiptsName), []byte(strings.Join(lines, "\n")+"\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	dst := newDrillComputer(t, id)
	if _, err := bundle.Import(ctx, tampered, dst.store.db, dst.root, src.trust()); err != nil {
		t.Fatalf("import: %v", err)
	}
	for _, r := range drillReceipts(t, dst, 0) {
		var fields struct {
			IssuedAt string `json:"issued_at"`
		}
		if err := json.Unmarshal([]byte(r.EventHeadReceiptJSON), &fields); err != nil {
			t.Fatal(err)
		}
		issued, err := time.Parse(time.RFC3339Nano, fields.IssuedAt)
		if err != nil {
			t.Fatal(err)
		}
		if d := r.CreatedAt.Sub(issued.UTC()); d > time.Microsecond || d < -time.Microsecond {
			t.Errorf("receipt %d imported created_at %s, the signed issued_at is %s", r.Sequence, r.CreatedAt, issued)
			break
		}
	}

	// R1: the verify-to-plan swap. The target forked at the head; the
	// attacker puts the target's own fork receipt into the bundle for the
	// plan read and restores the verified file after it.
	forked := newDrillComputer(t, id)
	if _, err := bundle.Import(ctx, out, forked.store.db, forked.root, src.trust()); err != nil {
		t.Fatalf("import: %v", err)
	}
	forked.appendEvent(computerevent.EventToolInvoked, "", nil)
	src.appendMixed(2, "later")
	out2 := filepath.Join(t.TempDir(), "bundle2")
	mustExport(t, src, out2)
	forkRow := drillReceipts(t, forked, m.Tape.HeadSeq)[0]
	path := filepath.Join(out2, bundle.ReceiptsName)
	good, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var swapped []string
	for _, line := range strings.Split(strings.TrimSpace(string(good)), "\n") {
		var row bundle.ReceiptRow
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			t.Fatal(err)
		}
		if row.Sequence == forkRow.Sequence {
			b, _ := json.Marshal(forkRow)
			line = string(b)
		}
		swapped = append(swapped, line)
	}
	bundle.ImportHooks.AfterVerify = func(string) {
		if err := os.WriteFile(path, []byte(strings.Join(swapped, "\n")+"\n"), 0o640); err != nil {
			t.Error(err)
		}
	}
	bundle.ImportHooks.AfterPlan = func(string) {
		if err := os.WriteFile(path, good, 0o640); err != nil {
			t.Error(err)
		}
	}
	t.Cleanup(func() { bundle.ImportHooks.AfterVerify, bundle.ImportHooks.AfterPlan = nil, nil })
	before := forked.head()
	rowsBefore := len(drillReceipts(t, forked, 0))
	_, err = bundle.Import(ctx, out2, forked.store.db, forked.root, src.trust())
	if !errors.Is(err, bundle.ErrRefused) || !strings.Contains(err.Error(), "changed after it was verified") {
		t.Fatalf("swap between verify and plan: import = %v, want a changed-bytes refusal", err)
	}
	if !reflect.DeepEqual(before, forked.head()) || len(drillReceipts(t, forked, 0)) != rowsBefore {
		t.Fatal("a refused import moved the head or inserted receipts")
	}
}
