package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// The host records a story's entities by default (B4,
// docs/evidence/token-efficiency-2026-11-09/followup/system-prompt-review.md:
// "Final-Report refusal as a procedure"). Before, a story research desk's final
// Report was refused until the desk copied System 1's role and kind answers,
// which the host had already asked for it, into choir.Relate("", entities):
// every run read StoryAnalysis in cell 1 and spent cells on the copy. Now the
// finish rule applies the analysis's top answers itself when the desk has not
// recorded the entities, through the same choir.Relate host step (same checks,
// same tape record, same export), and the desk overrides by calling Relate
// first. The default attaches only what needs no judgment of the desk's own:
//
//   - a candidate whose role answer ranks central or incidental first;
//   - for a phrase, only when its kind answer ranks a real type first (a
//     phrase System 1 judged not an entity, or none of these fit, is left
//     out: attaching it needs the desk's reason);
//   - nothing the analysis missed (that needs the novelty answer and a
//     reason).
//
// If the analysis holds no answer the host cannot default, and the rule
// refuses as before. Nothing resumes: this runs inside the final Report's own
// commit, once.

// defaultStoryEntities is the Relate call the analysis's top answers make, and
// whether the analysis answered anything (a call with no entities is the
// record that the story is about none of its candidates, which needs answers
// or an analysis that found no candidates).
func defaultStoryEntities(a *yaegikernel.StoryAnalysis) ([]yaegikernel.RelateEntity, bool) {
	if a == nil {
		return nil, false
	}
	var entities []yaegikernel.RelateEntity
	answered := false
	seen := map[string]bool{}
	for _, c := range a.Candidates {
		role := c.Role
		if role == nil || role.Error != "" || role.ReceiptRef == "" || role.QuestionID == "" {
			continue
		}
		answered = true
		if role.Top != types.EntityRoleCentral && role.Top != types.EntityRoleIncidental {
			continue
		}
		e := yaegikernel.RelateEntity{Role: role.Top, JudgmentRef: role.ReceiptRef, QuestionID: role.QuestionID}
		if c.EntityID != "" {
			e.EntityID = c.EntityID
		} else {
			kind := c.Kind
			name := strings.TrimSpace(c.Phrase)
			if kind == nil || kind.Error != "" || !types.EntityTypes[kind.Top] || name == "" {
				continue
			}
			e.Name, e.Type = name, kind.Top
		}
		if id := e.ResolvedID(); !seen[id] {
			seen[id] = true
			entities = append(entities, e)
		}
		if len(entities) == yaegikernel.RelateMaxEntities {
			break
		}
	}
	// An analysis that found no candidates, and says no error, is answered:
	// the empty call records it.
	if len(a.Candidates) == 0 && a.Error == "" {
		answered = true
	}
	return entities, answered
}

// recordDefaultStoryEntities records the story's entities from the run's
// analysis as choir.Relate("", entities) would. It returns an error when the
// host cannot (no analysis, no answer, or Relate refused); a platform-side
// failure is recorded by Relate itself and frees the rule on the next check.
func (r *rlmCallReduction) recordDefaultStoryEntities(ctx context.Context, doc string) error {
	rt := r.rt()
	analysis := rt.analyses.built(r.scope.RunID + "\x00" + doc)
	entities, ok := defaultStoryEntities(analysis)
	if !ok {
		return fmt.Errorf("the story's analysis holds no System 1 answer to apply")
	}
	payload, err := json.Marshal(yaegikernel.RelatePayload{Story: yaegikernel.RelateOwnStory, Entities: entities})
	if err != nil {
		return err
	}
	exec := toolregistry.ExecutionContext{RunID: r.scope.RunID, OwnerID: r.scope.OwnerID, ComputerID: r.scope.ComputerID, AgentID: r.scope.DeskAgentID, RunRecord: r.rec}
	_, err = rt.relateEgress(ctx, strings.TrimSpace(r.rec.AgentProfile), exec, payload)
	return err
}
