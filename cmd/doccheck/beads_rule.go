package main

import (
	"fmt"
	"sort"
	"strings"
)

// validateBeadsStore implements rule R8: report-only validation of the beads
// mission store (.beads/issues.jsonl). It is hermetic — it parses the JSONL
// export directly and never shells out to `bd`. If the store is absent it
// emits a single info note and skips, so doccheck still works in checkouts
// without beads.
//
// R8 reports (never fails the run):
//  1. dangling dependency edges (a dep target that does not exist)
//  2. dependency cycles among issues
//  3. binding drift: count of mg:* epics vs mission-graph.yaml node count
//  4. V-consistency: V (open children) for every epic that has children
func validateBeadsStore(beadsPath string, graphNodeCount int) []warning {
	var warnings []warning

	issues, err := readBeadsJSONL(beadsPath)
	if err != nil {
		// Absent or unreadable store is not a failure — report-only, skip.
		warnings = append(warnings, warning{Rule: "R8", Severity: "info", Path: beadsPath,
			Message: fmt.Sprintf("beads store not validated: %v", err),
			Hint:    "run `go run ./cmd/mgimport` to populate, or ignore in checkouts without beads"})
		return warnings
	}

	byID := make(map[string]bool, len(issues))
	for _, is := range issues {
		byID[is.ID] = true
	}

	// (1) dangling dependency edges
	for _, is := range issues {
		for _, dep := range is.Dependencies {
			if dep.DependsOnID != "" && !byID[dep.DependsOnID] {
				warnings = append(warnings, warning{Rule: "R8", Severity: "warning", Path: beadsPath,
					Message: fmt.Sprintf("beads issue %q has a %s edge to unknown issue %q", is.ID, dep.Type, dep.DependsOnID),
					Hint:    "the dependency target is missing from the store"})
			}
		}
	}

	// (2) cycle detection over blocks/blocked-by edges (parent-child excluded)
	if cycle := beadsDependencyCycle(issues, byID); len(cycle) > 0 {
		warnings = append(warnings, warning{Rule: "R8", Severity: "warning", Path: beadsPath,
			Message: fmt.Sprintf("beads dependency cycle detected: %s", strings.Join(cycle, " -> "))})
	}

	// (3) binding drift: mg:* epics vs mission-graph node count
	mgEpics := 0
	for _, is := range issues {
		if is.IssueType == "epic" && strings.HasPrefix(is.ExternalRef, "mg:") {
			mgEpics++
		}
	}
	if graphNodeCount > 0 && mgEpics != graphNodeCount {
		warnings = append(warnings, warning{Rule: "R8", Severity: "info", Path: beadsPath,
			Message: fmt.Sprintf("beads/mission-graph binding drift: %d mg:* epics vs %d graph nodes", mgEpics, graphNodeCount),
			Hint:    "re-run `go run ./cmd/mgimport` to converge the shadow"})
	}

	// (4) V-consistency for every epic that has children
	for _, is := range issues {
		if is.IssueType != "epic" {
			continue
		}
		if len(beadsEpicChildren(issues, is.ID)) == 0 {
			continue
		}
		v := beadsVariant(issues, is.ID)
		sev := "info"
		if v == 0 && is.Status != "closed" {
			// descent gate (G1) satisfied but epic still open — worth noting.
			sev = "warning"
		}
		warnings = append(warnings, warning{Rule: "R8", Severity: sev, Path: beadsPath,
			Message: fmt.Sprintf("epic %q V=%d (open conjectures)", is.ID, v)})
	}

	return warnings
}

// beadsDependencyCycle returns a cycle path over blocks/blocked-by edges, or nil.
// parent-child edges are excluded (hierarchy is not a blocking relation).
func beadsDependencyCycle(issues []beadsIssue, byID map[string]bool) []string {
	// adjacency: issue -> issues it depends on (is blocked by)
	adj := make(map[string][]string, len(issues))
	ids := make([]string, 0, len(issues))
	for _, is := range issues {
		ids = append(ids, is.ID)
		for _, dep := range is.Dependencies {
			if dep.Type == "parent-child" {
				continue
			}
			if dep.IssueID == is.ID && byID[dep.DependsOnID] {
				adj[is.ID] = append(adj[is.ID], dep.DependsOnID)
			}
		}
	}
	sort.Strings(ids)

	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := make(map[string]int, len(ids))
	var stack []string
	var found []string

	var dfs func(string) bool
	dfs = func(n string) bool {
		color[n] = gray
		stack = append(stack, n)
		neighbors := append([]string(nil), adj[n]...)
		sort.Strings(neighbors)
		for _, m := range neighbors {
			if color[m] == gray {
				// cycle: slice stack from first occurrence of m
				for i, s := range stack {
					if s == m {
						found = append(append([]string(nil), stack[i:]...), m)
						return true
					}
				}
			}
			if color[m] == white && dfs(m) {
				return true
			}
		}
		stack = stack[:len(stack)-1]
		color[n] = black
		return false
	}

	for _, id := range ids {
		if color[id] == white && dfs(id) {
			return found
		}
	}
	return nil
}
