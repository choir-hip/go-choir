package runtime

import (
	"context"
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/types"
)

type textureHandoffKind string

const (
	textureHandoffKindUserPrompt textureHandoffKind = "user_prompt"
	textureHandoffKindSourceOpen textureHandoffKind = "source_open"
	textureHandoffKindCorpusWake textureHandoffKind = "corpus_wake"
)

type textureHandoffRequest struct {
	Kind           textureHandoffKind
	CallerProfile  string
	Objective      string
	Title          string
	ChannelID      string
	InitialContent string
	SourceItemIDs  []string
}

type textureHandoffDecision struct {
	Kind textureHandoffKind

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

func textureHandoffKindForCaller(profile string) textureHandoffKind {
	switch canonicalAgentProfile(profile) {
	case agentprofile.Conductor:
		return textureHandoffKindUserPrompt
	case agentprofile.Processor:
		return textureHandoffKindSourceOpen
	case agentprofile.Reconciler:
		return textureHandoffKindCorpusWake
	default:
		return ""
	}
}

func (rt *Runtime) ensureTextureHandoff(ctx context.Context, parentRec *types.RunRecord, req textureHandoffRequest) (textureHandoffDecision, error) {
	if rt == nil {
		return textureHandoffDecision{}, fmt.Errorf("runtime unavailable")
	}
	switch req.Kind {
	case textureHandoffKindUserPrompt:
		if parentRec == nil || agentProfileForRun(parentRec) != agentprofile.Conductor {
			return textureHandoffDecision{}, fmt.Errorf("user_prompt handoff requires a conductor run")
		}
		decision, err := rt.ensureConductorTextureRoute(ctx, parentRec, req.Objective, req.InitialContent)
		if err != nil {
			return textureHandoffDecision{}, err
		}
		return textureHandoffDecision{
			Kind:           textureHandoffKindUserPrompt,
			DocID:          decision.DocID,
			Title:          decision.Title,
			UserRevisionID: decision.UserRevisionID,
			InitialLoopID:  decision.InitialLoopID,
			RevisionRunID:  decision.InitialLoopID,
			Conductor:      decision,
		}, nil
	case textureHandoffKindSourceOpen, textureHandoffKindCorpusWake:
		if req.Kind == textureHandoffKindCorpusWake && strings.TrimSpace(req.ChannelID) == "" {
			return textureHandoffDecision{}, fmt.Errorf("corpus_wake handoff requires existing doc_id as channel_id")
		}
		decision, err := rt.ensureCoagentTextureRevisionRoute(ctx, parentRec, coagentTextureRouteRequest{
			CallerProfile:  req.CallerProfile,
			Role:           agentprofile.Texture,
			Profile:        agentprofile.Texture,
			Objective:      req.Objective,
			Title:          req.Title,
			ChannelID:      req.ChannelID,
			InitialContent: req.InitialContent,
			SourceItemIDs:  req.SourceItemIDs,
		})
		if err != nil {
			return textureHandoffDecision{}, err
		}
		return textureHandoffDecision{
			Kind:            req.Kind,
			DocID:           decision.DocID,
			Title:           decision.Title,
			SeedRevisionID:  decision.SeedRevisionID,
			RevisionRunID:   decision.RevisionRunID,
			State:           decision.State,
			CreatedDocument: decision.CreatedDocument,
		}, nil
	default:
		return textureHandoffDecision{}, fmt.Errorf("unsupported texture handoff kind %q", req.Kind)
	}
}
