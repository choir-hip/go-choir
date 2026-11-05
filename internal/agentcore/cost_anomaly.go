package agentcore

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/llmcost"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/trust"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The cost-anomaly notice (modular architecture MA7.7, trust draft 9). Spend
// has no ceiling (owner, R8); spend far above a desk's own history notifies
// the owner, and the notice is a system record to the management inbox that
// wakes no one. The scan runs after a run ends, at most once per
// costWatchEvery for an owner, and a repeat of the same window commits the
// same record id, so a scan is idempotent.

const (
	costWatchEvery  = 10 * time.Minute
	costWatchEvents = 10000
)

// scanCostAnomalies reads the owner's recent cost entries, finds the desks
// whose spend in the last window is far above their own history, and commits
// one notice per anomaly to the owner's management inbox.
func (rt *Runtime) scanCostAnomalies(ctx context.Context, ownerID, computerID string, now time.Time) ([]trust.Anomaly, error) {
	if rt == nil || rt.store == nil || strings.TrimSpace(ownerID) == "" {
		return nil, nil
	}
	events, err := rt.store.ListEventsByOwner(store.WithReadConsistency(ctx, store.ReadRelaxed), ownerID, costWatchEvents)
	if err != nil {
		return nil, err
	}
	points := trust.PointsFromEntries(llmcost.ExtractCostEntries(events))
	anomalies := trust.DetectCostAnomalies(points, now, trust.DefaultAnomalyParams(), nil)
	inbox := persistentManagementAgentID(ownerID)
	for _, a := range anomalies {
		notice := a.Notice(inbox, computerID)
		req := types.CommitInboxRecordsRequest{OwnerID: ownerID, ComputerID: computerID, CommandID: "cost-anomaly:" + notice.RecordID, Records: []types.CommitmentRecord{notice}}
		if req.CommandDigest, err = store.ComputeCommitInboxRecordsDigest(req); err != nil {
			return anomalies, err
		}
		if _, err := rt.store.CommitInboxRecords(ctx, req); err != nil {
			// A continuing anomaly in the same hour has the same record id and
			// a new digest (its figures moved): the notice is already
			// committed, and a later desk's notice must still go.
			if errors.Is(err, store.ErrLifecycleCommandConflict) {
				continue
			}
			return anomalies, err
		}
	}
	return anomalies, nil
}

// watchCostAfterRun is the run-end trigger: it scans at most once per
// costWatchEvery per owner, in the background, and only logs a failure (the
// notice is a convenience, never a gate on the run).
func (rt *Runtime) watchCostAfterRun(rec *types.RunRecord) {
	if rt == nil || rec == nil || strings.TrimSpace(rec.OwnerID) == "" {
		return
	}
	now := time.Now().UTC()
	rt.costWatchMu.Lock()
	if last, ok := rt.costWatchLast[rec.OwnerID]; ok && now.Sub(last) < costWatchEvery {
		rt.costWatchMu.Unlock()
		return
	}
	if rt.costWatchLast == nil {
		rt.costWatchLast = map[string]time.Time{}
	}
	rt.costWatchLast[rec.OwnerID] = now
	rt.costWatchMu.Unlock()
	owner, computer := rec.OwnerID, rec.ComputerID
	rt.wg.Add(1)
	go func() {
		defer rt.wg.Done()
		if _, err := rt.scanCostAnomalies(context.Background(), owner, computer, now); err != nil {
			log.Printf("runtime: cost anomaly scan: %v", err)
		}
	}()
}
