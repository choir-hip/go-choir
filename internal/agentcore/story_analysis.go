package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/entitytext"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// The story analysis (docs/problems/related-stories-no-entity-layer-
// 2026-10-26.md, slice c0 §3 and §4). With research.related_story on, a
// story's research desk (research:<doc>) gets its story's entity analysis
// at its first cell, and every cell's frame carries it
// (choir.StoryAnalysis()). The host builds it once a run:
//   - the doc's head revision, title and text;
//   - the computer's own extraction (entitytext.Propose, weighted by the
//     library's document frequencies);
//   - corpusd's candidates (the owner's lexicon matched in the text, and the
//     proposals that are not entities yet), ranked and bounded;
//   - System 1's answers to the questions that follow from them: a role for
//     each candidate, a type for each phrase, and the story's novelty. The
//     host asks; the answers are this run's system1.judgment events, so
//     Relate's same-run basis holds. At most two calls of 12 questions.
//
// Restarts: nothing resumes. A crash closes the run; the next run on the
// story builds the analysis again, and an answer an earlier run recorded for
// the same questions over the same head is replayed into the new run as a
// fresh event, at no cost (review 304 item 1). The replay reads the
// system1:<computer> channel's newest 10,000 events: an analysis older than
// that window is asked again, one call's cost (review 304 item 7, a named
// residual).

// StoryEntityCandidateReader is the optional face of a StoryPublisher that
// reads a story's entity candidates from the platform.
type StoryEntityCandidateReader interface {
	EntityCandidates(ctx context.Context, req types.EntityCandidatesRequest) (types.EntityCandidatesResponse, error)
}

// The analysis questions' choices.
const (
	analysisRoleNotAbout = "not about"
	analysisNotAnEntity  = "not an entity"
	analysisNoneFit      = "none of these fit"
	analysisNoveltyYes   = "yes"
	analysisNoveltyNo    = "no"

	analysisSubjectRole    = "role"
	analysisSubjectType    = "type"
	analysisSubjectNovelty = "novelty"

	// analysisMaxQuestions is the budget: two calls of 12.
	analysisMaxQuestions = 2 * yaegikernel.System1MaxQuestions
	// analysisPhraseRunes bounds a name quoted in a question.
	analysisPhraseRunes = 120
	// analysisMemoRuns bounds the per-run memo.
	analysisMemoRuns = 256
)

var (
	analysisRoleChoices = []string{types.EntityRoleCentral, types.EntityRoleIncidental, analysisRoleNotAbout}
	analysisTypeChoices = []string{types.EntityTypePerson, types.EntityTypeOrganization, types.EntityTypePlace,
		types.EntityTypeProduct, types.EntityTypeWork, types.EntityTypeLaw, analysisNotAnEntity, analysisNoneFit}
)

// analysisMemo holds each run's analysis, built once.
type analysisMemo struct {
	mu    sync.Mutex
	byKey map[string]*analysisEntry
	order []string
}

type analysisEntry struct {
	once     sync.Once
	analysis *yaegikernel.StoryAnalysis
}

func (m *analysisMemo) entry(key string) *analysisEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.byKey == nil {
		m.byKey = map[string]*analysisEntry{}
	}
	if e, ok := m.byKey[key]; ok {
		return e
	}
	e := &analysisEntry{}
	m.byKey[key] = e
	m.order = append(m.order, key)
	for len(m.order) > analysisMemoRuns {
		delete(m.byKey, m.order[0])
		m.order = m.order[1:]
	}
	return e
}

// storyAnalysisFor is the analysis a desk cell's frame carries: nil unless
// the desk is a story's research desk on a computer with
// research.related_story on.
func (rt *Runtime) storyAnalysisFor(ctx context.Context, deskRole string, exec toolregistry.ExecutionContext) *yaegikernel.StoryAnalysis {
	if rt == nil || rt.store == nil || deskRole != agentprofile.Research {
		return nil
	}
	owner, computer, runID := strings.TrimSpace(exec.OwnerID), strings.TrimSpace(exec.ComputerID), strings.TrimSpace(exec.RunID)
	if owner == "" || computer == "" || runID == "" {
		return nil
	}
	if !rt.computerSettingOn(ctx, owner, computer, store.SettingResearchRelatedStory) {
		return nil
	}
	doc, err := rt.researchDeskDoc(ctx, exec, owner, computer)
	if err != nil {
		return nil
	}
	e := rt.analyses.entry(runID + "\x00" + doc)
	e.once.Do(func() {
		e.analysis = rt.buildStoryAnalysis(ctx, exec, owner, computer, runID, doc)
	})
	return e.analysis
}

func (rt *Runtime) buildStoryAnalysis(ctx context.Context, exec toolregistry.ExecutionContext, owner, computer, runID, docID string) *yaegikernel.StoryAnalysis {
	a := &yaegikernel.StoryAnalysis{Story: yaegikernel.StoryAnalysisStory{PublicationID: types.StoryPublicationID(owner, docID), DocID: docID}, Candidates: []yaegikernel.StoryAnalysisCandidate{}}
	doc, err := rt.store.GetLifecycleDocument(ctx, owner, computer, docID)
	if err != nil {
		a.Error = "the story could not be read: " + err.Error()
		return a
	}
	a.Story.Title, a.Story.RevisionID = strings.TrimSpace(doc.Title), doc.CurrentRevisionID
	if doc.CurrentRevisionID == "" {
		a.Error = "the story has no revision yet"
		return a
	}
	rev, err := rt.store.GetLifecycleRevision(ctx, owner, computer, doc.CurrentRevisionID)
	if err != nil {
		a.Error = "the story's head could not be read: " + err.Error()
		return a
	}
	if strings.TrimSpace(rev.Content) == "" {
		a.Error = "the story's head has no text"
		return a
	}

	reader, ok := rt.storyPublisher().(StoryEntityCandidateReader)
	if !ok || reader == nil {
		a.Error = "no platform to read the entity candidates from (choir.platform_url is not set on this computer)"
		return a
	}
	idf, err := rt.store.StoryLibraryIDF(ctx, owner)
	if err != nil {
		log.Printf("story analysis: %s: library frequencies: %v (rarity unknown)", docID, err)
		idf = nil
	}
	proposals := entitytext.WireProposals(entitytext.Propose(entitytext.Parse(doc.Title, rev.Content), idf))
	rctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), storyRelationExportTimeout)
	resp, err := reader.EntityCandidates(rctx, types.EntityCandidatesRequest{OwnerID: owner, ComputerID: computer, Title: doc.Title, Text: rev.Content, Proposals: proposals})
	cancel()
	if err != nil {
		log.Printf("story analysis: %s on run %s: candidates: %v", docID, runID, err)
		a.Error = "the platform did not answer the entity candidates read: " + err.Error()
		return a
	}
	a.Lexicon = resp.Lexicon
	for _, c := range resp.Candidates {
		a.Candidates = append(a.Candidates, yaegikernel.StoryAnalysisCandidate{EntityCandidate: c})
	}
	if len(a.Candidates) > 0 {
		rt.askAnalysisQuestions(ctx, exec, owner, computer, runID, a)
	}
	fitStoryAnalysis(a)
	return a
}

// analysisSlot ties a question to the candidate answer it fills.
type analysisSlot struct {
	question yaegikernel.System1Question
	subject  system1Subject
	fill     **yaegikernel.StoryAnalysisAnswer
}

// askAnalysisQuestions asks System 1 the analysis's questions: the novelty
// question first, then each candidate in rank order (a role, and for a
// phrase a type first) while the two-call budget holds; a candidate that
// does not fit is listed without answers.
func (rt *Runtime) askAnalysisQuestions(ctx context.Context, exec toolregistry.ExecutionContext, owner, computer, runID string, a *yaegikernel.StoryAnalysis) {
	perRun, perDay := rt.system1Limits(ctx, owner, computer)
	if perRun <= 0 || perDay <= 0 {
		a.Error = "System 1 is off on this computer, so the candidates carry no answers; ask with choir.System1 when it is on"
		return
	}
	if rt.candidateJudge == nil {
		a.Error = "no decision model is bound on this computer, so the candidates carry no answers"
		return
	}
	story := []string{system1StoryRefPrefix + a.Story.RevisionID}
	slots := []analysisSlot{{
		question: yaegikernel.System1Question{ID: "novelty", Text: noveltyQuestion(a.Candidates), Choices: []string{analysisNoveltyYes, analysisNoveltyNo}, ContextRefs: story},
		subject:  system1Subject{Kind: analysisSubjectNovelty},
		fill:     &a.Novelty,
	}}
	for i := range a.Candidates {
		c := &a.Candidates[i]
		need := 1
		if c.Source != types.EntityCandidateExisting {
			need = 2
		}
		if len(slots)+need > analysisMaxQuestions {
			continue
		}
		n := i + 1
		subject := system1Subject{EntityID: c.EntityID, Phrase: c.Phrase, Key: entitytext.Key(c.Phrase)}
		if need == 2 {
			s := subject
			s.Kind = analysisSubjectType
			slots = append(slots, analysisSlot{question: yaegikernel.System1Question{ID: fmt.Sprintf("type-%d", n), Text: typeQuestion(c.Phrase), Choices: analysisTypeChoices, ContextRefs: story}, subject: s, fill: &c.Kind})
		}
		s := subject
		s.Kind = analysisSubjectRole
		slots = append(slots, analysisSlot{question: yaegikernel.System1Question{ID: fmt.Sprintf("role-%d", n), Text: roleQuestion(c.EntityCandidate), Choices: analysisRoleChoices, ContextRefs: story}, subject: s, fill: &c.Role})
	}
	for start := 0; start < len(slots); start += yaegikernel.System1MaxQuestions {
		end := min(start+yaegikernel.System1MaxQuestions, len(slots))
		batch := slots[start:end]
		qs := make([]yaegikernel.System1Question, 0, len(batch))
		subjects := map[string]system1Subject{}
		for _, s := range batch {
			qs = append(qs, s.question)
			subjects[s.question.ID] = s.subject
		}
		valid, err := yaegikernel.ValidateSystem1HostQuestions(qs)
		if err != nil {
			log.Printf("story analysis: %s on run %s: a host question is malformed: %v", a.Story.DocID, runID, err)
			a.Error = "the analysis questions could not be asked: " + err.Error()
			return
		}
		raw, err := rt.system1Call(ctx, exec, owner, computer, runID, perRun, perDay, system1Ask{questions: valid, crossRun: true, subjects: subjects})
		if err != nil {
			log.Printf("story analysis: %s on run %s: System 1: %v", a.Story.DocID, runID, err)
			a.Error = "System 1 did not answer every analysis question: " + err.Error()
			return
		}
		var result system1Result
		if err := json.Unmarshal(raw, &result); err != nil {
			a.Error = "System 1's answer did not decode: " + err.Error()
			return
		}
		byID := map[string]system1Answer{}
		for _, ans := range result.Answers {
			byID[ans.ID] = ans
		}
		for _, s := range batch {
			ans := byID[s.question.ID]
			out := &yaegikernel.StoryAnalysisAnswer{ReceiptRef: result.ReceiptRef, QuestionID: s.question.ID, Probabilities: ans.Probabilities, Error: ans.Error}
			out.Top = topChoice(s.question.Choices, ans.Probabilities)
			*s.fill = out
		}
	}
}

// topChoice is the choice System 1 ranked first; a tie goes to the earlier
// choice.
func topChoice(choices []string, probs map[string]float64) string {
	top, best := "", -1.0
	for _, c := range choices {
		if p, ok := probs[c]; ok && p > best {
			top, best = c, p
		}
	}
	return top
}

func quotedName(name string) string {
	name = strings.Join(strings.Fields(name), " ")
	if r := []rune(name); len(r) > analysisPhraseRunes {
		name = string(r[:analysisPhraseRunes]) + "…"
	}
	return `"` + strings.ReplaceAll(name, `"`, `'`) + `"`
}

func roleQuestion(c types.EntityCandidate) string {
	var evidence string
	if c.Source == types.EntityCandidateExisting {
		kind := firstNonEmpty(c.Type, "entity")
		stories := 0
		if c.Facts != nil {
			stories = c.Facts.Stories
		}
		evidence = fmt.Sprintf("It is a known %s, in %d earlier stories; this story names it %d times", kind, stories, c.Count)
	} else {
		evidence = fmt.Sprintf("It is a phrase found in the story, named %d times", c.Count)
	}
	if c.InTitle {
		evidence += ", in the title too"
	}
	return fmt.Sprintf("Is this story centrally about %s (central), does it name it only in passing (incidental), or is it not about it (not about)? %s.", quotedName(firstNonEmpty(c.Name, c.Phrase)), evidence)
}

func typeQuestion(phrase string) string {
	return fmt.Sprintf("In this story, what is %s? Choose %q if it is not the name of one particular thing, and %q if it names one thing of a kind not listed.", quotedName(phrase), analysisNotAnEntity, analysisNoneFit)
}

// noveltyQuestion lists the candidates' names, as many as the question's
// bound allows.
func noveltyQuestion(cs []yaegikernel.StoryAnalysisCandidate) string {
	const head, tail, more = "Is this story centrally about a subject that is not one of these: ", "?", "; and others"
	var names []string
	for _, c := range cs {
		names = append(names, quotedName(firstNonEmpty(c.Name, c.Phrase)))
	}
	text := head + strings.Join(names, "; ") + tail
	for len(names) > 0 && utf8.RuneCountInString(text) > yaegikernel.System1MaxTextRunes {
		names = names[:len(names)-1]
		text = head + strings.Join(names, "; ") + more + tail
	}
	return text
}

// fitStoryAnalysis keeps the analysis within its bound: the prior stories
// and co-entities of the lowest-ranked candidates go first, then the
// lowest-ranked candidates.
func fitStoryAnalysis(a *yaegikernel.StoryAnalysis) {
	size := func() int {
		raw, err := json.Marshal(a)
		if err != nil {
			return 0
		}
		return len(raw)
	}
	for size() > yaegikernel.StoryAnalysisMaxBytes {
		if i := lastCandidateWithFacts(a.Candidates); i >= 0 {
			facts := *a.Candidates[i].Facts
			facts.PriorStories, facts.CoEntities = nil, nil
			a.Candidates[i].Facts = &facts
			a.Trimmed++
			continue
		}
		if len(a.Candidates) == 0 {
			return
		}
		a.Candidates = a.Candidates[:len(a.Candidates)-1]
		a.Dropped++
	}
}

func lastCandidateWithFacts(cs []yaegikernel.StoryAnalysisCandidate) int {
	for i := len(cs) - 1; i >= 0; i-- {
		if f := cs[i].Facts; f != nil && (len(f.PriorStories) > 0 || len(f.CoEntities) > 0) {
			return i
		}
	}
	return -1
}
