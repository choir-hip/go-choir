package textureowner

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
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

var errTextureSupervisionAppend = fmt.Errorf("Texture supervision append failed")

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
	Schema               string   `json:"schema,omitempty"`
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
	StartRequestDigest   string   `json:"start_request_digest,omitempty"`
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

	if current, err := h.Core.GetRun(ctx, rec.RunID, rec.OwnerID); err == nil {
		mergeStoredConductorRoute(rec, *current)
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
	if metadataStringValue(rec.Metadata, "input_source") == "prompt_bar" {
		if promptText := strings.TrimSpace(metadataStringValue(rec.Metadata, "seed_prompt")); promptText != "" {
			routeSeedPrompt = promptText
		}
	}
	userRevisionContent := routeSeedPrompt
	initialPrompt := strings.TrimSpace(objective)
	if metadataStringValue(rec.Metadata, "input_source") == "prompt_bar" {
		initialPrompt = routeSeedPrompt
	}
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
	computerID := strings.TrimSpace(h.Core.TextureSandboxID())
	if computerID == "" {
		return ConductorDecision{}, fmt.Errorf("start Texture lifecycle: computer identity unavailable")
	}
	lifecycleKey := strings.Join([]string{"choir:texture:lifecycle", rec.OwnerID, computerID, commandID}, ":")
	docID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(lifecycleKey+":document")).String()
	revisionID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(lifecycleKey+":revision:v0")).String()
	workItemID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(lifecycleKey+":work:initial")).String()
	trajectoryID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(lifecycleKey+":trajectory")).String()
	doc := types.Document{
		DocID: docID, OwnerID: rec.OwnerID, ComputerID: computerID, TrajectoryID: trajectoryID,
		Title: decision.Title, CreatedAt: now, UpdatedAt: now,
	}
	if metadataStringValue(rec.Metadata, "input_source") == "prompt_bar" {
		doc.Title = conductorWindowTitle(rec, routeSeedPrompt)
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
		CommandID: commandID, TrajectoryID: trajectoryID,
		Kind:           types.TrajectoryKindTask,
		SubjectRefs:    map[string]string{"artifact": "texture://documents/" + doc.DocID},
		SettlementRule: types.SettlementRule{Version: types.LifecycleReducerVersion, RequireNoOpenWorkItems: true, RequiredSubjectRefs: []string{"artifact"}},
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
	started, err := h.startSupervisionTrajectory(ctx, start)
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
	decision.Schema = types.DurableWorkSchemaV1
	decision.StartRequestDigest = start.StartRequestDigest
	decision.TrajectoryID = started.Trajectory.TrajectoryID

	if started.Agent != nil && started.WorkItem != nil {
		decision.SubjectID = started.Agent.AgentID
		decision.ObligationIDs = []string{started.WorkItem.WorkItemID}
	}
	decision.ReducerSeq = started.Trajectory.ReducerSeq
	decision.SnapshotCursor = started.Trajectory.ReducerSeq
	if started.Replay {
		snapshot, snapshotErr := h.Store.GetLifecycleSnapshot(ctx, rec.OwnerID, computerID, started.Trajectory.TrajectoryID)
		if snapshotErr != nil {
			return ConductorDecision{}, fmt.Errorf("reconstruct replayed Texture lifecycle: %w", snapshotErr)
		}
		if len(snapshot.Agents) != 1 || len(snapshot.WorkItems) == 0 {
			return ConductorDecision{}, fmt.Errorf("reconstruct replayed Texture lifecycle: durable subjects unavailable")
		}
		decision.SubjectID = snapshot.Agents[0].AgentID
		decision.ObligationIDs = make([]string, 0, len(snapshot.WorkItems))
		for _, work := range snapshot.WorkItems {
			decision.ObligationIDs = append(decision.ObligationIDs, work.WorkItemID)
		}
		decision.InitialLoopID = snapshot.Activation.RunID
	} else {
		initialRun, submitErr := h.submitTextureAgentRevisionRun(ctx, doc, rec.OwnerID, textureAgentRevisionRequest{
			Intent: "initial_conductor_workflow",
			Prompt: initialPrompt,
		}, 0)
		if submitErr != nil {
			return ConductorDecision{}, fmt.Errorf("start initial Texture agent revision: %w", submitErr)
		}
		decision.InitialLoopID = initialRun.RunID
	}
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
func (h *Handler) startSupervisionTrajectory(ctx context.Context, start types.StartLifecycleRequest) (types.LifecycleResult, error) {
	if h == nil || h.Core == nil || h.Store == nil {
		return types.LifecycleResult{}, fmt.Errorf("start supervision trajectory: authority unavailable")
	}
	intentID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(start.CommandID+":intent:v1")).String()
	revisionMetadata := start.InitialRevision.Metadata
	if len(revisionMetadata) == 0 {
		revisionMetadata = json.RawMessage(`{}`)
	}
	if !json.Valid(revisionMetadata) || revisionMetadata[0] != '{' {
		return types.LifecycleResult{}, fmt.Errorf("start supervision trajectory: revision metadata must be a JSON object")
	}
	metadataDigest := computerevent.DigestBytes(append(append([]byte(nil), revisionMetadata...), []byte(start.InitialRevision.Content)...))
	subjectRefs := make(map[string]string, len(start.SubjectRefs)+1)
	for key, value := range start.SubjectRefs {
		subjectRefs[key] = value
	}
	subjectRefs["doc_id"] = start.InitialDocument.DocID
	sourceGraph, err := json.Marshal(map[string]any{
		"body_doc": start.InitialRevision.BodyDoc, "source_entities": start.InitialRevision.SourceEntities,
		"citations": start.InitialRevision.Citations, "metadata": revisionMetadata,
	})
	if err != nil {
		return types.LifecycleResult{}, err
	}
	mutationBodies := []struct {
		kind string
		body any
	}{
		{"trajectory_started", map[string]any{
			"trajectory_kind": string(start.Kind), "subject_refs": subjectRefs,
			"intent_revision_id": intentID, "artifact_id": start.InitialDocument.DocID,
			"artifact_revision_id": start.InitialRevision.RevisionID, "texture_actor_id": start.Agent.AgentID,
			"initial_assignment_ids": []string{start.InitialWork.WorkItemID}, "objective": start.InitialWork.Objective,
		}},
		{"intent_revised", map[string]any{
			"intent_revision_id": intentID, "parent_intent_revision_id": nil,
			"intent": start.InitialWork.Objective, "material": false, "affected_targets": []any{},
		}},
		{"texture_revision", map[string]any{
			"artifact_id": start.InitialDocument.DocID, "revision_id": start.InitialRevision.RevisionID,
			"title": start.InitialDocument.Title, "parent_revision_id": nil, "content": start.InitialRevision.Content,
			"source_graph": json.RawMessage(sourceGraph), "metadata": revisionMetadata, "metadata_digest": metadataDigest,
			"narrative_kind": "owner_edit", "fulfills_intent_revision_id": intentID,
		}},
	}
	mutations := make([]computerevent.SupervisionMutation, 0, len(mutationBodies))
	for _, item := range mutationBodies {
		raw, err := json.Marshal(item.body)
		if err != nil {
			return types.LifecycleResult{}, err
		}
		mutations = append(mutations, computerevent.SupervisionMutation{Kind: item.kind, Body: raw})
	}
	transaction := computerevent.SupervisionTransaction{
		Schema: computerevent.SupervisionSchemaV1, Reducer: computerevent.SupervisionReducerV1,
		DigestRecipe: computerevent.SupervisionDigestRecipeV1, TransactionID: start.CommandID,
		TransactionClass: "open_trajectory", OwnerID: start.OwnerID, ComputerID: start.ComputerID,
		TrajectoryID: start.TrajectoryID, CommandID: start.CommandID, CommandDigest: computerevent.ZeroHead,
		Actor:     computerevent.SupervisionActor{ActorID: start.Agent.AgentID, Role: "texture", AuthorityRef: "texture:trajectory:" + start.TrajectoryID},
		Mutations: mutations,
	}
	if _, _, err := h.Core.AppendSupervisionTransaction(ctx, transaction); err != nil {
		return types.LifecycleResult{}, fmt.Errorf("%w: %w", errTextureSupervisionAppend, err)
	}
	snapshot, err := h.Store.GetLifecycleSnapshot(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	result := types.LifecycleResult{
		Trajectory: snapshot.Trajectory, Document: &snapshot.Document,
		Revision: &snapshot.HeadRevision, Events: snapshot.Events,
	}
	for index := range snapshot.WorkItems {
		if snapshot.WorkItems[index].WorkItemID == start.InitialWork.WorkItemID {
			result.WorkItem = &snapshot.WorkItems[index]
			break
		}
	}
	for index := range snapshot.Agents {
		if snapshot.Agents[index].AgentID == start.Agent.AgentID {
			result.Agent = &snapshot.Agents[index]
			break
		}
	}
	return result, nil
}

func (h *Handler) appendTextureRevision(ctx context.Context, doc types.Document, revision types.Revision, commandID string, actor computerevent.SupervisionActor) (types.Revision, error) {
	if h == nil || h.Core == nil || h.Store == nil {
		return types.Revision{}, fmt.Errorf("append Texture revision: authority unavailable")
	}
	if strings.TrimSpace(doc.ComputerID) == "" || strings.TrimSpace(doc.TrajectoryID) == "" {
		return types.Revision{}, h.refuseLegacyTextureWriter("append Texture revision without supervision scope")
	}
	snapshot, err := h.Store.GetSupervisionProjectionSnapshot(ctx, doc.OwnerID, doc.ComputerID, doc.TrajectoryID)
	if err != nil {
		return types.Revision{}, fmt.Errorf("append Texture revision: load supervision snapshot: %w", err)
	}
	if snapshot.Archived || snapshot.Settled || snapshot.IntentRevisionID == "" || snapshot.ArtifactHeadRevisionID == "" || snapshot.CanonicalEventHead == "" {
		return types.Revision{}, fmt.Errorf("append Texture revision: supervised artifact is not revisable")
	}
	textureActorID := currentTextureAgentID(doc.DocID)
	switch actor.Role {
	case "owner":
		if actor.ActorID != doc.OwnerID || actor.AuthorityRef != "owner:"+doc.OwnerID {
			return types.Revision{}, fmt.Errorf("append Texture revision: invalid owner authority")
		}
	case "texture":
		if actor.ActorID != textureActorID || actor.AuthorityRef != "texture:trajectory:"+doc.TrajectoryID {
			return types.Revision{}, fmt.Errorf("append Texture revision: invalid Texture authority")
		}
	default:
		return types.Revision{}, fmt.Errorf("append Texture revision: unsupported actor role %q", actor.Role)
	}
	metadata := revision.Metadata
	if len(metadata) == 0 {
		metadata = json.RawMessage(`{}`)
	}
	if !json.Valid(metadata) || metadata[0] != '{' {
		return types.Revision{}, fmt.Errorf("append Texture revision: revision metadata must be a JSON object")
	}
	sourceGraph, err := json.Marshal(map[string]any{
		"body_doc": revision.BodyDoc, "source_entities": revision.SourceEntities,
		"citations": revision.Citations, "metadata": metadata,
	})
	if err != nil {
		return types.Revision{}, fmt.Errorf("append Texture revision: source graph: %w", err)
	}
	narrativeKind := "owner_edit"
	if actor.Role == "texture" {
		narrativeKind = "texture_synthesis"
	}
	revisionBody, err := json.Marshal(map[string]any{
		"artifact_id": doc.DocID, "revision_id": revision.RevisionID, "title": doc.Title,
		"parent_revision_id": snapshot.ArtifactHeadRevisionID, "content": revision.Content, "source_graph": json.RawMessage(sourceGraph),
		"metadata":        metadata,
		"metadata_digest": computerevent.DigestBytes(append(append([]byte(nil), metadata...), []byte(revision.Content)...)),
		"narrative_kind":  narrativeKind, "fulfills_intent_revision_id": snapshot.IntentRevisionID,
	})
	if err != nil {
		return types.Revision{}, fmt.Errorf("append Texture revision: encode transaction body: %w", err)
	}
	parentID := snapshot.ArtifactHeadRevisionID
	revision.ParentRevisionID = parentID
	revision.ComputerID = doc.ComputerID
	revision.TrajectoryID = doc.TrajectoryID
	lifecycleVersion := uint64(snapshot.LifecycleVersion)
	transaction := computerevent.SupervisionTransaction{
		Schema: computerevent.SupervisionSchemaV1, Reducer: computerevent.SupervisionReducerV1,
		DigestRecipe: computerevent.SupervisionDigestRecipeV1, TransactionID: commandID,
		TransactionClass: "revise_artifact", OwnerID: doc.OwnerID, ComputerID: doc.ComputerID,
		TrajectoryID: doc.TrajectoryID, CommandID: commandID, CommandDigest: computerevent.ZeroHead,
		Actor: actor,
		Expected: computerevent.SupervisionExpected{
			CanonicalEventHead: &snapshot.CanonicalEventHead,
			LifecycleVersion:   &lifecycleVersion, IntentRevisionID: &snapshot.IntentRevisionID,
			ArtifactHeadRevisionID: &snapshot.ArtifactHeadRevisionID,
		},
		Mutations: []computerevent.SupervisionMutation{{Kind: "texture_revision", Body: revisionBody}},
	}
	if _, _, err := h.Core.AppendSupervisionTransaction(ctx, transaction); err != nil {
		return types.Revision{}, err
	}
	next, err := h.Store.GetLifecycleRevision(ctx, doc.OwnerID, doc.ComputerID, revision.RevisionID)
	if err != nil {
		return types.Revision{}, fmt.Errorf("append Texture revision: load projected revision: %w", err)
	}
	return next, nil
}

func (h *Handler) startTextureOwnerDocument(ctx context.Context, ownerID, title string, now time.Time) (types.Document, error) {
	if h == nil || h.Core == nil {
		return types.Document{}, fmt.Errorf("start Texture document: authority unavailable")
	}
	computerID := strings.TrimSpace(h.Core.TextureSandboxID())
	if computerID == "" {
		return types.Document{}, fmt.Errorf("start Texture document: Texture computer identity unavailable")
	}
	docID, trajectoryID := uuid.NewString(), uuid.NewString()
	revisionID, workItemID := uuid.NewString(), uuid.NewString()
	doc := types.Document{DocID: docID, OwnerID: ownerID, ComputerID: computerID, TrajectoryID: trajectoryID, Title: title, CreatedAt: now, UpdatedAt: now}
	revision := types.Revision{
		RevisionID: revisionID, DocID: docID, OwnerID: ownerID, ComputerID: computerID, TrajectoryID: trajectoryID,
		AuthorKind: types.AuthorUser, AuthorLabel: ownerID, Content: "# " + title + "\n", Citations: json.RawMessage("[]"), Metadata: json.RawMessage(`{}`), CreatedAt: now,
	}
	agentID := currentTextureAgentID(docID)
	start := types.StartLifecycleRequest{
		OwnerID: ownerID, ComputerID: computerID, CommandID: "texture-owner-document:" + docID, TrajectoryID: trajectoryID,
		Kind: types.TrajectoryKindTask, SubjectRefs: map[string]string{"artifact": "texture://documents/" + docID},
		SettlementRule:  types.SettlementRule{Version: types.LifecycleReducerVersion, RequireNoOpenWorkItems: true, RequiredSubjectRefs: []string{"artifact"}},
		InitialWork:     types.WorkItemRecord{WorkItemID: workItemID, Objective: "Maintain Texture document " + title, AssignedAgentID: agentID, AuthorityProfile: agentprofile.Texture},
		InitialDocument: doc, InitialRevision: revision,
		Agent: types.AgentRecord{AgentID: agentID, OwnerID: ownerID, ComputerID: computerID, SandboxID: computerID, Profile: agentprofile.Texture, Role: agentprofile.Texture, ChannelID: docID, CreatedAt: now, UpdatedAt: now},
	}
	start.StartRequestDigest, _ = store.ComputeStartLifecycleRequestDigest(start)
	result, err := h.startSupervisionTrajectory(ctx, start)
	if err != nil || result.Document == nil {
		if err == nil {
			err = fmt.Errorf("start Texture document: projected document unavailable")
		}
		return types.Document{}, err
	}
	return *result.Document, nil
}
