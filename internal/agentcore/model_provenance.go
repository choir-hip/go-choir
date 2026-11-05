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

// modelProvenanceMaxRuns bounds the lineage read: a document whose lineage
// passes it reads its newest runs only, and says so through Runs.
const modelProvenanceMaxRuns = 400

// DocumentModelProvenance returns the served models of a document's runs.
// The lineage is every run on the document's channel or one of its
// trajectories (Texture, the research desk, engineering casts), every run
// those runs requested (children), and the run that requested an
// engineering run of the lineage (the management turn that cast it). The
// caller declares the reader (the owner route does).
func (rt *Runtime) DocumentModelProvenance(ctx context.Context, ownerID, docID string, trajectoryIDs []string) (types.ModelProvenance, error) {
	if rt == nil || rt.store == nil {
		return types.ModelProvenance{}, fmt.Errorf("model provenance: runtime store unavailable")
	}
	runs, err := rt.documentLineageRuns(ctx, ownerID, docID, trajectoryIDs)
	if err != nil {
		return types.ModelProvenance{}, err
	}
	var b types.ModelProvenanceBuilder
	for _, run := range runs {
		role := modelProvenanceRole(&run)
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
		if reports, err := rt.store.ListEngineeringAssignmentReportsByRun(ctx, ownerID, run.RunID); err == nil {
			for _, report := range reports {
				if use := report.DelegateUse; use != nil && use.Calls > 0 {
					b.AddDelegate(run.RunID, role, firstNonEmpty(use.Model, types.ModelProviderClaudeDelegate), use.Calls)
				}
			}
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
// indexed run metadata (channel, trajectory, requesting run), never a scan
// of the owner's runs: at most modelProvenanceMaxRuns, newest first.
func (rt *Runtime) documentLineageRuns(ctx context.Context, ownerID, docID string, trajectoryIDs []string) ([]types.RunRecord, error) {
	computerID := strings.TrimSpace(rt.cfg.ComputerID)
	byID := map[string]types.RunRecord{}
	var queue []string
	add := func(runs []types.RunRecord) {
		for _, run := range runs {
			if _, seen := byID[run.RunID]; seen || len(byID) >= modelProvenanceMaxRuns {
				continue
			}
			byID[run.RunID] = run
			queue = append(queue, run.RunID)
		}
	}
	read := func(field, value string) error {
		runs, err := rt.store.ListRunsByLineageField(ctx, ownerID, computerID, field, value)
		if err != nil {
			return fmt.Errorf("model provenance: runs by %s: %w", field, err)
		}
		add(runs)
		return nil
	}
	if docID = strings.TrimSpace(docID); docID != "" {
		if err := read("channel_id", docID); err != nil {
			return nil, err
		}
	}
	seenTrajectory := map[string]bool{}
	for _, id := range trajectoryIDs {
		if id = strings.TrimSpace(id); id != "" && !seenTrajectory[id] {
			seenTrajectory[id] = true
			if err := read("trajectory_id", id); err != nil {
				return nil, err
			}
		}
	}
	// Every run a lineage run requested, transitively (research children,
	// verifier runs).
	for i := 0; i < len(queue); i++ {
		if err := read("requested_by_run_id", queue[i]); err != nil {
			return nil, err
		}
	}
	// The management turn that cast an engineering run of the lineage. It is
	// added after the walk: its other children serve other documents.
	var casters []string
	for _, run := range byID {
		if parent := strings.TrimSpace(run.RequestedByRunID); parent != "" && metadataStringValue(run.Metadata, "assignment_id") != "" {
			if _, in := byID[parent]; !in {
				casters = append(casters, parent)
			}
		}
	}
	for _, id := range casters {
		if _, in := byID[id]; in || len(byID) >= modelProvenanceMaxRuns {
			continue
		}
		if run, err := rt.getRunForComputer(ctx, ownerID, id); err == nil {
			byID[id] = run
		}
	}
	out := make([]types.RunRecord, 0, len(byID))
	for _, run := range byID {
		out = append(out, run)
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].CreatedAt.After(out[j].CreatedAt)
		}
		return out[i].RunID < out[j].RunID
	})
	return out, nil
}
