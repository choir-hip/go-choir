package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// beadsDependency mirrors one edge in a beads issue's dependencies[] array.
type beadsDependency struct {
	IssueID     string `json:"issue_id"`
	DependsOnID string `json:"depends_on_id"`
	Type        string `json:"type"`
}

// beadsIssue maps the fields of one record in .beads/issues.jsonl.
// Only the fields doccheck needs are mapped; unknown fields are ignored.
type beadsIssue struct {
	Type         string            `json:"_type"`
	ID           string            `json:"id"`
	Title        string            `json:"title"`
	Description  string            `json:"description"`
	Status       string            `json:"status"`
	Priority     int               `json:"priority"`
	IssueType    string            `json:"issue_type"`
	Labels       []string          `json:"labels"`
	Parent       string            `json:"parent"`
	Dependencies []beadsDependency `json:"dependencies"`
	CreatedAt    string            `json:"created_at"`
	UpdatedAt    string            `json:"updated_at"`
	ClosedAt     string            `json:"closed_at"`
	CloseReason  string            `json:"close_reason"`
	CommentCount int               `json:"comment_count"`
}

// readBeadsJSONL parses .beads/issues.jsonl line-by-line. Each non-blank line
// must be a single JSON object. Blank lines are tolerated. A malformed line
// returns a clear error naming the 1-based line number.
func readBeadsJSONL(path string) ([]beadsIssue, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("read beads store: %w", err)
	}
	defer f.Close()

	var issues []beadsIssue
	scanner := bufio.NewScanner(f)
	// beads records carry long design/notes fields; raise the line limit.
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	line := 0
	for scanner.Scan() {
		line++
		raw := strings.TrimSpace(scanner.Text())
		if raw == "" {
			continue
		}
		var issue beadsIssue
		if err := json.Unmarshal([]byte(raw), &issue); err != nil {
			return nil, fmt.Errorf("beads store %s: malformed JSON on line %d: %w", path, line, err)
		}
		issues = append(issues, issue)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("beads store %s: %w", path, err)
	}
	return issues, nil
}

// beadsParent returns the parent epic id of an issue. It prefers the explicit
// parent field; if absent (the .beads JSONL export omits it), it falls back to
// the issue's "parent-child" dependency edge, whose depends_on_id is the epic.
func beadsParent(issue beadsIssue) string {
	if issue.Parent != "" {
		return issue.Parent
	}
	for _, dep := range issue.Dependencies {
		if dep.Type == "parent-child" && dep.IssueID == issue.ID {
			return dep.DependsOnID
		}
	}
	return ""
}

// beadsEpicChildren returns the issues parented to epicID, preserving input
// order. Parentage is resolved via beadsParent (explicit field or parent-child
// dependency edge).
func beadsEpicChildren(issues []beadsIssue, epicID string) []beadsIssue {
	var children []beadsIssue
	for _, issue := range issues {
		if beadsParent(issue) == epicID {
			children = append(children, issue)
		}
	}
	return children
}

// beadsVariant returns V, the count of non-closed children of epicID. This is
// the migration's key derived metric: V=0 is the descent gate (G1) for closing
// an epic.
func beadsVariant(issues []beadsIssue, epicID string) int {
	v := 0
	for _, child := range beadsEpicChildren(issues, epicID) {
		if child.Status != "closed" {
			v++
		}
	}
	return v
}
