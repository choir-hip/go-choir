//go:build linux

package capsule

import (
	"context"
	"sort"

	"github.com/yusefmosiah/go-choir/internal/visualcapture"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// stageVisualEvidence copies the capsule's evidence directory into
// <temporary>/evidence/visual/ (beside release/, never under it: V20) and
// validates it against the staged release (stageEvidenceFiles). It returns
// nil when the capsule captured nothing. Files are opened beneath the capsule
// root without following symlinks, as release files are.
func (e *Executor) stageVisualEvidence(ctx context.Context, rootFD int, agentRunID, capsuleID string, changes []FileChange, temporary string, release []FrozenReleaseFile) (*VisualEvidence, error) {
	names, err := visualEvidenceNames(changes)
	if err != nil || len(names) == 0 {
		return nil, err
	}
	open := func(name string) (evidenceInput, error) {
		in, err := openBeneath(rootFD, visualcapture.EvidencePrefix+name)
		if err != nil {
			return nil, err
		}
		return in, nil
	}
	return stageEvidenceFiles(ctx, names, open, temporary, release, e.capsuleJobViews(agentRunID, capsuleID))
}

// capsuleJobViews lists the run's jobs in one capsule, oldest first: the
// frozen capsule's own jobs, never another capsule's of the same run (a
// capture receipt must come from the capsule whose evidence it vouches for).
func (e *Executor) capsuleJobViews(agentRunID, capsuleID string) []yaegikernel.JobView {
	e.mu.RLock()
	var jobs []*capsuleJob
	for _, job := range e.jobs {
		if job.agentRunID == agentRunID && job.capsuleID == capsuleID {
			jobs = append(jobs, job)
		}
	}
	sort.Slice(jobs, func(i, k int) bool { return jobs[i].status.StartedAt.Before(jobs[k].status.StartedAt) })
	views := make([]yaegikernel.JobView, 0, len(jobs))
	for _, job := range jobs {
		views = append(views, jobView(job.status, job.receiptRef, job.logRef))
	}
	e.mu.RUnlock()
	return views
}
