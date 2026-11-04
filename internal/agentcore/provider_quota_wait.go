package agentcore

import (
	"context"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/modelpolicy"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Provider quota parks (docs/problems/provider-quota-kills-desk-loops-2026-11-04.md).
//
// A desk loop on a pinned model (or its pool's last member) that meets an
// exhausted provider quota parks in the tool loop (toolregistry,
// WithProviderWait) instead of failing: it keeps its work item and its loop
// state and calls again when the quota is expected to reset. The quota is
// true contention on an external resource, so the park is published as one
// types.WaitState of kind provider_quota:
//
//   - on the run's trace (the tool loop's run.retry "provider_quota" event);
//   - for an engineering assignment, as the assignment's wait
//     (engineering_waits.go): management's pack (ActingPack.Waits) and,
//     once it has lasted waitAnnounceAfter, Texture's line through the
//     engineering progress relay;
//   - for every parked desk loop, in every desk's pack
//     (ActingPack.ProviderWaits) and on the obligations surface, so
//     management and Texture read "waiting for the Synthetic quota to reset
//     (about N min)" rather than a dead loop.
//
// The tracker is memory, like the assignment waits: the park is in-process,
// so a crash ends it with the run (the restart rule closes the run), and a
// planned restart that resumes the run parks it again if the quota has not
// reset.

type providerWaitTracker struct {
	mu      sync.Mutex
	entries map[string]*providerWaitEntry
}

type providerWaitEntry struct {
	ownerID, computerID, runID, agentID string
	wait                                *types.WaitState
	// announce is the pending re-observation that lets an engineering
	// assignment's wait reach Texture once it has lasted waitAnnounceAfter.
	announce *time.Timer
}

func (rt *Runtime) providerWaits() *providerWaitTracker {
	t := &rt.providerQuotaWaits
	t.mu.Lock()
	if t.entries == nil {
		t.entries = map[string]*providerWaitEntry{}
	}
	t.mu.Unlock()
	return t
}

// providerQuotaWaitHook is rec's tool loop's provider wait handler.
func (rt *Runtime) providerQuotaWaitHook(rec *types.RunRecord) toolregistry.ToolLoopProviderWaitFunc {
	if rt == nil || rec == nil {
		return nil
	}
	ownerID, computerID, runID, agentID := rec.OwnerID, rec.ComputerID, rec.RunID, rec.AgentID
	assignmentID := ""
	attempt := uint64(0)
	if agentProfileForRun(rec) == agentprofile.Engineering {
		assignmentID = metadataStringValue(rec.Metadata, "assignment_id")
		attempt = uint64(metadataIntValue(rec.Metadata, "assignment_attempt"))
	}
	subject := "run " + runID
	if assignmentID != "" {
		subject = "run " + runID + " of assignment " + assignmentID
	}
	// resolve reads the assignment at each observation, so one that ended
	// during the park is seen ended (review r1 Devin F2).
	resolve := func(ctx context.Context) *types.EngineeringAssignment {
		if assignmentID == "" || attempt == 0 {
			return nil
		}
		var lookup interface {
			GetEngineeringAssignment(context.Context, string, string, string, uint64) (types.EngineeringAssignment, error)
		} = rt.assignmentLookup
		if lookup == nil && rt.store != nil {
			lookup = rt.store
		}
		if lookup == nil {
			return nil
		}
		lctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		a, err := lookup.GetEngineeringAssignment(lctx, ownerID, computerID, assignmentID, attempt)
		if err != nil {
			log.Printf("runtime: provider quota wait of run %s: assignment %s: %v", runID, assignmentID, err)
			return nil
		}
		return &a
	}
	// seq orders the park's observations and its end: the announce timer's
	// observation and the end never interleave, and nothing is observed
	// after the end, so an ended wait is never brought back (review r1
	// Devin F1).
	var seq sync.Mutex
	parkEnded := false
	observe := func(ctx context.Context, w *types.WaitState) {
		seq.Lock()
		defer seq.Unlock()
		if parkEnded {
			return
		}
		if a := resolve(ctx); a != nil {
			rt.observeAssignmentWait(ctx, *a, func(since time.Time) *types.WaitState {
				c := w.Clone()
				if !since.IsZero() && since.Before(c.Since) {
					c.Since = since
				}
				return c
			})
		}
	}
	return func(ctx context.Context, w *types.WaitState, ended string) {
		t := rt.providerWaits()
		if !w.Active() {
			// The progress deadline runs again, moved out by the park.
			activationDeadlineFrom(ctx).resume()
			t.mu.Lock()
			e := t.entries[runID]
			delete(t.entries, runID)
			if e != nil && e.announce != nil {
				e.announce.Stop()
			}
			t.mu.Unlock()
			if e != nil {
				ended = firstNonEmpty(ended, toolregistry.ProviderWaitResumed)
				log.Printf("runtime: run %s (%s) park on the %s quota ended after %s: %s",
					runID, agentID, types.ProviderDisplayName(e.wait.Provider), shortWaitDuration(time.Since(e.wait.Since)), ended)
				seq.Lock()
				parkEnded = true
				if assignmentID != "" {
					rt.endAssignmentWaitOf(context.WithoutCancel(ctx), assignmentID, attempt, subject, ended)
				}
				seq.Unlock()
			}
			return
		}
		// Waiting on the quota is not a stall: the park suspends the
		// activation's progress deadline, up to its hard wall.
		activationDeadlineFrom(ctx).suspend(w)
		seq.Lock()
		parkEnded = false
		seq.Unlock()
		w = w.Clone()
		w.Subject = subject
		t.mu.Lock()
		e := t.entries[runID]
		first := e == nil
		if first {
			e = &providerWaitEntry{ownerID: ownerID, computerID: computerID, runID: runID, agentID: agentID}
			t.entries[runID] = e
		}
		e.wait = w
		if assignmentID != "" && e.announce == nil && time.Since(w.Since) < waitAnnounceAfter {
			// The park's next re-check may be half an hour away; Texture
			// hears of the wait once it has lasted waitAnnounceAfter.
			e.announce = time.AfterFunc(waitAnnounceAfter-time.Since(w.Since)+time.Second, func() {
				t.mu.Lock()
				cur := t.entries[runID]
				var current *types.WaitState
				if cur == e {
					current = cur.wait.Clone()
				}
				t.mu.Unlock()
				if current != nil {
					observe(context.Background(), current)
				}
			})
		}
		t.mu.Unlock()
		if first {
			log.Printf("runtime: run %s (%s) parked: %s", runID, agentID, w.Sentence)
		}
		observe(ctx, w)
	}
}

// providerWaitViews is the computer's desk loops parked on a provider
// quota, oldest first, each stamped now.
func (rt *Runtime) providerWaitViews(ownerID, computerID string) []types.WaitState {
	if rt == nil {
		return nil
	}
	now := time.Now()
	t := rt.providerWaits()
	t.mu.Lock()
	var out []types.WaitState
	for _, e := range t.entries {
		if (ownerID != "" && e.ownerID != ownerID) || (computerID != "" && e.computerID != computerID) || !e.wait.Active() {
			continue
		}
		w := e.wait.Clone()
		w.Stamp(now)
		out = append(out, *w)
	}
	t.mu.Unlock()
	sort.Slice(out, func(i, j int) bool {
		if !out[i].Since.Equal(out[j].Since) {
			return out[i].Since.Before(out[j].Since)
		}
		return out[i].Subject < out[j].Subject
	})
	return out
}

func shortWaitDuration(d time.Duration) string {
	if d < time.Minute {
		return d.Round(time.Second).String()
	}
	return strings.TrimSuffix(d.Round(time.Minute).String(), "0s")
}

// modelReselectForRun re-reads rec's model from the computer's current
// model policy, as the run was resolved at its start (modelpolicy
// EnrichMetadata with the same role, overlay and thread), so a provider
// quota park ends when the owner or main moves the role to another model
// (2026-11-04: live8's policy moved the arms to OpenRouter at 05:10 while
// their loops stayed parked on Synthetic). A run whose model the policy did
// not choose (no llm_policy_source: the caller's explicit selection) is not
// re-read.
func (rt *Runtime) modelReselectForRun(rec *types.RunRecord) toolregistry.ToolLoopModelReselectFunc {
	if rt == nil || rec == nil || rt.modelPolicy == nil || metadataStringValue(rec.Metadata, modelpolicy.MetadataPolicySource) == "" {
		return nil
	}
	ownerID := rec.OwnerID
	base := cloneMetadata(rec.Metadata)
	for _, key := range []string{modelpolicy.MetadataProvider, modelpolicy.MetadataModel, modelpolicy.MetadataReasoningEffort,
		modelpolicy.MetadataMaxTokens, modelpolicy.MetadataPolicySource, modelpolicy.MetadataPolicyError, modelpolicy.MetadataSpec,
		modelpolicy.MetadataAssignment, modelpolicy.MetadataFallbacks} {
		delete(base, key)
	}
	role := firstNonEmptyString(metadataStringValue(rec.Metadata, runMetadataAgentRole), metadataStringValue(rec.Metadata, runMetadataAgentProfile))
	if role == "" {
		role = agentProfileForRun(rec)
	}
	// The policy changed for this run when the spec it names for the role
	// changed: comparing specs, not assignments, keeps a pool's hash of a
	// thread-less run (a random key) from reading as a change.
	var specMu sync.Mutex
	lastSpec := metadataStringValue(rec.Metadata, modelpolicy.MetadataSpec)
	var rejectMu sync.Mutex
	lastReject := ""
	runID := rec.RunID
	return func(ctx context.Context) (provideriface.LLMSelection, []provideriface.LLMSelection, bool) {
		// Strict: only a policy file that was read and parsed, and names the
		// role's model, may move the run. A missing, unreadable or invalid
		// file, a bad overlay, or an unknown pool keeps the run on its model
		// (owner rule: a pinned model fails rather than falls back).
		meta, err := rt.modelPolicy.ResolveStrict(ownerID, role, base)
		if err != nil {
			rejectMu.Lock()
			first := err.Error() != lastReject
			lastReject = err.Error()
			rejectMu.Unlock()
			if first {
				log.Printf("runtime: run %s keeps its model while parked: model policy not usable: %v", runID, err)
			}
			return provideriface.LLMSelection{}, nil, false
		}
		spec := metadataStringValue(meta, modelpolicy.MetadataSpec)
		specMu.Lock()
		same := spec == lastSpec
		specMu.Unlock()
		if same {
			return provideriface.LLMSelection{}, nil, false
		}
		primary := provideriface.ResolvedLLMConfigFromMetadata(meta)
		if primary.Provider == "" || primary.Model == "" {
			return provideriface.LLMSelection{}, nil, false
		}
		specMu.Lock()
		lastSpec = spec
		specMu.Unlock()
		return primary, provideriface.FallbackLLMConfigsFromMetadata(meta, primary), true
	}
}
