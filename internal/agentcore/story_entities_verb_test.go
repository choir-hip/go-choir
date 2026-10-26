package agentcore

// choir.Relate, the research desk's record of a published story's entities
// (docs/problems/related-stories-no-entity-layer-2026-10-26.md, slice a;
// design docs/reports/related-stories-phase3-research-review-2026-10-20.md
// §3.2, §6 and §11 slice 1). The ways it can fail, written before the code:
//
//  E1 The verb works for a desk other than research, or on a computer that
//     did not turn research.related_story on.
//  E2 An attachment with no judgment behind it: a receipt the run never
//     recorded, another run's receipt, a question the receipt did not
//     answer, or a role that is not one of that question's choices.
//  E3 The desk overrides System 1 silently: a role System 1 did not rank
//     first needs a reason, and the tape says it overrode.
//  E5 Sprawl (R1): a new entity whose name matches an existing one is minted
//     a second time. The platform holds the entity rows and refuses the new
//     name with the match named; the refusal records nothing.
//  E6 The tape records an attachment the platform would refuse, or the
//     platform cannot be asked and the record is written unchecked.
//  E7 The export fails after the tape write: the cell is told, the tape keeps
//     the record, and repeating the call re-exports without a second record.
//  E9 A repeat of an older call flips a role back silently (review 298 F3):
//     central, then incidental, then the first call again must leave the
//     tape's latest role and Store B's equal. A call whose role differs from
//     the latest recorded one is a role change, recorded again; only a call
//     equal to the latest record is a replay.
//  E10 The desk's own story (review 304 item 3): "" or "story:head" binds to
//     the publication id of the doc the research desk is bound to
//     (research:<doc>, a lifecycle document of the owner on this computer),
//     the id corpusd gives that story; a desk bound to no doc refuses and
//     records, checks and exports nothing.
//  E8 The id of a new entity is the cell's: a new entity's id is derived on
//     the host from its normalised name (or its Wikidata id), so two calls
//     naming it alike name one node.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

type fakeEntityPublisher struct {
	*fakeRelationPublisher
	emu        sync.Mutex
	entityReqs []types.StoryEntitiesRequest
	entityErrs []error
	checks     []types.StoryEntitiesRequest
	checkErr   error
	onValidate func()
	onPublish  func()
}

func (f *fakeEntityPublisher) ValidateEntities(_ context.Context, req types.StoryEntitiesRequest) error {
	f.emu.Lock()
	f.checks = append(f.checks, req)
	hook, err := f.onValidate, f.checkErr
	f.emu.Unlock()
	if hook != nil {
		hook()
	}
	return err
}

func (f *fakeEntityPublisher) PublishEntities(_ context.Context, req types.StoryEntitiesRequest) error {
	if f.onPublish != nil {
		f.onPublish()
	}
	f.emu.Lock()
	defer f.emu.Unlock()
	f.entityReqs = append(f.entityReqs, req)
	if n := len(f.entityReqs) - 1; n < len(f.entityErrs) {
		return f.entityErrs[n]
	}
	return nil
}

func (f *fakeEntityPublisher) entityCalls() []types.StoryEntitiesRequest {
	f.emu.Lock()
	defer f.emu.Unlock()
	return append([]types.StoryEntitiesRequest(nil), f.entityReqs...)
}

func (f *fakeEntityPublisher) entityChecks() []types.StoryEntitiesRequest {
	f.emu.Lock()
	defer f.emu.Unlock()
	return append([]types.StoryEntitiesRequest(nil), f.checks...)
}

type entityFixture struct {
	*relateFixture
	epub *fakeEntityPublisher
}

func newEntityFixture(t *testing.T) *entityFixture {
	t.Helper()
	f := newRelateFixture(t)
	epub := &fakeEntityPublisher{fakeRelationPublisher: f.pub}
	f.rt.SetStoryPublisher(epub)
	return &entityFixture{relateFixture: f, epub: epub}
}

// entityQuestion is a System 1 question whose choices are the roles.
func entityQuestion(id string, refs ...string) yaegikernel.System1Question {
	return yaegikernel.System1Question{ID: id, Text: "Is this story centrally about Aleksandar Vucic?", Choices: []string{"central", "incidental", "not about"}, ContextRefs: refs}
}

func (f *entityFixture) judgedEntity(t *testing.T, runID string, ids ...string) string {
	t.Helper()
	var qs []yaegikernel.System1Question
	for _, id := range ids {
		qs = append(qs, entityQuestion(id, f.capture))
	}
	raw, err := f.call(t, agentprofile.Research, runID, qs...)
	if err != nil {
		t.Fatalf("system1: %v", err)
	}
	return decodeSystem1(t, raw).ReceiptRef
}

func (f *entityFixture) attach(role, runID string, entities ...yaegikernel.RelateEntity) (json.RawMessage, error) {
	payload, err := json.Marshal(yaegikernel.RelatePayload{Story: f.a, Entities: entities})
	if err != nil {
		return nil, err
	}
	return f.rt.deskEgress(context.Background(), role, f.exec(runID), yaegikernel.ActionRelate, payload)
}

func newVucic(receipt, questionID, role string) yaegikernel.RelateEntity {
	return yaegikernel.RelateEntity{Name: "Aleksandar Vucic", Type: types.EntityTypePerson, Role: role, JudgmentRef: receipt, QuestionID: questionID}
}

func (f *entityFixture) entityEvents(t *testing.T) []types.EventRecord {
	t.Helper()
	var out []types.EventRecord
	for _, e := range f.tape(t) {
		// Attachments only: a platform failure's marker is not one.
		if e.Kind == types.EventStoryEntities && e.Phase == storyEntitiesPhaseRecorded {
			out = append(out, e)
		}
	}
	return out
}

type attachDecoded struct {
	Story    string `json:"story"`
	Attached []struct {
		EntityID    string  `json:"entity_id"`
		Name        string  `json:"name"`
		Role        string  `json:"role"`
		New         bool    `json:"new"`
		Probability float64 `json:"probability"`
		Top         bool    `json:"top"`
		Overrode    bool    `json:"overrode"`
		Replayed    bool    `json:"replayed"`
	} `json:"attached"`
	Exported    bool   `json:"exported"`
	ExportError string `json:"export_error"`
}

func decodeAttach(t *testing.T, raw json.RawMessage) attachDecoded {
	t.Helper()
	var out attachDecoded
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
	return out
}

func TestRelateIsResearchOnlyAndOffUntilTheOwnerTurnsItOn(t *testing.T) { // E1
	f := newEntityFixture(t)
	receipt := f.judgedEntity(t, "run-e1", "q1")
	ent := newVucic(receipt, "q1", types.EntityRoleCentral)

	if _, err := f.attach(agentprofile.Research, "run-e1", ent); err == nil || !strings.Contains(err.Error(), "research.related_story") {
		t.Fatalf("setting off: err = %v, want the refusal to name research.related_story", err)
	}
	f.turnOn(t)
	for _, role := range []string{agentprofile.Management, agentprofile.Texture, agentprofile.Engineering, yaegikernel.SessionRoleResearchChild, yaegikernel.SessionRoleManagementTask, ""} {
		if _, err := f.attach(role, "run-e1", ent); err == nil || !strings.Contains(err.Error(), "research") {
			t.Errorf("role %q: err = %v, want a refusal naming the research desk", role, err)
		}
	}
	if n := len(f.entityEvents(t)); n != 0 || len(f.epub.entityCalls()) != 0 || len(f.epub.entityChecks()) != 0 {
		t.Fatalf("refused calls left %d tape events, %d exports, %d checks", n, len(f.epub.entityCalls()), len(f.epub.entityChecks()))
	}
}

func TestRelateNeedsAJudgmentThisRunRecorded(t *testing.T) { // E2
	f := newEntityFixture(t)
	f.turnOn(t)
	receipt := f.judgedEntity(t, "run-e2", "q1")
	otherRun := f.judgedEntity(t, "run-e2-other", "q9")
	pairReceipt := f.judged(t, "run-e2", "p1") // choices: same incident, later development, unrelated

	cases := []struct {
		name string
		ent  yaegikernel.RelateEntity
		want string
	}{
		{"a receipt the run never recorded", newVucic("system1:00000000-0000-0000-0000-000000000000", "q1", types.EntityRoleCentral), "no System 1 answer"},
		{"another run's receipt", newVucic(otherRun, "q9", types.EntityRoleCentral), "no System 1 answer"},
		{"a question the receipt did not answer", newVucic(receipt, "q2", types.EntityRoleCentral), "q2"},
		{"a role that is not a choice of the question", newVucic(pairReceipt, "p1", types.EntityRoleCentral), "choices"},
	}
	for _, tc := range cases {
		if _, err := f.attach(agentprofile.Research, "run-e2", tc.ent); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want a refusal naming %q", tc.name, err, tc.want)
		}
	}
	if n := len(f.entityEvents(t)); n != 0 || len(f.epub.entityCalls()) != 0 {
		t.Fatalf("refused calls left %d tape events and %d exports", n, len(f.epub.entityCalls()))
	}
}

// The happy path, E6 and E8: the platform is asked first, the tape holds the
// whole record, the export the minimum, and the new entity's id is the host's.
func TestRelateRecordsTheAttachmentOnTheTapeAndExportsIt(t *testing.T) {
	f := newEntityFixture(t)
	f.turnOn(t)
	receipt := f.judgedEntity(t, "run-ok", "q1", "q2")
	onTapeAtCheck := -1
	f.epub.onValidate = func() { onTapeAtCheck = len(f.entityEvents(t)) }

	existing := types.WikidataEntityID("Q403")
	raw, err := f.attach(agentprofile.Research, "run-ok",
		newVucic(receipt, "q1", types.EntityRoleCentral),
		yaegikernel.RelateEntity{EntityID: existing, Role: types.EntityRoleCentral, JudgmentRef: receipt, QuestionID: "q2"})
	if err != nil {
		t.Fatal(err)
	}
	got := decodeAttach(t, raw)
	if !got.Exported || got.ExportError != "" || got.Story != f.a || len(got.Attached) != 2 {
		t.Fatalf("result = %s", raw)
	}
	minted := types.MintedEntityID("Aleksandar Vucic")
	a := got.Attached[0]
	if a.EntityID != minted || !a.New || a.Role != "central" || a.Probability != 0.7 || !a.Top || a.Overrode || a.Replayed {
		t.Fatalf("attached[0] = %+v, want the minted id %s, new, central at 0.7, top", a, minted)
	}
	if b := got.Attached[1]; b.EntityID != existing || b.New {
		t.Fatalf("attached[1] = %+v, want the existing %s, not new", b, existing)
	}

	evs := f.entityEvents(t)
	if len(evs) != 1 || evs[0].Phase != "recorded" || evs[0].RunID != "run-ok" || evs[0].OwnerID != f.owner || evs[0].ChannelID != system1Channel(f.computer) {
		t.Fatalf("tape = %+v, want one recorded story.entities on the run and the system1 channel", evs)
	}
	var payload struct {
		Computer    string `json:"computer_id"`
		Story       string `json:"story"`
		Attachments []struct {
			EntityID    string  `json:"entity_id"`
			Name        string  `json:"name"`
			Type        string  `json:"type"`
			New         bool    `json:"new"`
			Role        string  `json:"role"`
			Probability float64 `json:"probability"`
			JudgmentRef string  `json:"judgment_ref"`
			QuestionID  string  `json:"question_id"`
		} `json:"attachments"`
	}
	if err := json.Unmarshal(evs[0].Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Computer != f.computer || payload.Story != f.a || len(payload.Attachments) != 2 {
		t.Fatalf("payload = %s", evs[0].Payload)
	}
	if p := payload.Attachments[0]; p.EntityID != minted || p.Name != "Aleksandar Vucic" || p.Type != "person" || !p.New || p.Probability != 0.7 || p.JudgmentRef != receipt || p.QuestionID != "q1" {
		t.Fatalf("attachment = %+v", p)
	}

	checks := f.epub.entityChecks()
	if len(checks) != 1 || onTapeAtCheck != 0 || checks[0].Story != f.a || len(checks[0].Entities) != 2 {
		t.Fatalf("validations = %+v with %d on the tape at the check, want one check of the story before the record", checks, onTapeAtCheck)
	}
	calls := f.epub.entityCalls()
	if len(calls) != 1 || calls[0].OwnerID != f.owner || calls[0].ComputerID != f.computer || calls[0].Story != f.a || len(calls[0].Entities) != 2 {
		t.Fatalf("export = %+v", calls)
	}
	if e := calls[0].Entities[0]; e.EntityID != minted || !e.New || e.Name != "Aleksandar Vucic" || e.Type != "person" || e.Role != "central" {
		t.Fatalf("exported %+v, want the new entity with its name, type and role", e)
	}
	if e := calls[0].Entities[1]; e.EntityID != existing || e.New || e.Name != "" {
		t.Fatalf("exported %+v, want the existing entity by id alone", e)
	}

	// A new entity with a Wikidata id is named by it.
	wd := newVucic(receipt, "q1", types.EntityRoleCentral)
	wd.Name, wd.WikidataID = "Ana Brnabic", "Q7254"
	raw, err = f.attach(agentprofile.Research, "run-ok", wd)
	if err != nil {
		t.Fatal(err)
	}
	if got := decodeAttach(t, raw); got.Attached[0].EntityID != "wd:Q7254" || !got.Attached[0].New {
		t.Fatalf("wikidata entity = %s", raw)
	}
}

func TestRelateOverridingSystem1NeedsAReason(t *testing.T) { // E3
	f := newEntityFixture(t)
	f.turnOn(t)
	receipt := f.judgedEntity(t, "run-e3", "q1") // central 0.7, incidental 0.15

	ent := newVucic(receipt, "q1", types.EntityRoleIncidental)
	for _, reason := range []string{"", "no", "because I said so"} {
		ent.Reason = reason
		if _, err := f.attach(agentprofile.Research, "run-e3", ent); err == nil || !strings.Contains(err.Error(), "reason") {
			t.Errorf("reason %q: err = %v, want a refusal asking for a reason", reason, err)
		}
	}
	if n := len(f.entityEvents(t)); n != 0 {
		t.Fatalf("%d events after refused overrides", n)
	}
	ent.Reason = "He is named once, in the last paragraph, as a past speaker."
	raw, err := f.attach(agentprofile.Research, "run-e3", ent)
	if err != nil {
		t.Fatal(err)
	}
	if a := decodeAttach(t, raw).Attached[0]; a.Top || !a.Overrode || a.Probability != 0.15 {
		t.Fatalf("attached = %+v, want not top, overrode, p 0.15", a)
	}
}

func TestRelateRecordsNothingThePlatformRefuses(t *testing.T) { // E5, E6
	f := newEntityFixture(t)
	f.turnOn(t)
	receipt := f.judgedEntity(t, "run-e5", "q1")

	// The platform's own check refuses with a 4xx: a fixable refusal
	// (review 304 item 2).
	f.epub.checkErr = &platformStatusError{Path: types.StoryEntitiesPath, Status: http.StatusUnprocessableEntity,
		Body: fmt.Sprintf(`entity 1: "Aleksandar Vucic" matches the existing entity %s ("Aleksandar Vučić"); attach it by entity_id`, types.MintedEntityID("Aleksandar Vucic"))}
	_, err := f.attach(agentprofile.Research, "run-e5", newVucic(receipt, "q1", types.EntityRoleCentral))
	if err == nil || !strings.Contains(err.Error(), "matches the existing entity") || !strings.Contains(err.Error(), "nothing was recorded") {
		t.Fatalf("err = %v, want the platform's match named and nothing recorded", err)
	}
	if n := len(f.entityEvents(t)); n != 0 || len(f.epub.entityCalls()) != 0 {
		t.Fatalf("a refused attachment left %d events and %d exports", n, len(f.epub.entityCalls()))
	}

	// No platform to ask: nothing is recorded either.
	f.epub.checkErr = nil
	f.rt.SetStoryPublisher(f.pub) // a publisher with no entity face
	if _, err := f.attach(agentprofile.Research, "run-e5", newVucic(receipt, "q1", types.EntityRoleCentral)); err == nil || !strings.Contains(err.Error(), "platform") {
		t.Fatalf("no entity publisher: err = %v, want a refusal naming the platform", err)
	}
	if n := len(f.entityEvents(t)); n != 0 {
		t.Fatalf("an unchecked attachment was recorded: %d events", n)
	}
}

func TestRelateExportFailureKeepsTheRecordAndARepeatReExports(t *testing.T) { // E7
	f := newEntityFixture(t)
	f.turnOn(t)
	f.epub.entityErrs = []error{fmt.Errorf("corpusd down")}
	receipt := f.judgedEntity(t, "run-e7", "q1")
	ent := newVucic(receipt, "q1", types.EntityRoleCentral)

	raw, err := f.attach(agentprofile.Research, "run-e7", ent)
	if err != nil {
		t.Fatalf("an export failure is a result, not a refusal of the record: %v", err)
	}
	if got := decodeAttach(t, raw); got.Exported || !strings.Contains(got.ExportError, "corpusd down") || got.Attached[0].Replayed {
		t.Fatalf("result = %s, want recorded, not exported, the error named", raw)
	}
	if n := len(f.entityEvents(t)); n != 1 {
		t.Fatalf("%d events after a failed export, want the record kept", n)
	}

	// The repeat names the entity by the id the first call returned.
	again := yaegikernel.RelateEntity{EntityID: types.MintedEntityID("Aleksandar Vucic"), Role: types.EntityRoleCentral, JudgmentRef: receipt, QuestionID: "q1"}
	raw, err = f.attach(agentprofile.Research, "run-e7", again)
	if err != nil {
		t.Fatal(err)
	}
	got := decodeAttach(t, raw)
	if !got.Exported || !got.Attached[0].Replayed {
		t.Fatalf("repeat = %s, want replayed and exported", raw)
	}
	if n := len(f.entityEvents(t)); n != 1 {
		t.Fatalf("%d events after the repeat, want no second record", n)
	}
	// The first export never reached Store B, so the repeat carries the
	// entity's description from the tape: the platform can still mint it.
	calls := f.epub.entityCalls()
	if len(calls) != 2 || calls[1].Entities[0].EntityID != again.EntityID || !calls[1].Entities[0].New || calls[1].Entities[0].Name != "Aleksandar Vucic" || calls[1].Entities[0].Type != "person" {
		t.Fatalf("exports = %+v; the repeat exports the attachment again, with the entity the tape minted", calls)
	}
}

func TestRelateRepeatOfAnOlderRoleIsRecordedAgain(t *testing.T) { // E9
	f := newEntityFixture(t)
	f.turnOn(t)
	receipt := f.judgedEntity(t, "run-e9", "q1") // central 0.7, incidental 0.15
	id := types.MintedEntityID("Aleksandar Vucic")
	central := newVucic(receipt, "q1", types.EntityRoleCentral)
	incidental := yaegikernel.RelateEntity{EntityID: id, Role: types.EntityRoleIncidental, JudgmentRef: receipt, QuestionID: "q1",
		Reason: "He is named once, in the last paragraph, as a past speaker."}

	steps := []struct {
		ent      yaegikernel.RelateEntity
		replayed bool
		events   int
	}{
		{central, false, 1},
		{incidental, false, 2},
		{central, false, 3},    // the older call again: a role change, recorded
		{central, true, 3},     // equal to the latest record: a replay
		{incidental, false, 4}, // and back
	}
	for i, step := range steps {
		raw, err := f.attach(agentprofile.Research, "run-e9", step.ent)
		if err != nil {
			t.Fatalf("step %d: %v", i, err)
		}
		got := decodeAttach(t, raw)
		if got.Attached[0].Replayed != step.replayed {
			t.Fatalf("step %d (%s): replayed = %v, want %v", i, step.ent.Role, got.Attached[0].Replayed, step.replayed)
		}
		if n := len(f.entityEvents(t)); n != step.events {
			t.Fatalf("step %d: %d events, want %d", i, n, step.events)
		}
		calls := f.epub.entityCalls()
		if last := calls[len(calls)-1]; last.Entities[0].Role != step.ent.Role {
			t.Fatalf("step %d: exported role %s, want %s", i, last.Entities[0].Role, step.ent.Role)
		}
	}
}

// Review 298 (DeepSeek seat) item 3: the export is a slow network call and
// must not hold relationMu, which serializes Relate and RelateStories.
func TestRelateExportDoesNotHoldTheRelationLock(t *testing.T) {
	f := newEntityFixture(t)
	f.turnOn(t)
	receipt := f.judgedEntity(t, "run-lock", "q1")
	held := true
	f.epub.onPublish = func() {
		if f.rt.relationMu.TryLock() {
			held = false
			f.rt.relationMu.Unlock()
		}
	}
	if _, err := f.attach(agentprofile.Research, "run-lock", newVucic(receipt, "q1", types.EntityRoleCentral)); err != nil {
		t.Fatal(err)
	}
	if held {
		t.Fatal("relationMu was held during the export")
	}
}

// Review 298 (DeepSeek seat) item 5: a repeat re-sends the description the
// entity was first minted with (the tape is read oldest first), not a later
// spelling of the same id.
func TestRelateRepeatCarriesTheFirstMintedDescription(t *testing.T) {
	f := newEntityFixture(t)
	f.turnOn(t)
	f.epub.entityErrs = []error{fmt.Errorf("corpusd down"), fmt.Errorf("corpusd down")}
	receipt := f.judgedEntity(t, "run-mint", "q1")
	first := yaegikernel.RelateEntity{Name: "Anthropic", Type: types.EntityTypeOrganization, Role: types.EntityRoleCentral, JudgmentRef: receipt, QuestionID: "q1"}
	second := yaegikernel.RelateEntity{Name: "Anthropic PBC", Type: types.EntityTypeOrganization, Role: types.EntityRoleIncidental, JudgmentRef: receipt, QuestionID: "q1",
		Reason: "Named once, as the maker of a model the story mentions in passing."}
	for _, e := range []yaegikernel.RelateEntity{first, second} {
		if _, err := f.attach(agentprofile.Research, "run-mint", e); err != nil {
			t.Fatal(err)
		}
	}
	id := types.MintedEntityID("Anthropic")
	if _, err := f.attach(agentprofile.Research, "run-mint", yaegikernel.RelateEntity{EntityID: id, Role: types.EntityRoleIncidental, JudgmentRef: receipt, QuestionID: "q1",
		Reason: "Named once, as the maker of a model the story mentions in passing."}); err != nil {
		t.Fatal(err)
	}
	calls := f.epub.entityCalls()
	if last := calls[len(calls)-1].Entities[0]; last.Name != "Anthropic" || !last.New {
		t.Fatalf("repeat exported %+v, want the first minted description (Anthropic)", last)
	}
}

func TestRelateOwnStoryBindsToTheDesksDocAndAnUnboundDeskRefuses(t *testing.T) { // E10
	f := newEntityFixture(t)
	f.turnOn(t)
	receipt := f.judgedEntity(t, "run-own", "q1")
	ent := newVucic(receipt, "q1", types.EntityRoleCentral)
	relate := func(agentID, story string) (json.RawMessage, error) {
		payload, err := json.Marshal(yaegikernel.RelatePayload{Story: story, Entities: []yaegikernel.RelateEntity{ent}})
		if err != nil {
			t.Fatal(err)
		}
		exec := f.exec("run-own")
		exec.AgentID = agentID
		return f.rt.deskEgress(context.Background(), agentprofile.Research, exec, yaegikernel.ActionRelate, payload)
	}

	// Unbound: no doc behind the desk id, or no doc in it.
	for _, agentID := range []string{"research:" + f.owner, "research", "research:", ""} {
		if _, err := relate(agentID, yaegikernel.RelateOwnStory); err == nil || !strings.Contains(err.Error(), "bound") {
			t.Errorf("desk %q: err = %v, want a refusal saying the desk is bound to no story", agentID, err)
		}
	}
	if n := len(f.entityEvents(t)); n != 0 || len(f.epub.entityCalls()) != 0 || len(f.epub.entityChecks()) != 0 {
		t.Fatalf("unbound calls left %d tape events, %d exports, %d checks", n, len(f.epub.entityCalls()), len(f.epub.entityChecks()))
	}

	// Bound: research:doc-system1 names a lifecycle document of the owner.
	want := types.StoryPublicationID(f.owner, "doc-system1")
	for _, story := range []string{"", yaegikernel.RelateOwnStory} {
		raw, err := relate("research:doc-system1", story)
		if err != nil {
			t.Fatalf("story %q: %v", story, err)
		}
		if got := decodeAttach(t, raw).Story; got != want {
			t.Fatalf("story %q: result story = %q, want the doc's publication id %q", story, got, want)
		}
	}
	checks, exports := f.epub.entityChecks(), f.epub.entityCalls()
	if len(checks) == 0 || len(exports) == 0 || checks[0].Story != want || exports[0].Story != want {
		t.Fatalf("checks %+v exports %+v, want both for %q", checks, exports, want)
	}
	events := f.entityEvents(t)
	if len(events) != 1 {
		t.Fatalf("tape holds %d story.entities events, want 1 (the second call is a replay)", len(events))
	}
	var rec storyEntitiesRecord
	if err := json.Unmarshal(events[0].Payload, &rec); err != nil || rec.Story != want {
		t.Fatalf("tape record story = %q, %v; want %q", rec.Story, err, want)
	}
}
