package runtime

import (
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func buildTraceTrajectoryIndex(runs []types.RunRecord, events []types.EventRecord) []traceTrajectorySummary {
	groupedRuns := make(map[string][]types.RunRecord)
	for _, run := range runs {
		trajectoryID := traceTrajectoryIDForRun(run)
		if trajectoryID != "" {
			groupedRuns[trajectoryID] = append(groupedRuns[trajectoryID], run)
		}
	}
	groupedEvents := make(map[string][]types.EventRecord)
	for _, ev := range events {
		trajectoryID := strings.TrimSpace(ev.TrajectoryID)
		if trajectoryID != "" {
			groupedEvents[trajectoryID] = append(groupedEvents[trajectoryID], ev)
		}
	}

	summaries := make([]traceTrajectorySummary, 0, len(groupedRuns)+len(groupedEvents))
	seen := make(map[string]bool, len(groupedRuns)+len(groupedEvents))
	for trajectoryID, runGroup := range groupedRuns {
		agents := buildTraceAgentNodes(runGroup)
		edges := buildTraceAgentEdges(runGroup)
		moments := buildTraceMomentSummaries(groupedEvents[trajectoryID], traceAgentIndex(agents))
		search := buildTraceSearchSummary(groupedEvents[trajectoryID])
		summaries = append(summaries, buildTraceTrajectorySummary(trajectoryID, runGroup, agents, edges, moments, 0, search))
		seen[trajectoryID] = true
	}
	for trajectoryID, eventGroup := range groupedEvents {
		if seen[trajectoryID] {
			continue
		}
		moments := buildTraceMomentSummaries(eventGroup, nil)
		search := buildTraceSearchSummary(eventGroup)
		summaries = append(summaries, buildTraceTrajectorySummary(trajectoryID, nil, nil, nil, moments, 0, search))
	}
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].LatestActivityAt > summaries[j].LatestActivityAt
	})
	return summaries
}

func buildTraceTrajectorySummary(trajectoryID string, runs []types.RunRecord, agents []traceAgentNode, edges []traceAgentEdge, moments []traceMomentSummary, messageCount int, search traceSearchSummary) traceTrajectorySummary {
	latestAt := time.Time{}
	var latestRun types.RunRecord
	for _, run := range runs {
		runAt := traceRunActivityTime(run)
		if runAt.After(latestAt) {
			latestAt = runAt
			latestRun = run
		}
	}
	for _, moment := range moments {
		if ts := parseTraceTime(moment.Timestamp); ts.After(latestAt) {
			latestAt = ts
		}
	}
	latestStreamSeq := int64(0)
	for _, moment := range moments {
		if moment.StreamSeq > latestStreamSeq {
			latestStreamSeq = moment.StreamSeq
		}
	}
	state := traceTrajectoryState(runs, latestRun.State)
	return traceTrajectorySummary{
		TrajectoryID:         trajectoryID,
		Title:                traceTrajectoryTitle(latestRun, trajectoryID),
		State:                state,
		Live:                 traceStateLive(state),
		LatestActivityAt:     formatTraceTime(latestAt),
		LeadAgents:           traceLeadAgents(agents),
		AgentCount:           len(agents),
		DelegationCount:      len(edges),
		MomentCount:          len(moments),
		MessageCount:         messageCount,
		LatestStreamSeq:      latestStreamSeq,
		SearchAttemptCount:   search.Attempts,
		SearchSuccessCount:   search.Successes,
		SearchRateLimitCount: search.RateLimits,
	}
}

func traceLeadAgents(agents []traceAgentNode) []string {
	leadAgents := make([]string, 0)
	for _, agent := range agents {
		if agent.Entry {
			leadAgents = append(leadAgents, agent.Label)
		}
	}
	if len(leadAgents) == 0 {
		for _, agent := range agents {
			leadAgents = append(leadAgents, agent.Label)
		}
	}
	sort.Strings(leadAgents)
	return leadAgents
}

func traceTrajectoryState(runs []types.RunRecord, fallback types.RunState) types.RunState {
	for _, state := range []types.RunState{types.RunRunning, types.RunPending, types.RunBlocked} {
		for _, run := range runs {
			if run.State == state {
				return state
			}
		}
	}
	return fallback
}

func traceStateLive(state types.RunState) bool {
	return state == types.RunPending || state == types.RunRunning || state == types.RunBlocked
}

func traceTrajectoryTitle(run types.RunRecord, trajectoryID string) string {
	if title := traceExcerpt(run.Prompt, 72); title != "" {
		return title
	}
	return trajectoryID
}
