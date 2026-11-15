package calibration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"
)

// Failure list for the wire fixtures (design 12.7), written before the
// fixtures: the phone station builds against them, so a fixture that the
// design's own rules do not produce is a defect the phone would encode.
//  1. a fixture file is missing, is not JSON, or lacks a case the design
//     names (each event type, each result code, a one-block finish, a
//     first-block skip, a wrong block count, a hash mismatch in both forms, a
//     conflicting second batch in both arrival orders, a repeat cut, an air
//     record with no settlement then a replay, a batch over the bound, the
//     edge texts, a title with a control character);
//  2. the expected codes or settlements of a heard scenario differ from what
//     the rules of 12.3 and 4.2 give (checked by a reference evaluation here:
//     the heard route itself is station L2);
//  3. the edge texts' block lists differ from SplitBlocks, or a scenario's
//     block count from its text's;
//  4. a stretch-line case lacks a valid and an invalid case for a validator row
//     of 3.2, or a lesson-line case lacks one.

const fixtureDir = "../../mobile/fixtures/learning"

func readFixture(t *testing.T, name string, into any) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(fixtureDir, name))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, into); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
}

func TestFixtureBlocksMatchSplitBlocks(t *testing.T) {
	var f struct {
		Cases []struct {
			Name       string   `json:"name"`
			Text       string   `json:"text"`
			Blocks     []string `json:"blocks"`
			BlockCount int      `json:"block_count"`
		} `json:"cases"`
	}
	readFixture(t, "blocks.json", &f)
	if len(f.Cases) < 8 {
		t.Fatalf("%d edge texts", len(f.Cases))
	}
	names := map[string]bool{}
	for _, c := range f.Cases {
		names[c.Name] = true
		got := SplitBlocks(c.Text)
		if len(got) != len(c.Blocks) || len(got) != c.BlockCount {
			t.Fatalf("%s: SplitBlocks gives %d blocks, fixture %d (count %d)", c.Name, len(got), len(c.Blocks), c.BlockCount)
		}
		for i := range got {
			if got[i] != c.Blocks[i] {
				t.Fatalf("%s: block %d = %q, fixture %q", c.Name, i, got[i], c.Blocks[i])
			}
		}
	}
	for _, want := range []string{"trailing_newline", "crlf", "blank_lines_with_spaces", "leading_blank_lines", "one_paragraph"} {
		if !names[want] {
			t.Fatalf("edge text %s missing", want)
		}
	}
}

// ---- reference evaluation of design 12.3 and 4.2, test-only ----

type fxEvent struct {
	EventID    string   `json:"event_id"`
	SessionID  string   `json:"session_id"`
	StretchID  string   `json:"stretch_id"`
	SegmentID  string   `json:"segment_id"`
	TextSHA256 string   `json:"text_sha256"`
	Seq        int      `json:"seq"`
	Type       string   `json:"type"`
	LastBlock  int      `json:"last_block"`
	BlockCount int      `json:"block_count"`
	StartedAt  string   `json:"started_at"`
	EndedAt    string   `json:"ended_at"`
	TurnID     string   `json:"turn_id,omitempty"`
	Cites      []string `json:"cites,omitempty"`
}

type fxResult struct {
	EventID string `json:"event_id"`
	Code    string `json:"code"`
	Reason  string `json:"reason,omitempty"`
}

type fxGenerated struct {
	Count        int     `json:"count"`
	IDPrefix     string  `json:"id_prefix"`
	Template     fxEvent `json:"template"`
	CitesPadByte int     `json:"cites_pad_bytes"`
}

type fxBatch struct {
	HTTPStatus int          `json:"http_status"`
	Events     []fxEvent    `json:"events"`
	Generated  *fxGenerated `json:"events_generated"`
	Results    []fxResult   `json:"results"`
	ResultsAll string       `json:"results_all"`
}

type fxScenario struct {
	Name     string            `json:"name"`
	About    string            `json:"about"`
	Segments map[string]string `json:"segments"`
	Pre      struct {
		AirWithoutSettlement []fxEvent `json:"air_without_settlement"`
	} `json:"pre"`
	Batches     []fxBatch         `json:"batches"`
	Settlements map[string]string `json:"settlements"`
}

type fxFile struct {
	Texts map[string]struct {
		Text   string `json:"text"`
		SHA256 string `json:"sha256"`
		Blocks int    `json:"blocks"`
	} `json:"texts"`
	Scenarios []fxScenario `json:"scenarios"`
}

var fxHex64 = regexp.MustCompile(`^[0-9a-f]{64}$`)

var fxEventID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$`)

type fxClaim struct {
	blocks int
	sha32  string
}

type fxState struct {
	claims  map[string]fxClaim
	stored  map[string]fxEvent
	order   []fxEvent
	settled map[string]string
}

func fxOutcome(e fxEvent, blocks int) string {
	if e.Type == "finished" || e.LastBlock == blocks-1 {
		return "yes"
	}
	return "no"
}

func fxTerminal(typ string) bool { return typ == "finished" || typ == "skipped" }

func (st *fxState) settleFromStored(seg string) {
	if _, ok := st.settled[seg]; ok {
		return
	}
	for _, e := range st.order {
		if e.SegmentID == seg && fxTerminal(e.Type) {
			st.settled[seg] = fxOutcome(e, st.claims[seg].blocks)
			return
		}
	}
}

func (st *fxState) one(e fxEvent) fxResult {
	r := fxResult{EventID: e.EventID}
	refuse := func(reason string) fxResult { r.Code, r.Reason = "refused", reason; return r }
	if !fxEventID.MatchString(e.EventID) {
		return refuse("bad_event_id")
	}
	if prev, ok := st.stored[e.EventID]; ok {
		if reflect.DeepEqual(prev, e) {
			if e.SegmentID != "" {
				st.settleFromStored(e.SegmentID)
			}
			r.Code = "duplicate"
			return r
		}
		r.Code = "changed"
		return r
	}
	switch e.Type {
	case "finished", "skipped", "cut", "resume", "paused", "left", "more", "less", "corrected":
	default:
		return refuse("unknown_type")
	}
	started, err1 := time.Parse(time.RFC3339, e.StartedAt)
	ended, err2 := time.Parse(time.RFC3339, e.EndedAt)
	if err1 != nil || err2 != nil {
		return refuse("bad_timestamp")
	}
	if ended.Before(started) {
		return refuse("ended_before_started")
	}
	signal := e.Type == "more" || e.Type == "less" || e.Type == "corrected"
	var claim fxClaim
	if e.SegmentID != "" {
		c, ok := st.claims[e.SegmentID]
		if !ok || e.StretchID != "fx" {
			return refuse("no_forecast")
		}
		claim = c
		if !signal {
			if h := strings.ToLower(e.TextSHA256); !fxHex64.MatchString(h) || h[:32] != claim.sha32 {
				return refuse("hash_mismatch")
			}
			if e.BlockCount == 0 {
				return refuse("block_count_zero")
			}
			if e.BlockCount != claim.blocks {
				return refuse("block_count_mismatch")
			}
			if e.LastBlock < 0 || e.LastBlock >= claim.blocks {
				return refuse("last_block_out_of_range")
			}
			if e.Type == "finished" && e.LastBlock != claim.blocks-1 {
				return refuse("finished_before_final_block")
			}
		}
	}
	st.stored[e.EventID] = e
	st.order = append(st.order, e)
	if signal || e.SegmentID == "" {
		r.Code = "accepted"
		return r
	}
	cur, settled := st.settled[e.SegmentID]
	switch {
	case !settled && fxTerminal(e.Type):
		st.settled[e.SegmentID] = fxOutcome(e, claim.blocks)
		r.Code = "accepted"
	case !settled:
		r.Code = "accepted"
	case fxTerminal(e.Type) && fxOutcome(e, claim.blocks) != cur:
		r.Code = "conflict-recorded"
	default:
		r.Code = "ignored"
	}
	return r
}

// batch returns the status and the results in the batch's own order.
func (st *fxState) batch(events []fxEvent) (int, []fxResult) {
	raw, _ := json.Marshal(events)
	if len(events) > 200 || len(raw) > 64<<10 {
		return 400, nil
	}
	idx := make([]int, len(events))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool {
		x, y := events[idx[a]], events[idx[b]]
		if x.StretchID != y.StretchID {
			return x.StretchID < y.StretchID
		}
		if x.SegmentID != y.SegmentID {
			return x.SegmentID < y.SegmentID
		}
		return x.Seq < y.Seq
	})
	results := make([]fxResult, len(events))
	for _, i := range idx {
		results[i] = st.one(events[i])
	}
	return 200, results
}

func TestHeardFixturesMatchTheRulesOfThe12Dot3(t *testing.T) {
	var f fxFile
	readFixture(t, "heard_batches.json", &f)
	for name, tx := range f.Texts {
		if got := len(SplitBlocks(tx.Text)); got != tx.Blocks || sha(tx.Text) != tx.SHA256 {
			t.Fatalf("text %s: blocks %d (fixture %d) or hash differs", name, got, tx.Blocks)
		}
	}
	codes, types_ := map[string]bool{}, map[string]bool{}
	reasons := map[string]bool{}
	for _, sc := range f.Scenarios {
		st := &fxState{claims: map[string]fxClaim{}, stored: map[string]fxEvent{}, settled: map[string]string{}}
		for seg, text := range sc.Segments {
			tx := f.Texts[text]
			st.claims[seg] = fxClaim{blocks: len(SplitBlocks(tx.Text)), sha32: tx.SHA256[:32]}
		}
		for _, e := range sc.Pre.AirWithoutSettlement {
			st.stored[e.EventID] = e
			st.order = append(st.order, e)
		}
		for bi, b := range sc.Batches {
			events := b.Events
			if g := b.Generated; g != nil {
				for i := 0; i < g.Count; i++ {
					e := g.Template
					e.EventID = g.IDPrefix + itoa(i)
					if g.CitesPadByte > 0 {
						e.Cites = append(append([]string(nil), e.Cites...), strings.Repeat("x", g.CitesPadByte))
					}
					events = append(events, e)
				}
			}
			status, results := st.batch(events)
			if status != b.HTTPStatus {
				t.Fatalf("%s batch %d: status %d, fixture %d", sc.Name, bi, status, b.HTTPStatus)
			}
			want := b.Results
			if b.ResultsAll != "" {
				want = nil
				for _, e := range events {
					want = append(want, fxResult{EventID: e.EventID, Code: b.ResultsAll})
				}
			}
			if !reflect.DeepEqual(results, want) && !(len(results) == 0 && len(want) == 0) {
				t.Fatalf("%s batch %d: results %+v, fixture %+v", sc.Name, bi, results, want)
			}
			for _, r := range want {
				codes[r.Code] = true
				if r.Reason != "" {
					reasons[r.Reason] = true
				}
			}
			for _, e := range events {
				types_[e.Type] = true
			}
		}
		if len(st.settled) != len(sc.Settlements) {
			t.Fatalf("%s: settlements %v, fixture %v", sc.Name, st.settled, sc.Settlements)
		}
		for seg, want := range sc.Settlements {
			if st.settled[seg] != want {
				t.Fatalf("%s: settlement of %s = %q, fixture %q", sc.Name, seg, st.settled[seg], want)
			}
		}
	}
	for _, code := range []string{"accepted", "duplicate", "ignored", "conflict-recorded", "changed", "refused"} {
		if !codes[code] {
			t.Fatalf("no fixture for result code %s", code)
		}
	}
	for _, typ := range []string{"finished", "skipped", "cut", "resume", "paused", "left", "more", "less", "corrected"} {
		if !types_[typ] {
			t.Fatalf("no fixture for event type %s", typ)
		}
	}
	for _, reason := range []string{"hash_mismatch", "block_count_mismatch", "block_count_zero", "last_block_out_of_range", "finished_before_final_block", "no_forecast", "unknown_type", "ended_before_started", "bad_event_id"} {
		if !reasons[reason] {
			t.Fatalf("no fixture for refusal %s", reason)
		}
	}
	names := map[string]bool{}
	for _, sc := range f.Scenarios {
		names[sc.Name] = true
	}
	for _, want := range []string{"one_block_finish", "first_block_skip_many_blocks", "first_block_skip_one_block", "wrong_block_count", "hash_mismatch_full", "hash_32_digit_form",
		"conflict_finish_then_skip", "conflict_skip_then_finish", "non_terminal_then_finish", "air_without_settlement_then_replay", "batch_over_event_bound", "one_batch_order_a", "one_batch_order_b",
		"hash_uppercase_accepted", "hash_not_hex", "timestamps_with_offsets", "signal_checks_are_the_forecast_check_only"} {
		if !names[want] {
			t.Fatalf("scenario %s missing", want)
		}
	}
}

func itoa(i int) string {
	b, _ := json.Marshal(i)
	return string(b)
}

func TestStretchLineFixturesCoverTheValidators(t *testing.T) {
	var f struct {
		Input        map[string]any `json:"input"`
		SegmentLines []struct {
			Name      string         `json:"name"`
			Validator string         `json:"validator"`
			Valid     bool           `json:"valid"`
			Line      map[string]any `json:"line"`
		} `json:"segment_lines"`
		LessonLines []struct {
			Name      string         `json:"name"`
			Validator string         `json:"validator"`
			Valid     bool           `json:"valid"`
			Line      map[string]any `json:"line"`
		} `json:"lesson_lines"`
		Runs []struct {
			Sequence     []string `json:"sequence"`
			EndsAfterIdx *int     `json:"ends_after_index"`
		} `json:"runs"`
	}
	readFixture(t, "stretch_lines.json", &f)
	if f.Input["candidates"] == nil {
		t.Fatal("no input")
	}
	valid, invalid := map[string]bool{}, map[string]bool{}
	for _, l := range f.SegmentLines {
		if l.Line == nil || l.Name == "" || l.Validator == "" {
			t.Fatalf("malformed case %+v", l)
		}
		if l.Valid {
			valid[l.Validator] = true
		} else {
			invalid[l.Validator] = true
		}
	}
	for _, v := range []string{"kind known", "text non-empty", "every cite in the candidate set", "content kind has p_heard and caused_by", "title one line, no control character",
		"title at most 200 bytes", "p_heard in [0,1]", "caused_by resolves", "same-response lesson cite", "ident, bridge, answer, result carry no forecast"} {
		if !invalid[v] {
			t.Fatalf("no invalid case for validator %q", v)
		}
	}
	for _, v := range []string{"p_heard in [0,1]", "caused_by resolves", "ident, bridge, answer, result carry no forecast", "all"} {
		if !valid[v] {
			t.Fatalf("no valid case for validator %q", v)
		}
	}
	hasValid, hasInvalid := false, false
	for _, l := range f.LessonLines {
		if l.Valid {
			hasValid = true
		} else {
			hasInvalid = true
		}
	}
	if !hasValid || !hasInvalid || len(f.LessonLines) < 8 {
		t.Fatal("lesson-line cases")
	}
	// The title control character case really holds one.
	found := false
	for _, l := range f.SegmentLines {
		if s, _ := l.Line["title"].(string); strings.ContainsAny(s, "\x07") {
			found = true
		}
	}
	if !found {
		t.Fatal("no title with a control character")
	}
	if len(f.Runs) != 2 || f.Runs[0].EndsAfterIdx != nil || f.Runs[1].EndsAfterIdx == nil {
		t.Fatalf("runs = %+v", f.Runs)
	}
}
