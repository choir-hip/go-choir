package agentcore

import (
	"context"
	"net/http"
	"sort"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// SL slice 3 (O1): GET /api/runtime/obligations answers "what is owed on
// this computer and why is it not moving" for the owner, without SSH. It
// reads only bounded, indexed lists; counts are capped by obligationsScanCap.

const obligationsScanCap = 1000

type obligationsResponse struct {
	ComputerID  string              `json:"computer_id"`
	GeneratedAt time.Time           `json:"generated_at"`
	Restart     obligationsRestart  `json:"restart"`
	Wakes       obligationsWakes    `json:"wakes"`
	Runs        map[string]int      `json:"runs"`
	OldestRun   map[string]string   `json:"oldest_run_at,omitempty"`
	WorkItems   obligationsCountAge `json:"open_work_items"`
	Actors      *obligationsActors  `json:"actors,omitempty"`
	Errors      []string            `json:"errors,omitempty"`
}

// ActorObligations is the actor tape's share of what is owed: unprocessed
// actor updates (due now or deferred) and activations in flight. The actor
// runtime adapter binds the reader; agentcore does not import the kernel.
type ActorObligations struct {
	Due           int
	Deferred      int
	MaxDeferCount int
	OldestCreated time.Time
	NextNotBefore time.Time
	ByKind        map[string]int
	Samples       []ActorObligationSample
	Truncated     bool
	InFlight      map[string]time.Time
}

// ActorObligationSample is one unprocessed actor update.
type ActorObligationSample struct {
	UpdateID   string
	ToAgentID  string
	Kind       string
	CreatedAt  time.Time
	NotBefore  time.Time
	DeferCount int
	Attempts   int
}

// SetActorObligationsReader binds the actor tape reader for the "what is
// owed" surface.
func (rt *Runtime) SetActorObligationsReader(fn func(context.Context) (ActorObligations, error)) {
	rt.deliveryHooksMu.Lock()
	defer rt.deliveryHooksMu.Unlock()
	rt.actorObligations = fn
}

type obligationsActors struct {
	Due           int                     `json:"due"`
	Deferred      int                     `json:"deferred"`
	MaxDeferCount int                     `json:"max_defer_count"`
	OldestCreated string                  `json:"oldest_unprocessed_at,omitempty"`
	NextNotBefore string                  `json:"next_not_before,omitempty"`
	ByKind        map[string]int          `json:"by_kind,omitempty"`
	Samples       []obligationsActorEvent `json:"samples,omitempty"`
	Truncated     bool                    `json:"truncated,omitempty"`
	InFlight      []obligationsInFlight   `json:"in_flight,omitempty"`
}

type obligationsActorEvent struct {
	UpdateID   string `json:"update_id"`
	ToAgentID  string `json:"to_agent_id"`
	Kind       string `json:"kind"`
	CreatedAt  string `json:"created_at,omitempty"`
	NotBefore  string `json:"not_before,omitempty"`
	DeferCount int    `json:"defer_count"`
	Attempts   int    `json:"attempts"`
}

type obligationsInFlight struct {
	AgentID    string `json:"agent_id"`
	StartedAt  string `json:"started_at"`
	AgeSeconds int    `json:"age_seconds"`
}

func rfc3339OrEmpty(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func (rt *Runtime) actorObligationsSection(ctx context.Context, now time.Time) (*obligationsActors, error) {
	rt.deliveryHooksMu.RLock()
	read := rt.actorObligations
	rt.deliveryHooksMu.RUnlock()
	if read == nil {
		return nil, nil
	}
	got, err := read(ctx)
	if err != nil {
		return nil, err
	}
	out := &obligationsActors{
		Due: got.Due, Deferred: got.Deferred, MaxDeferCount: got.MaxDeferCount,
		OldestCreated: rfc3339OrEmpty(got.OldestCreated), NextNotBefore: rfc3339OrEmpty(got.NextNotBefore),
		ByKind: got.ByKind, Truncated: got.Truncated,
	}
	for _, s := range got.Samples {
		out.Samples = append(out.Samples, obligationsActorEvent{
			UpdateID: s.UpdateID, ToAgentID: s.ToAgentID, Kind: s.Kind,
			CreatedAt: rfc3339OrEmpty(s.CreatedAt), NotBefore: rfc3339OrEmpty(s.NotBefore),
			DeferCount: s.DeferCount, Attempts: s.Attempts,
		})
	}
	for id, at := range got.InFlight {
		out.InFlight = append(out.InFlight, obligationsInFlight{AgentID: id, StartedAt: rfc3339OrEmpty(at), AgeSeconds: int(now.Sub(at).Seconds())})
	}
	sort.Slice(out.InFlight, func(i, j int) bool { return out.InFlight[i].AgeSeconds > out.InFlight[j].AgeSeconds })
	return out, nil
}

type obligationsRestart struct {
	Kind   string `json:"kind"`
	Reason string `json:"reason,omitempty"`
}

type obligationsWakes struct {
	Unprojected       int                        `json:"unprojected"`
	Retrying          int                        `json:"retrying"`
	OldestUnprojected string                     `json:"oldest_unprojected_at,omitempty"`
	UnprojectedByKind map[string]int             `json:"unprojected_by_kind,omitempty"`
	Exhausted         int                        `json:"exhausted"`
	ExhaustedSamples  []obligationsExhaustedWake `json:"exhausted_samples,omitempty"`
}

type obligationsExhaustedWake struct {
	Kind        string `json:"kind"`
	Target      string `json:"target_agent_id"`
	Attempts    int    `json:"attempts"`
	LastError   string `json:"last_error"`
	ExhaustedAt string `json:"exhausted_at,omitempty"`
}

type obligationsCountAge struct {
	Count  int    `json:"count"`
	Oldest string `json:"oldest_at,omitempty"`
}

// HandleObligations serves the owner's obligation-fate surface.
func (h *APIHandler) HandleObligations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	rt := h.rt
	ctx := r.Context()
	resp := obligationsResponse{ComputerID: rt.TextureComputerID(), GeneratedAt: time.Now().UTC(), Runs: map[string]int{}}
	if marker, planned := rt.BootWasPlannedRestart(); planned {
		resp.Restart = obligationsRestart{Kind: "planned", Reason: marker.Reason}
	} else {
		resp.Restart = obligationsRestart{Kind: "crash_or_stop"}
	}

	if wakes, err := rt.store.ListUnprojectedActorWakes(ctx); err != nil {
		resp.Errors = append(resp.Errors, "wakes: "+err.Error())
	} else {
		resp.Wakes.Unprojected = len(wakes)
		resp.Wakes.UnprojectedByKind = map[string]int{}
		var oldest time.Time
		for _, wake := range wakes {
			resp.Wakes.UnprojectedByKind[wake.Kind]++
			if !rt.wakeRetries.due(wake.CanonicalID) {
				resp.Wakes.Retrying++
			}
			if !wake.CreatedAt.IsZero() && (oldest.IsZero() || wake.CreatedAt.Before(oldest)) {
				oldest = wake.CreatedAt
			}
		}
		if !oldest.IsZero() {
			resp.Wakes.OldestUnprojected = oldest.UTC().Format(time.RFC3339)
		}
	}
	if exhausted, err := rt.store.ListExhaustedActorWakes(ctx); err != nil {
		resp.Errors = append(resp.Errors, "exhausted wakes: "+err.Error())
	} else {
		resp.Wakes.Exhausted = len(exhausted)
		for i, wake := range exhausted {
			if i == 20 {
				break
			}
			sample := obligationsExhaustedWake{Kind: wake.Kind, Target: wake.TargetAgentID, Attempts: wake.DispatchAttempts, LastError: wake.LastDispatchError}
			if !wake.ExhaustedAt.IsZero() {
				sample.ExhaustedAt = wake.ExhaustedAt.UTC().Format(time.RFC3339)
			}
			resp.Wakes.ExhaustedSamples = append(resp.Wakes.ExhaustedSamples, sample)
		}
	}

	states := []types.RunState{types.RunPending, types.RunRunning, types.RunPassivated}
	if runs, err := rt.store.ListRunsByOwnerStates(ctx, ownerID, rt.TextureComputerID(), states, obligationsScanCap); err != nil {
		resp.Errors = append(resp.Errors, "runs: "+err.Error())
	} else {
		oldest := map[string]time.Time{}
		for _, run := range runs {
			state := string(run.State)
			resp.Runs[state]++
			if at := run.UpdatedAt; !at.IsZero() && (oldest[state].IsZero() || at.Before(oldest[state])) {
				oldest[state] = at
			}
		}
		if len(oldest) > 0 {
			resp.OldestRun = map[string]string{}
			for state, at := range oldest {
				resp.OldestRun[state] = at.UTC().Format(time.RFC3339)
			}
		}
	}
	if items, err := rt.store.ListOpenAssignedLifecycleWorkItems(ctx, rt.TextureComputerID(), obligationsScanCap); err != nil {
		resp.Errors = append(resp.Errors, "work items: "+err.Error())
	} else {
		resp.WorkItems.Count = len(items)
		var oldest time.Time
		for _, item := range items {
			if !item.CreatedAt.IsZero() && (oldest.IsZero() || item.CreatedAt.Before(oldest)) {
				oldest = item.CreatedAt
			}
		}
		if !oldest.IsZero() {
			resp.WorkItems.Oldest = oldest.UTC().Format(time.RFC3339)
		}
	}
	if actors, err := rt.actorObligationsSection(ctx, resp.GeneratedAt); err != nil {
		resp.Errors = append(resp.Errors, "actors: "+err.Error())
	} else {
		resp.Actors = actors
	}
	writeAPIJSON(w, http.StatusOK, resp)
}
