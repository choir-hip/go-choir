package runtime

import (
	"context"
	"encoding/json"
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
		Objective            string   `json:"objective"`
		Role                 string   `json:"role"`
		Profile              string   `json:"profile,omitempty"`
		ChannelID            string   `json:"channel_id,omitempty"`
		Slot                 string   `json:"slot,omitempty"`
		Model                string   `json:"model,omitempty"`
		ModelPolicyOverlayID string   `json:"model_policy_overlay_id,omitempty"`
		Title                string   `json:"title,omitempty"`
		InitialContent       string   `json:"initial_content,omitempty"`
		SourceItemIDs        []string `json:"source_item_ids,omitempty"`
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
			"source_item_ids": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "For role=vtext from processor: the exact source item ids this story handoff covers. Required when the processor request contains multiple source items.",
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
			if (callerProfile == AgentProfileConductor ||
				callerProfile == AgentProfileProcessor ||
				callerProfile == AgentProfileReconciler) && profile == AgentProfileVText {
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
				// ensureCoagentVTextRevisionRoute owns source-item validation
				// for processor callers; do not pre-resolve here.
				sourceItemIDs := trimNonEmpty(in.SourceItemIDs)
				kind := vtextHandoffKindForCaller(callerProfile)
				decision, err := rt.ensureVTextHandoff(ctx, parentRec, vtextHandoffRequest{
					Kind:           kind,
					CallerProfile:  callerProfile,
					Objective:      in.Objective,
					Title:          in.Title,
					ChannelID:      in.ChannelID,
					InitialContent: in.InitialContent,
					SourceItemIDs:  sourceItemIDs,
				})
				if err != nil {
					return "", err
				}
				if kind == vtextHandoffKindUserPrompt {
					conductor := decision.Conductor
					return toolResultJSON(map[string]any{
						"action":                 conductor.Action,
						"app":                    conductor.App,
						"title":                  conductor.Title,
						"seed_prompt":            conductor.SeedPrompt,
						"initial_content":        conductor.InitialContent,
						"create_initial_version": conductor.CreateInitialVersion != nil && *conductor.CreateInitialVersion,
						"agent_id":               currentTextureAgentID(decision.DocID),
						"doc_id":                 decision.DocID,
						"user_revision_id":       decision.UserRevisionID,
						"framing_revision_id":    conductor.FramingRevisionID,
						"initial_revision_id":    conductor.InitialRevisionID,
						"initial_loop_id":        decision.InitialLoopID,
						"loop_id":                decision.RevisionRunID,
						"channel_id":             decision.DocID,
						"role":                   AgentProfileTexture,
						"profile":                AgentProfileTexture,
						"state":                  "open",
						"handoff_kind":           string(kind),
					})
				}
				return toolResultJSON(map[string]any{
					"agent_id":             currentTextureAgentID(decision.DocID),
					"doc_id":               decision.DocID,
					"seed_revision_id":     decision.SeedRevisionID,
					"loop_id":              decision.RevisionRunID,
					"revision_loop_id":     decision.RevisionRunID,
					"channel_id":           decision.DocID,
					"role":                 AgentProfileTexture,
					"profile":              AgentProfileTexture,
					"state":                decision.State,
					"title":                decision.Title,
					"created_document":     decision.CreatedDocument,
					"revised_existing_doc": !decision.CreatedDocument,
					"handoff_kind":         string(kind),
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
	SourceItemIDs  []string
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
	if callerProfile == AgentProfileProcessor {
		resolvedSourceItemIDs, err := resolveWireProcessorSourceItemIDs(parentRec, req.SourceItemIDs, true)
		if err != nil {
			return coagentVTextRouteDecision{}, err
		}
		req.SourceItemIDs = resolvedSourceItemIDs
	}
	sourceEntities := rt.coagentVTextSourceEntities(ctx, parentRec, req)
	doc, created, seedRevisionID, err := rt.coagentVTextTargetDocument(ctx, parentRec, req, now, sourceEntities)
	if err != nil {
		return coagentVTextRouteDecision{}, err
	}
	if callerProfile == AgentProfileProcessor {
		if _, err := rt.beginWireStoryResolutionWorkItem(ctx, parentRec, doc); err != nil {
			return coagentVTextRouteDecision{}, fmt.Errorf("open wire story resolution work item: %w", err)
		}
	}

	if err := rt.store.UpsertAgent(ctx, types.AgentRecord{
		AgentID:   currentTextureAgentID(doc.DocID),
		OwnerID:   ownerID,
		SandboxID: rt.cfg.SandboxID,
		Profile:   AgentProfileTexture,
		Role:      AgentProfileTexture,
		ChannelID: doc.DocID,
		CreatedAt: now,
		UpdatedAt: time.Now().UTC(),
	}); err != nil {
		return coagentVTextRouteDecision{}, fmt.Errorf("persist vtext appagent: %w", err)
	}

	prompt := buildCoagentVTextRevisionPrompt(parentRec, req, doc, created, sourceEntities)
	rec, err := rt.submitVTextAgentRevisionRun(ctx, doc, ownerID, vtextAgentRevisionRequest{
		Intent: "universal_wire_" + callerProfile + "_article_revision",
		Prompt: prompt,
	}, parentRec.RunID, 0)
	if err != nil {
		return coagentVTextRouteDecision{}, fmt.Errorf("start vtext article revision: %w", err)
	}
	if callerProfile == AgentProfileProcessor {
		if _, err := rt.recordWireProcessorDecision(ctx, parentRec, wireProcessorDecisionUpdate{
			Verdict:       wireProcessorDecisionOpenedVText,
			Summary:       "processor opened a VText story route for this request",
			DocID:         doc.DocID,
			RevisionRunID: rec.RunID,
			SourceItemIDs: req.SourceItemIDs,
			Complete:      true,
		}); err != nil {
			return coagentVTextRouteDecision{}, fmt.Errorf("record processor VText route decision: %w", err)
		}
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
		"source":                         "coagent_vtext_seed",
		"artifact_kind":                  "source_brief",
		"revision_role":                  vtextRevisionRoleInput,
		"input_origin":                   vtextInputOriginForCaller(req.CallerProfile),
		"vtext_version_stage":            "pre_article_brief",
		"created_from":                   canonicalAgentProfile(req.CallerProfile),
		"parent_loop_id":                 parentRec.RunID,
		"parent_agent_id":                strings.TrimSpace(parentRec.AgentID),
		"parent_channel_id":              strings.TrimSpace(parentRec.ChannelID),
		"requested_channel_id":           strings.TrimSpace(req.ChannelID),
		"seed_prompt":                    strings.TrimSpace(req.Objective),
		"source_network_cycle_id":        firstNonEmpty(metadataString(parentRec.Metadata, "source_network_cycle_id"), metadataString(parentRec.Metadata, "ingestion_handoff_cycle_id")),
		"source_network_request_id":      firstNonEmpty(metadataString(parentRec.Metadata, "source_network_request_id"), metadataString(parentRec.Metadata, "ingestion_handoff_request_id")),
		"source_network_request_kind":    firstNonEmpty(metadataString(parentRec.Metadata, "source_network_request_kind"), metadataString(parentRec.Metadata, "ingestion_handoff_request_kind")),
		"ingestion_handoff_cycle_id":     metadataString(parentRec.Metadata, "ingestion_handoff_cycle_id"),
		"ingestion_handoff_request_id":   metadataString(parentRec.Metadata, "ingestion_handoff_request_id"),
		"ingestion_handoff_request_kind": metadataString(parentRec.Metadata, "ingestion_handoff_request_kind"),
		"source_item_ids":                firstNonEmptySourceItemIDs(req.SourceItemIDs, metadataStringSlice(parentRec.Metadata["source_item_ids"])),
		"processor_key":                  metadataString(parentRec.Metadata, runMetadataProcessorKey),
		"reconciler_scope":               metadataString(parentRec.Metadata, runMetadataReconcilerScope),
		"selected_style_sources":         selectedStyles,
		"selected_style_rationale":       styleRationale,
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
		text = "Universal Wire article"
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
	b.WriteString("\n## Style.texture Source\n\n")
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
	b.WriteString("\n\nSelected Style.texture source context:\n")
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
	b.WriteString("\n- Use patch_texture to write the canonical Texture revision; do not leave the article only in the run result.")
	b.WriteString("\n- The current document head after this run must be a publishable article or correction/update draft, not a Source Brief, Working Revision, Evidence Gathering note, outline, or placeholder.")
	b.WriteString("\n- Treat processor/reconciler notes as source context, not final prose.")
	b.WriteString("\n- Preserve source handles and use native VText source refs like [label](source:entity_id) inside article prose; do not replace them with a plain source manifest or isolate them in an inventory section.")
	b.WriteString("\n- Use the selected Style.texture sources to shape voice, structure, and editorial judgment; do not name the selected Style.texture or style rationale in reader-facing prose unless it is genuinely part of the story.")
	b.WriteString("\n- Transclude related VTexts where editorially useful; do not render bare related-VText ID lists as article content.")
	b.WriteString("\n- Keep Style.texture selection, source inventories, provenance notes, revision state, and handoff mechanics out of the visible article body unless they are editorially necessary. They belong in revision metadata and native source/transclusion affordances, not as reader-facing sections.")
	b.WriteString("\n- Do not include placeholder metadata or publication labels such as \"Published: [Date TBD]\", \"Breaking News |\", \"Date:\", \"By Choir News\", \"Source:\", \"Story id\", \"State\", \"Source Handles\", \"Source Manifest\", \"Style.texture Source\", horizontal-rule separators, or tool/process notes in the article body.")
	b.WriteString("\n- If evidence is insufficient, write the best honest publishable draft with uncertainty and request researcher follow-up rather than inventing facts.")
	b.WriteString("\n\nDocument: ")
	b.WriteString(doc.DocID)
	b.WriteString(" / ")
	b.WriteString(doc.Title)
	b.WriteString("\nParent loop: ")
	b.WriteString(strings.TrimSpace(parentRec.RunID))
	if cycleID := firstNonEmpty(metadataString(parentRec.Metadata, "source_network_cycle_id"), metadataString(parentRec.Metadata, "ingestion_handoff_cycle_id")); cycleID != "" {
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
	for _, contentID := range coagentVTextSourceContentIDs(parentRec, req, seenText) {
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

func coagentVTextSourceContentIDs(parentRec *types.RunRecord, req coagentVTextRouteRequest, text string) []string {
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
	for _, value := range req.SourceItemIDs {
		add(value)
	}
	keys := []string{"source_item_ids", "source_content_ids", "source_content_id", "content_id"}
	if len(req.SourceItemIDs) == 0 {
		for _, key := range keys {
			for _, value := range metadataStringSlice(parentRec.Metadata[key]) {
				add(value)
			}
		}
	}
	for _, value := range contentItemIDsFromWorkerMessage(text) {
		add(value)
	}
	return out
}

func firstNonEmptySourceItemIDs(values ...[]string) []string {
	for _, value := range values {
		if len(value) == 0 {
			continue
		}
		out := make([]string, 0, len(value))
		for _, item := range value {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			out = append(out, item)
		}
		if len(out) > 0 {
			return out
		}
	}
	return nil
}

func resolveWireProcessorSourceItemIDs(rec *types.RunRecord, requested []string, requireExplicitForMulti bool) ([]string, error) {
	available := wireProcessorSourceItemIDs(rec)
	if len(requested) == 0 {
		switch {
		case len(available) == 0:
			return nil, nil
		case len(available) == 1:
			return append([]string(nil), available...), nil
		case requireExplicitForMulti:
			return nil, fmt.Errorf("source_item_ids required when processor request contains %d source items", len(available))
		default:
			return nil, nil
		}
	}
	if len(available) == 0 {
		return nil, fmt.Errorf("source_item_ids were provided but the processor run has no source_item_ids to bind")
	}
	allowed := make(map[string]bool, len(available))
	for _, itemID := range available {
		allowed[itemID] = true
	}
	seen := make(map[string]bool, len(requested))
	out := make([]string, 0, len(requested))
	for _, itemID := range requested {
		itemID = strings.TrimSpace(itemID)
		if itemID == "" || seen[itemID] {
			continue
		}
		if !allowed[itemID] {
			return nil, fmt.Errorf("source_item_id %s is not part of this processor request", itemID)
		}
		seen[itemID] = true
		out = append(out, itemID)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("source_item_ids must not be empty")
	}
	return out, nil
}

func wireProcessorSourceItemIDs(rec *types.RunRecord) []string {
	if rec == nil {
		return nil
	}
	seen := map[string]bool{}
	out := []string{}
	for _, itemID := range metadataStringSlice(rec.Metadata["source_item_ids"]) {
		itemID = strings.TrimSpace(itemID)
		if itemID == "" || seen[itemID] {
			continue
		}
		seen[itemID] = true
		out = append(out, itemID)
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

func coagentDefaultStyleCatalog() []types.WireStyleSource {
	return []types.WireStyleSource{
		{ID: "wire-style", Title: "Style.texture: Universal Wire", Label: "Wire", SourcePath: "styles/universal-wire.style.texture"},
		{ID: "claim-audit-style", Title: "Style.texture: Claim Audit", Label: "Audit", SourcePath: "styles/claim-audit.style.texture"},
		{ID: "market-brief-style", Title: "Style.texture: Market Brief", Label: "Market", SourcePath: "styles/market-brief.style.texture"},
	}
}

func coagentVTextSelectedStyles(req coagentVTextRouteRequest) ([]types.WireStyleSource, string) {
	styles := coagentDefaultStyleCatalog()
	byID := map[string]types.WireStyleSource{}
	for _, style := range styles {
		byID[style.ID] = style
	}
	text := strings.ToLower(strings.Join([]string{req.Title, req.Objective, req.InitialContent}, "\n"))
	addSummary := func(style types.WireStyleSource, summary string) types.WireStyleSource {
		style.Summary = summary
		return style
	}
	marketTerms := []string{"fed", "fomc", "treasury", "market", "inflation", "rates", "yield", "bank", "earnings", "oil", "energy", "currency", "dollar", "stocks", "bond"}
	auditTerms := []string{"correction", "correcting", "misinformation", "contradiction", "claim", "denied", "disputed", "uncertain", "unverified", "alleged", "propaganda", "false", "audit"}
	hasMarket := textContainsAny(text, marketTerms)
	hasAudit := textContainsAny(text, auditTerms)
	wire := addSummary(byID["wire-style"], "Fast, readable universal-wire treatment: direct headline, clear nut graf, source-rich context, visible uncertainty.")
	audit := addSummary(byID["claim-audit-style"], "Claim-audit treatment: foreground what is verified, what is disputed, who says so, and what evidence would change the story.")
	market := addSummary(byID["market-brief-style"], "Market-brief treatment: explain price, policy, institutional, and second-order effects without hiding uncertainty.")
	switch {
	case hasMarket && hasAudit:
		return []types.WireStyleSource{market, audit}, "The brief mixes market/policy mechanics with contested or uncertain claims, so use Market Brief as primary style with Claim Audit as a secondary constraint."
	case hasMarket:
		return []types.WireStyleSource{market}, "The brief centers market, policy, inflation, rates, or financial transmission, so Market Brief is the fitting Style.texture."
	case hasAudit:
		return []types.WireStyleSource{audit}, "The brief centers correction, contradiction, disputed claims, or verification risk, so Claim Audit is the fitting Style.texture."
	default:
		return []types.WireStyleSource{wire}, "The brief is a general news article request, so Universal Wire is the fitting Style.texture."
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
			agentID := strings.TrimSpace(in.AgentID)
			var target types.RunRecord
			targetFromCallerSlot := false
			if stringFromToolContext(ctx, toolCtxProfile) == AgentProfileVSuper {
				callerTrajectoryID := trajectoryIDForRun(ctxRunRecord(ctx))
				slot, found, err := rt.store.CoSuperSlotByAgentAndTrajectory(ctx, ownerID, callerTrajectoryID, agentID)
				if err != nil {
					return "", fmt.Errorf("lookup co-super slot before cancel: %w", err)
				}
				if !found {
					return "", fmt.Errorf("agent not active in caller trajectory: %s", in.AgentID)
				}
				slotRun, err := rt.store.GetRun(ctx, strings.TrimSpace(slot.RunID))
				if err != nil {
					return "", fmt.Errorf("lookup co-super slot run before cancel: %w", err)
				}
				if !slotRun.State.Active() {
					return "", fmt.Errorf("agent not active in caller trajectory: %s", in.AgentID)
				}
				target = slotRun
				targetFromCallerSlot = true
			}
			if !targetFromCallerSlot {
				if resident, found, err := rt.residentRunByAgent(ctx, ownerID, agentID); err != nil {
					return "", fmt.Errorf("lookup resident agent run: %w", err)
				} else if found {
					target = resident
				} else {
					latest, err := rt.store.GetLatestActiveRunByAgent(ctx, ownerID, agentID)
					if err != nil {
						if err == store.ErrNotFound {
							return "", fmt.Errorf("agent not found: %s", in.AgentID)
						}
						return "", fmt.Errorf("lookup active agent run: %w", err)
					}
					target = latest
				}
			}
			if targetFromCallerSlot {
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
