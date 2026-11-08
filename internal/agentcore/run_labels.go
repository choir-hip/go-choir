package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// Provenance labels that propagate (MA3.5, R15;
// docs/design/modular-architecture-plan-2026-11-04.md section 7.2).
//
// A run carries the union of the labels of everything it read. The union
// lives in a per-run accumulator on the host (Runtime.runLabels): a cell
// never holds it, cannot read it and cannot change it. The reducer stamps
// every record the run's cells commit with the accumulator's set
// (stampRequestRecord), and the host stamps an object a cell writes through
// a host verb the same way. A set only grows, so a derived value, which is
// written by a run that read its inputs, carries the union of its inputs'
// labels.
//
// Read points that feed the accumulator today:
//   - deskEgress (the durable record is the source of truth; the cache is
//     a view of it, flushed at each reducer commit): every host-mediated choir.* call of a desk cell that
//     returns outside content (Read, PollSource, SourceAPI, SearchStart,
//     SearchWireCorpus, SearchTexture, Research, ReadCapture, ReadEvidence, ListEvidence,
//     ProductAPI, Stories);
//   - the records a cell was shown through choir.Records(), by the labels
//     their provenance carries (cellRecords.labelsThrough);
//   - the Texture document a Texture cell's frame showed.

// runLabelAccumulator is one run's label union.
type runLabelAccumulator struct {
	mu  sync.Mutex
	set types.LabelSet
	// untrustedReads counts the reads that carried an untrusted label, so a
	// cell can tell whether it read outside content (runUntrustedReads).
	untrustedReads uint64
}

// taintRun folds labels into the run's accumulator. A run with no id has
// none to taint.
func (rt *Runtime) taintRun(runID string, labels ...types.LabelSet) {
	rt.foldRunLabels(runID, true, labels...)
}

// seedRunLabels folds labels the run inherits (its session's) into the
// accumulator. It is not a read, so it does not move the untrusted-read count
// that decides which cell's output is framed.
func (rt *Runtime) seedRunLabels(runID string, labels ...types.LabelSet) {
	rt.foldRunLabels(runID, false, labels...)
}

func (rt *Runtime) foldRunLabels(runID string, read bool, labels ...types.LabelSet) {
	runID = strings.TrimSpace(runID)
	if rt == nil || runID == "" {
		return
	}
	add := types.LabelSet(nil).Union(labels...)
	if len(add) == 0 {
		return
	}
	v, _ := rt.runLabels.LoadOrStore(runID, &runLabelAccumulator{})
	acc := v.(*runLabelAccumulator)
	acc.mu.Lock()
	acc.set = acc.set.Union(add)
	if read && add.Untrusted() {
		acc.untrustedReads++
	}
	acc.mu.Unlock()
}

// runUntrustedReads is how many reads of outside content the run has made.
// A cell that finds it moved when the cell ends read outside content.
func (rt *Runtime) runUntrustedReads(runID string) uint64 {
	runID = strings.TrimSpace(runID)
	if rt == nil || runID == "" {
		return 0
	}
	v, ok := rt.runLabels.Load(runID)
	if !ok {
		return 0
	}
	acc := v.(*runLabelAccumulator)
	acc.mu.Lock()
	defer acc.mu.Unlock()
	return acc.untrustedReads
}

// runLabelSet is the union of the labels of everything the run has read.
func (rt *Runtime) runLabelSet(runID string) types.LabelSet {
	runID = strings.TrimSpace(runID)
	if rt == nil || runID == "" {
		return nil
	}
	v, ok := rt.runLabels.Load(runID)
	if !ok {
		return nil
	}
	acc := v.(*runLabelAccumulator)
	acc.mu.Lock()
	defer acc.mu.Unlock()
	return acc.set
}

// storedRunLabels is the label set recorded on the run record, the source of
// truth (a planned-restart resume reactivates the same run with an empty
// cache). A run with no record has none.
func (rt *Runtime) storedRunLabels(ctx context.Context, ownerID, computerID, runID string) types.LabelSet {
	if rt == nil || rt.store == nil || strings.TrimSpace(ownerID) == "" || strings.TrimSpace(runID) == "" {
		return nil
	}
	set, err := rt.store.RunLabels(ctx, ownerID, firstNonEmpty(computerID, rt.cfg.ComputerID), runID)
	if err != nil {
		return nil
	}
	return set
}

// runLabelsOf is the labels of a run from the record and the cache, without
// writing: for a run that has ended, or a read of another run's labels.
func (rt *Runtime) runLabelsOf(ctx context.Context, ownerID, computerID, runID string) types.LabelSet {
	return rt.runLabelSet(runID).Union(rt.storedRunLabels(ctx, ownerID, computerID, runID))
}

// flushRunLabels persists the cache's labels on the run record and returns
// the set the record now holds, the cache's and the record's union. A
// reducer commit calls it before it stamps, so an act never carries a label
// the run record lacks. A run with no record has nowhere to persist to and
// keeps the cache's set.
func (rt *Runtime) flushRunLabels(ctx context.Context, ownerID, computerID, runID string) (types.LabelSet, error) {
	mem := rt.runLabelSet(runID)
	if rt == nil || rt.store == nil || strings.TrimSpace(ownerID) == "" || strings.TrimSpace(runID) == "" {
		return mem, nil
	}
	merged, err := rt.store.UnionRunLabels(ctx, ownerID, firstNonEmpty(computerID, rt.cfg.ComputerID), runID, mem)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return mem, nil
		}
		return nil, err
	}
	rt.taintRun(runID, merged)
	return merged.Union(mem), nil
}

// dropRunLabels forgets an ended run's accumulator.
func (rt *Runtime) dropRunLabels(runID string) {
	if rt != nil {
		rt.runLabels.Delete(strings.TrimSpace(runID))
	}
}

// Labels of the objects a read point returns.
var (
	// captureReadLabels: a capture is the owner's stored copy of outside
	// text.
	captureReadLabels = types.NewLabelSet(types.OwnerPrivateLabel("capture"), types.LabelWorldUntrusted)
	// worldReadLabels: text fetched from outside the computer.
	worldReadLabels = types.NewLabelSet(types.LabelWorldUntrusted)
)

// egressReadLabels is the label set an egress action's result carries, and
// whether the action reads at all. result is the host's own reply, read for
// the labels a stored object already carries.
func egressReadLabels(action yaegikernel.BrokerAction, payload, result json.RawMessage) (types.LabelSet, bool) {
	switch action {
	case yaegikernel.ActionReadCapture:
		// The floor every capture read carries, plus what the capture record
		// itself is labelled (MA3.3): the host reads it from the record.
		return captureReadLabels.Union(captureRecordLabels(result)), true
	case yaegikernel.ActionTextureSearch:
		// The Texture custodian's guarded search (MA4.6): the floor is its
		// own label, plus every owner-private label a hit carries, so the
		// disclosure gate sees owner-private Texture text.
		var body struct {
			Results []struct {
				Label string `json:"label"`
			} `json:"results"`
		}
		_ = json.Unmarshal(result, &body)
		labels := researchTextureLabels
		for _, r := range body.Results {
			if strings.HasPrefix(r.Label, types.LabelOwnerPrivatePrefix) {
				labels = labels.Union(types.NewLabelSet(r.Label))
			}
		}
		return labels, true
	case yaegikernel.ActionResearch:
		// Each child's Report carries its role's labels: a web child's the
		// world's, a Texture child's the owner's private Texture text
		// (MA4.6). A result the host cannot read is the world's.
		var results []yaegikernel.ResearchChildResult
		_ = json.Unmarshal(result, &results)
		labels := types.LabelSet(nil)
		for _, r := range results {
			if r.Role == researchTextureRole {
				labels = labels.Union(researchTextureLabels)
			} else {
				labels = labels.Union(worldReadLabels)
			}
		}
		if len(labels) == 0 {
			labels = worldReadLabels
		}
		return labels, true
	case yaegikernel.ActionSystem1:
		// The judge reads each capture a question names with the owner's
		// authority; its answers are drawn from that text, so the run is
		// tainted as a capture read is (MA6.3). Passages are the cell's own
		// text and add nothing.
		var req yaegikernel.System1Payload
		_ = json.Unmarshal(payload, &req)
		for _, q := range req.Questions {
			for _, ref := range q.ContextRefs {
				ref = strings.TrimSpace(ref)
				if ref != "" && !strings.HasPrefix(ref, yaegikernel.System1TextRefPrefix) && !strings.HasPrefix(ref, system1HostRefPrefix) {
					return captureReadLabels, true
				}
			}
		}
		return nil, false
	case yaegikernel.ActionRead, yaegikernel.ActionPollSource, yaegikernel.ActionSourceAPI,
		yaegikernel.ActionSearchStart, yaegikernel.ActionSearchWireCorpus:
		return worldReadLabels, true
	case yaegikernel.ActionReadEvidence:
		// An evidence object carries the labels its writer's run held.
		var obj struct {
			Metadata json.RawMessage `json:"metadata"`
		}
		_ = json.Unmarshal(result, &obj)
		return types.NewLabelSet(types.OwnerPrivateLabel("evidence")).Union(types.LabelsFromMetadata(obj.Metadata)), true
	case yaegikernel.ActionObjectGet, yaegikernel.ActionObjectList:
		return objectReadLabels(result), true
	case yaegikernel.ActionListEvidence:
		return types.NewLabelSet(types.OwnerPrivateLabel("evidence")), true
	case yaegikernel.ActionStories:
		return types.NewLabelSet(types.OwnerPrivateLabel("story")), true
	case yaegikernel.ActionProductAPI:
		var p yaegikernel.ProductAPIPayload
		_ = json.Unmarshal(payload, &p)
		return productAPIReadLabels(p.Method, p.Path), true
	}
	return nil, false
}

// productAPIReadLabels labels a desk's product API call by what its answer
// carries, not by its route's segment alone (2026-11-08: a story's Texture
// run that started a Texture through POST /api/prompt-bar was labelled
// prompt-bar, and every paper publish asked). A write answers a receipt and
// reads nothing. A desk's document list is the story index
// (textureowner.listedByDesk). The event index is built from the computer's
// captures. Any other route keeps its segment's label, a POST that answers
// content (model-policy/verify, run-acceptances/synthesize) included (Devin r2).
func productAPIReadLabels(method, path string) types.LabelSet {
	route := ""
	if u, err := url.Parse(strings.TrimSpace(path)); err == nil {
		route = strings.TrimSuffix(u.Path, "/")
	}
	if m := strings.ToUpper(strings.TrimSpace(method)); m != "" && m != http.MethodGet && productAPIReceiptWrites[route] {
		return types.NewLabelSet()
	}
	switch route {
	case "/api/texture/documents":
		return types.NewLabelSet(types.OwnerPrivateLabel("story"))
	case "/api/autopaper/events":
		return captureReadLabels
	}
	return types.NewLabelSet(types.OwnerPrivateLabel(productAPIObjectKind(path)))
}

// productAPIReceiptWrites are the desk writes whose answer is a receipt (a
// submission id and state, or the desk's own write echoed back).
var productAPIReceiptWrites = map[string]bool{
	"/api/prompt-bar":                  true,
	"/api/autopaper/sources/schedules": true,
}

// productAPIObjectKind names the kind a product API route serves: the
// segment after /api/ (documents, stories, ...). An unreadable path is
// "product", never empty, so the read still labels.
func productAPIObjectKind(path string) string {
	path = strings.TrimSpace(path)
	if u, err := url.Parse(path); err == nil {
		path = u.Path
	}
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) >= 2 && parts[0] == "api" && parts[1] != "" {
		return parts[1]
	}
	return "product"
}

// noteEgressRead taints the calling run with what a successful egress call
// returned.
func (rt *Runtime) noteEgressRead(ctx context.Context, exec toolregistry.ExecutionContext, action yaegikernel.BrokerAction, payload, result json.RawMessage) {
	if action == yaegikernel.ActionGetRunMemoryEntry {
		rt.taintRun(exec.RunID, rt.runMemoryReadLabels(ctx, exec, result))
		return
	}
	if labels, reads := egressReadLabels(action, payload, result); reads {
		rt.taintRun(exec.RunID, labels)
	}
}

// runMemoryReadLabels is the label set of a run-memory entry a cell read: the
// durable labels of the run the entry belongs to. An entry whose run records
// none (written before labels) is owner-private and untrusted.
func (rt *Runtime) runMemoryReadLabels(ctx context.Context, exec toolregistry.ExecutionContext, result json.RawMessage) types.LabelSet {
	var entry struct {
		RunID string `json:"loop_id"`
	}
	_ = json.Unmarshal(result, &entry)
	if set := rt.runLabelsOf(ctx, exec.OwnerID, exec.ComputerID, entry.RunID); len(set) > 0 {
		return set
	}
	return types.NewLabelSet(types.OwnerPrivateLabel("run_memory"), types.LabelWorldUntrusted)
}

// labelsOfRecord is the label set of a stored record read by a desk: what
// its provenance carries, plus what its own content is. A captures record
// is the capture read; an external origin is outside text.
func labelsOfRecord(rec types.CommitmentRecord) types.LabelSet {
	set := types.NewLabelSet(rec.Provenance.Labels...)
	if rec.Captures != nil && len(rec.Captures.Items) > 0 {
		set = set.Union(captureReadLabels)
	}
	if rec.Provenance.Origin == types.OriginExternal {
		set = set.Union(worldReadLabels)
	}
	return set
}

// labelsThrough is the union of the labels of the records a cell was shown:
// those in its view at or below the through mark (capToShown has already cut
// the mark to what choir.Records() returned).
func (c cellRecords) labelsThrough() types.LabelSet {
	var set types.LabelSet
	for _, l := range c.labels {
		if l.seq <= c.through {
			set = set.Union(l.set)
		}
	}
	return set
}

// recordLabel is one shown record's labels, at its view sequence.
type recordLabel struct {
	seq int64
	set types.LabelSet
}

// documentReadLabels is the label of the Texture document a cell was shown
// (the kind is the object, not the desk).
var documentReadLabels = types.NewLabelSet(types.OwnerPrivateLabel("document"))
