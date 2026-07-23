package textureowner

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

type coagentTextureRouteRequest struct {
	CallerProfile  string
	Role           string
	Profile        string
	Objective      string
	Title          string
	ChannelID      string
	InitialContent string
	SourceItemIDs  []string
}

type coagentTextureRouteDecision struct {
	DocID           string
	Title           string
	SeedRevisionID  string
	RevisionRunID   string
	State           types.RunState
	CreatedDocument bool
}

func (h *Handler) ensureCoagentTextureRevisionRoute(ctx context.Context, parentRec *types.RunRecord, req coagentTextureRouteRequest) (coagentTextureRouteDecision, error) {
	if h == nil || h.Store == nil {
		return coagentTextureRouteDecision{}, fmt.Errorf("runtime store unavailable")
	}
	if parentRec == nil {
		return coagentTextureRouteDecision{}, fmt.Errorf("texture route requires a parent run")
	}
	callerProfile := agentprofile.Canonical(req.CallerProfile)
	if callerProfile != agentprofile.Processor && callerProfile != agentprofile.Reconciler {
		return coagentTextureRouteDecision{}, fmt.Errorf("texture route requires processor or reconciler caller")
	}
	ownerID := strings.TrimSpace(parentRec.OwnerID)
	if ownerID == "" {
		return coagentTextureRouteDecision{}, fmt.Errorf("owner_id is required")
	}

	now := time.Now().UTC()
	if callerProfile == agentprofile.Processor {
		resolvedSourceItemIDs, err := resolveWireProcessorSourceItemIDs(parentRec, req.SourceItemIDs, true)
		if err != nil {
			return coagentTextureRouteDecision{}, err
		}
		req.SourceItemIDs = resolvedSourceItemIDs
	}
	sourceEntities := h.coagentTextureSourceEntities(ctx, parentRec, req)
	doc, created, seedRevisionID, err := h.coagentTextureTargetDocument(ctx, parentRec, req, now, sourceEntities)
	if err != nil {
		return coagentTextureRouteDecision{}, err
	}

	prompt := buildCoagentTextureRevisionPrompt(parentRec, req, doc, created, sourceEntities)
	if callerProfile == agentprofile.Reconciler {
		if existing, found, err := h.existingReconcilerTextureHandoff(ctx, parentRec, doc.DocID); err != nil {
			return coagentTextureRouteDecision{}, err
		} else if found {
			return coagentTextureRouteDecision{
				DocID:           doc.DocID,
				Title:           doc.Title,
				RevisionRunID:   existing.RunID,
				State:           existing.State,
				CreatedDocument: false,
			}, nil
		}
	}
	provenance := map[string]any{
		"input_origin":                   textureInputOriginForCaller(callerProfile),
		"requested_by_run_id":            parentRec.RunID,
		"source_network_cycle_id":        firstNonEmpty(metadataString(parentRec.Metadata, "source_network_cycle_id"), metadataString(parentRec.Metadata, "ingestion_handoff_cycle_id")),
		"source_network_request_id":      firstNonEmpty(metadataString(parentRec.Metadata, "source_network_request_id"), metadataString(parentRec.Metadata, "ingestion_handoff_request_id")),
		"source_network_request_kind":    firstNonEmpty(metadataString(parentRec.Metadata, "source_network_request_kind"), metadataString(parentRec.Metadata, "ingestion_handoff_request_kind")),
		"ingestion_handoff_cycle_id":     metadataString(parentRec.Metadata, "ingestion_handoff_cycle_id"),
		"ingestion_handoff_request_id":   metadataString(parentRec.Metadata, "ingestion_handoff_request_id"),
		"ingestion_handoff_request_kind": metadataString(parentRec.Metadata, "ingestion_handoff_request_kind"),
		"reconciler_scope":               metadataString(parentRec.Metadata, runMetadataReconcilerScope),
	}
	rec, err := h.submitTextureAgentRevisionRun(ctx, doc, ownerID, textureAgentRevisionRequest{
		Intent:           "universal_wire_" + callerProfile + "_article_revision",
		Prompt:           prompt,
		SourceEntities:   sourceEntities,
		RequestedByRunID: parentRec.RunID,
		Provenance:       provenance,
	}, 0)
	if err != nil {
		return coagentTextureRouteDecision{}, fmt.Errorf("start texture article revision: %w", err)
	}

	return coagentTextureRouteDecision{
		DocID:           doc.DocID,
		Title:           doc.Title,
		SeedRevisionID:  seedRevisionID,
		RevisionRunID:   rec.RunID,
		State:           rec.State,
		CreatedDocument: created,
	}, nil
}

func (h *Handler) existingReconcilerTextureHandoff(ctx context.Context, parentRec *types.RunRecord, docID string) (types.RunRecord, bool, error) {
	if h == nil || h.Store == nil || parentRec == nil {
		return types.RunRecord{}, false, nil
	}
	runs, err := h.Core.ListRunsByChannel(ctx, parentRec.OwnerID, strings.TrimSpace(docID), 200)
	if err != nil {
		return types.RunRecord{}, false, fmt.Errorf("list existing reconciler Texture handoffs: %w", err)
	}
	for _, run := range runs {
		if agentprofile.Canonical(agentProfileForRun(&run)) != agentprofile.Texture ||
			strings.TrimSpace(run.RequestedByRunID) != strings.TrimSpace(parentRec.RunID) ||
			metadataStringValue(run.Metadata, "request_intent") != "universal_wire_reconciler_article_revision" {
			continue
		}
		return run, true, nil
	}
	return types.RunRecord{}, false, nil
}

func (h *Handler) coagentTextureTargetDocument(ctx context.Context, parentRec *types.RunRecord, req coagentTextureRouteRequest, now time.Time, sourceEntities []textureSourceEntity) (types.Document, bool, string, error) {
	ownerID := strings.TrimSpace(parentRec.OwnerID)
	channelID := strings.TrimSpace(req.ChannelID)
	if channelID != "" {
		if doc, err := h.getTextureDocument(ctx, ownerID, channelID); err == nil {
			if strings.TrimSpace(doc.TrajectoryID) == "" || strings.TrimSpace(doc.ComputerID) == "" {
				return types.Document{}, false, "", fmt.Errorf("existing Texture document is not bound to durable lifecycle authority")
			}
			if _, agentErr := h.Store.GetAgentByScope(ctx, ownerID, doc.ComputerID, currentTextureAgentID(doc.DocID)); agentErr != nil {
				return types.Document{}, false, "", fmt.Errorf("load bound Texture subject: %w", agentErr)
			}
			return doc, false, "", nil
		} else if err != store.ErrNotFound {
			return types.Document{}, false, "", fmt.Errorf("lookup texture document %s: %w", channelID, err)
		}
	}

	commandSeed := firstNonEmpty(
		metadataString(parentRec.Metadata, "lifecycle_command_id"),
		metadataString(parentRec.Metadata, "source_network_request_id"),
		metadataString(parentRec.Metadata, "ingestion_handoff_request_id"),
	)
	computerID := strings.TrimSpace(parentRec.SandboxID)
	if commandSeed == "" || computerID == "" {
		return types.Document{}, false, "", fmt.Errorf("create Texture lifecycle: durable command and computer identity are required")
	}
	lifecycleKey := "choir:texture:source:" + commandSeed
	docID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(lifecycleKey+":document")).String()
	trajectoryID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(lifecycleKey+":trajectory")).String()
	seedRevisionID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(lifecycleKey+":revision:v0")).String()
	workItemID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(lifecycleKey+":work:initial")).String()
	title := coagentTextureTitle(req)
	doc := types.Document{
		DocID: docID, OwnerID: ownerID, ComputerID: computerID, TrajectoryID: trajectoryID,
		Title: title, CreatedAt: now, UpdatedAt: now,
	}

	seedContent := coagentTextureSeedContent(parentRec, req, sourceEntities)
	bodyDoc, sourceEntitiesJSON, projectedContent, err := markdownLineageStructuredRevision(doc.DocID, seedRevisionID, seedContent, sourceEntities, nil)
	if err != nil {
		return types.Document{}, false, "", fmt.Errorf("create texture seed body_doc: %w", err)
	}
	selectedStyles, styleRationale := coagentTextureSelectedStyles(req)
	seedMetaMap := map[string]any{
		"source":                         "coagent_texture_seed",
		"artifact_kind":                  "source_brief",
		"revision_role":                  textureRevisionRoleInput,
		"input_origin":                   textureInputOriginForCaller(req.CallerProfile),
		"texture_version_stage":          "pre_article_brief",
		"created_from":                   agentprofile.Canonical(req.CallerProfile),
		"requested_by_run_id":            parentRec.RunID,
		"requested_by_agent_id":          strings.TrimSpace(parentRec.AgentID),
		"requested_by_channel_id":        strings.TrimSpace(parentRec.ChannelID),
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
	seedMeta, _ := json.Marshal(seedMetaMap)
	seedRev := types.Revision{
		RevisionID: seedRevisionID, DocID: doc.DocID, OwnerID: ownerID,
		ComputerID: computerID, TrajectoryID: trajectoryID,
		AuthorKind:  types.AuthorAppAgent,
		AuthorLabel: strings.TrimSpace(firstNonEmpty(parentRec.AgentID, agentprofile.Canonical(req.CallerProfile))),
		Content:     projectedContent, BodyDoc: bodyDoc, SourceEntities: sourceEntitiesJSON,
		Citations: json.RawMessage("[]"), Metadata: seedMeta, CreatedAt: now,
	}
	agentID := currentTextureAgentID(doc.DocID)
	start := types.StartLifecycleRequest{
		OwnerID: ownerID, ComputerID: computerID, CommandID: lifecycleKey,
		TrajectoryID: trajectoryID, Kind: types.TrajectoryKindTask,
		SubjectRefs:    map[string]string{"artifact": "texture://documents/" + doc.DocID},
		SettlementRule: types.SettlementRule{Version: types.LifecycleReducerVersion, RequireNoOpenWorkItems: true, RequiredSubjectRefs: []string{"artifact"}},
		InitialWork: types.WorkItemRecord{
			WorkItemID: workItemID, Objective: firstNonEmpty(strings.TrimSpace(req.Objective), "Produce the requested Texture artifact."),
			AssignedAgentID: agentID, AuthorityProfile: agentprofile.Texture,
		},
		InitialDocument: doc, InitialRevision: seedRev,
		Agent: types.AgentRecord{
			AgentID: agentID, OwnerID: ownerID, ComputerID: computerID, SandboxID: computerID,
			Profile: agentprofile.Texture, Role: agentprofile.Texture, ChannelID: doc.DocID,
			CreatedAt: now, UpdatedAt: now,
		},
	}
	start.StartRequestDigest, _ = store.ComputeStartLifecycleRequestDigest(start)
	started, err := h.Store.StartLifecycle(ctx, start)
	if err != nil {
		return types.Document{}, false, "", fmt.Errorf("start Texture source lifecycle: %w", err)
	}
	h.emitTextureDocumentRevisionEventForRun(ctx, parentRec, *started.Revision)
	return *started.Document, true, started.Revision.RevisionID, nil
}

func coagentTextureTitle(req coagentTextureRouteRequest) string {
	if title := strings.TrimSpace(req.Title); title != "" {
		return title
	}
	text := strings.TrimSpace(req.Objective)
	for _, prefix := range []string{
		"Write a Texture article covering ",
		"Write a Texture article ",
		"Create Texture for ",
		"draft a correction/update Texture from ",
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
	if !strings.HasSuffix(strings.ToLower(text), ".texture") {
		text += ".texture"
	}
	return text
}

func coagentTextureSeedContent(parentRec *types.RunRecord, req coagentTextureRouteRequest, sourceEntities []textureSourceEntity) string {
	var b strings.Builder
	b.WriteString("# Source brief: ")
	b.WriteString(strings.TrimSuffix(coagentTextureTitle(req), ".texture"))
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
	_ = parentRec
	_ = sourceEntities
	return b.String()
}

func sourceEntityExcerptText(entity textureSourceEntity) string {
	if excerpt := strings.TrimSpace(metadataString(entity.ReaderSnapshot, "excerpt_text")); excerpt != "" {
		return excerpt
	}
	if text := strings.TrimSpace(metadataString(entity.ReaderSnapshot, "text_content")); text != "" {
		return truncateRunes(text, 2000)
	}
	for _, sel := range entity.Selectors {
		if quote := strings.TrimSpace(sel.TextQuote); quote != "" {
			return quote
		}
	}
	return ""
}

func buildCoagentTextureRevisionPrompt(parentRec *types.RunRecord, req coagentTextureRouteRequest, doc types.Document, created bool, sourceEntities []textureSourceEntity) string {
	var b strings.Builder
	selectedStyles, styleRationale := coagentTextureSelectedStyles(req)
	if created {
		b.WriteString("Write the first publication-quality article revision for this Texture document.")
	} else {
		b.WriteString("Revise this existing Texture document with the processor/reconciler update.")
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
		b.WriteString("\n\nNative source entities available to this article revision:\n")
		for _, entity := range sourceEntities {
			if strings.TrimSpace(entity.EntityID) == "" {
				continue
			}
			b.WriteString("- label=")
			b.WriteString(firstNonEmpty(entity.Label, entity.Kind, "Source"))
			b.WriteString(" source_entity_id=")
			b.WriteString(entity.EntityID)
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
		b.WriteString("\nArticle source requirement: reference at least ")
		b.WriteString(strconv.Itoa(requiredSourceRefs))
		b.WriteString(" distinct listed source entit")
		if requiredSourceRefs != 1 {
			b.WriteString("ies")
		} else {
			b.WriteString("y")
		}
		b.WriteString(" in reader-facing article prose through structured patch_texture insert_source_ref operations placed after the supported sentence or clause. Use display_mode expanded_ref only when a block excerpt is editorially required. If a source is immaterial, use mark_source_unused with a short rationale. Every material source must appear as a source_ref in the body; no source is silently ignored.")
		b.WriteString("\n\nSource briefs (excerpt text for synthesis):\n")
		for _, entity := range sourceEntities {
			if strings.TrimSpace(entity.EntityID) == "" {
				continue
			}
			excerpt := sourceEntityExcerptText(entity)
			label := firstNonEmpty(entity.Label, entity.Kind, "Source")
			b.WriteString("\n[")
			b.WriteString(entity.EntityID)
			b.WriteString("] ")
			b.WriteString(label)
			b.WriteString(":\n")
			if excerpt != "" {
				b.WriteString(excerpt)
				if !strings.HasSuffix(excerpt, "\n") {
					b.WriteString("\n")
				}
			} else {
				b.WriteString("(no reader text available for this source)\n")
			}
		}
	}
	b.WriteString("\n\nHard requirements:")
	b.WriteString("\n- Use patch_texture to write the canonical Texture revision; do not leave the article only in the run result.")
	b.WriteString("\n- The current document head after this run must be a publishable article or correction/update draft, not a Source Brief, Working Revision, Evidence Gathering note, outline, or placeholder.")
	b.WriteString("\n- Treat processor/reconciler notes as source context, not final prose.")
	b.WriteString("\n- Preserve source entities and use native Texture source_ref operations inside article prose; do not replace them with ordinary clickable links, markdown source links, Source: lines, a plain source manifest, or an inventory section.")
	b.WriteString("\n- Use the selected Style.texture sources to shape voice, structure, and editorial judgment; do not name the selected Style.texture or style rationale in reader-facing prose unless it is genuinely part of the story.")
	b.WriteString("\n- Transclude related Textures where editorially useful; do not render bare related-Texture ID lists as article content.")
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

func (h *Handler) coagentTextureSourceEntities(ctx context.Context, parentRec *types.RunRecord, req coagentTextureRouteRequest) []textureSourceEntity {
	if parentRec == nil {
		return nil
	}
	entities := decodeAvailableTextureSourceEntities(parentRec.Metadata)

	ownerID := strings.TrimSpace(parentRec.OwnerID)
	if h == nil || h.Store == nil || ownerID == "" {
		return entities
	}
	for _, contentID := range coagentTextureSourceContentIDs(parentRec, req) {
		item, err := h.Store.GetContentItem(ctx, ownerID, contentID)
		if err != nil {
			continue
		}
		entity := contentItemRefToSourceEntity(item)
		entity.Provenance.CreatedBy = firstNonEmpty(agentprofile.Canonical(req.CallerProfile), entity.Provenance.CreatedBy)
		entities, _ = mergeTextureSourceEntities(entities, []textureSourceEntity{entity})
	}
	return entities
}

func coagentTextureSourceContentIDs(parentRec *types.RunRecord, req coagentTextureRouteRequest) []string {
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

func coagentTextureSelectedStyles(req coagentTextureRouteRequest) ([]types.WireStyleSource, string) {
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
