package platform

import (
	"archive/tar"
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/klauspost/compress/zstd"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// Tests for the tape sweep. Each pins a failure mode written before the code
// in docs/problems/tape-sweeps-destroyed-and-orphans-2026-10-20.md (FM1-11).

const tapeAged = 30 * 24 * time.Hour

type fakeRegistry struct {
	owned     map[string]bool // lookup finds an ownership record
	listed    map[string]bool // present in the registry list
	listErr   error
	lookupErr error
}

func (r *fakeRegistry) LookupComputer(_ context.Context, id string) (bool, error) {
	if r.lookupErr != nil {
		return false, r.lookupErr
	}
	return r.owned[id], nil
}

func (r *fakeRegistry) ListComputerIDs(context.Context) ([]string, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}
	var out []string
	for id := range r.listed {
		out = append(out, id)
	}
	return out, nil
}

type tapeFixture struct {
	t     *testing.T
	store *Store
	svc   *Service
	root  string
	ctx   context.Context
	reg   *fakeRegistry
}

type tapeSeq struct {
	event, eventPin, payload, payloadPin string
}

type tapeChain struct {
	id   string
	seqs []tapeSeq
}

func newTapeFixture(t *testing.T) *tapeFixture {
	t.Helper()
	store, root := openTestPlatformStore(t)
	ensureGCRouteTables(t, store)
	artifactsRoot := filepath.Join(root, "artifacts")
	svc := NewService(store, artifactsRoot, "")
	reg := &fakeRegistry{owned: map[string]bool{}, listed: map[string]bool{}}
	svc.SetTapeSweepPolicy(TapeSweepPolicy{Mode: ArtifactGCModeActive})
	svc.SetComputerRegistry(reg)
	svc.tapeSweep.beforeDelete = nil
	return &tapeFixture{t: t, store: store, svc: svc, root: artifactsRoot, ctx: context.Background(), reg: reg}
}

func (f *tapeFixture) live(id string) {
	f.reg.owned[id] = true
	f.reg.listed[id] = true
}

func (f *tapeFixture) path(rel string) string { return filepath.Join(f.root, filepath.FromSlash(rel)) }

// put writes a content-addressed tape file aged age and returns its digest.
func (f *tapeFixture) put(ns string, data []byte, age time.Duration) string {
	f.t.Helper()
	digest := computerevent.DigestBytes(data)
	p := f.path(tapeSweepRel(ns, digest))
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(p, data, 0o640); err != nil {
		f.t.Fatal(err)
	}
	when := time.Now().Add(-age)
	if err := os.Chtimes(p, when, when); err != nil {
		f.t.Fatal(err)
	}
	return digest
}

func (f *tapeFixture) exists(ns, digest string) bool {
	_, err := os.Stat(f.path(tapeSweepRel(ns, digest)))
	return err == nil
}

// appendSeq writes one chain row (event, payload, two pin receipts) and
// moves the head. payload nil means a per-sequence payload.
func (f *tapeFixture) appendSeq(c *tapeChain, payload []byte, age time.Duration) tapeSeq {
	f.t.Helper()
	seq := uint64(len(c.seqs) + 1)
	if payload == nil {
		payload = []byte(fmt.Sprintf("payload %s %d", c.id, seq))
	}
	var s tapeSeq
	s.payload = f.put(tapeNSPayload, payload, age)
	s.payloadPin = f.put(tapeNSReceipt, []byte(fmt.Sprintf(`{"receipt_kind":"PinReceipt","computer_id":%q,"artifact_digest":%q,"pin_namespace":"computer-event-payload"}`, c.id, s.payload)), age)
	s.event = f.put(tapeNSEvent, []byte(fmt.Sprintf(`{"computer_id":%q,"sequence":%d,"output_artifact_refs":["artifact:sha256:%s"],"payload_commitment":%q}`, c.id, seq, s.payload, s.payload)), age)
	s.eventPin = f.put(tapeNSReceipt, []byte(fmt.Sprintf(`{"receipt_kind":"PinReceipt","computer_id":%q,"artifact_digest":%q,"pin_namespace":"computer-event"}`, c.id, s.event)), age)
	when := time.Now().Add(-age).UTC()
	insertTestAppendReceipt(f.t, f.store.db, c.id, fmt.Sprintf("k-%d", seq), platformTestDigest('c'), seq, "verification_recorded", s.event, s.eventPin, []string{s.payloadPin}, when)
	if _, err := f.store.db.Exec(`INSERT INTO computer_event_heads (computer_id, sequence, canonical_event_head, desired_event_head, effective_event_head, desired_state_commitment, effective_state_commitment, pending_transition_ref, reducer_version, credential_revocation_epoch, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT (computer_id) DO UPDATE SET sequence=excluded.sequence, canonical_event_head=excluded.canonical_event_head, updated_at=excluded.updated_at`,
		c.id, seq, s.event, s.event, s.event, platformTestDigest('f'), platformTestDigest('f'), nil, computerevent.ReducerVersionV1, 0, when, when); err != nil {
		f.t.Fatal(err)
	}
	c.seqs = append(c.seqs, s)
	return s
}

func (f *tapeFixture) chain(id string, n int, age time.Duration) *tapeChain {
	c := &tapeChain{id: id}
	for i := 0; i < n; i++ {
		f.appendSeq(c, nil, age)
	}
	return c
}

func (c *tapeChain) files() map[string][]string {
	out := map[string][]string{}
	for _, s := range c.seqs {
		out[tapeNSEvent] = append(out[tapeNSEvent], s.event)
		out[tapeNSReceipt] = append(out[tapeNSReceipt], s.eventPin, s.payloadPin)
		out[tapeNSPayload] = append(out[tapeNSPayload], s.payload)
	}
	return out
}

func (f *tapeFixture) allExist(c *tapeChain, want bool) {
	f.t.Helper()
	for ns, digests := range c.files() {
		for _, d := range digests {
			if f.exists(ns, d) != want {
				f.t.Fatalf("%s %s/%s exists=%v, want %v", c.id, ns, d, !want, want)
			}
		}
	}
}

func (f *tapeFixture) dryRun(req TapeSweepRequest) TapeSweepReport {
	f.t.Helper()
	req.Action = TapeSweepActionDryRun
	report, err := f.svc.RunTapeSweep(f.ctx, req)
	if err != nil {
		f.t.Fatalf("dry-run: %v", err)
	}
	return report
}

func (f *tapeFixture) apply(dry TapeSweepReport, req TapeSweepRequest) (TapeSweepReport, error) {
	f.t.Helper()
	req.Action = TapeSweepActionApply
	if req.RunID == "" {
		req.RunID = dry.RunID
	}
	if req.ManifestSHA256 == "" {
		req.ManifestSHA256 = dry.ManifestSHA256
	}
	req.Pause = "0s"
	return f.svc.RunTapeSweep(f.ctx, req)
}

func (f *tapeFixture) manifestItems(report TapeSweepReport) []tapeSweepItem {
	f.t.Helper()
	file, err := os.Open(report.ManifestPath)
	if err != nil {
		f.t.Fatal(err)
	}
	defer file.Close()
	var items []tapeSweepItem
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1<<20), 1<<24)
	for scanner.Scan() {
		var it tapeSweepItem
		if err := json.Unmarshal(scanner.Bytes(), &it); err != nil {
			f.t.Fatal(err)
		}
		if it.Kind == "item" {
			items = append(items, it)
		}
	}
	return items
}

func listedDigests(items []tapeSweepItem, sweep string) map[string]bool {
	out := map[string]bool{}
	for _, it := range items {
		if it.Sweep == sweep {
			out[it.Digest] = true
		}
	}
	return out
}

// insertClosure records a code closure (and a route slot for routedComputer
// when non-empty) whose JSON names digest.
func (f *tapeFixture) insertClosure(codeRef, digest, routedComputer string) {
	f.t.Helper()
	now := time.Now().UTC()
	if _, err := f.store.db.Exec(`INSERT INTO computer_version_code_closures (code_ref, source_commit, closure_json, created_at) VALUES (?,?,?,?)`, codeRef, "abc", `{"artifacts":[{"uri":"x","sha256":"`+digest+`"}]}`, now); err != nil {
		f.t.Fatal(err)
	}
	if routedComputer == "" {
		return
	}
	if _, err := f.store.db.Exec(`INSERT INTO computer_version_artifact_programs (artifact_program_ref, program_json, created_at) VALUES (?,?,?)`, "prog-"+codeRef, `{}`, now); err != nil {
		f.t.Fatal(err)
	}
	if _, err := f.store.db.Exec(`INSERT INTO computer_version_route_slots (route_slot_id, current_code_ref, current_artifact_program_ref, generation, latest_receipt_id, updated_at) VALUES (?,?,?,?,?,?)`, "computer:owner-1:"+routedComputer, codeRef, "prog-"+codeRef, 1, "rcpt", now); err != nil {
		f.t.Fatal(err)
	}
}

// FM1: a referenced file is never listed or deleted: a live chain's files, a
// payload a destroyed computer shares with a live one, and a payload only a
// code closure names. True orphans are listed and deleted.
func TestTapeSweepKeepsEveryReferencedFile(t *testing.T) {
	f := newTapeFixture(t)
	f.live("live-1")
	live := f.chain("live-1", 3, tapeAged)
	dead := f.chain("dead-1", 2, tapeAged)
	shared := []byte("shared observation")
	f.appendSeq(live, shared, tapeAged)
	deadShared := f.appendSeq(dead, shared, tapeAged)
	closureNamed := f.put(tapeNSPayload, []byte("named by a closure"), tapeAged)
	f.insertClosure("code-1", closureNamed, "")
	orphanPayload := f.put(tapeNSPayload, []byte("orphan payload"), tapeAged)
	orphanReceipt := f.put(tapeNSReceipt, []byte(`{"computer_id":"live-1","artifact_digest":"`+orphanPayload+`"}`), tapeAged)
	orphanEvent := f.put(tapeNSEvent, []byte(`{"computer_id":"live-1","sequence":9}`), tapeAged)

	dry := f.dryRun(TapeSweepRequest{AcceptUnrecordedForkLineage: true})
	items := f.manifestItems(dry)
	orphans := listedDigests(items, TapeSweepOrphans)
	if len(orphans) != 3 || !orphans[orphanPayload] || !orphans[orphanReceipt] || !orphans[orphanEvent] {
		t.Fatalf("orphans listed = %v, want exactly the three orphans", orphans)
	}
	destroyed := listedDigests(items, TapeSweepDestroyed)
	if destroyed[deadShared.payload] || destroyed[closureNamed] {
		t.Fatal("a payload a live record names was listed for deletion")
	}
	for ns, digests := range live.files() {
		for _, d := range digests {
			if orphans[d] || destroyed[d] {
				t.Fatalf("live file %s/%s listed", ns, d)
			}
		}
	}
	if dry.DestroyedListed != 1 || dry.DestroyedProofs[0].SharedPayloadsKept != 1 {
		t.Fatalf("destroyed listed=%d proofs=%+v", dry.DestroyedListed, dry.DestroyedProofs)
	}

	if _, err := f.apply(dry, TapeSweepRequest{AcceptUnrecordedForkLineage: true}); err != nil {
		t.Fatal(err)
	}
	f.allExist(live, true)
	if !f.exists(tapeNSPayload, closureNamed) || !f.exists(tapeNSPayload, deadShared.payload) {
		t.Fatal("a referenced payload was deleted")
	}
	for d := range orphans {
		for _, ns := range tapeNamespaces {
			if f.exists(ns, d) {
				t.Fatalf("orphan %s/%s survived apply", ns, d)
			}
		}
	}
	for _, s := range dead.seqs {
		if f.exists(tapeNSEvent, s.event) || f.exists(tapeNSReceipt, s.eventPin) || f.exists(tapeNSReceipt, s.payloadPin) {
			t.Fatal("destroyed computer's event or receipt survived apply")
		}
	}
}

// FM2: a partial reference set is never taken for complete.
func TestTapeSweepRefusesPartialReferenceSet(t *testing.T) {
	t.Run("unreadable event refuses the payload namespace", func(t *testing.T) {
		f := newTapeFixture(t)
		f.live("live-1")
		live := f.chain("live-1", 2, tapeAged)
		f.put(tapeNSPayload, []byte("orphan payload"), tapeAged)
		if err := os.Remove(f.path(tapeSweepRel(tapeNSEvent, live.seqs[0].event))); err != nil {
			t.Fatal(err)
		}
		dry := f.dryRun(TapeSweepRequest{Sweeps: []string{TapeSweepOrphans}})
		if dry.Orphans[tapeNSPayload].Refused == "" {
			t.Fatalf("payload namespace swept with an unreadable event: %+v", dry.Orphans)
		}
		for _, it := range f.manifestItems(dry) {
			if it.Namespace == tapeNSPayload {
				t.Fatalf("payload %s listed with a partial reference set", it.Digest)
			}
		}
	})
	t.Run("registry failure refuses the destroyed sweep", func(t *testing.T) {
		f := newTapeFixture(t)
		dead := f.chain("dead-1", 2, tapeAged)
		f.reg.listErr = errors.New("vmctl down")
		dry := f.dryRun(TapeSweepRequest{AcceptUnrecordedForkLineage: true, Sweeps: []string{TapeSweepDestroyed}})
		if dry.DestroyedRefused == "" || len(listedDigests(f.manifestItems(dry), TapeSweepDestroyed)) != 0 {
			t.Fatalf("destroyed sweep ran on a failed registry list: %+v", dry)
		}
		f.reg.listErr = nil
		f.reg.lookupErr = errors.New("vmctl lookup down")
		dry = f.dryRun(TapeSweepRequest{AcceptUnrecordedForkLineage: true, Sweeps: []string{TapeSweepDestroyed}})
		if dry.DestroyedRefused == "" || len(listedDigests(f.manifestItems(dry), TapeSweepDestroyed)) != 0 {
			t.Fatal("destroyed sweep ran on a failed registry lookup")
		}
		f.svc.SetComputerRegistry(nil)
		dry = f.dryRun(TapeSweepRequest{AcceptUnrecordedForkLineage: true, Sweeps: []string{TapeSweepDestroyed}})
		if dry.DestroyedRefused == "" {
			t.Fatal("destroyed sweep ran without a registry")
		}
		f.allExist(dead, true)
	})
	t.Run("missing extra-reference table refuses both sweeps", func(t *testing.T) {
		f := newTapeFixture(t)
		f.chain("dead-1", 1, tapeAged)
		f.put(tapeNSPayload, []byte("orphan payload"), tapeAged)
		if _, err := f.store.db.Exec(`DROP TABLE computer_route_projection_certificates`); err != nil {
			t.Fatal(err)
		}
		dry := f.dryRun(TapeSweepRequest{AcceptUnrecordedForkLineage: true})
		if dry.OrphansRefused == "" || dry.DestroyedRefused == "" || len(f.manifestItems(dry)) != 0 {
			t.Fatalf("swept with an unknown extra reference set: %+v", dry)
		}
	})
}

// FM3: in-flight pins are never orphans: younger than grace is kept, and a
// grace request under 24h is clamped up.
func TestTapeSweepGraceFloor(t *testing.T) {
	f := newTapeFixture(t)
	young := f.put(tapeNSPayload, []byte("pinned an hour ago"), time.Hour)
	twoHours := f.put(tapeNSReceipt, []byte(`{"pinned":"two hours ago"}`), 2*time.Hour)
	old := f.put(tapeNSPayload, []byte("old orphan"), tapeAged)
	dry := f.dryRun(TapeSweepRequest{Grace: "1h", Sweeps: []string{TapeSweepOrphans}})
	if dry.Grace != TapeSweepMinGrace.String() {
		t.Fatalf("grace = %s, want clamped to %s", dry.Grace, TapeSweepMinGrace)
	}
	listed := listedDigests(f.manifestItems(dry), TapeSweepOrphans)
	if listed[young] || listed[twoHours] || !listed[old] {
		t.Fatalf("listed = %v", listed)
	}
	if dry.Orphans[tapeNSPayload].InGrace != 1 || dry.Orphans[tapeNSReceipt].InGrace != 1 {
		t.Fatalf("in-grace counts = %+v", dry.Orphans)
	}
}

// FM4: an orphan referenced again between dry-run and apply is kept: by a
// new append (delta reference) or by a re-pin (reuse refreshes mtime).
func TestTapeSweepKeepsOrphanReReferencedBeforeApply(t *testing.T) {
	f := newTapeFixture(t)
	f.live("live-1")
	live := f.chain("live-1", 1, tapeAged)
	adoptedBytes := []byte("orphan later adopted")
	adopted := f.put(tapeNSPayload, adoptedBytes, tapeAged)
	repinnedBytes := []byte("orphan later re-pinned")
	repinned := f.put(tapeNSPayload, repinnedBytes, tapeAged)
	gone := f.put(tapeNSPayload, []byte("orphan for good"), tapeAged)
	dry := f.dryRun(TapeSweepRequest{Sweeps: []string{TapeSweepOrphans}})
	if n := len(listedDigests(f.manifestItems(dry), TapeSweepOrphans)); n != 3 {
		t.Fatalf("listed %d orphans, want 3", n)
	}
	f.appendSeq(live, adoptedBytes, tapeAged)
	artifacts := &EventArtifactService{platform: f.svc}
	if err := artifacts.writeImmutable(tapeSweepRel(tapeNSPayload, repinned), repinnedBytes); err != nil {
		t.Fatal(err)
	}
	report, err := f.apply(dry, TapeSweepRequest{Sweeps: []string{TapeSweepOrphans}})
	if err != nil {
		t.Fatal(err)
	}
	if !f.exists(tapeNSPayload, adopted) || !f.exists(tapeNSPayload, repinned) || f.exists(tapeNSPayload, gone) {
		t.Fatal("apply deleted a re-referenced orphan or kept the true one")
	}
	if report.Apply.KeptReferenced != 1 || report.Apply.KeptRefreshed != 1 || report.Apply.Deleted != 1 {
		t.Fatalf("apply counts = %+v", report.Apply)
	}
}

// FM5: a computer that is not destroyed is never listed, and one that comes
// back to life between dry-run and apply is skipped with no tombstone left.
func TestTapeSweepNeverDeletesALiveComputer(t *testing.T) {
	f := newTapeFixture(t)
	f.reg.listed["listed-1"] = true
	listed := f.chain("listed-1", 1, tapeAged)
	f.reg.owned["owned-1"] = true
	owned := f.chain("owned-1", 1, tapeAged)
	routed := f.chain("routed-1", 1, tapeAged)
	f.insertClosure("code-r", platformTestDigest('9'), "routed-1")
	recent := f.chain("recent-1", 1, 2*24*time.Hour)
	revived := f.chain("revived-1", 1, tapeAged)
	appended := f.chain("appended-1", 1, tapeAged)
	dry := f.dryRun(TapeSweepRequest{AcceptUnrecordedForkLineage: true, Sweeps: []string{TapeSweepDestroyed}})
	got := map[string]bool{}
	for _, it := range f.manifestItems(dry) {
		got[it.Computer] = true
	}
	if len(got) != 2 || !got["revived-1"] || !got["appended-1"] {
		t.Fatalf("listed computers = %v, want only revived-1 and appended-1", got)
	}
	f.reg.owned["revived-1"] = true
	f.appendSeq(appended, nil, 0)
	report, err := f.apply(dry, TapeSweepRequest{AcceptUnrecordedForkLineage: true, Sweeps: []string{TapeSweepDestroyed}})
	if err != nil {
		t.Fatal(err)
	}
	if report.Apply.Deleted != 0 || report.Apply.ComputersSkipped != 2 {
		t.Fatalf("apply counts = %+v", report.Apply)
	}
	for _, c := range []*tapeChain{listed, owned, routed, recent, revived, appended} {
		f.allExist(c, true)
		if _, ok, _ := f.store.TapeTombstone(f.ctx, c.id); ok {
			t.Fatalf("%s left tombstoned", c.id)
		}
	}
}

// FM6: with no fork-lineage record, nothing of a destroyed computer is
// listed or deleted unless the dry-run and the apply both acknowledge it.
func TestTapeSweepForkLineageFailsClosed(t *testing.T) {
	f := newTapeFixture(t)
	dead := f.chain("dead-1", 2, tapeAged)
	dry := f.dryRun(TapeSweepRequest{})
	if dry.DestroyedComputers != 1 || dry.DestroyedBlockedByLineage != 1 || dry.DestroyedListed != 0 {
		t.Fatalf("report = %+v", dry)
	}
	if n := len(listedDigests(f.manifestItems(dry), TapeSweepDestroyed)); n != 0 {
		t.Fatalf("%d destroyed files listed without the acknowledgement", n)
	}
	acked := f.dryRun(TapeSweepRequest{AcceptUnrecordedForkLineage: true})
	if _, err := f.apply(acked, TapeSweepRequest{}); err == nil {
		t.Fatal("apply without the acknowledgement ran an acknowledged manifest")
	}
	f.allExist(dead, true)
}

// FM7: apply runs only the reviewed manifest, only in active mode.
func TestTapeSweepApplyNeedsTheReviewedManifest(t *testing.T) {
	f := newTapeFixture(t)
	orphan := f.put(tapeNSPayload, []byte("orphan payload"), tapeAged)
	dry := f.dryRun(TapeSweepRequest{Sweeps: []string{TapeSweepOrphans}})
	if _, err := f.apply(dry, TapeSweepRequest{ManifestSHA256: platformTestDigest('0')}); err == nil {
		t.Fatal("apply ran with a wrong manifest hash")
	}
	if _, err := f.apply(dry, TapeSweepRequest{RunID: "ts-20260101T000000Z-00000000"}); err == nil {
		t.Fatal("apply ran with a wrong run id")
	}
	file, err := os.OpenFile(dry.ManifestPath, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	extra := f.put(tapeNSPayload, []byte("smuggled"), tapeAged)
	fmt.Fprintf(file, `{"kind":"item","sweep":"orphans","ns":"%s","digest":"%s","path":"%s"}`+"\n", tapeNSPayload, extra, tapeSweepRel(tapeNSPayload, extra))
	file.Close()
	if _, err := f.apply(dry, TapeSweepRequest{}); err == nil {
		t.Fatal("apply ran a tampered manifest")
	}
	f.svc.SetTapeSweepPolicy(TapeSweepPolicy{Mode: ArtifactGCModeDryRun})
	if _, err := f.apply(dry, TapeSweepRequest{}); !errors.Is(err, errTapeSweepActiveNotConfigured) {
		t.Fatalf("apply in dry-run mode: %v", err)
	}
	if !f.exists(tapeNSPayload, orphan) || !f.exists(tapeNSPayload, extra) {
		t.Fatal("a refused apply deleted a file")
	}
}

// FM8: nothing is deleted before a verified archive; a failed archive
// removes the tombstone; archive entries re-hash to their names and restore.
func TestTapeSweepArchivesBeforeDelete(t *testing.T) {
	t.Run("archive failure deletes nothing", func(t *testing.T) {
		f := newTapeFixture(t)
		dead := f.chain("dead-1", 2, tapeAged)
		blocker := filepath.Join(t.TempDir(), "not-a-dir")
		if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		f.svc.SetTapeSweepPolicy(TapeSweepPolicy{Mode: ArtifactGCModeActive, ArchiveDir: blocker})
		dry := f.dryRun(TapeSweepRequest{AcceptUnrecordedForkLineage: true, Sweeps: []string{TapeSweepDestroyed}})
		report, err := f.apply(dry, TapeSweepRequest{AcceptUnrecordedForkLineage: true, Sweeps: []string{TapeSweepDestroyed}})
		if err != nil {
			t.Fatal(err)
		}
		if report.Apply.Deleted != 0 || report.Apply.Errors == 0 {
			t.Fatalf("apply counts = %+v", report.Apply)
		}
		f.allExist(dead, true)
		if _, ok, _ := f.store.TapeTombstone(f.ctx, "dead-1"); ok {
			t.Fatal("tombstone left after a failed archive")
		}
	})
	t.Run("archive verifies and restores", func(t *testing.T) {
		f := newTapeFixture(t)
		dead := f.chain("dead-1", 3, tapeAged)
		original := map[string][]byte{}
		for ns, digests := range dead.files() {
			for _, d := range digests {
				raw, err := os.ReadFile(f.path(tapeSweepRel(ns, d)))
				if err != nil {
					t.Fatal(err)
				}
				original[tapeSweepRel(ns, d)] = raw
			}
		}
		dry := f.dryRun(TapeSweepRequest{AcceptUnrecordedForkLineage: true, Sweeps: []string{TapeSweepDestroyed}})
		report, err := f.apply(dry, TapeSweepRequest{AcceptUnrecordedForkLineage: true, Sweeps: []string{TapeSweepDestroyed}})
		if err != nil {
			t.Fatal(err)
		}
		if len(report.Apply.Archives) != 1 {
			t.Fatalf("archives = %v", report.Apply.Archives)
		}
		f.allExist(dead, false)
		archive := report.Apply.Archives[0]
		var sc TapeArchiveSidecar
		raw, err := os.ReadFile(archive + ".json")
		if err != nil || json.Unmarshal(raw, &sc) != nil {
			t.Fatalf("sidecar: %v", err)
		}
		if err := verifyTapeArchive(archive, sc); err != nil {
			t.Fatal(err)
		}
		tomb, ok, err := f.store.TapeTombstone(f.ctx, "dead-1")
		if err != nil || !ok || tomb.Status != tapeTombstoneDeleted || tomb.ArchiveSHA256 != sc.ArchiveSHA256 || tomb.FilesDeleted != int64(len(original)) {
			t.Fatalf("tombstone = %+v ok=%v err=%v", tomb, ok, err)
		}
		// Restore: extract every entry and compare with the original bytes.
		file, err := os.Open(archive)
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		zr, err := zstd.NewReader(file)
		if err != nil {
			t.Fatal(err)
		}
		defer zr.Close()
		tr := tar.NewReader(zr)
		restored := 0
		for {
			hdr, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
			body, _ := io.ReadAll(tr)
			if hdr.Name == tapeArchiveManifestName || strings.HasPrefix(hdr.Name, "rows/") {
				continue
			}
			if string(original[hdr.Name]) != string(body) {
				t.Fatalf("archived %s differs from the original", hdr.Name)
			}
			restored++
		}
		if restored != len(original) {
			t.Fatalf("restored %d of %d files", restored, len(original))
		}
	})
}

// FM9: readers of a tombstoned computer get the explicit archived/deleted
// error, not a chain gap: replay, payload fetch, and the head CAS.
func TestTapeSweepTombstoneRefusesReadersAndAppends(t *testing.T) {
	store, root := openTestPlatformStore(t)
	svc := NewService(store, filepath.Join(root, "artifacts"), filepath.Join(root, "platform-signing.key"))
	cas, artifacts, _, err := svc.ComputerEventRuntime()
	if err != nil {
		t.Fatal(err)
	}
	insertReplayTestChain(t, store, svc, "dead-1", []uint64{1, 2}, 2)
	if _, err := artifacts.EventsPage(context.Background(), "dead-1", 0, 10); err != nil {
		t.Fatalf("replay before the tombstone: %v", err)
	}
	if err := store.putTapeTombstone(context.Background(), TapeTombstone{ComputerID: "dead-1", Status: tapeTombstoneDeleted, HeadSequence: 2, CanonicalEventHead: platformTestDigest('1'), ManifestRun: "ts-x", ManifestSHA256: platformTestDigest('2'), ArchivePath: "/archive/dead-1.tar.zst", ProofJSON: "{}"}); err != nil {
		t.Fatal(err)
	}
	if _, err := artifacts.EventsPage(context.Background(), "dead-1", 0, 10); !errors.Is(err, ErrComputerTapeDeleted) || !strings.Contains(err.Error(), "/archive/dead-1.tar.zst") {
		t.Fatalf("replay of a tombstoned tape: %v", err)
	}
	if _, err := artifacts.FetchPayload("dead-1", platformTestDigest('a')); !errors.Is(err, ErrComputerTapeDeleted) {
		t.Fatalf("payload fetch of a tombstoned tape: %v", err)
	}

	eventID, err := computerevent.NewEventID()
	if err != nil {
		t.Fatal(err)
	}
	event := computerevent.Event{
		SchemaVersion: computerevent.SchemaVersionV1, EventID: eventID, ComputerID: "dead-1", Sequence: 3,
		PreviousHead: platformTestDigest('1'), EventKind: computerevent.EventVerificationRecorded,
		OccurredAt: time.Now().UTC().Format(time.RFC3339Nano), IdempotencyKey: "after-tombstone", ActorProfile: "engineering",
		AuthorityRef: "authority:test", PayloadCommitment: platformTestDigest('a'), PrivacyClass: "public",
		ReducerVersion: computerevent.ReducerVersionV1, ExpectedDesiredEventHead: platformTestDigest('1'), ExpectedEffectiveEventHead: platformTestDigest('1'),
		ExpectedDesiredStateCommitment: platformTestDigest('f'), ExpectedEffectiveStateCommitment: platformTestDigest('f'),
	}
	input := computerevent.TransitionInput{}
	pinIntent, err := computerevent.ComputePinIntentCommitment(event, input)
	if err != nil {
		t.Fatal(err)
	}
	event.RequestCommitment, err = computerevent.ComputeRequestCommitment(event, input, pinIntent, nil)
	if err != nil {
		t.Fatal(err)
	}
	request := computerevent.CASRequest{Event: event, EventDigest: platformTestDigest('3'), EventArtifactDigest: platformTestDigest('3'), EventPinReceiptDigest: platformTestDigest('4'), PinIntentCommitment: pinIntent, Input: input}
	if _, err := cas.CompareAndSwap(context.Background(), request); !errors.Is(err, ErrComputerTapeDeleted) {
		t.Fatalf("CAS on a tombstoned computer: %v", err)
	}
	if err := store.removeTapeTombstone(context.Background(), "dead-1"); err == nil {
		t.Fatal("a deleted tombstone was removed")
	}
}

// FM10: caps bound a run, and re-applying the same manifest finishes it
// idempotently.
func TestTapeSweepCapsAndIdempotentReapply(t *testing.T) {
	f := newTapeFixture(t)
	dead := f.chain("dead-1", 3, tapeAged) // 12 files
	dry := f.dryRun(TapeSweepRequest{AcceptUnrecordedForkLineage: true, Sweeps: []string{TapeSweepDestroyed}})
	if dry.DestroyedFiles != 12 {
		t.Fatalf("listed %d files, want 12", dry.DestroyedFiles)
	}
	first, err := f.apply(dry, TapeSweepRequest{AcceptUnrecordedForkLineage: true, MaxDeletes: 5, BatchSize: 2})
	if err != nil {
		t.Fatal(err)
	}
	if first.Apply.Deleted != 5 || first.Apply.DeferredByCap != 7 {
		t.Fatalf("capped apply = %+v", first.Apply)
	}
	tomb, ok, _ := f.store.TapeTombstone(f.ctx, "dead-1")
	if !ok || tomb.Status != tapeTombstoneArchived || tomb.FilesDeleted != 5 {
		t.Fatalf("tombstone after a capped run = %+v", tomb)
	}
	// Payloads go first: no event is gone while a payload it names remains.
	for _, s := range dead.seqs {
		if !f.exists(tapeNSEvent, s.event) {
			t.Fatal("an event was deleted before the capped run reached the events")
		}
	}
	second, err := f.apply(dry, TapeSweepRequest{AcceptUnrecordedForkLineage: true})
	if err != nil {
		t.Fatal(err)
	}
	if second.Apply.Deleted != 7 || second.Apply.Gone != 5 {
		t.Fatalf("resumed apply = %+v", second.Apply)
	}
	f.allExist(dead, false)
	tomb, _, _ = f.store.TapeTombstone(f.ctx, "dead-1")
	if tomb.Status != tapeTombstoneDeleted || tomb.FilesDeleted != 12 {
		t.Fatalf("tombstone after resume = %+v", tomb)
	}
	third, err := f.apply(dry, TapeSweepRequest{AcceptUnrecordedForkLineage: true})
	if err != nil || third.Apply.Deleted != 0 || third.Apply.Gone != 12 {
		t.Fatalf("idempotent re-apply = %+v, %v", third.Apply, err)
	}
}

// FM11: every unlink is preceded by a durable intent record in the GC log,
// against the manifest.
func TestTapeSweepLogsIntentBeforeEveryUnlink(t *testing.T) {
	f := newTapeFixture(t)
	f.put(tapeNSPayload, []byte("orphan a"), tapeAged)
	f.put(tapeNSPayload, []byte("orphan b"), tapeAged)
	dry := f.dryRun(TapeSweepRequest{Sweeps: []string{TapeSweepOrphans}})
	logPath := filepath.Join(f.root, artifactGCLogDir, time.Now().UTC().Format("2006-01-02")+".jsonl")
	f.svc.tapeSweep.beforeDelete = func(it tapeSweepItem) {
		raw, err := os.ReadFile(logPath)
		if err != nil || !strings.Contains(string(raw), `"op":"intent"`) || !strings.Contains(string(raw), it.Path) {
			t.Errorf("no durable intent for %s before its unlink", it.Path)
		}
	}
	if _, err := f.apply(dry, TapeSweepRequest{}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	deleted := 0
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		var rec tapeSweepLogRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatal(err)
		}
		if rec.Run != dry.RunID || rec.Manifest != dry.ManifestSHA256 {
			t.Fatalf("log record not bound to the manifest: %+v", rec)
		}
		if rec.Op == "deleted" {
			deleted++
		}
	}
	if deleted != 2 {
		t.Fatalf("logged %d deletions, want 2", deleted)
	}
}

// The endpoint is internal-only, asynchronous, and refuses apply unless the
// policy is active.
func TestHandleTapeSweepStartsAndReportsJobs(t *testing.T) {
	f := newTapeFixture(t)
	f.put(tapeNSPayload, []byte("orphan"), tapeAged)
	h := NewHandler(f.svc)
	call := func(method, body string, internal bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/internal/platform/artifact-gc/tape", strings.NewReader(body))
		req.RemoteAddr = "127.0.0.1:5555"
		if internal {
			req.Header.Set("X-Internal-Caller", "true")
		}
		rec := httptest.NewRecorder()
		h.HandleTapeSweep(rec, req)
		return rec
	}
	if rec := call(http.MethodPost, `{}`, false); rec.Code != http.StatusForbidden {
		t.Fatalf("external caller = %d", rec.Code)
	}
	rec := call(http.MethodPost, `{"sweeps":["orphans"]}`, true)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("dry-run start = %d %s", rec.Code, rec.Body.String())
	}
	var job TapeSweepJob
	if err := json.Unmarshal(rec.Body.Bytes(), &job); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(60 * time.Second)
	for {
		rec = call(http.MethodGet, "", true)
		if err := json.Unmarshal(rec.Body.Bytes(), &job); err != nil {
			t.Fatal(err)
		}
		if job.State != "running" || time.Now().After(deadline) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if job.State != "done" || job.Report == nil || job.Report.OrphanFiles != 1 || job.Report.ManifestSHA256 == "" {
		t.Fatalf("job = %+v", job)
	}
	f.svc.SetTapeSweepPolicy(TapeSweepPolicy{Mode: ArtifactGCModeDryRun})
	body := fmt.Sprintf(`{"action":"apply","run_id":%q,"manifest_sha256":%q}`, job.RunID, job.Report.ManifestSHA256)
	if rec := call(http.MethodPost, body, true); rec.Code != http.StatusConflict {
		t.Fatalf("apply on a dry-run policy = %d %s", rec.Code, rec.Body.String())
	}
}

// Orphan check bounded by the head snapshot
// (docs/problems/tape-sweeps-destroyed-and-orphans-2026-10-20.md, "the orphan
// check can never pass on a live host").
func TestTapeSweepOrphanReceiptCheckIsHeadBounded(t *testing.T) {
	insertBareReceipt := func(f *tapeFixture, id string, seq uint64) {
		t.Helper()
		d := platformTestDigest('a')
		insertTestAppendReceipt(t, f.store.db, id, fmt.Sprintf("bare-%d", seq), d, seq, "verification_recorded", d, d, nil, time.Now().UTC())
	}
	t.Run("live append during the walk does not refuse", func(t *testing.T) {
		f := newTapeFixture(t)
		f.live("live-1")
		c := f.chain("live-1", 3, tapeAged)
		f.svc.tapeSweep.afterReferences = func() {
			f.appendSeq(c, nil, 0) // receipt and head move past the snapshot
		}
		dry := f.dryRun(TapeSweepRequest{Sweeps: []string{TapeSweepOrphans}})
		if dry.OrphansRefused != "" {
			t.Fatalf("live append during the walk refused the orphan sweep: %s", dry.OrphansRefused)
		}
	})
	t.Run("receipt above the current head refuses and names it", func(t *testing.T) {
		f := newTapeFixture(t)
		f.live("live-1")
		f.chain("live-1", 3, tapeAged)
		insertBareReceipt(f, "live-1", 4) // head stays at 3
		dry := f.dryRun(TapeSweepRequest{Sweeps: []string{TapeSweepOrphans}})
		if dry.OrphansRefused == "" || !strings.Contains(dry.OrphansRefused, "live-1") || !strings.Contains(dry.OrphansRefused, "4") {
			t.Fatalf("leftover above the head not refused by name: %q", dry.OrphansRefused)
		}
	})
	t.Run("receipt above the current head after a live append still refuses", func(t *testing.T) {
		f := newTapeFixture(t)
		f.live("live-1")
		c := f.chain("live-1", 3, tapeAged)
		f.svc.tapeSweep.afterReferences = func() {
			f.appendSeq(c, nil, 0)            // sequence 4, head 4
			insertBareReceipt(f, "live-1", 9) // leftover past the head
		}
		dry := f.dryRun(TapeSweepRequest{Sweeps: []string{TapeSweepOrphans}})
		if dry.OrphansRefused == "" || !strings.Contains(dry.OrphansRefused, "live-1") || !strings.Contains(dry.OrphansRefused, "9") {
			t.Fatalf("leftover past the current head not refused: %q", dry.OrphansRefused)
		}
	})
	t.Run("receipt for a computer with no head refuses", func(t *testing.T) {
		f := newTapeFixture(t)
		f.live("live-1")
		f.chain("live-1", 2, tapeAged)
		insertBareReceipt(f, "ghost-1", 1)
		dry := f.dryRun(TapeSweepRequest{Sweeps: []string{TapeSweepOrphans}})
		if dry.OrphansRefused == "" || !strings.Contains(dry.OrphansRefused, "ghost-1") {
			t.Fatalf("headless receipt not refused by name: %q", dry.OrphansRefused)
		}
	})
	t.Run("a receipt missing below the snapshot head refuses", func(t *testing.T) {
		f := newTapeFixture(t)
		f.live("live-1")
		f.chain("live-1", 3, tapeAged)
		f.svc.tapeSweep.afterReferences = func() {
			if _, err := f.store.db.Exec(`DELETE FROM computer_event_append_receipts WHERE computer_id=? AND sequence=2`, "live-1"); err != nil {
				t.Fatal(err)
			}
		}
		dry := f.dryRun(TapeSweepRequest{Sweeps: []string{TapeSweepOrphans}})
		if dry.OrphansRefused == "" {
			t.Fatal("receipts below the snapshot head no longer match the walk, but the sweep was not refused")
		}
	})
}
