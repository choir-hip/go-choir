package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"unicode/utf8"

	"github.com/yusefmosiah/go-choir/internal/entitytext"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// Relate's source rules and the story export
// (docs/problems/related-stories-no-entity-layer-2026-10-26.md, slice c0 §5
// and §6, slice c "unpublished stories", review 304 item 8). When the run
// holds its story's analysis on the tape (answered events of this run whose
// analysis_story is the story), every attachment has a source:
//   - existing: an entity attached by id (existing_unmatched when the
//     analysis did not offer it, the alias gap);
//   - extracted: a new name the analysis typed. Its type basis is the
//     analysis's type answer, read here, never from the cell. Another type
//     than System 1's top, or a top of "not an entity" or "none of these
//     fit", needs a reason, and "none of these fit" counts a category gap
//     named by that reason;
//   - research_named: a new name the analysis missed. It needs the novelty
//     answer's p(yes) of at least types.EntityNoveltyYesP, or a reason.
//
// A call with no analysis of its story (another story, or an analysis the
// platform could not build) keeps slice a's rules.

const entitySourceResearchNamed = "research_named"

// analysisAnswerRef is one analysis answer as the tape holds it.
type analysisAnswerRef struct {
	ref, questionID string
	probabilities   map[string]float64
	top             string
	phrase          string
}

// analysisBasis is what this run's analysis of a story put on the tape.
type analysisBasis struct {
	present    bool
	typeByKey  map[string]analysisAnswerRef
	hits       map[string]bool
	candidates int
	novelty    *analysisAnswerRef
}

// readAnalysisBasis reads this run's analysis answers of story from its
// answered events.
func readAnalysisBasis(answered map[string]types.EventRecord, story string) analysisBasis {
	b := analysisBasis{typeByKey: map[string]analysisAnswerRef{}, hits: map[string]bool{}}
	// The answered map has no order; a question id appears in one call only.
	for id, ev := range answered {
		var rec struct {
			Story    string                    `json:"analysis_story"`
			Subjects map[string]system1Subject `json:"subjects"`
			Result   json.RawMessage           `json:"result"`
		}
		if json.Unmarshal(ev.Payload, &rec) != nil || rec.Story != story || len(rec.Subjects) == 0 {
			continue
		}
		var result system1Result
		if json.Unmarshal(rec.Result, &result) != nil {
			continue
		}
		b.present = true
		for _, ans := range result.Answers {
			subject, ok := rec.Subjects[ans.ID]
			if !ok || ans.Error != "" {
				continue
			}
			ref := analysisAnswerRef{ref: "system1:" + id, questionID: ans.ID, probabilities: ans.Probabilities, top: topChoice(ans.Choices, ans.Probabilities), phrase: subject.Phrase}
			switch subject.Kind {
			case analysisSubjectRole:
				b.candidates++
				if subject.EntityID != "" {
					b.hits[subject.EntityID] = true
				}
			case analysisSubjectType:
				b.typeByKey[subject.Key] = ref
			case analysisSubjectNovelty:
				r := ref
				b.novelty = &r
			}
		}
	}
	return b
}

// noveltyYes is the novelty answer's p(yes), 0 when there is none.
func (b analysisBasis) noveltyYes() float64 {
	if b.novelty == nil {
		return 0
	}
	return b.novelty.probabilities[analysisNoveltyYes]
}

// applySource sets an attachment's source and checks its basis against the
// analysis, counting it in counts. n is its place in the call.
func (b analysisBasis) applySource(n int, e yaegikernel.RelateEntity, a *storyEntityAttachment, counts *types.StoryEntityCounts, gaps *[]string) error {
	reasoned := utf8.RuneCountInString(e.Reason) >= yaegikernel.RelateStoriesMinOverrideReasonRunes
	switch {
	case e.EntityID != "":
		a.Source = types.EntityCandidateExisting
		counts.Existing++
		if !b.hits[e.EntityID] {
			counts.ExistingUnmatched++
		}
	default:
		ta, typed := b.typeByKey[entitytext.Key(e.Name)]
		if !typed {
			a.Source = entitySourceResearchNamed
			if p := b.noveltyYes(); p < types.EntityNoveltyYesP && !reasoned {
				return fmt.Errorf("choir.Relate: entity %d: %q is not among the analysis's candidates, and its novelty answer gives p(yes) %.2f, under %.2f; name an entity the analysis missed with a reason of at least %d characters", n, e.Name, p, types.EntityNoveltyYesP, yaegikernel.RelateStoriesMinOverrideReasonRunes)
			}
			counts.ResearchNamed++
			return nil
		}
		a.Source = types.EntityCandidateExtracted
		a.TypeJudgmentRef, a.TypeQuestionID, a.TypeTop = ta.ref, ta.questionID, ta.top
		counts.Extracted++
		if ta.top == e.Type {
			return nil
		}
		if !reasoned {
			return fmt.Errorf("choir.Relate: entity %d: System 1's type answer for %q (%s, question %s) ranks %q first, not %q; you decide, so give a reason of at least %d characters", n, e.Name, ta.ref, ta.questionID, ta.top, e.Type, yaegikernel.RelateStoriesMinOverrideReasonRunes)
		}
		a.TypeOverrode = true
		if ta.top == analysisNoneFit {
			counts.CategoryGap++
			*gaps = append(*gaps, e.Reason)
		}
	}
	return nil
}

// buildReview is a record's review: its counts, the novelty, the phrases
// System 1 judged not entities, and the alias candidates of what it
// attached.
func (b analysisBasis) buildReview(recordID string, counts types.StoryEntityCounts, gaps []string, attachments []storyEntityAttachment, analysis *yaegikernel.StoryAnalysis) *types.StoryEntityReview {
	counts.Candidates = b.candidates
	counts.Hits = len(b.hits)
	review := &types.StoryEntityReview{RecordID: recordID, Counts: counts, CategoryGaps: gaps, NoveltyYes: b.noveltyYes()}
	for _, ta := range b.typeByKey {
		if ta.top == analysisNotAnEntity && len(review.NotEntities) < types.EntityReviewMaxItems {
			review.NotEntities = append(review.NotEntities, types.StoryEntityNotEntity{Phrase: ta.phrase, P: ta.probabilities[analysisNotAnEntity]})
		}
	}
	if analysis == nil {
		return review
	}
	for _, a := range attachments {
		for _, c := range analysis.Candidates {
			same := (c.EntityID != "" && c.EntityID == a.EntityID) || (a.New && c.EntityID == "" && entitytext.Key(c.Phrase) == entitytext.Key(a.Name))
			if !same {
				continue
			}
			for _, alias := range c.AliasCandidates {
				if len(review.Aliases) < types.EntityReviewMaxItems {
					review.Aliases = append(review.Aliases, types.StoryEntityAlias{EntityID: a.EntityID, Alias: alias})
				}
			}
		}
	}
	return review
}

// publishStoryVersion publishes a story version and, when it publishes,
// exports the story's entities the tape holds (slice c, F6): the export of a
// story recorded before it published follows the publish.
func (rt *Runtime) publishStoryVersion(ctx context.Context, publisher StoryPublisher, v StoryVersion) {
	if err := publisher.PublishVersion(ctx, v); err != nil {
		log.Printf("story publish: %s v%d (revision %s) failed and stays a gap: %v", v.DocID, v.Revision, v.RevisionID, err)
		return
	}
	log.Printf("story publish: %s published v%d (revision %s)", v.DocID, v.Revision, v.RevisionID)
	if entities, ok := publisher.(StoryEntityPublisher); ok {
		rt.exportStoryEntities(ctx, entities, v.OwnerID, v.ComputerID, v.DocID)
	}
}

// exportStoryEntities exports the tape's entity records of a story: each
// entity once, at its latest recorded role (F7), a minted entity with its
// first description, and the latest review. It is the same re-runnable
// projection Relate's export is; a repeat writes nothing new in Store B.
func (rt *Runtime) exportStoryEntities(ctx context.Context, publisher StoryEntityPublisher, owner, computer, docID string) {
	if rt == nil || rt.store == nil || !rt.computerSettingOn(ctx, owner, computer, store.SettingResearchRelatedStory) {
		return
	}
	story := types.StoryPublicationID(owner, docID)
	tape, err := rt.system1ChannelTape(ctx, owner, computer)
	if err != nil {
		log.Printf("story entities: export of %s at publish: read the tape: %v", story, err)
		return
	}
	var order []string
	roles := map[string]string{}
	minted := map[string]storyEntityAttachment{}
	var review *types.StoryEntityReview
	for _, ev := range tape {
		if ev.Kind != types.EventStoryEntities || ev.Phase != storyEntitiesPhaseRecorded {
			continue
		}
		var rec storyEntitiesRecord
		if json.Unmarshal(ev.Payload, &rec) != nil || rec.Story != story || (rec.Computer != "" && rec.Computer != computer) {
			continue
		}
		for _, a := range rec.Attachments {
			if _, seen := roles[a.EntityID]; !seen {
				order = append(order, a.EntityID)
			}
			roles[a.EntityID] = a.Role
			if _, seen := minted[a.EntityID]; a.New && !seen {
				minted[a.EntityID] = a
			}
		}
		if rec.Review != nil {
			review = rec.Review
		}
	}
	if len(order) == 0 && review == nil {
		return
	}
	// A story about none of its candidates sends its review alone.
	for start := 0; start == 0 || start < len(order); start += yaegikernel.RelateMaxEntities {
		req := types.StoryEntitiesRequest{OwnerID: owner, ComputerID: computer, Story: story}
		if start == 0 {
			req.Review = review
		}
		for _, id := range order[start:min(start+yaegikernel.RelateMaxEntities, len(order))] {
			e := types.StoryEntity{EntityID: id, Role: roles[id]}
			if m, ok := minted[id]; ok {
				e.Name, e.Type, e.WikidataID, e.New = m.Name, m.Type, m.WikidataID, true
			}
			req.Entities = append(req.Entities, e)
		}
		ectx, cancel := context.WithTimeout(context.WithoutCancel(ctx), storyRelationExportTimeout)
		err := publisher.PublishEntities(ectx, req)
		cancel()
		if err != nil {
			log.Printf("story entities: export of %d attachments of %s at publish failed: %v", len(req.Entities), story, err)
			continue
		}
		log.Printf("story entities: exported %d attachments of %s at publish", len(req.Entities), story)
	}
}

// ownStoryOf is the publication id of the story a research desk is bound
// to, "" when it is bound to none.
func (rt *Runtime) ownStoryOf(ctx context.Context, agentID, owner, computer string) string {
	doc, err := rt.researchDeskDoc(ctx, agentID, owner, computer)
	if err != nil {
		return ""
	}
	return types.StoryPublicationID(owner, strings.TrimSpace(doc))
}
