package runtime

import (
	"context"
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/types"
)

type vtextHandoffKind string

const (
	vtextHandoffKindUserPrompt vtextHandoffKind = "user_prompt"
	vtextHandoffKindSourceOpen vtextHandoffKind = "source_open"
	vtextHandoffKindCorpusWake vtextHandoffKind = "corpus_wake"
)

type vtextHandoffRequest struct {
	Kind           vtextHandoffKind
	CallerProfile  string
	Objective      string
	Title          string
	ChannelID      string
	InitialContent string
	SourceItemIDs  []string
}

type vtextHandoffDecision struct {
	Kind vtextHandoffKind

	DocID           string
	Title           string
	UserRevisionID  string
	SeedRevisionID  string
	RevisionRunID   string
	InitialLoopID   string
	State           types.RunState
	CreatedDocument bool

	Conductor conductorDecision
}

func vtextHandoffKindForCaller(profile string) vtextHandoffKind {
	switch canonicalAgentProfile(profile) {
	case AgentProfileConductor:
		return vtextHandoffKindUserPrompt
	case AgentProfileProcessor:
		return vtextHandoffKindSourceOpen
	case AgentProfileReconciler:
		return vtextHandoffKindCorpusWake
	default:
		return ""
	}
}

func (rt *Runtime) ensureVTextHandoff(ctx context.Context, parentRec *types.RunRecord, req vtextHandoffRequest) (vtextHandoffDecision, error) {
	if rt == nil {
		return vtextHandoffDecision{}, fmt.Errorf("runtime unavailable")
	}
	switch req.Kind {
	case vtextHandoffKindUserPrompt:
		if parentRec == nil || agentProfileForRun(parentRec) != AgentProfileConductor {
			return vtextHandoffDecision{}, fmt.Errorf("user_prompt handoff requires a conductor run")
		}
		decision, err := rt.ensureConductorVTextRoute(ctx, parentRec, req.Objective, req.InitialContent)
		if err != nil {
			return vtextHandoffDecision{}, err
		}
		return vtextHandoffDecision{
			Kind:           vtextHandoffKindUserPrompt,
			DocID:          decision.DocID,
			Title:          decision.Title,
			UserRevisionID: decision.UserRevisionID,
			InitialLoopID:  decision.InitialLoopID,
			RevisionRunID:  decision.InitialLoopID,
			Conductor:      decision,
		}, nil
	case vtextHandoffKindSourceOpen, vtextHandoffKindCorpusWake:
		if req.Kind == vtextHandoffKindCorpusWake && strings.TrimSpace(req.ChannelID) == "" {
			return vtextHandoffDecision{}, fmt.Errorf("corpus_wake handoff requires existing doc_id as channel_id")
		}
		decision, err := rt.ensureCoagentVTextRevisionRoute(ctx, parentRec, coagentVTextRouteRequest{
			CallerProfile:  req.CallerProfile,
			Role:           AgentProfileVText,
			Profile:        AgentProfileVText,
			Objective:      req.Objective,
			Title:          req.Title,
			ChannelID:      req.ChannelID,
			InitialContent: req.InitialContent,
			SourceItemIDs:  req.SourceItemIDs,
		})
		if err != nil {
			return vtextHandoffDecision{}, err
		}
		return vtextHandoffDecision{
			Kind:            req.Kind,
			DocID:           decision.DocID,
			Title:           decision.Title,
			SeedRevisionID:  decision.SeedRevisionID,
			RevisionRunID:   decision.RevisionRunID,
			State:           decision.State,
			CreatedDocument: decision.CreatedDocument,
		}, nil
	default:
		return vtextHandoffDecision{}, fmt.Errorf("unsupported vtext handoff kind %q", req.Kind)
	}
}
