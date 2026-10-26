package agentcore

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// choir.RelateStories, the research desk's record of a judged pair relation
// (docs/problems/related-stories-thin-no-subject-nodes-2026-10-20.md, "Build:
// the related_story edge", B1 to B10). A research cell has read two published
// stories and asked System 1; this host step checks the pair against the
// System 1 answer it cites, writes a story.relation event on the computer's
// tape, and exports the pair to Store B as one related_story edge.
//
// What it guarantees:
//   - research only, and only on a computer whose owner turned on the
//     research.related_story setting (off by default; off is the rollback);
//   - a relation rests on a System 1 answer of this run: the receipt must be
//     an answered system1.judgment event of this run and computer, the
//     question must be in it, and the relation must be one of its choices; the
//     probability is read from that answer, never taken from the cell;
//   - the desk decides: a relation System 1 did not rank first needs a reason
//     of at least 20 characters, and the event records that it overrode;
//   - the tape is the authority (design R7): the record is appended
//     (acknowledged) before anything is exported, and the export is a
//     re-runnable projection. A repeated pair writes no second record and
//     exports again, which is the retry of a failed export;
//   - the platform is asked first, validate-only: a pair it would refuse
//     (not the owner's published story, over the degree bound) or cannot be
//     asked about is not recorded, so the tape holds no judgment that could
//     never become an edge;
//   - nothing resumes (restart rule): a crash between the record and the
//     export leaves the record on the tape and the edge absent until the desk
//     repeats the pair.

const storyRelationPhaseRecorded = "recorded"

// storyRelationExportTimeout bounds one export; a longer one is a failed
// export the desk can repeat.
const storyRelationExportTimeout = 30 * time.Second

type relateStoriesRecorded struct {
	StoryA      string  `json:"story_a"`
	StoryB      string  `json:"story_b"`
	Relation    string  `json:"relation"`
	Probability float64 `json:"probability"`
	Top         bool    `json:"top"`
	Overrode    bool    `json:"overrode"`
	Replayed    bool    `json:"replayed"`
}

type relateStoriesResult struct {
	Recorded    []relateStoriesRecorded `json:"recorded"`
	Exported    bool                    `json:"exported"`
	ExportError string                  `json:"export_error,omitempty"`
}

func (rt *Runtime) relateStoriesEgress(ctx context.Context, deskRole string, exec toolregistry.ExecutionContext, payload json.RawMessage) (json.RawMessage, error) {
	if deskRole != yaegikernel.SessionRoleResearch {
		return nil, fmt.Errorf("choir.RelateStories belongs to the research desk; this session is %q", deskRole)
	}
	if rt == nil || rt.store == nil {
		return nil, fmt.Errorf("choir.RelateStories host unavailable")
	}
	owner, computer, runID := strings.TrimSpace(exec.OwnerID), strings.TrimSpace(exec.ComputerID), strings.TrimSpace(exec.RunID)
	if owner == "" || computer == "" || runID == "" {
		return nil, fmt.Errorf("choir.RelateStories needs a run with an owner and a computer")
	}
	if !rt.computerSettingOn(ctx, owner, computer, store.SettingResearchRelatedStory) {
		return nil, fmt.Errorf("choir.RelateStories is off on this computer: the owner turns on the %s setting", store.SettingResearchRelatedStory)
	}
	var req yaegikernel.RelateStoriesPayload
	dec := json.NewDecoder(bytes.NewReader(payload))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		return nil, fmt.Errorf("choir.RelateStories: decode the pairs: %w", err)
	}
	pairs, err := yaegikernel.ValidateRelateStoriesPairs(req.Pairs)
	if err != nil {
		return nil, err
	}

	rt.relationMu.Lock()
	tape, err := rt.system1ChannelTape(ctx, owner, computer)
	if err != nil {
		rt.relationMu.Unlock()
		return nil, fmt.Errorf("choir.RelateStories: read the tape: %w", err)
	}
	answered := map[string]types.EventRecord{}
	recorded := map[string]bool{}
	for _, ev := range tape {
		switch {
		case ev.Kind == types.EventSystem1Judgment && ev.Phase == system1PhaseAnswered && ev.RunID == runID:
			answered[ev.EventID] = ev
		case ev.Kind == types.EventStoryRelation && ev.Phase == storyRelationPhaseRecorded:
			var rec struct {
				Computer string `json:"computer_id"`
				A        string `json:"story_a"`
				B        string `json:"story_b"`
				Relation string `json:"relation"`
			}
			if json.Unmarshal(ev.Payload, &rec) == nil && (rec.Computer == "" || rec.Computer == computer) {
				recorded[relationKey(rec.A, rec.B, rec.Relation)] = true
			}
		}
	}

	// Check every pair before recording any.
	type checked struct {
		pair   yaegikernel.RelateStoriesPair
		out    relateStoriesRecorded
		reason string
		model  string
	}
	var all []checked
	for i, p := range pairs {
		c, err := system1RelationBasis(p, answered, computer)
		if err != nil {
			rt.relationMu.Unlock()
			return nil, fmt.Errorf("choir.RelateStories: pair %d: %w", i+1, err)
		}
		if !c.top && utf8.RuneCountInString(p.Reason) < yaegikernel.RelateStoriesMinOverrideReasonRunes {
			rt.relationMu.Unlock()
			return nil, fmt.Errorf("choir.RelateStories: pair %d: System 1 did not rank %s first (it gave %.2f); you decide, so give a reason of at least %d characters", i+1, p.Relation, c.probability, yaegikernel.RelateStoriesMinOverrideReasonRunes)
		}
		all = append(all, checked{pair: p, model: c.model, out: relateStoriesRecorded{
			StoryA: p.StoryA, StoryB: p.StoryB, Relation: p.Relation, Probability: c.probability, Top: c.top, Overrode: !c.top,
			Replayed: recorded[relationKey(p.StoryA, p.StoryB, p.Relation)],
		}})
	}

	export := types.StoryRelationsRequest{OwnerID: owner, ComputerID: computer}
	for _, c := range all {
		export.Relations = append(export.Relations, types.StoryRelation{StoryA: c.pair.StoryA, StoryB: c.pair.StoryB, Relation: c.pair.Relation})
	}

	// Ask the platform before anything is recorded. The lock is held across
	// the call so a concurrent call cannot record the same pair meanwhile.
	if err := rt.validateStoryRelations(ctx, export); err != nil {
		rt.relationMu.Unlock()
		log.Printf("story relation: the platform did not take %d relations on run %s: %v", len(export.Relations), runID, err)
		return nil, fmt.Errorf("choir.RelateStories: nothing was recorded, the platform did not take the pairs: %w", err)
	}

	// Record: the tape first.
	result := relateStoriesResult{Recorded: make([]relateStoriesRecorded, 0, len(all))}
	for _, c := range all {
		key := relationKey(c.pair.StoryA, c.pair.StoryB, c.pair.Relation)
		if !c.out.Replayed && !recorded[key] {
			if err := rt.appendStoryRelationEvent(ctx, exec, owner, computer, runID, c.pair, c.out, c.model); err != nil {
				rt.relationMu.Unlock()
				log.Printf("story relation: record %s %s on run %s: %v", c.pair.StoryA, c.pair.StoryB, runID, err)
				return nil, fmt.Errorf("choir.RelateStories: the relation could not be recorded on the tape, so it is not exported: %w", err)
			}
			recorded[key] = true
		} else {
			// On the tape already, or recorded a pair ago in this call.
			c.out.Replayed = true
		}
		result.Recorded = append(result.Recorded, c.out)
	}
	rt.relationMu.Unlock()

	// Export: a projection of the tape; a failure leaves the record.
	if err := rt.exportStoryRelations(ctx, export); err != nil {
		result.ExportError = err.Error()
		log.Printf("story relation: export of %d relations on run %s failed: %v", len(export.Relations), runID, err)
	} else {
		result.Exported = true
	}
	return json.Marshal(result)
}

func relationKey(a, b, relation string) string { return a + "\x00" + b + "\x00" + relation }

type relationBasis struct {
	probability float64
	top         bool
	model       string
}

// system1RelationBasis finds the System 1 answer a pair rests on and reads
// the relation's probability from it. The relation must be one of the
// question's choices (spaces, hyphens and case do not matter).
func system1RelationBasis(p yaegikernel.RelateStoriesPair, answered map[string]types.EventRecord, computer string) (relationBasis, error) {
	id := strings.TrimPrefix(p.JudgmentRef, "system1:")
	ev, ok := answered[id]
	if !ok {
		return relationBasis{}, fmt.Errorf("no System 1 answer with receipt %q in this run; ask choir.System1 first and cite its receipt_ref", p.JudgmentRef)
	}
	var rec struct {
		Computer string          `json:"computer_id"`
		Model    string          `json:"model"`
		Result   json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(ev.Payload, &rec); err != nil || (rec.Computer != "" && rec.Computer != computer) {
		return relationBasis{}, fmt.Errorf("receipt %q is not an answer of this computer", p.JudgmentRef)
	}
	var result system1Result
	if err := json.Unmarshal(rec.Result, &result); err != nil {
		return relationBasis{}, fmt.Errorf("receipt %q holds an answer that did not decode: %w", p.JudgmentRef, err)
	}
	for _, a := range result.Answers {
		if a.ID != p.QuestionID {
			continue
		}
		if a.Error != "" {
			return relationBasis{}, fmt.Errorf("System 1 did not answer question %q: %s", p.QuestionID, a.Error)
		}
		choice := ""
		for _, c := range a.Choices {
			if normalizeRelationChoice(c) == p.Relation {
				choice = c
			}
		}
		if choice == "" {
			return relationBasis{}, fmt.Errorf("relation %q is not one of question %q's choices %v; ask with the choices %s, %s, %s", p.Relation, p.QuestionID, a.Choices,
				types.StoryRelationSameIncident, types.StoryRelationLaterDevelopment, types.StoryRelationSameSubject)
		}
		prob, ok := a.Probabilities[choice]
		if !ok {
			return relationBasis{}, fmt.Errorf("System 1 gave no probability for %q on question %q", choice, p.QuestionID)
		}
		top := true
		for c, other := range a.Probabilities {
			if c != choice && other > prob {
				top = false
			}
		}
		return relationBasis{probability: prob, top: top, model: rec.Model}, nil
	}
	return relationBasis{}, fmt.Errorf("receipt %q has no answer to question %q", p.JudgmentRef, p.QuestionID)
}

// normalizeRelationChoice spells a System 1 choice as a relation name:
// "Same incident" and "same-incident" are same_incident.
func normalizeRelationChoice(choice string) string {
	choice = strings.ToLower(strings.TrimSpace(choice))
	return strings.NewReplacer(" ", "_", "-", "_").Replace(choice)
}

func (rt *Runtime) appendStoryRelationEvent(ctx context.Context, exec toolregistry.ExecutionContext, owner, computer, runID string, p yaegikernel.RelateStoriesPair, out relateStoriesRecorded, model string) error {
	raw, err := json.Marshal(map[string]any{
		"computer_id": computer, "story_a": p.StoryA, "story_b": p.StoryB, "relation": p.Relation,
		"probability": out.Probability, "top": out.Top, "overrode": out.Overrode, "reason": p.Reason,
		"judgment_ref": p.JudgmentRef, "question_id": p.QuestionID, "model": model,
	})
	if err != nil {
		return err
	}
	ev := types.EventRecord{EventID: uuid.New().String(), Timestamp: time.Now().UTC(), RunID: runID, OwnerID: owner, AgentID: exec.AgentID,
		ChannelID: system1Channel(computer), Kind: types.EventStoryRelation, Phase: storyRelationPhaseRecorded, Payload: raw}
	return rt.store.AppendEvent(ctx, &ev)
}

// validateStoryRelations asks the platform to check the relations and write
// nothing. It fails when there is no platform to ask, or the platform
// refuses; the caller records nothing then.
func (rt *Runtime) validateStoryRelations(ctx context.Context, req types.StoryRelationsRequest) error {
	publisher, ok := rt.storyPublisher().(StoryRelationPublisher)
	if !ok || publisher == nil {
		return fmt.Errorf("no platform to check the pairs with (choir.platform_url is not set on this computer)")
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), storyRelationExportTimeout)
	defer cancel()
	return publisher.ValidateRelations(ctx, req)
}

// exportStoryRelations sends the relations to the platform. It fails when
// there is no platform to send them to, or the publisher cannot export.
func (rt *Runtime) exportStoryRelations(ctx context.Context, req types.StoryRelationsRequest) error {
	publisher, ok := rt.storyPublisher().(StoryRelationPublisher)
	if !ok || publisher == nil {
		return fmt.Errorf("no platform to export to (choir.platform_url is not set on this computer); the relation is on the tape, repeat the pair when it is")
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), storyRelationExportTimeout)
	defer cancel()
	return publisher.PublishRelations(ctx, req)
}
