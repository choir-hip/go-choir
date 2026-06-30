package runtime

import (
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func buildTraceAgentNodes(runs []types.RunRecord) []traceAgentNode {
	type agg struct {
		traceAgentNode
		latestAt time.Time
	}
	byAgent := make(map[string]*agg)
	childAgents := make(map[string]bool)
	for _, run := range runs {
		agentID := traceAgentID(run)
		entry := byAgent[agentID]
		if entry == nil {
			entry = &agg{traceAgentNode: traceAgentNode{
				AgentID:  agentID,
				Label:    traceAgentLabel(run),
				Profile:  traceRunProfile(run),
				Role:     traceRunRole(run),
				State:    run.State,
				RunCount: 0,
			}}
			byAgent[agentID] = entry
		}
		runAt := traceRunActivityTime(run)
		if runAt.After(entry.latestAt) {
			entry.latestAt = runAt
			entry.State = run.State
			entry.Label = traceAgentLabel(run)
			entry.Profile = traceRunProfile(run)
			entry.Role = traceRunRole(run)
		}
		entry.RunCount++
		if strings.TrimSpace(run.RequestedByRunID) != "" {
			childAgents[agentID] = true
		}
	}

	agents := make([]traceAgentNode, 0, len(byAgent))
	for agentID, entry := range byAgent {
		entry.Entry = !childAgents[agentID]
		entry.LatestActivityAt = formatTraceTime(entry.latestAt)
		agents = append(agents, entry.traceAgentNode)
	}
	sort.Slice(agents, func(i, j int) bool {
		if agents[i].Entry != agents[j].Entry {
			return agents[i].Entry
		}
		return agents[i].LatestActivityAt < agents[j].LatestActivityAt
	})
	return agents
}

func traceAgentIndex(agents []traceAgentNode) map[string]traceAgentNode {
	index := make(map[string]traceAgentNode, len(agents))
	for _, agent := range agents {
		index[agent.AgentID] = agent
	}
	return index
}

func buildTraceAgentEdges(runs []types.RunRecord) []traceAgentEdge {
	parentRuns := make(map[string]types.RunRecord, len(runs))
	for _, run := range runs {
		parentRuns[run.RunID] = run
	}
	type edgeKey struct{ from, to string }
	type edgeAgg struct {
		count  int
		latest time.Time
	}
	agg := make(map[edgeKey]*edgeAgg)
	for _, run := range runs {
		parent, ok := parentRuns[strings.TrimSpace(run.RequestedByRunID)]
		if !ok {
			continue
		}
		key := edgeKey{from: traceAgentID(parent), to: traceAgentID(run)}
		entry := agg[key]
		if entry == nil {
			entry = &edgeAgg{}
			agg[key] = entry
		}
		entry.count++
		if runAt := traceRunActivityTime(run); runAt.After(entry.latest) {
			entry.latest = runAt
		}
	}

	edges := make([]traceAgentEdge, 0, len(agg))
	for key, entry := range agg {
		edges = append(edges, traceAgentEdge{
			FromAgentID:      key.from,
			ToAgentID:        key.to,
			DelegationCount:  entry.count,
			LatestActivityAt: formatTraceTime(entry.latest),
		})
	}
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].LatestActivityAt == edges[j].LatestActivityAt {
			return edges[i].FromAgentID < edges[j].FromAgentID
		}
		return edges[i].LatestActivityAt < edges[j].LatestActivityAt
	})
	return edges
}

func traceAgentID(run types.RunRecord) string {
	if id := strings.TrimSpace(run.AgentID); id != "" {
		return id
	}
	if role := traceRunRole(run); role != "" {
		return role + ":" + run.RunID
	}
	return "loop:" + run.RunID
}

func traceAgentLabel(run types.RunRecord) string {
	role := traceRunRole(run)
	profile := traceRunProfile(run)
	switch {
	case role != "" && profile != "" && role != profile:
		return role + " · " + profile
	case role != "":
		return role
	case profile != "":
		return profile
	default:
		return shortTraceID(traceAgentID(run))
	}
}

func traceRunActivityTime(run types.RunRecord) time.Time {
	if !run.UpdatedAt.IsZero() {
		return run.UpdatedAt
	}
	return run.CreatedAt
}
