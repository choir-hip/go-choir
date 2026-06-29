package runtime

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

type traceTrajectoryListResponse struct {
	Trajectories []traceTrajectorySummary `json:"trajectories"`
}

type traceTrajectorySnapshotResponse struct {
	Trajectory  traceTrajectorySummary      `json:"trajectory"`
	Agents      []traceAgentNode            `json:"agents"`
	Edges       []traceAgentEdge            `json:"edges"`
	Moments     []traceMomentSummary        `json:"moments"`
	Search      traceSearchSummary          `json:"search"`
	Acceptances []types.RunAcceptanceRecord `json:"acceptances,omitempty"`
}

type traceMomentDetailResponse struct {
	TrajectoryID string              `json:"trajectory_id"`
	Moment       traceMomentSummary  `json:"moment"`
	Events       []types.EventRecord `json:"events"`
}

type traceTrajectorySummary struct {
	TrajectoryID         string         `json:"trajectory_id"`
	Title                string         `json:"title"`
	State                types.RunState `json:"state,omitempty"`
	Live                 bool           `json:"live"`
	LatestActivityAt     string         `json:"latest_activity_at,omitempty"`
	LeadAgents           []string       `json:"lead_agents,omitempty"`
	AgentCount           int            `json:"agent_count"`
	DelegationCount      int            `json:"delegation_count"`
	MomentCount          int            `json:"moment_count"`
	MessageCount         int            `json:"message_count"`
	LatestStreamSeq      int64          `json:"latest_stream_seq,omitempty"`
	SearchAttemptCount   int            `json:"search_attempt_count,omitempty"`
	SearchSuccessCount   int            `json:"search_success_count,omitempty"`
	SearchRateLimitCount int            `json:"search_rate_limit_count,omitempty"`
}

type traceAgentNode struct {
	AgentID          string         `json:"agent_id"`
	Label            string         `json:"label"`
	Profile          string         `json:"profile,omitempty"`
	Role             string         `json:"role,omitempty"`
	State            types.RunState `json:"state,omitempty"`
	RunCount         int            `json:"run_count"`
	LatestActivityAt string         `json:"latest_activity_at,omitempty"`
	Entry            bool           `json:"entry"`
}

type traceAgentEdge struct {
	FromAgentID      string `json:"from_agent_id"`
	ToAgentID        string `json:"to_agent_id"`
	DelegationCount  int    `json:"delegation_count"`
	LatestActivityAt string `json:"latest_activity_at,omitempty"`
}

type traceMomentSummary struct {
	MomentID     string          `json:"moment_id"`
	StreamSeq    int64           `json:"stream_seq"`
	Timestamp    string          `json:"timestamp"`
	Kind         types.EventKind `json:"kind"`
	Phase        string          `json:"phase,omitempty"`
	RunID        string          `json:"loop_id,omitempty"`
	AgentID      string          `json:"agent_id,omitempty"`
	AgentLabel   string          `json:"agent_label,omitempty"`
	AgentProfile string          `json:"agent_profile,omitempty"`
	AgentRole    string          `json:"agent_role,omitempty"`
	ChannelID    string          `json:"channel_id,omitempty"`
	Summary      string          `json:"summary"`
	Tone         string          `json:"tone"`
	HasDetail    bool            `json:"has_detail"`
	MessageSeq   int64           `json:"message_seq,omitempty"`
}

type traceTrajectoryBundle struct {
	Trajectory  traceTrajectorySummary
	Agents      []traceAgentNode
	Edges       []traceAgentEdge
	Moments     []traceMomentSummary
	Search      traceSearchSummary
	Acceptances []types.RunAcceptanceRecord
	events      []types.EventRecord
}

func (h *APIHandler) HandleTraceTrajectories(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}

	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/trace/trajectories")
	if path == "" || path == "/" {
		h.handleTraceTrajectoryIndex(w, r, ownerID)
		return
	}

	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "trajectory_id is required"})
		return
	}
	trajectoryID := strings.TrimSpace(parts[0])

	switch {
	case len(parts) == 1:
		h.handleTraceTrajectorySnapshot(w, r, ownerID, trajectoryID)
	case len(parts) == 3 && parts[1] == "moments":
		h.handleTraceTrajectoryMomentDetail(w, r, ownerID, trajectoryID, strings.TrimSpace(parts[2]))
	default:
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "trace route not found"})
	}
}

func (h *APIHandler) handleTraceTrajectoryIndex(w http.ResponseWriter, r *http.Request, ownerID string) {
	limit := traceLimit(r, 200, 1000)
	runs, err := h.rt.ListRunsByOwner(r.Context(), ownerID, limit)
	if err != nil {
		log.Printf("runtime trace: list owner runs: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list trajectories"})
		return
	}
	events, err := h.rt.Store().ListEventsByOwner(r.Context(), ownerID, limit*4)
	if err != nil {
		log.Printf("runtime trace: list owner events: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list trajectories"})
		return
	}

	writeAPIJSON(w, http.StatusOK, traceTrajectoryListResponse{
		Trajectories: buildTraceTrajectoryIndex(runs, events),
	})
}

func (h *APIHandler) handleTraceTrajectorySnapshot(w http.ResponseWriter, r *http.Request, ownerID, trajectoryID string) {
	bundle, err := h.loadTraceTrajectoryBundle(r.Context(), ownerID, trajectoryID)
	if err != nil {
		if err == store.ErrNotFound {
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "trajectory not found"})
			return
		}
		log.Printf("runtime trace: load trajectory snapshot: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to load trajectory"})
		return
	}
	writeAPIJSON(w, http.StatusOK, traceTrajectorySnapshotResponse{
		Trajectory:  bundle.Trajectory,
		Agents:      bundle.Agents,
		Edges:       bundle.Edges,
		Moments:     bundle.Moments,
		Search:      bundle.Search,
		Acceptances: bundle.Acceptances,
	})
}

func (h *APIHandler) handleTraceTrajectoryMomentDetail(w http.ResponseWriter, r *http.Request, ownerID, trajectoryID, momentID string) {
	if momentID == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "moment_id is required"})
		return
	}
	bundle, err := h.loadTraceTrajectoryBundle(r.Context(), ownerID, trajectoryID)
	if err != nil {
		if err == store.ErrNotFound {
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "trajectory not found"})
			return
		}
		log.Printf("runtime trace: load moment detail: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to load moment detail"})
		return
	}
	for _, ev := range bundle.events {
		if ev.EventID != momentID {
			continue
		}
		writeAPIJSON(w, http.StatusOK, traceMomentDetailResponse{
			TrajectoryID: trajectoryID,
			Moment:       buildTraceMomentSummary(ev, traceAgentIndex(bundle.Agents)),
			Events:       []types.EventRecord{ev},
		})
		return
	}
	writeAPIJSON(w, http.StatusNotFound, apiError{Error: "moment not found"})
}

func (h *APIHandler) loadTraceTrajectoryBundle(ctx context.Context, ownerID, trajectoryID string) (traceTrajectoryBundle, error) {
	runs, err := h.rt.ListRunsByOwner(ctx, ownerID, 1000)
	if err != nil {
		return traceTrajectoryBundle{}, fmt.Errorf("list owner runs: %w", err)
	}
	filteredRuns := make([]types.RunRecord, 0)
	for _, run := range runs {
		if traceTrajectoryIDForRun(run) == trajectoryID {
			filteredRuns = append(filteredRuns, run)
		}
	}
	events, err := h.rt.Store().ListEventsByTrajectory(ctx, ownerID, trajectoryID, 2000)
	if err != nil {
		return traceTrajectoryBundle{}, fmt.Errorf("list trajectory events: %w", err)
	}
	messages, err := h.rt.Store().ListChannelMessagesByTrajectory(ctx, ownerID, trajectoryID, 1000)
	if err != nil {
		return traceTrajectoryBundle{}, fmt.Errorf("list trajectory messages: %w", err)
	}
	acceptances, err := h.rt.Store().ListRunAcceptancesByTrajectory(ctx, ownerID, trajectoryID, 100)
	if err != nil {
		return traceTrajectoryBundle{}, fmt.Errorf("list trajectory run acceptances: %w", err)
	}
	if len(filteredRuns) == 0 && len(events) == 0 && len(messages) == 0 && len(acceptances) == 0 {
		return traceTrajectoryBundle{}, store.ErrNotFound
	}

	agents := buildTraceAgentNodes(filteredRuns)
	edges := buildTraceAgentEdges(filteredRuns)
	moments := buildTraceMomentSummaries(events, traceAgentIndex(agents))
	search := buildTraceSearchSummary(events)
	trajectory := buildTraceTrajectorySummary(trajectoryID, filteredRuns, agents, edges, moments, len(messages), search)

	return traceTrajectoryBundle{
		Trajectory:  trajectory,
		Agents:      agents,
		Edges:       edges,
		Moments:     moments,
		Search:      search,
		Acceptances: acceptances,
		events:      events,
	}, nil
}

func traceLimit(r *http.Request, fallback, max int) int {
	limit := fallback
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= max {
			limit = n
		}
	}
	return limit
}
