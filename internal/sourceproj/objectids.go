package sourceproj

import (
	"context"
	"fmt"
	"strings"
)

// ObjectIDs returns the git object id (a tree for a directory, a blob for a
// file) of each path as commit holds it, in order. It is the source identity
// the shot verdict reuse compares across two bases
// (docs/design/shot-verdict-reuse-2026-11-09.md section 12): equal ids are
// equal bytes, whatever else the commits changed. A commit that is not in
// repo, a path the commit does not hold, or an id that is not a full object
// id is an error, never an empty id, so an unreadable value cannot compare
// equal to another.
func ObjectIDs(ctx context.Context, repo, commit string, paths ...string) ([]string, error) {
	commit = strings.ToLower(strings.TrimSpace(commit))
	resolved, ok := resolveCommit(ctx, repo, commit)
	if !ok || resolved != commit {
		return nil, fmt.Errorf("the commit %q is not in the repository", commit)
	}
	ids := make([]string, 0, len(paths))
	for _, p := range paths {
		p = strings.Trim(strings.TrimSpace(p), "/")
		if p == "" || strings.HasPrefix(p, "-") || strings.Contains(p, "..") {
			return nil, fmt.Errorf("the path %q is not a path in a commit", p)
		}
		id, err := git(ctx, repo, nil, nil, "rev-parse", "--verify", "--quiet", resolved+":"+p)
		if err != nil || !commitID.MatchString(id) {
			return nil, fmt.Errorf("the commit %s holds no %s", commit[:12], p)
		}
		ids = append(ids, id)
	}
	return ids, nil
}
