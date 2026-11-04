package agentcore

import (
	"context"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Provider quota: switch, never pause
// (docs/problems/provider-quota-kills-desk-loops-2026-11-04.md; owner,
// 2026-11-04: "On quota issues, we shouldn't stop but switch models. Always
// continue, never pause.").
//
// A quota moves the run to the next model that can serve it inside the tool
// loop (toolregistry, WithContinuationChain and WithQuotaLedger) and the
// next call goes out at once; the switch is a run.retry model_switch event
// on the run's trace. Nothing in this file is a park.
//
// Only one case waits: every model of the chain out of quota at once. The
// tool loop then retries the chain a bounded number of times with a short
// backoff and tells the runtime through WithProviderIncident. The incident
// is one types.WaitState of kind provider_quota, kept in memory here:
//
//   - on the obligations surface and in every desk's pack
//     (ActingPack.ProviderWaits), Texture's included, so no desk reads a
//     silent loop;
//   - for an engineering assignment, as the assignment's wait
//     (engineering_waits.go): management's pack and, once it has lasted
//     waitAnnounceAfter, Texture's line through the engineering relay.
//
// The tracker is memory: a crash ends the incident with the run (the
// restart rule closes the run); nothing here resumes work.

type providerWaitTracker struct {
	mu      sync.Mutex
	entries map[string]*providerWaitEntry
}

type providerWaitEntry struct {
	ownerID, computerID, runID, agentID string
	wait                                *types.WaitState
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

// quotaContinuationChain is rec's tool loop's continuation chain: the
// models the computer's policy allows (modelpolicy ContinuationChain).
func (rt *Runtime) quotaContinuationChain(rec *types.RunRecord) toolregistry.ToolLoopContinuationFunc {
	if rt == nil || rt.modelPolicy == nil || rec == nil {
		return nil
	}
	ownerID := rec.OwnerID
	return func(ctx context.Context) []provideriface.LLMSelection {
		return rt.modelPolicy.ContinuationChain(ctx, ownerID)
	}
}

// providerQuotaIncidentHook is rec's tool loop's incident handler.
func (rt *Runtime) providerQuotaIncidentHook(rec *types.RunRecord) toolregistry.ToolLoopProviderIncidentFunc {
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
	// during the incident is seen ended.
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
			log.Printf("runtime: provider quota incident of run %s: assignment %s: %v", runID, assignmentID, err)
			return nil
		}
		return &a
	}
	return func(ctx context.Context, w *types.WaitState, ended string) {
		t := rt.providerWaits()
		if !w.Active() {
			t.mu.Lock()
			e := t.entries[runID]
			delete(t.entries, runID)
			t.mu.Unlock()
			if e != nil {
				log.Printf("runtime: run %s (%s): every-model quota incident ended after %s: %s",
					runID, agentID, shortWaitDuration(time.Since(e.wait.Since)), firstNonEmpty(ended, toolregistry.ProviderIncidentCleared))
				if assignmentID != "" {
					rt.endAssignmentWaitOf(context.WithoutCancel(ctx), assignmentID, attempt, subject, firstNonEmpty(ended, toolregistry.ProviderIncidentCleared))
				}
			}
			return
		}
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
		t.mu.Unlock()
		if first {
			log.Printf("runtime: run %s (%s): every model is out of quota: %s", runID, agentID, w.Sentence)
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
}

// providerWaitViews is the computer's runs in an every-model quota
// incident, oldest first, each stamped now.
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
