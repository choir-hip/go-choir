package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func RegisterCoAgentTools(registry *ToolRegistry, rt *Runtime, spec AgentRoleSpec) error {
	tools := []Tool{
		newCastAgentTool(rt),
		newCastAgentUpdateTool(rt),
		newWaitAgentTool(rt),
		newCancelAgentTool(rt),
	}
	if len(spec.AllowedDelegateTargets) > 0 {
		tools = append([]Tool{newSpawnAgentTool(rt, spec)}, tools...)
	}
	for _, tool := range tools {
		if err := registry.Register(tool); err != nil {
			return err
		}
	}
	return nil
}

func newSpawnAgentTool(rt *Runtime, spec AgentRoleSpec) Tool {
	type args struct {
		Objective            string `json:"objective"`
		Role                 string `json:"role"`
		Profile              string `json:"profile,omitempty"`
		ChannelID            string `json:"channel_id,omitempty"`
		Slot                 string `json:"slot,omitempty"`
		Model                string `json:"model,omitempty"`
		ModelPolicyOverlayID string `json:"model_policy_overlay_id,omitempty"`
		Title                string `json:"title,omitempty"`
		InitialContent       string `json:"initial_content,omitempty"`
	}
	allowedTargets := canonicalAllowedDelegateTargets(spec.AllowedDelegateTargets)
	roleDescription := "Canonical role/profile name. Allowed target roles for this caller: " + strings.Join(allowedTargets, ", ") + "."
	description := "Spawn an allowed child agent run for the current " + spec.Profile + " profile."
	if spec.Profile == AgentProfileConductor {
		description = "Open a VText document from a top-level conductor route. Conductor does not spawn researcher, super, or co-super workers."
	}
	return Tool{
		Name:        "spawn_agent",
		Description: description,
		Parameters: jsonSchemaObject(map[string]any{
			"objective":               map[string]any{"type": "string"},
			"role":                    map[string]any{"type": "string", "enum": allowedTargets, "description": roleDescription},
			"profile":                 map[string]any{"type": "string", "enum": allowedTargets, "description": "Optional canonical profile override. Usually omit; if set, it must be one of the allowed target roles for this caller."},
			"channel_id":              map[string]any{"type": "string"},
			"slot":                    map[string]any{"type": "string", "enum": []string{"implementation", "verifier"}, "description": "For vsuper spawning co-super children: use implementation for the candidate writer first; use verifier only after implementation commit/package/blocker evidence exists. Reusing a live slot returns the existing child instead of launching a duplicate."},
			"model":                   map[string]any{"type": "string"},
			"model_policy_overlay_id": map[string]any{"type": "string", "description": "Optional owner-visible model policy overlay id from System/model-policy-overlays/<id>.toml. Use this for eval/model arms instead of passing provider metadata directly."},
			"title":                   map[string]any{"type": "string", "description": "For role=vtext from processor or reconciler: optional VText document title for a new article."},
			"initial_content": map[string]any{
				"type":        "string",
				"description": "For role=vtext from processor or reconciler: optional source/brief seed revision for the VText before the VText agent writes the article.",
			},
		}, []string{"objective", "role"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var in args
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", fmt.Errorf("decode spawn_agent args: %w", err)
			}
			parentID := stringFromToolContext(ctx, toolCtxRunID)
			ownerID := stringFromToolContext(ctx, toolCtxOwnerID)
			if parentID == "" || ownerID == "" {
				return "", fmt.Errorf("spawn_agent missing run context")
			}
			role := normalizeDelegateTargetValue(in.Role, allowedTargets)
			if role == "" {
				return "", fmt.Errorf("role must not be empty")
			}
			callerProfile := stringFromToolContext(ctx, toolCtxProfile)
			profile := normalizeDelegateTargetValue(in.Profile, allowedTargets)
			if profile == "" {
				profile = role
			}
			if !canDelegateTo(callerProfile, profile) {
				return "", fmt.Errorf("%s cannot delegate to %s", callerProfile, profile)
			}
			slot := normalizeVSuperCoSuperSlot(in.Slot)
			if strings.TrimSpace(in.Slot) != "" && slot == "" {
				return "", fmt.Errorf("spawn_agent slot must be implementation or verifier")
			}
			if callerProfile == AgentProfileVSuper && profile == AgentProfileCoSuper && slot == "" {
				return "", fmt.Errorf("vsuper spawn_agent role=co-super requires slot=\"implementation\" or slot=\"verifier\" to avoid duplicate child runs")
			}
			constraints := map[string]any{
				runMetadataAgentRole:    role,
				runMetadataAgentProfile: profile,
			}
			if slot != "" {
				constraints[runMetadataCoSuperSlot] = slot
			}
			if channelID := strings.TrimSpace(in.ChannelID); channelID != "" {
				constraints[runMetadataChannelID] = channelID
			}
			if model := strings.TrimSpace(in.Model); model != "" {
				constraints[runMetadataModel] = model
			}
			if overlayID := strings.TrimSpace(in.ModelPolicyOverlayID); overlayID != "" {
				constraints[runMetadataLLMPolicyOverlayID] = overlayID
			}
			if callerProfile == AgentProfileConductor && profile == AgentProfileVText {
				parentRec, _ := ctx.Value(toolCtxRunRecord).(*types.RunRecord)
				if parentRec == nil {
					parentRec = &types.RunRecord{
						RunID:        parentID,
						OwnerID:      ownerID,
						AgentProfile: callerProfile,
					}
				}
				decision, err := rt.ensureConductorVTextRoute(ctx, parentRec, in.Objective, in.InitialContent)
				if err != nil {
					return "", err
				}
				return toolResultJSON(map[string]any{
					"action":                 decision.Action,
					"app":                    decision.App,
					"title":                  decision.Title,
					"seed_prompt":            decision.SeedPrompt,
					"initial_content":        decision.InitialContent,
					"create_initial_version": decision.CreateInitialVersion != nil && *decision.CreateInitialVersion,
					"agent_id":               "vtext:" + decision.DocID,
					"doc_id":                 decision.DocID,
					"user_revision_id":       decision.UserRevisionID,
					"framing_revision_id":    decision.FramingRevisionID,
					"initial_revision_id":    decision.InitialRevisionID,
					"initial_loop_id":        decision.InitialLoopID,
					"loop_id":                decision.InitialLoopID,
					"channel_id":             decision.DocID,
					"role":                   role,
					"profile":                profile,
					"state":                  "open",
				})
			}
			if (callerProfile == AgentProfileProcessor || callerProfile == AgentProfileReconciler) && profile == AgentProfileVText {
				parentRec, _ := ctx.Value(toolCtxRunRecord).(*types.RunRecord)
				if parentRec == nil {
					parentRec = &types.RunRecord{
						RunID:        parentID,
						OwnerID:      ownerID,
						AgentID:      stringFromToolContext(ctx, toolCtxAgentID),
						AgentProfile: callerProfile,
						AgentRole:    stringFromToolContext(ctx, toolCtxRole),
						ChannelID:    stringFromToolContext(ctx, toolCtxChannelID),
					}
				}
				decision, err := rt.ensureCoagentVTextRevisionRoute(ctx, parentRec, coagentVTextRouteRequest{
					CallerProfile:  callerProfile,
					Role:           role,
					Profile:        profile,
					Objective:      in.Objective,
					Title:          in.Title,
					ChannelID:      in.ChannelID,
					InitialContent: in.InitialContent,
				})
				if err != nil {
					return "", err
				}
				return toolResultJSON(map[string]any{
					"agent_id":             "vtext:" + decision.DocID,
					"doc_id":               decision.DocID,
					"seed_revision_id":     decision.SeedRevisionID,
					"loop_id":              decision.RevisionRunID,
					"revision_loop_id":     decision.RevisionRunID,
					"channel_id":           decision.DocID,
					"role":                 role,
					"profile":              profile,
					"state":                decision.State,
					"title":                decision.Title,
					"created_document":     decision.CreatedDocument,
					"revised_existing_doc": !decision.CreatedDocument,
				})
			}
			child, err := rt.StartChildRun(ctx, parentID, in.Objective, ownerID, constraints)
			if err != nil {
				return "", err
			}
			result := map[string]any{
				"agent_id":   child.AgentID,
				"loop_id":    child.RunID,
				"channel_id": child.ChannelID,
				"role":       role,
				"profile":    profile,
				"state":      child.State,
			}
			if slot := metadataStringValue(child.Metadata, runMetadataCoSuperSlot); slot != "" {
				result["slot"] = slot
			}
			if metadataBoolValue(child.Metadata, runMetadataSpawnReused) {
				result["reused_existing_child"] = true
			}
			return toolResultJSON(result)
		},
	}
}

type coagentVTextRouteRequest struct {
	CallerProfile  string
	Role           string
	Profile        string
	Objective      string
	Title          string
	ChannelID      string
	InitialContent string
}

type coagentVTextRouteDecision struct {
	DocID           string
	Title           string
	SeedRevisionID  string
	RevisionRunID   string
	State           types.RunState
	CreatedDocument bool
}

func (rt *Runtime) ensureCoagentVTextRevisionRoute(ctx context.Context, parentRec *types.RunRecord, req coagentVTextRouteRequest) (coagentVTextRouteDecision, error) {
	if rt == nil || rt.store == nil {
		return coagentVTextRouteDecision{}, fmt.Errorf("runtime store unavailable")
	}
	if parentRec == nil {
		return coagentVTextRouteDecision{}, fmt.Errorf("vtext route requires a parent run")
	}
	callerProfile := canonicalAgentProfile(req.CallerProfile)
	if callerProfile != AgentProfileProcessor && callerProfile != AgentProfileReconciler {
		return coagentVTextRouteDecision{}, fmt.Errorf("vtext route requires processor or reconciler caller")
	}
	ownerID := strings.TrimSpace(parentRec.OwnerID)
	if ownerID == "" {
		return coagentVTextRouteDecision{}, fmt.Errorf("owner_id is required")
	}

	now := time.Now().UTC()
	sourceEntities := rt.coagentVTextSourceEntities(ctx, parentRec, req)
	doc, created, seedRevisionID, err := rt.coagentVTextTargetDocument(ctx, parentRec, req, now, sourceEntities)
	if err != nil {
		return coagentVTextRouteDecision{}, err
	}

	if err := rt.store.UpsertAgent(ctx, types.AgentRecord{
		AgentID:   "vtext:" + doc.DocID,
		OwnerID:   ownerID,
		SandboxID: rt.cfg.SandboxID,
		Profile:   AgentProfileVText,
		Role:      AgentProfileVText,
		ChannelID: doc.DocID,
		CreatedAt: now,
		UpdatedAt: time.Now().UTC(),
	}); err != nil {
		return coagentVTextRouteDecision{}, fmt.Errorf("persist vtext appagent: %w", err)
	}
	if _, err := rt.EnsurePersistentSuperAgent(ctx, ownerID); err != nil {
		return coagentVTextRouteDecision{}, fmt.Errorf("persist persistent super appagent: %w", err)
	}

	prompt := buildCoagentVTextRevisionPrompt(parentRec, req, doc, created, sourceEntities)
	rec, err := rt.submitVTextAgentRevisionRun(ctx, doc, ownerID, vtextAgentRevisionRequest{
		Intent: "global_wire_" + callerProfile + "_article_revision",
		Prompt: prompt,
	}, parentRec.RunID, 0)
	if err != nil {
		return coagentVTextRouteDecision{}, fmt.Errorf("start vtext article revision: %w", err)
	}

	return coagentVTextRouteDecision{
		DocID:           doc.DocID,
		Title:           doc.Title,
		SeedRevisionID:  seedRevisionID,
		RevisionRunID:   rec.RunID,
		State:           rec.State,
		CreatedDocument: created,
	}, nil
}

func (rt *Runtime) coagentVTextTargetDocument(ctx context.Context, parentRec *types.RunRecord, req coagentVTextRouteRequest, now time.Time, sourceEntities []vtextSourceEntity) (types.Document, bool, string, error) {
	ownerID := strings.TrimSpace(parentRec.OwnerID)
	channelID := strings.TrimSpace(req.ChannelID)
	if channelID != "" {
		if doc, err := rt.store.GetDocument(ctx, channelID, ownerID); err == nil {
			return doc, false, "", nil
		} else if err != store.ErrNotFound {
			return types.Document{}, false, "", fmt.Errorf("lookup vtext document %s: %w", channelID, err)
		}
	}

	title := coagentVTextTitle(req)
	doc := types.Document{
		DocID:     uuid.New().String(),
		OwnerID:   ownerID,
		Title:     title,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := rt.store.CreateDocument(ctx, doc); err != nil {
		return types.Document{}, false, "", fmt.Errorf("create vtext document: %w", err)
	}

	seedContent := coagentVTextSeedContent(parentRec, req, sourceEntities)
	seedRevisionID := uuid.New().String()
	selectedStyles, styleRationale := coagentVTextSelectedStyles(req)
	seedMetaMap := map[string]any{
		"source":                      "coagent_vtext_seed",
		"artifact_kind":               "source_brief",
		"article_version":             false,
		"vtext_version_stage":         "pre_article_brief",
		"created_from":                canonicalAgentProfile(req.CallerProfile),
		"parent_loop_id":              parentRec.RunID,
		"parent_agent_id":             strings.TrimSpace(parentRec.AgentID),
		"parent_channel_id":           strings.TrimSpace(parentRec.ChannelID),
		"requested_channel_id":        strings.TrimSpace(req.ChannelID),
		"seed_prompt":                 strings.TrimSpace(req.Objective),
		"source_network_cycle_id":     firstNonEmpty(metadataString(parentRec.Metadata, "source_network_cycle_id"), metadataString(parentRec.Metadata, "source_maxx_cycle_id")),
		"source_network_request_id":   firstNonEmpty(metadataString(parentRec.Metadata, "source_network_request_id"), metadataString(parentRec.Metadata, "source_maxx_request_id")),
		"source_network_request_kind": firstNonEmpty(metadataString(parentRec.Metadata, "source_network_request_kind"), metadataString(parentRec.Metadata, "source_maxx_request_kind")),
		"source_maxx_cycle_id":        metadataString(parentRec.Metadata, "source_maxx_cycle_id"),
		"source_maxx_request_id":      metadataString(parentRec.Metadata, "source_maxx_request_id"),
		"source_maxx_request_kind":    metadataString(parentRec.Metadata, "source_maxx_request_kind"),
		"source_item_ids":             parentRec.Metadata["source_item_ids"],
		"processor_key":               metadataString(parentRec.Metadata, runMetadataProcessorKey),
		"reconciler_scope":            metadataString(parentRec.Metadata, runMetadataReconcilerScope),
		"selected_style_sources":      selectedStyles,
		"selected_style_rationale":    styleRationale,
	}
	if len(sourceEntities) > 0 {
		seedMetaMap["source_entities"] = sourceEntities
	}
	seedMeta, _ := json.Marshal(seedMetaMap)
	seedRev := types.Revision{
		RevisionID: seedRevisionID,
		DocID:      doc.DocID,
		OwnerID:    ownerID,
		AuthorKind: types.AuthorAppAgent,
		AuthorLabel: strings.TrimSpace(firstNonEmpty(
			parentRec.AgentID,
			canonicalAgentProfile(req.CallerProfile),
		)),
		Content:   seedContent,
		Citations: json.RawMessage("[]"),
		Metadata:  seedMeta,
		CreatedAt: now,
	}
	if err := rt.store.CreateRevision(ctx, seedRev); err != nil {
		return types.Document{}, false, "", fmt.Errorf("create vtext seed revision: %w", err)
	}
	doc.CurrentRevisionID = seedRevisionID
	rt.emitVTextDocumentRevisionEventForRun(ctx, parentRec, seedRev)
	return doc, true, seedRevisionID, nil
}

func coagentVTextTitle(req coagentVTextRouteRequest) string {
	if title := strings.TrimSpace(req.Title); title != "" {
		return title
	}
	text := strings.TrimSpace(req.Objective)
	for _, prefix := range []string{
		"Write a VText article covering ",
		"Write a VText article ",
		"Create VText for ",
		"draft a correction/update VText from ",
		"write a source-grounded article from ",
	} {
		if rest := strings.TrimSpace(strings.TrimPrefix(text, prefix)); rest != text && rest != "" {
			text = rest
			break
		}
	}
	text = strings.Trim(text, " .\n\t")
	if text == "" {
		text = "Global Wire article"
	}
	if len(text) > 96 {
		text = strings.TrimSpace(text[:96])
	}
	if !strings.HasSuffix(strings.ToLower(text), ".vtext") {
		text += ".vtext"
	}
	return text
}

func coagentVTextSeedContent(parentRec *types.RunRecord, req coagentVTextRouteRequest, sourceEntities []vtextSourceEntity) string {
	var b strings.Builder
	b.WriteString("# Source brief: ")
	b.WriteString(strings.TrimSuffix(coagentVTextTitle(req), ".vtext"))
	b.WriteString("\n\n")
	if initial := strings.TrimSpace(req.InitialContent); initial != "" {
		b.WriteString(initial)
	} else {
		b.WriteString(strings.TrimSpace(req.Objective))
	}
	seed := b.String()
	if seed[len(seed)-1] != '\n' {
		b.WriteString("\n")
	}
	selectedStyles, styleRationale := coagentVTextSelectedStyles(req)
	b.WriteString("\n## Style.vtext Source\n\n")
	for _, style := range selectedStyles {
		b.WriteString("- ")
		b.WriteString(style.Title)
		b.WriteString(" (")
		b.WriteString(firstNonEmpty(style.DocID, style.SourcePath, style.ID))
		b.WriteString(")\n")
	}
	b.WriteString("\nSelection rationale: ")
	b.WriteString(styleRationale)
	b.WriteString("\n")
	if len(sourceEntities) > 0 {
		b.WriteString("\n## Source Handles\n\n")
		for _, entity := range sourceEntities {
			if strings.TrimSpace(entity.EntityID) == "" {
				continue
			}
			b.WriteString("- [")
			b.WriteString(firstNonEmpty(entity.Label, entity.Kind, "Source"))
			b.WriteString("](source:")
			b.WriteString(entity.EntityID)
			b.WriteString(")\n")
		}
	}
	return b.String()
}

func buildCoagentVTextRevisionPrompt(parentRec *types.RunRecord, req coagentVTextRouteRequest, doc types.Document, created bool, sourceEntities []vtextSourceEntity) string {
	var b strings.Builder
	selectedStyles, styleRationale := coagentVTextSelectedStyles(req)
	if created {
		b.WriteString("Write the first publication-quality article revision for this VText document.")
	} else {
		b.WriteString("Revise this existing VText document with the processor/reconciler update.")
	}
	b.WriteString("\n\nArticle request:\n")
	b.WriteString(strings.TrimSpace(req.Objective))
	if initial := strings.TrimSpace(req.InitialContent); initial != "" {
		b.WriteString("\n\nProcessor/reconciler brief to preserve as source context:\n")
		b.WriteString(initial)
	}
	b.WriteString("\n\nSelected Style.vtext source context:\n")
	for _, style := range selectedStyles {
		b.WriteString("- ")
		b.WriteString(style.Title)
		b.WriteString(" [")
		b.WriteString(style.ID)
		b.WriteString("] source=")
		b.WriteString(firstNonEmpty(style.DocID, style.SourcePath, style.ID))
		if style.Summary != "" {
			b.WriteString(" — ")
			b.WriteString(style.Summary)
		}
		b.WriteString("\n")
	}
	b.WriteString("Selection rationale: ")
	b.WriteString(styleRationale)
	if len(sourceEntities) > 0 {
		requiredSourceRefs := min(3, len(sourceEntities))
		b.WriteString("\n\nNative source handles available to this article revision:\n")
		for _, entity := range sourceEntities {
			if strings.TrimSpace(entity.EntityID) == "" {
				continue
			}
			b.WriteString("- [")
			b.WriteString(firstNonEmpty(entity.Label, entity.Kind, "Source"))
			b.WriteString("](source:")
			b.WriteString(entity.EntityID)
			b.WriteString(")")
			if entity.Target.ContentID != "" {
				b.WriteString(" content_id=")
				b.WriteString(entity.Target.ContentID)
			}
			if entity.Target.ItemID != "" {
				b.WriteString(" source_service_item=")
				b.WriteString(entity.Target.ItemID)
			}
			b.WriteString("\n")
		}
		b.WriteString("\nArticle citation requirement: cite at least ")
		b.WriteString(strconv.Itoa(requiredSourceRefs))
		b.WriteString(" distinct listed native source handle")
		if requiredSourceRefs != 1 {
			b.WriteString("s")
		}
		b.WriteString(" in reader-facing article prose using [label](source:entity_id). Citations that appear only in Source Handles, Source Manifest, source inventories, notes, or metadata sections do not satisfy this requirement.")
	}
	b.WriteString("\n\nHard requirements:")
	b.WriteString("\n- Use edit_vtext to write the canonical VText revision; do not leave the article only in the run result.")
	b.WriteString("\n- The current document head after this run must be a publishable article or correction/update draft, not a Source Brief, Working Revision, Evidence Gathering note, outline, or placeholder.")
	b.WriteString("\n- Treat processor/reconciler notes as source context, not final prose.")
	b.WriteString("\n- Preserve source handles and use native VText source refs like [label](source:entity_id) inside article prose; do not replace them with a plain source manifest or isolate them in an inventory section.")
	b.WriteString("\n- Use the selected Style.vtext sources to shape voice, structure, and editorial judgment; do not name the selected Style.vtext or style rationale in reader-facing prose unless it is genuinely part of the story.")
	b.WriteString("\n- Transclude related VTexts where editorially useful; do not render bare related-VText ID lists as article content.")
	b.WriteString("\n- Keep Style.vtext selection, source inventories, provenance notes, revision state, and handoff mechanics out of the visible article body unless they are editorially necessary. They belong in revision metadata and native source/transclusion affordances, not as reader-facing sections.")
	b.WriteString("\n- Do not include placeholder metadata or publication labels such as \"Published: [Date TBD]\", \"Breaking News |\", \"Date:\", \"By Choir News\", \"Source:\", \"Story id\", \"State\", \"Source Handles\", \"Source Manifest\", \"Style.vtext Source\", horizontal-rule separators, or tool/process notes in the article body.")
	b.WriteString("\n- If evidence is insufficient, write the best honest publishable draft with uncertainty and request researcher follow-up rather than inventing facts.")
	b.WriteString("\n\nDocument: ")
	b.WriteString(doc.DocID)
	b.WriteString(" / ")
	b.WriteString(doc.Title)
	b.WriteString("\nParent loop: ")
	b.WriteString(strings.TrimSpace(parentRec.RunID))
	if cycleID := firstNonEmpty(metadataString(parentRec.Metadata, "source_network_cycle_id"), metadataString(parentRec.Metadata, "source_maxx_cycle_id")); cycleID != "" {
		b.WriteString("\nSource network cycle: ")
		b.WriteString(cycleID)
	}
	return b.String()
}

func (rt *Runtime) coagentVTextSourceEntities(ctx context.Context, parentRec *types.RunRecord, req coagentVTextRouteRequest) []vtextSourceEntity {
	if parentRec == nil {
		return nil
	}
	entities := decodeVTextSourceEntities(parentRec.Metadata["source_entities"])
	seenText := strings.Join([]string{strings.TrimSpace(req.Objective), strings.TrimSpace(req.InitialContent)}, "\n")
	for _, itemID := range sourceServiceItemIDsFromText(seenText) {
		entities, _ = mergeVTextSourceEntities(entities, []vtextSourceEntity{sourceServiceItemRefToSourceEntity(itemID, seenText)})
	}

	ownerID := strings.TrimSpace(parentRec.OwnerID)
	if rt == nil || rt.store == nil || ownerID == "" {
		return entities
	}
	for _, contentID := range coagentVTextSourceContentIDs(parentRec, seenText) {
		item, err := rt.Store().GetContentItem(ctx, ownerID, contentID)
		if err != nil {
			continue
		}
		entity := contentItemRefToSourceEntity(item, seenText)
		entity.Provenance.CreatedBy = firstNonEmpty(canonicalAgentProfile(req.CallerProfile), entity.Provenance.CreatedBy)
		entities, _ = mergeVTextSourceEntities(entities, []vtextSourceEntity{entity})
	}
	return entities
}

func coagentVTextSourceContentIDs(parentRec *types.RunRecord, text string) []string {
	seen := map[string]bool{}
	out := []string{}
	add := func(raw string) {
		raw = strings.Trim(strings.TrimSpace(raw), `"'`)
		if raw == "" || seen[raw] {
			return
		}
		seen[raw] = true
		out = append(out, raw)
	}
	for _, key := range []string{"source_item_ids", "source_content_ids", "source_content_id", "content_id"} {
		for _, value := range metadataStringSlice(parentRec.Metadata[key]) {
			add(value)
		}
	}
	for _, value := range contentItemIDsFromWorkerMessage(text) {
		add(value)
	}
	return out
}

func metadataStringSlice(value any) []string {
	switch typed := value.(type) {
	case nil:
		return nil
	case string:
		return []string{typed}
	case []string:
		return typed
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if s := strings.TrimSpace(fmt.Sprint(item)); s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		data, err := json.Marshal(typed)
		if err != nil {
			return nil
		}
		var out []string
		if err := json.Unmarshal(data, &out); err == nil {
			return out
		}
		var anyOut []any
		if err := json.Unmarshal(data, &anyOut); err != nil {
			return nil
		}
		return metadataStringSlice(anyOut)
	}
}

func coagentDefaultStyleCatalog() []types.GlobalWireStyleSource {
	return []types.GlobalWireStyleSource{
		{ID: "wire-style", Title: "Style.vtext: Global Wire", Label: "Wire", SourcePath: "styles/global-wire.style.vtext"},
		{ID: "claim-audit-style", Title: "Style.vtext: Claim Audit", Label: "Audit", SourcePath: "styles/claim-audit.style.vtext"},
		{ID: "market-brief-style", Title: "Style.vtext: Market Brief", Label: "Market", SourcePath: "styles/market-brief.style.vtext"},
	}
}

func coagentVTextSelectedStyles(req coagentVTextRouteRequest) ([]types.GlobalWireStyleSource, string) {
	styles := coagentDefaultStyleCatalog()
	byID := map[string]types.GlobalWireStyleSource{}
	for _, style := range styles {
		byID[style.ID] = style
	}
	text := strings.ToLower(strings.Join([]string{req.Title, req.Objective, req.InitialContent}, "\n"))
	addSummary := func(style types.GlobalWireStyleSource, summary string) types.GlobalWireStyleSource {
		style.Summary = summary
		return style
	}
	marketTerms := []string{"fed", "fomc", "treasury", "market", "inflation", "rates", "yield", "bank", "earnings", "oil", "energy", "currency", "dollar", "stocks", "bond"}
	auditTerms := []string{"correction", "correcting", "misinformation", "contradiction", "claim", "denied", "disputed", "uncertain", "unverified", "alleged", "propaganda", "false", "audit"}
	hasMarket := textContainsAny(text, marketTerms)
	hasAudit := textContainsAny(text, auditTerms)
	wire := addSummary(byID["wire-style"], "Fast, readable global-wire treatment: direct headline, clear nut graf, source-rich context, visible uncertainty.")
	audit := addSummary(byID["claim-audit-style"], "Claim-audit treatment: foreground what is verified, what is disputed, who says so, and what evidence would change the story.")
	market := addSummary(byID["market-brief-style"], "Market-brief treatment: explain price, policy, institutional, and second-order effects without hiding uncertainty.")
	switch {
	case hasMarket && hasAudit:
		return []types.GlobalWireStyleSource{market, audit}, "The brief mixes market/policy mechanics with contested or uncertain claims, so use Market Brief as primary style with Claim Audit as a secondary constraint."
	case hasMarket:
		return []types.GlobalWireStyleSource{market}, "The brief centers market, policy, inflation, rates, or financial transmission, so Market Brief is the fitting Style.vtext."
	case hasAudit:
		return []types.GlobalWireStyleSource{audit}, "The brief centers correction, contradiction, disputed claims, or verification risk, so Claim Audit is the fitting Style.vtext."
	default:
		return []types.GlobalWireStyleSource{wire}, "The brief is a general news article request, so Global Wire is the fitting Style.vtext."
	}
}

func textContainsAny(text string, terms []string) bool {
	for _, term := range terms {
		if strings.Contains(text, term) {
			return true
		}
	}
	return false
}

func normalizeDelegateTargetValue(raw string, allowedTargets []string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	direct := canonicalAgentProfile(raw)
	if delegateTargetAllowed(direct, allowedTargets) {
		return direct
	}
	if !looksLikeNoisyToolArg(raw) {
		return direct
	}
	seen := map[string]bool{}
	var match string
	for _, token := range strings.FieldsFunc(raw, func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '-' || r == '_')
	}) {
		candidate := canonicalAgentProfile(token)
		if !delegateTargetAllowed(candidate, allowedTargets) || seen[candidate] {
			continue
		}
		seen[candidate] = true
		if match != "" {
			return direct
		}
		match = candidate
	}
	if match != "" {
		return match
	}
	return direct
}

func delegateTargetAllowed(candidate string, allowedTargets []string) bool {
	candidate = canonicalAgentProfile(candidate)
	for _, allowed := range allowedTargets {
		if candidate == canonicalAgentProfile(allowed) {
			return true
		}
	}
	return false
}

func looksLikeNoisyToolArg(raw string) bool {
	return strings.ContainsAny(raw, "<>{}[]()/\\\n\r\t")
}

func normalizeVSuperCoSuperSlot(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "implementation", "implementer", "worker", "writer", "builder":
		return "implementation"
	case "verifier", "verification", "reviewer", "review", "checker", "tester":
		return "verifier"
	default:
		return ""
	}
}

func canonicalAllowedDelegateTargets(targets []string) []string {
	out := make([]string, 0, len(targets))
	seen := make(map[string]bool, len(targets))
	for _, target := range targets {
		target = canonicalAgentProfile(target)
		if target == "" || seen[target] {
			continue
		}
		seen[target] = true
		out = append(out, target)
	}
	return out
}

func newCastAgentTool(rt *Runtime) Tool {
	type args struct {
		AgentID   string `json:"agent_id"`
		ChannelID string `json:"channel_id,omitempty"`
		From      string `json:"from,omitempty"`
		Role      string `json:"role,omitempty"`
		Content   string `json:"content"`
	}
	return Tool{
		Name:        "cast_agent",
		Description: "Send an addressed asynchronous message to an existing agent without blocking.",
		Parameters: jsonSchemaObject(map[string]any{
			"agent_id":   map[string]any{"type": "string"},
			"channel_id": map[string]any{"type": "string"},
			"from":       map[string]any{"type": "string"},
			"role":       map[string]any{"type": "string"},
			"content":    map[string]any{"type": "string"},
		}, []string{"agent_id", "content"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var in args
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", fmt.Errorf("decode cast_agent args: %w", err)
			}
			targetAgentID := strings.TrimSpace(in.AgentID)
			if targetAgentID == "" {
				return "", fmt.Errorf("agent_id must not be empty")
			}
			target, err := rt.store.GetAgent(ctx, targetAgentID)
			if err != nil {
				return "", fmt.Errorf("cast_agent target lookup: %w", err)
			}
			channelID := strings.TrimSpace(in.ChannelID)
			if channelID == "" {
				channelID = strings.TrimSpace(target.ChannelID)
			}
			if channelID == "" {
				return "", fmt.Errorf("cast_agent target %s has no channel_id", targetAgentID)
			}
			if err := enforceEmailAppagentCastRule(ctx, target); err != nil {
				return "", err
			}
			if err := enforceSkipLevelCastRule(ctx, rt, targetAgentID, nil); err != nil {
				return "", err
			}
			from := strings.TrimSpace(in.From)
			if from == "" {
				from = stringFromToolContext(ctx, toolCtxRunID)
			}
			role := strings.TrimSpace(in.Role)
			if role == "" {
				role = stringFromToolContext(ctx, toolCtxRole)
			}
			cursor, err := rt.ChannelCast(ctx, channelID, targetAgentID, "", from, role, in.Content)
			if err != nil {
				return "", err
			}
			return toolResultJSON(map[string]any{
				"agent_id":   targetAgentID,
				"channel_id": channelID,
				"cursor":     cursor,
				"status":     "cast",
			})
		},
	}
}

func newCastAgentUpdateTool(rt *Runtime) Tool {
	type recipient struct {
		AgentID string `json:"agent_id"`
		RunID   string `json:"loop_id,omitempty"`
	}
	type args struct {
		MessageClass string      `json:"message_class,omitempty"`
		ChannelID    string      `json:"channel_id,omitempty"`
		From         string      `json:"from,omitempty"`
		Role         string      `json:"role,omitempty"`
		Content      string      `json:"content"`
		Recipients   []recipient `json:"recipients"`
	}
	return Tool{
		Name:        "cast_agent_update",
		Description: "Send one typed update to multiple agents with copy-aware delivery. Super-to-co-super directives must include the supervising vsuper in the same recipients list.",
		Parameters: jsonSchemaObject(map[string]any{
			"message_class": map[string]any{"type": "string", "enum": []string{"phase_checkpoint", "evidence_ready", "blocker", "clarification_request", "directive", "cancel", "narrative_revision"}},
			"channel_id":    map[string]any{"type": "string"},
			"from":          map[string]any{"type": "string"},
			"role":          map[string]any{"type": "string"},
			"content":       map[string]any{"type": "string"},
			"recipients": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"agent_id": map[string]any{"type": "string"},
						"loop_id":  map[string]any{"type": "string"},
					},
					"required": []string{"agent_id"},
				},
			},
		}, []string{"content", "recipients"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var in args
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", fmt.Errorf("decode cast_agent_update args: %w", err)
			}
			if len(in.Recipients) == 0 {
				return "", fmt.Errorf("recipients must not be empty")
			}
			copyGroupID := uuid.NewString()
			recipientIDs := make([]string, 0, len(in.Recipients))
			for _, rec := range in.Recipients {
				agentID := strings.TrimSpace(rec.AgentID)
				if agentID == "" {
					return "", fmt.Errorf("recipient agent_id must not be empty")
				}
				recipientIDs = append(recipientIDs, agentID)
			}
			for _, agentID := range recipientIDs {
				if err := enforceSkipLevelCastRule(ctx, rt, agentID, recipientIDs); err != nil {
					return "", err
				}
			}
			from := strings.TrimSpace(in.From)
			if from == "" {
				from = stringFromToolContext(ctx, toolCtxRunID)
			}
			role := strings.TrimSpace(in.Role)
			if role == "" {
				role = stringFromToolContext(ctx, toolCtxRole)
			}
			messageClass := strings.TrimSpace(in.MessageClass)
			if messageClass == "" {
				messageClass = "phase_checkpoint"
			}
			content := fmt.Sprintf("[message_class=%s copy_group_id=%s]\n%s", messageClass, copyGroupID, strings.TrimSpace(in.Content))
			cursors := make(map[string]uint64, len(in.Recipients))
			for _, rec := range in.Recipients {
				targetAgentID := strings.TrimSpace(rec.AgentID)
				target, err := rt.store.GetAgent(ctx, targetAgentID)
				if err != nil {
					return "", fmt.Errorf("cast_agent_update target lookup %s: %w", targetAgentID, err)
				}
				channelID := strings.TrimSpace(in.ChannelID)
				if channelID == "" {
					channelID = strings.TrimSpace(target.ChannelID)
				}
				if channelID == "" {
					return "", fmt.Errorf("cast_agent_update target %s has no channel_id", targetAgentID)
				}
				if err := enforceEmailAppagentCastRule(ctx, target); err != nil {
					return "", err
				}
				cursor, err := rt.ChannelCast(ctx, channelID, targetAgentID, strings.TrimSpace(rec.RunID), from, role, content)
				if err != nil {
					return "", err
				}
				cursors[targetAgentID] = cursor
			}
			return toolResultJSON(map[string]any{
				"status":        "cast",
				"copy_group_id": copyGroupID,
				"message_class": messageClass,
				"recipients":    recipientIDs,
				"cursors":       cursors,
			})
		},
	}
}

func enforceEmailAppagentCastRule(ctx context.Context, target types.AgentRecord) error {
	if canonicalAgentProfile(target.Profile) != AgentProfileEmail {
		return nil
	}
	callerProfile := canonicalAgentProfile(stringFromToolContext(ctx, toolCtxProfile))
	return fmt.Errorf("%s cannot send arbitrary coagent messages to Email appagent %s; use a VText-owned request_email_draft artifact handoff", callerProfile, target.AgentID)
}

func enforceSkipLevelCastRule(ctx context.Context, rt *Runtime, targetAgentID string, copiedAgentIDs []string) error {
	if rt == nil || rt.store == nil {
		return nil
	}
	if stringFromToolContext(ctx, toolCtxProfile) != AgentProfileSuper {
		return nil
	}
	ownerID := stringFromToolContext(ctx, toolCtxOwnerID)
	if ownerID == "" {
		return nil
	}
	target, err := rt.store.GetAgent(ctx, targetAgentID)
	if err != nil {
		return fmt.Errorf("cast target lookup: %w", err)
	}
	if canonicalAgentProfile(target.Profile) != AgentProfileCoSuper {
		return nil
	}
	run, err := rt.store.GetLatestActiveRunByAgent(ctx, ownerID, targetAgentID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("lookup co-super active run: %w", err)
	}
	parentRunID := strings.TrimSpace(run.ParentRunID)
	if parentRunID == "" {
		return nil
	}
	parent, err := rt.store.GetRun(ctx, parentRunID)
	if err != nil {
		return fmt.Errorf("lookup co-super supervisor run: %w", err)
	}
	if agentProfileForRun(&parent) != AgentProfileVSuper {
		return nil
	}
	supervisorAgentID := agentIDForRun(&parent)
	for _, copied := range copiedAgentIDs {
		if strings.TrimSpace(copied) == supervisorAgentID {
			return nil
		}
	}
	return fmt.Errorf("private skip-level directive rejected: super -> co-super messages must copy supervising vsuper %s in the same cast_agent_update recipients", supervisorAgentID)
}

func newWaitAgentTool(rt *Runtime) Tool {
	type args struct {
		AgentID     string   `json:"agent_id"`
		ChannelID   string   `json:"channel_id,omitempty"`
		Cursor      uint64   `json:"cursor,omitempty"`
		Roles       []string `json:"roles,omitempty"`
		TimeoutMS   int      `json:"timeout_ms,omitempty"`
		MaxMessages int      `json:"max_messages,omitempty"`
	}
	return Tool{
		Name:        "wait_agent",
		Description: "Block briefly for channel messages from an existing agent. Use after spawn_agent or cast_agent when coordination depends on the child result; pass the cast_agent cursor when waiting for a reply to that cast.",
		Parameters: jsonSchemaObject(map[string]any{
			"agent_id":     map[string]any{"type": "string"},
			"channel_id":   map[string]any{"type": "string"},
			"cursor":       map[string]any{"type": "integer", "minimum": 0, "description": "Channel cursor returned by cast_agent or a previous wait_agent call. Omit or use 0 to inspect existing messages."},
			"roles":        map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Optional message roles to return, for example result, error, status, or verifier."},
			"timeout_ms":   map[string]any{"type": "integer", "minimum": 1, "description": "Bounded wait duration. Defaults to 30000ms and is capped at 120000ms."},
			"max_messages": map[string]any{"type": "integer", "minimum": 1, "description": "Maximum matching messages to return. Defaults to 10 and is capped at 25."},
		}, []string{"agent_id"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var in args
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", fmt.Errorf("decode wait_agent args: %w", err)
			}
			ownerID := stringFromToolContext(ctx, toolCtxOwnerID)
			if ownerID == "" {
				return "", fmt.Errorf("wait_agent missing owner context")
			}
			targetAgentID := strings.TrimSpace(in.AgentID)
			if targetAgentID == "" {
				return "", fmt.Errorf("agent_id must not be empty")
			}
			target, err := rt.store.GetAgent(ctx, targetAgentID)
			if err != nil {
				return "", fmt.Errorf("wait_agent target lookup: %w", err)
			}
			if strings.TrimSpace(target.OwnerID) != "" && strings.TrimSpace(target.OwnerID) != ownerID {
				return "", fmt.Errorf("wait_agent target %s is not owned by caller", targetAgentID)
			}
			channelID := strings.TrimSpace(in.ChannelID)
			if channelID == "" {
				channelID = strings.TrimSpace(target.ChannelID)
			}
			if channelID == "" {
				channelID = stringFromToolContext(ctx, toolCtxChannelID)
			}
			if channelID == "" {
				return "", fmt.Errorf("wait_agent target %s has no channel_id", targetAgentID)
			}
			timeout := waitAgentTimeout(in.TimeoutMS)
			maxMessages := waitAgentMaxMessages(in.MaxMessages)
			roles := waitAgentRoleSet(in.Roles)

			targetRuns, latestRun := waitAgentTargetRuns(ctx, rt, ownerID, channelID, targetAgentID)
			cursor := in.Cursor
			if matched, nextCursor, err := waitAgentReadMatching(rt, channelID, cursor, targetAgentID, targetRuns, roles, maxMessages); err != nil {
				return "", err
			} else if len(matched) > 0 {
				return waitAgentResultJSON("messages", targetAgentID, channelID, nextCursor, matched, latestRun, targetRuns, maxMessages)
			} else {
				cursor = nextCursor
			}
			if latestRun != nil && latestRun.State.Terminal() {
				return waitAgentResultJSON("target_terminal_without_new_message", targetAgentID, channelID, cursor, nil, latestRun, targetRuns, maxMessages)
			}

			waitCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			for {
				msgs, nextCursor, err := rt.ChannelWait(waitCtx, channelID, cursor)
				if err != nil {
					targetRuns, latestRun = waitAgentTargetRuns(ctx, rt, ownerID, channelID, targetAgentID)
					if waitCtx.Err() != nil {
						status := "timeout"
						if latestRun != nil && latestRun.State.Terminal() {
							status = "target_terminal_without_matching_message"
						}
						return waitAgentResultJSON(status, targetAgentID, channelID, cursor, nil, latestRun, targetRuns, maxMessages)
					}
					return "", err
				}
				cursor = nextCursor
				targetRuns, latestRun = waitAgentTargetRuns(ctx, rt, ownerID, channelID, targetAgentID)
				matched := waitAgentFilterMessages(msgs, targetAgentID, targetRuns, roles, maxMessages)
				if len(matched) > 0 {
					return waitAgentResultJSON("messages", targetAgentID, channelID, cursor, matched, latestRun, targetRuns, maxMessages)
				}
				if latestRun != nil && latestRun.State.Terminal() {
					return waitAgentResultJSON("target_terminal_without_matching_message", targetAgentID, channelID, cursor, nil, latestRun, targetRuns, maxMessages)
				}
			}
		},
	}
}

func waitAgentTimeout(timeoutMS int) time.Duration {
	if timeoutMS <= 0 {
		return 30 * time.Second
	}
	timeout := time.Duration(timeoutMS) * time.Millisecond
	if timeout > 120*time.Second {
		return 120 * time.Second
	}
	return timeout
}

func waitAgentMaxMessages(maxMessages int) int {
	switch {
	case maxMessages <= 0:
		return 10
	case maxMessages > 25:
		return 25
	default:
		return maxMessages
	}
}

func waitAgentRoleSet(roles []string) map[string]bool {
	if len(roles) == 0 {
		return nil
	}
	out := make(map[string]bool, len(roles))
	for _, role := range roles {
		if role = strings.TrimSpace(role); role != "" {
			out[role] = true
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func waitAgentTargetRuns(ctx context.Context, rt *Runtime, ownerID, channelID, targetAgentID string) ([]types.RunRecord, *types.RunRecord) {
	runs, err := rt.store.ListRunsByChannel(ctx, ownerID, channelID, 200)
	if err != nil {
		return nil, nil
	}
	matches := make([]types.RunRecord, 0, 4)
	for _, run := range runs {
		if strings.TrimSpace(run.AgentID) != targetAgentID {
			continue
		}
		matches = append(matches, run)
	}
	if len(matches) == 0 {
		return matches, nil
	}
	latest := matches[0]
	return matches, &latest
}

func waitAgentReadMatching(rt *Runtime, channelID string, cursor uint64, targetAgentID string, targetRuns []types.RunRecord, roles map[string]bool, maxMessages int) ([]ChannelMessage, uint64, error) {
	msgs, nextCursor, err := rt.ChannelRead(channelID, cursor)
	if err != nil {
		return nil, cursor, err
	}
	return waitAgentFilterMessages(msgs, targetAgentID, targetRuns, roles, maxMessages), nextCursor, nil
}

func waitAgentFilterMessages(msgs []ChannelMessage, targetAgentID string, targetRuns []types.RunRecord, roles map[string]bool, maxMessages int) []ChannelMessage {
	runIDs := make(map[string]bool, len(targetRuns))
	for _, run := range targetRuns {
		if runID := strings.TrimSpace(run.RunID); runID != "" {
			runIDs[runID] = true
		}
	}
	out := make([]ChannelMessage, 0, len(msgs))
	for _, msg := range msgs {
		if len(roles) > 0 && !roles[strings.TrimSpace(msg.Role)] {
			continue
		}
		fromAgentID := strings.TrimSpace(msg.FromAgentID)
		fromRunID := strings.TrimSpace(msg.FromRunID)
		from := strings.TrimSpace(msg.From)
		if fromAgentID != targetAgentID && from != targetAgentID && !runIDs[fromRunID] && !runIDs[from] {
			continue
		}
		out = append(out, msg)
		if len(out) >= maxMessages {
			break
		}
	}
	return out
}

func waitAgentResultJSON(status, targetAgentID, channelID string, cursor uint64, messages []ChannelMessage, latestRun *types.RunRecord, targetRuns []types.RunRecord, maxMessages int) (string, error) {
	result := map[string]any{
		"status":     status,
		"agent_id":   targetAgentID,
		"channel_id": channelID,
		"cursor":     cursor,
		"messages":   waitAgentMessageSummaries(messages),
	}
	if latestRun != nil {
		result["latest_target_run"] = waitAgentRunSummary(*latestRun)
	}
	if len(targetRuns) > 0 {
		limit := maxMessages
		if limit > len(targetRuns) {
			limit = len(targetRuns)
		}
		summaries := make([]map[string]any, 0, limit)
		for _, run := range targetRuns[:limit] {
			summaries = append(summaries, waitAgentRunSummary(run))
		}
		result["target_runs"] = summaries
	}
	return toolResultJSON(result)
}

func waitAgentMessageSummaries(messages []ChannelMessage) []map[string]any {
	out := make([]map[string]any, 0, len(messages))
	for _, msg := range messages {
		out = append(out, map[string]any{
			"seq":           msg.Seq,
			"from":          msg.From,
			"from_agent_id": msg.FromAgentID,
			"from_loop_id":  msg.FromRunID,
			"to_agent_id":   msg.ToAgentID,
			"role":          msg.Role,
			"content":       truncateWaitAgentText(msg.Content, 8000),
			"timestamp":     msg.Timestamp.UTC().Format(time.RFC3339Nano),
		})
	}
	return out
}

func waitAgentRunSummary(run types.RunRecord) map[string]any {
	summary := map[string]any{
		"loop_id":    run.RunID,
		"agent_id":   run.AgentID,
		"profile":    run.AgentProfile,
		"role":       run.AgentRole,
		"state":      run.State,
		"channel_id": run.ChannelID,
	}
	if run.Result != "" {
		summary["result"] = truncateWaitAgentText(run.Result, 4000)
	}
	if run.Error != "" {
		summary["error"] = truncateWaitAgentText(run.Error, 2000)
	}
	return summary
}

func truncateWaitAgentText(value string, limit int) string {
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return value[:limit] + fmt.Sprintf("\n\n[truncated %d bytes, showing first %d bytes]", len(value), limit)
}

func newCancelAgentTool(rt *Runtime) Tool {
	type args struct {
		AgentID string `json:"agent_id"`
	}
	return Tool{
		Name:        "cancel_agent",
		Description: "Cancel the latest active loop for an existing agent by agent id.",
		Parameters: jsonSchemaObject(map[string]any{
			"agent_id": map[string]any{"type": "string"},
		}, []string{"agent_id"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var in args
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", fmt.Errorf("decode cancel_agent args: %w", err)
			}
			ownerID := stringFromToolContext(ctx, toolCtxOwnerID)
			if ownerID == "" {
				return "", fmt.Errorf("cancel_agent missing owner context")
			}
			target, err := rt.store.GetLatestActiveRunByAgent(ctx, ownerID, strings.TrimSpace(in.AgentID))
			if err != nil {
				if err == store.ErrNotFound {
					return "", fmt.Errorf("agent not found: %s", in.AgentID)
				}
				return "", fmt.Errorf("lookup active agent run: %w", err)
			}
			if stringFromToolContext(ctx, toolCtxProfile) == AgentProfileVSuper &&
				strings.TrimSpace(target.ParentRunID) == stringFromToolContext(ctx, toolCtxRunID) {
				eventsForRun, err := rt.store.ListEvents(ctx, target.RunID, 1000)
				if err != nil {
					return "", fmt.Errorf("check child export evidence before cancel: %w", err)
				}
				if hasSuccessfulToolResult(eventsForRun, "publish_app_change_package") {
					return toolResultJSON(map[string]any{
						"agent_id": in.AgentID,
						"loop_id":  target.RunID,
						"status":   "not_cancelled",
						"reason":   "child already produced publish_app_change_package evidence; incorporate the child package instead of cancelling it",
					})
				}
			}
			if err := rt.CancelRun(ctx, target.RunID, ownerID); err != nil {
				return "", err
			}
			return toolResultJSON(map[string]any{
				"agent_id": in.AgentID,
				"loop_id":  target.RunID,
				"status":   "cancelled",
			})
		},
	}
}
