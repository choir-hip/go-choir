package types

// MA1.3, the request record. Ways this could fail, written first:
//
//  1. A request is minted without p_success, weight or settlement, or with a
//     p_success outside [0,1] (NaN, negative, above one).
//  2. A cell sets provenance, origin or the materiality procedure by passing
//     them in the request fields, or smuggles an unknown field past the parse.
//  3. A request of an impact class of I1 or above carries weight 0 (or no
//     weight), so a trivial weight hides a material act. A cast, whose
//     catalogue floor is I1, is refused at weight 0 even when it states no class.
//  4. The materiality procedure is chosen by the committing desk, or its hash
//     does not change when the frozen weight changes, or when the ref does.
//  5. A kind with no procedure in the catalogue is scored as zero instead of
//     recorded as having none.
//  6. A record written before the slice (no p_success, weight, settlement,
//     origin; directive subtype "note") fails to decode, fails the write-side
//     range check, or loses its fields on a round trip.
//  7. The retired "note" subtype is writable again, or a platform notice is
//     no longer recognised as one.
//  8. A request kind's wire shape changes without a golden changing (the
//     fixtures below pin every request kind).

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fptr(f float64) *float64 { return &f }

func goodFields() *RequestFields {
	return &RequestFields{PSuccess: fptr(0.8), Weight: fptr(0.3), Settlement: SettlementRecipient}
}

// goldenRequestRecord builds the fixture of one request kind, as the reducer
// stamps it.
func goldenRequestRecord(t *testing.T, kind string) CommitmentRecord {
	t.Helper()
	rec := CommitmentRecord{
		SchemaID: CommitmentRecordSchemaV1, RecordID: "cell-1:" + kind + ":tray-1", Discrepancy: DiscrepancyUnresolved,
		Provenance:  CommitmentProvenance{AgentID: "texture:doc-1", ModelID: "prov/model", CommittedAt: "2026-11-04T10:00:00Z", Desk: "texture:doc-1", RunID: "run-1", Origin: OriginDeskRun},
		EmitterKind: CommitmentEmitterAgent, Inbox: "management:o", SenderInbox: "texture:doc-1", Addressee: "management:o",
	}
	switch kind {
	case RequestCast:
		rec.Kind, rec.Directive = CommitmentKindDirective, &CommitmentDirective{Subtype: CommitmentDirectiveCast, Objective: "publish the page", Body: `{"kind":"execution_request"}`}
	case RequestEscalate:
		rec.Kind, rec.Directive = CommitmentKindDirective, &CommitmentDirective{Subtype: CommitmentDirectiveEscalate, Body: "the build is blocked"}
	case RequestFeed:
		rec.Inbox = "texture:doc-2"
		rec.Kind, rec.Directive = CommitmentKindDirective, &CommitmentDirective{Subtype: CommitmentDirectiveFeed, Body: "adds the diagnosis\n\nFed captures: cap-1"}
		rec.Disposition = &CommitmentDisposition{Kind: DispositionFeed, Captures: []string{"cap-1"}, Reason: "adds the diagnosis"}
	case RequestDecline:
		rec.Inbox = "source:rss"
		rec.Kind, rec.Directive = CommitmentKindDirective, &CommitmentDirective{Subtype: CommitmentDirectiveDecline, Body: "no news event\n\nDeclined captures: cap-3"}
		rec.Disposition = &CommitmentDisposition{Kind: DispositionDecline, Captures: []string{"cap-3"}, Reason: "no news event"}
	case RequestAsk:
		rec.Kind = CommitmentKindPrecommit
		rec.Precommit = &CommitmentPrecommit{Question: "which story?", Resolver: "management:o"}
		rec.Prediction = CommitmentPrediction{Hypothesis: "which story?", Questions: []TypedQuestion{{Question: "which story?"}}, CommittedAt: "2026-11-04T10:00:00Z"}
	default:
		t.Fatalf("no fixture for %s", kind)
	}
	f := goodFields()
	if kind == RequestAsk || kind == RequestFeed || kind == RequestDecline {
		f.Weight = fptr(0)
	}
	if err := rec.StampRequest(kind, f); err != nil {
		t.Fatalf("stamp %s: %v", kind, err)
	}
	return rec
}

// TestRequestRecordGoldens pins the wire shape of every request kind. Run with
// UPDATE_GOLDEN=1 to rewrite; a changed shape is a reviewed change.
func TestRequestRecordGoldens(t *testing.T) {
	for _, kind := range []string{RequestCast, RequestAsk, RequestEscalate, RequestFeed, RequestDecline} {
		t.Run(kind, func(t *testing.T) {
			rec := goldenRequestRecord(t, kind)
			got, err := json.MarshalIndent(rec, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			got = append(got, '\n')
			path := filepath.Join("testdata", "request_records", kind+".golden.json")
			if os.Getenv("UPDATE_GOLDEN") != "" {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, got, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("golden missing (UPDATE_GOLDEN=1 writes it): %v", err)
			}
			if string(want) != string(got) {
				t.Fatalf("%s wire shape changed:\n got %s\nwant %s", kind, got, want)
			}
			// The golden decodes back, with the frozen hash intact.
			var back CommitmentRecord
			if err := json.Unmarshal(want, &back); err != nil {
				t.Fatal(err)
			}
			if back.PSuccess == nil || *back.PSuccess != 0.8 || back.Weight == nil || back.Settlement != SettlementRecipient ||
				back.MaterialityProcedure == nil || !back.MaterialityProcedure.Verify() || back.Provenance.Origin != OriginDeskRun {
				t.Fatalf("golden %s does not read back whole: %+v", kind, back)
			}
			if err := ValidateRequestRecord(back); err != nil {
				t.Fatalf("golden %s fails the write-side check: %v", kind, err)
			}
		})
	}
}

func TestRequestFieldsRefuseMissingAndOutOfRange(t *testing.T) {
	cases := []struct {
		name string
		f    *RequestFields
		want string
	}{
		{"nil", nil, "p_success, weight and settlement"},
		{"no p_success", &RequestFields{Weight: fptr(0.3), Settlement: "recipient"}, "p_success is required"},
		{"negative p", &RequestFields{PSuccess: fptr(-0.1), Weight: fptr(0.3), Settlement: "recipient"}, "outside [0,1]"},
		{"p above one", &RequestFields{PSuccess: fptr(1.01), Weight: fptr(0.3), Settlement: "recipient"}, "outside [0,1]"},
		{"p NaN", &RequestFields{PSuccess: fptr(math.NaN()), Weight: fptr(0.3), Settlement: "recipient"}, "outside [0,1]"},
		{"no weight", &RequestFields{PSuccess: fptr(0.5), Settlement: "recipient"}, "weight is required"},
		{"weight above one", &RequestFields{PSuccess: fptr(0.5), Weight: fptr(2), Settlement: "recipient"}, "outside [0,1]"},
		{"no settlement", &RequestFields{PSuccess: fptr(0.5), Weight: fptr(0.3)}, "settlement is required"},
		{"bad settlement", &RequestFields{PSuccess: fptr(0.5), Weight: fptr(0.3), Settlement: "vibes"}, "neither"},
		{"bad class", &RequestFields{PSuccess: fptr(0.5), Weight: fptr(0.3), Settlement: "recipient", ImpactClass: "I9"}, "I0..I4"},
	}
	for _, c := range cases {
		err := ValidateRequestFields(RequestCast, c.f)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want it to contain %q", c.name, err, c.want)
		}
	}
	for _, kind := range []string{RequestCast, RequestAsk, RequestEscalate, RequestFeed, RequestDecline} {
		if err := ValidateRequestFields(kind, goodFields()); err != nil {
			t.Errorf("%s refused a good request: %v", kind, err)
		}
	}
	// A rule kind settles too.
	f := goodFields()
	f.Settlement = string(PrecommitRuleVerifyPass)
	if err := ValidateRequestFields(RequestAsk, f); err != nil {
		t.Errorf("a rule-kind settlement was refused: %v", err)
	}
}

func TestParseRequestFieldsRefusesWhatOnlyTheReducerStamps(t *testing.T) {
	for _, key := range []string{"provenance", "origin", "origin_class", "materiality_procedure", "agent_id", "model_id", "desk", "run_id", "hash"} {
		raw := `{"p_success":0.8,"weight":0.3,"settlement":"recipient","` + key + `":{"origin":"owner"}}`
		if _, err := ParseRequestFields(raw); err == nil || !strings.Contains(err.Error(), "stamped by the reducer") {
			t.Errorf("a cell set %q: err = %v", key, err)
		}
	}
	if _, err := ParseRequestFields(`{"p_success":0.8,"weight":0.3,"settlement":"recipient","extra":1}`); err == nil {
		t.Error("an unknown field was accepted")
	}
	if _, err := ParseRequestFields(``); err == nil {
		t.Error("an empty argument was accepted")
	}
	if _, err := ParseRequestFields(`[1]`); err == nil {
		t.Error("a non-object was accepted")
	}
	if f, err := ParseRequestFields(`{"p_success":0.8,"weight":0.3,"settlement":"recipient","impact_class":"I2"}`); err != nil || f.ImpactClass != ImpactI2 {
		t.Errorf("a good argument: %+v, %v", f, err)
	}
}

func TestWeightZeroOnAnI1ClassIsRefused(t *testing.T) {
	// Stated I2 with weight 0: refused for any kind.
	for _, kind := range []string{RequestAsk, RequestFeed, RequestPrecommit} {
		if err := ValidateImpactWeight(kind, ImpactI2, fptr(0)); err == nil {
			t.Errorf("%s at I2 with weight 0 was accepted", kind)
		}
		if err := ValidateImpactWeight(kind, ImpactI2, nil); err == nil {
			t.Errorf("%s at I2 with no weight was accepted", kind)
		}
		if err := ValidateImpactWeight(kind, ImpactI1, fptr(0.1)); err != nil {
			t.Errorf("%s at I1 with weight 0.1 refused: %v", kind, err)
		}
		if err := ValidateImpactWeight(kind, ImpactI0, fptr(0)); err != nil {
			t.Errorf("%s at I0 with weight 0 refused: %v", kind, err)
		}
	}
	// A cast's catalogue floor is I1: weight 0 is refused with no class stated,
	// and stating I0 does not lower the floor.
	for _, class := range []ImpactClass{"", ImpactI0} {
		f := goodFields()
		f.Weight, f.ImpactClass = fptr(0), class
		if err := ValidateRequestFields(RequestCast, f); err == nil || !strings.Contains(err.Error(), "weight 0 is refused") {
			t.Errorf("cast at stated class %q with weight 0: err = %v", class, err)
		}
		if err := ValidateRequestFields(RequestDecline, f); err != nil {
			t.Errorf("a decline (floor I0) at weight 0 was refused: %v", err)
		}
	}
}

func TestMaterialityFreezeIsHashedWithTheProcedureAndTheWeights(t *testing.T) {
	a, ok := FreezeMateriality(RequestCast, 0.3, "")
	if !ok || !a.Verify() || a.ProcedureRef != "request.cast.v1" || a.Custodian != "management" || a.NoProcedure {
		t.Fatalf("cast freeze = %+v", a)
	}
	b, _ := FreezeMateriality(RequestCast, 0.31, "")
	if a.Hash == b.Hash {
		t.Error("the hash ignores the frozen weight")
	}
	c := a
	c.ProcedureRef = "request.cast.v2"
	if c.Digest() == a.Hash {
		t.Error("the hash ignores the procedure ref")
	}
	c = a
	c.Weight = 0.9
	if c.Verify() {
		t.Error("a tampered weight still verifies")
	}
	// Deterministic.
	if a2, _ := FreezeMateriality(RequestCast, 0.3, ""); a2.Hash != a.Hash {
		t.Error("the freeze is not deterministic")
	}
	// A kind with no procedure says so: it is not scored zero.
	n, ok := FreezeMateriality(RequestDecline, 0, "")
	if !ok || !n.NoProcedure || n.ProcedureRef != "" || !n.Verify() {
		t.Fatalf("decline freeze = %+v", n)
	}
	if _, ok := FreezeMateriality("gossip", 0.1, ""); ok {
		t.Error("an uncatalogued kind froze")
	}
	// Every catalogue row names a custodian.
	for _, e := range MaterialityCatalogue {
		if e.Custodian == "" {
			t.Errorf("catalogue row %s has no custodian", e.Kind)
		}
	}
}

// TestOldRecordsStillDecodeAndPassTheWriteCheck: a record written before the
// slice has no request fields and the retired "note" subtype.
func TestOldRecordsStillDecodeAndPassTheWriteCheck(t *testing.T) {
	old := `{"schema_id":"commitment_record.v1","record_id":"cell-9:note:tray-1","prediction":{"hypothesis":"","committed_at":""},
"observation":{"excerpt":"","source_ref":"","observed_at":""},"scores":null,"discrepancy":"unresolved",
"provenance":{"agent_id":"management:o","model_id":"m","committed_at":"2026-10-01T00:00:00Z"},
"kind":"directive","directive":{"subtype":"note","body":"declined cap-9"},"inbox":"source:rss","sender_inbox":"management:o"}`
	var rec CommitmentRecord
	if err := json.Unmarshal([]byte(old), &rec); err != nil {
		t.Fatal(err)
	}
	if rec.PSuccess != nil || rec.Weight != nil || rec.Settlement != "" || rec.MaterialityProcedure != nil || rec.Provenance.Origin != "" {
		t.Fatalf("absent fields decoded as present: %+v", rec)
	}
	if rec.RecordKind() != CommitmentKindDirective || !rec.Directive.Subtype.IsNotice() {
		t.Fatalf("old note not read as a notice: %+v", rec.Directive)
	}
	if err := ValidateRequestRecord(rec); err != nil {
		t.Fatalf("an old record fails the write-side check: %v", err)
	}
	if rec.OriginClass() != "unstamped" {
		t.Fatalf("origin class = %q", rec.OriginClass())
	}
	// An old record re-encodes without gaining request fields.
	raw, _ := json.Marshal(rec)
	for _, k := range []string{"p_success", "weight", "settlement", "materiality_procedure", `"origin"`} {
		if strings.Contains(string(raw), k) {
			t.Errorf("an old record gained %s on re-encode: %s", k, raw)
		}
	}
	// The retired subtype is not writable; the notice subtype is.
	if IsValidDirectiveSubtype(CommitmentDirectiveSubtype("note")) {
		t.Error("the retired note subtype is writable")
	}
	for _, s := range []CommitmentDirectiveSubtype{CommitmentDirectiveNotice, CommitmentDirectiveCast, CommitmentDirectiveEscalate,
		CommitmentDirectiveFeed, CommitmentDirectiveDecline, CommitmentDirectiveRetract} {
		if !IsValidDirectiveSubtype(s) {
			t.Errorf("%s is not writable", s)
		}
	}
	// A system notice built today is recognised as a notice.
	if !CommitmentDirectiveNotice.IsNotice() {
		t.Error("the notice subtype is not a notice")
	}
}

func TestWriteCheckRefusesOutOfRangeFieldsOnARecord(t *testing.T) {
	rec := goldenRequestRecord(t, RequestCast)
	rec.PSuccess = fptr(1.5)
	if err := ValidateRequestRecord(rec); err == nil {
		t.Error("p_success 1.5 passed the record check")
	}
	rec = goldenRequestRecord(t, RequestCast)
	rec.Weight = fptr(0)
	if err := ValidateRequestRecord(rec); err == nil {
		t.Error("a cast at weight 0 passed the record check")
	}
	rec = goldenRequestRecord(t, RequestCast)
	rec.Provenance.Origin = "wizard"
	if err := ValidateRequestRecord(rec); err == nil {
		t.Error("an unknown origin class passed the record check")
	}
	rec = goldenRequestRecord(t, RequestCast)
	rec.Settlement = "vibes"
	if err := ValidateRequestRecord(rec); err == nil {
		t.Error("an unknown settlement passed the record check")
	}
}
