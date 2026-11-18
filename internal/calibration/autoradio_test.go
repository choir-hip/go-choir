package calibration

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// Failure list for the Autoradio forecast constructor, the reservation tests
// and the reserved settle method (learning-engine design 3.3, 4.3, 12),
// written before the code:
//
//  1. NewAutoradioForecast writes a claim whose agent, type, p, claim text,
//     criterion, idempotency key, project, deadline, source_ref or tags differ
//     from the table of 3.3 and 12.1; a replay is not byte-identical
//     (SameClaim) or has another id; a lesson tag is written for another cause
//     class, or missing for the lesson class.
//  2. The constructor accepts a bad stretch id, a segment id that is not the
//     stretch id plus ".s" and an index, a segment id over 40 bytes, a kind
//     outside the grammar, a title with a control character, an empty model,
//     a p outside [0,1] or NaN, a hash that is not 64 hex digits, a block count
//     below 1, an unknown cause class, a caused_by ref that does not match its
//     class, or a ref so long the criterion exceeds its bound.
//  3. The forecast does not carry the desk and record_kind subject refs, or
//     the owner-private label of its kind (design 12.2).
//  4. The reserved test calls a name reserved only when it equals
//     "autoradio-dj": a padded, differently cased or leading-hyphen variant
//     passes; a tag in another case passes; "my-autoradio" or a tag
//     "radio-autoradio" is refused (over-reach); a claim is judged by its
//     request or id prefix instead of the stored agent and tags.
//  5. The settle method accepts a rule outside the closed registry, a claim
//     whose stored agent is not exactly autoradio-dj (a "Autoradio-DJ" claim
//     written to the store directly), a claim without the server-written tags,
//     a record that is not a forecast claim, an outcome other than heard or
//     not heard, evidence that is not exactly one air ref of the grammar of
//     12.5; it writes a settlement without the label, with another id than
//     "<claim id>:settle:rule:autoradio_heard", without a Brier score; a replay
//     writes a second record; the loser of a race is not told it lost; a
//     conflicting existing settlement is reported as won.
//  6. The settle method has a caller other than the heard route.
//  7. SplitBlocks counts the edge texts of 12.3 differently than the fixture.

const (
	testSHA   = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	testNow   = "2026-11-15T10:00:00Z"
	testStart = "2026-11-15T09:00:00Z"
)

func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	v, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func forecastInput(t *testing.T, stretch string, seg int) AutoradioForecastInput {
	t.Helper()
	return AutoradioForecastInput{
		StretchID: stretch, SegmentID: stretch + ".s" + string(rune('0'+seg)), Kind: "story", Title: "A quiet harbour",
		Model: "deepseek-v4.1-flash", P: 0.7, Cause: CauseLesson, CausedBy: "lesson:L12", LessonID: "L12",
		TextSHA256: testSHA, Blocks: 3, StretchStart: mustTime(t, testStart),
	}
}

func TestNewAutoradioForecastFields(t *testing.T) {
	in := forecastInput(t, "st1", 0)
	rec, err := NewAutoradioForecast(in, mustTime(t, testNow))
	if err != nil {
		t.Fatal(err)
	}
	if !IsClaim(&rec) {
		t.Fatal("not a claim")
	}
	c := rec.Calibration.Claim
	if rec.Calibration.Agent != AutoradioAgent || AutoradioAgent != "autoradio-dj" {
		t.Fatalf("agent = %q", rec.Calibration.Agent)
	}
	if rec.Provenance.ModelID != "deepseek-v4.1-flash" || c.SessionID != "st1" {
		t.Fatalf("model %q session %q", rec.Provenance.ModelID, c.SessionID)
	}
	if c.Type != types.CalibrationClaimForecast || c.Probability != 0.7 {
		t.Fatalf("type %q p %v", c.Type, c.Probability)
	}
	if c.Text != "The listener hears segment st1.s0 (story: A quiet harbour) through." {
		t.Fatalf("claim = %q", c.Text)
	}
	if c.Criterion != "caused_by=lesson:L12" {
		t.Fatalf("criterion = %q", c.Criterion)
	}
	if c.Project != "story" || c.SourceRef != "autoradio:st1/st1.s0" {
		t.Fatalf("project %q source_ref %q", c.Project, c.SourceRef)
	}
	if c.Deadline != "2026-11-15T21:00:00Z" {
		t.Fatalf("deadline = %q, want start + 12h", c.Deadline)
	}
	wantTags := []string{"autoradio", "stretch:st1", "seg:st1.s0", "kind:story", "cause:lesson", "telling:full", "lesson:L12", "line:lesson:L12", "sha:" + testSHA[:32], "blocks:3"}
	if !reflect.DeepEqual(c.Tags, wantTags) {
		t.Fatalf("tags = %v", c.Tags)
	}
	for _, tag := range c.Tags {
		if len(tag) > 64 {
			t.Fatalf("tag %q over 64 bytes", tag)
		}
	}
	wantID := ClaimRecordID(AutoradioAgent, "st1/st1.s0", "")
	if rec.RecordID != wantID || !strings.HasPrefix(rec.RecordID, "calib:autoradio-dj:") {
		t.Fatalf("id = %q want %q", rec.RecordID, wantID)
	}
	if rec.SubjectRefs["desk"] != "app:autoradio" || rec.SubjectRefs["record_kind"] != "autoradio.forecast" {
		t.Fatalf("subject refs = %v", rec.SubjectRefs)
	}
	if err := types.CheckAutoradioLabel(rec); err != nil {
		t.Fatal(err)
	}
	if !types.NewLabelSet(rec.Provenance.Labels...).Has(types.OwnerPrivateLabel("autoradio_forecast")) {
		t.Fatalf("labels = %v", rec.Provenance.Labels)
	}
	// A replay at another commit time says the same thing.
	again, err := NewAutoradioForecast(in, mustTime(t, "2026-11-15T10:05:00Z"))
	if err != nil || again.RecordID != rec.RecordID || !SameClaim(rec, again) {
		t.Fatalf("replay differs: %v", err)
	}
	// Another segment is another claim.
	other, _ := NewAutoradioForecast(forecastInput(t, "st1", 1), mustTime(t, testNow))
	if other.RecordID == rec.RecordID {
		t.Fatal("two segments share a claim id")
	}
}

func TestNewAutoradioForecastCauseClasses(t *testing.T) {
	now := mustTime(t, testNow)
	for _, tc := range []struct {
		name      string
		cause     string
		causedBy  string
		lessonID  string
		criterion string
		hasLesson bool
	}{
		{"instruction turn", CauseInstruction, "instruction:turn/t-1", "", "caused_by=instruction:turn/t-1", false},
		{"instruction note", CauseInstruction, "instruction:note/ab12", "", "caused_by=instruction:note/ab12", false},
		{"edition", CauseEdition, "edition:P3", "", "caused_by=edition:P3", false},
		{"importance no ref", CauseImportance, "", "", "caused_by=importance", false},
		{"explore named", CauseExplore, "explore", "", "caused_by=explore", false},
		{"lesson", CauseLesson, "lesson:L7", "L7", "caused_by=lesson:L7", true},
	} {
		in := forecastInput(t, "st2", 0)
		in.Cause, in.CausedBy, in.LessonID = tc.cause, tc.causedBy, tc.lessonID
		rec, err := NewAutoradioForecast(in, now)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		c := rec.Calibration.Claim
		if c.Criterion != tc.criterion {
			t.Fatalf("%s: criterion %q", tc.name, c.Criterion)
		}
		lesson := false
		for _, tag := range c.Tags {
			if strings.HasPrefix(tag, "lesson:") {
				lesson = true
			}
		}
		if lesson != tc.hasLesson {
			t.Fatalf("%s: lesson tag present = %v, tags %v", tc.name, lesson, c.Tags)
		}
		if !contains(c.Tags, "cause:"+tc.cause) {
			t.Fatalf("%s: tags %v", tc.name, c.Tags)
		}
	}
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func TestNewAutoradioForecastRefuses(t *testing.T) {
	now := mustTime(t, testNow)
	long := "instruction:turn/" + strings.Repeat("a", 1990)
	for name, mutate := range map[string]func(*AutoradioForecastInput){
		"empty stretch":        func(in *AutoradioForecastInput) { in.StretchID = "" },
		"stretch bad char":     func(in *AutoradioForecastInput) { in.StretchID = "st 1" },
		"stretch leading dot":  func(in *AutoradioForecastInput) { in.StretchID = ".st1" },
		"stretch too long":     func(in *AutoradioForecastInput) { in.StretchID = strings.Repeat("a", 33) },
		"segment not stretch":  func(in *AutoradioForecastInput) { in.SegmentID = "other.s0" },
		"segment no index":     func(in *AutoradioForecastInput) { in.SegmentID = "st1.s" },
		"segment bad index":    func(in *AutoradioForecastInput) { in.SegmentID = "st1.sx" },
		"kind empty":           func(in *AutoradioForecastInput) { in.Kind = "" },
		"kind spaces":          func(in *AutoradioForecastInput) { in.Kind = "front page" },
		"kind upper":           func(in *AutoradioForecastInput) { in.Kind = "Story" },
		"title control":        func(in *AutoradioForecastInput) { in.Title = "a\x07b" },
		"title newline":        func(in *AutoradioForecastInput) { in.Title = "a\nb" },
		"title tab":            func(in *AutoradioForecastInput) { in.Title = "a\tb" },
		"title over 200 bytes": func(in *AutoradioForecastInput) { in.Title = strings.Repeat("x", 201) },
		"model empty":          func(in *AutoradioForecastInput) { in.Model = "" },
		"p above 1":            func(in *AutoradioForecastInput) { in.P = 1.01 },
		"p below 0":            func(in *AutoradioForecastInput) { in.P = -0.01 },
		"p NaN":                func(in *AutoradioForecastInput) { in.P = math.NaN() },
		"p Inf":                func(in *AutoradioForecastInput) { in.P = math.Inf(1) },
		"hash short":           func(in *AutoradioForecastInput) { in.TextSHA256 = testSHA[:32] },
		"hash not hex":         func(in *AutoradioForecastInput) { in.TextSHA256 = strings.Repeat("z", 64) },
		"hash upper":           func(in *AutoradioForecastInput) { in.TextSHA256 = strings.ToUpper(testSHA) },
		"blocks zero":          func(in *AutoradioForecastInput) { in.Blocks = 0 },
		"blocks negative":      func(in *AutoradioForecastInput) { in.Blocks = -1 },
		"unknown cause":        func(in *AutoradioForecastInput) { in.Cause = "hunch" },
		"lesson cause no id":   func(in *AutoradioForecastInput) { in.LessonID = "" },
		"lesson id bad":        func(in *AutoradioForecastInput) { in.LessonID = "12" },
		"lesson ref mismatch":  func(in *AutoradioForecastInput) { in.CausedBy = "lesson:L13" },
		"lesson id on other":   func(in *AutoradioForecastInput) { in.Cause, in.CausedBy = CauseEdition, "edition:P1" },
		"edition bad ref":      func(in *AutoradioForecastInput) { in.Cause, in.CausedBy, in.LessonID = CauseEdition, "edition:3", "" },
		"instruction bad ref":  func(in *AutoradioForecastInput) { in.Cause, in.CausedBy, in.LessonID = CauseInstruction, "turn/t1", "" },
		"instruction wrong class": func(in *AutoradioForecastInput) {
			in.Cause, in.CausedBy, in.LessonID = CauseInstruction, "lesson:L1", ""
		},
		"importance with ref": func(in *AutoradioForecastInput) {
			in.Cause, in.CausedBy, in.LessonID = CauseImportance, "edition:P1", ""
		},
		"ref too long": func(in *AutoradioForecastInput) { in.Cause, in.CausedBy, in.LessonID = CauseInstruction, long, "" },
		"no start":     func(in *AutoradioForecastInput) { in.StretchStart = time.Time{} },
	} {
		in := forecastInput(t, "st1", 0)
		mutate(&in)
		if _, err := NewAutoradioForecast(in, now); err == nil {
			t.Fatalf("%s: accepted %+v", name, in)
		}
	}
	// Endpoints are allowed and scored as stated (design 3.2).
	for _, p := range []float64{0, 1} {
		in := forecastInput(t, "st1", 0)
		in.P = p
		if _, err := NewAutoradioForecast(in, now); err != nil {
			t.Fatalf("p=%v refused: %v", p, err)
		}
	}
}

func TestReservedNames(t *testing.T) {
	for _, name := range []string{"autoradio-dj", " autoradio-dj", "Autoradio-DJ", "AUTORADIO", "autoradio", "autoradio-x", "autoradio_x", "-autoradio-dj", "\tAutoradio", "autoradio.dj"} {
		if !IsReservedAgent(name) {
			t.Fatalf("agent %q not reserved", name)
		}
	}
	for _, name := range []string{"", "claude-code/lead", "my-autoradio", "radio", "auto-radio", "autoradi", "choir-autoradio"} {
		if IsReservedAgent(name) {
			t.Fatalf("agent %q reserved", name)
		}
	}
	for _, tag := range []string{"autoradio", "Autoradio", " autoradio", "AUTORADIO:stretch", "autoradio-x", "autoradio:cause"} {
		if !IsReservedTag(tag) {
			t.Fatalf("tag %q not reserved", tag)
		}
	}
	for _, tag := range []string{"", "radio-autoradio", "auto", "stretch:autoradio", "mission"} {
		if IsReservedTag(tag) {
			t.Fatalf("tag %q reserved", tag)
		}
	}
}

func claimRecord(t *testing.T, id, agent string, tags ...string) types.CommitmentRecord {
	t.Helper()
	rec, err := NewClaimRecord(ClaimRequest{
		Agent: AgentIdentity{Name: "x", Model: "m"}, Claim: "c", P: ptr(0.5), Tags: nil, IdempotencyKey: id,
	}, mustTime(t, testNow), "")
	if err != nil {
		t.Fatal(err)
	}
	rec.RecordID = id
	rec.Calibration.Agent = agent
	rec.Calibration.Claim.Tags = tags
	return rec
}

func TestIsReservedClaimJudgesStoredAgentAndTags(t *testing.T) {
	// The id prefix and the request are not the test; the stored agent and tags are.
	if IsReservedClaim(nil) {
		t.Fatal("nil is reserved")
	}
	if IsReservedClaim(&types.CommitmentRecord{}) {
		t.Fatal("a record with no claim is reserved")
	}
	byAgent := claimRecord(t, "calib:plain:1", " Autoradio-DJ")
	byTag := claimRecord(t, "calib:plain:2", "other", "mission", "AutoRadio")
	byID := claimRecord(t, "calib:autoradio-dj:3", "legacy-agent", "mission")
	clean := claimRecord(t, "calib:plain:4", "my-autoradio", "radio-autoradio")
	if !IsReservedClaim(&byAgent) || !IsReservedClaim(&byTag) {
		t.Fatal("stored agent or tag not detected")
	}
	if IsReservedClaim(&byID) {
		t.Fatal("an id prefix alone reserved a claim")
	}
	if IsReservedClaim(&clean) {
		t.Fatal("a claim outside the namespace was reserved")
	}
	// A settlement of a reserved claim carries the claimant and is reserved.
	settlement := types.CommitmentRecord{Calibration: &types.CommitmentCalibration{Schema: types.CalibrationSchemaV1, Agent: "autoradio-dj", Settlement: &types.CalibrationSettlement{ClaimID: "x"}}}
	if !IsReservedRecord(&settlement) || !IsReservedRecord(&byAgent) || IsReservedRecord(&clean) {
		t.Fatal("IsReservedRecord")
	}
}

// ---- the reserved settle method ----

type memLedger struct {
	records map[string]types.CommitmentRecord
	// raceSettlement, when set, is stored under its id the moment the settle
	// method looks the id up and finds nothing: another writer won first.
	raceSettlement *types.CommitmentRecord
	// raceOnAppend, when set, is stored instead of the record being appended
	// under its id: another writer won between the existence read and the append.
	raceOnAppend *types.CommitmentRecord
	appended     []string
}

func newMemLedger() *memLedger { return &memLedger{records: map[string]types.CommitmentRecord{}} }

func (m *memLedger) AppendCommitmentRecord(_ context.Context, _, _ string, rec types.CommitmentRecord) (string, error) {
	if m.raceOnAppend != nil && m.raceOnAppend.RecordID == rec.RecordID {
		rec = *m.raceOnAppend
	}
	if _, ok := m.records[rec.RecordID]; ok {
		return rec.RecordID, nil
	}
	m.records[rec.RecordID] = rec
	m.appended = append(m.appended, rec.RecordID)
	return rec.RecordID, nil
}

func (m *memLedger) GetCommitmentRecord(_ context.Context, _, _, id string) (*types.CommitmentRecord, error) {
	if m.raceSettlement != nil && m.raceSettlement.RecordID == id {
		if _, ok := m.records[id]; !ok {
			m.records[id] = *m.raceSettlement
		}
	}
	rec, ok := m.records[id]
	if !ok {
		return nil, nil
	}
	return &rec, nil
}

func seedForecast(t *testing.T, m *memLedger, stretch string, seg int) types.CommitmentRecord {
	t.Helper()
	rec, err := NewAutoradioForecast(forecastInput(t, stretch, seg), mustTime(t, testNow))
	if err != nil {
		t.Fatal(err)
	}
	m.records[rec.RecordID] = rec
	return rec
}

func TestSettleAutoradioHeard(t *testing.T) {
	ctx := context.Background()
	m := newMemLedger()
	claim := seedForecast(t, m, "st1", 0)
	at := mustTime(t, "2026-11-15T10:30:00Z")
	res, err := SettleAutoradio(ctx, m, "owner", "computer", claim.RecordID, AutoradioHeardRule, true, "ev-100", at)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Won {
		t.Fatal("first writer did not win")
	}
	s := res.Record
	if s.RecordID != claim.RecordID+":settle:rule:autoradio_heard" {
		t.Fatalf("settlement id = %q", s.RecordID)
	}
	st := s.Calibration.Settlement
	if st.ScorerMethod != types.CalibrationScorerRule || st.ScorerName != "autoradio_heard" || st.Outcome != types.CalibrationOutcomeTrue || st.ClaimID != claim.RecordID {
		t.Fatalf("settlement = %+v", st)
	}
	if st.Brier == nil || math.Abs(*st.Brier-0.09) > 1e-9 {
		t.Fatalf("brier = %v, want (0.7-1)^2", st.Brier)
	}
	if !reflect.DeepEqual(s.EvidenceRefs, []string{"air:ev-100"}) {
		t.Fatalf("evidence = %v", s.EvidenceRefs)
	}
	if s.Calibration.Agent != AutoradioAgent {
		t.Fatalf("settlement agent = %q", s.Calibration.Agent)
	}
	if s.SubjectRefs["desk"] != "app:autoradio" || s.SubjectRefs["record_kind"] != "autoradio.settlement" {
		t.Fatalf("subject refs = %v", s.SubjectRefs)
	}
	if err := types.CheckAutoradioLabel(s); err != nil {
		t.Fatal(err)
	}
	if !types.NewLabelSet(s.Provenance.Labels...).Has(types.OwnerPrivateLabel("autoradio_settlement")) {
		t.Fatalf("labels = %v", s.Provenance.Labels)
	}
	if got := m.records[s.RecordID]; got.RecordID == "" {
		t.Fatal("settlement not appended")
	}
	// The rule governs.
	views := JoinClaims([]types.CommitmentRecord{claim, s}, at.Add(time.Hour))
	if len(views) != 1 || views[0].GoverningMethod != types.CalibrationScorerRule || views[0].Outcome != types.CalibrationOutcomeTrue || views[0].Status != StatusSettled {
		t.Fatalf("view = %+v", views)
	}

	// A replay of the same event writes nothing new and wins nothing.
	before := len(m.appended)
	again, err := SettleAutoradio(ctx, m, "owner", "computer", claim.RecordID, AutoradioHeardRule, true, "ev-100", at.Add(time.Minute))
	if err != nil || again.Won || len(m.appended) != before {
		t.Fatalf("replay: won=%v appended=%d err=%v", again.Won, len(m.appended)-before, err)
	}
	// A different outcome from a later event: the first settlement stands; the
	// caller is told it did not write (it records a conflict instead).
	later, err := SettleAutoradio(ctx, m, "owner", "computer", claim.RecordID, AutoradioHeardRule, false, "ev-101", at.Add(time.Hour))
	if err != nil || later.Won || len(m.appended) != before {
		t.Fatalf("later disagreeing event: won=%v appended=%d err=%v", later.Won, len(m.appended)-before, err)
	}
	if later.Record.Calibration.Settlement.Outcome != types.CalibrationOutcomeTrue || !reflect.DeepEqual(later.Record.EvidenceRefs, []string{"air:ev-100"}) {
		t.Fatalf("stored settlement changed: %+v", later.Record)
	}
}

func TestSettleAutoradioLosesARace(t *testing.T) {
	ctx := context.Background()
	m := newMemLedger()
	claim := seedForecast(t, m, "st1", 0)
	at := mustTime(t, "2026-11-15T10:30:00Z")
	winner, err := NewAutoradioSettlementRecord(claim, AutoradioHeardRule, false, "ev-1", at.Add(-time.Second))
	if err != nil {
		t.Fatal(err)
	}
	m.raceSettlement = &winner
	res, err := SettleAutoradio(ctx, m, "owner", "computer", claim.RecordID, AutoradioHeardRule, true, "ev-2", at)
	if err != nil {
		t.Fatal(err)
	}
	if res.Won {
		t.Fatal("the loser was told it won (the store answers success on a conflict)")
	}
	if res.Record.Calibration.Settlement.Outcome != types.CalibrationOutcomeFalse {
		t.Fatalf("returned the loser's record: %+v", res.Record.Calibration.Settlement)
	}
}

func TestSettleAutoradioRefuses(t *testing.T) {
	ctx := context.Background()
	at := mustTime(t, "2026-11-15T10:30:00Z")
	m := newMemLedger()
	claim := seedForecast(t, m, "st1", 0)

	for name, rule := range map[string]string{"empty": "", "other": "autoradio_other", "case": "Autoradio_Heard", "padded": " autoradio_heard", "owner word": "owner", "generic": "tests"} {
		if _, err := SettleAutoradio(ctx, m, "o", "c", claim.RecordID, rule, true, "ev-1", at); err == nil {
			t.Fatalf("rule %s (%q) accepted", name, rule)
		}
	}
	for name, ev := range map[string]string{"empty": "", "colon": "air:ev-1", "other ref": "commit:abc", "air empty": "air:", "bad id": "air:a b", "dot start": "air:.x", "long id": "air:" + strings.Repeat("a", 65)} {
		if _, err := SettleAutoradio(ctx, m, "o", "c", claim.RecordID, AutoradioHeardRule, true, ev, at); err == nil {
			t.Fatalf("evidence %s (%q) accepted", name, ev)
		}
	}
	if _, err := SettleAutoradio(ctx, m, "o", "c", "calib:autoradio-dj:missing", AutoradioHeardRule, true, "ev-1", at); err == nil {
		t.Fatal("missing claim settled")
	}

	// A claim whose stored agent is not exactly autoradio-dj, whatever its id.
	for name, mutate := range map[string]func(*types.CommitmentRecord){
		"cased agent":  func(r *types.CommitmentRecord) { r.Calibration.Agent = "Autoradio-DJ" },
		"padded agent": func(r *types.CommitmentRecord) { r.Calibration.Agent = " autoradio-dj" },
		"other agent":  func(r *types.CommitmentRecord) { r.Calibration.Agent = "autoradio-x" },
		"no tags":      func(r *types.CommitmentRecord) { r.Calibration.Claim.Tags = nil },
		"no stretch tag": func(r *types.CommitmentRecord) {
			r.Calibration.Claim.Tags = dropPrefix(r.Calibration.Claim.Tags, "stretch:")
		},
		"no seg tag": func(r *types.CommitmentRecord) {
			r.Calibration.Claim.Tags = dropPrefix(r.Calibration.Claim.Tags, "seg:")
		},
		"no sha tag": func(r *types.CommitmentRecord) {
			r.Calibration.Claim.Tags = dropPrefix(r.Calibration.Claim.Tags, "sha:")
		},
		"no blocks tag": func(r *types.CommitmentRecord) {
			r.Calibration.Claim.Tags = dropPrefix(r.Calibration.Claim.Tags, "blocks:")
		},
		"no autoradio tag": func(r *types.CommitmentRecord) {
			r.Calibration.Claim.Tags = dropPrefix(r.Calibration.Claim.Tags, "autoradio")
		},
		"not a forecast": func(r *types.CommitmentRecord) { r.Calibration.Claim.Type = types.CalibrationClaimAssertion },
		"no record kind": func(r *types.CommitmentRecord) { r.SubjectRefs = nil },
		"wrong record kind": func(r *types.CommitmentRecord) {
			r.SubjectRefs = map[string]string{"desk": "app:autoradio", "record_kind": "autoradio.air"}
		},
	} {
		forged := claim
		forged.Calibration = &types.CommitmentCalibration{Schema: claim.Calibration.Schema, Agent: claim.Calibration.Agent}
		cl := *claim.Calibration.Claim
		cl.Tags = append([]string(nil), cl.Tags...)
		forged.Calibration.Claim = &cl
		forged.SubjectRefs = map[string]string{"desk": "app:autoradio", "record_kind": "autoradio.forecast"}
		forged.RecordID = "calib:autoradio-dj:forged-" + strings.ReplaceAll(name, " ", "-")
		mutate(&forged)
		mm := newMemLedger()
		mm.records[forged.RecordID] = forged
		if _, err := SettleAutoradio(ctx, mm, "o", "c", forged.RecordID, AutoradioHeardRule, true, "ev-1", at); err == nil {
			t.Fatalf("%s: settled", name)
		}
		if len(mm.appended) != 0 {
			t.Fatalf("%s: wrote %v", name, mm.appended)
		}
	}

	// A generic claim cannot be settled through the reserved path.
	generic := claimRecord(t, "calib:plain:9", "claude-code/lead", "mission")
	m.records[generic.RecordID] = generic
	if _, err := SettleAutoradio(ctx, m, "o", "c", generic.RecordID, AutoradioHeardRule, true, "ev-1", at); err == nil {
		t.Fatal("generic claim settled by the reserved rule")
	}
	// Nothing above wrote.
	if len(m.appended) != 0 {
		t.Fatalf("refusals wrote %v", m.appended)
	}
}

func dropPrefix(tags []string, prefix string) []string {
	var out []string
	for _, tag := range tags {
		if !strings.HasPrefix(tag, prefix) {
			out = append(out, tag)
		}
	}
	return out
}

// ---- SplitBlocks ----

func TestSplitBlocksEdgeTexts(t *testing.T) {
	for _, tc := range []struct {
		name, text string
		want       []string
	}{
		{"one paragraph", "Only one paragraph here.", []string{"Only one paragraph here."}},
		{"trailing newline", "A first.\n\nA second.\n", []string{"A first.", "A second."}},
		{"crlf", "A first.\r\n\r\nA second.\r\n\r\nA third.", []string{"A first.", "A second.", "A third."}},
		{"blank lines with spaces", "A first.\n   \t \nA second.", []string{"A first.", "A second."}},
		{"leading blank lines", "\n\n\nA first.\n\nA second.", []string{"A first.", "A second."}},
		{"many blank lines", "A first.\n\n\n\n\nA second.", []string{"A first.", "A second."}},
		{"single newline joins", "Line one\nline two.\n\nA second.", []string{"Line one\nline two.", "A second."}},
		{"lone cr", "A first.\r\rA second.", []string{"A first.", "A second."}},
		{"empty", "", nil},
		{"only blanks", " \n\n \n", nil},
	} {
		got := SplitBlocks(tc.text)
		if !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("%s: SplitBlocks = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func sha(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}
