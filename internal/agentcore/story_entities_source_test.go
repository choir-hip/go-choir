package agentcore

// Relate's source rules and the export of a story's entities
// (docs/problems/related-stories-no-entity-layer-2026-10-26.md, slice c0
// §5 and §6, slice c "unpublished stories", review 304 item 8). When the
// run has its story's analysis, every attachment has a source: existing (a
// candidate the lexicon found), extracted (a phrase the analysis typed), or
// research_named (a name the analysis missed). Ways it can fail, written
// first:
//
//  S1 A mint's type is the cell's: the type of an extracted phrase is read
//     host-side from the analysis's type answer; another type is an
//     override that needs a reason.
//  S2 A phrase System 1 judged "not an entity" or "none of these fit" is
//     minted silently: either needs a reason; "none of these fit" counts a
//     category gap named by the reason.
//  S3 Research names entities freely: a name the analysis missed needs the
//     novelty answer's p(yes) of at least 0.35, or a reason.
//  S4 The counts by source are lost: the record carries a review with them,
//     the novelty, the not-entities and the alias candidates of what was
//     attached, and the export sends it under the record's id.
//  S5 The rules reach a call with no analysis (another story): its old
//     rules hold.
//  F6 A story not published yet cannot be recorded, or is exported before
//     it publishes, or never after: the check is pending (validate-only,
//     every entity check), the write's refusal is "pending", and publishing
//     the story exports the tape's records.
//  F7 The export at publish writes a role twice or flips a newer role back:
//     it sends each entity once, with the tape's latest role.

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

func (f *analysisFixture) relateEntities(t *testing.T, runID string, entities ...yaegikernel.RelateEntity) (attachDecoded, error) {
	t.Helper()
	payload, _ := json.Marshal(yaegikernel.RelatePayload{Entities: entities})
	raw, err := f.rt.deskEgress(context.Background(), agentprofile.Research, f.deskExec(runID, "doc-story"), yaegikernel.ActionRelate, payload)
	if err != nil {
		return attachDecoded{}, err
	}
	return decodeAttach(t, raw), nil
}

func (f *analysisFixture) records(t *testing.T) []storyEntitiesRecord {
	t.Helper()
	var out []storyEntitiesRecord
	for _, e := range f.entityEvents(t) {
		var rec storyEntitiesRecord
		if err := json.Unmarshal(e.Payload, &rec); err != nil {
			t.Fatal(err)
		}
		out = append(out, rec)
	}
	return out
}

func phraseMint(c yaegikernel.StoryAnalysisCandidate, typ, reason string) yaegikernel.RelateEntity {
	return yaegikernel.RelateEntity{Name: c.Phrase, Type: typ, Role: types.EntityRoleCentral, JudgmentRef: c.Role.ReceiptRef, QuestionID: c.Role.QuestionID, Reason: reason}
}

const sourceReason = "the story names it as the maker of the model throughout"

func TestRelateMintOfAPhraseTakesItsTypeFromTheAnalysis(t *testing.T) { // S1
	f := newAnalysisFixture(t)
	f.turnOn(t)
	a := f.analyse(t, agentprofile.Research, "run-s1", "doc-story")
	anthropic, nato := a.Candidates[1], a.Candidates[2]
	if anthropic.Kind == nil || anthropic.Kind.Top != types.EntityTypePerson {
		t.Fatalf("Anthropic's type answer = %+v", anthropic.Kind)
	}
	if _, err := f.relateEntities(t, "run-s1", phraseMint(anthropic, types.EntityTypePerson, "")); err != nil {
		t.Fatalf("mint at System 1's top type: %v", err)
	}
	if _, err := f.relateEntities(t, "run-s1", phraseMint(nato, types.EntityTypeOrganization, "")); err == nil || !strings.Contains(err.Error(), "type") || !strings.Contains(err.Error(), nato.Kind.QuestionID) {
		t.Fatalf("mint at another type with no reason: %v, want a refusal naming the type answer", err)
	}
	if _, err := f.relateEntities(t, "run-s1", phraseMint(nato, types.EntityTypeOrganization, sourceReason)); err != nil {
		t.Fatalf("mint at another type with a reason: %v", err)
	}
	recs := f.records(t)
	if len(recs) != 2 {
		t.Fatalf("records = %d, want 2", len(recs))
	}
	first, second := recs[0].Attachments[0], recs[1].Attachments[0]
	if first.Source != types.EntityCandidateExtracted || first.TypeTop != types.EntityTypePerson || first.TypeOverrode || first.TypeQuestionID != anthropic.Kind.QuestionID || first.TypeJudgmentRef != anthropic.Kind.ReceiptRef {
		t.Fatalf("first attachment = %+v, want extracted at its top type with the type basis", first)
	}
	if second.Source != types.EntityCandidateExtracted || !second.TypeOverrode || second.TypeTop != types.EntityTypePerson {
		t.Fatalf("second attachment = %+v, want an extracted type override", second)
	}
	if recs[1].Review == nil || recs[1].Review.Counts.Overridden != 1 {
		t.Fatalf("second review = %+v, want one override", recs[1].Review)
	}
}

func TestRelateMintAgainstNotAnEntityOrNoneFitNeedsAReason(t *testing.T) { // S2
	f := newAnalysisFixture(t)
	f.turnOn(t)
	f.judge.prefer = map[string]string{"type-2": analysisNoneFit, "type-3": analysisNotAnEntity}
	a := f.analyse(t, agentprofile.Research, "run-s2", "doc-story")
	anthropic, nato := a.Candidates[1], a.Candidates[2]
	for _, c := range []yaegikernel.StoryAnalysisCandidate{anthropic, nato} {
		if _, err := f.relateEntities(t, "run-s2", phraseMint(c, types.EntityTypeOrganization, "")); err == nil || !strings.Contains(err.Error(), c.Kind.Top) {
			t.Fatalf("%s at top %q with no reason: %v, want a refusal naming the answer", c.Phrase, c.Kind.Top, err)
		}
	}
	gap := "an AI laboratory, a kind the list lacks"
	if _, err := f.relateEntities(t, "run-s2", phraseMint(anthropic, types.EntityTypeOrganization, gap), phraseMint(nato, types.EntityTypeOrganization, sourceReason)); err != nil {
		t.Fatalf("both with reasons: %v", err)
	}
	recs := f.records(t)
	if len(recs) != 1 || recs[0].Review == nil {
		t.Fatalf("records = %+v", recs)
	}
	review := recs[0].Review
	if review.Counts.CategoryGap != 1 || len(review.CategoryGaps) != 1 || review.CategoryGaps[0] != gap || review.Counts.Overridden != 2 {
		t.Fatalf("review = %+v, want one category gap named by its reason and two overrides", review)
	}
	found := false
	for _, ne := range review.NotEntities {
		if ne.Phrase == nato.Phrase && ne.P == nato.Kind.Probabilities[analysisNotAnEntity] {
			found = true
		}
	}
	if !found {
		t.Fatalf("not-entities = %+v, want %s with its p", review.NotEntities, nato.Phrase)
	}
}

func TestRelateNamingAnEntityTheAnalysisMissedNeedsNoveltyOrAReason(t *testing.T) { // S3
	for _, novel := range []bool{true, false} {
		f := newAnalysisFixture(t)
		f.turnOn(t)
		if !novel {
			f.judge.prefer = map[string]string{"novelty": analysisNoveltyNo}
		}
		a := f.analyse(t, agentprofile.Research, "run-s3", "doc-story")
		receipt := f.judgedEntity(t, "run-s3", "kq1")
		// The role is System 1's top choice, so only the novelty rule is
		// asked.
		named := yaegikernel.RelateEntity{Name: "Kyiv Institute of Technology", Type: types.EntityTypeOrganization, Role: types.EntityRoleCentral, JudgmentRef: receipt, QuestionID: "kq1",
			Reason: "the story quotes its director on the meeting"}
		bare := named
		bare.Reason = ""
		_, err := f.relateEntities(t, "run-s3", bare)
		if novel {
			if err != nil {
				t.Fatalf("novelty p(yes) %.2f: a research-named entity was refused: %v", a.Novelty.Probabilities[analysisNoveltyYes], err)
			}
			recs := f.records(t)
			if len(recs) != 1 || recs[0].Attachments[0].Source != entitySourceResearchNamed {
				t.Fatalf("records = %+v, want one research_named attachment", recs)
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), "novelty") {
			t.Fatalf("novelty p(yes) %.2f: %v, want a refusal naming the novelty answer", a.Novelty.Probabilities[analysisNoveltyYes], err)
		}
		if _, err := f.relateEntities(t, "run-s3", named); err != nil {
			t.Fatalf("with a reason: %v", err)
		}
	}
}

func TestRelateRecordsTheReviewAndExportsItUnderTheRecordID(t *testing.T) { // S4
	f := newAnalysisFixture(t)
	f.turnOn(t)
	f.cpub.resp.Candidates[0].AliasCandidates = []string{"Zelenskyy"}
	a := f.analyse(t, agentprofile.Research, "run-s4", "doc-story")
	zelenskyy, anthropic := a.Candidates[0], a.Candidates[1]
	receipt := f.judgedEntity(t, "run-s4", "kq1", "kq2")
	got, err := f.relateEntities(t, "run-s4",
		yaegikernel.RelateEntity{EntityID: zelenskyy.EntityID, Role: types.EntityRoleCentral, JudgmentRef: zelenskyy.Role.ReceiptRef, QuestionID: zelenskyy.Role.QuestionID},
		phraseMint(anthropic, types.EntityTypePerson, ""),
		yaegikernel.RelateEntity{Name: "Kyiv Institute of Technology", Type: types.EntityTypeOrganization, Role: types.EntityRoleCentral, JudgmentRef: receipt, QuestionID: "kq1"},
		yaegikernel.RelateEntity{EntityID: "wd:Q42", Role: types.EntityRoleCentral, JudgmentRef: receipt, QuestionID: "kq2"},
	)
	if err != nil {
		t.Fatalf("relate: %v", err)
	}
	if !got.Exported {
		t.Fatalf("result = %+v, want exported", got)
	}
	events := f.entityEvents(t)
	if len(events) != 1 {
		t.Fatalf("records = %d", len(events))
	}
	exports := f.epub.entityCalls()
	if len(exports) != 1 || exports[0].Review == nil {
		t.Fatalf("exports = %+v, want one carrying the review", exports)
	}
	review := exports[0].Review
	want := types.StoryEntityCounts{Candidates: 3, Hits: 1, Existing: 2, Extracted: 1, ResearchNamed: 1, ExistingUnmatched: 1}
	if review.RecordID != events[0].EventID || review.Counts != want {
		t.Fatalf("review = %+v, want record %s and counts %+v", review, events[0].EventID, want)
	}
	if review.NoveltyYes != a.Novelty.Probabilities[analysisNoveltyYes] {
		t.Fatalf("novelty = %v, want %v", review.NoveltyYes, a.Novelty.Probabilities[analysisNoveltyYes])
	}
	if len(review.Aliases) != 1 || review.Aliases[0] != (types.StoryEntityAlias{EntityID: zelenskyy.EntityID, Alias: "Zelenskyy"}) {
		t.Fatalf("aliases = %+v", review.Aliases)
	}
	if checks := f.epub.entityChecks(); len(checks) != 1 || checks[0].Review == nil || checks[0].Review.RecordID != events[0].EventID {
		t.Fatalf("the platform's check did not see the review: %+v", checks)
	}
}

func TestRelateOfAnotherStoryKeepsItsRules(t *testing.T) { // S5
	f := newAnalysisFixture(t)
	f.turnOn(t)
	f.analyse(t, agentprofile.Research, "run-s5", "doc-story")
	receipt := f.judgedEntity(t, "run-s5", "q1")
	if _, err := f.attach(agentprofile.Research, "run-s5", newVucic(receipt, "q1", types.EntityRoleCentral)); err != nil {
		t.Fatalf("a mint on another story, with no analysis of it: %v", err)
	}
	if checks := f.epub.entityChecks(); len(checks) != 1 || checks[0].Pending || checks[0].Review != nil {
		t.Fatalf("another story's check = %+v, want a published-story check with no review", checks)
	}
}

func TestOwnStoryIsCheckedPendingAndExportedWhenItPublishes(t *testing.T) { // F6, F7
	f := newAnalysisFixture(t)
	f.turnOn(t)
	a := f.analyse(t, agentprofile.Research, "run-f6", "doc-story")
	zelenskyy, anthropic := a.Candidates[0], a.Candidates[1]
	unpublished := &platformStatusError{Path: types.StoryEntitiesPath, Status: http.StatusBadRequest, Body: "pub-x is not a published Autopaper story of owner o"}
	f.epub.entityErrs = []error{unpublished, unpublished}

	got, err := f.relateEntities(t, "run-f6",
		yaegikernel.RelateEntity{EntityID: zelenskyy.EntityID, Role: types.EntityRoleCentral, JudgmentRef: zelenskyy.Role.ReceiptRef, QuestionID: zelenskyy.Role.QuestionID},
		phraseMint(anthropic, types.EntityTypePerson, ""))
	if err != nil {
		t.Fatalf("relate an unpublished story: %v", err)
	}
	if got.Exported || !got.ExportPending || got.ExportError != "" {
		t.Fatalf("result = %+v, want export pending until the story publishes", got)
	}
	checks := f.epub.entityChecks()
	if len(checks) != 1 || !checks[0].Pending || !checks[0].ValidateOnly {
		t.Fatalf("checks = %+v, want one pending validate-only check", checks)
	}
	// The desk changes its mind about Zelenskyy's role.
	if _, err := f.relateEntities(t, "run-f6", yaegikernel.RelateEntity{EntityID: zelenskyy.EntityID, Role: types.EntityRoleIncidental, JudgmentRef: zelenskyy.Role.ReceiptRef, QuestionID: zelenskyy.Role.QuestionID,
		Reason: "on reflection it is about the company, he is the host"}); err != nil {
		t.Fatalf("role change: %v", err)
	}
	before := len(f.epub.entityCalls())

	// The story publishes: the tape's records go out, each entity once, at
	// its latest role, the mint with its description, with the review.
	f.rt.publishStoryVersion(context.Background(), f.epub, StoryVersion{OwnerID: f.owner, ComputerID: f.computer, DocID: "doc-story", RevisionID: "rev-doc-story", Revision: 1})
	exports := f.epub.entityCalls()[before:]
	if len(exports) != 1 {
		t.Fatalf("exports at publish = %d, want 1", len(exports))
	}
	out := exports[0]
	if out.Story != types.StoryPublicationID(f.owner, "doc-story") || out.Pending || out.ValidateOnly || len(out.Entities) != 2 {
		t.Fatalf("export at publish = %+v", out)
	}
	roles := map[string]types.StoryEntity{}
	for _, e := range out.Entities {
		roles[e.EntityID] = e
	}
	if roles[zelenskyy.EntityID].Role != types.EntityRoleIncidental {
		t.Fatalf("Zelenskyy exported as %q, want the latest role incidental", roles[zelenskyy.EntityID].Role)
	}
	mint := roles[types.MintedEntityID(anthropic.Phrase)]
	if !mint.New || mint.Name != anthropic.Phrase || mint.Type != types.EntityTypePerson {
		t.Fatalf("Anthropic exported as %+v, want the mint's description", mint)
	}
	if out.Review == nil || out.Review.RecordID != f.entityEvents(t)[1].EventID {
		t.Fatalf("export review = %+v, want the latest record's", out.Review)
	}

	// A story with no record exports nothing; off exports nothing.
	seedStoryDoc(t, f.s, f.owner, f.computer, "doc-quiet", "Quiet", "Nothing happened.")
	before = len(f.epub.entityCalls())
	f.rt.publishStoryVersion(context.Background(), f.epub, StoryVersion{OwnerID: f.owner, ComputerID: f.computer, DocID: "doc-quiet", RevisionID: "rev-doc-quiet", Revision: 1})
	if n := len(f.epub.entityCalls()) - before; n != 0 {
		t.Fatalf("a story with no record exported %d times", n)
	}
}
