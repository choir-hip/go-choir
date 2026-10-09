package agentcore

import (
	"context"
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/coagentpacket"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/vocabmigrate"
)

// SMG (2026-10-06, owner directive): the typed JSON tools are deleted.
// Persistent Management reports and assignment cancellation are in-cell
// choir.* verbs committed through the record-native reducer; the binding
// validations below moved host-side onto those verb paths. The registration
// entry points are gone — buildDeskCellRegistry installs desk_go_eval only.

// requirePersistentManagementExecution asserts the caller is the exact
// non-lifecycle persistent Management run (SMG). Kept as the shared gate for
// the verb carriers; the typed-tool registration is deleted.
func requirePersistentManagementRunRecord(rec *types.RunRecord) error {
	if rec == nil || rec.AgentID != persistentManagementAgentID(rec.OwnerID) || rec.AgentProfile != agentprofile.Management || rec.AgentRole != agentprofile.Management || rec.TrajectoryID != "" {
		return fmt.Errorf("assigned Engineering verbs require the exact non-lifecycle persistent Management")
	}
	return nil
}

func requirePersistentManagementExecution(ctx context.Context) (*types.RunRecord, error) {
	execution := toolregistry.ExecutionContextFrom(ctx)
	rec := execution.RunRecord
	if err := requirePersistentManagementRunRecord(rec); err != nil {
		return nil, err
	}
	return rec, nil
}

// cancelAssignedEngineeringForRun is the verb carrier for
// cancel_co_super_assignment (SMG): the reducer invokes it for
// choir.CancelAssignment intents after the persistent-management gate.
// Validation is byte-for-byte the deleted tool's: exact persistent
// Management run, assignment id + reason, executor-acknowledged durable
// revoke via rt.cancelAssignedEngineering.
func (rt *Runtime) cancelAssignedEngineeringForRun(ctx context.Context, parent types.RunRecord, assignmentID, reason string) (types.EngineeringAssignmentCommandResult, types.EngineeringAssignment, error) {
	if err := requirePersistentManagementRunRecord(&parent); err != nil {
		return types.EngineeringAssignmentCommandResult{}, types.EngineeringAssignment{}, err
	}
	result, err := rt.cancelAssignedEngineering(ctx, parent, strings.TrimSpace(assignmentID), 1, reason)
	if err != nil {
		return types.EngineeringAssignmentCommandResult{}, types.EngineeringAssignment{}, err
	}
	assignment := result.Assignment
	if !result.Replay && result.Update != nil {
		rt.wakeUpdatedCoagent(ctx, *result.Update)
	}
	return result, assignment, nil
}

// cancel_co_super_assignment's typed-tool registration is deleted (SMG); the
// cell verb choir.CancelAssignment reaches cancelAssignedEngineeringForRun
// through the record-native reducer.

func rejectReportAuthorityInputs(actions []types.CoagentPacketAction) error {
	forbidden := map[string]bool{
		"ownerid": true, "computerid": true, "trajectoryid": true, "agentid": true, "targetagentid": true,
		"sourcerunid": true, "runid": true, "loopid": true, "workitemid": true, "targetworkitemid": true,
		"producerworkitemid": true, "controlbindingid": true, "controlid": true, "updateid": true,
		"producerupdateid": true, "channelid": true,
	}
	canonicalKey := func(key string) string {
		return strings.Map(func(r rune) rune {
			if r >= 'A' && r <= 'Z' {
				return r + ('a' - 'A')
			}
			if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
				return r
			}
			return -1
		}, strings.TrimSpace(key))
	}
	var walk func(any) error
	walk = func(value any) error {
		switch typed := value.(type) {
		case map[string]any:
			for key, nested := range typed {
				if forbidden[canonicalKey(key)] {
					return fmt.Errorf("report_to_texture action inputs cannot author lifecycle authority key %q", key)
				}
				if err := walk(nested); err != nil {
					return err
				}
			}
		case []any:
			for _, nested := range typed {
				if err := walk(nested); err != nil {
					return err
				}
			}
		}
		return nil
	}
	for _, action := range actions {
		if err := walk(action.Inputs); err != nil {
			return err
		}
	}
	return nil
}

// persistentManagementBoundReport is the verb carrier for the deleted
// report_to_texture tool (SMG 2026-10-06). The record-native reducer calls it
// for a persistent-Management run's choir.ReportPacket: the staged packet is
// already decoded + validated; workDisposition comes from packet
// work_disposition ("" = open). Validation is byte-for-byte the deleted
// tool's: exact persistent-Management run, one delivered lifecycle control
// trajectory, all delivered controls scoped to one trajectory/work/Texture
// source, an authenticated (run-memory-seen) control binding, and exactly
// one bound work item on the Texture source run.
func (rt *Runtime) persistentManagementBoundReport(ctx context.Context, parent types.RunRecord, packet types.CoagentSourcePacketPayload, workDisposition types.WorkItemStatus) (types.LifecycleResult, error) {
	if err := requirePersistentManagementRunRecord(&parent); err != nil {
		return types.LifecycleResult{}, err
	}
	if err := rejectReportAuthorityInputs(packet.Actions); err != nil {
		return types.LifecycleResult{}, err
	}
	packet = coagentpacket.Normalize(packet)
	if err := coagentpacket.Validate(packet); err != nil {
		return types.LifecycleResult{}, err
	}
	if workDisposition == "" {
		workDisposition = types.WorkItemOpen
	}
	if workDisposition != types.WorkItemOpen && workDisposition != types.WorkItemCompleted {
		return types.LifecycleResult{}, fmt.Errorf("report work_disposition must be open or completed")
	}
	trajectoryID := lifecycleControlTrajectoryForRun(&parent)
	// Drain carriers mint via ResolvePersistentManagementLiveOccurrence →
	// reconcile with request_source=update_coagent and a backlog of
	// mixed-trajectory packets; their run TrajectoryID is the mint-defaulted
	// runID and assignment_trajectory_id is absent. Resolve the report's
	// trajectory from the delivered control set instead — the carrier may
	// report one trajectory's worth of controls at a time.
	isDrainCarrier := metadataStringValue(parent.Metadata, "request_source") == "update_coagent"
	if isDrainCarrier {
		trajectoryID = ""
	}
	if trajectoryID == "" {
		if isDrainCarrier {
			// Pick the trajectory of the most recent delivered control; the
			// carrier iterates the backlog and reports each trajectory's set
			// separately.
			deliveredForTraj, trajErr := rt.listAllLifecyclePacketsDeliveredToRun(ctx, &parent)
			if trajErr != nil {
				return types.LifecycleResult{}, trajErr
			}
			for _, p := range deliveredForTraj {
				if p.Direction == types.LifecyclePacketDirectionControl && strings.TrimSpace(p.TrajectoryID) != "" {
					trajectoryID = p.TrajectoryID
				}
			}
		}
		if trajectoryID == "" {
			return types.LifecycleResult{}, fmt.Errorf("report requires one exact delivered lifecycle control trajectory")
		}
	}
	trajectory, err := rt.store.GetLifecycleTrajectory(ctx, parent.OwnerID, parent.ComputerID, trajectoryID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	var delivered []types.CoagentSourcePacket
	if trajectory.Status == types.TrajectoryLive {
		delivered, err = rt.listAllLifecyclePacketsDeliveredToRun(ctx, &parent)
	} else {
		delivered, err = rt.store.ListHistoricalLifecycleControlsDeliveredToRun(ctx, parent.OwnerID, parent.ComputerID, trajectoryID, parent.AgentID, parent.RunID)
	}
	if err != nil {
		return types.LifecycleResult{}, err
	}
	// Drain carriers bind packets across trajectories; a report resolves one
	// trajectory at a time. Restrict the consumable set to the resolved
	// trajectory so the homogeneity check below sees a single-trajectory
	// window.
	if isDrainCarrier {
		filtered := delivered[:0]
		for _, p := range delivered {
			if strings.TrimSpace(p.TrajectoryID) == trajectoryID {
				filtered = append(filtered, p)
			}
		}
		delivered = filtered
	}
	memoryEntries, err := rt.store.ListRunMemoryEntries(ctx, parent.OwnerID, parent.RunID)
	if err != nil {
		return types.LifecycleResult{}, fmt.Errorf("report load durable run memory: %w", err)
	}
	memorySeen, _, _ := lifecycleInjectionIDsFromRunMemory(&parent, memoryEntries)
	authenticatedDelivered := make([]types.CoagentSourcePacket, 0, len(delivered))
	consumedDeliveryIDs := make([]string, 0, len(delivered))
	allControls := make([]types.CoagentSourcePacket, 0, len(delivered))
	for _, deliveredPacket := range delivered {
		if deliveredPacket.Direction == types.LifecyclePacketDirectionControl {
			allControls = append(allControls, deliveredPacket)
		}
		if !memorySeen[strings.TrimSpace(deliveredPacket.UpdateID)] {
			// Delivery can commit while a provider call is already in flight.
			// It remains pending until the next authenticated runtime append.
			continue
		}
		authenticatedDelivered = append(authenticatedDelivered, deliveredPacket)
		consumedDeliveryIDs = append(consumedDeliveryIDs, deliveredPacket.UpdateID)
	}
	if len(allControls) == 0 {
		return types.LifecycleResult{}, fmt.Errorf("report requires an exact delivered control binding")
	}
	scope := allControls[0]
	if scope.TargetWorkItemID == "" || scope.AgentID == "" || scope.SourceRunID == "" {
		return types.LifecycleResult{}, fmt.Errorf("report control binding is incomplete")
	}
	// Validate every downward control across every page before selecting
	// report authority; an unauthenticated later arrival cannot conceal a
	// cross-trajectory/work/Texture-source corruption.
	for _, candidate := range allControls[1:] {
		if candidate.TrajectoryID != scope.TrajectoryID || candidate.TargetWorkItemID != scope.TargetWorkItemID ||
			candidate.AgentID != scope.AgentID || candidate.SourceRunID != scope.SourceRunID || candidate.ChannelID != scope.ChannelID {
			return types.LifecycleResult{}, fmt.Errorf("report delivered controls span more than one trajectory/work/Texture scope")
		}
	}
	controls := make([]types.CoagentSourcePacket, 0, len(authenticatedDelivered))
	for _, deliveredPacket := range authenticatedDelivered {
		if deliveredPacket.Direction == types.LifecyclePacketDirectionControl {
			controls = append(controls, deliveredPacket)
		}
	}
	if len(controls) == 0 {
		return types.LifecycleResult{}, fmt.Errorf("report requires an authenticated delivered control binding")
	}
	control := controls[0]
	for _, candidate := range controls[1:] {
		// MessageSeq is the immutable occurrence order. ReducerSeq advances
		// again when a partial report incorporates an older delivery.
		if candidate.MessageSeq > control.MessageSeq || (candidate.MessageSeq == control.MessageSeq && candidate.UpdateID > control.UpdateID) {
			control = candidate
		}
	}
	targetRun, err := rt.store.GetLifecycleRun(ctx, parent.OwnerID, parent.ComputerID, control.SourceRunID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	targetWorkSet := lifecycleControlWorkIDsForRun(&targetRun)
	if len(targetWorkSet) != 1 {
		return types.LifecycleResult{}, fmt.Errorf("report Texture source run must bind exactly one lifecycle work item")
	}
	targetWorkID := ""
	for id := range targetWorkSet {
		targetWorkID = id
	}
	execution := toolregistry.ExecutionContextFrom(ctx)
	if strings.TrimSpace(execution.ToolCallID) == "" {
		return types.LifecycleResult{}, fmt.Errorf("report requires authenticated provider tool-call identity")
	}
	occurrence := objectgraph.SHA256([]byte(strings.Join([]string{vocabmigrate.IdentitySeedPersistentSuperReportV1, parent.OwnerID, parent.ComputerID, parent.RunID, execution.ToolCallID}, "\x00")))
	producerUpdateID := "super-report:" + occurrence
	content := strings.TrimSpace(packet.Summary)
	payloadDigest, err := store.ComputeLifecycleUpdatePayloadDigest(packet, content)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	var consumedForReport []string
	if trajectory.Status == types.TrajectoryLive {
		consumedForReport = consumedDeliveryIDs
	}
	req := types.QueueLifecycleUpdateRequest{
		OwnerID: parent.OwnerID, ComputerID: parent.ComputerID, CommandID: "queue-" + producerUpdateID,
		TrajectoryID: trajectoryID, TargetAgentID: control.AgentID, ProducerAgentID: parent.AgentID,
		ControlBindingID: control.UpdateID, TargetWorkItemID: targetWorkID,
		ConsumedDeliveryUpdateIDs: consumedForReport,
		ProducerUpdateID:          producerUpdateID, UpdateID: "result:" + occurrence,
		ChannelID: control.ChannelID, Role: agentprofile.Management, SourceRunID: parent.RunID,
		Packet: packet, Content: content, WorkDisposition: workDisposition,
		WorkItemID: control.TargetWorkItemID, PayloadDigest: payloadDigest,
	}
	req.CommandDigest, err = store.ComputeQueuePersistentManagementReportDigest(req)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	queued, err := rt.store.QueueLifecycleUpdate(ctx, req)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	if !queued.Replay && queued.Update != nil && queued.Update.Disposition == types.UpdatePending {
		rt.wakeUpdatedCoagent(ctx, *queued.Update)
	}
	return queued, nil
}
