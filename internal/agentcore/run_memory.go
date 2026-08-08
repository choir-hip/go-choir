package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/yusefmosiah/go-choir/internal/provideriface"

	"github.com/yusefmosiah/go-choir/internal/modelcatalog"
	runtimestore "github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

var (
	// ErrContextOverflowRecoveryFailed marks a context overflow that remained
	// blocked after the runtime compacted durable run memory and retried once.
	ErrContextOverflowRecoveryFailed = errors.New("context overflow recovery failed")

	// ErrRunMemoryCompactionFailed marks a failure in durable memory compaction
	// before a provider call.
	ErrRunMemoryCompactionFailed = errors.New("run memory compaction failed")

	runMemoryCompactionLocks sync.Map
)

type runMemoryManager struct {
	store                     *runtimestore.Store
	rec                       *types.RunRecord
	cfg                       provideriface.Config
	emit                      provideriface.EventEmitFunc
	provider                  provideriface.ToolLoopProvider
	llmConfig                 provideriface.LLMSelection
	promptOverheadTokens      int
	overflowRecoveryAttempted bool
	compactionInProgress      bool
}

func newRunMemoryManager(store *runtimestore.Store, rec *types.RunRecord, cfg provideriface.Config, emit provideriface.EventEmitFunc) *runMemoryManager {
	return &runMemoryManager{
		store: store,
		rec:   rec,
		cfg:   provideriface.NormalizeConfig(cfg),
		emit:  emit,
	}
}

func (m *runMemoryManager) withLLMCompactor(provider provideriface.ToolLoopProvider, llmConfig provideriface.LLMSelection, promptOverheadTokens int) *runMemoryManager {
	m.provider = provider
	m.llmConfig = llmConfig
	m.promptOverheadTokens = promptOverheadTokens
	return m
}

func (m *runMemoryManager) initialize(ctx context.Context, initialMessages []json.RawMessage) ([]json.RawMessage, error) {
	entries, err := m.store.ListRunMemoryEntries(ctx, m.rec.OwnerID, m.rec.RunID)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		if err := m.seedActorMemorySnapshot(ctx); err != nil {
			return nil, err
		}
		for _, msg := range initialMessages {
			if err := m.appendMessage(ctx, runMemoryMessageRole(msg), msg); err != nil {
				return nil, err
			}
		}
	}
	return m.contextMessages(ctx)
}

func (m *runMemoryManager) seedActorMemorySnapshot(ctx context.Context) error {
	if m == nil || m.store == nil || m.rec == nil {
		return nil
	}
	ownerID := strings.TrimSpace(m.rec.OwnerID)
	computerID := strings.TrimSpace(m.rec.SandboxID)
	agentID := strings.TrimSpace(m.rec.AgentID)
	runID := strings.TrimSpace(m.rec.RunID)
	if ownerID == "" || computerID == "" || agentID == "" || runID == "" {
		return nil
	}
	sourceRunID, priorEntries, err := m.store.LatestActorRunMemoryEntries(ctx, ownerID, computerID, agentID, runID)
	if err != nil {
		if errors.Is(err, runtimestore.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("load actor memory snapshot: %w", err)
	}
	if len(priorEntries) == 0 {
		return nil
	}
	checkpoint, tail := latestRunMemoryCheckpointAndTail(priorEntries)
	summary := summarizeRunMemoryMessages(checkpoint, tail, "actor_rewarm")
	if strings.TrimSpace(summary) == "" {
		return nil
	}
	sourceEntryIDs := make([]string, 0, len(priorEntries))
	for _, entry := range priorEntries {
		if id := strings.TrimSpace(entry.EntryID); id != "" {
			sourceEntryIDs = append(sourceEntryIDs, id)
		}
	}
	_, err = m.store.AppendRunMemoryEntry(ctx, types.RunMemoryEntry{
		RunID:        runID,
		OwnerID:      ownerID,
		AgentID:      agentID,
		Kind:         types.RunMemoryEntryCompaction,
		Summary:      "Actor memory snapshot from prior activation " + sourceRunID + "\n\n" + summary,
		TokensBefore: estimateRawMessagesTokens(buildRunMemoryContext(priorEntries)),
		Reason:       "actor_rewarm",
		Details: map[string]any{
			"source_loop_id":    sourceRunID,
			"source_entry_ids":  sourceEntryIDs,
			"checkpoint_status": "deterministic_actor_snapshot",
		},
	})
	if err != nil {
		return fmt.Errorf("append actor memory snapshot: %w", err)
	}
	return nil
}

func latestRunMemoryCheckpointAndTail(entries []types.RunMemoryEntry) (string, []types.RunMemoryEntry) {
	for i := len(entries) - 1; i >= 0; i-- {
		if entries[i].Kind == types.RunMemoryEntryCompaction {
			latest := entries[i]
			tail := make([]types.RunMemoryEntry, 0, len(entries)-i-1)
			if strings.TrimSpace(latest.FirstKeptEntryID) != "" {
				keep := false
				for _, entry := range entries[:i] {
					if entry.EntryID == latest.FirstKeptEntryID {
						keep = true
					}
					if keep && entry.Kind == types.RunMemoryEntryMessage && len(entry.Message) > 0 {
						tail = append(tail, entry)
					}
				}
			}
			for _, entry := range entries[i+1:] {
				if entry.Kind == types.RunMemoryEntryMessage && len(entry.Message) > 0 {
					tail = append(tail, entry)
				}
			}
			return strings.TrimSpace(latest.Summary), tail
		}
	}
	return "", entries
}

func (m *runMemoryManager) hooks() toolregistry.ToolLoopMemoryHooks {
	return toolregistry.ToolLoopMemoryHooks{
		BeforeProviderCall: m.beforeProviderCall,
		AfterAppendMessage: m.afterAppendMessage,
		OnProviderError:    m.onProviderError,
	}
}

func (m *runMemoryManager) beforeProviderCall(ctx context.Context, _ []json.RawMessage) ([]json.RawMessage, error) {
	if _, err := m.compactIfNeeded(ctx, "threshold", false); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRunMemoryCompactionFailed, err)
	}
	return m.contextMessages(ctx)
}

func (m *runMemoryManager) afterAppendMessage(ctx context.Context, role string, msg json.RawMessage) error {
	if strings.TrimSpace(role) == "" {
		role = runMemoryMessageRole(msg)
	}
	return m.appendMessage(ctx, role, msg)
}

func (m *runMemoryManager) onProviderError(ctx context.Context, _ []json.RawMessage, providerErr error) ([]json.RawMessage, bool, error) {
	if !isContextOverflowError(providerErr) {
		return nil, false, nil
	}
	if m.overflowRecoveryAttempted {
		return nil, false, fmt.Errorf("%w: %w", ErrContextOverflowRecoveryFailed, providerErr)
	}
	m.overflowRecoveryAttempted = true

	compacted, err := m.compactIfNeeded(ctx, "context_overflow", true)
	if err != nil {
		return nil, false, fmt.Errorf("%w: %v: %w", ErrContextOverflowRecoveryFailed, err, providerErr)
	}
	if !compacted {
		return nil, false, fmt.Errorf("%w: no compactable run memory: %w", ErrContextOverflowRecoveryFailed, providerErr)
	}
	payload, _ := json.Marshal(map[string]any{
		"reason":  "context_overflow",
		"attempt": 1,
	})
	m.emit(types.EventRunRetry, "run_memory", payload)

	messages, err := m.contextMessages(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("%w: rebuild compacted context: %v: %w", ErrContextOverflowRecoveryFailed, err, providerErr)
	}
	return messages, true, nil
}

func (m *runMemoryManager) appendMessage(ctx context.Context, role string, msg json.RawMessage) error {
	_, err := m.store.AppendRunMemoryEntry(ctx, types.RunMemoryEntry{
		RunID:   m.rec.RunID,
		OwnerID: m.rec.OwnerID,
		AgentID: m.rec.AgentID,
		Kind:    types.RunMemoryEntryMessage,
		Role:    role,
		Message: cloneRawMessage(msg),
	})
	return err
}

func (m *runMemoryManager) contextMessages(ctx context.Context) ([]json.RawMessage, error) {
	entries, err := m.store.ListRunMemoryEntries(ctx, m.rec.OwnerID, m.rec.RunID)
	if err != nil {
		return nil, err
	}
	return buildRunMemoryContext(entries), nil
}

func (m *runMemoryManager) compactIfNeeded(ctx context.Context, reason string, force bool) (bool, error) {
	entries, err := m.store.ListRunMemoryEntries(ctx, m.rec.OwnerID, m.rec.RunID)
	if err != nil {
		return false, err
	}
	if len(entries) == 0 {
		if force && canCheckpointRunEventLedger(reason) {
			return m.compactRunEventLedger(ctx, entries, reason)
		}
		return false, nil
	}
	if !force {
		tokens := m.estimatePromptPressureTokens(entries)
		if tokens <= m.effectiveContextThresholdTokens() {
			return false, nil
		}
		if entries[len(entries)-1].Kind == types.RunMemoryEntryCompaction {
			return false, nil
		}
	}
	if m.compactionInProgress {
		return false, fmt.Errorf("run memory compaction already in progress for run %s", m.rec.RunID)
	}
	lockKey := m.runCompactionLockKey()
	if lockKey != "" {
		if _, loaded := runMemoryCompactionLocks.LoadOrStore(lockKey, true); loaded {
			return false, fmt.Errorf("run memory compaction already in progress for run %s", m.rec.RunID)
		}
		defer runMemoryCompactionLocks.Delete(lockKey)
	}

	keepRecentTokens := m.cfg.RunMemoryKeepRecentTokens
	if force && reason == "context_overflow" && len(runMemoryMessageIndexes(entries)) <= 1 {
		keepRecentTokens = 0
	}
	plan, ok := planRunMemoryCompaction(entries, keepRecentTokens, reason)
	if !ok {
		if force && canCheckpointRunEventLedger(reason) {
			return m.compactRunEventLedger(ctx, entries, reason)
		}
		return false, nil
	}
	m.compactionInProgress = true
	defer func() {
		m.compactionInProgress = false
	}()
	startPayload, _ := json.Marshal(map[string]any{
		"reason":           reason,
		"tokens_before":    plan.TokensBefore,
		"message_count":    plan.CompactedMessages + plan.KeptMessages,
		"threshold_tokens": m.effectiveContextThresholdTokens(),
	})
	m.emit(types.EventRunCompactionStarted, "run_memory", startPayload)

	compaction, err := m.generateLLMCompaction(ctx, plan)
	if err != nil {
		failedPayload, _ := json.Marshal(map[string]any{
			"reason":         reason,
			"tokens_before":  plan.TokensBefore,
			"provider_error": err.Error(),
			"fallback":       "deterministic_emergency",
		})
		m.emit(types.EventRunProgress, "run_memory_compaction_failed", failedPayload)
		compaction = deterministicEmergencyRunMemoryCompaction(plan, err)
	}

	entry, err := m.store.AppendRunMemoryEntry(ctx, types.RunMemoryEntry{
		RunID:            m.rec.RunID,
		OwnerID:          m.rec.OwnerID,
		AgentID:          m.rec.AgentID,
		Kind:             types.RunMemoryEntryCompaction,
		Summary:          compaction.Summary,
		FirstKeptEntryID: plan.FirstKeptEntryID,
		TokensBefore:     plan.TokensBefore,
		Reason:           reason,
		Model:            strings.TrimSpace(m.llmConfig.Model),
		Details: map[string]any{
			"compacted_messages":        plan.CompactedMessages,
			"kept_messages":             plan.KeptMessages,
			"tokens_after":              plan.TokensAfterEstimate,
			"raw_entry_ids":             plan.RawEntryIDs,
			"raw_tool_result_entry_ids": plan.RawToolResultEntryIDs,
			"checkpoint":                compaction.Details,
			"checkpoint_status":         compaction.Status,
			"checkpoint_provider":       compaction.Provider,
			"checkpoint_model":          compaction.Model,
			"threshold_tokens":          m.effectiveContextThresholdTokens(),
			"prompt_overhead_tokens":    m.promptOverheadTokens,
		},
	})
	if err != nil {
		return false, err
	}

	donePayload, _ := json.Marshal(map[string]any{
		"entry_id":                  entry.EntryID,
		"reason":                    reason,
		"tokens_before":             plan.TokensBefore,
		"tokens_after":              plan.TokensAfterEstimate,
		"first_kept_entry_id":       plan.FirstKeptEntryID,
		"compacted_messages":        plan.CompactedMessages,
		"kept_messages":             plan.KeptMessages,
		"raw_entry_ids":             plan.RawEntryIDs,
		"raw_tool_result_entry_ids": plan.RawToolResultEntryIDs,
		"checkpoint_status":         compaction.Status,
		"checkpoint_provider":       compaction.Provider,
		"checkpoint_model":          compaction.Model,
		"threshold_tokens":          m.effectiveContextThresholdTokens(),
	})
	m.emit(types.EventRunCompactionCompleted, "run_memory", donePayload)
	return true, nil
}

func (m *runMemoryManager) runCompactionLockKey() string {
	if m == nil || m.rec == nil || strings.TrimSpace(m.rec.RunID) == "" {
		return ""
	}
	return strings.TrimSpace(m.rec.OwnerID) + "/" + strings.TrimSpace(m.rec.RunID)
}

func canCheckpointRunEventLedger(reason string) bool {
	return strings.TrimSpace(reason) == "continuation_selection"
}

func (m *runMemoryManager) compactRunEventLedger(ctx context.Context, entries []types.RunMemoryEntry, reason string) (bool, error) {
	eventsForRun, err := m.store.ListEvents(ctx, m.rec.RunID, 500)
	if err != nil {
		return false, err
	}
	sourceText, summarizedEvents, omittedDeltaEvents := serializeRunEventLedgerForCheckpoint(m.rec, eventsForRun)
	if strings.TrimSpace(sourceText) == "" {
		return false, nil
	}
	tokensBefore := estimateTextTokens(sourceText)
	summary := summarizeRunEventLedgerCheckpoint(reason, sourceText)
	tokensAfter := estimateRawMessageTokens(compactionSummaryMessage(summary, tokensBefore))

	startPayload, _ := json.Marshal(map[string]any{
		"reason":               reason,
		"source":               "run_event_ledger",
		"tokens_before":        tokensBefore,
		"message_count":        len(eventsForRun),
		"event_count":          len(eventsForRun),
		"summarized_events":    summarizedEvents,
		"omitted_delta_events": omittedDeltaEvents,
	})
	m.emit(types.EventRunCompactionStarted, "run_memory", startPayload)

	entry, err := m.store.AppendRunMemoryEntry(ctx, types.RunMemoryEntry{
		RunID:        m.rec.RunID,
		OwnerID:      m.rec.OwnerID,
		AgentID:      m.rec.AgentID,
		Kind:         types.RunMemoryEntryCompaction,
		Summary:      summary,
		TokensBefore: tokensBefore,
		Reason:       reason,
		Details: map[string]any{
			"source":               "run_event_ledger",
			"source_state":         string(m.rec.State),
			"compacted_messages":   0,
			"kept_messages":        0,
			"tokens_after":         tokensAfter,
			"event_count":          len(eventsForRun),
			"summarized_events":    summarizedEvents,
			"omitted_delta_events": omittedDeltaEvents,
			"prior_memory_entries": len(entries),
		},
	})
	if err != nil {
		return false, err
	}

	donePayload, _ := json.Marshal(map[string]any{
		"entry_id":             entry.EntryID,
		"reason":               reason,
		"source":               "run_event_ledger",
		"source_state":         string(m.rec.State),
		"tokens_before":        tokensBefore,
		"tokens_after":         tokensAfter,
		"compacted_messages":   0,
		"kept_messages":        0,
		"event_count":          len(eventsForRun),
		"summarized_events":    summarizedEvents,
		"omitted_delta_events": omittedDeltaEvents,
	})
	m.emit(types.EventRunCompactionCompleted, "run_memory", donePayload)
	return true, nil
}

type runMemoryCompactionPlan struct {
	Summary               string
	PreviousSummary       string
	Reason                string
	FirstKeptEntryID      string
	TokensBefore          int
	TokensAfterEstimate   int
	CompactedMessages     int
	KeptMessages          int
	RawEntryIDs           []string
	RawToolResultEntryIDs []string
	SummarizedEntries     []types.RunMemoryEntry
}

type runMemoryLLMCompaction struct {
	Summary  string
	Details  map[string]any
	Status   string
	Provider string
	Model    string
}

type runMemoryCheckpoint struct {
	CurrentObjective       string   `json:"current_objective"`
	ActiveTask             string   `json:"active_task"`
	UserHardConstraints    []string `json:"user_hard_constraints"`
	CompletedWork          []string `json:"completed_work"`
	KeyDecisions           []string `json:"key_decisions"`
	OpenObligations        []string `json:"open_obligations"`
	FailedAttempts         []string `json:"failed_attempts"`
	SourceEvidenceHandles  []string `json:"source_evidence_handles"`
	RawEntryHandles        []string `json:"raw_entry_handles"`
	RawToolResultHandles   []string `json:"raw_tool_result_handles"`
	FilesDocsResources     []string `json:"files_docs_resources"`
	BlockersUncertainties  []string `json:"blockers_uncertainties"`
	NextActions            []string `json:"next_actions"`
	RetrievalInstructions  []string `json:"retrieval_instructions"`
	ContinuationCheckpoint string   `json:"continuation_checkpoint"`
}

func planRunMemoryCompaction(entries []types.RunMemoryEntry, keepRecentTokens int, reason string) (runMemoryCompactionPlan, bool) {
	latestCompactionIdx := -1
	for i := range entries {
		if entries[i].Kind == types.RunMemoryEntryCompaction {
			latestCompactionIdx = i
		}
	}

	boundaryStart := latestCompactionIdx + 1
	previousSummary := ""
	if latestCompactionIdx >= 0 {
		previousSummary = entries[latestCompactionIdx].Summary
	}

	messageIdxs := runMemoryMessageIndexes(entries[boundaryStart:])
	for i := range messageIdxs {
		messageIdxs[i] += boundaryStart
	}
	if len(messageIdxs) == 0 {
		return runMemoryCompactionPlan{}, false
	}

	if keepRecentTokens < 0 {
		keepRecentTokens = 0
	}
	firstKeptIdx := len(entries)
	keptTokens := 0
	if keepRecentTokens > 0 {
		for i := len(messageIdxs) - 1; i >= 0; i-- {
			idx := messageIdxs[i]
			msgTokens := estimateRawMessageTokens(entries[idx].Message)
			if firstKeptIdx != len(entries) && keptTokens+msgTokens > keepRecentTokens {
				break
			}
			firstKeptIdx = idx
			keptTokens += msgTokens
		}
	}
	if keepRecentTokens > 0 && firstKeptIdx == len(entries) {
		firstKeptIdx = messageIdxs[len(messageIdxs)-1]
	}
	firstKeptIdx = adjustRunMemoryCut(entries, firstKeptIdx, boundaryStart)

	var summarized []types.RunMemoryEntry
	var keptMessages int
	for _, idx := range messageIdxs {
		if idx < firstKeptIdx {
			summarized = append(summarized, entries[idx])
			continue
		}
		keptMessages++
	}
	if len(summarized) == 0 && strings.TrimSpace(previousSummary) == "" {
		return runMemoryCompactionPlan{}, false
	}

	firstKeptEntryID := ""
	if firstKeptIdx < len(entries) {
		firstKeptEntryID = entries[firstKeptIdx].EntryID
	}
	tokensBefore := estimateRawMessagesTokens(buildRunMemoryContext(entries))
	summary := summarizeRunMemoryMessages(previousSummary, summarized, reason)
	rawEntryIDs, rawToolResultEntryIDs := runMemoryRawEntryIDs(summarized)

	afterEstimateMessages := []json.RawMessage{compactionSummaryMessage(summary, tokensBefore)}
	for _, idx := range messageIdxs {
		if idx >= firstKeptIdx {
			afterEstimateMessages = append(afterEstimateMessages, entries[idx].Message)
		}
	}

	return runMemoryCompactionPlan{
		Summary:               summary,
		PreviousSummary:       previousSummary,
		Reason:                reason,
		FirstKeptEntryID:      firstKeptEntryID,
		TokensBefore:          tokensBefore,
		TokensAfterEstimate:   estimateRawMessagesTokens(afterEstimateMessages),
		CompactedMessages:     len(summarized),
		KeptMessages:          keptMessages,
		RawEntryIDs:           rawEntryIDs,
		RawToolResultEntryIDs: rawToolResultEntryIDs,
		SummarizedEntries:     append([]types.RunMemoryEntry(nil), summarized...),
	}, true
}

func (m *runMemoryManager) effectiveContextThresholdTokens() int {
	if m.cfg.RunMemoryContextThresholdTokens > 0 {
		return m.cfg.RunMemoryContextThresholdTokens
	}
	window := modelcatalog.ContextWindowTokensForModel(m.llmConfig.Model)
	if window <= 0 {
		window = modelcatalog.DefaultContextWindowTokens
	}
	threshold := int(float64(window) * provideriface.DefaultRunMemoryContextThresholdRatio)
	if threshold <= 0 {
		return provideriface.DefaultRunMemoryPromptReserveTokens
	}
	return threshold
}

func (m *runMemoryManager) estimatePromptPressureTokens(entries []types.RunMemoryEntry) int {
	return estimateRawMessagesTokens(buildRunMemoryContext(entries)) +
		m.promptOverheadTokens +
		provideriface.DefaultRunMemoryPromptReserveTokens
}

func (m *runMemoryManager) generateLLMCompaction(ctx context.Context, plan runMemoryCompactionPlan) (runMemoryLLMCompaction, error) {
	if m.provider == nil {
		return runMemoryLLMCompaction{}, fmt.Errorf("llm compactor provider is not configured")
	}
	if strings.TrimSpace(m.llmConfig.Provider) == "" || strings.TrimSpace(m.llmConfig.Model) == "" {
		return runMemoryLLMCompaction{}, fmt.Errorf("llm compactor model selection is incomplete")
	}
	system := strings.Join([]string{
		"You are Choir's runtime run-memory compactor.",
		"Convert durable run memory into a concise typed checkpoint for the same agent run.",
		"Return only one JSON object matching the requested schema.",
		"Do not continue the conversation. Do not expose hidden reasoning.",
		"Preserve exact entry_id handles so the future agent can call get_run_memory_entry when details are needed.",
	}, "\n")
	userText := buildRunMemoryCompactionPrompt(m.rec, plan)
	msg, _ := json.Marshal(map[string]any{
		"role": "user",
		"content": []any{
			map[string]string{"type": "text", "text": userText},
		},
	})
	resp, err := m.provider.CallWithTools(ctx, provideriface.ToolLoopRequest{
		Provider:        m.llmConfig.Provider,
		Model:           m.llmConfig.Model,
		ReasoningEffort: m.llmConfig.ReasoningEffort,
		System:          system,
		Messages:        []json.RawMessage{msg},
		MaxTokens:       provideriface.MaxInteractiveOutputTokensForSelection(m.llmConfig, agentProfileForRun(m.rec)),
	})
	if err != nil {
		return runMemoryLLMCompaction{}, err
	}
	checkpoint, err := parseRunMemoryCheckpoint(resp.Text)
	if err != nil {
		return runMemoryLLMCompaction{}, err
	}
	summary := renderRunMemoryCheckpointSummary(checkpoint, plan)
	details := checkpointDetails(checkpoint)
	details["llm_stop_reason"] = resp.StopReason
	details["llm_response_model"] = resp.Model
	details["llm_input_tokens"] = resp.Usage.InputTokens
	details["llm_output_tokens"] = resp.Usage.OutputTokens
	details["reasoning_content_present"] = strings.TrimSpace(resp.ReasoningContent) != ""
	return runMemoryLLMCompaction{
		Summary:  summary,
		Details:  details,
		Status:   "llm_checkpoint",
		Provider: m.llmConfig.Provider,
		Model:    m.llmConfig.Model,
	}, nil
}

func buildRunMemoryCompactionPrompt(rec *types.RunRecord, plan runMemoryCompactionPlan) string {
	var b strings.Builder
	b.WriteString("Create a typed checkpoint for this Choir run. Return only JSON with these fields:\n")
	b.WriteString(`{"current_objective":"","active_task":"","user_hard_constraints":[],"completed_work":[],"key_decisions":[],"open_obligations":[],"failed_attempts":[],"source_evidence_handles":[],"raw_entry_handles":[],"raw_tool_result_handles":[],"files_docs_resources":[],"blockers_uncertainties":[],"next_actions":[],"retrieval_instructions":[],"continuation_checkpoint":""}`)
	b.WriteString("\n\nRules:\n")
	b.WriteString("- Preserve user hard constraints, current objective, tool obligations, failed attempts, source/evidence handles, and next actions.\n")
	b.WriteString("- Include compacted raw entry ids in raw_entry_handles when they may need exact retrieval.\n")
	b.WriteString("- Include tool-result entry ids in raw_tool_result_handles.\n")
	b.WriteString("- Use get_run_memory_entry(entry_id) in retrieval_instructions when exact raw content may matter.\n")
	b.WriteString("- Do not invent facts. Say uncertain when uncertain.\n")
	b.WriteString("- Keep continuation_checkpoint concise but useful.\n")
	if rec != nil {
		b.WriteString("\nRun record:\n")
		fmt.Fprintf(&b, "- loop_id=%s state=%s agent_profile=%s agent_role=%s\n", rec.RunID, rec.State, rec.AgentProfile, rec.AgentRole)
		if strings.TrimSpace(rec.Prompt) != "" {
			fmt.Fprintf(&b, "- original_prompt=%s\n", truncateForRunMemory(rec.Prompt, 1400))
		}
	}
	if strings.TrimSpace(plan.PreviousSummary) != "" {
		b.WriteString("\nPrevious checkpoint summary:\n")
		b.WriteString(truncateForRunMemory(plan.PreviousSummary, 3000))
		b.WriteString("\n")
	}
	b.WriteString("\nCompaction metadata:\n")
	fmt.Fprintf(&b, "- reason=%s\n", plan.Reason)
	fmt.Fprintf(&b, "- tokens_before_estimate=%d\n", plan.TokensBefore)
	fmt.Fprintf(&b, "- first_kept_entry_id=%s\n", plan.FirstKeptEntryID)
	fmt.Fprintf(&b, "- raw_entry_ids=%s\n", strings.Join(plan.RawEntryIDs, ","))
	fmt.Fprintf(&b, "- raw_tool_result_entry_ids=%s\n", strings.Join(plan.RawToolResultEntryIDs, ","))
	b.WriteString("\nCompacted messages:\n")
	for _, entry := range plan.SummarizedEntries {
		fmt.Fprintf(&b, "- entry_id=%s role=%s seq=%d content=%s\n",
			entry.EntryID,
			firstNonEmpty(entry.Role, runMemoryMessageRole(entry.Message)),
			entry.Seq,
			describeRunMemoryMessageForLLM(entry.Message),
		)
	}
	return b.String()
}

func parseRunMemoryCheckpoint(text string) (runMemoryCheckpoint, error) {
	text = strings.TrimSpace(text)
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start < 0 || end <= start {
		return runMemoryCheckpoint{}, fmt.Errorf("llm compaction response did not contain JSON object: %s", truncateForRunMemory(text, 500))
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(text[start:end+1]), &raw); err != nil {
		return runMemoryCheckpoint{}, fmt.Errorf("decode llm compaction checkpoint: %w", err)
	}
	checkpoint := runMemoryCheckpoint{
		CurrentObjective:       checkpointStringField(raw, "current_objective"),
		ActiveTask:             checkpointStringField(raw, "active_task"),
		UserHardConstraints:    checkpointStringListField(raw, "user_hard_constraints"),
		CompletedWork:          checkpointStringListField(raw, "completed_work"),
		KeyDecisions:           checkpointStringListField(raw, "key_decisions"),
		OpenObligations:        checkpointStringListField(raw, "open_obligations"),
		FailedAttempts:         checkpointStringListField(raw, "failed_attempts"),
		SourceEvidenceHandles:  checkpointStringListField(raw, "source_evidence_handles"),
		RawEntryHandles:        checkpointStringListField(raw, "raw_entry_handles"),
		RawToolResultHandles:   checkpointStringListField(raw, "raw_tool_result_handles"),
		FilesDocsResources:     checkpointStringListField(raw, "files_docs_resources"),
		BlockersUncertainties:  checkpointStringListField(raw, "blockers_uncertainties"),
		NextActions:            checkpointStringListField(raw, "next_actions"),
		RetrievalInstructions:  checkpointStringListField(raw, "retrieval_instructions"),
		ContinuationCheckpoint: checkpointStringField(raw, "continuation_checkpoint"),
	}
	if strings.TrimSpace(checkpoint.CurrentObjective) == "" &&
		strings.TrimSpace(checkpoint.ActiveTask) == "" &&
		strings.TrimSpace(checkpoint.ContinuationCheckpoint) == "" {
		return runMemoryCheckpoint{}, fmt.Errorf("llm compaction checkpoint is empty")
	}
	return checkpoint, nil
}

func checkpointStringField(raw map[string]any, key string) string {
	value, ok := raw[key]
	if !ok {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case []any:
		values := checkpointStringListField(raw, key)
		return strings.Join(values, "; ")
	default:
		encoded, _ := json.Marshal(typed)
		return strings.TrimSpace(string(encoded))
	}
}

func checkpointStringListField(raw map[string]any, key string) []string {
	value, ok := raw[key]
	if !ok {
		return nil
	}
	switch typed := value.(type) {
	case string:
		if strings.TrimSpace(typed) == "" {
			return nil
		}
		return []string{strings.TrimSpace(typed)}
	case []string:
		return cleanCheckpointStrings(typed)
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			switch v := item.(type) {
			case string:
				out = append(out, v)
			default:
				encoded, _ := json.Marshal(v)
				out = append(out, string(encoded))
			}
		}
		return cleanCheckpointStrings(out)
	default:
		encoded, _ := json.Marshal(typed)
		return cleanCheckpointStrings([]string{string(encoded)})
	}
}

func renderRunMemoryCheckpointSummary(checkpoint runMemoryCheckpoint, plan runMemoryCompactionPlan) string {
	var b strings.Builder
	b.WriteString("Run memory LLM checkpoint\n")
	b.WriteString("Reason: ")
	b.WriteString(plan.Reason)
	b.WriteString("\n\n")
	writeCheckpointScalar(&b, "Current objective", checkpoint.CurrentObjective)
	writeCheckpointScalar(&b, "Active task", checkpoint.ActiveTask)
	writeCheckpointList(&b, "User hard constraints", checkpoint.UserHardConstraints)
	writeCheckpointList(&b, "Completed work", checkpoint.CompletedWork)
	writeCheckpointList(&b, "Key decisions", checkpoint.KeyDecisions)
	writeCheckpointList(&b, "Open obligations", checkpoint.OpenObligations)
	writeCheckpointList(&b, "Failed attempts / do not repeat", checkpoint.FailedAttempts)
	writeCheckpointList(&b, "Source/evidence/artifact handles", checkpoint.SourceEvidenceHandles)
	writeCheckpointList(&b, "Raw entry handles", checkpoint.RawEntryHandles)
	writeCheckpointList(&b, "Raw tool-result handles", checkpoint.RawToolResultHandles)
	writeCheckpointList(&b, "Files/docs/resources", checkpoint.FilesDocsResources)
	writeCheckpointList(&b, "Blockers and uncertainties", checkpoint.BlockersUncertainties)
	writeCheckpointList(&b, "Next actions", checkpoint.NextActions)
	writeCheckpointList(&b, "Retrieval instructions", checkpoint.RetrievalInstructions)
	writeCheckpointScalar(&b, "Continuation checkpoint", checkpoint.ContinuationCheckpoint)
	if len(plan.RawEntryIDs) > 0 {
		b.WriteString("\nCompacted raw entry ids available via get_run_memory_entry:\n")
		for _, id := range plan.RawEntryIDs {
			fmt.Fprintf(&b, "- %s\n", id)
		}
	}
	return strings.TrimSpace(b.String())
}

func checkpointDetails(checkpoint runMemoryCheckpoint) map[string]any {
	return map[string]any{
		"current_objective":       checkpoint.CurrentObjective,
		"active_task":             checkpoint.ActiveTask,
		"user_hard_constraints":   checkpoint.UserHardConstraints,
		"completed_work":          checkpoint.CompletedWork,
		"key_decisions":           checkpoint.KeyDecisions,
		"open_obligations":        checkpoint.OpenObligations,
		"failed_attempts":         checkpoint.FailedAttempts,
		"source_evidence_handles": checkpoint.SourceEvidenceHandles,
		"raw_entry_handles":       checkpoint.RawEntryHandles,
		"raw_tool_result_handles": checkpoint.RawToolResultHandles,
		"files_docs_resources":    checkpoint.FilesDocsResources,
		"blockers_uncertainties":  checkpoint.BlockersUncertainties,
		"next_actions":            checkpoint.NextActions,
		"retrieval_instructions":  checkpoint.RetrievalInstructions,
		"continuation_checkpoint": checkpoint.ContinuationCheckpoint,
	}
}

func deterministicEmergencyRunMemoryCompaction(plan runMemoryCompactionPlan, cause error) runMemoryLLMCompaction {
	details := map[string]any{
		"fallback":                    "deterministic_emergency",
		"fallback_is_readiness_proof": false,
		"fallback_error":              "",
		"raw_entry_handles":           plan.RawEntryIDs,
		"raw_tool_result_handles":     plan.RawToolResultEntryIDs,
	}
	if cause != nil {
		details["fallback_error"] = cause.Error()
	}
	return runMemoryLLMCompaction{
		Summary:  plan.Summary + "\n\nEmergency fallback: LLM compaction failed. This deterministic checkpoint preserves raw entry handles for recovery but is not provider-readiness evidence.",
		Details:  details,
		Status:   "emergency_fallback",
		Provider: "",
		Model:    "",
	}
}

func writeCheckpointScalar(b *strings.Builder, label, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	fmt.Fprintf(b, "%s: %s\n\n", label, value)
}

func writeCheckpointList(b *strings.Builder, label string, values []string) {
	cleaned := cleanCheckpointStrings(values)
	if len(cleaned) == 0 {
		return
	}
	b.WriteString(label)
	b.WriteString(":\n")
	for _, value := range cleaned {
		fmt.Fprintf(b, "- %s\n", value)
	}
	b.WriteString("\n")
}

func cleanCheckpointStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func buildRunMemoryContext(entries []types.RunMemoryEntry) []json.RawMessage {
	latestCompactionIdx := -1
	for i := range entries {
		if entries[i].Kind == types.RunMemoryEntryCompaction {
			latestCompactionIdx = i
		}
	}
	if latestCompactionIdx == -1 {
		return runMemoryMessages(entries)
	}

	latest := entries[latestCompactionIdx]
	messages := []json.RawMessage{compactionSummaryMessage(latest.Summary, latest.TokensBefore)}
	if latest.FirstKeptEntryID != "" {
		keep := false
		for i := 0; i < latestCompactionIdx; i++ {
			if entries[i].EntryID == latest.FirstKeptEntryID {
				keep = true
			}
			if keep && entries[i].Kind == types.RunMemoryEntryMessage && len(entries[i].Message) > 0 {
				messages = append(messages, cloneRawMessage(entries[i].Message))
			}
		}
	}
	for i := latestCompactionIdx + 1; i < len(entries); i++ {
		if entries[i].Kind == types.RunMemoryEntryMessage && len(entries[i].Message) > 0 {
			messages = append(messages, cloneRawMessage(entries[i].Message))
		}
	}
	return messages
}

func runMemoryMessages(entries []types.RunMemoryEntry) []json.RawMessage {
	messages := make([]json.RawMessage, 0, len(entries))
	for _, entry := range entries {
		if entry.Kind == types.RunMemoryEntryMessage && len(entry.Message) > 0 {
			messages = append(messages, cloneRawMessage(entry.Message))
		}
	}
	return messages
}

func runMemoryMessageIndexes(entries []types.RunMemoryEntry) []int {
	indexes := make([]int, 0, len(entries))
	for i, entry := range entries {
		if entry.Kind == types.RunMemoryEntryMessage && len(entry.Message) > 0 {
			indexes = append(indexes, i)
		}
	}
	return indexes
}

func adjustRunMemoryCut(entries []types.RunMemoryEntry, firstKeptIdx, boundaryStart int) int {
	if firstKeptIdx >= len(entries) {
		return firstKeptIdx
	}
	if entries[firstKeptIdx].Kind != types.RunMemoryEntryMessage {
		for i := firstKeptIdx + 1; i < len(entries); i++ {
			if entries[i].Kind == types.RunMemoryEntryMessage {
				return i
			}
		}
		return len(entries)
	}
	if !isToolResultOnlyMessage(entries[firstKeptIdx].Message) {
		return firstKeptIdx
	}
	for i := firstKeptIdx - 1; i >= boundaryStart; i-- {
		if entries[i].Kind != types.RunMemoryEntryMessage {
			continue
		}
		if assistantMessageHasToolUse(entries[i].Message) {
			return i
		}
		return i
	}
	return firstKeptIdx
}

func summarizeRunMemoryMessages(previousSummary string, entries []types.RunMemoryEntry, reason string) string {
	var b strings.Builder
	b.WriteString("Run memory checkpoint\n")
	b.WriteString("Reason: ")
	b.WriteString(reason)
	b.WriteString("\n\n")
	b.WriteString("Operational invariant: continue the same run from this checkpoint, preserving user intent, tool results, open obligations, and safety constraints.\n\n")
	if strings.TrimSpace(previousSummary) != "" {
		b.WriteString("Previous checkpoint:\n")
		b.WriteString(truncateForRunMemory(previousSummary, 2000))
		b.WriteString("\n\n")
	}
	b.WriteString("Compacted conversation:\n")
	for _, entry := range entries {
		b.WriteString("- ")
		if strings.TrimSpace(entry.EntryID) != "" {
			b.WriteString("entry_id=")
			b.WriteString(entry.EntryID)
			b.WriteString(" ")
		}
		b.WriteString(entry.Role)
		if entry.Role == "" {
			b.WriteString(runMemoryMessageRole(entry.Message))
		}
		b.WriteString(": ")
		b.WriteString(describeRunMemoryMessage(entry.Message))
		b.WriteString("\n")
	}
	return b.String()
}

func runMemoryRawEntryIDs(entries []types.RunMemoryEntry) ([]string, []string) {
	all := make([]string, 0, len(entries))
	toolResults := []string{}
	for _, entry := range entries {
		if strings.TrimSpace(entry.EntryID) == "" {
			continue
		}
		all = append(all, entry.EntryID)
		if isToolResultOnlyMessage(entry.Message) {
			toolResults = append(toolResults, entry.EntryID)
		}
	}
	return all, toolResults
}

func summarizeRunEventLedgerCheckpoint(reason, sourceText string) string {
	var b strings.Builder
	b.WriteString("Run memory checkpoint\n")
	b.WriteString("Reason: ")
	b.WriteString(reason)
	b.WriteString("\n\n")
	b.WriteString("Operational invariant: continue the same run from this checkpoint, preserving user intent, evidence-bearing events, open obligations, and safety constraints.\n\n")
	b.WriteString("Source: durable run record and event ledger. The provider-message log had no compactable messages for this control-plane run, so this checkpoint preserves the durable evidence the continuation will actually depend on.\n\n")
	b.WriteString("Run event ledger:\n")
	b.WriteString(sourceText)
	return b.String()
}

func serializeRunEventLedgerForCheckpoint(rec *types.RunRecord, eventsForRun []types.EventRecord) (string, int, int) {
	if rec == nil {
		return "", 0, 0
	}
	var b strings.Builder
	b.WriteString("Run record:\n")
	fmt.Fprintf(&b, "- loop_id=%s state=%s agent_id=%s agent_profile=%s agent_role=%s channel_id=%s trajectory_id=%s\n",
		rec.RunID,
		rec.State,
		rec.AgentID,
		rec.AgentProfile,
		rec.AgentRole,
		rec.ChannelID,
		metadataStringValue(rec.Metadata, runMetadataTrajectoryID),
	)
	if strings.TrimSpace(rec.Prompt) != "" {
		fmt.Fprintf(&b, "- prompt=%s\n", truncateForRunMemory(rec.Prompt, 700))
	}
	if strings.TrimSpace(rec.Result) != "" {
		fmt.Fprintf(&b, "- result=%s\n", truncateForRunMemory(rec.Result, 900))
	}
	if strings.TrimSpace(rec.Error) != "" {
		fmt.Fprintf(&b, "- error=%s\n", truncateForRunMemory(rec.Error, 700))
	}
	if len(rec.Metadata) > 0 {
		if metadataJSON, err := json.Marshal(rec.Metadata); err == nil {
			fmt.Fprintf(&b, "- metadata=%s\n", truncateForRunMemory(string(metadataJSON), 900))
		}
	}

	interesting := make([]types.EventRecord, 0, len(eventsForRun))
	omittedDeltaEvents := 0
	for _, ev := range eventsForRun {
		if ev.Kind == types.EventRunDelta {
			omittedDeltaEvents++
			continue
		}
		interesting = append(interesting, ev)
	}
	const maxEvents = 40
	omittedInteresting := 0
	if len(interesting) > maxEvents {
		omittedInteresting = len(interesting) - maxEvents
		interesting = interesting[omittedInteresting:]
	}

	b.WriteString("\nEvents:\n")
	if len(eventsForRun) == 0 {
		b.WriteString("- no persisted events found for this run\n")
	}
	if omittedInteresting > 0 {
		fmt.Fprintf(&b, "- omitted_earlier_events=%d\n", omittedInteresting)
	}
	if omittedDeltaEvents > 0 {
		fmt.Fprintf(&b, "- omitted_stream_delta_events=%d\n", omittedDeltaEvents)
	}
	for _, ev := range interesting {
		payload := truncateForRunMemory(string(ev.Payload), 450)
		fmt.Fprintf(&b, "- seq=%d stream_seq=%d kind=%s phase=%s agent_id=%s channel_id=%s payload=%s\n",
			ev.Seq,
			ev.StreamSeq,
			ev.Kind,
			ev.Phase,
			ev.AgentID,
			ev.ChannelID,
			payload,
		)
	}
	return b.String(), len(interesting), omittedDeltaEvents
}

func compactionSummaryMessage(summary string, tokensBefore int) json.RawMessage {
	if strings.TrimSpace(summary) == "" {
		summary = "Run memory checkpoint with no additional summary."
	}
	text := fmt.Sprintf("Context checkpoint summary. Compacted prior provider context estimated at %d tokens.\n\n%s", tokensBefore, summary)
	msg, _ := json.Marshal(map[string]any{
		"role": "user",
		"content": []any{
			map[string]string{"type": "text", "text": text},
		},
	})
	return msg
}

func runMemoryMessageRole(msg json.RawMessage) string {
	var parsed struct {
		Role string `json:"role"`
	}
	if err := json.Unmarshal(msg, &parsed); err != nil {
		return ""
	}
	return parsed.Role
}

func isToolResultOnlyMessage(msg json.RawMessage) bool {
	var parsed struct {
		Role    string `json:"role"`
		Content any    `json:"content"`
	}
	if err := json.Unmarshal(msg, &parsed); err != nil {
		return false
	}
	if parsed.Role != "user" {
		return false
	}
	items, ok := parsed.Content.([]any)
	if !ok || len(items) == 0 {
		return false
	}
	for _, item := range items {
		block, ok := item.(map[string]any)
		if !ok {
			return false
		}
		blockType, _ := block["type"].(string)
		if blockType != "tool_result" {
			return false
		}
	}
	return true
}

func assistantMessageHasToolUse(msg json.RawMessage) bool {
	var parsed struct {
		Role    string `json:"role"`
		Content any    `json:"content"`
	}
	if err := json.Unmarshal(msg, &parsed); err != nil {
		return false
	}
	if parsed.Role != "assistant" {
		return false
	}
	items, ok := parsed.Content.([]any)
	if !ok {
		return false
	}
	for _, item := range items {
		block, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if blockType, _ := block["type"].(string); blockType == "tool_use" {
			return true
		}
	}
	return false
}

func describeRunMemoryMessage(msg json.RawMessage) string {
	var parsed struct {
		Role    string `json:"role"`
		Content any    `json:"content"`
	}
	if err := json.Unmarshal(msg, &parsed); err != nil {
		return truncateForRunMemory(string(msg), 500)
	}
	switch content := parsed.Content.(type) {
	case string:
		return truncateForRunMemory(content, 500)
	case []any:
		parts := make([]string, 0, len(content))
		for _, item := range content {
			block, ok := item.(map[string]any)
			if !ok {
				continue
			}
			blockType, _ := block["type"].(string)
			switch blockType {
			case "text":
				if text, _ := block["text"].(string); text != "" {
					parts = append(parts, "text="+truncateForRunMemory(text, 350))
				}
			case "tool_use":
				name, _ := block["name"].(string)
				id, _ := block["id"].(string)
				parts = append(parts, fmt.Sprintf("tool_use name=%s id=%s", name, id))
			case "tool_result":
				id, _ := block["tool_use_id"].(string)
				contentText, _ := block["content"].(string)
				parts = append(parts, fmt.Sprintf("tool_result id=%s content=%s", id, truncateForRunMemory(contentText, 250)))
			default:
				if blockType != "" {
					parts = append(parts, blockType)
				}
			}
		}
		if len(parts) > 0 {
			return strings.Join(parts, "; ")
		}
	}
	return truncateForRunMemory(string(msg), 500)
}

func describeRunMemoryMessageForLLM(msg json.RawMessage) string {
	var parsed struct {
		Content any `json:"content"`
	}
	if err := json.Unmarshal(msg, &parsed); err != nil {
		return truncateForRunMemory(string(msg), 4000)
	}
	switch content := parsed.Content.(type) {
	case string:
		return truncateForRunMemory(content, 4000)
	case []any:
		parts := make([]string, 0, len(content))
		for _, item := range content {
			block, ok := item.(map[string]any)
			if !ok {
				continue
			}
			blockType, _ := block["type"].(string)
			switch blockType {
			case "text":
				if text, _ := block["text"].(string); text != "" {
					parts = append(parts, "text="+truncateForRunMemory(text, 2500))
				}
			case "tool_use":
				name, _ := block["name"].(string)
				id, _ := block["id"].(string)
				args, _ := json.Marshal(block["input"])
				parts = append(parts, fmt.Sprintf("tool_use name=%s id=%s input=%s", name, id, truncateForRunMemory(string(args), 1000)))
			case "tool_result":
				id, _ := block["tool_use_id"].(string)
				contentText, _ := block["content"].(string)
				parts = append(parts, fmt.Sprintf("tool_result id=%s content=%s", id, truncateForRunMemory(contentText, 1500)))
			default:
				if blockType != "" {
					parts = append(parts, blockType)
				}
			}
		}
		if len(parts) > 0 {
			return strings.Join(parts, "; ")
		}
	}
	return truncateForRunMemory(string(msg), 4000)
}

func estimateRawMessagesTokens(messages []json.RawMessage) int {
	total := 0
	for _, msg := range messages {
		total += estimateRawMessageTokens(msg)
	}
	return total
}

func estimateRawMessageTokens(msg json.RawMessage) int {
	if len(msg) == 0 {
		return 0
	}
	return len(msg)/4 + 1
}

func estimateTextTokens(text string) int {
	if text == "" {
		return 0
	}
	return len(text)/4 + 1
}

func cloneRawMessage(msg json.RawMessage) json.RawMessage {
	if len(msg) == 0 {
		return nil
	}
	cloned := make(json.RawMessage, len(msg))
	copy(cloned, msg)
	return cloned
}

func truncateForRunMemory(s string, limit int) string {
	s = strings.TrimSpace(s)
	if limit <= 0 || len(s) <= limit {
		return s
	}
	if limit <= 3 {
		return s[:limit]
	}
	return strings.TrimSpace(s[:limit-3]) + "..."
}

func isContextOverflowError(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	if strings.Contains(text, "context deadline exceeded") ||
		strings.Contains(text, "context canceled") ||
		strings.Contains(text, "context cancelled") {
		return false
	}
	if strings.Contains(text, "context") {
		for _, marker := range []string{
			"overflow",
			"too long",
			"too many tokens",
			"exceed",
			"length",
			"window",
		} {
			if strings.Contains(text, marker) {
				return true
			}
		}
	}
	for _, marker := range []string{
		"prompt is too long",
		"maximum context",
		"request_too_large",
		"input is too long",
		"too many input tokens",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func isRunMemoryBlockedError(err error) bool {
	return errors.Is(err, ErrContextOverflowRecoveryFailed) ||
		errors.Is(err, ErrRunMemoryCompactionFailed) ||
		isContextOverflowError(err)
}
