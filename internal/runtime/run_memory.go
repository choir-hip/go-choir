package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	runtimestore "github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

var (
	// ErrContextOverflowRecoveryFailed marks a context overflow that remained
	// blocked after the runtime compacted durable run memory and retried once.
	ErrContextOverflowRecoveryFailed = errors.New("context overflow recovery failed")

	// ErrRunMemoryCompactionFailed marks a failure in durable memory compaction
	// before a provider call.
	ErrRunMemoryCompactionFailed = errors.New("run memory compaction failed")
)

type runMemoryManager struct {
	store                     *runtimestore.Store
	rec                       *types.RunRecord
	cfg                       Config
	emit                      EventEmitFunc
	overflowRecoveryAttempted bool
}

func newRunMemoryManager(store *runtimestore.Store, rec *types.RunRecord, cfg Config, emit EventEmitFunc) *runMemoryManager {
	return &runMemoryManager{
		store: store,
		rec:   rec,
		cfg:   normalizeConfig(cfg),
		emit:  emit,
	}
}

func (m *runMemoryManager) initialize(ctx context.Context, initialMessages []json.RawMessage) ([]json.RawMessage, error) {
	entries, err := m.store.ListRunMemoryEntries(ctx, m.rec.OwnerID, m.rec.RunID)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		for _, msg := range initialMessages {
			if err := m.appendMessage(ctx, runMemoryMessageRole(msg), msg); err != nil {
				return nil, err
			}
		}
	}
	return m.contextMessages(ctx)
}

func (m *runMemoryManager) hooks() ToolLoopMemoryHooks {
	return ToolLoopMemoryHooks{
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
		tokens := estimateRawMessagesTokens(buildRunMemoryContext(entries))
		if tokens <= m.cfg.RunMemoryContextThresholdTokens {
			return false, nil
		}
		if entries[len(entries)-1].Kind == types.RunMemoryEntryCompaction {
			return false, nil
		}
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

	startPayload, _ := json.Marshal(map[string]any{
		"reason":        reason,
		"tokens_before": plan.TokensBefore,
		"message_count": plan.CompactedMessages + plan.KeptMessages,
	})
	m.emit(types.EventRunCompactionStarted, "run_memory", startPayload)

	entry, err := m.store.AppendRunMemoryEntry(ctx, types.RunMemoryEntry{
		RunID:            m.rec.RunID,
		OwnerID:          m.rec.OwnerID,
		AgentID:          m.rec.AgentID,
		Kind:             types.RunMemoryEntryCompaction,
		Summary:          plan.Summary,
		FirstKeptEntryID: plan.FirstKeptEntryID,
		TokensBefore:     plan.TokensBefore,
		Reason:           reason,
		Details: map[string]any{
			"compacted_messages": plan.CompactedMessages,
			"kept_messages":      plan.KeptMessages,
			"tokens_after":       plan.TokensAfterEstimate,
		},
	})
	if err != nil {
		return false, err
	}

	donePayload, _ := json.Marshal(map[string]any{
		"entry_id":            entry.EntryID,
		"reason":              reason,
		"tokens_before":       plan.TokensBefore,
		"tokens_after":        plan.TokensAfterEstimate,
		"first_kept_entry_id": plan.FirstKeptEntryID,
		"compacted_messages":  plan.CompactedMessages,
		"kept_messages":       plan.KeptMessages,
	})
	m.emit(types.EventRunCompactionCompleted, "run_memory", donePayload)
	return true, nil
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
	Summary             string
	FirstKeptEntryID    string
	TokensBefore        int
	TokensAfterEstimate int
	CompactedMessages   int
	KeptMessages        int
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

	afterEstimateMessages := []json.RawMessage{compactionSummaryMessage(summary, tokensBefore)}
	for _, idx := range messageIdxs {
		if idx >= firstKeptIdx {
			afterEstimateMessages = append(afterEstimateMessages, entries[idx].Message)
		}
	}

	return runMemoryCompactionPlan{
		Summary:             summary,
		FirstKeptEntryID:    firstKeptEntryID,
		TokensBefore:        tokensBefore,
		TokensAfterEstimate: estimateRawMessagesTokens(afterEstimateMessages),
		CompactedMessages:   len(summarized),
		KeptMessages:        keptMessages,
	}, true
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
