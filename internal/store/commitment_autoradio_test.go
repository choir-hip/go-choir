package store

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/calibration"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Failure list for the Autoradio store queries and the label guard (design
// 4.3, 12.2), written before the code:
//
//  1. ListCalibrationRecordsExcludingAgentPrefixPage returns a record whose stored
//     agent begins with the prefix in any case or padded, or applies the limit
//     before the exclusion (more reserved records than the limit hide an
//     ordinary one); or drops an ordinary agent that merely contains the prefix.
//  2. The claims-by-tag page matches a tag by prefix or substring (stretch:s1
//     matches stretch:s10), returns settlements, or another computer's or
//     owner's records, an ordinary claim that carries the stretch tag, or a
//     record of another kind (review round 1, DeepSeek 4).
//  3. A cursor-paged read repeats a record, skips one, or returns a record
//     out of order across page sizes 1, 2, 3 and larger than the set; the
//     cursor of the last page is not empty; an exact multiple of the page size
//     costs an extra empty page.
//  4. AppendCommitmentRecord accepts a record of record_kind autoradio.* that
//     lacks its owner-private label, carries another kind's, or is a kind
//     outside the table; or refuses a record of another desk.

func openAutoradioStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "autoradio.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

const (
	arOwner    = "owner-ar"
	arComputer = "computer-ar"
)

func arForecast(t *testing.T, stretch string, seg int, at time.Time) types.CommitmentRecord {
	t.Helper()
	rec, err := calibration.NewAutoradioForecast(calibration.AutoradioForecastInput{
		StretchID: stretch, SegmentID: fmt.Sprintf("%s.s%d", stretch, seg), Kind: "story", Title: "t", Model: "m", P: 0.5,
		Cause: calibration.CauseExplore, TextSHA256: strings.Repeat("ab", 32), Blocks: 2, StretchStart: at,
	}, at)
	if err != nil {
		t.Fatal(err)
	}
	rec.ComputerID = arComputer
	return rec
}

func arPut(t *testing.T, s *Store, owner, computer string, rec types.CommitmentRecord) {
	t.Helper()
	rec.ComputerID = computer
	if _, err := s.AppendCommitmentRecord(context.Background(), owner, computer, rec); err != nil {
		t.Fatal(err)
	}
}

func ordinaryClaim(t *testing.T, agent, key string, at time.Time) types.CommitmentRecord {
	t.Helper()
	p := 0.5
	rec, err := calibration.NewClaimRecord(calibration.ClaimRequest{Agent: calibration.AgentIdentity{Name: agent, Model: "m"}, Claim: "c " + key, P: &p, IdempotencyKey: key}, at, "")
	if err != nil {
		t.Fatal(err)
	}
	return rec
}

func TestListCalibrationRecordsExcludesAnAgentPrefixBeforeTheLimit(t *testing.T) {
	s := openAutoradioStore(t)
	ctx := context.Background()
	at := time.Date(2026, 11, 15, 9, 0, 0, 0, time.UTC)
	arPut(t, s, arOwner, arComputer, ordinaryClaim(t, "claude-code/lead", "old", at))
	arPut(t, s, arOwner, arComputer, ordinaryClaim(t, "my-autoradio", "contains", at))
	for i := 0; i < 12; i++ {
		arPut(t, s, arOwner, arComputer, arForecast(t, "st1", i, at.Add(time.Duration(i+1)*time.Second)))
	}
	// Legacy variants stored directly (the generic route no longer writes them).
	for i, agent := range []string{"Autoradio-DJ", " autoradio-dj", "AUTORADIO", "-autoradio-dj"} {
		rec := ordinaryClaim(t, "plain", fmt.Sprintf("v%d", i), at.Add(time.Minute))
		rec.Calibration.Agent = agent
		rec.Provenance.AgentID = "agent:" + agent
		arPut(t, s, arOwner, arComputer, rec)
	}
	// Another computer and another owner hold reserved and ordinary records too.
	arPut(t, s, arOwner, "computer-other", ordinaryClaim(t, "elsewhere", "e1", at))
	arPut(t, s, "owner-other", arComputer, ordinaryClaim(t, "elsewhere", "e2", at))

	// SQL leaves out what its trimmed, lower-cased prefix test sees; a variant that
	// only Slug folds in (hyphen-led) comes through and is the caller's to drop
	// while it pages until its limit is filled (calibration handler test).
	recs, next, err := s.ListCalibrationRecordsExcludingAgentPrefixPage(ctx, arOwner, arComputer, "", "autoradio", "", 5)
	if err != nil || next != "" {
		t.Fatalf("page: next=%q err=%v", next, err)
	}
	got := map[string]bool{}
	for _, r := range recs {
		got[r.Calibration.Agent] = true
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(r.Calibration.Agent)), "autoradio") {
			t.Fatalf("returned reserved agent %q", r.Calibration.Agent)
		}
	}
	if len(recs) != 3 || !got["claude-code/lead"] || !got["my-autoradio"] || !got["-autoradio-dj"] {
		t.Fatalf("with 15 reserved records newer than the limit of 5, got %d records %v: the limit was applied before the exclusion", len(recs), got)
	}
	// Paged newest first with a cursor: one at a time yields the same three, once.
	var order []string
	before := ""
	for i := 0; i < 10; i++ {
		page, nx, err := s.ListCalibrationRecordsExcludingAgentPrefixPage(ctx, arOwner, arComputer, "", "autoradio", before, 1)
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range page {
			order = append(order, r.Calibration.Agent)
		}
		if nx == "" {
			break
		}
		before = nx
	}
	if len(order) != 3 || order[0] != "-autoradio-dj" || order[2] != "claude-code/lead" {
		t.Fatalf("paged order = %v", order)
	}
	// An agent filter composes with the exclusion.
	recs, _, err = s.ListCalibrationRecordsExcludingAgentPrefixPage(ctx, arOwner, arComputer, "claude-code/lead", "autoradio", "", 10)
	if err != nil || len(recs) != 1 {
		t.Fatalf("agent filter: %d %v", len(recs), err)
	}
	recs, _, err = s.ListCalibrationRecordsExcludingAgentPrefixPage(ctx, arOwner, arComputer, "autoradio-dj", "autoradio", "", 10)
	if err != nil || len(recs) != 0 {
		t.Fatalf("a reserved agent filter with the exclusion returned %d (%v)", len(recs), err)
	}
}

func TestListCalibrationClaimsByTagPage(t *testing.T) {
	s := openAutoradioStore(t)
	ctx := context.Background()
	at := time.Date(2026, 11, 15, 9, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		arPut(t, s, arOwner, arComputer, arForecast(t, "s1", i, at))
	}
	for i := 0; i < 2; i++ {
		arPut(t, s, arOwner, arComputer, arForecast(t, "s10", i, at))
	}
	// A settlement of one s1 forecast, and records of another computer and owner.
	f := arForecast(t, "s1", 0, at)
	res, err := calibration.SettleAutoradio(ctx, s, arOwner, arComputer, f.RecordID, calibration.AutoradioHeardRule, true, "ev-1", at.Add(time.Hour))
	if err != nil || !res.Won {
		t.Fatalf("settle: %v", err)
	}
	// An ordinary claim of another agent that carries the same stretch tag is
	// not a stretch forecast; nor is an autoradio-dj claim of another record kind.
	foreign := ordinaryClaim(t, "someone", "foreign", at)
	foreign.Calibration.Claim.Tags = []string{"stretch:s1"}
	arPut(t, s, arOwner, arComputer, foreign)
	notForecast := arForecast(t, "s1", 8, at)
	notForecast.SubjectRefs["record_kind"] = "autoradio.segment"
	notForecast.Provenance.Labels = []string{types.OwnerPrivateLabel("autoradio_segment")}
	arPut(t, s, arOwner, arComputer, notForecast)
	other := arForecast(t, "s1", 7, at)
	arPut(t, s, arOwner, "computer-other", other)
	arPut(t, s, "owner-other", arComputer, other)

	page, next, err := s.ListCalibrationClaimsByTagPage(ctx, arOwner, arComputer, "autoradio-dj", "autoradio.forecast", "stretch:s1", "", 100)
	if err != nil || next != "" {
		t.Fatalf("page: next=%q err=%v", next, err)
	}
	if len(page) != 3 {
		t.Fatalf("stretch:s1 matched %d claims, want exactly 3 (s10 and settlements are not it)", len(page))
	}
	for _, r := range page {
		if !calibration.IsClaim(&r) || r.ComputerID != arComputer {
			t.Fatalf("returned %+v", r.RecordID)
		}
	}
	for _, tag := range []string{"stretch:s", "stretch", "s1", "STRETCH:s1", "autoradio-x"} {
		page, _, err := s.ListCalibrationClaimsByTagPage(ctx, arOwner, arComputer, "autoradio-dj", "autoradio.forecast", tag, "", 100)
		if err != nil || len(page) != 0 {
			t.Fatalf("tag %q matched %d (%v): a tag is matched exactly", tag, len(page), err)
		}
	}
	if page, _, _ := s.ListCalibrationClaimsByTagPage(ctx, arOwner, arComputer, "autoradio-dj", "autoradio.forecast", "autoradio", "", 100); len(page) != 5 {
		t.Fatalf("tag autoradio matched %d, want 5", len(page))
	}
	if _, _, err := s.ListCalibrationClaimsByTagPage(ctx, arOwner, arComputer, "autoradio-dj", "autoradio.forecast", "", "", 10); err == nil {
		t.Fatal("empty tag accepted")
	}
}

func TestCalibrationPagesCoverEverythingOnce(t *testing.T) {
	s := openAutoradioStore(t)
	ctx := context.Background()
	at := time.Date(2026, 11, 15, 9, 0, 0, 0, time.UTC)
	var want []string
	for i := 0; i < 7; i++ {
		f := arForecast(t, "pg", i, at.Add(time.Duration(i)*time.Second))
		arPut(t, s, arOwner, arComputer, f)
		want = append(want, f.RecordID)
		if i%2 == 0 {
			res, err := calibration.SettleAutoradio(ctx, s, arOwner, arComputer, f.RecordID, calibration.AutoradioHeardRule, true, fmt.Sprintf("ev-%d", i), at.Add(time.Hour))
			if err != nil || !res.Won {
				t.Fatal(err)
			}
			want = append(want, res.Record.RecordID)
		}
	}
	arPut(t, s, arOwner, arComputer, ordinaryClaim(t, "someone", "k", at))
	for _, size := range []int{1, 2, 3, 11, 50} {
		var got []string
		cursor, pages := "", 0
		for {
			recs, next, err := s.ListCalibrationRecordsPage(ctx, arOwner, arComputer, "autoradio-dj", cursor, size)
			if err != nil {
				t.Fatal(err)
			}
			pages++
			for _, r := range recs {
				got = append(got, r.RecordID)
			}
			if next == "" {
				break
			}
			if len(recs) != size {
				t.Fatalf("size %d: a short page (%d) with a cursor", size, len(recs))
			}
			cursor = next
			if pages > 100 {
				t.Fatal("cursor does not advance")
			}
		}
		if len(got) != len(want) {
			t.Fatalf("size %d: got %d records, want %d", size, len(got), len(want))
		}
		seen := map[string]bool{}
		for _, id := range got {
			if seen[id] {
				t.Fatalf("size %d: %s repeated", size, id)
			}
			seen[id] = true
		}
		for _, id := range want {
			if !seen[id] {
				t.Fatalf("size %d: %s skipped", size, id)
			}
		}
		if size >= 11 && pages != 1 {
			// 11 records: an exact multiple of the page size costs no extra empty page.
			t.Fatalf("size %d: %d pages", size, pages)
		}
	}
	// An exact multiple: 11 records, size 11 gives one page and no cursor.
	recs, next, err := s.ListCalibrationRecordsPage(ctx, arOwner, arComputer, "autoradio-dj", "", 11)
	if err != nil || len(recs) != 11 || next != "" {
		t.Fatalf("exact multiple: %d records next=%q err=%v", len(recs), next, err)
	}
	// A record written after the first page is not skipped by the cursor.
	recs, next, _ = s.ListCalibrationRecordsPage(ctx, arOwner, arComputer, "autoradio-dj", "", 4)
	arPut(t, s, arOwner, arComputer, arForecast(t, "pg", 9, at.Add(time.Minute)))
	tail, _, _ := s.ListCalibrationRecordsPage(ctx, arOwner, arComputer, "autoradio-dj", next, 100)
	if len(recs)+len(tail) != len(want)+1 {
		t.Fatalf("a record written between pages was lost: %d + %d", len(recs), len(tail))
	}
	for _, bad := range []string{"x", "-1", "1.5"} {
		if _, _, err := s.ListCalibrationRecordsPage(ctx, arOwner, arComputer, "autoradio-dj", bad, 5); err == nil {
			t.Fatalf("cursor %q accepted", bad)
		}
	}
}

func TestAppendRefusesAnUnlabelledAutoradioRecord(t *testing.T) {
	s := openAutoradioStore(t)
	ctx := context.Background()
	for _, k := range types.AutoradioRecordKinds {
		rec := types.CommitmentRecord{
			SchemaID: types.CommitmentRecordSchemaV1, RecordID: "rec:" + k.Kind, Kind: types.CommitmentKindReport,
			Discrepancy: types.DiscrepancyUnresolved, EmitterKind: types.CommitmentEmitterAgent,
			Provenance:  types.CommitmentProvenance{AgentID: "app:autoradio:o", CommittedAt: "2026-11-15T09:00:00Z"},
			SubjectRefs: map[string]string{"desk": "app:autoradio", "record_kind": k.Kind}, ComputerID: arComputer,
		}
		if _, err := s.AppendCommitmentRecord(ctx, arOwner, arComputer, rec); err == nil {
			t.Fatalf("%s with no label was appended", k.Kind)
		}
		rec.Provenance.Labels = []string{types.OwnerPrivateLabel("autoradio_turn_x")}
		if _, err := s.AppendCommitmentRecord(ctx, arOwner, arComputer, rec); err == nil {
			t.Fatalf("%s with another kind's label was appended", k.Kind)
		}
		label, _ := types.AutoradioKindLabel(k.Kind)
		rec.Provenance.Labels = []string{label}
		if _, err := s.AppendCommitmentRecord(ctx, arOwner, arComputer, rec); err != nil {
			t.Fatalf("%s with its label: %v", k.Kind, err)
		}
		if got, _ := s.GetCommitmentRecord(ctx, arOwner, arComputer, rec.RecordID); got == nil {
			t.Fatalf("%s not stored", k.Kind)
		}
	}
	unknown := types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: "rec:unknown", Kind: types.CommitmentKindReport,
		Provenance:  types.CommitmentProvenance{AgentID: "app:autoradio:o", Labels: []string{types.OwnerPrivateLabel("autoradio_nope")}},
		SubjectRefs: map[string]string{"record_kind": "autoradio.nope"},
	}
	if _, err := s.AppendCommitmentRecord(ctx, arOwner, arComputer, unknown); err == nil {
		t.Fatal("an unknown autoradio kind was appended")
	}
	elsewhere := types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: "rec:texture", Kind: types.CommitmentKindReport,
		Provenance:  types.CommitmentProvenance{AgentID: "desk:texture"},
		SubjectRefs: map[string]string{"record_kind": "texture.note"},
	}
	if _, err := s.AppendCommitmentRecord(ctx, arOwner, arComputer, elsewhere); err != nil {
		t.Fatalf("another desk's record refused: %v", err)
	}
}

// ListCommitmentRecordsBySubjectRefOfComputer (run sheet replay): another
// computer's rows do not count toward the limit; a row whose envelope does
// not decode is counted and skipped, not an error.
func TestListBySubjectRefOfComputerFiltersInTheQueryAndSkipsUndecodableRows(t *testing.T) {
	s := openAutoradioStore(t)
	ctx := context.Background()
	label, _ := types.AutoradioKindLabel("autoradio.sheet_op")
	mk := func(id, computer string) types.CommitmentRecord {
		return types.CommitmentRecord{
			SchemaID: types.CommitmentRecordSchemaV1, RecordID: id, Kind: types.CommitmentKindDirective,
			Discrepancy: types.DiscrepancyUnresolved, ComputerID: computer,
			Directive:   &types.CommitmentDirective{Subtype: types.CommitmentDirectiveNotice, Objective: id},
			Provenance:  types.CommitmentProvenance{AgentID: "app:autoradio:o1", Labels: types.NewLabelSet(label).Strings()},
			SubjectRefs: map[string]string{"desk": "app:autoradio", "record_kind": "autoradio.sheet_op", "sheet_ops": "owner"},
		}
	}
	for i := 0; i < 6; i++ {
		if _, err := s.AppendCommitmentRecord(ctx, "o1", "c1", mk(fmt.Sprintf("mine-%d", i), "c1")); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 10; i++ { // newer rows of another computer
		if _, err := s.AppendCommitmentRecord(ctx, "o1", "c2", mk(fmt.Sprintf("theirs-%d", i), "c2")); err != nil {
			t.Fatal(err)
		}
	}
	recs, bad, rows, err := s.ListCommitmentRecordsBySubjectRefOfComputer(ctx, "o1", "c1", "sheet_ops", "owner", 4)
	if err != nil || len(recs) != 4 || bad != 0 || rows != 4 {
		t.Fatalf("limit 4: %d records, %d bad, %d rows, %v (the other computer's rows must not use the limit)", len(recs), bad, rows, err)
	}
	// A row that matches the query but whose envelope does not decode.
	now := time.Now().UTC()
	body := map[string]any{"record_id": "broken", "computer_id": "c1", "scores": "not a list",
		"subject_refs": map[string]string{"sheet_ops": "owner", "record_kind": "autoradio.sheet_op"}}
	obj, err := lifecycleObject(ogKindCommitmentRecord, "o1", "c1", "broken", body, map[string]any{"record_id": "broken"}, now, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ogStore.PutBatchConditional(ctx, nil, objectgraph.Batch{Objects: []objectgraph.Object{obj}}); err != nil {
		t.Fatal(err)
	}
	recs, bad, rows, err = s.ListCommitmentRecordsBySubjectRefOfComputer(ctx, "o1", "c1", "sheet_ops", "owner", 100)
	if err != nil || len(recs) != 6 || bad != 1 || rows != 7 {
		t.Fatalf("with a broken row: %d records, %d bad, %d rows, %v", len(recs), bad, rows, err)
	}
}
