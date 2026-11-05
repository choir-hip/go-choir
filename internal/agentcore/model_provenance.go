package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// Model provenance of a Texture document
// (docs/problems/model-provenance-unwired-2026-11-05.md): the models that
// actually served the runs of the document's lineage, by desk role, read
// from the runs' model.served lines (the tool loop's record of each
// answered call, after any quota switch or pool failover) and from the
// Claude delegate's use on engineering reports. Never the model policy's
// prediction.

// modelProvenanceMaxRuns bounds the lineage: a document whose channel and
// trajectories hold more runs keeps its newest (by creation time) and says
// so through Runs. The engineering casters are added on top, at most one
// per kept engineering run.
const modelProvenanceMaxRuns = 400

// DocumentModelProvenance returns the served models of a document's runs.
// The lineage is every run on the document's channel or one of its
// trajectories (Texture, the research desk, engineering casts) and the run
// that requested an engineering run of the lineage (the management turn
// that cast it). Research children are not runs: their lines are written
// on their parent run under the research_child role. Every read is indexed
// (channel_id, trajectory_id, run_id, a point lookup), so the cost is
// bounded by the lineage, never by the store. The caller declares the
// reader (the owner route does).
func (rt *Runtime) DocumentModelProvenance(ctx context.Context, ownerID, docID string, trajectoryIDs []string) (types.ModelProvenance, error) {
	if rt == nil || rt.store == nil {
		return types.ModelProvenance{}, fmt.Errorf("model provenance: runtime store unavailable")
	}
	runs, trajectories, err := rt.documentLineageRuns(ctx, ownerID, docID, trajectoryIDs)
	if err != nil {
		return types.ModelProvenance{}, err
	}
	var b types.ModelProvenanceBuilder
	roleOf := map[string]string{}
	for _, run := range runs {
		role := modelProvenanceRole(&run)
		roleOf[run.RunID] = role
		b.AddRun(run.RunID)
		evs, err := rt.store.ListRunEventsOfKind(ctx, ownerID, run.RunID, types.EventModelServed)
		if err != nil {
			return types.ModelProvenance{}, fmt.Errorf("model provenance: served lines of %s: %w", run.RunID, err)
		}
		recorded := false
		for _, ev := range evs {
			var rec types.ModelServedRecord
			if json.Unmarshal(ev.Payload, &rec) != nil || rec.Schema != types.ModelServedSchema {
				continue
			}
			lineRole := role
			if strings.TrimSpace(rec.Role) != "" {
				lineRole = rec.Role
			}
			if rec.SelectedSource == "" && lineRole == role {
				rec.SelectedSource = modelProvenanceSelectionSource(&run)
			}
			b.AddServed(run.RunID, lineRole, rec)
			if lineRole == role && len(rec.Models) > 0 {
				recorded = true
			}
		}
		if !recorded {
			// A run that ended before model.served existed (or never called
			// a model) shows its selection, marked as not recorded.
			if provider, model := metadataStringValue(run.Metadata, "llm_provider"), metadataStringValue(run.Metadata, "llm_model"); model != "" && modelProvenanceRunEnded(&run) {
				b.AddSelectionOnly(run.RunID, role, provider, model, modelProvenanceSelectionSource(&run))
			}
		}
	}
	// The Claude delegate's use, from the engineering reports of the
	// lineage's trajectories, attributed to the run that reported it.
	for _, trajectoryID := range trajectories {
		reports, err := rt.store.ListEngineeringAssignmentReportsByTrajectory(ctx, ownerID, trajectoryID)
		if err != nil {
			return types.ModelProvenance{}, fmt.Errorf("model provenance: engineering reports of %s: %w", trajectoryID, err)
		}
		for _, report := range reports {
			use := report.DelegateUse
			if use == nil || use.Calls <= 0 {
				continue
			}
			role, in := roleOf[report.RunID]
			if !in {
				// The engineering run fell past the cap; its delegate use
				// still counts.
				run, err := rt.getRunForComputer(ctx, ownerID, report.RunID)
				if err != nil {
					continue
				}
				role = modelProvenanceRole(&run)
			}
			b.AddDelegate(report.RunID, role, firstNonEmpty(use.Model, types.ModelProviderClaudeDelegate), use.Calls)
		}
	}
	return b.Build(docID), nil
}

func modelProvenanceRunEnded(run *types.RunRecord) bool {
	return run.State.Terminal() || run.State == types.RunPassivated
}

// modelProvenanceRole is the desk role a run's calls served: its role
// (management_task for a task) else its profile.
func modelProvenanceRole(run *types.RunRecord) string {
	if role := agentRoleForRun(run); strings.TrimSpace(role) != "" {
		return role
	}
	if role := strings.TrimSpace(firstNonEmpty(run.AgentRole, run.AgentProfile)); role != "" {
		return role
	}
	return "unknown"
}

// modelProvenanceSelectionSource names how the run's model was selected:
// the policy source (owner policy, management task pin, ...) and the pool
// assignment.
func modelProvenanceSelectionSource(run *types.RunRecord) string {
	source := metadataStringValue(run.Metadata, "llm_policy_source")
	assignment := metadataStringValue(run.Metadata, "llm_assignment")
	switch {
	case source != "" && assignment != "" && assignment != source:
		return source + "/" + assignment
	case source != "":
		return source
	default:
		return assignment
	}
}

// documentLineageRuns gathers the runs of a document's lineage through the
// indexed run metadata (channel, trajectory) and point lookups of casters,
// never a scan of the owner's runs: the newest modelProvenanceMaxRuns by
// creation time, newest first, and every trajectory they carry.
func (rt *Runtime) documentLineageRuns(ctx context.Context, ownerID, docID string, trajectoryIDs []string) ([]types.RunRecord, []string, error) {
	computerID := strings.TrimSpace(rt.cfg.ComputerID)
	byID := map[string]types.RunRecord{}
	read := func(field, value string) error {
		runs, err := rt.store.ListRunsByLineageField(ctx, ownerID, computerID, field, value)
		if err != nil {
			return fmt.Errorf("model provenance: runs by %s: %w", field, err)
		}
		for _, run := range runs {
			byID[run.RunID] = run
		}
		return nil
	}
	if docID = strings.TrimSpace(docID); docID != "" {
		if err := read("channel_id", docID); err != nil {
			return nil, nil, err
		}
	}
	var trajectories []string
	seenTrajectory := map[string]bool{}
	addTrajectory := func(id string) {
		if id = strings.TrimSpace(id); id != "" && !seenTrajectory[id] {
			seenTrajectory[id] = true
			trajectories = append(trajectories, id)
		}
	}
	for _, id := range trajectoryIDs {
		addTrajectory(id)
	}
	for _, id := range append([]string(nil), trajectories...) {
		if err := read("trajectory_id", id); err != nil {
			return nil, nil, err
		}
	}
	out := make([]types.RunRecord, 0, len(byID))
	for _, run := range byID {
		out = append(out, run)
	}
	sortRunsNewestFirst(out)
	if len(out) > modelProvenanceMaxRuns {
		out = out[:modelProvenanceMaxRuns]
	}
	// The management turn that cast an engineering run of the lineage. Its
	// other children serve other documents, so only it is added.
	kept := map[string]bool{}
	for _, run := range out {
		kept[run.RunID] = true
		addTrajectory(trajectoryIDForRun(&run))
	}
	var casters []types.RunRecord
	for _, run := range out {
		parent := strings.TrimSpace(run.RequestedByRunID)
		if parent == "" || kept[parent] || metadataStringValue(run.Metadata, "assignment_id") == "" {
			continue
		}
		kept[parent] = true
		if caster, err := rt.getRunForComputer(ctx, ownerID, parent); err == nil {
			casters = append(casters, caster)
		}
	}
	out = append(out, casters...)
	sortRunsNewestFirst(out)
	return out, trajectories, nil
}

func sortRunsNewestFirst(runs []types.RunRecord) {
	sort.Slice(runs, func(i, j int) bool {
		if !runs[i].CreatedAt.Equal(runs[j].CreatedAt) {
			return runs[i].CreatedAt.After(runs[j].CreatedAt)
		}
		return runs[i].RunID < runs[j].RunID
	})
}
