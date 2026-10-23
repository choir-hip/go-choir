package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/storeschema"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The stream_seq lease (docs/problems/stream-seq-high-water-mark-design-2026-10-23.md,
// round 3 at b849092b7, section 6 "Tests first"). The numbers in the names
// are that list's. Ways the build can fail, written before the code:
//  1. A restart (or a swap, or a rollback) trusts a lease that a lineage the
//     lease does not know has outrun: a stream_seq is handed out twice. Tests
//     1 to 6, 8 to 11.
//  2. The exact seed is a band and not the maximum, or skips the og half when
//     the first window holds an event. Tests 12 to 17.
//  3. The allocator hands out a number above the durable lease; two seeders
//     run; a refill wedges; a lease that failed to land is used anyway.
//     Tests 18 to 26.
//  4. A trusted restart still reads the event kind. Test 34.
//  5. The lease op is not a pure function of its fields, so a base and a
//     replay disagree. Tests 31, 32.

// shrinkLease sets the lease constants for one test. The test seams are
// variables so a block of eight can be exhausted in a unit test; the
// production values are the design's constants table.
func shrinkLease(t *testing.T, block int64) {
	t.Helper()
	oldBlock, oldGap, oldCeil := streamSeqLeaseBlock, streamSeqExactGap, streamSeqLeaseCeiling
	oldReady, oldFloor, oldMeasured := leaseReadyDeadline, leaseSeedingDeadlineFloor, leaseSeedingMeasured
	oldBackoff, oldPage := leaseRefillBackoff, leaseScanPage
	t.Cleanup(func() {
		streamSeqLeaseBlock, streamSeqExactGap, streamSeqLeaseCeiling = oldBlock, oldGap, oldCeil
		leaseReadyDeadline, leaseSeedingDeadlineFloor, leaseSeedingMeasured = oldReady, oldFloor, oldMeasured
		leaseRefillBackoff, leaseScanPage = oldBackoff, oldPage
	})
	streamSeqLeaseBlock = block
	leaseReadyDeadline = 2 * time.Second
	leaseSeedingDeadlineFloor = 5 * time.Second
	leaseSeedingMeasured = 0
	leaseRefillBackoff = 20 * time.Millisecond
}

func leaseCtx() context.Context { return context.Background() }

func leaseStats(s *Store) StreamSeqLeaseStats { return s.StreamSeqLeaseStats() }

// receiptVersion reads the epoch the workspace's receipt names (0 when absent).
func receiptVersion(t *testing.T, s *Store) uint64 {
	t.Helper()
	receipt, found, err := storeschema.Read(filepath.Join(s.TexturePath(), storeschema.File))
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		return 0
	}
	return receipt.Version
}

// setReceipt rewrites the workspace's receipt to an epoch, as another binary
// would; version 0 removes it. The store may be open: it is a file.
func setReceipt(t *testing.T, workspace string, version uint64) {
	t.Helper()
	path := filepath.Join(workspace, storeschema.File)
	if version == 0 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		return
	}
	raw, err := json.Marshal(storeschema.Receipt{Schema: storeschema.Name, Version: version})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func markerPresent(t *testing.T, workspace string) bool {
	t.Helper()
	present, err := storeschema.LeaseUntrusted(workspace)
	if err != nil {
		t.Fatal(err)
	}
	return present
}

// putStoredEvent stores an event with an explicit stream_seq, bypassing the
// allocator: what a binary that predates the lease, or a stamp, left behind.
func putStoredEvent(t *testing.T, s *Store, id, run string, stream int64, at time.Time) {
	t.Helper()
	rec := &types.EventRecord{EventID: id, RunID: run, OwnerID: "alice", Seq: stream, StreamSeq: stream, Timestamp: at,
		Kind: types.EventRunProgress, Payload: json.RawMessage(`{"id":"` + id + `"}`)}
	obj, err := buildEventObject(rec, at)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ogStore.PutBatch(leaseCtx(), objectgraph.Batch{Objects: []objectgraph.Object{obj}}); err != nil {
		t.Fatal(err)
	}
}

func appendNext(t *testing.T, s *Store, id string) int64 {
	t.Helper()
	return appendSeedEvent(t, s, id, "run-"+id, time.Now().UTC()).StreamSeq
}

func appendN(t *testing.T, s *Store, prefix string, n int) []int64 {
	t.Helper()
	out := make([]int64, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, appendNext(t, s, fmt.Sprintf("%s-%d", prefix, i)))
	}
	return out
}

// appendOld appends n events stamped an hour ago, so objects written after
// them (seedWakeOutbox stamps now) sit in the newer windows.
func appendOld(t *testing.T, s *Store, prefix string, n int) []int64 {
	t.Helper()
	base := time.Now().UTC().Add(-time.Hour)
	out := make([]int64, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, appendSeedEvent(t, s, fmt.Sprintf("%s-%d", prefix, i), "run-"+prefix, base.Add(time.Duration(i)*time.Second)).StreamSeq)
	}
	return out
}

func readLeaseRow(t *testing.T, s *Store) (leaseRow, bool) {
	t.Helper()
	row, found, err := s.readLease(leaseCtx())
	if err != nil {
		t.Fatal(err)
	}
	return row, found
}

// rangePageCalls counts the exact seed's primary-key id range pages (the
// linear walk); keysetPageCalls the kind-predicate pages (the fallback, a
// TopN per page). pageOpCalls is both.
func rangePageCalls(s *Store) int {
	total := 0
	for op, st := range s.EngineMutexStats() {
		if strings.Contains(op, "ListKindIDRangePage") {
			total += int(st.Calls)
		}
	}
	return total
}

func keysetPageCalls(s *Store) int {
	total := 0
	for op, st := range s.EngineMutexStats() {
		if strings.Contains(op, "ListKindMetadataPage") {
			total += int(st.Calls)
		}
	}
	return total
}

func pageOpCalls(s *Store) int { return rangePageCalls(s) + keysetPageCalls(s) }

func eventKindReads(s *Store) []string {
	var out []string
	for op, st := range s.EngineMutexStats() {
		// The per-run seq seed (ListObjectsByMetadata by run_id) is the run
		// counter's own, not the stream_seq seed.
		if st.Calls > 0 && strings.Contains(op, "choir.event") && !strings.Contains(op, "ListObjectsByMetadata") {
			out = append(out, fmt.Sprintf("%s x%d", op, st.Calls))
		}
	}
	return out
}

// ---- The constants table ------------------------------------------------

func TestLeaseConstantsAreTheDesignsTable(t *testing.T) {
	if streamSeqLeaseBlock != 65536 {
		t.Errorf("N = %d, want 65,536", streamSeqLeaseBlock)
	}
	if streamSeqExactGap != 1<<20 {
		t.Errorf("G = %d, want 2^20", streamSeqExactGap)
	}
	if streamSeqLeaseCeiling != 1<<40 {
		t.Errorf("C = %d, want 2^40", streamSeqLeaseCeiling)
	}
	if leaseReadyDeadline != 30*time.Second {
		t.Errorf("ready deadline = %v, want 30s", leaseReadyDeadline)
	}
	if leaseSeedingDeadlineFloor != 10*time.Minute {
		t.Errorf("seeding deadline floor = %v, want 10 minutes", leaseSeedingDeadlineFloor)
	}
	if leaseRefillBackoff != time.Second {
		t.Errorf("refill backoff = %v, want 1s", leaseRefillBackoff)
	}
	if leaseScanPage != 5000 {
		t.Errorf("page size = %d, want 5,000", leaseScanPage)
	}
	if leaseFixedTime != time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC) {
		t.Errorf("lease timestamp = %v", leaseFixedTime)
	}
	if leaseOwnerID != "__system__" || leaseIdentityKey != "stream_seq_lease" {
		t.Errorf("owner/identity = %q/%q", leaseOwnerID, leaseIdentityKey)
	}
	// The seeding deadline is the larger of ten minutes and three times the
	// measured exact seed.
	oldMeasured := leaseSeedingMeasured
	defer func() { leaseSeedingMeasured = oldMeasured }()
	leaseSeedingMeasured = time.Minute
	if got := leaseSeedingDeadline(); got != 10*time.Minute {
		t.Errorf("deadline with a 1m seed = %v, want the 10 minute floor", got)
	}
	leaseSeedingMeasured = 20 * time.Minute
	if got := leaseSeedingDeadline(); got != 60*time.Minute {
		t.Errorf("deadline with a 20m seed = %v, want 3x = 60m", got)
	}
}

// ---- Shape of the row and the observability block -----------------------

func TestLeaseRowShapeAndFixedTimestamps(t *testing.T) {
	s, _ := openSeedStore(t)
	appendN(t, s, "a", 3)
	obj, err := s.ogStore.GetObject(leaseCtx(), leaseCanonicalID(t))
	if err != nil {
		t.Fatalf("lease object missing after the first allocation: %v", err)
	}
	if obj.ObjectKind != objectgraph.KindStreamSeqLease || obj.OwnerID != "__system__" {
		t.Fatalf("lease object kind/owner = %s/%s", obj.ObjectKind, obj.OwnerID)
	}
	fixed := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	if !obj.CreatedAt.Equal(fixed) || !obj.UpdatedAt.Equal(fixed) {
		t.Fatalf("lease timestamps = %v / %v, want 2000-01-01T00:00:00Z (a pure function of the fields)", obj.CreatedAt, obj.UpdatedAt)
	}
	if string(obj.Body) != string(obj.Metadata) {
		t.Fatalf("lease body and metadata differ:\n%s\n%s", obj.Body, obj.Metadata)
	}
	var raw map[string]any
	if err := json.Unmarshal(obj.Body, &raw); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"reserved_through", "allocated_through", "block", "writer_epoch", "basis", "head_sequence"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("lease body lacks %q: %s", key, obj.Body)
		}
	}
	row, found := readLeaseRow(t, s)
	if !found || row.Basis != "exact" || row.WriterEpoch != 3 || row.Block != streamSeqLeaseBlock {
		t.Fatalf("row = %+v found=%v", row, found)
	}
	if row.Reserved != row.Allocated+streamSeqLeaseBlock {
		t.Fatalf("a seeder writes R = A + N: R=%d A=%d N=%d", row.Reserved, row.Allocated, streamSeqLeaseBlock)
	}
}

func TestLeaseObservabilityBlockShape(t *testing.T) {
	s, path := openSeedStore(t)
	appendN(t, s, "a", 2)
	st := leaseStats(s)
	if st.State != "ready" || !st.Trusted && st.UntrustReason == "" {
		t.Fatalf("stats = %+v", st)
	}
	raw, err := json.Marshal(st)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	want := []string{"state", "reserved_through", "allocated_through", "trusted", "untrust_reason", "seeded_from",
		"seed_ms", "cross_check_max", "refill_failed", "last_lease_error", "marker_present", "seed_failed", "lease_tombstoned", "seed_undecodable_rows"}
	for _, key := range want {
		if _, ok := fields[key]; !ok {
			t.Errorf("observability block lacks %q: %s", key, raw)
		}
	}
	if len(fields) != len(want) {
		t.Errorf("observability block has %d fields, want exactly %d: %s", len(fields), len(want), raw)
	}
	if st.SeededFrom != "exact" || st.UntrustReason != "no_lease" || st.Trusted {
		t.Fatalf("first boot of a store with no lease: %+v", st)
	}
	s = reopenAt(t, s, path)
	appendN(t, s, "b", 1)
	st = leaseStats(s)
	if st.SeededFrom != "lease" || !st.Trusted || st.UntrustReason != "" || st.MarkerPresent {
		t.Fatalf("planned restart: %+v", st)
	}
	if st.ReservedThrough < st.AllocatedThrough {
		t.Fatalf("reserved %d below allocated %d", st.ReservedThrough, st.AllocatedThrough)
	}
}

// ---- Trust (tests 1 to 11) ---------------------------------------------

// Test 1. Receipt 2 (an epoch-2 binary rewrote it), Open writes 3, a restart
// before any append, reopen: the marker is present, the reason is receipt,
// and the seed is exact.
func TestLeaseReceiptTwoWritesTheMarkerAndForcesTheExactSeed(t *testing.T) {
	s, path := openSeedStore(t)
	first := appendN(t, s, "a", 3)
	row, _ := readLeaseRow(t, s)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	workspace := deriveTextureWorkspacePath(path)
	setReceipt(t, workspace, 2) // the epoch-2 interlude rewrote it

	s, err := Open(path) // writes 3
	if err != nil {
		t.Fatal(err)
	}
	if got := receiptVersion(t, s); got != 3 {
		t.Fatalf("Open left receipt %d, want 3", got)
	}
	if !markerPresent(t, workspace) {
		t.Fatal("Open read receipt 2 and wrote receipt 3 without the untrust marker")
	}
	if err := s.Close(); err != nil { // restart before any append
		t.Fatal(err)
	}
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if !markerPresent(t, workspace) {
		t.Fatal("the marker did not survive a restart: receipt 3 would launder receipt 2")
	}
	next := appendNext(t, s, "after")
	st := leaseStats(s)
	if st.Trusted || st.UntrustReason != "receipt" || st.SeededFrom != "exact" {
		t.Fatalf("stats after receipt 2 = %+v, want untrusted for receipt, exact", st)
	}
	if next <= first[len(first)-1] || next <= row.Reserved {
		t.Fatalf("next stream_seq %d is not above the old lineage (last %d, R %d)", next, first[len(first)-1], row.Reserved)
	}
	if markerPresent(t, workspace) {
		t.Fatal("the exact seed landed its lease and did not clear the marker")
	}
}

// Test 2. A lease row is present and the receipt is absent: untrusted.
func TestLeaseWithAnAbsentReceiptIsUntrusted(t *testing.T) {
	s, path := openSeedStore(t)
	appendN(t, s, "a", 3)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	workspace := deriveTextureWorkspacePath(path)
	setReceipt(t, workspace, 0)
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if !markerPresent(t, workspace) {
		t.Fatal("an absent receipt on a workspace that holds rows did not write the marker")
	}
	appendNext(t, s, "after")
	if st := leaseStats(s); st.Trusted || st.SeededFrom != "exact" {
		t.Fatalf("stats = %+v", st)
	}
}

// A workspace with no rows and no receipt is a fresh store, not a swap: no
// marker, and the first allocation is the cheap exact seed of nothing.
func TestFreshStoreWritesNoMarker(t *testing.T) {
	s, path := openSeedStore(t)
	if markerPresent(t, deriveTextureWorkspacePath(path)) {
		t.Fatal("a fresh store carries an untrust marker")
	}
	if got := receiptVersion(t, s); got != 3 {
		t.Fatalf("receipt = %d, want 3", got)
	}
}

// Test 3 (the store half). A workspace with receipt 3 and a lease, after an
// epoch-2 interlude that allocated past R: untrusted once, no reissue; the
// second boot is trusted again. (The base half is in internal/projectionbase.)
func TestLeaseAfterAnEpochTwoInterludeIsUntrustedOnceAndNeverReissues(t *testing.T) {
	s, path := openSeedStore(t)
	appendN(t, s, "a", 3)
	row, _ := readLeaseRow(t, s)
	past := row.Reserved + 5000 // the interlude's numbers, above R
	putStoredEvent(t, s, "interlude", "run-i", past, time.Now().UTC())
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	workspace := deriveTextureWorkspacePath(path)
	setReceipt(t, workspace, 2) // the epoch-2 binary's Open
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	n := appendNext(t, s, "forward")
	if n <= past {
		t.Fatalf("stream_seq %d reissued a number the epoch-2 interlude stored (%d)", n, past)
	}
	if st := leaseStats(s); st.UntrustReason != "receipt" || st.SeededFrom != "exact" {
		t.Fatalf("first boot forward: %+v", st)
	}
	s = reopenAt(t, s, path)
	n2 := appendNext(t, s, "second")
	if n2 <= n {
		t.Fatalf("second boot stream_seq %d is not above the first boot's %d", n2, n)
	}
	if st := leaseStats(s); !st.Trusted || st.SeededFrom != "lease" {
		t.Fatalf("second boot should be trusted again: %+v", st)
	}
}

// Test 6. ReplaceWorkspace leaves the marker in the fresh workspace.
func TestReplaceWorkspaceLeavesTheMarker(t *testing.T) {
	s, path := openSeedStore(t)
	appendN(t, s, "a", 2)
	quarantine := filepath.Join(filepath.Dir(path), "lease-quarantine-"+filepath.Base(path))
	t.Cleanup(func() { _ = os.RemoveAll(quarantine) })
	fresh, _, err := s.ReplaceWorkspace(quarantine)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = fresh.Close() })
	if !markerPresent(t, fresh.TexturePath()) {
		t.Fatal("ReplaceWorkspace left no untrust marker in the fresh workspace")
	}
	if n := appendNext(t, fresh, "after"); n <= streamSeqExactGap {
		t.Fatalf("first stream_seq of a replaced workspace = %d", n)
	}
	if markerPresent(t, fresh.TexturePath()) {
		t.Fatal("the exact seed of the fresh workspace did not clear the marker")
	}
}

// Test 8 (behaviour). Open reads the receipt before it writes it: if it wrote
// first, receipt 3 would hide that the prior receipt was 2.
func TestOpenReadsTheReceiptBeforeItWritesIt(t *testing.T) {
	s, path := openSeedStore(t)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	workspace := deriveTextureWorkspacePath(path)
	for _, tc := range []struct {
		prior  uint64
		marker bool
	}{{2, true}, {1, true}, {0, true}, {3, false}} {
		_ = storeschema.ClearLeaseUntrusted(workspace)
		setReceipt(t, workspace, tc.prior)
		s, err := Open(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := markerPresent(t, workspace); got != tc.marker {
			t.Errorf("prior receipt %d: marker present = %v, want %v", tc.prior, got, tc.marker)
		}
		if got := receiptVersion(t, s); got != 3 {
			t.Errorf("prior receipt %d: Open left %d, want 3", tc.prior, got)
		}
		_ = s.Close()
	}
}

// Test 9. A marker-write failure aborts Open, and the receipt is unchanged: a
// store never opens with receipt 3 and no marker for a prior receipt of 2.
func TestMarkerWriteFailureAbortsOpenAndLeavesTheReceipt(t *testing.T) {
	s, path := openSeedStore(t)
	appendN(t, s, "a", 1)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	workspace := deriveTextureWorkspacePath(path)
	setReceipt(t, workspace, 2)
	// A directory where the marker file belongs: the rename cannot publish.
	if err := os.Mkdir(filepath.Join(workspace, storeschema.LeaseUntrustedFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if opened, err := Open(path); err == nil {
		_ = opened.Close()
		t.Fatal("Open succeeded although the untrust marker could not be written")
	}
	receipt, found, err := storeschema.Read(filepath.Join(workspace, storeschema.File))
	if err != nil || !found || receipt.Version != 2 {
		t.Fatalf("receipt after the aborted Open = %+v found=%v err=%v, want it still 2", receipt, found, err)
	}
	// Repair the obstacle: the next Open writes the marker and receipt 3.
	if err := os.Remove(filepath.Join(workspace, storeschema.LeaseUntrustedFile)); err != nil {
		t.Fatal(err)
	}
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if !markerPresent(t, workspace) || receiptVersion(t, s) != 3 {
		t.Fatal("after the repair Open did not write both the marker and the receipt")
	}
}

// Test 10. A self-inconsistent row is untrusted.
func TestInconsistentLeaseRowsAreUntrusted(t *testing.T) {
	shrinkLease(t, 8)
	rows := map[string]func(r leaseRow) leaseRow{
		"R above A+3N/2": func(r leaseRow) leaseRow { r.Reserved = r.Allocated + 13; return r },
		"A above R":      func(r leaseRow) leaseRow { r.Allocated = r.Reserved + 1; return r },
		"epoch below 3":  func(r leaseRow) leaseRow { r.WriterEpoch = 2; return r },
		"zero block":     func(r leaseRow) leaseRow { r.Block = 0; return r },
		"negative A":     func(r leaseRow) leaseRow { r.Allocated = -1; return r },
	}
	for name, mutate := range rows {
		t.Run(name, func(t *testing.T) {
			s, path := openSeedStore(t)
			appendN(t, s, "a", 3)
			row, _ := readLeaseRow(t, s)
			bad := mutate(row)
			if err := s.writeLease(leaseCtx(), bad); err != nil {
				t.Fatal(err)
			}
			s = reopenAt(t, s, path)
			next := appendNext(t, s, "after")
			st := leaseStats(s)
			if st.Trusted || st.UntrustReason != "inconsistent" || st.SeededFrom != "exact" {
				t.Fatalf("stats = %+v, want untrusted for inconsistent, exact", st)
			}
			if next <= row.Reserved {
				t.Fatalf("stream_seq %d not above the real lease %d", next, row.Reserved)
			}
		})
	}
	t.Run("the bound is inclusive: R = A + 3N/2 is consistent", func(t *testing.T) {
		row := leaseRow{Reserved: 100 + 12, Allocated: 100, Block: 8, WriterEpoch: 3, Basis: "lease"}
		if !row.consistent() {
			t.Fatal("R = A + 3N/2 is what a refill writes at the trigger boundary")
		}
		row.Reserved = 100 + 13
		if row.consistent() {
			t.Fatal("R = A + 3N/2 + 1 is not consistent")
		}
	})
}

// Test 11 (bunch 164 F2). A legitimate lease whose cross-check underestimates
// the maximum, because the windows hold no event, is trusted, and the second
// boot does no exact seed: the exact scan is a once-per-swap cost, not a
// per-boot one.
func TestLegitimateLeaseWithABlindCrossCheckIsTrustedEveryBoot(t *testing.T) {
	shrinkSeedWindows(t, 20, 200)
	s, path := openSeedStore(t)
	appendOld(t, s, "a", 5)
	row, _ := readLeaseRow(t, s)
	seedWakeOutbox(t, s, 400, 0) // newer objects of another kind: both windows miss every event
	for boot := 1; boot <= 2; boot++ {
		s = reopenAt(t, s, path)
		n := appendSeedEvent(t, s, fmt.Sprintf("boot-%d", boot), "run-boot", time.Now().UTC().Add(-time.Hour)).StreamSeq
		st := leaseStats(s)
		if !st.Trusted || st.SeededFrom != "lease" || st.UntrustReason != "" {
			t.Fatalf("boot %d: stats = %+v, want trusted from the lease", boot, st)
		}
		if st.CrossCheckMax != 0 {
			t.Fatalf("boot %d: the blind cross-check read %d", boot, st.CrossCheckMax)
		}
		if calls := pageOpCalls(s); calls != 0 {
			t.Fatalf("boot %d: a trusted boot ran %d exact-seed pages", boot, calls)
		}
		if n <= row.Reserved {
			t.Fatalf("boot %d: stream_seq %d not above R=%d", boot, n, row.Reserved)
		}
		row, _ = readLeaseRow(t, s)
	}
}

// A stale lease: the stored maximum is above R. The cross-check sees it, the
// lease is untrusted for violation, and the exact seed starts above both.
func TestLeaseBelowTheCrossCheckIsViolated(t *testing.T) {
	s, path := openSeedStore(t)
	appendN(t, s, "a", 3)
	row, _ := readLeaseRow(t, s)
	stale := row.Reserved + 77
	putStoredEvent(t, s, "stale", "run-s", stale, time.Now().UTC())
	s = reopenAt(t, s, path)
	buf := captureLog(t)
	n := appendNext(t, s, "after")
	st := leaseStats(s)
	if st.Trusted || st.UntrustReason != "violated" || st.SeededFrom != "exact" {
		t.Fatalf("stats = %+v, want untrusted for violated", st)
	}
	if n <= stale {
		t.Fatalf("stream_seq %d reissued the stored %d", n, stale)
	}
	if !strings.Contains(buf.String(), fmt.Sprint(row.Reserved)) || !strings.Contains(buf.String(), fmt.Sprint(stale)) {
		t.Fatalf("the violation log line must give both numbers (R=%d, stored=%d):\n%s", row.Reserved, stale, buf.String())
	}
}

// ---- Exact seed (tests 12 to 17) -----------------------------------------

// Test 12. An epoch-2 high number behind newer events and many objects is not
// reissued after forward.
func TestExactSeedFindsAHighNumberBehindNewerEventsAndObjects(t *testing.T) {
	shrinkSeedWindows(t, 20, 200)
	s, path := openSeedStore(t)
	base := time.Now().UTC().Add(-time.Hour)
	putStoredEvent(t, s, "high", "run-h", 9000, base)
	for i := 1; i <= 520; i++ { // more than the old newest-500 read
		putStoredEvent(t, s, fmt.Sprintf("low-%d", i), "run-l", int64(i), base.Add(time.Duration(i)*time.Second))
	}
	seedWakeOutbox(t, s, 250, 0) // behind both scaled windows
	s = reopenAt(t, s, path)
	n := appendNext(t, s, "forward")
	if n != 9000+streamSeqExactGap+1 {
		t.Fatalf("stream_seq = %d, want %d (exact maximum 9000 plus the gap)", n, 9000+streamSeqExactGap+1)
	}
	if st := leaseStats(s); st.SeededFrom != "exact" || st.UntrustReason != "no_lease" {
		t.Fatalf("stats = %+v", st)
	}
}

// Test 13. Untrusted runs the full exact seed even when the first window holds
// an event.
func TestUntrustedRunsTheFullExactSeedEvenWhenTheFirstWindowHoldsAnEvent(t *testing.T) {
	s, path := openSeedStore(t)
	appendN(t, s, "a", 4)
	row, _ := readLeaseRow(t, s)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if err := storeschema.WriteLeaseUntrusted(deriveTextureWorkspacePath(path), "install"); err != nil {
		t.Fatal(err)
	}
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	n := appendNext(t, s, "after")
	if rangePageCalls(s) == 0 {
		t.Fatal("an untrusted lease did not run the exact seed (no id range pages were read)")
	}
	if keysetPageCalls(s) != 0 {
		t.Fatalf("a normal store took the quadratic kind-predicate fallback %d times", keysetPageCalls(s))
	}
	// seed = max(og, table, R') + G with R' = the self-consistent lease's R.
	if want := row.Reserved + streamSeqExactGap + 1; n != want {
		t.Fatalf("stream_seq = %d, want %d (R %d plus the gap)", n, want, row.Reserved)
	}
	if st := leaseStats(s); st.UntrustReason != "marker" || st.SeededFrom != "exact" || st.MarkerPresent {
		t.Fatalf("stats = %+v (a seeded lease clears the marker)", st)
	}
}

// The exact seed pages the kind by keyset, a page at a time, and a row whose
// metadata has no stream_seq has its body decoded.
func TestExactSeedPagesTheKindAndDecodesABodyWithoutMetadataSeq(t *testing.T) {
	old := leaseScanPage
	leaseScanPage = 3
	t.Cleanup(func() { leaseScanPage = old })
	s, path := openSeedStore(t)
	base := time.Now().UTC().Add(-time.Hour)
	for i := 1; i <= 10; i++ {
		putStoredEvent(t, s, fmt.Sprintf("e-%d", i), "run", int64(i), base.Add(time.Duration(i)*time.Second))
	}
	// An event whose metadata lacks stream_seq: the body carries 7777.
	at := base.Add(time.Minute)
	rec := eventTableRecord("legacy", "run-x", "alice", "traj-x", types.EventRunProgress, 1, 7777, at)
	obj, err := buildEventObject(rec, at)
	if err != nil {
		t.Fatal(err)
	}
	obj.Metadata = []byte(`{"event_id":"legacy"}`)
	obj.ContentHash = objectgraph.ContentHash(obj.ObjectKind, obj.Body, obj.Metadata)
	if err := s.ogStore.PutBatch(leaseCtx(), objectgraph.Batch{Objects: []objectgraph.Object{obj}}); err != nil {
		t.Fatal(err)
	}
	s = reopenAt(t, s, path)
	n := appendNext(t, s, "after")
	if want := int64(7777) + streamSeqExactGap + 1; n != want {
		t.Fatalf("stream_seq = %d, want %d", n, want)
	}
	if calls := rangePageCalls(s); calls < 4 {
		t.Fatalf("11 events at 3 a page took %d range page reads, want at least 4", calls)
	}
	if keysetPageCalls(s) != 0 {
		t.Fatalf("a normal store took the kind-predicate fallback %d times", keysetPageCalls(s))
	}
}

// Test 14 (a). With batching off a failed append in the previous process
// published a number it never stored. A lease-unaware process left no lease:
// migration adds the gap, so the published number is not reissued.
func TestMigrationGapCoversANumberPublishedAndNeverStored(t *testing.T) {
	s, path := openSeedStore(t)
	base := time.Now().UTC().Add(-time.Hour)
	for i := int64(1); i <= 40; i++ {
		putStoredEvent(t, s, fmt.Sprintf("e-%d", i), "run", i, base.Add(time.Duration(i)*time.Second))
	}
	const published = 50 // allocated by the old process, the append failed, the bus carried it
	s = reopenAt(t, s, path)
	if n := appendNext(t, s, "after"); n <= published {
		t.Fatalf("stream_seq %d reissued the published-never-stored %d", n, published)
	} else if n != 40+streamSeqExactGap+1 {
		t.Fatalf("stream_seq = %d, want %d", n, 40+streamSeqExactGap+1)
	}
}

// Test 14 (b). A lease-aware previous process: a failed append (the tape
// refused) published a number; the lease covers it after a restart even with
// batching off.
func TestLeaseCoversANumberPublishedByAFailedAppend(t *testing.T) {
	ctx := leaseCtx()
	h := newTapeHarness(t)
	ok := h.event("ok", "run-a", "owner-1", types.EventRunStarted, eventTableBase)
	if err := h.store.AppendEvent(ctx, ok); err != nil {
		t.Fatal(err)
	}
	h.tape.mu.Lock()
	h.tape.pinErr = func([]byte) error { return errors.New("platform outage") }
	h.tape.mu.Unlock()
	failed := h.event("failed", "run-a", "owner-1", types.EventRunStarted, eventTableBase.Add(time.Second))
	if err := h.store.AppendEvent(ctx, failed); err == nil {
		t.Fatal("the append should have failed")
	}
	if failed.StreamSeq == 0 {
		t.Fatal("the failed append took no number; the test cannot show the published one")
	}
	h.tape.mu.Lock()
	h.tape.pinErr = nil
	h.tape.mu.Unlock()
	h.reopen()
	after := h.event("after", "run-a", "owner-1", types.EventRunStarted, eventTableBase.Add(2*time.Second))
	if err := h.store.AppendEvent(ctx, after); err != nil {
		t.Fatal(err)
	}
	if after.StreamSeq <= failed.StreamSeq {
		t.Fatalf("stream_seq %d after the restart reissued the failed append's %d", after.StreamSeq, failed.StreamSeq)
	}
	if st := leaseStats(h.store); !st.Trusted || st.SeededFrom != "lease" {
		t.Fatalf("stats = %+v, want a trusted lease seed", st)
	}
}

// Test 15. DeepSeek's bunch-156 counterexample still allocates above 12: the
// table's newest is 9, an og event holds 12 and is newer, 400 newer objects of
// another kind push it out of both windows, and there is no lease.
func TestBunch156CounterexampleStillAllocatesAbove12(t *testing.T) {
	shrinkSeedWindows(t, 20, 200)
	h := newTapeHarness(t)
	ctx := leaseCtx()
	put := func(stamped bool, id string, stream int64, at time.Time) {
		t.Helper()
		op := eventTableOp(t, eventTableRecord(id, "run-a", "owner-1", "traj-a", types.EventRunProgress, stream, stream, at), stamped)
		if err := h.store.AppendProjectionOps(ctx, []computerevent.ProjectionOp{op}); err != nil {
			t.Fatalf("project %s: %v", id, err)
		}
	}
	for i := int64(1); i <= 3; i++ {
		put(false, fmt.Sprintf("og-%d", i), i, eventTableBase.Add(time.Duration(i)*time.Second))
	}
	for i := int64(4); i <= 9; i++ {
		put(true, fmt.Sprintf("tb-%d", i), i, eventTableBase.Add(time.Duration(i)*time.Second))
	}
	put(false, "og-12", 12, eventTableBase.Add(20*time.Second))
	seedWakeOutbox(t, h.store, 400, 0)
	h.reopen()
	rec := h.event("after", "run-z", "owner-1", types.EventRunStarted, eventTableBase.Add(time.Hour))
	if err := h.store.AppendEvent(ctx, rec); err != nil {
		t.Fatal(err)
	}
	if rec.StreamSeq <= 12 {
		t.Fatalf("stream_seq = %d, want above the og event's 12 and the table's 9", rec.StreamSeq)
	}
	if want := 12 + streamSeqExactGap + 1; rec.StreamSeq != want {
		t.Fatalf("stream_seq = %d, want %d", rec.StreamSeq, want)
	}
}

// Test 16. A fresh store's first stream_seq is 2^20 + 1: the exact seed of an
// empty store is 0, plus the gap.
func TestFreshStoreFirstStreamSeqIs2To20Plus1(t *testing.T) {
	s, _ := openSeedStore(t)
	if n := appendNext(t, s, "first"); n != (1<<20)+1 {
		t.Fatalf("first stream_seq = %d, want %d", n, (1<<20)+1)
	}
}

// The exact seed's table half: a number in choir_events (stamped) above the
// og half counts, wherever its owner sorts.
func TestExactSeedReadsEveryOwnersTableMaximum(t *testing.T) {
	ctx := leaseCtx()
	h := newTapeHarness(t)
	h.appendStamped(
		eventTableRecord("a-1", "run-a", "alpha", "traj-a", types.EventToolResult, 1, 30, eventTableBase.Add(time.Second)),
		eventTableRecord("b-1", "run-b", "mid", "traj-b", types.EventToolResult, 1, 900, eventTableBase.Add(2*time.Second)),
		eventTableRecord("c-1", "run-c", "zulu", "traj-c", types.EventToolResult, 1, 40, eventTableBase.Add(3*time.Second)),
		eventTableRecord("c-2", "run-c", "zulu", "traj-c", types.EventToolResult, 2, 41, eventTableBase.Add(4*time.Second)),
	)
	h.reopen()
	rec := h.event("after", "run-z", "owner-1", types.EventRunStarted, eventTableBase.Add(time.Hour))
	if err := h.store.AppendEvent(ctx, rec); err != nil {
		t.Fatal(err)
	}
	if want := 900 + streamSeqExactGap + 1; rec.StreamSeq != want {
		t.Fatalf("stream_seq = %d, want %d: an owner's table maximum was missed", rec.StreamSeq, want)
	}
}

// Test 17. A forged self-consistent R above exact + C is ignored, with a log
// line naming it, and the exact seed does not burn the number space on it.
func TestForgedLeaseAboveTheCeilingIsIgnoredWithALogLine(t *testing.T) {
	s, path := openSeedStore(t)
	appendN(t, s, "a", 3)
	stored := leaseStats(s).AllocatedThrough
	oldCeil := streamSeqLeaseCeiling
	streamSeqLeaseCeiling = 1000
	t.Cleanup(func() { streamSeqLeaseCeiling = oldCeil })
	forged := leaseRow{Reserved: 1 << 30, Allocated: 1<<30 - streamSeqLeaseBlock, Block: streamSeqLeaseBlock, WriterEpoch: 3, Basis: "exact"}
	if !forged.consistent() {
		t.Fatal("the forged row must be self-consistent for the test to mean anything")
	}
	if err := s.writeLease(leaseCtx(), forged); err != nil {
		t.Fatal(err)
	}
	s = reopenAt(t, s, path)
	buf := captureLog(t)
	n := appendNext(t, s, "after")
	st := leaseStats(s)
	if st.Trusted || st.UntrustReason != "ceiling" || st.SeededFrom != "exact" {
		t.Fatalf("stats = %+v, want untrusted for ceiling", st)
	}
	if n >= 1<<30 {
		t.Fatalf("stream_seq %d followed the forged R", n)
	}
	if n <= stored {
		t.Fatalf("stream_seq %d is not above the stored %d", n, stored)
	}
	if !strings.Contains(buf.String(), "ignored") || !strings.Contains(buf.String(), fmt.Sprint(1<<30)) {
		t.Fatalf("the ignored lease was not logged with its R:\n%s", buf.String())
	}
}

// ---- Concurrency (tests 18 to 26) ----------------------------------------

// blockScan makes the exact seed pause after its first page until released.
func blockScan(s *Store) (reached <-chan struct{}, release func()) {
	reachedCh := make(chan struct{})
	gate := make(chan struct{})
	var once sync.Once
	s.leaseScanHook = func(page int) {
		once.Do(func() { close(reachedCh) })
		<-gate
	}
	var releaseOnce sync.Once
	return reachedCh, func() { releaseOnce.Do(func() { close(gate) }) }
}

// Test 18. Two concurrent first allocations during migration: one seeder, no
// unleased number.
func TestConcurrentFirstAllocationsRunOneSeeder(t *testing.T) {
	s, path := openSeedStore(t)
	base := time.Now().UTC().Add(-time.Hour)
	for i := int64(1); i <= 5; i++ {
		putStoredEvent(t, s, fmt.Sprintf("e-%d", i), "run", i, base.Add(time.Duration(i)*time.Second))
	}
	s = reopenAt(t, s, path)
	var seeds atomic.Int32
	s.leaseWriteHook = func(phase string, reserved int64) error {
		if phase == "seed" {
			seeds.Add(1)
		}
		return nil
	}
	reached, release := blockScan(s)
	t.Cleanup(release)
	results := make(chan int64, 2)
	for i := 0; i < 2; i++ {
		go func(i int) { results <- appendNext(t, s, fmt.Sprintf("c-%d", i)) }(i)
	}
	<-reached
	select {
	case n := <-results:
		t.Fatalf("an allocation returned %d while the exact seed was still scanning", n)
	case <-time.After(150 * time.Millisecond):
	}
	release()
	a, b := <-results, <-results
	if a == b {
		t.Fatalf("both allocators got %d", a)
	}
	if seeds.Load() != 1 {
		t.Fatalf("the lease was seeded %d times, want 1", seeds.Load())
	}
	row, found := readLeaseRow(t, s)
	if !found || a > row.Reserved || b > row.Reserved {
		t.Fatalf("numbers %d, %d above the durable lease %d", a, b, row.Reserved)
	}
	if min := 5 + streamSeqExactGap + 1; a < min && b < min {
		t.Fatalf("numbers %d, %d are not above the exact seed", a, b)
	}
}

// Test 19. A refill failure clears the in-flight flag, and the next allocation
// refills after the backoff.
func TestRefillFailureClearsTheFlagAndRetriesAfterTheBackoff(t *testing.T) {
	shrinkLease(t, 8)
	leaseRefillBackoff = 80 * time.Millisecond
	s, _ := openSeedStore(t)
	var mu sync.Mutex
	var refillTimes []time.Time
	s.leaseWriteHook = func(phase string, reserved int64) error {
		if phase != "refill" {
			return nil
		}
		mu.Lock()
		defer mu.Unlock()
		refillTimes = append(refillTimes, time.Now())
		if len(refillTimes) == 1 {
			return errors.New("tape outage")
		}
		return nil
	}
	first := appendNext(t, s, "first") // seed: R = first-1+8
	row, _ := readLeaseRow(t, s)
	var last int64
	for i := 0; last < row.Reserved; i++ { // use the whole block; the trigger fires at half
		last = appendNext(t, s, fmt.Sprintf("u-%d", i))
	}
	_ = first
	// The block is spent; the next allocation needs the refill that failed.
	n := appendNext(t, s, "past")
	if n <= row.Reserved {
		t.Fatalf("stream_seq %d is not above the old lease %d", n, row.Reserved)
	}
	st := leaseStats(s)
	if st.RefillFailed != 1 || st.LastLeaseError == "" {
		t.Fatalf("stats = %+v, want refill_failed=1 and a last_lease_error", st)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(refillTimes) < 2 {
		t.Fatalf("the refill was attempted %d times, want a retry", len(refillTimes))
	}
	if gap := refillTimes[1].Sub(refillTimes[0]); gap < leaseRefillBackoff {
		t.Fatalf("the retry came %v after the failure, before the %v backoff", gap, leaseRefillBackoff)
	}
	if after, _ := readLeaseRow(t, s); after.Reserved <= row.Reserved {
		t.Fatalf("the durable lease did not advance: %d -> %d", row.Reserved, after.Reserved)
	}
}

// Test 20. Two allocators trigger one refill.
func TestTwoAllocatorsTriggerOneRefill(t *testing.T) {
	shrinkLease(t, 8)
	s, _ := openSeedStore(t)
	appendNext(t, s, "first")
	var refills atomic.Int32
	gate := make(chan struct{})
	s.leaseWriteHook = func(phase string, reserved int64) error {
		if phase == "refill" {
			refills.Add(1)
			<-gate
		}
		return nil
	}
	var wg sync.WaitGroup
	for g := 0; g < 2; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 3; i++ { // together they pass the half-block trigger
				appendNext(t, s, fmt.Sprintf("g%d-%d", g, i))
			}
		}(g)
	}
	wg.Wait()
	time.Sleep(100 * time.Millisecond)
	if got := refills.Load(); got != 1 {
		close(gate)
		t.Fatalf("%d refills started, want 1 (single flight)", got)
	}
	close(gate)
}

// Test 21. A refill error after the CAS: R never decreases.
func TestRefillErrorAfterTheCASNeverLowersR(t *testing.T) {
	shrinkLease(t, 8)
	s, _ := openSeedStore(t)
	appendNext(t, s, "first")
	var phases []string
	var mu sync.Mutex
	s.leaseWriteHook = func(phase string, reserved int64) error {
		mu.Lock()
		defer mu.Unlock()
		phases = append(phases, phase)
		if phase == "refill_after" && len(phases) < 6 {
			return errors.New("finalize failed after the CAS")
		}
		return nil
	}
	durable := int64(0)
	check := func(when string) {
		t.Helper()
		row, _ := readLeaseRow(t, s)
		if row.Reserved < durable {
			t.Fatalf("%s: the durable R fell from %d to %d", when, durable, row.Reserved)
		}
		durable = row.Reserved
		if mem := leaseStats(s).ReservedThrough; mem < 0 {
			t.Fatalf("%s: in-memory R %d", when, mem)
		}
	}
	check("seed")
	prevMem := leaseStats(s).ReservedThrough
	for i := 0; i < 40; i++ {
		appendNext(t, s, fmt.Sprintf("n-%d", i))
		check(fmt.Sprintf("after allocation %d", i))
		mem := leaseStats(s).ReservedThrough
		if mem < prevMem {
			t.Fatalf("allocation %d: in-memory R fell from %d to %d", i, prevMem, mem)
		}
		prevMem = mem
	}
	if st := leaseStats(s); st.RefillFailed == 0 {
		t.Fatalf("no refill failure was counted: %+v", st)
	}
}

// Test 22. A seeder lease-append failure: no number is handed out, and the
// state returns to unseeded.
func TestSeederAppendFailureHandsOutNoNumber(t *testing.T) {
	shrinkLease(t, 8)
	leaseSeedingDeadlineFloor = 300 * time.Millisecond
	s, _ := openSeedStore(t)
	var fail atomic.Bool
	fail.Store(true)
	s.leaseWriteHook = func(phase string, reserved int64) error {
		if phase == "seed" && fail.Load() {
			return errors.New("tape outage")
		}
		return nil
	}
	rec := &types.EventRecord{EventID: "doomed", RunID: "run-d", OwnerID: "alice", Timestamp: time.Now().UTC(),
		Kind: types.EventRunProgress, Payload: json.RawMessage(`{}`)}
	if err := s.AppendEvent(leaseCtx(), rec); err == nil {
		t.Fatal("an append with no lease succeeded")
	}
	if rec.StreamSeq != 0 {
		t.Fatalf("the failed allocation took stream_seq %d", rec.StreamSeq)
	}
	st := leaseStats(s)
	if st.State != "unseeded" || st.LastLeaseError == "" {
		t.Fatalf("stats = %+v, want unseeded with a last_lease_error", st)
	}
	if got := s.maxStreamSeqForTest(); got != 0 {
		t.Fatalf("maxStreamSeq = %d after a failed seed, want 0", got)
	}
	fail.Store(false)
	leaseSeedingDeadlineFloor = time.Minute // the deadline above was the subject; recovery is not racing it
	waitLeaseReady(t, s)
	if n := appendNext(t, s, "recovered"); n <= streamSeqExactGap {
		t.Fatalf("recovered stream_seq = %d", n)
	}
}

// Test 23. The folded path (AllocateEventSeq) waits on an exhausted lease and
// fails at the ready deadline; it never takes a number above the lease.
func TestFoldedAllocationWaitsOnAnExhaustedLease(t *testing.T) {
	shrinkLease(t, 4)
	s, _ := openSeedStore(t)
	first := appendNext(t, s, "first")
	row, _ := readLeaseRow(t, s)
	gate := make(chan struct{})
	s.leaseWriteHook = func(phase string, reserved int64) error {
		if phase == "refill" {
			<-gate
		}
		return nil
	}
	for n := first; n < row.Reserved; { // spend the block exactly to R
		n = appendNext(t, s, fmt.Sprintf("s-%d", n))
	}
	done := make(chan *types.EventRecord, 1)
	go func() {
		rec := &types.EventRecord{EventID: "folded", RunID: "run-f", OwnerID: "alice", Kind: types.EventRunProgress}
		if err := s.AllocateEventSeq(leaseCtx(), rec); err != nil {
			t.Errorf("folded allocation: %v", err)
		}
		done <- rec
	}()
	select {
	case rec := <-done:
		t.Fatalf("the folded allocation returned %d with the lease exhausted and the refill held", rec.StreamSeq)
	case <-time.After(150 * time.Millisecond):
	}
	close(gate)
	rec := <-done
	if rec.StreamSeq <= row.Reserved {
		t.Fatalf("folded stream_seq %d is not above the old lease %d", rec.StreamSeq, row.Reserved)
	}
	row2, _ := readLeaseRow(t, s)
	if rec.StreamSeq > row2.Reserved {
		t.Fatalf("folded stream_seq %d above the durable lease %d", rec.StreamSeq, row2.Reserved)
	}
	// And with the refill failing, it fails closed at the ready deadline,
	// taking no number.
	leaseReadyDeadline = 100 * time.Millisecond
	s.leaseWriteHook = func(phase string, reserved int64) error {
		if phase == "refill" {
			return errors.New("tape outage")
		}
		return nil
	}
	for n := rec.StreamSeq; n < row2.Reserved; {
		n = appendNext(t, s, fmt.Sprintf("t-%d", n))
	}
	stuck := &types.EventRecord{EventID: "stuck", RunID: "run-f", OwnerID: "alice", Kind: types.EventRunProgress}
	start := time.Now()
	if err := s.AllocateEventSeq(leaseCtx(), stuck); err == nil {
		t.Fatal("a folded allocation past an unrefillable lease succeeded")
	}
	if stuck.StreamSeq != 0 {
		t.Fatalf("the failed folded allocation took stream_seq %d", stuck.StreamSeq)
	}
	if took := time.Since(start); took < 90*time.Millisecond || took > 5*time.Second {
		t.Fatalf("the ready deadline fired after %v", took)
	}
}

// Test 24. With batching on and a sealer reservation held, the first
// allocation waits for the lease: no deadlock, the deadlines fire, and the
// reservation is given back.
func TestBatchingReservationDoesNotDeadlockTheLeaseWait(t *testing.T) {
	shrinkLease(t, 4)
	leaseSeedingDeadlineFloor = 200 * time.Millisecond
	leaseReadyDeadline = 200 * time.Millisecond
	ctx := leaseCtx()
	h := newTapeHarness(t)
	sl := h.enableBatching(time.Hour)
	var failSeed, failRefill atomic.Bool
	failSeed.Store(true)
	h.store.leaseWriteHook = func(phase string, reserved int64) error {
		switch {
		case phase == "seed" && failSeed.Load():
			return errors.New("tape outage")
		case phase == "refill" && failRefill.Load():
			return errors.New("tape outage")
		}
		return nil
	}
	rec := h.event("t-0", "run-t", "owner-1", types.EventToolResult, eventTableBase)
	done := make(chan error, 1)
	go func() { done <- h.store.AppendEvent(ctx, rec) }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("a batched append succeeded with no lease")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("deadlock: the batched first allocation never returned")
	}
	sl.mu.Lock()
	held := sl.reserved
	sl.mu.Unlock()
	if held != 0 {
		t.Fatalf("the failed append kept %d sealer reservations", held)
	}
	// The seed recovers; then exhaust the block with refills failing: the
	// ready deadline fires, again without holding a reservation.
	failSeed.Store(false)
	failRefill.Store(true)
	waitLeaseReady(t, h.store) // the seeder's backoff step, not a racing deadline
	var firstErr error
	for i := 1; i < 40 && firstErr == nil; i++ {
		r := h.event(fmt.Sprintf("t-%d", i), "run-t", "owner-1", types.EventToolResult, eventTableBase.Add(time.Duration(i)*time.Second))
		start := time.Now()
		firstErr = h.store.AppendEvent(ctx, r)
		if firstErr != nil && time.Since(start) > 3*time.Second {
			t.Fatalf("the ready deadline fired after %v", time.Since(start))
		}
	}
	if firstErr == nil {
		t.Fatal("a refill that never lands did not stop the allocator")
	}
	sl.mu.Lock()
	held = sl.reserved
	sl.mu.Unlock()
	if held != 0 {
		t.Fatalf("the deadline error kept %d sealer reservations", held)
	}
}

// Test 25. The seeder refuses when the local head is not the platform head.
func TestSeederRefusesWhenTheHeadsDiffer(t *testing.T) {
	shrinkLease(t, 8)
	leaseSeedingDeadlineFloor = 200 * time.Millisecond
	ctx := leaseCtx()
	h := newTapeHarness(t)
	var skew atomic.Bool
	h.setPlatformHeadSkew(&skew)
	skew.Store(true) // the platform is ahead of the local projection
	rec := h.event("a-1", "run-a", "owner-1", types.EventRunStarted, eventTableBase)
	err := h.store.AppendEvent(ctx, rec)
	if err == nil {
		t.Fatal("the seeder ran with the local head not at the platform head")
	}
	if rec.StreamSeq != 0 {
		t.Fatalf("a number was taken without a lease: %d", rec.StreamSeq)
	}
	// Between attempts the state is unseeded; during a retry, seeding. Never ready.
	if st := leaseStats(h.store); st.State == "ready" || st.LastLeaseError == "" {
		t.Fatalf("stats = %+v, want not ready with the refusal named", st)
	}
	if _, found, _ := h.store.readLease(ctx); found {
		t.Fatal("a lease was written although the precondition failed")
	}
	skew.Store(false)
	leaseSeedingDeadlineFloor = time.Minute // the deadline above was the subject; recovery is not racing it
	waitLeaseReady(t, h.store)
	ok := h.event("a-2", "run-a", "owner-1", types.EventRunStarted, eventTableBase.Add(time.Second))
	if err := h.store.AppendEvent(ctx, ok); err != nil {
		t.Fatalf("after the heads agree: %v", err)
	}
}

// Test 26. An eager seed started after replay, with an allocation arriving
// mid-seed: it waits and gets seed + 1, not an error.
func TestEagerSeedWithAnAllocationMidSeedWaitsAndGetsSeedPlusOne(t *testing.T) {
	s, path := openSeedStore(t)
	base := time.Now().UTC().Add(-time.Hour)
	for i := int64(1); i <= 5; i++ {
		putStoredEvent(t, s, fmt.Sprintf("e-%d", i), "run", i, base.Add(time.Duration(i)*time.Second))
	}
	s = reopenAt(t, s, path)
	reached, release := blockScan(s)
	t.Cleanup(release)
	s.StartStreamSeqSeed()
	<-reached
	got := make(chan int64, 1)
	errc := make(chan error, 1)
	go func() {
		rec := &types.EventRecord{EventID: "mid", RunID: "run-m", OwnerID: "alice", Timestamp: time.Now().UTC(),
			Kind: types.EventRunProgress, Payload: json.RawMessage(`{}`)}
		if err := s.AppendEvent(leaseCtx(), rec); err != nil {
			errc <- err
			return
		}
		got <- rec.StreamSeq
	}()
	select {
	case n := <-got:
		t.Fatalf("the allocation returned %d mid-seed", n)
	case err := <-errc:
		t.Fatalf("the allocation failed mid-seed instead of waiting: %v", err)
	case <-time.After(150 * time.Millisecond):
	}
	if st := leaseStats(s); st.State != "seeding" {
		t.Fatalf("state mid-seed = %q", st.State)
	}
	release()
	select {
	case n := <-got:
		if want := 5 + streamSeqExactGap + 1; n != want {
			t.Fatalf("stream_seq = %d, want seed + 1 = %d", n, want)
		}
	case err := <-errc:
		t.Fatalf("append: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("the allocation never completed")
	}
	// Starting the seed again is a no-op.
	s.StartStreamSeqSeed()
}

// A lease write never reaches the bus ahead of the CAS: no number is above the
// durable R, ever, under parallel load with small blocks.
func TestNoNumberIsEverAboveTheDurableLease(t *testing.T) {
	shrinkLease(t, 16)
	s, _ := openSeedStore(t)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var all []int64
	for g := 0; g < 6; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 60; i++ {
				n := appendNext(t, s, fmt.Sprintf("p%d-%d", g, i))
				row, _ := readLeaseQuiet(s)
				if n > row.Reserved {
					t.Errorf("stream_seq %d above the durable lease %d", n, row.Reserved)
				}
				mu.Lock()
				all = append(all, n)
				mu.Unlock()
			}
		}(g)
	}
	wg.Wait()
	seen := map[int64]bool{}
	for _, n := range all {
		if seen[n] {
			t.Fatalf("stream_seq %d handed out twice", n)
		}
		seen[n] = true
	}
}

// Reopen (a rematerialize or a rebase in-process) keeps the lease state.
func TestReopenKeepsTheLeaseState(t *testing.T) {
	s, path := openSeedStore(t)
	first := appendNext(t, s, "a")
	before := leaseStats(s)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Reopen(path); err != nil {
		t.Fatal(err)
	}
	after := leaseStats(s)
	if after.State != "ready" || after.ReservedThrough != before.ReservedThrough {
		t.Fatalf("Reopen changed the lease state: %+v -> %+v", before, after)
	}
	if n := appendNext(t, s, "b"); n != first+1 {
		t.Fatalf("stream_seq after Reopen = %d, want %d", n, first+1)
	}
}

// Close stops lease work: no goroutine of the lease outlives the store.
func TestCloseStopsAnInFlightSeederAndRefill(t *testing.T) {
	s, path := openSeedStore(t)
	base := time.Now().UTC().Add(-time.Hour)
	putStoredEvent(t, s, "e-1", "run", 1, base)
	s = reopenAt(t, s, path)
	reached, release := blockScan(s)
	s.StartStreamSeqSeed()
	<-reached
	closed := make(chan error, 1)
	go func() { closed <- s.Close() }()
	time.Sleep(50 * time.Millisecond)
	release()
	select {
	case err := <-closed:
		if err != nil {
			t.Fatalf("close: %v", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("Close did not return while the seeder was mid-scan")
	}
}

// ---- Witness and replay (tests 31, 32) ------------------------------------

// Test 31 (store half). The lease op on the tape carries the fixed timestamps,
// and the object it writes is a pure function of its fields. (The base-versus
// replay witness comparison is in internal/agentcore.)
func TestLeaseOpOnTheTapeCarriesTheFixedTimestamps(t *testing.T) {
	ctx := leaseCtx()
	h := newTapeHarness(t)
	rec := h.event("a-1", "run-a", "owner-1", types.EventRunStarted, eventTableBase)
	if err := h.store.AppendEvent(ctx, rec); err != nil {
		t.Fatal(err)
	}
	var found int
	for _, record := range h.tape.leaseSnapshot() {
		for _, op := range h.tape.batchOps(t, record) {
			if !strings.Contains(op.CanonicalID, "choir.stream_seq_lease") {
				continue
			}
			found++
			var obj objectgraph.Object
			if err := json.Unmarshal(op.Body, &obj); err != nil {
				t.Fatal(err)
			}
			fixed := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
			if !obj.CreatedAt.Equal(fixed) || !obj.UpdatedAt.Equal(fixed) {
				t.Fatalf("lease op timestamps = %v / %v", obj.CreatedAt, obj.UpdatedAt)
			}
			if op.Table != "" {
				t.Fatalf("the lease op is stamped for table %q", op.Table)
			}
		}
	}
	if found != 1 {
		t.Fatalf("the tape carries %d lease ops after one allocation, want 1", found)
	}
}

// Test 32. The lease row survives MigrateAndFenceServingVocabulary after a
// replay with applied rows, with its key and content unchanged.
func TestLeaseRowSurvivesTheVocabularyFence(t *testing.T) {
	ctx := leaseCtx()
	s, _ := openSeedStore(t)
	appendN(t, s, "a", 2)
	before, err := s.ogStore.GetObject(ctx, leaseCanonicalID(t))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.MigrateAndFenceServingVocabulary(ctx, true, nil); err != nil {
		t.Fatal(err)
	}
	after, err := s.ogStore.GetObject(ctx, leaseCanonicalID(t))
	if err != nil {
		t.Fatalf("the lease row did not survive the fence: %v", err)
	}
	if after.CanonicalID != before.CanonicalID || string(after.Body) != string(before.Body) ||
		string(after.Metadata) != string(before.Metadata) || !after.UpdatedAt.Equal(before.UpdatedAt) || after.ContentHash != before.ContentHash {
		t.Fatalf("the fence rewrote the lease row:\n%+v\n%+v", before, after)
	}
}

// ---- Kept from round 1 (tests 34 to 36) ------------------------------------

// Test 34. No read of the event kind on a trusted reopen.
func TestTrustedReopenReadsNoEventKind(t *testing.T) {
	s, path := openSeedStore(t)
	old := time.Now().UTC().Add(-time.Hour)
	for i := 0; i < 40; i++ {
		appendSeedEvent(t, s, fmt.Sprintf("e-%d", i), fmt.Sprintf("run-%d", i%4), old.Add(time.Duration(i)*time.Second))
	}
	seedWakeOutbox(t, s, 20, 10)
	s = reopenAt(t, s, path)
	n := appendSeedEvent(t, s, "after", "run-after", time.Now().UTC()).StreamSeq
	if n <= 40 {
		t.Fatalf("stream_seq %d after the restart", n)
	}
	if reads := eventKindReads(s); len(reads) != 0 {
		t.Fatalf("a trusted reopen read the event kind: %v", reads)
	}
	if calls := pageOpCalls(s); calls != 0 {
		t.Fatalf("a trusted reopen ran %d exact-seed pages", calls)
	}
	if st := leaseStats(s); !st.Trusted || st.SeededFrom != "lease" {
		t.Fatalf("stats = %+v", st)
	}
}

// The cross-check windows are kept (test 36): a first window that holds an
// event is the only one read; a first window of other kinds widens.
func TestCrossCheckKeepsTheWindows(t *testing.T) {
	shrinkSeedWindows(t, 20, 200)
	for name, tc := range map[string]struct {
		others    int
		wantReads int
	}{"first window holds an event": {8, 1}, "widens past other kinds": {60, 2}} {
		t.Run(name, func(t *testing.T) {
			s, path := openSeedStore(t)
			appendOld(t, s, "a", 5)
			seedWakeOutbox(t, s, tc.others, 0)
			s = reopenAt(t, s, path)
			appendNext(t, s, "after")
			if calls := s.EngineMutexStats()["(*DoltStore).ListNewestObjectHeaders"].Calls; int(calls) != tc.wantReads {
				t.Fatalf("the cross-check read %d windows, want %d", calls, tc.wantReads)
			}
			if st := leaseStats(s); st.CrossCheckMax == 0 && tc.others == 8 {
				t.Fatalf("the cross-check saw no event in the first window: %+v", st)
			}
		})
	}
}

// The cross-check in the table half: a stamped table event newer than R makes
// the lease stale.
func TestCrossCheckSeesTheTableNewest(t *testing.T) {
	ctx := leaseCtx()
	h := newTapeHarness(t)
	first := h.event("a-1", "run-a", "owner-1", types.EventRunStarted, eventTableBase)
	if err := h.store.AppendEvent(ctx, first); err != nil {
		t.Fatal(err)
	}
	row, _, _ := h.store.readLease(ctx)
	h.appendStamped(eventTableRecord("t-1", "run-t", "owner-1", "traj-t", types.EventToolResult, 1, row.Reserved+9, eventTableBase.Add(time.Second)))
	h.reopen()
	next := h.event("a-2", "run-a", "owner-1", types.EventRunStarted, eventTableBase.Add(time.Minute))
	if err := h.store.AppendEvent(ctx, next); err != nil {
		t.Fatal(err)
	}
	if next.StreamSeq <= row.Reserved+9 {
		t.Fatalf("stream_seq %d reissued the table's %d", next.StreamSeq, row.Reserved+9)
	}
	if st := leaseStats(h.store); st.UntrustReason != "violated" {
		t.Fatalf("stats = %+v, want violated", st)
	}
}

// Bunch 169, medium-low: the seeder's retry has a backoff and a counter. A
// failing seed rescans the event history; an allocation arriving every few
// milliseconds must not start a rescan each time, and the operator must see
// the failures without SQL. Choice: backoff (doubling per consecutive
// failure, capped) plus a counter, surfaced in last_lease_error and
// seed_failed. The head-delta retry (re-reading only rows since the scan's
// head) was not built: it needs a classification of every op kind since the
// scan's head, and an error in it silently reuses a stale scan.
func TestSeederRetriesWithABackoffAndACounter(t *testing.T) {
	shrinkLease(t, 8)
	leaseRefillBackoff = 60 * time.Millisecond
	leaseSeedingDeadlineFloor = time.Minute // an upper bound only: the test counts attempts and gaps
	s, _ := openSeedStore(t)
	var mu sync.Mutex
	var attempts []time.Time
	s.leaseWriteHook = func(phase string, reserved int64) error {
		if phase != "seed" {
			return nil
		}
		mu.Lock()
		defer mu.Unlock()
		attempts = append(attempts, time.Now())
		if len(attempts) <= 3 {
			return errors.New("tape outage")
		}
		return nil
	}
	n := appendNext(t, s, "after-failures")
	if n <= streamSeqExactGap {
		t.Fatalf("stream_seq = %d", n)
	}
	st := leaseStats(s)
	if st.SeedFailed != 3 {
		t.Fatalf("seed_failed = %d, want 3: %+v", st.SeedFailed, st)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(attempts) != 4 {
		t.Fatalf("the seed ran %d times, want 3 failures and 1 success", len(attempts))
	}
	for i := 1; i < len(attempts); i++ {
		want := leaseRefillBackoff << (i - 1) // 60ms, 120ms, 240ms
		if gap := attempts[i].Sub(attempts[i-1]); gap < want {
			t.Fatalf("retry %d came %v after the failure, before its %v backoff", i, gap, want)
		}
	}
	if st.State != "ready" {
		t.Fatalf("state = %q", st.State)
	}
}

func TestSeederFailureNamesTheAttemptInLastLeaseError(t *testing.T) {
	shrinkLease(t, 8)
	leaseSeedingDeadlineFloor = 150 * time.Millisecond
	s, _ := openSeedStore(t)
	s.leaseWriteHook = func(phase string, reserved int64) error { return errors.New("tape outage") }
	rec := &types.EventRecord{EventID: "x", RunID: "run-x", OwnerID: "alice", Kind: types.EventRunProgress, Payload: json.RawMessage(`{}`)}
	if err := s.AppendEvent(leaseCtx(), rec); err == nil {
		t.Fatal("append succeeded with no lease")
	}
	st := leaseStats(s)
	if !strings.Contains(st.LastLeaseError, "seed attempt ") || !strings.Contains(st.LastLeaseError, "tape outage") {
		t.Fatalf("last_lease_error = %q", st.LastLeaseError)
	}
	if st.SeedFailed < 1 {
		t.Fatalf("seed_failed = %d", st.SeedFailed)
	}
}

// Bunch 169, low: a tombstoned lease row makes the next boot untrusted; the
// seeder logs it and counts it.
func TestTombstonedLeaseRowIsUntrustedLoggedAndCounted(t *testing.T) {
	s, path := openSeedStore(t)
	appendN(t, s, "a", 3)
	row, _ := readLeaseRow(t, s)
	obj, err := s.ogStore.GetObject(leaseCtx(), leaseCanonicalID(t))
	if err != nil {
		t.Fatal(err)
	}
	obj.Tombstone = true // a tombstone passes the fence, as in CheckWrite
	if err := s.ogStore.PutObject(leaseCtx(), obj); err != nil {
		t.Fatal(err)
	}
	s = reopenAt(t, s, path)
	buf := captureLog(t)
	n := appendNext(t, s, "after")
	st := leaseStats(s)
	if st.Trusted || st.SeededFrom != "exact" || st.LeaseTombstoned != 1 {
		t.Fatalf("stats = %+v, want untrusted, exact, lease_tombstoned=1", st)
	}
	if !strings.Contains(buf.String(), "tombstoned") {
		t.Fatalf("a tombstoned lease was not logged:\n%s", buf.String())
	}
	if n <= row.Reserved-streamSeqLeaseBlock {
		t.Fatalf("stream_seq %d is not above the stored numbers", n)
	}
}

// DeepSeek bunch 175, medium-low: a Close must wake every waiter promptly. A
// waiter parked on the lease selected only on the state-change channel, the
// timer and its own ctx; Close cancelled the work without closing anything the
// waiter watches, so it slept to the seeding deadline (10 minutes) unless
// Close could take eventMu. The waiter now also watches a closing channel
// Close closes without a lock.
func TestCloseWakesAParkedWaiterBeforeTheSeedingDeadline(t *testing.T) {
	shrinkLease(t, 8)
	leaseSeedingDeadlineFloor = time.Minute
	s, path := openSeedStore(t)
	putStoredEvent(t, s, "e-1", "run", 1, time.Now().UTC().Add(-time.Hour))
	s = reopenAt(t, s, path)
	reached, release := blockScan(s)
	t.Cleanup(release)
	s.StartStreamSeqSeed()
	<-reached // the seeder is mid-scan; allocations park
	waited := make(chan error, 1)
	started := time.Now()
	go func() {
		rec := &types.EventRecord{EventID: "parked", RunID: "run-p", OwnerID: "alice", Timestamp: time.Now().UTC(),
			Kind: types.EventRunProgress, Payload: json.RawMessage(`{}`)}
		waited <- s.AppendEvent(leaseCtx(), rec)
	}()
	time.Sleep(100 * time.Millisecond) // let it park
	closed := make(chan error, 1)
	go func() { closed <- s.Close() }()
	select {
	case err := <-waited:
		if err == nil || !errors.Is(err, errLeaseClosing) {
			t.Fatalf("the parked waiter returned %v, want the closing error", err)
		}
		if took := time.Since(started); took > 10*time.Second {
			t.Fatalf("the parked waiter woke after %v, not promptly", took)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("a parked waiter slept on after Close began")
	}
	release() // the seeder is still mid-scan; let Close drain it
	select {
	case err := <-closed:
		if err != nil {
			t.Fatalf("close: %v", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("Close did not return")
	}
}

// Devin bunch 175, low: an event row the exact seed cannot decode is counted
// and surfaced, not silently read as zero.
func TestExactSeedCountsUndecodableEventRows(t *testing.T) {
	s, path := openSeedStore(t)
	at := time.Now().UTC().Add(-time.Hour)
	putStoredEvent(t, s, "good", "run", 77, at)
	rec := eventTableRecord("bad", "run-x", "alice", "traj-x", types.EventRunProgress, 1, 5, at)
	obj, err := buildEventObject(rec, at)
	if err != nil {
		t.Fatal(err)
	}
	obj.Metadata = []byte(`{"event_id":"bad"}`) // no stream_seq in metadata
	obj.Body = []byte(`"not an event record"`)  // and a body that is not an event
	obj.ContentHash = objectgraph.ContentHash(obj.ObjectKind, obj.Body, obj.Metadata)
	if err := s.ogStore.PutBatch(leaseCtx(), objectgraph.Batch{Objects: []objectgraph.Object{obj}}); err != nil {
		t.Fatal(err)
	}
	s = reopenAt(t, s, path)
	buf := captureLog(t)
	n := appendNext(t, s, "after")
	if want := int64(77) + streamSeqExactGap + 1; n != want {
		t.Fatalf("stream_seq = %d, want %d", n, want)
	}
	st := leaseStats(s)
	if st.SeedUndecodableRows != 1 {
		t.Fatalf("seed_undecodable_rows = %d, want 1: %+v", st.SeedUndecodableRows, st)
	}
	if !strings.Contains(buf.String(), "undecodable") {
		t.Fatalf("the skip was not logged:\n%s", buf.String())
	}
}

// Devin bunch 175, nit: marker_present agrees with the snapshot: it is
// published with the state, so it is true before the seed and false after an
// exact seed cleared it.
func TestMarkerPresentIsPublishedWithTheSnapshot(t *testing.T) {
	s, path := openSeedStore(t)
	appendN(t, s, "a", 2)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if err := storeschema.WriteLeaseUntrusted(deriveTextureWorkspacePath(path), "install"); err != nil {
		t.Fatal(err)
	}
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if st := leaseStats(s); !st.MarkerPresent || st.State != "unseeded" {
		t.Fatalf("before the seed: %+v, want a marker and unseeded", st)
	}
	appendNext(t, s, "after")
	if st := leaseStats(s); st.MarkerPresent || st.State != "ready" {
		t.Fatalf("after the exact seed: %+v, want no marker and ready", st)
	}
}
