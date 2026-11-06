package agentcore

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/lease"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// Leases (MA3.4). The lease service lives in internal/lease; this file is how
// the runtime uses it: the run's context carries the authority and the run's
// id (so the reader gate and the capture frame handler can ask), a finished
// run ends its leases, the boot pass ends every lease the last process left
// open, and a desk's lease request is made from the run record.

// Leases is the lease service, or nil on a runtime without a store. The
// autoputer's owner routes list and revoke through it.
func (rt *Runtime) Leases() *lease.Service {
	if rt == nil {
		return nil
	}
	return rt.leases
}

// leaseAuthority is the authority a run's context carries; a nil service
// attaches nothing.
func (rt *Runtime) leaseAuthority() objectgraph.LeaseAuthority {
	if rt == nil || rt.leases == nil {
		return nil
	}
	return rt.leases
}

// endRunLeases ends the leases of a run that finished or was closed.
func (rt *Runtime) endRunLeases(ctx context.Context, runID, reason string) {
	if rt == nil || rt.leases == nil || strings.TrimSpace(runID) == "" {
		return
	}
	if n := rt.leases.EndRun(ctx, runID, reason); n > 0 {
		log.Printf("runtime: ended %d lease(s) of run %s (%s)", n, runID, reason)
	}
}

// endLeasesAtBoot reads the leases the tape holds as active and ends all of
// them with the fate "interrupted by a restart". A lease is per-task and a
// run is never resumed with its lease, whether the restart crashed (the runs
// are closed) or was planned (a resumed run asks again).
func (rt *Runtime) endLeasesAtBoot(ctx context.Context) {
	if rt.leases == nil {
		return
	}
	ownerID := strings.TrimSpace(rt.selfdevRouteOwnerID)
	if ownerID == "" {
		ownerID = strings.TrimSpace(os.Getenv("CHOIR_OWNER_ID"))
	}
	computerID := strings.TrimSpace(rt.TextureComputerID())
	if ownerID == "" {
		return
	}
	if err := rt.leases.Load(ctx, ownerID, computerID); err != nil {
		log.Printf("runtime: leases at boot: load: %v", err)
		return
	}
	if n := rt.leases.EndAll(ctx, lease.ReasonRestart); n > 0 {
		log.Printf("runtime: ended %d lease(s) open before this boot as interrupted by a restart", n)
	}
}

// RequestLease is the host side of a desk's lease request. The desk, run,
// agent and model come from the run record, never from the cell; the cell
// supplies the scope, the purpose and the request fields (p_success, weight,
// settlement).
func (rt *Runtime) RequestLease(ctx context.Context, rec *types.RunRecord, scope lease.Scope, purpose string, duration time.Duration, fields *types.RequestFields) (lease.Record, error) {
	if rt.leases == nil {
		return lease.Record{}, lease.ErrRefused
	}
	return rt.leases.Request(ctx, lease.Request{
		OwnerID: rec.OwnerID, ComputerID: rec.ComputerID, Desk: rec.AgentProfile, RunID: rec.RunID,
		AgentID: rec.AgentID, ModelID: recordModelID(ctx, rec), Scope: scope, Purpose: purpose, Duration: duration, Fields: fields,
	})
}

// requestLeaseEgress serves choir.RequestLease from a desk cell. The holder
// is the session's desk role and the run is the cell's run, both bound by the
// frame layer; the cell supplies only the scope, the purpose and the request
// fields. The result is the lease as JSON; a refusal is an error the cell
// reads.
func (rt *Runtime) requestLeaseEgress(ctx context.Context, deskRole string, exec toolregistry.ExecutionContext, payload json.RawMessage) (json.RawMessage, error) {
	if rt == nil || rt.leases == nil {
		return nil, fmt.Errorf("choir.RequestLease: no lease service on this computer")
	}
	var req yaegikernel.RequestLeasePayload
	dec := json.NewDecoder(bytes.NewReader(payload))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		return nil, fmt.Errorf("choir.RequestLease: decode the call: %w", err)
	}
	rec := exec.RunRecord
	if rec == nil {
		rec = &types.RunRecord{RunID: exec.RunID, OwnerID: exec.OwnerID, ComputerID: exec.ComputerID, AgentID: exec.AgentID}
	}
	held := *rec
	held.AgentProfile = strings.TrimSpace(deskRole)
	if held.RunID == "" {
		held.RunID = exec.RunID
	}
	granted, err := rt.RequestLease(ctx, &held,
		lease.Scope{Kinds: req.Kinds, CaptureIDs: req.CaptureIDs, CaptureLabels: req.CaptureLabels},
		req.Purpose, time.Duration(req.DurationSeconds)*time.Second, req.Req)
	if err != nil {
		if errors.Is(err, lease.ErrRefused) {
			return nil, fmt.Errorf("choir.RequestLease: %w", err)
		}
		return nil, err
	}
	return json.Marshal(granted)
}
