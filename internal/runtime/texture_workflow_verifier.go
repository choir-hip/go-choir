package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/agentprofile"
)

type TextureWorkflowVerificationOptions struct {
	OwnerID                     string
	TrajectoryID                string
	PromptSubmissionID          string
	RequireResearchUpdates      bool
	RequireWorkerUpdates        bool
	RequirePersistentSuper      bool
	RequireCoSuper              bool
	RequireSearchToolEvent      bool
	RequireArtifactWriteEvent   bool
	RequireVerificationCmdEvent bool
	RequireWorkerConsumption    bool
	RequireToolBackedWorkerRuns bool
}

type TextureWorkflowVerificationReport struct {
	Guarantees []string `json:"guarantees"`
}

func (rt *Runtime) VerifyTextureWorkflow(ctx context.Context, opts TextureWorkflowVerificationOptions) (TextureWorkflowVerificationReport, error) {
	report := TextureWorkflowVerificationReport{}
	guarantee := func(text string) {
		report.Guarantees = append(report.Guarantees, text)
	}
	ownerID := strings.TrimSpace(opts.OwnerID)
	trajectoryID := strings.TrimSpace(opts.TrajectoryID)
	submissionID := strings.TrimSpace(opts.PromptSubmissionID)
	if ownerID == "" || trajectoryID == "" || submissionID == "" {
		return report, fmt.Errorf("owner_id, trajectory_id, and prompt_submission_id are required")
	}

	conductor, err := rt.store.GetRun(ctx, submissionID)
	if err != nil {
		return report, fmt.Errorf("load prompt submission run: %w", err)
	}
	if conductor.OwnerID != ownerID {
		return report, fmt.Errorf("prompt submission owner = %q, want %q", conductor.OwnerID, ownerID)
	}
	if agentProfileForRun(&conductor) != agentprofile.Conductor || agentRoleForRun(&conductor) != agentprofile.Conductor {
		return report, fmt.Errorf("prompt submission is %q/%q, want conductor/conductor", agentProfileForRun(&conductor), agentRoleForRun(&conductor))
	}
	if metadataStringValue(conductor.Metadata, "input_source") != "prompt_bar" {
		return report, fmt.Errorf("prompt submission input_source = %q, want prompt_bar", metadataStringValue(conductor.Metadata, "input_source"))
	}
	if trajectoryIDForRun(&conductor) != trajectoryID {
		return report, fmt.Errorf("prompt submission trajectory_id = %q, want %q", trajectoryIDForRun(&conductor), trajectoryID)
	}
	guarantee("server created the conductor run from prompt-bar metadata")

	decision := fillConductorDecisionFromRun(&conductor, conductorDecision{})
	if raw := strings.TrimSpace(conductor.Result); raw != "" {
		_ = json.Unmarshal([]byte(raw), &decision)
		decision = fillConductorDecisionFromRun(&conductor, decision)
	}
	if decision.Action != "open_app" || !isTextureDecisionApp(decision.App) || strings.TrimSpace(decision.DocID) == "" {
		return report, fmt.Errorf("conductor did not route to Texture: %+v", decision)
	}
	doc, err := rt.store.GetDocument(ctx, decision.DocID, ownerID)
	if err != nil {
		return report, fmt.Errorf("load routed texture document: %w", err)
	}
	guarantee("conductor routed to a durable texture document")

	runs, err := rt.ListRunsByOwner(ctx, ownerID, 1000)
	if err != nil {
		return report, fmt.Errorf("list owner runs: %w", err)
	}
	trajectoryRuns := filterRunsByTrajectory(runs, trajectoryID)
	if len(trajectoryRuns) == 0 {
		return report, fmt.Errorf("trajectory %s has no runs", trajectoryID)
	}
	runByID := make(map[string]types.RunRecord, len(trajectoryRuns))
	textureRunIDs := map[string]bool{}
	for _, run := range trajectoryRuns {
		runByID[run.RunID] = run
		if agentProfileForRun(&run) == agentprofile.Texture {
			textureRunIDs[run.RunID] = true
		}
	}
	if len(textureRunIDs) == 0 {
		return report, fmt.Errorf("trajectory %s has no texture run", trajectoryID)
	}
	events, err := rt.store.ListEventsByTrajectory(ctx, ownerID, trajectoryID, 2000)
	if err != nil {
		return report, fmt.Errorf("list trajectory events: %w", err)
	}
	if len(events) == 0 {
		return report, fmt.Errorf("trajectory %s has no events", trajectoryID)
	}
	if err := verifyAllowedTextureDelegation(trajectoryRuns); err != nil {
		return report, err
	}
	guarantee("texture delegated only allowed work directly")
	if opts.RequireToolBackedWorkerRuns {
		if err := verifyWorkerRunToolCausality(trajectoryRuns, events); err != nil {
			return report, err
		}
		guarantee("worker run creation is backed by parent tool results")
	}

	if opts.RequirePersistentSuper {
		if err := verifyPersistentSuperPath(ownerID, trajectoryRuns); err != nil {
			return report, err
		}
		guarantee("privileged execution flowed through persistent super")
	}
	if opts.RequireCoSuper {
		if err := verifyCoSuperParents(trajectoryRuns); err != nil {
			return report, err
		}
		guarantee("co-super execution was spawned only by super or vsuper")
	}

	if opts.RequireSearchToolEvent && !eventsContainSuccessfulWebSearch(events) {
		return report, fmt.Errorf("missing successful web_search tool result with provider/results")
	}
	if opts.RequireSearchToolEvent {
		guarantee("live search requirement has a successful web_search tool result")
	}
	if opts.RequireArtifactWriteEvent && !eventsContainAnySuccessfulTool(events, "write_file", "edit_file") {
		return report, fmt.Errorf("missing successful artifact write tool result")
	}
	if opts.RequireArtifactWriteEvent {
		guarantee("artifact write requirement has a successful file-write tool result")
	}
	if opts.RequireVerificationCmdEvent && !eventsContainSuccessfulBashVerification(events) {
		return report, fmt.Errorf("missing successful bash verification result")
	}
	if opts.RequireVerificationCmdEvent {
		guarantee("verification requirement has a successful command result")
	}

	updates, err := rt.store.ListWorkerUpdatesByTrajectory(ctx, ownerID, trajectoryID, 1000)
	if err != nil {
		return report, fmt.Errorf("list worker updates: %w", err)
	}
	textureUpdates := workerUpdatesForTextureDoc(updates, doc.DocID)
	if opts.RequireResearchUpdates {
		researchUpdateCount := 0
		for _, update := range updates {
			if update.Role != agentprofile.Researcher {
				continue
			}
			researchUpdateCount++
			if !textureAgentIDMatchesDoc(update.TargetAgentID, doc.DocID) || update.ChannelID != doc.DocID || update.MessageSeq == 0 {
				return report, fmt.Errorf("research coagent update %s is not routed to Texture document %s", update.UpdateID, doc.DocID)
			}
			if coagentPacketPayloadEmpty(update.Packet) {
				return report, fmt.Errorf("research coagent update %s has no source packet payload", update.UpdateID)
			}
		}
		if researchUpdateCount == 0 {
			return report, fmt.Errorf("missing structured research coagent updates")
		}
		guarantee("researchers emitted structured coagent source packets")
	}
	if opts.RequireWorkerUpdates {
		workerUpdateCount := 0
		for _, update := range updates {
			if update.Role == agentprofile.Researcher {
				continue
			}
			if !textureAgentIDMatchesDoc(update.TargetAgentID, doc.DocID) || update.ChannelID != doc.DocID || update.MessageSeq == 0 {
				continue
			}
			workerUpdateCount++
			if coagentPacketPayloadEmpty(update.Packet) {
				return report, fmt.Errorf("worker update %s has no source packet payload", update.UpdateID)
			}
		}
		if workerUpdateCount == 0 {
			return report, fmt.Errorf("missing structured worker updates")
		}
		guarantee("execution workers emitted structured artifacts/tests/results")
	}
	if opts.RequireArtifactWriteEvent {
		if err := verifyArtifactWritesCoverWorkerUpdates(events, textureUpdates); err != nil {
			return report, err
		}
		guarantee("artifact write result matches a structured worker artifact")
	}
	if opts.RequireVerificationCmdEvent {
		if err := verifyBashCoversWorkerUpdateTests(events, textureUpdates); err != nil {
			return report, err
		}
		guarantee("verification command result matches structured worker tests")
	}

	revisions, err := rt.store.ListRevisionsByDoc(ctx, doc.DocID, ownerID, 200)
	if err != nil {
		return report, fmt.Errorf("list Texture revisions: %w", err)
	}
	if err := verifyTextureRevisionCausality(revisions, events, textureUpdates, opts.RequireWorkerConsumption); err != nil {
		return report, err
	}
	guarantee("Texture revisions have valid causal parents")
	guarantee("Texture appagent revisions support N:1 loop-to-revision causality through Texture write tools")
	if opts.RequireWorkerConsumption {
		guarantee("Texture consumed worker update message sequences in a later revision")
	}

	return report, nil
}

func filterRunsByTrajectory(runs []types.RunRecord, trajectoryID string) []types.RunRecord {
	out := []types.RunRecord{}
	for _, run := range runs {
		if trajectoryIDForRun(&run) == trajectoryID {
			out = append(out, run)
		}
	}
	return out
}

func verifyAllowedTextureDelegation(runs []types.RunRecord) error {
	runByID := make(map[string]types.RunRecord, len(runs))
	for _, run := range runs {
		runByID[run.RunID] = run
	}
	for _, run := range runs {
		if strings.TrimSpace(run.RequestedByRunID) == "" {
			continue
		}
		parent, ok := runByID[run.RequestedByRunID]
		if !ok {
			continue
		}
		if agentProfileForRun(&parent) == agentprofile.Texture {
			switch agentProfileForRun(&run) {
			case agentprofile.Researcher:
			default:
				return fmt.Errorf("texture run %s directly delegated to disallowed %s run %s", parent.RunID, agentProfileForRun(&run), run.RunID)
			}
		}
	}
	return nil
}

func workerUpdatesForTextureDoc(updates []types.CoagentSourcePacket, docID string) []types.CoagentSourcePacket {
	out := []types.CoagentSourcePacket{}
	for _, update := range updates {
		if textureAgentIDMatchesDoc(update.TargetAgentID, docID) && update.ChannelID == docID && update.MessageSeq > 0 {
			out = append(out, update)
		}
	}
	return out
}

func verifyPersistentSuperPath(ownerID string, runs []types.RunRecord) error {
	wantAgentID := persistentSuperAgentID(ownerID)
	for _, run := range runs {
		if agentProfileForRun(&run) != agentprofile.Super {
			continue
		}
		if run.AgentID != wantAgentID {
			return fmt.Errorf("super run %s used agent_id %q, want persistent %q", run.RunID, run.AgentID, wantAgentID)
		}
		if metadataStringValue(run.Metadata, "request_source") != "update_coagent" {
			return fmt.Errorf("super run %s request_source = %q, want update_coagent", run.RunID, metadataStringValue(run.Metadata, "request_source"))
		}
		if metadataStringValue(run.Metadata, "requested_by_profile") == agentprofile.Texture {
			return nil
		}
	}
	return fmt.Errorf("missing persistent super inbox run requested by texture")
}

func verifyCoSuperParents(runs []types.RunRecord) error {
	runByID := make(map[string]types.RunRecord, len(runs))
	coSuperCount := 0
	for _, run := range runs {
		runByID[run.RunID] = run
	}
	for _, run := range runs {
		if agentProfileForRun(&run) != agentprofile.CoSuper {
			continue
		}
		coSuperCount++
		parent, ok := runByID[run.RequestedByRunID]
		parentProfile := agentProfileForRun(&parent)
		if !ok || (parentProfile != agentprofile.Super && parentProfile != agentprofile.VSuper) {
			return fmt.Errorf("co-super run %s parent profile = %q, want super or vsuper", run.RunID, parentProfile)
		}
	}
	if coSuperCount == 0 {
		return fmt.Errorf("missing co-super run")
	}
	return nil
}

func verifyWorkerRunToolCausality(runs []types.RunRecord, events []types.EventRecord) error {
	runByID := make(map[string]types.RunRecord, len(runs))
	for _, run := range runs {
		runByID[run.RunID] = run
	}
	for _, run := range runs {
		if strings.TrimSpace(run.RequestedByRunID) == "" {
			continue
		}
		parent, ok := runByID[run.RequestedByRunID]
		if !ok {
			continue
		}
		parentProfile := agentProfileForRun(&parent)
		childProfile := agentProfileForRun(&run)
		switch {
		case childProfile == agentprofile.Texture:
			if !isTextureAgentRevisionTaskType(metadataStringValue(run.Metadata, "type")) {
				return fmt.Errorf("child Texture run %s is not a Texture agent revision", run.RunID)
			}
		case parentProfile == agentprofile.Texture && childProfile == agentprofile.Researcher:
			if !toolResultOutputLoopID(events, parent.RunID, "spawn_agent", run.RunID) {
				return fmt.Errorf("researcher run %s lacks parent texture spawn_agent result", run.RunID)
			}
		case parentProfile == agentprofile.Super && (childProfile == agentprofile.CoSuper || childProfile == agentprofile.Researcher):
			if !toolResultOutputLoopID(events, parent.RunID, "spawn_agent", run.RunID) {
				return fmt.Errorf("%s run %s lacks parent super spawn_agent result", childProfile, run.RunID)
			}
		case parentProfile == agentprofile.VSuper && (childProfile == agentprofile.CoSuper || childProfile == agentprofile.Researcher):
			if !toolResultOutputLoopID(events, parent.RunID, "spawn_agent", run.RunID) {
				return fmt.Errorf("%s run %s lacks parent vsuper spawn_agent result", childProfile, run.RunID)
			}
		case parentProfile == agentprofile.Conductor && childProfile == agentprofile.Texture:
			// Initial texture setup is product orchestration, not worker delegation.
		default:
			return fmt.Errorf("run %s (%s) has unsupported parent %s (%s)", run.RunID, childProfile, parent.RunID, parentProfile)
		}
	}
	return nil
}

func eventsContainAnySuccessfulTool(events []types.EventRecord, tools ...string) bool {
	want := map[string]bool{}
	for _, tool := range tools {
		want[tool] = true
	}
	for _, ev := range events {
		if ev.Kind != types.EventToolResult {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal(ev.Payload, &payload); err != nil {
			continue
		}
		tool, _ := payload["tool"].(string)
		isError, _ := payload["is_error"].(bool)
		if want[tool] && !isError {
			return true
		}
	}
	return false
}

func eventsContainSuccessfulWebSearch(events []types.EventRecord) bool {
	for _, payload := range successfulToolResultPayloads(events, "web_search") {
		var output struct {
			Provider string           `json:"provider"`
			Results  []map[string]any `json:"results"`
		}
		if err := json.Unmarshal([]byte(toolPayloadOutput(payload)), &output); err == nil &&
			strings.TrimSpace(output.Provider) != "" && len(output.Results) > 0 {
			return true
		}
	}
	return false
}

func eventsContainSuccessfulBashVerification(events []types.EventRecord) bool {
	for _, payload := range successfulToolResultPayloads(events, "bash") {
		var output struct {
			Command  string `json:"command"`
			ExitCode int    `json:"exit_code"`
		}
		if err := json.Unmarshal([]byte(toolPayloadOutput(payload)), &output); err == nil &&
			strings.TrimSpace(output.Command) != "" && output.ExitCode == 0 {
			return true
		}
	}
	return false
}

func verifyArtifactWritesCoverWorkerUpdates(events []types.EventRecord, updates []types.CoagentSourcePacket) error {
	artifacts := workerUpdateArtifacts(updates)
	if len(artifacts) == 0 {
		return fmt.Errorf("artifact write required but no structured worker artifact was reported")
	}
	written := []string{}
	for _, tool := range []string{"write_file", "edit_file"} {
		for _, payload := range successfulToolResultPayloads(events, tool) {
			var output struct {
				Path string `json:"path"`
			}
			if err := json.Unmarshal([]byte(toolPayloadOutput(payload)), &output); err == nil && strings.TrimSpace(output.Path) != "" {
				written = append(written, output.Path)
			}
		}
	}
	for _, artifact := range artifacts {
		for _, path := range written {
			if pathMatchesArtifact(path, artifact) {
				return nil
			}
		}
	}
	return fmt.Errorf("successful file write paths %v do not match reported artifacts %v", written, artifacts)
}

func verifyBashCoversWorkerUpdateTests(events []types.EventRecord, updates []types.CoagentSourcePacket) error {
	tests := workerUpdateTests(updates)
	artifacts := workerUpdateArtifacts(updates)
	for _, payload := range successfulToolResultPayloads(events, "bash") {
		var output struct {
			Command  string `json:"command"`
			ExitCode int    `json:"exit_code"`
		}
		if err := json.Unmarshal([]byte(toolPayloadOutput(payload)), &output); err != nil || output.ExitCode != 0 {
			continue
		}
		command := strings.TrimSpace(output.Command)
		for _, test := range tests {
			if test != "" && strings.Contains(command, test) {
				return nil
			}
		}
		for _, artifact := range artifacts {
			if artifact != "" && strings.Contains(command, artifact) {
				return nil
			}
		}
	}
	if len(tests) == 0 && len(artifacts) == 0 {
		return nil
	}
	return fmt.Errorf("successful bash commands do not cover reported tests/artifacts")
}

func workerUpdateArtifacts(updates []types.CoagentSourcePacket) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, update := range updates {
		for _, artifact := range coagentPacketSourceURIs(update.Packet, "file_artifact", "patch", "screenshot", "video_artifact", "benchmark_log") {
			artifact = strings.TrimSpace(filepathSlash(artifact))
			if key, value := splitTypedWorkerUpdateRef(artifact); key != "" {
				artifact = value
			}
			if artifact != "" && !seen[artifact] {
				seen[artifact] = true
				out = append(out, artifact)
			}
		}
	}
	return out
}

func workerUpdateTests(updates []types.CoagentSourcePacket) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, update := range updates {
		for _, test := range coagentPacketSourceURIs(update.Packet, "test_run") {
			test = strings.TrimSpace(test)
			if key, value := splitTypedWorkerUpdateRef(test); key != "" {
				test = value
			}
			if test != "" && !seen[test] {
				seen[test] = true
				out = append(out, test)
			}
		}
	}
	return out
}

func pathMatchesArtifact(path, artifact string) bool {
	path = filepathSlash(strings.TrimSpace(path))
	artifact = filepathSlash(strings.TrimSpace(artifact))
	return path == artifact || strings.HasSuffix(path, "/"+artifact)
}

func filepathSlash(path string) string {
	return strings.ReplaceAll(path, "\\", "/")
}

func successfulToolResultPayloads(events []types.EventRecord, tool string) []map[string]any {
	var out []map[string]any
	for _, ev := range events {
		if ev.Kind != types.EventToolResult {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal(ev.Payload, &payload); err != nil {
			continue
		}
		got, _ := payload["tool"].(string)
		isError, _ := payload["is_error"].(bool)
		if got == tool && !isError {
			out = append(out, payload)
		}
	}
	return out
}

func successfulToolResultPayloadsForRun(events []types.EventRecord, runID, tool string) []map[string]any {
	var out []map[string]any
	for _, ev := range events {
		if ev.RunID != runID || ev.Kind != types.EventToolResult {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal(ev.Payload, &payload); err != nil {
			continue
		}
		got, _ := payload["tool"].(string)
		isError, _ := payload["is_error"].(bool)
		if got == tool && !isError {
			out = append(out, payload)
		}
	}
	return out
}

func toolPayloadOutput(payload map[string]any) string {
	out, _ := payload["output"].(string)
	return out
}

func toolResultOutputLoopID(events []types.EventRecord, runID, tool, loopID string) bool {
	for _, payload := range successfulToolResultPayloadsForRun(events, runID, tool) {
		var output struct {
			RunID string `json:"loop_id"`
		}
		if err := json.Unmarshal([]byte(toolPayloadOutput(payload)), &output); err == nil && output.RunID == loopID {
			return true
		}
	}
	return false
}

func verifyTextureRevisionCausality(revisions []types.Revision, events []types.EventRecord, updates []types.CoagentSourcePacket, requireWorkerConsumption bool) error {
	if len(revisions) == 0 {
		return fmt.Errorf("texture document has no revisions")
	}
	revisionByID := make(map[string]types.Revision, len(revisions))
	for _, revision := range revisions {
		revisionByID[revision.RevisionID] = revision
	}
	for _, revision := range revisions {
		if revision.ParentRevisionID != "" {
			if _, ok := revisionByID[revision.ParentRevisionID]; !ok {
				return fmt.Errorf("revision %s parent %s is missing", revision.RevisionID, revision.ParentRevisionID)
			}
		}
		if revision.AuthorKind != types.AuthorAppAgent {
			continue
		}
		meta := decodeRevisionMetadata(revision.Metadata)
		source := metadataString(meta, "source")
		if !isTextureWriteToolName(source) {
			return fmt.Errorf("appagent revision %s source = %q, want Texture write tool", revision.RevisionID, source)
		}
		loopID := metadataString(meta, "loop_id")
		if loopID == "" || len(successfulToolResultPayloadsForRun(events, loopID, source)) == 0 {
			return fmt.Errorf("appagent revision %s missing successful %s tool result for loop %q", revision.RevisionID, source, loopID)
		}
	}
	if requireWorkerConsumption {
		needed := map[int64]bool{}
		for _, update := range updates {
			if update.MessageSeq > 0 {
				needed[update.MessageSeq] = false
			}
		}
		for _, revision := range revisions {
			meta := decodeRevisionMetadata(revision.Metadata)
			for _, seq := range consumedWorkerSeqs(meta) {
				if _, ok := needed[seq]; ok {
					needed[seq] = true
				}
			}
		}
		for seq, found := range needed {
			if !found {
				return fmt.Errorf("worker update message seq %d was not consumed by a Texture revision", seq)
			}
		}
	}
	return nil
}

func consumedWorkerSeqs(meta map[string]any) []int64 {
	raw, _ := meta["worker_updates_consumed"].([]any)
	seqs := []int64{}
	for _, item := range raw {
		entry, _ := item.(map[string]any)
		switch seq := entry["seq"].(type) {
		case float64:
			seqs = append(seqs, int64(seq))
		case int64:
			seqs = append(seqs, seq)
		case int:
			seqs = append(seqs, int64(seq))
		}
	}
	return seqs
}
