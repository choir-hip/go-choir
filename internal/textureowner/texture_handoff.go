package textureowner

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/modelpolicy"
	"github.com/yusefmosiah/go-choir/internal/provider"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// HandoffKind identifies the product path that transfers work to Texture.
type HandoffKind string

const (
	HandoffKindUserPrompt HandoffKind = "user_prompt"
	HandoffKindSourceOpen HandoffKind = "source_open"
	HandoffKindCorpusWake HandoffKind = "corpus_wake"
)

// HandoffRequest is the owner-local request for a Texture lifecycle handoff.
type HandoffRequest struct {
	Kind           HandoffKind
	CallerProfile  string
	Objective      string
	Title          string
	ChannelID      string
	InitialContent string
	SourceItemIDs  []string
}

// ConductorDecision is the durable prompt-bar result that opens Texture.
type ConductorDecision struct {
	Action               string   `json:"action"`
	App                  string   `json:"app,omitempty"`
	Title                string   `json:"title,omitempty"`
	SeedPrompt           string   `json:"seed_prompt,omitempty"`
	InitialContent       string   `json:"initial_content,omitempty"`
	CreateInitialVersion *bool    `json:"create_initial_version,omitempty"`
	Message              string   `json:"message,omitempty"`
	SourceURL            string   `json:"source_url,omitempty"`
	MediaType            string   `json:"media_type,omitempty"`
	AppHint              string   `json:"app_hint,omitempty"`
	ContentID            string   `json:"content_id,omitempty"`
	DocID                string   `json:"doc_id,omitempty"`
	UserRevisionID       string   `json:"user_revision_id,omitempty"`
	FramingRevisionID    string   `json:"framing_revision_id,omitempty"`
	InitialRevisionID    string   `json:"initial_revision_id,omitempty"`
	InitialLoopID        string   `json:"initial_loop_id,omitempty"`
	CommandID            string   `json:"command_id,omitempty"`
	TrajectoryID         string   `json:"trajectory_id,omitempty"`
	SubjectID            string   `json:"subject_id,omitempty"`
	ObligationIDs        []string `json:"obligation_ids,omitempty"`
	ReducerSeq           int64    `json:"reducer_seq,omitempty"`
	SnapshotCursor       int64    `json:"snapshot_cursor,omitempty"`
}

// HandoffDecision records the durable Texture objects created or reused by a handoff.
type HandoffDecision struct {
	Kind HandoffKind

	DocID           string
	Title           string
	UserRevisionID  string
	SeedRevisionID  string
	RevisionRunID   string
	InitialLoopID   string
	State           types.RunState
	CreatedDocument bool

	Conductor ConductorDecision
}

// HandoffKindForCaller maps lifecycle actor profiles to their Texture product path.
func HandoffKindForCaller(profile string) HandoffKind {
	switch agentprofile.Canonical(profile) {
	case agentprofile.Conductor:
		return HandoffKindUserPrompt
	case agentprofile.Processor:
		return HandoffKindSourceOpen
	case agentprofile.Reconciler:
		return HandoffKindCorpusWake
	default:
		return ""
	}
}

// EnsureTextureHandoff owns the product transition from a lifecycle actor into Texture.
func (h *Handler) EnsureTextureHandoff(ctx context.Context, parentRec *types.RunRecord, req HandoffRequest) (HandoffDecision, error) {
	if h == nil {
		return HandoffDecision{}, fmt.Errorf("texture owner unavailable")
	}
	switch req.Kind {
	case HandoffKindUserPrompt:
		if parentRec == nil || agentProfileForRun(parentRec) != agentprofile.Conductor {
			return HandoffDecision{}, fmt.Errorf("user_prompt handoff requires a conductor run")
		}
		decision, err := h.ensureConductorTextureRoute(ctx, parentRec, req.Objective, req.InitialContent)
		if err != nil {
			return HandoffDecision{}, err
		}
		return HandoffDecision{
			Kind:           HandoffKindUserPrompt,
			DocID:          decision.DocID,
			Title:          decision.Title,
			UserRevisionID: decision.UserRevisionID,
			InitialLoopID:  decision.InitialLoopID,
			RevisionRunID:  decision.InitialLoopID,
			Conductor:      decision,
		}, nil
	case HandoffKindSourceOpen, HandoffKindCorpusWake:
		if req.Kind == HandoffKindCorpusWake && strings.TrimSpace(req.ChannelID) == "" {
			return HandoffDecision{}, fmt.Errorf("corpus_wake handoff requires existing doc_id as channel_id")
		}
		decision, err := h.ensureCoagentTextureRevisionRoute(ctx, parentRec, coagentTextureRouteRequest{
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
			return HandoffDecision{}, err
		}
		return HandoffDecision{
			Kind:            req.Kind,
			DocID:           decision.DocID,
			Title:           decision.Title,
			SeedRevisionID:  decision.SeedRevisionID,
			RevisionRunID:   decision.RevisionRunID,
			State:           decision.State,
			CreatedDocument: decision.CreatedDocument,
		}, nil
	default:
		return HandoffDecision{}, fmt.Errorf("unsupported texture handoff kind %q", req.Kind)
	}
}

func (h *Handler) ensureConductorTextureRoute(ctx context.Context, rec *types.RunRecord, objective, initialContent string) (ConductorDecision, error) {
	if rec == nil || agentProfileForRun(rec) != agentprofile.Conductor {
		return ConductorDecision{}, fmt.Errorf("conductor route requires a conductor record")
	}
	if h.Store == nil || h.Core == nil {
		return ConductorDecision{}, fmt.Errorf("texture lifecycle unavailable")
	}

	if current, err := h.Store.GetRun(ctx, rec.RunID); err == nil {
		mergeStoredConductorRoute(rec, current)
	}

	var parsedDecision ConductorDecision
	if raw := strings.TrimSpace(rec.Result); raw != "" {
		if err := json.Unmarshal([]byte(raw), &parsedDecision); err == nil {
			if strings.TrimSpace(initialContent) == "" {
				initialContent = parsedDecision.InitialContent
			}
			if parsedDecision.Action == "open_app" &&
				isTextureDecisionApp(parsedDecision.App) &&
				strings.TrimSpace(parsedDecision.DocID) != "" {
				return fillConductorDecisionFromRun(rec, parsedDecision), nil
			}
		}
	}
	existing := fillConductorDecisionFromRun(rec, ConductorDecision{})
	if existing.Action == "open_app" && isTextureDecisionApp(existing.App) && strings.TrimSpace(existing.DocID) != "" {
		return existing, nil
	}

	now := time.Now().UTC()
	decision := fillConductorDecisionFromRun(rec, parsedDecision)
	decision.CreateInitialVersion = boolPointer(false)
	decision.InitialContent = ""
	initialContent = ""
	_ = initialContent
	routeSeedPrompt := firstNonEmptyString(
		strings.TrimSpace(decision.SeedPrompt),
		provider.ConductorSeedPrompt(rec),
		strings.TrimSpace(rec.Prompt),
		metadataStringValue(rec.Metadata, "seed_prompt"),
	)
	userRevisionContent := routeSeedPrompt
	if metadataStringValue(rec.Metadata, "input_source") == "prompt_bar" {
		if promptText := strings.TrimSpace(metadataStringValue(rec.Metadata, "seed_prompt")); promptText != "" {
			userRevisionContent = promptText
		}
	}
	initialPrompt := strings.TrimSpace(objective)
	if initialPrompt == "" {
		initialPrompt = routeSeedPrompt
	}
	if initialPrompt == "" {
		initialPrompt = "Create the first useful current-state version of this Texture document."
	}
	commandID := strings.TrimSpace(metadataStringValue(rec.Metadata, "lifecycle_command_id"))
	if commandID == "" {
		return ConductorDecision{}, fmt.Errorf("start Texture lifecycle: durable command identity unavailable")
	}
	lifecycleKey := "choir:texture:lifecycle:" + commandID
	docID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(lifecycleKey+":document")).String()
	revisionID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(lifecycleKey+":revision:v0")).String()
	workItemID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(lifecycleKey+":work:initial")).String()
	trajectoryID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(lifecycleKey+":trajectory")).String()
	computerID := strings.TrimSpace(h.Core.TextureSandboxID())
	if computerID == "" {
		return ConductorDecision{}, fmt.Errorf("start Texture lifecycle: computer identity unavailable")
	}
	doc := types.Document{
		DocID: docID, OwnerID: rec.OwnerID, ComputerID: computerID, TrajectoryID: trajectoryID,
		Title: decision.Title, CreatedAt: now, UpdatedAt: now,
	}
	if strings.TrimSpace(doc.Title) == "" {
		doc.Title = "Texture"
	}
	userRevisionMetadata := map[string]any{
		"seed_prompt": routeSeedPrompt, "conductor_loop_id": rec.RunID,
		"trajectory_id": trajectoryID, modelpolicy.MetadataPolicyOverlayID: metadataString(rec.Metadata, modelpolicy.MetadataPolicyOverlayID),
		"owner_email": metadataString(rec.Metadata, "owner_email"), "created_from": "conductor",
		"source": "user_prompt", "revision_role": "input", "input_origin": "user_prompt",
		"texture_version": "v0", "prompt_unix_ts": now.Unix(),
	}
	userRevMeta, _ := json.Marshal(userRevisionMetadata)
	userRev := types.Revision{
		RevisionID: revisionID, DocID: doc.DocID, OwnerID: rec.OwnerID, ComputerID: computerID, TrajectoryID: trajectoryID,
		AuthorKind: types.AuthorUser, AuthorLabel: rec.OwnerID, Content: userRevisionContent,
		Citations: json.RawMessage("[]"), Metadata: userRevMeta, CreatedAt: now,
	}
	agentID := currentTextureAgentID(doc.DocID)
	start := types.StartLifecycleRequest{
		OwnerID: rec.OwnerID, ComputerID: computerID,
		CommandID: lifecycleKey, TrajectoryID: trajectoryID,
		Kind:        types.TrajectoryKindTask,
		SubjectRefs: map[string]string{"artifact": "texture://documents/" + doc.DocID},
		SettlementRule: types.SettlementRule{
			RequireNoOpenWorkItems: true, RequiredSubjectRefs: []string{"artifact"},
		},
		InitialWork: types.WorkItemRecord{
			WorkItemID: workItemID, Objective: initialPrompt, AssignedAgentID: agentID,
			AuthorityProfile: agentprofile.Texture,
		},
		InitialDocument: doc, InitialRevision: userRev,
		Agent: types.AgentRecord{
			AgentID: agentID, OwnerID: rec.OwnerID, ComputerID: computerID, SandboxID: computerID,
			Profile: agentprofile.Texture, Role: agentprofile.Texture, ChannelID: doc.DocID,
			CreatedAt: now, UpdatedAt: now,
		},
	}
	start.StartRequestDigest, _ = store.ComputeStartLifecycleRequestDigest(start)
	started, err := h.Store.StartLifecycle(ctx, start)
	if err != nil {
		return ConductorDecision{}, fmt.Errorf("start Texture lifecycle: %w", err)
	}
	doc, userRev = *started.Document, *started.Revision
	h.emitTextureDocumentRevisionEventForRun(ctx, rec, userRev)
	decision.DocID = doc.DocID
	decision.UserRevisionID = userRev.RevisionID
	if decision.InitialRevisionID == "" {
		decision.InitialRevisionID = userRev.RevisionID
	}
	decision.CommandID = start.CommandID
	decision.TrajectoryID = started.Trajectory.TrajectoryID
	decision.SubjectID = started.Agent.AgentID
	decision.ObligationIDs = []string{started.WorkItem.WorkItemID}
	decision.ReducerSeq = started.Trajectory.ReducerSeq
	decision.SnapshotCursor = started.Trajectory.ReducerSeq
	initialRun, err := h.submitTextureAgentRevisionRun(ctx, doc, rec.OwnerID, textureAgentRevisionRequest{
		Intent: "initial_conductor_workflow",
		Prompt: initialPrompt,
	}, 0)
	if err != nil {
		return ConductorDecision{}, fmt.Errorf("start initial Texture agent revision: %w", err)
	}
	decision.InitialLoopID = initialRun.RunID
	decision = fillConductorDecisionFromRun(rec, decision)

	if rec.Metadata == nil {
		rec.Metadata = make(map[string]any)
	}
	rec.Metadata["trajectory_id"] = trajectoryID
	rec.Metadata["doc_id"] = decision.DocID
	rec.Metadata["user_revision_id"] = decision.UserRevisionID
	rec.Metadata["initial_revision_id"] = decision.InitialRevisionID
	rec.Metadata["initial_loop_id"] = decision.InitialLoopID
	if out, err := json.Marshal(decision); err == nil {
		rec.Result = string(out)
	}
	rec.UpdatedAt = time.Now().UTC()
	if err := h.Store.UpdateRun(ctx, *rec); err != nil {
		return ConductorDecision{}, fmt.Errorf("persist conductor route: %w", err)
	}
	return decision, nil
}

func conductorRequestedApp(rec *types.RunRecord) string {
	if rec == nil {
		return agentprofile.Texture
	}
	requestedApp := metadataStringValue(rec.Metadata, "requested_app")
	if strings.TrimSpace(requestedApp) == "" {
		requestedApp = agentprofile.Texture
	}
	if isTextureDecisionApp(requestedApp) {
		return agentprofile.Texture
	}
	return strings.TrimSpace(requestedApp)
}

func isTextureDecisionApp(app string) bool {
	return strings.EqualFold(strings.TrimSpace(app), agentprofile.Texture)
}

func conductorWindowTitle(rec *types.RunRecord, seedPrompt string) string {
	if rec == nil {
		if title := strings.TrimSpace(seedPrompt); title != "" {
			return title
		}
		return "Texture"
	}
	title := metadataStringValue(rec.Metadata, "initial_document_title")
	if title == "" {
		title = strings.TrimSpace(seedPrompt)
	}
	if title == "" {
		title = "Texture"
	}
	return title
}

func fillConductorDecisionFromRun(rec *types.RunRecord, decision ConductorDecision) ConductorDecision {
	seedPrompt := provider.ConductorSeedPrompt(rec)
	requestedApp := conductorRequestedApp(rec)
	if strings.TrimSpace(decision.Action) == "" {
		decision.Action = "open_app"
	}
	if decision.Action == "open_app" {
		if strings.TrimSpace(decision.App) == "" {
			decision.App = requestedApp
		}
		if strings.TrimSpace(decision.Title) == "" {
			decision.Title = conductorWindowTitle(rec, seedPrompt)
		}
		if strings.TrimSpace(decision.SeedPrompt) == "" {
			decision.SeedPrompt = seedPrompt
		}
		if isTextureDecisionApp(decision.App) {
			decision.App = agentprofile.Texture
			decision.CreateInitialVersion = boolPointer(false)
			decision.InitialContent = ""
		}
		if rec != nil {
			if decision.SourceURL == "" {
				decision.SourceURL = metadataStringValue(rec.Metadata, "content_source_url")
			}
			if decision.MediaType == "" {
				decision.MediaType = metadataStringValue(rec.Metadata, "content_media_type")
			}
			if decision.AppHint == "" {
				decision.AppHint = metadataStringValue(rec.Metadata, "content_app_hint")
			}
			if decision.ContentID == "" {
				decision.ContentID = metadataStringValue(rec.Metadata, "content_id")
			}
			if decision.DocID == "" {
				decision.DocID = metadataStringValue(rec.Metadata, "doc_id")
			}
			if decision.UserRevisionID == "" {
				decision.UserRevisionID = metadataStringValue(rec.Metadata, "user_revision_id")
			}
			if decision.FramingRevisionID == "" {
				decision.FramingRevisionID = metadataStringValue(rec.Metadata, "framing_revision_id")
			}
			if decision.InitialRevisionID == "" {
				decision.InitialRevisionID = metadataStringValue(rec.Metadata, "initial_revision_id")
			}
			if decision.InitialLoopID == "" {
				decision.InitialLoopID = metadataStringValue(rec.Metadata, "initial_loop_id")
			}
		}
	}
	if decision.Action == "toast" && strings.TrimSpace(decision.Message) == "" {
		decision.Message = "Conductor acknowledged the request."
	}
	return decision
}

func mergeStoredConductorRoute(rec *types.RunRecord, stored types.RunRecord) {
	if rec == nil {
		return
	}
	if rec.Metadata == nil {
		rec.Metadata = make(map[string]any)
	}
	for _, key := range []string{"doc_id", "user_revision_id", "framing_revision_id", "initial_revision_id", "initial_loop_id"} {
		if value := metadataStringValue(stored.Metadata, key); value != "" {
			rec.Metadata[key] = value
		}
	}
	var storedDecision ConductorDecision
	if err := json.Unmarshal([]byte(strings.TrimSpace(stored.Result)), &storedDecision); err == nil &&
		storedDecision.Action == "open_app" &&
		isTextureDecisionApp(storedDecision.App) &&
		strings.TrimSpace(storedDecision.DocID) != "" {
		rec.Result = stored.Result
	}
}

func boolPointer(value bool) *bool { return &value }

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
